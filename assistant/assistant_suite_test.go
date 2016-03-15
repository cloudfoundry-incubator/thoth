package assistant_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAssistant(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Assistant Suite")
}
