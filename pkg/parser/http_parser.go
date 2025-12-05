// Package parser provides functionality for parsing .http and .rest files
// into structured HTTP request models with support for variables, multipart
// forms, and embedded scripts.
package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ideaspaper/restclient/internal/constants"
	"github.com/ideaspaper/restclient/internal/httputil"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/models"
)

// ParseState represents the current state of parsing
type ParseState int

const (
	ParseStateURL ParseState = iota
	ParseStateHeader
	ParseStateBody
	ParseStatePostScript
)

// ParseWarning represents a warning generated during parsing
type ParseWarning struct {
	BlockIndex int    // Index of the block (0-based)
	Line       int    // Line number within the block (0-based)
	Message    string // Warning message
}

// ParseResult contains the parsed requests and any warnings
type ParseResult struct {
	Requests []*models.HttpRequest
	Warnings []ParseWarning
}

// HttpRequestParser parses HTTP request files (.http, .rest)
type HttpRequestParser struct {
	content        string
	defaultHeaders map[string]string
	baseDir        string
	warnings       []ParseWarning
}

// NewHttpRequestParser creates a new parser
func NewHttpRequestParser(content string, defaultHeaders map[string]string, baseDir string) *HttpRequestParser {
	if defaultHeaders == nil {
		defaultHeaders = map[string]string{
			constants.HeaderUserAgent: constants.DefaultUserAgent,
		}
	}
	return &HttpRequestParser{
		content:        content,
		defaultHeaders: defaultHeaders,
		baseDir:        baseDir,
		warnings:       []ParseWarning{},
	}
}

// addWarning adds a warning to the parser's warning list
func (p *HttpRequestParser) addWarning(blockIndex, line int, message string) {
	p.warnings = append(p.warnings, ParseWarning{
		BlockIndex: blockIndex,
		Line:       line,
		Message:    message,
	})
}

// RequestDelimiter is used to separate multiple requests in one file
const RequestDelimiter = "###"

// ParseAll parses all requests from the content (backward compatible - ignores warnings)
func (p *HttpRequestParser) ParseAll() ([]*models.HttpRequest, error) {
	result := p.ParseAllWithWarnings()
	return result.Requests, nil
}

// DuplicateName holds information about a request with a duplicate name
type DuplicateName struct {
	Name   string
	Method string
	URL    string
	Index  int // 0-based index of the request
}

// ParseAllWithWarnings parses all requests and returns warnings for invalid blocks
func (p *HttpRequestParser) ParseAllWithWarnings() *ParseResult {
	blocks := splitRequestBlocks(p.content)
	var requests []*models.HttpRequest
	p.warnings = []ParseWarning{} // Reset warnings

	// Track names to detect duplicates
	nameToRequests := make(map[string][]DuplicateName)

	for i, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		req, err := p.ParseRequest(block)
		if err != nil {
			// Collect warning instead of silently skipping
			p.addWarning(i, 0, fmt.Sprintf("skipped invalid request block: %v", err))
			continue
		}

		// Track request name for duplicate detection
		reqName := req.Metadata.Name
		if reqName == "" {
			reqName = req.Name
		}
		if reqName != "" {
			nameToRequests[reqName] = append(nameToRequests[reqName], DuplicateName{
				Name:   reqName,
				Method: req.Method,
				URL:    req.URL,
				Index:  len(requests),
			})
		}

		requests = append(requests, req)
	}

	// Add warnings for duplicate names
	for name, dupes := range nameToRequests {
		if len(dupes) > 1 {
			var details []string
			for _, d := range dupes {
				details = append(details, fmt.Sprintf("request %d: %s %s", d.Index+1, d.Method, d.URL))
			}
			p.addWarning(dupes[0].Index, 0, fmt.Sprintf(
				"duplicate @name '%s' found in %d requests (%s). First match will be used when selecting by name",
				name, len(dupes), strings.Join(details, "; ")))
		}
	}

	return &ParseResult{
		Requests: requests,
		Warnings: p.warnings,
	}
}

