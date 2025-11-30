package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	"github.com/ideaspaper/restclient/pkg/client"
)

const (
	configFileName = "config"
	configFileType = "json"
)

// Config represents the application configuration
type Config struct {
	// HTTP client settings
	FollowRedirects bool `json:"followRedirect" mapstructure:"followRedirect"`
	TimeoutMs       int  `json:"timeoutInMilliseconds" mapstructure:"timeoutInMilliseconds"`
	RememberCookies bool `json:"rememberCookiesForSubsequentRequests" mapstructure:"rememberCookiesForSubsequentRequests"`

	// Default headers
	DefaultHeaders map[string]string `json:"defaultHeaders" mapstructure:"defaultHeaders"`

	// Environment variables
	EnvironmentVariables map[string]map[string]string `json:"environmentVariables" mapstructure:"environmentVariables"`

	// Current environment
	CurrentEnvironment string `json:"currentEnvironment" mapstructure:"currentEnvironment"`

	// SSL settings
	InsecureSSL    bool `json:"insecureSSL" mapstructure:"insecureSSL"`
	ProxyStrictSSL bool `json:"proxyStrictSSL" mapstructure:"proxyStrictSSL"`

	// Proxy settings
	Proxy                string   `json:"proxy" mapstructure:"proxy"`
	ExcludeHostsForProxy []string `json:"excludeHostsForProxy" mapstructure:"excludeHostsForProxy"`

	// Certificates
	Certificates map[string]CertificateConfig `json:"certificates" mapstructure:"certificates"`

	// Display settings
	PreviewOption string `json:"previewOption" mapstructure:"previewOption"` // full, headers, body, exchange
	ShowColors    bool   `json:"showColors" mapstructure:"showColors"`

	// Internal: viper instance and config path (not serialized)
	v          *viper.Viper `json:"-" mapstructure:"-"`
	configPath string       `json:"-" mapstructure:"-"`
}

// CertificateConfig holds certificate paths
type CertificateConfig struct {
	Cert       string `json:"cert,omitempty" mapstructure:"cert"`
	Key        string `json:"key,omitempty" mapstructure:"key"`
	PFX        string `json:"pfx,omitempty" mapstructure:"pfx"`
	Passphrase string `json:"passphrase,omitempty" mapstructure:"passphrase"`
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

// setDefaults sets default values in viper
func setDefaults(v *viper.Viper) {
	v.SetDefault("followRedirect", true)
	v.SetDefault("timeoutInMilliseconds", 0)
	v.SetDefault("rememberCookiesForSubsequentRequests", true)
	v.SetDefault("defaultHeaders", map[string]string{
		"User-Agent": "restclient-cli",
	})
	v.SetDefault("environmentVariables", map[string]map[string]string{
		"$shared": {},
	})
	v.SetDefault("currentEnvironment", "")
	v.SetDefault("insecureSSL", false)
	v.SetDefault("proxyStrictSSL", true)
	v.SetDefault("certificates", make(map[string]CertificateConfig))
	v.SetDefault("previewOption", "full")
	v.SetDefault("showColors", true)
}

// LoadConfig loads configuration from the default path using Viper
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".restclient")
	return LoadConfigFromDir(configDir)
}

