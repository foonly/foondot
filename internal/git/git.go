package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"foonly.dev/foondot/internal/utils"
)

// Change represents a file modification, addition, deletion, or rename within the git repository.
type Change struct {
	Status  string // M, A, D, R, etc.
	Path    string // Path relative to repo root
	OldPath string // Original path for renames and copies, relative to repo root
}

// Options control how Sync works.
type Options struct {
	// Strategy resolves conflicts when pulling: "manual", "local" or "remote".
	Strategy string
	// DryRun only lists the local changes that would be committed, without
	// pulling, staging, committing or pushing anything.
	DryRun bool
}

// Sync orchestrates the automatic synchronization process for the dotfiles directory.
// It pulls remote changes with rebase, stages local modifications, creates a contextual commit message,
// and pushes the resulting commit back to the remote repository. Without a remote
// or an upstream branch, it only commits locally.
func Sync(dotfilesDir string, opts Options) error {
	if err := checkRepo(dotfilesDir); err != nil {
		return err
	}

	remote, err := planRemote(dotfilesDir)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return showPending(dotfilesDir, remote)
	}

	// 1. Pull changes first using rebase and autostash.
	// This ensures we have the latest remote changes and helps avoid merge commits.
	if !remote.pull {
		utils.PrintValue("Skipping pull", remote.reason)
	} else if err := pull(dotfilesDir); err != nil {
		if isRebasing(dotfilesDir) {
			utils.PrintValue("Conflicts detected during pull. Applying strategy", opts.Strategy)
			if err := resolveRebase(dotfilesDir, opts.Strategy); err != nil {
				return fmt.Errorf("failed to resolve conflicts: %w. Please resolve manually", err)
			}
		} else {
			return fmt.Errorf("failed to pull changes: %w. Please resolve conflicts manually", err)
		}
	}

	utils.PrintValue("Checking for changes in", dotfilesDir)

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
	if err := checkConflictMarkers(dotfilesDir, changes); err != nil {
		return err
	}

	// 3. Generate a human-readable commit message based on the staged changes.
	message := generateCommitMessage(dotfilesDir, changes)
	utils.PrintValue("Committing", message)
	if err := commit(dotfilesDir, message); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// 4. Push the new commit to the remote repository.
	if !remote.push {
		utils.PrintValue("Skipping push", remote.reason)
	} else {
		utils.PrintMessage("Pushing changes...")
		if err := push(dotfilesDir); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}
	}

	utils.PrintMessage("Sync completed successfully")
	return nil
}

// remotePlan says whether sync pulls from and pushes to the remote.
type remotePlan struct {
	pull bool
	push bool
	// reason explains why pulling or pushing is skipped.
	reason string
}

// planRemote decides whether sync can pull and push:
//   - Without any remote, or if the current branch has no upstream branch,
//     sync only commits locally.
//   - If the upstream branch is configured but doesn't exist yet, e.g. after
//     cloning an empty repository, there is nothing to pull, but pushing creates it.
//
// An error is returned if HEAD is not on a branch, or if git fails.
func planRemote(dir string) (remotePlan, error) {
	cmd := exec.Command("git", "remote")
	cmd.Dir = dir
	out, err := gitOutput(cmd)
	if err != nil {
		return remotePlan{}, fmt.Errorf("failed to list git remotes: %w", err)
	}
	remotes := strings.Fields(string(out))
	if len(remotes) == 0 {
		return remotePlan{reason: "no git remote is configured"}, nil
	}

	cmd = exec.Command("git", "symbolic-ref", "--short", "-q", "HEAD")
	cmd.Dir = dir
	out, err = gitOutput(cmd)
	if err != nil {
		return remotePlan{}, fmt.Errorf("HEAD is not on a branch, check out a branch before syncing: %w", err)
	}
	branch := strings.TrimSpace(string(out))

	hasUpstream, err := hasGitConfig(dir, "branch."+branch+".remote")
	if err == nil && hasUpstream {
		hasUpstream, err = hasGitConfig(dir, "branch."+branch+".merge")
	}
	if err != nil {
		return remotePlan{}, fmt.Errorf("failed to read the upstream branch: %w", err)
	}
	if !hasUpstream {
		remote := "<remote>"
		if len(remotes) == 1 {
			remote = remotes[0]
		}
		return remotePlan{
			reason: fmt.Sprintf("branch %s has no upstream branch, set one with `git push -u %s %s`", branch, remote, branch),
		}, nil
	}

	// The remote-tracking branch only exists once the upstream branch has been fetched or pushed.
	cmd = exec.Command("git", "rev-parse", "--verify", "-q", "@{upstream}")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return remotePlan{push: true, reason: "the upstream branch doesn't exist yet"}, nil
	}
	return remotePlan{pull: true, push: true}, nil
}

