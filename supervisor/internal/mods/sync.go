// Package mods installs mods and modpacks for the images that support them.
// DeployKit only passes lists of ids (contract v1: DK_MODS_*, DK_MODPACK_*);
// all downloading happens here, inside the game container, and only from
// fixed hosts.
package mods

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Ref is one entry of a DK_MODS_* list: "project" or "project:version".
type Ref struct {
	Project string
	Version string
}

// ParseList parses the comma-separated format DeployKit's game-mods-env.ts
// renders. An empty list is valid and means "no mods".
func ParseList(s string) ([]Ref, error) {
	var refs []Ref
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, v, _ := strings.Cut(part, ":")
		if p == "" || strings.ContainsAny(p, "/\\ ") || strings.ContainsAny(v, "/\\ :") {
			return nil, fmt.Errorf("invalid mod reference %q", part)
		}
		refs = append(refs, Ref{Project: p, Version: v})
	}
	return refs, nil
}

type resolved struct {
	Key      string
	Filename string
	URL      string
	Version  string
	Hashes   Hashes
}

type SyncOptions struct {
	Source      string // modrinth | curseforge
	List        string
	Dir         string
	State       string
	Loaders     []string
	GameVersion string
}

// Sync makes Dir hold exactly the listed mods (plus required dependencies):
// it downloads what is missing or changed and deletes what it installed
// before but is no longer listed. Files it did not install are untouched.
func (c *Client) Sync(o SyncOptions, log io.Writer) error {
	refs, err := ParseList(o.List)
	if err != nil {
		return err
	}
	lock, err := readLock(o.State)
	if err != nil {
		return fmt.Errorf("read %s: %w", o.State, err)
	}

	var want []resolved
	if len(refs) > 0 {
		switch o.Source {
		case "modrinth":
			want, err = c.resolveModrinth(refs, o.Loaders, o.GameVersion)
		case "curseforge":
			want, err = c.resolveCurseForge(refs, o.Loaders, o.GameVersion)
		default:
			return fmt.Errorf("unknown source %q", o.Source)
		}
		if err != nil {
			return err
		}
	}

	next := &Lock{Entries: map[string]LockEntry{}}
	for _, r := range want {
		if err := safeFilename(r.Filename); err != nil {
			return err
		}
		dest := filepath.Join(o.Dir, r.Filename)
		prev, had := lock.Entries[r.Key]
		if had && prev.Path == dest && fileMatches(dest, r.Hashes) {
			next.Entries[r.Key] = prev
			continue
		}
		fmt.Fprintf(log, "mods: %s %s -> %s\n", o.Source, r.Key, r.Filename)
		if err := c.download(r.URL, dest, r.Hashes); err != nil {
			return fmt.Errorf("%s: %w", r.Key, err)
		}
		if had && prev.Path != dest {
			removeWithin(o.Dir, prev.Path, log)
		}
		next.Entries[r.Key] = LockEntry{Path: dest, Version: r.Version, SHA1: r.Hashes.SHA1, SHA512: r.Hashes.SHA512}
	}

	stale := make([]string, 0)
	for key := range lock.Entries {
		if _, ok := next.Entries[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		fmt.Fprintf(log, "mods: removing %s\n", key)
		removeWithin(o.Dir, lock.Entries[key].Path, log)
	}
	return writeJSON(o.State, next)
}

// removeWithin deletes a path read back from a lock or state file, but only
// when it lies inside base: those files are on the data volume and may have
// been edited.
func removeWithin(base, path string, log io.Writer) {
	if !within(base, path) {
		fmt.Fprintf(log, "mods: not removing %s: outside %s\n", path, base)
		return
	}
	removeFile(path, log)
}

func removeFile(path string, log io.Writer) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(log, "mods: cannot remove %s: %v\n", path, err)
	}
}
