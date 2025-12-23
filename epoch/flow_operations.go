package epoch

import (
	"fmt"

	"github.com/bytedance/sonic/ast"
)

// Flow-based operation interfaces matching actual migration flow:
// - Requests ALWAYS migrate Client Version → HEAD Version (ToNextVersion)
// - Responses ALWAYS migrate HEAD Version → Client Version (ToPreviousVersion)

// RequestToNextVersionOperation applies when migrating requests from client version to HEAD
// This is the ONLY direction requests flow
type RequestToNextVersionOperation interface {
	ApplyToRequest(node *ast.Node) error
	GetFieldMapping() map[string]string     // For error transformation
	Inverse() RequestToNextVersionOperation // For OpenAPI schema generation (HEAD→Client)
}

// ResponseToPreviousVersionOperation applies when migrating responses from HEAD to client version
// This is the ONLY direction responses flow
type ResponseToPreviousVersionOperation interface {
	ApplyToResponse(node *ast.Node) error
	GetFieldMapping() map[string]string // For error transformation
}

// ============================================================================
// Request Operations - TO NEXT VERSION (Client→HEAD) - ONLY DIRECTION
// ============================================================================

// RequestAddField adds a field when request migrates from client to HEAD
// Use case: HEAD version requires a field that older clients don't send
type RequestAddField struct {
	Name    string
	Default interface{}
}

func (op *RequestAddField) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Only add if field doesn't exist
	if node.Get(op.Name).Exists() {
		return nil
	}

	return SetNodeField(node, op.Name, op.Default)
}

func (op *RequestAddField) GetFieldMapping() map[string]string {
	return nil // No field rename
}

// Inverse returns the opposite operation for schema generation
// AddField (Client→HEAD) becomes RemoveField (HEAD→Client)
func (op *RequestAddField) Inverse() RequestToNextVersionOperation {
	return &RequestRemoveField{
		Name: op.Name,
	}
}

// RequestAddFieldWithDefault adds a field ONLY if missing (Cadwyn-style default handling)
// Use case: Making a field required - add default for old clients that don't send it
type RequestAddFieldWithDefault struct {
	Name    string
	Default interface{}
}

func (op *RequestAddFieldWithDefault) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Only add if field doesn't exist (explicit check for missing field)
	if node.Get(op.Name).Exists() {
		return nil
	}

	return SetNodeField(node, op.Name, op.Default)
}

func (op *RequestAddFieldWithDefault) GetFieldMapping() map[string]string {
	return nil
}

// Inverse returns the opposite operation for schema generation
// AddFieldWithDefault (Client→HEAD) becomes RemoveField (HEAD→Client)
func (op *RequestAddFieldWithDefault) Inverse() RequestToNextVersionOperation {
	return &RequestRemoveField{
		Name: op.Name,
	}
}

// RequestRemoveField removes a field when request migrates from client to HEAD
// Use case: HEAD version removed a deprecated field
type RequestRemoveField struct {
	Name string
}

func (op *RequestRemoveField) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	return DeleteNodeField(node, op.Name)
}

func (op *RequestRemoveField) GetFieldMapping() map[string]string {
	return nil // No field rename
}

// Inverse returns the opposite operation for schema generation
// RemoveField (Client→HEAD) becomes AddField with nil default (HEAD→Client)
// The nil default is safe for schema generation (defaults are a runtime concern)
func (op *RequestRemoveField) Inverse() RequestToNextVersionOperation {
	return &RequestAddField{
		Name:    op.Name,
		Default: nil, // Safe for OpenAPI schemas
	}
}

// RequestRenameField renames a field when request migrates from client to HEAD
// Use case: HEAD version renamed "name" to "full_name"
type RequestRenameField struct {
	OlderVersionName string // Field name in older/client version
	NewerVersionName string // Field name in newer/HEAD version
}

func (op *RequestRenameField) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Check if old field exists
	if !node.Get(op.OlderVersionName).Exists() {
		return nil
	}

	// Get the value of the old field
	value := node.Get(op.OlderVersionName)
	if value == nil {
		return nil
	}

	// Set new field with the value
	if err := SetNodeField(node, op.NewerVersionName, value); err != nil {
		return fmt.Errorf("failed to set field %s: %w", op.NewerVersionName, err)
	}

	// Delete old field
	return DeleteNodeField(node, op.OlderVersionName)
}

func (op *RequestRenameField) GetFieldMapping() map[string]string {
	// When transforming error messages, map new field name back to old
	return map[string]string{op.NewerVersionName: op.OlderVersionName}
}

// Inverse returns the opposite operation for schema generation
// RenameField (Client→HEAD: older→newer) becomes RenameField (HEAD→Client: newer→older)
// This is a perfect inversion - completely reversible
func (op *RequestRenameField) Inverse() RequestToNextVersionOperation {
	return &RequestRenameField{
		OlderVersionName: op.NewerVersionName, // Swap directions
		NewerVersionName: op.OlderVersionName,
	}
}

// RequestCustom applies a custom transformation function
type RequestCustom struct {
	Fn func(*ast.Node) error
}

