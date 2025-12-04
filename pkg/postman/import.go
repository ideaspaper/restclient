package postman

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ideaspaper/restclient/pkg/errors"
)

// ImportOptions configures how collections are imported
type ImportOptions struct {
	// OutputDir is the directory where .http files will be created (multi-file mode)
	OutputDir string
	// OutputFile is the file path for single-file mode output
	OutputFile string
	// SingleFile creates a single .http file instead of one per folder
	SingleFile bool
	// IncludeVariables includes collection variables as file variables
	IncludeVariables bool
	// IncludeScripts includes pre-request and test scripts
	IncludeScripts bool
}

// NameTracker tracks used names and provides unique names for duplicates
type NameTracker struct {
	usedNames map[string]int
}

// NewNameTracker creates a new name tracker
func NewNameTracker() *NameTracker {
	return &NameTracker{
		usedNames: make(map[string]int),
	}
}

// GetUniqueName returns a unique name, appending _2, _3, etc. for duplicates
func (nt *NameTracker) GetUniqueName(name string) string {
	if name == "" {
		return ""
	}

	count, exists := nt.usedNames[name]
	if !exists {
		nt.usedNames[name] = 1
		return name
	}

	// Increment count and return unique name
	nt.usedNames[name] = count + 1
	return fmt.Sprintf("%s_%d", name, count+1)
}

// DefaultImportOptions returns sensible defaults
func DefaultImportOptions() ImportOptions {
	return ImportOptions{
		OutputDir:        ".",
		OutputFile:       "",
		SingleFile:       false,
		IncludeVariables: true,
		IncludeScripts:   true,
	}
}

// ImportResult contains information about imported requests
type ImportResult struct {
	FilesCreated   []string
	RequestsCount  int
	FoldersCount   int
	VariablesCount int
	Errors         []string
}

// Import reads a Postman collection file and converts it to .http file(s)
func Import(collectionPath string, opts ImportOptions) (*ImportResult, error) {
	data, err := os.ReadFile(collectionPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collection file")
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, errors.Wrap(err, "failed to parse collection JSON")
	}

	return ImportCollection(&collection, opts)
}

