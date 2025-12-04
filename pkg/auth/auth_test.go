package auth

import (
	"strings"
	"testing"

	"github.com/ideaspaper/restclient/pkg/models"
)

func TestNewProcessor(t *testing.T) {
	p := NewProcessor()
	if p == nil {
		t.Error("NewProcessor() returned nil")
	}
	if p.digestCreds == nil {
		t.Error("digestCreds map should be initialized")
	}
}

func TestProcessAuthBasic(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "username:password format",
			header:   "Basic user:pass",
			expected: "Basic dXNlcjpwYXNz",
		},
		{
			name:     "already encoded",
			header:   "Basic dXNlcjpwYXNz",
			expected: "Basic dXNlcjpwYXNz",
		},
		{
			name:     "username password separate",
			header:   "Basic myuser mypassword",
			expected: "Basic bXl1c2VyOm15cGFzc3dvcmQ=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessor()
			req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
				"Authorization": tt.header,
			}, nil, "", "")

			err := p.ProcessAuth(req)
			if err != nil {
				t.Fatalf("ProcessAuth() error = %v", err)
			}

			if req.Headers["Authorization"] != tt.expected {
				t.Errorf("Authorization = %v, want %v", req.Headers["Authorization"], tt.expected)
			}
		})
	}
}

func TestProcessAuthDigest(t *testing.T) {
	p := NewProcessor()
	req := models.NewHttpRequest("GET", "http://test.com/resource", map[string]string{
		"Authorization": "Digest user password123",
	}, nil, "", "")

	err := p.ProcessAuth(req)
	if err != nil {
		t.Fatalf("ProcessAuth() error = %v", err)
	}

	// Digest auth should remove the header initially (added after challenge)
	if _, exists := req.Headers["Authorization"]; exists {
		t.Error("Authorization header should be removed for digest auth")
	}

	// Credentials should be stored
	creds, ok := p.GetDigestCredentials("http://test.com/resource")
	if !ok {
		t.Error("Digest credentials should be stored")
	}
	if creds.Username != "user" {
		t.Errorf("Username = %v, want user", creds.Username)
	}
	if creds.Password != "password123" {
		t.Errorf("Password = %v, want password123", creds.Password)
	}
}

func TestProcessAuthNoHeader(t *testing.T) {
	p := NewProcessor()
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{}, nil, "", "")

	err := p.ProcessAuth(req)
	if err != nil {
		t.Fatalf("ProcessAuth() error = %v", err)
	}
}

