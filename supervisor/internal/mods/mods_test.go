package mods

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type fakeAPI struct {
	srv       *httptest.Server
	files     map[string][]byte // path -> content
	versions  map[string]mrVersion
	projects  map[string][]string // project -> version ids, newest first
	downloads atomic.Int32
}

func sha512hex(b []byte) string { s := sha512.Sum512(b); return hex.EncodeToString(s[:]) }
func sha1hex(b []byte) string   { s := sha1.Sum(b); return hex.EncodeToString(s[:]) }

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{files: map[string][]byte{}, versions: map[string]mrVersion{}, projects: map[string][]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasPrefix(p, "/files/"):
			f.downloads.Add(1)
			b, ok := f.files[p]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(b)
		case strings.HasPrefix(p, "/v2/version/"):
			v, ok := f.versions[strings.TrimPrefix(p, "/v2/version/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(v)
		case strings.HasPrefix(p, "/v2/project/") && strings.HasSuffix(p, "/version"):
			proj := strings.TrimSuffix(strings.TrimPrefix(p, "/v2/project/"), "/version")
			out := []mrVersion{}
			for _, id := range f.projects[proj] {
				out = append(out, f.versions[id])
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) addVersion(project, id, filename string, content []byte, deps ...string) {
	path := "/files/" + id + "/" + filename
	f.files[path] = content
	v := mrVersion{ID: id, ProjectID: project, VersionNumber: id, VersionType: "release"}
	mf := mrFile{URL: f.srv.URL + path, Filename: filename, Primary: true}
	mf.Hashes.SHA512 = sha512hex(content)
	v.Files = []mrFile{mf}
	for _, d := range deps {
		v.Dependencies = append(v.Dependencies, mrDependency{ProjectID: d, DependencyType: "required"})
	}
	f.versions[id] = v
	f.projects[project] = append([]string{id}, f.projects[project]...)
}

func (f *fakeAPI) client() *Client {
	c := NewClient()
	c.ModrinthAPI = f.srv.URL
	c.AllowHost = func(string) bool { return true }
	return c
}

func TestParseList(t *testing.T) {
	refs, err := ParseList(" AANobbMI , P7dR8mSH:tFw0iWAk ,")
	if err != nil || len(refs) != 2 || refs[1] != (Ref{"P7dR8mSH", "tFw0iWAk"}) {
		t.Fatalf("refs=%v err=%v", refs, err)
	}
	if refs, _ := ParseList(""); len(refs) != 0 {
		t.Fatal("empty list must parse to nothing")
	}
	for _, bad := range []string{"../x", "a:b:c", "a b"} {
		if _, err := ParseList(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSyncInstallsUpdatesAndRemoves(t *testing.T) {
	api := newFakeAPI(t)
	api.addVersion("fabric-api", "fa1", "fabric-api-1.jar", []byte("fabric api"))
	api.addVersion("sodium", "so1", "sodium-1.jar", []byte("sodium"), "fabric-api")
	api.addVersion("lithium", "li1", "lithium-1.jar", []byte("lithium"))
	c := api.client()

	dir := t.TempDir()
	modsDir := filepath.Join(dir, "mods")
	state := filepath.Join(dir, ".deploykit", "mods-modrinth.json")
	_ = os.MkdirAll(modsDir, 0o755)
	manual := filepath.Join(modsDir, "uploaded-by-hand.jar")
	_ = os.WriteFile(manual, []byte("mine"), 0o644)

	sync := func(list string) {
		t.Helper()
		err := c.Sync(SyncOptions{Source: "modrinth", List: list, Dir: modsDir, State: state, Loaders: []string{"fabric"}, GameVersion: "1.21.4"}, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
	}
	exists := func(name string) bool { _, err := os.Stat(filepath.Join(modsDir, name)); return err == nil }

	sync("sodium,lithium")
	if !exists("sodium-1.jar") || !exists("fabric-api-1.jar") || !exists("lithium-1.jar") {
		t.Fatal("mods or dependency missing")
	}
	n := api.downloads.Load()

	sync("sodium,lithium")
	if api.downloads.Load() != n {
		t.Fatal("unchanged sync downloaded again")
	}

	api.addVersion("sodium", "so2", "sodium-2.jar", []byte("sodium two"), "fabric-api")
	sync("sodium")
	if exists("sodium-1.jar") || !exists("sodium-2.jar") || exists("lithium-1.jar") || !exists("fabric-api-1.jar") {
		t.Fatal("update/removal wrong")
	}

	sync("")
	if exists("sodium-2.jar") || exists("fabric-api-1.jar") {
		t.Fatal("empty list must remove every installed mod")
	}
	if !exists("uploaded-by-hand.jar") {
		t.Fatal("a file DeployKit did not install was removed")
	}
}

func TestSyncPinnedVersionAndHashMismatch(t *testing.T) {
	api := newFakeAPI(t)
	api.addVersion("lithium", "li1", "lithium-1.jar", []byte("lithium"))
	api.addVersion("lithium", "li2", "lithium-2.jar", []byte("lithium2"))
	c := api.client()
	dir := t.TempDir()
	o := SyncOptions{Source: "modrinth", List: "lithium:li1", Dir: dir, State: filepath.Join(dir, "s.json")}
	if err := c.Sync(o, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "lithium-1.jar")); err != nil {
		t.Fatal("pinned version not installed")
	}
	api.files["/files/li2/lithium-2.jar"] = []byte("tampered")
	o.List = "lithium:li2"
	if err := c.Sync(o, io.Discard); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("err = %v", err)
	}
}

func TestDisallowedHost(t *testing.T) {
	c := NewClient()
	c.ModrinthAPI = "https://evil.example"
	err := c.Sync(SyncOptions{Source: "modrinth", List: "x", Dir: t.TempDir(), State: filepath.Join(t.TempDir(), "s")}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not an allowed host") {
		t.Fatalf("err = %v", err)
	}
}

func buildMrpack(t *testing.T, index any, extra map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("modrinth.index.json")
	_ = json.NewEncoder(w).Encode(index)
	for name, body := range extra {
		w, _ := zw.Create(name)
		_, _ = io.WriteString(w, body)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestMrpack(t *testing.T) {
	api := newFakeAPI(t)
	mod := []byte("mod jar")
	api.files["/files/m/mod.jar"] = mod
	index := map[string]any{
		"files": []map[string]any{
			{"path": "mods/mod.jar", "hashes": map[string]string{"sha1": sha1hex(mod)}, "downloads": []string{api.srv.URL + "/files/m/mod.jar"}},
			{"path": "mods/client.jar", "hashes": map[string]string{"sha1": "00"}, "downloads": []string{api.srv.URL + "/nope"}, "env": map[string]string{"server": "unsupported"}},
		},
		"dependencies": map[string]string{"minecraft": "1.21.1", "fabric-loader": "0.16.5"},
	}
	pack := buildMrpack(t, index, map[string]string{"overrides/config/a.toml": "a=1", "server-overrides/server.properties": "motd=x"})
	api.addVersion("cobblemon", "pk1", "pack.mrpack", pack)
	c := api.client()

	dir := t.TempDir()
	o := PackOptions{Source: "modrinth", Project: "cobblemon", Dir: dir, State: filepath.Join(dir, ".deploykit", "pack.json"), EnvOut: filepath.Join(dir, ".deploykit", "pack.env")}
	if err := c.Modpack(o, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"mods/mod.jar", "config/a.toml", "server.properties"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s missing", f)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mods/client.jar")); err == nil {
		t.Error("client-only file installed")
	}
	env, _ := os.ReadFile(o.EnvOut)
	for _, want := range []string{"DK_RESOLVED_GAME_VERSION='1.21.1'", "DK_RESOLVED_LOADER='fabric'", "DK_RESOLVED_LOADER_VERSION='0.16.5'"} {
		if !strings.Contains(string(env), want) {
			t.Errorf("env %q missing %q", env, want)
		}
	}

	// Unchanged pack: no download, overrides not rewritten.
	_ = os.WriteFile(filepath.Join(dir, "config/a.toml"), []byte("edited"), 0o644)
	n := api.downloads.Load()
	if err := c.Modpack(o, io.Discard); err != nil {
		t.Fatal(err)
	}
	if api.downloads.Load() != n {
		t.Fatal("unchanged pack downloaded again")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "config/a.toml")); string(b) != "edited" {
		t.Fatal("overrides rewritten on an unchanged pack")
	}

	// Uninstall removes the pack's mods.
	o.Project = ""
	if err := c.Modpack(o, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mods/mod.jar")); err == nil {
		t.Fatal("pack mod survived uninstall")
	}
}

func TestMrpackRejectsTraversal(t *testing.T) {
	api := newFakeAPI(t)
	mod := []byte("x")
	api.files["/files/m/x.jar"] = mod
	index := map[string]any{
		"files":        []map[string]any{{"path": "../../etc/evil.jar", "hashes": map[string]string{"sha1": sha1hex(mod)}, "downloads": []string{api.srv.URL + "/files/m/x.jar"}}},
		"dependencies": map[string]string{"minecraft": "1.21.1"},
	}
	api.addVersion("evil", "e1", "evil.mrpack", buildMrpack(t, index, nil))
	dir := t.TempDir()
	err := api.client().Modpack(PackOptions{Source: "modrinth", Project: "evil", Dir: dir, State: filepath.Join(dir, "s"), EnvOut: filepath.Join(dir, "e")}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("err = %v", err)
	}
}

func TestSafeJoin(t *testing.T) {
	for _, bad := range []string{"", "/abs", "..", "../x", "a/../../x"} {
		if _, err := safeJoin("/data", bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if p, err := safeJoin("/data", "mods/a.jar"); err != nil || p != filepath.Join("/data", "mods", "a.jar") {
		t.Fatalf("p=%s err=%v", p, err)
	}
}
