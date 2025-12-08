package session

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSessionManager(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		httpFilePath string
		sessionName  string
		wantErr      bool
	}{
		{
			name:         "directory-based session",
			httpFilePath: filepath.Join(tempDir, "project", "api.http"),
			sessionName:  "",
			wantErr:      false,
		},
		{
			name:         "named session",
			httpFilePath: "",
			sessionName:  "my-api",
			wantErr:      false,
		},
		{
			name:         "default session when no path or name",
			httpFilePath: "",
			sessionName:  "",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, err := NewSessionManager(tempDir, tt.httpFilePath, tt.sessionName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSessionManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if sm == nil {
				t.Error("NewSessionManager() returned nil")
			}
		})
	}
}

func TestSessionManager_Variables(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSessionManager(tempDir, "", "test-vars")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Test SetVariable and GetVariable
	sm.SetVariable("token", "abc123")
	sm.SetVariable("userId", float64(42))
	sm.SetVariable("isActive", true)

	// Test GetVariable
	val, ok := sm.GetVariable("token")
	if !ok {
		t.Error("GetVariable() should return ok=true for existing variable")
	}
	if val != "abc123" {
		t.Errorf("GetVariable() = %v, want %v", val, "abc123")
	}

	// Test GetVariableAsString
	strVal, ok := sm.GetVariableAsString("token")
	if !ok || strVal != "abc123" {
		t.Errorf("GetVariableAsString() = %v, want %v", strVal, "abc123")
	}

	strVal, ok = sm.GetVariableAsString("userId")
	if !ok || strVal != "42" {
		t.Errorf("GetVariableAsString() for number = %v, want %v", strVal, "42")
	}

	// Test non-existent variable
	_, ok = sm.GetVariable("nonexistent")
	if ok {
		t.Error("GetVariable() should return ok=false for non-existent variable")
	}

	// Test GetAllVariables
	allVars := sm.GetAllVariables()
	if len(allVars) != 3 {
		t.Errorf("GetAllVariables() length = %d, want %d", len(allVars), 3)
	}

	// Test ClearVariables
	sm.ClearVariables()
	allVars = sm.GetAllVariables()
	if len(allVars) != 0 {
		t.Errorf("After ClearVariables(), GetAllVariables() length = %d, want %d", len(allVars), 0)
	}
}

