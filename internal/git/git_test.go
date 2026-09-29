package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolateGit makes git ignore the user's configuration and gives it an identity.
func isolateGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(globalConfig, nil, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	t.Setenv("GIT_SEQUENCE_EDITOR", "true")
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "--", name)
	run(t, dir, "commit", "-m", "change "+name)
}

func readFile(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// conflictingClone returns a clone whose pull --rebase stops on a conflict in
// "file": the clone has committed "local" and the remote has "remote".
func conflictingClone(t *testing.T) string {
	t.Helper()
	return conflictingCloneFile(t, "file")
}

// conflictingCloneFile is like conflictingClone, with the conflict in the named file.
func conflictingCloneFile(t *testing.T, name string) string {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	other := filepath.Join(root, "other")

	run(t, root, "init", "--bare", "-b", "main", remote)
	run(t, root, "clone", remote, local)
	commitFile(t, local, name, "base\n")
	run(t, local, "push", "-u", "origin", "main")

	run(t, root, "clone", remote, other)
	commitFile(t, other, name, "remote\n")
	run(t, other, "push")

	commitFile(t, local, name, "local\n")

	if err := pull(local); err == nil {
		t.Fatal("expected pull to fail with a conflict")
	}
	if !isRebasing(local) {
		t.Fatal("expected repository to be rebasing after conflicting pull")
	}
	return local
}

func TestResolveRebaseManualAbortsInRepo(t *testing.T) {
	local := conflictingClone(t)
	// Run from an unrelated directory to make sure the abort targets the repo.
	t.Chdir(t.TempDir())

	err := resolveRebase(local, "manual")

	if err == nil {
		t.Error("expected manual strategy to return an error")
	}
	if isRebasing(local) {
		t.Error("rebase was not aborted in the dotfiles repository")
	}
	if got := readFile(t, filepath.Join(local, "file")); got != "local\n" {
		t.Errorf("file = %q, want local content restored", got)
	}
}

func TestResolveRebaseLocal(t *testing.T) {
	local := conflictingClone(t)

	if err := resolveRebase(local, "local"); err != nil {
		t.Fatal(err)
	}

	if isRebasing(local) {
		t.Error("still rebasing")
	}
	if got := readFile(t, filepath.Join(local, "file")); got != "local\n" {
		t.Errorf("file = %q, want %q", got, "local\n")
	}
}

func TestResolveRebaseRemote(t *testing.T) {
	local := conflictingClone(t)

	if err := resolveRebase(local, "remote"); err != nil {
		t.Fatal(err)
	}

	if isRebasing(local) {
		t.Error("still rebasing")
	}
	if got := readFile(t, filepath.Join(local, "file")); got != "remote\n" {
		t.Errorf("file = %q, want %q", got, "remote\n")
	}
}

func TestResolveRebaseContinueFails(t *testing.T) {
	local := conflictingClone(t)
	// Without a git identity, every `rebase --continue` fails to commit while
	// leaving no conflicted files behind.
	run(t, local, "config", "user.useConfigOnly", "true")
	for _, env := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		os.Unsetenv(env)
	}

	done := make(chan error, 1)
	go func() { done <- resolveRebase(local, "local") }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error when rebase --continue keeps failing")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("resolveRebase did not return")
	}
}

func TestResolveRebaseUnusualFilenames(t *testing.T) {
	for _, name := range []string{"--ours", "-f", "ä file"} {
		t.Run(name, func(t *testing.T) {
			local := conflictingCloneFile(t, name)

			if err := resolveRebase(local, "local"); err != nil {
				t.Fatal(err)
			}

			if isRebasing(local) {
				t.Error("still rebasing")
			}
			if got := readFile(t, filepath.Join(local, name)); got != "local\n" {
				t.Errorf("file = %q, want %q", got, "local\n")
			}
		})
	}
}

func TestGetChanges(t *testing.T) {
	isolateGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "-b", "main")
	commitFile(t, dir, "modified", "1\n")
	commitFile(t, dir, "old name", "same\n")
	commitFile(t, dir, "deleted", "1\n")

	for name, content := range map[string]string{"modified": "2\n", "ä added": "new\n", " leading": "x\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, dir, "mv", "old name", "new name")
	run(t, dir, "rm", "-q", "deleted")
	run(t, dir, "add", "-A")

	changes, err := getChanges(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]Change{
		" leading": {Status: "A", Path: " leading"},
		"deleted":  {Status: "D", Path: "deleted"},
		"modified": {Status: "M", Path: "modified"},
		"new name": {Status: "R", Path: "new name", OldPath: "old name"},
		"ä added":  {Status: "A", Path: "ä added"},
	}
	if len(changes) != len(want) {
		t.Fatalf("got %d changes %+v, want %d", len(changes), changes, len(want))
	}
	for _, c := range changes {
		if w, ok := want[c.Path]; !ok || c != w {
			t.Errorf("unexpected change %+v", c)
		}
	}
}

