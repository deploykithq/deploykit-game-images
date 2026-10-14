package console

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

type recorder struct {
	mu   sync.Mutex
	sent []string
}

func (r *recorder) Send(c string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, c)
	return nil
}

func TestRouter(t *testing.T) {
	rec := &recorder{}
	var logs []string
	stopped := false
	r := &Router{
		Sender: rec,
		Verbs:  map[string][]string{"save": {"save-all flush"}},
		Status: func() string { return "status ok" },
		Stop:   func() { stopped = true },
		Log:    func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) },
	}
	r.Run(strings.NewReader("say hi\r\n\n/dk save\n/dk status\n/dk save-off\n/dk nope\n/dkx\n/dk stop\n"))
	want := []string{"say hi", "save-all flush", "/dkx"}
	if strings.Join(rec.sent, "|") != strings.Join(want, "|") {
		t.Fatalf("sent = %q", rec.sent)
	}
	if !stopped {
		t.Fatal("stop verb not handled")
	}
	joined := strings.Join(logs, "|")
	for _, w := range []string{"verb save ok", "status ok", "verb save-off unsupported", `unknown verb "nope"`} {
		if !strings.Contains(joined, w) {
			t.Errorf("logs %q missing %q", joined, w)
		}
	}
}

func TestPipe(t *testing.T) {
	var b bytes.Buffer
	p := &Pipe{W: &b}
	_ = p.Send("exit")
	if b.String() != "exit\n" {
		t.Fatalf("got %q", b.String())
	}
}

// fakeRCON is a Source-style server: empty RESPONSE_VALUE then AUTH_RESPONSE,
// and a split response for "long".
func fakeRCON(t *testing.T, password string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				for {
					id, typ, body, err := readPacket(c)
					if err != nil {
						return
					}
					switch typ {
					case typeAuth:
						_ = writePacket(c, id, typeResponse, "")
						if body == password {
							_ = writePacket(c, id, typeAuthResp, "")
						} else {
							_ = writePacket(c, -1, typeAuthResp, "")
						}
					case typeExec:
						if body == "long" {
							_ = writePacket(c, id, typeResponse, "part1\n")
							_ = writePacket(c, id, typeResponse, "part2")
						} else {
							_ = writePacket(c, id, typeResponse, "echo:"+body)
						}
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

func TestRCON(t *testing.T) {
	addr := fakeRCON(t, "pw")
	var out []string
	r := &RCON{Addr: addr, Password: "pw", Out: func(l string) { out = append(out, l) }}
	defer r.Close()
	if err := r.Send("status"); err != nil {
		t.Fatal(err)
	}
	if err := r.Send("long"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(out, "|") != "echo:status|part1|part2" {
		t.Fatalf("out = %q", out)
	}
	bad := &RCON{Addr: addr, Password: "wrong"}
	if err := bad.Send("x"); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("err = %v", err)
	}
}
