package postman

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/ideaspaper/restclient/internal/constants"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/parser"
	"github.com/ideaspaper/restclient/pkg/variables"
)

// secretUserInputRegex matches {{:varName!secret}} patterns in variable values
var secretUserInputRegex = regexp.MustCompile(`\{\{:([a-zA-Z_][a-zA-Z0-9_]*)!secret\}\}`)

// ExportOptions configures how .http files are exported
type ExportOptions struct {
	// CollectionName is the name for the Postman collection
	CollectionName string
	// CollectionDescription is the description for the collection
	CollectionDescription string
	// IncludeVariables includes file variables as collection variables
	IncludeVariables bool
	// IncludeScripts includes pre-request and test scripts
	IncludeScripts bool
	// PrettyPrint outputs formatted JSON
	PrettyPrint bool
}

// DefaultExportOptions returns sensible defaults
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		IncludeVariables: true,
		IncludeScripts:   true,
		PrettyPrint:      true,
	}
}

// ExportResult contains information about the export
type ExportResult struct {
	RequestsCount     int
	VariablesCount    int
	CollectionPath    string
	VariableConflicts []VariableConflict
}

// VariableConflict represents a variable that exists in multiple files with different values
type VariableConflict struct {
	Name      string
	Files     []string
	Values    []string
	UsedValue string
	UsedFile  string
}

// Export converts one or more .http files to a Postman collection
func Export(httpFiles []string, outputPath string, opts ExportOptions) (*ExportResult, error) {
	result := &ExportResult{}

	// Determine collection name
	collectionName := opts.CollectionName
	if collectionName == "" {
		if len(httpFiles) == 1 {
			base := filepath.Base(httpFiles[0])
			collectionName = strings.TrimSuffix(base, filepath.Ext(base))
		} else {
			collectionName = "Exported Collection"
		}
	}

	collection := NewCollection(collectionName)

	if opts.CollectionDescription != "" {
		collection.Info.Description = &Description{
			Content: opts.CollectionDescription,
			Type:    "text/plain",
		}
	}

	collection.Info.PostmanID = uuid.New().String()

	// Collect all variables from all files, tracking sources for conflict detection
	allVariables := make(map[string]string)
	variableSources := make(map[string][]struct {
		file  string
		value string
	})

	for _, httpFile := range httpFiles {
		content, err := os.ReadFile(httpFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %s", httpFile)
		}

		// Parse file variables
		if opts.IncludeVariables {
			fileVars := variables.ParseFileVariables(string(content))
			for k, v := range fileVars {
				// Track source for conflict detection
				variableSources[k] = append(variableSources[k], struct {
					file  string
					value string
				}{file: httpFile, value: v})

				// First file wins
				if _, exists := allVariables[k]; !exists {
					allVariables[k] = v
				}
			}
		}

		// Parse requests
		baseDir := filepath.Dir(httpFile)
		p := parser.NewHttpRequestParser(string(content), nil, baseDir)
		requests, err := p.ParseAll()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse file %s", httpFile)
		}

		// Convert to Postman items
		for _, req := range requests {
			item := convertRequestToItem(req, opts)
			collection.Item = append(collection.Item, item)
			result.RequestsCount++
		}
	}

	// Detect variable conflicts (same name, different values across files)
	if opts.IncludeVariables {
		for varName, sources := range variableSources {
			if len(sources) > 1 {
				// Check if values differ
				firstValue := sources[0].value
				hasConflict := false
				for _, src := range sources[1:] {
					if src.value != firstValue {
						hasConflict = true
						break
					}
				}

				if hasConflict {
					conflict := VariableConflict{
						Name:      varName,
						UsedValue: allVariables[varName],
						UsedFile:  sources[0].file,
					}
					for _, src := range sources {
						conflict.Files = append(conflict.Files, src.file)
						conflict.Values = append(conflict.Values, src.value)
					}
					result.VariableConflicts = append(result.VariableConflicts, conflict)
				}
			}
		}
	}

	// Add collection variables
	if opts.IncludeVariables {
		for k, v := range allVariables {
			variable := Variable{
				Key:   k,
				Value: v,
				Type:  "string",
			}

			// Check if this variable value contains a secret user input pattern
			// e.g., @apiKey = {{:apiKey!secret}}
			if secretUserInputRegex.MatchString(v) {
				variable.Type = "secret"
				// Also add [secret] marker in description for round-trip compatibility
				variable.Description = &Description{
					Content: "[secret]",
					Type:    "text/plain",
				}
			}

			collection.Variable = append(collection.Variable, variable)
			result.VariablesCount++
		}
	}

	// Serialize to JSON
	var jsonData []byte
	var err error
	if opts.PrettyPrint {
		jsonData, err = json.MarshalIndent(collection, "", "  ")
	} else {
		jsonData, err = json.Marshal(collection)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize collection")
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return nil, errors.Wrap(err, "failed to write collection file")
	}

	result.CollectionPath = outputPath
	return result, nil
}

