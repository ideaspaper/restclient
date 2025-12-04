// Package auth provides authentication handlers for HTTP requests.
// It supports Basic, Digest, and AWS Signature v4 authentication schemes.
package auth

import (
	"crypto/hmac"
	"crypto/md5"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/models"
)

// Processor handles authentication processing for HTTP requests
type Processor struct {
	digestCreds map[string]DigestCredentials
}

// NewProcessor creates a new authentication processor
func NewProcessor() *Processor {
	return &Processor{
		digestCreds: make(map[string]DigestCredentials),
	}
}

// DigestCredentials stores digest auth credentials
type DigestCredentials struct {
	Username string
	Password string
}

// DigestChallenge holds parsed digest challenge from WWW-Authenticate header
type DigestChallenge struct {
	Realm     string
	Nonce     string
	Opaque    string
	QOP       string
	Algorithm string
}

// ProcessAuth processes authentication headers in the request
// It returns nil if no auth processing is needed or on success.
func (p *Processor) ProcessAuth(request *models.HttpRequest) error {
	authHeader := ""
	for k, v := range request.Headers {
		if strings.EqualFold(k, "authorization") {
			authHeader = v
			break
		}
	}

	if authHeader == "" {
		return nil
	}

	parts := strings.Fields(authHeader)
	if len(parts) < 2 {
		return nil
	}

	scheme := strings.ToLower(parts[0])

	switch scheme {
	case "basic":
		return p.processBasicAuth(request, parts[1:])
	case "digest":
		// Digest auth is handled during response (see BuildDigestAuth)
		// Just store credentials for later
		return p.storeDigestCredentials(request, parts[1:])
	case "aws":
		return p.processAWSAuth(request, parts[1:])
	}

	return nil
}

// GetDigestCredentials returns stored digest credentials for a URL
func (p *Processor) GetDigestCredentials(urlStr string) (DigestCredentials, bool) {
	creds, ok := p.digestCreds[urlStr]
	return creds, ok
}

// processBasicAuth processes Basic authentication
func (p *Processor) processBasicAuth(request *models.HttpRequest, args []string) error {
	if len(args) == 0 {
		return nil
	}

	credentials := args[0]

	// Check if already base64 encoded
	if _, err := base64.StdEncoding.DecodeString(credentials); err == nil {
		// Already encoded, use as-is
		UpdateAuthHeader(request, "Basic "+credentials)
		return nil
	}

	// Check for username:password format
	if strings.Contains(credentials, ":") {
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
		UpdateAuthHeader(request, "Basic "+encoded)
		return nil
	}

	// username password format (two separate args)
	if len(args) >= 2 {
		creds := args[0] + ":" + strings.Join(args[1:], " ")
		encoded := base64.StdEncoding.EncodeToString([]byte(creds))
		UpdateAuthHeader(request, "Basic "+encoded)
	}

	return nil
}

// storeDigestCredentials stores digest credentials for later use
func (p *Processor) storeDigestCredentials(request *models.HttpRequest, args []string) error {
	if len(args) < 2 {
		return nil
	}

	p.digestCreds[request.URL] = DigestCredentials{
		Username: args[0],
		Password: strings.Join(args[1:], " "),
	}

	// Remove the Authorization header for now (will be added after challenge)
	DeleteAuthHeader(request)
	return nil
}

// processAWSAuth processes AWS Signature v4 authentication
func (p *Processor) processAWSAuth(request *models.HttpRequest, args []string) error {
	if len(args) < 2 {
		return errors.NewValidationError("AWS auth", "requires accessKeyId and secretAccessKey")
	}

	accessKeyID := args[0]
	secretAccessKey := args[1]

	// Parse optional parameters
	var sessionToken, region, service string
	for _, arg := range args[2:] {
		if strings.HasPrefix(arg, "token:") {
			sessionToken = strings.TrimPrefix(arg, "token:")
		} else if strings.HasPrefix(arg, "region:") {
			region = strings.TrimPrefix(arg, "region:")
		} else if strings.HasPrefix(arg, "service:") {
			service = strings.TrimPrefix(arg, "service:")
		}
	}

	// Parse URL to get region and service if not provided
	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		return errors.Wrap(err, "failed to parse URL for AWS auth")
	}

	if region == "" || service == "" {
		// Try to extract from URL (e.g., s3.us-east-1.amazonaws.com)
		parts := strings.Split(parsedURL.Host, ".")
		if len(parts) >= 3 {
			if service == "" {
				service = parts[0]
			}
			if region == "" && len(parts) >= 4 {
				region = parts[1]
			}
		}
	}

	if region == "" {
		region = "us-east-1"
	}
	if service == "" {
		service = "execute-api"
	}

	// Sign the request
	signer := &AWSSigner{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    sessionToken,
		Region:          region,
		Service:         service,
	}

	return signer.Sign(request)
}

// ParseDigestChallenge parses the WWW-Authenticate header for digest auth
func ParseDigestChallenge(header string) DigestChallenge {
	challenge := DigestChallenge{}

	re := regexp.MustCompile(`(\w+)=(?:"([^"]+)"|([^,\s]+))`)
	matches := re.FindAllStringSubmatch(header, -1)

	for _, match := range matches {
		key := strings.ToLower(match[1])
		value := match[2]
		if value == "" {
			value = match[3]
		}

		switch key {
		case "realm":
			challenge.Realm = value
		case "nonce":
			challenge.Nonce = value
		case "opaque":
			challenge.Opaque = value
		case "qop":
			challenge.QOP = value
		case "algorithm":
			challenge.Algorithm = value
		}
	}

	return challenge
}

