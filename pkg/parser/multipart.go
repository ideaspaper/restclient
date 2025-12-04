package parser

import (
	"regexp"
	"strings"

	"github.com/ideaspaper/restclient/pkg/models"
)

// parseMultipartParts parses multipart form data from the body
func (p *HttpRequestParser) parseMultipartParts(rawBody string, contentType string) []models.MultipartPart {
	var parts []models.MultipartPart

	// Extract boundary from Content-Type
	boundary := extractBoundary(contentType)
	if boundary == "" {
		return parts
	}

	// Split by boundary
	delimiter := "--" + boundary
	sections := strings.Split(rawBody, delimiter)

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" || section == "--" {
			continue
		}

		// Remove trailing -- for last part
		section = strings.TrimSuffix(section, "--")
		section = strings.TrimSpace(section)

		if section == "" {
			continue
		}

		// Parse part
		part := parseMultipartSection(section)
		if part.Name != "" {
			// Check if value is a file reference
			if after, found := strings.CutPrefix(strings.TrimSpace(part.Value), "< "); found {
				part.FilePath = strings.TrimSpace(after)
				part.IsFile = true
				part.Value = ""
			}
			parts = append(parts, part)
		}
	}

	return parts
}

// extractBoundary extracts the boundary from Content-Type header
func extractBoundary(contentType string) string {
	for _, part := range strings.Split(contentType, ";") {
		part = strings.TrimSpace(part)
		lowerPart := strings.ToLower(part)
		if after, found := strings.CutPrefix(lowerPart, "boundary="); found {
			// Use the original case for the boundary value
			boundary := part[len("boundary="):]
			boundary = strings.Trim(boundary, `"`)
			_ = after // boundary value extracted from original case
			return boundary
		}
	}
	return ""
}

// parseMultipartSection parses a single multipart section
func parseMultipartSection(section string) models.MultipartPart {
	var part models.MultipartPart

	// Split headers from body (separated by empty line)
	parts := strings.SplitN(section, "\r\n\r\n", 2)
	if len(parts) == 1 {
		parts = strings.SplitN(section, "\n\n", 2)
	}

	if len(parts) == 0 {
		return part
	}

	headerSection := parts[0]
	if len(parts) > 1 {
		part.Value = strings.TrimSpace(parts[1])
	}

	// Parse Content-Disposition
	dispositionRegex := regexp.MustCompile(`(?i)Content-Disposition:\s*form-data;\s*(.+)`)
	if matches := dispositionRegex.FindStringSubmatch(headerSection); matches != nil {
		params := matches[1]

		// Extract name
		nameRegex := regexp.MustCompile(`name="([^"]+)"`)
		if nameMatches := nameRegex.FindStringSubmatch(params); nameMatches != nil {
			part.Name = nameMatches[1]
		}

		// Extract filename
		filenameRegex := regexp.MustCompile(`filename="([^"]+)"`)
		if fnMatches := filenameRegex.FindStringSubmatch(params); fnMatches != nil {
			part.FileName = fnMatches[1]
			part.IsFile = true
		}
	}

	// Parse Content-Type
	ctRegex := regexp.MustCompile(`(?i)Content-Type:\s*(.+)`)
	if matches := ctRegex.FindStringSubmatch(headerSection); matches != nil {
		part.ContentType = strings.TrimSpace(matches[1])
	}

	return part
}

// isMultiPartFormData checks if content type is multipart form data
func isMultiPartFormData(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "multipart/form-data")
}
