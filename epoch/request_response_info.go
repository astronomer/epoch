package epoch

import (
	"errors"
	"net/http"
	"reflect"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
)

// RequestInfo contains information about a Gin request for migration
type RequestInfo struct {
	Body        *ast.Node // Sonic AST Node preserves field order
	Headers     http.Header
	Cookies     map[string]string
	QueryParams map[string]string
	GinContext  *gin.Context

	// Chain-level schema matching context (prevents re-matching in multi-step migrations)
	schemaMatched     bool
	matchedSchemaType reflect.Type
}

// NewRequestInfo creates a new RequestInfo from a Gin context
func NewRequestInfo(c *gin.Context, body *ast.Node) *RequestInfo {
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
	Body       *ast.Node // Sonic AST Node preserves field order
	StatusCode int
	Headers    http.Header
	GinContext *gin.Context

	// Chain-level schema matching context (prevents re-matching in multi-step migrations)
	schemaMatched     bool
	matchedSchemaType reflect.Type
}

// NewResponseInfo creates a new ResponseInfo from a Gin context
func NewResponseInfo(c *gin.Context, body *ast.Node) *ResponseInfo {
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

// Helper methods for RequestInfo

// GetField gets a field from the request body
func (r *RequestInfo) GetField(key string) *ast.Node {
	if r.Body == nil {
		return nil
	}
	return r.Body.Get(key)
}

// GetFieldString gets a field value as a string
func (r *RequestInfo) GetFieldString(key string) (string, error) {
	node := r.GetField(key)
	if node == nil || !node.Exists() {
		return "", errors.New("field not found")
	}
	return node.String()
}

// GetFieldInt gets a field value as an int64
func (r *RequestInfo) GetFieldInt(key string) (int64, error) {
	node := r.GetField(key)
	if node == nil || !node.Exists() {
		return 0, errors.New("field not found")
	}
	return node.Int64()
}

// GetFieldFloat gets a field value as a float64
func (r *RequestInfo) GetFieldFloat(key string) (float64, error) {
	node := r.GetField(key)
	if node == nil || !node.Exists() {
		return 0, errors.New("field not found")
	}
	return node.Float64()
}

// SetField sets a field in the request body
func (r *RequestInfo) SetField(key string, value interface{}) error {
	if r.Body == nil {
		return errors.New("body is nil")
	}
	_, err := r.Body.SetAny(key, value)
	return err
}

// DeleteField deletes a field from the request body
func (r *RequestInfo) DeleteField(key string) error {
	if r.Body == nil {
		return nil
	}
	_, err := r.Body.Unset(key)
	return err
}

// HasField checks if a field exists in the request body
func (r *RequestInfo) HasField(key string) bool {
	if r.Body == nil {
		return false
	}
	field := r.Body.Get(key)
	return field != nil && field.Exists()
}

// TransformArrayField applies a transformation to each item in an array field
// If key is empty, transforms the root body if it's an array
func (r *RequestInfo) TransformArrayField(key string, transformer func(*ast.Node) error) error {
	if r.Body == nil {
		return nil
	}

	var array *ast.Node

	if key == "" {
		// Transform root body if it's an array
		if r.Body.TypeSafe() == ast.V_ARRAY {
			array = r.Body
		} else {
			// Not an array, nothing to do
			return nil
		}
	} else {
		// Transform named array field
		array = r.Body.Get(key)
		if array == nil || array.TypeSafe() != ast.V_ARRAY {
			return nil
		}
	}

	// Iterate over the array
	length, err := array.Len()
	if err != nil {
		return err
	}

	for i := 0; i < length; i++ {
		item := array.Index(i)
		if err := transformer(item); err != nil {
			return err
		}
	}

	return nil
}

// Helper methods for ResponseInfo

// GetField gets a field from the response body
func (r *ResponseInfo) GetField(key string) *ast.Node {
	if r.Body == nil {
		return nil
	}
	return r.Body.Get(key)
}

// GetFieldString gets a field value as a string
func (r *ResponseInfo) GetFieldString(key string) (string, error) {
	node := r.GetField(key)
	if node == nil || !node.Exists() {
		return "", errors.New("field not found")
	}
	return node.String()
}

// GetFieldInt gets a field value as an int64
func (r *ResponseInfo) GetFieldInt(key string) (int64, error) {
	node := r.GetField(key)
	if node == nil || !node.Exists() {
		return 0, errors.New("field not found")
	}
	return node.Int64()
}

// GetFieldFloat gets a field value as a float64
func (r *ResponseInfo) GetFieldFloat(key string) (float64, error) {
	node := r.GetField(key)
	if node == nil || !node.Exists() {
		return 0, errors.New("field not found")
	}
	return node.Float64()
}

// SetField sets a field in the response body
func (r *ResponseInfo) SetField(key string, value interface{}) error {
	if r.Body == nil {
		return errors.New("body is nil")
	}
	_, err := r.Body.SetAny(key, value)
	return err
}

// DeleteField deletes a field from the response body
func (r *ResponseInfo) DeleteField(key string) error {
	if r.Body == nil {
		return nil
	}
	_, err := r.Body.Unset(key)
	return err
}

// HasField checks if a field exists in the response body
func (r *ResponseInfo) HasField(key string) bool {
	if r.Body == nil {
		return false
	}
	field := r.Body.Get(key)
	return field != nil && field.Exists()
}

// TransformArrayField applies a transformation to each item in an array field
// If key is empty, transforms the root body if it's an array
func (r *ResponseInfo) TransformArrayField(key string, transformer func(*ast.Node) error) error {
	if r.Body == nil {
		return nil
	}

	var array *ast.Node

	if key == "" {
		// Transform root body if it's an array
		if r.Body.TypeSafe() == ast.V_ARRAY {
			array = r.Body
		} else {
			// Not an array, nothing to do
			return nil
		}
	} else {
		// Transform named array field
		array = r.Body.Get(key)
		if array == nil || array.TypeSafe() != ast.V_ARRAY {
			return nil
		}
	}

	// Iterate over the array
	length, err := array.Len()
	if err != nil {
		return err
	}

	for i := 0; i < length; i++ {
		item := array.Index(i)
		if err := transformer(item); err != nil {
			return err
		}
	}

	return nil
}

// TransformNestedArrays recursively finds and transforms all arrays nested within an object
func (r *ResponseInfo) TransformNestedArrays(transformer func(*ast.Node) error) error {
	if r.Body == nil {
		return nil
	}

	// Only process if the body is an object
	if r.Body.TypeSafe() != ast.V_OBJECT {
		return nil
	}

	return transformNestedArraysRecursive(r.Body, transformer)
}

// transformNestedArraysRecursive is a helper that recursively transforms arrays within a node
func transformNestedArraysRecursive(node *ast.Node, transformer func(*ast.Node) error) error {
	if node == nil {
		return nil
	}

	nodeType := node.TypeSafe()

	switch nodeType {
	case ast.V_OBJECT:
		// Convert to map to get all keys
		objMap, err := node.Map()
		if err != nil {
			return err
		}

		// Iterate through all fields
		for key := range objMap {
			value := node.Get(key)
			if value == nil {
				continue
			}

			valueType := value.TypeSafe()

			if valueType == ast.V_ARRAY {
				// Found an array - transform each item
				length, err := value.Len()
				if err != nil {
					return err
				}

				for i := 0; i < length; i++ {
					item := value.Index(i)
					if item == nil {
						continue
					}

					// Apply transformer to the array item
					if err := transformer(item); err != nil {
						return err
					}

					// Recursively process nested structures within the array item
					if item.TypeSafe() == ast.V_OBJECT {
						if err := transformNestedArraysRecursive(item, transformer); err != nil {
							return err
						}
					}
				}
			} else if valueType == ast.V_OBJECT {
				// Recursively process nested objects
				if err := transformNestedArraysRecursive(value, transformer); err != nil {
					return err
				}
			}
		}

	case ast.V_ARRAY:
		// Handle arrays at the current level
		length, err := node.Len()
		if err != nil {
			return err
		}

		for i := 0; i < length; i++ {
			item := node.Index(i)
			if item == nil {
				continue
			}

			// Apply transformer to the array item
			if err := transformer(item); err != nil {
				return err
			}

			// Recursively process nested structures
			if err := transformNestedArraysRecursive(item, transformer); err != nil {
				return err
			}
		}
	}

	return nil
}
