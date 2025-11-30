package output

import (
	"strings"
	"testing"

	"github.com/ideaspaper/restclient/pkg/models"
)

func TestNewFormatter(t *testing.T) {
	// Test with colors enabled
	f := NewFormatter(true)
	if f == nil {
		t.Fatal("Formatter should not be nil")
	}
	if !f.colorEnabled {
		t.Error("colorEnabled should be true")
	}

	// Test with colors disabled
	f = NewFormatter(false)
	if f.colorEnabled {
		t.Error("colorEnabled should be false")
	}
}

func TestFormatResponse(t *testing.T) {
	f := NewFormatter(false) // Disable colors for easier testing

	resp := &models.HttpResponse{
		StatusCode:      200,
		HttpVersion:     "HTTP/1.1",
		Headers:         map[string][]string{"Content-Type": {"application/json"}},
		Body:            `{"message": "success"}`,
		BodySizeInBytes: 21,
	}

	formatted := f.FormatResponse(resp)

	if !strings.Contains(formatted, "HTTP/1.1 200 OK") {
		t.Error("Should contain status line")
	}
	if !strings.Contains(formatted, "Content-Type: application/json") {
		t.Error("Should contain headers")
	}
	if !strings.Contains(formatted, "message") {
		t.Error("Should contain body")
	}
	if !strings.Contains(formatted, "21 bytes") {
		t.Error("Should contain size info")
	}
}

func TestFormatHeaders(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{
			"Content-Type":   {"application/json"},
			"Content-Length": {"100"},
			"X-Custom":       {"value1", "value2"},
		},
	}

	formatted := f.FormatHeaders(resp)

	if !strings.Contains(formatted, "Content-Type: application/json") {
		t.Error("Should contain Content-Type header")
	}
	if !strings.Contains(formatted, "Content-Length: 100") {
		t.Error("Should contain Content-Length header")
	}
	// Multi-value headers
	if !strings.Contains(formatted, "X-Custom: value1") {
		t.Error("Should contain first X-Custom value")
	}
	if !strings.Contains(formatted, "X-Custom: value2") {
		t.Error("Should contain second X-Custom value")
	}
}

func TestFormatBody_JSON(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    `{"name":"test","count":42,"active":true,"data":null}`,
	}

	formatted := f.FormatBody(resp)

	// Should be pretty-printed
	if !strings.Contains(formatted, "\"name\": \"test\"") {
		t.Error("JSON should be pretty-printed")
	}
	if !strings.Contains(formatted, "\"count\": 42") {
		t.Error("Should contain number")
	}
	if !strings.Contains(formatted, "\"active\": true") {
		t.Error("Should contain boolean")
	}
	if !strings.Contains(formatted, "\"data\": null") {
		t.Error("Should contain null")
	}
}

func TestFormatBody_InvalidJSON(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    `{invalid json}`,
	}

	formatted := f.FormatBody(resp)

	// Should return original body for invalid JSON
	if formatted != `{invalid json}` {
		t.Errorf("Invalid JSON should return original body, got: %s", formatted)
	}
}

func TestFormatBody_XML(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{"Content-Type": {"application/xml"}},
		Body:    `<root><item>test</item></root>`,
	}

	formatted := f.FormatBody(resp)

	// Currently returns as-is (no XML formatting implemented)
	if formatted != `<root><item>test</item></root>` {
		t.Error("XML should be returned as-is")
	}
}

func TestFormatBody_PlainText(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{"Content-Type": {"text/plain"}},
		Body:    "Hello, World!",
	}

	formatted := f.FormatBody(resp)

	if formatted != "Hello, World!" {
		t.Errorf("Plain text should be returned as-is, got: %s", formatted)
	}
}

func TestFormatStatusLine(t *testing.T) {
	f := NewFormatter(false)

	tests := []struct {
		statusCode int
		httpVer    string
		want       string
	}{
		{200, "HTTP/1.1", "HTTP/1.1 200 OK"},
		{201, "HTTP/1.1", "HTTP/1.1 201 Created"},
		{301, "HTTP/1.1", "HTTP/1.1 301 Moved Permanently"},
		{400, "HTTP/1.1", "HTTP/1.1 400 Bad Request"},
		{404, "HTTP/2", "HTTP/2 404 Not Found"},
		{500, "HTTP/1.1", "HTTP/1.1 500 Internal Server Error"},
		{418, "HTTP/1.1", "HTTP/1.1 418 "}, // Unknown status
	}

	for _, tt := range tests {
		resp := &models.HttpResponse{
			StatusCode:  tt.statusCode,
			HttpVersion: tt.httpVer,
		}
		got := f.formatStatusLine(resp)
		if got != tt.want {
			t.Errorf("formatStatusLine(%d) = %q, want %q", tt.statusCode, got, tt.want)
		}
	}
}

