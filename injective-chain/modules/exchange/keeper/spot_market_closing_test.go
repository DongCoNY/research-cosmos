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

var _ = Describe("Spot markets closing", func() {
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
		ETH              string = "ETH0"
		USDT             string = "USDT0"
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		hooks["setup"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				subaccountID := common.HexToHash(acc.SubaccountIDs[0])
				bankBalances := bankKeeper.GetAllBalances(
					ctx,
					types.SubaccountIDToSdkAddress(subaccountID),
				)
				deposits := keeper.GetDeposits(ctx, subaccountID)

				balancesTracker.SetBankBalancesAndSubaccountDeposits(
					subaccountID,
					bankBalances,
					deposits,
				)
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
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/spot/market_closing", file)
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
		fmt.Println(
			"Orders: ",
			testexchange.GetReadableSlice(
				orders,
				" | ",
				func(ord *types.TrimmedSpotLimitOrder) string {
					side := "sell"
					if ord.IsBuy {
						side = "buy"
					}
					return fmt.Sprintf(
						"p:%v(q:%v) side:%v",
						ord.Price.TruncateInt(),
						ord.Fillable.TruncateInt(),
						side,
					)
				},
			),
		)
	}
	_ = printOrders

	var verifySpotOrder = testexchange.VerifySpotOrder

	_ = verifySpotOrder

	Context("When market is paused", func() {

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc1_paused", true)
			})
			It("they are not cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2), "there were no resting orders")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH).
						String(),
				).To(Equal(f2d(0).String()), "ETH deposit changed after pausing market")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), USDT).
						String(),
				).To(Equal(f2d(0).String()), "USDT deposit changed after pausing market")
			})
		})

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc2_paused_cancel", true)
			})
			It("they can be cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1), "resting order was not canceled")
			})
		})
	})

	Context("When market is demolished", func() {

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc1_demolished", true)
			})
			It("they are cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0), "there were resting orders")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH).
						String(),
				).To(Equal(f2d(0).String()), "ETH deposit changed after pausing market")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), USDT).
						String(),
				).To(Equal(f2d(0).String()), "USDT deposit changed after pausing market")
			})
		})
	})

	Context("When market is expired", func() {

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc1_expired", true)
			})
			It("they are not cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2), "there were no resting orders")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH).
						String(),
				).To(Equal(f2d(0).String()), "ETH deposit changed after pausing market")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), USDT).
						String(),
				).To(Equal(f2d(0).String()), "USDT deposit changed after pausing market")
			})
		})

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				hooks["error"] = func(result *te.TestPlayerHookParams) {
					Expect(result.Error).ToNot(BeNil(), "no error was returned")
					Expect(
						result.Error,
					).To(MatchError(types.ErrSpotMarketNotFound), "wrong error returned")
				}
				runTest("mc2_expired_cancel", true)
			})
			It("they can be cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1), "resting order was not canceled")
			})
		})
	})

	Context("When market is force-settled", func() {

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc1_forced", true)
			})
			It("they are cancelled and whole locked margin (including fees) is returned", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0), "there were no resting orders")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), ETH).
						String(),
				).To(Equal(f2d(0).String()), "ETH deposit wasn't equal to initial one after force-settling market")
				Expect(
					balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), USDT).
						String(),
				).To(Equal(f2d(0).String()), "USDT deposit wasn't equal to initial one after force-settling market")
			})
		})
	})
})
