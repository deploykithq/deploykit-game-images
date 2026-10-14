package mods

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// CLI implements `dk-entrypoint mods <sync|modpack> ...`. Hooks call it; the
// CurseForge key is read from DK_CURSEFORGE_API_KEY, never from a flag, so it
// does not show up in a process listing.
func CLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: dk-entrypoint mods <sync|modpack> [flags]")
		return 2
	}
	c := NewClient()
	switch args[0] {
	case "sync":
		fs := flag.NewFlagSet("mods sync", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var o SyncOptions
		var loaders string
		fs.StringVar(&o.Source, "source", "", "modrinth or curseforge")
		fs.StringVar(&o.List, "list", "", "comma-separated project[:version] list")
		fs.StringVar(&o.Dir, "dir", "", "directory the files go to")
		fs.StringVar(&o.State, "state", "", "lock file")
		fs.StringVar(&loaders, "loader", "", "comma-separated loaders to filter by")
		fs.StringVar(&o.GameVersion, "game-version", "", "game version to filter by")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if o.Source == "" || o.Dir == "" || o.State == "" {
			fmt.Fprintln(stderr, "mods sync: --source, --dir and --state are required")
			return 2
		}
		for _, l := range strings.Split(loaders, ",") {
			if l = strings.TrimSpace(l); l != "" {
				o.Loaders = append(o.Loaders, l)
			}
		}
		if err := c.Sync(o, stdout); err != nil {
			fmt.Fprintf(stderr, "mods sync: %v\n", err)
			return 1
		}
		return 0
	case "modpack":
		fs := flag.NewFlagSet("mods modpack", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var o PackOptions
		fs.StringVar(&o.Source, "source", "", "modrinth or curseforge")
		fs.StringVar(&o.Project, "project", "", "modpack project (empty uninstalls)")
		fs.StringVar(&o.Version, "version", "", "modpack version/file id (empty = latest)")
		fs.StringVar(&o.Dir, "dir", "", "server directory")
		fs.StringVar(&o.State, "state", "", "state file")
		fs.StringVar(&o.EnvOut, "env-out", "", "file to write DK_RESOLVED_* to")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if o.Dir == "" || o.State == "" || o.EnvOut == "" {
			fmt.Fprintln(stderr, "mods modpack: --dir, --state and --env-out are required")
			return 2
		}
		if err := c.Modpack(o, stdout); err != nil {
			fmt.Fprintf(stderr, "mods modpack: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "mods: unknown command %q\n", args[0])
		return 2
	}
}
