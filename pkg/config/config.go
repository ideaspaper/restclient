// Package config provides global application configuration management.
// This only contains CLI display preferences. All HTTP behavior settings
// are stored per-session in session config files.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	"github.com/ideaspaper/restclient/internal/paths"
	"github.com/ideaspaper/restclient/pkg/errors"
)

const (
	configFileName = "config"
	configFileType = "json"
)

// Config represents the global application configuration.
// This only contains CLI display preferences. All HTTP behavior settings
// are stored per-session in session config files.
type Config struct {
	// Display settings (CLI preferences)
	PreviewOption string `json:"previewOption" mapstructure:"previewOption"` // full, headers, body, exchange
	ShowColors    bool   `json:"showColors" mapstructure:"showColors"`

	// Internal: viper instance and config path (not serialized)
	v          *viper.Viper `json:"-" mapstructure:"-"`
	configPath string       `json:"-" mapstructure:"-"`
}

// DefaultConfig returns a new config with default values
func DefaultConfig() *Config {
	return &Config{
		PreviewOption: "full",
		ShowColors:    true,
	}
}

// SetViperDefaults sets default values in a viper instance based on DefaultConfig.
// This should be used by cmd/root.go to avoid duplicating default values.
func SetViperDefaults(v *viper.Viper) {
	defaults := DefaultConfig()
	v.SetDefault("previewOption", defaults.PreviewOption)
	v.SetDefault("showColors", defaults.ShowColors)
}

// setDefaults sets default values in viper (internal helper, uses SetViperDefaults)
func setDefaults(v *viper.Viper) {
	SetViperDefaults(v)
}

// LoadConfig loads configuration from the default path using Viper
func LoadConfig() (*Config, error) {
	configDir, err := paths.AppDataDir("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config directory")
	}

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
		return nil, errors.Wrap(err, "failed to read config file")
	}

	// Unmarshal into Config struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to parse config file")
	}

	cfg.v = v
	cfg.configPath = configPath

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
		return nil, errors.Wrap(err, "failed to read config file")
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to parse config file")
	}

	cfg.v = v
	cfg.configPath = filePath

	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	if c.configPath == "" {
		configDir, err := paths.AppDataDir("")
		if err != nil {
			return errors.Wrap(err, "failed to get config directory")
		}
		c.configPath = filepath.Join(configDir, configFileName+"."+configFileType)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	// Update viper with current config values
	if c.v == nil {
		c.v = viper.New()
	}

	c.v.Set("previewOption", c.PreviewOption)
	c.v.Set("showColors", c.ShowColors)

	return c.v.WriteConfigAs(c.configPath)
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
		jsonBytes, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return "", errors.Wrap(err, "failed to marshal config to JSON")
		}
		return string(jsonBytes), nil
	}

	// Fallback to manual JSON marshaling
	return "", errors.NewValidationError("config", "viper not initialized")
}

// Reload reloads the configuration from the config file
func (c *Config) Reload() error {
	if c.v == nil {
		return errors.NewValidationError("config", "viper not initialized")
	}

	if err := c.v.ReadInConfig(); err != nil {
		return errors.Wrap(err, "failed to reload config")
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