// hasGitConfig reports whether a git config key is set.
func hasGitConfig(dir, key string) (bool, error) {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = dir
	_, err := gitOutput(cmd)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		// Exit code 1 means the key isn't set.
		return false, nil
	}
	return err == nil, err
}

// showPending lists the local changes that the next sync would commit,
// without pulling, staging or committing anything. It returns an error if any
// of them contain conflict markers, since sync would refuse to commit them.
func showPending(dir string, remote remotePlan) error {
	changes, err := getChanges(dir, "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("failed to get git status: %w", err)
	}
	if len(changes) == 0 {
		utils.PrintMessage("No local changes to sync.")
		return nil
	}

	if remote.push {
		utils.PrintMessage("Sync would commit and push these local changes (remote changes are not fetched):")
	} else {
		utils.PrintMessage("Sync would commit these local changes, without pushing because " + remote.reason + ":")
	}
	for _, change := range changes {
		switch {
		case deletedInWorktree(change.Status) && strings.HasPrefix(change.Status, "A"):
			// Staged, then deleted again: staging everything leaves nothing to commit.
			continue
		case deletedInWorktree(change.Status) && change.OldPath != "":
			utils.PrintValue("  Deleted", change.OldPath)
		case change.OldPath != "":
			utils.PrintChange("  Renamed", change.OldPath, change.Path)
		default:
			utils.PrintValue("  "+describeStatus(change.Status), change.Path)
		}
	}

	return checkConflictMarkers(dir, changes)
}

// describeStatus turns a porcelain status code into a word for listing changes,
// describing what staging everything would do.
func describeStatus(status string) string {
	switch {
	case strings.Contains(status, "D"):
		return "Deleted"
	case strings.Contains(status, "?"), strings.HasPrefix(status, "A"), strings.HasPrefix(status, "C"):
		return "Added"
	default:
		return "Modified"
	}
}

// deletedInWorktree reports whether a porcelain status code (with spaces
// trimmed) says the file no longer exists in the working tree.
func deletedInWorktree(status string) bool {
	return strings.HasSuffix(status, "D")
}

// runGit runs a git command and includes git's output in the returned error,
// so failures don't just report "exit status 1".
func runGit(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	return gitError(err, out)
}

// gitOutput runs a git command and returns its standard output. Git's error
// output is included in the returned error.
func gitOutput(cmd *exec.Cmd) ([]byte, error) {
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, gitError(err, exitErr.Stderr)
	}
	return out, err
}

// gitError adds git's output to a failed command's error.
func gitError(err error, output []byte) error {
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(string(output)); msg != "" {
		return fmt.Errorf("%w: %s", err, msg)
	}
	return err
}

// checkConflictMarkers returns an error if any modified, added or renamed file
// contains git conflict markers, or can't be checked.
func checkConflictMarkers(dir string, changes []Change) error {
	for _, change := range changes {
		// "?" marks untracked files, which only occur in a dry run, before staging.
		if !strings.ContainsAny(change.Status, "MAUR?") || deletedInWorktree(change.Status) {
			continue
		}
		found, err := utils.ContainsConflictMarkers(filepath.Join(dir, change.Path))
		if err != nil {
			return fmt.Errorf("could not check %s for conflict markers: %w", change.Path, err)
		}
		if found {
			return fmt.Errorf("file %s contains conflict markers. Please resolve manually before syncing", change.Path)
		}
	}
	return nil
}

// checkRepo returns an error unless the directory is the top level of a git work tree.
// A subdirectory isn't enough, since sync stages and commits the whole work tree.
func checkRepo(dir string) error {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := gitOutput(cmd)
	if err != nil {
		return fmt.Errorf("directory %s is not a git repository: %w", dir, err)
	}
	topLevel := strings.TrimSpace(string(out))
	// Compare resolved paths, git reports the top level with symlinks resolved.
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	resolvedTop, err := filepath.EvalSymlinks(topLevel)
	if err != nil {
		return err
	}
	if resolvedDir != resolvedTop {
		return fmt.Errorf("directory %s is inside the git repository %s, but must be the top level of its own repository", dir, topLevel)
	}
	return nil
}

// isRebasing checks if the repository is currently in the middle of a rebase operation.
// Git is asked for the location of its rebase state, since .git may be a file,
// e.g. in worktrees and submodules.
func isRebasing(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-path", "rebase-merge", "--git-path", "rebase-apply")
	cmd.Dir = dir
	out, err := gitOutput(cmd)
	if err != nil {
		return false
	}
	for statePath := range strings.Lines(string(out)) {
		statePath = strings.TrimSpace(statePath)
		if !filepath.IsAbs(statePath) {
			statePath = filepath.Join(dir, statePath)
		}
		if _, err := os.Stat(statePath); err == nil {
			return true
		}
	}
	return false
}