// ExportToCollection converts .http files to a Postman Collection struct
func ExportToCollection(httpFiles []string, opts ExportOptions) (*Collection, error) {
	collectionName := opts.CollectionName
	if collectionName == "" {
		if len(httpFiles) == 1 {
			base := filepath.Base(httpFiles[0])
			collectionName = strings.TrimSuffix(base, filepath.Ext(base))
		} else {
			collectionName = "Exported Collection"
		}
	}

	collection := NewCollection(collectionName)

	if opts.CollectionDescription != "" {
		collection.Info.Description = &Description{
			Content: opts.CollectionDescription,
			Type:    "text/plain",
		}
	}

	collection.Info.PostmanID = uuid.New().String()

	allVariables := make(map[string]string)

	for _, httpFile := range httpFiles {
		content, err := os.ReadFile(httpFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %s", httpFile)
		}

		if opts.IncludeVariables {
			fileVars := variables.ParseFileVariables(string(content))
			for k, v := range fileVars {
				allVariables[k] = v
			}
		}

		baseDir := filepath.Dir(httpFile)
		p := parser.NewHttpRequestParser(string(content), nil, baseDir)
		requests, err := p.ParseAll()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse file %s", httpFile)
		}

		for _, req := range requests {
			item := convertRequestToItem(req, opts)
			collection.Item = append(collection.Item, item)
		}
	}

	if opts.IncludeVariables {
		for k, v := range allVariables {
			variable := Variable{
				Key:   k,
				Value: v,
				Type:  "string",
			}

			// Check if this variable value contains a secret user input pattern
			if secretUserInputRegex.MatchString(v) {
				variable.Type = "secret"
				variable.Description = &Description{
					Content: "[secret]",
					Type:    "text/plain",
				}
			}

			collection.Variable = append(collection.Variable, variable)
		}
	}

	return collection, nil
}

// convertRequestToItem converts an HttpRequest to a Postman Item
func convertRequestToItem(req *models.HttpRequest, opts ExportOptions) Item {
	item := Item{
		ID:   uuid.New().String(),
		Name: req.Metadata.Name,
	}

	if item.Name == "" {
		item.Name = req.Name
	}
	if item.Name == "" {
		// Generate name from method and URL path
		item.Name = generateRequestName(req.Method, req.URL)
	}

	// Set description from note
	if req.Metadata.Note != "" {
		item.Description = &Description{
			Content: req.Metadata.Note,
			Type:    "text/plain",
		}
	}

	// Build request
	item.Request = convertRequest(req)

	// Add scripts
	if opts.IncludeScripts {
		if req.Metadata.PreScript != "" {
			convertedScript := convertScriptToPostman(req.Metadata.PreScript)
			item.Event = append(item.Event, Event{
				Listen: "prerequest",
				Script: &Script{
					Type: "text/javascript",
					Exec: strings.Split(convertedScript, "\n"),
				},
			})
		}

		if req.Metadata.PostScript != "" {
			convertedScript := convertScriptToPostman(req.Metadata.PostScript)
			item.Event = append(item.Event, Event{
				Listen: "test",
				Script: &Script{
					Type: "text/javascript",
					Exec: strings.Split(convertedScript, "\n"),
				},
			})
		}
	}

	return item
}

