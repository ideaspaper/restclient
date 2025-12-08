package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/pkg/client"
	"github.com/ideaspaper/restclient/pkg/errors"
)

const (
	sessionConfigFileName = "config.json"
	sessionConfigVersion  = 1
)

// SessionConfig represents session-scoped configuration that travels with a session directory.
type SessionConfig struct {
	Version     int                     `json:"version"`
	Environment SessionEnvironmentBlock `json:"environment"`
	HTTP        SessionHTTPBlock        `json:"http"`
	TLS         SessionTLSBlock         `json:"tls"`
}

// SessionEnvironmentBlock contains environment-specific settings.
type SessionEnvironmentBlock struct {
	Current         string            `json:"current"`
	RememberCookies bool              `json:"rememberCookiesForSubsequentRequests"`
	DefaultHeaders  map[string]string `json:"defaultHeaders"`
}

// SessionHTTPBlock controls HTTP client behaviour for a session.
type SessionHTTPBlock struct {
	TimeoutMs      int    `json:"timeoutInMilliseconds"`
	FollowRedirect bool   `json:"followRedirect"`
	CookieJar      string `json:"cookieJar"`
}

// SessionTLSBlock mirrors TLS/proxy options at the session scope.
type SessionTLSBlock struct {
	InsecureSSL          bool                               `json:"insecureSSL"`
	Proxy                string                             `json:"proxy"`
	ExcludeHostsForProxy []string                           `json:"excludeHostsForProxy"`
	Certificates         map[string]SessionCertificateBlock `json:"certificates"`
}

// SessionCertificateBlock stores certificate references for a host.
type SessionCertificateBlock struct {
	Cert       string `json:"cert,omitempty"`
	Key        string `json:"key,omitempty"`
	PFX        string `json:"pfx,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// LoadOrCreateSessionConfig loads an existing session config or recreates it with defaults.
func LoadOrCreateSessionConfig(fs filesystem.FileSystem, sessionDir string) (*SessionConfig, error) {
	if fs == nil {
		fs = filesystem.Default
	}
	if sessionDir == "" {
		return nil, errors.NewValidationError("sessionDir", "session directory is required")
	}

	if err := fs.MkdirAll(sessionDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to ensure session directory")
	}

	configPath := filepath.Join(sessionDir, sessionConfigFileName)
	data, err := fs.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultSessionConfig()
			if err := writeSessionConfig(fs, configPath, cfg); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, errors.Wrap(err, "failed to read session config")
	}

	cfg := &SessionConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		cfg = defaultSessionConfig()
		if writeErr := writeSessionConfig(fs, configPath, cfg); writeErr != nil {
			return nil, writeErr
		}
		return cfg, nil
	}

	cfg.ensureIntegrity()
	if cfg.Version != sessionConfigVersion {
		cfg.Version = sessionConfigVersion
		if err := writeSessionConfig(fs, configPath, cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// SaveSessionConfig persists the provided session config to disk.
func SaveSessionConfig(fs filesystem.FileSystem, sessionDir string, cfg *SessionConfig) error {
	if fs == nil {
		fs = filesystem.Default
	}
	if sessionDir == "" {
		return errors.NewValidationError("sessionDir", "session directory is required")
	}
	if cfg == nil {
		return errors.NewValidationError("config", "session config is required")
	}

	cfg.ensureIntegrity()
	cfg.Version = sessionConfigVersion

	if err := fs.MkdirAll(sessionDir, 0755); err != nil {
		return errors.Wrap(err, "failed to ensure session directory")
	}

	configPath := filepath.Join(sessionDir, sessionConfigFileName)
	return writeSessionConfig(fs, configPath, cfg)
}

func defaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		Version: sessionConfigVersion,
		Environment: SessionEnvironmentBlock{
			Current:         "",
			RememberCookies: true,
			DefaultHeaders: map[string]string{
				"User-Agent": "restclient-cli",
			},
		},
		HTTP: SessionHTTPBlock{
			TimeoutMs:      0,
			FollowRedirect: true,
			CookieJar:      cookiesFileName,
		},
		TLS: SessionTLSBlock{
			InsecureSSL:          false,
			Proxy:                "",
			ExcludeHostsForProxy: []string{},
			Certificates:         make(map[string]SessionCertificateBlock),
		},
	}
}

// DefaultSessionConfig returns a new session config with default values (exported for testing/fallback)
func DefaultSessionConfig() *SessionConfig {
	return defaultSessionConfig()
}

func (c *SessionConfig) ensureIntegrity() {
	if c.Environment.DefaultHeaders == nil {
		c.Environment.DefaultHeaders = make(map[string]string)
	}
	if c.TLS.Certificates == nil {
		c.TLS.Certificates = make(map[string]SessionCertificateBlock)
	}
	if c.HTTP.CookieJar == "" {
		c.HTTP.CookieJar = cookiesFileName
	}
}

func writeSessionConfig(fs filesystem.FileSystem, path string, cfg *SessionConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal session config")
	}
	if err := fs.WriteFile(path, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write session config")
	}
	return nil
}

// ToClientConfig converts SessionConfig to client.ClientConfig for HTTP requests
func (c *SessionConfig) ToClientConfig() *client.ClientConfig {
	cfg := client.DefaultConfig()

	cfg.FollowRedirects = c.HTTP.FollowRedirect
	cfg.InsecureSSL = c.TLS.InsecureSSL
	cfg.RememberCookies = c.Environment.RememberCookies
	cfg.Proxy = c.TLS.Proxy
	cfg.ExcludeProxy = c.TLS.ExcludeHostsForProxy

	if c.HTTP.TimeoutMs > 0 {
		cfg.Timeout = time.Duration(c.HTTP.TimeoutMs) * time.Millisecond
	}

	// Merge default headers
	for k, v := range c.Environment.DefaultHeaders {
		cfg.DefaultHeaders[k] = v
	}

	// Convert certificates
	for host, cert := range c.TLS.Certificates {
		cfg.Certificates[host] = client.Certificate{
			Cert:       cert.Cert,
			Key:        cert.Key,
			PFX:        cert.PFX,
			Passphrase: cert.Passphrase,
		}
	}

	return cfg
}

// CurrentEnvironment returns the current environment name
func (c *SessionConfig) CurrentEnvironment() string {
	return c.Environment.Current
}

// SetCurrentEnvironment sets the current environment name
func (c *SessionConfig) SetCurrentEnvironment(env string) {
	c.Environment.Current = env
}

// RememberCookies returns whether cookies should be persisted
func (c *SessionConfig) RememberCookies() bool {
	return c.Environment.RememberCookies
}

// DefaultHeaders returns the default headers map
func (c *SessionConfig) DefaultHeaders() map[string]string {
	return c.Environment.DefaultHeaders
}
