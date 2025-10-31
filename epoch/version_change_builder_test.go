package epoch

import (
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
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
})
