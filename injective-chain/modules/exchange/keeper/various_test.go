package keeper_test

import (
	"fmt"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Various tests", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		ctx              sdk.Context
		accounts         []testexchange.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		testInput        testexchange.TestInput
		simulationError  error
		hooks            map[string]testexchange.TestPlayerHook
		tp               testexchange.TestPlayer
	)
	BeforeEach(func() {
		hooks = make(map[string]testexchange.TestPlayerHook)
	})

	var verifyEstimateQuoteTotalDepositChange = func(accountIdx int, expectedChange float64, ignoreFees bool) {
		marketId := testInput.Spots[0].MarketID
		market := keeper.GetSpotMarketByID(ctx, marketId)

		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		funds := keeper.GetSpendableFunds(ctx, subaccountID, market.QuoteDenom)

		balanceDelta := funds.Sub(tp.BalancesTracker.GetTotalBalancePlusBank(accountIdx, market.QuoteDenom)).MustFloat64()
		maxDiff := 1.0
		if ignoreFees {
			maxDiff = maxDiff + balanceDelta*market.TakerFeeRate.MustFloat64()
		}

		Expect(math.Abs(balanceDelta-expectedChange) <= maxDiff).To(BeTrue(), fmt.Sprintf("Quote change %v for account: %v should equal: %v", balanceDelta, accountIdx, expectedChange))
	}

	var verifyEstimateBaseTotalDepositChange = func(accountIdx int, expectedChange float64, ignoreFees bool) {
		marketId := testInput.Spots[0].MarketID
		market := keeper.GetSpotMarketByID(ctx, marketId)

		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		funds := keeper.GetSpendableFunds(ctx, subaccountID, market.BaseDenom)

		diff := funds.Sub(tp.BalancesTracker.GetTotalBalancePlusBank(accountIdx, market.BaseDenom))
		maxFees := sdk.ZeroDec()
		if ignoreFees {
			maxFees = diff.Mul(market.TakerFeeRate)
		}
		Expect(math.Abs(diff.Add(maxFees).MustFloat64()-expectedChange) <= 1).To(BeTrue(), fmt.Sprintf("Base change %v for account: %v should equal: %v", diff, accountIdx, expectedChange))
	}

	var setup = func(testSetup testexchange.TestPlayer) {
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

	var verifyDerivativePosition = func(accountIdx, quantity int64, isLong bool) {
		subaccountId := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		testexchange.VerifyPosition(injectiveApp, ctx, subaccountId, marketId, quantity, isLong)
	}
	_ = verifyDerivativePosition
	// getAllDerivativeOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllDerivativeOrdersSorted = func() []*types.TrimmedDerivativeLimitOrder {
		return testexchange.GetAllDerivativeOrdersSorted(injectiveApp, ctx, mainSubaccountId, marketId)
	}
	_ = getAllDerivativeOrdersSorted

	var verifyOrder = testexchange.VerifyDerivativeOrder
	_ = verifyOrder

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/various", file)
		tp = testexchange.LoadReplayableTest(filepath)
		setup(tp)
		simulationError = tp.ReplayTest(testexchange.DefaultFlags, &hooks)
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

	Context("Test basic batch update case", func() {
		BeforeEach(func() {
			runTest("test_batch_01", true)
		})
		It("should execute", func() {
			market1 := testInput.Perps[0].MarketID
			market2 := testInput.Perps[1].MarketID

			orders := keeper.GetAllTraderDerivativeLimitOrders(ctx, market1, mainSubaccountId)
			Expect(len(orders)).To(Equal(3))

			orders2 := keeper.GetAllTraderDerivativeLimitOrders(ctx, market2, mainSubaccountId)
			Expect(len(orders2)).To(Equal(0))
		})
	})

	Context("Test creating spot orders", func() {
		BeforeEach(func() {
			hooks["1"] = func(*testexchange.TestPlayerHookParams) {
				marketId1 := testInput.Spots[0].MarketID
				market := keeper.GetSpotMarketByID(ctx, marketId1)

				balancesQuote := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, mainSubaccountId, market.QuoteDenom)
				balancesBase := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, mainSubaccountId, market.BaseDenom)

				fmt.Printf("Balance before: Acc %v, quote: %v, base: %v\n", mainSubaccountId, balancesQuote, balancesBase)
			}
			runTest("test_spot_01", true)
		})
		It("should execute", func() {
			marketId1 := testInput.Spots[0].MarketID
			market := keeper.GetSpotMarketByID(ctx, marketId1)
			balancesQuote := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, mainSubaccountId, market.QuoteDenom)
			balancesBase := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, mainSubaccountId, market.BaseDenom)

			verifyEstimateQuoteTotalDepositChange(0, 100500, true)
			verifyEstimateBaseTotalDepositChange(0, -100, true)
			fmt.Fprintf(GinkgoWriter, "Acc %v, quote: %v, base: %v\n", mainSubaccountId, balancesQuote, balancesBase)
		})

	})

})
