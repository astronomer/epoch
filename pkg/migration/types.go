package migration

import (
	"net/http"
)

// RequestInfo contains information about an HTTP request for migration
type RequestInfo struct {
	Body        interface{}
	Headers     http.Header
	Cookies     map[string]string
	QueryParams map[string]string
	Request     *http.Request
}

// NewRequestInfo creates a new RequestInfo from an HTTP request
func NewRequestInfo(r *http.Request, body interface{}) *RequestInfo {
	// Copy headers
	headers := make(http.Header)
	for k, v := range r.Header {
		headers[k] = v
	}

	// Copy cookies
	cookies := make(map[string]string)
	for _, cookie := range r.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}

	// Copy query params
	queryParams := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			queryParams[k] = v[0]
		}
	}

	return &RequestInfo{
		Body:        body,
		Headers:     headers,
		Cookies:     cookies,
		QueryParams: queryParams,
		Request:     r,
	}
}

// ResponseInfo contains information about an HTTP response for migration
type ResponseInfo struct {
	Body       interface{}
	StatusCode int
	Headers    http.Header
	response   http.ResponseWriter
}

// NewResponseInfo creates a new ResponseInfo from an HTTP response
func NewResponseInfo(w http.ResponseWriter, body interface{}) *ResponseInfo {
	// Copy headers
	headers := make(http.Header)
	for k, v := range w.Header() {
		headers[k] = v
	}

	return &ResponseInfo{
		Body:       body,
		StatusCode: 200, // Default status code
		Headers:    headers,
		response:   w,
	}
}

// SetCookie sets a cookie on the response
func (r *ResponseInfo) SetCookie(cookie *http.Cookie) {
	if r.response != nil {
		http.SetCookie(r.response, cookie)
	}
}

// AlterRequestInstruction defines how to modify a request during migration
type AlterRequestInstruction struct {
	Schemas     []interface{} // Types this instruction applies to
	Path        string        // Path this instruction applies to (if path-based)
	Methods     []string      // HTTP methods this applies to (if path-based)
	Transformer func(*RequestInfo) error
}

// AlterResponseInstruction defines how to modify a response during migration
type AlterResponseInstruction struct {
	Schemas           []interface{} // Types this instruction applies to
	Path              string        // Path this instruction applies to (if path-based)
	Methods           []string      // HTTP methods this applies to (if path-based)
	MigrateHTTPErrors bool          // Whether to migrate error responses
	Transformer       func(*ResponseInfo) error
}

// SchemaInstruction defines changes to schema/model structures
type SchemaInstruction struct {
	Schema     interface{}
	Name       string
	Type       string // "had", "didnt_exist", etc.
	Attributes map[string]interface{}
	IsHidden   bool
}

// EndpointInstruction defines changes to API endpoints
type EndpointInstruction struct {
	Path       string
	Methods    []string
	FuncName   string
	Type       string // "existed", "didnt_exist", "had"
	Attributes map[string]interface{}
	IsHidden   bool
}

// EnumInstruction defines changes to enum types
type EnumInstruction struct {
	Enum     interface{}
	Type     string // "had_members", "didnt_have_members"
	Members  map[string]interface{}
	IsHidden bool
}

// ConvertRequestToNextVersionFor creates a request migration instruction for specific schemas
func ConvertRequestToNextVersionFor(schemas []interface{}, transformer func(*RequestInfo) error) *AlterRequestInstruction {
	return &AlterRequestInstruction{
		Schemas:     schemas,
		Transformer: transformer,
	}
}

// ConvertRequestToNextVersionForPath creates a request migration instruction for a specific path
func ConvertRequestToNextVersionForPath(path string, methods []string, transformer func(*RequestInfo) error) *AlterRequestInstruction {
	return &AlterRequestInstruction{
		Path:        path,
		Methods:     methods,
		Transformer: transformer,
	}
}

// ConvertResponseToPreviousVersionFor creates a response migration instruction for specific schemas
func ConvertResponseToPreviousVersionFor(schemas []interface{}, transformer func(*ResponseInfo) error, migrateHTTPErrors bool) *AlterResponseInstruction {
	return &AlterResponseInstruction{
		Schemas:           schemas,
		Transformer:       transformer,
		MigrateHTTPErrors: migrateHTTPErrors,
	}
}

// ConvertResponseToPreviousVersionForPath creates a response migration instruction for a specific path
func ConvertResponseToPreviousVersionForPath(path string, methods []string, transformer func(*ResponseInfo) error, migrateHTTPErrors bool) *AlterResponseInstruction {
	return &AlterResponseInstruction{
		Path:              path,
		Methods:           methods,
		Transformer:       transformer,
		MigrateHTTPErrors: migrateHTTPErrors,
	}
}
