package keeper_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKeeper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keeper Suite")
}
