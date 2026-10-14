package mods

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// PackState is what a modpack install leaves behind: which pack it was, the
// files it downloaded (removed when the pack changes) and what the pack needs
// from the server (Minecraft version, loader).
type PackState struct {
	Pack  string            `json:"pack"`
	Files []string          `json:"files"`
	Env   map[string]string `json:"env"`
}

type PackOptions struct {
	Source  string
	Project string
	Version string
	Dir     string
	State   string
	EnvOut  string
}

// Modpack installs (or, with an empty project, uninstalls) a modpack into Dir
// and writes DK_RESOLVED_* lines to EnvOut for the install hook to source.
// The pack is only downloaded again when the pack or version changes, so
// overrides — usually config files — are not rewritten on every start.
func (c *Client) Modpack(o PackOptions, log io.Writer) error {
	prev := &PackState{}
	if raw, err := os.ReadFile(o.State); err == nil {
		if err := json.Unmarshal(raw, prev); err != nil {
			return fmt.Errorf("read %s: %w", o.State, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if o.Project == "" {
		if prev.Pack != "" {
			fmt.Fprintf(log, "modpack: removing %s\n", prev.Pack)
			for _, f := range prev.Files {
				removeFile(f, log)
			}
		}
		if err := writeJSON(o.State, &PackState{}); err != nil {
			return err
		}
		return writeEnv(o.EnvOut, nil)
	}

	if o.Version == "" {
		// Pin "latest" first, so an unchanged pack is recognised without
		// downloading it and rewriting its overrides on every start.
		v, err := c.latestPackVersion(o.Source, o.Project)
		if err != nil {
			return err
		}
		o.Version = v
	}
	id := o.Source + ":" + o.Project + ":" + o.Version
	if prev.Pack == id {
		fmt.Fprintf(log, "modpack: %s already installed\n", id)
		return writeEnv(o.EnvOut, prev.Env)
	}

	var (
		next *PackState
		err  error
	)
	switch o.Source {
	case "modrinth":
		next, err = c.installMrpack(o, log)
	case "curseforge":
		next, err = c.installCFPack(o, log)
	default:
		return fmt.Errorf("unknown modpack source %q", o.Source)
	}
	if err != nil {
		return err
	}
	if prev.Pack == next.Pack {
		fmt.Fprintf(log, "modpack: %s already installed\n", next.Pack)
		return writeEnv(o.EnvOut, prev.Env)
	}

	keep := map[string]bool{}
	for _, f := range next.Files {
		keep[f] = true
	}
	for _, f := range prev.Files {
		if !keep[f] {
			removeFile(f, log)
		}
	}
	if err := writeJSON(o.State, next); err != nil {
		return err
	}
	return writeEnv(o.EnvOut, next.Env)
}

func writeEnv(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		// Values come from pack metadata: quote for the shell that sources it.
		fmt.Fprintf(&b, "%s='%s'\n", k, strings.ReplaceAll(env[k], "'", `'\''`))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func loaderEnv(gameVersion, loader, loaderVersion string) map[string]string {
	return map[string]string{
		"DK_RESOLVED_GAME_VERSION":   gameVersion,
		"DK_RESOLVED_LOADER":         loader,
		"DK_RESOLVED_LOADER_VERSION": loaderVersion,
	}
}

func (c *Client) latestPackVersion(source, project string) (string, error) {
	switch source {
	case "modrinth":
		v, err := c.mrLatest(project, nil, "")
		if err != nil {
			return "", fmt.Errorf("modrinth modpack %s: %w", project, err)
		}
		return v.ID, nil
	case "curseforge":
		h, err := c.cfHeaders()
		if err != nil {
			return "", err
		}
		var mod struct {
			Data struct {
				MainFileID int `json:"mainFileId"`
			} `json:"data"`
		}
		if err := c.getJSON(c.CurseForgeAPI+"/v1/mods/"+url.PathEscape(project), h, &mod); err != nil {
			return "", fmt.Errorf("curseforge modpack %s: %w", project, err)
		}
		if mod.Data.MainFileID == 0 {
			return "", fmt.Errorf("curseforge modpack %s has no main file", project)
		}
		return strconv.Itoa(mod.Data.MainFileID), nil
	}
	return "", fmt.Errorf("unknown modpack source %q", source)
}

// --- Modrinth (.mrpack) ---

type mrIndex struct {
	Files []struct {
		Path   string `json:"path"`
		Hashes struct {
			SHA1   string `json:"sha1"`
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
		Env *struct {
			Server string `json:"server"`
		} `json:"env"`
		Downloads []string `json:"downloads"`
	} `json:"files"`
	Dependencies map[string]string `json:"dependencies"`
}

func (c *Client) installMrpack(o PackOptions, log io.Writer) (*PackState, error) {
	v, err := c.mrVersion(o.Version)
	if err != nil {
		return nil, fmt.Errorf("modrinth modpack %s: %w", o.Project, err)
	}
	pack := "modrinth:" + o.Project + ":" + o.Version
	f, err := v.primary()
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(f.Filename, ".mrpack") {
		return nil, fmt.Errorf("modrinth version %s is not a modpack", v.ID)
	}
	fmt.Fprintf(log, "modpack: installing %s (%s)\n", f.Filename, v.VersionNumber)

	zr, cleanup, err := c.fetchZip(f.URL, Hashes{SHA1: f.Hashes.SHA1, SHA512: f.Hashes.SHA512})
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var idx mrIndex
	if err := readZipJSON(zr, "modrinth.index.json", &idx); err != nil {
		return nil, err
	}

	state := &PackState{Pack: pack}
	for _, file := range idx.Files {
		if file.Env != nil && file.Env.Server == "unsupported" {
			continue
		}
		dest, err := safeJoin(o.Dir, file.Path)
		if err != nil {
			return nil, err
		}
		want := Hashes{SHA1: file.Hashes.SHA1, SHA512: file.Hashes.SHA512}
		if !fileMatches(dest, want) {
			if err := c.downloadAny(file.Downloads, dest, want); err != nil {
				return nil, fmt.Errorf("%s: %w", file.Path, err)
			}
		}
		state.Files = append(state.Files, dest)
	}
	for _, prefix := range []string{"overrides/", "server-overrides/"} {
		if err := extractPrefix(zr, prefix, o.Dir); err != nil {
			return nil, err
		}
	}

	deps := idx.Dependencies
	switch {
	case deps["neoforge"] != "":
		state.Env = loaderEnv(deps["minecraft"], "neoforge", deps["neoforge"])
	case deps["forge"] != "":
		state.Env = loaderEnv(deps["minecraft"], "forge", deps["forge"])
	case deps["fabric-loader"] != "":
		state.Env = loaderEnv(deps["minecraft"], "fabric", deps["fabric-loader"])
	case deps["quilt-loader"] != "":
		return nil, errors.New("quilt modpacks are not supported")
	default:
		state.Env = loaderEnv(deps["minecraft"], "vanilla", "")
	}
	return state, nil
}

func (c *Client) downloadAny(urls []string, dest string, want Hashes) error {
	var last error = errors.New("no download URL")
	for _, u := range urls {
		if last = c.download(u, dest, want); last == nil {
			return nil
		}
	}
	return last
}

// --- CurseForge ---

type cfManifest struct {
	Minecraft struct {
		Version    string `json:"version"`
		ModLoaders []struct {
			ID      string `json:"id"`
			Primary bool   `json:"primary"`
		} `json:"modLoaders"`
	} `json:"minecraft"`
	Files []struct {
		ProjectID int  `json:"projectID"`
		FileID    int  `json:"fileID"`
		Required  bool `json:"required"`
	} `json:"files"`
	Overrides string `json:"overrides"`
}

func (c *Client) installCFPack(o PackOptions, log io.Writer) (*PackState, error) {
	f, err := c.cfFile(o.Project, o.Version)
	if err != nil {
		return nil, fmt.Errorf("curseforge modpack %s: %w", o.Project, err)
	}
	pack := "curseforge:" + o.Project + ":" + o.Version
	r, err := cfResolved(f)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(log, "modpack: installing %s\n", f.FileName)
	zr, cleanup, err := c.fetchZip(r.URL, r.Hashes)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var m cfManifest
	if err := readZipJSON(zr, "manifest.json", &m); err != nil {
		return nil, err
	}

	ids := make([]int, 0, len(m.Files))
	for _, mf := range m.Files {
		if mf.Required {
			ids = append(ids, mf.FileID)
		}
	}
	state := &PackState{Pack: pack}
	if len(ids) > 0 {
		files, err := c.cfFiles(ids)
		if err != nil {
			return nil, err
		}
		for i := range files {
			cf := &files[i]
			if !strings.HasSuffix(strings.ToLower(cf.FileName), ".jar") {
				continue // resource packs, shaders: client side
			}
			if !cf.serverSide() {
				fmt.Fprintf(log, "modpack: skipping client-only %s\n", cf.FileName)
				continue
			}
			res, err := cfResolved(cf)
			if err != nil {
				fmt.Fprintf(log, "modpack: %v\n", err)
				continue
			}
			dest := filepath.Join(o.Dir, "mods", res.Filename)
			if !fileMatches(dest, res.Hashes) {
				if err := c.download(res.URL, dest, res.Hashes); err != nil {
					return nil, fmt.Errorf("%s: %w", res.Filename, err)
				}
			}
			state.Files = append(state.Files, dest)
		}
	}
	overrides := m.Overrides
	if overrides == "" {
		overrides = "overrides"
	}
	if err := extractPrefix(zr, strings.TrimSuffix(overrides, "/")+"/", o.Dir); err != nil {
		return nil, err
	}

	loader, loaderVersion := "vanilla", ""
	for i, l := range m.Minecraft.ModLoaders {
		if l.Primary || i == 0 {
			loader, loaderVersion, _ = strings.Cut(l.ID, "-")
			if l.Primary {
				break
			}
		}
	}
	if loader == "quilt" {
		return nil, errors.New("quilt modpacks are not supported")
	}
	state.Env = loaderEnv(m.Minecraft.Version, loader, loaderVersion)
	return state, nil
}

// --- zip helpers ---

func (c *Client) fetchZip(u string, want Hashes) (*zip.Reader, func(), error) {
	dir, err := os.MkdirTemp("", "dk-pack-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "pack.zip")
	if err := c.download(u, path, want); err != nil {
		cleanup()
		return nil, nil, err
	}
	zf, err := zip.OpenReader(path)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return &zf.Reader, func() { zf.Close(); cleanup() }, nil
}

func readZipJSON(zr *zip.Reader, name string, v any) error {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return json.NewDecoder(rc).Decode(v)
		}
	}
	return fmt.Errorf("modpack has no %s", name)
}

// extractPrefix writes every regular file under prefix into dir.
func extractPrefix(zr *zip.Reader, prefix, dir string) error {
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefix) || f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			continue // symlinks and devices never leave a pack
		}
		dest, err := safeJoin(dir, strings.TrimPrefix(f.Name, prefix))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(rc, 1<<30))
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
