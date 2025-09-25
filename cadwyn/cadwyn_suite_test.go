package cadwyn

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCadwyn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cadwyn Suite")
}
