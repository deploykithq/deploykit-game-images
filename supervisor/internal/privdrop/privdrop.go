// Package privdrop prepares the writable directories as root and then drops
// the supervisor to the game user.
//
// DeployKit runs game containers with no-new-privileges and keeps only CHOWN,
// SETUID and SETGID of the privileged capabilities, so the drop happens through
// syscalls here — never through a setuid binary such as gosu or su, which
// no-new-privileges would neuter.
package privdrop

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Prepare creates each directory and makes uid:gid own it. A directory whose
// top level already belongs to uid is left alone, so the recursive walk only
// happens on a fresh volume or one restored from elsewhere.
func Prepare(dirs []string, uid, gid int) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == uid && int(st.Gid) == gid {
			continue
		}
		err = filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return os.Lchown(path, uid, gid)
		})
		if err != nil {
			return fmt.Errorf("chown %s: %w", dir, err)
		}
	}
	return nil
}

// Drop switches every thread of the process to uid:gid with no supplementary
// groups. It is a no-op when not running as root (local tests).
func Drop(uid, gid int) error {
	if os.Geteuid() != 0 {
		return nil
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid: %w", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid: %w", err)
	}
	if os.Geteuid() != uid {
		return fmt.Errorf("still uid %d after setuid(%d)", os.Geteuid(), uid)
	}
	return nil
}
