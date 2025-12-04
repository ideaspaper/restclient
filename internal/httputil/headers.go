// Package httputil provides HTTP-related utility functions.
package httputil

import (
	"strings"
)

// GetHeader retrieves a header value case-insensitively from a map[string]string.
// Returns the value and true if found, or empty string and false if not found.
func GetHeader(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// GetHeaderFromSlice retrieves a header value case-insensitively from a map[string][]string.
// Returns the first value and true if found, or empty string and false if not found.
func GetHeaderFromSlice(headers map[string][]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0], true
		}
	}
	return "", false
}

// HasHeader checks if a header exists (case-insensitive) in a map[string]string.
func HasHeader(headers map[string]string, name string) bool {
	_, ok := GetHeader(headers, name)
	return ok
}

// HasHeaderFromSlice checks if a header exists (case-insensitive) in a map[string][]string.
func HasHeaderFromSlice(headers map[string][]string, name string) bool {
	_, ok := GetHeaderFromSlice(headers, name)
	return ok
}

// SetHeader sets a header value. If the header already exists (case-insensitive),
// it updates the existing key; otherwise, it adds a new entry.
func SetHeader(headers map[string]string, name, value string) {
	// First, check if the header already exists (case-insensitive)
	for k := range headers {
		if strings.EqualFold(k, name) {
			headers[k] = value
			return
		}
	}
	// If not found, add with the provided name
	headers[name] = value
}

// DeleteHeader removes a header (case-insensitive) from a map[string]string.
// Returns true if the header was found and deleted, false otherwise.
func DeleteHeader(headers map[string]string, name string) bool {
	for k := range headers {
		if strings.EqualFold(k, name) {
			delete(headers, k)
			return true
		}
	}
	return false
}
