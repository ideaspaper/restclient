package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
			"User-Agent": "restclient-cli",
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
		return nil, fmt.Errorf("no request line found")
	}

	// Set scripts in metadata
	if len(preScriptLines) > 0 {
		metadata.PreScript = strings.Join(preScriptLines, "\n")
	}
	if len(postScriptLines) > 0 {
		metadata.PostScript = strings.Join(postScriptLines, "\n")
	}

	// Parse request line
	method, url := parseRequestLine(strings.Join(requestLines, ""))

	// Parse headers
	headers := parseHeaders(headerLines, p.defaultHeaders, url)

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
			contentType, _ := getHeaderCaseInsensitive(headers, "Content-Type")
			if contentType == "" || strings.Contains(contentType, "application/json") {
				isGraphQL = true
			}
		}
	}

	// Parse body
	body, rawBody, err := p.parseBody(bodyLines, headers, isGraphQL)
	if err != nil {
		return nil, err
	}

	// Handle Host header for relative URLs
	if hostHeader, ok := getHeaderCaseInsensitive(headers, "Host"); ok && strings.HasPrefix(url, "/") {
		scheme := "http"
		if strings.Contains(hostHeader, ":443") || strings.Contains(hostHeader, ":8443") {
			scheme = "https"
		}
		url = fmt.Sprintf("%s://%s%s", scheme, hostHeader, url)
	}

	req := models.NewHttpRequest(method, url, headers, body, rawBody, metadata.Name)
	req.Metadata = metadata

	// Parse multipart parts if applicable
	contentType, _ := getHeaderCaseInsensitive(headers, "Content-Type")
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

// parseRequestLine parses the request line (method URL HTTP-version)
func parseRequestLine(line string) (method, url string) {
	line = strings.TrimSpace(line)

	// Match HTTP method
	methodRegex := regexp.MustCompile(`^(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT|TRACE|LOCK|UNLOCK|PROPFIND|PROPPATCH|COPY|MOVE|MKCOL|MKCALENDAR|ACL|SEARCH)\s+`)
	if matches := methodRegex.FindStringSubmatch(strings.ToUpper(line)); matches != nil {
		method = matches[1]
		line = line[len(matches[0]):]
	} else {
		method = "GET"
	}

	url = line

	// Remove HTTP version suffix
	versionRegex := regexp.MustCompile(`\s+HTTP/[\d.]+\s*$`)
	url = versionRegex.ReplaceAllString(url, "")
	url = strings.TrimSpace(url)

	return method, url
}

// parseHeaders parses header lines into a map
func parseHeaders(lines []string, defaultHeaders map[string]string, url string) map[string]string {
	headers := make(map[string]string)

	// Copy default headers (except Host for non-relative URLs)
	for k, v := range defaultHeaders {
		if strings.EqualFold(k, "host") && !strings.HasPrefix(url, "/") {
			continue
		}
		headers[k] = v
	}

	headerNames := make(map[string]string) // lowercase -> original case

	for _, line := range lines {
		colonIdx := strings.Index(line, ":")
		var name, value string
		if colonIdx == -1 {
			name = strings.TrimSpace(line)
			value = ""
		} else {
			name = strings.TrimSpace(line[:colonIdx])
			value = strings.TrimSpace(line[colonIdx+1:])
		}

		lowerName := strings.ToLower(name)
		if existingName, ok := headerNames[lowerName]; ok {
			// Combine values
			splitter := ","
			if lowerName == "cookie" {
				splitter = ";"
			}
			headers[existingName] = headers[existingName] + splitter + value
		} else {
			headerNames[lowerName] = name
			headers[name] = value
		}
	}

	return headers
}

