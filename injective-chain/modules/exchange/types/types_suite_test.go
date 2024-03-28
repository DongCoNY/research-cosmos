package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func TestTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types Suite")
}

var _ = Describe("Types test", func() {
	Describe("Tick Size tests", func() {
		It("should have error", func() {
			price, err := sdk.NewDecFromStr("0.000000000010337895")
			Expect(err).To(BeNil())
			minPriceTickSize, err := sdk.NewDecFromStr("1000000000.000000000000000000")
			Expect(err).To(BeNil())

			isBreached := types.BreachesMinimumTickSize(price, minPriceTickSize)
			Expect(isBreached).To(BeTrue())
		})

		It("should have error", func() {
			price, err := sdk.NewDecFromStr("0.000000000010337895")
			Expect(err).To(BeNil())
			minPriceTickSize, err := sdk.NewDecFromStr("0.00000000001")
			Expect(err).To(BeNil())

			isBreached := types.BreachesMinimumTickSize(price, minPriceTickSize)
			Expect(isBreached).To(BeTrue())
		})

		It("should have error", func() {
			price, err := sdk.NewDecFromStr("0.00000000021000000")
			Expect(err).To(BeNil())
			minPriceTickSize, err := sdk.NewDecFromStr("0.0000000000010")
			Expect(err).To(BeNil())

			isBreached := types.BreachesMinimumTickSize(price, minPriceTickSize)
			Expect(isBreached).To(BeFalse())
		})
	})
})
