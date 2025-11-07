package openapi

import (
	"github.com/astronomer/epoch/epoch"
)

// SchemaGeneratorConfig holds configuration for the OpenAPI schema generator
type SchemaGeneratorConfig struct {
	// VersionBundle contains all versions and their migration definitions
	VersionBundle *epoch.VersionBundle

	// TypeRegistry contains endpoint-to-type mappings from WrapHandler().Returns()/.Accepts()
	TypeRegistry *epoch.EndpointRegistry

	// OutputFormat specifies the output format ("yaml" or "json")
	OutputFormat string

	// IncludeMigrationMetadata adds x-epoch-migrations extensions to schemas (optional)
	IncludeMigrationMetadata bool

	// ComponentNamePrefix is added to all generated component names (e.g., "Astro")
	// Results in component names like "AstroUser" instead of "User"
	ComponentNamePrefix string
}

// SchemaDirection indicates whether we're generating request or response schemas
type SchemaDirection int

const (
	// SchemaDirectionRequest generates schemas for requests (Client → HEAD transformations)
	SchemaDirectionRequest SchemaDirection = iota
	// SchemaDirectionResponse generates schemas for responses (HEAD → Client transformations)
	SchemaDirectionResponse
)

// String returns the string representation of SchemaDirection
func (sd SchemaDirection) String() string {
	switch sd {
	case SchemaDirectionRequest:
		return "request"
	case SchemaDirectionResponse:
		return "response"
	default:
		return "unknown"
	}
}
