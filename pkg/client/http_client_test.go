package client

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ideaspaper/restclient/internal/constants"
	"github.com/ideaspaper/restclient/pkg/auth"
	"github.com/ideaspaper/restclient/pkg/models"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0", config.Timeout)
	}
	if !config.FollowRedirects {
		t.Error("FollowRedirects should be true by default")
	}
	if config.InsecureSSL {
		t.Error("InsecureSSL should be false by default for security")
	}
	if !config.RememberCookies {
		t.Error("RememberCookies should be true by default")
	}
	if config.DefaultHeaders[constants.HeaderUserAgent] != constants.DefaultUserAgent {
		t.Errorf("User-Agent = %v, want %s", config.DefaultHeaders[constants.HeaderUserAgent], constants.DefaultUserAgent)
	}
}

func TestNewHttpClient(t *testing.T) {
	client, err := NewHttpClient(nil)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewHttpClient() returned nil client")
	}
}

func TestNewHttpClientWithConfig(t *testing.T) {
	config := &ClientConfig{
		FollowRedirects: false,
		InsecureSSL:     false,
		DefaultHeaders:  map[string]string{"X-Custom": "value"},
	}

	client, err := NewHttpClient(config)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewHttpClient() returned nil client")
	}
}

func TestSendGetRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/test" {
			t.Errorf("Expected /test, got %s", r.URL.Path)
		}
		w.Header().Set(constants.HeaderContentType, constants.MIMEApplicationJSON)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "hello"}`))
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", make(map[string]string), nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "hello") {
		t.Errorf("Body = %v, want to contain 'hello'", resp.Body)
	}
}

func TestSendPostRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "test data") {
			t.Errorf("Body = %v, want to contain 'test data'", string(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("POST", server.URL+"/create", map[string]string{
		constants.HeaderContentType: constants.MIMETextPlain,
	}, strings.NewReader("test data"), "test data", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("StatusCode = %v, want 201", resp.StatusCode)
	}
}

func TestSendWithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("X-Custom-Header = %v, want custom-value", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %v, want Bearer test-token", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", map[string]string{
		"X-Custom-Header": "custom-value",
		"Authorization":   "Bearer test-token",
	}, nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}

func TestDefaultHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(constants.HeaderUserAgent) != constants.DefaultUserAgent {
			t.Errorf("User-Agent = %v, want %s", r.Header.Get(constants.HeaderUserAgent), constants.DefaultUserAgent)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", make(map[string]string), nil, "", "")

	_, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestNoFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/target")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.FollowRedirects = false
	client, _ := NewHttpClient(config)

	req := models.NewHttpRequest("GET", server.URL+"/redirect", make(map[string]string), nil, "", "")
	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != 302 {
		t.Errorf("StatusCode = %v, want 302 (not followed)", resp.StatusCode)
	}
}

func TestFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/target")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path == "/target" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("target reached"))
			return
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.FollowRedirects = true
	client, _ := NewHttpClient(config)

	req := models.NewHttpRequest("GET", server.URL+"/redirect", make(map[string]string), nil, "", "")
	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200 (followed redirect)", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "target reached") {
		t.Error("Should have followed redirect to target")
	}
}

func TestBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Authorization should start with 'Basic ', got: %s", auth)
		}
		// Check decoded credentials
		encoded := strings.TrimPrefix(auth, "Basic ")
		decoded, _ := base64.StdEncoding.DecodeString(encoded)
		if string(decoded) != "user:pass" {
			t.Errorf("Decoded credentials = %v, want 'user:pass'", string(decoded))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", map[string]string{
		"Authorization": "Basic user:pass",
	}, nil, "", "")

	_, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestCookieJar(t *testing.T) {
	visits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visits++
		if visits == 1 {
			// First visit: set cookie
			http.SetCookie(w, &http.Cookie{
				Name:  "session",
				Value: "abc123",
			})
			w.WriteHeader(http.StatusOK)
			return
		}
		// Second visit: check cookie
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != "abc123" {
			t.Error("Cookie not remembered")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)

	// First request
	req1 := models.NewHttpRequest("GET", server.URL+"/set-cookie", make(map[string]string), nil, "", "")
	_, err := client.Send(req1)
	if err != nil {
		t.Fatalf("First Send() error = %v", err)
	}

	// Second request should have cookie
	req2 := models.NewHttpRequest("GET", server.URL+"/check-cookie", make(map[string]string), nil, "", "")
	_, err = client.Send(req2)
	if err != nil {
		t.Fatalf("Second Send() error = %v", err)
	}
}

func TestClearCookies(t *testing.T) {
	client, _ := NewHttpClient(nil)
	client.ClearCookies()

	if client.cookieJar == nil {
		t.Error("ClearCookies() should create a new cookie jar")
	}
}

func TestUpdateAuthHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"Authorization": "old-value",
	}, nil, "", "")

	auth.UpdateAuthHeader(req, "new-value")

	if req.Headers["Authorization"] != "new-value" {
		t.Errorf("Authorization = %v, want new-value", req.Headers["Authorization"])
	}
}

func TestDeleteAuthHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"Authorization": "some-value",
	}, nil, "", "")

	auth.DeleteAuthHeader(req)

	if _, exists := req.Headers["Authorization"]; exists {
		t.Error("Authorization header should be deleted")
	}
}

func TestParseDigestChallenge(t *testing.T) {
	header := `Digest realm="test@realm", nonce="abc123", qop="auth", algorithm=MD5, opaque="xyz789"`

	challenge := auth.ParseDigestChallenge(header)

	if challenge.Realm != "test@realm" {
		t.Errorf("realm = %v, want test@realm", challenge.Realm)
	}
	if challenge.Nonce != "abc123" {
		t.Errorf("nonce = %v, want abc123", challenge.Nonce)
	}
	if challenge.QOP != "auth" {
		t.Errorf("qop = %v, want auth", challenge.QOP)
	}
	if challenge.Algorithm != "MD5" {
		t.Errorf("algorithm = %v, want MD5", challenge.Algorithm)
	}
	if challenge.Opaque != "xyz789" {
		t.Errorf("opaque = %v, want xyz789", challenge.Opaque)
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1 := auth.GenerateNonce()
	nonce2 := auth.GenerateNonce()

	if len(nonce1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("nonce length = %v, want 32", len(nonce1))
	}

	if nonce1 == nonce2 {
		t.Error("nonces should be different")
	}
}

func TestResponseTiming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", make(map[string]string), nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.Timing.Total <= 0 {
		t.Error("Timing.Total should be positive")
	}
}

func TestGzipResponse(t *testing.T) {
	// This test verifies the client can handle gzip responses
	// The httptest server doesn't automatically gzip, so we test with uncompressed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.MIMEApplicationJSON)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", make(map[string]string), nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !strings.Contains(resp.Body, "ok") {
		t.Errorf("Body = %v, want to contain 'ok'", resp.Body)
	}
}

func TestMultipleHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("Expected %s, got %s", method, r.Method)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client, _ := NewHttpClient(nil)
			req := models.NewHttpRequest(method, server.URL+"/test", make(map[string]string), nil, "", "")

			_, err := client.Send(req)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
		})
	}
}

func TestBuildDigestResponse(t *testing.T) {
	challenge := auth.DigestChallenge{
		Realm:     "test-realm",
		Nonce:     "abc123",
		QOP:       "auth",
		Algorithm: "MD5",
	}

	response := auth.BuildDigestAuth("user", "pass", "GET", "/resource", challenge)

	if !strings.Contains(response, `username="user"`) {
		t.Error("Response should contain username")
	}
	if !strings.Contains(response, `realm="test-realm"`) {
		t.Error("Response should contain realm")
	}
	if !strings.Contains(response, `nonce="abc123"`) {
		t.Error("Response should contain nonce")
	}
	if !strings.Contains(response, `uri="/resource"`) {
		t.Error("Response should contain uri")
	}
	if !strings.Contains(response, `qop=auth`) {
		t.Error("Response should contain qop")
	}
	if !strings.Contains(response, `response="`) {
		t.Error("Response should contain response hash")
	}
}

