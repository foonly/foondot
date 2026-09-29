package dots

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/utils"
)

// setupHome creates a temporary home directory containing an empty
// "dotfiles" folder, and a linker for it running on host "thishost".
func setupHome(t *testing.T) (l *linker, home string, dotfilesDir string) {
	t.Helper()
	home = t.TempDir()
	dotfilesDir = filepath.Join(home, "dotfiles")
	mkdir(t, dotfilesDir)
	l = &linker{
		home:        home,
		dotfilesDir: dotfilesDir,
		hostname:    "thishost",
		backupDir:   filepath.Join(home, ".local", "share", "foondot", "backup"),
		tracked:     []string{},
	}
	return l, home, dotfilesDir
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, file string) {
	t.Helper()
	writeContent(t, file, "content")
}

func writeContent(t *testing.T, file, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(file))
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readContent(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func symlink(t *testing.T, source, target string) {
	t.Helper()
	if err := os.Symlink(source, target); err != nil {
		t.Fatal(err)
	}
}

func TestFilterDotsHostname(t *testing.T) {
	l, _, _ := setupHome(t)
	dots := []config.Item{
		{Source: "all", Target: ".all"},
		{Source: "mine", Target: ".mine", Hostname: []string{"thishost", "other"}},
		{Source: "theirs", Target: ".theirs", Hostname: []string{"other"}},
	}

	got, complete := l.filterDots(dots)

	if !complete {
		t.Error("expected complete result")
	}
	var sources []string
	for _, d := range got {
		sources = append(sources, d.Source)
	}
	if !slices.Equal(sources, []string{"all", "mine"}) {
		t.Errorf("got sources %v, want [all mine]", sources)
	}
}

func TestFilterDotsWildcard(t *testing.T) {
	l, _, dotfilesDir := setupHome(t)
	writeFile(t, filepath.Join(dotfilesDir, "bin", "a"))
	writeFile(t, filepath.Join(dotfilesDir, "bin", "b"))

	got, complete := l.filterDots([]config.Item{{Source: "bin/*", Target: ".local/bin"}})

	if !complete {
		t.Error("expected complete result")
	}
	want := []config.Item{
		{Source: "bin/a", Target: ".local/bin/a"},
		{Source: "bin/b", Target: ".local/bin/b"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Source != want[i].Source || got[i].Target != want[i].Target {
			t.Errorf("item %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestFilterDotsWildcardUnreadable(t *testing.T) {
	l, _, _ := setupHome(t)

	got, complete := l.filterDots([]config.Item{
		{Source: "missing/*", Target: ".local/bin"},
		{Source: "bashrc", Target: ".bashrc"},
	})

	if complete {
		t.Error("expected incomplete result when a wildcard source can't be read")
	}
	if len(got) != 1 || got[0].Source != "bashrc" {
		t.Errorf("got %v, want only bashrc", got)
	}
}

func TestCleanTargets(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	writeFile(t, filepath.Join(dotfilesDir, "keep"))
	writeFile(t, filepath.Join(dotfilesDir, "stale"))
	writeFile(t, filepath.Join(home, "elsewhere"))

	keep := filepath.Join(home, ".keep")
	stale := filepath.Join(home, ".stale")
	foreign := filepath.Join(home, ".foreign")
	notLink := filepath.Join(home, ".notlink")
	missing := filepath.Join(home, ".missing")

	symlink(t, filepath.Join(dotfilesDir, "keep"), keep)
	symlink(t, filepath.Join(dotfilesDir, "stale"), stale)
	// A tracked link the user replaced with their own symlink.
	symlink(t, filepath.Join(home, "elsewhere"), foreign)
	writeFile(t, notLink)

	l.tracked = []string{keep, stale, foreign, notLink, missing}

	l.cleanTargets([]config.Item{{Source: "keep", Target: ".keep"}})

	if !slices.Equal(l.tracked, []string{keep}) {
		t.Errorf("tracked targets = %v, want [%s]", l.tracked, keep)
	}
	if utils.GetType(keep) != utils.IsSymlink {
		t.Error("current link was removed")
	}
	if utils.GetType(stale) != utils.NotExists {
		t.Error("stale link was not removed")
	}
	if utils.GetType(foreign) != utils.IsSymlink {
		t.Error("symlink not pointing into dotfiles was removed")
	}
	if utils.GetType(notLink) != utils.IsFile {
		t.Error("regular file was touched")
	}
}

func TestPointsInto(t *testing.T) {
	_, home, dotfilesDir := setupHome(t)

	relative := filepath.Join(home, ".relative")
	symlink(t, "dotfiles/x", relative)
	sibling := filepath.Join(home, ".sibling")
	// Shares the "dotfiles" prefix but is a different directory.
	symlink(t, filepath.Join(home, "dotfiles-old", "x"), sibling)
	escape := filepath.Join(home, ".escape")
	symlink(t, filepath.Join(dotfilesDir, "..", "x"), escape)

	if !pointsInto(relative, dotfilesDir) {
		t.Error("relative link into dotfiles not recognized")
	}
	if pointsInto(sibling, dotfilesDir) {
		t.Error("link into sibling directory treated as inside dotfiles")
	}
	if pointsInto(escape, dotfilesDir) {
		t.Error("link escaping via .. treated as inside dotfiles")
	}
	if pointsInto(filepath.Join(home, ".nothing"), dotfilesDir) {
		t.Error("nonexistent path treated as a link into dotfiles")
	}
}

func TestTargetPath(t *testing.T) {
	l, home, _ := setupHome(t)

	if got, want := l.targetPath(".bashrc"), filepath.Join(home, ".bashrc"); got != want {
		t.Errorf("relative target: got %s, want %s", got, want)
	}
	if got, want := l.targetPath("/etc/foo/"), "/etc/foo"; got != want {
		t.Errorf("absolute target: got %s, want %s", got, want)
	}
}

func TestLinkAbsoluteTarget(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	writeFile(t, filepath.Join(dotfilesDir, "conf"))
	// An absolute target outside the home directory.
	outside := filepath.Join(t.TempDir(), "etc", "conf")

	if _, err := l.handleDot(config.Item{Source: "conf", Target: outside}); err != nil {
		t.Fatal(err)
	}

	if utils.GetType(outside) != utils.IsSymlink {
		t.Errorf("expected symlink at %s", outside)
	}
	if utils.GetType(filepath.Join(home, outside)) != utils.NotExists {
		t.Error("absolute target was created under the home directory")
	}

	// The absolute target must count as current, so cleanup keeps it.
	l.cleanTargets([]config.Item{{Source: "conf", Target: outside}})
	if !slices.Equal(l.tracked, []string{outside}) || utils.GetType(outside) != utils.IsSymlink {
		t.Error("cleanup removed a current absolute target")
	}
}

func TestHandleDotMovesTargetToMissingSource(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	target := filepath.Join(home, ".config", "app")
	writeContent(t, target, "existing")

	if linked, err := l.handleDot(config.Item{Source: "app/config", Target: ".config/app"}); !linked || err != nil {
		t.Fatalf("got (%v, %v), want link to be created", linked, err)
	}

	source := filepath.Join(dotfilesDir, "app", "config")
	if got := readContent(t, source); got != "existing" {
		t.Errorf("source = %q, want the former target content", got)
	}
	if utils.GetType(target) != utils.IsSymlink {
		t.Error("target is not a symlink")
	}
}

func TestHandleDotSkipsExistingWithoutForce(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	writeContent(t, filepath.Join(dotfilesDir, "bashrc"), "source")
	target := filepath.Join(home, ".bashrc")
	writeContent(t, target, "target")

	if linked, err := l.handleDot(config.Item{Source: "bashrc", Target: ".bashrc"}); linked || err == nil {
		t.Errorf("got (%v, %v), want an error without force", linked, err)
	}
	if got := readContent(t, target); got != "target" {
		t.Errorf("target = %q, want it untouched", got)
	}
}

func TestHandleDotForceBacksUpOutsideDotfiles(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	writeContent(t, filepath.Join(dotfilesDir, "bashrc"), "source")
	target := filepath.Join(home, ".bashrc")
	backup := filepath.Join(l.backupDir, target)
	item := config.Item{Source: "bashrc", Target: ".bashrc"}
	l.force = true

	for i, want := range []string{backup, backup + ".1"} {
		writeContent(t, target+".tmp", "target "+strconv.Itoa(i))
		os.Remove(target)
		if err := os.Rename(target+".tmp", target); err != nil {
			t.Fatal(err)
		}

		if linked, err := l.handleDot(item); !linked || err != nil {
			t.Fatalf("run %d: got (%v, %v), want link to be created", i, linked, err)
		}
		if got := readContent(t, want); got != "target "+strconv.Itoa(i) {
			t.Errorf("run %d: backup %s = %q", i, want, got)
		}
	}

	entries, err := os.ReadDir(dotfilesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dotfiles folder contains %d entries, want only the source", len(entries))
	}
}

func TestPrepareTargetSourceReportsErrors(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	writeFile(t, filepath.Join(dotfilesDir, "conf"))
	// The target's parent is a file, so its directory can't be created.
	writeFile(t, filepath.Join(home, "blocker"))
	target := filepath.Join(home, "blocker", "conf")

	if err := l.prepareTargetSource(target, filepath.Join(dotfilesDir, "conf")); err == nil {
		t.Error("expected an error when the target directory can't be created")
	}
	if linked, err := l.handleDot(config.Item{Source: "conf", Target: "blocker/conf"}); linked || err == nil {
		t.Errorf("got (%v, %v), want an error", linked, err)
	}
}

func TestHandleDotExistingLinks(t *testing.T) {
	l, home, dotfilesDir := setupHome(t)
	source := filepath.Join(dotfilesDir, "bashrc")
	writeFile(t, source)
	item := config.Item{Source: "bashrc", Target: ".bashrc"}
	target := filepath.Join(home, ".bashrc")

	// Already linked, e.g. by an earlier run whose tracked links were lost.
	symlink(t, source, target)
	linked, err := l.handleDot(item)
	if linked || err != nil {
		t.Errorf("already linked: got (%v, %v), want (false, nil)", linked, err)
	}
	if !slices.Equal(l.tracked, []string{target}) {
		t.Errorf("already linked: tracked %v, want the existing link", l.tracked)
	}

	// Linked somewhere else.
	os.Remove(target)
	symlink(t, filepath.Join(home, "elsewhere"), target)
	if _, err := l.handleDot(item); err == nil {
		t.Error("link to another location: expected an error without force")
	}
	l.force = true
	if linked, err := l.handleDot(item); !linked || err != nil {
		t.Errorf("link to another location with force: got (%v, %v), want relinked", linked, err)
	}
	if !linksTo(target, source) {
		t.Error("link was not replaced")
	}
}

func TestHandleDotMissingSource(t *testing.T) {
	l, _, _ := setupHome(t)

	if _, err := l.handleDot(config.Item{Source: "missing", Target: ".missing"}); err == nil {
		t.Error("expected an error for a missing source")
	}
}

func TestLinkReturnsFailures(t *testing.T) {
	_, home, dotfilesDir := setupHome(t)
	writeFile(t, filepath.Join(dotfilesDir, "ok"))
	opts := Options{Home: home, Hostname: "thishost", DataDir: filepath.Join(t.TempDir(), "foondot")}
	cfg := config.Config{Dotfiles: "dotfiles", Dots: []config.Item{
		{Source: "ok", Target: ".ok"},
		{Source: "missing", Target: ".missing"},
	}}

	if err := Link(cfg, opts); err == nil {
		t.Error("expected an error when a dotfile can't be linked")
	}
	if utils.GetType(filepath.Join(home, ".ok")) != utils.IsSymlink {
		t.Error("working dotfile was not linked")
	}

	cfg.Dots = cfg.Dots[:1]
	if err := Link(cfg, opts); err != nil {
		t.Errorf("all linked: unexpected error %v", err)
	}
}
