package models

import (
	"net/http"
	"strings"
	"time"

	"github.com/ideaspaper/restclient/internal/constants"
)

// HttpResponse represents an HTTP response
type HttpResponse struct {
	StatusCode       int
	StatusMessage    string
	HttpVersion      string
	Headers          map[string][]string
	Body             string
	BodySizeInBytes  int
	HeadersSizeBytes int
	BodyBuffer       []byte
	Timing           ResponseTiming
	Request          *HttpRequest
}

// ResponseTiming contains timing information for the response
type ResponseTiming struct {
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration
	Total            time.Duration
}

// NewHttpResponse creates a new HttpResponse from an http.Response
func NewHttpResponse(resp *http.Response, bodyBuffer []byte, timing ResponseTiming, request *HttpRequest) *HttpResponse {
	// Calculate header size
	headerSize := 0
	headers := make(map[string][]string)
	for k, v := range resp.Header {
		headers[k] = v
		headerSize += len(k) + 2 // ": "
		for i, hv := range v {
			headerSize += len(hv)
			if i < len(v)-1 {
				headerSize += 2 // ", "
			}
		}
		headerSize += 2 // "\r\n"
	}

	return &HttpResponse{
		StatusCode:       resp.StatusCode,
		StatusMessage:    resp.Status,
		HttpVersion:      resp.Proto,
		Headers:          headers,
		Body:             string(bodyBuffer),
		BodySizeInBytes:  len(bodyBuffer),
		HeadersSizeBytes: headerSize,
		BodyBuffer:       bodyBuffer,
		Timing:           timing,
		Request:          request,
	}
}

// ContentType returns the content type of the response
func (r *HttpResponse) ContentType() string {
	for k, v := range r.Headers {
		if strings.EqualFold(k, "content-type") && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// GetHeader returns a header value (case-insensitive)
func (r *HttpResponse) GetHeader(name string) string {
	for k, v := range r.Headers {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// IsJSON returns true if the response is JSON
func (r *HttpResponse) IsJSON() bool {
	ct := r.ContentType()
	return strings.Contains(ct, constants.MIMEApplicationJSON) || strings.Contains(ct, "+json")
}

// IsXML returns true if the response is XML
func (r *HttpResponse) IsXML() bool {
	ct := r.ContentType()
	return strings.Contains(ct, "application/xml") || strings.Contains(ct, "text/xml") || strings.Contains(ct, "+xml")
}

// IsHTML returns true if the response is HTML
func (r *HttpResponse) IsHTML() bool {
	ct := r.ContentType()
	return strings.Contains(ct, "text/html")
}
