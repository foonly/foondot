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

const (
	defaultConfigFileName = "foondot.toml"
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

//go:embed version.txt
var version string

/**
 * Main function.
 */
func main() {
	configFile := flag.String("config", path.Join(xdg.ConfigHome, defaultConfigFileName), "Config file location")
	flag.Parse()
	fmt.Println("Version:", version)
	readConfig(*configFile)
}

/**
 * Reads the config file and handles the dotfiles.
 */
func readConfig(configFile string) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println("Config file not found in", configFile)
		os.Exit(1)
	}

	var cfg Config

	// Reading from a TOML file
	err = toml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		fmt.Println("Error reading TOML file:", err)
		os.Exit(2)
	}

	for _, element := range cfg.Dots {
		handleDot(element, cfg.Dotfiles)
	}
}

/**
 * Handles a single dotfile.
 */
func handleDot(item Item, dotfiles string) {
	source := path.Join(xdg.Home, dotfiles, item.Source)
	target := path.Join(xdg.Home, item.Target)
	dir := path.Dir(target)
	err := os.Mkdir(dir, os.ModePerm)
	if err == nil {
		// No error means directory was created.
		fmt.Println("Created directory", dir)
	}
	fileType := getType(target)

	if fileType == isSymlink {
		// Remove target if it's a symlink.
		os.Remove(target)
	}
	if fileType == isDirectory || fileType == isFile {
		// Target is not a symlink.
		if getType(source) == notExists {
			err := os.Rename(target, source)
			if err == nil {
				fmt.Println("Moving before linking:", target, "=>", source)
			}
		}
	}
	err = os.Symlink(source, target)
	fmt.Println("Linking:", source, "=>", target)

	if err != nil {
		fmt.Println("Error linking:", target)
	}

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
