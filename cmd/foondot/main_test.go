package foondot

import (
	"os"
	"path/filepath"
	"testing"

	"foonly.dev/foondot/internal/utils"
)

// testEnv returns an environment with a temporary home, config and data folder.
func testEnv(t *testing.T) environment {
	t.Helper()
	home := t.TempDir()
	return environment{
		version:    "test",
		home:       home,
		configHome: filepath.Join(home, ".config"),
		dataDir:    filepath.Join(home, ".local", "share", "foondot"),
		hostname:   "thishost",
	}
}

func writeFile(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeTestConfig writes a config linking dotfiles/bashrc to ~/.bashrc and
// returns its path.
func writeTestConfig(t *testing.T, env environment) string {
	t.Helper()
	file := filepath.Join(env.configHome, "foondot.toml")
	writeFile(t, file, "dotfiles = \"dotfiles\"\ndots = [{ source = \"bashrc\", target = \".bashrc\" }]\n")
	writeFile(t, filepath.Join(env.home, "dotfiles", "bashrc"), "source\n")
	return file
}

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no command", nil, 0},
		{"help", []string{"-h"}, 0},
		{"version", []string{"-v"}, 0},
		{"unknown command", []string{"bogus"}, 2},
		{"unexpected argument", []string{"link", "extra"}, 2},
		{"unknown flag", []string{"-x"}, 2},
		{"unknown flag after command", []string{"link", "-x"}, 2},
		{"dry run of link", []string{"link", "-n"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testEnv(t)

			if got := run(tt.args, env); got != tt.want {
				t.Errorf("exit code = %d, want %d", got, tt.want)
			}
			// None of these may create a config file or anything else.
			if entries, _ := os.ReadDir(env.home); len(entries) != 0 {
				t.Errorf("home folder not empty: %v", entries)
			}
		})
	}
}

func TestRunCreatesDefaultConfig(t *testing.T) {
	env := testEnv(t)

	if got := run([]string{"link"}, env); got != 0 {
		t.Errorf("exit code = %d, want 0", got)
	}
	if utils.GetType(filepath.Join(env.configHome, "foondot.toml")) != utils.IsFile {
		t.Error("default config was not created")
	}
}

func TestRunConfigErrors(t *testing.T) {
	env := testEnv(t)
	invalid := filepath.Join(env.home, "invalid.toml")
	writeFile(t, invalid, "dot = []\n")

	if got := run([]string{"link", "-c", filepath.Join(env.home, "missing.toml")}, env); got != 1 {
		t.Errorf("missing config: exit code = %d, want 1", got)
	}
	if got := run([]string{"link", "-c", invalid}, env); got != 2 {
		t.Errorf("invalid config: exit code = %d, want 2", got)
	}
}

func TestRunLink(t *testing.T) {
	env := testEnv(t)
	configFile := writeTestConfig(t, env)

	if got := run([]string{"-c", configFile, "link"}, env); got != 0 {
		t.Fatalf("exit code = %d, want 0", got)
	}
	if utils.GetType(filepath.Join(env.home, ".bashrc")) != utils.IsSymlink {
		t.Error("dotfile was not linked")
	}
	if utils.GetType(filepath.Join(env.dataDir, "dots.json")) != utils.IsFile {
		t.Error("tracked links were not saved to the data folder")
	}
}

func TestRunFlagsAfterCommand(t *testing.T) {
	env := testEnv(t)
	configFile := writeTestConfig(t, env)
	// Both source and target exist, so linking needs -f.
	writeFile(t, filepath.Join(env.home, ".bashrc"), "target\n")

	if got := run([]string{"link", "-c", configFile}, env); got != 1 {
		t.Errorf("without -f: exit code = %d, want 1", got)
	}
	if got := run([]string{"link", "-c", configFile, "-f"}, env); got != 0 {
		t.Errorf("with -f after the command: exit code = %d, want 0", got)
	}
	if utils.GetType(filepath.Join(env.home, ".bashrc")) != utils.IsSymlink {
		t.Error("-f after the command was ignored")
	}
}

func TestRunSyncFailure(t *testing.T) {
	env := testEnv(t)
	configFile := writeTestConfig(t, env)

	// The dotfiles folder is not a git repository.
	if got := run([]string{"sync", "-c", configFile}, env); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
}
