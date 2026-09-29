package git

import (
	"os"
	"os/exec"
	"path"
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
	globalConfig := path.Join(t.TempDir(), "gitconfig")
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
	if err := os.WriteFile(path.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", name)
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
	isolateGit(t)
	root := t.TempDir()
	remote := path.Join(root, "remote.git")
	local := path.Join(root, "local")
	other := path.Join(root, "other")

	run(t, root, "init", "--bare", "-b", "main", remote)
	run(t, root, "clone", remote, local)
	commitFile(t, local, "file", "base\n")
	run(t, local, "push", "-u", "origin", "main")

	run(t, root, "clone", remote, other)
	commitFile(t, other, "file", "remote\n")
	run(t, other, "push")

	commitFile(t, local, "file", "local\n")

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
	if got := readFile(t, path.Join(local, "file")); got != "local\n" {
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
	if got := readFile(t, path.Join(local, "file")); got != "local\n" {
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
	if got := readFile(t, path.Join(local, "file")); got != "remote\n" {
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
