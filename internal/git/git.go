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

// Change represents a file modification, addition, deletion, or rename within the git repository.
type Change struct {
	Status string // M, A, D, R, etc.
	Path   string // Path relative to repo root
}

// Sync orchestrates the automatic synchronization process for the dotfiles directory.
// It pulls remote changes with rebase, stages local modifications, creates a contextual commit message,
// and pushes the resulting commit back to the remote repository.
func Sync(cfg config.Config) error {
	dotfilesDir := path.Join(xdg.Home, cfg.Dotfiles)

	if !isRepo(dotfilesDir) {
		return fmt.Errorf("directory %s is not a git repository", dotfilesDir)
	}

	// 1. Pull changes first using rebase and autostash.
	// This ensures we have the latest remote changes and helps avoid merge commits.
	if err := pull(dotfilesDir); err != nil {
		if isRebasing(dotfilesDir) {
			utils.PrintMessage("Conflicts detected during pull. Applying strategy:", cfg.SyncStrategy)
			if err := resolveRebase(dotfilesDir, cfg.SyncStrategy); err != nil {
				return fmt.Errorf("failed to resolve conflicts: %w. Please resolve manually", err)
			}
		} else {
			return fmt.Errorf("failed to pull changes: %w. Please resolve conflicts manually", err)
		}
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

	// Safety check: Ensure no conflict markers are about to be committed.
	for _, change := range changes {
		// We only check files that were modified, added, or renamed
		if strings.Contains(change.Status, "M") || strings.Contains(change.Status, "A") || strings.Contains(change.Status, "U") || strings.Contains(change.Status, "R") {
			filePath := change.Path
			// Handle rename format: "old -> new"
			if strings.Contains(filePath, " -> ") {
				parts := strings.Split(filePath, " -> ")
				filePath = strings.Trim(parts[1], "\"")
			}

			fullPath := path.Join(dotfilesDir, filePath)
			if utils.ContainsConflictMarkers(fullPath) {
				return fmt.Errorf("file %s contains conflict markers. Please resolve manually before syncing", filePath)
			}
		}
	}

	// 3. Generate a human-readable commit message based on the staged changes.
	message := generateCommitMessage(dotfilesDir, changes)
	utils.PrintMessage("Committing", message)
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

// isRepo determines whether the specified directory path resides within a valid git work tree.
func isRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// isRebasing checks if the repository is currently in the middle of a rebase operation.
func isRebasing(dir string) bool {
	rebaseApply := path.Join(dir, ".git", "rebase-apply")
	rebaseMerge := path.Join(dir, ".git", "rebase-merge")
	_, errApply := os.Stat(rebaseApply)
	_, errMerge := os.Stat(rebaseMerge)
	return !os.IsNotExist(errApply) || !os.IsNotExist(errMerge)
}

// getConflictedFiles returns a list of files that currently have merge conflicts.
func getConflictedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// resolveRebase attempts to resolve a rebase conflict based on the provided strategy.
func resolveRebase(dir string, strategy string) error {
	if strategy == "manual" {
		// Abort rebase to leave the repo in a clean state (as much as possible)
		exec.Command("git", "rebase", "--abort").Run()
		return fmt.Errorf("sync strategy set to 'manual'. Aborting rebase")
	}

	// Rebase might involve multiple commits, so we might need to resolve multiple times.
	for isRebasing(dir) {
		conflicts, err := getConflictedFiles(dir)
		if err != nil {
			return err
		}

		// During a rebase:
		// --ours is the branch we are rebasing onto (remote/upstream)
		// --theirs is the branch we are moving (local changes)
		checkoutFlag := "--theirs"
		if strategy == "remote" {
			checkoutFlag = "--ours"
		}

		for _, file := range conflicts {
			if file == "" {
				continue
			}
			utils.PrintMessage("Auto-resolving conflict in", file, "using", strategy, "version")
			checkoutCmd := exec.Command("git", "checkout", checkoutFlag, file)
			checkoutCmd.Dir = dir
			if err := checkoutCmd.Run(); err != nil {
				return fmt.Errorf("failed to checkout %s version of %s: %w", strategy, file, err)
			}

			addCmd := exec.Command("git", "add", file)
			addCmd.Dir = dir
			if err := addCmd.Run(); err != nil {
				return fmt.Errorf("failed to add resolved file %s: %w", file, err)
			}
		}

		// Continue the rebase. We set GIT_EDITOR=true to avoid opening an editor for commit messages.
		continueCmd := exec.Command("git", "rebase", "--continue")
		continueCmd.Dir = dir
		continueCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		// We don't check for error immediately here because rebase continue might fail
		// if there are more conflicts in the next commit, which we handle in the next iteration.
		_ = continueCmd.Run()

		if !isRebasing(dir) {
			break
		}
	}

	return nil
}

// getChanges retrieves and parses a list of repository file changes by executing 'git status --porcelain'.
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

// pull fetches and integrates remote changes into the local branch using rebase and autostash to avoid merge commits.
func pull(dir string) error {
	utils.PrintMessage("Pulling changes...")
	cmd := exec.Command("git", "pull", "--rebase", "--autostash")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stageAll stages all modifications, additions, and deletions in the working directory using 'git add -A'.
func stageAll(dir string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	return cmd.Run()
}

// commit creates a new git commit containing the currently staged changes, utilizing the provided message.
func commit(dir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	return cmd.Run()
}

// push uploads local repository commits to the configured upstream remote tracking branch.
func push(dir string) error {
	cmd := exec.Command("git", "push")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// hasInHEAD verifies if a specific file or directory (key) exists in the latest local commit (HEAD).
func hasInHEAD(dir, key string) bool {
	cmd := exec.Command("git", "ls-tree", "HEAD", "--", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// If HEAD does not exist yet (e.g. initial commit), git will report an unknown
		// revision. Treat that specific case as "not in HEAD" without noise.
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "unknown revision") && strings.Contains(stderr, "HEAD") {
				return false
			}
			// For other git failures, surface a diagnostic but still return false to match
			// the existing boolean-only API.
			fmt.Fprintf(os.Stderr, "git ls-tree HEAD -- %s failed: %s\n", key, strings.TrimSpace(stderr))
		} else {
			fmt.Fprintf(os.Stderr, "git ls-tree HEAD -- %s error: %v\n", key, err)
		}
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// hasInIndex verifies if a specific file or directory (key) is currently tracked in the git index (staging area).
func hasInIndex(dir, key string) bool {
	cmd := exec.Command("git", "ls-files", "--", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			fmt.Fprintf(os.Stderr, "git ls-files -- %s failed: %s\n", key, strings.TrimSpace(stderr))
		} else {
			fmt.Fprintf(os.Stderr, "git ls-files -- %s error: %v\n", key, err)
		}
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// generateCommitMessage constructs a human-readable commit message by analyzing the changes.
// It groups modifications by top-level directory or file to summarize additions, updates, and removals.
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

// formatSection converts a set of item names into a grammatically correct string list with the given prefix.
// For example: "Updated a, b and c".
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