// FindDuplicateNames returns a map of duplicate names to their occurrences
func FindDuplicateNames(requests []*models.HttpRequest) map[string][]DuplicateName {
	nameToRequests := make(map[string][]DuplicateName)

	for i, req := range requests {
		reqName := req.Metadata.Name
		if reqName == "" {
			reqName = req.Name
		}
		if reqName != "" {
			nameToRequests[reqName] = append(nameToRequests[reqName], DuplicateName{
				Name:   reqName,
				Method: req.Method,
				URL:    req.URL,
				Index:  i,
			})
		}
	}

	// Filter to only include duplicates
	duplicates := make(map[string][]DuplicateName)
	for name, dupes := range nameToRequests {
		if len(dupes) > 1 {
			duplicates[name] = dupes
		}
	}

	return duplicates
}

// splitRequestBlocks splits content by ### delimiter
func splitRequestBlocks(content string) []string {
	// Split by ### delimiter (3 or more # characters, optionally followed by text)
	re := regexp.MustCompile(`(?m)^#{3,}.*$`)
	parts := re.Split(content, -1)
	return parts
}

// ParseRequest parses a single HTTP request from text
func (p *HttpRequestParser) ParseRequest(rawText string) (*models.HttpRequest, error) {
	lines := strings.Split(rawText, "\n")

	var requestLines []string
	var headerLines []string
	var bodyLines []string
	var preScriptLines []string
	var postScriptLines []string
	var metadata models.RequestMetadata

	state := ParseStateURL
	foundRequestLine := false
	inPreScript := false
	inPostScript := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Check for pre-request script file reference: < ./script.js (not < {% which is inline)
		if !foundRequestLine && strings.HasPrefix(trimmedLine, "<") && !strings.HasPrefix(trimmedLine, "< {%") {
			scriptPath := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "<"))
			// Check if it looks like a script file path (ends with .js)
			if strings.HasSuffix(scriptPath, ".js") {
				content, err := p.readFileContent(scriptPath, "")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to read pre-request script file '%s': %v\n", scriptPath, err)
				} else {
					preScriptLines = append(preScriptLines, content)
				}
				continue
			}
		}

		// Check for pre-request script start: < {%
		if !foundRequestLine && strings.HasPrefix(trimmedLine, "< {%") {
			inPreScript = true
			// Check if script content is on the same line
			rest := strings.TrimPrefix(trimmedLine, "< {%")
			if strings.Contains(rest, "%}") {
				// Single line script
				scriptContent := strings.TrimSuffix(rest, "%}")
				preScriptLines = append(preScriptLines, strings.TrimSpace(scriptContent))
				inPreScript = false
			}
			continue
		}

		// Check for pre-script end
		if inPreScript {
			if strings.Contains(trimmedLine, "%}") {
				// End of pre-script
				beforeEnd := strings.Split(trimmedLine, "%}")[0]
				if strings.TrimSpace(beforeEnd) != "" {
					preScriptLines = append(preScriptLines, beforeEnd)
				}
				inPreScript = false
			} else {
				preScriptLines = append(preScriptLines, line)
			}
			continue
		}

		// Check for post-response script file reference: > ./script.js (not > {% which is inline)
		if foundRequestLine && strings.HasPrefix(trimmedLine, ">") && !strings.HasPrefix(trimmedLine, "> {%") {
			scriptPath := strings.TrimSpace(strings.TrimPrefix(trimmedLine, ">"))
			// Check if it looks like a script file path (ends with .js)
			if strings.HasSuffix(scriptPath, ".js") {
				content, err := p.readFileContent(scriptPath, "")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to read post-response script file '%s': %v\n", scriptPath, err)
				} else {
					postScriptLines = append(postScriptLines, content)
				}
				state = ParseStatePostScript
				continue
			}
		}

		// Check for post-response script start: > {%
		if foundRequestLine && strings.HasPrefix(trimmedLine, "> {%") {
			inPostScript = true
			state = ParseStatePostScript
			// Check if script content is on the same line
			rest := strings.TrimPrefix(trimmedLine, "> {%")
			if strings.Contains(rest, "%}") {
				// Single line script
				scriptContent := strings.TrimSuffix(rest, "%}")
				postScriptLines = append(postScriptLines, strings.TrimSpace(scriptContent))
				inPostScript = false
			}
			continue
		}

		// Check for post-script end
		if inPostScript {
			if strings.Contains(trimmedLine, "%}") {
				// End of post-script
				beforeEnd := strings.Split(trimmedLine, "%}")[0]
				if strings.TrimSpace(beforeEnd) != "" {
					postScriptLines = append(postScriptLines, beforeEnd)
				}
				inPostScript = false
			} else {
				postScriptLines = append(postScriptLines, line)
			}
			continue
		}

		// Skip if we're in post-script state but not inside a script block
		if state == ParseStatePostScript && !inPostScript {
			continue
		}

		// Skip empty lines at the beginning
		if !foundRequestLine && trimmedLine == "" {
			continue
		}

		// Check for metadata comments (# @name, // @name, etc.)
		if meta, ok := parseMetadata(trimmedLine); ok {
			applyMetadata(&metadata, meta)
			continue
		}

		// Skip regular comments at the beginning
		if !foundRequestLine && isComment(trimmedLine) {
			continue
		}

		// Skip file variable declarations (@varName = value)
		if !foundRequestLine && isFileVariable(trimmedLine) {
			continue
		}

		nextLine := ""
		if i+1 < len(lines) {
			nextLine = strings.TrimSpace(lines[i+1])
		}

		switch state {
		case ParseStateURL:
			if !foundRequestLine && trimmedLine != "" && !isComment(trimmedLine) {
				requestLines = append(requestLines, trimmedLine)
				foundRequestLine = true

				// Check if next line is a query string continuation
				if isQueryStringContinuation(nextLine) {
					continue
				}

				// Check if there's more content
				if nextLine == "" {
					// Empty line means body follows (skip the blank line)
					i++
					state = ParseStateBody
				} else if !isComment(nextLine) {
					state = ParseStateHeader
				}
			} else if isQueryStringContinuation(trimmedLine) {
				// Query string continuation
				requestLines = append(requestLines, trimmedLine)
			}

		case ParseStateHeader:
			if trimmedLine == "" {
				state = ParseStateBody
			} else if !isComment(trimmedLine) {
				headerLines = append(headerLines, trimmedLine)
			}

		case ParseStateBody:
			// Check if this line starts a post-script
			if strings.HasPrefix(trimmedLine, "> {%") {
				inPostScript = true
				state = ParseStatePostScript
				rest := strings.TrimPrefix(trimmedLine, "> {%")
				if strings.Contains(rest, "%}") {
					scriptContent := strings.TrimSuffix(rest, "%}")
					postScriptLines = append(postScriptLines, strings.TrimSpace(scriptContent))
					inPostScript = false
				}
				continue
			}
			bodyLines = append(bodyLines, line) // Preserve original line with whitespace
		}
	}

	if !foundRequestLine {
		return nil, errors.NewParseError("", 0, "no request line found")
	}

	// Set scripts in metadata
	if len(preScriptLines) > 0 {
		metadata.PreScript = strings.Join(preScriptLines, "\n")
	}
	if len(postScriptLines) > 0 {
		metadata.PostScript = strings.Join(postScriptLines, "\n")
	}

	// Parse request line
	reqLineResult := parseRequestLine(strings.Join(requestLines, ""))
	method := reqLineResult.Method
	url := reqLineResult.URL

	// Collect request line warnings (will be added to parser warnings later if needed)
	var requestWarnings []string
	requestWarnings = append(requestWarnings, reqLineResult.Warnings...)

	// Parse headers
	headersResult := parseHeadersWithWarnings(headerLines, p.defaultHeaders, url)
	headers := headersResult.Headers
	requestWarnings = append(requestWarnings, headersResult.Warnings...)

	// Check for GraphQL request
	isGraphQL := false
	for k, v := range headers {
		if strings.EqualFold(k, "X-Request-Type") && strings.EqualFold(v, "GraphQL") {
			isGraphQL = true
			delete(headers, k)
			break
		}
	}

	// Auto-detect GraphQL by URL path or content-type
	if !isGraphQL {
		if strings.HasSuffix(url, "/graphql") || strings.Contains(url, "/graphql?") {
			contentType, _ := httputil.GetHeader(headers, constants.HeaderContentType)
			if contentType == "" || strings.Contains(contentType, constants.MIMEApplicationJSON) {
				isGraphQL = true
			}
		}
	}

	// Parse body
	bodyResult := p.parseBodyWithWarnings(bodyLines, headers, isGraphQL)
	body := bodyResult.Body
	rawBody := bodyResult.RawBody
	requestWarnings = append(requestWarnings, bodyResult.Warnings...)

	// Handle Host header for relative URLs
	if hostHeader, ok := httputil.GetHeader(headers, constants.HeaderHost); ok && strings.HasPrefix(url, "/") {
		scheme := "http"
		if strings.Contains(hostHeader, ":443") || strings.Contains(hostHeader, ":8443") {
			scheme = "https"
		}
		url = fmt.Sprintf("%s://%s%s", scheme, hostHeader, url)
	}

	req := models.NewHttpRequest(method, url, headers, body, rawBody, metadata.Name)
	req.Metadata = metadata
	req.Warnings = requestWarnings

	// Parse multipart parts if applicable
	contentType, _ := httputil.GetHeader(headers, constants.HeaderContentType)
	if isMultiPartFormData(contentType) {
		req.MultipartParts = p.parseMultipartParts(rawBody, contentType)
	}

	return req, nil
}

