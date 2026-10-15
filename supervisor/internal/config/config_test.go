package config

import (
	"strings"
	"testing"
	"time"
)

const valid = `
game: terraria
phases:
  - name: install
    run: [/opt/deploykit/hooks/install.sh]
start: [/opt/deploykit/hooks/start.sh]
console:
  adapter: stdin
ready:
  pattern: Server started
stop:
  command: exit
  timeout: 45s
verbs:
  save: [save]
`

func TestParseValid(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.User.UID != 1000 || cfg.DataDir != "/data" || cfg.ServerDir != "/server" || cfg.Workdir != "/data" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if cfg.Stop.Timeout != 45*time.Second {
		t.Fatalf("timeout = %v", cfg.Stop.Timeout)
	}
	if !cfg.ReadyRegexp.MatchString("Server started") {
		t.Fatal("ready regexp not compiled")
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"unknown field":   valid + "bogus: 1\n",
		"bad adapter":     strings.Replace(valid, "adapter: stdin", "adapter: telnet", 1),
		"bad verb":        strings.Replace(valid, "save: [save]", "restart: [x]", 1),
		"bad signal":      strings.Replace(valid, "command: exit", "signal: SIGKILL", 1),
		"bad pattern":     strings.Replace(valid, "Server started", "(", 1),
		"no start":        strings.Replace(valid, "start: [/opt/deploykit/hooks/start.sh]", "", 1),
		"rcon no port":    strings.Replace(valid, "adapter: stdin", "adapter: rcon", 1),
		"root user":       valid + "user: {uid: 0, gid: 5}\n",
		"data dir /etc":   valid + "dataDir: /etc\n",
		"server dir /usr": valid + "serverDir: /usr/local/bin\n",
		"command no cons": strings.Replace(strings.Replace(valid, "adapter: stdin", "adapter: none", 1), "verbs:\n  save: [save]\n", "", 1),
	}
	for name, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
