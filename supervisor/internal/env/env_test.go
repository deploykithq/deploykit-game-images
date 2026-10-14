package env

import (
	"errors"
	"testing"
)

func fakeSources(files map[string]string, mounts ...string) Sources {
	set := map[string]bool{}
	for _, m := range mounts {
		set[m] = true
	}
	return Sources{
		ReadFile: func(p string) ([]byte, error) {
			if v, ok := files[p]; ok {
				return []byte(v), nil
			}
			return nil, errors.New("missing")
		},
		IsMount: func(p string) bool { return set[p] },
	}
}

var in = Input{DataDir: "/data", ServerDir: "/server", RunDir: "/run/deploykit"}

func TestResolveMemoryFromCgroup(t *testing.T) {
	r := Resolve(nil, in, fakeSources(map[string]string{"/sys/fs/cgroup/memory.max": "4294967296\n"}))
	if Lookup(r.Env, "DK_MEMORY_MB") != "4096" || Lookup(r.Env, "DK_HEAP_MB") != "3072" {
		t.Fatalf("env = %v", r.Env)
	}
}

func TestResolveExplicitWins(t *testing.T) {
	r := Resolve([]string{"DK_MEMORY_MB=2048", "DK_HEAP_MB=1500"}, in,
		fakeSources(map[string]string{"/sys/fs/cgroup/memory.max": "4294967296"}))
	if Lookup(r.Env, "DK_MEMORY_MB") != "2048" || Lookup(r.Env, "DK_HEAP_MB") != "1500" {
		t.Fatalf("env = %v", r.Env)
	}
}

func TestResolveNoLimit(t *testing.T) {
	r := Resolve(nil, in, fakeSources(map[string]string{"/sys/fs/cgroup/memory.max": "max"}))
	if Lookup(r.Env, "DK_MEMORY_MB") != "" || Lookup(r.Env, "DK_HEAP_MB") != "1024" {
		t.Fatalf("env = %v", r.Env)
	}
}

func TestServerDirFallback(t *testing.T) {
	if r := Resolve(nil, in, fakeSources(nil, "/data")); r.ServerDir != "/data/.server" {
		t.Fatalf("server dir = %s", r.ServerDir)
	}
	if r := Resolve(nil, in, fakeSources(nil, "/data", "/server")); r.ServerDir != "/server" {
		t.Fatalf("server dir = %s", r.ServerDir)
	}
	if r := Resolve(nil, in, fakeSources(nil)); r.ServerDir != "/server" {
		t.Fatalf("server dir = %s", r.ServerDir)
	}
}

func TestRCONPasswordIsAlwaysGenerated(t *testing.T) {
	rc := in
	rc.RCON = true
	r := Resolve([]string{"DK_RCON_PASSWORD=leaked"}, rc, fakeSources(nil))
	if p := Lookup(r.Env, "DK_RCON_PASSWORD"); p == "" || p == "leaked" || len(p) != 32 {
		t.Fatalf("password = %q", p)
	}
	r = Resolve([]string{"DK_RCON_PASSWORD=leaked"}, in, fakeSources(nil))
	if Lookup(r.Env, "DK_RCON_PASSWORD") != "" {
		t.Fatal("password must not survive without rcon")
	}
}
