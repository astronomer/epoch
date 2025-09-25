package cadwyn

import (
	"context"
	"fmt"
	"net/http/httptest"
	"reflect"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionChange", func() {
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

		It("should create a version change with instructions", func() {
			schemaInst := &SchemaInstruction{
				Name: "test_field",
				Type: "field_added",
			}

			change := NewVersionChange("Test change", v1, v2, schemaInst)
			Expect(change).NotTo(BeNil())
			Expect(change.GetSchemaInstructions()).To(HaveLen(1))
		})

		It("should organize different instruction types", func() {
			schemaInst := &SchemaInstruction{Name: "field1", Type: "field_added"}
			enumInst := &EnumInstruction{Type: "had_members"}
			endpointInst := &EndpointInstruction{Path: "/test", Type: "endpoint_added"}

			change := NewVersionChange("Test change", v1, v2, schemaInst, enumInst, endpointInst)

			Expect(change.GetSchemaInstructions()).To(HaveLen(1))
			Expect(change.GetEnumInstructions()).To(HaveLen(1))
			Expect(change.GetEndpointInstructions()).To(HaveLen(1))
		})
	})

	Describe("SetHiddenFromChangelog", func() {
		It("should set and get hidden status", func() {
			change := NewVersionChange("Test change", v1, v2)
			Expect(change.IsHiddenFromChangelog()).To(BeFalse())

			change.SetHiddenFromChangelog(true)
			Expect(change.IsHiddenFromChangelog()).To(BeTrue())
		})
	})

	Describe("MigrateRequest", func() {
		var (
			change      *VersionChange
			requestInfo *RequestInfo
		)

		BeforeEach(func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			requestInfo = NewRequestInfo(c, map[string]interface{}{"name": "test"})
		})

		Context("with schema-based instructions", func() {
			It("should apply request transformations", func() {
				transformCalled := false
				requestInst := &AlterRequestInstruction{
					Schemas: []interface{}{TestUser{}},
					Transformer: func(req *RequestInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, requestInst)

				err := change.MigrateRequest(ctx, requestInfo, reflect.TypeOf(TestUser{}), 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})

			It("should handle transformation errors", func() {
				requestInst := &AlterRequestInstruction{
					Schemas: []interface{}{TestUser{}},
					Transformer: func(req *RequestInfo) error {
						return fmt.Errorf("transformation failed")
					},
				}

				change = NewVersionChange("Test change", v1, v2, requestInst)

				err := change.MigrateRequest(ctx, requestInfo, reflect.TypeOf(TestUser{}), 0)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("request schema migration failed"))
			})
		})

		Context("with path-based instructions", func() {
			It("should apply path-based transformations", func() {
				transformCalled := false
				requestInst := &AlterRequestInstruction{
					Path:    "/test",
					Methods: []string{"GET"},
					Transformer: func(req *RequestInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, requestInst)
				change.BindRouteToRequestMigrations(1, "/test")

				err := change.MigrateRequest(ctx, requestInfo, nil, 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})
		})
	})

	Describe("MigrateResponse", func() {
		var (
			change       *VersionChange
			responseInfo *ResponseInfo
		)

		BeforeEach(func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			responseInfo = NewResponseInfo(c, map[string]interface{}{"id": 1})
			responseInfo.StatusCode = 200
		})

		Context("with schema-based instructions", func() {
			It("should apply response transformations", func() {
				transformCalled := false
				responseInst := &AlterResponseInstruction{
					Schemas: []interface{}{TestUser{}},
					Transformer: func(resp *ResponseInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, responseInst)

				err := change.MigrateResponse(ctx, responseInfo, reflect.TypeOf(TestUser{}), 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})

			It("should skip error responses when MigrateHTTPErrors is false", func() {
				transformCalled := false
				responseInfo.StatusCode = 400 // Error status

				responseInst := &AlterResponseInstruction{
					Schemas:           []interface{}{TestUser{}},
					MigrateHTTPErrors: false,
					Transformer: func(resp *ResponseInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, responseInst)

				err := change.MigrateResponse(ctx, responseInfo, reflect.TypeOf(TestUser{}), 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeFalse())
			})

			It("should migrate error responses when MigrateHTTPErrors is true", func() {
				transformCalled := false
				responseInfo.StatusCode = 400 // Error status

				responseInst := &AlterResponseInstruction{
					Schemas:           []interface{}{TestUser{}},
					MigrateHTTPErrors: true,
					Transformer: func(resp *ResponseInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, responseInst)

				err := change.MigrateResponse(ctx, responseInfo, reflect.TypeOf(TestUser{}), 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})
		})
	})

	Describe("BindRouteToRequestMigrations", func() {
		It("should bind path instructions to route ID", func() {
			requestInst := &AlterRequestInstruction{
				Path:        "/test",
				Methods:     []string{"GET"},
				Transformer: func(req *RequestInfo) error { return nil },
			}

			change := NewVersionChange("Test change", v1, v2, requestInst)
			change.BindRouteToRequestMigrations(1, "/test")

			// The binding is internal, so we test by triggering migration
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			requestInfo := NewRequestInfo(c, nil)

			err := change.MigrateRequest(ctx, requestInfo, nil, 1)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("BindRouteToResponseMigrations", func() {
		It("should bind path instructions to route ID", func() {
			responseInst := &AlterResponseInstruction{
				Path:        "/test",
				Methods:     []string{"GET"},
				Transformer: func(resp *ResponseInfo) error { return nil },
			}

			change := NewVersionChange("Test change", v1, v2, responseInst)
			change.BindRouteToResponseMigrations(1, "/test")

			// The binding is internal, so we test by triggering migration
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			responseInfo := NewResponseInfo(c, nil)

			err := change.MigrateResponse(ctx, responseInfo, nil, 1)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("MigrationChain", func() {
	var (
		v1, v2, v3 *Version
		change1    *VersionChange
		change2    *VersionChange
		chain      *MigrationChain
		ctx        context.Context
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		v3, _ = NewSemverVersion("3.0.0")

		change1 = NewVersionChange("Change 1->2", v1, v2)
		change2 = NewVersionChange("Change 2->3", v2, v3)

		chain = NewMigrationChain([]*VersionChange{change1, change2})
		ctx = context.Background()
		gin.SetMode(gin.TestMode)
	})

	Describe("NewMigrationChain", func() {
		It("should create a migration chain", func() {
			Expect(chain).NotTo(BeNil())
			Expect(chain.GetChanges()).To(HaveLen(2))
		})
	})

	Describe("AddChange", func() {
		It("should add a new change to the chain", func() {
			v4, _ := NewSemverVersion("4.0.0")
			change3 := NewVersionChange("Change 3->4", v3, v4)

			chain.AddChange(change3)
			Expect(chain.GetChanges()).To(HaveLen(3))
		})
	})

	Describe("GetMigrationPath", func() {
		It("should return changes for forward migration", func() {
			path := chain.GetMigrationPath(v1, v3)
			Expect(path).To(HaveLen(2))
		})

		It("should return changes for backward migration", func() {
			path := chain.GetMigrationPath(v3, v1)
			Expect(path).To(HaveLen(2))
		})

		It("should return empty path for same version", func() {
			path := chain.GetMigrationPath(v1, v1)
			Expect(path).To(HaveLen(0))
		})
	})

	Describe("MigrateRequest", func() {
		var requestInfo *RequestInfo

		BeforeEach(func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			requestInfo = NewRequestInfo(c, nil)
		})

		It("should apply migrations in sequence", func() {
			err := chain.MigrateRequest(ctx, requestInfo, v1, v3, nil, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for unknown starting version", func() {
			unknownVersion, _ := NewSemverVersion("0.5.0")
			err := chain.MigrateRequest(ctx, requestInfo, unknownVersion, v2, nil, 0)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no migration path found"))
		})
	})

	Describe("MigrateResponse", func() {
		var responseInfo *ResponseInfo

		BeforeEach(func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			responseInfo = NewResponseInfo(c, nil)
		})

		It("should apply reverse migrations", func() {
			err := chain.MigrateResponse(ctx, responseInfo, v3, v1, nil, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for unknown starting version", func() {
			unknownVersion, _ := NewSemverVersion("4.0.0")
			err := chain.MigrateResponse(ctx, responseInfo, unknownVersion, v1, nil, 0)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no migration path found"))
		})
	})
})
