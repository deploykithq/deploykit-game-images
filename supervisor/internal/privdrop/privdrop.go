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
//
// The walk runs as root over content the game user may have shaped, so it
// never follows a symlink (Lchown, and WalkDir does not descend into one),
// stays on the directory's own filesystem, and leaves alone regular files
// with more than one link: a hard link to a root-owned file elsewhere must
// not hand that file to the game user.
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
		rootDev, err := deviceOf(dir)
		if err != nil {
			return err
		}
		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			st, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return nil
			}
			if uint64(st.Dev) != rootDev {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.Mode().IsRegular() && st.Nlink > 1 {
				return nil
			}
			return os.Lchown(path, uid, gid)
		})
		if err != nil {
			return fmt.Errorf("chown %s: %w", dir, err)
		}
	}
	return nil
}

// deviceOf is the device dir lives on: the walk does not cross into other
// filesystems mounted below it.
func deviceOf(dir string) (uint64, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("stat %s: no device", dir)
	}
	return uint64(st.Dev), nil
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
