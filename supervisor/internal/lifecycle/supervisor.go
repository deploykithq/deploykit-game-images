// Package lifecycle runs a game: the preparation phases, the game process,
// its console, readiness, and a graceful stop.
package lifecycle

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/deploykithq/deploykit-game-images/supervisor/internal/config"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/console"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/env"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/logx"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/proc"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/ready"
)

// killGrace is how long the game gets after the escalation SIGTERM before
// SIGKILL.
const killGrace = 5 * time.Second

type Supervisor struct {
	Cfg     *config.Config
	Env     []string
	RunDir  string
	Stdin   io.Reader
	Out     *logx.Writer
	ErrOut  io.Writer
	Reaper  *proc.Reaper
	Signals <-chan os.Signal

	// StopTimeout overrides Cfg.Stop.Timeout (DK_STOP_TIMEOUT).
	StopTimeout time.Duration

	mu      sync.Mutex
	state   string
	pid     int
	started time.Time
	watcher *ready.Watcher
	stopReq chan struct{}
}

func (s *Supervisor) setState(st string) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
}

// Run returns the exit code for the container.
func (s *Supervisor) Run() int {
	ready.Clear(s.RunDir)
	s.stopReq = make(chan struct{}, 1)
	s.Out.Logf("game=%s contract=1", s.Cfg.Game)

	for _, ph := range s.Cfg.Phases {
		s.setState(ph.Name)
		s.Out.Logf("phase=%s", ph.Name)
		code, interrupted := s.runPhase(ph)
		if interrupted {
			s.Out.Logf("stopped code=%d phase=%s", code, ph.Name)
			return code
		}
		if code != 0 {
			s.Out.Logf("phase=%s failed code=%d", ph.Name, code)
			return code
		}
	}
	return s.runGame()
}

