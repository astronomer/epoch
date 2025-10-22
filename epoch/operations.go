package epoch

import (
	"fmt"

	"github.com/bytedance/sonic/ast"
)

// Operation represents a declarative schema operation that can be applied bidirectionally
type Operation interface {
	// ApplyToRequest applies the forward transformation (v1 -> v2)
	ApplyToRequest(node *ast.Node) error

	// ApplyToResponse applies the backward transformation (v2 -> v1)
	ApplyToResponse(node *ast.Node) error

	// GetFieldMapping returns field name mappings for error transformation
	// Returns a map of new field name -> old field name
	GetFieldMapping() map[string]string
}

// FieldRenameOp renames a field
type FieldRenameOp struct {
	From string // Old field name (in v1)
	To   string // New field name (in v2)
}

// ApplyToRequest converts from old name to new name (v1 -> v2)
func (op *FieldRenameOp) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Check if old field exists
	if !node.Get(op.From).Exists() {
		return nil
	}

	// Get the value of the old field
	value := node.Get(op.From)
	if value == nil {
		return nil
	}

	// Set new field with the value
	if err := SetNodeField(node, op.To, value); err != nil {
		return fmt.Errorf("failed to set field %s: %w", op.To, err)
	}

	// Delete old field
	return DeleteNodeField(node, op.From)
}

// ApplyToResponse converts from new name to old name (v2 -> v1)
func (op *FieldRenameOp) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Check if new field exists
	if !node.Get(op.To).Exists() {
		return nil
	}

	// Get the value of the new field
	value := node.Get(op.To)
	if value == nil {
		return nil
	}

	// Set old field with the value
	if err := SetNodeField(node, op.From, value); err != nil {
		return fmt.Errorf("failed to set field %s: %w", op.From, err)
	}

	// Delete new field
	return DeleteNodeField(node, op.To)
}

// GetFieldMapping returns the field mapping for error transformation
func (op *FieldRenameOp) GetFieldMapping() map[string]string {
	// In v2->v1 migration, we need to transform "To" -> "From" in error messages
	return map[string]string{op.To: op.From}
}

// FieldAddOp adds a field with a default value
type FieldAddOp struct {
	Name    string
	Default interface{}
}

// ApplyToRequest adds field if missing (v1 -> v2)
func (op *FieldAddOp) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	// Only add if field doesn't exist
	if node.Get(op.Name).Exists() {
		return nil
	}

	return SetNodeField(node, op.Name, op.Default)
}

// ApplyToResponse removes the field (v2 -> v1)
func (op *FieldAddOp) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	return DeleteNodeField(node, op.Name)
}

// GetFieldMapping returns empty map (no field rename)
func (op *FieldAddOp) GetFieldMapping() map[string]string {
	return nil
}

// FieldRemoveOp removes a field
type FieldRemoveOp struct {
	Name string
}

// ApplyToRequest removes the field (v1 -> v2)
func (op *FieldRemoveOp) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	return DeleteNodeField(node, op.Name)
}

// ApplyToResponse adds the field back (v2 -> v1, but we don't have the value)
func (op *FieldRemoveOp) ApplyToResponse(node *ast.Node) error {
	// When migrating back, we can't restore a removed field without knowing its value
	// This is a limitation - users should use custom transformers if they need this
	return nil
}

// GetFieldMapping returns empty map (no field rename)
func (op *FieldRemoveOp) GetFieldMapping() map[string]string {
	return nil
}

// EnumValueMapOp maps enum values
type EnumValueMapOp struct {
	Field   string
	Mapping map[string]string // old value -> new value (for forward migration)
}

// ApplyToRequest maps enum values forward (v1 -> v2)
func (op *EnumValueMapOp) ApplyToRequest(node *ast.Node) error {
	if node == nil {
		return nil
	}

	fieldNode := node.Get(op.Field)
	if !fieldNode.Exists() {
		return nil
	}

	// Get the current value
	currentValue, err := fieldNode.String()
	if err != nil {
		return nil // Not a string, skip
	}

	// Map to new value if mapping exists
	if newValue, exists := op.Mapping[currentValue]; exists {
		return SetNodeField(node, op.Field, newValue)
	}

	return nil
}

// ApplyToResponse maps enum values backward (v2 -> v1)
func (op *EnumValueMapOp) ApplyToResponse(node *ast.Node) error {
	if node == nil {
		return nil
	}

	fieldNode := node.Get(op.Field)
	if !fieldNode.Exists() {
		return nil
	}

	// Get the current value
	currentValue, err := fieldNode.String()
	if err != nil {
		return nil // Not a string, skip
	}

	// Create reverse mapping
	reverseMapping := make(map[string]string)
	for old, new := range op.Mapping {
		reverseMapping[new] = old
	}

	// Map to old value if mapping exists
	if oldValue, exists := reverseMapping[currentValue]; exists {
		return SetNodeField(node, op.Field, oldValue)
	}

	return nil
}

// GetFieldMapping returns empty map (enum values, not field names)
func (op *EnumValueMapOp) GetFieldMapping() map[string]string {
	return nil
}

// OperationList is a collection of operations
type OperationList []Operation

// ApplyToRequest applies all operations to a request node
func (ops OperationList) ApplyToRequest(node *ast.Node) error {
	for _, op := range ops {
		if err := op.ApplyToRequest(node); err != nil {
			return err
		}
	}
	return nil
}

// ApplyToResponse applies all operations to a response node
func (ops OperationList) ApplyToResponse(node *ast.Node) error {
	for _, op := range ops {
		if err := op.ApplyToResponse(node); err != nil {
			return err
		}
	}
	return nil
}

// GetFieldMappings returns combined field mappings from all operations
func (ops OperationList) GetFieldMappings() map[string]string {
	result := make(map[string]string)
	for _, op := range ops {
		for k, v := range op.GetFieldMapping() {
			result[k] = v
		}
	}
	return result
}
