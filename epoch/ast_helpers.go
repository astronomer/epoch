package epoch

import (
	"errors"

	"github.com/bytedance/sonic/ast"
)

// Helper functions for working with individual AST nodes in transformers
// These are useful inside TransformArrayField callbacks and for direct node manipulation

// SetNodeField sets a field on an AST node
func SetNodeField(node *ast.Node, key string, value interface{}) error {
	if node == nil {
		return errors.New("node is nil")
	}
	_, err := node.SetAny(key, value)
	return err
}

// DeleteNodeField deletes a field from an AST node
func DeleteNodeField(node *ast.Node, key string) error {
	if node == nil {
		return nil
	}
	_, err := node.Unset(key)
	return err
}

// GetNodeField gets a field from an AST node
func GetNodeField(node *ast.Node, key string) *ast.Node {
	if node == nil {
		return nil
	}
	return node.Get(key)
}

// GetNodeFieldString gets a field value as a string from an AST node
func GetNodeFieldString(node *ast.Node, key string) (string, error) {
	field := GetNodeField(node, key)
	if field == nil || !field.Exists() {
		return "", errors.New("field not found")
	}
	return field.String()
}

// GetNodeFieldInt gets a field value as an int64 from an AST node
func GetNodeFieldInt(node *ast.Node, key string) (int64, error) {
	field := GetNodeField(node, key)
	if field == nil || !field.Exists() {
		return 0, errors.New("field not found")
	}
	return field.Int64()
}

// GetNodeFieldFloat gets a field value as a float64 from an AST node
func GetNodeFieldFloat(node *ast.Node, key string) (float64, error) {
	field := GetNodeField(node, key)
	if field == nil || !field.Exists() {
		return 0, errors.New("field not found")
	}
	return field.Float64()
}

// HasNodeField checks if a field exists on an AST node
func HasNodeField(node *ast.Node, key string) bool {
	if node == nil {
		return false
	}
	field := node.Get(key)
	return field != nil && field.Exists()
}

// RenameNodeField renames a field on an AST node (moves old field to new key and deletes old)
func RenameNodeField(node *ast.Node, oldKey, newKey string) error {
	if node == nil {
		return errors.New("node is nil")
	}

	// If renaming to same key, do nothing
	if oldKey == newKey {
		return nil
	}

	// Get the old field
	oldField := node.Get(oldKey)
	if oldField == nil || !oldField.Exists() {
		return nil // Field doesn't exist, nothing to rename
	}

	// Get the value and set it on the new key
	value, err := oldField.Interface()
	if err != nil {
		return err
	}

	if err := SetNodeField(node, newKey, value); err != nil {
		return err
	}

	// Delete the old field
	return DeleteNodeField(node, oldKey)
}

// CopyNodeField copies a field from one AST node to another
func CopyNodeField(fromNode *ast.Node, toNode *ast.Node, key string) error {
	if fromNode == nil || toNode == nil {
		return errors.New("source or destination node is nil")
	}

	field := fromNode.Get(key)
	if field == nil || !field.Exists() {
		return nil // Field doesn't exist, nothing to copy
	}

	value, err := field.Interface()
	if err != nil {
		return err
	}

	return SetNodeField(toNode, key, value)
}

// GetNodeType returns the type of an AST node safely
func GetNodeType(node *ast.Node) int {
	if node == nil {
		return ast.V_NULL
	}
	return node.TypeSafe()
}

// IsNodeArray checks if an AST node is an array
func IsNodeArray(node *ast.Node) bool {
	return GetNodeType(node) == ast.V_ARRAY
}

// IsNodeObject checks if an AST node is an object
func IsNodeObject(node *ast.Node) bool {
	return GetNodeType(node) == ast.V_OBJECT
}

// GetNodeArrayLength returns the length of an array node
func GetNodeArrayLength(node *ast.Node) (int, error) {
	if !IsNodeArray(node) {
		return 0, errors.New("node is not an array")
	}
	return node.Len()
}

// GetNodeArrayItem returns an item from an array node at the specified index
func GetNodeArrayItem(node *ast.Node, index int) (*ast.Node, error) {
	if !IsNodeArray(node) {
		return nil, errors.New("node is not an array")
	}

	length, err := node.Len()
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= length {
		return nil, errors.New("array index out of bounds")
	}

	return node.Index(index), nil
}
