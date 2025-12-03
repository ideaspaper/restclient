package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/md5"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ideaspaper/restclient/pkg/models"
)

// HTTPDoer defines the interface for sending HTTP requests.
// This abstraction enables dependency injection and easier testing.
type HTTPDoer interface {
	Send(request *models.HttpRequest) (*models.HttpResponse, error)
	SendWithContext(ctx context.Context, request *models.HttpRequest) (*models.HttpResponse, error)
	GetCookies(urlStr string) []*http.Cookie
	SetCookies(urlStr string, cookies []*http.Cookie)
	ClearCookies()
}

// Ensure HttpClient implements HTTPDoer
var _ HTTPDoer = (*HttpClient)(nil)

// ClientConfig holds client configuration
type ClientConfig struct {
	Timeout         time.Duration
	FollowRedirects bool
	InsecureSSL     bool
	Proxy           string
	ExcludeProxy    []string
	RememberCookies bool
	DefaultHeaders  map[string]string
	Certificates    map[string]Certificate
}

// Certificate holds TLS certificate configuration
type Certificate struct {
	Cert       string
	Key        string
	PFX        string
	Passphrase string
}

// DefaultConfig returns a default client configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:         0, // No timeout
		FollowRedirects: true,
		InsecureSSL:     false,
		RememberCookies: true,
		DefaultHeaders: map[string]string{
			"User-Agent": "restclient-cli",
		},
		Certificates: make(map[string]Certificate),
	}
}

// HttpClient is an HTTP client with auth support
type HttpClient struct {
	config      *ClientConfig
	client      *http.Client
	cookieJar   *cookiejar.Jar
	digestCreds map[string]digestCredentials
}

// NewHttpClient creates a new HTTP client
func NewHttpClient(config *ClientConfig) (*HttpClient, error) {
	if config == nil {
		config = DefaultConfig()
	}

	var jar *cookiejar.Jar
	if config.RememberCookies {
		var err error
		jar, err = cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create cookie jar: %w", err)
		}
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSSL,
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Configure proxy
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", config.Proxy, err)
		}
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			// Check excluded hosts
			for _, exclude := range config.ExcludeProxy {
				if strings.EqualFold(req.URL.Host, exclude) ||
					strings.HasSuffix(strings.ToLower(req.URL.Host), "."+strings.ToLower(exclude)) {
					return nil, nil
				}
			}
			return proxyURL, nil
		}
	}

	client := &http.Client{
		Transport: transport,
		Jar:       jar, // Will be nil if RememberCookies is false
	}

	if config.Timeout > 0 {
		client.Timeout = config.Timeout
	}

	if !config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &HttpClient{
		config:      config,
		client:      client,
		cookieJar:   jar,
		digestCreds: make(map[string]digestCredentials),
	}, nil
}

// Send sends an HTTP request and returns the response
func (c *HttpClient) Send(request *models.HttpRequest) (*models.HttpResponse, error) {
	return c.SendWithContext(context.Background(), request)
}