func TestInvalidProxyURL(t *testing.T) {
	config := &ClientConfig{
		Proxy: "://invalid-proxy-url",
	}

	_, err := NewHttpClient(config)
	if err == nil {
		t.Error("NewHttpClient() should return error for invalid proxy URL")
	}
	if !strings.Contains(err.Error(), "invalid proxy URL") {
		t.Errorf("Error should mention invalid proxy URL, got: %v", err)
	}
}

func TestValidProxyURL(t *testing.T) {
	config := &ClientConfig{
		Proxy: "http://proxy.example.com:8080",
	}

	client, err := NewHttpClient(config)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewHttpClient() returned nil client")
	}
}

// TestDigestAuth tests the full digest authentication flow
func TestDigestAuth(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		auth := r.Header.Get("Authorization")

		if requestCount == 1 {
			// First request: no auth or invalid auth, return 401 with challenge
			if auth == "" || !strings.HasPrefix(auth, "Digest ") {
				w.Header().Set("WWW-Authenticate", `Digest realm="test@realm", nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093", qop="auth", algorithm=MD5`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		// Second request (or first with valid auth): verify digest auth
		if strings.HasPrefix(auth, "Digest ") {
			// Verify required digest fields are present
			if !strings.Contains(auth, "username=") ||
				!strings.Contains(auth, "realm=") ||
				!strings.Contains(auth, "nonce=") ||
				!strings.Contains(auth, "response=") {
				t.Error("Digest auth missing required fields")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authenticated"))
			return
		}

		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/protected", map[string]string{
		"Authorization": "Digest user password",
	}, nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}

// TestDigestAuthWithBody tests digest auth retry preserves request body
func TestDigestAuthWithBody(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if requestCount == 1 {
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="abc123", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify body is present on retry
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "test-data") {
			t.Errorf("Body not preserved on retry, got: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("POST", server.URL+"/protected", map[string]string{
		constants.HeaderAuthorization: "Digest user password",
		constants.HeaderContentType:   constants.MIMETextPlain,
	}, strings.NewReader("test-data"), "test-data", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}

// TestMultipartFormData tests sending multipart/form-data requests
func TestMultipartFormData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get(constants.HeaderContentType)
		if !strings.Contains(contentType, constants.MIMEMultipartFormData) {
			t.Errorf("Content-Type = %v, want %s", contentType, constants.MIMEMultipartFormData)
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}

		// Check text field
		if r.FormValue("name") != "test-name" {
			t.Errorf("name field = %v, want test-name", r.FormValue("name"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("POST", server.URL+"/upload", map[string]string{}, nil, "", "")
	req.MultipartParts = []models.MultipartPart{
		{Name: "name", Value: "test-name", IsFile: false},
	}

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}

// TestMultipartFileUpload tests uploading a file via multipart/form-data
func TestMultipartFileUpload(t *testing.T) {
	// Create a temporary file to upload
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-upload.txt")
	testContent := "This is test file content"
	if err := os.WriteFile(tmpFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}

		file, header, err := r.FormFile("document")
		if err != nil {
			t.Fatalf("FormFile error: %v", err)
		}
		defer file.Close()

		if header.Filename != "test-upload.txt" {
			t.Errorf("Filename = %v, want test-upload.txt", header.Filename)
		}

		content, _ := io.ReadAll(file)
		if string(content) != testContent {
			t.Errorf("File content = %v, want %v", string(content), testContent)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("POST", server.URL+"/upload", map[string]string{}, nil, "", "")
	req.MultipartParts = []models.MultipartPart{
		{Name: "document", FilePath: tmpFile, IsFile: true},
	}

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}

// TestMultipartWithCustomContentType tests file upload with explicit content type
func TestMultipartWithCustomContentType(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "data.json")
	if err := os.WriteFile(tmpFile, []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}

		file, header, err := r.FormFile("config")
		if err != nil {
			t.Fatalf("FormFile error: %v", err)
		}
		defer file.Close()

		// Check that custom content type was set
		if header.Header.Get(constants.HeaderContentType) != constants.MIMEApplicationJSON {
			t.Errorf("Content-Type = %v, want %s", header.Header.Get(constants.HeaderContentType), constants.MIMEApplicationJSON)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("POST", server.URL+"/upload", map[string]string{}, nil, "", "")
	req.MultipartParts = []models.MultipartPart{
		{Name: "config", FilePath: tmpFile, IsFile: true, ContentType: constants.MIMEApplicationJSON},
	}

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}

// TestMultipartFileNotFound tests error handling when file doesn't exist
func TestMultipartFileNotFound(t *testing.T) {
	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("POST", "http://example.com/upload", map[string]string{}, nil, "", "")
	req.MultipartParts = []models.MultipartPart{
		{Name: "file", FilePath: "/nonexistent/file.txt", IsFile: true},
	}

	_, err := client.Send(req)
	if err == nil {
		t.Error("Send() should return error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("Error should mention failed to open file, got: %v", err)
	}
}

// TestGzipResponseDecoding tests automatic gzip response decompression
func TestGzipResponseDecoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentEncoding, "gzip")
		w.Header().Set(constants.HeaderContentType, constants.MIMEApplicationJSON)

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write([]byte(`{"message": "compressed"}`))
		gz.Close()

		w.Write(buf.Bytes())
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/gzip", make(map[string]string), nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !strings.Contains(resp.Body, "compressed") {
		t.Errorf("Body = %v, want to contain 'compressed'", resp.Body)
	}
}

// TestBasicAuthAlreadyEncoded tests that pre-encoded basic auth is used as-is
func TestBasicAuthAlreadyEncoded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if auth != expected {
			t.Errorf("Authorization = %v, want %v", auth, expected)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	// Send pre-encoded credentials
	encoded := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	req := models.NewHttpRequest("GET", server.URL+"/test", map[string]string{
		"Authorization": "Basic " + encoded,
	}, nil, "", "")

	_, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

// TestBasicAuthSeparateUsernamePassword tests "Basic username password" format
func TestBasicAuthSeparateUsernamePassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Authorization should start with 'Basic '")
			return
		}
		encoded := strings.TrimPrefix(auth, "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Errorf("Failed to decode: %v", err)
			return
		}
		if string(decoded) != "myuser:mypass" {
			t.Errorf("Decoded = %v, want 'myuser:mypass'", string(decoded))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test", map[string]string{
		"Authorization": "Basic myuser mypass",
	}, nil, "", "")

	_, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

// TestProxyExclusion tests that excluded hosts bypass the proxy
func TestProxyExclusion(t *testing.T) {
	// We can't easily test actual proxy behavior without a proxy server,
	// but we can test that the config is accepted and client is created
	config := &ClientConfig{
		Proxy:        "http://proxy.example.com:8080",
		ExcludeProxy: []string{"localhost", "internal.company.com"},
	}

	client, err := NewHttpClient(config)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewHttpClient() returned nil")
	}
}

// TestTimeoutConfig tests that timeout is properly configured
func TestTimeoutConfig(t *testing.T) {
	config := &ClientConfig{
		Timeout: 5000000000, // 5 seconds in nanoseconds
	}

	client, err := NewHttpClient(config)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client.client.Timeout != config.Timeout {
		t.Errorf("Timeout = %v, want %v", client.client.Timeout, config.Timeout)
	}
}

// TestInsecureSSLConfig tests that InsecureSSL config is accepted
func TestInsecureSSLConfig(t *testing.T) {
	config := &ClientConfig{
		InsecureSSL: true,
	}

	client, err := NewHttpClient(config)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewHttpClient() returned nil")
	}
}

// TestNoCookieJar tests client without cookie jar
func TestNoCookieJar(t *testing.T) {
	config := &ClientConfig{
		RememberCookies: false,
	}

	client, err := NewHttpClient(config)
	if err != nil {
		t.Fatalf("NewHttpClient() error = %v", err)
	}
	if client.cookieJar != nil {
		t.Error("Cookie jar should be nil when RememberCookies is false")
	}
}

// TestBuildDigestResponseWithoutQop tests digest without qop (older servers)
func TestBuildDigestResponseWithoutQop(t *testing.T) {
	challenge := auth.DigestChallenge{
		Realm:     "test-realm",
		Nonce:     "abc123",
		QOP:       "", // No qop
		Algorithm: "MD5",
	}

	response := auth.BuildDigestAuth("user", "pass", "GET", "/resource", challenge)

	// Without qop, response should not contain nc or cnonce
	if strings.Contains(response, "nc=") {
		t.Error("Response without qop should not contain nc")
	}
	if strings.Contains(response, "cnonce=") {
		t.Error("Response without qop should not contain cnonce")
	}
	if !strings.Contains(response, `response="`) {
		t.Error("Response should contain response hash")
	}
}

// TestBuildDigestResponseMD5Sess tests MD5-sess algorithm variant
func TestBuildDigestResponseMD5Sess(t *testing.T) {
	challenge := auth.DigestChallenge{
		Realm:     "test-realm",
		Nonce:     "abc123",
		QOP:       "auth",
		Algorithm: "MD5-sess",
		Opaque:    "opaque-value",
	}

	response := auth.BuildDigestAuth("user", "pass", "GET", "/resource", challenge)

	if !strings.Contains(response, `algorithm=MD5-sess`) {
		t.Error("Response should contain algorithm=MD5-sess")
	}
	if !strings.Contains(response, `opaque="opaque-value"`) {
		t.Error("Response should contain opaque value")
	}
}

// TestRequestWithQueryString tests requests with query parameters
func TestRequestWithQueryString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("foo") != "bar" {
			t.Errorf("Query param foo = %v, want bar", r.URL.Query().Get("foo"))
		}
		if r.URL.Query().Get("num") != "123" {
			t.Errorf("Query param num = %v, want 123", r.URL.Query().Get("num"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewHttpClient(nil)
	req := models.NewHttpRequest("GET", server.URL+"/test?foo=bar&num=123", make(map[string]string), nil, "", "")

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}
}