// getHeaderCaseInsensitive gets a header value case-insensitively
func getHeaderCaseInsensitive(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// parseBody parses the request body
func (p *HttpRequestParser) parseBody(lines []string, headers map[string]string, isGraphQL bool) (io.Reader, string, error) {
	if len(lines) == 0 {
		return nil, "", nil
	}

	// Trim leading empty lines
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	lines = lines[start:]

	if len(lines) == 0 {
		return nil, "", nil
	}

	contentType, _ := getHeaderCaseInsensitive(headers, "Content-Type")

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
				// If file not found, use line as-is
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

	rawBody := strings.Join(bodyParts, lineEnding)

	// Handle form-urlencoded
	if isFormUrlEncoded(contentType) {
		// Remove newlines for form data, keep & as separator
		parts := strings.Split(rawBody, "\n")
		var formParts []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				// Remove leading &
				part = strings.TrimPrefix(part, "&")
				formParts = append(formParts, part)
			}
		}
		rawBody = strings.Join(formParts, "&")
	}

	// Handle GraphQL
	if isGraphQL {
		rawBody = createGraphQLBody(rawBody)
	}

	return strings.NewReader(rawBody), rawBody, nil
}

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
			if strings.HasPrefix(strings.TrimSpace(part.Value), "< ") {
				filePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part.Value), "< "))
				part.FilePath = filePath
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
	parts := strings.Split(contentType, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "boundary=") {
			boundary := strings.TrimPrefix(part, "boundary=")
			boundary = strings.TrimPrefix(boundary, "Boundary=")
			boundary = strings.TrimPrefix(boundary, "BOUNDARY=")
			boundary = strings.Trim(boundary, `"`)
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

// readFileContent reads content from a file
func (p *HttpRequestParser) readFileContent(filePath, encoding string) (string, error) {
	// Try absolute path first
	if filepath.IsAbs(filePath) {
		return readFile(filePath, encoding)
	}

	// Try relative to base directory
	if p.baseDir != "" {
		absPath := filepath.Join(p.baseDir, filePath)
		if content, err := readFile(absPath, encoding); err == nil {
			return content, nil
		}
	}

	// Try current working directory
	cwd, _ := os.Getwd()
	absPath := filepath.Join(cwd, filePath)
	return readFile(absPath, encoding)
}

// readFile reads a file and returns its content
func readFile(path, encoding string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// isFormUrlEncoded checks if content type is form-urlencoded
func isFormUrlEncoded(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded")
}

// isMultiPartFormData checks if content type is multipart form data
func isMultiPartFormData(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "multipart/form-data")
}

// createGraphQLBody wraps the body in GraphQL JSON format
func createGraphQLBody(body string) string {
	// Split into query and variables
	parts := strings.SplitN(body, "\n\n", 2)
	query := parts[0]
	variables := "{}"
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		variables = strings.TrimSpace(parts[1])
	}

	// Extract operation name from query, mutation, or subscription
	operationName := ""
	opRegex := regexp.MustCompile(`^\s*(?:query|mutation|subscription)\s+(\w+)`)
	if matches := opRegex.FindStringSubmatch(query); matches != nil {
		operationName = matches[1]
	}

	// Build JSON payload
	query = strings.ReplaceAll(query, "\\", "\\\\")
	query = strings.ReplaceAll(query, "\"", "\\\"")
	query = strings.ReplaceAll(query, "\n", "\\n")
	query = strings.ReplaceAll(query, "\r", "\\r")
	query = strings.ReplaceAll(query, "\t", "\\t")

	result := fmt.Sprintf(`{"query":"%s"`, query)
	if operationName != "" {
		result += fmt.Sprintf(`,"operationName":"%s"`, operationName)
	}
	result += fmt.Sprintf(`,"variables":%s}`, variables)

	return result
}

// ParseFile parses an HTTP request file
func ParseFile(filePath string, defaultHeaders map[string]string) ([]*models.HttpRequest, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	baseDir := filepath.Dir(filePath)
	parser := NewHttpRequestParser(string(content), defaultHeaders, baseDir)
	return parser.ParseAll()
}

// ParseFileWithWarnings parses an HTTP request file and returns warnings
func ParseFileWithWarnings(filePath string, defaultHeaders map[string]string) (*ParseResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
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
		return nil, fmt.Errorf("request index %d out of range (0-%d)", index, len(requests)-1)
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

	return nil, fmt.Errorf("request with name '%s' not found", name)
}
