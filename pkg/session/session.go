// Package session provides session management for persisting cookies and
// variables across HTTP requests, supporting both directory-scoped and
// named session modes.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/internal/paths"
	"github.com/ideaspaper/restclient/pkg/errors"
)

const (
	sessionDirName  = "session"
	dirSessionsDir  = "dirs"
	namedSessionDir = "named"
	cookiesFileName = "cookies.json"
	varsFileName    = "variables.json"
)

// Cookie represents a serializable cookie
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	MaxAge   int       `json:"maxAge,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HttpOnly bool      `json:"httpOnly,omitempty"`
	SameSite string    `json:"sameSite,omitempty"`
}

// SessionManager manages session persistence for cookies and variables
type SessionManager struct {
	fs          filesystem.FileSystem
	baseDir     string
	sessionPath string
	cookies     map[string][]Cookie // host -> cookies
	variables   map[string]any      // variable name -> value
}

// NewSessionManager creates a new session manager
// If sessionName is provided, uses named session; otherwise uses directory-based session
func NewSessionManager(baseDir, httpFilePath, sessionName string) (*SessionManager, error) {
	return NewSessionManagerWithFS(filesystem.Default, baseDir, httpFilePath, sessionName)
}

// NewSessionManagerWithFS creates a new session manager with a custom file system.
// This is primarily useful for testing.
func NewSessionManagerWithFS(fs filesystem.FileSystem, baseDir, httpFilePath, sessionName string) (*SessionManager, error) {
	if fs == nil {
		fs = filesystem.Default
	}

	if baseDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get app data directory")
		}
		baseDir = appDir
	}

	var sessionPath string
	if sessionName != "" {
		sessionPath = filepath.Join(baseDir, sessionDirName, namedSessionDir, sessionName)
	} else if httpFilePath != "" {
		// Use directory of .http file to create a directory-specific session
		dirPath := filepath.Dir(httpFilePath)
		absPath, err := filepath.Abs(dirPath)
		if err != nil {
			absPath = dirPath
		}
		hash := hashPath(absPath)
		sessionPath = filepath.Join(baseDir, sessionDirName, dirSessionsDir, hash)
	} else {
		sessionPath = filepath.Join(baseDir, sessionDirName, namedSessionDir, "default")
	}

	s := &SessionManager{
		fs:          fs,
		baseDir:     baseDir,
		sessionPath: sessionPath,
		cookies:     make(map[string][]Cookie),
		variables:   make(map[string]any),
	}

	return s, nil
}

// hashPath creates a short hash of a path for directory naming
func hashPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8]) // Use first 8 bytes (16 hex chars)
}

// Load loads both cookies and variables from disk
func (s *SessionManager) Load() error {
	if err := s.LoadCookies(); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to load cookies")
	}
	if err := s.LoadVariables(); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to load variables")
	}
	return nil
}

// Save saves both cookies and variables to disk
func (s *SessionManager) Save() error {
	if err := s.SaveCookies(); err != nil {
		return errors.Wrap(err, "failed to save cookies")
	}
	if err := s.SaveVariables(); err != nil {
		return errors.Wrap(err, "failed to save variables")
	}
	return nil
}

// LoadCookies loads cookies from disk
func (s *SessionManager) LoadCookies() error {
	path := filepath.Join(s.sessionPath, cookiesFileName)
	data, err := s.fs.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "failed to read cookies file")
	}

	if err := json.Unmarshal(data, &s.cookies); err != nil {
		return errors.Wrap(err, "failed to parse cookies file")
	}
	return nil
}

// SaveCookies saves cookies to disk
func (s *SessionManager) SaveCookies() error {
	if err := s.fs.MkdirAll(s.sessionPath, 0755); err != nil {
		return errors.Wrap(err, "failed to create session directory")
	}

	// Clean expired cookies before saving
	s.cleanExpiredCookies()

	data, err := json.MarshalIndent(s.cookies, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cookies")
	}

	path := filepath.Join(s.sessionPath, cookiesFileName)
	return s.fs.WriteFile(path, data, 0644)
}

// LoadVariables loads variables from disk
func (s *SessionManager) LoadVariables() error {
	path := filepath.Join(s.sessionPath, varsFileName)
	data, err := s.fs.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "failed to read variables file")
	}

	if err := json.Unmarshal(data, &s.variables); err != nil {
		return errors.Wrap(err, "failed to parse variables file")
	}
	return nil
}

// SaveVariables saves variables to disk
func (s *SessionManager) SaveVariables() error {
	if err := s.fs.MkdirAll(s.sessionPath, 0755); err != nil {
		return errors.Wrap(err, "failed to create session directory")
	}

	data, err := json.MarshalIndent(s.variables, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal variables")
	}

	path := filepath.Join(s.sessionPath, varsFileName)
	return s.fs.WriteFile(path, data, 0644)
}

// GetCookiesForURL returns cookies for a given URL
func (s *SessionManager) GetCookiesForURL(urlStr string) []*http.Cookie {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}

	host := parsedURL.Host
	if h, _, err := splitHostPort(host); err == nil {
		host = h
	}

	storedCookies, ok := s.cookies[host]
	if !ok {
		return nil
	}

	var httpCookies []*http.Cookie
	now := time.Now()

	for _, c := range storedCookies {
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}

		httpCookie := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			MaxAge:   c.MaxAge,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
		}

		// Convert SameSite string to http.SameSite
		switch c.SameSite {
		case "Strict":
			httpCookie.SameSite = http.SameSiteStrictMode
		case "Lax":
			httpCookie.SameSite = http.SameSiteLaxMode
		case "None":
			httpCookie.SameSite = http.SameSiteNoneMode
		}

		httpCookies = append(httpCookies, httpCookie)
	}

	return httpCookies
}

// SetCookiesFromResponse stores cookies from an HTTP response
func (s *SessionManager) SetCookiesFromResponse(urlStr string, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return
	}

	host := parsedURL.Host
	if h, _, err := splitHostPort(host); err == nil {
		host = h
	}

	existing := s.cookies[host]
	cookieMap := make(map[string]Cookie, len(existing))
	for _, c := range existing {
		cookieMap[c.Name] = c
	}

	for _, c := range cookies {

		sameSite := ""
		switch c.SameSite {
		case http.SameSiteStrictMode:
			sameSite = "Strict"
		case http.SameSiteLaxMode:
			sameSite = "Lax"
		case http.SameSiteNoneMode:
			sameSite = "None"
		}

		cookieMap[c.Name] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			MaxAge:   c.MaxAge,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
			SameSite: sameSite,
		}
	}

	var updatedCookies []Cookie
	for _, c := range cookieMap {
		updatedCookies = append(updatedCookies, c)
	}

	s.cookies[host] = updatedCookies

}

// GetVariable gets a session variable
func (s *SessionManager) GetVariable(name string) (any, bool) {
	val, ok := s.variables[name]
	return val, ok
}

// GetVariableAsString gets a session variable as string
func (s *SessionManager) GetVariableAsString(name string) (string, bool) {
	val, ok := s.variables[name]
	if !ok {
		return "", false
	}

	switch v := val.(type) {
	case string:
		return v, true
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%v", v), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// SetVariable sets a session variable
func (s *SessionManager) SetVariable(name string, value any) {
	s.variables[name] = value
}

// GetAllVariables returns all session variables
func (s *SessionManager) GetAllVariables() map[string]any {
	return s.variables
}

// GetAllCookies returns all cookies
func (s *SessionManager) GetAllCookies() map[string][]Cookie {
	return s.cookies
}

// ClearCookies clears all cookies
func (s *SessionManager) ClearCookies() {
	s.cookies = make(map[string][]Cookie)
}

// ClearVariables clears all variables
func (s *SessionManager) ClearVariables() {
	s.variables = make(map[string]any)
}

// ClearAll clears both cookies and variables
func (s *SessionManager) ClearAll() {
	s.ClearCookies()
	s.ClearVariables()
}

// Delete removes the session directory from disk
func (s *SessionManager) Delete() error {
	return s.fs.RemoveAll(s.sessionPath)
}

// cleanExpiredCookies removes expired cookies from memory
func (s *SessionManager) cleanExpiredCookies() {
	now := time.Now()
	for host, cookies := range s.cookies {
		var validCookies []Cookie
		for _, c := range cookies {
			if c.Expires.IsZero() || c.Expires.After(now) {
				validCookies = append(validCookies, c)
			}
		}
		if len(validCookies) > 0 {
			s.cookies[host] = validCookies
		} else {
			delete(s.cookies, host)
		}
	}
}

// splitHostPort splits a host:port string
func splitHostPort(hostport string) (host, port string, err error) {
	for i := len(hostport) - 1; i >= 0; i-- {
		if hostport[i] == ':' {
			return hostport[:i], hostport[i+1:], nil
		}
		if hostport[i] == ']' {
			// IPv6 literal without port
			break
		}
	}
	return hostport, "", errors.NewValidationError("address", "missing port")
}

// GetSessionPath returns the session path (for display purposes)
func (s *SessionManager) GetSessionPath() string {
	return s.sessionPath
}

// ListAllSessions returns all session directories
func ListAllSessions(baseDir string) ([]string, error) {
	return ListAllSessionsWithFS(filesystem.Default, baseDir)
}

// ListAllSessionsWithFS returns all session directories using a custom file system.
func ListAllSessionsWithFS(fs filesystem.FileSystem, baseDir string) ([]string, error) {
	if baseDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return nil, err
		}
		baseDir = appDir
	}

	var sessions []string

	namedPath := filepath.Join(baseDir, sessionDirName, namedSessionDir)
	if entries, err := fs.ReadDir(namedPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				sessions = append(sessions, "named:"+entry.Name())
			}
		}
	}

	dirsPath := filepath.Join(baseDir, sessionDirName, dirSessionsDir)
	if entries, err := fs.ReadDir(dirsPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				sessions = append(sessions, "dir:"+entry.Name())
			}
		}
	}

	return sessions, nil
}

// ClearAllSessions removes all session data
func ClearAllSessions(baseDir string) error {
	return ClearAllSessionsWithFS(filesystem.Default, baseDir)
}

// ClearAllSessionsWithFS removes all session data using a custom file system.
func ClearAllSessionsWithFS(fs filesystem.FileSystem, baseDir string) error {
	if baseDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return err
		}
		baseDir = appDir
	}

	sessionDir := filepath.Join(baseDir, sessionDirName)
	return fs.RemoveAll(sessionDir)
}
