// Package postman provides import and export functionality for Postman Collection v2.1.0 format.
package postman

import (
	"encoding/json"
	"errors"
	"strconv"
)

// Collection represents a Postman Collection v2.1.0
type Collection struct {
	Info                    Info       `json:"info"`
	Item                    []Item     `json:"item"`
	Event                   []Event    `json:"event,omitempty"`
	Variable                []Variable `json:"variable,omitempty"`
	Auth                    *Auth      `json:"auth,omitempty"`
	ProtocolProfileBehavior any        `json:"protocolProfileBehavior,omitempty"`
}

// Info contains metadata about the collection
type Info struct {
	Name        string       `json:"name"`
	PostmanID   string       `json:"_postman_id,omitempty"`
	Description *Description `json:"description,omitempty"`
	Version     any          `json:"version,omitempty"`
	Schema      string       `json:"schema"`
}

// Item can be either a request item or a folder (item-group)
type Item struct {
	// Common fields
	Name        string       `json:"name,omitempty"`
	Description *Description `json:"description,omitempty"`
	Variable    []Variable   `json:"variable,omitempty"`
	Event       []Event      `json:"event,omitempty"`

	// Folder (item-group) specific
	Item []Item `json:"item,omitempty"`

	// Request item specific
	ID                      string     `json:"id,omitempty"`
	Request                 *Request   `json:"request,omitempty"`
	Response                []Response `json:"response,omitempty"`
	ProtocolProfileBehavior any        `json:"protocolProfileBehavior,omitempty"`
	Auth                    *Auth      `json:"auth,omitempty"`
}

// IsFolder returns true if this item is a folder (contains sub-items)
func (i *Item) IsFolder() bool {
	return len(i.Item) > 0 || i.Request == nil
}

// Request represents an HTTP request
type Request struct {
	URL         *URL         `json:"url,omitempty"`
	Auth        *Auth        `json:"auth,omitempty"`
	Proxy       *Proxy       `json:"proxy,omitempty"`
	Certificate *Cert        `json:"certificate,omitempty"`
	Method      string       `json:"method,omitempty"`
	Description *Description `json:"description,omitempty"`
	Header      []Header     `json:"header,omitempty"`
	Body        *Body        `json:"body,omitempty"`
}

// RequestString handles the case where request is just a string URL
type RequestString string

// URL represents a request URL
type URL struct {
	Raw      string     `json:"raw,omitempty"`
	Protocol string     `json:"protocol,omitempty"`
	Host     any        `json:"host,omitempty"` // Can be string or []string
	Path     any        `json:"path,omitempty"` // Can be string or []any
	Port     string     `json:"port,omitempty"`
	Query    []Query    `json:"query,omitempty"`
	Hash     string     `json:"hash,omitempty"`
	Variable []Variable `json:"variable,omitempty"`
}

// UnmarshalJSON handles both string and object URL formats
func (u *URL) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		u.Raw = str
		return nil
	}

	// Try object format
	type urlObj URL
	var obj urlObj
	if err := json.Unmarshal(data, &obj); err != nil {
		return errors.New("URL must be a string or object")
	}
	*u = URL(obj)
	return nil
}

// GetRaw returns the raw URL string
func (u *URL) GetRaw() string {
	if u == nil {
		return ""
	}
	return u.Raw
}

// GetHost returns the host as a string
func (u *URL) GetHost() string {
	if u == nil || u.Host == nil {
		return ""
	}
	switch h := u.Host.(type) {
	case string:
		return h
	case []any:
		parts := make([]string, 0, len(h))
		for _, p := range h {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			result := ""
			for i, p := range parts {
				if i > 0 {
					result += "."
				}
				result += p
			}
			return result
		}
	}
	return ""
}

// GetPath returns the path as a string
func (u *URL) GetPath() string {
	if u == nil || u.Path == nil {
		return ""
	}
	switch p := u.Path.(type) {
	case string:
		return p
	case []any:
		parts := make([]string, 0, len(p))
		for _, segment := range p {
			switch s := segment.(type) {
			case string:
				parts = append(parts, s)
			case map[string]any:
				// Path variable object with type and value
				if val, ok := s["value"].(string); ok {
					parts = append(parts, val)
				}
			}
		}
		if len(parts) > 0 {
			return "/" + joinPath(parts)
		}
	}
	return ""
}

func joinPath(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "/"
		}
		result += p
	}
	return result
}