// convertRequest converts the request details
func convertRequest(req *models.HttpRequest) *Request {
	pmReq := &Request{
		Method: req.Method,
	}

	// Parse URL
	pmReq.URL = parseURL(req.URL)

	// Convert headers
	for key, value := range req.Headers {
		// Skip certain headers that Postman handles internally
		if strings.EqualFold(key, "content-length") {
			continue
		}

		// Check for auth headers
		if strings.EqualFold(key, "authorization") {
			auth := parseAuthHeader(value)
			if auth != nil {
				pmReq.Auth = auth
				continue
			}
		}

		pmReq.Header = append(pmReq.Header, Header{
			Key:   key,
			Value: value,
		})
	}

	// Convert body
	if req.RawBody != "" {
		pmReq.Body = convertBody(req)
	}

	return pmReq
}

// parseURL parses a URL string into Postman URL structure
func parseURL(rawURL string) *URL {
	pmURL := &URL{
		Raw: rawURL,
	}

	// Handle Postman-style variables (convert {{var}} to {{var}} - they're compatible)
	// But we need to parse without breaking on variables

	// Check if URL starts with a variable like {{baseUrl}}
	// In this case, url.Parse won't work correctly, so we need special handling
	varPrefixRegex := regexp.MustCompile(`^(\{\{[^}]+\}\})(.*)$`)
	if matches := varPrefixRegex.FindStringSubmatch(rawURL); matches != nil {
		// URL starts with a variable (e.g., {{baseUrl}}/path)
		hostVar := matches[1]   // e.g., "{{baseUrl}}"
		remainder := matches[2] // e.g., "/users/1/albums" or "/users?id=1"

		// Set host as the variable
		pmURL.Host = hostVar

		// Parse the remainder for path and query
		if remainder != "" {
			// Split query string if present
			pathPart := remainder
			queryPart := ""
			if idx := strings.Index(remainder, "?"); idx != -1 {
				pathPart = remainder[:idx]
				queryPart = remainder[idx+1:]
			}

			// Handle path - remove leading slash and split
			if pathPart != "" && pathPart != "/" {
				pathParts := strings.Split(strings.TrimPrefix(pathPart, "/"), "/")
				if len(pathParts) > 0 && pathParts[0] != "" {
					pmURL.Path = pathParts
				}
			}

			// Handle query parameters
			if queryPart != "" {
				parsedQuery, err := url.ParseQuery(queryPart)
				if err == nil {
					for key, values := range parsedQuery {
						for _, value := range values {
							k, v := key, value // Create local copies for pointer
							pmURL.Query = append(pmURL.Query, Query{
								Key:   &k,
								Value: &v,
							})
						}
					}
				}
			}
		}

		// Extract path variables (e.g., :id or {{id}})
		pathVarRegex := regexp.MustCompile(`:(\w+)|{{(\w+)}}`)
		if matches := pathVarRegex.FindAllStringSubmatch(rawURL, -1); matches != nil {
			for _, match := range matches {
				varName := match[1]
				if varName == "" {
					varName = match[2]
				}
				pmURL.Variable = append(pmURL.Variable, Variable{
					Key: varName,
				})
			}
		}

		return pmURL
	}

	// Try to parse the URL normally
	parsed, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, just use the raw URL
		return pmURL
	}

	pmURL.Protocol = parsed.Scheme

	// Handle host
	// NOTE: Postman's schema allows Host to be either a string or []string.
	// We use []string for multi-part hosts (e.g., "api.example.com" -> ["api", "example", "com"])
	// and string for single-part hosts (e.g., "localhost"). This matches Postman's behavior.
	host := parsed.Hostname()
	if host != "" {
		// Split host into parts
		parts := strings.Split(host, ".")
		if len(parts) > 1 {
			pmURL.Host = parts
		} else {
			pmURL.Host = host
		}
	}

	// Handle port
	if parsed.Port() != "" {
		pmURL.Port = parsed.Port()
	}

	// Handle path
	if parsed.Path != "" {
		pathParts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
		if len(pathParts) > 0 && pathParts[0] != "" {
			pmURL.Path = pathParts
		}
	}

	// Handle query parameters
	if parsed.RawQuery != "" {
		for key, values := range parsed.Query() {
			for _, value := range values {
				k, v := key, value // Create local copies for pointer
				pmURL.Query = append(pmURL.Query, Query{
					Key:   &k,
					Value: &v,
				})
			}
		}
	}

	// Handle fragment
	if parsed.Fragment != "" {
		pmURL.Hash = parsed.Fragment
	}

	// Extract path variables (e.g., :id or {{id}})
	pathVarRegex := regexp.MustCompile(`:(\w+)|{{(\w+)}}`)
	if matches := pathVarRegex.FindAllStringSubmatch(rawURL, -1); matches != nil {
		for _, match := range matches {
			varName := match[1]
			if varName == "" {
				varName = match[2]
			}
			pmURL.Variable = append(pmURL.Variable, Variable{
				Key: varName,
			})
		}
	}

	return pmURL
}

