package foondot

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"path"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/dots"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

func Execute() {
	defaultConfigFile := path.Join(xdg.ConfigHome, config.DefaultConfigFileName)
	configFile := flag.String("c", defaultConfigFile, "Config file location")
	force := flag.Bool("f", false, "Force relink, and move files out of the way")
	showVersion := flag.Bool("v", false, "Show version")
	showColor := flag.Bool("cc", false, "Show color")
	flag.Parse()

	config.Hostname, _ = os.Hostname()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "Version: %s\nHostname: %s\n", config.Version, config.Hostname)
		os.Exit(0)
	}

	// Check if using default config file and if it exists.
	if *configFile == defaultConfigFile && utils.GetType(*configFile) == utils.NotExists {
		config.CreateDefaultConfig(defaultConfigFile)
		os.Exit(0)
	}

	if *showColor {
		utils.Color = true
	}
	cfg := config.ReadConfig(*configFile)
	if cfg.Color {
		utils.Color = true
	}

	config.ReadDotsData()

	dotFiles := dots.FilterDots(cfg.Dots)

	numberLinked := 0
	for _, element := range dotFiles {
		if dots.HandleDot(element, cfg.Dotfiles, *force) {
			numberLinked++
		}
	}

	dots.CleanTargets(dotFiles)

	config.WriteDotsData()

	if *force {
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
