package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.FollowRedirects {
		t.Error("FollowRedirects should be true by default")
	}
	if cfg.TimeoutMs != 0 {
		t.Errorf("TimeoutMs should be 0, got %d", cfg.TimeoutMs)
	}
	if !cfg.RememberCookies {
		t.Error("RememberCookies should be true by default")
	}
	if cfg.InsecureSSL {
		t.Error("InsecureSSL should be false by default")
	}
	if !cfg.ProxyStrictSSL {
		t.Error("ProxyStrictSSL should be true by default")
	}
	if cfg.PreviewOption != "full" {
		t.Errorf("PreviewOption should be 'full', got %s", cfg.PreviewOption)
	}
	if !cfg.ShowColors {
		t.Error("ShowColors should be true by default")
	}
	if cfg.DefaultHeaders["User-Agent"] != "restclient-cli" {
		t.Errorf("User-Agent should be 'restclient-cli', got %s", cfg.DefaultHeaders["User-Agent"])
	}
	if cfg.CurrentEnvironment != "" {
		t.Errorf("CurrentEnvironment should be empty by default, got %s", cfg.CurrentEnvironment)
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
	if !cfg.FollowRedirects {
		t.Error("FollowRedirects should be true by default")
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
		"followRedirect": false,
		"timeoutInMilliseconds": 5000,
		"currentEnvironment": "test"
	}`
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if cfg.FollowRedirects {
		t.Error("FollowRedirects should be false")
	}
	if cfg.TimeoutMs != 5000 {
		t.Errorf("TimeoutMs should be 5000, got %d", cfg.TimeoutMs)
	}
	if cfg.CurrentEnvironment != "test" {
		t.Errorf("CurrentEnvironment should be 'test', got %s", cfg.CurrentEnvironment)
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
		"insecureSSL": true,
		"proxy": "http://proxy.example.com:8080"
	}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadConfigFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfigFromFile failed: %v", err)
	}

	if !cfg.InsecureSSL {
		t.Error("InsecureSSL should be true")
	}
	if cfg.Proxy != "http://proxy.example.com:8080" {
		t.Errorf("Proxy should be 'http://proxy.example.com:8080', got %s", cfg.Proxy)
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
	cfg.TimeoutMs = 10000
	cfg.CurrentEnvironment = "production"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	loadedCfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if loadedCfg.TimeoutMs != 10000 {
		t.Errorf("TimeoutMs should be 10000, got %d", loadedCfg.TimeoutMs)
	}
	if loadedCfg.CurrentEnvironment != "production" {
		t.Errorf("CurrentEnvironment should be 'production', got %s", loadedCfg.CurrentEnvironment)
	}
}

func TestToClientConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FollowRedirects = false
	cfg.InsecureSSL = true
	cfg.TimeoutMs = 5000
	cfg.Proxy = "http://proxy:8080"
	cfg.ExcludeHostsForProxy = []string{"localhost"}
	cfg.Certificates = map[string]CertificateConfig{
		"example.com": {Cert: "/path/to/cert", Key: "/path/to/key"},
	}

	clientCfg := cfg.ToClientConfig()

	if clientCfg.FollowRedirects {
		t.Error("FollowRedirects should be false")
	}
	if !clientCfg.InsecureSSL {
		t.Error("InsecureSSL should be true")
	}
	if clientCfg.Timeout.Milliseconds() != 5000 {
		t.Errorf("Timeout should be 5000ms, got %d", clientCfg.Timeout.Milliseconds())
	}
	if clientCfg.Proxy != "http://proxy:8080" {
		t.Errorf("Proxy should be 'http://proxy:8080', got %s", clientCfg.Proxy)
	}
	if len(clientCfg.ExcludeProxy) != 1 || clientCfg.ExcludeProxy[0] != "localhost" {
		t.Error("ExcludeProxy not set correctly")
	}
	if clientCfg.Certificates["example.com"].Cert != "/path/to/cert" {
		t.Error("Certificates not copied correctly")
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
	configContent := `{"timeoutInMilliseconds": 1000}`
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	if cfg.TimeoutMs != 1000 {
		t.Errorf("TimeoutMs should be 1000, got %d", cfg.TimeoutMs)
	}

	// Update config file
	newContent := `{"timeoutInMilliseconds": 2000}`
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write updated config: %v", err)
	}

	// Reload
	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if cfg.TimeoutMs != 2000 {
		t.Errorf("TimeoutMs should be 2000 after reload, got %d", cfg.TimeoutMs)
	}
}

func TestReload_NoViper(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Reload(); err == nil {
		t.Error("Reload should fail when viper is not initialized")
	}
}
