// Package constants provides common constants used throughout the application.
package constants

// HTTP Header names (canonical form)
const (
	HeaderContentType     = "Content-Type"
	HeaderAuthorization   = "Authorization"
	HeaderAccept          = "Accept"
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderCacheControl    = "Cache-Control"
	HeaderContentLength   = "Content-Length"
	HeaderContentEncoding = "Content-Encoding"
	HeaderCookie          = "Cookie"
	HeaderSetCookie       = "Set-Cookie"
	HeaderUserAgent       = "User-Agent"
	HeaderHost            = "Host"
	HeaderConnection      = "Connection"
	HeaderLocation        = "Location"
)

// MIME types
const (
	MIMEApplicationJSON           = "application/json"
	MIMEApplicationXML            = "application/xml"
	MIMEApplicationFormURLEncoded = "application/x-www-form-urlencoded"
	MIMEMultipartFormData         = "multipart/form-data"
	MIMETextPlain                 = "text/plain"
	MIMETextHTML                  = "text/html"
	MIMETextXML                   = "text/xml"
	MIMEOctetStream               = "application/octet-stream"
)

// Authentication schemes
const (
	AuthSchemeBearer = "Bearer"
	AuthSchemeBasic  = "Basic"
	AuthSchemeDigest = "Digest"
)

// HTTP Methods
const (
	MethodGET     = "GET"
	MethodPOST    = "POST"
	MethodPUT     = "PUT"
	MethodDELETE  = "DELETE"
	MethodPATCH   = "PATCH"
	MethodHEAD    = "HEAD"
	MethodOPTIONS = "OPTIONS"
)

// File extensions
const (
	ExtHTTP = ".http"
	ExtREST = ".rest"
	ExtJSON = ".json"
	ExtEnv  = ".env"
)

// Default values
const (
	DefaultUserAgent = "restclient-cli"
	DefaultTimeout   = 0 // No timeout
)