// ImportCollection converts a parsed Postman collection to .http file(s)
func ImportCollection(collection *Collection, opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{}

	// Validate schema version
	if collection.Info.Schema != SchemaV21 {
		return nil, errors.NewValidationErrorWithValue("Postman collection schema", collection.Info.Schema, fmt.Sprintf("unsupported (expected %s)", SchemaV21))
	}

	if opts.SingleFile {
		// Determine output file path
		filePath := opts.OutputFile
		if filePath == "" {
			// Default: use collection name in current directory
			filePath = filepath.Join(opts.OutputDir, sanitizeFileName(collection.Info.Name)+".http")
		}

		// Create parent directory if needed
		parentDir := filepath.Dir(filePath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return nil, errors.Wrap(err, "failed to create output directory")
		}

		// Create a single .http file with all requests
		content := generateHttpFile(collection, opts)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write file %s", filePath)
		}

		result.FilesCreated = append(result.FilesCreated, filePath)
		result.RequestsCount = countRequests(collection.Item)
		result.FoldersCount = countFolders(collection.Item)
		result.VariablesCount = len(collection.Variable)
	} else {
		// Create output directory with collection name as root folder
		collectionDir := filepath.Join(opts.OutputDir, sanitizeFileName(collection.Info.Name))
		if err := os.MkdirAll(collectionDir, 0755); err != nil {
			return nil, errors.Wrap(err, "failed to create output directory")
		}
		// Create separate files for each folder
		if err := processItems(collection, collection.Item, collectionDir, collectionDir, opts, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// processItems recursively processes items and creates .http files
func processItems(collection *Collection, items []Item, currentDir string, rootDir string, opts ImportOptions, result *ImportResult) error {
	var rootRequests []Item

	for _, item := range items {
		if item.IsFolder() {
			result.FoldersCount++

			if len(item.Item) > 0 {
				// Create subdirectory for folder
				folderDir := filepath.Join(currentDir, sanitizeFileName(item.Name))
				if err := os.MkdirAll(folderDir, 0755); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to create folder %s: %v", folderDir, err))
					continue
				}

				// Process folder items recursively
				if err := processItems(collection, item.Item, folderDir, rootDir, opts, result); err != nil {
					result.Errors = append(result.Errors, err.Error())
				}
			}
		} else {
			rootRequests = append(rootRequests, item)
			result.RequestsCount++
		}
	}

	// Write root-level requests to a file
	if len(rootRequests) > 0 {
		var content strings.Builder

		// Add variables at the top if this is the root collection directory
		if currentDir == rootDir && opts.IncludeVariables && len(collection.Variable) > 0 {
			result.VariablesCount = len(collection.Variable)
			writeVariables(&content, collection.Variable)
		}

		// Create name tracker for deduplication (per file)
		nameTracker := NewNameTracker()

		for i, item := range rootRequests {
			if i > 0 {
				content.WriteString("\n###\n\n")
			}
			writeRequest(&content, &item, collection.Auth, opts, nameTracker)
		}

		// Determine filename
		fileName := "requests.http"
		if currentDir != rootDir {
			// Use parent folder name
			fileName = filepath.Base(currentDir) + ".http"
		} else if collection.Info.Name != "" {
			fileName = sanitizeFileName(collection.Info.Name) + ".http"
		}

		filePath := filepath.Join(currentDir, fileName)
		if err := os.WriteFile(filePath, []byte(content.String()), 0644); err != nil {
			return errors.Wrapf(err, "failed to write file %s", filePath)
		}

		result.FilesCreated = append(result.FilesCreated, filePath)
	}

	return nil
}

// generateHttpFile generates a single .http file content from a collection
func generateHttpFile(collection *Collection, opts ImportOptions) string {
	var content strings.Builder

	// Write description as comment
	if collection.Info.Description != nil && collection.Info.Description.Content != "" {
		writeComment(&content, collection.Info.Description.Content)
		content.WriteString("\n")
	}

	// Write variables
	if opts.IncludeVariables && len(collection.Variable) > 0 {
		writeVariables(&content, collection.Variable)
	}

	// Create name tracker for deduplication
	nameTracker := NewNameTracker()

	// Write all requests
	writeItems(&content, collection.Item, collection.Auth, opts, 0, nameTracker)

	return content.String()
}

// writeItems recursively writes items to the content builder
func writeItems(content *strings.Builder, items []Item, collectionAuth *Auth, opts ImportOptions, depth int, nameTracker *NameTracker) {
	first := true
	for _, item := range items {
		if item.IsFolder() {
			// Write folder separator with name
			if !first {
				content.WriteString("\n")
			}
			content.WriteString("###")
			if item.Name != "" {
				content.WriteString(" ")
				content.WriteString(item.Name)
			}
			content.WriteString("\n\n")

			// Write folder description
			if item.Description != nil && item.Description.Content != "" {
				writeComment(content, item.Description.Content)
				content.WriteString("\n")
			}

			// Recursively write folder items
			writeItems(content, item.Item, collectionAuth, opts, depth+1, nameTracker)
		} else {
			if !first {
				content.WriteString("\n###\n\n")
			}
			writeRequest(content, &item, collectionAuth, opts, nameTracker)
		}
		first = false
	}
}

// writeVariables writes collection variables as .http file variables
func writeVariables(content *strings.Builder, variables []Variable) {
	for _, v := range variables {
		if v.Disabled {
			content.WriteString("# ")
		}
		key := v.Key
		if key == "" {
			key = v.ID
		}
		value := v.GetValue()
		content.WriteString(fmt.Sprintf("@%s = %s\n", key, value))
	}
	content.WriteString("\n")
}

// writeComment writes a multi-line comment
func writeComment(content *strings.Builder, text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		content.WriteString("# ")
		content.WriteString(line)
		content.WriteString("\n")
	}
}

