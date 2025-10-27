package epoch

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MigrationTypes", func() {
	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
	})

	Describe("RequestInfo", func() {
		var (
			c           *gin.Context
			requestInfo *RequestInfo
		)

		BeforeEach(func() {
			req := httptest.NewRequest("GET", "/test?param1=value1&param2=value2", nil)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer token")
			req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
			req.AddCookie(&http.Cookie{Name: "user", Value: "john"})

			recorder := httptest.NewRecorder()
			c, _ = gin.CreateTestContext(recorder)
			c.Request = req

			bodyJSON := `{"name":"test","value":42}`
			bodyNode, _ := sonic.Get([]byte(bodyJSON))
			requestInfo = NewRequestInfo(c, &bodyNode)
		})

		Describe("NewRequestInfo", func() {
			It("should create RequestInfo with all components", func() {
				Expect(requestInfo).NotTo(BeNil())
				Expect(requestInfo.Body).NotTo(BeNil())
				// Check body has expected fields using helper methods
				Expect(requestInfo.HasField("name")).To(BeTrue())
				Expect(requestInfo.HasField("value")).To(BeTrue())

				// Get field values using convenience helper methods
				nameVal, err := requestInfo.GetFieldString("name")
				Expect(err).NotTo(HaveOccurred())
				Expect(nameVal).To(Equal("test"))

				valueVal, err := requestInfo.GetFieldInt("value")
				Expect(err).NotTo(HaveOccurred())
				Expect(valueVal).To(Equal(int64(42)))

				Expect(requestInfo.GinContext).To(Equal(c))
			})

			It("should copy headers correctly", func() {
				Expect(requestInfo.Headers.Get("Content-Type")).To(Equal("application/json"))
				Expect(requestInfo.Headers.Get("Authorization")).To(Equal("Bearer token"))
			})

			It("should copy cookies correctly", func() {
				Expect(requestInfo.Cookies).To(HaveKeyWithValue("session", "abc123"))
				Expect(requestInfo.Cookies).To(HaveKeyWithValue("user", "john"))
			})

			It("should copy query parameters correctly", func() {
				Expect(requestInfo.QueryParams).To(HaveKeyWithValue("param1", "value1"))
				Expect(requestInfo.QueryParams).To(HaveKeyWithValue("param2", "value2"))
			})

			It("should handle empty query parameters", func() {
				req := httptest.NewRequest("GET", "/test", nil)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = req

				requestInfo := NewRequestInfo(c, nil)
				Expect(requestInfo.QueryParams).To(BeEmpty())
			})

			It("should handle multiple values in query parameters", func() {
				req := httptest.NewRequest("GET", "/test?param=value1&param=value2", nil)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = req

				requestInfo := NewRequestInfo(c, nil)
				// Should take the first value when multiple values exist
				Expect(requestInfo.QueryParams).To(HaveKeyWithValue("param", "value1"))
			})
		})
	})

	Describe("ResponseInfo", func() {
		var (
			c            *gin.Context
			responseInfo *ResponseInfo
			recorder     *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			recorder = httptest.NewRecorder()
			c, _ = gin.CreateTestContext(recorder)

			// Set some response headers
			c.Header("Content-Type", "application/json")
			c.Header("X-Custom-Header", "custom-value")

			bodyJSON := `{"id":1,"name":"test"}`
			bodyNode, _ := sonic.Get([]byte(bodyJSON))
			responseInfo = NewResponseInfo(c, &bodyNode)
		})

		Describe("NewResponseInfo", func() {
			It("should create ResponseInfo with all components", func() {
				Expect(responseInfo).NotTo(BeNil())
				Expect(responseInfo.Body).NotTo(BeNil())
				// Check body has expected fields using helper methods
				Expect(responseInfo.HasField("id")).To(BeTrue())
				Expect(responseInfo.HasField("name")).To(BeTrue())

				// Get field values using convenience helper methods
				idVal, err := responseInfo.GetFieldInt("id")
				Expect(err).NotTo(HaveOccurred())
				Expect(idVal).To(Equal(int64(1)))

				nameVal, err := responseInfo.GetFieldString("name")
				Expect(err).NotTo(HaveOccurred())
				Expect(nameVal).To(Equal("test"))

				Expect(responseInfo.GinContext).To(Equal(c))
			})

			It("should copy response headers correctly", func() {
				Expect(responseInfo.Headers.Get("Content-Type")).To(Equal("application/json"))
				Expect(responseInfo.Headers.Get("X-Custom-Header")).To(Equal("custom-value"))
			})

			It("should get status code from writer", func() {
				// The status code should be 200 by default (or 0 if not set)
				Expect(responseInfo.StatusCode).To(BeNumerically(">=", 0))
			})
		})

		Describe("SetCookie", func() {
			It("should set cookie on Gin context", func() {
				cookie := &http.Cookie{
					Name:     "test-cookie",
					Value:    "test-value",
					MaxAge:   3600,
					Path:     "/",
					Domain:   "example.com",
					Secure:   true,
					HttpOnly: true,
				}

				responseInfo.SetCookie(cookie)

				// Verify cookie was set by checking the Set-Cookie header
				setCookieHeader := recorder.Header().Get("Set-Cookie")
				Expect(setCookieHeader).To(ContainSubstring("test-cookie=test-value"))
			})

			It("should handle nil Gin context gracefully", func() {
				responseInfo.GinContext = nil
				cookie := &http.Cookie{Name: "test", Value: "value"}

				// Should not panic
				Expect(func() {
					responseInfo.SetCookie(cookie)
				}).NotTo(Panic())
			})
		})

		Describe("TransformNestedArrays", func() {
			It("should transform fields in nested arrays within objects", func() {
				// Create response with nested arrays
				bodyJSON := `{
					"items": [
						{"name": "Item 1", "value": 100},
						{"name": "Item 2", "value": 200}
					],
					"metadata": {
						"total": 2
					}
				}`
				bodyNode, _ := sonic.Get([]byte(bodyJSON))
				testResponseInfo := NewResponseInfo(c, &bodyNode)

				// Transform function that renames "name" to "title"
				transformer := func(node *ast.Node) error {
					if node.Get("name").Exists() {
						nameValue := node.Get("name")
						value, _ := nameValue.String()
						err := SetNodeField(node, "title", value)
						if err != nil {
							return err
						}
						return DeleteNodeField(node, "name")
					}
					return nil
				}

				err := testResponseInfo.TransformNestedArrays(transformer)
				Expect(err).NotTo(HaveOccurred())

				// Verify transformations were applied
				items := testResponseInfo.Body.Get("items")
				Expect(items.Exists()).To(BeTrue())

				firstItem := items.Index(0)
				Expect(firstItem.Get("title").Exists()).To(BeTrue())
				Expect(firstItem.Get("name").Exists()).To(BeFalse())

				titleValue, _ := firstItem.Get("title").String()
				Expect(titleValue).To(Equal("Item 1"))
			})

			It("should handle deeply nested arrays", func() {
				bodyJSON := `{
					"categories": [
						{
							"name": "Category 1",
							"items": [
								{
									"name": "Item 1",
									"details": [
										{"name": "Detail 1"}
									]
								}
							]
						}
					]
				}`
				bodyNode, _ := sonic.Get([]byte(bodyJSON))
				testResponseInfo := NewResponseInfo(c, &bodyNode)

				// Transform function that adds "processed" field
				transformer := func(node *ast.Node) error {
					return SetNodeField(node, "processed", true)
				}

				err := testResponseInfo.TransformNestedArrays(transformer)
				Expect(err).NotTo(HaveOccurred())

				// Verify transformations at all levels
				categories := testResponseInfo.Body.Get("categories")
				category := categories.Index(0)
				Expect(category.Get("processed").Exists()).To(BeTrue())

				items := category.Get("items")
				item := items.Index(0)
				Expect(item.Get("processed").Exists()).To(BeTrue())

				details := item.Get("details")
				detail := details.Index(0)
				Expect(detail.Get("processed").Exists()).To(BeTrue())
			})

			It("should not process non-object bodies", func() {
				// Test with array body (should not process)
				bodyJSON := `[{"name": "Item 1"}, {"name": "Item 2"}]`
				bodyNode, _ := sonic.Get([]byte(bodyJSON))
				testResponseInfo := NewResponseInfo(c, &bodyNode)

				transformerCalled := false
				transformer := func(node *ast.Node) error {
					transformerCalled = true
					return nil
				}

				err := testResponseInfo.TransformNestedArrays(transformer)
				Expect(err).NotTo(HaveOccurred())
				Expect(transformerCalled).To(BeFalse())
			})

			It("should handle nil body gracefully", func() {
				testResponseInfo := &ResponseInfo{Body: nil}

				transformer := func(node *ast.Node) error {
					return nil
				}

				err := testResponseInfo.TransformNestedArrays(transformer)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should propagate transformer errors", func() {
				bodyJSON := `{
					"items": [
						{"name": "Item 1"}
					]
				}`
				bodyNode, _ := sonic.Get([]byte(bodyJSON))
				testResponseInfo := NewResponseInfo(c, &bodyNode)

				transformer := func(node *ast.Node) error {
					return errors.New("transformation failed")
				}

				err := testResponseInfo.TransformNestedArrays(transformer)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("transformation failed"))
			})
		})
	})

	Describe("AlterRequestInstruction", func() {
		It("should create instruction with schemas", func() {
			transformer := func(req *RequestInfo) error { return nil }
			instruction := &AlterRequestInstruction{
				Schemas:     []interface{}{TestUser{}},
				Transformer: transformer,
			}

			Expect(instruction.Schemas).To(HaveLen(1))
			Expect(instruction.Transformer).NotTo(BeNil())
		})

		It("should create instruction with path and methods", func() {
			transformer := func(req *RequestInfo) error { return nil }
			instruction := &AlterRequestInstruction{
				Path:        "/users",
				Methods:     []string{"GET", "POST"},
				Transformer: transformer,
			}

			Expect(instruction.Path).To(Equal("/users"))
			Expect(instruction.Methods).To(ContainElements("GET", "POST"))
		})
	})

	Describe("AlterResponseInstruction", func() {
		It("should create instruction with schemas", func() {
			transformer := func(resp *ResponseInfo) error { return nil }
			instruction := &AlterResponseInstruction{
				Schemas:           []interface{}{TestUser{}},
				MigrateHTTPErrors: true,
				Transformer:       transformer,
			}

			Expect(instruction.Schemas).To(HaveLen(1))
			Expect(instruction.MigrateHTTPErrors).To(BeTrue())
			Expect(instruction.Transformer).NotTo(BeNil())
		})

		It("should create instruction with path and methods", func() {
			transformer := func(resp *ResponseInfo) error { return nil }
			instruction := &AlterResponseInstruction{
				Path:              "/users",
				Methods:           []string{"GET", "POST"},
				MigrateHTTPErrors: false,
				Transformer:       transformer,
			}

			Expect(instruction.Path).To(Equal("/users"))
			Expect(instruction.Methods).To(ContainElements("GET", "POST"))
			Expect(instruction.MigrateHTTPErrors).To(BeFalse())
		})

	})
})