// parseMetadata checks if a line contains metadata and extracts it
func parseMetadata(line string) (map[string]string, bool) {
	// Match: # @key value or // @key value
	re := regexp.MustCompile(`^\s*(?:#|\/{2})\s*@([\w-]+)(?:\s+(.*?))?\s*$`)
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return nil, false
	}

	key := strings.ToLower(matches[1])
	value := ""
	if len(matches) > 2 {
		value = strings.TrimSpace(matches[2])
	}

	return map[string]string{key: value}, true
}

// applyMetadata applies parsed metadata to RequestMetadata
func applyMetadata(metadata *models.RequestMetadata, meta map[string]string) {
	for k, v := range meta {
		switch k {
		case "name":
			metadata.Name = v
		case "note":
			// Concatenate multiple notes with newlines
			if metadata.Note != "" {
				metadata.Note = metadata.Note + "\n" + v
			} else {
				metadata.Note = v
			}
		case "no-redirect":
			metadata.NoRedirect = true
		case "no-cookie-jar":
			metadata.NoCookieJar = true
		case "prompt":
			parts := strings.SplitN(v, " ", 2)
			pv := models.PromptVariable{Name: parts[0]}
			if len(parts) > 1 {
				pv.Description = parts[1]
			}
			// Check if it's a password field
			lowerName := strings.ToLower(pv.Name)
			if lowerName == "password" || lowerName == "passwd" || lowerName == "pass" {
				pv.IsPassword = true
			}
			metadata.Prompts = append(metadata.Prompts, pv)
		}
	}
}

