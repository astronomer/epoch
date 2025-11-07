package openapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

func TestWriter_NewWriter(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		expectedFormat string
	}{
		{
			name:           "yaml format",
			format:         "yaml",
			expectedFormat: "yaml",
		},
		{
			name:           "json format",
			format:         "json",
			expectedFormat: "json",
		},
		{
			name:           "invalid format defaults to yaml",
			format:         "xml",
			expectedFormat: "yaml",
		},
		{
			name:           "empty format defaults to yaml",
			format:         "",
			expectedFormat: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := NewWriter(tt.format)
			if writer.format != tt.expectedFormat {
				t.Errorf("expected format %s, got %s", tt.expectedFormat, writer.format)
			}
		})
	}
}

func TestWriter_WriteSpec_YAML(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "test_spec.yaml")

	// Create a simple OpenAPI spec
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Write the spec
	writer := NewWriter("yaml")
	err := writer.WriteSpec(spec, filepath)
	if err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Fatal("spec file was not created")
	}

	// Read and verify content
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}

	// Parse YAML to verify it's valid
	var parsed map[string]interface{}
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Check basic structure
	if parsed["openapi"] != "3.0.0" {
		t.Error("openapi version not correct in output")
	}

	info, ok := parsed["info"].(map[string]interface{})
	if !ok {
		t.Fatal("info section not found")
	}

	if info["title"] != "Test API" {
		t.Error("title not correct in output")
	}
}

func TestWriter_WriteSpec_JSON(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "test_spec.json")

	// Create a simple OpenAPI spec
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Write the spec
	writer := NewWriter("json")
	err := writer.WriteSpec(spec, filepath)
	if err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Fatal("spec file was not created")
	}

	// Read the file
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}

	// Verify it's valid JSON by parsing it
	loader := openapi3.NewLoader()
	_, err = loader.LoadFromData(data)
	if err != nil {
		t.Fatalf("failed to parse JSON spec: %v", err)
	}
}

func TestWriter_ValidateSpec(t *testing.T) {
	tests := []struct {
		name      string
		spec      *openapi3.T
		expectErr bool
	}{
		{
			name: "valid spec",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(),
			},
			expectErr: false,
		},
		{
			name: "missing info",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Paths:   openapi3.NewPaths(),
			},
			expectErr: true,
		},
		{
			name: "missing openapi version",
			spec: &openapi3.T{
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(),
			},
			expectErr: true,
		},
	}

	writer := NewWriter("yaml")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writer.ValidateSpec(tt.spec)
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestWriter_WriteVersionedSpecs(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	filenamePattern := filepath.Join(tmpDir, "api_%s.yaml")

	// Create multiple specs
	specs := map[string]*openapi3.T{
		"v1": {
			OpenAPI: "3.0.0",
			Info: &openapi3.Info{
				Title:   "Test API v1",
				Version: "1.0.0",
			},
			Paths: openapi3.NewPaths(),
		},
		"v2": {
			OpenAPI: "3.0.0",
			Info: &openapi3.Info{
				Title:   "Test API v2",
				Version: "2.0.0",
			},
			Paths: openapi3.NewPaths(),
		},
		"head": {
			OpenAPI: "3.0.0",
			Info: &openapi3.Info{
				Title:   "Test API HEAD",
				Version: "head",
			},
			Paths: openapi3.NewPaths(),
		},
	}

	// Write versioned specs
	writer := NewWriter("yaml")
	err := writer.WriteVersionedSpecs(specs, filenamePattern)
	if err != nil {
		t.Fatalf("failed to write versioned specs: %v", err)
	}

	// Verify all files were created
	for version := range specs {
		filename := filepath.Join(tmpDir, "api_"+version+".yaml")
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("spec file for version %s was not created", version)
		}
	}

	// Verify content of one file
	v1File := filepath.Join(tmpDir, "api_v1.yaml")
	data, err := os.ReadFile(v1File)
	if err != nil {
		t.Fatalf("failed to read v1 spec: %v", err)
	}

	var parsed map[string]interface{}
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to parse v1 spec: %v", err)
	}

	info, ok := parsed["info"].(map[string]interface{})
	if !ok {
		t.Fatal("info section not found in v1 spec")
	}

	if info["title"] != "Test API v1" {
		t.Error("v1 spec has wrong title")
	}
}

func TestWriter_WriteSpec_InvalidSpec(t *testing.T) {
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "invalid_spec.yaml")

	// Create an invalid spec (missing required fields)
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		// Missing Info which is required
		Paths: openapi3.NewPaths(),
	}

	writer := NewWriter("yaml")
	err := writer.WriteSpec(spec, filepath)
	if err == nil {
		t.Error("expected error for invalid spec, got none")
	}
}

func TestWriter_WriteSpec_WithComponents(t *testing.T) {
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "spec_with_components.yaml")

	// Create a spec with components
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "API with Components",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"User": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}),
			},
		},
	}

	writer := NewWriter("yaml")
	err := writer.WriteSpec(spec, filepath)
	if err != nil {
		t.Fatalf("failed to write spec with components: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read spec: %v", err)
	}

	// Parse and validate
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loadedSpec, err := loader.LoadFromData(data)
	if err != nil {
		t.Fatalf("failed to load spec: %v", err)
	}

	// Validate
	err = loadedSpec.Validate(context.Background())
	if err != nil {
		t.Fatalf("spec validation failed: %v", err)
	}

	// Check that User schema exists
	if loadedSpec.Components == nil || loadedSpec.Components.Schemas == nil {
		t.Fatal("components.schemas not found")
	}

	userSchema, exists := loadedSpec.Components.Schemas["User"]
	if !exists {
		t.Error("User schema not found in components")
	}

	if userSchema.Value == nil {
		t.Fatal("User schema value is nil")
	}

	if len(userSchema.Value.Properties) != 2 {
		t.Errorf("expected 2 properties in User schema, got %d", len(userSchema.Value.Properties))
	}
}
