package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"

	"path"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

/**
 * Constants
 */
const (
	defaultConfigFileName = "foondot.toml"
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

/**
 * A single dotfile.
 */
type Item struct {
	Source string
	Target string
}

/**
 * The config file structure.
 */
type Config struct {
	Version  int
	Dotfiles string
	Dots     []Item
}

var version = "undefined"

/**
 * Main function.
 */
func main() {
	configFile := flag.String("c", path.Join(xdg.ConfigHome, defaultConfigFileName), "Config file location")
	force := flag.Bool("f", false, "Force overwrite")
	showVersion := flag.Bool("v", false, "Show version")
	flag.Parse()
	if *showVersion {
		fmt.Fprintf(os.Stdout, "Version: %s\n", version)
		os.Exit(0)
	}
	cfg := readConfig(*configFile)

	numberLinked := 0
	for _, element := range cfg.Dots {
		if handleDot(element, cfg.Dotfiles, *force) {
			numberLinked++
		}
	}

	if *force {
		fmt.Fprintf(os.Stdout, "Force mode enabled\n")
	}
	if numberLinked == 0 {
		fmt.Fprintf(os.Stdout, "No new dotfiles linked.\n")
	} else if numberLinked == len(cfg.Dots) {
		fmt.Fprintf(os.Stdout, "All %d dotfiles linked.\n", len(cfg.Dots))
	} else {
		fmt.Fprintf(os.Stdout, "%d of %d dotfiles linked.\n", numberLinked, len(cfg.Dots))
	}
}

/**
 * Reads the config file.
 */
func readConfig(configFile string) Config {
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%sConfig file not found in %s%s%s\n", colorRed, colorYellow, configFile, colorNone)
		os.Exit(1)
	}

	var cfg Config

	// Reading from a TOML file
	err = toml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%sError reading TOML file: %s%s%s\n%s\n", colorRed, colorYellow, configFile, colorNone, err)
		os.Exit(2)
	}

	return cfg
}

/**
 * Handles a single dotfile.
 */
func handleDot(item Item, dotfiles string, force bool) bool {
	source := path.Join(xdg.Home, dotfiles, item.Source)
	target := path.Join(xdg.Home, item.Target)

	prepareTargetSource(target, source, force)

	return doLink(source, target)
}

/**
 * Prepare target & source.
 */
func prepareTargetSource(target string, source string, force bool) {
	dir := path.Dir(target)
	err := os.Mkdir(dir, os.ModePerm)
	if err == nil {
		// No error means directory was created.
		fmt.Fprintf(os.Stdout, "Created directory %s%s%s\n", colorYellow, dir, colorNone)
	}

	targetType := getType(target)

	if targetType == isSymlink && force {
		// Remove target if it's a symlink.
		os.Remove(target)
	}
	if targetType == isDirectory || targetType == isFile {
		// Target is not a symlink.
		isDirFile := "file"
		if targetType == isDirectory {
			isDirFile = "directory"
		}
		fmt.Fprintf(os.Stdout, "Target is a %s: %s%s%s\n", isDirFile, colorYellow, target, colorNone)
		sourceType := getType(source)

		if sourceType == notExists {
			err := os.Rename(target, source)
			if err == nil {
				fmt.Fprintf(os.Stdout, "Moving before linking: %s%s%s => %s%s%s\n", colorYellow, target, colorNone, colorYellow, source, colorNone)
			}
		} else if force {
			sourceConflict := source + ".conflict"

			err := os.Rename(target, sourceConflict)
			if err == nil {
				fmt.Fprintf(os.Stdout, "Both source and target exist, forcing move out of the way: %s%s%s => %s%s%s\n", colorYellow, target, colorNone, colorYellow, sourceConflict, colorNone)
			}
		}
	}
}

/**
 * Do the actual linking.
 */
func doLink(source string, target string) bool {
	sourceType := getType(source)
	targetType := getType(target)

	if sourceType == notExists {
		fmt.Fprintf(os.Stdout, "%sSource does not exist: %s%s%s\n", colorRed, colorYellow, source, colorNone)
		return false
	}
	if sourceType == isSymlink {
		fmt.Fprintf(os.Stdout, "%sSource is a symlink: %s%s%s\n", colorRed, colorYellow, source, colorNone)
		return false
	}

	if targetType == notExists && (sourceType == isDirectory || sourceType == isFile) {
		err := os.Symlink(source, target)
		fmt.Fprintf(os.Stdout, "Linking: %s%s%s => %s%s%s\n", colorYellow, source, colorNone, colorYellow, target, colorNone)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Error linking: %s%s%s\n", colorYellow, target, colorNone)
		}
		return err == nil
	}
	return false
}

/**
 * Returns the type of a file.
 */
func getType(fileName string) int {
	stat, err := os.Lstat(fileName)
	if errors.Is(err, os.ErrNotExist) {
		return notExists
	} else if err != nil {
		return isFailed
	}
	if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
		return isSymlink
	}
	if stat.Mode()&os.ModeDir == os.ModeDir {
		return isDirectory
	}
	return isFile
}