// isComment checks if a line is a comment
func isComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")
}

// isFileVariable checks if a line is a file variable declaration (@varName = value)
func isFileVariable(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "@") {
		return false
	}
	// Check for @varName = value pattern
	return strings.Contains(trimmed, "=")
}

// isQueryStringContinuation checks if line is a query string continuation
func isQueryStringContinuation(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "?") || strings.HasPrefix(trimmed, "&")
}

// validHTTPMethods contains all valid HTTP methods
var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true,
	"LOCK": true, "UNLOCK": true, "PROPFIND": true, "PROPPATCH": true,
	"COPY": true, "MOVE": true, "MKCOL": true, "MKCALENDAR": true,
	"ACL": true, "SEARCH": true,
}

// parseRequestLineResult contains the parsed request line and any warnings
type parseRequestLineResult struct {
	Method   string
	URL      string
	Warnings []string
}

// parseRequestLine parses the request line (method URL HTTP-version)
func parseRequestLine(line string) parseRequestLineResult {
	result := parseRequestLineResult{}
	line = strings.TrimSpace(line)

	// Match HTTP method
	methodRegex := regexp.MustCompile(`^(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT|TRACE|LOCK|UNLOCK|PROPFIND|PROPPATCH|COPY|MOVE|MKCOL|MKCALENDAR|ACL|SEARCH)\s+`)
	if matches := methodRegex.FindStringSubmatch(strings.ToUpper(line)); matches != nil {
		result.Method = matches[1]
		line = line[len(matches[0]):]
	} else {
		// Check if line starts with something that looks like a method (word followed by space)
		wordRegex := regexp.MustCompile(`^([A-Za-z]+)\s+`)
		if wordMatches := wordRegex.FindStringSubmatch(line); wordMatches != nil {
			possibleMethod := strings.ToUpper(wordMatches[1])
			if !validHTTPMethods[possibleMethod] {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"unknown HTTP method '%s', defaulting to GET. Valid methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS",
					wordMatches[1]))
			}
		}
		result.Method = "GET"
	}

	result.URL = line

	// Remove HTTP version suffix
	versionRegex := regexp.MustCompile(`\s+HTTP/[\d.]+\s*$`)
	result.URL = versionRegex.ReplaceAllString(result.URL, "")
	result.URL = strings.TrimSpace(result.URL)

	return result
}

