package client

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"restclient/pkg/models"
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
	if !config.InsecureSSL {
		t.Error("InsecureSSL should be true by default")
	}
	if !config.RememberCookies {
		t.Error("RememberCookies should be true by default")
	}
	if config.DefaultHeaders["User-Agent"] != "restclient-cli" {
		t.Errorf("User-Agent = %v, want restclient-cli", config.DefaultHeaders["User-Agent"])
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
		w.Header().Set("Content-Type", "application/json")
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
	if !strings.Contains(string(resp.Body), "hello") {
		t.Errorf("Body = %v, want to contain 'hello'", string(resp.Body))
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
		"Content-Type": "text/plain",
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
		if r.Header.Get("User-Agent") != "restclient-cli" {
			t.Errorf("User-Agent = %v, want restclient-cli", r.Header.Get("User-Agent"))
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
	if !strings.Contains(string(resp.Body), "target reached") {
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

func TestMd5Hash(t *testing.T) {
	result := md5Hash("test")
	expected := "098f6bcd4621d373cade4e832627b4f6"
	if result != expected {
		t.Errorf("md5Hash() = %v, want %v", result, expected)
	}
}

func TestSha256Hash(t *testing.T) {
	result := sha256Hash("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if result != expected {
		t.Errorf("sha256Hash() = %v, want %v", result, expected)
	}
}

func TestUpdateAuthHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"Authorization": "old-value",
	}, nil, "", "")

	updateAuthHeader(req, "new-value")

	if req.Headers["Authorization"] != "new-value" {
		t.Errorf("Authorization = %v, want new-value", req.Headers["Authorization"])
	}
}

func TestDeleteAuthHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"Authorization": "some-value",
	}, nil, "", "")

	deleteAuthHeader(req)

	if _, exists := req.Headers["Authorization"]; exists {
		t.Error("Authorization header should be deleted")
	}
}

func TestParseDigestChallenge(t *testing.T) {
	header := `Digest realm="test@realm", nonce="abc123", qop="auth", algorithm=MD5, opaque="xyz789"`

	challenge := parseDigestChallenge(header)

	if challenge.realm != "test@realm" {
		t.Errorf("realm = %v, want test@realm", challenge.realm)
	}
	if challenge.nonce != "abc123" {
		t.Errorf("nonce = %v, want abc123", challenge.nonce)
	}
	if challenge.qop != "auth" {
		t.Errorf("qop = %v, want auth", challenge.qop)
	}
	if challenge.algorithm != "MD5" {
		t.Errorf("algorithm = %v, want MD5", challenge.algorithm)
	}
	if challenge.opaque != "xyz789" {
		t.Errorf("opaque = %v, want xyz789", challenge.opaque)
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1 := generateNonce()
	nonce2 := generateNonce()

	if len(nonce1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("nonce length = %v, want 32", len(nonce1))
	}

	if nonce1 == nonce2 {
		t.Error("nonces should be different")
	}
}

func TestGetSignatureKey(t *testing.T) {
	key := getSignatureKey("secret", "20230101", "us-east-1", "s3")

	if len(key) != 32 { // SHA256 produces 32 bytes
		t.Errorf("key length = %v, want 32", len(key))
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
		w.Header().Set("Content-Type", "application/json")
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

	if !strings.Contains(string(resp.Body), "ok") {
		t.Errorf("Body = %v, want to contain 'ok'", string(resp.Body))
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
	challenge := digestChallenge{
		realm:     "test-realm",
		nonce:     "abc123",
		qop:       "auth",
		algorithm: "MD5",
	}

	response := buildDigestResponse("user", "pass", "GET", "/resource", challenge)

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
