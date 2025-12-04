package httputil

import (
	"testing"
)

func TestGetHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		key     string
		wantVal string
		wantOK  bool
	}{
		{
			name:    "exact match",
			headers: map[string]string{"Content-Type": "application/json"},
			key:     "Content-Type",
			wantVal: "application/json",
			wantOK:  true,
		},
		{
			name:    "case insensitive lowercase",
			headers: map[string]string{"Content-Type": "application/json"},
			key:     "content-type",
			wantVal: "application/json",
			wantOK:  true,
		},
		{
			name:    "case insensitive uppercase",
			headers: map[string]string{"content-type": "application/json"},
			key:     "CONTENT-TYPE",
			wantVal: "application/json",
			wantOK:  true,
		},
		{
			name:    "not found",
			headers: map[string]string{"Content-Type": "application/json"},
			key:     "Authorization",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "empty headers",
			headers: map[string]string{},
			key:     "Content-Type",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "nil headers",
			headers: nil,
			key:     "Content-Type",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "mixed case in headers",
			headers: map[string]string{"X-Custom-Header": "value123"},
			key:     "x-custom-header",
			wantVal: "value123",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOK := GetHeader(tt.headers, tt.key)
			if gotVal != tt.wantVal {
				t.Errorf("GetHeader() value = %v, want %v", gotVal, tt.wantVal)
			}
			if gotOK != tt.wantOK {
				t.Errorf("GetHeader() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestGetHeaderFromSlice(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		key     string
		wantVal string
		wantOK  bool
	}{
		{
			name:    "exact match single value",
			headers: map[string][]string{"Content-Type": {"application/json"}},
			key:     "Content-Type",
			wantVal: "application/json",
			wantOK:  true,
		},
		{
			name:    "multiple values returns first",
			headers: map[string][]string{"Accept": {"text/html", "application/json"}},
			key:     "Accept",
			wantVal: "text/html",
			wantOK:  true,
		},
		{
			name:    "case insensitive",
			headers: map[string][]string{"Content-Type": {"application/json"}},
			key:     "content-type",
			wantVal: "application/json",
			wantOK:  true,
		},
		{
			name:    "not found",
			headers: map[string][]string{"Content-Type": {"application/json"}},
			key:     "Authorization",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "empty slice",
			headers: map[string][]string{"Content-Type": {}},
			key:     "Content-Type",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "nil headers",
			headers: nil,
			key:     "Content-Type",
			wantVal: "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOK := GetHeaderFromSlice(tt.headers, tt.key)
			if gotVal != tt.wantVal {
				t.Errorf("GetHeaderFromSlice() value = %v, want %v", gotVal, tt.wantVal)
			}
			if gotOK != tt.wantOK {
				t.Errorf("GetHeaderFromSlice() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestHasHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		key     string
		want    bool
	}{
		{
			name:    "header exists",
			headers: map[string]string{"Content-Type": "application/json"},
			key:     "Content-Type",
			want:    true,
		},
		{
			name:    "header exists case insensitive",
			headers: map[string]string{"Content-Type": "application/json"},
			key:     "content-type",
			want:    true,
		},
		{
			name:    "header not exists",
			headers: map[string]string{"Content-Type": "application/json"},
			key:     "Authorization",
			want:    false,
		},
		{
			name:    "empty headers",
			headers: map[string]string{},
			key:     "Content-Type",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasHeader(tt.headers, tt.key); got != tt.want {
				t.Errorf("HasHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasHeaderFromSlice(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		key     string
		want    bool
	}{
		{
			name:    "header exists with values",
			headers: map[string][]string{"Content-Type": {"application/json"}},
			key:     "Content-Type",
			want:    true,
		},
		{
			name:    "header exists case insensitive",
			headers: map[string][]string{"Content-Type": {"application/json"}},
			key:     "content-type",
			want:    true,
		},
		{
			name:    "header exists but empty",
			headers: map[string][]string{"Content-Type": {}},
			key:     "Content-Type",
			want:    false,
		},
		{
			name:    "header not exists",
			headers: map[string][]string{"Content-Type": {"application/json"}},
			key:     "Authorization",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasHeaderFromSlice(tt.headers, tt.key); got != tt.want {
				t.Errorf("HasHeaderFromSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetHeader(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		key         string
		value       string
		wantHeaders map[string]string
	}{
		{
			name:        "set new header",
			headers:     map[string]string{},
			key:         "Content-Type",
			value:       "application/json",
			wantHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:        "update existing header",
			headers:     map[string]string{"Content-Type": "text/plain"},
			key:         "Content-Type",
			value:       "application/json",
			wantHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:        "update existing case insensitive preserves original key",
			headers:     map[string]string{"Content-Type": "text/plain"},
			key:         "content-type",
			value:       "application/json",
			wantHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:        "add to existing headers",
			headers:     map[string]string{"Accept": "application/json"},
			key:         "Content-Type",
			value:       "application/json",
			wantHeaders: map[string]string{"Accept": "application/json", "Content-Type": "application/json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetHeader(tt.headers, tt.key, tt.value)
			for k, want := range tt.wantHeaders {
				if got, _ := GetHeader(tt.headers, k); got != want {
					t.Errorf("SetHeader() headers[%s] = %v, want %v", k, got, want)
				}
			}
			if len(tt.headers) != len(tt.wantHeaders) {
				t.Errorf("SetHeader() header count = %v, want %v", len(tt.headers), len(tt.wantHeaders))
			}
		})
	}
}

func TestDeleteHeader(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		key         string
		wantDeleted bool
		wantHeaders map[string]string
	}{
		{
			name:        "delete existing header",
			headers:     map[string]string{"Content-Type": "application/json", "Accept": "text/html"},
			key:         "Content-Type",
			wantDeleted: true,
			wantHeaders: map[string]string{"Accept": "text/html"},
		},
		{
			name:        "delete case insensitive",
			headers:     map[string]string{"Content-Type": "application/json"},
			key:         "content-type",
			wantDeleted: true,
			wantHeaders: map[string]string{},
		},
		{
			name:        "delete non-existing header",
			headers:     map[string]string{"Content-Type": "application/json"},
			key:         "Authorization",
			wantDeleted: false,
			wantHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:        "delete from empty headers",
			headers:     map[string]string{},
			key:         "Content-Type",
			wantDeleted: false,
			wantHeaders: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeleteHeader(tt.headers, tt.key)
			if got != tt.wantDeleted {
				t.Errorf("DeleteHeader() = %v, want %v", got, tt.wantDeleted)
			}
			if len(tt.headers) != len(tt.wantHeaders) {
				t.Errorf("DeleteHeader() header count = %v, want %v", len(tt.headers), len(tt.wantHeaders))
			}
			for k, want := range tt.wantHeaders {
				if v, _ := GetHeader(tt.headers, k); v != want {
					t.Errorf("DeleteHeader() remaining header %s = %v, want %v", k, v, want)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkGetHeader(b *testing.B) {
	headers := map[string]string{
		"Content-Type":    "application/json",
		"Authorization":   "Bearer token",
		"Accept":          "application/json",
		"X-Custom-Header": "value",
	}

	b.ResetTimer()
	for range b.N {
		GetHeader(headers, "x-custom-header")
	}
}

func BenchmarkSetHeader(b *testing.B) {
	b.ResetTimer()
	for range b.N {
		headers := make(map[string]string)
		SetHeader(headers, "Content-Type", "application/json")
	}
}

func BenchmarkDeleteHeader(b *testing.B) {
	b.ResetTimer()
	for range b.N {
		headers := map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token",
		}
		DeleteHeader(headers, "content-type")
	}
}