// SendWithContext sends an HTTP request with context
func (c *HttpClient) SendWithContext(ctx context.Context, request *models.HttpRequest) (*models.HttpResponse, error) {
	// Process authentication before sending
	if err := c.processAuth(request); err != nil {
		return nil, fmt.Errorf("auth processing failed: %w", err)
	}

	// Apply default headers
	for k, v := range c.config.DefaultHeaders {
		if _, exists := request.Headers[k]; !exists {
			request.Headers[k] = v
		}
	}

	// Create standard HTTP request
	var bodyReader io.Reader
	var contentType string

	// Check if we have multipart parts to send
	if len(request.MultipartParts) > 0 {
		var err error
		bodyReader, contentType, err = c.createMultipartBody(request.MultipartParts)
		if err != nil {
			return nil, fmt.Errorf("failed to create multipart body: %w", err)
		}
	} else if request.RawBody != "" {
		bodyReader = strings.NewReader(request.RawBody)
	} else if request.Body != nil {
		// Check if the Body is a non-nil interface holding a nil pointer
		switch v := request.Body.(type) {
		case *strings.Reader:
			if v != nil {
				bodyReader = v
			}
		default:
			bodyReader = request.Body
		}
	}

	req, err := http.NewRequestWithContext(ctx, request.Method, request.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range request.Headers {
		req.Header.Set(k, v)
	}

	// Override Content-Type for multipart if we generated it
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	timing := models.ResponseTiming{}
	startTime := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	timing.Total = time.Since(startTime)

	// Handle Digest auth challenge (HTTP 401)
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		if strings.HasPrefix(strings.ToLower(authHeader), "digest ") {
			// Retry with digest auth if we have credentials
			if digestResp, err := c.handleDigestAuth(ctx, request, resp, authHeader); err == nil {
				// Close the original response body before replacing
				resp.Body.Close()
				resp = digestResp
				// Note: the new response body will be closed by the defer above
				// since we reassigned resp
			} else if _, hasCredentials := c.digestCreds[request.URL]; hasCredentials {
				// Log warning if retry failed but credentials were provided
				fmt.Fprintf(os.Stderr, "Warning: digest auth retry failed: %v\n", err)
			}
		}
	}

	var bodyBuffer bytes.Buffer
	reader := resp.Body

	// Handle gzip encoding
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err == nil {
			reader = gzReader
			defer gzReader.Close()
		}
	}

	_, err = io.Copy(&bodyBuffer, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return models.NewHttpResponse(resp, bodyBuffer.Bytes(), timing, request), nil
}

// processAuth processes authentication headers
func (c *HttpClient) processAuth(request *models.HttpRequest) error {
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
		return c.processBasicAuth(request, parts[1:])
	case "digest":
		// Digest auth is handled during response (see handleDigestAuth)
		// Just store credentials for later
		return c.storeDigestCredentials(request, parts[1:])
	case "aws":
		return c.processAWSAuth(request, parts[1:])
	}

	return nil
}

// processBasicAuth processes Basic authentication
func (c *HttpClient) processBasicAuth(request *models.HttpRequest, args []string) error {
	if len(args) == 0 {
		return nil
	}

	credentials := args[0]

	// Check if already base64 encoded
	if _, err := base64.StdEncoding.DecodeString(credentials); err == nil {
		// Already encoded, use as-is
		updateAuthHeader(request, "Basic "+credentials)
		return nil
	}

	// Check for username:password format
	if strings.Contains(credentials, ":") {
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
		updateAuthHeader(request, "Basic "+encoded)
		return nil
	}

	// username password format (two separate args)
	if len(args) >= 2 {
		creds := args[0] + ":" + strings.Join(args[1:], " ")
		encoded := base64.StdEncoding.EncodeToString([]byte(creds))
		updateAuthHeader(request, "Basic "+encoded)
	}

	return nil
}

// digestCredentials stores digest auth credentials
type digestCredentials struct {
	username string
	password string
}

// storeDigestCredentials stores digest credentials for later use
func (c *HttpClient) storeDigestCredentials(request *models.HttpRequest, args []string) error {
	if len(args) < 2 {
		return nil
	}

	c.digestCreds[request.URL] = digestCredentials{
		username: args[0],
		password: strings.Join(args[1:], " "),
	}

	// Remove the Authorization header for now (will be added after challenge)
	deleteAuthHeader(request)
	return nil
}

