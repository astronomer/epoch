package epoch

import (
	"context"

	"github.com/bytedance/sonic"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Declarative Builder API", func() {
	var (
		v1 *Version
		v2 *Version
		v3 *Version
	)

	BeforeEach(func() {
		v1, _ = NewDateVersion("2024-01-01")
		v2, _ = NewDateVersion("2024-06-01")
		v3, _ = NewDateVersion("2025-01-01")
	})

	Describe("FieldRename Operation", func() {
		type User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		It("should rename field in request (forward migration)", func() {
			// Create migration: v1 has "name", v2 has "full_name"
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Rename name to full_name").
				Schema(User{}).
				RenameField("name", "full_name").
				Build()

			// Test request migration (v1 -> v2)
			requestJSON := `{"name": "John Doe"}`
			reqInfo := createRequestInfo(requestJSON)

			err := migration.MigrateRequest(context.Background(), reqInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should have "full_name" now
			Expect(reqInfo.Body.Get("full_name").Exists()).To(BeTrue())
			fullName, _ := reqInfo.Body.Get("full_name").String()
			Expect(fullName).To(Equal("John Doe"))

			// Should NOT have "name"
			Expect(reqInfo.Body.Get("name").Exists()).To(BeFalse())
		})

		It("should rename field in response (backward migration)", func() {
			// Create migration: v1 has "name", v2 has "full_name"
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Rename name to full_name").
				Schema(User{}).
				RenameField("name", "full_name").
				Build()

			// Test response migration (v2 -> v1)
			responseJSON := `{"full_name": "Jane Smith"}`
			respInfo := createResponseInfo(responseJSON, 200)

			err := migration.MigrateResponse(context.Background(), respInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should have "name" now
			Expect(respInfo.Body.Get("name").Exists()).To(BeTrue())
			name, _ := respInfo.Body.Get("name").String()
			Expect(name).To(Equal("Jane Smith"))

			// Should NOT have "full_name"
			Expect(respInfo.Body.Get("full_name").Exists()).To(BeFalse())
		})

		It("should transform field names in error messages", func() {
			// Create migration: v1 has "name", v2 has "full_name"
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Rename name to full_name").
				Schema(User{}).
				RenameField("name", "full_name").
				Build()

			// Test error message transformation
			errorJSON := `{"error": "Field 'full_name' is required"}`
			respInfo := createResponseInfo(errorJSON, 400)

			err := migration.MigrateResponse(context.Background(), respInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Error message should have "name" instead of "full_name"
			errorMsg, _ := respInfo.Body.Get("error").String()
			Expect(errorMsg).To(ContainSubstring("name"))
			Expect(errorMsg).ToNot(ContainSubstring("full_name"))
		})
	})

	Describe("FieldAdd Operation", func() {
		type User struct {
			ID    int    `json:"id"`
			Email string `json:"email"`
		}

		It("should add field with default in request", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Add email field").
				Schema(User{}).
				AddField("email", "unknown@example.com").
				Build()

			// Request without email
			requestJSON := `{"id": 1}`
			reqInfo := createRequestInfo(requestJSON)

			err := migration.MigrateRequest(context.Background(), reqInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should have email now
			email, _ := reqInfo.Body.Get("email").String()
			Expect(email).To(Equal("unknown@example.com"))
		})

		It("should not override existing field value", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Add email field").
				Schema(User{}).
				AddField("email", "default@example.com").
				Build()

			// Request with email already set
			requestJSON := `{"id": 1, "email": "user@example.com"}`
			reqInfo := createRequestInfo(requestJSON)

			err := migration.MigrateRequest(context.Background(), reqInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should keep original email
			email, _ := reqInfo.Body.Get("email").String()
			Expect(email).To(Equal("user@example.com"))
		})

		It("should remove field in response (backward migration)", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Add email field").
				Schema(User{}).
				AddField("email", "unknown@example.com").
				Build()

			// Response with email
			responseJSON := `{"id": 1, "email": "test@example.com"}`
			respInfo := createResponseInfo(responseJSON, 200)

			err := migration.MigrateResponse(context.Background(), respInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Email should be removed
			Expect(respInfo.Body.Get("email").Exists()).To(BeFalse())
		})
	})

	Describe("EnumValueMap Operation", func() {
		type User struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		}

		It("should map enum values in request", func() {
			migration := NewVersionChangeBuilder(v2, v3).
				Description("Map status values").
				Schema(User{}).
				MapEnumValues("status", map[string]string{
					"pending":   "inactive",
					"suspended": "inactive",
				}).
				Build()

			// Request with "pending" status
			requestJSON := `{"id": 1, "status": "pending"}`
			reqInfo := createRequestInfo(requestJSON)

			err := migration.MigrateRequest(context.Background(), reqInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should be mapped to "inactive"
			status, _ := reqInfo.Body.Get("status").String()
			Expect(status).To(Equal("inactive"))
		})

		It("should reverse map enum values in response", func() {
			migration := NewVersionChangeBuilder(v2, v3).
				Description("Map status values").
				Schema(User{}).
				MapEnumValues("status", map[string]string{
					"pending":   "inactive",
					"suspended": "inactive",
				}).
				Build()

			// Response with "inactive" status
			responseJSON := `{"id": 1, "status": "inactive"}`
			respInfo := createResponseInfo(responseJSON, 200)

			err := migration.MigrateResponse(context.Background(), respInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should be reverse mapped to one of "pending" or "suspended"
			// (map iteration order is not guaranteed, so both are valid)
			status, _ := respInfo.Body.Get("status").String()
			Expect(status).To(Or(Equal("pending"), Equal("suspended")))
		})
	})

	Describe("Multiple Operations", func() {
		type User struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		It("should apply multiple operations in sequence", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Multiple field operations").
				Schema(User{}).
				RenameField("name", "full_name").
				AddField("email", "unknown@example.com").
				RemoveField("temp_field").
				Build()

			// Request with old structure
			requestJSON := `{"id": 1, "name": "John", "temp_field": "old"}`
			reqInfo := createRequestInfo(requestJSON)

			err := migration.MigrateRequest(context.Background(), reqInfo, nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Check results
			fullName, _ := reqInfo.Body.Get("full_name").String()
			Expect(fullName).To(Equal("John"))
			Expect(reqInfo.Body.Get("name").Exists()).To(BeFalse())

			email, _ := reqInfo.Body.Get("email").String()
			Expect(email).To(Equal("unknown@example.com"))

			Expect(reqInfo.Body.Get("temp_field").Exists()).To(BeFalse())
		})
	})

	Describe("Chaining Multiple Schemas", func() {
		type User struct {
			Name string `json:"name"`
		}

		type Product struct {
			Title string `json:"title"`
		}

		It("should apply operations to multiple schemas", func() {
			migration := NewVersionChangeBuilder(v1, v2).
				Description("Update multiple schemas").
				Schema(User{}).
				RenameField("name", "full_name").
				Schema(Product{}).
				RenameField("title", "product_name").
				Build()

			// Both transformations should be compiled
			Expect(migration).ToNot(BeNil())
			Expect(migration.Description()).To(Equal("Update multiple schemas"))
		})
	})
})

// Helper functions

func createRequestInfo(jsonStr string) *RequestInfo {
	root, _ := sonic.GetFromString(jsonStr)
	return &RequestInfo{
		Body: &root,
	}
}

func createResponseInfo(jsonStr string, statusCode int) *ResponseInfo {
	root, _ := sonic.GetFromString(jsonStr)
	return &ResponseInfo{
		Body:       &root,
		StatusCode: statusCode,
	}
}
