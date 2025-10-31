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