func TestSessionManager_Cookies(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSessionManager(tempDir, "", "test-cookies")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Create test cookies
	cookies := []*http.Cookie{
		{
			Name:     "session_id",
			Value:    "abc123",
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
		},
		{
			Name:   "user_pref",
			Value:  "dark_mode",
			Path:   "/",
			MaxAge: 3600,
		},
	}

	// Test SetCookiesFromResponse
	sm.SetCookiesFromResponse("https://api.example.com/login", cookies)

	// Test GetCookiesForURL
	retrieved := sm.GetCookiesForURL("https://api.example.com/users")
	if len(retrieved) != 2 {
		t.Errorf("GetCookiesForURL() returned %d cookies, want %d", len(retrieved), 2)
	}

	// Check cookie values
	found := false
	for _, c := range retrieved {
		if c.Name == "session_id" && c.Value == "abc123" {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetCookiesForURL() did not return expected cookie")
	}

	// Test GetAllCookies
	allCookies := sm.GetAllCookies()
	if len(allCookies) != 1 { // One host
		t.Errorf("GetAllCookies() returned %d hosts, want %d", len(allCookies), 1)
	}

	// Test ClearCookies
	sm.ClearCookies()
	allCookies = sm.GetAllCookies()
	if len(allCookies) != 0 {
		t.Errorf("After ClearCookies(), GetAllCookies() length = %d, want %d", len(allCookies), 0)
	}
}

func TestSessionManager_CookieExpiry(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSessionManager(tempDir, "", "test-expiry")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Create cookies with different expiry times
	cookies := []*http.Cookie{
		{
			Name:    "expired",
			Value:   "old_value",
			Path:    "/",
			Expires: time.Now().Add(-1 * time.Hour), // Expired
		},
		{
			Name:    "valid",
			Value:   "new_value",
			Path:    "/",
			Expires: time.Now().Add(1 * time.Hour), // Still valid
		},
	}

	sm.SetCookiesFromResponse("https://api.example.com", cookies)

	// Get cookies should filter out expired ones
	retrieved := sm.GetCookiesForURL("https://api.example.com")
	if len(retrieved) != 1 {
		t.Errorf("GetCookiesForURL() returned %d cookies, want %d (expired should be filtered)", len(retrieved), 1)
	}

	if len(retrieved) > 0 && retrieved[0].Name != "valid" {
		t.Error("GetCookiesForURL() returned wrong cookie (should be 'valid')")
	}
}

func TestSessionManager_Persistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create and populate first session manager
	sm1, err := NewSessionManager(tempDir, "", "test-persist")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	sm1.SetVariable("authToken", "secret123")
	sm1.SetCookiesFromResponse("https://api.example.com", []*http.Cookie{
		{Name: "session", Value: "xyz789", Path: "/"},
	})

	// Save to disk
	if err := sm1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create new session manager and load
	sm2, err := NewSessionManager(tempDir, "", "test-persist")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	if err := sm2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify variables persisted
	val, ok := sm2.GetVariableAsString("authToken")
	if !ok || val != "secret123" {
		t.Errorf("After Load(), GetVariableAsString() = %v, want %v", val, "secret123")
	}

	// Verify cookies persisted
	cookies := sm2.GetCookiesForURL("https://api.example.com")
	if len(cookies) != 1 {
		t.Errorf("After Load(), GetCookiesForURL() returned %d cookies, want %d", len(cookies), 1)
	}
	if len(cookies) > 0 && cookies[0].Name != "session" {
		t.Error("After Load(), cookie name mismatch")
	}
}

func TestSessionManager_Delete(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSessionManager(tempDir, "", "test-delete")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	sm.SetVariable("test", "value")
	if err := sm.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify files exist
	sessionPath := sm.GetSessionPath()
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		t.Error("Session directory should exist after Save()")
	}

	// Delete session
	if err := sm.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify files deleted
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("Session directory should not exist after Delete()")
	}
}

func TestSessionManager_DirectoryScoping(t *testing.T) {
	tempDir := t.TempDir()

	// Create two session managers for different directories
	sm1, _ := NewSessionManager(tempDir, "/project1/api.http", "")
	sm2, _ := NewSessionManager(tempDir, "/project2/api.http", "")

	sm1.SetVariable("projectName", "project1")
	sm2.SetVariable("projectName", "project2")

	// Verify they have different session paths
	if sm1.GetSessionPath() == sm2.GetSessionPath() {
		t.Error("Different directories should have different session paths")
	}

	// Save both
	sm1.Save()
	sm2.Save()

	// Reload and verify isolation
	sm1New, _ := NewSessionManager(tempDir, "/project1/api.http", "")
	sm1New.Load()
	val, _ := sm1New.GetVariableAsString("projectName")
	if val != "project1" {
		t.Errorf("Project 1 variable = %v, want %v", val, "project1")
	}

	sm2New, _ := NewSessionManager(tempDir, "/project2/api.http", "")
	sm2New.Load()
	val, _ = sm2New.GetVariableAsString("projectName")
	if val != "project2" {
		t.Errorf("Project 2 variable = %v, want %v", val, "project2")
	}
}

