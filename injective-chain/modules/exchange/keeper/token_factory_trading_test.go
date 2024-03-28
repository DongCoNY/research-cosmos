package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Token factory trading", func() {
	var (
		testPlayer       *te.TestPlayer
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		ctx              sdk.Context
		accounts         []te.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		testInput        te.TestInput
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		balancesTracker  *te.BalancesTracker
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		hooks["post-setup"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				subaccountID := common.HexToHash(acc.SubaccountIDs[0])
				bankBalances := injectiveApp.BankKeeper.GetAllBalances(ctx, types.SubaccountIDToSdkAddress(subaccountID))
				balancesTracker.SetBankBalancesAndSubaccountDeposits(subaccountID, bankBalances, keeper.GetDeposits(ctx, subaccountID))
			}
		}
	})

	var setup = func(testSetup *te.TestPlayer) {
		testPlayer = testSetup
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
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
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/token_factory", file)
		test := te.LoadReplayableTest(filepath)
		setup(&test)
		simulationError = test.ReplayTest(te.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedSpotLimitOrder {
		return te.GetAllSpotOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	var getAllOrdersSorted = func() []*types.TrimmedSpotLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	printOrders := func(orders []*types.TrimmedSpotLimitOrder) {
		fmt.Println("Orders: ", te.GetReadableSlice(orders, " | ", func(ord *types.TrimmedSpotLimitOrder) string {
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v) side:%v", ord.Price.TruncateInt(), ord.Fillable.TruncateInt(), side)
		}))
	}
	_ = printOrders

	var verifySpotOrder = te.VerifySpotOrder

	_ = verifySpotOrder

	Context("Spot market can be launched and traded", func() {

		When("Market is using token factory denom as base asset", func() {

			Context("Limit and market orders can be placed and matched", func() {
				BeforeEach(func() {
					hooks["limit-trade"] = func(params *te.TestPlayerHookParams) {
						denom := (*testPlayer.TokenFactoryDenoms)[0]
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), denom)).To(Equal(sdk.NewDec(5)), fmt.Sprintf("'%s' total balance did not increase by 5 for account 0", denom))
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), denom)).To(Equal(sdk.NewDec(-5)), fmt.Sprintf("'%s' total balance did not decrease by 5 for account 1", denom))
					}
					runTest("tf_spot_trading_base_asset", true)
				})
				It("all orders are filled", func() {
					denom := (*testPlayer.TokenFactoryDenoms)[0]
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), denom).String()).To(Equal(f2d(0).String()), fmt.Sprintf("'%s' total balance did not return to initial state for account 0", denom))
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), denom).String()).To(Equal(f2d(0).String()), fmt.Sprintf("'%s' total balance did not return to initial state for account 1", denom))
				})
			})
		})

		When("Market is using token factory denom as quote asset", func() {

			Context("Limit and market orders are placed and matched", func() {
				limitExecutionPrice := f2d(10.5)
				limitOrderValue := limitExecutionPrice.Mul(f2d(5))
				BeforeEach(func() {
					hooks["limit-trade"] = func(params *te.TestPlayerHookParams) {
						denom := (*testPlayer.TokenFactoryDenoms)[0]
						takerFeeRate := testPlayer.TestInput.Spots[1].TakerFeeRate

						expectedAddr0Change := limitOrderValue.Add(limitOrderValue.Mul(takerFeeRate))
						expectedAddr1Change := limitOrderValue.Sub(limitOrderValue.Mul(takerFeeRate))

						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), denom).Mul(f2d(-1)).String()).To(Equal(expectedAddr0Change.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 0", denom))
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), denom).String()).To(Equal(expectedAddr1Change.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 1", denom))
					}
					runTest("tf_spot_trading_quote_asset", true)
				})
				It("all orders are filled", func() {
					denom := (*testPlayer.TokenFactoryDenoms)[0]
					takerFeeRate := testPlayer.TestInput.Spots[1].TakerFeeRate
					makerFeeRate := testPlayer.TestInput.Spots[1].MakerFeeRate
					marketExecutionPrice := f2d(11)
					marketOrderValue := marketExecutionPrice.Mul(f2d(5))

					expectedAddr0Change := marketOrderValue.Sub(marketOrderValue.Mul(takerFeeRate)).Sub(limitOrderValue.Add(limitOrderValue.Mul(takerFeeRate)))
					expectedAddr1Change := limitOrderValue.Sub(limitOrderValue.Mul(takerFeeRate)).Sub(marketOrderValue.Add(marketOrderValue.Mul(makerFeeRate)))
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), denom).String()).To(Equal(expectedAddr0Change.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 0", denom))
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), denom).String()).To(Equal(expectedAddr1Change.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 1", denom))
				})
			})
		})

		When("Market is using token factory denoms as both quote and base assets", func() {

			Context("Limit and market orders are placed and matched", func() {
				limitExecutionPrice := f2d(10.5)
				limitOrderValue := limitExecutionPrice.Mul(f2d(5))
				BeforeEach(func() {
					hooks["limit-trade"] = func(params *te.TestPlayerHookParams) {
						quoteDenom := (*testPlayer.TokenFactoryDenoms)[0]
						takerFeeRate := testPlayer.TestInput.Spots[1].TakerFeeRate

						expectedAddr0QuoteChange := limitOrderValue.Add(limitOrderValue.Mul(takerFeeRate))
						expectedAddr1QuoteChange := limitOrderValue.Sub(limitOrderValue.Mul(takerFeeRate))
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), quoteDenom).Mul(f2d(-1)).String()).To(Equal(expectedAddr0QuoteChange.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 0", quoteDenom))
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), quoteDenom).String()).To(Equal(expectedAddr1QuoteChange.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 1", quoteDenom))

						baseDenom := (*testPlayer.TokenFactoryDenoms)[1]
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), baseDenom).String()).To(Equal(f2d(5).String()), fmt.Sprintf("'%s' total balance did not change correctly for account 0", baseDenom))
						Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), baseDenom).String()).To(Equal(f2d(-5).String()), fmt.Sprintf("'%s' total balance did not change correctly for account 1", baseDenom))
					}
					runTest("tf_spot_trading_both_assets", true)
				})
				It("all orders are filled", func() {
					quoteDenom := (*testPlayer.TokenFactoryDenoms)[0]
					takerFeeRate := testPlayer.TestInput.Spots[1].TakerFeeRate
					makerFeeRate := testPlayer.TestInput.Spots[1].MakerFeeRate
					marketExecutionPrice := f2d(11)
					marketOrderValue := marketExecutionPrice.Mul(f2d(5))

					expectedAddr0QuoteChange := marketOrderValue.Sub(limitOrderValue).Sub(limitOrderValue.Mul(takerFeeRate).Add(marketOrderValue.Mul(takerFeeRate)))
					expectedAddr1QuoteChange := limitOrderValue.Sub(limitOrderValue.Mul(takerFeeRate)).Sub(marketOrderValue.Add(marketOrderValue.Mul(makerFeeRate)))
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), quoteDenom).String()).To(Equal(expectedAddr0QuoteChange.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 0", quoteDenom))
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), quoteDenom).String()).To(Equal(expectedAddr1QuoteChange.String()), fmt.Sprintf("'%s' total balance did not change correctly for account 1", quoteDenom))

					baseDenom := (*testPlayer.TokenFactoryDenoms)[1]

					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), baseDenom).String()).To(Equal(f2d(0).String()), fmt.Sprintf("'%s' total balance did not change correctly for account 0", baseDenom))
					Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), baseDenom).String()).To(Equal(f2d(0).String()), fmt.Sprintf("'%s' total balance did not change correctly for account 1", baseDenom))
				})
			})
		})
	})
})
