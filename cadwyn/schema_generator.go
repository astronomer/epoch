package cadwyn

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// SchemaGenerator generates version-specific Go structs
// This is inspired by Python Cadwyn's schema_generation.py
type SchemaGenerator struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
	generators     map[string]*VersionSpecificGenerator
}

// VersionSpecificGenerator handles generation for a specific version
type VersionSpecificGenerator struct {
	version        *Version
	structWrappers map[reflect.Type]*StructWrapper
}

// StructWrapper wraps a Go struct with version-specific modifications
type StructWrapper struct {
	originalType reflect.Type
	name         string
	fields       map[string]*FieldWrapper
	version      string
	packagePath  string
}

// FieldWrapper represents a struct field with version-specific properties
type FieldWrapper struct {
	name         string
	goType       string
	jsonTag      string
	isRequired   bool
	defaultValue interface{}
	isDeleted    bool
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator(versionBundle *VersionBundle, migrationChain *MigrationChain) *SchemaGenerator {
	sg := &SchemaGenerator{
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
		generators:     make(map[string]*VersionSpecificGenerator),
	}

	sg.buildVersionSpecificGenerators()
	return sg
}

// buildVersionSpecificGenerators creates generators for each version
func (sg *SchemaGenerator) buildVersionSpecificGenerators() {
	// Start with head version
	headGen := &VersionSpecificGenerator{
		version:        sg.versionBundle.GetHeadVersion(),
		structWrappers: make(map[reflect.Type]*StructWrapper),
	}
	sg.generators["head"] = headGen

	// Create generators for each version by applying changes in reverse
	for _, v := range sg.versionBundle.GetVersions() {
		versionGen := &VersionSpecificGenerator{
			version:        v,
			structWrappers: make(map[reflect.Type]*StructWrapper),
		}
		sg.generators[v.String()] = versionGen
	}
}

// GenerateStruct generates Go code for a struct at a specific version
func (sg *SchemaGenerator) GenerateStruct(structType reflect.Type, targetVersion string) (string, error) {
	generator, exists := sg.generators[targetVersion]
	if !exists {
		return "", fmt.Errorf("version %s not found", targetVersion)
	}

	wrapper, err := sg.getOrCreateStructWrapper(structType, generator)
	if err != nil {
		return "", fmt.Errorf("failed to create struct wrapper: %w", err)
	}

	return sg.generateGoCode(wrapper)
}

// GeneratePackage generates Go code for all structs in a package at a specific version
func (sg *SchemaGenerator) GeneratePackage(packagePath string, targetVersion string) (map[string]string, error) {
	_, exists := sg.generators[targetVersion]
	if !exists {
		return nil, fmt.Errorf("version %s not found", targetVersion)
	}

	result := make(map[string]string)

	// Find all structs in the package (this would need reflection or AST parsing)
	// For now, return a placeholder
	result["generated.go"] = fmt.Sprintf(`// Generated for version %s
package %s

// Package-level generation would go here
// This requires more sophisticated reflection or AST parsing
`, targetVersion, extractPackageName(packagePath))

	return result, nil
}

// getOrCreateStructWrapper gets or creates a struct wrapper for a specific version
func (sg *SchemaGenerator) getOrCreateStructWrapper(structType reflect.Type, generator *VersionSpecificGenerator) (*StructWrapper, error) {
	if wrapper, exists := generator.structWrappers[structType]; exists {
		return wrapper, nil
	}

	// Create base wrapper from the struct
	wrapper := &StructWrapper{
		originalType: structType,
		name:         structType.Name(),
		fields:       make(map[string]*FieldWrapper),
		version:      generator.version.String(),
		packagePath:  structType.PkgPath(),
	}

	// Extract fields from the struct
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldWrapper := &FieldWrapper{
			name:       field.Name,
			goType:     field.Type.String(),
			jsonTag:    field.Tag.Get("json"),
			isRequired: !strings.Contains(field.Tag.Get("json"), "omitempty"),
		}

		wrapper.fields[field.Name] = fieldWrapper
	}

	// Apply version-specific changes
	if err := sg.applyVersionChanges(wrapper, generator.version); err != nil {
		return nil, fmt.Errorf("failed to apply version changes: %w", err)
	}

	generator.structWrappers[structType] = wrapper
	return wrapper, nil
}

