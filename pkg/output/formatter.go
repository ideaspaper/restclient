package output

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/fatih/color"

	"github.com/ideaspaper/restclient/pkg/models"
)

// Formatter handles response formatting and colorization
type Formatter struct {
	colorEnabled bool

	// Colors
	statusSuccess     *color.Color
	statusRedirect    *color.Color
	statusClientError *color.Color
	statusServerError *color.Color
	headerName        *color.Color
	headerValue       *color.Color
	jsonKey           *color.Color
	jsonString        *color.Color
	jsonNumber        *color.Color
	jsonBool          *color.Color
	jsonNull          *color.Color
}

// NewFormatter creates a new formatter
func NewFormatter(colorEnabled bool) *Formatter {
	f := &Formatter{
		colorEnabled:      colorEnabled,
		statusSuccess:     color.New(color.FgGreen, color.Bold),
		statusRedirect:    color.New(color.FgYellow, color.Bold),
		statusClientError: color.New(color.FgRed, color.Bold),
		statusServerError: color.New(color.FgRed, color.Bold),
		headerName:        color.New(color.FgCyan),
		headerValue:       color.New(color.FgWhite),
		jsonKey:           color.New(color.FgCyan),
		jsonString:        color.New(color.FgGreen),
		jsonNumber:        color.New(color.FgYellow),
		jsonBool:          color.New(color.FgMagenta),
		jsonNull:          color.New(color.FgRed),
	}

	if !colorEnabled {
		color.NoColor = true
	}

	return f
}

// FormatResponse formats the full HTTP response
func (f *Formatter) FormatResponse(resp *models.HttpResponse) string {
	var buf bytes.Buffer

	// Status line
	buf.WriteString(f.formatStatusLine(resp))
	buf.WriteString("\n")

	// Headers
	buf.WriteString(f.FormatHeaders(resp))
	buf.WriteString("\n")

	// Body
	if resp.Body != "" {
		buf.WriteString(f.FormatBody(resp))
	}

	// Timing info
	buf.WriteString("\n")
	buf.WriteString(f.formatTiming(resp))

	return buf.String()
}

