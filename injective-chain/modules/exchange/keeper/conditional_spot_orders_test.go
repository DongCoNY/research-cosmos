package keeper_test

import (
	"fmt"
	"math"
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

type conditionalSpotOrder struct {
	order        types.IOrder
	triggerPrice sdk.Dec
	higher       bool // if true trigger when mark > triggerPrice, else when mark < triggerPrice
	orderHash    []byte
}

var _ = Describe("Conditional spot orders", func() {
	var injectiveApp *simapp.InjectiveApp
	var keeper keeper.Keeper
	var ctx sdk.Context
	var accounts []testexchange.Account
	var mainSubaccountId common.Hash
	var marketId common.Hash
	var testInput testexchange.TestInput
	var simulationError error
	var hooks map[string]func(error)
	var initialDeposits []map[string]*types.Deposit

	BeforeEach(func() {
		hooks = make(map[string]func(error))
		initialDeposits = make([]map[string]*types.Deposit, 0)
		hooks["init"] = func(err error) {
			for _, acc := range accounts {
				var deposits = keeper.GetDeposits(ctx, common.HexToHash(acc.SubaccountIDs[0]))
				initialDeposits = append(initialDeposits, deposits)
			}
		}
	})

	var setup = func(testSetup testexchange.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		marketId = testInput.Spots[0].MarketID
	}

	var verifyPosition = func(quantity int64, isLong bool) {
		testexchange.VerifyPosition(injectiveApp, ctx, mainSubaccountId, marketId, quantity, isLong)
	}
	_ = verifyPosition

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedSpotLimitOrder {
		return testexchange.GetAllSpotOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSorted = func() []*types.TrimmedSpotLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	var getAllConditionalSpotOrdersSorted = func(subaccountId common.Hash, marketId common.Hash) []conditionalSpotOrder {
		orders := make([]conditionalSpotOrder, 0)
		higherTrue := true
		higherFalse := false
		for _, hash := range keeper.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketId, true, true, types.MarketType_Spot, subaccountId) {
			order, dir := keeper.GetConditionalSpotMarketOrderBySubaccountIDAndHash(ctx, marketId, &higherTrue, subaccountId, hash)
			orders = append(orders, conditionalSpotOrder{order, *order.TriggerPrice, dir, order.OrderHash})
		}
		for _, hash := range keeper.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketId, false, true, types.MarketType_Spot, subaccountId) {
			order, dir := keeper.GetConditionalSpotMarketOrderBySubaccountIDAndHash(ctx, marketId, &higherFalse, subaccountId, hash)
			orders = append(orders, conditionalSpotOrder{order, *order.TriggerPrice, dir, order.OrderHash})
		}
		for _, hash := range keeper.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketId, true, false, types.MarketType_Spot, subaccountId) {
			order, dir := keeper.GetConditionalSpotLimitOrderBySubaccountIDAndHash(ctx, marketId, &higherTrue, subaccountId, hash)
			orders = append(orders, conditionalSpotOrder{order, *order.TriggerPrice, dir, order.OrderHash})
		}
		for _, hash := range keeper.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketId, false, false, types.MarketType_Spot, subaccountId) {
			order, dir := keeper.GetConditionalSpotLimitOrderBySubaccountIDAndHash(ctx, marketId, &higherFalse, subaccountId, hash)
			orders = append(orders, conditionalSpotOrder{order, *order.TriggerPrice, dir, order.OrderHash})
		}
		sort.SliceStable(orders, func(i, j int) bool {
			if orders[i].triggerPrice.Equal(orders[j].triggerPrice) {
				if orders[i].order.GetQuantity().Equal(orders[j].order.GetQuantity()) {
					return !orders[i].higher
				}
				return orders[i].order.GetQuantity().GT(orders[j].order.GetQuantity())
			}
			return orders[i].triggerPrice.LT(orders[j].triggerPrice)
		})
		return orders
	}
	_ = getAllConditionalSpotOrdersSorted

	var verifyWhetherOrderIsPostOnly = func(orderHash common.Hash, shouldBePostOnly bool) {
		orderFound := keeper.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketId, nil, mainSubaccountId, orderHash)
		if shouldBePostOnly {
			Expect(orderFound.GetOrderType().IsPostOnly()).To(BeTrue(), fmt.Sprintf("Order '%v' should be post-only", orderHash.String()))
		} else {
			Expect(orderFound.GetOrderType().IsPostOnly()).To(BeFalse(), fmt.Sprintf("Order '%v' should not be post-only", orderHash.String()))
		}
	}

	_ = verifyWhetherOrderIsPostOnly

	var getAvailableQuoteBalance = func(accountIdx int, denom string) sdk.Dec {
		balancesQuote := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, common.HexToHash(accounts[accountIdx].SubaccountIDs[0]), denom)
		return balancesQuote.AvailableBalance
	}

	_ = getAvailableQuoteBalance

	var f2d = testexchange.NewDecFromFloat
	var verifyOrder = testexchange.VerifyDerivativeOrder
	_ = verifyOrder

	var verifyConditionalOrder = func(orders []conditionalSpotOrder, orderIdx int, triggerPrice float64, higher *bool, quantity float64, price float64) {
		Expect(orders[orderIdx].triggerPrice.TruncateInt().String()).To(Equal(f2d(triggerPrice).TruncateInt().String()), fmt.Sprintf("Trigger price for order %d", orderIdx))

		if higher != nil {
			Expect(orders[orderIdx].higher).To(BeEquivalentTo(*higher), fmt.Sprintf("Trigger 'higher' for order %d", orderIdx))
		}
		if price > 0 {
			Expect(orders[orderIdx].order.GetPrice().TruncateInt().String()).To(Equal(f2d(price).TruncateInt().String()), fmt.Sprintf("Conditional order price for order %d", orderIdx))
		}
		if quantity > 0 {
			Expect(orders[orderIdx].order.GetQuantity().TruncateInt().String()).To(Equal(f2d(quantity).TruncateInt().String()), fmt.Sprintf("Conditional order quantity for order %d", orderIdx))
		}
	}
	_ = verifyConditionalOrder

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/conditionals/spot", file)
		test := testexchange.LoadReplayableTest(filepath)
		setup(test)
		simulationError = test.ReplayTestWithLegacyHooks(testexchange.DefaultFlags, &hooks, nil)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printOrders := func(orders []*types.TrimmedDerivativeLimitOrder) {
		fmt.Fprintln(GinkgoWriter, "Orders: ", testexchange.GetReadableSlice(orders, " | ", func(ord *types.TrimmedDerivativeLimitOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v%v) side:%v", ord.Price.TruncateInt(), ord.Fillable.TruncateInt(), ro, side)
		}))
	}
	_ = printOrders

	printConditionalOrders := func(orders []conditionalOrder) {
		fmt.Println(testexchange.GetReadableSlice(orders, " | ", func(ord conditionalOrder) string {
			ro := ""
			if ord.order.GetMargin().IsZero() {
				ro = " ro"
			}
			stopType := ""
			if !ord.triggerPrice.IsZero() {
				if ord.order.IsBuy() {
					if ord.higher {
						stopType = " sl"
					} else {
						stopType = " tp"
					}
				} else {
					if ord.higher {
						stopType = " tp"
					} else {
						stopType = " sl"
					}
				}
			}
			side := "sell"
			if ord.order.IsBuy() {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v%v%v) tp:%v side:%v", ord.order.GetPrice().TruncateInt(), ord.order.GetQuantity().TruncateInt(), ro, stopType, ord.triggerPrice.TruncateInt(), side)
		}))
	}
	_ = printConditionalOrders

	var verifyEstimateQuoteDepositChange = func(accountIdx int, expectedChange float64, total bool, ignoreFees bool) {
		marketId := testInput.Spots[0].MarketID
		market := keeper.GetSpotMarketByID(ctx, marketId)
		balancesQuote := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, common.HexToHash(accounts[accountIdx].SubaccountIDs[0]), market.QuoteDenom)
		var balanceDelta float64
		if total {
			balanceDelta = balancesQuote.TotalBalance.Sub(initialDeposits[accountIdx][market.QuoteDenom].TotalBalance).MustFloat64()
		} else {
			balanceDelta = balancesQuote.AvailableBalance.Sub(initialDeposits[accountIdx][market.QuoteDenom].AvailableBalance).MustFloat64()
		}
		maxDiff := 1.0
		if ignoreFees {
			maxDiff = maxDiff + math.Abs(balanceDelta)*market.TakerFeeRate.MustFloat64()
		}

		Expect(math.Abs(balanceDelta-expectedChange) <= maxDiff).To(BeTrue(), fmt.Sprintf("Quote currency change %v for account: %v should equal: %v", balanceDelta, accountIdx, expectedChange))
	}
	_ = verifyEstimateQuoteDepositChange

	var verifyEstimateBaseDepositChange = func(accountIdx int, expectedChange float64, total bool, ignoreFees bool) {
		marketId := testInput.Spots[0].MarketID
		market := keeper.GetSpotMarketByID(ctx, marketId)
		balancesBase := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, common.HexToHash(accounts[accountIdx].SubaccountIDs[0]), market.BaseDenom)
		var balanceDelta float64
		if total {
			balanceDelta = balancesBase.TotalBalance.Sub(initialDeposits[accountIdx][market.BaseDenom].TotalBalance).MustFloat64()
		} else {
			balanceDelta = balancesBase.AvailableBalance.Sub(initialDeposits[accountIdx][market.BaseDenom].AvailableBalance).MustFloat64()
		}
		maxDiff := 1.0
		if ignoreFees {
			maxDiff = maxDiff + balanceDelta*market.TakerFeeRate.MustFloat64()
		}
		Expect(math.Abs(balanceDelta-expectedChange) <= maxDiff).To(BeTrue(), fmt.Sprintf("Base currency change %v for account: %v should equal: %v", balanceDelta, accountIdx, expectedChange))
	}
	_ = verifyEstimateBaseDepositChange

	// Commented out for now, but will be needed later
	//PContext("Basics", func() {
	//
	//	Context("B.1 Create conditional spot stop loss limit order", func() {
	//		BeforeEach(func() {
	//			runTest("test_spot_cond_b1", true)
	//		})
	//		It("new order should be stop-loss", func() {
	//			verifyEstimateBaseDepositChange(1, -100, false, true)    // margin for sell order
	//			verifyEstimateQuoteDepositChange(2, -90000, false, true) // margin for buy order
	//
	//			verifyEstimateBaseDepositChange(0, -100, false, true) // margin for stop sell
	//
	//			condOrders := getAllConditionalSpotOrdersSorted(mainSubaccountId, marketId)
	//			Expect(len(condOrders)).To(Equal(1))
	//			verifyConditionalOrder(condOrders, 0, 800, &expectFalse, 100, 790)
	//		})
	//	})
	//
	//	Context("B.2 Create conditional spot stop loss market order", func() {
	//		BeforeEach(func() {
	//			runTest("test_spot_cond_b2", true)
	//		})
	//		It("new order should be stop-loss", func() {
	//			condOrders := getAllConditionalSpotOrdersSorted(mainSubaccountId, marketId)
	//			Expect(len(condOrders)).To(Equal(1))
	//			verifyConditionalOrder(condOrders, 0, 800, &expectFalse, 100, 790)
	//		})
	//	})
	//})

	Context("Conditional spot orders cannot be placed", func() {

		Context("C1. User cannot place a conditional stop-sell spot order", func() {
			var initialalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("spot_c1", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalSpotOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})

		Context("C2. User cannot place a conditional stop-buy spot order", func() {
			var initialalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("spot_c2", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalSpotOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})

		Context("C3. User cannot place a conditional take-sell spot order", func() {
			var initialalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("spot_c3", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalSpotOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})

		Context("C4. User cannot place a conditional take-buy spot order", func() {
			var initialalance sdk.Dec
			BeforeEach(func() {
				hooks["start"] = func(err error) {
					initialalance = getAvailableQuoteBalance(0, "USDT0")
				}
				hooks["submit"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("spot_c4", true)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalSpotOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(initialalance.String()).To(Equal(getAvailableQuoteBalance(0, "USDT0").String()))
			})
		})
	})

})
