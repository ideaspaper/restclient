// Package filesystem provides a file system abstraction for testability.
package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MockFileSystem is a mock implementation of FileSystem for testing.
// It uses an in-memory map to simulate files and directories.
type MockFileSystem struct {
	mu sync.RWMutex

	// Files stores file contents by path.
	Files map[string][]byte

	// Dirs stores directory paths (value is always true).
	Dirs map[string]bool

	// Err is a default error to return (if set, overrides normal behavior).
	Err error

	// ErrByPath allows setting specific errors for specific paths.
	ErrByPath map[string]error

	// HomeDir is the home directory to return from UserHomeDir.
	HomeDir string
}

// Ensure MockFileSystem implements FileSystem
var _ FileSystem = (*MockFileSystem)(nil)

// NewMockFileSystem creates a new MockFileSystem.
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		Files:     make(map[string][]byte),
		Dirs:      make(map[string]bool),
		ErrByPath: make(map[string]error),
		HomeDir:   "/home/testuser",
	}
}

// pathError returns any error configured for this path.
func (m *MockFileSystem) pathError(name string) error {
	if m.Err != nil {
		return m.Err
	}
	if err, ok := m.ErrByPath[name]; ok {
		return err
	}
	return nil
}

// ReadFile reads the named file from the mock file system.
func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.pathError(name); err != nil {
		return nil, err
	}

	data, ok := m.Files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return data, nil
}

// Stat returns a mock FileInfo for the named file or directory.
func (m *MockFileSystem) Stat(name string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.pathError(name); err != nil {
		return nil, err
	}

	// Check if it's a file
	if data, ok := m.Files[name]; ok {
		return &mockFileInfo{
			name:  filepath.Base(name),
			size:  int64(len(data)),
			isDir: false,
		}, nil
	}

	// Check if it's a directory
	if m.Dirs[name] {
		return &mockFileInfo{
			name:  filepath.Base(name),
			size:  0,
			isDir: true,
		}, nil
	}

	// Check if any file exists under this path (implicit directory)
	for p := range m.Files {
		if strings.HasPrefix(p, name+"/") {
			return &mockFileInfo{
				name:  filepath.Base(name),
				size:  0,
				isDir: true,
			}, nil
		}
	}

	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// Open opens the named file for reading.
// Note: Returns an os.File pointer for interface compatibility, but for mocks
// this will return nil with an error since we can't create real file handles.
// Use ReadFile for mock testing instead.
func (m *MockFileSystem) Open(name string) (*os.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.pathError(name); err != nil {
		return nil, err
	}

	if _, ok := m.Files[name]; !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	// We can't return a real *os.File from a mock, so we return an error
	// that indicates the mock limitation. For testing, use ReadFile instead.
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
}

// WriteFile writes data to the named file in the mock file system.
func (m *MockFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.pathError(name); err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(name)
	if dir != "." && dir != "/" {
		m.Dirs[dir] = true
	}

	m.Files[name] = data
	return nil
}

// MkdirAll creates a directory path in the mock file system.
func (m *MockFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.pathError(path); err != nil {
		return err
	}

	// Create all path components
	parts := strings.Split(path, "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			current = "/"
			continue
		}
		if current == "/" {
			current = "/" + part
		} else {
			current = current + "/" + part
		}
		m.Dirs[current] = true
	}
	return nil
}

// Remove removes the named file or empty directory.
func (m *MockFileSystem) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.pathError(name); err != nil {
		return err
	}

	// Try to remove as file
	if _, ok := m.Files[name]; ok {
		delete(m.Files, name)
		return nil
	}

	// Try to remove as directory
	if m.Dirs[name] {
		// Check if directory is empty
		for p := range m.Files {
			if strings.HasPrefix(p, name+"/") {
				return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
			}
		}
		delete(m.Dirs, name)
		return nil
	}

	return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
}

// RemoveAll removes path and any children it contains.
func (m *MockFileSystem) RemoveAll(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.pathError(path); err != nil {
		return err
	}

	// Remove the path itself if it's a file
	delete(m.Files, path)
	delete(m.Dirs, path)

	// Remove all files and dirs under this path
	prefix := path + "/"
	for p := range m.Files {
		if strings.HasPrefix(p, prefix) {
			delete(m.Files, p)
		}
	}
	for p := range m.Dirs {
		if strings.HasPrefix(p, prefix) {
			delete(m.Dirs, p)
		}
	}

	return nil
}