func TestProcessAuthAWS(t *testing.T) {
	p := NewProcessor()
	req := models.NewHttpRequest("GET", "https://s3.us-east-1.amazonaws.com/bucket/key", map[string]string{
		"Authorization": "AWS AKIAIOSFODNN7EXAMPLE wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}, nil, "", "")

	err := p.ProcessAuth(req)
	if err != nil {
		t.Fatalf("ProcessAuth() error = %v", err)
	}

	// AWS auth should set the Authorization header
	authHeader := req.Headers["Authorization"]
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		t.Errorf("Authorization should start with AWS4-HMAC-SHA256, got %v", authHeader)
	}

	// Should set X-Amz-Date
	if _, exists := req.Headers["X-Amz-Date"]; !exists {
		t.Error("X-Amz-Date header should be set")
	}
}

func TestProcessAuthAWSWithSessionToken(t *testing.T) {
	p := NewProcessor()
	req := models.NewHttpRequest("GET", "https://s3.us-east-1.amazonaws.com/bucket/key", map[string]string{
		"Authorization": "AWS AKIAIOSFODNN7EXAMPLE wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY token:mytoken",
	}, nil, "", "")

	err := p.ProcessAuth(req)
	if err != nil {
		t.Fatalf("ProcessAuth() error = %v", err)
	}

	if req.Headers["X-Amz-Security-Token"] != "mytoken" {
		t.Errorf("X-Amz-Security-Token = %v, want mytoken", req.Headers["X-Amz-Security-Token"])
	}
}

func TestProcessAuthAWSMissingCredentials(t *testing.T) {
	p := NewProcessor()
	req := models.NewHttpRequest("GET", "https://s3.us-east-1.amazonaws.com/bucket/key", map[string]string{
		"Authorization": "AWS AKIAIOSFODNN7EXAMPLE", // Missing secret key
	}, nil, "", "")

	err := p.ProcessAuth(req)
	if err == nil {
		t.Error("ProcessAuth() should return error for missing secret key")
	}
}

func TestParseDigestChallenge(t *testing.T) {
	header := `Digest realm="test@realm", nonce="abc123", qop="auth", algorithm=MD5, opaque="xyz789"`

	challenge := ParseDigestChallenge(header)

	if challenge.Realm != "test@realm" {
		t.Errorf("Realm = %v, want test@realm", challenge.Realm)
	}
	if challenge.Nonce != "abc123" {
		t.Errorf("Nonce = %v, want abc123", challenge.Nonce)
	}
	if challenge.QOP != "auth" {
		t.Errorf("QOP = %v, want auth", challenge.QOP)
	}
	if challenge.Algorithm != "MD5" {
		t.Errorf("Algorithm = %v, want MD5", challenge.Algorithm)
	}
	if challenge.Opaque != "xyz789" {
		t.Errorf("Opaque = %v, want xyz789", challenge.Opaque)
	}
}

func TestBuildDigestAuth(t *testing.T) {
	challenge := DigestChallenge{
		Realm:     "test-realm",
		Nonce:     "abc123",
		QOP:       "auth",
		Algorithm: "MD5",
	}

	response := BuildDigestAuth("user", "pass", "GET", "/resource", challenge)

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

func TestBuildDigestAuthWithoutQOP(t *testing.T) {
	challenge := DigestChallenge{
		Realm:     "test-realm",
		Nonce:     "abc123",
		Algorithm: "MD5",
		// No QOP
	}

	response := BuildDigestAuth("user", "pass", "GET", "/resource", challenge)

	// Without QOP, response should not contain nc or cnonce
	if strings.Contains(response, "nc=") {
		t.Error("Response without QOP should not contain nc")
	}
	if strings.Contains(response, "cnonce=") {
		t.Error("Response without QOP should not contain cnonce")
	}
}

func TestBuildDigestAuthMD5Sess(t *testing.T) {
	challenge := DigestChallenge{
		Realm:     "test-realm",
		Nonce:     "abc123",
		QOP:       "auth",
		Algorithm: "MD5-sess",
		Opaque:    "opaque-value",
	}

	response := BuildDigestAuth("user", "pass", "GET", "/resource", challenge)

	if !strings.Contains(response, `algorithm=MD5-sess`) {
		t.Error("Response should contain algorithm=MD5-sess")
	}
	if !strings.Contains(response, `opaque="opaque-value"`) {
		t.Error("Response should contain opaque value")
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1 := GenerateNonce()
	nonce2 := GenerateNonce()

	if len(nonce1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("nonce length = %v, want 32", len(nonce1))
	}

	if nonce1 == nonce2 {
		t.Error("nonces should be different")
	}
}

func TestUpdateAuthHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"Authorization": "old-value",
	}, nil, "", "")

	UpdateAuthHeader(req, "new-value")

	if req.Headers["Authorization"] != "new-value" {
		t.Errorf("Authorization = %v, want new-value", req.Headers["Authorization"])
	}
}

func TestUpdateAuthHeaderNewHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{}, nil, "", "")

	UpdateAuthHeader(req, "new-value")

	if req.Headers["Authorization"] != "new-value" {
		t.Errorf("Authorization = %v, want new-value", req.Headers["Authorization"])
	}
}

func TestDeleteAuthHeader(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"Authorization": "some-value",
	}, nil, "", "")

	DeleteAuthHeader(req)

	if _, exists := req.Headers["Authorization"]; exists {
		t.Error("Authorization header should be deleted")
	}
}

func TestDeleteAuthHeaderCaseInsensitive(t *testing.T) {
	req := models.NewHttpRequest("GET", "http://test.com", map[string]string{
		"authorization": "some-value",
	}, nil, "", "")

	DeleteAuthHeader(req)

	if _, exists := req.Headers["authorization"]; exists {
		t.Error("Authorization header should be deleted (case insensitive)")
	}
}

func TestAWSSignerSign(t *testing.T) {
	signer := &AWSSigner{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Region:          "us-east-1",
		Service:         "s3",
	}

	req := models.NewHttpRequest("GET", "https://s3.us-east-1.amazonaws.com/bucket/key", map[string]string{
		"Host": "s3.us-east-1.amazonaws.com",
	}, nil, "", "")

	err := signer.Sign(req)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	authHeader := req.Headers["Authorization"]
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/") {
		t.Errorf("Authorization header format incorrect: %v", authHeader)
	}
	if !strings.Contains(authHeader, "SignedHeaders=") {
		t.Error("Authorization header should contain SignedHeaders")
	}
	if !strings.Contains(authHeader, "Signature=") {
		t.Error("Authorization header should contain Signature")
	}
}

func TestAWSSignerSignWithSessionToken(t *testing.T) {
	signer := &AWSSigner{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "mytoken",
		Region:          "us-east-1",
		Service:         "s3",
	}

	req := models.NewHttpRequest("GET", "https://s3.us-east-1.amazonaws.com/bucket/key", map[string]string{}, nil, "", "")

	err := signer.Sign(req)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if req.Headers["X-Amz-Security-Token"] != "mytoken" {
		t.Errorf("X-Amz-Security-Token = %v, want mytoken", req.Headers["X-Amz-Security-Token"])
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
