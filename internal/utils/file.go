package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileType is the kind of filesystem entry at a path, see GetType.
type FileType int

// File types returned by GetType.
const (
	IsFailed FileType = iota
	NotExists
	IsSymlink
	IsDirectory
	IsFile
)

// GetType determines the type of a file or directory without following
// symlinks. It returns IsFailed if the path can't be examined.
func GetType(fileName string) FileType {
	stat, err := os.Lstat(fileName)
	if errors.Is(err, fs.ErrNotExist) {
		return NotExists
	} else if err != nil {
		return IsFailed
	}
	if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
		return IsSymlink
	}
	if stat.Mode()&os.ModeDir == os.ModeDir {
		return IsDirectory
	}
	return IsFile
}

// WriteFileAtomic writes data to a file by writing a temporary file in the
// same directory and renaming it over the target, so the file is never left
// partially written.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(filename), "."+filepath.Base(filename)+".*.tmp")
	if err != nil {
		return err
	}
	// Removing fails harmlessly once the file has been renamed.
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filename)
}

// ContainsConflictMarkers checks if a file contains git conflict markers.
// Only lines starting with a complete set of markers count, so text like a
// Markdown "=======" heading isn't reported. Anything other than a regular
// file (symlinks, directories such as submodules) is not checked. An error is
// returned if the file can't be read.
func ContainsConflictMarkers(filePath string) (bool, error) {
	stat, err := os.Lstat(filePath)
	if err != nil {
		return false, err
	}
	if !stat.Mode().IsRegular() {
		return false, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	var start, middle, end bool
	for line := range strings.Lines(string(data)) {
		line = strings.TrimRight(line, "\r\n")
		switch {
		case isMarker(line, "<<<<<<<"):
			start = true
		case line == "=======":
			middle = true
		case isMarker(line, ">>>>>>>"):
			end = true
		}
	}
	return start && middle && end, nil
}

// isMarker checks if a line is a git conflict marker, i.e. the marker alone or
// followed by a space and a label, such as "<<<<<<< HEAD".
func isMarker(line string, marker string) bool {
	rest, ok := strings.CutPrefix(line, marker)
	return ok && (rest == "" || rest[0] == ' ')
}
