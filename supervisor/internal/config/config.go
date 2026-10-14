// Package config loads /etc/deploykit/game.yaml, the per-image description the
// supervisor runs: which hooks prepare the server, how the game is started, how
// console lines reach it, how readiness is detected and how it is stopped.
package config

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultPath is where every image installs its game.yaml.
const DefaultPath = "/etc/deploykit/game.yaml"

// Console adapters. The contract always takes console input on stdin; the
// adapter is how the supervisor forwards a line to the game.
const (
	AdapterStdin = "stdin"
	AdapterRCON  = "rcon"
	AdapterNone  = "none"
)

// Verbs a game.yaml may translate. "status" and "stop" are built in.
var translatableVerbs = map[string]bool{"save": true, "save-off": true, "save-on": true}

var stopSignals = map[string]bool{
	"SIGINT": true, "SIGTERM": true, "SIGQUIT": true, "SIGHUP": true, "SIGUSR1": true, "SIGUSR2": true,
}

type Phase struct {
	Name string   `yaml:"name"`
	Run  []string `yaml:"run"`
}

type RCON struct {
	// Port is the container-local RCON port. PortEnv, when set and present in
	// the environment, wins: CS2's RCON listens on the game port, which is
	// only known at start.
	Port    int    `yaml:"port"`
	PortEnv string `yaml:"portEnv"`
}

type Console struct {
	Adapter string `yaml:"adapter"`
	RCON    RCON   `yaml:"rcon"`
}

type Ready struct {
	Pattern string `yaml:"pattern"`
}

type Stop struct {
	// Command is sent through the console adapter; Signal is delivered to the
	// game process instead. With neither, the game gets SIGTERM.
	Command string        `yaml:"command"`
	Signal  string        `yaml:"signal"`
	Timeout time.Duration `yaml:"timeout"`
}

type User struct {
	UID int `yaml:"uid"`
	GID int `yaml:"gid"`
}

type Config struct {
	Game      string              `yaml:"game"`
	User      User                `yaml:"user"`
	DataDir   string              `yaml:"dataDir"`
	ServerDir string              `yaml:"serverDir"`
	Workdir   string              `yaml:"workdir"`
	Phases    []Phase             `yaml:"phases"`
	Start     []string            `yaml:"start"`
	Console   Console             `yaml:"console"`
	Ready     Ready               `yaml:"ready"`
	Stop      Stop                `yaml:"stop"`
	Verbs     map[string][]string `yaml:"verbs"`

	ReadyRegexp *regexp.Regexp `yaml:"-"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

func Parse(raw []byte) (*Config, error) {
	cfg := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("game.yaml: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("game.yaml: %w", err)
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.User.UID == 0 && c.User.GID == 0 {
		c.User = User{UID: 1000, GID: 1000}
	}
	if c.DataDir == "" {
		c.DataDir = "/data"
	}
	if c.ServerDir == "" {
		c.ServerDir = "/server"
	}
	if c.Workdir == "" {
		c.Workdir = c.DataDir
	}
	if c.Console.Adapter == "" {
		c.Console.Adapter = AdapterNone
	}
	if c.Stop.Timeout == 0 {
		c.Stop.Timeout = 30 * time.Second
	}
}

func (c *Config) validate() error {
	if c.Game == "" {
		return fmt.Errorf("game is required")
	}
	if c.User.UID <= 0 || c.User.GID <= 0 {
		return fmt.Errorf("user must not be root")
	}
	if len(c.Start) == 0 {
		return fmt.Errorf("start is required")
	}
	for i, p := range c.Phases {
		if p.Name == "" || len(p.Run) == 0 {
			return fmt.Errorf("phases[%d] needs a name and a run command", i)
		}
	}
	switch c.Console.Adapter {
	case AdapterStdin, AdapterNone:
	case AdapterRCON:
		if c.Console.RCON.Port <= 0 || c.Console.RCON.Port > 65535 {
			return fmt.Errorf("console.rcon.port must be a port number")
		}
	default:
		return fmt.Errorf("console.adapter must be stdin, rcon or none")
	}
	if c.Ready.Pattern == "" {
		return fmt.Errorf("ready.pattern is required")
	}
	re, err := regexp.Compile(c.Ready.Pattern)
	if err != nil {
		return fmt.Errorf("ready.pattern: %w", err)
	}
	c.ReadyRegexp = re
	if c.Stop.Signal != "" && !stopSignals[c.Stop.Signal] {
		return fmt.Errorf("stop.signal %q is not supported", c.Stop.Signal)
	}
	if c.Stop.Signal != "" && c.Stop.Command != "" {
		return fmt.Errorf("stop takes a command or a signal, not both")
	}
	if c.Stop.Command != "" && c.Console.Adapter == AdapterNone {
		return fmt.Errorf("stop.command needs a console adapter")
	}
	for verb, cmds := range c.Verbs {
		if !translatableVerbs[verb] {
			return fmt.Errorf("verbs.%s is not a translatable verb (save, save-off, save-on)", verb)
		}
		if len(cmds) == 0 {
			return fmt.Errorf("verbs.%s is empty", verb)
		}
		if c.Console.Adapter == AdapterNone {
			return fmt.Errorf("verbs need a console adapter")
		}
	}
	return nil
}
