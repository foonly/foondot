package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainsConflictMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"conflict", "a\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> 1234abc (change)\nb\n", true},
		{"diff3 conflict", "<<<<<<< HEAD\nours\n||||||| base\nbase\n=======\ntheirs\n>>>>>>> 1234abc\n", true},
		{"windows line endings", "<<<<<<< HEAD\r\nours\r\n=======\r\ntheirs\r\n>>>>>>> 1234abc\r\n", true},
		{"no markers", "just text\n", false},
		{"markdown heading", "Title\n=======\n\ntext\n", false},
		{"markers inside lines", "a <<<<<<< b\nx =======\nc >>>>>>> d\n", false},
		{"longer runs", "<<<<<<<<\n========\n>>>>>>>>\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(file, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := ContainsConflictMarkers(file)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsConflictMarkersNotRegular(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "missing"), link); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{dir, link} {
		got, err := ContainsConflictMarkers(p)
		if err != nil || got {
			t.Errorf("%s: got (%v, %v), want (false, nil)", p, got, err)
		}
	}
}

func TestContainsConflictMarkersMissing(t *testing.T) {
	if _, err := ContainsConflictMarkers(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.json")
	if err := os.WriteFile(file, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(file, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("content = %q, want %q", data, "new")
	}
	stat, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0644 {
		t.Errorf("mode = %v, want 0644", stat.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want no leftover temporary files", len(entries))
	}
}

func TestWriteFileAtomicFailureKeepsOldFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.json")
	if err := os.WriteFile(file, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	// The target is a directory now, so the rename fails.
	target := filepath.Join(dir, "sub")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "x"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(target, []byte("new"), 0644); err == nil {
		t.Error("expected an error when replacing a non-empty directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("directory has %d entries, want no leftover temporary files", len(entries))
	}
}
