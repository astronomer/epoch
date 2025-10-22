package epoch

import (
	"context"
	"fmt"
	"net/http/httptest"

	"github.com/bytedance/sonic"
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
			requestInst := &AlterRequestInstruction{
				Path: "/test",
			}

			change := NewVersionChange("Test change", v1, v2, requestInst)
			Expect(change).NotTo(BeNil())
			Expect(change.Description()).To(Equal("Test change"))
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
			bodyNode, _ := sonic.Get([]byte(`{"name":"test"}`))
			requestInfo = NewRequestInfo(c, &bodyNode)
		})

		Context("with path-based instructions", func() {
			It("should apply request transformations", func() {
				transformCalled := false
				requestInst := &AlterRequestInstruction{
					Path: "/test",
					Transformer: func(req *RequestInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, requestInst)

				// Create context with proper path
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil)
				bodyNode, _ := sonic.Get([]byte(`{}`))
				requestInfo = NewRequestInfo(c, &bodyNode)

				err := change.MigrateRequest(ctx, requestInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})

			It("should handle transformation errors", func() {
				requestInst := &AlterRequestInstruction{
					Path: "/test",
					Transformer: func(req *RequestInfo) error {
						return fmt.Errorf("transformation failed")
					},
				}

				change = NewVersionChange("Test change", v1, v2, requestInst)

				// Create context with proper path
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil)
				bodyNode, _ := sonic.Get([]byte(`{}`))
				requestInfo = NewRequestInfo(c, &bodyNode)

				err := change.MigrateRequest(ctx, requestInfo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("request path migration failed"))
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

				// Create a test context with proper request path
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil)
				bodyNode, _ := sonic.Get([]byte(`{}`))
				requestInfo = NewRequestInfo(c, &bodyNode)

				err := change.MigrateRequest(ctx, requestInfo)
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
			bodyNode, _ := sonic.Get([]byte(`{"id":1}`))
			responseInfo = NewResponseInfo(c, &bodyNode)
			responseInfo.StatusCode = 200
		})

		Context("with path-based instructions", func() {
			It("should apply response transformations", func() {
				transformCalled := false
				responseInst := &AlterResponseInstruction{
					Path: "/test",
					Transformer: func(resp *ResponseInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, responseInst)

				// Create context with proper path
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil)
				bodyNode, _ := sonic.Get([]byte(`{"id":1}`))
				responseInfo = NewResponseInfo(c, &bodyNode)
				responseInfo.StatusCode = 200

				err := change.MigrateResponse(ctx, responseInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})

			It("should skip error responses when MigrateHTTPErrors is false", func() {
				transformCalled := false

				responseInst := &AlterResponseInstruction{
					Path:              "/test",
					MigrateHTTPErrors: false,
					Transformer: func(resp *ResponseInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, responseInst)

				// Create context with proper path and error status
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil)
				bodyNode, _ := sonic.Get([]byte(`{"id":1}`))
				responseInfo = NewResponseInfo(c, &bodyNode)
				responseInfo.StatusCode = 400 // Error status

				err := change.MigrateResponse(ctx, responseInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeFalse())
			})

			It("should migrate error responses when MigrateHTTPErrors is true", func() {
				transformCalled := false

				responseInst := &AlterResponseInstruction{
					Path:              "/test",
					MigrateHTTPErrors: true,
					Transformer: func(resp *ResponseInfo) error {
						transformCalled = true
						return nil
					},
				}

				change = NewVersionChange("Test change", v1, v2, responseInst)

				// Create context with proper path and error status
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil)
				bodyNode, _ := sonic.Get([]byte(`{"id":1}`))
				responseInfo = NewResponseInfo(c, &bodyNode)
				responseInfo.StatusCode = 400 // Error status

				err := change.MigrateResponse(ctx, responseInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformCalled).To(BeTrue())
			})
		})
	})

	// NOTE: BindRouteToRequestMigrations and BindRouteToResponseMigrations were removed
	// as they were dead code. The new path-based routing uses direct path matching
	// via matchesPath() in MigrateRequest and MigrateResponse.
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

		chain, _ = NewMigrationChain([]*VersionChange{change1, change2})
		ctx = context.Background()
		gin.SetMode(gin.TestMode)
	})

	Describe("NewMigrationChain", func() {
		It("should create a migration chain", func() {
			Expect(chain).NotTo(BeNil())
			Expect(chain.GetChanges()).To(HaveLen(2))
		})

		It("should detect simple cycles", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Create a cycle: v1 -> v2 -> v1
			changes := []*VersionChange{
				NewVersionChange("v1 to v2", v1, v2),
				NewVersionChange("v2 to v1", v2, v1), // Cycle!
			}

			_, err := NewMigrationChain(changes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cycle detected"))
		})

		It("should detect complex cycles", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")
			v4, _ := NewDateVersion("2025-06-01")

			// Create a cycle: v2 -> v3 -> v4 -> v2
			changes := []*VersionChange{
				NewVersionChange("v1 to v2", v1, v2),
				NewVersionChange("v2 to v3", v2, v3),
				NewVersionChange("v3 to v4", v3, v4),
				NewVersionChange("v4 to v2", v4, v2), // Cycle back to v2!
			}

			_, err := NewMigrationChain(changes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cycle detected"))
		})

		It("should allow acyclic graphs", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")

			// No cycles - linear chain
			changes := []*VersionChange{
				NewVersionChange("v1 to v2", v1, v2),
				NewVersionChange("v2 to v3", v2, v3),
			}

			chain, err := NewMigrationChain(changes)
			Expect(err).NotTo(HaveOccurred())
			Expect(chain).NotTo(BeNil())
			Expect(chain.GetChanges()).To(HaveLen(2))
		})

		It("should allow multiple disconnected chains", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")
			v4, _ := NewDateVersion("2025-06-01")

			// Two separate chains: v1 -> v2 and v3 -> v4
			changes := []*VersionChange{
				NewVersionChange("v1 to v2", v1, v2),
				NewVersionChange("v3 to v4", v3, v4),
			}

			chain, err := NewMigrationChain(changes)
			Expect(err).NotTo(HaveOccurred())
			Expect(chain).NotTo(BeNil())
			Expect(chain.GetChanges()).To(HaveLen(2))
		})

		It("should detect self-loops", func() {
			v1, _ := NewDateVersion("2024-01-01")

			// Self-loop: v1 -> v1
			changes := []*VersionChange{
				NewVersionChange("v1 to v1", v1, v1), // Self-loop!
			}

			_, err := NewMigrationChain(changes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cycle detected"))
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
			err := chain.MigrateRequest(ctx, requestInfo, v1, v3)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for unknown starting version", func() {
			unknownVersion, _ := NewSemverVersion("0.5.0")
			err := chain.MigrateRequest(ctx, requestInfo, unknownVersion, v2)
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
			err := chain.MigrateResponse(ctx, responseInfo, v3, v1)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for unknown starting version", func() {
			unknownVersion, _ := NewSemverVersion("4.0.0")
			err := chain.MigrateResponse(ctx, responseInfo, unknownVersion, v1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no migration path found"))
		})

		Context("with multiple changes at the same version level", func() {
			var multiChain *MigrationChain
			var userMigrationApplied, productMigrationApplied, orderMigrationApplied bool

			// Define realistic schema types for testing
			type User struct {
				ID       int
				FullName string
				Email    string
				Phone    string
			}

			type Product struct {
				ID          int
				Name        string
				Price       float64
				Description string
				Currency    string
			}

			type Order struct {
				ID        int
				UserID    int
				ProductID int
				Quantity  int
				CreatedAt string
			}

			BeforeEach(func() {
				userMigrationApplied = false
				productMigrationApplied = false
				orderMigrationApplied = false

				// Create multiple changes all from v2 to v3 (like User, Product, Order migrations)
				// Simulate realistic backward migrations (v3 -> v2)

				// User migration: v3 has "full_name" and "phone", v2 has "name" without phone
				userChange := NewVersionChange("User v2->v3: Add phone, rename name to full_name", v2, v3,
					&AlterResponseInstruction{
						Path: "/test",
						Transformer: func(resp *ResponseInfo) error {
							if resp.Body != nil {
								// Backward migration: v3 -> v2
								if fullNameNode := resp.GetField("full_name"); fullNameNode != nil && fullNameNode.Exists() {
									fullNameStr, _ := fullNameNode.String()
									resp.SetField("name", fullNameStr)
									resp.DeleteField("full_name")
								}
								resp.DeleteField("phone") // v2 doesn't have phone
								userMigrationApplied = true
							}
							return nil
						},
					},
				)

				// Product migration: v3 has "description" and "currency", v2 has "desc" without currency
				productChange := NewVersionChange("Product v2->v3: Add currency, rename desc to description", v2, v3,
					&AlterResponseInstruction{
						Path: "/test",
						Transformer: func(resp *ResponseInfo) error {
							if resp.Body != nil {
								// Backward migration: v3 -> v2
								if descriptionNode := resp.GetField("description"); descriptionNode != nil && descriptionNode.Exists() {
									descriptionStr, _ := descriptionNode.String()
									resp.SetField("desc", descriptionStr)
									resp.DeleteField("description")
								}
								resp.DeleteField("currency") // v2 doesn't have currency
								productMigrationApplied = true
							}
							return nil
						},
					},
				)

				// Order migration: v3 has "created_at", v2 doesn't
				orderChange := NewVersionChange("Order v2->v3: Add created_at timestamp", v2, v3,
					&AlterResponseInstruction{
						Path: "/test",
						Transformer: func(resp *ResponseInfo) error {
							if resp.Body != nil {
								// Backward migration: v3 -> v2
								resp.DeleteField("created_at") // v2 doesn't have created_at
								orderMigrationApplied = true
							}
							return nil
						},
					},
				)

				multiChain, _ = NewMigrationChain([]*VersionChange{userChange, productChange, orderChange})
			})

			It("should apply ALL changes with the same FromVersion when migrating backward", func() {
				// Setup response data representing a v3 response with all fields
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("GET", "/test", nil) // Add proper request with path
				responseJSON := `{"id":1,"full_name":"Alice Johnson","email":"alice@example.com","phone":"+1-555-0100","product_id":100,"name":"Laptop","price":999.99,"description":"High-performance laptop","currency":"USD","order_id":1000,"user_id":1,"quantity":2,"created_at":"2024-01-01T00:00:00Z"}`
				responseNode, _ := sonic.Get([]byte(responseJSON))
				responseInfo := NewResponseInfo(c, &responseNode)

				// Migrate from v3 to v2 - should apply all three v2->v3 changes in reverse
				err := multiChain.MigrateResponse(ctx, responseInfo, v3, v2)
				Expect(err).NotTo(HaveOccurred())

				// Verify ALL three transformations were applied
				Expect(userMigrationApplied).To(BeTrue(), "User migration should be applied")
				Expect(productMigrationApplied).To(BeTrue(), "Product migration should be applied")
				Expect(orderMigrationApplied).To(BeTrue(), "Order migration should be applied")

				// Verify the response body has all v2 transformations
				// User fields: should have "name" instead of "full_name", no "phone"
				Expect(responseInfo.HasField("name")).To(BeTrue())
				nameStr, err := responseInfo.GetFieldString("name")
				Expect(err).NotTo(HaveOccurred())
				Expect(nameStr).To(Equal("Alice Johnson"), "full_name should be renamed to name")

				Expect(responseInfo.HasField("full_name")).To(BeFalse(), "full_name should not exist in v2")
				Expect(responseInfo.HasField("phone")).To(BeFalse(), "phone should not exist in v2")

				Expect(responseInfo.HasField("email")).To(BeTrue())
				emailStr, err := responseInfo.GetFieldString("email")
				Expect(err).NotTo(HaveOccurred())
				Expect(emailStr).To(Equal("alice@example.com"), "email should remain")

				// Product fields: should have "desc" instead of "description", no "currency"
				Expect(responseInfo.HasField("desc")).To(BeTrue())
				descStr, err := responseInfo.GetFieldString("desc")
				Expect(err).NotTo(HaveOccurred())
				Expect(descStr).To(Equal("High-performance laptop"), "description should be renamed to desc")

				Expect(responseInfo.HasField("description")).To(BeFalse(), "description should not exist in v2")
				Expect(responseInfo.HasField("currency")).To(BeFalse(), "currency should not exist in v2")

				// Order fields: should not have "created_at"
				Expect(responseInfo.HasField("created_at")).To(BeFalse(), "created_at should not exist in v2")

				Expect(responseInfo.HasField("quantity")).To(BeTrue())
				quantityInt, err := responseInfo.GetFieldInt("quantity")
				Expect(err).NotTo(HaveOccurred())
				Expect(quantityInt).To(Equal(int64(2)), "quantity should remain")
			})

			It("should collect multiple changes at the same version level via GetMigrationPath", func() {
				// GetMigrationPath should return all 3 changes when going from v3 to v2
				path := multiChain.GetMigrationPath(v3, v2)
				Expect(path).To(HaveLen(3), "should include all changes from v2->v3")

				// Verify all changes are included by checking their descriptions
				descriptions := []string{}
				for _, change := range path {
					descriptions = append(descriptions, change.Description())
				}

				Expect(descriptions).To(ContainElement("User v2->v3: Add phone, rename name to full_name"),
					"User migration should be in the path")
				Expect(descriptions).To(ContainElement("Product v2->v3: Add currency, rename desc to description"),
					"Product migration should be in the path")
				Expect(descriptions).To(ContainElement("Order v2->v3: Add created_at timestamp"),
					"Order migration should be in the path")
			})
		})
	})
})
