// Package logx is the single writer shared by the game's output and the
// supervisor's own "[deploykit]" lines, so a supervisor line never lands in the
// middle of a partial line the game printed (a prompt without a newline).
package logx

import (
	"fmt"
	"io"
	"sync"
)

const Prefix = "[deploykit] "

type Writer struct {
	mu      sync.Mutex
	out     io.Writer
	midLine bool
}

func New(out io.Writer) *Writer { return &Writer{out: out} }

// Write passes game output through untouched.
func (w *Writer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.out.Write(p)
	if n > 0 {
		w.midLine = p[n-1] != '\n'
	}
	return n, err
}

// Logf writes one "[deploykit] ..." line, breaking the current line first if
// the game left one open.
func (w *Writer) Logf(format string, args ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.midLine {
		_, _ = io.WriteString(w.out, "\n")
		w.midLine = false
	}
	_, _ = fmt.Fprintf(w.out, Prefix+format+"\n", args...)
}
