package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

/**
 * File types.
 */
const (
	IsFailed = iota
	NotExists
	IsSymlink
	IsDirectory
	IsFile
)

/**
 * Determines the type of a file or directory.
 *
 * @param fileName The path to the file or directory.
 * @return An integer representing the file type (notExists, isSymlink, isDirectory, isFile, isFailed).
 */
func GetType(fileName string) int {
	stat, err := os.Lstat(fileName)
	if errors.Is(err, os.ErrNotExist) {
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

/**
 * Writes data to a file by writing a temporary file in the same directory and
 * renaming it over the target, so the file is never left partially written.
 *
 * @param filename The path to the file.
 * @param data The data to write.
 * @param perm The permissions of the file.
 * @return error Set if the file couldn't be written.
 */
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

/**
 * Checks if a file contains git conflict markers. Only lines starting with a
 * complete set of markers count, so text like a Markdown "=======" heading
 * isn't reported. Anything other than a regular file (symlinks, directories
 * such as submodules) is not checked.
 *
 * @param filePath The path to the file.
 * @return bool True if conflict markers are found, false otherwise.
 * @return error Set if the file could not be read.
 */
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

/**
 * Checks if a line is a git conflict marker, i.e. the marker alone or followed
 * by a space and a label, such as "<<<<<<< HEAD".
 */
func isMarker(line string, marker string) bool {
	rest, ok := strings.CutPrefix(line, marker)
	return ok && (rest == "" || rest[0] == ' ')
}