// handleDigestAuth handles Digest authentication challenge
func (c *HttpClient) handleDigestAuth(ctx context.Context, request *models.HttpRequest, resp *http.Response, authHeader string) (*http.Response, error) {
	creds, ok := c.digestCreds[request.URL]
	if !ok {
		return resp, nil
	}

	challenge := parseDigestChallenge(authHeader)

	uri, _ := url.Parse(request.URL)
	digestAuth := buildDigestResponse(
		creds.username,
		creds.password,
		request.Method,
		uri.RequestURI(),
		challenge,
	)

	// Update request with digest auth
	updateAuthHeader(request, "Digest "+digestAuth)

	// Resend request - handle both regular body and multipart
	var bodyReader io.Reader
	var contentType string

	if len(request.MultipartParts) > 0 {
		var err error
		bodyReader, contentType, err = c.createMultipartBody(request.MultipartParts)
		if err != nil {
			return resp, fmt.Errorf("failed to create multipart body for digest retry: %w", err)
		}
	} else if request.RawBody != "" {
		bodyReader = strings.NewReader(request.RawBody)
	}

	req, err := http.NewRequestWithContext(ctx, request.Method, request.URL, bodyReader)
	if err != nil {
		return resp, err
	}

	for k, v := range request.Headers {
		req.Header.Set(k, v)
	}

	// Override Content-Type for multipart if we generated it
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.client.Do(req)
}

// digestChallenge holds parsed digest challenge
type digestChallenge struct {
	realm     string
	nonce     string
	opaque    string
	qop       string
	algorithm string
}

// parseDigestChallenge parses the WWW-Authenticate header
func parseDigestChallenge(header string) digestChallenge {
	challenge := digestChallenge{}

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
			challenge.realm = value
		case "nonce":
			challenge.nonce = value
		case "opaque":
			challenge.opaque = value
		case "qop":
			challenge.qop = value
		case "algorithm":
			challenge.algorithm = value
		}
	}

	return challenge
}

// buildDigestResponse builds the digest auth response
func buildDigestResponse(username, password, method, uri string, challenge digestChallenge) string {
	ha1 := md5Hash(fmt.Sprintf("%s:%s:%s", username, challenge.realm, password))

	if strings.EqualFold(challenge.algorithm, "md5-sess") {
		cnonce := generateNonce()
		ha1 = md5Hash(fmt.Sprintf("%s:%s:%s", ha1, challenge.nonce, cnonce))
	}

	ha2 := md5Hash(fmt.Sprintf("%s:%s", method, uri))

	var response string
	var params []string

	if strings.Contains(challenge.qop, "auth") {
		nc := "00000001"
		cnonce := generateNonce()
		response = md5Hash(fmt.Sprintf("%s:%s:%s:%s:auth:%s",
			ha1, challenge.nonce, nc, cnonce, ha2))

		params = []string{
			fmt.Sprintf(`username="%s"`, username),
			fmt.Sprintf(`realm="%s"`, challenge.realm),
			fmt.Sprintf(`nonce="%s"`, challenge.nonce),
			fmt.Sprintf(`uri="%s"`, uri),
			`qop=auth`,
			fmt.Sprintf(`nc=%s`, nc),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
			fmt.Sprintf(`response="%s"`, response),
		}
	} else {
		response = md5Hash(fmt.Sprintf("%s:%s:%s", ha1, challenge.nonce, ha2))

		params = []string{
			fmt.Sprintf(`username="%s"`, username),
			fmt.Sprintf(`realm="%s"`, challenge.realm),
			fmt.Sprintf(`nonce="%s"`, challenge.nonce),
			fmt.Sprintf(`uri="%s"`, uri),
			fmt.Sprintf(`response="%s"`, response),
		}
	}

	if challenge.opaque != "" {
		params = append(params, fmt.Sprintf(`opaque="%s"`, challenge.opaque))
	}

	if challenge.algorithm != "" {
		params = append(params, fmt.Sprintf(`algorithm=%s`, challenge.algorithm))
	}

	return strings.Join(params, ", ")
}

// processAWSAuth processes AWS Signature v4 authentication
func (c *HttpClient) processAWSAuth(request *models.HttpRequest, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("AWS auth requires accessKeyId and secretAccessKey")
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
		return fmt.Errorf("failed to parse URL for AWS auth: %w", err)
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
	signer := &awsSigner{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		sessionToken:    sessionToken,
		region:          region,
		service:         service,
	}

	return signer.sign(request)
}