// parseAuthHeader parses an Authorization header into Postman auth
func parseAuthHeader(value string) *Auth {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) < 2 {
		return nil
	}

	authType := strings.ToLower(parts[0])
	credentials := parts[1]

	switch authType {
	case "basic":
		// Format: Basic username:password or Basic base64encoded
		var username, password string
		if strings.Contains(credentials, ":") {
			creds := strings.SplitN(credentials, ":", 2)
			username = creds[0]
			if len(creds) > 1 {
				password = creds[1]
			}
		}
		return &Auth{
			Type: "basic",
			Basic: []AuthAttribute{
				{Key: "username", Value: username, Type: "string"},
				{Key: "password", Value: password, Type: "string"},
			},
		}

	case "bearer":
		return &Auth{
			Type: "bearer",
			Bearer: []AuthAttribute{
				{Key: "token", Value: credentials, Type: "string"},
			},
		}

	case "digest":
		// Format: Digest username password
		creds := strings.SplitN(credentials, " ", 2)
		username := creds[0]
		password := ""
		if len(creds) > 1 {
			password = creds[1]
		}
		return &Auth{
			Type: "digest",
			Digest: []AuthAttribute{
				{Key: "username", Value: username, Type: "string"},
				{Key: "password", Value: password, Type: "string"},
			},
		}

	case "aws":
		// Format: AWS accessKey secretKey [token:... region:... service:...]
		awsAuth := &Auth{
			Type:  "awsv4",
			AWSv4: []AuthAttribute{},
		}

		// Parse AWS credentials
		awsParts := strings.Fields(credentials)
		if len(awsParts) >= 2 {
			awsAuth.AWSv4 = append(awsAuth.AWSv4,
				AuthAttribute{Key: "accessKey", Value: awsParts[0], Type: "string"},
				AuthAttribute{Key: "secretKey", Value: awsParts[1], Type: "string"},
			)

			// Parse optional parameters
			for _, part := range awsParts[2:] {
				if strings.HasPrefix(part, "token:") {
					awsAuth.AWSv4 = append(awsAuth.AWSv4,
						AuthAttribute{Key: "sessionToken", Value: strings.TrimPrefix(part, "token:"), Type: "string"})
				} else if strings.HasPrefix(part, "region:") {
					awsAuth.AWSv4 = append(awsAuth.AWSv4,
						AuthAttribute{Key: "region", Value: strings.TrimPrefix(part, "region:"), Type: "string"})
				} else if strings.HasPrefix(part, "service:") {
					awsAuth.AWSv4 = append(awsAuth.AWSv4,
						AuthAttribute{Key: "service", Value: strings.TrimPrefix(part, "service:"), Type: "string"})
				}
			}
		}

		return awsAuth
	}

	return nil
}

