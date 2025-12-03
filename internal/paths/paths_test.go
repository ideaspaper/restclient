package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeDir(t *testing.T) {
	got, err := HomeDir()
	if err != nil {
		t.Fatalf("HomeDir() error = %v", err)
	}
	if got == "" {
		t.Error("HomeDir() returned empty string")
	}

	// Should match os.UserHomeDir
	expected, _ := os.UserHomeDir()
	if got != expected {
		t.Errorf("HomeDir() = %q, want %q", got, expected)
	}
}

func TestAppDataDir(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	tests := []struct {
		name     string
		subdir   string
		expected string
	}{
		{
			name:     "base directory",
			subdir:   "",
			expected: filepath.Join(homeDir, AppDirName),
		},
		{
			name:     "session subdirectory",
			subdir:   "session",
			expected: filepath.Join(homeDir, AppDirName, "session"),
		},
		{
			name:     "nested subdirectory",
			subdir:   "session/named",
			expected: filepath.Join(homeDir, AppDirName, "session/named"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AppDataDir(tt.subdir)
			if err != nil {
				t.Fatalf("AppDataDir(%q) error = %v", tt.subdir, err)
			}
			if got != tt.expected {
				t.Errorf("AppDataDir(%q) = %q, want %q", tt.subdir, got, tt.expected)
			}
		})
	}
}

func TestDefaultConfigPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() error = %v", err)
	}

	expected := filepath.Join(homeDir, AppDirName, "config.json")
	if got != expected {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, expected)
	}
}

func TestDefaultHistoryPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	got, err := DefaultHistoryPath()
	if err != nil {
		t.Fatalf("DefaultHistoryPath() error = %v", err)
	}

	expected := filepath.Join(homeDir, AppDirName, "request_history.json")
	if got != expected {
		t.Errorf("DefaultHistoryPath() = %q, want %q", got, expected)
	}
}

func TestDefaultSessionDir(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	got, err := DefaultSessionDir()
	if err != nil {
		t.Fatalf("DefaultSessionDir() error = %v", err)
	}

	expected := filepath.Join(homeDir, AppDirName, "session")
	if got != expected {
		t.Errorf("DefaultSessionDir() = %q, want %q", got, expected)
	}
}

func TestExists(t *testing.T) {
	// Test with existing file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if !Exists(tmpFile.Name()) {
		t.Errorf("Exists(%q) = false, want true", tmpFile.Name())
	}

	// Test with non-existing file
	if Exists("/nonexistent/path/to/file") {
		t.Error("Exists('/nonexistent/path/to/file') = true, want false")
	}
}

func TestIsFile(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if !IsFile(tmpFile.Name()) {
		t.Errorf("IsFile(%q) = false, want true", tmpFile.Name())
	}

	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if IsFile(tmpDir) {
		t.Errorf("IsFile(%q) = true, want false (it's a directory)", tmpDir)
	}
}

func TestIsDir(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if !IsDir(tmpDir) {
		t.Errorf("IsDir(%q) = false, want true", tmpDir)
	}

	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if IsDir(tmpFile.Name()) {
		t.Errorf("IsDir(%q) = true, want false (it's a file)", tmpFile.Name())
	}
}

func TestEnsureDir(t *testing.T) {
	// Create a temp base directory
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a nested path that doesn't exist
	nestedPath := filepath.Join(tmpDir, "a", "b", "c")

	if err := EnsureDir(nestedPath); err != nil {
		t.Fatalf("EnsureDir(%q) error = %v", nestedPath, err)
	}

	if !IsDir(nestedPath) {
		t.Errorf("EnsureDir(%q) didn't create the directory", nestedPath)
	}

	// Calling EnsureDir again should succeed (idempotent)
	if err := EnsureDir(nestedPath); err != nil {
		t.Fatalf("EnsureDir(%q) on existing dir error = %v", nestedPath, err)
	}
}