func TestSessionManager_NamedSessions(t *testing.T) {
	tempDir := t.TempDir()

	// Create named session
	sm1, _ := NewSessionManager(tempDir, "", "my-api")
	sm1.SetVariable("apiVersion", "v2")
	sm1.Save()

	// Create another named session
	sm2, _ := NewSessionManager(tempDir, "", "other-api")
	sm2.SetVariable("apiVersion", "v1")
	sm2.Save()

	// Verify they have different session paths
	if sm1.GetSessionPath() == sm2.GetSessionPath() {
		t.Error("Different named sessions should have different session paths")
	}

	// Reload and verify
	sm1New, _ := NewSessionManager(tempDir, "", "my-api")
	sm1New.Load()
	val, _ := sm1New.GetVariableAsString("apiVersion")
	if val != "v2" {
		t.Errorf("my-api apiVersion = %v, want %v", val, "v2")
	}
}

func TestListAllSessions(t *testing.T) {
	tempDir := t.TempDir()

	// Create some sessions
	sm1, _ := NewSessionManager(tempDir, "", "api-1")
	sm1.SetVariable("test", "1")
	sm1.Save()

	sm2, _ := NewSessionManager(tempDir, "", "api-2")
	sm2.SetVariable("test", "2")
	sm2.Save()

	sm3, _ := NewSessionManager(tempDir, "/some/project/api.http", "")
	sm3.SetVariable("test", "3")
	sm3.Save()

	// List all sessions
	sessions, err := ListAllSessions(tempDir)
	if err != nil {
		t.Fatalf("ListAllSessions() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("ListAllSessions() returned %d sessions, want %d", len(sessions), 3)
	}

	// Verify named sessions are listed
	hasNamed := false
	hasDir := false
	for _, s := range sessions {
		if s == "named:api-1" || s == "named:api-2" {
			hasNamed = true
		}
		if len(s) > 4 && s[:4] == "dir:" {
			hasDir = true
		}
	}

	if !hasNamed {
		t.Error("ListAllSessions() should include named sessions")
	}
	if !hasDir {
		t.Error("ListAllSessions() should include directory-based sessions")
	}
}

func TestClearAllSessions(t *testing.T) {
	tempDir := t.TempDir()

	// Create some sessions
	sm1, _ := NewSessionManager(tempDir, "", "test-clear-1")
	sm1.SetVariable("test", "1")
	sm1.Save()

	sm2, _ := NewSessionManager(tempDir, "/project/api.http", "")
	sm2.SetVariable("test", "2")
	sm2.Save()

	// Verify sessions exist
	sessions, _ := ListAllSessions(tempDir)
	if len(sessions) == 0 {
		t.Fatal("Sessions should exist before clear")
	}

	// Clear all
	if err := ClearAllSessions(tempDir); err != nil {
		t.Fatalf("ClearAllSessions() error = %v", err)
	}

	// Verify all cleared
	sessions, _ = ListAllSessions(tempDir)
	if len(sessions) != 0 {
		t.Errorf("After ClearAllSessions(), ListAllSessions() returned %d sessions, want 0", len(sessions))
	}
}

func TestHashPath(t *testing.T) {
	// Same path should produce same hash
	hash1 := hashPath("/Users/test/project")
	hash2 := hashPath("/Users/test/project")
	if hash1 != hash2 {
		t.Error("Same path should produce same hash")
	}

	// Different paths should produce different hashes
	hash3 := hashPath("/Users/test/other-project")
	if hash1 == hash3 {
		t.Error("Different paths should produce different hashes")
	}

	// Hash should be consistent length (16 hex chars = 8 bytes)
	if len(hash1) != 16 {
		t.Errorf("Hash length = %d, want 16", len(hash1))
	}
}

func TestCookieWithPort(t *testing.T) {
	tempDir := t.TempDir()

	sm, err := NewSessionManager(tempDir, "", "test-port")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Set cookie with port in URL
	sm.SetCookiesFromResponse("https://localhost:8080/api", []*http.Cookie{
		{Name: "test", Value: "value", Path: "/"},
	})

	// Should retrieve with or without port
	cookies := sm.GetCookiesForURL("https://localhost:8080/other")
	if len(cookies) != 1 {
		t.Errorf("GetCookiesForURL() returned %d cookies, want 1", len(cookies))
	}
}
