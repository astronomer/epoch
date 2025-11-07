package openapi

import (
	"context"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// Writer handles writing OpenAPI specs to files
type Writer struct {
	format string // "yaml" or "json"
}

// NewWriter creates a new spec writer
func NewWriter(format string) *Writer {
	if format != "yaml" && format != "json" {
		format = "yaml" // Default to YAML
	}
	return &Writer{
		format: format,
	}
}

// WriteSpec writes an OpenAPI spec to a file
func (w *Writer) WriteSpec(spec *openapi3.T, filepath string) error {
	// Validate the spec first
	if err := w.ValidateSpec(spec); err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}

	var data []byte
	var err error

	if w.format == "json" {
		data, err = spec.MarshalJSON()
	} else {
		// Use yaml for both "yaml" and default
		data, err = yaml.Marshal(spec)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ValidateSpec validates an OpenAPI spec
func (w *Writer) ValidateSpec(spec *openapi3.T) error {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	return spec.Validate(context.Background(), openapi3.DisableExamplesValidation())
}

// WriteVersionedSpecs writes multiple versioned specs to files
// filenamePattern should contain a %s placeholder for the version string
// Example: "docs/api_v1alpha1_%s.yaml"
func (w *Writer) WriteVersionedSpecs(specs map[string]*openapi3.T, filenamePattern string) error {
	for version, spec := range specs {
		filepath := fmt.Sprintf(filenamePattern, version)
		if err := w.WriteSpec(spec, filepath); err != nil {
			return fmt.Errorf("failed to write spec for version %s: %w", version, err)
		}
	}
	return nil
}
