package parser

import (
	"regexp"
	"strings"

	"restclient/pkg/models"
)

// CurlParser parses cURL commands into HttpRequest
type CurlParser struct {
	command        string
	defaultHeaders map[string]string
}

// NewCurlParser creates a new cURL parser
func NewCurlParser(command string, defaultHeaders map[string]string) *CurlParser {
	if defaultHeaders == nil {
		defaultHeaders = map[string]string{
			"User-Agent": "restclient-cli",
		}
	}
	return &CurlParser{
		command:        command,
		defaultHeaders: defaultHeaders,
	}
}

// Parse parses a cURL command into an HttpRequest
func (p *CurlParser) Parse() (*models.HttpRequest, error) {
	// Normalize the command
	cmd := mergeIntoSingleLine(p.command)
	cmd = mergeMultipleSpacesIntoSingle(cmd)
	cmd = strings.TrimSpace(cmd)

	// Fix common issues
	// Handle -XMETHOD (no space between -X and method)
	cmd = regexp.MustCompile(`(-X)(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT|TRACE)`).
		ReplaceAllString(cmd, "$1 $2")
	// Handle -I/--head as -X HEAD
	cmd = regexp.MustCompile(`(-I|--head)(\s+)`).ReplaceAllString(cmd, "-X HEAD$2")

	args := parseArgs(cmd)

	// Parse URL
	url := extractURL(args)
	if url == "" {
		// Try to find URL from -L, --location, --url, or --compressed
		if val := getArgValue(args, "-L", "--location", "--url"); val != "" {
			url = val
		} else if val := getArgValue(args, "--compressed"); val != "" && strings.HasPrefix(val, "http") {
			url = val
		}
	}

	// Parse headers
	headers := make(map[string]string)
	for k, v := range p.defaultHeaders {
		headers[k] = v
	}

	headerValues := getArgValues(args, "-H", "--header")
	for _, h := range headerValues {
		colonIdx := strings.Index(h, ":")
		if colonIdx > 0 {
			name := strings.TrimSpace(h[:colonIdx])
			value := strings.TrimSpace(h[colonIdx+1:])
			headers[name] = value
		}
	}

	// Parse cookie
	cookieString := getArgValue(args, "-b", "--cookie")
	if cookieString != "" && strings.Contains(cookieString, "=") {
		headers["Cookie"] = cookieString
	}

	// Parse user authentication
	user := getArgValue(args, "-u", "--user")
	if user != "" {
		headers["Authorization"] = "Basic " + base64Encode(user)
	}

	// Parse body
	body := getArgValue(args, "-d", "--data", "--data-ascii", "--data-binary", "--data-raw")

	// Handle multiple -d flags
	dataValues := getArgValues(args, "-d", "--data", "--data-ascii", "--data-binary", "--data-raw")
	if len(dataValues) > 1 {
		body = strings.Join(dataValues, "&")
	}

	// Handle @file reference in body
	if strings.HasPrefix(body, "@") {
		// File reference - would need to be resolved
		body = body[1:] // Remove @ for now
	}

	// Set Content-Type if body exists and header not set
	if body != "" {
		if _, hasContentType := getHeaderCI(headers, "Content-Type"); !hasContentType {
			headers["Content-Type"] = "application/x-www-form-urlencoded"
		}
	}

	// Parse method
	method := getArgValue(args, "-X", "--request")
	if method == "" {
		if body != "" {
			method = "POST"
		} else {
			method = "GET"
		}
	}
	method = strings.ToUpper(method)

	// Check for -I (HEAD request)
	if hasArg(args, "-I", "--head") && method == "GET" {
		method = "HEAD"
	}

	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	return models.NewHttpRequest(method, url, headers, bodyReader, body, ""), nil
}

