package variables

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// VariableType represents the type of variable
type VariableType int

const (
	VariableTypeSystem VariableType = iota
	VariableTypeEnvironment
	VariableTypeFile
	VariableTypeRequest
	VariableTypePrompt
)

// Variable represents a resolved variable
type Variable struct {
	Name    string
	Value   string
	Type    VariableType
	Error   string
	Warning string
}

// VariableProcessor processes variables in request content
type VariableProcessor struct {
	providers      []VariableProvider
	resolvedCache  map[string]string
	fileVariables  map[string]string
	environment    string
	envVariables   map[string]map[string]string
	requestResults map[string]RequestResult
	promptHandler  func(name, description string, isPassword bool) (string, error)
	currentDir     string
}

// VariableProvider is an interface for variable providers
type VariableProvider interface {
	Has(name string) bool
	Get(name string) (*Variable, error)
}

// RequestResult holds the result of a named request for request variables
type RequestResult struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
}

// NewVariableProcessor creates a new variable processor
func NewVariableProcessor() *VariableProcessor {
	return &VariableProcessor{
		resolvedCache:  make(map[string]string),
		fileVariables:  make(map[string]string),
		envVariables:   make(map[string]map[string]string),
		requestResults: make(map[string]RequestResult),
		currentDir:     ".",
	}
}

// SetEnvironment sets the current environment
func (vp *VariableProcessor) SetEnvironment(env string) {
	vp.environment = env
}

// SetEnvironmentVariables sets environment variables from config
func (vp *VariableProcessor) SetEnvironmentVariables(vars map[string]map[string]string) {
	vp.envVariables = vars
}

// SetFileVariables sets file variables
func (vp *VariableProcessor) SetFileVariables(vars map[string]string) {
	vp.fileVariables = vars
}

// SetRequestResult stores a request result for request variables
func (vp *VariableProcessor) SetRequestResult(name string, result RequestResult) {
	vp.requestResults[name] = result
}

// SetPromptHandler sets the handler for prompt variables
func (vp *VariableProcessor) SetPromptHandler(handler func(name, description string, isPassword bool) (string, error)) {
	vp.promptHandler = handler
}

// SetCurrentDir sets the current directory for dotenv resolution
func (vp *VariableProcessor) SetCurrentDir(dir string) {
	vp.currentDir = dir
}

// Process processes all variables in the given text
func (vp *VariableProcessor) Process(text string) (string, error) {
	varRegex := regexp.MustCompile(`\{\{(.+?)\}\}`)

	result := text
	lastIndex := 0
	var builder strings.Builder

	matches := varRegex.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		builder.WriteString(text[lastIndex:match[0]])

		varName := strings.TrimSpace(text[match[2]:match[3]])
		value, err := vp.resolveVariable(varName)
		if err != nil {
			// Keep original if cannot resolve
			builder.WriteString(text[match[0]:match[1]])
		} else {
			builder.WriteString(value)
		}

		lastIndex = match[1]
	}
	builder.WriteString(text[lastIndex:])
	result = builder.String()

	return result, nil
}

// resolveVariable resolves a single variable
func (vp *VariableProcessor) resolveVariable(name string) (string, error) {
	// Check cache first
	if val, ok := vp.resolvedCache[name]; ok {
		return val, nil
	}

	// Check if it's URL-encoded variable reference
	isEncoded := strings.HasPrefix(name, "%")
	if isEncoded {
		name = name[1:]
	}

	var value string
	var err error

	// System variables (start with $)
	if strings.HasPrefix(name, "$") {
		value, err = vp.resolveSystemVariable(name)
	} else if strings.Contains(name, ".response.") || strings.Contains(name, ".request.") {
		// Request variable
		value, err = vp.resolveRequestVariable(name)
	} else {
		// Try file variables, then environment variables
		if val, ok := vp.fileVariables[name]; ok {
			// Recursively resolve any variables in the file variable value
			value, _ = vp.Process(val)
		} else {
			value, err = vp.resolveEnvironmentVariable(name)
		}
	}

	if err != nil {
		return "", err
	}

	// URL encode if needed
	if isEncoded {
		value = urlEncode(value)
	}

	// Cache the result
	vp.resolvedCache[name] = value
	return value, nil
}

