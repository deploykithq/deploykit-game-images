package mods

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Lock records what a sync installed, so the next sync can remove exactly the
// files DeployKit put there — never a jar the user uploaded by hand.
type Lock struct {
	Entries map[string]LockEntry `json:"entries"`
}

type LockEntry struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA1    string `json:"sha1,omitempty"`
	SHA512  string `json:"sha512,omitempty"`
}

func (e LockEntry) hashes() Hashes { return Hashes{SHA1: e.SHA1, SHA512: e.SHA512} }

func readLock(path string) (*Lock, error) {
	l := &Lock{Entries: map[string]LockEntry{}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, l); err != nil {
		return nil, err
	}
	if l.Entries == nil {
		l.Entries = map[string]LockEntry{}
	}
	return l, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
