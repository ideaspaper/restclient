// Package variables provides variable resolution and substitution for HTTP
// request templates, supporting environment variables, system functions,
// file variables, and user prompts.
package variables

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ideaspaper/restclient/internal/httputil"
	"github.com/ideaspaper/restclient/pkg/errors"
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
	resolvedCache  map[string]string
	fileVariables  map[string]string
	environment    string
	envVariables   map[string]map[string]string
	requestResults map[string]RequestResult
	promptHandler  func(name, description string, isPassword bool) (string, error)
	currentDir     string
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
func (v *VariableProcessor) SetEnvironment(env string) {
	v.environment = env
}

// SetEnvironmentVariables sets environment variables from config
func (v *VariableProcessor) SetEnvironmentVariables(vars map[string]map[string]string) {
	v.envVariables = vars
}

// SetFileVariables merges file variables into existing ones
func (v *VariableProcessor) SetFileVariables(vars map[string]string) {
	maps.Copy(v.fileVariables, vars)
}

// SetRequestResult stores a request result for request variables
func (v *VariableProcessor) SetRequestResult(name string, result RequestResult) {
	v.requestResults[name] = result
}

// SetPromptHandler sets the handler for prompt variables
func (v *VariableProcessor) SetPromptHandler(handler func(name, description string, isPassword bool) (string, error)) {
	v.promptHandler = handler
}

// SetCurrentDir sets the current directory for dotenv resolution
func (v *VariableProcessor) SetCurrentDir(dir string) {
	v.currentDir = dir
}

// Process processes all variables in the given text
func (v *VariableProcessor) Process(text string) (string, error) {
	varRegex := regexp.MustCompile(`\{\{(.+?)\}\}`)

	lastIndex := 0
	var builder strings.Builder

	matches := varRegex.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		builder.WriteString(text[lastIndex:match[0]])

		varName := strings.TrimSpace(text[match[2]:match[3]])
		value, err := v.resolveVariable(varName)
		if err != nil {
			return "", errors.Wrapf(err, "failed to resolve variable %s", varName)
		}

		builder.WriteString(value)
		lastIndex = match[1]
	}
	builder.WriteString(text[lastIndex:])

	return builder.String(), nil
}

// resolveVariable resolves a single variable
func (v *VariableProcessor) resolveVariable(rawName string) (string, error) {
	key := strings.TrimSpace(rawName)
	if key == "" {
		return "", errors.NewValidationError("variable", "empty variable name")
	}

	if val, ok := v.resolvedCache[key]; ok {
		return val, nil
	}

	isEncoded := strings.HasPrefix(key, "%")
	name := key
	if isEncoded {
		name = key[1:]
		if name == "" {
			return "", errors.NewValidationError("variable", "url-encoded variable missing name")
		}
	}

	var value string
	var err error

	switch {
	case strings.HasPrefix(name, "$"):
		value, err = v.resolveSystemVariable(name)
	case strings.Contains(name, ".response.") || strings.Contains(name, ".request."):
		value, err = v.resolveRequestVariable(name)
	default:
		value, err = v.resolveFromScopes(name)
	}

	if err != nil {
		return "", err
	}

	if isEncoded {
		value = urlEncode(value)
	}

	v.resolvedCache[key] = value
	return value, nil
}

func (v *VariableProcessor) resolveFromScopes(name string) (string, error) {
	if val, ok := v.fileVariables[name]; ok {
		resolved, err := v.Process(val)
		if err != nil {
			return "", errors.Wrapf(err, "failed to resolve file variable %s", name)
		}
		return resolved, nil
	}

	return v.resolveEnvironmentVariable(name)
}

// resolveSystemVariable resolves system variables like $guid, $timestamp, etc.
func (v *VariableProcessor) resolveSystemVariable(name string) (string, error) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", errors.NewValidationError("system variable", "empty variable name")
	}

	varName := parts[0]

	switch varName {
	case "$guid", "$uuid":
		return uuid.New().String(), nil

	case "$timestamp":
		return v.resolveTimestamp(parts[1:])

	case "$datetime":
		return v.resolveDatetime(parts[1:], false)

	case "$localDatetime":
		return v.resolveDatetime(parts[1:], true)

	case "$randomInt":
		return v.resolveRandomInt(parts[1:])

	case "$processEnv":
		return v.resolveProcessEnv(parts[1:])

	case "$dotenv":
		return v.resolveDotenv(parts[1:])

	case "$prompt":
		return v.resolvePrompt(parts[1:])

	default:
		return "", errors.NewValidationErrorWithValue("system variable", varName, "unknown variable")
	}
}

// resolveTimestamp resolves $timestamp with optional offset
func (v *VariableProcessor) resolveTimestamp(args []string) (string, error) {
	t := time.Now().UTC()

	if len(args) >= 2 {
		offset, err := strconv.Atoi(args[0])
		if err != nil {
			return "", errors.NewValidationErrorWithValue("timestamp offset", args[0], "must be an integer")
		}
		unit := args[1]
		t = addDuration(t, offset, unit)
	}

	return strconv.FormatInt(t.Unix(), 10), nil
}