// awsSigner implements AWS Signature v4
type awsSigner struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	region          string
	service         string
}

// sign signs the request with AWS Signature v4
func (s *awsSigner) sign(request *models.HttpRequest) error {
	t := time.Now().UTC()
	dateStamp := t.Format("20060102")
	amzDate := t.Format("20060102T150405Z")

	request.Headers["X-Amz-Date"] = amzDate
	if s.sessionToken != "" {
		request.Headers["X-Amz-Security-Token"] = s.sessionToken
	}

	// Remove existing Authorization header
	deleteAuthHeader(request)

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
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s.region, s.service)
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hash(canonicalRequest),
	}, "\n")

	// Calculate signature
	signingKey := getSignatureKey(s.secretAccessKey, dateStamp, s.region, s.service)
	signature := hmacSHA256Hex(signingKey, stringToSign)

	// Build authorization header
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, s.accessKeyID, credentialScope, signedHeaders, signature)

	request.Headers["Authorization"] = authHeader

	return nil
}

func updateAuthHeader(request *models.HttpRequest, value string) {
	for k := range request.Headers {
		if strings.EqualFold(k, "authorization") {
			request.Headers[k] = value
			return
		}
	}
	request.Headers["Authorization"] = value
}

func deleteAuthHeader(request *models.HttpRequest) {
	for k := range request.Headers {
		if strings.EqualFold(k, "authorization") {
			delete(request.Headers, k)
			return
		}
	}
}

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

func generateNonce() string {
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

// ClearCookies clears all stored cookies
func (c *HttpClient) ClearCookies() {
	if c.config.RememberCookies {
		jar, _ := cookiejar.New(nil)
		c.cookieJar = jar
		c.client.Jar = jar
	}
}

// GetCookies returns all cookies for a given URL
func (c *HttpClient) GetCookies(urlStr string) []*http.Cookie {
	if c.cookieJar == nil {
		return nil
	}
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}
	return c.cookieJar.Cookies(parsedURL)
}

// SetCookies sets cookies for a given URL
func (c *HttpClient) SetCookies(urlStr string, cookies []*http.Cookie) {
	if c.cookieJar == nil || len(cookies) == 0 {
		return
	}
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return
	}
	c.cookieJar.SetCookies(parsedURL, cookies)
}

// GetResponseCookies extracts cookies from an HTTP response
func GetResponseCookies(resp *http.Response) []*http.Cookie {
	if resp == nil {
		return nil
	}
	return resp.Cookies()
}

// createMultipartBody creates a multipart form body from parts
func (c *HttpClient) createMultipartBody(parts []models.MultipartPart) (io.Reader, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, part := range parts {
		if part.IsFile && part.FilePath != "" {
			// File upload
			file, err := os.Open(part.FilePath)
			if err != nil {
				return nil, "", fmt.Errorf("failed to open file %s: %w", part.FilePath, err)
			}
			defer file.Close()

			// Determine filename
			filename := part.FileName
			if filename == "" {
				filename = filepath.Base(part.FilePath)
			}

			// Create form file with proper headers
			var formWriter io.Writer
			if part.ContentType != "" {
				// Create custom part with content type
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, part.Name, filename))
				h.Set("Content-Type", part.ContentType)
				formWriter, err = writer.CreatePart(h)
			} else {
				formWriter, err = writer.CreateFormFile(part.Name, filename)
			}
			if err != nil {
				return nil, "", fmt.Errorf("failed to create form file: %w", err)
			}

			_, err = io.Copy(formWriter, file)
			if err != nil {
				return nil, "", fmt.Errorf("failed to copy file content: %w", err)
			}
		} else {
			// Regular form field
			err := writer.WriteField(part.Name, part.Value)
			if err != nil {
				return nil, "", fmt.Errorf("failed to write form field: %w", err)
			}
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}
