// Package lastfile provides functionality to persist and retrieve the last used .http/.rest file path.
package lastfile

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ideaspaper/restclient/internal/paths"
	"github.com/ideaspaper/restclient/pkg/errors"
)

// getLastFilePath returns the path to store the last file path.
// This is a variable to allow overriding in tests.
var getLastFilePath = paths.DefaultLastFilePath

// Save stores the given file path as the last used file.
// The path is converted to an absolute path before saving.
func Save(filePath string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to get absolute path")
	}

	lastFilePath, err := getLastFilePath()
	if err != nil {
		return errors.Wrap(err, "failed to get last file path location")
	}

	// Ensure the directory exists
	dir := filepath.Dir(lastFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, "failed to create directory")
	}

	if err := os.WriteFile(lastFilePath, []byte(absPath), 0644); err != nil {
		return errors.Wrap(err, "failed to save last file path")
	}

	return nil
}

// Load retrieves the last used file path.
// Returns an empty string and nil error if no last file is stored.
// If the stored file no longer exists, clears the lastfile and returns an error.
func Load() (string, error) {
	lastFilePath, err := getLastFilePath()
	if err != nil {
		return "", errors.Wrap(err, "failed to get last file path location")
	}

	data, err := os.ReadFile(lastFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", errors.Wrap(err, "failed to read last file path")
	}

	storedPath := strings.TrimSpace(string(data))
	if storedPath == "" {
		return "", nil
	}

	// Check if the stored file still exists
	if _, err := os.Stat(storedPath); err != nil {
		if os.IsNotExist(err) {
			// Clear the lastfile since the referenced file no longer exists
			_ = Clear() // Ignore error from Clear, the main error is more important
			return "", errors.NewValidationError("last file", "the previously used file has been removed: "+storedPath)
		}
		return "", errors.Wrap(err, "failed to check if last file exists")
	}

	return storedPath, nil
}

// Clear removes the stored last file path.
func Clear() error {
	lastFilePath, err := getLastFilePath()
	if err != nil {
		return errors.Wrap(err, "failed to get last file path location")
	}

	if err := os.Remove(lastFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already cleared
		}
		return errors.Wrap(err, "failed to clear last file path")
	}

	return nil
}

// Exists returns true if there is a stored last file path (regardless of whether the file exists).
func Exists() bool {
	lastFilePath, err := getLastFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(lastFilePath)
	return err == nil
}
