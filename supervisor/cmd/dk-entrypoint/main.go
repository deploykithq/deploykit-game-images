// dk-entrypoint is PID 1 of every DeployKit game image (contract v1).
//
//	dk-entrypoint            run the game described by /etc/deploykit/game.yaml
//	dk-entrypoint health     exit 0 once the game reported ready (HEALTHCHECK)
//	dk-entrypoint mods ...   install mods/modpacks (called from hooks)
//	dk-entrypoint version
package main

import (
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/deploykithq/deploykit-game-images/supervisor/internal/config"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/env"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/lifecycle"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/logx"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/mods"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/privdrop"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/proc"
	"github.com/deploykithq/deploykit-game-images/supervisor/internal/ready"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "health":
			if ready.Healthy(runDir()) {
				os.Exit(0)
			}
			os.Exit(1)
		case "mods":
			os.Exit(mods.CLI(os.Args[2:], os.Stdout, os.Stderr))
		case "version":
			fmt.Println(Version)
			return
		case "run":
		default:
			fmt.Fprintf(os.Stderr, "usage: dk-entrypoint [run|health|mods|version]\n")
			os.Exit(2)
		}
	}
	os.Exit(run())
}

const defaultRunDir = "/run/deploykit"

// runDir and configPath honour DK_RUN_DIR and DK_CONFIG only when not
// running as root (tests, local runs). As root the supervisor chowns the run
// directory to the game user before dropping privileges, and the HEALTHCHECK
// runs as root: neither may be steered by the environment.
func runDir() string {
	if d := os.Getenv("DK_RUN_DIR"); d != "" && os.Geteuid() != 0 {
		return d
	}
	return defaultRunDir
}

func configPath() string {
	if p := os.Getenv("DK_CONFIG"); p != "" && os.Geteuid() != 0 {
		return p
	}
	return config.DefaultPath
}

func run() int {
	out := logx.New(os.Stdout)

	// Notify before anything else: as PID 1 an unhandled SIGTERM is ignored.
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	cfg, err := config.Load(configPath())
	if err != nil {
		out.Logf("fatal: %v", err)
		return 1
	}

	home, name := "", ""
	if u, err := user.LookupId(strconv.Itoa(cfg.User.UID)); err == nil {
		home, name = u.HomeDir, u.Username
	}
	rd := runDir()
	res := env.Resolve(os.Environ(), env.Input{
		DataDir:   cfg.DataDir,
		ServerDir: cfg.ServerDir,
		RunDir:    rd,
		Home:      home,
		User:      name,
		RCON:      cfg.Console.Adapter == config.AdapterRCON,
	}, env.OSSources())

	if os.Geteuid() == 0 {
		dirs := []string{cfg.DataDir, res.ServerDir, rd}
		if home != "" {
			dirs = append(dirs, home)
		}
		if err := privdrop.Prepare(dirs, cfg.User.UID, cfg.User.GID); err != nil {
			out.Logf("fatal: %v", err)
			return 1
		}
		if err := privdrop.Drop(cfg.User.UID, cfg.User.GID); err != nil {
			out.Logf("fatal: %v", err)
			return 1
		}
	}

	var stopTimeout time.Duration
	if v := env.Lookup(res.Env, "DK_STOP_TIMEOUT"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			stopTimeout = time.Duration(secs) * time.Second
		} else if d, err := time.ParseDuration(v); err == nil && d > 0 {
			stopTimeout = d
		}
	}

	s := &lifecycle.Supervisor{
		Cfg:         cfg,
		Env:         res.Env,
		RunDir:      rd,
		Stdin:       os.Stdin,
		Out:         out,
		ErrOut:      out,
		Reaper:      proc.NewReaper(),
		Signals:     sigs,
		StopTimeout: stopTimeout,
	}
	return s.Run()
}
