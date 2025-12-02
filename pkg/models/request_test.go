package models

import (
	"testing"
)

func TestHttpRequestValidate(t *testing.T) {
	tests := []struct {
		name       string
		request    *HttpRequest
		wantValid  bool
		wantFields []string // Expected error fields
	}{
		{
			name: "valid GET request",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "https://api.example.com/users",
				Headers: map[string]string{"Accept": "application/json"},
			},
			wantValid: true,
		},
		{
			name: "valid POST request with body",
			request: &HttpRequest{
				Method:  "POST",
				URL:     "https://api.example.com/users",
				Headers: map[string]string{"Content-Type": "application/json"},
				RawBody: `{"name": "John"}`,
			},
			wantValid: true,
		},
		{
			name: "empty method",
			request: &HttpRequest{
				Method:  "",
				URL:     "https://api.example.com/users",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"Method"},
		},
		{
			name: "invalid method",
			request: &HttpRequest{
				Method:  "INVALID",
				URL:     "https://api.example.com/users",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"Method"},
		},
		{
			name: "empty URL",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"URL"},
		},
		{
			name: "URL without scheme",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "api.example.com/users",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"URL"},
		},
		{
			name: "URL with invalid scheme",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "ftp://api.example.com/users",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"URL"},
		},
		{
			name: "URL with unresolved variable",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "https://{{baseUrl}}/users",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"URL"},
		},
		{
			name: "URL with spaces",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "https://api.example.com/users with spaces",
				Headers: map[string]string{},
			},
			wantValid:  false,
			wantFields: []string{"URL"},
		},
		{
			name: "header with unresolved variable",
			request: &HttpRequest{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"Authorization": "Bearer {{token}}",
				},
			},
			wantValid:  false,
			wantFields: []string{"Header:Authorization"},
		},
		{
			name: "authorization header with placeholder",
			request: &HttpRequest{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"Authorization": "Bearer your-token-here",
				},
			},
			wantValid:  false,
			wantFields: []string{"Header:Authorization"},
		},
		{
			name: "empty Content-Type header",
			request: &HttpRequest{
				Method: "POST",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"Content-Type": "",
				},
			},
			wantValid:  false,
			wantFields: []string{"Header:Content-Type"},
		},
		{
			name: "multiple validation errors",
			request: &HttpRequest{
				Method: "",
				URL:    "",
				Headers: map[string]string{
					"Authorization": "{{token}}",
				},
			},
			wantValid:  false,
			wantFields: []string{"Method", "URL", "Header:Authorization"},
		},
		{
			name: "valid with http scheme",
			request: &HttpRequest{
				Method:  "GET",
				URL:     "http://localhost:8080/api",
				Headers: map[string]string{},
			},
			wantValid: true,
		},
		{
			name: "valid all HTTP methods",
			request: &HttpRequest{
				Method:  "DELETE",
				URL:     "https://api.example.com/users/1",
				Headers: map[string]string{},
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.request.Validate()

			if result.IsValid() != tt.wantValid {
				t.Errorf("Validate() IsValid = %v, want %v", result.IsValid(), tt.wantValid)
				if !result.IsValid() {
					t.Logf("Validation errors: %v", result.Error())
				}
			}

			if !tt.wantValid {
				// Check that expected fields are in errors
				for _, field := range tt.wantFields {
					found := false
					for _, err := range result.Errors {
						if err.Field == field {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected validation error for field %q, but not found", field)
					}
				}
			}
		})
	}
}

func TestValidationResultError(t *testing.T) {
	tests := []struct {
		name    string
		result  *ValidationResult
		wantErr string
	}{
		{
			name:    "empty result",
			result:  &ValidationResult{},
			wantErr: "",
		},
		{
			name: "single error",
			result: &ValidationResult{
				Errors: []ValidationError{
					{Field: "URL", Message: "URL is required"},
				},
			},
			wantErr: "URL: URL is required",
		},
		{
			name: "multiple errors",
			result: &ValidationResult{
				Errors: []ValidationError{
					{Field: "Method", Message: "method is required"},
					{Field: "URL", Message: "URL is required"},
				},
			},
			wantErr: "Method: method is required; URL: URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Error()
			if got != tt.wantErr {
				t.Errorf("Error() = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestValidationErrorString(t *testing.T) {
	err := ValidationError{Field: "URL", Message: "URL is required"}
	want := "URL: URL is required"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestValidateAllMethods(t *testing.T) {
	validMethods := []string{
		"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS",
		"CONNECT", "TRACE", "LOCK", "UNLOCK", "PROPFIND", "PROPPATCH",
		"COPY", "MOVE", "MKCOL", "MKCALENDAR", "ACL", "SEARCH",
	}

	for _, method := range validMethods {
		t.Run(method, func(t *testing.T) {
			req := &HttpRequest{
				Method:  method,
				URL:     "https://api.example.com",
				Headers: map[string]string{},
			}
			result := req.Validate()
			if !result.IsValid() {
				t.Errorf("Method %s should be valid, got errors: %v", method, result.Error())
			}
		})
	}
}