// UserHomeDir returns the mock home directory.
func (m *MockFileSystem) UserHomeDir() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Err != nil {
		return "", m.Err
	}
	return m.HomeDir, nil
}

// ReadDir reads the named directory and returns all its directory entries.
func (m *MockFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.pathError(name); err != nil {
		return nil, err
	}

	// Check if the directory exists
	exists := false
	if m.Dirs[name] {
		exists = true
	}
	// Also check if any file exists under this path (implicit directory)
	prefix := name + "/"
	for p := range m.Files {
		if strings.HasPrefix(p, prefix) {
			exists = true
			break
		}
	}
	if !exists {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	var entries []fs.DirEntry
	seen := make(map[string]bool)

	// Find all direct children (files)
	for path := range m.Files {
		if strings.HasPrefix(path, prefix) {
			rest := path[len(prefix):]
			// Get the first component (direct child)
			if idx := strings.Index(rest, "/"); idx >= 0 {
				// This is a directory
				dirName := rest[:idx]
				if !seen[dirName] {
					seen[dirName] = true
					entries = append(entries, &mockDirEntry{name: dirName, isDir: true})
				}
			} else {
				// This is a file
				if !seen[rest] {
					seen[rest] = true
					entries = append(entries, &mockDirEntry{name: rest, isDir: false})
				}
			}
		}
	}

	// Find all direct children (explicit directories)
	for path := range m.Dirs {
		if strings.HasPrefix(path, prefix) {
			rest := path[len(prefix):]
			// Get the first component (direct child)
			if idx := strings.Index(rest, "/"); idx >= 0 {
				dirName := rest[:idx]
				if !seen[dirName] {
					seen[dirName] = true
					entries = append(entries, &mockDirEntry{name: dirName, isDir: true})
				}
			} else if rest != "" && !seen[rest] {
				seen[rest] = true
				entries = append(entries, &mockDirEntry{name: rest, isDir: true})
			}
		}
	}

	return entries, nil
}

// --- Helper methods for test setup ---

// WithFile adds a file to the mock file system and returns the mock for chaining.
func (m *MockFileSystem) WithFile(path string, content []byte) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Files[path] = content
	// Ensure parent directories exist
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		m.Dirs[dir] = true
	}
	return m
}

// WithFileString adds a file with string content.
func (m *MockFileSystem) WithFileString(path, content string) *MockFileSystem {
	return m.WithFile(path, []byte(content))
}

// WithDir adds a directory to the mock file system.
func (m *MockFileSystem) WithDir(path string) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Dirs[path] = true
	return m
}

// WithError sets a default error to return for all operations.
func (m *MockFileSystem) WithError(err error) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Err = err
	return m
}

// WithPathError sets an error for a specific path.
func (m *MockFileSystem) WithPathError(path string, err error) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ErrByPath[path] = err
	return m
}

// WithHomeDir sets the home directory.
func (m *MockFileSystem) WithHomeDir(path string) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HomeDir = path
	return m
}

// Reset clears all files, directories, and errors.
func (m *MockFileSystem) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Files = make(map[string][]byte)
	m.Dirs = make(map[string]bool)
	m.ErrByPath = make(map[string]error)
	m.Err = nil
}

// FileCount returns the number of files in the mock file system.
func (m *MockFileSystem) FileCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Files)
}

// HasFile returns true if the file exists.
func (m *MockFileSystem) HasFile(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.Files[path]
	return ok
}

// mockFileInfo implements fs.FileInfo for the mock file system.
type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() any           { return nil }

// mockDirEntry implements fs.DirEntry for the mock file system.
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string { return m.name }
func (m *mockDirEntry) IsDir() bool  { return m.isDir }
func (m *mockDirEntry) Type() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	} else {
		return 0
	}
}
func (m *mockDirEntry) Info() (fs.FileInfo, error) {
	return &mockFileInfo{name: m.name, isDir: m.isDir}, nil
}