// FormatHeaders formats response headers
func (f *Formatter) FormatHeaders(resp *models.HttpResponse) string {
	var buf bytes.Buffer

	// Sort headers for consistent output
	for _, k := range slices.Sorted(maps.Keys(resp.Headers)) {
		values := resp.Headers[k]
		for _, v := range values {
			if f.colorEnabled {
				buf.WriteString(f.headerName.Sprint(k))
				buf.WriteString(": ")
				buf.WriteString(f.headerValue.Sprint(v))
			} else {
				buf.WriteString(fmt.Sprintf("%s: %s", k, v))
			}
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// FormatBody formats the response body
func (f *Formatter) FormatBody(resp *models.HttpResponse) string {
	body := resp.Body

	// Try to format based on content type
	if resp.IsJSON() {
		formatted := f.formatJSON(body)
		if formatted != "" {
			return formatted
		}
	}

	if resp.IsXML() {
		formatted := f.formatXML(body)
		if formatted != "" {
			return formatted
		}
	}

	return body
}

// formatStatusLine formats the HTTP status line
func (f *Formatter) formatStatusLine(resp *models.HttpResponse) string {
	statusColor := f.getStatusColor(resp.StatusCode)

	line := fmt.Sprintf("%s %d %s", resp.HttpVersion, resp.StatusCode, getStatusText(resp.StatusCode))

	if f.colorEnabled && statusColor != nil {
		return statusColor.Sprint(line)
	}
	return line
}

// getStatusColor returns the appropriate color for a status code
func (f *Formatter) getStatusColor(code int) *color.Color {
	switch {
	case code >= 200 && code < 300:
		return f.statusSuccess
	case code >= 300 && code < 400:
		return f.statusRedirect
	case code >= 400 && code < 500:
		return f.statusClientError
	case code >= 500:
		return f.statusServerError
	default:
		return nil
	}
}

// formatTiming formats the timing information
func (f *Formatter) formatTiming(resp *models.HttpResponse) string {
	t := resp.Timing

	var parts []string

	if t.Total > 0 {
		parts = append(parts, fmt.Sprintf("Total: %s", t.Total))
	}

	sizeInfo := fmt.Sprintf("Response: %d bytes", resp.BodySizeInBytes)

	if len(parts) == 0 {
		if f.colorEnabled {
			return color.New(color.FgHiBlack).Sprintf("(%s)", sizeInfo)
		}
		return fmt.Sprintf("(%s)", sizeInfo)
	}

	if f.colorEnabled {
		return color.New(color.FgHiBlack).Sprintf("(%s, %s)", strings.Join(parts, ", "), sizeInfo)
	}
	return fmt.Sprintf("(%s, %s)", strings.Join(parts, ", "), sizeInfo)
}

// formatJSON formats and colorizes JSON
func (f *Formatter) formatJSON(body string) string {
	// First, try to parse and pretty-print
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return ""
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ""
	}

	if !f.colorEnabled {
		return string(pretty)
	}

	// Colorize the JSON
	return f.colorizeJSON(string(pretty))
}

// colorizeJSON adds colors to JSON output
func (f *Formatter) colorizeJSON(jsonStr string) string {
	var result bytes.Buffer
	inString := false
	escaped := false

	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		if escaped {
			result.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' {
			result.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' {
			if !inString {
				// Starting a string - find the end
				end := i + 1
				for end < len(jsonStr) {
					if jsonStr[end] == '\\' {
						end += 2
						continue
					}
					if jsonStr[end] == '"' {
						break
					}
					end++
				}

				str := jsonStr[i : end+1]

				// Check if this is a key (followed by :)
				afterQuote := end + 1
				for afterQuote < len(jsonStr) && (jsonStr[afterQuote] == ' ' || jsonStr[afterQuote] == '\t') {
					afterQuote++
				}

				if afterQuote < len(jsonStr) && jsonStr[afterQuote] == ':' {
					result.WriteString(f.jsonKey.Sprint(str))
				} else {
					result.WriteString(f.jsonString.Sprint(str))
				}

				i = end
				continue
			}
		}

		// Check for numbers
		if !inString && (c >= '0' && c <= '9' || c == '-') {
			end := i
			for end < len(jsonStr) && (jsonStr[end] >= '0' && jsonStr[end] <= '9' || jsonStr[end] == '.' || jsonStr[end] == '-' || jsonStr[end] == 'e' || jsonStr[end] == 'E' || jsonStr[end] == '+') {
				end++
			}
			result.WriteString(f.jsonNumber.Sprint(jsonStr[i:end]))
			i = end - 1
			continue
		}

		// Check for booleans and null
		if !inString {
			remaining := jsonStr[i:]
			if strings.HasPrefix(remaining, "true") {
				result.WriteString(f.jsonBool.Sprint("true"))
				i += 3
				continue
			}
			if strings.HasPrefix(remaining, "false") {
				result.WriteString(f.jsonBool.Sprint("false"))
				i += 4
				continue
			}
			if strings.HasPrefix(remaining, "null") {
				result.WriteString(f.jsonNull.Sprint("null"))
				i += 3
				continue
			}
		}

		result.WriteByte(c)
	}

	return result.String()
}

// formatXML formats XML with proper indentation
func (f *Formatter) formatXML(body string) string {
	// Decode and re-encode with indentation
	decoder := xml.NewDecoder(strings.NewReader(body))
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Return original body on parse error
			return body
		}

		if err := encoder.EncodeToken(token); err != nil {
			return body
		}
	}

	if err := encoder.Flush(); err != nil {
		return body
	}

	result := buf.String()
	if result == "" {
		return body
	}

	return result
}

// getStatusText returns the text description for an HTTP status code
func getStatusText(code int) string {
	statusTexts := map[int]string{
		100: "Continue",
		101: "Switching Protocols",
		200: "OK",
		201: "Created",
		202: "Accepted",
		204: "No Content",
		206: "Partial Content",
		301: "Moved Permanently",
		302: "Found",
		303: "See Other",
		304: "Not Modified",
		307: "Temporary Redirect",
		308: "Permanent Redirect",
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		406: "Not Acceptable",
		408: "Request Timeout",
		409: "Conflict",
		410: "Gone",
		415: "Unsupported Media Type",
		422: "Unprocessable Entity",
		429: "Too Many Requests",
		500: "Internal Server Error",
		501: "Not Implemented",
		502: "Bad Gateway",
		503: "Service Unavailable",
		504: "Gateway Timeout",
	}

	if text, ok := statusTexts[code]; ok {
		return text
	}
	return ""
}

// FormatError formats an error message
func (f *Formatter) FormatError(err error) string {
	if f.colorEnabled {
		return color.New(color.FgRed).Sprintf("Error: %s", err.Error())
	}
	return fmt.Sprintf("Error: %s", err.Error())
}

// FormatSuccess formats a success message
func (f *Formatter) FormatSuccess(msg string) string {
	if f.colorEnabled {
		return color.New(color.FgGreen).Sprint(msg)
	}
	return msg
}

// FormatInfo formats an info message
func (f *Formatter) FormatInfo(msg string) string {
	if f.colorEnabled {
		return color.New(color.FgCyan).Sprint(msg)
	}
	return msg
}
