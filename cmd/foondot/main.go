package foondot

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/dots"
	"foonly.dev/foondot/internal/git"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

func Execute() {
	defaultConfigFile := path.Join(xdg.ConfigHome, config.DefaultConfigFileName)

	// Global flags
	showVersion := flag.Bool("v", false, "Show version")
	showColor := flag.Bool("cc", false, "Show color")
	configFile := flag.String("c", defaultConfigFile, "Config file location")
	force := flag.Bool("f", false, "Force relink, and move files out of the way")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [command]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  link    Link dotfiles (default)\n")
		fmt.Fprintf(os.Stderr, "  sync    Sync dotfiles with git\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	config.Hostname, _ = os.Hostname()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "Version: %s\nHostname: %s\n", config.Version, config.Hostname)
		os.Exit(0)
	}

	if *showColor {
		utils.Color = true
	}

	// Check if using default config file and if it exists.
	if *configFile == defaultConfigFile && utils.GetType(*configFile) == utils.NotExists {
		config.CreateDefaultConfig(defaultConfigFile)
		os.Exit(0)
	}

	cfg := config.ReadConfig(*configFile)
	if cfg.Color {
		utils.Color = true
	}

	// Subcommand parsing
	args := flag.Args()
	command := "sync"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "link":
		dots.Link(cfg, *force)
	case "sync":
		err := git.Sync(cfg)
		if err != nil {
			utils.PrintError("Sync failed", err.Error())
			os.Exit(1)
		}
	default:
		utils.PrintError("Unknown command", command)
	}
}
