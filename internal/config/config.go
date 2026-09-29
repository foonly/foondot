package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

const (
	DefaultConfigFileName = "foondot.toml"
	dataFolderName        = "foondot"
	dotsDataFileName      = "dots.json"
	backupFolderName      = "backup"
)

// Item represents a single dotfile symlink configuration.
type Item struct {
	// Source is the path to the source file, relative to the dotfiles directory.
	Source string
	// Target is the path to the target location, either relative to $HOME or absolute.
	Target string
	// Hostname lists the hosts this symlink applies to. If empty, it applies to all hosts.
	Hostname []string
}

// Config represents the application's configuration settings.
type Config struct {
	Dotfiles     string `toml:"dotfiles"      comment:"Path to your dotfiles relative to your $HOME directory"`
	Color        bool   `toml:"color"         comment:"Enable color output"`
	SyncStrategy string `toml:"sync_strategy" comment:"Strategy for resolving git conflicts (manual, local, remote)"`
	Dots         []Item `toml:"dots"          comment:"A dot entry representing a symlink, 'source' is relative to 'dotfiles'\nand 'target' shall be relative to $HOME directory or absolute.\nExample:\ndots = [{source = 'bash/bashrc', target = '.bashrc'}]"`
}

// DotfilesDir returns the absolute path to the dotfiles directory.
func (c Config) DotfilesDir(home string) string {
	return filepath.Join(home, c.Dotfiles)
}

// ReadConfig reads and validates the configuration from a TOML file.
// Unknown keys are rejected, since a typo could otherwise leave the dots list
// empty and cause every link to be removed.
func ReadConfig(configFile string) (Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	err = toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&cfg)
	if err != nil {
		var strictErr *toml.StrictMissingError
		if errors.As(err, &strictErr) {
			return Config{}, fmt.Errorf("unknown keys in %s:\n%s", configFile, strictErr.String())
		}
		return Config{}, fmt.Errorf("error reading %s: %w", configFile, err)
	}

	if cfg.SyncStrategy == "" {
		cfg.SyncStrategy = "manual"
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config file %s: %w", configFile, err)
	}

	return cfg, nil
}

// validateConfig checks configuration values that can't be expressed in the TOML types.
func validateConfig(cfg Config) error {
	switch cfg.SyncStrategy {
	case "manual", "local", "remote":
	default:
		return fmt.Errorf("sync_strategy must be 'manual', 'local' or 'remote', got '%s'", cfg.SyncStrategy)
	}
	return nil
}

// CreateDefaultConfig writes a default configuration file, creating its
// directory if needed.
func CreateDefaultConfig(configFile string) error {
	defaultConfig := Config{
		Dotfiles:     "dotfiles",
		Color:        false,
		SyncStrategy: "manual",
		Dots:         []Item{},
	}

	utils.PrintValue("Creating config file in", configFile)

	data, err := toml.Marshal(defaultConfig)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

// DataDir returns the folder for foondot's data, such as the tracked links
// and backups. The folder is not created.
func DataDir() string {
	return filepath.Join(xdg.DataHome, dataFolderName)
}

// BackupDir returns the folder inside dataDir where targets are backed up
// when forcing a relink. The folder is not created.
func BackupDir(dataDir string) string {
	return filepath.Join(dataDir, backupFolderName)
}

// ReadDotsData reads the list of tracked link targets from dataDir.
// A missing file is not an error and gives an empty list. Any other error is
// returned, since continuing would overwrite the file and lose track of all links.
func ReadDotsData(dataDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, dotsDataFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}
	var dotsData []string
	if err := json.Unmarshal(data, &dotsData); err != nil {
		return nil, err
	}
	return dotsData, nil
}

// WriteDotsData writes the list of tracked link targets to dataDir, creating
// the folder if needed. The file is replaced atomically.
func WriteDotsData(dataDir string, dotsData []string) error {
	data, err := json.Marshal(dotsData)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	return utils.WriteFileAtomic(filepath.Join(dataDir, dotsDataFileName), data, 0644)
}
