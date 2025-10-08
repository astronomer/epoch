package epoch

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// SchemaGenerator is currently WIP and not used in the project.
// SchemaGenerator generates version-specific Go structs with advanced AST-like capabilities
type SchemaGenerator struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
	generators     map[string]*VersionSpecificGenerator
	typeRegistry   *TypeRegistry
	astCache       *ASTCache
	mu             sync.RWMutex
}

// TypeRegistry manages type definitions and their relationships
type TypeRegistry struct {
	types          map[string]*TypeInfo
	packages       map[string]*PackageInfo
	versionedTypes map[string]map[string]*TypeInfo // version -> type name -> type info
	mu             sync.RWMutex
}

// TypeInfo contains detailed information about a type
type TypeInfo struct {
	Name          string
	Package       string
	Kind          reflect.Kind
	Fields        map[string]*FieldInfo
	Methods       map[string]*MethodInfo
	Tags          map[string]string
	Constraints   []string
	Documentation string
	SourceFile    string
	IsVersioned   bool
}

// FieldInfo contains detailed information about a struct field
type FieldInfo struct {
	Name          string
	Type          string
	JsonTag       string
	ValidateTags  []string
	IsRequired    bool
	IsPointer     bool
	DefaultValue  interface{}
	IsDeleted     bool
	Documentation string
	Constraints   []string
}

// MethodInfo contains information about struct methods
type MethodInfo struct {
	Name          string
	Parameters    []ParameterInfo
	ReturnTypes   []string
	IsExported    bool
	Documentation string
}

// ParameterInfo contains information about method parameters
type ParameterInfo struct {
	Name string
	Type string
}

// PackageInfo contains information about a package
type PackageInfo struct {
	Name    string
	Path    string
	Types   map[string]*TypeInfo
	Imports []string
}

// ASTCache caches parsed AST nodes for performance
type ASTCache struct {
	files map[string]*ast.File
	mu    sync.RWMutex
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
	defaultValue interface{} // Default value for field (used during schema generation)
	isDeleted    bool
}

// NewSchemaGenerator creates a new advanced schema generator
func NewSchemaGenerator(versionBundle *VersionBundle, migrationChain *MigrationChain) *SchemaGenerator {
	sg := &SchemaGenerator{
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
		generators:     make(map[string]*VersionSpecificGenerator),
		typeRegistry:   NewTypeRegistry(),
		astCache:       NewASTCache(),
	}

	sg.buildVersionSpecificGenerators()

	return sg
}

// NewTypeRegistry creates a new type registry
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types:          make(map[string]*TypeInfo),
		packages:       make(map[string]*PackageInfo),
		versionedTypes: make(map[string]map[string]*TypeInfo),
	}
}

// NewASTCache creates a new AST cache
func NewASTCache() *ASTCache {
	return &ASTCache{
		files: make(map[string]*ast.File),
	}
}

// Get retrieves a cached AST file (thread-safe)
func (ac *ASTCache) Get(filename string) (*ast.File, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	file, exists := ac.files[filename]
	return file, exists
}

// Set stores an AST file in the cache (thread-safe)
func (ac *ASTCache) Set(filename string, file *ast.File) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.files[filename] = file
}

// Clear removes all cached files (thread-safe)
func (ac *ASTCache) Clear() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.files = make(map[string]*ast.File)
}