// resolveSystemVariable resolves system variables like $guid, $timestamp, etc.
func (vp *VariableProcessor) resolveSystemVariable(name string) (string, error) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty system variable")
	}

	varName := parts[0]

	switch varName {
	case "$guid", "$uuid":
		return uuid.New().String(), nil

	case "$timestamp":
		return vp.resolveTimestamp(parts[1:])

	case "$datetime":
		return vp.resolveDatetime(parts[1:], false)

	case "$localDatetime":
		return vp.resolveDatetime(parts[1:], true)

	case "$randomInt":
		return vp.resolveRandomInt(parts[1:])

	case "$processEnv":
		return vp.resolveProcessEnv(parts[1:])

	case "$dotenv":
		return vp.resolveDotenv(parts[1:])

	case "$prompt":
		return vp.resolvePrompt(parts[1:])

	default:
		return "", fmt.Errorf("unknown system variable: %s", varName)
	}
}

// resolveTimestamp resolves $timestamp with optional offset
func (vp *VariableProcessor) resolveTimestamp(args []string) (string, error) {
	t := time.Now().UTC()

	if len(args) >= 2 {
		offset, err := strconv.Atoi(args[0])
		if err != nil {
			return "", fmt.Errorf("invalid timestamp offset: %s", args[0])
		}
		unit := args[1]
		t = addDuration(t, offset, unit)
	}

	return strconv.FormatInt(t.Unix(), 10), nil
}

// resolveDatetime resolves $datetime with format and optional offset
func (vp *VariableProcessor) resolveDatetime(args []string, local bool) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("datetime format required")
	}

	format := args[0]
	t := time.Now()
	if !local {
		t = t.UTC()
	}

	// Apply offset if provided
	if len(args) >= 3 {
		offset, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("invalid datetime offset: %s", args[1])
		}
		unit := args[2]
		t = addDuration(t, offset, unit)
	}

	// Format the time
	switch format {
	case "rfc1123":
		return t.Format(time.RFC1123), nil
	case "iso8601":
		return t.Format(time.RFC3339), nil
	default:
		// Custom format - convert from Day.js format to Go format
		goFormat := convertDateFormat(format)
		return t.Format(goFormat), nil
	}
}

// resolveRandomInt resolves $randomInt min max
func (vp *VariableProcessor) resolveRandomInt(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("randomInt requires min and max arguments")
	}

	min, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid min value: %s", args[0])
	}

	max, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid max value: %s", args[1])
	}

	if min >= max {
		return "", fmt.Errorf("min must be less than max")
	}

	// Generate random number using uint32 to avoid overflow issues
	rangeSize := uint32(max - min)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomNum := uint32(randomBytes[0])<<24 | uint32(randomBytes[1])<<16 | uint32(randomBytes[2])<<8 | uint32(randomBytes[3])
	result := int(randomNum%rangeSize) + min

	return strconv.Itoa(result), nil
}

// resolveProcessEnv resolves $processEnv varName
func (vp *VariableProcessor) resolveProcessEnv(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("processEnv requires variable name")
	}

	varName := args[0]

	// Check if it's a reference to environment variable name
	if strings.HasPrefix(varName, "%") {
		// Get the environment variable name from config
		envVarName := varName[1:]
		if val, err := vp.resolveEnvironmentVariable(envVarName); err == nil {
			varName = val
		}
	}

	return os.Getenv(varName), nil
}

// resolveDotenv resolves $dotenv varName
func (vp *VariableProcessor) resolveDotenv(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("dotenv requires variable name")
	}

	varName := args[0]

	// Check if it's a reference
	if strings.HasPrefix(varName, "%") {
		envVarName := varName[1:]
		if val, err := vp.resolveEnvironmentVariable(envVarName); err == nil {
			varName = val
		}
	}

	// Find .env file using Viper
	dir := vp.currentDir
	var envFile string

	// Try environment-specific .env first
	if vp.environment != "" {
		envSpecific := filepath.Join(dir, ".env."+vp.environment)
		if _, err := os.Stat(envSpecific); err == nil {
			envFile = envSpecific
		}
	}

	// Fall back to .env
	if envFile == "" {
		envFile = filepath.Join(dir, ".env")
	}

	// Use Viper to read the .env file
	v := viper.New()
	v.SetConfigFile(envFile)
	v.SetConfigType("env")

	if err := v.ReadInConfig(); err != nil {
		return "", fmt.Errorf("dotenv file not found: %w", err)
	}

	val := v.GetString(varName)
	if val == "" && !v.IsSet(varName) {
		return "", fmt.Errorf("dotenv variable not found: %s", varName)
	}

	return val, nil
}

