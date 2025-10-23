package main

import (
	"encoding/json"
	"os"
	"path"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

/**
 * Item represents a single dotfile symlink configuration.
 *
 * Fields:
 *   Source:   The path to the source file, relative to the dotfiles directory.
 *   Target:   The path to the target location, either relative to $HOME or absolute.
 *   Hostname: A slice of hostnames for which this symlink should be applied. If empty, applies to all hosts.
 */
type Item struct {
	Source   string
	Target   string
	Hostname []string
}

/**
 * Config represents the application's configuration settings.
 *
 * Fields:
 *   Dotfiles: Path to the user's dotfiles directory, relative to $HOME.
 *   Color:    Whether to enable color output in the application's messages.
 *   Dots:     A slice of Item structs, each representing a dotfile symlink configuration.
 */
type Config struct {
	Dotfiles string `toml:"dotfiles" comment:"Path to your dotfiles relative to your $HOME directory"`
	Color    bool   `toml:"color"    comment:"Enable color output"`
	Dots     []Item `toml:"dots"     comment:"A dot entry representing a symlink, 'source' is relative to 'dotfiles'\nand 'target' shall be relative to $HOME directory or absolute.\nExample:\ndots = [{source = 'bash/bashrc', target = '.bashrc'}]"`
}

var dotsData = []string{}

/**
 * Reads the configuration from the specified TOML config file.
 * Exits the program with an error message if the file cannot be read or parsed.
 *
 * @param configFile The path to the configuration file.
 * @return Config The parsed configuration struct.
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
 * Creates a default configuration file at the specified path.
 * If the file cannot be created or written, the program exits with an error message.
 *
 * @param configFile The path where the default configuration file will be created.
 */
func createDefaultConfig(configFile string) {
	defaultConfig := Config{
		Dotfiles: "dotfiles",
		Color:    false,
		Dots:     []Item{},
	}

	printMessage("Creating config file in", configFile)

	data, err := toml.Marshal(defaultConfig)
	if err != nil {
		printError("Error marshaling default config", err.Error())
		os.Exit(3)
	}

	err = os.WriteFile(configFile, data, 0644)
	if err != nil {
		printError("Error writing default config", configFile, err.Error())
		os.Exit(4)
	}
}

/**
 * Reads the dots data from the JSON file specified by dotsDataFileName.
 * If the file does not exist, the function returns without error.
 * If the file exists but cannot be read or parsed, the program exits with an error message.
 *
 * The dots data is unmarshaled into the global variable dotsData.
 */
func readDotsData() {
	filename := getDataFilename(dotsDataFileName)
	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(data), &dotsData)
	if err != nil {
		printError("Error reading JSON file", filename, err.Error())
		os.Exit(2)
	}
}

/**
 * Writes the current dots data to the JSON file specified by dotsDataFileName.
 * If the data cannot be marshaled or written, the program exits with an error message.
 *
 * The dots data is marshaled from the global variable dotsData.
 */
func writeDotsData() {
	filename := getDataFilename(dotsDataFileName)
	data, err := json.Marshal(dotsData)
	if err != nil {
		printError("Error marshaling dots data", err.Error())
		os.Exit(3)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		printError("Error writing dots data", filename, err.Error())
		os.Exit(4)
	}
}

/**
 * Returns the full path to a data file within the application's data directory.
 * If the data directory does not exist, it is created.
 *
 * @param filename The name of the data file.
 * @return string The full path to the data file within the data directory.
 */
func getDataFilename(filename string) string {
	dataFolder := path.Join(xdg.DataHome, dataFolderName)
	if getType(dataFolder) == notExists {
		err := os.MkdirAll(dataFolder, 0755)
		if err != nil {
			printError("Error creating data folder", dataFolder, err.Error())
			os.Exit(5)
		}
	}

	return path.Join(dataFolder, filename)
}