// Query represents a URL query parameter
type Query struct {
	Key         *string      `json:"key"`             // Can be null per Postman schema
	Value       *string      `json:"value,omitempty"` // Can be null per Postman schema
	Disabled    bool         `json:"disabled,omitempty"`
	Description *Description `json:"description,omitempty"`
}

// GetKey returns the key string or empty if nil
func (q *Query) GetKey() string {
	if q.Key == nil {
		return ""
	}
	return *q.Key
}

// GetValue returns the value string or empty if nil
func (q *Query) GetValue() string {
	if q.Value == nil {
		return ""
	}
	return *q.Value
}

// Header represents an HTTP header
type Header struct {
	Key         string       `json:"key"`
	Value       string       `json:"value"`
	Disabled    bool         `json:"disabled,omitempty"`
	Description *Description `json:"description,omitempty"`
}

// Body represents the request body
type Body struct {
	Mode       string           `json:"mode,omitempty"`
	Raw        string           `json:"raw,omitempty"`
	GraphQL    *GraphQL         `json:"graphql,omitempty"`
	URLEncoded []URLEncodedPair `json:"urlencoded,omitempty"`
	FormData   []FormDataPair   `json:"formdata,omitempty"`
	File       *File            `json:"file,omitempty"`
	Options    *BodyOptions     `json:"options,omitempty"`
	Disabled   bool             `json:"disabled,omitempty"`
}

// GraphQL represents GraphQL request body
type GraphQL struct {
	Query     string `json:"query,omitempty"`
	Variables string `json:"variables,omitempty"`
}

// URLEncodedPair represents a URL-encoded form parameter
type URLEncodedPair struct {
	Key         string       `json:"key"`
	Value       string       `json:"value,omitempty"`
	Disabled    bool         `json:"disabled,omitempty"`
	Description *Description `json:"description,omitempty"`
}

// FormDataPair represents a form-data field
type FormDataPair struct {
	Key         string       `json:"key"`
	Value       string       `json:"value,omitempty"`
	Src         any          `json:"src,omitempty"` // Can be string, []string, or null
	Disabled    bool         `json:"disabled,omitempty"`
	Type        string       `json:"type,omitempty"` // "text" or "file"
	ContentType string       `json:"contentType,omitempty"`
	Description *Description `json:"description,omitempty"`
}

// File represents a file to upload
type File struct {
	Src     any    `json:"src,omitempty"` // Can be string or null
	Content string `json:"content,omitempty"`
}

// BodyOptions contains additional body configuration
type BodyOptions struct {
	Raw *RawOptions `json:"raw,omitempty"`
}

// RawOptions contains options for raw body mode
type RawOptions struct {
	Language string `json:"language,omitempty"`
}

// Response represents a saved response
type Response struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name,omitempty"`
	OriginalRequest *Request `json:"originalRequest,omitempty"`
	ResponseTime    any      `json:"responseTime,omitempty"` // Can be null, string, or number
	Timings         any      `json:"timings,omitempty"`
	Header          any      `json:"header,omitempty"` // Can be []Header, string, or null
	Cookie          []Cookie `json:"cookie,omitempty"`
	Body            string   `json:"body,omitempty"`
	Status          string   `json:"status,omitempty"`
	Code            int      `json:"code,omitempty"`
}

// Cookie represents an HTTP cookie
type Cookie struct {
	Domain     string `json:"domain"`
	Expires    string `json:"expires,omitempty"`
	MaxAge     string `json:"maxAge,omitempty"`
	HostOnly   bool   `json:"hostOnly,omitempty"`
	HTTPOnly   bool   `json:"httpOnly,omitempty"`
	Name       string `json:"name,omitempty"`
	Path       string `json:"path"`
	Secure     bool   `json:"secure,omitempty"`
	Session    bool   `json:"session,omitempty"`
	Value      string `json:"value,omitempty"`
	Extensions []any  `json:"extensions,omitempty"`
}

