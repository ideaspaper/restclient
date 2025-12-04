// Package filesystem provides a file system abstraction for testability.
package filesystem

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// FileSystem defines the interface for file system operations.
// This abstraction allows for easy mocking in tests.
type FileSystem interface {
	// Read operations
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	Open(name string) (*os.File, error)

	// Write operations
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(path string) error

	// Directory operations
	ReadDir(name string) ([]fs.DirEntry, error)

	// Path operations
	UserHomeDir() (string, error)
}

// OSFileSystem implements FileSystem using the real OS file system.
type OSFileSystem struct{}

// Default is the default file system implementation using OS calls.
var Default FileSystem = &OSFileSystem{}

// ReadFile reads the named file and returns the contents.
func (OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Stat returns a FileInfo describing the named file.
func (OSFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// Open opens the named file for reading.
func (OSFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// WriteFile writes data to the named file, creating it if necessary.
func (OSFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// MkdirAll creates a directory named path, along with any necessary parents.
func (OSFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove removes the named file or empty directory.
func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes path and any children it contains.
func (OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// UserHomeDir returns the current user's home directory.
func (OSFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// ReadDir reads the named directory and returns all its directory entries.
func (OSFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

// Helper functions that use the Default file system

// Exists returns true if the path exists (file or directory).
func Exists(path string) bool {
	_, err := Default.Stat(path)
	return err == nil
}

// IsFile returns true if the path is a file.
func IsFile(path string) bool {
	info, err := Default.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsDir returns true if the path is a directory.
func IsDir(path string) bool {
	info, err := Default.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return Default.MkdirAll(path, 0755)
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	source, err := Default.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	data, err := io.ReadAll(source)
	if err != nil {
		return err
	}

	return Default.WriteFile(dst, data, 0644)
}

// AppDataDir returns the path to the application's data directory.
// If subdir is provided, it returns the path to that subdirectory.
func AppDataDir(appDirName, subdir string) (string, error) {
	home, err := Default.UserHomeDir()
	if err != nil {
		return "", err
	}
	if subdir == "" {
		return filepath.Join(home, appDirName), nil
	}
	return filepath.Join(home, appDirName, subdir), nil
}
