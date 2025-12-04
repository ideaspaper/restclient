package lastfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestEnv sets up a temporary directory for testing and returns a cleanup function
func setupTestEnv(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "restclient-lastfile-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Override the getLastFilePath function to use our temp directory
	originalGetLastFilePath := getLastFilePath
	getLastFilePath = func() (string, error) {
		return filepath.Join(tmpDir, "lastfile"), nil
	}

	cleanup := func() {
		getLastFilePath = originalGetLastFilePath
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestSave(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(testFile, []byte("GET https://example.com"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Save the file path
	if err := Save(testFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file was saved
	lastFilePath, _ := getLastFilePath()
	data, err := os.ReadFile(lastFilePath)
	if err != nil {
		t.Fatalf("failed to read lastfile: %v", err)
	}

	savedPath := strings.TrimSpace(string(data))
	absTestFile, _ := filepath.Abs(testFile)
	if savedPath != absTestFile {
		t.Errorf("saved path mismatch: got %q, want %q", savedPath, absTestFile)
	}
}

func TestSave_ConvertsToAbsolutePath(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(testFile, []byte("GET https://example.com"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Save with a relative path (this will be converted to absolute)
	if err := Save(testFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify it's an absolute path
	lastFilePath, _ := getLastFilePath()
	data, _ := os.ReadFile(lastFilePath)
	savedPath := strings.TrimSpace(string(data))

	if !filepath.IsAbs(savedPath) {
		t.Errorf("saved path should be absolute: %q", savedPath)
	}
}

func TestLoad_ReturnsEmptyWhenNoFile(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	path, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if path != "" {
		t.Errorf("expected empty string, got %q", path)
	}
}

func TestLoad_ReturnsPath(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create and save a test file
	testFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(testFile, []byte("GET https://example.com"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := Save(testFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load the path
	path, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	absTestFile, _ := filepath.Abs(testFile)
	if path != absTestFile {
		t.Errorf("loaded path mismatch: got %q, want %q", path, absTestFile)
	}
}

func TestLoad_ErrorWhenFileDeleted(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create and save a test file
	testFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(testFile, []byte("GET https://example.com"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := Save(testFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete the test file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("failed to remove test file: %v", err)
	}

	// Load should return an error
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when file is deleted, got nil")
	}

	// Error message should mention the file was removed
	if !strings.Contains(err.Error(), "removed") {
		t.Errorf("error should mention file was removed: %v", err)
	}

	// The lastfile should be cleared after the error
	if Exists() {
		t.Error("lastfile should be cleared after detecting deleted file")
	}

	// Subsequent Load should return empty string with no error
	path, err := Load()
	if err != nil {
		t.Errorf("second Load should not error after clear: %v", err)
	}
	if path != "" {
		t.Errorf("second Load should return empty string, got %q", path)
	}
}

func TestLoad_EmptyFileContent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Write an empty lastfile
	lastFilePath, _ := getLastFilePath()
	if err := os.MkdirAll(filepath.Dir(lastFilePath), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(lastFilePath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty lastfile: %v", err)
	}

	path, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if path != "" {
		t.Errorf("expected empty string for empty file, got %q", path)
	}
}

func TestLoad_WhitespaceOnlyFileContent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Write a whitespace-only lastfile
	lastFilePath, _ := getLastFilePath()
	if err := os.MkdirAll(filepath.Dir(lastFilePath), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(lastFilePath, []byte("   \n\t  "), 0644); err != nil {
		t.Fatalf("failed to write whitespace lastfile: %v", err)
	}

	path, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if path != "" {
		t.Errorf("expected empty string for whitespace-only file, got %q", path)
	}
}

func TestClear(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create and save a test file
	testFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(testFile, []byte("GET https://example.com"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := Save(testFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Clear the last file
	if err := Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Load should return empty string
	path, err := Load()
	if err != nil {
		t.Fatalf("Load after Clear failed: %v", err)
	}

	if path != "" {
		t.Errorf("expected empty string after Clear, got %q", path)
	}
}

func TestClear_NoFile(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Clear when no file exists should not fail
	if err := Clear(); err != nil {
		t.Fatalf("Clear should not fail when no file exists: %v", err)
	}
}

func TestExists(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Initially should not exist
	if Exists() {
		t.Error("Exists should return false when no file saved")
	}

	// Create and save a test file
	testFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(testFile, []byte("GET https://example.com"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := Save(testFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Now should exist
	if !Exists() {
		t.Error("Exists should return true after Save")
	}

	// Clear and check again
	if err := Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if Exists() {
		t.Error("Exists should return false after Clear")
	}
}

func TestSaveReplacesExisting(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create two test files
	testFile1 := filepath.Join(tmpDir, "test1.http")
	testFile2 := filepath.Join(tmpDir, "test2.http")
	if err := os.WriteFile(testFile1, []byte("GET https://example.com/1"), 0644); err != nil {
		t.Fatalf("failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("GET https://example.com/2"), 0644); err != nil {
		t.Fatalf("failed to create test file 2: %v", err)
	}

	// Save first file
	if err := Save(testFile1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Save second file (should replace)
	if err := Save(testFile2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load should return second file
	path, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	absTestFile2, _ := filepath.Abs(testFile2)
	if path != absTestFile2 {
		t.Errorf("expected %q, got %q", absTestFile2, path)
	}
}
