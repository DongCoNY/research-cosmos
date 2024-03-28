package keeper_test

import (
	"fmt"
	"sort"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func f2d(f64Value float64) sdk.Dec {
	return sdk.MustNewDecFromStr(fmt.Sprintf("%f", f64Value))
}

func fn2d(nullableFloat *float64) *sdk.Dec {
	if nullableFloat != nil {
		dec := f2d(*nullableFloat)
		return &dec
	} else {
		return nil
	}
}

var _ = Describe("Check decimal exceed checker function", func() {
	Context("When decimal exceed", func() {
		It("should return false", func() {
			dec := sdk.MustNewDecFromStr("111.2345")
			Expect(keeper.CheckIfExceedDecimals(dec, 3)).To(Equal(true))
		})
	})

	Context("When decimal does not exceed", func() {
		It("should return false", func() {
			dec := sdk.MustNewDecFromStr("111.234500000")
			Expect(keeper.CheckIfExceedDecimals(dec, 4)).To(Equal(false))
		})
	})

	Context("When provide too big decimal", func() {
		It("should return false", func() {
			dec := sdk.MustNewDecFromStr("111.2345")
			Expect(keeper.CheckIfExceedDecimals(dec, 18)).To(Equal(false))
		})
	})

	Context("When provide negative price", func() {
		It("should return false", func() {
			dec := sdk.MustNewDecFromStr("-111.2345")
			Expect(keeper.CheckIfExceedDecimals(dec, 4)).To(Equal(false))
		})
	})
})

func TestPrefixSub(t *testing.T) {
	cases := map[string]struct {
		src      []byte
		expEnd   []byte
		expPanic bool
	}{
		"normal":                 {src: []byte{1, 3, 4}, expEnd: []byte{1, 3, 3}},
		"normal short":           {src: []byte{79}, expEnd: []byte{78}},
		"empty case":             {src: []byte{}},
		"roll-over example 1":    {src: []byte{17, 28, 0}, expEnd: []byte{17, 27, 255}},
		"roll-over example 2":    {src: []byte{15, 42, 0, 0}, expEnd: []byte{15, 41, 255, 255}},
		"pathological roll-over": {src: []byte{0, 0, 0, 0}},
		"nil prohibited":         {expPanic: true},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			if tc.expPanic {
				require.Panics(t, func() {
					keeper.SubtractBitFromPrefix(tc.src)
				})
				return
			}
			end := keeper.SubtractBitFromPrefix(tc.src)
			assert.Equal(t, tc.expEnd, end)
		})
	}
}

var _ = Describe("Check Order sorting function", func() {

	Context("When reference prices are set in various ways", func() {
		It("should be sorted", func() {

			referencePrice := sdk.MustNewDecFromStr("5")
			orders := []*types.TrimmedSpotLimitOrder{
				{
					Price: sdk.NewDec(1),
					IsBuy: true,
				}, {
					Price: sdk.NewDec(3),
					IsBuy: true,
				}, {
					Price: sdk.NewDec(2),
					IsBuy: true,
				}, {
					Price: sdk.NewDec(4),
					IsBuy: true,
				},
			}

			sort.SliceStable(orders, func(i, j int) bool {
				return keeper.GetIsOrderLess(referencePrice, orders[i].Price, orders[j].Price, orders[i].IsBuy, orders[j].IsBuy, false)
			})

			for idx := range orders {
				if idx == len(orders)-1 {
					break
				}
				Expect(orders[idx].Price.GT(orders[idx+1].Price)).To(BeTrue())
			}

			sort.SliceStable(orders, func(i, j int) bool {
				return keeper.GetIsOrderLess(referencePrice, orders[i].Price, orders[j].Price, orders[i].IsBuy, orders[j].IsBuy, true)
			})

			for idx := range orders {
				if idx == len(orders)-1 {
					break
				}
				Expect(orders[idx].Price.LT(orders[idx+1].Price)).To(BeTrue())
			}

		})
	})
})

var _ = Describe("Check Order sorting function", func() {
	It("should be sorted", func() {
		referencePrice := sdk.MustNewDecFromStr("5")
		orders := []*types.TrimmedSpotLimitOrder{
			{
				Price: sdk.NewDec(1),
				IsBuy: true,
			}, {
				Price: sdk.NewDec(3),
				IsBuy: true,
			}, {
				Price: sdk.NewDec(2),
				IsBuy: true,
			}, {
				Price: sdk.NewDec(4),
				IsBuy: true,
			},
		}

		sort.SliceStable(orders, func(i, j int) bool {
			return keeper.GetIsOrderLess(referencePrice, orders[i].Price, orders[j].Price, orders[i].IsBuy, orders[j].IsBuy, false)
		})

		for idx := range orders {
			if idx == len(orders)-1 {
				break
			}
			Expect(orders[idx].Price.GT(orders[idx+1].Price)).To(BeTrue())
		}

		sort.SliceStable(orders, func(i, j int) bool {
			return keeper.GetIsOrderLess(referencePrice, orders[i].Price, orders[j].Price, orders[i].IsBuy, orders[j].IsBuy, true)
		})

		for idx := range orders {
			if idx == len(orders)-1 {
				break
			}
			Expect(orders[idx].Price.LT(orders[idx+1].Price)).To(BeTrue())
		}

	})
})
