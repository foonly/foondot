package main

import (
	"fmt"
	"os"

	"path"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

func main() {
	readConfig()
}

type Item struct {
	Source string
	Target string
}

type Config struct {
	Version  int
	Dotfiles string
	Dots     []Item
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func readConfig() {
	data, err := os.ReadFile(path.Join(xdg.ConfigHome, "dotsync/config.toml"))
	check(err)

	var cfg Config

	// Reading from a TOML file
	err = toml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		fmt.Println("Error reading TOML file:", err)
		os.Exit(1)
	}

	for _, element := range cfg.Dots {
		handleDot(element, cfg.Dotfiles)
	}
}

func handleDot(item Item, dotfiles string) {
	fmt.Println("Source: ", path.Join(xdg.ConfigHome, dotfiles, item.Source))
	fmt.Println("Target: ", path.Join(xdg.ConfigHome, item.Target))
}