func TestGetStatusColor(t *testing.T) {
	f := NewFormatter(true)

	tests := []struct {
		code    int
		isNil   bool
		colorIs string
	}{
		{200, false, "success"},
		{201, false, "success"},
		{301, false, "redirect"},
		{302, false, "redirect"},
		{400, false, "clientError"},
		{404, false, "clientError"},
		{500, false, "serverError"},
		{503, false, "serverError"},
		{100, true, "none"},
	}

	for _, tt := range tests {
		got := f.getStatusColor(tt.code)
		if tt.isNil && got != nil {
			t.Errorf("getStatusColor(%d) should be nil", tt.code)
		}
		if !tt.isNil && got == nil {
			t.Errorf("getStatusColor(%d) should not be nil", tt.code)
		}
	}
}

func TestGetStatusText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{999, ""}, // Unknown
	}

	for _, tt := range tests {
		got := getStatusText(tt.code)
		if got != tt.want {
			t.Errorf("getStatusText(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestFormatError(t *testing.T) {
	f := NewFormatter(false)

	err := &testError{msg: "connection refused"}
	formatted := f.FormatError(err)

	if formatted != "Error: connection refused" {
		t.Errorf("FormatError = %q, want 'Error: connection refused'", formatted)
	}
}

func TestFormatSuccess(t *testing.T) {
	f := NewFormatter(false)

	formatted := f.FormatSuccess("Operation completed")

	if formatted != "Operation completed" {
		t.Errorf("FormatSuccess = %q, want 'Operation completed'", formatted)
	}
}

func TestFormatInfo(t *testing.T) {
	f := NewFormatter(false)

	formatted := f.FormatInfo("Processing request...")

	if formatted != "Processing request..." {
		t.Errorf("FormatInfo = %q, want 'Processing request...'", formatted)
	}
}

func TestColorizeJSON(t *testing.T) {
	f := NewFormatter(true)

	jsonStr := `{
  "name": "test",
  "count": 42,
  "active": true,
  "data": null
}`

	colorized := f.colorizeJSON(jsonStr)

	// Should not be empty
	if colorized == "" {
		t.Error("colorizeJSON should return non-empty string")
	}

	// Length should be >= original (colors add escape sequences)
	if len(colorized) < len(jsonStr) {
		t.Error("Colorized output should be at least as long as input")
	}
}

func TestFormatJSON_NestedObjects(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    `{"user":{"name":"John","age":30},"items":[1,2,3]}`,
	}

	formatted := f.FormatBody(resp)

	if !strings.Contains(formatted, "\"user\":") {
		t.Error("Should contain nested object")
	}
	if !strings.Contains(formatted, "\"name\": \"John\"") {
		t.Error("Should contain nested property")
	}
	if !strings.Contains(formatted, "\"items\":") {
		t.Error("Should contain array")
	}
}

func TestFormatJSON_EmptyBody(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    "",
	}

	formatted := f.FormatBody(resp)

	if formatted != "" {
		t.Errorf("Empty body should return empty string, got: %s", formatted)
	}
}

func TestFormatTiming(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		BodySizeInBytes: 1024,
		Timing: models.ResponseTiming{
			Total: 150000000, // 150ms as nanoseconds
		},
	}

	formatted := f.formatTiming(resp)

	if !strings.Contains(formatted, "1024 bytes") {
		t.Error("Should contain body size")
	}
	if !strings.Contains(formatted, "Total:") {
		t.Error("Should contain timing info")
	}
}

func TestFormatTiming_NoTotal(t *testing.T) {
	f := NewFormatter(false)

	resp := &models.HttpResponse{
		BodySizeInBytes: 512,
		Timing:          models.ResponseTiming{},
	}

	formatted := f.formatTiming(resp)

	if !strings.Contains(formatted, "512 bytes") {
		t.Error("Should contain body size")
	}
}

// Helper test error type
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