func TestCheckConflictMarkers(t *testing.T) {
	dir := t.TempDir()
	name := "ä file"
	conflict := "<<<<<<< HEAD\na\n=======\nb\n>>>>>>> 1234abc\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(conflict), 0644); err != nil {
		t.Fatal(err)
	}

	if err := checkConflictMarkers(dir, []Change{{Status: "A", Path: name}}); err == nil {
		t.Error("expected conflict markers to be detected")
	}
	if err := checkConflictMarkers(dir, []Change{{Status: "M", Path: "missing"}}); err == nil {
		t.Error("expected an error for a file that can't be checked")
	}
	if err := checkConflictMarkers(dir, []Change{{Status: "D", Path: "missing"}}); err != nil {
		t.Errorf("deleted files should not be checked: %v", err)
	}
}

func TestCommitErrorIncludesGitMessage(t *testing.T) {
	isolateGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stageAll(dir); err != nil {
		t.Fatal(err)
	}
	// Without a git identity the commit fails.
	run(t, dir, "config", "user.useConfigOnly", "true")
	for _, env := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		os.Unsetenv(env)
	}

	err := commit(dir, "message")

	if err == nil {
		t.Fatal("expected commit to fail")
	}
	if !strings.Contains(err.Error(), "no email was given") {
		t.Errorf("error %q doesn't include git's message", err)
	}
}

func TestGitOutputErrorIncludesStderr(t *testing.T) {
	isolateGit(t)
	// Not a repository, so git status fails.
	_, err := getChanges(t.TempDir())

	if err == nil {
		t.Fatal("expected an error outside a repository")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q doesn't include git's message", err)
	}
}

