package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

// Change represents a change in the git repository.
type Change struct {
	Status string // M, A, D, R, etc.
	Path   string // Path relative to repo root
}

// Sync handles the automatic git sync process for the dotfiles directory.
// It performs a pull with rebase, stages all changes, creates a smart commit message,
// and pushes the result back to the remote repository.
func Sync(cfg config.Config) error {
	dotfilesDir := path.Join(xdg.Home, cfg.Dotfiles)

	if !isRepo(dotfilesDir) {
		return fmt.Errorf("directory %s is not a git repository", dotfilesDir)
	}

	// 1. Pull changes first using rebase and autostash.
	// This ensures we have the latest remote changes and helps avoid merge commits.
	if err := pull(dotfilesDir); err != nil {
		return fmt.Errorf("failed to pull changes: %w. Please resolve conflicts manually", err)
	}

	utils.PrintMessage("Checking for changes in", dotfilesDir)

	// 2. Stage all changes before generating the status.
	// This ensures untracked files are included and represented correctly in the porcelain output.
	if err := stageAll(dotfilesDir); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	changes, err := getChanges(dotfilesDir)
	if err != nil {
		return fmt.Errorf("failed to get git status: %w", err)
	}

	if len(changes) == 0 {
		utils.PrintMessage("No local changes to sync.")
		return nil
	}

	// 3. Generate a human-readable commit message based on the staged changes.
	message := generateCommitMessage(dotfilesDir, changes)
	utils.PrintMessage("Committing:", message)
	if err := commit(dotfilesDir, message); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// 4. Push the new commit to the remote repository.
	utils.PrintMessage("Pushing changes...")
	if err := push(dotfilesDir); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	utils.PrintMessage("Sync completed successfully")
	return nil
}

// isRepo checks if the given path is inside a git repository.
func isRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// getChanges returns a list of changed files using git status --porcelain.
func getChanges(dir string) ([]Change, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, nil
	}

	lines := strings.Split(outputStr, "\n")
	var changes []Change

	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		// Porcelain format: XY PATH or XY PATH1 -> PATH2
		status := strings.TrimSpace(line[:2])
		filePath := strings.Trim(line[3:], "\"")

		changes = append(changes, Change{
			Status: status,
			Path:   filePath,
		})
	}

	return changes, nil
}

// pull performs a git pull --rebase with --autostash.
func pull(dir string) error {
	utils.PrintMessage("Pulling changes...")
	cmd := exec.Command("git", "pull", "--rebase", "--autostash")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stageAll runs git add -A to stage all changes.
func stageAll(dir string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	return cmd.Run()
}

// commit runs git commit -m <message> with the provided message.
func commit(dir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	return cmd.Run()
}

// push runs git push to send changes to the remote tracking branch.
func push(dir string) error {
	cmd := exec.Command("git", "push")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// hasInHEAD checks if a file or directory exists in the HEAD commit.
func hasInHEAD(dir, key string) bool {
	cmd := exec.Command("git", "ls-tree", "HEAD", key)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return len(strings.TrimSpace(string(out))) > 0
}

// hasInIndex checks if a file or directory exists in the git index.
func hasInIndex(dir, key string) bool {
	cmd := exec.Command("git", "ls-files", "--", key)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return len(strings.TrimSpace(string(out))) > 0
}

// generateCommitMessage creates a smart message based on the changed files and directories.
func generateCommitMessage(dir string, changes []Change) string {
	added := make(map[string]bool)
	updated := make(map[string]bool)
	removed := make(map[string]bool)

	keys := make(map[string]bool)

	for _, c := range changes {
		// Handle renames (Status starts with R). Format is usually "R  old -> new"
		if len(c.Status) > 0 && c.Status[0] == 'R' {
			parts := strings.Split(c.Path, " -> ")
			if len(parts) == 2 {
				oldPath := strings.Trim(parts[0], "\"")
				keys[strings.Split(oldPath, "/")[0]] = true

				newPath := strings.Split(strings.Trim(parts[1], "\""), "/")[0]
				keys[newPath] = true
				continue
			}
		}

		// Group changes by the top-level directory or file name.
		parts := strings.Split(c.Path, "/")
		keys[parts[0]] = true
	}

	for key := range keys {
		inHEAD := hasInHEAD(dir, key)
		inIndex := hasInIndex(dir, key)

		if inHEAD && inIndex {
			updated[key] = true
		} else if inHEAD && !inIndex {
			removed[key] = true
		} else if !inHEAD && inIndex {
			added[key] = true
		}
	}

	var sections []string
	if msg := formatSection("Updated", updated); msg != "" {
		sections = append(sections, msg)
	}
	if msg := formatSection("Added", added); msg != "" {
		sections = append(sections, msg)
	}
	if msg := formatSection("Removed", removed); msg != "" {
		sections = append(sections, msg)
	}

	if len(sections) == 0 {
		return "Sync dotfiles"
	}

	return strings.Join(sections, ", ")
}

// formatSection creates a human-readable list for a specific action (e.g., "Updated a, b and c").
func formatSection(prefix string, items map[string]bool) string {
	if len(items) == 0 {
		return ""
	}
	var keys []string
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 1 {
		return prefix + " " + keys[0]
	}

	last := keys[len(keys)-1]
	rest := keys[:len(keys)-1]
	return prefix + " " + strings.Join(rest, ", ") + " and " + last
}
