package lifecycle

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/deploykithq/deploykit-game-images/supervisor/internal/config"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/logx"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/proc"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/ready"
)

// The test binary doubles as the fake game: TestMain runs fakeGame when
// FAKE_GAME is set.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FAKE_GAME"); mode != "" {
		fakeGame(mode)
		return
	}
	os.Exit(m.Run())
}

func fakeGame(mode string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	fmt.Println("booting")
	fmt.Print("Server started\n> ")
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	for {
		select {
		case l := <-lines:
			switch l {
			case "exit":
				fmt.Println("saving world... bye")
				os.Exit(0)
			case "crash":
				os.Exit(3)
			default:
				fmt.Println("echo:" + l)
			}
		case s := <-sigs:
			if mode == "stubborn" {
				fmt.Println("ignoring", s)
				continue
			}
			fmt.Println("got", s)
			os.Exit(0)
		}
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var reaper = proc.NewReaper()

type harness struct {
	out    *syncBuf
	stdin  *io.PipeWriter
	sigs   chan os.Signal
	runDir string
	code   chan int
}

func start(t *testing.T, yaml, mode string) *harness {
	t.Helper()
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Start = []string{os.Args[0]}
	cfg.Workdir = t.TempDir()
	pr, pw := io.Pipe()
	h := &harness{out: &syncBuf{}, stdin: pw, sigs: make(chan os.Signal, 1), runDir: t.TempDir(), code: make(chan int, 1)}
	s := &Supervisor{
		Cfg:     cfg,
		Env:     append(os.Environ(), "FAKE_GAME="+mode),
		RunDir:  h.runDir,
		Stdin:   pr,
		Out:     logx.New(h.out),
		ErrOut:  h.out,
		Reaper:  reaper,
		Signals: h.sigs,
	}
	go func() { h.code <- s.Run() }()
	t.Cleanup(func() { pw.Close() })
	return h
}

func (h *harness) waitFor(t *testing.T, s string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(h.out.String(), s) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; output:\n%s", s, h.out.String())
}

func (h *harness) exit(t *testing.T) int {
	t.Helper()
	select {
	case c := <-h.code:
		return c
	case <-time.After(15 * time.Second):
		t.Fatalf("supervisor did not exit; output:\n%s", h.out.String())
		return -1
	}
}

const stdinGame = `
game: fake
phases:
  - name: install
    run: [sh, -c, "echo installing"]
start: [placeholder]
console: {adapter: stdin}
ready: {pattern: "Server started"}
stop: {command: exit, timeout: 2s}
verbs: {save: [save]}
`

func TestStdinGameLifecycle(t *testing.T) {
	h := start(t, stdinGame, "normal")
	h.waitFor(t, "[deploykit] ready")
	if !ready.Healthy(h.runDir) {
		t.Fatal("ready marker missing")
	}
	// The supervisor line must not be glued to the "> " prompt.
	if strings.Contains(h.out.String(), "> [deploykit]") {
		t.Fatalf("log line joined a partial line:\n%s", h.out.String())
	}
	fmt.Fprintln(h.stdin, "say hi")
	h.waitFor(t, "echo:say hi")
	fmt.Fprintln(h.stdin, "/dk save")
	h.waitFor(t, "echo:save")
	fmt.Fprintln(h.stdin, "/dk status")
	h.waitFor(t, "state=running ready=true")
	h.sigs <- syscall.SIGTERM
	if code := h.exit(t); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	out := h.out.String()
	for _, want := range []string{"phase=install", "installing", "stopping signal=terminated", "saving world", "stopped code=0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if ready.Healthy(h.runDir) {
		t.Fatal("ready marker survived the stop")
	}
}

func TestGameCrashPropagatesCode(t *testing.T) {
	h := start(t, stdinGame, "normal")
	h.waitFor(t, "[deploykit] ready")
	fmt.Fprintln(h.stdin, "crash")
	if code := h.exit(t); code != 3 {
		t.Fatalf("exit code %d", code)
	}
}

func TestStopVerb(t *testing.T) {
	h := start(t, stdinGame, "normal")
	h.waitFor(t, "[deploykit] ready")
	fmt.Fprintln(h.stdin, "/dk stop")
	if code := h.exit(t); code != 0 {
		t.Fatalf("exit code %d", code)
	}
}

func TestSignalStopEscalates(t *testing.T) {
	yaml := strings.Replace(stdinGame, "stop: {command: exit, timeout: 2s}", "stop: {signal: SIGINT, timeout: 300ms}", 1)
	h := start(t, yaml, "stubborn")
	h.waitFor(t, "[deploykit] ready")
	h.sigs <- syscall.SIGTERM
	code := h.exit(t)
	if code != 128+int(syscall.SIGKILL) {
		t.Fatalf("exit code %d, output:\n%s", code, h.out.String())
	}
	h.waitFor(t, "sending SIGKILL")
}

func TestFailingPhaseAborts(t *testing.T) {
	yaml := strings.Replace(stdinGame, `run: [sh, -c, "echo installing"]`, `run: [sh, -c, "exit 4"]`, 1)
	h := start(t, yaml, "normal")
	if code := h.exit(t); code != 4 {
		t.Fatalf("exit code %d", code)
	}
	h.waitFor(t, "phase=install failed code=4")
}

func TestSignalDuringPhase(t *testing.T) {
	yaml := strings.Replace(stdinGame, `run: [sh, -c, "echo installing"]`, `run: [sh, -c, "echo downloading; sleep 30"]`, 1)
	h := start(t, yaml, "normal")
	h.waitFor(t, "downloading")
	h.sigs <- syscall.SIGTERM
	if code := h.exit(t); code != 143 {
		t.Fatalf("exit code %d", code)
	}
}
