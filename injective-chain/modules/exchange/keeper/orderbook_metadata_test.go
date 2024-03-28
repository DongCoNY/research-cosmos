package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Orderbook metadata tests", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		ctx              sdk.Context
		accounts         []te.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		isSpot           bool
		testInput        te.TestInput
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		tp               te.TestPlayer
	)
	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
	})

	var setup = func(testSetup te.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		_ = mainSubaccountId
		testInput = testSetup.TestInput
		if len(testInput.Spots) > 0 {
			marketId = testInput.Spots[0].MarketID
			isSpot = true
		}
		if len(testInput.Perps) > 0 {
			marketId = testInput.Perps[0].MarketID
		}
		if len(testInput.BinaryMarkets) > 0 {
			marketId = testInput.BinaryMarkets[0].MarketID
		}
	}
	var runTest = func(file string, stopOnError bool, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/orderbook", file)
		tp = te.LoadReplayableTest(filepath)
		setup(tp)
		simulationError = tp.ReplayTest(testexchange.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printOrders := func(orders []*types.DerivativeLimitOrder) {
		fmt.Println(te.GetReadableSlice(orders, "-", func(ord *types.DerivativeLimitOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			return fmt.Sprintf("p:%v(q:%v%v)", ord.OrderInfo.Price.TruncateInt(), ord.OrderInfo.Quantity.TruncateInt(), ro)
		}))
	}
	_ = printOrders

	getOrderbook := func(isBuy bool) map[float64]float64 {
		var limit uint64 = 1000
		orderbook := keeper.GetOrderbookPriceLevels(ctx, isSpot, marketId, isBuy, &limit, nil, nil)
		mapped := make(map[float64]float64, len(orderbook))
		for _, level := range orderbook {
			mapped[level.P.MustFloat64()] = level.Q.MustFloat64()
		}
		return mapped
	}
	getOrderbookLimit := func(isBuy bool, limit uint64) map[float64]float64 {
		orderbook := keeper.GetOrderbookPriceLevels(ctx, isSpot, marketId, isBuy, &limit, nil, nil)
		mapped := make(map[float64]float64, len(orderbook))
		for _, level := range orderbook {
			mapped[level.P.MustFloat64()] = level.Q.MustFloat64()
		}
		return mapped
	}

	getOrderbookLimitNotional := func(isBuy bool, notionalLimit float64) map[float64]float64 {
		orderbook := keeper.GetOrderbookPriceLevels(ctx, isSpot, marketId, isBuy, nil, fn2d(&notionalLimit), nil)
		mapped := make(map[float64]float64, len(orderbook))
		for _, level := range orderbook {
			mapped[level.P.MustFloat64()] = level.Q.MustFloat64()
		}
		return mapped
	}
	getOrderbookLimitQuantity := func(isBuy bool, quantityLimit float64) map[float64]float64 {
		orderbook := keeper.GetOrderbookPriceLevels(ctx, isSpot, marketId, isBuy, nil, nil, fn2d(&quantityLimit))
		mapped := make(map[float64]float64, len(orderbook))
		for _, level := range orderbook {
			mapped[level.P.MustFloat64()] = level.Q.MustFloat64()
		}
		return mapped
	}

	Context("Test basic derivative orderbook", func() {
		BeforeEach(func() {
			hooks["block-1"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(0))
				Expect(sellLevels[10.0]).To(Equal(10.0))
			}
			runTest("orderbook_derivatives_01", true, true)
		})
		It("should be correct", func() {
			buyLevels := getOrderbook(true)
			sellLevels := getOrderbook(false)
			Expect(len(buyLevels)).To(Equal(0))
			Expect(sellLevels[10.0]).To(Equal(1.0))
		})
	})

	Context("Performing basic derivative orderbook operations", func() {
		BeforeEach(func() {
			hooks["block-1-setup"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				buyWithNotional := getOrderbookLimitNotional(true, 120.0)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(2))
				Expect(buyLevels[10.0]).To(Equal(13.0))
				Expect(buyLevels[9.0]).To(Equal(8.0))

				Expect(len(buyWithNotional)).To(Equal(1))
				Expect(buyWithNotional[10.0]).To(Equal(13.0))

				Expect(len(sellLevels)).To(Equal(2))
				Expect(sellLevels[11.0]).To(Equal(10.0))
				Expect(sellLevels[12.0]).To(Equal(10.0))
			}
			hooks["block-2-cancel"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(2))
				Expect(buyLevels[10.0]).To(Equal(13.0))
				Expect(buyLevels[9.0]).To(Equal(8.0))

				Expect(len(sellLevels)).To(Equal(1))
				Expect(sellLevels[11.0]).To(Equal(10.0))
			}
			runTest("orderbook_derivatives_02", true, true)
		})
		It("should be correct when orders match", func() {
			buyLevels := getOrderbook(true)
			sellLevels := getOrderbook(false)
			Expect(len(buyLevels)).To(Equal(2))
			Expect(buyLevels[10.0]).To(Equal(7.0))
			Expect(buyLevels[9.0]).To(Equal(8.0))

			Expect(len(sellLevels)).To(Equal(1))
			Expect(sellLevels[11.0]).To(Equal(10.0))
		})
	})

	Context("Performing more advanced derivative orderbook operations", func() {
		BeforeEach(func() {
			hooks["block-1-setup"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(0))
				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-2-ro-mo-co"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[10.0]).To(Equal(5.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-3-ro-co-li"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[10.0]).To(Equal(5.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-4-trigger"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(2))
				Expect(buyLevels[10.0]).To(Equal(3.0))
				Expect(buyLevels[8.0]).To(Equal(12.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-5-atomic-match"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[8.0]).To(Equal(12.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-6-new-limit"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[8.0]).To(Equal(12.0))

				Expect(len(sellLevels)).To(Equal(1))
				Expect(sellLevels[11.0]).To(Equal(10.0))
			}
			runTest("orderbook_derivatives_03", true, false)
		})
		It("should be correct when orders match", func() {
			buyLevels := getOrderbook(true)
			sellLevels := getOrderbook(false)
			Expect(len(buyLevels)).To(Equal(1))
			Expect(buyLevels[8.0]).To(Equal(12.0))

			Expect(len(sellLevels)).To(Equal(1))
			Expect(sellLevels[11.0]).To(Equal(10.0))

			Expect(simulationError).To(Not(BeNil()))
		})
	})

	Context("Performing liquidations of derivative orderbook that do not pause the market", func() {
		BeforeEach(func() {
			hooks["block-1-setup"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[500.0]).To(Equal(20.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-2-price-drop"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[500.0]).To(Equal(20.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			runTest("orderbook_derivatives_04", true, true)
		})
		It("should be correct when orders match", func() {
			market := keeper.GetDerivativeMarketByID(ctx, marketId)
			Expect(market.GetMarketStatus()).To(Equal(types.MarketStatus_Active), "market wasn't active")

			buyLevels := getOrderbook(true)
			sellLevels := getOrderbook(false)
			Expect(len(buyLevels)).To(Equal(1), "buy order of the other user was cancelled even though market was not paused")
			Expect(len(sellLevels)).To(Equal(0))
		})
	})

	Context("Performing liquidations of derivative orderbook that pause the market", func() {
		BeforeEach(func() {
			hooks["block-1-setup"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[500.0]).To(Equal(20.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			hooks["block-2-price-drop"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(1))
				Expect(buyLevels[500.0]).To(Equal(20.0))

				Expect(len(sellLevels)).To(Equal(0))
			}
			runTest("orderbook_derivatives_05", true, true)
		})
		It("should be correct, when market is paused", func() {
			market := keeper.GetDerivativeMarketByID(ctx, marketId)
			Expect(market.GetMarketStatus()).To(Equal(types.MarketStatus_Paused), "market wasn't paused")

			buyLevels := getOrderbook(true)
			sellLevels := getOrderbook(false)
			Expect(len(buyLevels)).To(Equal(0), "there were some buy orders")
			Expect(len(sellLevels)).To(Equal(0), "there were some sell orders")
		})
	})

	Context("Performing basic spot orderbook operations", func() {
		BeforeEach(func() {
			hooks["block-1-setup"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(2))
				Expect(buyLevels[10.0]).To(Equal(13.0))
				Expect(buyLevels[9.0]).To(Equal(8.0))

				Expect(len(sellLevels)).To(Equal(2))
				Expect(sellLevels[11.0]).To(Equal(10.0))
				Expect(sellLevels[12.0]).To(Equal(10.0))
			}
			hooks["block-2-cancel"] = func(params *te.TestPlayerHookParams) {
				buyLevels := getOrderbook(true)
				sellLevels := getOrderbook(false)
				Expect(len(buyLevels)).To(Equal(2))
				Expect(buyLevels[10.0]).To(Equal(13.0))
				Expect(buyLevels[9.0]).To(Equal(8.0))

				Expect(len(sellLevels)).To(Equal(1))
				Expect(sellLevels[11.0]).To(Equal(10.0))
			}
			runTest("orderbook_spot_01", true, true)
		})
		It("should be correct when orders match", func() {
			buyLevels := getOrderbook(true)
			sellLevels := getOrderbook(false)
			Expect(len(buyLevels)).To(Equal(2))
			Expect(buyLevels[10.0]).To(Equal(7.0))
			Expect(buyLevels[9.0]).To(Equal(8.0))

			Expect(len(sellLevels)).To(Equal(1))
			Expect(sellLevels[11.0]).To(Equal(10.0))
		})
	})

	Context("Retrieving orderbook with notional limit", func() {
		It("should retrieve properly limited orders when passing notional limit", func() {
			runTest("orderbook_spot_02", true, true)

			buyLevels := getOrderbookLimitNotional(true, 100.0)
			Expect(len(buyLevels)).To(Equal(1))
			Expect(buyLevels[12.0]).To(Equal(10.0))

			buyLevels = getOrderbookLimitNotional(true, 240.0)
			Expect(len(buyLevels)).To(Equal(3))
			Expect(buyLevels[12.0]).To(Equal(10.0))
			Expect(buyLevels[11.0]).To(Equal(10.0))
			Expect(buyLevels[10.0]).To(Equal(13.0))

			buyLevels = getOrderbookLimitNotional(true, 430.0)
			Expect(len(buyLevels)).To(Equal(4))
			Expect(buyLevels[12.0]).To(Equal(10.0))
			Expect(buyLevels[11.0]).To(Equal(10.0))
			Expect(buyLevels[10.0]).To(Equal(13.0))
			Expect(buyLevels[9.0]).To(Equal(8.0))

			buyLevels = getOrderbookLimitNotional(true, 500.0) // over max
			Expect(len(buyLevels)).To(Equal(4))
			Expect(buyLevels[12.0]).To(Equal(10.0))
			Expect(buyLevels[11.0]).To(Equal(10.0))
			Expect(buyLevels[10.0]).To(Equal(13.0))
			Expect(buyLevels[9.0]).To(Equal(8.0))

		})
	})

	Context("Retrieving orderbook with quantity limit", func() {
		It("should retrieve properly limited orders when passing quantity limit", func() {
			runTest("orderbook_spot_03", true, true)

			sellLevels := getOrderbookLimitQuantity(false, 7.0)
			Expect(len(sellLevels)).To(Equal(1))
			Expect(sellLevels[9.0]).To(Equal(8.0))

			sellLevels = getOrderbookLimitQuantity(false, 25.0)
			Expect(len(sellLevels)).To(Equal(3))
			Expect(sellLevels[9.0]).To(Equal(8.0))
			Expect(sellLevels[10.0]).To(Equal(13.0))
			Expect(sellLevels[11.0]).To(Equal(10.0))

			sellLevels = getOrderbookLimitQuantity(false, 40.0)
			Expect(len(sellLevels)).To(Equal(4))
			Expect(sellLevels[9.0]).To(Equal(8.0))
			Expect(sellLevels[10.0]).To(Equal(13.0))
			Expect(sellLevels[11.0]).To(Equal(10.0))
			Expect(sellLevels[12.0]).To(Equal(10.0))

			sellLevels = getOrderbookLimitQuantity(false, 50.0) // over max
			Expect(len(sellLevels)).To(Equal(4))
			Expect(sellLevels[9.0]).To(Equal(8.0))
			Expect(sellLevels[10.0]).To(Equal(13.0))
			Expect(sellLevels[11.0]).To(Equal(10.0))
			Expect(sellLevels[12.0]).To(Equal(10.0))

		})
	})

	Context("Retrieving orderbook with levels limit", func() {
		It("should retrieve properly limited orders when passing notional limit", func() {
			runTest("orderbook_spot_02", true, true)

			buyLevels := getOrderbookLimit(true, 0)
			Expect(len(buyLevels)).To(Equal(0))

			buyLevels = getOrderbookLimit(true, 1)
			Expect(len(buyLevels)).To(Equal(1))
			Expect(buyLevels[12.0]).To(Equal(10.0))

			buyLevels = getOrderbookLimit(true, 3)
			Expect(len(buyLevels)).To(Equal(3))
			Expect(buyLevels[12.0]).To(Equal(10.0))
			Expect(buyLevels[11.0]).To(Equal(10.0))
			Expect(buyLevels[10.0]).To(Equal(13.0))

			buyLevels = getOrderbookLimit(true, 4) // over max
			Expect(len(buyLevels)).To(Equal(4))
			Expect(buyLevels[12.0]).To(Equal(10.0))
			Expect(buyLevels[11.0]).To(Equal(10.0))
			Expect(buyLevels[10.0]).To(Equal(13.0))
			Expect(buyLevels[9.0]).To(Equal(8.0))
		})
	})

	Context("Testing GRPC query - buy orders", func() {
		BeforeEach(func() {
			runTest("orderbook_spot_both_sides", true, true)
		})
		It("Should return correct results with limit and single side", func() {
			request := &types.QuerySpotOrderbookRequest{
				MarketId:                marketId.String(),
				Limit:                   1,
				OrderSide:               types.OrderSide_Buy,
				LimitCumulativeNotional: nil,
				LimitCumulativeQuantity: nil,
			}
			orderbook := te.MustNotErr(keeper.SpotOrderbook(sdk.WrapSDKContext(ctx), request))
			Expect(len(orderbook.BuysPriceLevel)).To(Equal(1))
			Expect(len(orderbook.SellsPriceLevel)).To(Equal(0))
		})
	})

	// tests below are only based on invariants
	Context("When transient RO are cancelled", func() {
		It("orderbook levels shouldn't be affected", func() {
			runTest("orderbook_derivatives_ro", true, true)
		})
	})

	Context("When transient RO is matched with PO", func() {
		It("orderbook levels should be zero", func() {
			runTest("orderbook_derivatives_transient", true, true)
		})
	})
})
