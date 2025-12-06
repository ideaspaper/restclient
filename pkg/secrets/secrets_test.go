package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore()

	if store == nil {
		t.Fatal("NewStore should not return nil")
	}

	if store.EnvironmentVariables == nil {
		t.Fatal("EnvironmentVariables should be initialized")
	}

	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		t.Error("$shared environment should be initialized")
	}
}

func TestLoadFromDir_NoFile(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-secrets-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Load from empty directory should return empty store
	store, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	if store == nil {
		t.Fatal("store should not be nil")
	}

	if store.EnvironmentVariables == nil {
		t.Error("EnvironmentVariables should be initialized")
	}

	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		t.Error("$shared environment should be initialized")
	}

	expectedPath := filepath.Join(tmpDir, "secrets.json")
	if store.path != expectedPath {
		t.Errorf("path should be %s, got %s", expectedPath, store.path)
	}
}

func TestLoadFromDir_WithFile(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-secrets-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a secrets file
	secretsContent := `{
		"environmentVariables": {
			"$shared": {"API_KEY": "shared-key"},
			"dev": {"BASE_URL": "http://localhost:8080"},
			"prod": {"BASE_URL": "https://api.example.com"}
		}
	}`
	secretsPath := filepath.Join(tmpDir, "secrets.json")
	if err := os.WriteFile(secretsPath, []byte(secretsContent), 0600); err != nil {
		t.Fatalf("failed to write secrets file: %v", err)
	}

	store, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	if store.EnvironmentVariables["$shared"]["API_KEY"] != "shared-key" {
		t.Error("shared API_KEY not loaded correctly")
	}

	if store.EnvironmentVariables["dev"]["BASE_URL"] != "http://localhost:8080" {
		t.Error("dev BASE_URL not loaded correctly")
	}

	if store.EnvironmentVariables["prod"]["BASE_URL"] != "https://api.example.com" {
		t.Error("prod BASE_URL not loaded correctly")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "restclient-secrets-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	secretsContent := `{
		"environmentVariables": {
			"$shared": {},
			"test": {"TOKEN": "test-token"}
		}
	}`
	if _, err := tmpFile.WriteString(secretsContent); err != nil {
		t.Fatalf("failed to write secrets: %v", err)
	}
	tmpFile.Close()

	store, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if store.EnvironmentVariables["test"]["TOKEN"] != "test-token" {
		t.Error("test TOKEN not loaded correctly")
	}
}

func TestLoadFromFile_NoFile(t *testing.T) {
	// Try to load non-existent file
	store, err := LoadFromFile("/non/existent/path/secrets.json")
	if err != nil {
		t.Fatalf("LoadFromFile should not fail for non-existent file: %v", err)
	}

	if store == nil {
		t.Fatal("store should not be nil")
	}

	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		t.Error("$shared environment should be initialized")
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "restclient-secrets-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write invalid JSON
	if _, err := tmpFile.WriteString("not valid json"); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
	tmpFile.Close()

	_, err = LoadFromFile(tmpFile.Name())
	if err == nil {
		t.Error("LoadFromFile should fail for invalid JSON")
	}
}

func TestStoreSave(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-secrets-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore()
	store.path = filepath.Join(tmpDir, "secrets.json")
	store.EnvironmentVariables["dev"] = map[string]string{"KEY": "value"}

	if err := store.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(store.path); os.IsNotExist(err) {
		t.Error("secrets file should exist after save")
	}

	// Verify file permissions (owner read/write only)
	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	// On Unix, check for 0600 permissions
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions should be 0600, got %o", info.Mode().Perm())
	}

	// Reload and verify
	loadedStore, err := LoadFromFile(store.path)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if loadedStore.EnvironmentVariables["dev"]["KEY"] != "value" {
		t.Error("saved data not loaded correctly")
	}
}

func TestStoreSave_CreatesDirectory(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-secrets-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use a nested path that doesn't exist
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "secrets.json")

	store := NewStore()
	store.path = nestedPath

	if err := store.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("secrets file should exist after save")
	}
}

func TestGetEnvironment(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["$shared"] = map[string]string{
		"SHARED_VAR": "shared-value",
		"OVERRIDE":   "shared-override",
	}
	store.EnvironmentVariables["dev"] = map[string]string{
		"DEV_VAR":  "dev-value",
		"OVERRIDE": "dev-override",
	}

	// Test getting $shared only
	shared := store.GetEnvironment("$shared")
	if shared["SHARED_VAR"] != "shared-value" {
		t.Error("SHARED_VAR should be 'shared-value'")
	}

	// Test getting dev (should include shared vars)
	dev := store.GetEnvironment("dev")
	if dev["SHARED_VAR"] != "shared-value" {
		t.Error("dev should include SHARED_VAR from $shared")
	}
	if dev["DEV_VAR"] != "dev-value" {
		t.Error("DEV_VAR should be 'dev-value'")
	}
	if dev["OVERRIDE"] != "dev-override" {
		t.Error("OVERRIDE should be 'dev-override' (env takes precedence)")
	}

	// Test getting non-existent environment (should return shared vars)
	nonExistent := store.GetEnvironment("nonexistent")
	if nonExistent["SHARED_VAR"] != "shared-value" {
		t.Error("nonexistent env should still return shared vars")
	}

	// Test empty environment name
	empty := store.GetEnvironment("")
	if empty["SHARED_VAR"] != "shared-value" {
		t.Error("empty env should return shared vars")
	}
}

func TestListEnvironments(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{}
	store.EnvironmentVariables["prod"] = map[string]string{}
	store.EnvironmentVariables["staging"] = map[string]string{}

	envs := store.ListEnvironments()

	// Should not include $shared
	for _, env := range envs {
		if env == "$shared" {
			t.Error("ListEnvironments should not include $shared")
		}
	}

	// Should have 3 environments
	if len(envs) != 3 {
		t.Errorf("should have 3 environments, got %d", len(envs))
	}

	// Check all envs are present
	envMap := make(map[string]bool)
	for _, env := range envs {
		envMap[env] = true
	}
	if !envMap["dev"] {
		t.Error("dev should be in list")
	}
	if !envMap["prod"] {
		t.Error("prod should be in list")
	}
	if !envMap["staging"] {
		t.Error("staging should be in list")
	}
}

func TestHasEnvironment(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{}

	if !store.HasEnvironment("$shared") {
		t.Error("should have $shared environment")
	}

	if !store.HasEnvironment("dev") {
		t.Error("should have dev environment")
	}

	if store.HasEnvironment("nonexistent") {
		t.Error("should not have nonexistent environment")
	}
}

func TestAddEnvironment(t *testing.T) {
	store := NewStore()

	// Add new environment
	err := store.AddEnvironment("dev", map[string]string{"KEY": "value"})
	if err != nil {
		t.Fatalf("AddEnvironment failed: %v", err)
	}

	if !store.HasEnvironment("dev") {
		t.Error("dev environment should exist")
	}

	if store.EnvironmentVariables["dev"]["KEY"] != "value" {
		t.Error("KEY should be 'value'")
	}

	// Add environment with nil vars
	err = store.AddEnvironment("empty", nil)
	if err != nil {
		t.Fatalf("AddEnvironment with nil vars failed: %v", err)
	}

	if store.EnvironmentVariables["empty"] == nil {
		t.Error("empty environment should have initialized map")
	}

	// Try to add $shared (should fail)
	err = store.AddEnvironment("$shared", nil)
	if err == nil {
		t.Error("should not be able to add $shared environment")
	}
}

func TestRemoveEnvironment(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{}

	// Remove existing environment
	err := store.RemoveEnvironment("dev")
	if err != nil {
		t.Fatalf("RemoveEnvironment failed: %v", err)
	}

	if store.HasEnvironment("dev") {
		t.Error("dev environment should not exist after removal")
	}

	// Try to remove non-existent environment
	err = store.RemoveEnvironment("nonexistent")
	if err == nil {
		t.Error("should fail when removing non-existent environment")
	}

	// Try to remove $shared (should fail)
	err = store.RemoveEnvironment("$shared")
	if err == nil {
		t.Error("should not be able to remove $shared environment")
	}
}

func TestSetVariable(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{}

	// Set variable in existing environment
	err := store.SetVariable("dev", "KEY", "value")
	if err != nil {
		t.Fatalf("SetVariable failed: %v", err)
	}

	if store.EnvironmentVariables["dev"]["KEY"] != "value" {
		t.Error("KEY should be 'value'")
	}

	// Set variable in $shared
	err = store.SetVariable("$shared", "SHARED_KEY", "shared-value")
	if err != nil {
		t.Fatalf("SetVariable in $shared failed: %v", err)
	}

	if store.EnvironmentVariables["$shared"]["SHARED_KEY"] != "shared-value" {
		t.Error("SHARED_KEY should be 'shared-value'")
	}

	// Try to set variable in non-existent environment
	err = store.SetVariable("nonexistent", "KEY", "value")
	if err == nil {
		t.Error("should fail when setting variable in non-existent environment")
	}
}

func TestGetVariable(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{"KEY": "value"}

	// Get existing variable
	val, exists := store.GetVariable("dev", "KEY")
	if !exists {
		t.Error("KEY should exist")
	}
	if val != "value" {
		t.Errorf("KEY should be 'value', got '%s'", val)
	}

	// Get non-existent variable
	_, exists = store.GetVariable("dev", "NONEXISTENT")
	if exists {
		t.Error("NONEXISTENT should not exist")
	}

	// Get variable from non-existent environment
	_, exists = store.GetVariable("nonexistent", "KEY")
	if exists {
		t.Error("should not find variable in non-existent environment")
	}
}

func TestUnsetVariable(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{"KEY": "value"}

	// Unset existing variable
	err := store.UnsetVariable("dev", "KEY")
	if err != nil {
		t.Fatalf("UnsetVariable failed: %v", err)
	}

	if _, exists := store.EnvironmentVariables["dev"]["KEY"]; exists {
		t.Error("KEY should not exist after unset")
	}

	// Try to unset non-existent variable
	err = store.UnsetVariable("dev", "NONEXISTENT")
	if err == nil {
		t.Error("should fail when unsetting non-existent variable")
	}

	// Try to unset variable in non-existent environment
	err = store.UnsetVariable("nonexistent", "KEY")
	if err == nil {
		t.Error("should fail when unsetting variable in non-existent environment")
	}
}

func TestGetVariables(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["dev"] = map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	// Get variables from existing environment
	vars, ok := store.GetVariables("dev")
	if !ok {
		t.Error("should find dev environment")
	}
	if len(vars) != 2 {
		t.Errorf("should have 2 variables, got %d", len(vars))
	}
	if vars["KEY1"] != "value1" {
		t.Error("KEY1 should be 'value1'")
	}

	// Get variables from non-existent environment
	_, ok = store.GetVariables("nonexistent")
	if ok {
		t.Error("should not find non-existent environment")
	}
}

func TestPath(t *testing.T) {
	store := NewStore()
	store.path = "/some/path/secrets.json"

	if store.Path() != "/some/path/secrets.json" {
		t.Errorf("Path should return '/some/path/secrets.json', got '%s'", store.Path())
	}
}

func TestStoreJSONSerialization(t *testing.T) {
	store := NewStore()
	store.EnvironmentVariables["$shared"] = map[string]string{"SHARED": "value"}
	store.EnvironmentVariables["dev"] = map[string]string{"DEV": "dev-value"}

	// Serialize
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Deserialize
	var loaded Store
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if loaded.EnvironmentVariables["$shared"]["SHARED"] != "value" {
		t.Error("SHARED variable not serialized correctly")
	}
	if loaded.EnvironmentVariables["dev"]["DEV"] != "dev-value" {
		t.Error("DEV variable not serialized correctly")
	}

	// path field should not be serialized
	if loaded.path != "" {
		t.Error("path field should not be serialized")
	}
}

func TestLoadFromDir_InitializesNilMaps(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-secrets-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a secrets file with nil-like empty structure
	secretsContent := `{}`
	secretsPath := filepath.Join(tmpDir, "secrets.json")
	if err := os.WriteFile(secretsPath, []byte(secretsContent), 0600); err != nil {
		t.Fatalf("failed to write secrets file: %v", err)
	}

	store, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	if store.EnvironmentVariables == nil {
		t.Error("EnvironmentVariables should be initialized even from empty JSON")
	}

	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		t.Error("$shared should be initialized even from empty JSON")
	}
}

func TestLoadFromFile_InitializesNilMaps(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "restclient-secrets-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write JSON without $shared
	secretsContent := `{"environmentVariables": {"dev": {"KEY": "value"}}}`
	if _, err := tmpFile.WriteString(secretsContent); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
	tmpFile.Close()

	store, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// $shared should be initialized even if not in file
	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		t.Error("$shared should be initialized even if not in JSON")
	}
}
