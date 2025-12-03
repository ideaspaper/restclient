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

	"github.com/ideaspaper/restclient/internal/paths"
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

// SessionManager manages session persistence (cookies and variables)
type SessionManager struct {
	baseDir     string
	sessionPath string
	cookies     map[string][]Cookie    // host -> cookies
	variables   map[string]interface{} // variable name -> value
}

// NewSessionManager creates a new session manager
// If sessionName is provided, uses named session; otherwise uses directory-based session
func NewSessionManager(baseDir, httpFilePath, sessionName string) (*SessionManager, error) {
	if baseDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return nil, fmt.Errorf("failed to get app data directory: %w", err)
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

	sm := &SessionManager{
		baseDir:     baseDir,
		sessionPath: sessionPath,
		cookies:     make(map[string][]Cookie),
		variables:   make(map[string]interface{}),
	}

	return sm, nil
}

// hashPath creates a short hash of a path for directory naming
func hashPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8]) // Use first 8 bytes (16 hex chars)
}

// Load loads both cookies and variables from disk
func (sm *SessionManager) Load() error {
	if err := sm.LoadCookies(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load cookies: %w", err)
	}
	if err := sm.LoadVariables(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load variables: %w", err)
	}
	return nil
}

// Save saves both cookies and variables to disk
func (sm *SessionManager) Save() error {
	if err := sm.SaveCookies(); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}
	if err := sm.SaveVariables(); err != nil {
		return fmt.Errorf("failed to save variables: %w", err)
	}
	return nil
}

// LoadCookies loads cookies from disk
func (sm *SessionManager) LoadCookies() error {
	path := filepath.Join(sm.sessionPath, cookiesFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read cookies file: %w", err)
	}

	if err := json.Unmarshal(data, &sm.cookies); err != nil {
		return fmt.Errorf("failed to parse cookies file: %w", err)
	}
	return nil
}

// SaveCookies saves cookies to disk
func (sm *SessionManager) SaveCookies() error {
	if err := os.MkdirAll(sm.sessionPath, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Clean expired cookies before saving
	sm.cleanExpiredCookies()

	data, err := json.MarshalIndent(sm.cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	path := filepath.Join(sm.sessionPath, cookiesFileName)
	return os.WriteFile(path, data, 0644)
}

// LoadVariables loads variables from disk
func (sm *SessionManager) LoadVariables() error {
	path := filepath.Join(sm.sessionPath, varsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read variables file: %w", err)
	}

	if err := json.Unmarshal(data, &sm.variables); err != nil {
		return fmt.Errorf("failed to parse variables file: %w", err)
	}
	return nil
}

// SaveVariables saves variables to disk
func (sm *SessionManager) SaveVariables() error {
	if err := os.MkdirAll(sm.sessionPath, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	data, err := json.MarshalIndent(sm.variables, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	path := filepath.Join(sm.sessionPath, varsFileName)
	return os.WriteFile(path, data, 0644)
}

// GetCookiesForURL returns cookies for a given URL
func (sm *SessionManager) GetCookiesForURL(urlStr string) []*http.Cookie {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}

	host := parsedURL.Host
	if h, _, err := splitHostPort(host); err == nil {
		host = h
	}

	storedCookies, ok := sm.cookies[host]
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
func (sm *SessionManager) SetCookiesFromResponse(urlStr string, cookies []*http.Cookie) {
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

	existing := sm.cookies[host]
	cookieMap := make(map[string]Cookie)
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

	sm.cookies[host] = updatedCookies
}

// GetVariable gets a session variable
func (sm *SessionManager) GetVariable(name string) (interface{}, bool) {
	val, ok := sm.variables[name]
	return val, ok
}

// GetVariableAsString gets a session variable as string
func (sm *SessionManager) GetVariableAsString(name string) (string, bool) {
	val, ok := sm.variables[name]
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
func (sm *SessionManager) SetVariable(name string, value interface{}) {
	sm.variables[name] = value
}

// GetAllVariables returns all session variables
func (sm *SessionManager) GetAllVariables() map[string]interface{} {
	return sm.variables
}

// GetAllCookies returns all cookies
func (sm *SessionManager) GetAllCookies() map[string][]Cookie {
	return sm.cookies
}

// ClearCookies clears all cookies
func (sm *SessionManager) ClearCookies() {
	sm.cookies = make(map[string][]Cookie)
}

// ClearVariables clears all variables
func (sm *SessionManager) ClearVariables() {
	sm.variables = make(map[string]interface{})
}

// ClearAll clears both cookies and variables
func (sm *SessionManager) ClearAll() {
	sm.ClearCookies()
	sm.ClearVariables()
}

// Delete removes the session directory from disk
func (sm *SessionManager) Delete() error {
	return os.RemoveAll(sm.sessionPath)
}

// cleanExpiredCookies removes expired cookies from memory
func (sm *SessionManager) cleanExpiredCookies() {
	now := time.Now()
	for host, cookies := range sm.cookies {
		var validCookies []Cookie
		for _, c := range cookies {
			if c.Expires.IsZero() || c.Expires.After(now) {
				validCookies = append(validCookies, c)
			}
		}
		if len(validCookies) > 0 {
			sm.cookies[host] = validCookies
		} else {
			delete(sm.cookies, host)
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
	return hostport, "", fmt.Errorf("missing port in address")
}

// GetSessionPath returns the session path (for display purposes)
func (sm *SessionManager) GetSessionPath() string {
	return sm.sessionPath
}

// ListAllSessions returns all session directories
func ListAllSessions(baseDir string) ([]string, error) {
	if baseDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return nil, err
		}
		baseDir = appDir
	}

	var sessions []string

	namedPath := filepath.Join(baseDir, sessionDirName, namedSessionDir)
	if entries, err := os.ReadDir(namedPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				sessions = append(sessions, "named:"+entry.Name())
			}
		}
	}

	dirsPath := filepath.Join(baseDir, sessionDirName, dirSessionsDir)
	if entries, err := os.ReadDir(dirsPath); err == nil {
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
	if baseDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return err
		}
		baseDir = appDir
	}

	sessionDir := filepath.Join(baseDir, sessionDirName)
	return os.RemoveAll(sessionDir)
}
