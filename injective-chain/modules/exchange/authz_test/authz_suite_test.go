package authztest

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestExchangeAuthz(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exchange typed authz test scenarios")
}