func (s *Supervisor) command(argv []string) *exec.Cmd {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = s.Env
	cmd.Dir = s.Cfg.Workdir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// runPhase runs one hook to completion. A stop signal during a phase (a long
// SteamCMD download) ends it and the container: there is nothing to save yet.
func (s *Supervisor) runPhase(ph config.Phase) (code int, interrupted bool) {
	cmd := s.command(ph.Run)
	cmd.Stdout = s.Out
	cmd.Stderr = s.ErrOut
	done, err := s.Reaper.Start(cmd)
	if err != nil {
		s.Out.Logf("phase=%s cannot start: %v", ph.Name, err)
		return 127, false
	}
	select {
	case ws := <-done:
		return proc.ExitCode(ws), false
	case sig := <-s.Signals:
		s.Out.Logf("stopping signal=%v phase=%s", sig, ph.Name)
		pid := cmd.Process.Pid
		proc.KillGroup(pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(killGrace):
			proc.KillGroup(pid, syscall.SIGKILL)
			<-done
		}
		return 128 + int(syscall.SIGTERM), true
	}
}

func (s *Supervisor) runGame() int {
	s.setState("start")
	s.Out.Logf("phase=start")

	cmd := s.command(s.Cfg.Start)
	cmd.Env = env.ForGame(s.Env)
	var sender console.Sender = console.None{}
	var gameStdin io.WriteCloser
	if s.Cfg.Console.Adapter == config.AdapterStdin {
		pr, pw, err := os.Pipe()
		if err != nil {
			s.Out.Logf("cannot create stdin pipe: %v", err)
			return 1
		}
		cmd.Stdin = pr
		gameStdin = pw
		sender = &console.Pipe{W: pw}
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		s.Out.Logf("cannot create output pipe: %v", err)
		return 1
	}
	cmd.Stdout = outW
	cmd.Stderr = outW

	s.watcher = ready.NewWatcher(s.Cfg.ReadyRegexp, s.onReady)
	copied := make(chan struct{})
	go func() {
		_ = s.watcher.Copy(s.Out, outR)
		close(copied)
	}()

	done, err := s.Reaper.Start(cmd)
	outW.Close()
	if err != nil {
		s.Out.Logf("cannot start game: %v", err)
		outR.Close()
		return 127
	}
	if f, ok := cmd.Stdin.(*os.File); ok {
		f.Close()
	}
	pid := cmd.Process.Pid
	s.mu.Lock()
	s.pid, s.started, s.state = pid, time.Now(), "running"
	s.mu.Unlock()

	if s.Cfg.Console.Adapter == config.AdapterRCON {
		port := s.Cfg.Console.RCON.Port
		if pe := s.Cfg.Console.RCON.PortEnv; pe != "" {
			if p, err := strconv.Atoi(env.Lookup(s.Env, pe)); err == nil && p > 0 {
				port = p
			}
		}
		rc := &console.RCON{
			Addr:     net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
			Password: env.Lookup(s.Env, "DK_RCON_PASSWORD"),
			Out:      func(l string) { _, _ = s.Out.Write([]byte(l + "\n")) },
		}
		defer rc.Close()
		sender = rc
	}

	router := &console.Router{
		Sender: sender,
		Verbs:  s.Cfg.Verbs,
		Status: s.status,
		Stop:   func() { s.requestStop() },
		Log:    s.Out.Logf,
	}
	if s.Stdin != nil {
		go router.Run(s.Stdin)
	}

	var (
		stopping bool
		timer    <-chan time.Time
		killed   bool
	)
	for {
		select {
		case ws := <-done:
			select {
			case <-copied:
			case <-time.After(2 * time.Second):
			}
			if gameStdin != nil {
				gameStdin.Close()
			}
			ready.Clear(s.RunDir)
			code := proc.ExitCode(ws)
			s.Out.Logf("stopped code=%d", code)
			return code
		case sig := <-s.Signals:
			if !stopping {
				stopping = true
				timer = s.beginStop(pid, sender, fmt.Sprintf("signal=%v", sig))
			}
		case <-s.stopReq:
			if !stopping {
				stopping = true
				timer = s.beginStop(pid, sender, "verb=stop")
			}
		case <-timer:
			if !killed {
				killed = true
				s.Out.Logf("stop timeout, sending SIGTERM")
				proc.KillGroup(pid, syscall.SIGTERM)
				timer = time.After(killGrace)
			} else {
				s.Out.Logf("game did not exit, sending SIGKILL")
				proc.KillGroup(pid, syscall.SIGKILL)
				timer = nil
			}
		}
	}
}

func (s *Supervisor) beginStop(pid int, sender console.Sender, why string) <-chan time.Time {
	s.setState("stopping")
	ready.Clear(s.RunDir)
	s.Out.Logf("stopping %s", why)
	stop := s.Cfg.Stop
	switch {
	case stop.Signal != "":
		_ = syscall.Kill(pid, signalByName(stop.Signal))
	case stop.Command != "":
		if err := sender.Send(stop.Command); err != nil {
			s.Out.Logf("stop command failed (%v), sending SIGTERM", err)
			proc.KillGroup(pid, syscall.SIGTERM)
		}
	default:
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	timeout := stop.Timeout
	if s.StopTimeout > 0 {
		timeout = s.StopTimeout
	}
	return time.After(timeout)
}

func (s *Supervisor) requestStop() {
	select {
	case s.stopReq <- struct{}{}:
	default:
	}
}

func (s *Supervisor) onReady() {
	if err := ready.Mark(s.RunDir); err != nil {
		s.Out.Logf("cannot write ready marker: %v", err)
	}
	s.Out.Logf("ready")
}

func (s *Supervisor) status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	up := ""
	if !s.started.IsZero() {
		up = time.Since(s.started).Round(time.Second).String()
	}
	isReady := s.watcher != nil && s.watcher.Fired() && s.state == "running"
	return fmt.Sprintf("status game=%s state=%s ready=%t pid=%d uptime=%s console=%s",
		s.Cfg.Game, s.state, isReady, s.pid, up, s.Cfg.Console.Adapter)
}

func signalByName(name string) syscall.Signal {
	switch name {
	case "SIGINT":
		return syscall.SIGINT
	case "SIGQUIT":
		return syscall.SIGQUIT
	case "SIGHUP":
		return syscall.SIGHUP
	case "SIGUSR1":
		return syscall.SIGUSR1
	case "SIGUSR2":
		return syscall.SIGUSR2
	default:
		return syscall.SIGTERM
	}
}
