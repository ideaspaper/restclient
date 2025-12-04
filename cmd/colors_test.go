package cmd

import (
	"testing"

	"github.com/fatih/color"
)

func TestGetMethodColor(t *testing.T) {
	// Disable color for testing to ensure consistent output
	color.NoColor = true
	defer func() { color.NoColor = false }()

	tests := []struct {
		method string
	}{
		{"GET"},
		{"POST"},
		{"PUT"},
		{"DELETE"},
		{"PATCH"},
		{"HEAD"},    // default case
		{"OPTIONS"}, // default case
		{"UNKNOWN"}, // default case
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			c := getMethodColor(tt.method)
			if c == nil {
				t.Errorf("getMethodColor(%q) returned nil", tt.method)
			}
			// Verify the color can format a string without error
			result := c.Sprint(tt.method)
			if result != tt.method {
				// With NoColor=true, output should be the plain method
				t.Errorf("getMethodColor(%q).Sprint() = %q, want %q", tt.method, result, tt.method)
			}
		})
	}
}

func TestPrintListIndex(t *testing.T) {
	// Test with colors disabled
	v.Set("showColors", false)

	tests := []struct {
		index int
		want  string
	}{
		{1, "[1]"},
		{10, "[10]"},
		{0, "[0]"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := printListIndex(tt.index)
			if got != tt.want {
				t.Errorf("printListIndex(%d) = %q, want %q", tt.index, got, tt.want)
			}
		})
	}
}

func TestPrintMethod(t *testing.T) {
	// Test with colors disabled
	v.Set("showColors", false)

	tests := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			got := printMethod(method)
			if got != method {
				t.Errorf("printMethod(%q) = %q, want %q", method, got, method)
			}
		})
	}
}

func TestPrintDimText(t *testing.T) {
	// Test with colors disabled
	v.Set("showColors", false)

	text := "some dim text"
	got := printDimText(text)
	if got != text {
		t.Errorf("printDimText(%q) = %q, want %q", text, got, text)
	}
}

func TestFormatKey(t *testing.T) {
	// Test with colors disabled
	v.Set("showColors", false)

	key := "Content-Type"
	got := formatKey(key)
	if got != key {
		t.Errorf("formatKey(%q) = %q, want %q", key, got, key)
	}
}

func TestPrintMarker(t *testing.T) {
	// Test with colors disabled
	v.Set("showColors", false)

	tests := []struct {
		marked bool
		want   string
	}{
		{true, "* "},
		{false, "  "},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := printMarker(tt.marked)
			if got != tt.want {
				t.Errorf("printMarker(%v) = %q, want %q", tt.marked, got, tt.want)
			}
		})
	}
}

func TestUseColors(t *testing.T) {
	// Test that useColors respects the showColors config
	v.Set("showColors", true)
	if !useColors() {
		t.Error("useColors() = false, want true when showColors is true")
	}

	v.Set("showColors", false)
	if useColors() {
		t.Error("useColors() = true, want false when showColors is false")
	}
}

func TestPrintTestPassAndFail(t *testing.T) {
	// These functions print to stdout, so we just verify they don't panic
	// Test with colors disabled
	v.Set("showColors", false)

	// Should not panic
	printTestPass("test name")
	printTestFail("test name", "error message")

	// Test with colors enabled
	v.Set("showColors", true)
	printTestPass("test name with colors")
	printTestFail("test name with colors", "error message with colors")
}