// Event represents a script event (prerequest or test)
type Event struct {
	ID       string  `json:"id,omitempty"`
	Listen   string  `json:"listen"` // "prerequest" or "test"
	Script   *Script `json:"script,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
}

// Script represents a JavaScript script
type Script struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Exec any    `json:"exec,omitempty"` // Can be []string or string
	Src  *URL   `json:"src,omitempty"`
	Name string `json:"name,omitempty"`
}

// GetExec returns the script content as a single string
func (s *Script) GetExec() string {
	if s == nil || s.Exec == nil {
		return ""
	}
	switch e := s.Exec.(type) {
	case string:
		return e
	case []any:
		result := ""
		for i, line := range e {
			if s, ok := line.(string); ok {
				if i > 0 {
					result += "\n"
				}
				result += s
			}
		}
		return result
	case []string:
		result := ""
		for i, line := range e {
			if i > 0 {
				result += "\n"
			}
			result += line
		}
		return result
	}
	return ""
}

// Variable represents a collection variable
type Variable struct {
	ID          string       `json:"id,omitempty"`
	Key         string       `json:"key,omitempty"`
	Value       any          `json:"value,omitempty"`
	Type        string       `json:"type,omitempty"` // "string", "boolean", "any", "number"
	Name        string       `json:"name,omitempty"`
	Description *Description `json:"description,omitempty"`
	System      bool         `json:"system,omitempty"`
	Disabled    bool         `json:"disabled,omitempty"`
}

// GetValue returns the variable value as a string
func (v *Variable) GetValue() string {
	if v.Value == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case string:
		return val
	case float64:
		// Format float without trailing zeros for integers
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

// Auth represents authentication configuration
type Auth struct {
	Type     string          `json:"type"`
	NoAuth   any             `json:"noauth,omitempty"`
	APIKey   []AuthAttribute `json:"apikey,omitempty"`
	AWSv4    []AuthAttribute `json:"awsv4,omitempty"`
	Basic    []AuthAttribute `json:"basic,omitempty"`
	Bearer   []AuthAttribute `json:"bearer,omitempty"`
	Digest   []AuthAttribute `json:"digest,omitempty"`
	EdgeGrid []AuthAttribute `json:"edgegrid,omitempty"`
	Hawk     []AuthAttribute `json:"hawk,omitempty"`
	NTLM     []AuthAttribute `json:"ntlm,omitempty"`
	OAuth1   []AuthAttribute `json:"oauth1,omitempty"`
	OAuth2   []AuthAttribute `json:"oauth2,omitempty"`
}

// GetAttribute returns the value of an auth attribute by key
func (a *Auth) GetAttribute(attrs []AuthAttribute, key string) string {
	for _, attr := range attrs {
		if attr.Key == key {
			if s, ok := attr.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// AuthAttribute represents a key-value pair for authentication
type AuthAttribute struct {
	Key   string `json:"key"`
	Value any    `json:"value,omitempty"`
	Type  string `json:"type,omitempty"`
}

// Proxy represents proxy configuration
type Proxy struct {
	Match    string `json:"match,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Tunnel   bool   `json:"tunnel,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Cert represents an SSL certificate
type Cert struct {
	Name       string    `json:"name,omitempty"`
	Matches    []string  `json:"matches,omitempty"`
	Key        *CertFile `json:"key,omitempty"`
	Cert       *CertFile `json:"cert,omitempty"`
	Passphrase string    `json:"passphrase,omitempty"`
}

// CertFile represents a certificate file path
type CertFile struct {
	Src string `json:"src,omitempty"`
}

// Description can be a string or an object
type Description struct {
	Content string `json:"content,omitempty"`
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
}

// UnmarshalJSON handles both string and object description formats
func (d *Description) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		d.Content = str
		d.Type = "text/plain"
		return nil
	}

	// Try object format
	type descObj struct {
		Content string `json:"content,omitempty"`
		Type    string `json:"type,omitempty"`
		Version string `json:"version,omitempty"`
	}
	var obj descObj
	if err := json.Unmarshal(data, &obj); err == nil {
		d.Content = obj.Content
		d.Type = obj.Type
		d.Version = obj.Version
		return nil
	}

	return nil
}

// MarshalJSON outputs description as a string if only content is set
func (d *Description) MarshalJSON() ([]byte, error) {
	if d == nil {
		return json.Marshal(nil)
	}
	if d.Type == "" && d.Version == "" {
		return json.Marshal(d.Content)
	}
	type descObj struct {
		Content string `json:"content,omitempty"`
		Type    string `json:"type,omitempty"`
		Version string `json:"version,omitempty"`
	}
	return json.Marshal(descObj{
		Content: d.Content,
		Type:    d.Type,
		Version: d.Version,
	})
}

// SchemaV21 is the Postman Collection v2.1.0 schema URL
const SchemaV21 = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

// NewCollection creates a new empty Postman collection
func NewCollection(name string) *Collection {
	return &Collection{
		Info: Info{
			Name:   name,
			Schema: SchemaV21,
		},
		Item: []Item{},
	}
}