// buildVersionSpecificGenerators creates generators for each version
func (sg *SchemaGenerator) buildVersionSpecificGenerators() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

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
	sg.mu.RLock()
	generator, exists := sg.generators[targetVersion]
	sg.mu.RUnlock()

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
	sg.mu.RLock()
	generator, exists := sg.generators[targetVersion]
	sg.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("version %s not found", targetVersion)
	}

	// Register the package in our type registry
	if err := sg.typeRegistry.RegisterPackage(packagePath); err != nil {
		return nil, fmt.Errorf("failed to register package %s: %w", packagePath, err)
	}

	result := make(map[string]string)
	packageName := extractPackageName(packagePath)

	// Get package info from registry
	sg.typeRegistry.mu.RLock()
	packageInfo, exists := sg.typeRegistry.packages[packagePath]
	sg.typeRegistry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("package %s not found in registry", packagePath)
	}

	// Generate version-specific types for each struct in the package
	var generatedTypes []string
	var imports []string

	// Standard imports for generated code
	imports = append(imports, "encoding/json", "fmt", "time")

	for typeName, typeInfo := range packageInfo.Types {
		if typeInfo.Kind == reflect.Struct && typeInfo.IsVersioned {
			// Generate the struct for this version
			structCode, err := sg.generateVersionedStruct(typeInfo, generator)
			if err != nil {
				return nil, fmt.Errorf("failed to generate struct %s: %w", typeName, err)
			}
			generatedTypes = append(generatedTypes, structCode)
		}
	}

	// Sort types for consistent output
	sort.Strings(generatedTypes)

	// Build the complete file
	var builder strings.Builder

	// Package declaration
	builder.WriteString(fmt.Sprintf("// Code generated for version %s. DO NOT EDIT.\n", targetVersion))
	builder.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	// Imports
	if len(imports) > 0 {
		builder.WriteString("import (\n")
		for _, imp := range imports {
			builder.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}
		builder.WriteString(")\n\n")
	}

	// Generated types
	for _, typeCode := range generatedTypes {
		builder.WriteString(typeCode)
		builder.WriteString("\n\n")
	}

	// Add version-specific utility functions
	builder.WriteString(sg.generateUtilityFunctions(targetVersion))

	formatted, err := sg.formatGoCode(builder.String())
	if err != nil {
		// If formatting fails, return unformatted code
		formatted = builder.String()
	}

	result[fmt.Sprintf("%s_v%s.go", packageName, strings.ReplaceAll(targetVersion, ".", "_"))] = formatted
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