// writeRequest writes a single request item
func writeRequest(content *strings.Builder, item *Item, collectionAuth *Auth, opts ImportOptions, nameTracker *NameTracker) {
	req := item.Request
	if req == nil {
		return
	}

	// Write request name metadata (with deduplication)
	if item.Name != "" {
		uniqueName := nameTracker.GetUniqueName(item.Name)
		content.WriteString(fmt.Sprintf("# @name %s\n", uniqueName))
	}

	// Write description as note
	if item.Description != nil && item.Description.Content != "" {
		content.WriteString(fmt.Sprintf("# @note %s\n", strings.ReplaceAll(item.Description.Content, "\n", " ")))
	}

	// Write pre-request script
	if opts.IncludeScripts {
		for _, event := range item.Event {
			if event.Listen == "prerequest" && event.Script != nil {
				script := event.Script.GetExec()
				if script != "" {
					script = convertScriptFromPostman(script)
					content.WriteString("< {%\n")
					for _, line := range strings.Split(script, "\n") {
						content.WriteString(line)
						content.WriteString("\n")
					}
					content.WriteString("%}\n")
				}
			}
		}
	}

	// Write request line
	method := req.Method
	if method == "" {
		method = "GET"
	}

	url := getRequestURL(req)
	content.WriteString(fmt.Sprintf("%s %s\n", method, url))

	// Write headers
	for _, header := range req.Header {
		if header.Disabled {
			content.WriteString("# ")
		}
		content.WriteString(fmt.Sprintf("%s: %s\n", header.Key, header.Value))
	}

	// Write auth header if needed
	writeAuthHeader(content, req.Auth, collectionAuth)

	// Write body
	if req.Body != nil && !req.Body.Disabled {
		writeBody(content, req.Body)
	}

	// Write post-request script (test script)
	if opts.IncludeScripts {
		for _, event := range item.Event {
			if event.Listen == "test" && event.Script != nil {
				script := event.Script.GetExec()
				if script != "" {
					script = convertScriptFromPostman(script)
					content.WriteString("\n> {%\n")
					for _, line := range strings.Split(script, "\n") {
						content.WriteString(line)
						content.WriteString("\n")
					}
					content.WriteString("%}\n")
				}
			}
		}
	}
}

// getRequestURL constructs the full URL from the request
func getRequestURL(req *Request) string {
	if req.URL == nil {
		return ""
	}

	// If raw URL is available, use it (preserving variables like {{baseUrl}})
	if req.URL.Raw != "" {
		return req.URL.Raw
	}

	// Otherwise, construct from parts
	var url strings.Builder

	if req.URL.Protocol != "" {
		url.WriteString(req.URL.Protocol)
		url.WriteString("://")
	}

	host := req.URL.GetHost()
	if host != "" {
		url.WriteString(host)
	}

	if req.URL.Port != "" {
		url.WriteString(":")
		url.WriteString(req.URL.Port)
	}

	path := req.URL.GetPath()
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			url.WriteString("/")
		}
		url.WriteString(path)
	}

	// Add query parameters
	if len(req.URL.Query) > 0 {
		first := true
		for _, q := range req.URL.Query {
			if q.Disabled {
				continue
			}
			key := q.GetKey()
			if key == "" {
				continue // Skip null/empty keys
			}
			if first {
				url.WriteString("?")
				first = false
			} else {
				url.WriteString("&")
			}
			url.WriteString(key)
			if val := q.GetValue(); val != "" {
				url.WriteString("=")
				url.WriteString(val)
			}
		}
	}

	if req.URL.Hash != "" {
		url.WriteString("#")
		url.WriteString(req.URL.Hash)
	}

	return url.String()
}

