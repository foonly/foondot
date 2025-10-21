package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"

	"path"

	"github.com/adrg/xdg"
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

var version = "undefined"
var color = false
var hostname = "unknown"

/**
 * Main function.
 */
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
* Handles a single dotfile item, determining source and target paths,
* preparing the target location, and creating the symlink.
*
* @param item The dotfile item to handle.
* @param dotfiles The base directory for dotfiles.
* @param force Whether to force relinking and move existing files.
* @return True if the link was successfully created, false otherwise.
 */
func handleDot(item Item, dotfiles string, force bool) bool {

	// Skip if hostname is defined and not matching a record in the array.
	if len(item.Hostname) > 0 && !slices.Contains(item.Hostname, hostname) {
		return false
	}

	source := path.Join(xdg.Home, dotfiles, item.Source)
	target := path.Join(xdg.Home, item.Target)

	prepareTargetSource(target, source, force)

	return doLink(source, target)
}

/**
 * Prepares the target location for a symlink. This includes creating parent
 * directories, removing existing symlinks (if force is enabled), and moving
 * existing files or directories out of the way to avoid conflicts.
 *
 * @param target The path to the target location for the symlink.
 * @param source The path to the source file or directory that will be linked.
 * @param force Whether to force relinking, moving existing files if necessary.
 */
func prepareTargetSource(target string, source string, force bool) {
	targetDir := path.Dir(target)
	if getType(targetDir) == notExists {
		err := os.MkdirAll(targetDir, os.ModePerm)
		if err == nil {
			// No error means directory was created.
			printMessage("Created directory", targetDir)
		}
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
			sourceDir := path.Dir(source)
			if getType(sourceDir) == notExists {
				err := os.MkdirAll(sourceDir, os.ModePerm)
				if err == nil {
					// No error means directory was created.
					printMessage("Created directory", sourceDir)
				} else {
					printError("Couldn't create directory", sourceDir)
				}
			}

			moveErr := os.Rename(target, source)
			if moveErr == nil {
				printMessage("Moving before linking", target, source)
			}
		} else if force {
			printMessage("force", source)
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
 * Creates a symbolic link from source to target. Checks if source exists and
 * is not a symlink. Checks if the target does not exist and the source is
 * either a directory or a file.
 *
 * @param source The path to the source file or directory.
 * @param target The path to the target location for the symlink.
 * @return True if the link was successfully created, false otherwise.
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
 * Determines the type of a file or directory.
 *
 * @param fileName The path to the file or directory.
 * @return An integer representing the file type (notExists, isSymlink, isDirectory, isFile, isFailed).
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
