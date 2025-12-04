package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSFileSystem(t *testing.T) {
	fs := &OSFileSystem{}
	tempDir := t.TempDir()

	t.Run("WriteFile and ReadFile", func(t *testing.T) {
		path := filepath.Join(tempDir, "test.txt")
		content := []byte("hello, world")

		if err := fs.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		data, err := fs.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(data) != string(content) {
			t.Errorf("expected %q, got %q", string(content), string(data))
		}
	})

	t.Run("Stat", func(t *testing.T) {
		path := filepath.Join(tempDir, "stat_test.txt")
		if err := fs.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		info, err := fs.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Error("expected file, not directory")
		}
		if info.Size() != 4 {
			t.Errorf("expected size 4, got %d", info.Size())
		}
	})

	t.Run("Open", func(t *testing.T) {
		path := filepath.Join(tempDir, "open_test.txt")
		content := []byte("readable content")
		if err := fs.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		file, err := fs.Open(path)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer file.Close()

		data := make([]byte, len(content))
		n, err := file.Read(data)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if n != len(content) {
			t.Errorf("expected to read %d bytes, got %d", len(content), n)
		}
	})

	t.Run("MkdirAll", func(t *testing.T) {
		path := filepath.Join(tempDir, "a", "b", "c")
		if err := fs.MkdirAll(path, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		info, err := fs.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		path := filepath.Join(tempDir, "remove_test.txt")
		if err := fs.WriteFile(path, []byte("delete me"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if err := fs.Remove(path); err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		if _, err := fs.Stat(path); !os.IsNotExist(err) {
			t.Error("file should not exist after Remove")
		}
	})

	t.Run("RemoveAll", func(t *testing.T) {
		dir := filepath.Join(tempDir, "removeall_test")
		if err := fs.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		if err := fs.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if err := fs.RemoveAll(dir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}

		if _, err := fs.Stat(dir); !os.IsNotExist(err) {
			t.Error("directory should not exist after RemoveAll")
		}
	})

	t.Run("UserHomeDir", func(t *testing.T) {
		home, err := fs.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir failed: %v", err)
		}
		if home == "" {
			t.Error("home directory should not be empty")
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Exists", func(t *testing.T) {
		path := filepath.Join(tempDir, "exists_test.txt")

		if Exists(path) {
			t.Error("file should not exist yet")
		}

		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if !Exists(path) {
			t.Error("file should exist after creation")
		}
	})

	t.Run("IsFile", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "isfile_test.txt")
		dirPath := filepath.Join(tempDir, "isfile_dir")

		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		if !IsFile(filePath) {
			t.Error("filePath should be a file")
		}
		if IsFile(dirPath) {
			t.Error("dirPath should not be a file")
		}
		if IsFile(filepath.Join(tempDir, "nonexistent")) {
			t.Error("nonexistent path should return false")
		}
	})

	t.Run("IsDir", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "isdir_test.txt")
		dirPath := filepath.Join(tempDir, "isdir_dir")

		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		if IsDir(filePath) {
			t.Error("filePath should not be a directory")
		}
		if !IsDir(dirPath) {
			t.Error("dirPath should be a directory")
		}
		if IsDir(filepath.Join(tempDir, "nonexistent")) {
			t.Error("nonexistent path should return false")
		}
	})

	t.Run("EnsureDir", func(t *testing.T) {
		path := filepath.Join(tempDir, "ensure", "nested", "dir")

		if err := EnsureDir(path); err != nil {
			t.Fatalf("EnsureDir failed: %v", err)
		}

		if !IsDir(path) {
			t.Error("directory should exist after EnsureDir")
		}

		// Should be idempotent
		if err := EnsureDir(path); err != nil {
			t.Fatalf("EnsureDir second call failed: %v", err)
		}
	})

	t.Run("CopyFile", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "copy_src.txt")
		dstPath := filepath.Join(tempDir, "copy_dst.txt")
		content := []byte("content to copy")

		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if err := CopyFile(srcPath, dstPath); err != nil {
			t.Fatalf("CopyFile failed: %v", err)
		}

		data, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(data) != string(content) {
			t.Errorf("expected %q, got %q", string(content), string(data))
		}
	})

	t.Run("AppDataDir", func(t *testing.T) {
		home, _ := os.UserHomeDir()

		dir, err := AppDataDir(".testapp", "")
		if err != nil {
			t.Fatalf("AppDataDir failed: %v", err)
		}
		expected := filepath.Join(home, ".testapp")
		if dir != expected {
			t.Errorf("expected %q, got %q", expected, dir)
		}

		dir, err = AppDataDir(".testapp", "subdir")
		if err != nil {
			t.Fatalf("AppDataDir with subdir failed: %v", err)
		}
		expected = filepath.Join(home, ".testapp", "subdir")
		if dir != expected {
			t.Errorf("expected %q, got %q", expected, dir)
		}
	})
}

func TestDefault(t *testing.T) {
	if Default == nil {
		t.Error("Default FileSystem should not be nil")
	}

	_, ok := Default.(*OSFileSystem)
	if !ok {
		t.Error("Default should be an *OSFileSystem")
	}
}
