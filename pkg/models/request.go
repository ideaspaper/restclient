package models

import (
	"io"
	"net/http"
	"strings"
)

// HttpRequest represents an HTTP request
type HttpRequest struct {
	Method         string
	URL            string
	Headers        map[string]string
	Body           io.Reader
	RawBody        string
	Name           string
	IsCancelled    bool
	Metadata       RequestMetadata
	MultipartParts []MultipartPart // For multipart/form-data
}

// MultipartPart represents a part in a multipart/form-data request
type MultipartPart struct {
	Name        string // Field name
	Value       string // Field value (for text fields)
	FileName    string // Original filename (for file uploads)
	FilePath    string // Local path to the file
	ContentType string // MIME type of the content
	IsFile      bool   // Whether this is a file upload
}

// RequestMetadata holds request-level settings
type RequestMetadata struct {
	Name        string
	Note        string
	NoRedirect  bool
	NoCookieJar bool
	Prompts     []PromptVariable
}

// PromptVariable represents a variable that requires user input
type PromptVariable struct {
	Name        string
	Description string
	IsPassword  bool
}

// NewHttpRequest creates a new HttpRequest
func NewHttpRequest(method, url string, headers map[string]string, body io.Reader, rawBody, name string) *HttpRequest {
	return &HttpRequest{
		Method:      strings.ToUpper(method),
		URL:         url,
		Headers:     headers,
		Body:        body,
		RawBody:     rawBody,
		Name:        name,
		IsCancelled: false,
	}
}

// ContentType returns the content type of the request
func (r *HttpRequest) ContentType() string {
	for k, v := range r.Headers {
		if strings.EqualFold(k, "content-type") {
			return v
		}
	}
	return ""
}

// ToStdRequest converts HttpRequest to standard http.Request
func (r *HttpRequest) ToStdRequest() (*http.Request, error) {
	req, err := http.NewRequest(r.Method, r.URL, r.Body)
	if err != nil {
		return nil, err
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// HistoricalHttpRequest represents a saved request in history
type HistoricalHttpRequest struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body,omitempty"`
	StartTime int64             `json:"startTime"`
}

// NewHistoricalHttpRequest creates a historical request from an HttpRequest
func NewHistoricalHttpRequest(req *HttpRequest, startTime int64) *HistoricalHttpRequest {
	return &HistoricalHttpRequest{
		Method:    req.Method,
		URL:       req.URL,
		Headers:   req.Headers,
		Body:      req.RawBody,
		StartTime: startTime,
	}
}
