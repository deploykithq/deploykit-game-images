// Package env resolves the environment every hook and the game process see:
// the DK_* inputs DeployKit passes, completed with the values the image can
// work out for itself (directories, memory, a per-boot RCON password).
package env

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

// Sources abstracts the filesystem so Resolve can be tested.
type Sources struct {
	ReadFile func(string) ([]byte, error)
	// IsMount reports whether path is a mount point.
	IsMount func(string) bool
}

func OSSources() Sources {
	return Sources{ReadFile: os.ReadFile, IsMount: IsMount}
}

type Input struct {
	DataDir   string
	ServerDir string
	RunDir    string
	Home      string
	User      string
	RCON      bool
}

// Result is the resolved environment plus the directories the supervisor must
// create and hand to the game user.
type Result struct {
	Env       []string
	ServerDir string
}

// Resolve returns environ with the DK_* defaults applied. Values DeployKit set
// explicitly win, except DK_RCON_PASSWORD, which is always generated here: the
// RCON server is container-local and its password is never DeployKit's to know.
func Resolve(environ []string, in Input, src Sources) Result {
	m := toMap(environ)

	serverDir := in.ServerDir
	if !src.IsMount(in.ServerDir) && src.IsMount(in.DataDir) {
		serverDir = strings.TrimRight(in.DataDir, "/") + "/.server"
	}
	m["DK_DATA_DIR"] = in.DataDir
	m["DK_SERVER_DIR"] = serverDir
	m["DK_RUN_DIR"] = in.RunDir
	m["DK_STATE_DIR"] = strings.TrimRight(in.DataDir, "/") + "/.deploykit"
	if in.Home != "" {
		m["HOME"] = in.Home
	}
	if in.User != "" {
		m["USER"] = in.User
	}

	if m["DK_MEMORY_MB"] == "" {
		if mb := cgroupMemoryMB(src); mb > 0 {
			m["DK_MEMORY_MB"] = strconv.Itoa(mb)
		}
	}
	if m["DK_HEAP_MB"] == "" {
		heap := 1024
		if mb, err := strconv.Atoi(m["DK_MEMORY_MB"]); err == nil && mb > 0 {
			heap = mb * 3 / 4
		}
		m["DK_HEAP_MB"] = strconv.Itoa(heap)
	}

	delete(m, "DK_RCON_PASSWORD")
	if in.RCON {
		m["DK_RCON_PASSWORD"] = randomToken(16)
	}

	return Result{Env: fromMap(m), ServerDir: serverDir}
}

// cgroupMemoryMB reads the container's memory limit (cgroup v2, then v1).
// It returns 0 when there is no limit.
func cgroupMemoryMB(src Sources) int {
	for _, p := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		raw, err := src.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(raw))
		if s == "max" {
			return 0
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 || n >= 1<<60 {
			return 0
		}
		return int(n / (1024 * 1024))
	}
	return 0
}

// IsMount reports whether path is listed as a mount point in mountinfo.
func IsMount(path string) bool {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) > 4 && unescapeMount(fields[4]) == path {
			return true
		}
	}
	return false
}

func unescapeMount(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func toMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}

func fromMap(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

// Lookup returns the value of key in environ.
func Lookup(environ []string, key string) string {
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// installOnlyPrefixes are the inputs only the install and mods phases use. The game
// process does not get them: every mod runs inside it, and none has any
// business reading the owner's CurseForge API key.
var installOnlyPrefixes = []string{"DK_CURSEFORGE_API_KEY=", "DK_MODS_", "DK_MODPACK_"}

// ForGame returns environ without the install-only inputs.
func ForGame(environ []string) []string {
	out := make([]string, 0, len(environ))
next:
	for _, kv := range environ {
		for _, p := range installOnlyPrefixes {
			if strings.HasPrefix(kv, p) {
				continue next
			}
		}
		out = append(out, kv)
	}
	return out
}
