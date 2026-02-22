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
	command := "link"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "link":
		runLink(cfg, *force)
	case "sync":
		err := git.Sync(cfg)
		if err != nil {
			utils.PrintError("Sync failed:", err.Error())
			os.Exit(1)
		}
	default:
		// Fallback to link for backward compatibility or unrecognized commands
		runLink(cfg, *force)
	}
}

func runLink(cfg config.Config, force bool) {
	config.ReadDotsData()

	dotFiles := dots.FilterDots(cfg.Dots)

	numberLinked := 0
	for _, element := range dotFiles {
		if dots.HandleDot(element, cfg.Dotfiles, force) {
			numberLinked++
		}
	}

	dots.CleanTargets(dotFiles)

	config.WriteDotsData()

	if force {
		fmt.Fprintf(os.Stdout, "Force mode enabled\n")
	}
	if numberLinked == 0 {
		fmt.Fprintf(os.Stdout, "No new dotfiles linked.\n")
	} else if numberLinked == len(dotFiles) {
		fmt.Fprintf(os.Stdout, "All %d dotfiles linked.\n", len(dotFiles))
	} else {
		fmt.Fprintf(os.Stdout, "%d of %d dotfiles linked.\n", numberLinked, len(dotFiles))
	}
}