// resolveDatetime resolves $datetime with format and optional offset
func (v *VariableProcessor) resolveDatetime(args []string, local bool) (string, error) {
	if len(args) == 0 {
		return "", errors.NewValidationError("$datetime", "format argument required")
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
			return "", errors.NewValidationErrorWithValue("datetime offset", args[1], "must be an integer")
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
func (v *VariableProcessor) resolveRandomInt(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.NewValidationError("$randomInt", "requires min and max arguments")
	}

	min, err := strconv.Atoi(args[0])
	if err != nil {
		return "", errors.NewValidationErrorWithValue("min", args[0], "must be an integer")
	}

	max, err := strconv.Atoi(args[1])
	if err != nil {
		return "", errors.NewValidationErrorWithValue("max", args[1], "must be an integer")
	}

	if min >= max {
		return "", errors.NewValidationError("$randomInt", "min must be less than max")
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
func (v *VariableProcessor) resolveProcessEnv(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.NewValidationError("$processEnv", "requires variable name")
	}

	varName := args[0]

	// Check if it's a reference to environment variable name
	if strings.HasPrefix(varName, "%") {
		// Get the environment variable name from config
		envVarName := varName[1:]
		if val, err := v.resolveEnvironmentVariable(envVarName); err == nil {
			varName = val
		}
	}

	return os.Getenv(varName), nil
}

// resolveDotenv resolves $dotenv varName
func (v *VariableProcessor) resolveDotenv(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.NewValidationError("$dotenv", "requires variable name")
	}

	varName := args[0]

	// Check if it's a reference
	if strings.HasPrefix(varName, "%") {
		envVarName := varName[1:]
		if val, err := v.resolveEnvironmentVariable(envVarName); err == nil {
			varName = val
		}
	}

	// Find .env file using Viper
	dir := v.currentDir
	var envFile string

	// Try environment-specific .env first
	if v.environment != "" {
		envSpecific := filepath.Join(dir, ".env."+v.environment)
		if _, err := os.Stat(envSpecific); err == nil {
			envFile = envSpecific
		}
	}

	// Fall back to .env
	if envFile == "" {
		envFile = filepath.Join(dir, ".env")
	}

	// Use Viper to read the .env file
	vp := viper.New()
	vp.SetConfigFile(envFile)
	vp.SetConfigType("env")

	if err := vp.ReadInConfig(); err != nil {
		return "", errors.Wrapf(err, "dotenv file not found: %s", envFile)
	}

	val := vp.GetString(varName)
	if val == "" && !vp.IsSet(varName) {
		return "", errors.NewValidationErrorWithValue("dotenv variable", varName, "not found")
	}

	return val, nil
}

// resolvePrompt resolves $prompt variable by prompting the user for input
func (v *VariableProcessor) resolvePrompt(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.NewValidationError("$prompt", "requires a variable name")
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
	if v.promptHandler != nil {
		return v.promptHandler(varName, description, isPassword)
	}

	return "", errors.NewValidationErrorWithValue("$prompt", varName, "no prompt handler configured")
}

// resolveEnvironmentVariable resolves an environment variable from config
func (v *VariableProcessor) resolveEnvironmentVariable(name string) (string, error) {
	// Check shared environment first
	if shared, ok := v.envVariables["$shared"]; ok {
		if val, ok := shared[name]; ok {
			resolved, err := v.Process(val)
			if err != nil {
				return "", errors.Wrapf(err, "failed to resolve shared environment variable %s", name)
			}
			return resolved, nil
		}
	}

	if v.environment != "" {
		if env, ok := v.envVariables[v.environment]; ok {
			if val, ok := env[name]; ok {
				resolved, err := v.Process(val)
				if err != nil {
					return "", errors.Wrapf(err, "failed to resolve environment variable %s", name)
				}
				return resolved, nil
			}
		}
	}

	return "", errors.NewValidationErrorWithValue("variable", name, "not found")
}

// resolveRequestVariable resolves a request variable (e.g., loginAPI.response.body.$.token)
func (v *VariableProcessor) resolveRequestVariable(name string) (string, error) {
	// Parse the variable reference
	// Format: requestName.(response|request).(body|headers).(path|headerName)
	parts := strings.SplitN(name, ".", 4)
	if len(parts) < 4 {
		return "", errors.NewValidationErrorWithValue("request variable", name, "invalid format (expected: requestName.response.body.path)")
	}

	requestName := parts[0]
	reqOrResp := parts[1]
	bodyOrHeaders := parts[2]
	path := parts[3]

	result, ok := v.requestResults[requestName]
	if !ok {
		return "", errors.NewValidationErrorWithValue("request", requestName, "not found in cache")
	}

	if reqOrResp == "response" {
		if bodyOrHeaders == "body" {
			// JSONPath or XPath extraction
			return extractFromBody(result.Body, path)
		} else if bodyOrHeaders == "headers" {
			// Header extraction
			if val, ok := httputil.GetHeaderFromSlice(result.Headers, path); ok {
				return val, nil
			}
			return "", errors.NewValidationErrorWithValue("header", path, "not found")
		}
	}

	return "", errors.NewValidationErrorWithValue("request variable", name, "unsupported format")
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

// urlEncode performs RFC 3986 percent-encoding.
// It encodes all characters except unreserved characters (A-Z, a-z, 0-9, -, _, ., ~).
// This properly handles multi-byte UTF-8 characters by encoding each byte.
func urlEncode(s string) string {
	var builder strings.Builder
	for i := range len(s) {
		c := s[i]
		if isURLSafe(c) {
			builder.WriteByte(c)
		} else {
			builder.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return builder.String()
}

// isURLSafe checks if a byte is an unreserved character per RFC 3986
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

	return "", errors.NewValidationErrorWithValue("path", path, "unsupported path format (only JSONPath with $. or $[ prefix is supported)")
}

// extractJSONPath extracts a value using JSONPath with proper JSON parsing
func extractJSONPath(body, path string) (string, error) {
	// Parse JSON into a generic structure
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "", errors.Wrap(err, "invalid JSON body")
	}

	// Remove $ prefix and split path
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")

	// Handle empty path (just "$")
	if path == "" {
		return valueToString(data)
	}

	// Parse and navigate the path
	current := data
	parts := splitJSONPath(path)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Handle array index like "[0]" or "items[0]"
		if strings.Contains(part, "[") {
			bracketIdx := strings.Index(part, "[")
			fieldName := part[:bracketIdx]
			indexStr := part[bracketIdx+1 : strings.Index(part, "]")]

			// Navigate to field first if there's a field name
			if fieldName != "" {
				obj, ok := current.(map[string]any)
				if !ok {
					return "", errors.NewValidationError("JSONPath", fmt.Sprintf("expected object at '%s', got %T", fieldName, current))
				}
				val, exists := obj[fieldName]
				if !exists {
					return "", errors.NewValidationError("JSONPath", fmt.Sprintf("field '%s' not found", fieldName))
				}
				current = val
			}

			// Now get array element
			arr, ok := current.([]any)
			if !ok {
				return "", errors.NewValidationError("JSONPath", fmt.Sprintf("expected array, got %T", current))
			}

			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return "", errors.NewValidationErrorWithValue("array index", indexStr, "must be a valid integer")
			}

			if index < 0 || index >= len(arr) {
				return "", errors.NewValidationError("array index", fmt.Sprintf("index %d out of bounds (length %d)", index, len(arr)))
			}

			current = arr[index]
		} else {
			// Regular field access
			obj, ok := current.(map[string]any)
			if !ok {
				return "", errors.NewValidationError("JSONPath", fmt.Sprintf("expected object at '%s', got %T", part, current))
			}

			val, exists := obj[part]
			if !exists {
				return "", errors.NewValidationError("JSONPath", fmt.Sprintf("field '%s' not found", part))
			}
			current = val
		}
	}

	return valueToString(current)
}

