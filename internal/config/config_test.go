package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestValidateConfigSyncStrategy(t *testing.T) {
	for _, strategy := range []string{"manual", "local", "remote"} {
		if err := validateConfig(Config{SyncStrategy: strategy}); err != nil {
			t.Errorf("%s: unexpected error %v", strategy, err)
		}
	}
	for _, strategy := range []string{"remot", "Local", "theirs"} {
		if err := validateConfig(Config{SyncStrategy: strategy}); err == nil {
			t.Errorf("%s: expected an error", strategy)
		}
	}
}

func writeDotsData(t *testing.T, dataDir, content string) {
	t.Helper()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, dotsDataFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadDotsData(t *testing.T) {
	root := t.TempDir()

	got, err := ReadDotsData(filepath.Join(root, "missing"))
	if err != nil || len(got) != 0 {
		t.Errorf("missing file: got (%v, %v), want empty list", got, err)
	}

	valid := filepath.Join(root, "valid")
	writeDotsData(t, valid, `["/a","/b"]`)
	got, err = ReadDotsData(valid)
	if err != nil || !slices.Equal(got, []string{"/a", "/b"}) {
		t.Errorf("valid file: got (%v, %v)", got, err)
	}

	broken := filepath.Join(root, "broken")
	writeDotsData(t, broken, `["/a",`)
	if _, err := ReadDotsData(broken); err == nil {
		t.Error("broken file: expected an error")
	}

	// A directory can't be read as a file, which must not count as missing.
	unreadable := filepath.Join(root, "unreadable")
	if err := os.MkdirAll(filepath.Join(unreadable, dotsDataFileName), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDotsData(unreadable); err == nil {
		t.Error("unreadable file: expected an error")
	}
}

func TestWriteDotsDataRoundTrip(t *testing.T) {
	// The data folder doesn't exist yet and must be created.
	dataDir := filepath.Join(t.TempDir(), "foondot")
	want := []string{"/home/u/.bashrc", "/etc/foo"}

	if err := WriteDotsData(dataDir, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDotsData(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