// GetVersionSpecificType returns the version-specific type for a given struct
func (sg *SchemaGenerator) GetVersionSpecificType(structType reflect.Type, targetVersion string) (reflect.Type, error) {
	sg.mu.RLock()
	generator, exists := sg.generators[targetVersion]
	sg.mu.RUnlock()

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
	sg.mu.RLock()
	defer sg.mu.RUnlock()

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

// extractPackageName extracts the package name from a package path
func extractPackageName(packagePath string) string {
	if packagePath == "" {
		return "main"
	}

	// Extract the last part of the path as the package name
	parts := strings.Split(packagePath, "/")
	return parts[len(parts)-1]
}

// RegisterPackage registers a package in the type registry
func (tr *TypeRegistry) RegisterPackage(packagePath string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if _, exists := tr.packages[packagePath]; exists {
		return nil // Already registered
	}

	packageInfo := &PackageInfo{
		Name:  extractPackageName(packagePath),
		Path:  packagePath,
		Types: make(map[string]*TypeInfo),
	}

	tr.packages[packagePath] = packageInfo
	return nil
}

// generateVersionedStruct generates Go code for a versioned struct
func (sg *SchemaGenerator) generateVersionedStruct(typeInfo *TypeInfo, generator *VersionSpecificGenerator) (string, error) {
	var builder strings.Builder

	// Documentation comment
	if typeInfo.Documentation != "" {
		builder.WriteString(fmt.Sprintf("// %s\n", typeInfo.Documentation))
	}
	builder.WriteString(fmt.Sprintf("// Generated for version %s\n", generator.version.String()))

	// Struct definition
	builder.WriteString(fmt.Sprintf("type %s struct {\n", typeInfo.Name))

	// Sort fields for consistent output
	fieldNames := make([]string, 0, len(typeInfo.Fields))
	for name := range typeInfo.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	// Generate fields
	for _, fieldName := range fieldNames {
		field := typeInfo.Fields[fieldName]
		if field.IsDeleted {
			continue
		}

		// Field documentation
		if field.Documentation != "" {
			builder.WriteString(fmt.Sprintf("\t// %s\n", field.Documentation))
		}

		// Field definition
		fieldType := field.Type
		if field.IsPointer {
			fieldType = "*" + fieldType
		}

		// Build tags
		var tags []string
		if field.JsonTag != "" {
			if field.IsRequired {
				tags = append(tags, fmt.Sprintf(`json:"%s"`, field.JsonTag))
			} else {
				tags = append(tags, fmt.Sprintf(`json:"%s,omitempty"`, field.JsonTag))
			}
		}

		for _, validateTag := range field.ValidateTags {
			tags = append(tags, fmt.Sprintf(`validate:"%s"`, validateTag))
		}

		tagString := ""
		if len(tags) > 0 {
			tagString = fmt.Sprintf(" `%s`", strings.Join(tags, " "))
		}

		builder.WriteString(fmt.Sprintf("\t%s %s%s\n", field.Name, fieldType, tagString))
	}

	builder.WriteString("}")

	return builder.String(), nil
}

// generateUtilityFunctions generates version-specific utility functions
func (sg *SchemaGenerator) generateUtilityFunctions(targetVersion string) string {
	var builder strings.Builder

	builder.WriteString("// Version returns the version of this generated code\n")
	builder.WriteString("func Version() string {\n")
	builder.WriteString(fmt.Sprintf("\treturn \"%s\"\n", targetVersion))
	builder.WriteString("}\n\n")

	builder.WriteString("// GeneratedAt returns when this code was generated\n")
	builder.WriteString("func GeneratedAt() string {\n")
	builder.WriteString("\treturn \"time.Now().Format(time.RFC3339)\"\n")
	builder.WriteString("}\n\n")

	// Add migration helper functions
	builder.WriteString(sg.generateMigrationHelpers(targetVersion))

	return builder.String()
}

// generateMigrationHelpers generates helper functions for data migration
func (sg *SchemaGenerator) generateMigrationHelpers(targetVersion string) string {
	var builder strings.Builder

	builder.WriteString("// MigrationHelpers provides utilities for data migration\n")
	builder.WriteString("type MigrationHelpers struct{}\n\n")

	builder.WriteString("// NewMigrationHelpers creates migration helpers for this version\n")
	builder.WriteString("func NewMigrationHelpers() *MigrationHelpers {\n")
	builder.WriteString("\treturn &MigrationHelpers{}\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// ValidateStruct validates a struct according to version-specific rules\n")
	builder.WriteString("func (mh *MigrationHelpers) ValidateStruct(data interface{}) error {\n")
	builder.WriteString("\t// Version-specific validation logic would go here\n")
	builder.WriteString("\treturn nil\n")
	builder.WriteString("}\n\n")

	builder.WriteString("// TransformFromPrevious transforms data from the previous version\n")
	builder.WriteString("func (mh *MigrationHelpers) TransformFromPrevious(data map[string]interface{}) (map[string]interface{}, error) {\n")
	builder.WriteString("\t// Version-specific transformation logic would go here\n")
	builder.WriteString("\treturn data, nil\n")
	builder.WriteString("}\n\n")

	return builder.String()
}

// RegisterType registers a type in the type registry with full introspection
func (sg *SchemaGenerator) RegisterType(t reflect.Type) error {
	sg.typeRegistry.mu.Lock()
	defer sg.typeRegistry.mu.Unlock()

	// Handle pointer types by getting the element type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	typeName := t.Name()
	if typeName == "" {
		return fmt.Errorf("anonymous types are not supported")
	}

	// Create type info with full introspection
	typeInfo := &TypeInfo{
		Name:    typeName,
		Package: t.PkgPath(),
		Kind:    t.Kind(),
		Fields:  make(map[string]*FieldInfo),
		Methods: make(map[string]*MethodInfo),
		Tags:    make(map[string]string),
	}

	// Introspect struct fields if it's a struct
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			fieldInfo := &FieldInfo{
				Name:         field.Name,
				Type:         sg.getTypeString(field.Type),
				JsonTag:      field.Tag.Get("json"),
				ValidateTags: sg.parseValidateTags(field.Tag.Get("validate")),
				IsRequired:   !strings.Contains(field.Tag.Get("json"), "omitempty"),
				IsPointer:    field.Type.Kind() == reflect.Ptr,
			}

			typeInfo.Fields[field.Name] = fieldInfo
		}
	}

	sg.typeRegistry.types[typeName] = typeInfo
	return nil
}

// getTypeString returns a string representation of a type
func (sg *SchemaGenerator) getTypeString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + sg.getTypeString(t.Elem())
	case reflect.Slice:
		return "[]" + sg.getTypeString(t.Elem())
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), sg.getTypeString(t.Elem()))
	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", sg.getTypeString(t.Key()), sg.getTypeString(t.Elem()))
	default:
		if t.PkgPath() != "" && t.Name() != "" {
			return t.Name() // For named types, return just the name
		}
		return t.String()
	}
}

// parseValidateTags parses validation tags into a slice
func (sg *SchemaGenerator) parseValidateTags(validateTag string) []string {
	if validateTag == "" {
		return nil
	}
	return strings.Split(validateTag, ",")
}