// convertBody converts the request body to Postman format
func convertBody(req *models.HttpRequest) *Body {
	body := &Body{}

	contentType := req.ContentType()

	// Check for multipart form-data
	if strings.Contains(strings.ToLower(contentType), "multipart/form-data") && len(req.MultipartParts) > 0 {
		body.Mode = "formdata"
		for _, part := range req.MultipartParts {
			fd := FormDataPair{
				Key: part.Name,
			}
			if part.IsFile {
				fd.Type = "file"
				if part.FilePath != "" {
					fd.Src = part.FilePath
				}
			} else {
				fd.Type = "text"
				fd.Value = part.Value
			}
			if part.ContentType != "" {
				fd.ContentType = part.ContentType
			}
			body.FormData = append(body.FormData, fd)
		}
		return body
	}

	// Check for form-urlencoded
	if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		body.Mode = "urlencoded"
		pairs := strings.Split(req.RawBody, "&")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) >= 1 {
				ue := URLEncodedPair{
					Key: kv[0],
				}
				if len(kv) > 1 {
					ue.Value = kv[1]
				}
				body.URLEncoded = append(body.URLEncoded, ue)
			}
		}
		return body
	}

	// Check for GraphQL
	if isGraphQLRequest(contentType, req.URL) {
		// Try to parse as GraphQL JSON
		var gqlPayload map[string]any
		if err := json.Unmarshal([]byte(req.RawBody), &gqlPayload); err == nil {
			if query, ok := gqlPayload["query"].(string); ok {
				body.Mode = "graphql"
				body.GraphQL = &GraphQL{
					Query: query,
				}
				if vars, ok := gqlPayload["variables"]; ok {
					if varBytes, err := json.Marshal(vars); err == nil {
						body.GraphQL.Variables = string(varBytes)
					}
				}
				return body
			}
		}
	}

	// Check for file reference
	if strings.HasPrefix(strings.TrimSpace(req.RawBody), "< ") {
		body.Mode = "file"
		filePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.RawBody), "< "))
		body.File = &File{
			Src: filePath,
		}
		return body
	}

	// Default to raw body
	body.Mode = "raw"
	body.Raw = req.RawBody

	// Set language hint based on content type
	if strings.Contains(strings.ToLower(contentType), constants.MIMEApplicationJSON) {
		body.Options = &BodyOptions{
			Raw: &RawOptions{
				Language: "json",
			},
		}
	} else if strings.Contains(strings.ToLower(contentType), "application/xml") ||
		strings.Contains(strings.ToLower(contentType), "text/xml") {
		body.Options = &BodyOptions{
			Raw: &RawOptions{
				Language: "xml",
			},
		}
	} else if strings.Contains(strings.ToLower(contentType), "text/html") {
		body.Options = &BodyOptions{
			Raw: &RawOptions{
				Language: "html",
			},
		}
	} else if strings.Contains(strings.ToLower(contentType), "text/javascript") ||
		strings.Contains(strings.ToLower(contentType), "application/javascript") {
		body.Options = &BodyOptions{
			Raw: &RawOptions{
				Language: "javascript",
			},
		}
	}

	return body
}

// isGraphQLRequest checks if the request is likely a GraphQL request
func isGraphQLRequest(contentType, url string) bool {
	if strings.HasSuffix(url, "/graphql") || strings.Contains(url, "/graphql?") {
		return true
	}
	return strings.Contains(strings.ToLower(contentType), "application/graphql")
}

// generateRequestName generates a name from method and URL
func generateRequestName(method, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return method + " Request"
	}

	path := parsed.Path
	if path == "" || path == "/" {
		return method + " " + parsed.Host
	}

	// Get last path segment
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Clean up path variables
		lastPart = strings.ReplaceAll(lastPart, "{{", "")
		lastPart = strings.ReplaceAll(lastPart, "}}", "")
		lastPart = strings.TrimPrefix(lastPart, ":")
		return method + " " + lastPart
	}

	return method + " Request"
}

