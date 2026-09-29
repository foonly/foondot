package foondot

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/dots"
	"foonly.dev/foondot/internal/git"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

// environment holds everything foondot takes from the system, so it can be
// replaced in tests.
type environment struct {
	version    string
	home       string
	configHome string
	dataDir    string
	hostname   string
}

// Execute runs foondot with the command line arguments and exits.
func Execute(version string) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	env := environment{
		version:    version,
		home:       xdg.Home,
		configHome: xdg.ConfigHome,
		dataDir:    config.DataDir(),
		hostname:   hostname,
	}
	os.Exit(run(os.Args[1:], env))
}

// run runs foondot with the given arguments and returns the exit code:
// 0 on success, 1 if the command failed and 2 for invalid usage. Reading the
// config file exits with 1 if it's missing and 2 if it's invalid.
func run(args []string, env environment) int {
	defaultConfigFile := filepath.Join(env.configHome, config.DefaultConfigFileName)

	flags := flag.NewFlagSet("foondot", flag.ContinueOnError)
	showVersion := flags.Bool("v", false, "Show version")
	showColor := flags.Bool("cc", false, "Show color")
	configFile := flags.String("c", defaultConfigFile, "Config file location")
	force := flags.Bool("f", false, "Force relink, and move files out of the way")
	dryRun := flags.Bool("n", false, "Show what sync would commit and push, without changing anything")

	flags.Usage = func() {
		out := flags.Output()
		fmt.Fprintf(out, "Usage: foondot [flags] [command]\n\n")
		fmt.Fprintf(out, "Commands:\n")
		fmt.Fprintf(out, "  link    Link dotfiles\n")
		fmt.Fprintf(out, "  sync    Sync dotfiles with git\n\n")
		fmt.Fprintf(out, "Flags:\n")
		flags.PrintDefaults()
	}
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}

	// Subcommand parsing
	command := ""
	if flags.NArg() > 0 {
		command = flags.Arg(0)
		// Parse again so flags are also accepted after the subcommand, e.g. `foondot link -f`.
		if code, ok := parseFlags(flags, flags.Args()[1:]); !ok {
			return code
		}
		if flags.NArg() > 0 {
			utils.PrintError("Unexpected argument", flags.Arg(0))
			flags.Usage()
			return 2
		}
	}

	if *showVersion {
		fmt.Fprintf(os.Stdout, "Version: %s\nHostname: %s\n", env.version, env.hostname)
		return 0
	}

	// Without a command, do nothing but show usage.
	if command == "" {
		flags.Usage()
		return 0
	}

	if command != "link" && command != "sync" {
		utils.PrintError("Unknown command", command)
		flags.Usage()
		return 2
	}

	// A dry run of link isn't supported, so don't let -n silently link.
	if *dryRun && command != "sync" {
		utils.PrintError("The -n flag only applies to", "sync")
		return 2
	}

	if *showColor {
		utils.Color = true
	}

	// Check if using default config file and if it exists.
	if *configFile == defaultConfigFile && utils.GetType(*configFile) == utils.NotExists {
		if err := config.CreateDefaultConfig(defaultConfigFile); err != nil {
			utils.PrintErrorCause("Error writing default config", defaultConfigFile, err)
			return 1
		}
		return 0
	}

	cfg, err := config.ReadConfig(*configFile)
	if errors.Is(err, fs.ErrNotExist) {
		utils.PrintError("Config file not found in", *configFile)
		return 1
	} else if err != nil {
		utils.PrintWarning(err.Error())
		return 2
	}
	if cfg.Color {
		utils.Color = true
	}

	switch command {
	case "link":
		err = dots.Link(cfg, dots.Options{
			Home:     env.home,
			Hostname: env.hostname,
			DataDir:  env.dataDir,
			Force:    *force,
		})
		if err != nil {
			utils.PrintError("Link failed", err.Error())
			return 1
		}
	case "sync":
		err = git.Sync(cfg.DotfilesDir(env.home), git.Options{
			Strategy: cfg.SyncStrategy,
			DryRun:   *dryRun,
		})
		if err != nil {
			utils.PrintError("Sync failed", err.Error())
			return 1
		}
	}
	return 0
}

// parseFlags parses args into flags. It returns false with the exit code if
// foondot should stop: 0 after showing help, 2 for invalid flags.
func parseFlags(flags *flag.FlagSet, args []string) (int, bool) {
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return 0, false
	} else if err != nil {
		return 2, false
	}
	return 0, true
}
