package main

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

/**
 * A single dotfile.
 */
type Item struct {
	Source   string
	Target   string
	Hostname []string
}

/**
 * The config file structure.
 */
type Config struct {
	Dotfiles string `toml:"dotfiles" comment:"Path to your dotfiles relative to your $HOME directory"`
	Color    bool   `toml:"color"    comment:"Enable color output"`
	Dots     []Item `toml:"dots"     comment:"A dot entry representing a symlink, 'source' is relative to 'dotfiles'\nand 'target' shall be relative to $HOME directory or absolute.\nExample:\ndots = [{source = 'bash/bashrc', target = '.bashrc'}]"`
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
 * Create an empty default config file.
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