// applyVersionChanges applies version changes to transform the struct
func (sg *SchemaGenerator) applyVersionChanges(wrapper *StructWrapper, targetVersion *Version) error {
	// Find all changes that need to be applied to get from head to target version
	changes := sg.migrationChain.GetChanges()

	// Apply changes in reverse order (from head back to target version)
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]

		// Only apply changes that are newer than our target version
		if change.FromVersion().Equal(targetVersion) {
			break
		}

		// Apply schema instructions in reverse
		for _, instruction := range change.GetSchemaInstructions() {
			if err := sg.applySchemaInstruction(wrapper, instruction, true); err != nil {
				return fmt.Errorf("failed to apply schema instruction: %w", err)
			}
		}
	}

	return nil
}

// applySchemaInstruction applies a single schema instruction
func (sg *SchemaGenerator) applySchemaInstruction(wrapper *StructWrapper, instruction *SchemaInstruction, reverse bool) error {
	switch instruction.Type {
	case "field_added":
		if reverse {
			// Remove the field
			delete(wrapper.fields, instruction.Name)
		} else {
			// Add the field (forward direction)
			wrapper.fields[instruction.Name] = &FieldWrapper{
				name:   instruction.Name,
				goType: "string", // Default type, should come from instruction
			}
		}
	case "field_removed":
		if reverse {
			// Add the field back
			wrapper.fields[instruction.Name] = &FieldWrapper{
				name:   instruction.Name,
				goType: "string", // Should come from instruction
			}
		} else {
			// Remove the field
			delete(wrapper.fields, instruction.Name)
		}
	case "field_renamed":
		oldName := instruction.Attributes["old_name"].(string)
		newName := instruction.Name
		if reverse {
			// Rename back
			if field, exists := wrapper.fields[newName]; exists {
				field.name = oldName
				wrapper.fields[oldName] = field
				delete(wrapper.fields, newName)
			}
		} else {
			// Rename forward
			if field, exists := wrapper.fields[oldName]; exists {
				field.name = newName
				wrapper.fields[newName] = field
				delete(wrapper.fields, oldName)
			}
		}
	}

	return nil
}

// generateGoCode generates the actual Go code for a struct wrapper
func (sg *SchemaGenerator) generateGoCode(wrapper *StructWrapper) (string, error) {
	var builder strings.Builder

	// Package declaration
	packageName := extractPackageName(wrapper.packagePath)
	builder.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	// Struct definition
	builder.WriteString(fmt.Sprintf("// %s represents the %s version of the struct\n", wrapper.name, wrapper.version))
	builder.WriteString(fmt.Sprintf("type %s struct {\n", wrapper.name))

	// Fields
	for _, field := range wrapper.fields {
		if field.isDeleted {
			continue
		}

		jsonTag := field.jsonTag
		if jsonTag == "" {
			jsonTag = strings.ToLower(field.name)
		}

		builder.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n",
			field.name, field.goType, jsonTag))
	}

	builder.WriteString("}\n")

	return sg.formatGoCode(builder.String())
}

// formatGoCode formats the generated Go code
func (sg *SchemaGenerator) formatGoCode(code string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", code, parser.ParseComments)
	if err != nil {
		// If parsing fails, return unformatted code
		return code, nil
	}

	var buf strings.Builder
	if err := format.Node(&buf, fset, node); err != nil {
		return code, nil
	}

	return buf.String(), nil
}

// Helper function to extract package name from path
func extractPackageName(packagePath string) string {
	parts := strings.Split(packagePath, "/")
	if len(parts) == 0 {
		return "main"
	}
	return parts[len(parts)-1]
}

// GetVersionSpecificType returns the version-specific type for a given struct
func (sg *SchemaGenerator) GetVersionSpecificType(structType reflect.Type, targetVersion string) (reflect.Type, error) {
	generator, exists := sg.generators[targetVersion]
	if !exists {
		return nil, fmt.Errorf("version %s not found", targetVersion)
	}

	wrapper, err := sg.getOrCreateStructWrapper(structType, generator)
	if err != nil {
		return nil, err
	}

	// For now, return the original type
	// In a full implementation, we'd create new types dynamically
	_ = wrapper
	return structType, nil
}

// ListVersionedStructs returns all structs that have version-specific changes
func (sg *SchemaGenerator) ListVersionedStructs() map[string][]reflect.Type {
	result := make(map[string][]reflect.Type)

	for versionStr, generator := range sg.generators {
		var types []reflect.Type
		for structType := range generator.structWrappers {
			types = append(types, structType)
		}
		result[versionStr] = types
	}

	return result
}