// splitJSONPath splits a JSONPath into parts, handling array indices correctly
func splitJSONPath(path string) []string {
	var parts []string
	var current strings.Builder

	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '.' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else if c == '[' {
			// Include array index with the field name
			current.WriteByte(c)
			// Read until closing bracket
			for i++; i < len(path) && path[i] != ']'; i++ {
				current.WriteByte(path[i])
			}
			if i < len(path) {
				current.WriteByte(']')
			}
		} else {
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// valueToString converts a JSON value to its string representation
func valueToString(v any) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case float64:
		// Check if it's an integer
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10), nil
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(val), nil
	case nil:
		return "null", nil
	case map[string]any, []any:
		// For objects and arrays, return JSON representation
		bytes, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	default:
		return fmt.Sprintf("%v", val), nil
	}
}

// DuplicateVariable holds information about a duplicate variable
type DuplicateVariable struct {
	Name     string
	OldValue string
	NewValue string
}

// ParseFileVariablesResult contains parsed variables and any duplicates found
type ParseFileVariablesResult struct {
	Variables  map[string]string
	Duplicates []DuplicateVariable
}

// ParseFileVariables parses file variables from content
func ParseFileVariables(content string) map[string]string {
	result := ParseFileVariablesWithDuplicates(content)
	return result.Variables
}

// ParseFileVariablesWithDuplicates parses file variables and returns duplicate information
func ParseFileVariablesWithDuplicates(content string) *ParseFileVariablesResult {
	vars := make(map[string]string)
	var duplicates []DuplicateVariable

	// Match @variableName = value
	re := regexp.MustCompile(`(?m)^\s*@([^\s=]+)\s*=\s*(.*?)\s*$`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		name := match[1]
		value := match[2]

		// Process escape sequences
		value = processEscapes(value)

		// Check for duplicate
		if oldValue, exists := vars[name]; exists {
			duplicates = append(duplicates, DuplicateVariable{
				Name:     name,
				OldValue: oldValue,
				NewValue: value,
			})
		}

		vars[name] = value
	}

	return &ParseFileVariablesResult{
		Variables:  vars,
		Duplicates: duplicates,
	}
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
