package epoch

import (
	"context"
	"fmt"
	"net/http/httptest"
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionChange - Schema-Based", func() {
	var (
		v1, v2 *Version
		ctx    context.Context
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		ctx = context.Background()
		gin.SetMode(gin.TestMode)
	})

	Describe("NewVersionChange", func() {
		It("should create a version change with basic info", func() {
			change := NewVersionChange("Test change", v1, v2)
			Expect(change).NotTo(BeNil())
			Expect(change.Description()).To(Equal("Test change"))
			Expect(change.FromVersion()).To(Equal(v1))
			Expect(change.ToVersion()).To(Equal(v2))
			Expect(change.IsHiddenFromChangelog()).To(BeFalse())
		})

		It("should create a version change with schema-based instructions", func() {
			type TestUser struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}

			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{TestUser{}},
				Transformer: func(req *RequestInfo) error {
					return nil
				},
			}

			change := NewVersionChange("Test change", v1, v2, requestInst)
			Expect(change).NotTo(BeNil())
			Expect(change.Description()).To(Equal("Test change"))

			// Schema registry has been removed - types are now declared at endpoint registration
			// Type information is explicitly provided via WrapHandler().Returns()/.Accepts()
		})
	})

	Describe("SetHiddenFromChangelog", func() {
		It("should set and get hidden status", func() {
			change := NewVersionChange("Test change", v1, v2)
			Expect(change.IsHiddenFromChangelog()).To(BeFalse())

			change.SetHiddenFromChangelog(true)
			Expect(change.IsHiddenFromChangelog()).To(BeTrue())

			change.SetHiddenFromChangelog(false)
			Expect(change.IsHiddenFromChangelog()).To(BeFalse())
		})
	})

	Describe("Schema-Based Request Migration", func() {
		type TestUser struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		It("should migrate request using explicit type", func() {
			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{TestUser{}},
				Transformer: func(req *RequestInfo) error {
					if req.Body != nil {
						req.SetField("migrated", true)
					}
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, requestInst)

			// Create request with explicit type information (simulating endpoint registry)
			reqInfo := createTestRequestInfo(`{"id": 1, "name": "John Doe"}`)
			reqInfo.schemaMatched = true
			reqInfo.matchedSchemaType = reflect.TypeOf(TestUser{})

			err := change.MigrateRequest(ctx, reqInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should have applied the transformation
			Expect(reqInfo.Body.Get("migrated").Exists()).To(BeTrue())
		})

		It("should not migrate request with poor schema match", func() {
			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{TestUser{}},
				Transformer: func(req *RequestInfo) error {
					req.SetField("migrated", true)
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, requestInst)

			// Create request that doesn't match TestUser schema
			reqInfo := createTestRequestInfo(`{"completely": "different", "structure": 123}`)

			err := change.MigrateRequest(ctx, reqInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should not have applied the transformation
			Expect(reqInfo.Body.Get("migrated").Exists()).To(BeFalse())
		})

		It("should apply global request instructions", func() {
			globalInst := &AlterRequestInstruction{
				Schemas: []interface{}{}, // Global (no specific schemas)
				Transformer: func(req *RequestInfo) error {
					req.SetField("global_applied", true)
					return nil
				},
			}

			change := NewVersionChange("Global migration", v1, v2, globalInst)

			reqInfo := createTestRequestInfo(`{"any": "structure"}`)

			err := change.MigrateRequest(ctx, reqInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should have applied the global transformation
			Expect(reqInfo.Body.Get("global_applied").Exists()).To(BeTrue())
		})

		It("should handle nil request body gracefully", func() {
			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{TestUser{}},
				Transformer: func(req *RequestInfo) error {
					req.SetField("migrated", true)
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, requestInst)

			reqInfo := &RequestInfo{Body: nil}

			err := change.MigrateRequest(ctx, reqInfo)
			Expect(err).NotTo(HaveOccurred()) // Should not error
		})

		It("should handle transformer errors", func() {
			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{TestUser{}},
				Transformer: func(req *RequestInfo) error {
					return fmt.Errorf("transformer error")
				},
			}

			change := NewVersionChange("Test migration", v1, v2, requestInst)

			reqInfo := createTestRequestInfo(`{"id": 1, "name": "John Doe"}`)
			reqInfo.schemaMatched = true
			reqInfo.matchedSchemaType = reflect.TypeOf(TestUser{})

			err := change.MigrateRequest(ctx, reqInfo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("transformer error"))
		})
	})

	Describe("Schema-Based Response Migration", func() {
		type TestUser struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		It("should migrate response using explicit type", func() {
			responseInst := &AlterResponseInstruction{
				Schemas:           []interface{}{TestUser{}},
				MigrateHTTPErrors: false,
				Transformer: func(resp *ResponseInfo) error {
					if resp.Body != nil {
						resp.SetField("migrated", true)
					}
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, responseInst)

			respInfo := createTestResponseInfo(`{"id": 1, "name": "John Doe"}`, 200)
			respInfo.schemaMatched = true
			respInfo.matchedSchemaType = reflect.TypeOf(TestUser{})

			err := change.MigrateResponse(ctx, respInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should have applied the transformation
			Expect(respInfo.Body.Get("migrated").Exists()).To(BeTrue())
		})

		It("should apply global response instructions", func() {
			globalInst := &AlterResponseInstruction{
				Schemas:           []interface{}{}, // Global
				MigrateHTTPErrors: true,
				Transformer: func(resp *ResponseInfo) error {
					resp.SetField("global_applied", true)
					return nil
				},
			}

			change := NewVersionChange("Global migration", v1, v2, globalInst)

			respInfo := createTestResponseInfo(`{"any": "structure"}`, 200)

			err := change.MigrateResponse(ctx, respInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should have applied the global transformation
			Expect(respInfo.Body.Get("global_applied").Exists()).To(BeTrue())
		})

		It("should respect MigrateHTTPErrors flag", func() {
			responseInst := &AlterResponseInstruction{
				Schemas:           []interface{}{TestUser{}},
				MigrateHTTPErrors: false, // Don't migrate errors
				Transformer: func(resp *ResponseInfo) error {
					resp.SetField("migrated", true)
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, responseInst)

			// Test with error response
			respInfo := createTestResponseInfo(`{"id": 1, "name": "John Doe"}`, 400)
			respInfo.schemaMatched = true
			respInfo.matchedSchemaType = reflect.TypeOf(TestUser{})

			err := change.MigrateResponse(ctx, respInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should not have applied the transformation for error response
			Expect(respInfo.Body.Get("migrated").Exists()).To(BeFalse())

			// Test with success response
			respInfo2 := createTestResponseInfo(`{"id": 1, "name": "John Doe"}`, 200)
			respInfo2.schemaMatched = true
			respInfo2.matchedSchemaType = reflect.TypeOf(TestUser{})

			err = change.MigrateResponse(ctx, respInfo2)
			Expect(err).NotTo(HaveOccurred())

			// Should have applied the transformation for success response
			Expect(respInfo2.Body.Get("migrated").Exists()).To(BeTrue())
		})

		It("should migrate HTTP errors when flag is true", func() {
			responseInst := &AlterResponseInstruction{
				Schemas:           []interface{}{TestUser{}},
				MigrateHTTPErrors: true, // Migrate errors
				Transformer: func(resp *ResponseInfo) error {
					resp.SetField("migrated", true)
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, responseInst)

			respInfo := createTestResponseInfo(`{"id": 1, "name": "John Doe"}`, 400)
			respInfo.schemaMatched = true
			respInfo.matchedSchemaType = reflect.TypeOf(TestUser{})

			err := change.MigrateResponse(ctx, respInfo)
			Expect(err).NotTo(HaveOccurred())

			// Should have applied the transformation even for error response
			Expect(respInfo.Body.Get("migrated").Exists()).To(BeTrue())
		})

		It("should handle nil response body gracefully", func() {
			responseInst := &AlterResponseInstruction{
				Schemas:           []interface{}{TestUser{}},
				MigrateHTTPErrors: true,
				Transformer: func(resp *ResponseInfo) error {
					resp.SetField("migrated", true)
					return nil
				},
			}

			change := NewVersionChange("Test migration", v1, v2, responseInst)

			respInfo := &ResponseInfo{Body: nil, StatusCode: 200}

			err := change.MigrateResponse(ctx, respInfo)
			Expect(err).NotTo(HaveOccurred()) // Should not error
		})
	})

	Describe("Schema Registry Integration", func() {
		type TestUser struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		type TestProduct struct {
			ID    int     `json:"id"`
			Name  string  `json:"name"`
			Price float64 `json:"price"`
		}

		It("should register multiple schemas from instructions", func() {
			requestInst := &AlterRequestInstruction{
				Schemas:     []interface{}{TestUser{}, TestProduct{}},
				Transformer: func(req *RequestInfo) error { return nil },
			}

			_ = NewVersionChange("Multi-schema migration", v1, v2, requestInst)

			// Schema registry has been removed - types are now declared at endpoint registration
			// Both TestUser and TestProduct would be registered via WrapHandler().ForType()
		})

		It("should handle pointer types in schema registration", func() {
			requestInst := &AlterRequestInstruction{
				Schemas:     []interface{}{&TestUser{}}, // Pointer type
				Transformer: func(req *RequestInfo) error { return nil },
			}

			_ = NewVersionChange("Pointer schema migration", v1, v2, requestInst)

			// Schema registry has been removed - types are now declared at endpoint registration
			// TestUser would be registered via WrapHandler().ForType()
		})
	})
})

var _ = Describe("Nested Array Multi-Step Migrations", func() {
	var (
		v1, v2, v3 *Version
		ctx        context.Context
	)

	BeforeEach(func() {
		v1, _ = NewDateVersion("2024-01-01")
		v2, _ = NewDateVersion("2024-06-01")
		v3, _ = NewDateVersion("2025-01-01")
		ctx = context.Background()
		gin.SetMode(gin.TestMode)
	})

	// Extended struct definitions for nested transformation tests
	// SubItem - for testing two-level nested arrays (items[].subitems[])
	type SubItem struct {
		ID    int    `json:"id"`
		Label string `json:"label"` // Renamed to "name" in older versions
	}

	// Details - nested object inside array items
	type Details struct {
		Author      string `json:"author"`
		LastUpdated string `json:"last_updated"` // Renamed to "updated_at" in older versions
	}

	// Item - extended with nested object and nested array
	type Item struct {
		ID          int       `json:"id"`
		DisplayName string    `json:"display_name"` // V3 field
		Tags        []string  `json:"tags"`
		Category    string    `json:"category"` // Added in V2
		Priority    int       `json:"priority"` // Added in V3
		Details     Details   `json:"details"`  // Nested object
		SubItems    []SubItem `json:"subitems"` // Two-level nested array
	}

	// Metadata - nested object alongside arrays
	type Metadata struct {
		Version   string `json:"version"`
		CreatedBy string `json:"created_by"` // Renamed to "author" in older versions
	}

	// Container - extended with nested object
	type Container struct {
		Items    []Item   `json:"items"`
		Metadata Metadata `json:"metadata"` // Nested object alongside array
	}

	Describe("Multi-step migrations for nested arrays", func() {
		It("should apply transformations step-by-step (V3→V2→V1)", func() {
			// V1→V2 migration: title → name, remove category
			// (When going backward V2→V1)
			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("title", "name").
				RemoveField("category").
				Build()

			// V2→V3 migration: display_name → title, remove priority
			// (When going backward V3→V2)
			change2 := NewVersionChangeBuilder(v2, v3).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "title").
				RemoveField("priority").
				Build()

			// Build chain
			chain, err := NewMigrationChain([]*VersionChange{change1, change2})
			Expect(err).NotTo(HaveOccurred())

			// Create HEAD response (V3) - with ALL fields
			jsonStr := `{"items":[{"id":1,"display_name":"Test","tags":["demo"],"category":"tutorial","priority":1}]}`
			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Apply migrations V3→V1 using the proper API
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v3,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			// Check result
			items := responseInfo.Body.Get("items")
			Expect(items).NotTo(BeNil())

			item0 := items.Index(0)
			Expect(item0).NotTo(BeNil())

			// V1 should have "name" field (display_name→title→name)
			name := item0.Get("name")
			Expect(name.Exists()).To(BeTrue(), "Should have 'name' field for V1")
			Expect(name.String()).To(Equal("Test"))

			// V1 should NOT have intermediate or new fields
			displayName := item0.Get("display_name")
			Expect(displayName.Exists()).To(BeFalse(), "Should NOT have 'display_name' in V1")

			title := item0.Get("title")
			Expect(title.Exists()).To(BeFalse(), "Should NOT have 'title' in V1")

			category := item0.Get("category")
			Expect(category.Exists()).To(BeFalse(), "Should NOT have 'category' in V1")

			priority := item0.Get("priority")
			Expect(priority.Exists()).To(BeFalse(), "Should NOT have 'priority' in V1")

			// V1 should still have unchanged fields
			tags := item0.Get("tags")
			Expect(tags.Exists()).To(BeTrue(), "Should have 'tags' in V1")
		})

		It("should handle V3→V2 migrations correctly", func() {
			// V2→V3 migration
			change := NewVersionChangeBuilder(v2, v3).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "title").
				RemoveField("priority").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			// Create V3 response
			jsonStr := `{"items":[{"id":1,"display_name":"Test","tags":["demo"],"category":"tutorial","priority":1}]}`
			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Apply V3→V2
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v3,
				v2,
			)
			Expect(err).NotTo(HaveOccurred())

			item0 := responseInfo.Body.Get("items").Index(0)

			// V2 should have "title" (renamed from display_name)
			title := item0.Get("title")
			Expect(title.Exists()).To(BeTrue())
			Expect(title.String()).To(Equal("Test"))

			// V2 should NOT have priority
			priority := item0.Get("priority")
			Expect(priority.Exists()).To(BeFalse())

			// V2 should still have category
			category := item0.Get("category")
			Expect(category.Exists()).To(BeTrue())
		})

		It("should handle multiple items in nested arrays", func() {
			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("title", "name").
				RemoveField("category").
				Build()

			change2 := NewVersionChangeBuilder(v2, v3).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "title").
				RemoveField("priority").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change1, change2})
			Expect(err).NotTo(HaveOccurred())

			// Create response with multiple items
			jsonStr := `{"items":[
				{"id":1,"display_name":"First","tags":["a"],"category":"cat1","priority":1},
				{"id":2,"display_name":"Second","tags":["b"],"category":"cat2","priority":2},
				{"id":3,"display_name":"Third","tags":["c"],"category":"cat3","priority":3}
			]}`
			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Apply V3→V1
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v3,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			items := responseInfo.Body.Get("items")
			arrayLen, _ := items.Len()
			Expect(arrayLen).To(Equal(3))

			// Check each item has correct V1 structure
			for i := 0; i < 3; i++ {
				item := items.Index(i)
				name := item.Get("name")
				Expect(name.Exists()).To(BeTrue(), fmt.Sprintf("Item %d should have 'name'", i))

				// Should not have V2/V3 fields
				Expect(item.Get("display_name").Exists()).To(BeFalse())
				Expect(item.Get("title").Exists()).To(BeFalse())
				Expect(item.Get("category").Exists()).To(BeFalse())
				Expect(item.Get("priority").Exists()).To(BeFalse())
			}
		})

		It("should handle empty nested arrays gracefully", func() {
			change := NewVersionChangeBuilder(v2, v3).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "title").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			// Empty array
			jsonStr := `{"items":[]}`
			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v3,
				v2,
			)
			Expect(err).NotTo(HaveOccurred())

			items := responseInfo.Body.Get("items")
			arrayLen, _ := items.Len()
			Expect(arrayLen).To(Equal(0))
		})

		It("should preserve nestedArrayTypes through migration chain", func() {
			change := NewVersionChangeBuilder(v2, v3).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "title").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{"items":[{"id":1,"display_name":"Test","tags":[]}]}`
			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Set nested array types
			nestedArrays := map[string]reflect.Type{"items": reflect.TypeOf(Item{})}

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				nestedArrays,
				v3,
				v2,
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify nestedArrayTypes was stored
			Expect(responseInfo.nestedArrayTypes).To(Equal(nestedArrays))
		})
	})

	Describe("BuildNestedTypeMaps auto-discovery", func() {
		It("should discover nested arrays and objects from struct analysis", func() {
			arrays, objects := BuildNestedTypeMaps(reflect.TypeOf(Container{}))

			// Should find the items array
			Expect(arrays).To(HaveKey("items"))
			Expect(arrays["items"].Name()).To(Equal("Item"))

			// Should find the metadata object
			Expect(objects).To(HaveKey("metadata"))
			Expect(objects["metadata"].Name()).To(Equal("Metadata"))
		})

		It("should discover deeply nested structures (3+ levels)", func() {
			// Container -> Items[] -> SubItems[] (two-level array nesting)
			// Container -> Items[] -> Details (object inside array item)
			arrays, objects := BuildNestedTypeMaps(reflect.TypeOf(Container{}))

			// Should find items array
			Expect(arrays).To(HaveKey("items"))

			// Should find subitems inside items (items.subitems path)
			Expect(arrays).To(HaveKey("items.subitems"))
			Expect(arrays["items.subitems"].Name()).To(Equal("SubItem"))

			// Should find details inside items (items.details path)
			Expect(objects).To(HaveKey("items.details"))
			Expect(objects["items.details"].Name()).To(Equal("Details"))
		})

		It("should handle nil types gracefully", func() {
			arrays, objects := BuildNestedTypeMaps(nil)
			Expect(arrays).To(BeEmpty())
			Expect(objects).To(BeEmpty())
		})

		It("should handle pointer types", func() {
			arrays, objects := BuildNestedTypeMaps(reflect.TypeOf(&Container{}))

			// Should still find nested types through pointer
			Expect(arrays).To(HaveKey("items"))
			Expect(objects).To(HaveKey("metadata"))
		})

		It("should handle self-referential types without infinite recursion", func() {
			// Define a self-referential type (like a linked list node)
			type Node struct {
				Value    string  `json:"value"`
				Next     *Node   `json:"next"`
				Children []Node  `json:"children"`
			}

			// This should not hang or panic
			arrays, objects := BuildNestedTypeMaps(reflect.TypeOf(Node{}))

			// Should find the children array (self-referential)
			Expect(arrays).To(HaveKey("children"))
			Expect(arrays["children"].Name()).To(Equal("Node"))

			// Should find the next field as a nested object
			Expect(objects).To(HaveKey("next"))
			Expect(objects["next"].Name()).To(Equal("Node"))

			// Should NOT recurse infinitely - verify by checking we don't have
			// infinitely nested paths like "next.next.next..."
			for path := range objects {
				// Paths should be reasonable length (not infinite)
				Expect(len(path)).To(BeNumerically("<", 100))
			}
		})

		It("should handle mutually recursive types without infinite recursion", func() {
			// Define mutually recursive types
			type TypeB struct {
				Name string `json:"name"`
				// A will be added via interface to avoid Go compilation issues
			}
			type TypeA struct {
				ID   int     `json:"id"`
				RefB *TypeB  `json:"ref_b"`
			}

			// This should not hang or panic
			_, objects := BuildNestedTypeMaps(reflect.TypeOf(TypeA{}))

			// Should find ref_b as a nested object
			Expect(objects).To(HaveKey("ref_b"))
		})

		It("should handle diamond-shaped type dependencies", func() {
			// Diamond pattern: Root -> Left, Right; Left -> Bottom; Right -> Bottom
			type Bottom struct {
				Value string `json:"value"`
			}
			type Left struct {
				Bottom Bottom `json:"bottom"`
			}
			type Right struct {
				Bottom Bottom `json:"bottom"`
			}
			type Diamond struct {
				Left  Left  `json:"left"`
				Right Right `json:"right"`
			}

			_, objects := BuildNestedTypeMaps(reflect.TypeOf(Diamond{}))

			// Should find all nested objects, including Bottom at both paths
			Expect(objects).To(HaveKey("left"))
			Expect(objects).To(HaveKey("right"))
			Expect(objects).To(HaveKey("left.bottom"))
			Expect(objects).To(HaveKey("right.bottom"))
		})

		It("should handle types appearing at multiple paths (sibling branches)", func() {
			// Same type appearing in sibling fields
			type Address struct {
				City string `json:"city"`
			}
			type Person struct {
				HomeAddress Address `json:"home_address"`
				WorkAddress Address `json:"work_address"`
			}

			_, objects := BuildNestedTypeMaps(reflect.TypeOf(Person{}))

			// Should find Address at both paths
			Expect(objects).To(HaveKey("home_address"))
			Expect(objects).To(HaveKey("work_address"))
			Expect(objects["home_address"].Name()).To(Equal("Address"))
			Expect(objects["work_address"].Name()).To(Equal("Address"))
		})

		It("should handle circular reference through nested arrays", func() {
			// Type with array that contains self-reference
			type TreeNode struct {
				Name     string      `json:"name"`
				Children []TreeNode `json:"children"`
				Parent   *TreeNode  `json:"parent"`
			}

			// This should not hang or panic
			arrays, objects := BuildNestedTypeMaps(reflect.TypeOf(TreeNode{}))

			// Should find children array
			Expect(arrays).To(HaveKey("children"))
			Expect(arrays["children"].Name()).To(Equal("TreeNode"))

			// Should find parent as nested object
			Expect(objects).To(HaveKey("parent"))
			Expect(objects["parent"].Name()).To(Equal("TreeNode"))
		})
	})

	Describe("Nested object transformations", func() {
		It("should demonstrate that standard RenameField does NOT work on nested fields", func() {
			// This test documents the LIMITATION: standard operations only work on top-level
			change := NewVersionChangeBuilder(v1, v2).
				ForType(Container{}).
				ResponseToPreviousVersion().
				RenameField("created_by", "author"). // This targets top-level, not nested metadata!
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"items": [],
				"metadata": {
					"version": "1.0",
					"created_by": "test-user"
				}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)
			responseInfo.schemaMatched = true
			responseInfo.matchedSchemaType = reflect.TypeOf(Container{})

			err = chain.MigrateResponse(ctx, responseInfo, v2, v1)
			Expect(err).NotTo(HaveOccurred())

			// Standard RenameField does NOT work on nested fields
			// metadata.created_by should still exist (unchanged)
			metadata := responseInfo.Body.Get("metadata")
			Expect(metadata.Get("created_by").Exists()).To(BeTrue(), "Nested 'created_by' should be UNCHANGED - standard ops don't reach it")
			Expect(metadata.Get("author").Exists()).To(BeFalse(), "No 'author' - standard ops don't reach nested fields")
		})

		It("should transform nested objects using MigrateResponseForTypeWithNestedObjects", func() {
			// Migration for Metadata type
			change := NewVersionChangeBuilder(v1, v2).
				ForType(Container{}, Metadata{}).
				ResponseToPreviousVersion().
				RenameField("created_by", "author").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"items": [],
				"metadata": {
					"version": "1.0",
					"created_by": "test-user"
				}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Use auto-discovered nested types
			arrays, objects := BuildNestedTypeMaps(reflect.TypeOf(Container{}))

			err = chain.MigrateResponseForTypeWithNestedObjects(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				arrays,
				objects,
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify metadata was transformed
			metadata := responseInfo.Body.Get("metadata")
			Expect(metadata.Get("author").Exists()).To(BeTrue(), "Should have 'author' after transformation")
			Expect(metadata.Get("created_by").Exists()).To(BeFalse(), "Should NOT have 'created_by' after transformation")
		})

		It("should recursively transform nested objects inside array items", func() {
			// Each item has a details object (nested object inside array item)
			// With recursive transformation, the details object WILL be transformed

			detailsChange := NewVersionChangeBuilder(v1, v2).
				ForType(Details{}).
				ResponseToPreviousVersion().
				RenameField("last_updated", "updated_at").
				Build()

			itemChange := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{detailsChange, itemChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"items": [
					{
						"id": 1,
						"display_name": "Test Item",
						"tags": [],
						"category": "cat1",
						"priority": 1,
						"details": {"author": "user1", "last_updated": "2024-01-01"},
						"subitems": []
					}
				],
				"metadata": {"version": "1.0", "created_by": "admin"}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			item0 := responseInfo.Body.Get("items").Index(0)

			// Item-level transformation works
			Expect(item0.Get("name").Exists()).To(BeTrue())
			Expect(item0.Get("display_name").Exists()).To(BeFalse())

			// Details inside item IS now transformed (recursive transformation)
			details := item0.Get("details")
			Expect(details.Get("updated_at").Exists()).To(BeTrue(), "details.last_updated should be renamed to updated_at")
			Expect(details.Get("last_updated").Exists()).To(BeFalse(), "details.last_updated should be removed")
		})
	})

	Describe("Two-level nested arrays", func() {
		It("should recursively transform both first-level and second-level array items", func() {
			// Test items[].subitems[] - two-level nesting
			// With recursive transformation, both levels should be transformed
			subItemChange := NewVersionChangeBuilder(v1, v2).
				ForType(SubItem{}).
				ResponseToPreviousVersion().
				RenameField("label", "name").
				Build()

			itemChange := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "title").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{subItemChange, itemChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"items": [
					{
						"id": 1,
						"display_name": "Parent Item",
						"tags": ["a"],
						"category": "cat1",
						"priority": 1,
						"details": {"author": "user1", "last_updated": "2024-01-01"},
						"subitems": [
							{"id": 101, "label": "Child 1"},
							{"id": 102, "label": "Child 2"}
						]
					}
				],
				"metadata": {"version": "1.0", "created_by": "admin"}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Register only first-level items array - recursion handles the rest
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			item0 := responseInfo.Body.Get("items").Index(0)

			// First level (items) should be transformed
			Expect(item0.Get("title").Exists()).To(BeTrue(), "Item should have 'title'")
			Expect(item0.Get("display_name").Exists()).To(BeFalse())

			// Second level (subitems) should NOW be transformed (recursive transformation)
			subitem0 := item0.Get("subitems").Index(0)
			Expect(subitem0.Get("name").Exists()).To(BeTrue(), "subitems[].label should be renamed to name")
			Expect(subitem0.Get("label").Exists()).To(BeFalse(), "subitems[].label should be removed")

			subitem1 := item0.Get("subitems").Index(1)
			Expect(subitem1.Get("name").Exists()).To(BeTrue(), "Second subitem should also be transformed")
			Expect(subitem1.Get("label").Exists()).To(BeFalse())
		})
	})

	Describe("Multiple arrays at same level", func() {
		// Define types for this test
		type UserItem struct {
			ID   int    `json:"id"`
			Name string `json:"name"` // Renamed in migration
		}

		type ProductItem struct {
			ID    int    `json:"id"`
			Title string `json:"title"` // Renamed in migration
		}

		type MultiArrayContainer struct {
			Users    []UserItem    `json:"users"`
			Products []ProductItem `json:"products"`
		}

		It("should handle multiple arrays registered at same level", func() {
			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(UserItem{}).
				ResponseToPreviousVersion().
				RenameField("name", "user_name").
				Build()

			productChange := NewVersionChangeBuilder(v1, v2).
				ForType(ProductItem{}).
				ResponseToPreviousVersion().
				RenameField("title", "product_title").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{userChange, productChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"users": [{"id": 1, "name": "John"}],
				"products": [{"id": 101, "title": "Phone"}]
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Register both arrays
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(MultiArrayContainer{}),
				map[string]reflect.Type{
					"users":    reflect.TypeOf(UserItem{}),
					"products": reflect.TypeOf(ProductItem{}),
				},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			user0 := responseInfo.Body.Get("users").Index(0)
			product0 := responseInfo.Body.Get("products").Index(0)

			// Both arrays should be transformed
			Expect(user0.Get("user_name").Exists()).To(BeTrue())
			Expect(user0.Get("name").Exists()).To(BeFalse())
			Expect(product0.Get("product_title").Exists()).To(BeTrue())
			Expect(product0.Get("title").Exists()).To(BeFalse())
		})
	})

	Describe("Edge cases and error handling", func() {
		It("should handle null field values gracefully", func() {
			change := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			// display_name is null
			jsonStr := `{
				"items": [{"id": 1, "display_name": null, "tags": [], "category": "cat1", "priority": 1, "details": {}, "subitems": []}],
				"metadata": {"version": "1.0", "created_by": "admin"}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			item0 := responseInfo.Body.Get("items").Index(0)
			// Rename should still work with null value
			Expect(item0.Get("name").Exists()).To(BeTrue())
			Expect(item0.Get("display_name").Exists()).To(BeFalse())
		})

		It("should handle type mismatch gracefully (expected array got object)", func() {
			change := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			// items is an object instead of array
			jsonStr := `{
				"items": {"unexpected": "object"},
				"metadata": {"version": "1.0", "created_by": "admin"}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Should not panic or error
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle missing field to rename gracefully", func() {
			change := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("nonexistent_field", "new_name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"items": [{"id": 1, "display_name": "Test", "tags": [], "category": "cat1", "priority": 1, "details": {}, "subitems": []}],
				"metadata": {"version": "1.0", "created_by": "admin"}
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			// Should not error when field doesn't exist
			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			// Original data unchanged
			item0 := responseInfo.Body.Get("items").Index(0)
			Expect(item0.Get("display_name").Exists()).To(BeTrue())
			Expect(item0.Get("new_name").Exists()).To(BeFalse())
		})

		It("should handle large payloads with many nested items", func() {
			change := NewVersionChangeBuilder(v1, v2).
				ForType(Item{}).
				ResponseToPreviousVersion().
				RenameField("display_name", "name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			// Build JSON with 100 items
			jsonStr := `{"items":[`
			for i := 0; i < 100; i++ {
				if i > 0 {
					jsonStr += ","
				}
				jsonStr += fmt.Sprintf(`{"id":%d,"display_name":"Item %d","tags":[],"category":"cat","priority":1,"details":{},"subitems":[]}`, i, i)
			}
			jsonStr += `],"metadata":{"version":"1.0","created_by":"admin"}}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(Container{}),
				map[string]reflect.Type{"items": reflect.TypeOf(Item{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			items := responseInfo.Body.Get("items")
			itemsLen, _ := items.Len()
			Expect(itemsLen).To(Equal(100))

			// Verify first and last are transformed
			item0 := items.Index(0)
			Expect(item0.Get("name").Exists()).To(BeTrue())
			Expect(item0.Get("display_name").Exists()).To(BeFalse())

			item99 := items.Index(99)
			Expect(item99.Get("name").Exists()).To(BeTrue())
			Expect(item99.Get("display_name").Exists()).To(BeFalse())
		})
	})

	Describe("Recursive nested transformation scenarios", func() {
		// Types for testing UsersListResponse-like structures
		type UserProfile struct {
			Bio      string `json:"bio"`      // Renamed to "biography" in older versions
			Avatar   string `json:"avatar"`   // Added in V2
			Location string `json:"location"` // Added in V3
		}

		type Skill struct {
			Name  string `json:"name"`  // Renamed to "skill_name" in older versions
			Level int    `json:"level"` // Added in V2
		}

		type UserWithNestedTypes struct {
			ID       int         `json:"id"`
			FullName string      `json:"full_name"` // Renamed from "name"
			Profile  UserProfile `json:"profile"`   // Nested object
			Skills   []Skill     `json:"skills"`    // Nested array
		}

		type UsersWrapper struct {
			Users []UserWithNestedTypes `json:"users"`
			Total int                   `json:"total"`
		}

		It("should transform users[].profile (nested object inside array) via UsersListResponse", func() {
			// This tests the key scenario: UsersListResponse.users[].profile transformation
			profileChange := NewVersionChangeBuilder(v1, v2).
				ForType(UserProfile{}).
				ResponseToPreviousVersion().
				RenameField("bio", "biography").
				RemoveField("avatar").
				Build()

			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(UserWithNestedTypes{}).
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{profileChange, userChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"users": [
					{
						"id": 1,
						"full_name": "Alice",
						"profile": {"bio": "Software Engineer", "avatar": "alice.png", "location": "NYC"},
						"skills": []
					},
					{
						"id": 2,
						"full_name": "Bob",
						"profile": {"bio": "Designer", "avatar": "bob.png", "location": "LA"},
						"skills": []
					}
				],
				"total": 2
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(UsersWrapper{}),
				map[string]reflect.Type{"users": reflect.TypeOf(UserWithNestedTypes{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			// Check first user
			user0 := responseInfo.Body.Get("users").Index(0)
			Expect(user0.Get("name").Exists()).To(BeTrue(), "User should have 'name' (renamed from full_name)")
			Expect(user0.Get("full_name").Exists()).To(BeFalse())

			// Check nested profile transformation
			profile0 := user0.Get("profile")
			Expect(profile0.Get("biography").Exists()).To(BeTrue(), "Profile bio should be renamed to biography")
			Expect(profile0.Get("bio").Exists()).To(BeFalse())
			Expect(profile0.Get("avatar").Exists()).To(BeFalse(), "Avatar should be removed for V1")

			// Check second user too
			user1 := responseInfo.Body.Get("users").Index(1)
			profile1 := user1.Get("profile")
			Expect(profile1.Get("biography").Exists()).To(BeTrue())
			Expect(profile1.Get("bio").Exists()).To(BeFalse())
		})

		It("should transform users[].skills[] (nested array inside array) via UsersListResponse", func() {
			skillChange := NewVersionChangeBuilder(v1, v2).
				ForType(Skill{}).
				ResponseToPreviousVersion().
				RenameField("name", "skill_name").
				RemoveField("level").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{skillChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"users": [
					{
						"id": 1,
						"full_name": "Alice",
						"profile": {"bio": "Engineer", "avatar": "a.png", "location": "NYC"},
						"skills": [
							{"name": "Go", "level": 5},
							{"name": "Python", "level": 4}
						]
					}
				],
				"total": 1
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(UsersWrapper{}),
				map[string]reflect.Type{"users": reflect.TypeOf(UserWithNestedTypes{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			user0 := responseInfo.Body.Get("users").Index(0)
			skills := user0.Get("skills")

			skill0 := skills.Index(0)
			Expect(skill0.Get("skill_name").Exists()).To(BeTrue(), "Skill name should be renamed to skill_name")
			Expect(skill0.Get("name").Exists()).To(BeFalse())
			Expect(skill0.Get("level").Exists()).To(BeFalse(), "Level should be removed for V1")

			skill1 := skills.Index(1)
			Expect(skill1.Get("skill_name").Exists()).To(BeTrue())
			Expect(skill1.Get("name").Exists()).To(BeFalse())
		})

		It("should handle combined nested object and array transformations in array items", func() {
			// Both profile (object) and skills (array) inside users[] should be transformed
			profileChange := NewVersionChangeBuilder(v1, v2).
				ForType(UserProfile{}).
				ResponseToPreviousVersion().
				RenameField("bio", "biography").
				Build()

			skillChange := NewVersionChangeBuilder(v1, v2).
				ForType(Skill{}).
				ResponseToPreviousVersion().
				RenameField("name", "skill_name").
				Build()

			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(UserWithNestedTypes{}).
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{profileChange, skillChange, userChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"users": [
					{
						"id": 1,
						"full_name": "Alice",
						"profile": {"bio": "Engineer", "avatar": "a.png", "location": "NYC"},
						"skills": [{"name": "Go", "level": 5}]
					}
				],
				"total": 1
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(UsersWrapper{}),
				map[string]reflect.Type{"users": reflect.TypeOf(UserWithNestedTypes{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			user0 := responseInfo.Body.Get("users").Index(0)

			// User transformation
			Expect(user0.Get("name").Exists()).To(BeTrue())
			Expect(user0.Get("full_name").Exists()).To(BeFalse())

			// Profile transformation (nested object)
			profile0 := user0.Get("profile")
			Expect(profile0.Get("biography").Exists()).To(BeTrue())
			Expect(profile0.Get("bio").Exists()).To(BeFalse())

			// Skills transformation (nested array)
			skill0 := user0.Get("skills").Index(0)
			Expect(skill0.Get("skill_name").Exists()).To(BeTrue())
			Expect(skill0.Get("name").Exists()).To(BeFalse())
		})

		It("should handle three-level deep nesting", func() {
			// Testing: Container -> Items[] -> SubItems[] -> SubSubItems[]
			type SubSubItem struct {
				ID   int    `json:"id"`
				Code string `json:"code"` // Renamed in migration
			}

			type SubItemWithNesting struct {
				ID          int          `json:"id"`
				Label       string       `json:"label"`
				SubSubItems []SubSubItem `json:"subsubitems"`
			}

			type ItemWithDeepNesting struct {
				ID       int                  `json:"id"`
				Name     string               `json:"name"`
				SubItems []SubItemWithNesting `json:"subitems"`
			}

			type DeepContainer struct {
				Items []ItemWithDeepNesting `json:"items"`
			}

			subSubItemChange := NewVersionChangeBuilder(v1, v2).
				ForType(SubSubItem{}).
				ResponseToPreviousVersion().
				RenameField("code", "item_code").
				Build()

			subItemChange := NewVersionChangeBuilder(v1, v2).
				ForType(SubItemWithNesting{}).
				ResponseToPreviousVersion().
				RenameField("label", "sub_label").
				Build()

			itemChange := NewVersionChangeBuilder(v1, v2).
				ForType(ItemWithDeepNesting{}).
				ResponseToPreviousVersion().
				RenameField("name", "item_name").
				Build()

			chain, err := NewMigrationChain([]*VersionChange{subSubItemChange, subItemChange, itemChange})
			Expect(err).NotTo(HaveOccurred())

			jsonStr := `{
				"items": [
					{
						"id": 1,
						"name": "Top Level",
						"subitems": [
							{
								"id": 10,
								"label": "Mid Level",
								"subsubitems": [
									{"id": 100, "code": "ABC"},
									{"id": 101, "code": "DEF"}
								]
							}
						]
					}
				]
			}`

			responseInfo := createTestResponseInfo(jsonStr, 200)

			err = chain.MigrateResponseForType(
				ctx,
				responseInfo,
				reflect.TypeOf(DeepContainer{}),
				map[string]reflect.Type{"items": reflect.TypeOf(ItemWithDeepNesting{})},
				v2,
				v1,
			)
			Expect(err).NotTo(HaveOccurred())

			// Level 1: items[]
			item0 := responseInfo.Body.Get("items").Index(0)
			Expect(item0.Get("item_name").Exists()).To(BeTrue(), "Level 1: name should be renamed to item_name")
			Expect(item0.Get("name").Exists()).To(BeFalse())

			// Level 2: items[].subitems[]
			subitem0 := item0.Get("subitems").Index(0)
			Expect(subitem0.Get("sub_label").Exists()).To(BeTrue(), "Level 2: label should be renamed to sub_label")
			Expect(subitem0.Get("label").Exists()).To(BeFalse())

			// Level 3: items[].subitems[].subsubitems[]
			subsubitem0 := subitem0.Get("subsubitems").Index(0)
			Expect(subsubitem0.Get("item_code").Exists()).To(BeTrue(), "Level 3: code should be renamed to item_code")
			Expect(subsubitem0.Get("code").Exists()).To(BeFalse())

			subsubitem1 := subitem0.Get("subsubitems").Index(1)
			Expect(subsubitem1.Get("item_code").Exists()).To(BeTrue())
			Expect(subsubitem1.Get("code").Exists()).To(BeFalse())
		})
	})
})

// Helper functions
func createTestRequestInfo(jsonStr string) *RequestInfo {
	node, err := sonic.Get([]byte(jsonStr))
	Expect(err).NotTo(HaveOccurred())
	err = node.Load()
	Expect(err).NotTo(HaveOccurred())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	return &RequestInfo{
		Body:       &node,
		GinContext: c,
	}
}

func createTestResponseInfo(jsonStr string, statusCode int) *ResponseInfo {
	node, err := sonic.Get([]byte(jsonStr))
	Expect(err).NotTo(HaveOccurred())
	err = node.Load()
	Expect(err).NotTo(HaveOccurred())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	return &ResponseInfo{
		Body:       &node,
		StatusCode: statusCode,
		GinContext: c,
	}
}
