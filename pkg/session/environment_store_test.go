package session

import (
	"path/filepath"
	"testing"

	"github.com/ideaspaper/restclient/internal/filesystem"
)

func TestLoadOrCreateEnvironmentStore_CreatesDefaults(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	sessionDir := "/sessions/test"

	store, err := LoadOrCreateEnvironmentStore(fs, sessionDir)
	if err != nil {
		t.Fatalf("LoadOrCreateEnvironmentStore() error = %v", err)
	}

	if store.Version != environmentStoreVersion {
		t.Errorf("Version = %d, want %d", store.Version, environmentStoreVersion)
	}

	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		t.Fatal("$shared environment should exist")
	}

	if _, ok := store.EnvironmentVariables["development"]; !ok {
		t.Fatal("development environment should exist by default")
	}

	path := filepath.Join(sessionDir, environmentStoreFileName)
	if !fs.HasFile(path) {
		t.Fatalf("expected %s to be created", path)
	}
}

func TestEnvironmentStore_LoadExisting(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	sessionDir := "/sessions/existing"

	data := []byte(`{
		"version": 1,
		"environmentVariables": {
			"$shared": {"token": "abc"},
			"prod": {"url": "https://api"}
		}
	}`)
	fs.WithFile(filepath.Join(sessionDir, environmentStoreFileName), data)

	store, err := LoadOrCreateEnvironmentStore(fs, sessionDir)
	if err != nil {
		t.Fatalf("LoadOrCreateEnvironmentStore() error = %v", err)
	}

	if !store.HasEnvironment("prod") {
		t.Fatal("expected prod environment to exist")
	}

	merged := store.GetEnvironment("prod")
	if merged["token"] != "abc" || merged["url"] != "https://api" {
		t.Fatalf("unexpected merged environment: %+v", merged)
	}
}

func TestEnvironmentStore_Save(t *testing.T) {
	fs := filesystem.NewMockFileSystem()
	sessionDir := "/sessions/save"

	store := defaultEnvironmentStore()
	store.EnvironmentVariables["staging"] = map[string]string{"url": "https://staging"}

	if err := SaveEnvironmentStore(fs, sessionDir, store); err != nil {
		t.Fatalf("SaveEnvironmentStore() error = %v", err)
	}

	path := filepath.Join(sessionDir, environmentStoreFileName)
	if !fs.HasFile(path) {
		t.Fatalf("expected %s to exist", path)
	}
}

func TestEnvironmentStore_Mutations(t *testing.T) {
	store := defaultEnvironmentStore()

	if err := store.AddEnvironment("staging", map[string]string{"url": "https://staging"}); err != nil {
		t.Fatalf("AddEnvironment() error = %v", err)
	}

	if !store.HasEnvironment("staging") {
		t.Fatal("staging environment should exist")
	}

	if err := store.SetVariable("staging", "token", "abc"); err != nil {
		t.Fatalf("SetVariable() error = %v", err)
	}

	if val, ok := store.GetVariable("staging", "token"); !ok || val != "abc" {
		t.Fatalf("GetVariable() = %v,%v, want abc,true", val, ok)
	}

	if err := store.UnsetVariable("staging", "token"); err != nil {
		t.Fatalf("UnsetVariable() error = %v", err)
	}

	if err := store.RemoveEnvironment("staging"); err != nil {
		t.Fatalf("RemoveEnvironment() error = %v", err)
	}

	if store.HasEnvironment("staging") {
		t.Fatal("staging should be removed")
	}
}
