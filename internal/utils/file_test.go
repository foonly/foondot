package utils

import (
	"os"
	"path"
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
			file := path.Join(t.TempDir(), "file")
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
	link := path.Join(dir, "link")
	if err := os.Symlink(path.Join(dir, "missing"), link); err != nil {
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
	if _, err := ContainsConflictMarkers(path.Join(t.TempDir(), "missing")); err == nil {
		t.Error("expected an error for a missing file")
	}
}
