// Package proc starts child processes and reaps every child of the
// supervisor, including orphans re-parented to it as PID 1.
//
// One goroutine owns wait4(-1): os/exec's Wait would race it for the same
// children, so processes started here are never Wait()ed — their exit status
// arrives on the channel Start returns.
package proc

import (
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

type Reaper struct {
	mu      sync.Mutex
	waiting map[int]chan syscall.WaitStatus
	sigchld chan os.Signal
}

func NewReaper() *Reaper {
	r := &Reaper{
		waiting: map[int]chan syscall.WaitStatus{},
		sigchld: make(chan os.Signal, 1),
	}
	signal.Notify(r.sigchld, syscall.SIGCHLD)
	go r.loop()
	return r
}

// Start starts cmd and returns a channel that receives its wait status once.
func (r *Reaper) Start(cmd *exec.Cmd) (<-chan syscall.WaitStatus, error) {
	// Holding the lock across Start means a child that exits immediately is
	// reaped by loop() only after its channel is registered.
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ch := make(chan syscall.WaitStatus, 1)
	r.waiting[cmd.Process.Pid] = ch
	return ch, nil
}

func (r *Reaper) loop() {
	for range r.sigchld {
		r.reapAll()
	}
}

func (r *Reaper) reapAll() {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if err == syscall.EINTR {
			continue
		}
		if pid <= 0 || err != nil {
			return
		}
		r.mu.Lock()
		if ch, ok := r.waiting[pid]; ok {
			delete(r.waiting, pid)
			ch <- ws
		}
		r.mu.Unlock()
	}
}

// ExitCode maps a wait status to a shell-style exit code.
func ExitCode(ws syscall.WaitStatus) int {
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ws.ExitStatus()
}

// KillGroup signals the process group led by pid (children start with
// Setpgid), falling back to the process itself.
func KillGroup(pid int, sig syscall.Signal) {
	if err := syscall.Kill(-pid, sig); err != nil {
		_ = syscall.Kill(pid, sig)
	}
}
