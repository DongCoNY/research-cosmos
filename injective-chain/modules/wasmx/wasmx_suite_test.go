package wasmx_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWasmx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wasmx Suite")
}