// writeAuthHeader writes authentication header based on auth config
func writeAuthHeader(content *strings.Builder, reqAuth, collectionAuth *Auth) {
	auth := reqAuth
	if auth == nil {
		auth = collectionAuth
	}
	if auth == nil {
		return
	}

	switch auth.Type {
	case "basic":
		username := auth.GetAttribute(auth.Basic, "username")
		password := auth.GetAttribute(auth.Basic, "password")
		if username != "" || password != "" {
			content.WriteString(fmt.Sprintf("Authorization: Basic %s:%s\n", username, password))
		}
	case "bearer":
		token := auth.GetAttribute(auth.Bearer, "token")
		if token != "" {
			content.WriteString(fmt.Sprintf("Authorization: Bearer %s\n", token))
		}
	case "digest":
		username := auth.GetAttribute(auth.Digest, "username")
		password := auth.GetAttribute(auth.Digest, "password")
		if username != "" {
			content.WriteString(fmt.Sprintf("Authorization: Digest %s %s\n", username, password))
		}
	case "awsv4":
		accessKey := auth.GetAttribute(auth.AWSv4, "accessKey")
		secretKey := auth.GetAttribute(auth.AWSv4, "secretKey")
		region := auth.GetAttribute(auth.AWSv4, "region")
		service := auth.GetAttribute(auth.AWSv4, "service")
		sessionToken := auth.GetAttribute(auth.AWSv4, "sessionToken")
		if accessKey != "" && secretKey != "" {
			authLine := fmt.Sprintf("Authorization: AWS %s %s", accessKey, secretKey)
			if sessionToken != "" {
				authLine += fmt.Sprintf(" token:%s", sessionToken)
			}
			if region != "" {
				authLine += fmt.Sprintf(" region:%s", region)
			}
			if service != "" {
				authLine += fmt.Sprintf(" service:%s", service)
			}
			content.WriteString(authLine + "\n")
		}
	case "apikey":
		key := auth.GetAttribute(auth.APIKey, "key")
		value := auth.GetAttribute(auth.APIKey, "value")
		in := auth.GetAttribute(auth.APIKey, "in")
		if key != "" && value != "" && in == "header" {
			content.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
		// Note: query parameter API keys are handled via URL
	case "noauth":
		// No authentication
	}
}

// writeBody writes the request body
func writeBody(content *strings.Builder, body *Body) {
	content.WriteString("\n")

	switch body.Mode {
	case "raw":
		content.WriteString(body.Raw)
		if !strings.HasSuffix(body.Raw, "\n") {
			content.WriteString("\n")
		}
	case "urlencoded":
		first := true
		for _, param := range body.URLEncoded {
			if param.Disabled {
				// Write disabled params as comments on separate lines
				content.WriteString(fmt.Sprintf("# %s=%s\n", param.Key, param.Value))
				continue
			}
			if !first {
				content.WriteString("&")
			}
			content.WriteString(param.Key)
			content.WriteString("=")
			content.WriteString(param.Value)
			first = false
		}
		if !first {
			content.WriteString("\n")
		}
	case "formdata":
		// Form data is handled via multipart, we'll write as comment for reference
		content.WriteString("# Multipart form-data:\n")
		for _, param := range body.FormData {
			if param.Disabled {
				content.WriteString("# ")
			}
			if param.Type == "file" {
				content.WriteString(fmt.Sprintf("# %s: [file]\n", param.Key))
			} else {
				content.WriteString(fmt.Sprintf("# %s: %s\n", param.Key, param.Value))
			}
		}
	case "graphql":
		if body.GraphQL != nil {
			// GraphQL requests are wrapped in JSON
			gql := map[string]any{
				"query": body.GraphQL.Query,
			}
			if body.GraphQL.Variables != "" {
				var vars any
				if err := json.Unmarshal([]byte(body.GraphQL.Variables), &vars); err == nil {
					gql["variables"] = vars
				} else {
					gql["variables"] = body.GraphQL.Variables
				}
			}
			if jsonBytes, err := json.MarshalIndent(gql, "", "  "); err == nil {
				content.WriteString(string(jsonBytes))
				content.WriteString("\n")
			}
		}
	case "file":
		if body.File != nil {
			if src, ok := body.File.Src.(string); ok && src != "" {
				content.WriteString(fmt.Sprintf("< %s\n", src))
			}
		}
	}
}

// sanitizeFileName makes a string safe for use as a filename
func sanitizeFileName(name string) string {
	// Replace invalid characters
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	// Trim spaces and dots from ends
	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")
	if result == "" {
		result = "collection"
	}
	return result
}

// countRequests counts the total number of requests in items
func countRequests(items []Item) int {
	count := 0
	for _, item := range items {
		if item.IsFolder() {
			count += countRequests(item.Item)
		} else {
			count++
		}
	}
	return count
}

// countFolders counts the total number of folders in items
func countFolders(items []Item) int {
	count := 0
	for _, item := range items {
		if item.IsFolder() {
			count++
			count += countFolders(item.Item)
		}
	}
	return count
}

// convertScriptFromPostman converts Postman scripts back to rest-client format
// It transforms pm.* API calls to client.* equivalents
func convertScriptFromPostman(script string) string {
	result := script

	// Convert pm.test() to client.test()
	result = strings.ReplaceAll(result, "pm.test(", "client.test(")

	// Convert pm.expect(...).to.be.true back to client.assert()
	// pm.expect(condition, "message").to.be.true -> client.assert(condition, "message")
	// Handle both single and double quotes
	expectWithDoubleQuoteMsgRegex := regexp.MustCompile(`pm\.expect\(([^,]+),\s*"([^"]+)"\)\.to\.be\.true`)
	result = expectWithDoubleQuoteMsgRegex.ReplaceAllString(result, `client.assert($1, "$2")`)

	expectWithSingleQuoteMsgRegex := regexp.MustCompile(`pm\.expect\(([^,]+),\s*'([^']+)'\)\.to\.be\.true`)
	result = expectWithSingleQuoteMsgRegex.ReplaceAllString(result, `client.assert($1, '$2')`)

	// pm.expect(condition).to.be.true -> client.assert(condition)
	expectSimpleRegex := regexp.MustCompile(`pm\.expect\(([^)]+)\)\.to\.be\.true`)
	result = expectSimpleRegex.ReplaceAllString(result, `client.assert($1)`)

	// Convert console.log() to client.log()
	result = strings.ReplaceAll(result, "console.log(", "client.log(")

	// Convert pm.globals.set() to client.global.set()
	result = strings.ReplaceAll(result, "pm.globals.set(", "client.global.set(")

	// Convert pm.globals.get() to client.global.get()
	result = strings.ReplaceAll(result, "pm.globals.get(", "client.global.get(")

	// Convert pm.response.code to response.status
	result = strings.ReplaceAll(result, "pm.response.code", "response.status")

	// Convert pm.response.json() to response.body
	// Need to handle pm.response.json().property -> response.body.property
	result = strings.ReplaceAll(result, "pm.response.json().", "response.body.")
	result = strings.ReplaceAll(result, "pm.response.json())", "response.body)")
	result = strings.ReplaceAll(result, "pm.response.json()", "response.body")

	// Convert pm.response.headers.get() to response.headers.valueOf()
	result = strings.ReplaceAll(result, "pm.response.headers.get(", "response.headers.valueOf(")

	// Convert pm.variables.replaceIn("{{$guid}}") to $uuid()
	result = strings.ReplaceAll(result, `pm.variables.replaceIn("{{$guid}}")`, "$uuid()")

	// Convert Date.now() to $timestamp()
	result = strings.ReplaceAll(result, "Date.now()", "$timestamp()")

	// Convert new Date().toISOString() to $isoTimestamp()
	result = strings.ReplaceAll(result, "new Date().toISOString()", "$isoTimestamp()")

	// Convert Math.floor(Math.random() * (max - min + 1)) + min) back to $randomInt(min, max)
	randomIntRegex := regexp.MustCompile(`\(Math\.floor\(Math\.random\(\)\s*\*\s*\((\d+)\s*-\s*(\d+)\s*\+\s*1\)\)\s*\+\s*(\d+)\)`)
	result = randomIntRegex.ReplaceAllString(result, `$$randomInt($3, $1)`)

	// Convert Array(n).fill(0).map(() => Math.random().toString(36).charAt(2)).join('') back to $randomString(n)
	randomStringRegex := regexp.MustCompile(`Array\((\d+)\)\.fill\(0\)\.map\(\(\)\s*=>\s*Math\.random\(\)\.toString\(36\)\.charAt\(2\)\)\.join\(''\)`)
	result = randomStringRegex.ReplaceAllString(result, `$$randomString($1)`)

	// Convert btoa() to $base64()
	result = strings.ReplaceAll(result, "btoa(", "$base64(")

	// Convert atob() to $base64Decode()
	result = strings.ReplaceAll(result, "atob(", "$base64Decode(")

	// Convert CryptoJS hash functions
	// CryptoJS.MD5(str).toString() -> $md5(str)
	md5Regex := regexp.MustCompile(`CryptoJS\.MD5\(([^)]+)\)\.toString\(\)`)
	result = md5Regex.ReplaceAllString(result, `$$md5($1)`)

	// CryptoJS.SHA256(str).toString() -> $sha256(str)
	sha256Regex := regexp.MustCompile(`CryptoJS\.SHA256\(([^)]+)\)\.toString\(\)`)
	result = sha256Regex.ReplaceAllString(result, `$$sha256($1)`)

	// CryptoJS.SHA512(str).toString() -> $sha512(str)
	sha512Regex := regexp.MustCompile(`CryptoJS\.SHA512\(([^)]+)\)\.toString\(\)`)
	result = sha512Regex.ReplaceAllString(result, `$$sha512($1)`)

	return result
}
