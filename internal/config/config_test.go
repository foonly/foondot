package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateConfigSyncStrategy(t *testing.T) {
	for _, strategy := range []string{"manual", "local", "remote"} {
		if err := validateConfig(Config{SyncStrategy: strategy}); err != nil {
			t.Errorf("%s: unexpected error %v", strategy, err)
		}
	}
	for _, strategy := range []string{"remot", "Local", "theirs"} {
		if err := validateConfig(Config{SyncStrategy: strategy}); err == nil {
			t.Errorf("%s: expected an error", strategy)
		}
	}
}

func writeDotsData(t *testing.T, dataDir, content string) {
	t.Helper()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, dotsDataFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadDotsData(t *testing.T) {
	root := t.TempDir()

	got, err := ReadDotsData(filepath.Join(root, "missing"))
	if err != nil || len(got) != 0 {
		t.Errorf("missing file: got (%v, %v), want empty list", got, err)
	}

	valid := filepath.Join(root, "valid")
	writeDotsData(t, valid, `["/a","/b"]`)
	got, err = ReadDotsData(valid)
	if err != nil || !slices.Equal(got, []string{"/a", "/b"}) {
		t.Errorf("valid file: got (%v, %v)", got, err)
	}

	broken := filepath.Join(root, "broken")
	writeDotsData(t, broken, `["/a",`)
	if _, err := ReadDotsData(broken); err == nil {
		t.Error("broken file: expected an error")
	}

	// A directory can't be read as a file, which must not count as missing.
	unreadable := filepath.Join(root, "unreadable")
	if err := os.MkdirAll(filepath.Join(unreadable, dotsDataFileName), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDotsData(unreadable); err == nil {
		t.Error("unreadable file: expected an error")
	}
}

func TestWriteDotsDataRoundTrip(t *testing.T) {
	// The data folder doesn't exist yet and must be created.
	dataDir := filepath.Join(t.TempDir(), "foondot")
	want := []string{"/home/u/.bashrc", "/etc/foo"}

	if err := WriteDotsData(dataDir, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDotsData(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "foondot.toml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestReadConfig(t *testing.T) {
	file := writeConfig(t, `
dotfiles = "dots"
color = true
dots = [
    { source = "bashrc", target = ".bashrc" },
    { source = "sway", target = ".config/sway", hostname = ["laptop"] },
]
`)

	cfg, err := ReadConfig(file)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Dotfiles != "dots" || !cfg.Color {
		t.Errorf("got %+v", cfg)
	}
	if cfg.SyncStrategy != "manual" {
		t.Errorf("sync_strategy = %q, want default %q", cfg.SyncStrategy, "manual")
	}
	if len(cfg.Dots) != 2 || cfg.Dots[1].Target != ".config/sway" || !slices.Equal(cfg.Dots[1].Hostname, []string{"laptop"}) {
		t.Errorf("dots = %+v", cfg.Dots)
	}
	if got := cfg.DotfilesDir("/home/u"); got != "/home/u/dots" {
		t.Errorf("DotfilesDir = %s", got)
	}
}

func TestReadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"unknown key", "dotfiles = \"dotfiles\"\ndot = []\n", "unknown keys"},
		{"unknown key in dot", "dots = [{ source = \"a\", taget = \"b\" }]\n", "unknown keys"},
		{"invalid syntax", "dotfiles = \n", "error reading"},
		{"wrong type", "color = \"yes\"\n", "error reading"},
		{"invalid strategy", "sync_strategy = \"remot\"\n", "sync_strategy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadConfig(writeConfig(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

func TestReadConfigMissing(t *testing.T) {
	_, err := ReadConfig(filepath.Join(t.TempDir(), "missing.toml"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist", err)
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	// The config folder doesn't exist yet and must be created.
	file := filepath.Join(t.TempDir(), "config", "foondot.toml")

	if err := CreateDefaultConfig(file); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadConfig(file)
	if err != nil {
		t.Fatalf("default config can't be read back: %v", err)
	}
	if cfg.Dotfiles != "dotfiles" || cfg.SyncStrategy != "manual" || len(cfg.Dots) != 0 {
		t.Errorf("got %+v", cfg)
	}
}
