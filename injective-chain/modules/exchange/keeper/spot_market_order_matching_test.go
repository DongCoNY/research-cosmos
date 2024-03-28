package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Spot markets and order matching", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		bankKeeper       bankkeeper.Keeper
		ctx              sdk.Context
		accounts         []te.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		testInput        te.TestInput
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		balancesTracker  *te.BalancesTracker
		ETH0             string = "ETH0"
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		hooks["init"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				subaccountID := common.HexToHash(acc.SubaccountIDs[0])
				bankBalances := bankKeeper.GetAllBalances(ctx, types.SubaccountIDToSdkAddress(subaccountID))
				deposits := keeper.GetDeposits(ctx, subaccountID)

				balancesTracker.SetBankBalancesAndSubaccountDeposits(subaccountID, bankBalances, deposits)
			}
		}
	})

	var setup = func(testSetup te.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		bankKeeper = injectiveApp.BankKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		if len(testInput.Spots) > 0 {
			marketId = testInput.Spots[0].MarketID
		}
		if len(testInput.Perps) > 0 {
			marketId = testInput.Perps[0].MarketID
		}
		if len(testInput.BinaryMarkets) > 0 {
			marketId = testInput.BinaryMarkets[0].MarketID
		}
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/spot/order_matching", file)
		test := te.LoadReplayableTest(filepath)
		setup(test)
		simulationError = test.ReplayTest(testexchange.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedSpotLimitOrder {
		return testexchange.GetAllSpotOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	var getAllOrdersSorted = func() []*types.TrimmedSpotLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	printOrders := func(orders []*types.TrimmedSpotLimitOrder) {
		fmt.Println("Orders: ", testexchange.GetReadableSlice(orders, " | ", func(ord *types.TrimmedSpotLimitOrder) string {
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v) side:%v", ord.Price.TruncateInt(), ord.Fillable.TruncateInt(), side)
		}))
	}
	_ = printOrders

	var verifySpotOrder = testexchange.VerifySpotOrder

	_ = verifySpotOrder

	Context("Various order types can be processed in a single block", func() {

		Context("VO.1 Limit and market orders from the same subaccount", func() {
			BeforeEach(func() {
				runTest("vo1_both_filled", true)
			})
			It("are both filled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
				Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH0).String()).To(Equal(f2d(9).String()), "incorrect base asset deposit change")
			})
		})

		Context("VO.2 Limit and market orders from the same subaccount", func() {
			BeforeEach(func() {
				runTest("vo2_market_filled", true)
			})
			It("market is fully filled, limit becomes resting", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifySpotOrder(orders, 0, 4, 4, 10, true)
				Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH0).String()).To(Equal(f2d(5).String()), "incorrect base asset deposit change")
			})
		})

		Context("VO.3 Limit and market orders from the same subaccount", func() {
			BeforeEach(func() {
				runTest("vo3_market_partially_filled", true)
			})
			It("market is partially filled and limit becomes resting", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifySpotOrder(orders, 0, 4, 4, 10, true)
				Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH0).String()).To(Equal(f2d(3).String()), "incorrect base asset deposit change")
			})
		})
	})
})
