// Package console turns the supervisor's stdin — the only console channel in
// the contract — into commands for the game, and handles the reserved
// "/dk <verb>" lines.
package console

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"sync"
)

// Sender delivers one console line to the game.
type Sender interface {
	Send(cmd string) error
}

// Pipe writes lines to the game's own stdin.
type Pipe struct {
	mu sync.Mutex
	W  io.Writer
}

func (p *Pipe) Send(cmd string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, err := io.WriteString(p.W, cmd+"\n")
	return err
}

// ErrUnsupported is returned by None.
var ErrUnsupported = errors.New("this game has no console")

type None struct{}

func (None) Send(string) error { return ErrUnsupported }

// Router reads console lines and dispatches them.
type Router struct {
	Sender Sender
	Verbs  map[string][]string
	Status func() string
	Stop   func()
	Log    func(format string, args ...any)
}

// Run reads until in is closed. A container started without OpenStdin hands
// the supervisor an empty stdin, so EOF simply ends the loop.
func (r *Router) Run(in io.Reader) {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 4096), 64*1024)
	for sc.Scan() {
		r.Handle(sc.Text())
	}
}

func (r *Router) Handle(line string) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return
	}
	if line == "/dk" || strings.HasPrefix(line, "/dk ") {
		r.verb(strings.TrimSpace(strings.TrimPrefix(line, "/dk")))
		return
	}
	if err := r.Sender.Send(line); err != nil {
		r.Log("console error: %v", err)
	}
}

func (r *Router) verb(v string) {
	switch v {
	case "status":
		r.Log("%s", r.Status())
	case "stop":
		r.Stop()
	case "save", "save-off", "save-on":
		cmds, ok := r.Verbs[v]
		if !ok {
			r.Log("verb %s unsupported", v)
			return
		}
		for _, c := range cmds {
			if err := r.Sender.Send(c); err != nil {
				r.Log("verb %s failed: %v", v, err)
				return
			}
		}
		r.Log("verb %s ok", v)
	default:
		r.Log("unknown verb %q (status, stop, save, save-off, save-on)", v)
	}
}
