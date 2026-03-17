package epoch

import (
	"github.com/gin-gonic/gin"
)

// CapturedFieldsKey is the context key for storing captured request field values
// These are used by response transformers to restore original values when
// a field was removed from the request and needs to be added back to the response
const CapturedFieldsKey = "epoch_captured_fields"

// GetCapturedFields retrieves the captured fields map from a Gin context
func GetCapturedFields(c *gin.Context) map[string]interface{} {
	if c == nil {
		return nil
	}
	if val, exists := c.Get(CapturedFieldsKey); exists {
		if fields, ok := val.(map[string]interface{}); ok {
			return fields
		}
	}
	return nil
}

// SetCapturedField stores a captured field value in the Gin context
// The field is keyed by field name only. Since the Gin context is request-scoped,
// there's no risk of collision between different requests. This allows captured
// values to flow from request types to response types with the same field name.
func SetCapturedField(c *gin.Context, fieldName string, value interface{}) {
	if c == nil {
		return
	}
	fields := GetCapturedFields(c)
	if fields == nil {
		fields = make(map[string]interface{})
		c.Set(CapturedFieldsKey, fields)
	}
	fields[fieldName] = value
}

// GetCapturedField retrieves a specific captured field value
func GetCapturedField(c *gin.Context, fieldName string) (interface{}, bool) {
	fields := GetCapturedFields(c)
	if fields == nil {
		return nil, false
	}
	val, exists := fields[fieldName]
	return val, exists
}

// HasCapturedField checks if a field has been captured
func HasCapturedField(c *gin.Context, fieldName string) bool {
	_, exists := GetCapturedField(c, fieldName)
	return exists
}