// resolvePrompt resolves $prompt variable by prompting the user for input
func (vp *VariableProcessor) resolvePrompt(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("$prompt requires a variable name")
	}

	varName := args[0]
	description := ""
	if len(args) > 1 {
		description = strings.Join(args[1:], " ")
	}

	// Check if this is a password field
	isPassword := false
	lowerName := strings.ToLower(varName)
	if lowerName == "password" || lowerName == "passwd" || lowerName == "pass" || lowerName == "secret" {
		isPassword = true
	}

	// Use the prompt handler if available
	if vp.promptHandler != nil {
		return vp.promptHandler(varName, description, isPassword)
	}

	return "", fmt.Errorf("no prompt handler configured for $prompt %s", varName)
}

// resolveEnvironmentVariable resolves an environment variable from config
func (vp *VariableProcessor) resolveEnvironmentVariable(name string) (string, error) {
	// Check shared environment first
	if shared, ok := vp.envVariables["$shared"]; ok {
		if val, ok := shared[name]; ok {
			// Recursively resolve
			resolved, _ := vp.Process(val)
			return resolved, nil
		}
	}

	// Check current environment
	if vp.environment != "" {
		if env, ok := vp.envVariables[vp.environment]; ok {
			if val, ok := env[name]; ok {
				resolved, _ := vp.Process(val)
				return resolved, nil
			}
		}
	}

	return "", fmt.Errorf("environment variable not found: %s", name)
}

// resolveRequestVariable resolves a request variable (e.g., loginAPI.response.body.$.token)
func (vp *VariableProcessor) resolveRequestVariable(name string) (string, error) {
	// Parse the variable reference
	// Format: requestName.(response|request).(body|headers).(path|headerName)
	parts := strings.SplitN(name, ".", 4)
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid request variable format: %s", name)
	}

	requestName := parts[0]
	reqOrResp := parts[1]
	bodyOrHeaders := parts[2]
	path := parts[3]

	result, ok := vp.requestResults[requestName]
	if !ok {
		return "", fmt.Errorf("request '%s' not found in cache", requestName)
	}

	if reqOrResp == "response" {
		if bodyOrHeaders == "body" {
			// JSONPath or XPath extraction
			return extractFromBody(result.Body, path)
		} else if bodyOrHeaders == "headers" {
			// Header extraction
			for k, v := range result.Headers {
				if strings.EqualFold(k, path) && len(v) > 0 {
					return v[0], nil
				}
			}
			return "", fmt.Errorf("header '%s' not found", path)
		}
	}

	return "", fmt.Errorf("unsupported request variable: %s", name)
}

// addDuration adds a duration to a time based on unit
func addDuration(t time.Time, offset int, unit string) time.Time {
	switch unit {
	case "y":
		return t.AddDate(offset, 0, 0)
	case "M":
		return t.AddDate(0, offset, 0)
	case "w":
		return t.AddDate(0, 0, offset*7)
	case "d":
		return t.AddDate(0, 0, offset)
	case "h":
		return t.Add(time.Duration(offset) * time.Hour)
	case "m":
		return t.Add(time.Duration(offset) * time.Minute)
	case "s":
		return t.Add(time.Duration(offset) * time.Second)
	case "ms":
		return t.Add(time.Duration(offset) * time.Millisecond)
	default:
		return t
	}
}

// convertDateFormat converts Day.js format to Go format
func convertDateFormat(format string) string {
	// Remove quotes
	format = strings.Trim(format, "'\"")

	// Ordered replacements - longer tokens MUST come before shorter ones
	// to prevent partial matching (e.g., YYYY before YY)
	replacements := []struct{ from, to string }{
		// 4-char tokens first
		{"YYYY", "2006"},
		{"MMMM", "January"},
		{"dddd", "Monday"},
		{"DDDD", "002"},
		// 3-char tokens
		{"MMM", "Jan"},
		{"ddd", "Mon"},
		{"SSS", "000"},
		// 2-char tokens
		{"YY", "06"},
		{"MM", "01"},
		{"DD", "02"},
		{"HH", "15"},
		{"hh", "03"},
		{"mm", "04"},
		{"ss", "05"},
		{"ZZ", "-0700"},
	}

	result := format
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.from, r.to)
	}

	return result
}