// parseArgs parses a command line string into arguments
func parseArgs(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else if c == '\\' && i+1 < len(cmd) {
				// Escape character
				i++
				current.WriteByte(cmd[i])
			} else {
				current.WriteByte(c)
			}
		} else {
			if c == '"' || c == '\'' {
				inQuote = true
				quoteChar = c
			} else if c == ' ' || c == '\t' {
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			} else if c == '\\' && i+1 < len(cmd) {
				// Escape character outside quotes
				i++
				current.WriteByte(cmd[i])
			} else {
				current.WriteByte(c)
			}
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// extractURL finds the URL from arguments (first non-flag argument after "curl")
func extractURL(args []string) string {
	skipNext := false
	inCurl := false

	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if strings.ToLower(arg) == "curl" {
			inCurl = true
			continue
		}

		if !inCurl {
			continue
		}

		// Skip flags and their values
		if strings.HasPrefix(arg, "-") {
			// Check if this flag takes a value
			if isValueFlag(arg) {
				skipNext = true
			}
			continue
		}

		// This should be the URL
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return arg
		}
	}

	// Try to find any URL-like argument
	for _, arg := range args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return arg
		}
	}

	return ""
}

// isValueFlag returns true if the flag takes a value
func isValueFlag(flag string) bool {
	valueFlags := map[string]bool{
		"-X": true, "--request": true,
		"-H": true, "--header": true,
		"-d": true, "--data": true, "--data-ascii": true, "--data-binary": true, "--data-raw": true,
		"-u": true, "--user": true,
		"-b": true, "--cookie": true,
		"-o": true, "--output": true,
		"-A": true, "--user-agent": true,
		"-e": true, "--referer": true,
		"-L": true, "--location": true,
		"-T": true, "--upload-file": true,
		"--url": true,
		"-c":    true, "--cookie-jar": true,
	}
	return valueFlags[flag]
}

// getArgValue gets the value of a flag from arguments
func getArgValue(args []string, flags ...string) string {
	for i := 0; i < len(args); i++ {
		for _, flag := range flags {
			if args[i] == flag && i+1 < len(args) {
				return args[i+1]
			}
			// Handle --flag=value or -f=value
			if strings.HasPrefix(args[i], flag+"=") {
				return args[i][len(flag)+1:]
			}
		}
	}
	return ""
}

// getArgValues gets all values of a flag from arguments
func getArgValues(args []string, flags ...string) []string {
	var values []string
	for i := 0; i < len(args); i++ {
		for _, flag := range flags {
			if args[i] == flag && i+1 < len(args) {
				values = append(values, args[i+1])
				i++
				break
			}
			// Handle --flag=value or -f=value
			if strings.HasPrefix(args[i], flag+"=") {
				values = append(values, args[i][len(flag)+1:])
				break
			}
		}
	}
	return values
}

// hasArg checks if a flag exists in arguments
func hasArg(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag {
				return true
			}
		}
	}
	return false
}

// getHeaderCI gets a header value case-insensitively
func getHeaderCI(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// mergeIntoSingleLine removes line continuation characters
func mergeIntoSingleLine(text string) string {
	text = strings.ReplaceAll(text, "\\\r\n", " ")
	text = strings.ReplaceAll(text, "\\\n", " ")
	text = strings.ReplaceAll(text, "\\\r", " ")
	return text
}

// mergeMultipleSpacesIntoSingle replaces multiple spaces with single space
func mergeMultipleSpacesIntoSingle(text string) string {
	re := regexp.MustCompile(`\s{2,}`)
	return re.ReplaceAllString(text, " ")
}

// base64Encode encodes a string to base64
func base64Encode(s string) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	input := []byte(s)
	var output strings.Builder

	for i := 0; i < len(input); i += 3 {
		var b1, b2, b3 byte
		b1 = input[i]
		if i+1 < len(input) {
			b2 = input[i+1]
		}
		if i+2 < len(input) {
			b3 = input[i+2]
		}

		output.WriteByte(base64Chars[b1>>2])
		output.WriteByte(base64Chars[((b1&0x03)<<4)|(b2>>4)])

		if i+1 < len(input) {
			output.WriteByte(base64Chars[((b2&0x0f)<<2)|(b3>>6)])
		} else {
			output.WriteByte('=')
		}

		if i+2 < len(input) {
			output.WriteByte(base64Chars[b3&0x3f])
		} else {
			output.WriteByte('=')
		}
	}

	return output.String()
}

// IsCurlCommand checks if the text is a cURL command
func IsCurlCommand(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(strings.ToLower(trimmed), "curl ")
}

// ParseCurl parses a cURL command string
func ParseCurl(command string, defaultHeaders map[string]string) (*models.HttpRequest, error) {
	parser := NewCurlParser(command, defaultHeaders)
	return parser.Parse()
}
