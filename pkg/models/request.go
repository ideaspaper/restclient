package models

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// ValidationError represents a request validation error
type ValidationError struct {
	Field   string // Field that failed validation (e.g., "URL", "Header:Content-Type")
	Message string // Description of the validation error
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains all validation errors for a request
type ValidationResult struct {
	Errors []ValidationError
}

// IsValid returns true if there are no validation errors
func (v *ValidationResult) IsValid() bool {
	return len(v.Errors) == 0
}

// AddError adds a validation error to the result
func (v *ValidationResult) AddError(field, message string) {
	v.Errors = append(v.Errors, ValidationError{Field: field, Message: message})
}

// Error returns a combined error message for all validation errors
func (v *ValidationResult) Error() string {
	if v.IsValid() {
		return ""
	}
	var msgs []string
	for _, err := range v.Errors {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

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
	PreScript   string // JavaScript to run before the request
	PostScript  string // JavaScript to run after the response
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

// validMethods contains all valid HTTP methods
var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true, "CONNECT": true,
	"TRACE": true, "LOCK": true, "UNLOCK": true, "PROPFIND": true,
	"PROPPATCH": true, "COPY": true, "MOVE": true, "MKCOL": true,
	"MKCALENDAR": true, "ACL": true, "SEARCH": true,
}

// headerNameRegex validates header names (RFC 7230)
var headerNameRegex = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)

// Validate validates the HTTP request and returns any validation errors
func (r *HttpRequest) Validate() *ValidationResult {
	result := &ValidationResult{}

	// Validate method
	if r.Method == "" {
		result.AddError("Method", "method is required")
	} else if !validMethods[strings.ToUpper(r.Method)] {
		result.AddError("Method", fmt.Sprintf("invalid HTTP method: %s", r.Method))
	}

	// Validate URL
	r.validateURL(result)

	// Validate headers
	r.validateHeaders(result)

	return result
}

// validateURL validates the request URL
func (r *HttpRequest) validateURL(result *ValidationResult) {
	if r.URL == "" {
		result.AddError("URL", "URL is required")
		return
	}

	// Check for unresolved variables (common mistake)
	if strings.Contains(r.URL, "{{") && strings.Contains(r.URL, "}}") {
		result.AddError("URL", "URL contains unresolved variables (check your environment configuration)")
		return
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(r.URL)
	if err != nil {
		result.AddError("URL", fmt.Sprintf("invalid URL: %v", err))
		return
	}

	// Check for scheme
	if parsedURL.Scheme == "" {
		result.AddError("URL", "URL must include scheme (http:// or https://)")
		return
	}

	// Validate scheme
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		result.AddError("URL", fmt.Sprintf("unsupported URL scheme: %s (use http or https)", parsedURL.Scheme))
		return
	}

	// Check for host
	if parsedURL.Host == "" {
		result.AddError("URL", "URL must include a host")
		return
	}

	// Check for spaces in URL (common mistake)
	if strings.Contains(r.URL, " ") {
		result.AddError("URL", "URL contains spaces (URLs should be properly encoded)")
	}
}

// validateHeaders validates request headers
func (r *HttpRequest) validateHeaders(result *ValidationResult) {
	for name, value := range r.Headers {
		// Validate header name
		if name == "" {
			result.AddError("Header", "header name cannot be empty")
			continue
		}

		// Check for valid header name characters (RFC 7230)
		if !headerNameRegex.MatchString(name) {
			result.AddError(fmt.Sprintf("Header:%s", name), "header name contains invalid characters")
		}

		// Check for unresolved variables in header value
		if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
			result.AddError(fmt.Sprintf("Header:%s", name), "header value contains unresolved variables")
		}

		// Validate specific headers
		lowerName := strings.ToLower(name)
		switch lowerName {
		case "content-type":
			if value == "" {
				result.AddError("Header:Content-Type", "Content-Type header is empty")
			}
		case "authorization":
			// Check for placeholder values
			lowerValue := strings.ToLower(value)
			if strings.Contains(lowerValue, "your-token") ||
				strings.Contains(lowerValue, "your_token") ||
				strings.Contains(lowerValue, "<token>") ||
				strings.Contains(lowerValue, "[token]") {
				result.AddError("Header:Authorization", "Authorization header appears to contain a placeholder value")
			}
		}
	}
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
