package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
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
	"strings"
	"time"

	"github.com/ideaspaper/restclient/internal/constants"
	"github.com/ideaspaper/restclient/pkg/auth"
	"github.com/ideaspaper/restclient/pkg/errors"
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
			constants.HeaderUserAgent: constants.DefaultUserAgent,
		},
		Certificates: make(map[string]Certificate),
	}
}

// HttpClient is an HTTP client with auth support
type HttpClient struct {
	config        *ClientConfig
	client        *http.Client
	cookieJar     *cookiejar.Jar
	authProcessor *auth.Processor
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
			return nil, errors.Wrap(err, "failed to create cookie jar")
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
			return nil, errors.NewValidationErrorWithValue("proxy URL", config.Proxy, err.Error())
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
		config:        config,
		client:        client,
		cookieJar:     jar,
		authProcessor: auth.NewProcessor(),
	}, nil
}

// Send sends an HTTP request and returns the response
func (c *HttpClient) Send(request *models.HttpRequest) (*models.HttpResponse, error) {
	return c.SendWithContext(context.Background(), request)
}

// SendWithContext sends an HTTP request with context
func (c *HttpClient) SendWithContext(ctx context.Context, request *models.HttpRequest) (*models.HttpResponse, error) {
	// Process authentication before sending
	if err := c.authProcessor.ProcessAuth(request); err != nil {
		return nil, errors.NewRequestErrorWithURL("auth", request.Method, request.URL, err)
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
			return nil, errors.NewRequestErrorWithURL("multipart", request.Method, request.URL, err)
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
		return nil, errors.NewRequestErrorWithURL("build", request.Method, request.URL, err)
	}

	for k, v := range request.Headers {
		req.Header.Set(k, v)
	}

	// Override Content-Type for multipart if we generated it
	if contentType != "" {
		req.Header.Set(constants.HeaderContentType, contentType)
	}

	timing := models.ResponseTiming{}
	startTime := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.NewRequestErrorWithURL("send", request.Method, request.URL, err)
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
			} else if _, hasCredentials := c.authProcessor.GetDigestCredentials(request.URL); hasCredentials {
				// Log warning if retry failed but credentials were provided
				fmt.Fprintf(os.Stderr, "Warning: digest auth retry failed: %v\n", err)
			}
		}
	}

	var bodyBuffer bytes.Buffer
	reader := resp.Body

	// Handle gzip encoding
	if resp.Header.Get(constants.HeaderContentEncoding) == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err == nil {
			reader = gzReader
			defer gzReader.Close()
		}
	}

	_, err = io.Copy(&bodyBuffer, reader)
	if err != nil {
		return nil, errors.NewRequestErrorWithURL("read response", request.Method, request.URL, err)
	}

	return models.NewHttpResponse(resp, bodyBuffer.Bytes(), timing, request), nil
}

// handleDigestAuth handles Digest authentication challenge
func (c *HttpClient) handleDigestAuth(ctx context.Context, request *models.HttpRequest, resp *http.Response, authHeader string) (*http.Response, error) {
	creds, ok := c.authProcessor.GetDigestCredentials(request.URL)
	if !ok {
		return resp, nil
	}

	challenge := auth.ParseDigestChallenge(authHeader)

	uri, _ := url.Parse(request.URL)
	digestAuth := auth.BuildDigestAuth(
		creds.Username,
		creds.Password,
		request.Method,
		uri.RequestURI(),
		challenge,
	)

	// Update request with digest auth
	auth.UpdateAuthHeader(request, "Digest "+digestAuth)

	// Resend request - handle both regular body and multipart
	var bodyReader io.Reader
	var contentType string

	if len(request.MultipartParts) > 0 {
		var err error
		bodyReader, contentType, err = c.createMultipartBody(request.MultipartParts)
		if err != nil {
			return resp, errors.Wrap(err, "failed to create multipart body for digest retry")
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
		req.Header.Set(constants.HeaderContentType, contentType)
	}

	return c.client.Do(req)
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
				return nil, "", errors.Wrapf(err, "failed to open file %s", part.FilePath)
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
				h.Set(constants.HeaderContentType, part.ContentType)
				formWriter, err = writer.CreatePart(h)
			} else {
				formWriter, err = writer.CreateFormFile(part.Name, filename)
			}
			if err != nil {
				return nil, "", errors.Wrap(err, "failed to create form file")
			}

			_, err = io.Copy(formWriter, file)
			if err != nil {
				return nil, "", errors.Wrap(err, "failed to copy file content")
			}
		} else {
			// Regular form field
			err := writer.WriteField(part.Name, part.Value)
			if err != nil {
				return nil, "", errors.Wrap(err, "failed to write form field")
			}
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to close multipart writer")
	}

	return body, writer.FormDataContentType(), nil
}