// convertScriptToPostman converts rest-client scripts to Postman-compatible scripts
// It transforms client.* API calls to pm.* equivalents
func convertScriptToPostman(script string) string {
	result := script

	// Convert client.test() to pm.test()
	// client.test("name", function() { ... }) -> pm.test("name", function() { ... })
	result = strings.ReplaceAll(result, "client.test(", "pm.test(")

	// Convert client.assert() to pm.expect().to.be.true
	// client.assert(condition, "message") -> pm.expect(condition).to.be.true
	// Handle both single and double quotes
	assertWithDoubleQuoteRegex := regexp.MustCompile(`client\.assert\(([^,]+),\s*"([^"]+)"\)`)
	result = assertWithDoubleQuoteRegex.ReplaceAllString(result, `pm.expect($1, "$2").to.be.true`)

	assertWithSingleQuoteRegex := regexp.MustCompile(`client\.assert\(([^,]+),\s*'([^']+)'\)`)
	result = assertWithSingleQuoteRegex.ReplaceAllString(result, `pm.expect($1, '$2').to.be.true`)

	// Convert simple client.assert(condition) without message
	simpleAssertRegex := regexp.MustCompile(`client\.assert\(([^,)]+)\)`)
	result = simpleAssertRegex.ReplaceAllString(result, `pm.expect($1).to.be.true`)

	// Convert client.log() to console.log()
	result = strings.ReplaceAll(result, "client.log(", "console.log(")

	// Convert client.global.set() to pm.globals.set()
	result = strings.ReplaceAll(result, "client.global.set(", "pm.globals.set(")

	// Convert client.global.get() to pm.globals.get()
	result = strings.ReplaceAll(result, "client.global.get(", "pm.globals.get(")

	// Convert response.status to pm.response.code
	result = strings.ReplaceAll(result, "response.status", "pm.response.code")

	// Convert response.body to pm.response.json() for JSON responses
	// This is tricky because response.body is direct access vs pm.response.json() is a function
	// We need to be careful with this replacement
	result = strings.ReplaceAll(result, "response.body.", "pm.response.json().")
	result = strings.ReplaceAll(result, "response.body)", "pm.response.json())")

	// Convert response.headers.valueOf() to pm.response.headers.get()
	result = strings.ReplaceAll(result, "response.headers.valueOf(", "pm.response.headers.get(")

	// Convert utility functions
	// $uuid() -> pm.variables.replaceIn("{{$guid}}")
	result = strings.ReplaceAll(result, "$uuid()", `pm.variables.replaceIn("{{$guid}}")`)

	// $timestamp() -> Date.now()
	result = strings.ReplaceAll(result, "$timestamp()", "Date.now()")

	// $isoTimestamp() -> new Date().toISOString()
	result = strings.ReplaceAll(result, "$isoTimestamp()", "new Date().toISOString()")

	// $randomInt(min, max) - keep as is, need to provide helper or use Math.random
	randomIntRegex := regexp.MustCompile(`\$randomInt\((\d+),\s*(\d+)\)`)
	result = randomIntRegex.ReplaceAllString(result, `(Math.floor(Math.random() * ($2 - $1 + 1)) + $1)`)

	// $randomString(length) - convert to a simple implementation
	randomStringRegex := regexp.MustCompile(`\$randomString\((\d+)\)`)
	result = randomStringRegex.ReplaceAllString(result, `Array($1).fill(0).map(() => Math.random().toString(36).charAt(2)).join('')`)

	// $base64(str) -> btoa(str) or require('btoa')(str) in Postman
	result = strings.ReplaceAll(result, "$base64(", "btoa(")

	// $base64Decode(str) -> atob(str)
	result = strings.ReplaceAll(result, "$base64Decode(", "atob(")

	// Hash functions - Postman uses CryptoJS
	// $md5(str) -> CryptoJS.MD5(str).toString()
	md5Regex := regexp.MustCompile(`\$md5\(([^)]+)\)`)
	result = md5Regex.ReplaceAllString(result, `CryptoJS.MD5($1).toString()`)

	// $sha256(str) -> CryptoJS.SHA256(str).toString()
	sha256Regex := regexp.MustCompile(`\$sha256\(([^)]+)\)`)
	result = sha256Regex.ReplaceAllString(result, `CryptoJS.SHA256($1).toString()`)

	// $sha512(str) -> CryptoJS.SHA512(str).toString()
	sha512Regex := regexp.MustCompile(`\$sha512\(([^)]+)\)`)
	result = sha512Regex.ReplaceAllString(result, `CryptoJS.SHA512($1).toString()`)

	return result
}
