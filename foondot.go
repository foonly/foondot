package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

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
	Color    bool
	Dots     []Item
}

var version = "undefined"
var color = false

/**
 * Main function.
 */
func main() {
	configFile := flag.String("c", path.Join(xdg.ConfigHome, defaultConfigFileName), "Config file location")
	force := flag.Bool("f", false, "Force relink, and move files out of the way")
	showVersion := flag.Bool("v", false, "Show version")
	showColor := flag.Bool("cc", false, "Show color")
	flag.Parse()

	if *showColor {
		color = true
	}
	cfg := readConfig(*configFile)
	if cfg.Color {
		color = true
	}

	if *showVersion {
		fmt.Fprintf(os.Stdout, "Version: %s\n", version)
		os.Exit(0)
	}

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
		printError("Config file not found in", configFile)
		os.Exit(1)
	}

	var cfg Config

	// Reading from a TOML file
	err = toml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		printError("Error reading TOML file", configFile, err.Error())
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
		printMessage("Created directory", dir)
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
		printError("Target is a "+isDirFile, target)
		sourceType := getType(source)

		if sourceType == notExists {
			err := os.Rename(target, source)
			if err == nil {
				printMessage("Moving before linking", target, source)
			}
		} else if force {
			sourceConflict := source + ".conflict"
			count := 0
			for {
				// Find an available filename
				conflictType := getType(sourceConflict)
				if conflictType == notExists {
					break
				}
				count++
				sourceConflict = source + ".conflict." + strconv.Itoa(count)
			}

			err := os.Rename(target, sourceConflict)
			if err == nil {
				printMessage("Both source and target exist, forcing move out of the way", target, sourceConflict)
			} else {
				printError("Couldn't backup target, skipping", target)
			}
		} else {
			printError("Both source and target exist. Skipping", source, "Use -f to override.")
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
		printError("Source does not exist", source)
		return false
	}
	if sourceType == isSymlink {
		printError("Source is a symlink", source)
		return false
	}

	if targetType == notExists && (sourceType == isDirectory || sourceType == isFile) {
		err := os.Symlink(source, target)
		printMessage("Linking", source, target)
		if err != nil {
			printError("Error linking", target)
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

/**
 * Prints a message to the console.
 */
func printMessage(text ...string) {
	if len(text) >= 3 {
		if color {
			fmt.Fprintf(os.Stdout, "%s: %s%s%s => %s%s%s\n", text[0], colorGreen, text[1], colorNone, colorYellow, text[2], colorNone)
		} else {
			fmt.Fprintf(os.Stdout, "%s: %s => %s\n", text[0], text[1], text[2])
		}
	} else if len(text) == 2 {
		if color {
			fmt.Fprintf(os.Stdout, "%s: %s%s%s\n", text[0], colorYellow, text[1], colorNone)
		} else {
			fmt.Fprintf(os.Stdout, "%s: %s\n", text[0], text[1])
		}
	}
}

/**
 * Prints an error message to the console.
 */
func printError(text ...string) {
	if len(text) >= 3 {
		if color {
			fmt.Fprintf(os.Stderr, "%s%s: %s%s%s\n%s\n", colorRed, text[0], colorYellow, text[1], colorNone, text[2])
		} else {
			fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", text[0], text[1], text[2])
		}
	} else if len(text) == 2 {
		if color {
			fmt.Fprintf(os.Stderr, "%s%s: %s%s%s\n", colorRed, text[0], colorYellow, text[1], colorNone)
		} else {
			fmt.Fprintf(os.Stderr, "%s: %s\n", text[0], text[1])
		}
	} else {
		if color {
			fmt.Fprintf(os.Stderr, "%s%s\n", colorRed, text[0])
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", text[0])
		}
	}
}
