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

	Describe("Multi-step migrations for nested arrays", func() {
		type Item struct {
			ID          int      `json:"id"`
			DisplayName string   `json:"display_name"` // V3 field
			Tags        []string `json:"tags"`
			Category    string   `json:"category"` // Added in V2
			Priority    int      `json:"priority"` // Added in V3
		}

		type Container struct {
			Items []Item `json:"items"`
		}

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