// urlEncode performs percent-encoding
func urlEncode(s string) string {
	var builder strings.Builder
	for _, c := range s {
		if isURLSafe(byte(c)) {
			builder.WriteRune(c)
		} else {
			builder.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return builder.String()
}

// isURLSafe checks if a character is safe for URLs
func isURLSafe(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~'
}

// extractFromBody extracts a value from body using JSONPath
func extractFromBody(body, path string) (string, error) {
	// Handle * for full body
	if path == "*" {
		return body, nil
	}

	// JSONPath (starts with $)
	if strings.HasPrefix(path, "$.") || strings.HasPrefix(path, "$[") {
		return extractJSONPath(body, path)
	}

	return "", fmt.Errorf("unsupported path format: %s (only JSONPath with $. or $[ prefix is supported)", path)
}

// extractJSONPath extracts a value using JSONPath
func extractJSONPath(body, path string) (string, error) {
	// Simple JSONPath implementation for common cases
	// Full implementation would use a JSONPath library

	// Remove $. prefix
	path = strings.TrimPrefix(path, "$.")
	parts := strings.Split(path, ".")

	// Parse JSON manually for simple cases
	current := body
	for _, part := range parts {
		// Handle array index
		if strings.Contains(part, "[") {
			name := part[:strings.Index(part, "[")]
			indexStr := part[strings.Index(part, "[")+1 : strings.Index(part, "]")]

			current = extractJSONField(current, name)
			if current == "" {
				return "", fmt.Errorf("field '%s' not found", name)
			}

			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return "", fmt.Errorf("invalid array index: %s", indexStr)
			}

			current = extractJSONArrayElement(current, index)
		} else {
			current = extractJSONField(current, part)
		}

		if current == "" {
			return "", fmt.Errorf("path not found: %s", path)
		}
	}

	// Remove quotes from string values
	current = strings.Trim(current, "\"")
	return current, nil
}

// extractJSONField extracts a field from JSON
func extractJSONField(json, field string) string {
	// Find "field": or "field" :
	pattern := fmt.Sprintf(`"%s"\s*:\s*`, regexp.QuoteMeta(field))
	re := regexp.MustCompile(pattern)
	loc := re.FindStringIndex(json)
	if loc == nil {
		return ""
	}

	start := loc[1]
	return extractJSONValue(json[start:])
}

// extractJSONValue extracts a JSON value starting at the current position
func extractJSONValue(json string) string {
	json = strings.TrimSpace(json)
	if len(json) == 0 {
		return ""
	}

	switch json[0] {
	case '"':
		// String value
		end := 1
		for end < len(json) {
			if json[end] == '"' && json[end-1] != '\\' {
				return json[:end+1]
			}
			end++
		}
	case '{':
		// Object
		depth := 1
		end := 1
		for end < len(json) && depth > 0 {
			if json[end] == '{' {
				depth++
			} else if json[end] == '}' {
				depth--
			}
			end++
		}
		return json[:end]
	case '[':
		// Array
		depth := 1
		end := 1
		for end < len(json) && depth > 0 {
			if json[end] == '[' {
				depth++
			} else if json[end] == ']' {
				depth--
			}
			end++
		}
		return json[:end]
	default:
		// Number, boolean, null
		end := 0
		for end < len(json) && json[end] != ',' && json[end] != '}' && json[end] != ']' && json[end] != ' ' && json[end] != '\n' {
			end++
		}
		return json[:end]
	}

	return ""
}

// extractJSONArrayElement extracts an element from a JSON array
func extractJSONArrayElement(json string, index int) string {
	json = strings.TrimSpace(json)
	if len(json) < 2 || json[0] != '[' {
		return ""
	}

	// Skip opening bracket
	json = json[1:]
	currentIndex := 0

	for len(json) > 0 && json[0] != ']' {
		json = strings.TrimSpace(json)
		value := extractJSONValue(json)
		if value == "" {
			return ""
		}

		if currentIndex == index {
			return value
		}

		json = json[len(value):]
		json = strings.TrimSpace(json)
		if len(json) > 0 && json[0] == ',' {
			json = json[1:]
		}
		currentIndex++
	}

	return ""
}

// ParseFileVariables parses file variables from content
func ParseFileVariables(content string) map[string]string {
	vars := make(map[string]string)

	// Match @variableName = value
	re := regexp.MustCompile(`(?m)^\s*@([^\s=]+)\s*=\s*(.*?)\s*$`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		name := match[1]
		value := match[2]

		// Process escape sequences
		value = processEscapes(value)
		vars[name] = value
	}

	return vars
}

// processEscapes processes escape sequences in a string
func processEscapes(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			default:
				result.WriteByte(s[i+1])
			}
			i += 2
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// GenerateGUID generates a new UUID
func GenerateGUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}
