package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

// Implementation of tests specified in https://injectivelabs.ontestpad.com/script/15#1274/2/

var _ = Describe("Reduce-only improvements - derivative markets", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           keeper.Keeper
		ctx              sdk.Context
		accounts         []testexchange.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		testInput        testexchange.TestInput
		simulationError  error
	)

	var setup = func(testSetup testexchange.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		marketId = testInput.Perps[0].MarketID
	}

	var verifyPosition = func(quantity int64, isLong bool) {
		testexchange.VerifyPosition(injectiveApp, ctx, mainSubaccountId, marketId, quantity, isLong)
	}
	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSorted = func() []*types.TrimmedDerivativeLimitOrder {
		return testexchange.GetAllDerivativeOrdersSorted(injectiveApp, ctx, mainSubaccountId, marketId)
	}

	var verifyOrder = testexchange.VerifyDerivativeOrder

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/reduceonly/derivatives", file)
		test := testexchange.LoadReplayableTest(filepath)
		setup(test)
		simulationError = test.ReplayTest(testexchange.DefaultFlags, nil)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printOrders := func(orders []*types.TrimmedDerivativeLimitOrder) {
		fmt.Println(testexchange.GetReadableSlice(orders, "-", func(ord *types.TrimmedDerivativeLimitOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			return fmt.Sprintf("p:%v(q:%v%v)", ord.Price.TruncateInt(), ord.Quantity.TruncateInt(), ro)
		}))
	}
	_ = printOrders

	Context("Cannot place RO in the same direction as open position", func() {
		BeforeEach(func() {
			runTest("ro1_reject_same_dir", false)
		})
		It(" new RO should be rejected", func() {
			Expect(simulationError).NotTo(BeNil())
			verifyPosition(10, true)

			orders := keeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, mainSubaccountId)
			Expect(len(orders)).To(Equal(0))
		})
	})

	// TODO : Cannot place RO if position is below bankrupcy price (add support for liquidations, add capturing errors)

	Context("RO is cancelled when position is closed by matched market short", func() {
		BeforeEach(func() {
			runTest("ro3_cancel_ro_when_position_close_by_mo", true)
		})
		It("RO should be cancelled", func() {
			verifyPosition(0, true)
			orders := getAllOrdersSorted()
			Expect(len(orders)).To(Equal(0))
		})
	})

	Context("RO is cancelled when position is closed by matched limit sell", func() {
		BeforeEach(func() {
			runTest("ro4_cancel_ro_when_position_close_by_lo", true)
		})
		It("RO should be cancelled", func() {
			verifyPosition(0, true)
			orders := getAllOrdersSorted()
			Expect(len(orders)).To(Equal(0))
		})
	})

	Context("RO.5 RO is cancelled when position is liquidated", func() {
		BeforeEach(func() {
			runTest("ro5_cancel_ro_when_position_liquidated", false)
		})
		It("RO should be cancelled", func() {
			verifyPosition(0, true)
			orders := getAllOrdersSorted()
			Expect(len(orders)).To(Equal(0))
		})
	})

	Context("RO is cancelled when position quantity drops below RO quantity", func() {
		BeforeEach(func() {
			runTest("ro6_cancel_ro_when_position_size_drops", true)
		})
		It("RO should be cancelled", func() {
			verifyPosition(10, true)
			orders := getAllOrdersSorted()
			Expect(len(orders)).To(Equal(0))
		})
	})

	Context("RO.7 worse-priced RO is cancelled when position quantity drops below sum(RO quantity)", func() {
		BeforeEach(func() {
			runTest("ro7_cancel_one_of_ro_when_position_size_drops", true)
		})
		It("Worse-priced RO should be cancelled", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()
			Expect(len(orders)).To(Equal(1))
			verifyOrder(orders, 0, 8, 12, true)
		})
	})

	Context("RO.8 new RO has better price and sum(RO quantity) <= sum(P quantity)", func() {
		BeforeEach(func() {
			runTest("ro8_new_ro_better_price_no_cancel", true)
		})
		It("new order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 3, 4, true)
			verifyOrder(orders, 1, 7, 5, true)
		})
	})

	Context("RO.8a new RO has better price than resting vanilla and RO orders", func() {
		BeforeEach(func() {
			runTest("ro8a_new_ro_better_price_no_cancel", true)
		})
		It("new RO should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 3, 4, true)
			verifyOrder(orders, 1, 7, 5, true)
			verifyOrder(orders, 2, 3, 6, false)
		})
	})

	Context("RO.8a-m new market RO has better price than resting vanilla and RO orders", func() {
		BeforeEach(func() {
			runTest("ro8am_new_ro_better_price_no_cancel", true)
		})
		It("new RO should be placed (and executed)", func() {
			verifyPosition(7, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 7, 5, true)
			verifyOrder(orders, 1, 3, 6, false)
		})
	})

	Context("RO.8b new RO has better price than resting vanilla and RO orders", func() {
		BeforeEach(func() {
			runTest("ro8b_new_ro_better_price_wrecks_havoc", true)
		})
		It("new RO should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(5))
			verifyOrder(orders, 0, 3, 3, true)
			verifyOrder(orders, 1, 1, 4, true)
			verifyOrder(orders, 2, 2, 5, true)
			verifyOrder(orders, 3, 3, 6, true)
			verifyOrder(orders, 4, 4, 7, false)
		})
	})

	Context("RO.8c new RO has better price than resting vanilla and RO orders and worst RO is canceled", func() {
		BeforeEach(func() {
			runTest("ro8c_new_ro_better_price_wrecks_havoc_and_cancels", true)
		})
		It("new RO should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(5))
			verifyOrder(orders, 0, 4, 3, true)
			verifyOrder(orders, 1, 1, 4, true)
			verifyOrder(orders, 2, 2, 5, true)
			verifyOrder(orders, 3, 3, 6, true)
			verifyOrder(orders, 4, 4, 7, false)
		})
	})

	Context("RO.8d new RO has better price than resting vanilla and in middle of RO orders", func() {
		BeforeEach(func() {
			runTest("ro8d_new_ro_better_price_shouldnt_cancel", true)
		})
		It("new RO should be placed, no RO should be cancelled", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(4))
			verifyOrder(orders, 0, 3, 3, true)
			verifyOrder(orders, 1, 3, 4, true)
			verifyOrder(orders, 2, 4, 6, true)
			verifyOrder(orders, 3, 3, 7, false)
		})
	})

	Context("RO.9 new RO has better price and sum(RO quantity) > sum(P quantity) <more complex version, where amount needed > amount to be cancelled>", func() {
		BeforeEach(func() {
			runTest("ro9_new_ro_better_price_cancel_old", true)
		})
		It("new order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 8, 40, true)
			verifyOrder(orders, 1, 2, 50, true)
		})
	})

	Context("RO.10 new RO has better price and sum(RO quantity) > sum(P quantity) <more complex version 2, where amount needed < amount to be cancelled>", func() {
		BeforeEach(func() {
			runTest("ro10_new_ro_better_price_cancel_old", true)
		})
		It("new order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 5, 3, true)
			verifyOrder(orders, 1, 2, 4, true)
			verifyOrder(orders, 2, 3, 5, true)
		})
	})

	Context("RO.11 new RO has better price and sum(RO quantity) > sum(P quantity) <more complex version 2, where amount needed = amount to be cancelled>", func() {
		BeforeEach(func() {
			runTest("ro11_new_ro_better_price_cancel_old", true)
		})
		It("new order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 7, 3, true)
			verifyOrder(orders, 1, 3, 4, true)
		})
	})

	Context("RO.12 new RO has better price and sum(RO quantity) > sum(P quantity) (but new RO quantity > P quantity, so that new RO needs to be resized)", func() {
		BeforeEach(func() {
			runTest("ro12_new_ro_resize_old_cancel", true)
		})
		It("new order should be placed and resized", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(1))
			verifyOrder(orders, 0, 10, 4, true)
		})
	})

	Context("RO.13 new RO has equal/worse price and sum(RO quantity) <= sum(P quantity)", func() {
		BeforeEach(func() {
			runTest("ro13_new_ro_equal_no_cancel", true)
		})
		It("new order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 7, 5, true)
			verifyOrder(orders, 1, 3, 5, true)
		})
	})

	Context("RO.14 new RO has equal/worse price and sum(RO quantity) > sum(P quantity)", func() {
		BeforeEach(func() {
			runTest("ro14_new_ro_resize", true)
		})
		It("new order should be resized", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 7, 5, true)
			verifyOrder(orders, 1, 3, 5, true)
		})
	})

	Context("RO.14a new RO has equal price and sum(resting RO quantity) = sum(P quantity)", func() {
		BeforeEach(func() {
			runTest("ro14a_new_ro_rejected_no_vanilla", false)
		})
		It("new order should be rejected", func() {
			Expect(simulationError).NotTo(BeNil())
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 3, 4, true)
			verifyOrder(orders, 1, 7, 5, true)
		})
	})

	Context("RO.15 new RO is rejected when sum(RO quantity) > sum(P quantity, RESTING quantity)", func() {
		BeforeEach(func() {
			runTest("ro15_new_ro_rejected", false)
		})
		It("new order should be rejected", func() {
			Expect(simulationError).NotTo(BeNil())
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 7, 5, true)
			verifyOrder(orders, 1, 3, 5, false)
		})
	})

	Context("RO.16 new RO is placed when sum(RO quantity) <= sum(P quantity, RESTING quantity)", func() {
		BeforeEach(func() {
			runTest("ro16_new_ro_placed", true)
		})
		It("new order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 4, 5, true)
			verifyOrder(orders, 1, 3, 5, false)
			verifyOrder(orders, 2, 3, 5, true)
		})
	})

	// 17-18 removed due to wrong requirements

	Context("RO.19 new RO is placed when sum(RO quantity) > sum(P quantity, RESTING quantity) (when resting vanilla orders have worse price)", func() {
		BeforeEach(func() {
			runTest("ro19_new_ro_placed_but_resized", true)
		})
		It("new order should be placed and resized", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 6, 9, true)
			verifyOrder(orders, 1, 5, 10, false)
		})
	})

	/**
	RO.20 new RO is placed, resting RO cancelled and new vanilla is placed, when sum(RO quantity) > sum(P quantity, RESTING quantity)
	and new RO has better price than the resting one and new vanilla order has the best price of all of them
	*/
	Context("RO.20a new RO is placed and resting rejected, when sum(RO quantity) > sum(P quantity, RESTING quantity) and resting vanilla orders equal price, but there's also a new vanilla placed with better price)  ", func() {
		BeforeEach(func() {
			runTest("ro20a_new_ro_resized_because_placed_after_vanilla", true)
		})
		It("new RO order should be resized, old ro cancelled", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 6, 8, false)
			verifyOrder(orders, 1, 4, 9, true)
			verifyOrder(orders, 2, 5, 10, false)
		})
	})

	/**
	RO.20 new RO is rejected, resting RO cancelled and new vanilla is placed, when sum(RO quantity) > sum(P quantity, RESTING quantity)
	and new RO has better price than the resting one and new vanilla order has the best price of all of them (and is processed before new RO order)
	*/
	Context("RO.20b new RO is placed and resting rejected, when sum(RO quantity) > sum(P quantity, RESTING quantity) and resting vanilla orders equal price, but there's also a new vanilla placed with better price)  ", func() {
		BeforeEach(func() {
			runTest("ro20b_new_ro_cancelled_by_vanilla", true)
		})
		It("new RO order should be resized, old ro cancelled", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 6, 8, false)
			verifyOrder(orders, 1, 5, 10, false)
		})
	})

	Context("RO.21 new RO is placed when sum(RO quantity) > sum(P quantity, RESTING quantity) (when resting vanilla orders have worse price, but there's a new one placed with better price AND there's a LONG limit order)", func() {
		BeforeEach(func() {
			runTest("ro21", true)
		})
		It("new RO order should be rejected, old ro cancelled", func() {
			verifyPosition(16, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 6, 8, false)
			verifyOrder(orders, 1, 5, 10, false)
		})
	})

	Context("RO.22 new LIMIT LONG changes everything if it has been filled before RO, so that resting RO is not cancelled and new is added", func() {
		BeforeEach(func() {
			runTest("ro22_limit_and_ro_in_same_block", true)
		})
		It("new RO order should not be rejected", func() {
			verifyPosition(16, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(2))
			verifyOrder(orders, 0, 6, 8, true)
			verifyOrder(orders, 1, 5, 10, false)
		})
	})

	Context("RO.23 resting vanilla order is ignored, when placing a new RO order due it it's worse price, but the new RO still is resized", func() {
		BeforeEach(func() {
			runTest("ro23_vanilla_ignored", true)
		})
		It("new RO order should be resizes, old RO should stay", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 5, 10, true)
			verifyOrder(orders, 1, 5, 11, true)
			verifyOrder(orders, 2, 5, 12, false)
		})
	})

	Context("Extra case - sandwiched vanilla order cancels worse RO order", func() {
		BeforeEach(func() {
			runTest("ro_x1_worst_ro_not_cancelled", true)
		})
		It("worst RO order should be cancelled", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 6, 3450, false)
			verifyOrder(orders, 1, 2, 3500, true)
			verifyOrder(orders, 2, 4, 3550, false)
		})
	})

	Context("Extra case 2 - sandwiched vanilla order cancels worse RO order v2", func() {
		BeforeEach(func() {
			runTest("ro_x2_worst_ro_not_cancelled", true)
		})
		It("worst RO order should be cancelled", func() {
			verifyPosition(9, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 7, 3400, false)
			verifyOrder(orders, 1, 1, 3500, true)
			verifyOrder(orders, 2, 2, 3550, false)
		})
	})

	Context("RO.59 new RO is rejected, when better/equal priced vanilla orders size is equal to position size", func() {
		BeforeEach(func() {
			runTest("ro59", false)
		})
		It("new RO order should be rejected", func() {
			Expect(simulationError).NotTo(BeNil())
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 4, 10, false)
			verifyOrder(orders, 1, 3, 11, false)
			verifyOrder(orders, 2, 3, 12, false)
		})
	})

	Context("RO.60 new RO is rejected, when better/equal priced vanilla orders and RO size is equal to position size and worst order is vanilla", func() {
		BeforeEach(func() {
			runTest("ro60", false)
		})
		It("new RO order should be rejected", func() {
			Expect(simulationError).NotTo(BeNil())
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 4, 10, false)
			verifyOrder(orders, 1, 3, 11, true)
			verifyOrder(orders, 2, 3, 12, false)
		})
	})

	Context("RO.61 new RO is rejected, when better/equal priced vanilla orders and RO size is equal to position size and worst order is RO", func() {
		BeforeEach(func() {
			runTest("ro61", false)
		})
		It("new RO order should be rejected", func() {
			Expect(simulationError).NotTo(BeNil())
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 4, 10, false)
			verifyOrder(orders, 1, 3, 11, false)
			verifyOrder(orders, 2, 3, 12, true)
		})
	})

	Context("RO.62 new vanilla is placed, when better/equal priced vanilla orders and RO size is equal to position size and worst order is RO (which is canceled)", func() {
		BeforeEach(func() {
			runTest("ro62", true)
		})
		It("new vanilla order should be placed and RO should be canceled", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(3))
			verifyOrder(orders, 0, 4, 10, false)
			verifyOrder(orders, 1, 3, 11, false)
			verifyOrder(orders, 2, 1, 12, false)
		})
	})

	Context("RO.63 new vanilla is placed, when better/equal priced vanilla orders and RO size is equal to position size and worst order is vanilla order", func() {
		BeforeEach(func() {
			runTest("ro63", true)
		})
		It("new vanilla order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(4))
			verifyOrder(orders, 0, 4, 10, false)
			verifyOrder(orders, 1, 3, 11, true)
			verifyOrder(orders, 2, 3, 12, false)
			verifyOrder(orders, 3, 1, 12, false)
		})
	})

	Context("RO.64 new RO is placed, when better priced vanilla orders and RO size is equal to position size and worst order is vanilla", func() {
		BeforeEach(func() {
			runTest("ro64", true)
		})
		It("new RO order should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(4))
			verifyOrder(orders, 0, 4, 10, true)
			verifyOrder(orders, 1, 3, 11, false)
			verifyOrder(orders, 2, 1, 11, true)
			verifyOrder(orders, 3, 3, 12, false)
		})
	})

	Context("RO.65 new ROa are placed, when worse priced vanilla orders and RO size is equal to position size", func() {
		BeforeEach(func() {
			runTest("ro65", true)
		})
		It("new RO orders should be placed", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(5))
			verifyOrder(orders, 0, 6, 8, true)
			verifyOrder(orders, 1, 4, 9, true)
			verifyOrder(orders, 2, 4, 10, false)
			verifyOrder(orders, 3, 3, 11, false)
			verifyOrder(orders, 4, 3, 12, false)
		})
	})

	Context("RO.66 new ROs are placed and resized, when worse priced vanilla orders and RO size is equal to position size", func() {
		BeforeEach(func() {
			runTest("ro66", true)
		})
		It("new RO orders should be placed and resized", func() {
			verifyPosition(10, true)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(4))
			verifyOrder(orders, 0, 6, 8, true)
			verifyOrder(orders, 1, 4, 10, false)
			verifyOrder(orders, 2, 3, 11, false)
			verifyOrder(orders, 3, 3, 12, false)
		})
	})

	Context("MARKET REDUCE-ONLY EXCEPTIONS", func() {

		Context("RO.24 market RO is fully cancelled, if it cannot be filled", func() {
			BeforeEach(func() {
				runTest("ro24_market_ro_is_rejected", false)
			})
			It("MO shouldn't affect resting RO", func() {
				Expect(simulationError).NotTo(BeNil()) // MO rejected on no liquidity
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 10, true)
			})
		})

		Context("RO.25 unfilled portion of market RO is cancelled", func() {
			BeforeEach(func() {
				runTest("ro25_unfilled_portion_of_market_ro_is_cancelled", true)
			})
			It("unfilled part of MO should be cancelled, RO should be cancelled as well", func() {
				verifyPosition(8, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})

		Context("RO.67 better priced RO market order cancels only necesary quantity of RO orders", func() {
			BeforeEach(func() {
				runTest("ro67", true)
			})
			It("unfilled part of MO should be cancelled, some resting RO should stay", func() {
				verifyPosition(8, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				verifyOrder(orders, 0, 4, 9, true)
			})
		})

		Context("RO.68 better priced RO market order doesn't cancel resting RO if there is enough quantity", func() {
			BeforeEach(func() {
				runTest("ro68", true)
			})
			It("unfilled part of MO should be cancelled, all resting RO should stay", func() {
				verifyPosition(8, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2))

				verifyOrder(orders, 0, 2, 9, true)
				verifyOrder(orders, 1, 2, 10, true)
			})
		})

		Context("RO.69 worse priced RO market order doesn't cancel resting RO if there is enough quantity", func() {
			BeforeEach(func() {
				runTest("ro69", false)
			})
			It("worse-priced resting RO should stay", func() {
				Expect(simulationError).NotTo(BeNil()) // MO rejected on no liquidity
				verifyPosition(8, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				verifyOrder(orders, 0, 2, 9, true)
			})
		})

		Context("RO.70 worse priced RO market order cannot be placed if there is enough quantity, but no liquidity", func() {
			BeforeEach(func() {
				runTest("ro70", false)
			})
			It("worse-priced resting RO should stay", func() {
				Expect(simulationError).NotTo(BeNil()) // MO rejected on no liquidity
				verifyPosition(6, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})

		Context("RO.71 better-priced market RO is filled and resting RO with worst price is cancelled", func() {
			BeforeEach(func() {
				runTest("ro71", true)
			})
			It("worse-priced resting RO should stay", func() {
				verifyPosition(8, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				verifyOrder(orders, 0, 8, 8, true)
			})
		})

		Context("RO.72 better-priced market RO is filled and resting RO with worst price and vanilla stay", func() {
			BeforeEach(func() {
				runTest("ro72", true)
			})
			It("worse-priced vanilla order is not impacted", func() {
				verifyPosition(8, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2))

				verifyOrder(orders, 0, 8, 8, true)
				verifyOrder(orders, 1, 2, 9, false)
			})
		})

		Context("RO.73 sandwiched market RO order is filled and leaves worse-priced RO untouched, when there's enough quantity", func() {
			BeforeEach(func() {
				runTest("ro73", true)
			})
			It("better-priced resting ROs stay", func() {
				verifyPosition(5, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				verifyOrder(orders, 0, 2, 10, true)
			})
		})

		Context("RO.74 sandwiched market RO order is filled and cancels all resting ROs, when there's not enough quantity", func() {
			BeforeEach(func() {
				runTest("ro74", true)
			})
			It("better-priced resting ROs stay", func() {
				verifyPosition(5, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})
	})

	Context("NEW VANILLA ORDER COMPETITION", func() {

		Context("RO.27 new LIMIT SHORT order has EQUAL price to worst reduce-only and = quantity and is filled before RO thus leaving the RO untouched", func() {
			BeforeEach(func() {
				runTest("ro27", true)
			})
			It("resting RO order should stay", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 5, 10, false)
				verifyOrder(orders, 1, 5, 10, true)
			})
		})

		Context("RO.28 new LIMIT SHORT order that is NOT filled has EQUAL price to worst reduce-only and > quantity and thus doesn't cancel the resting RO", func() {
			BeforeEach(func() {
				runTest("ro28", true)
			})
			It("resting RO order should stay", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 6, 10, false)
			})
		})

		Context("RO.29 new LIMIT SHORT order that is NOT filled has BETTER price than worst reduce-only and > quantity and thus cancels both ROs", func() {
			BeforeEach(func() {
				runTest("ro29", true)
			})
			It("resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 7, 9, false)
			})
		})

		Context("RO.30 new LIMIT SHORT order that is NOT filled has BETTER price than SOME reduce-only and > quantity and thus cancels only worse RO", func() {
			BeforeEach(func() {
				runTest("ro30", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 3, 10, true)
				verifyOrder(orders, 1, 7, 11, false)
			})
		})

		Context("RO.31 new LIMIT SHORT order that is NOT filled has BETTER price than worst reduce-only and > quantity and cancels only one RO due to processing order", func() {
			BeforeEach(func() {
				runTest("ro31", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 7, 9, false)
				verifyOrder(orders, 1, 3, 10, true)
			})
		})

		Context("RO.32 new LIMIT SHORT order that is NOT filled and has BETTER price than worst reduce-only and < quantity doesn't touch existing ROs", func() {
			BeforeEach(func() {
				runTest("ro32", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 4, 9, false)
				verifyOrder(orders, 1, 4, 10, true)
				verifyOrder(orders, 2, 2, 11, true)
			})
		})

		Context("RO.33 new LIMIT SHORT order that is NOT filled and has BETTER price than SOME worst reduce-only and < quantity doesn't touch existing ROs", func() {
			BeforeEach(func() {
				runTest("ro33", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 4, 10, true)
				verifyOrder(orders, 1, 4, 11, false)
				verifyOrder(orders, 2, 2, 12, true)
			})
		})

		Context("RO.34 two new LIMIT SHORT orders that are NOT filled and have BETTER price than SOME worst reduce-only and < quantity do not cancel any ROs", func() {
			BeforeEach(func() {
				runTest("ro34", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(4))
				verifyOrder(orders, 0, 2, 10, true)
				verifyOrder(orders, 1, 2, 11, false)
				verifyOrder(orders, 2, 2, 11, false)
				verifyOrder(orders, 3, 2, 12, true)
			})
		})

		Context("RO.34 two new LIMIT SHORT orders that are NOT filled and have BETTER price than SOME worst reduce-only and < quantity do not cancel any ROs", func() {
			BeforeEach(func() {
				runTest("ro34", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(4))
				verifyOrder(orders, 0, 2, 10, true)
				verifyOrder(orders, 1, 2, 11, false)
				verifyOrder(orders, 2, 2, 11, false)
				verifyOrder(orders, 3, 2, 12, true)
			})
		})

		Context("RO.35 multiple ROs should be cancelled, when a BETTER-priced RO is placed", func() {
			BeforeEach(func() {
				runTest("ro35", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 8, 3, true)
				verifyOrder(orders, 1, 1, 4, true)
				verifyOrder(orders, 2, 1, 4, true)
			})
		})

		Context("RO.36 multiple ROs should be cancelled, when a BETTER-priced LIMIT order is placed", func() {
			BeforeEach(func() {
				runTest("ro36", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 8, 3, false)
				verifyOrder(orders, 1, 1, 4, true)
				verifyOrder(orders, 2, 1, 4, true)
			})
		})

		Context("RO.37 some multiple ROs should be cancelled, when a LIMIT order with price better than SOME RO's is placed", func() {
			BeforeEach(func() {
				runTest("ro37", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(4))
				verifyOrder(orders, 0, 1, 4, true)
				verifyOrder(orders, 1, 1, 4, true)
				verifyOrder(orders, 2, 4, 5, false)

				Expect(orders[3].Price.TruncateInt().String()).To(Equal(testexchange.NewDecFromFloat(6).TruncateInt().String()))
				Expect(orders[3].Quantity.Sub(sdk.NewDec(2)).GTE(sdk.ZeroDec())).To(BeTrue()) // quantity at least 2, but we aren't sure which order exactly will stay
				Expect(orders[3].IsReduceOnly()).To(BeTrue())
			})
		})

		Context("RO.38 better priced RO's are not cancelled, when a LIMIT order with price better than SOME RO's is placed even if it's size is equal to position size", func() {
			BeforeEach(func() {
				runTest("ro38", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 1, 4, true)
				verifyOrder(orders, 1, 1, 4, true)
				verifyOrder(orders, 2, 10, 5, false)
			})
		})
	})

	Context("MARKET orders", func() {

		Context("RO.39 new MARKET SHORT cancels only the required amount of resting ROs", func() {
			BeforeEach(func() {
				runTest("ro39", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(3, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 1, 4, true)
				verifyOrder(orders, 1, 1, 4, true)
			})
		})

		Context("RO.45 new MARKET SHORT order has EQUAL price to worst reduce-only and is filled before RO thus leaving the RO untouched", func() {
			BeforeEach(func() {
				runTest("ro45", true)
			})
			It("resting RO should stay", func() {
				verifyPosition(5, true)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 10, true)
			})
		})
	})

	Context("SELL direction", func() {
		Context("RO.46 worse-priced RO is cancelled when position quantity drops below sum(RO quantity) (was RO.7)", func() {
			BeforeEach(func() {
				runTest("ro46", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 7, 5, true)
			})
		})

		Context("RO.47 new RO has better price and sum(RO quantity) > sum(P quantity) (where amount needed > amount to be cancelled)", func() {
			BeforeEach(func() {
				runTest("ro47", true)
			})
			It("worst resting RO orders should be cancelled", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 8, 7, true)
				verifyOrder(orders, 1, 2, 6, true)
			})
		})

		Context("RO.48 new RO has equal price as existing one and its quantity is too high", func() {
			BeforeEach(func() {
				runTest("ro48", true)
			})
			It("new RO order with equal price should be resized", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 7, 5, true)
				verifyOrder(orders, 1, 3, 5, true)
			})
		})

		Context("RO.49 new RO has equal price as existing one, but there is also a vanilla order with same price", func() {
			BeforeEach(func() {
				runTest("ro49", false)
			})
			It("new RO order with equal price should be rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 7, 5, true)
				verifyOrder(orders, 1, 3, 5, false)
			})
		})

		Context("RO.50 sandwiched vanilla order cancels worse RO order", func() {
			BeforeEach(func() {
				runTest("ro50", true)
			})
			It("new vanilla order cancels some ROs with worse price", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 2, 14, true)
				verifyOrder(orders, 1, 6, 13, true)
				verifyOrder(orders, 2, 4, 12, false)
			})
		})

		Context("RO.51 new RO is rejected, even if resting resting vanilla order has worse price, but there are new RO and vanilla orders submitted in the same block", func() {
			BeforeEach(func() {
				runTest("ro51", false)
			})
			It("new RO order is rejected", func() {
				verifyPosition(16, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 6, 11, false)
				verifyOrder(orders, 1, 5, 10, false)
			})
		})

		Context("RO.52 resting vanilla order is ignored, when placing a new RO order due it it's worse price, but the new RO still is resized", func() {
			BeforeEach(func() {
				runTest("ro52", true)
			})
			It("new RO order is resized to fit reducible quantity", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 5, 12, true)
				verifyOrder(orders, 1, 5, 11, true)
				verifyOrder(orders, 2, 5, 10, false)
			})
		})

		Context("RO.53 new LIMIT BUY order that is NOT filled has EQUAL price to worst reduce-only and bigger quantity and thus cancels the resting RO", func() {
			BeforeEach(func() {
				runTest("ro53", true)
			})
			It("new limit buy order cancels a resting one", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 6, 10, false)
			})
		})

		Context("RO.54 new LIMIT LONG order that is NOT filled has BETTER price than SOME reduce-only and bigger quantity quantity and thus cancels only worse RO", func() {
			BeforeEach(func() {
				runTest("ro54", true)
			})
			It("new limit buy order cancels the worst resting one", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 3, 14, true)
				verifyOrder(orders, 1, 7, 13, false)
			})
		})

		Context("RO.55 multiple ROs should be cancelled, when a BETTER-priced RO is placed", func() {
			BeforeEach(func() {
				runTest("ro55", true)
			})
			It("new RO cancels multiple existing ones", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 8, 7, true)
				verifyOrder(orders, 1, 1, 6, true)
				verifyOrder(orders, 2, 1, 6, true)
			})
		})

		Context("RO.56 multiple ROs should be cancelled, when a BETTER-priced LIMIT order is placed", func() {
			BeforeEach(func() {
				runTest("ro56", true)
			})
			It("new vanilla cancels multiple existing ones", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(3))
				verifyOrder(orders, 0, 8, 7, false)
				verifyOrder(orders, 1, 1, 6, true)
				verifyOrder(orders, 2, 1, 6, true)
			})
		})

		Context("RO.57 some multiple ROs should be cancelled, when a LIMIT order with price better than SOME RO's is placed", func() {
			BeforeEach(func() {
				runTest("ro57", true)
			})
			It("new vanilla cancels multiple existing ones", func() {
				verifyPosition(10, false)

				orders := getAllOrdersSorted()

				Expect(len(orders)).To(Equal(4))
				verifyOrder(orders, 0, 1, 8, true)
				verifyOrder(orders, 1, 1, 8, true)
				verifyOrder(orders, 2, 4, 7, false)

				Expect(orders[3].Price.TruncateInt().String()).To(Equal(testexchange.NewDecFromFloat(6).TruncateInt().String()))
				Expect(orders[3].Quantity.Sub(sdk.NewDec(2)).GTE(sdk.ZeroDec())).To(BeTrue()) // quantity at least 2, but we aren't sure which order exactly will stay
				Expect(orders[3].IsReduceOnly()).To(BeTrue())
			})
		})
	})

	Context("RO.58 if position is zeroed existing worse-priced RO orders stay in the orderbook", func() {
		BeforeEach(func() {
			runTest("ro58", true)
		})
		It("closing position removes all resting RO", func() {
			verifyPosition(0, false)

			orders := getAllOrdersSorted()

			Expect(len(orders)).To(Equal(0))
		})
	})

	Context("Special test case market grabbing limit", func() {
		BeforeEach(func() {
			runTest("ro_special_case", true)
		})
		It("new RO order should be cancelled", func() {
			verifyPosition(0, false)

			orders := getAllOrdersSorted()
			printOrders(orders)

			Expect(len(orders)).To(Equal(0))
		})
	})

	Context("Cancelling", func() {
		Context("RO.75 RO order can be cancelled", func() {
			BeforeEach(func() {
				runTest("ro75", true)
			})
			It("cancelling orders works", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 11, true)
			})
		})

		Context("RO.76 When quantity is freed by cancelling old RO order new RO order can be placed", func() {
			BeforeEach(func() {
				runTest("ro76", true)
			})
			It("new order is placed when old is canceled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 5, 9, true)
				verifyOrder(orders, 1, 5, 11, true)
			})
		})

		Context("RO.77 When quantity is freed by cancelling old vanilla order then new RO order can be placed", func() {
			BeforeEach(func() {
				runTest("ro77", true)
			})
			It("new order is placed when old is canceled", func() {
				verifyPosition(10, true)

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2))
				verifyOrder(orders, 0, 5, 10, true)
				verifyOrder(orders, 1, 5, 11, true)
			})
		})
	})
})