func (op *RequestCustom) ApplyToRequest(node *ast.Node) error {
	if node == nil || op.Fn == nil {
		return nil
	}
	return op.Fn(node)
}

func (op *RequestCustom) GetFieldMapping() map[string]string {
	return nil
}

// Inverse returns nil because custom operations cannot be automatically inverted
// Custom operations will be skipped during schema generation
func (op *RequestCustom) Inverse() RequestToNextVersionOperation {
	return nil // Not invertible
}

// ============================================================================
// Response Operations - TO PREVIOUS VERSION (HEAD→Client) - ONLY DIRECTION
// ============================================================================

// ResponseAddField adds a field when response migrates from HEAD to client
// Use case: Client expects a field that HEAD removed
type ResponseAddField struct {
	Name    string
	Default interface{}
}

func (op *ResponseAddField) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Only add if field doesn't exist
	if node.Get(op.Name).Exists() {
		return nil
	}

	return SetNodeField(node, op.Name, op.Default)
}

func (op *ResponseAddField) GetFieldMapping() map[string]string {
	return nil // No field rename
}

// ResponseRemoveField removes a field when response migrates from HEAD to client
// Use case: HEAD added a new field that old clients shouldn't see
type ResponseRemoveField struct {
	Name string
}

func (op *ResponseRemoveField) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	return DeleteNodeField(node, op.Name)
}

func (op *ResponseRemoveField) GetFieldMapping() map[string]string {
	return nil // No field rename
}

// ResponseRemoveFieldIfDefault removes a field ONLY if it equals the default value
// Use case: Making a field optional - remove if it has the default value
type ResponseRemoveFieldIfDefault struct {
	Name    string
	Default interface{}
}

func (op *ResponseRemoveFieldIfDefault) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Check if field exists
	fieldNode := node.Get(op.Name)
	if !fieldNode.Exists() {
		return nil
	}

	// Get the field value
	fieldValue, err := fieldNode.Interface()
	if err != nil {
		return nil // Can't compare, don't remove
	}

	// Compare with default - only remove if they match
	if fieldValue == op.Default {
		return DeleteNodeField(node, op.Name)
	}

	return nil
}

func (op *ResponseRemoveFieldIfDefault) GetFieldMapping() map[string]string {
	return nil
}

// ResponseRenameField renames a field when response migrates from HEAD to client
// Use case: HEAD renamed "name" to "full_name", rename back for old clients
type ResponseRenameField struct {
	NewerVersionName string // Field name in newer/HEAD version
	OlderVersionName string // Field name in older/client version
}

func (op *ResponseRenameField) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Check if new field exists
	if !node.Get(op.NewerVersionName).Exists() {
		return nil
	}

	// Get the value of the new field
	value := node.Get(op.NewerVersionName)
	if value == nil {
		return nil
	}

	// Set old field with the value
	if err := SetNodeField(node, op.OlderVersionName, value); err != nil {
		return fmt.Errorf("failed to set field %s: %w", op.OlderVersionName, err)
	}

	// Delete new field
	return DeleteNodeField(node, op.NewerVersionName)
}

func (op *ResponseRenameField) GetFieldMapping() map[string]string {
	// When transforming error messages, map new field name to old
	return map[string]string{op.NewerVersionName: op.OlderVersionName}
}

// ResponseCustom applies a custom transformation function
type ResponseCustom struct {
	Fn func(*ast.Node) error
}

func (op *ResponseCustom) ApplyToResponse(node *ast.Node) error {
	if node == nil || op.Fn == nil {
		return nil
	}
	return op.Fn(node)
}

func (op *ResponseCustom) GetFieldMapping() map[string]string {
	return nil
}

// ============================================================================
// Operation Lists for managing collections of operations
// ============================================================================

// RequestToNextVersionOperationList manages operations for request migration (Client→HEAD)
type RequestToNextVersionOperationList []RequestToNextVersionOperation

// Apply applies all operations to a request node
func (ops RequestToNextVersionOperationList) Apply(node *ast.Node) error {
	for _, op := range ops {
		if err := op.ApplyToRequest(node); err != nil {
			return err
		}
	}
	return nil
}

// GetFieldMappings returns combined field mappings from all operations
func (ops RequestToNextVersionOperationList) GetFieldMappings() map[string]string {
	result := make(map[string]string)
	for _, op := range ops {
		for k, v := range op.GetFieldMapping() {
			result[k] = v
		}
	}
	return result
}

// ResponseToPreviousVersionOperationList manages operations for response migration (HEAD→Client)
type ResponseToPreviousVersionOperationList []ResponseToPreviousVersionOperation

// Apply applies all operations to a response node
func (ops ResponseToPreviousVersionOperationList) Apply(node *ast.Node) error {
	for _, op := range ops {
		if err := op.ApplyToResponse(node); err != nil {
			return err
		}
	}
	return nil
}

// GetFieldMappings returns combined field mappings from all operations
func (ops ResponseToPreviousVersionOperationList) GetFieldMappings() map[string]string {
	result := make(map[string]string)
	for _, op := range ops {
		for k, v := range op.GetFieldMapping() {
			result[k] = v
		}
	}
	return result
}
