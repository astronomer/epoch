package schema

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/isaacchung/cadwyn-go/pkg/migration"
)

// Schema represents the structure of a Go type
type Schema struct {
	Name    string
	Type    reflect.Type
	Fields  []Field
	Version string
	Tags    map[string]string
}

// Field represents a field in a struct
type Field struct {
	Name       string
	Type       reflect.Type
	JSONName   string
	Tag        reflect.StructTag
	Index      int
	IsExported bool
	IsEmbedded bool
	OmitEmpty  bool
}

// SchemaAnalyzer provides utilities for analyzing struct schemas
type SchemaAnalyzer struct {
	cache map[reflect.Type]*Schema
}

// NewSchemaAnalyzer creates a new schema analyzer
func NewSchemaAnalyzer() *SchemaAnalyzer {
	return &SchemaAnalyzer{
		cache: make(map[reflect.Type]*Schema),
	}
}

// AnalyzeStruct analyzes a struct and returns its schema
func (sa *SchemaAnalyzer) AnalyzeStruct(v interface{}) (*Schema, error) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", t.Kind())
	}

	// Check cache first
	if cached, exists := sa.cache[t]; exists {
		return cached, nil
	}

	schema := &Schema{
		Name:   t.Name(),
		Type:   t,
		Fields: make([]Field, 0, t.NumField()),
		Tags:   make(map[string]string),
	}

	// Analyze fields
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		field := sa.analyzeField(structField, i)
		schema.Fields = append(schema.Fields, field)
	}

	// Extract version information from struct tags
	if versionTag := sa.extractVersionTag(t); versionTag != "" {
		schema.Version = versionTag
	}

	// Cache the result
	sa.cache[t] = schema

	return schema, nil
}

// analyzeField analyzes a struct field
func (sa *SchemaAnalyzer) analyzeField(sf reflect.StructField, index int) Field {
	field := Field{
		Name:       sf.Name,
		Type:       sf.Type,
		Tag:        sf.Tag,
		Index:      index,
		IsExported: sf.IsExported(),
		IsEmbedded: sf.Anonymous,
	}

	// Parse JSON tag
	if jsonTag := sf.Tag.Get("json"); jsonTag != "" {
		field.JSONName, field.OmitEmpty = sa.parseJSONTag(jsonTag)
	} else {
		field.JSONName = sf.Name
	}

	return field
}

// parseJSONTag parses a JSON struct tag
func (sa *SchemaAnalyzer) parseJSONTag(tag string) (name string, omitEmpty bool) {
	if tag == "-" {
		return "", false
	}

	parts := strings.Split(tag, ",")
	name = parts[0]

	for i := 1; i < len(parts); i++ {
		if parts[i] == "omitempty" {
			omitEmpty = true
		}
	}

	return name, omitEmpty
}

// extractVersionTag extracts version information from struct tags
func (sa *SchemaAnalyzer) extractVersionTag(t reflect.Type) string {
	// Look for a version field or tag
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if versionTag := field.Tag.Get("cadwyn_version"); versionTag != "" {
			return versionTag
		}
	}
	return ""
}

// CompareSchemas compares two schemas and returns the differences
func (sa *SchemaAnalyzer) CompareSchemas(oldSchema, newSchema *Schema) (*SchemaDiff, error) {
	diff := &SchemaDiff{
		OldSchema: oldSchema,
		NewSchema: newSchema,
		Changes:   make([]FieldDiff, 0),
	}

	// Create field maps for easier comparison
	oldFields := make(map[string]Field)
	newFields := make(map[string]Field)

	for _, field := range oldSchema.Fields {
		oldFields[field.JSONName] = field
	}

	for _, field := range newSchema.Fields {
		newFields[field.JSONName] = field
	}

	// Find added fields
	for name, newField := range newFields {
		if _, exists := oldFields[name]; !exists {
			diff.Changes = append(diff.Changes, FieldDiff{
				Type:      FieldDiffTypeAdded,
				FieldName: name,
				NewField:  &newField,
			})
		}
	}

	// Find removed fields
	for name, oldField := range oldFields {
		if _, exists := newFields[name]; !exists {
			diff.Changes = append(diff.Changes, FieldDiff{
				Type:      FieldDiffTypeRemoved,
				FieldName: name,
				OldField:  &oldField,
			})
		}
	}

	// Find modified fields
	for name, newField := range newFields {
		if oldField, exists := oldFields[name]; exists {
			if !sa.fieldsEqual(oldField, newField) {
				diff.Changes = append(diff.Changes, FieldDiff{
					Type:      FieldDiffTypeModified,
					FieldName: name,
					OldField:  &oldField,
					NewField:  &newField,
				})
			}
		}
	}

	return diff, nil
}

// fieldsEqual checks if two fields are equal
func (sa *SchemaAnalyzer) fieldsEqual(f1, f2 Field) bool {
	return f1.Type == f2.Type &&
		f1.JSONName == f2.JSONName &&
		f1.OmitEmpty == f2.OmitEmpty
}

