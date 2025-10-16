package main

import (
	"bufio"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"path"

	"github.com/adrg/xdg"
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
	configFile := flag.String("config", path.Join(xdg.ConfigHome, "dotsync"), "Config file location")
	flag.Parse()
	fmt.Println("Version:", version)
	cfg := readConfig(*configFile)
	executeDots(cfg)
}

/**
 * Reads the config file and handles the dotfiles.
 */
func readConfig(configFile string) Config {
	file, err := os.Open(configFile)
	if err != nil {
		fmt.Println("Config file not found in", configFile)
		os.Exit(1)
	}
	defer file.Close()

	// Initialize cfg
	var cfg Config

	inDots := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		if inDots {
			// Read dot definitions.
			fields := strings.Fields(line)
			if len(fields) == 2 {
				var item Item
				item.Source = fields[0]
				item.Target = fields[1]
				cfg.Dots = append(cfg.Dots, item)
			}
		} else {
			// Read pre dots config.
			if strings.HasPrefix(line, "[dots]") {
				inDots = true
				continue
			}
			config := strings.SplitN(line, "=", 2)
			switch strings.TrimSpace(config[0]) {
			case "version":
				version, err := strconv.Atoi(strings.TrimSpace(config[1]))
				if err == nil {
					cfg.Version = version
				}
			case "dotfiles":
				cfg.Dotfiles = strings.TrimSpace(config[1])
			default:
				fmt.Println("Unknown config directive:", config[0])
			}
		}
	}

	return cfg
}

/**
 * Reads the data file.
 */
func readData() []Item {
	var data []Item
	fileName := path.Join(xdg.DataHome, "dotsyncData")
	fmt.Println(fileName)
	return data
}

func executeDots(cfg Config) {
	data := readData()
	fmt.Println(data)
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