// LoadConfigFromDir loads configuration from a specific directory using Viper
func LoadConfigFromDir(dir string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure Viper
	v.SetConfigName(configFileName)
	v.SetConfigType(configFileType)
	v.AddConfigPath(dir)

	// Allow environment variable overrides with prefix RESTCLIENT_
	v.SetEnvPrefix("RESTCLIENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	configPath := filepath.Join(dir, configFileName+"."+configFileType)

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
			cfg := DefaultConfig()
			cfg.v = v
			cfg.configPath = configPath
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into Config struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.v = v
	cfg.configPath = configPath

	// Ensure maps are initialized
	if cfg.DefaultHeaders == nil {
		cfg.DefaultHeaders = make(map[string]string)
	}
	if cfg.EnvironmentVariables == nil {
		cfg.EnvironmentVariables = make(map[string]map[string]string)
	}
	if cfg.Certificates == nil {
		cfg.Certificates = make(map[string]CertificateConfig)
	}

	return cfg, nil
}

// LoadConfigFromFile loads configuration from a specific file path
func LoadConfigFromFile(filePath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	v.SetConfigFile(filePath)

	// Allow environment variable overrides
	v.SetEnvPrefix("RESTCLIENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			cfg := DefaultConfig()
			cfg.v = v
			cfg.configPath = filePath
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.v = v
	cfg.configPath = filePath

	// Ensure maps are initialized
	if cfg.DefaultHeaders == nil {
		cfg.DefaultHeaders = make(map[string]string)
	}
	if cfg.EnvironmentVariables == nil {
		cfg.EnvironmentVariables = make(map[string]map[string]string)
	}
	if cfg.Certificates == nil {
		cfg.Certificates = make(map[string]CertificateConfig)
	}

	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	if c.configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		c.configPath = filepath.Join(homeDir, ".restclient", configFileName+"."+configFileType)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Update viper with current config values
	if c.v == nil {
		c.v = viper.New()
	}

	c.v.Set("followRedirect", c.FollowRedirects)
	c.v.Set("timeoutInMilliseconds", c.TimeoutMs)
	c.v.Set("rememberCookiesForSubsequentRequests", c.RememberCookies)
	c.v.Set("defaultHeaders", c.DefaultHeaders)
	c.v.Set("environmentVariables", c.EnvironmentVariables)
	c.v.Set("currentEnvironment", c.CurrentEnvironment)
	c.v.Set("insecureSSL", c.InsecureSSL)
	c.v.Set("proxyStrictSSL", c.ProxyStrictSSL)
	c.v.Set("proxy", c.Proxy)
	c.v.Set("excludeHostsForProxy", c.ExcludeHostsForProxy)
	c.v.Set("certificates", c.Certificates)
	c.v.Set("previewOption", c.PreviewOption)
	c.v.Set("showColors", c.ShowColors)

	return c.v.WriteConfigAs(c.configPath)
}

// GetEnvironment returns the environment variables for the current environment
func (c *Config) GetEnvironment() map[string]string {
	result := make(map[string]string)

	// Copy shared environment first
	if shared, ok := c.EnvironmentVariables["$shared"]; ok {
		maps.Copy(result, shared)
	}

	// Overlay current environment
	if c.CurrentEnvironment != "" {
		if env, ok := c.EnvironmentVariables[c.CurrentEnvironment]; ok {
			maps.Copy(result, env)
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
		cfg.Timeout = time.Duration(c.TimeoutMs) * time.Millisecond
	}

	maps.Copy(cfg.DefaultHeaders, c.DefaultHeaders)

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

// GetViper returns the underlying viper instance
func (c *Config) GetViper() *viper.Viper {
	return c.v
}

// BindFlags binds cobra flags to viper configuration
func (c *Config) BindFlags(v *viper.Viper) {
	c.v = v
}

// ExportToJSON exports the config as JSON string
func (c *Config) ExportToJSON() (string, error) {
	// Use viper's AllSettings for a complete export
	if c.v != nil {
		settings := c.v.AllSettings()
		return fmt.Sprintf("%v", settings), nil
	}

	// Fallback to manual JSON marshaling
	return "", fmt.Errorf("viper not initialized")
}

// Reload reloads the configuration from the config file
func (c *Config) Reload() error {
	if c.v == nil {
		return fmt.Errorf("viper not initialized")
	}

	if err := c.v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	return c.v.Unmarshal(c)
}

// WatchConfig enables live config reloading
func (c *Config) WatchConfig(onChange func()) {
	if c.v != nil {
		c.v.OnConfigChange(func(e fsnotify.Event) {
			// Reload config on change
			c.v.Unmarshal(c)
			if onChange != nil {
				onChange()
			}
		})
		c.v.WatchConfig()
	}
}
