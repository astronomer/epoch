package epoch

import (
	"net/http/httptest"
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test types for type-based migrations
type BuilderTestUser struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Status   string `json:"status"`
}

type BuilderTestProduct struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description,omitempty"`
	Currency    string  `json:"currency,omitempty"`
}

var _ = Describe("SchemaVersionChangeBuilder", func() {
	var (
		v1, v2 *Version
	)

	BeforeEach(func() {
		var err error
		v1, err = NewDateVersion("2024-01-01")
		Expect(err).NotTo(HaveOccurred())

		v2, err = NewDateVersion("2024-06-01")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Cadwyn-Style API", func() {
		It("should create migration with clear direction semantics", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(BuilderTestUser{}).
				RequestToNextVersion().
				AddField("email", "default@example.com"). // Add email when going to v2
				ResponseToPreviousVersion().
				RemoveField("email"). // Remove email from responses for v1 clients
				Build()

			Expect(migration).NotTo(BeNil())
			Expect(migration.Description()).To(Equal("Add email field to User"))
			Expect(migration.FromVersion()).To(Equal(v1))
			Expect(migration.ToVersion()).To(Equal(v2))
		})

		It("should support multiple schemas in one migration", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Update User and Product schemas").
				ForType(BuilderTestUser{}).
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				ForType(BuilderTestProduct{}).
				ResponseToPreviousVersion().
				RemoveField("currency"). // Remove new field for v1 clients
				Build()

			Expect(migration).NotTo(BeNil())
			Expect(migration.Description()).To(Equal("Update User and Product schemas"))
		})

		It("should support global custom transformers", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Global custom operations").
				CustomRequest(func(req *RequestInfo) error {
					// Custom logic for all requests
					return nil
				}).
				CustomResponse(func(resp *ResponseInfo) error {
					// Custom logic for all responses
					return nil
				}).
				Build()

			Expect(migration).NotTo(BeNil())
		})

		It("should require at least one schema or custom transformer", func() {
			Expect(func() {
				NewVersionChangeBuilder(v1, v2).
					Description("Empty migration").
					Build()
			}).To(Panic())
		})

		It("should generate default description if none provided", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				ForType(BuilderTestUser{}).
				RequestToNextVersion().
				AddField("email", "test@example.com").
				Build()

			Expect(migration.Description()).To(Equal("Migration from 2024-01-01 to 2024-06-01"))
		})
	})

	Describe("Direction-Specific Operations", func() {
		var testNode *ast.Node

		BeforeEach(func() {
			jsonData := `{
				"id": 1,
				"name": "John Doe",
				"full_name": "John Doe",
				"email": "john@example.com",
				"phone": "+1-555-0100",
				"status": "active"
			}`
			node, err := sonic.Get([]byte(jsonData))
			Expect(err).NotTo(HaveOccurred())
			err = node.Load()
			Expect(err).NotTo(HaveOccurred())
			testNode = &node
		})

		It("should apply RequestToNextVersion operations correctly", func() {
			migration := NewVersionChangeBuilder(v1, v2). // v1→v2 migration
									ForType(BuilderTestUser{}).
									RequestToNextVersion().
									AddField("created_at", "2024-01-01").
									RenameField("name", "full_name").
									Build()

			// Create a mock RequestInfo
			requestInfo := &RequestInfo{Body: testNode}

			// Apply the migration (should use RequestToNextVersion operations)
			instructions := migration.instructionsToMigrateToPreviousVersion
			for _, instruction := range instructions {
				if reqInst, ok := instruction.(*AlterRequestInstruction); ok {
					err := reqInst.Transformer(requestInfo)
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Verify transformations
			createdAtNode := testNode.Get("created_at")
			Expect(createdAtNode.Exists()).To(BeTrue())

			nameNode := testNode.Get("name")
			Expect(nameNode.Exists()).To(BeFalse())

			fullNameNode := testNode.Get("full_name")
			Expect(fullNameNode.Exists()).To(BeTrue())
		})

		It("should apply ResponseToPreviousVersion operations correctly", func() {
			migration := NewVersionChangeBuilder(v2, v1). // v2→v1 migration
									ForType(BuilderTestUser{}).
									ResponseToPreviousVersion().
									RemoveField("email").
									AddField("legacy_field", "legacy_value").
									Build()

			// Create a mock ResponseInfo
			responseInfo := &ResponseInfo{Body: testNode, StatusCode: 200}

			// Apply the migration (should use ResponseToPreviousVersion operations)
			instructions := migration.instructionsToMigrateToPreviousVersion
			for _, instruction := range instructions {
				if respInst, ok := instruction.(*AlterResponseInstruction); ok {
					err := respInst.Transformer(responseInfo)
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Verify transformations
			emailNode := testNode.Get("email")
			Expect(emailNode.Exists()).To(BeFalse())

			legacyNode := testNode.Get("legacy_field")
			Expect(legacyNode.Exists()).To(BeTrue())
		})
	})

	Describe("Builder Fluency", func() {
		It("should allow chaining between different direction builders", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Complex chaining example").
				ForType(BuilderTestUser{}).
				RequestToNextVersion().
				AddField("email", "default@example.com").
				ResponseToPreviousVersion().
				RemoveField("phone").
				ForType(BuilderTestProduct{}).
				ResponseToPreviousVersion().
				RemoveField("currency").
				Build()

			Expect(migration).NotTo(BeNil())
			Expect(migration.Description()).To(Equal("Complex chaining example"))
		})

		It("should allow returning to schema builder from direction builders", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				ForType(BuilderTestProduct{}). // Should return to schema builder
				ResponseToPreviousVersion().
				RemoveField("description").
				Build()

			Expect(migration).NotTo(BeNil())
		})
	})

	Describe("Error Transformation Functions", func() {
		Describe("toPascalCaseString", func() {
			It("should convert snake_case to PascalCase", func() {
				Expect(toPascalCaseString("user_name")).To(Equal("UserName"))
				Expect(toPascalCaseString("first_name")).To(Equal("FirstName"))
				Expect(toPascalCaseString("email_address")).To(Equal("EmailAddress"))
			})

			It("should handle single words", func() {
				Expect(toPascalCaseString("name")).To(Equal("Name"))
				Expect(toPascalCaseString("email")).To(Equal("Email"))
			})

			It("should handle empty strings", func() {
				Expect(toPascalCaseString("")).To(Equal(""))
			})

			It("should handle common API naming conventions", func() {
				Expect(toPascalCaseString("user_id")).To(Equal("UserId"))
				Expect(toPascalCaseString("api_key")).To(Equal("ApiKey"))
			})

			It("should handle multiple underscores", func() {
				Expect(toPascalCaseString("very_long_field_name")).To(Equal("VeryLongFieldName"))
			})
		})

		Describe("replaceFieldNamesInErrorString", func() {
			It("should replace field names in simple error messages", func() {
				fieldMapping := map[string]string{
					"better_new_name": "name",
				}
				input := "Field better_new_name is required"
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(ContainSubstring("name"))
				Expect(output).NotTo(ContainSubstring("better_new_name"))
			})

			It("should replace PascalCase field names", func() {
				fieldMapping := map[string]string{
					"better_new_name": "name",
				}
				input := "Field BetterNewName is required"
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(ContainSubstring("Name"))
				Expect(output).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should replace quoted field names", func() {
				fieldMapping := map[string]string{
					"better_new_name": "old_name",
				}
				input := "Missing field 'better_new_name'"
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(ContainSubstring("'old_name'"))
				Expect(output).NotTo(ContainSubstring("'better_new_name'"))
			})

			It("should replace double-quoted field names", func() {
				fieldMapping := map[string]string{
					"new_field": "old_field",
				}
				input := `Field "new_field" is invalid`
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(ContainSubstring(`"old_field"`))
				Expect(output).NotTo(ContainSubstring(`"new_field"`))
			})

			It("should handle Gin validation error format", func() {
				fieldMapping := map[string]string{
					"better_new_name": "name",
				}
				input := "Key: 'User.BetterNewName' Error:Field validation for 'BetterNewName' failed"
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(ContainSubstring("Key: 'User.Name'"))
				Expect(output).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should handle multiple field replacements", func() {
				fieldMapping := map[string]string{
					"new_email":   "email",
					"new_address": "address",
				}
				input := "Fields new_email and new_address are required"
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(ContainSubstring("email"))
				Expect(output).To(ContainSubstring("address"))
				Expect(output).NotTo(ContainSubstring("new_email"))
				Expect(output).NotTo(ContainSubstring("new_address"))
			})

			It("should return unchanged string when no mappings match", func() {
				fieldMapping := map[string]string{
					"field1": "field2",
				}
				input := "This message has no matching fields"
				output := replaceFieldNamesInErrorString(input, fieldMapping)
				Expect(output).To(Equal(input))
			})
		})

		Describe("transformStringsInNode", func() {
			It("should transform string fields in simple objects", func() {
				jsonData := `{"error": "Field BetterNewName is required"}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformStringsInNode(&node, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				errorNode := node.Get("error")
				errorStr, _ := errorNode.String()
				Expect(errorStr).To(ContainSubstring("Name"))
				Expect(errorStr).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should transform nested object string fields", func() {
				jsonData := `{
					"error": {
						"message": "Field BetterNewName is required",
						"details": "The BetterNewName field cannot be empty"
					}
				}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformStringsInNode(&node, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				messageNode := node.Get("error").Get("message")
				messageStr, _ := messageNode.String()
				Expect(messageStr).To(ContainSubstring("Name"))

				detailsNode := node.Get("error").Get("details")
				detailsStr, _ := detailsNode.String()
				Expect(detailsStr).To(ContainSubstring("Name"))
			})

			It("should transform strings in arrays", func() {
				jsonData := `{"fields": ["BetterNewName", "OtherField"]}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformStringsInNode(&node, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				fieldsNode := node.Get("fields")
				Expect(fieldsNode.Exists()).To(BeTrue())

				// Check that array was transformed
				raw, _ := fieldsNode.Raw()
				Expect(raw).To(ContainSubstring("Name"))
				Expect(raw).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should handle RFC 7807 Problem Details format", func() {
				jsonData := `{
					"type": "https://example.com/probs/validation-error",
					"title": "Validation Error",
					"status": 400,
					"detail": "The field BetterNewName is required",
					"instance": "/api/users"
				}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformStringsInNode(&node, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				detailNode := node.Get("detail")
				detailStr, _ := detailNode.String()
				Expect(detailStr).To(ContainSubstring("Name"))
				Expect(detailStr).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should preserve non-string fields", func() {
				jsonData := `{
					"message": "Field BetterNewName is required",
					"statusCode": 400,
					"timestamp": 1234567890,
					"success": false
				}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformStringsInNode(&node, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// String field should be transformed
				messageNode := node.Get("message")
				messageStr, _ := messageNode.String()
				Expect(messageStr).To(ContainSubstring("Name"))

				// Non-string fields should be preserved
				statusNode := node.Get("statusCode")
				statusInt, _ := statusNode.Int64()
				Expect(statusInt).To(Equal(int64(400)))

				timestampNode := node.Get("timestamp")
				Expect(timestampNode.Exists()).To(BeTrue())

				successNode := node.Get("success")
				successBool, _ := successNode.Bool()
				Expect(successBool).To(BeFalse())
			})
		})

		Describe("transformErrorFieldNamesInResponse", func() {
			It("should transform 400 error responses", func() {
				jsonData := `{"error": "Field BetterNewName is required"}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				resp := &ResponseInfo{
					Body:       &node,
					StatusCode: 400,
				}

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformErrorFieldNamesInResponse(resp, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				errorNode := resp.Body.Get("error")
				errorStr, _ := errorNode.String()
				Expect(errorStr).To(ContainSubstring("Name"))
			})

			It("should not transform non-400 responses", func() {
				jsonData := `{"error": "Field BetterNewName is required"}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				resp := &ResponseInfo{
					Body:       &node,
					StatusCode: 500, // Not a 400
				}

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformErrorFieldNamesInResponse(resp, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// Should remain unchanged
				errorNode := resp.Body.Get("error")
				errorStr, _ := errorNode.String()
				Expect(errorStr).To(ContainSubstring("BetterNewName"))
			})

			It("should handle nil body gracefully", func() {
				resp := &ResponseInfo{
					Body:       nil,
					StatusCode: 400,
				}

				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err := transformErrorFieldNamesInResponse(resp, fieldMapping)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle empty field mapping", func() {
				jsonData := `{"error": "Some error message"}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				resp := &ResponseInfo{
					Body:       &node,
					StatusCode: 400,
				}

				fieldMapping := map[string]string{} // Empty mapping

				err = transformErrorFieldNamesInResponse(resp, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// Should remain unchanged
				errorNode := resp.Body.Get("error")
				errorStr, _ := errorNode.String()
				Expect(errorStr).To(Equal("Some error message"))
			})
		})

		Describe("transformArrayField", func() {
			It("should transform string arrays", func() {
				jsonData := `{"fields": ["BetterNewName", "OtherField", "Name"]}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldsNode := node.Get("fields")
				fieldMapping := map[string]string{
					"better_new_name": "old_name",
				}

				err = transformStringsInArrayField(&node, "fields", fieldsNode, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// Verify transformation
				updatedFieldsNode := node.Get("fields")
				raw, _ := updatedFieldsNode.Raw()
				Expect(raw).To(ContainSubstring("OldName"))
				Expect(raw).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should handle mixed-type arrays", func() {
				jsonData := `{"data": ["BetterNewName", 123, true, {"field": "value"}]}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				dataNode := node.Get("data")
				fieldMapping := map[string]string{
					"better_new_name": "name",
				}

				err = transformStringsInArrayField(&node, "data", dataNode, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// Verify string was transformed but other types preserved
				updatedDataNode := node.Get("data")
				raw, _ := updatedDataNode.Raw()
				Expect(raw).To(ContainSubstring("Name"))
				Expect(raw).To(ContainSubstring("123"))
				Expect(raw).To(ContainSubstring("true"))
			})

			It("should handle empty arrays", func() {
				jsonData := `{"fields": []}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				fieldsNode := node.Get("fields")
				fieldMapping := map[string]string{
					"field1": "field2",
				}

				err = transformStringsInArrayField(&node, "fields", fieldsNode, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// Should still be an empty array
				updatedFieldsNode := node.Get("fields")
				length, _ := updatedFieldsNode.Len()
				Expect(length).To(Equal(0))
			})

			It("should recursively transform objects in arrays", func() {
				jsonData := `{"items": [{"name": "BetterNewName"}, {"name": "OtherField"}]}`
				node, err := sonic.Get([]byte(jsonData))
				Expect(err).NotTo(HaveOccurred())
				err = node.Load()
				Expect(err).NotTo(HaveOccurred())

				itemsNode := node.Get("items")
				fieldMapping := map[string]string{
					"better_new_name": "old_name",
				}

				err = transformStringsInArrayField(&node, "items", itemsNode, fieldMapping)
				Expect(err).NotTo(HaveOccurred())

				// Verify objects in array were recursively transformed
				// String replacement converts BetterNewName → OldName (PascalCase)
				updatedItemsNode := node.Get("items")
				raw, _ := updatedItemsNode.Raw()
				Expect(raw).To(ContainSubstring("OldName"))
				Expect(raw).NotTo(ContainSubstring("BetterNewName"))
			})
		})
	})

	Describe("Auto-Capture Field Behavior", func() {
		Describe("Context Helpers", func() {
			var ginContext *gin.Context

			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				w := httptest.NewRecorder()
				ginContext, _ = gin.CreateTestContext(w)
			})

			It("should store and retrieve captured field values", func() {
				SetCapturedField(ginContext, "email", "test@example.com")

				value, exists := GetCapturedField(ginContext, "email")
				Expect(exists).To(BeTrue())
				Expect(value).To(Equal("test@example.com"))
			})

			It("should return false for non-existent captured fields", func() {
				_, exists := GetCapturedField(ginContext, "nonexistent")
				Expect(exists).To(BeFalse())
			})

			It("should allow same field name to flow from request to response types", func() {
				// This is the key use case: RemoveField on RequestType captures value,
				// AddField on ResponseType restores it - they share the same field name key
				SetCapturedField(ginContext, "description", "User description")

				// Later, response transformer can retrieve it
				value, exists := GetCapturedField(ginContext, "description")
				Expect(exists).To(BeTrue())
				Expect(value).To(Equal("User description"))
			})

			It("should handle complex types as values", func() {
				complexValue := map[string]interface{}{
					"nested": "value",
					"count":  float64(42),
				}
				SetCapturedField(ginContext, "metadata", complexValue)

				value, exists := GetCapturedField(ginContext, "metadata")
				Expect(exists).To(BeTrue())
				Expect(value).To(HaveKeyWithValue("nested", "value"))
				Expect(value).To(HaveKeyWithValue("count", float64(42)))
			})

			It("should handle nil context gracefully", func() {
				// Should not panic
				SetCapturedField(nil, "field", "value")

				value, exists := GetCapturedField(nil, "field")
				Expect(exists).To(BeFalse())
				Expect(value).To(BeNil())
			})

			It("should check field existence with HasCapturedField", func() {
				SetCapturedField(ginContext, "email", "test@example.com")

				Expect(HasCapturedField(ginContext, "email")).To(BeTrue())
				Expect(HasCapturedField(ginContext, "other")).To(BeFalse())
			})
		})

		Describe("restoreCapturedFieldsToNode", func() {
			var ginContext *gin.Context

			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				w := httptest.NewRecorder()
				ginContext, _ = gin.CreateTestContext(w)
			})

			It("should restore captured values for AddField operations", func() {
				targetType := reflect.TypeOf(BuilderTestUser{})

				// Simulate captured value from request
				SetCapturedField(ginContext, "email", "captured@example.com")

				// Response operations that include AddField for email
				responseOps := ResponseToPreviousVersionOperationList{
					&ResponseAddField{Name: "email", Default: "default@example.com"},
				}

				// Create a response node without the email field
				node, _ := sonic.Get([]byte(`{"id": 1, "name": "Test"}`))
				_ = node.Load()

				// Restore captured fields
				restoreCapturedFieldsToNode(ginContext, targetType, responseOps, &node)

				// Email should now be populated with captured value
				emailNode := node.Get("email")
				Expect(emailNode.Exists()).To(BeTrue())
				email, _ := emailNode.String()
				Expect(email).To(Equal("captured@example.com"))
			})

			It("should not override existing fields in response", func() {
				targetType := reflect.TypeOf(BuilderTestUser{})

				// Simulate captured value
				SetCapturedField(ginContext, "email", "captured@example.com")

				responseOps := ResponseToPreviousVersionOperationList{
					&ResponseAddField{Name: "email", Default: "default@example.com"},
				}

				// Response already has email from handler
				node, _ := sonic.Get([]byte(`{"id": 1, "email": "handler@example.com"}`))
				_ = node.Load()

				restoreCapturedFieldsToNode(ginContext, targetType, responseOps, &node)

				// Should keep handler's value, not captured or default
				email, _ := node.Get("email").String()
				Expect(email).To(Equal("handler@example.com"))
			})

			It("should handle nil context gracefully", func() {
				targetType := reflect.TypeOf(BuilderTestUser{})
				responseOps := ResponseToPreviousVersionOperationList{
					&ResponseAddField{Name: "email", Default: "default@example.com"},
				}

				node, _ := sonic.Get([]byte(`{"id": 1}`))
				_ = node.Load()

				// Should not panic with nil context
				restoreCapturedFieldsToNode(nil, targetType, responseOps, &node)

				// Email should not be added (no context to get captured value from)
				Expect(node.Get("email").Exists()).To(BeFalse())
			})

			It("should restore multiple captured fields", func() {
				targetType := reflect.TypeOf(BuilderTestUser{})

				SetCapturedField(ginContext, "email", "test@example.com")
				SetCapturedField(ginContext, "phone", "+1-555-0100")

				responseOps := ResponseToPreviousVersionOperationList{
					&ResponseAddField{Name: "email", Default: "default@example.com"},
					&ResponseAddField{Name: "phone", Default: "000-000-0000"},
				}

				node, _ := sonic.Get([]byte(`{"id": 1}`))
				_ = node.Load()

				restoreCapturedFieldsToNode(ginContext, targetType, responseOps, &node)

				email, _ := node.Get("email").String()
				phone, _ := node.Get("phone").String()
				Expect(email).To(Equal("test@example.com"))
				Expect(phone).To(Equal("+1-555-0100"))
			})
		})
	})
})