func TestCheckRepo(t *testing.T) {
	isolateGit(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	run(t, root, "init", "-q", "-b", "main", repo)
	sub := filepath.Join(repo, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	notRepo := filepath.Join(root, "plain")
	if err := os.Mkdir(notRepo, 0755); err != nil {
		t.Fatal(err)
	}

	if err := checkRepo(repo); err != nil {
		t.Errorf("top level: unexpected error %v", err)
	}
	if err := checkRepo(link); err != nil {
		t.Errorf("symlink to top level: unexpected error %v", err)
	}
	if err := checkRepo(sub); err == nil {
		t.Error("subdirectory: expected an error")
	}
	if err := checkRepo(notRepo); err == nil {
		t.Error("not a repository: expected an error")
	}
}

func TestIsRebasingWorktree(t *testing.T) {
	isolateGit(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	run(t, root, "init", "-q", "-b", "main", repo)
	commitFile(t, repo, "a", "1\n")
	commitFile(t, repo, "b", "2\n")
	worktree := filepath.Join(root, "worktree")
	run(t, repo, "worktree", "add", "-q", "-b", "other", worktree)

	if isRebasing(worktree) {
		t.Fatal("worktree reported as rebasing before starting a rebase")
	}

	// In a worktree, .git is a file, and the rebase state lives in the main repository.
	cmd := exec.Command("git", "rebase", "--exec", "false", "HEAD~1")
	cmd.Dir = worktree
	if err := cmd.Run(); err == nil {
		t.Fatal("expected rebase to stop")
	}

	if !isRebasing(worktree) {
		t.Error("rebase in worktree not detected")
	}
	if isRebasing(repo) {
		t.Error("main repository reported as rebasing")
	}
	run(t, worktree, "rebase", "--abort")
}

// syncedClone returns a clone with one pushed commit, and its bare remote.
func syncedClone(t *testing.T) (local string, remote string) {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	local = filepath.Join(root, "dotfiles")
	run(t, root, "init", "-q", "--bare", "-b", "main", remote)
	run(t, root, "clone", "-q", remote, local)
	if err := os.MkdirAll(filepath.Join(local, "sway"), 0755); err != nil {
		t.Fatal(err)
	}
	commitFile(t, local, "sway/config", "1\n")
	run(t, local, "push", "-q", "-u", "origin", "main")
	return local, remote
}

func writeTestFile(t *testing.T, file, content string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncPushesChanges(t *testing.T) {
	local, remote := syncedClone(t)
	writeTestFile(t, filepath.Join(local, "sway", "config"), "2\n")
	writeTestFile(t, filepath.Join(local, "bashrc"), "new\n")

	if err := Sync(local, Options{Strategy: "manual"}); err != nil {
		t.Fatal(err)
	}

	if got := run(t, remote, "log", "-1", "--format=%s"); got != "Updated sway, Added bashrc\n" {
		t.Errorf("pushed commit message = %q", got)
	}
	if got := run(t, local, "status", "--porcelain"); got != "" {
		t.Errorf("working tree not clean after sync:\n%s", got)
	}
}

func TestSyncWithoutChanges(t *testing.T) {
	local, remote := syncedClone(t)
	before := run(t, remote, "rev-parse", "HEAD")

	if err := Sync(local, Options{Strategy: "manual"}); err != nil {
		t.Fatal(err)
	}

	if after := run(t, remote, "rev-parse", "HEAD"); after != before {
		t.Error("sync without changes created a commit")
	}
}

func TestSyncPullsRemoteChanges(t *testing.T) {
	local, remote := syncedClone(t)
	other := filepath.Join(t.TempDir(), "other")
	run(t, filepath.Dir(other), "clone", "-q", remote, other)
	commitFile(t, other, "remote-file", "x\n")
	run(t, other, "push", "-q")

	if err := Sync(local, Options{Strategy: "manual"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(local, "remote-file")); err != nil {
		t.Errorf("remote change not pulled: %v", err)
	}
}

func TestSyncRefusesConflictMarkers(t *testing.T) {
	local, remote := syncedClone(t)
	before := run(t, remote, "rev-parse", "HEAD")
	writeTestFile(t, filepath.Join(local, "sway", "config"), "<<<<<<< HEAD\na\n=======\nb\n>>>>>>> 1234abc\n")

	err := Sync(local, Options{Strategy: "manual"})

	if err == nil || !strings.Contains(err.Error(), "conflict markers") {
		t.Errorf("got %v, want a conflict marker error", err)
	}
	if after := run(t, remote, "rev-parse", "HEAD"); after != before {
		t.Error("file with conflict markers was pushed")
	}
}

func TestSyncNotARepository(t *testing.T) {
	isolateGit(t)
	if err := Sync(t.TempDir(), Options{Strategy: "manual"}); err == nil {
		t.Error("expected an error outside a repository")
	}
}

func TestSyncDryRun(t *testing.T) {
	local, remote := syncedClone(t)
	commitFile(t, local, "old name", "same\n")
	run(t, local, "push", "-q")
	remoteBefore := run(t, remote, "rev-parse", "HEAD")
	localBefore := run(t, local, "rev-parse", "HEAD")
	writeTestFile(t, filepath.Join(local, "sway", "config"), "2\n")
	writeTestFile(t, filepath.Join(local, "secret"), "token\n")
	run(t, local, "mv", "old name", "new name")
	statusBefore := run(t, local, "status", "--porcelain")

	if err := Sync(local, Options{Strategy: "manual", DryRun: true}); err != nil {
		t.Fatal(err)
	}

	if got := run(t, local, "status", "--porcelain"); got != statusBefore {
		t.Errorf("dry run changed the working tree or index:\n%s\nwant:\n%s", got, statusBefore)
	}
	if run(t, local, "rev-parse", "HEAD") != localBefore || run(t, remote, "rev-parse", "HEAD") != remoteBefore {
		t.Error("dry run created or pushed a commit")
	}
}

func TestSyncDryRunReportsConflictMarkers(t *testing.T) {
	local, _ := syncedClone(t)
	// An untracked file, which a real sync would stage and then refuse.
	writeTestFile(t, filepath.Join(local, "new"), "<<<<<<< HEAD\na\n=======\nb\n>>>>>>> 1234abc\n")

	err := Sync(local, Options{Strategy: "manual", DryRun: true})

	if err == nil || !strings.Contains(err.Error(), "conflict markers") {
		t.Errorf("got %v, want a conflict marker error", err)
	}
}

func TestDescribeStatus(t *testing.T) {
	for status, want := range map[string]string{
		"??": "Added", "A": "Added", "AM": "Added",
		"M": "Modified", "MM": "Modified",
		"D": "Deleted", "AD": "Deleted", "MD": "Deleted",
	} {
		if got := describeStatus(status); got != want {
			t.Errorf("%q: got %s, want %s", status, got, want)
		}
	}
}

func TestSyncDryRunStagedThenDeleted(t *testing.T) {
	local, _ := syncedClone(t)
	// Staged with conflict markers, then deleted: there is nothing left to check.
	file := filepath.Join(local, "gone")
	writeTestFile(t, file, "<<<<<<< HEAD\na\n=======\nb\n>>>>>>> 1234abc\n")
	run(t, local, "add", "gone")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	run(t, local, "rm", "-q", "--cached", "sway/config")

	if err := Sync(local, Options{Strategy: "manual", DryRun: true}); err != nil {
		t.Errorf("unexpected error %v", err)
	}
}
