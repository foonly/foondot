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
	remote := path.Join(root, "remote.git")
	local := path.Join(root, "local")
	other := path.Join(root, "other")

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
			if got := readFile(t, path.Join(local, name)); got != "local\n" {
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
		if err := os.WriteFile(path.Join(dir, name), []byte(content), 0644); err != nil {
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
	if err := os.WriteFile(path.Join(dir, name), []byte(conflict), 0644); err != nil {
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
