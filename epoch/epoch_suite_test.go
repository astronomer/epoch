package epoch

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEpoch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epoch Suite")
}
