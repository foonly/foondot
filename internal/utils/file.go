package utils

import (
	"errors"
	"os"
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
