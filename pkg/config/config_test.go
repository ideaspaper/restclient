package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PreviewOption != "full" {
		t.Errorf("PreviewOption should be 'full', got %s", cfg.PreviewOption)
	}
	if !cfg.ShowColors {
		t.Error("ShowColors should be true by default")
	}
}

func TestLoadConfigFromDir_NoFile(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Load config from empty directory should return defaults
	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("config should not be nil")
	}
	if cfg.PreviewOption != "full" {
		t.Error("PreviewOption should be 'full' by default")
	}
	if !cfg.ShowColors {
		t.Error("ShowColors should be true by default")
	}
}

func TestLoadConfigFromDir_WithFile(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a config file
	configContent := `{
		"previewOption": "headers",
		"showColors": false
	}`
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if cfg.PreviewOption != "headers" {
		t.Errorf("PreviewOption should be 'headers', got %s", cfg.PreviewOption)
	}
	if cfg.ShowColors {
		t.Error("ShowColors should be false")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "restclient-config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `{
		"previewOption": "body",
		"showColors": true
	}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadConfigFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfigFromFile failed: %v", err)
	}

	if cfg.PreviewOption != "body" {
		t.Errorf("PreviewOption should be 'body', got %s", cfg.PreviewOption)
	}
	if !cfg.ShowColors {
		t.Error("ShowColors should be true")
	}
}

func TestConfigSave(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.configPath = filepath.Join(tmpDir, "config.json")
	cfg.PreviewOption = "exchange"
	cfg.ShowColors = false

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	loadedCfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if loadedCfg.PreviewOption != "exchange" {
		t.Errorf("PreviewOption should be 'exchange', got %s", loadedCfg.PreviewOption)
	}
	if loadedCfg.ShowColors {
		t.Error("ShowColors should be false")
	}
}

func TestExportToJSON(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	jsonStr, err := cfg.ExportToJSON()
	if err != nil {
		t.Fatalf("ExportToJSON failed: %v", err)
	}

	// Should be valid JSON (starts with { and ends with })
	if jsonStr == "" {
		t.Error("ExportToJSON should return non-empty string")
	}
	if len(jsonStr) < 2 || jsonStr[0] != '{' || jsonStr[len(jsonStr)-1] != '}' {
		t.Errorf("ExportToJSON should return valid JSON object, got: %s", jsonStr)
	}
}

func TestReload(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "restclient-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write initial config
	configContent := `{"previewOption": "headers"}`
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if cfg.PreviewOption != "headers" {
		t.Errorf("PreviewOption should be 'headers', got %s", cfg.PreviewOption)
	}

	// Update config file
	newContent := `{"previewOption": "body"}`
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write updated config: %v", err)
	}

	// Reload
	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if cfg.PreviewOption != "body" {
		t.Errorf("PreviewOption should be 'body' after reload, got %s", cfg.PreviewOption)
	}
}

func TestReload_NoViper(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Reload(); err == nil {
		t.Error("Reload should fail when viper is not initialized")
	}
}
