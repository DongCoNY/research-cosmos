package wasmbinding_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWasmbinding(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wasmbinding Suite")
}