// parseHeaders parses header lines into a map (backward compatible, no warnings)
func parseHeaders(lines []string, defaultHeaders map[string]string, url string) map[string]string {
	result := parseHeadersWithWarnings(lines, defaultHeaders, url)
	return result.Headers
}

// parseHeadersResult contains the parsed headers and any warnings
type parseHeadersResult struct {
	Headers  map[string]string
	Warnings []string
}

// parseHeadersWithWarnings parses header lines into a map and returns warnings
func parseHeadersWithWarnings(lines []string, defaultHeaders map[string]string, url string) parseHeadersResult {
	result := parseHeadersResult{
		Headers: make(map[string]string),
	}

	// Copy default headers (except Host for non-relative URLs)
	for k, v := range defaultHeaders {
		if strings.EqualFold(k, constants.HeaderHost) && !strings.HasPrefix(url, "/") {
			continue
		}
		result.Headers[k] = v
	}

	headerNames := make(map[string]string) // lowercase -> original case

	for _, line := range lines {
		colonIdx := strings.Index(line, ":")
		var name, value string
		if colonIdx == -1 {
			name = strings.TrimSpace(line)
			value = ""
			// Warn about malformed header
			if name != "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"malformed header '%s': missing colon separator. Expected format: 'Header-Name: value'",
					name))
			}
		} else {
			name = strings.TrimSpace(line[:colonIdx])
			value = strings.TrimSpace(line[colonIdx+1:])
		}

		lowerName := strings.ToLower(name)
		if existingName, ok := headerNames[lowerName]; ok {
			// Combine values
			splitter := ","
			if strings.EqualFold(lowerName, constants.HeaderCookie) {
				splitter = ";"
			}
			result.Headers[existingName] = result.Headers[existingName] + splitter + value
		} else {
			headerNames[lowerName] = name
			result.Headers[name] = value
		}
	}

	return result
}

// parseBodyResult contains the parsed body and any warnings
type parseBodyResult struct {
	Body     io.Reader
	RawBody  string
	Warnings []string
}

// parseBody parses the request body (backward compatible, no warnings)
func (p *HttpRequestParser) parseBody(lines []string, headers map[string]string, isGraphQL bool) (io.Reader, string, error) {
	result := p.parseBodyWithWarnings(lines, headers, isGraphQL)
	return result.Body, result.RawBody, nil
}

