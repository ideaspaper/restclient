// Package paths provides common path utilities for the application.
package paths

import (
	"path/filepath"

	"github.com/ideaspaper/restclient/internal/filesystem"
)

const (
	// AppDirName is the name of the application's data directory
	AppDirName = ".restclient"
)

// HomeDir returns the user's home directory.
func HomeDir() (string, error) {
	return filesystem.Default.UserHomeDir()
}

// AppDataDir returns the path to the application's data directory.
// If subdir is provided, it returns the path to that subdirectory.
func AppDataDir(subdir string) (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	if subdir == "" {
		return filepath.Join(home, AppDirName), nil
	}
	return filepath.Join(home, AppDirName, subdir), nil
}

// DefaultConfigPath returns the path to the default config file.
func DefaultConfigPath() (string, error) {
	dir, err := AppDataDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// DefaultHistoryPath returns the path to the default history file.
func DefaultHistoryPath() (string, error) {
	dir, err := AppDataDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "request_history.json"), nil
}

// DefaultSessionDir returns the path to the default session directory.
func DefaultSessionDir() (string, error) {
	return AppDataDir("session")
}

// DefaultLastFilePath returns the path to the file storing the last used .http/.rest file path.
func DefaultLastFilePath() (string, error) {
	dir, err := AppDataDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lastfile"), nil
}

// Exists returns true if the path exists (file or directory).
func Exists(path string) bool {
	return filesystem.Exists(path)
}

// IsFile returns true if the path is a file.
func IsFile(path string) bool {
	return filesystem.IsFile(path)
}

// IsDir returns true if the path is a directory.
func IsDir(path string) bool {
	return filesystem.IsDir(path)
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return filesystem.EnsureDir(path)
}
