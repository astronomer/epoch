package epoch

// AlterRequestInstruction defines how to modify a request during migration
type AlterRequestInstruction struct {
	Schemas     []interface{} // Types this instruction applies to
	Path        string        // Path this instruction applies to (if path-based)
	Methods     []string      // HTTP methods this applies to (if path-based)
	Transformer func(*RequestInfo) error
}

// AlterResponseInstruction defines how to modify a response during migration
type AlterResponseInstruction struct {
	Schemas           []interface{} // Types this instruction applies to
	Path              string        // Path this instruction applies to (if path-based)
	Methods           []string      // HTTP methods this applies to (if path-based)
	MigrateHTTPErrors bool          // Whether to migrate error responses
	Transformer       func(*ResponseInfo) error
}

// SchemaInstruction defines changes to schema/model structures
type SchemaInstruction struct {
	Schema     interface{}
	Name       string
	Type       string // "had", "didnt_exist", etc.
	Attributes map[string]interface{}
	IsHidden   bool
}

// EndpointInstruction defines changes to API endpoints
type EndpointInstruction struct {
	Path       string
	Methods    []string
	FuncName   string
	Type       string // "existed", "didnt_exist", "had"
	Attributes map[string]interface{}
	IsHidden   bool
}

// EnumInstruction defines changes to enum types
type EnumInstruction struct {
	Enum     interface{}
	Type     string // "had_members", "didnt_have_members"
	Members  map[string]interface{}
	IsHidden bool
}

// ConvertRequestToNextVersionFor creates a request migration instruction for specific schemas
func ConvertRequestToNextVersionFor(schemas []interface{}, transformer func(*RequestInfo) error) *AlterRequestInstruction {
	return &AlterRequestInstruction{
		Schemas:     schemas,
		Transformer: transformer,
	}
}

// ConvertRequestToNextVersionForPath creates a request migration instruction for a specific path
func ConvertRequestToNextVersionForPath(path string, methods []string, transformer func(*RequestInfo) error) *AlterRequestInstruction {
	return &AlterRequestInstruction{
		Path:        path,
		Methods:     methods,
		Transformer: transformer,
	}
}

// ConvertResponseToPreviousVersionFor creates a response migration instruction for specific schemas
func ConvertResponseToPreviousVersionFor(schemas []interface{}, transformer func(*ResponseInfo) error, migrateHTTPErrors bool) *AlterResponseInstruction {
	return &AlterResponseInstruction{
		Schemas:           schemas,
		Transformer:       transformer,
		MigrateHTTPErrors: migrateHTTPErrors,
	}
}

// ConvertResponseToPreviousVersionForPath creates a response migration instruction for a specific path
func ConvertResponseToPreviousVersionForPath(path string, methods []string, transformer func(*ResponseInfo) error, migrateHTTPErrors bool) *AlterResponseInstruction {
	return &AlterResponseInstruction{
		Path:              path,
		Methods:           methods,
		Transformer:       transformer,
		MigrateHTTPErrors: migrateHTTPErrors,
	}
}
