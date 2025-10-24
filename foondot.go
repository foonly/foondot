package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"path"

	"github.com/adrg/xdg"
)

/**
 * Constants
 */
const (
	defaultConfigFileName = "foondot.toml"
	dataFolderName        = "foondot"
	dotsDataFileName      = "dots.json"
	colorNone             = "\033[0m"
	colorRed              = "\033[0;31m"
	colorGreen            = "\033[0;32m"
	colorYellow           = "\033[0;33m"
)

/**
 * File types.
 */
const (
	isFailed    = iota
	notExists   = iota
	isSymlink   = iota
	isDirectory = iota
	isFile      = iota
)

var version = "undefined"
var color = false
var hostname = "unknown"

func main() {
	defaultConfigFile := path.Join(xdg.ConfigHome, defaultConfigFileName)
	configFile := flag.String("c", defaultConfigFile, "Config file location")
	force := flag.Bool("f", false, "Force relink, and move files out of the way")
	showVersion := flag.Bool("v", false, "Show version")
	showColor := flag.Bool("cc", false, "Show color")
	flag.Parse()

	hostname, _ = os.Hostname()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "Version: %s\nHostname: %s\n", version, hostname)
		os.Exit(0)
	}

	// Check if using default config file and if it exists.
	if *configFile == defaultConfigFile && getType(*configFile) == notExists {
		createDefaultConfig(defaultConfigFile)
		os.Exit(0)
	}

	if *showColor {
		color = true
	}
	cfg := readConfig(*configFile)
	if cfg.Color {
		color = true
	}

	readDotsData()

	dots := filterDots(cfg.Dots)

	numberLinked := 0
	for _, element := range filterDots(dots) {
		if handleDot(element, cfg.Dotfiles, *force) {
			numberLinked++
		}
	}

	if *force {
		fmt.Fprintf(os.Stdout, "Force mode enabled\n")
	}
	writeDotsData()
	if numberLinked == 0 {
		fmt.Fprintf(os.Stdout, "No new dotfiles linked.\n")
	} else if numberLinked == len(dots) {
		fmt.Fprintf(os.Stdout, "All %d dotfiles linked.\n", len(dots))
	} else {
		fmt.Fprintf(os.Stdout, "%d of %d dotfiles linked.\n", numberLinked, len(dots))
	}
}
