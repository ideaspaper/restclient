// Package userinput handles detection and processing of user input variables
// in HTTP request URLs using the {{:paramName}} syntax.
package userinput

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Pattern represents a detected user input pattern in a URL.
type Pattern struct {
	Name     string // Parameter name (e.g., "id")
	Original string // Original pattern (e.g., "{{:id}}")
	Position int    // Position in URL
}

// userInputRegex matches {{:paramName}} patterns where paramName is alphanumeric with underscores.
var userInputRegex = regexp.MustCompile(`\{\{:([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// Detector detects {{:paramName}} patterns in URLs.
type Detector struct{}

// NewDetector creates a new user input pattern detector.
func NewDetector() *Detector {
	return &Detector{}
}

// Detect finds all {{:paramName}} patterns in a URL.
// Returns patterns in order of appearance, with duplicates removed.
// If the same parameter name appears multiple times, only the first occurrence is returned.
func (d *Detector) Detect(url string) []Pattern {
	matches := userInputRegex.FindAllStringSubmatchIndex(url, -1)
	if len(matches) == 0 {
		return nil
	}

	// Track unique parameter names to avoid duplicates
	seen := make(map[string]bool)
	var patterns []Pattern

	for _, match := range matches {
		// match[0:1] = full match start/end
		// match[2:3] = capture group (param name) start/end
		fullStart := match[0]
		fullEnd := match[1]
		nameStart := match[2]
		nameEnd := match[3]

		name := url[nameStart:nameEnd]
		original := url[fullStart:fullEnd]

		// Skip if we've already seen this parameter name
		if seen[name] {
			continue
		}
		seen[name] = true

		patterns = append(patterns, Pattern{
			Name:     name,
			Original: original,
			Position: fullStart,
		})
	}

	// Sort by position to maintain order of appearance
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Position < patterns[j].Position
	})

	return patterns
}

// HasPatterns checks if the URL contains any user input patterns.
func (d *Detector) HasPatterns(url string) bool {
	return userInputRegex.MatchString(url)
}

// Replace substitutes {{:paramName}} patterns with provided values.
// Values are URL-encoded for safe inclusion in URLs.
// All occurrences of a parameter are replaced, even if it appears multiple times.
func (d *Detector) Replace(urlStr string, values map[string]string) string {
	result := urlStr

	for name, value := range values {
		pattern := "{{:" + name + "}}"
		// URL-encode the value for safe inclusion in URLs
		encodedValue := url.PathEscape(value)
		result = strings.ReplaceAll(result, pattern, encodedValue)
	}

	return result
}

// ReplaceRaw substitutes {{:paramName}} patterns with provided values without URL encoding.
// This is useful when the value is already encoded or when encoding is not desired.
func (d *Detector) ReplaceRaw(urlStr string, values map[string]string) string {
	result := urlStr

	for name, value := range values {
		pattern := "{{:" + name + "}}"
		result = strings.ReplaceAll(result, pattern, value)
	}

	return result
}

// GenerateKey creates a session storage key from a URL pattern.
// The key is created by normalizing the URL to include only the path and
// user input patterns, allowing different requests with the same pattern
// structure to share stored values.
func (d *Detector) GenerateKey(urlStr string) string {
	// Parse the URL to extract relevant parts
	parsed, err := url.Parse(urlStr)
	if err != nil {
		// If we can't parse, use the whole URL as the key
		return urlStr
	}

	// Build key from host + path + query with patterns
	var keyParts []string

	// Include host for uniqueness across different APIs
	if parsed.Host != "" {
		keyParts = append(keyParts, parsed.Host)
	}

	// Include path
	if parsed.Path != "" {
		keyParts = append(keyParts, parsed.Path)
	}

	// Include raw query to preserve user input patterns
	if parsed.RawQuery != "" {
		keyParts = append(keyParts, "?"+parsed.RawQuery)
	}

	return strings.Join(keyParts, "")
}

// ExtractPatternNames returns a list of unique parameter names from a URL.
func (d *Detector) ExtractPatternNames(url string) []string {
	patterns := d.Detect(url)
	names := make([]string, len(patterns))
	for i, p := range patterns {
		names[i] = p.Name
	}
	return names
}
