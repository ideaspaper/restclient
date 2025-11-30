package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"restclient/pkg/client"
)

const configFileName = "config.json"

// Config represents the application configuration
type Config struct {
	// HTTP client settings
	FollowRedirects bool `json:"followRedirect"`
	TimeoutMs       int  `json:"timeoutInMilliseconds"`
	RememberCookies bool `json:"rememberCookiesForSubsequentRequests"`

	// Default headers
	DefaultHeaders map[string]string `json:"defaultHeaders"`

	// Environment variables
	EnvironmentVariables map[string]map[string]string `json:"environmentVariables"`

	// Current environment
	CurrentEnvironment string `json:"currentEnvironment"`

	// SSL settings
	InsecureSSL    bool `json:"insecureSSL"`
	ProxyStrictSSL bool `json:"proxyStrictSSL"`

	// Proxy settings
	Proxy                string   `json:"proxy"`
	ExcludeHostsForProxy []string `json:"excludeHostsForProxy"`

	// Certificates
	Certificates map[string]CertificateConfig `json:"certificates"`

	// Display settings
	PreviewOption string `json:"previewOption"` // full, headers, body, exchange
	ShowColors    bool   `json:"showColors"`

	// File path (not serialized)
	configPath string `json:"-"`
}

// CertificateConfig holds certificate paths
type CertificateConfig struct {
	Cert       string `json:"cert,omitempty"`
	Key        string `json:"key,omitempty"`
	PFX        string `json:"pfx,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// DefaultConfig returns a new config with default values
func DefaultConfig() *Config {
	return &Config{
		FollowRedirects: true,
		TimeoutMs:       0,
		RememberCookies: true,
		DefaultHeaders: map[string]string{
			"User-Agent": "restclient-cli",
		},
		EnvironmentVariables: map[string]map[string]string{
			"$shared": {},
		},
		CurrentEnvironment: "",
		InsecureSSL:        false,
		ProxyStrictSSL:     true,
		Certificates:       make(map[string]CertificateConfig),
		PreviewOption:      "full",
		ShowColors:         true,
	}
}

// LoadConfig loads configuration from the default path
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".restclient")
	return LoadConfigFromDir(configDir)
}

// LoadConfigFromDir loads configuration from a specific directory
func LoadConfigFromDir(dir string) (*Config, error) {
	configPath := filepath.Join(dir, configFileName)

	// If config doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		cfg.configPath = configPath
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.configPath = configPath
	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	if c.configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		c.configPath = filepath.Join(homeDir, ".restclient", configFileName)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(c.configPath, data, 0644)
}

// GetEnvironment returns the environment variables for the current environment
func (c *Config) GetEnvironment() map[string]string {
	result := make(map[string]string)

	// Copy shared environment first
	if shared, ok := c.EnvironmentVariables["$shared"]; ok {
		for k, v := range shared {
			result[k] = v
		}
	}

	// Overlay current environment
	if c.CurrentEnvironment != "" {
		if env, ok := c.EnvironmentVariables[c.CurrentEnvironment]; ok {
			for k, v := range env {
				result[k] = v
			}
		}
	}

	return result
}

// SetEnvironment sets the current environment
func (c *Config) SetEnvironment(name string) error {
	if name != "" {
		if _, ok := c.EnvironmentVariables[name]; !ok && name != "$shared" {
			return fmt.Errorf("environment '%s' not found", name)
		}
	}
	c.CurrentEnvironment = name
	return nil
}

// ListEnvironments returns a list of available environments
func (c *Config) ListEnvironments() []string {
	var envs []string
	for name := range c.EnvironmentVariables {
		if name != "$shared" {
			envs = append(envs, name)
		}
	}
	return envs
}

// AddEnvironment adds a new environment
func (c *Config) AddEnvironment(name string, vars map[string]string) error {
	if name == "$shared" {
		return fmt.Errorf("cannot use reserved name '$shared'")
	}
	if vars == nil {
		vars = make(map[string]string)
	}
	c.EnvironmentVariables[name] = vars
	return nil
}

// RemoveEnvironment removes an environment
func (c *Config) RemoveEnvironment(name string) error {
	if name == "$shared" {
		return fmt.Errorf("cannot remove shared environment")
	}
	if _, ok := c.EnvironmentVariables[name]; !ok {
		return fmt.Errorf("environment '%s' not found", name)
	}
	delete(c.EnvironmentVariables, name)
	if c.CurrentEnvironment == name {
		c.CurrentEnvironment = ""
	}
	return nil
}

// SetEnvironmentVariable sets a variable in an environment
func (c *Config) SetEnvironmentVariable(env, name, value string) error {
	if _, ok := c.EnvironmentVariables[env]; !ok {
		return fmt.Errorf("environment '%s' not found", env)
	}
	c.EnvironmentVariables[env][name] = value
	return nil
}

// GetEnvironmentVariable gets a variable from an environment
func (c *Config) GetEnvironmentVariable(env, name string) (string, bool) {
	if envVars, ok := c.EnvironmentVariables[env]; ok {
		val, exists := envVars[name]
		return val, exists
	}
	return "", false
}

// ToClientConfig converts Config to client.ClientConfig
func (c *Config) ToClientConfig() *client.ClientConfig {
	cfg := client.DefaultConfig()

	cfg.FollowRedirects = c.FollowRedirects
	cfg.InsecureSSL = c.InsecureSSL
	cfg.RememberCookies = c.RememberCookies
	cfg.Proxy = c.Proxy
	cfg.ExcludeProxy = c.ExcludeHostsForProxy

	if c.TimeoutMs > 0 {
		cfg.Timeout = 0 // Will be set from TimeoutMs
	}

	for k, v := range c.DefaultHeaders {
		cfg.DefaultHeaders[k] = v
	}

	for host, cert := range c.Certificates {
		cfg.Certificates[host] = client.Certificate{
			Cert:       cert.Cert,
			Key:        cert.Key,
			PFX:        cert.PFX,
			Passphrase: cert.Passphrase,
		}
	}

	return cfg
}

// LoadOrCreateConfig loads existing config or creates a new one with defaults
func LoadOrCreateConfig() (*Config, error) {
	cfg, err := LoadConfig()
	if err != nil {
		// Return default config if loading fails
		return DefaultConfig(), nil
	}
	return cfg, nil
}

// ExportToJSON exports the config as JSON string
func (c *Config) ExportToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportFromJSON imports config from JSON string
func (c *Config) ImportFromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), c)
}
