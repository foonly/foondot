package dots

import (
	"os"
	"path"
	"slices"
	"strconv"
	"testing"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

// setupHome points xdg.Home at a temporary directory containing an empty
// "dotfiles" folder, and resets global state touched by the dots package.
func setupHome(t *testing.T) (home string, dotfilesDir string) {
	t.Helper()
	home = t.TempDir()
	dotfilesDir = path.Join(home, "dotfiles")
	mkdir(t, dotfilesDir)

	oldHome, oldDataHome, oldHostname, oldData := xdg.Home, xdg.DataHome, config.Hostname, config.DotsData
	t.Cleanup(func() {
		xdg.Home, xdg.DataHome, config.Hostname, config.DotsData = oldHome, oldDataHome, oldHostname, oldData
	})
	xdg.Home = home
	xdg.DataHome = path.Join(home, ".local", "share")
	config.Hostname = "thishost"
	config.DotsData = []string{}
	return home, dotfilesDir
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
	mkdir(t, path.Dir(file))
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
	setupHome(t)
	dots := []config.Item{
		{Source: "all", Target: ".all"},
		{Source: "mine", Target: ".mine", Hostname: []string{"thishost", "other"}},
		{Source: "theirs", Target: ".theirs", Hostname: []string{"other"}},
	}

	got, complete := filterDots("dotfiles", dots)

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
	_, dotfilesDir := setupHome(t)
	writeFile(t, path.Join(dotfilesDir, "bin", "a"))
	writeFile(t, path.Join(dotfilesDir, "bin", "b"))

	got, complete := filterDots("dotfiles", []config.Item{{Source: "bin/*", Target: ".local/bin"}})

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
	setupHome(t)

	got, complete := filterDots("dotfiles", []config.Item{
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
	home, dotfilesDir := setupHome(t)
	writeFile(t, path.Join(dotfilesDir, "keep"))
	writeFile(t, path.Join(dotfilesDir, "stale"))
	writeFile(t, path.Join(home, "elsewhere"))

	keep := path.Join(home, ".keep")
	stale := path.Join(home, ".stale")
	foreign := path.Join(home, ".foreign")
	notLink := path.Join(home, ".notlink")
	missing := path.Join(home, ".missing")

	symlink(t, path.Join(dotfilesDir, "keep"), keep)
	symlink(t, path.Join(dotfilesDir, "stale"), stale)
	// A tracked link the user replaced with their own symlink.
	symlink(t, path.Join(home, "elsewhere"), foreign)
	writeFile(t, notLink)

	config.DotsData = []string{keep, stale, foreign, notLink, missing}

	cleanTargets(dotfilesDir, []config.Item{{Source: "keep", Target: ".keep"}})

	if !slices.Equal(config.DotsData, []string{keep}) {
		t.Errorf("tracked targets = %v, want [%s]", config.DotsData, keep)
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
	home, dotfilesDir := setupHome(t)

	relative := path.Join(home, ".relative")
	symlink(t, "dotfiles/x", relative)
	sibling := path.Join(home, ".sibling")
	// Shares the "dotfiles" prefix but is a different directory.
	symlink(t, path.Join(home, "dotfiles-old", "x"), sibling)
	escape := path.Join(home, ".escape")
	symlink(t, path.Join(dotfilesDir, "..", "x"), escape)

	if !pointsInto(relative, dotfilesDir) {
		t.Error("relative link into dotfiles not recognized")
	}
	if pointsInto(sibling, dotfilesDir) {
		t.Error("link into sibling directory treated as inside dotfiles")
	}
	if pointsInto(escape, dotfilesDir) {
		t.Error("link escaping via .. treated as inside dotfiles")
	}
	if pointsInto(path.Join(home, ".nothing"), dotfilesDir) {
		t.Error("nonexistent path treated as a link into dotfiles")
	}
}

func TestTargetPath(t *testing.T) {
	home, _ := setupHome(t)

	if got, want := targetPath(".bashrc"), path.Join(home, ".bashrc"); got != want {
		t.Errorf("relative target: got %s, want %s", got, want)
	}
	if got, want := targetPath("/etc/foo/"), "/etc/foo"; got != want {
		t.Errorf("absolute target: got %s, want %s", got, want)
	}
}

func TestLinkAbsoluteTarget(t *testing.T) {
	home, dotfilesDir := setupHome(t)
	writeFile(t, path.Join(dotfilesDir, "conf"))
	// An absolute target outside the home directory.
	outside := path.Join(t.TempDir(), "etc", "conf")

	handleDot(config.Item{Source: "conf", Target: outside}, "dotfiles", false)

	if utils.GetType(outside) != utils.IsSymlink {
		t.Errorf("expected symlink at %s", outside)
	}
	if utils.GetType(path.Join(home, outside)) != utils.NotExists {
		t.Error("absolute target was created under the home directory")
	}

	// The absolute target must count as current, so cleanup keeps it.
	cleanTargets(dotfilesDir, []config.Item{{Source: "conf", Target: outside}})
	if !slices.Equal(config.DotsData, []string{outside}) || utils.GetType(outside) != utils.IsSymlink {
		t.Error("cleanup removed a current absolute target")
	}
}

func TestHandleDotMovesTargetToMissingSource(t *testing.T) {
	home, dotfilesDir := setupHome(t)
	target := path.Join(home, ".config", "app")
	writeContent(t, target, "existing")

	if !handleDot(config.Item{Source: "app/config", Target: ".config/app"}, "dotfiles", false) {
		t.Fatal("expected link to be created")
	}

	source := path.Join(dotfilesDir, "app", "config")
	if got := readContent(t, source); got != "existing" {
		t.Errorf("source = %q, want the former target content", got)
	}
	if utils.GetType(target) != utils.IsSymlink {
		t.Error("target is not a symlink")
	}
}

func TestHandleDotSkipsExistingWithoutForce(t *testing.T) {
	home, dotfilesDir := setupHome(t)
	writeContent(t, path.Join(dotfilesDir, "bashrc"), "source")
	target := path.Join(home, ".bashrc")
	writeContent(t, target, "target")

	if handleDot(config.Item{Source: "bashrc", Target: ".bashrc"}, "dotfiles", false) {
		t.Error("expected no link without force")
	}
	if got := readContent(t, target); got != "target" {
		t.Errorf("target = %q, want it untouched", got)
	}
}

func TestHandleDotForceBacksUpOutsideDotfiles(t *testing.T) {
	home, dotfilesDir := setupHome(t)
	writeContent(t, path.Join(dotfilesDir, "bashrc"), "source")
	target := path.Join(home, ".bashrc")
	backup := path.Join(config.BackupFolder(), target)
	item := config.Item{Source: "bashrc", Target: ".bashrc"}

	for i, want := range []string{backup, backup + ".1"} {
		writeContent(t, target+".tmp", "target "+strconv.Itoa(i))
		os.Remove(target)
		if err := os.Rename(target+".tmp", target); err != nil {
			t.Fatal(err)
		}

		if !handleDot(item, "dotfiles", true) {
			t.Fatalf("run %d: expected link to be created", i)
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
	home, dotfilesDir := setupHome(t)
	writeFile(t, path.Join(dotfilesDir, "conf"))
	// The target's parent is a file, so its directory can't be created.
	writeFile(t, path.Join(home, "blocker"))
	target := path.Join(home, "blocker", "conf")

	if err := prepareTargetSource(target, path.Join(dotfilesDir, "conf"), false); err == nil {
		t.Error("expected an error when the target directory can't be created")
	}
	if handleDot(config.Item{Source: "conf", Target: "blocker/conf"}, "dotfiles", false) {
		t.Error("expected handleDot to fail")
	}
}
