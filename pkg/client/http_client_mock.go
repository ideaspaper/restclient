// Package client provides HTTP client implementations.
package client

import (
	"context"
	"net/http"
	"sync"

	"github.com/ideaspaper/restclient/pkg/models"
)

// MockHTTPClient is a mock implementation of HTTPDoer for testing.
// It allows setting up expected responses and recording request history.
type MockHTTPClient struct {
	mu sync.Mutex

	// Response is the response to return from Send/SendWithContext.
	Response *models.HttpResponse

	// Error is the error to return from Send/SendWithContext.
	Error error

	// Requests records all requests made to this mock.
	Requests []*models.HttpRequest

	// ResponseFunc allows dynamic response generation based on the request.
	// If set, it takes precedence over Response/Error fields.
	ResponseFunc func(req *models.HttpRequest) (*models.HttpResponse, error)

	// Cookies stores cookies by URL for GetCookies/SetCookies.
	Cookies map[string][]*http.Cookie
}

// Ensure MockHTTPClient implements HTTPDoer
var _ HTTPDoer = (*MockHTTPClient)(nil)

// NewMockHTTPClient creates a new MockHTTPClient.
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		Requests: make([]*models.HttpRequest, 0),
		Cookies:  make(map[string][]*http.Cookie),
	}
}

// Send sends an HTTP request and returns the mock response.
func (m *MockHTTPClient) Send(request *models.HttpRequest) (*models.HttpResponse, error) {
	return m.SendWithContext(context.Background(), request)
}

// SendWithContext sends an HTTP request with context and returns the mock response.
func (m *MockHTTPClient) SendWithContext(ctx context.Context, request *models.HttpRequest) (*models.HttpResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request
	m.Requests = append(m.Requests, request)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use ResponseFunc if provided
	if m.ResponseFunc != nil {
		return m.ResponseFunc(request)
	}

	return m.Response, m.Error
}

// GetCookies returns cookies for the given URL.
func (m *MockHTTPClient) GetCookies(urlStr string) []*http.Cookie {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Cookies[urlStr]
}

// SetCookies sets cookies for the given URL.
func (m *MockHTTPClient) SetCookies(urlStr string, cookies []*http.Cookie) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Cookies[urlStr] = cookies
}

// ClearCookies removes all stored cookies.
func (m *MockHTTPClient) ClearCookies() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Cookies = make(map[string][]*http.Cookie)
}

// --- Helper methods for test setup ---

// WithResponse sets the response to return and returns the mock for chaining.
func (m *MockHTTPClient) WithResponse(resp *models.HttpResponse) *MockHTTPClient {
	m.Response = resp
	return m
}

// WithError sets the error to return and returns the mock for chaining.
func (m *MockHTTPClient) WithError(err error) *MockHTTPClient {
	m.Error = err
	return m
}

// WithResponseFunc sets a dynamic response function and returns the mock for chaining.
func (m *MockHTTPClient) WithResponseFunc(fn func(req *models.HttpRequest) (*models.HttpResponse, error)) *MockHTTPClient {
	m.ResponseFunc = fn
	return m
}

// Reset clears all recorded requests and stored cookies.
func (m *MockHTTPClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Requests = make([]*models.HttpRequest, 0)
	m.Cookies = make(map[string][]*http.Cookie)
	m.Response = nil
	m.Error = nil
	m.ResponseFunc = nil
}

// RequestCount returns the number of requests recorded.
func (m *MockHTTPClient) RequestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.Requests)
}

// LastRequest returns the most recent request, or nil if none recorded.
func (m *MockHTTPClient) LastRequest() *models.HttpRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.Requests) == 0 {
		return nil
	}
	return m.Requests[len(m.Requests)-1]
}
