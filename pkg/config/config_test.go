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
	if _, ok := cfg.EnvironmentVariables["$shared"]; !ok {
		t.Error("EnvironmentVariables should contain $shared")
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

func TestGetEnvironment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnvironmentVariables = map[string]map[string]string{
		"$shared": {"sharedVar": "shared-value", "overrideVar": "shared-override"},
		"dev":     {"devVar": "dev-value", "overrideVar": "dev-override"},
	}
	cfg.CurrentEnvironment = "dev"

	env := cfg.GetEnvironment()

	if env["sharedVar"] != "shared-value" {
		t.Errorf("sharedVar should be 'shared-value', got %s", env["sharedVar"])
	}
	if env["devVar"] != "dev-value" {
		t.Errorf("devVar should be 'dev-value', got %s", env["devVar"])
	}
	// Current environment should override shared
	if env["overrideVar"] != "dev-override" {
		t.Errorf("overrideVar should be 'dev-override', got %s", env["overrideVar"])
	}
}

func TestSetEnvironment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnvironmentVariables = map[string]map[string]string{
		"$shared": {},
		"dev":     {},
		"prod":    {},
	}

	// Set valid environment
	if err := cfg.SetEnvironment("dev"); err != nil {
		t.Errorf("SetEnvironment should not fail for valid env: %v", err)
	}
	if cfg.CurrentEnvironment != "dev" {
		t.Errorf("CurrentEnvironment should be 'dev', got %s", cfg.CurrentEnvironment)
	}

	// Set invalid environment
	if err := cfg.SetEnvironment("invalid"); err == nil {
		t.Error("SetEnvironment should fail for invalid env")
	}

	// Set empty (clear) environment
	if err := cfg.SetEnvironment(""); err != nil {
		t.Errorf("SetEnvironment should not fail for empty env: %v", err)
	}
	if cfg.CurrentEnvironment != "" {
		t.Error("CurrentEnvironment should be empty")
	}
}

func TestListEnvironments(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnvironmentVariables = map[string]map[string]string{
		"$shared": {},
		"dev":     {},
		"prod":    {},
		"staging": {},
	}

	envs := cfg.ListEnvironments()

	// Should not include $shared
	for _, env := range envs {
		if env == "$shared" {
			t.Error("ListEnvironments should not include $shared")
		}
	}

	// Should have 3 environments
	if len(envs) != 3 {
		t.Errorf("Expected 3 environments, got %d", len(envs))
	}
}

func TestAddEnvironment(t *testing.T) {
	cfg := DefaultConfig()

	// Add new environment
	if err := cfg.AddEnvironment("test", map[string]string{"var1": "value1"}); err != nil {
		t.Errorf("AddEnvironment failed: %v", err)
	}

	if cfg.EnvironmentVariables["test"]["var1"] != "value1" {
		t.Error("Environment variable not set correctly")
	}

	// Cannot use $shared as name
	if err := cfg.AddEnvironment("$shared", nil); err == nil {
		t.Error("AddEnvironment should fail for $shared")
	}

	// Add with nil vars
	if err := cfg.AddEnvironment("empty", nil); err != nil {
		t.Errorf("AddEnvironment with nil should not fail: %v", err)
	}
	if cfg.EnvironmentVariables["empty"] == nil {
		t.Error("Empty environment should have empty map, not nil")
	}
}

func TestRemoveEnvironment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnvironmentVariables = map[string]map[string]string{
		"$shared": {},
		"dev":     {},
	}
	cfg.CurrentEnvironment = "dev"

	// Remove current environment
	if err := cfg.RemoveEnvironment("dev"); err != nil {
		t.Errorf("RemoveEnvironment failed: %v", err)
	}

	if _, ok := cfg.EnvironmentVariables["dev"]; ok {
		t.Error("dev environment should be removed")
	}
	if cfg.CurrentEnvironment != "" {
		t.Error("CurrentEnvironment should be cleared when removed")
	}

	// Cannot remove $shared
	if err := cfg.RemoveEnvironment("$shared"); err == nil {
		t.Error("RemoveEnvironment should fail for $shared")
	}

	// Cannot remove non-existent
	if err := cfg.RemoveEnvironment("nonexistent"); err == nil {
		t.Error("RemoveEnvironment should fail for non-existent env")
	}
}

func TestSetEnvironmentVariable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnvironmentVariables = map[string]map[string]string{
		"$shared": {},
		"dev":     {},
	}

	// Set variable in existing environment
	if err := cfg.SetEnvironmentVariable("dev", "apiKey", "secret123"); err != nil {
		t.Errorf("SetEnvironmentVariable failed: %v", err)
	}

	if cfg.EnvironmentVariables["dev"]["apiKey"] != "secret123" {
		t.Error("Variable not set correctly")
	}

	// Set in non-existent environment
	if err := cfg.SetEnvironmentVariable("nonexistent", "var", "value"); err == nil {
		t.Error("SetEnvironmentVariable should fail for non-existent env")
	}
}

func TestGetEnvironmentVariable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnvironmentVariables = map[string]map[string]string{
		"dev": {"apiKey": "secret123"},
	}

	// Get existing variable
	val, exists := cfg.GetEnvironmentVariable("dev", "apiKey")
	if !exists {
		t.Error("Variable should exist")
	}
	if val != "secret123" {
		t.Errorf("Expected 'secret123', got %s", val)
	}

	// Get non-existent variable
	_, exists = cfg.GetEnvironmentVariable("dev", "nonexistent")
	if exists {
		t.Error("Non-existent variable should not exist")
	}

	// Get from non-existent environment
	_, exists = cfg.GetEnvironmentVariable("nonexistent", "apiKey")
	if exists {
		t.Error("Variable from non-existent env should not exist")
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