// parseBodyWithWarnings parses the request body and returns warnings
func (p *HttpRequestParser) parseBodyWithWarnings(lines []string, headers map[string]string, isGraphQL bool) parseBodyResult {
	result := parseBodyResult{}

	if len(lines) == 0 {
		return result
	}

	// Trim leading empty lines
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	lines = lines[start:]

	if len(lines) == 0 {
		return result
	}

	contentType, _ := httputil.GetHeader(headers, constants.HeaderContentType)

	// Check for file reference
	inputFileRegex := regexp.MustCompile(`^<(?:@(?:(\w+))?)?[ \t]+(.+?)\s*$`)

	var bodyParts []string
	for _, line := range lines {
		if matches := inputFileRegex.FindStringSubmatch(line); matches != nil {
			// File reference: < filepath or <@ filepath or <@encoding filepath
			encoding := matches[1]
			filePath := strings.TrimSpace(matches[2])

			content, err := p.readFileContent(filePath, encoding)
			if err != nil {
				// Warn about missing file reference
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"file reference '%s' could not be read: %v. Using literal content instead",
					filePath, err))
				bodyParts = append(bodyParts, line)
			} else {
				bodyParts = append(bodyParts, content)
			}
		} else {
			bodyParts = append(bodyParts, line)
		}
	}

	// Join body lines
	lineEnding := "\n"
	if isMultiPartFormData(contentType) {
		lineEnding = "\r\n"
	}

	result.RawBody = strings.Join(bodyParts, lineEnding)

	// Handle form-urlencoded
	if isFormUrlEncoded(contentType) {
		// Remove newlines for form data, keep & as separator
		parts := strings.Split(result.RawBody, "\n")
		var formParts []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				// Remove leading &
				part = strings.TrimPrefix(part, "&")
				formParts = append(formParts, part)
			}
		}
		result.RawBody = strings.Join(formParts, "&")
	}

	// Handle GraphQL
	if isGraphQL {
		result.RawBody = createGraphQLBody(result.RawBody)
	}

	result.Body = strings.NewReader(result.RawBody)
	return result
}

// ParseFile parses an HTTP request file
func ParseFile(filePath string, defaultHeaders map[string]string) ([]*models.HttpRequest, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.NewParseErrorWithCause(filePath, 0, "failed to read file", err)
	}

	baseDir := filepath.Dir(filePath)
	parser := NewHttpRequestParser(string(content), defaultHeaders, baseDir)
	return parser.ParseAll()
}

// ParseFileWithWarnings parses an HTTP request file and returns warnings
func ParseFileWithWarnings(filePath string, defaultHeaders map[string]string) (*ParseResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.NewParseErrorWithCause(filePath, 0, "failed to read file", err)
	}

	baseDir := filepath.Dir(filePath)
	parser := NewHttpRequestParser(string(content), defaultHeaders, baseDir)
	return parser.ParseAllWithWarnings(), nil
}

// ParseFileAt parses a specific request from a file (by index)
func ParseFileAt(filePath string, index int, defaultHeaders map[string]string) (*models.HttpRequest, error) {
	requests, err := ParseFile(filePath, defaultHeaders)
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(requests) {
		return nil, errors.NewValidationErrorWithValue("request index", fmt.Sprintf("%d", index),
			fmt.Sprintf("out of range (0-%d)", len(requests)-1))
	}

	return requests[index], nil
}

// ParseFileByName parses a named request from a file
func ParseFileByName(filePath string, name string, defaultHeaders map[string]string) (*models.HttpRequest, error) {
	requests, err := ParseFile(filePath, defaultHeaders)
	if err != nil {
		return nil, err
	}

	for _, req := range requests {
		if req.Name == name || req.Metadata.Name == name {
			return req, nil
		}
	}

	return nil, errors.NewValidationErrorWithValue("request name", name, "not found")
}