// BuildDigestAuth builds the digest auth response header value
func BuildDigestAuth(username, password, method, uri string, challenge DigestChallenge) string {
	ha1 := md5Hash(fmt.Sprintf("%s:%s:%s", username, challenge.Realm, password))

	if strings.EqualFold(challenge.Algorithm, "md5-sess") {
		cnonce := GenerateNonce()
		ha1 = md5Hash(fmt.Sprintf("%s:%s:%s", ha1, challenge.Nonce, cnonce))
	}

	ha2 := md5Hash(fmt.Sprintf("%s:%s", method, uri))

	var response string
	var params []string

	if strings.Contains(challenge.QOP, "auth") {
		nc := "00000001"
		cnonce := GenerateNonce()
		response = md5Hash(fmt.Sprintf("%s:%s:%s:%s:auth:%s",
			ha1, challenge.Nonce, nc, cnonce, ha2))

		params = []string{
			fmt.Sprintf(`username="%s"`, username),
			fmt.Sprintf(`realm="%s"`, challenge.Realm),
			fmt.Sprintf(`nonce="%s"`, challenge.Nonce),
			fmt.Sprintf(`uri="%s"`, uri),
			`qop=auth`,
			fmt.Sprintf(`nc=%s`, nc),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
			fmt.Sprintf(`response="%s"`, response),
		}
	} else {
		response = md5Hash(fmt.Sprintf("%s:%s:%s", ha1, challenge.Nonce, ha2))

		params = []string{
			fmt.Sprintf(`username="%s"`, username),
			fmt.Sprintf(`realm="%s"`, challenge.Realm),
			fmt.Sprintf(`nonce="%s"`, challenge.Nonce),
			fmt.Sprintf(`uri="%s"`, uri),
			fmt.Sprintf(`response="%s"`, response),
		}
	}

	if challenge.Opaque != "" {
		params = append(params, fmt.Sprintf(`opaque="%s"`, challenge.Opaque))
	}

	if challenge.Algorithm != "" {
		params = append(params, fmt.Sprintf(`algorithm=%s`, challenge.Algorithm))
	}

	return strings.Join(params, ", ")
}

// AWSSigner implements AWS Signature v4
type AWSSigner struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Service         string
}

// Sign signs the request with AWS Signature v4
func (s *AWSSigner) Sign(request *models.HttpRequest) error {
	t := time.Now().UTC()
	dateStamp := t.Format("20060102")
	amzDate := t.Format("20060102T150405Z")

	request.Headers["X-Amz-Date"] = amzDate
	if s.SessionToken != "" {
		request.Headers["X-Amz-Security-Token"] = s.SessionToken
	}

	// Remove existing Authorization header
	DeleteAuthHeader(request)

	// Create canonical request
	parsedURL, _ := url.Parse(request.URL)
	canonicalURI := parsedURL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := parsedURL.RawQuery

	// Canonical headers
	var signedHeadersList []string
	canonicalHeaders := ""
	for k := range request.Headers {
		signedHeadersList = append(signedHeadersList, strings.ToLower(k))
	}
	signedHeadersList = append(signedHeadersList, "host")

	slices.Sort(signedHeadersList)

	for _, header := range signedHeadersList {
		if header == "host" {
			canonicalHeaders += fmt.Sprintf("host:%s\n", parsedURL.Host)
		} else {
			for k, v := range request.Headers {
				if strings.ToLower(k) == header {
					canonicalHeaders += fmt.Sprintf("%s:%s\n", header, strings.TrimSpace(v))
					break
				}
			}
		}
	}

	signedHeaders := strings.Join(signedHeadersList, ";")

	// Payload hash
	payloadHash := sha256Hash(request.RawBody)

	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// Create string to sign
	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s.Region, s.Service)
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hash(canonicalRequest),
	}, "\n")

	// Calculate signature
	signingKey := getSignatureKey(s.SecretAccessKey, dateStamp, s.Region, s.Service)
	signature := hmacSHA256Hex(signingKey, stringToSign)

	// Build authorization header
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, s.AccessKeyID, credentialScope, signedHeaders, signature)

	request.Headers["Authorization"] = authHeader

	return nil
}

// UpdateAuthHeader updates or sets the Authorization header
func UpdateAuthHeader(request *models.HttpRequest, value string) {
	for k := range request.Headers {
		if strings.EqualFold(k, "authorization") {
			request.Headers[k] = value
			return
		}
	}
	request.Headers["Authorization"] = value
}

// DeleteAuthHeader removes the Authorization header
func DeleteAuthHeader(request *models.HttpRequest) {
	for k := range request.Headers {
		if strings.EqualFold(k, "authorization") {
			delete(request.Headers, k)
			return
		}
	}
}

// GenerateNonce generates a random nonce for auth purposes
func GenerateNonce() string {
	b := make([]byte, 16)
	if _, err := cryptoRand.Read(b); err != nil {
		// Fallback: use time-based seed (shouldn't happen in practice)
		t := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(t >> (i % 8))
			t = t*1103515245 + 12345
		}
	}
	return hex.EncodeToString(b)
}

// Helper functions

func md5Hash(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

func getSignatureKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}