// SchemaDiff represents the difference between two schemas
type SchemaDiff struct {
	OldSchema *Schema
	NewSchema *Schema
	Changes   []FieldDiff
}

// FieldDiffType represents the type of field difference
type FieldDiffType int

const (
	FieldDiffTypeAdded FieldDiffType = iota
	FieldDiffTypeRemoved
	FieldDiffTypeModified
	FieldDiffTypeRenamed
)

// String returns the string representation of a field diff type
func (fdt FieldDiffType) String() string {
	switch fdt {
	case FieldDiffTypeAdded:
		return "added"
	case FieldDiffTypeRemoved:
		return "removed"
	case FieldDiffTypeModified:
		return "modified"
	case FieldDiffTypeRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FieldDiff represents a difference in a field
type FieldDiff struct {
	Type      FieldDiffType
	FieldName string
	OldField  *Field
	NewField  *Field
}

// ToMigrationChanges converts schema differences to migration field changes
func (sd *SchemaDiff) ToMigrationChanges(analyzer *SchemaAnalyzer) []migration.FieldChange {
	var changes []migration.FieldChange

	for _, diff := range sd.Changes {
		switch diff.Type {
		case FieldDiffTypeAdded:
			// Add field with zero value as default
			defaultValue := analyzer.getZeroValue(diff.NewField.Type)
			changes = append(changes, migration.AddField(diff.FieldName, defaultValue))

		case FieldDiffTypeRemoved:
			changes = append(changes, migration.RemoveField(diff.FieldName))

		case FieldDiffTypeModified:
			// For modified fields, we might need type conversion
			if diff.OldField.Type != diff.NewField.Type {
				transformFunc := analyzer.createTypeConverter(diff.OldField.Type, diff.NewField.Type)
				changes = append(changes, migration.TransformField(diff.FieldName, transformFunc))
			}

		case FieldDiffTypeRenamed:
			// This would require additional logic to detect renames
			// For now, treat as remove + add
			changes = append(changes, migration.RemoveField(diff.FieldName))
			if diff.NewField != nil {
				defaultValue := analyzer.getZeroValue(diff.NewField.Type)
				changes = append(changes, migration.AddField(diff.NewField.JSONName, defaultValue))
			}
		}
	}

	return changes
}

// getZeroValue returns the zero value for a given type
func (sa *SchemaAnalyzer) getZeroValue(t reflect.Type) interface{} {
	switch t.Kind() {
	case reflect.String:
		return ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uint(0)
	case reflect.Float32, reflect.Float64:
		return 0.0
	case reflect.Bool:
		return false
	case reflect.Slice:
		return []interface{}{}
	case reflect.Map:
		return map[string]interface{}{}
	case reflect.Ptr:
		return nil
	default:
		return nil
	}
}

// createTypeConverter creates a function to convert between types
func (sa *SchemaAnalyzer) createTypeConverter(from, to reflect.Type) func(interface{}) interface{} {
	return func(value interface{}) interface{} {
		// Simple type conversions
		if from.Kind() == reflect.String && to.Kind() == reflect.Int {
			if str, ok := value.(string); ok {
				// Simple string to int conversion (in practice, you'd want proper parsing)
				return len(str) // Just as an example
			}
		}

		if from.Kind() == reflect.Int && to.Kind() == reflect.String {
			if i, ok := value.(int); ok {
				return fmt.Sprintf("%d", i)
			}
		}

		// Add more type conversions as needed
		return value
	}
}

// StructTransformer provides utilities for transforming structs
type StructTransformer struct {
	analyzer *SchemaAnalyzer
}

// NewStructTransformer creates a new struct transformer
func NewStructTransformer() *StructTransformer {
	return &StructTransformer{
		analyzer: NewSchemaAnalyzer(),
	}
}

// TransformStruct transforms a struct based on schema differences
func (st *StructTransformer) TransformStruct(source interface{}, targetSchema *Schema) (interface{}, error) {
	sourceSchema, err := st.analyzer.AnalyzeStruct(source)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze source schema: %w", err)
	}

	diff, err := st.analyzer.CompareSchemas(sourceSchema, targetSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to compare schemas: %w", err)
	}

	changes := diff.ToMigrationChanges(st.analyzer)
	return migration.ApplyFieldChanges(source, changes)
}

// GenerateMigrationChanges generates migration changes based on schema differences
func (st *StructTransformer) GenerateMigrationChanges(oldStruct, newStruct interface{}) ([]migration.FieldChange, error) {
	oldSchema, err := st.analyzer.AnalyzeStruct(oldStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze old struct: %w", err)
	}

	newSchema, err := st.analyzer.AnalyzeStruct(newStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze new struct: %w", err)
	}

	diff, err := st.analyzer.CompareSchemas(oldSchema, newSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to compare schemas: %w", err)
	}

	return diff.ToMigrationChanges(st.analyzer), nil
}

// Global schema analyzer instance
var sa = NewSchemaAnalyzer()