// getConflictedFiles returns a list of files that currently have merge conflicts.
func getConflictedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "-z", "--diff-filter=U")
	cmd.Dir = dir
	out, err := gitOutput(cmd)
	if err != nil {
		return nil, err
	}
	// With -z, paths are NUL terminated and not quoted.
	var files []string
	for file := range strings.SplitSeq(string(out), "\x00") {
		if file != "" {
			files = append(files, file)
		}
	}
	return files, nil
}

// resolveRebase attempts to resolve a rebase conflict based on the provided strategy.
func resolveRebase(dir string, strategy string) error {
	if strategy == "manual" {
		// Abort rebase to leave the repo in a clean state (as much as possible)
		abortCmd := exec.Command("git", "rebase", "--abort")
		abortCmd.Dir = dir
		if err := runGit(abortCmd); err != nil {
			return fmt.Errorf("sync strategy set to 'manual', but aborting the rebase failed: %w", err)
		}
		return fmt.Errorf("sync strategy set to 'manual'. Aborting rebase")
	}

	// Rebase might involve multiple commits, so we might need to resolve multiple times.
	var continueErr error
	for isRebasing(dir) {
		conflicts, err := getConflictedFiles(dir)
		if err != nil {
			return err
		}
		// Every iteration must resolve at least one conflict, otherwise the rebase
		// stopped for another reason and retrying would loop forever.
		if len(conflicts) == 0 {
			if continueErr != nil {
				return fmt.Errorf("rebase stopped without conflicts: %w", continueErr)
			}
			return fmt.Errorf("rebase stopped without conflicts")
		}

		// During a rebase:
		// --ours is the branch we are rebasing onto (remote/upstream)
		// --theirs is the branch we are moving (local changes)
		checkoutFlag := "--theirs"
		if strategy == "remote" {
			checkoutFlag = "--ours"
		}

		for _, file := range conflicts {
			utils.PrintValue("Auto-resolving conflict using "+strategy+" version", file)
			checkoutCmd := exec.Command("git", "checkout", checkoutFlag, "--", file)
			checkoutCmd.Dir = dir
			if err := runGit(checkoutCmd); err != nil {
				return fmt.Errorf("failed to checkout %s version of %s: %w", strategy, file, err)
			}

			addCmd := exec.Command("git", "add", "--", file)
			addCmd.Dir = dir
			if err := runGit(addCmd); err != nil {
				return fmt.Errorf("failed to add resolved file %s: %w", file, err)
			}
		}

		// Continue the rebase. We set GIT_EDITOR=true to avoid opening an editor for commit messages.
		continueCmd := exec.Command("git", "rebase", "--continue")
		continueCmd.Dir = dir
		continueCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		// Rebase continue fails if there are more conflicts in the next commit,
		// which we handle in the next iteration.
		continueErr = runGit(continueCmd)
	}

	return nil
}

// getChanges retrieves and parses a list of repository file changes by executing
// 'git status --porcelain -z', with any extra arguments.
func getChanges(dir string, args ...string) ([]Change, error) {
	cmd := exec.Command("git", append([]string{"status", "--porcelain", "-z"}, args...)...)
	cmd.Dir = dir
	output, err := gitOutput(cmd)
	if err != nil {
		return nil, err
	}

	// With -z, entries are NUL terminated and paths are not quoted.
	// Format: "XY PATH", renames and copies are followed by an extra "ORIG_PATH" entry.
	entries := strings.Split(string(output), "\x00")
	var changes []Change

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		change := Change{
			Status: strings.TrimSpace(entry[:2]),
			Path:   entry[3:],
		}
		if (entry[0] == 'R' || entry[0] == 'C') && i+1 < len(entries) {
			i++
			change.OldPath = entries[i]
		}
		changes = append(changes, change)
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
	return runGit(cmd)
}

// commit creates a new git commit containing the currently staged changes, utilizing the provided message.
func commit(dir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	return runGit(cmd)
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
		// Surface a diagnostic but still return false to match the boolean-only API.
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
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
	// Before the first commit there is no HEAD, and everything is added.
	headCmd := exec.Command("git", "rev-parse", "--verify", "-q", "HEAD")
	headCmd.Dir = dir
	headExists := headCmd.Run() == nil

	added := make(map[string]bool)
	updated := make(map[string]bool)
	removed := make(map[string]bool)

	keys := make(map[string]bool)

	for _, c := range changes {
		// Group changes by the top-level directory or file name.
		// Renames count for both the old and the new location.
		keys[strings.Split(c.Path, "/")[0]] = true
		if c.OldPath != "" {
			keys[strings.Split(c.OldPath, "/")[0]] = true
		}
	}

	for key := range keys {
		inHEAD := headExists && hasInHEAD(dir, key)
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
