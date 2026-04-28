package git

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRepoHasRemote_NoRemote pins the regression for #7: a fresh git
// repo with no configured remote must report repoHasRemote == false so
// Sync's pull/push guards skip those round-trips and still allow the
// local commit to land.
func TestRepoHasRemote_NoRemote(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if got := repoHasRemote(dir); got {
		t.Fatalf("repoHasRemote on remote-less repo = true, want false")
	}
}

// TestRepoHasRemote_WithRemote sanity: once a remote is configured the
// helper reports true. Uses the repo's own .git directory as a dummy
// remote URL so the test has no network dependency.
func TestRepoHasRemote_WithRemote(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "remote", "add", "origin",
		filepath.Join(dir, ".git")).Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}
	if got := repoHasRemote(dir); !got {
		t.Fatalf("repoHasRemote on repo with `origin` = false, want true")
	}
}
