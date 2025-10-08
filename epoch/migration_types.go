package epoch

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestInfo contains information about a Gin request for migration
type RequestInfo struct {
	Body        interface{}
	Headers     http.Header
	Cookies     map[string]string
	QueryParams map[string]string
	GinContext  *gin.Context
}

// NewRequestInfo creates a new RequestInfo from a Gin context
func NewRequestInfo(c *gin.Context, body interface{}) *RequestInfo {
	// Copy headers
	headers := make(http.Header)
	if c.Request != nil && c.Request.Header != nil {
		for k, v := range c.Request.Header {
			headers[k] = v
		}
	}

	// Copy cookies
	cookies := make(map[string]string)
	if c.Request != nil {
		for _, cookie := range c.Request.Cookies() {
			cookies[cookie.Name] = cookie.Value
		}
	}

	// Copy query params
	queryParams := make(map[string]string)
	if c.Request != nil && c.Request.URL != nil {
		for k, v := range c.Request.URL.Query() {
			if len(v) > 0 {
				queryParams[k] = v[0]
			}
		}
	}

	return &RequestInfo{
		Body:        body,
		Headers:     headers,
		Cookies:     cookies,
		QueryParams: queryParams,
		GinContext:  c,
	}
}

// ResponseInfo contains information about a Gin response for migration
type ResponseInfo struct {
	Body       interface{}
	StatusCode int
	Headers    http.Header
	GinContext *gin.Context
}

// NewResponseInfo creates a new ResponseInfo from a Gin context
func NewResponseInfo(c *gin.Context, body interface{}) *ResponseInfo {
	// Copy headers
	headers := make(http.Header)
	for k, v := range c.Writer.Header() {
		headers[k] = v
	}

	return &ResponseInfo{
		Body:       body,
		StatusCode: c.Writer.Status(),
		Headers:    headers,
		GinContext: c,
	}
}

// SetCookie sets a cookie on the response
func (r *ResponseInfo) SetCookie(cookie *http.Cookie) {
	if r.GinContext != nil {
		r.GinContext.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)
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
