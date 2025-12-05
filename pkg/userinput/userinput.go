// Package userinput handles detection and processing of user input variables
// in HTTP request URLs using the {{:paramName}} syntax.
package userinput

import (
	"net/url"
	"regexp"
	"strings"
)

// Pattern represents a detected user input pattern in a URL.
type Pattern struct {
	Name     string // Canonical parameter name (e.g., "id")
	Original string // Original matched pattern (e.g., "{{:id!secret}}")
	Position int    // Position in content
	IsSecret bool   // Whether the pattern uses the !secret suffix
}

// userInputRegex matches {{:paramName}} patterns with optional !secret suffix.
var userInputRegex = regexp.MustCompile(`\{\{:([a-zA-Z_][a-zA-Z0-9_]*)(!secret)?\}\}`)

// Detector detects {{:paramName}} patterns in URLs.
type Detector struct{}

// NewDetector creates a new user input pattern detector.
func NewDetector() *Detector {
	return &Detector{}
}

// Detect finds all {{:paramName}} patterns in content and returns first occurrences per name
// ordered by appearance. If any occurrence of a parameter is marked !secret, the result inherits it.
func (d *Detector) Detect(content string) []Pattern {
	all := d.FindAll(content)
	if len(all) == 0 {
		return nil
	}

	order := make([]string, 0, len(all))
	lookup := make(map[string]Pattern)

	for _, p := range all {
		if existing, ok := lookup[p.Name]; ok {
			if p.IsSecret && !existing.IsSecret {
				existing.IsSecret = true
				existing.Original = p.Original // Update Original to the secret variant
				lookup[p.Name] = existing
			}
			continue
		}
		lookup[p.Name] = p
		order = append(order, p.Name)
	}

	results := make([]Pattern, 0, len(order))
	for _, name := range order {
		results = append(results, lookup[name])
	}

	return results
}

// FindAll returns every {{:paramName}} match in the given content in scanning order (including duplicates).
func (d *Detector) FindAll(content string) []Pattern {
	matches := userInputRegex.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	patterns := make([]Pattern, 0, len(matches))

	for _, match := range matches {
		fullStart := match[0]
		fullEnd := match[1]
		nameStart := match[2]
		nameEnd := match[3]

		name := content[nameStart:nameEnd]
		original := content[fullStart:fullEnd]
		isSecret := match[4] >= 0 && match[5] >= 0
		canonicalName := name
		if isSecret {
			canonicalName = strings.TrimSuffix(name, "!secret")
		}

		if canonicalName == "" {
			continue
		}

		patterns = append(patterns, Pattern{
			Name:     canonicalName,
			Original: original,
			Position: fullStart,
			IsSecret: isSecret,
		})
	}

	return patterns
}

// HasPatterns checks if the content contains any user input patterns.
func (d *Detector) HasPatterns(content string) bool {
	return userInputRegex.MatchString(content)
}

// Replace substitutes {{:paramName}} patterns with provided values.
// Values are URL-encoded for safe inclusion in URLs and match both secret/non-secret variants.
func (d *Detector) Replace(content string, values map[string]string) string {
	return d.replaceInternal(content, values, true)
}

// ReplaceRaw substitutes {{:paramName}} patterns with provided values without URL encoding.
func (d *Detector) ReplaceRaw(content string, values map[string]string) string {
	return d.replaceInternal(content, values, false)
}

func (d *Detector) replaceInternal(content string, values map[string]string, encode bool) string {
	if len(values) == 0 {
		return content
	}

	replacer := userInputRegex.ReplaceAllStringFunc(content, func(match string) string {
		submatches := userInputRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		name := submatches[1]
		canonicalName := name
		if strings.HasSuffix(name, "!secret") {
			canonicalName = strings.TrimSuffix(name, "!secret")
		}

		value, ok := values[canonicalName]
		if !ok {
			return match
		}

		if encode {
			value = url.PathEscape(value)
		}
		return value
	})

	return replacer
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
