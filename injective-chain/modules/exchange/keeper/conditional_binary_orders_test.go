package keeper_test

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

// Implementation of tests specified in

type conditionalBinaryOrder struct {
	order        types.IOrder
	triggerPrice sdk.Dec
	higher       bool // if true trigger when mark > triggerPrice, else when mark < triggerPrice
	orderHash    []byte
}

var _ = Describe("Conditional binary orders", func() {
	var injectiveApp *simapp.InjectiveApp
	var keeper keeper.Keeper
	var ctx sdk.Context
	var accounts []testexchange.Account
	var mainSubaccountId common.Hash
	var marketId common.Hash
	var testInput testexchange.TestInput
	var simulationError error
	var hooks map[string]func(error)

	BeforeEach(func() {
		hooks = make(map[string]func(error))
	})

	var setup = func(testSetup testexchange.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		marketId = testInput.BinaryMarkets[0].MarketID
	}

	var getAllConditionalOrdersSorted = func(subaccountId common.Hash, marketId common.Hash) []*types.TrimmedDerivativeConditionalOrder {
		orders := keeper.GetAllSubaccountConditionalOrders(ctx, marketId, subaccountId)
		sort.SliceStable(orders, func(i, j int) bool {
			if orders[i].TriggerPrice.Equal(orders[j].TriggerPrice) {
				if orders[i].Quantity.Equal(orders[j].Quantity) {
					return !orders[i].IsBuy
				}
				return orders[i].Quantity.GT(orders[j].Quantity)
			}
			return orders[i].TriggerPrice.LT(orders[j].TriggerPrice)
		})
		return orders
	}

	var getAvailableQuoteBalance = func(accountIdx int, denom string) sdk.Dec {
		balancesQuote := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, common.HexToHash(accounts[accountIdx].SubaccountIDs[0]), denom)
		return balancesQuote.AvailableBalance
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/conditionals/binary", file)
		test := testexchange.LoadReplayableTest(filepath)
		setup(test)
		simulationError = test.ReplayTestWithLegacyHooks(testexchange.DefaultFlags, &hooks, nil)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	Context("Conditional binary orders cannot be placed", func() {

		Context("C.1 User cannot place a conditional stop-sell binary order", func() {
			var initialSetupBalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialSetupBalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("binary_c1", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialSetupBalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})

		Context("C.2 User cannot place a conditional stop-buy binary order", func() {
			var initialSetupBalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialSetupBalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("binary_c2", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialSetupBalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})

		Context("C.3 User cannot place a conditional take-sell binary order", func() {
			var initialSetupBalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialSetupBalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("binary_c3", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialSetupBalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})

		Context("C.4 User cannot place a conditional take-buy binary order", func() {
			var initialSetupBalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialSetupBalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("binary_c4", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialSetupBalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})
	})

})
