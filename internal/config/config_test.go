package config

import (
	"os"
	"path"
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

func TestReadDotsData(t *testing.T) {
	dir := t.TempDir()

	got, err := readDotsData(path.Join(dir, "missing.json"))
	if err != nil || len(got) != 0 {
		t.Errorf("missing file: got (%v, %v), want empty list", got, err)
	}

	valid := path.Join(dir, "valid.json")
	if err := os.WriteFile(valid, []byte(`["/a","/b"]`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = readDotsData(valid)
	if err != nil || !slices.Equal(got, []string{"/a", "/b"}) {
		t.Errorf("valid file: got (%v, %v)", got, err)
	}

	broken := path.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte(`["/a",`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDotsData(broken); err == nil {
		t.Error("broken file: expected an error")
	}

	// A directory can't be read as a file, which must not count as missing.
	if _, err := readDotsData(dir); err == nil {
		t.Error("unreadable file: expected an error")
	}
}
