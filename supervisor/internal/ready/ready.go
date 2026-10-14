// Package ready watches the game's output for its readiness line and keeps
// the marker file the HEALTHCHECK reads.
package ready

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const markerName = "ready"

// Watcher matches complete output lines against a pattern until it fires once.
// Output is copied through untouched; only the matching works on lines.
type Watcher struct {
	re      *regexp.Regexp
	onReady func()

	mu    sync.Mutex
	fired bool
}

func NewWatcher(re *regexp.Regexp, onReady func()) *Watcher {
	return &Watcher{re: re, onReady: onReady}
}

func (w *Watcher) Fired() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.fired
}

// Copy copies src to dst until EOF, watching each line. Several streams
// (stdout, stderr) may share one Watcher.
func (w *Watcher) Copy(dst io.Writer, src io.Reader) error {
	buf := make([]byte, 32*1024)
	var line []byte
	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if _, werr := dst.Write(chunk); werr != nil {
				return werr
			}
			if !w.Fired() {
				line = append(line, chunk...)
				for {
					i := bytes.IndexByte(line, '\n')
					if i < 0 {
						break
					}
					w.check(line[:i])
					line = line[i+1:]
				}
				// A prompt-less game may never end its last line; cap it.
				if len(line) > 64*1024 {
					line = line[len(line)-64*1024:]
				}
			}
		}
		if err == io.EOF {
			if len(line) > 0 {
				w.check(line)
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (w *Watcher) check(line []byte) {
	if !w.re.Match(bytes.TrimRight(line, "\r")) {
		return
	}
	w.mu.Lock()
	if w.fired {
		w.mu.Unlock()
		return
	}
	w.fired = true
	w.mu.Unlock()
	w.onReady()
}

func Mark(runDir string) error {
	return os.WriteFile(filepath.Join(runDir, markerName), []byte("ready\n"), 0o644)
}

func Clear(runDir string) {
	_ = os.Remove(filepath.Join(runDir, markerName))
}

// Healthy is what `dk-entrypoint health` reports.
func Healthy(runDir string) bool {
	_, err := os.Stat(filepath.Join(runDir, markerName))
	return err == nil
}
