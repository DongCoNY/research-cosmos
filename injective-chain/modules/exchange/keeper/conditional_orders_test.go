package keeper_test

import (
	"fmt"
	"math"
	"sort"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

type conditionalOrder struct {
	order        types.IDerivativeOrder
	triggerPrice sdk.Dec
	higher       bool // if true trigger when mark > triggerPrice, else when mark < triggerPrice
}

var _ = Describe("Conditional orders", func() {
	var (
		injectiveApp       *simapp.InjectiveApp
		k                  keeper.Keeper
		bankKeeper         bankkeeper.Keeper
		ctx                sdk.Context
		accounts           []testexchange.Account
		mainSubaccountId   common.Hash
		marketId           common.Hash
		testInput          testexchange.TestInput
		simulationError    error
		expectTrue         = true
		expectFalse        = false
		hooks              map[string]func(error)
		app                *simapp.InjectiveApp
		market             keeper.DerivativeMarketI
		subaccountIdBuyer  = testexchange.SampleSubaccountAddr1
		subaccountIdSeller = testexchange.SampleSubaccountAddr2
		senderBuyer        = types.SubaccountIDToSdkAddress(subaccountIdBuyer)
		senderSeller       = types.SubaccountIDToSdkAddress(subaccountIdSeller)
		deposit            = &types.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		startingPrice           = sdk.NewDec(2000)
		margin                  = sdk.NewDec(1000)
		oracleBase, oracleQuote string
		oracleType              oracletypes.OracleType
		err                     error
		tp                      testexchange.TestPlayer
	)

	BeforeEach(func() {
		hooks = make(map[string]func(error))

		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		oracleBase, oracleQuote, oracleType = testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, senderBuyer, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, senderSeller, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, senderBuyer, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		market, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			testInput.Perps[0].Ticker,
			testInput.Perps[0].QuoteDenom,
			oracleBase,
			oracleQuote,
			0,
			oracleType,
			testInput.Perps[0].InitialMarginRatio,
			testInput.Perps[0].MaintenanceMarginRatio,
			testInput.Perps[0].MakerFeeRate,
			testInput.Perps[0].TakerFeeRate,
			testInput.Perps[0].MinPriceTickSize,
			testInput.Perps[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)

		marketId = market.MarketID()

		testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer.String(), sdk.NewCoins(
			sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()),
			sdk.NewCoin(testInput.Perps[0].BaseDenom, deposit.AvailableBalance.TruncateInt()),
		))
		testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(
			sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()),
			sdk.NewCoin(testInput.Perps[0].BaseDenom, deposit.AvailableBalance.TruncateInt()),
		))
	})

	var setup = func(testSetup testexchange.TestPlayer) {
		injectiveApp = testSetup.App
		k = injectiveApp.ExchangeKeeper
		bankKeeper = injectiveApp.BankKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		marketId = testInput.Perps[0].MarketID
	}

	var verifyPosition = func(quantity int64, isLong bool) {
		testexchange.VerifyPosition(injectiveApp, ctx, mainSubaccountId, marketId, quantity, isLong)
	}

	var verifyOtherPosition = func(quantity int64, isLong bool, subaccountId common.Hash) {
		testexchange.VerifyPosition(injectiveApp, ctx, subaccountId, marketId, quantity, isLong)
	}

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedDerivativeLimitOrder {
		return testexchange.GetAllDerivativeOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSorted = func() []*types.TrimmedDerivativeLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	var getAllConditionalOrdersSorted = func(subaccountId common.Hash, marketId common.Hash) []*types.TrimmedDerivativeConditionalOrder {
		orders := k.GetAllSubaccountConditionalOrders(ctx, marketId, subaccountId)
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
	_ = getAllConditionalOrdersSorted

	var verifyWhetherOrderIsPostOnly = func(orderHash common.Hash, shouldBePostOnly bool) {
		orderFound := k.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketId, nil, mainSubaccountId, orderHash)
		if shouldBePostOnly {
			Expect(orderFound.GetOrderType().IsPostOnly()).To(BeTrue(), fmt.Sprintf("Order '%v' should be post-only", orderHash.String()))
		} else {
			Expect(orderFound.GetOrderType().IsPostOnly()).To(BeFalse(), fmt.Sprintf("Order '%v' should not be post-only", orderHash.String()))
		}
	}

	_ = verifyWhetherOrderIsPostOnly

	var f2d = testexchange.NewDecFromFloat
	var verifyOrder = testexchange.VerifyDerivativeOrder
	_ = verifyOrder

	var verifyFillableAndQuantity = func(orders []*types.TrimmedDerivativeLimitOrder, orderIdx int, quantity float64, fillable float64) {
		Expect(orders[orderIdx].Fillable.TruncateInt().String()).To(Equal(f2d(fillable).TruncateInt().String()), fmt.Sprintf("Fillable for order %d", orderIdx))
		Expect(orders[orderIdx].Quantity.TruncateInt().String()).To(Equal(f2d(quantity).TruncateInt().String()), fmt.Sprintf("Quantity for order %d", orderIdx))
	}
	_ = verifyFillableAndQuantity

	var verifyConditionalOrder = func(orders []*types.TrimmedDerivativeConditionalOrder, orderIdx int, triggerPrice float64, higher *bool, quantity float64, price float64, isReduceOnly bool) {
		Expect(orders[orderIdx].TriggerPrice.TruncateInt().String()).To(Equal(f2d(triggerPrice).TruncateInt().String()), fmt.Sprintf("Trigger price for order %d", orderIdx))

		//if higher != nil {
		//	Expect(orders[orderIdx].higher).To(BeEquivalentTo(*higher), fmt.Sprintf("Trigger 'higher' for order %d", orderIdx))
		//}
		if price > 0 {
			Expect(orders[orderIdx].Price.TruncateInt().String()).To(Equal(f2d(price).TruncateInt().String()), fmt.Sprintf("Conditional order price for order %d", orderIdx))
		}
		if quantity > 0 {
			Expect(orders[orderIdx].Quantity.TruncateInt().String()).To(Equal(f2d(quantity).TruncateInt().String()), fmt.Sprintf("Conditional order quantity for order %d", orderIdx))
		}
		if isReduceOnly {
			Expect(orders[orderIdx].Margin.IsZero()).To(BeTrue(), fmt.Sprintf("Order %d should be reduce-only", orderIdx))
		} else {
			Expect(orders[orderIdx].Margin.IsPositive()).To(BeTrue(), fmt.Sprintf("Order %d should be vanilla", orderIdx))
		}
	}
	_ = verifyConditionalOrder

	var getAvailableQuoteBalancePlusBank = func(accountIdx int) sdk.Dec {
		denom := testInput.Perps[0].Market.QuoteDenom
		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		balancesQuote := k.GetDeposit(ctx, subaccountID, denom).AvailableBalance
		bankBalance := sdk.ZeroInt()
		if types.IsDefaultSubaccountID(subaccountID) {
			accountAddr := types.SubaccountIDToSdkAddress(subaccountID)
			bankBalance = bankKeeper.GetBalance(ctx, accountAddr, denom).Amount
		}

		return balancesQuote.Add(bankBalance.ToDec())
	}

	_ = getAvailableQuoteBalancePlusBank

	var getTakerFee = func(marketId int) sdk.Dec {
		marketHexId := testInput.Perps[marketId].MarketID
		market := k.GetDerivativeMarketByID(ctx, marketHexId)

		return market.TakerFeeRate
	}

	_ = getTakerFee

	var verifyEstimateQuoteFeeDepositChange = func(accountIdx int, condOrders []*types.TrimmedDerivativeConditionalOrder, previousAvailableBalance sdk.Dec, previousTakerFee sdk.Dec) {
		marketId := testInput.Perps[0].MarketID
		market := k.GetDerivativeMarketByID(ctx, marketId)

		feePercDiff := market.TakerFeeRate.Sub(previousTakerFee)
		totalFeeChange := sdk.ZeroDec()

		for _, ord := range condOrders {
			//skip RO orders as there's no fee charged anyway
			if ord.Margin.IsZero() {
				continue
			}
			feeDiff := ord.Price.Mul(ord.Quantity).Mul(feePercDiff)
			totalFeeChange = totalFeeChange.Add(feeDiff)
		}

		expectedAvailable := previousAvailableBalance.Sub(totalFeeChange)

		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])

		currentAvailable := k.GetSpendableFunds(ctx, subaccountID, market.QuoteDenom)

		maxDiff := 1.0
		realDiff := currentAvailable.Sub(expectedAvailable).MustFloat64()
		Expect(math.Abs(realDiff) <= maxDiff).To(BeTrue(), fmt.Sprintf("Available balance change for account id '%v' should have changed by %v %v", accountIdx, realDiff, market.QuoteDenom))

		if feePercDiff.IsNegative() {
			Expect(previousAvailableBalance.MustFloat64() < currentAvailable.MustFloat64()).To(BeTrue(), fmt.Sprint("Available balance should have increased"))
		} else {
			Expect(previousAvailableBalance.MustFloat64() > currentAvailable.MustFloat64()).To(BeTrue(), fmt.Sprint("Available balance should have decreased"))
		}
	}

	_ = verifyEstimateQuoteFeeDepositChange

	var calculateTakerFeeForAllOrders = func(condOrders []*types.TrimmedDerivativeConditionalOrder, orders []*types.TrimmedDerivativeLimitOrder) sdk.Dec {
		marketId := testInput.Perps[0].MarketID
		market := k.GetDerivativeMarketByID(ctx, marketId)

		totalFee := sdk.ZeroDec()

		if condOrders != nil {
			for _, ord := range condOrders {
				//skip RO orders as there's no fee charged anyway
				if ord.Margin.IsZero() {
					continue
				}
				feeDiff := ord.Price.Mul(ord.Quantity).Mul(market.TakerFeeRate)
				totalFee = totalFee.Add(feeDiff)
			}
		}

		if orders != nil {
			for _, ord := range orders {
				//skip RO orders as there's no fee charged anyway
				if ord.Margin.IsZero() {
					continue
				}
				feeDiff := ord.Price.Mul(ord.Quantity).Mul(market.TakerFeeRate)
				totalFee = totalFee.Add(feeDiff)
			}
		}

		return totalFee.Abs()
	}

	_ = calculateTakerFeeForAllOrders

	var calculateMarginHoldForTakerOrder = func(price, quantity, margin sdk.Dec) sdk.Dec {
		marketId := testInput.Perps[0].MarketID
		market := k.GetDerivativeMarketByID(ctx, marketId)
		fee := price.Mul(quantity).Mul(market.TakerFeeRate)

		return margin.Add(fee)
	}

	_ = calculateMarginHoldForTakerOrder

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/conditionals", file)
		tp = testexchange.LoadReplayableTest(filepath)
		setup(tp)
		//TODO check why orderbook metadata invariant fails for conditionals and enable invariants checking
		simulationError = tp.ReplayTestWithLegacyHooks(testexchange.DoNotCheckInvariants, &hooks, nil)
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
			return fmt.Sprintf("p:%v(q:%v [f:%v]%v) side:%v", ord.Price.TruncateInt(), ord.Quantity.TruncateInt(), ord.Fillable.TruncateInt(), ro, side)
		}))
	}
	_ = printOrders

	printConditionalOrders := func(orders []*types.TrimmedDerivativeConditionalOrder) {
		fmt.Fprintln(GinkgoWriter, "Conditional orders: ", testexchange.GetReadableSlice(orders, " | ", func(ord *types.TrimmedDerivativeConditionalOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			stopType := ""
			if !ord.TriggerPrice.IsZero() {
				stopType = fmt.Sprintf(" tp: %v", ord.TriggerPrice.TruncateInt())
				//if ord.IsBuy {
				//	if ord.higher {
				//		stopType = " sl"
				//	} else {
				//		stopType = " tp"
				//	}
				//} else {
				//	if ord.higher {
				//		stopType = " tp"
				//	} else {
				//		stopType = " sl"
				//	}
				//}
			}
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v%v%v) tp:%v side:%v", ord.Price.TruncateInt(), ord.Quantity.TruncateInt(), ro, stopType, ord.TriggerPrice.TruncateInt(), side)
		}))
	}
	_ = printConditionalOrders

	Context("Conversion", func() {

		Context("CO.1 When during order creation trigger price is > current price and side is BUY order becomes a stop loss - (higher = false)", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("co1", true)
			})
			It("new order should be stop-loss", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 10, 9, false)

				newOrderMarginHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)
				expectedBalance := preSetupAvailableBalance.Sub(newOrderMarginHold)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("CO.2 When during order creation trigger price is < current price and side is BUY order becomes a take-profit - (higher = true)", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("co2", true)
			})
			It("new order should be take-profit", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 11, &expectTrue, 10, 12, false)

				newOrderMarginHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)
				expectedBalance := preSetupAvailableBalance.Sub(newOrderMarginHold)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("CO.3 When during order creation trigger price is > current price and side is SELL order becomes a take-profit - (higher = true)", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("co3", true)
			})
			It("new order should be take-profit", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 11, &expectTrue, 10, 11, false)

				newOrderMarginHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)
				expectedBalance := preSetupAvailableBalance.Sub(newOrderMarginHold)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("CO.4 When during order creation trigger price is < current price and side is SELL order becomes a stop-loss - (higher = false)", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("co4", true)
			})
			It("new order should be stop-loss", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 10, 10, false)

				newOrderMarginHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)
				expectedBalance := preSetupAvailableBalance.Sub(newOrderMarginHold)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})
	})

	Context("Limits", func() {

		Context("L.1 User can place 19 vanilla orders and 1 conditional on BUY side and 19 vanilla orders and 1 conditional on SELL side", func() {
			BeforeEach(func() {
				runTest("l1", true)
			})
			It("all should be allowed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(19))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 15, &expectTrue, 1, 0, false)
			})
		})

		Context("L.2 User cannot place 20 vanilla orders and 1 conditional on BUY side, but can place 1 conditional order on SELL side", func() {
			BeforeEach(func() {
				runTest("l2", false)
			})
			It("20 orders on one side should be allowed and one more on the other", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(20))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 6, &expectFalse, 1, 5, false)
			})
		})

		Context("L.3 User cannot place 20 vanilla orders and 1 conditional on SELL side, but can place 1 conditional order on BUY side", func() {
			BeforeEach(func() {
				runTest("l3", false)
			})
			It("20 orders on one side should be allowed and one more on the other", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(20))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 6, &expectFalse, 1, 5, false)
			})
		})

		Context("L.4 User can place 19 conditional orders and 1 vanilla on BUY side and 19 conditional orders and 1 vanilla on SELL side", func() {
			BeforeEach(func() {
				runTest("l4", true)
			})
			It("40 orders on both sides should be allowed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(38))
			})
		})

		Context("L.5 User can place 20 conditional orders on BUY side and 20 conditional orders on SELL side", func() {
			BeforeEach(func() {
				runTest("l5", true)
			})
			It("40 conditional orders on both sides should be allowed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(40))
			})
		})

		Context("L.6 User cannot place RO conditional TP order without open position", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("l6", false)
			})
			It("RO is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("L.6 User cannot place RO conditional SL order without open position", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("l6-sl", false)
			})
			It("RO is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("L.6A User can place RO conditional order if she has open vanilla order in the same market", func() {
			BeforeEach(func() {
				runTest("l6a", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		Context("L.6A User can place RO conditional TP order if she has open vanilla order in the same market", func() {
			BeforeEach(func() {
				runTest("l6a-tp", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		// Main purpose of such limitation was to prevent spam, so we can allow vanilla order on the other side
		Context("L.6AA User can place RO conditional TP order if she has open vanilla order in the same market, but on the same side", func() {
			BeforeEach(func() {
				runTest("l6aa", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		Context("L.6AA User can place RO conditional SL order if she has open vanilla order in the same market, but on the same side", func() {
			BeforeEach(func() {
				runTest("l6aa-sl", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		Context("L.6B User cannot place RO conditional SL order if she has open vanilla order in another market", func() {
			BeforeEach(func() {
				runTest("l6b", false)
			})
			It("RO is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.6B User cannot place RO conditional TP order if she has open vanilla order in another market", func() {
			BeforeEach(func() {
				runTest("l6b-tp", false)
			})
			It("RO is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.6C User cannot place RO conditional S: order if she has open position in another market", func() {
			BeforeEach(func() {
				runTest("l6c", false)
			})
			It("RO is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.6C User cannot place RO conditional TP order if she has open position in another market", func() {
			BeforeEach(func() {
				runTest("l6c-tp", false)
			})
			It("RO is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.7 User can place RO conditional SL order in the same direction as open position", func() {
			BeforeEach(func() {
				runTest("l7", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))

				verifyConditionalOrder(condOrders, 0, 14, nil, 1, 15, true)
			})
		})

		Context("L.7 User can place RO conditional order in the same direction as open position", func() {
			BeforeEach(func() {
				runTest("l7-short", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))

				verifyConditionalOrder(condOrders, 0, 14, nil, 1, 15, true)
			})
		})

		Context("L.7A User can place RO conditional buy order in the same direction as open position, but it will be rejected on trigger", func() {
			BeforeEach(func() {
				runTest("l7a", true)
			})
			It("RO is placed and then rejected", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				printConditionalOrders(condOrders)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.7A User can place RO conditional sell order in the same direction as open position, but it will be rejected on trigger", func() {
			BeforeEach(func() {
				runTest("l7a-short", true)
			})
			It("RO is placed and then rejected", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				printConditionalOrders(condOrders)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.8 User with long position can place conditional RO with Q > position Q", func() {
			BeforeEach(func() {
				runTest("l8", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 11, 9, true)
			})
		})

		Context("L.8 User with short position can place conditional RO with Q > position Q", func() {
			BeforeEach(func() {
				runTest("l8-short", true)
			})
			It("RO is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 11, 13, true)
			})
		})

		Context("L.9 User with long position can place conditional ROs with Q > position Q", func() {
			BeforeEach(func() {
				runTest("l9", true)
			})
			It("both RO are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 6, 9, true)
				verifyConditionalOrder(condOrders, 1, 10, &expectFalse, 5, 9, true)
			})
		})

		Context("L.9 User with short position can place conditional ROs with Q > position Q", func() {
			BeforeEach(func() {
				runTest("l9-short", true)
			})
			It("both RO are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 6, 14, true)
				verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 14, true)
			})
		})

		Context("L.10 User with long position can place conditional market order with quantity bigger than the size of the position", func() {
			BeforeEach(func() {
				runTest("l10", true)
			})
			It("market order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 11, 9, false)
			})
		})

		Context("L.10 User with short position can place conditional market order with quantity bigger than the size of the position", func() {
			BeforeEach(func() {
				runTest("l10-short", true)
			})
			It("market order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 11, 14, false)
			})
		})

		Context("L.11 User with long position can place conditional limit order with quantity bigger than the size of the position", func() {
			BeforeEach(func() {
				runTest("l11", true)
			})
			It("limit order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 11, 9, false)
			})
		})

		Context("L.11 User with short position can place conditional limit order with quantity bigger than the size of the position", func() {
			BeforeEach(func() {
				runTest("l11-short", true)
			})
			It("limit order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 11, 14, false)
			})
		})

		//no PO support for now
		Context("L.12 User can place conditional limit PO order with quantity bigger than the size of the position", func() {
			BeforeEach(func() {
				runTest("l12", true)
			})
			/*It("limit po order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 14, &expectTrue, 11, 15, false)
				verifyWhetherOrderIsPostOnly(common.HexToHash(condOrders[0].OrderHash), true)
			})*/
		})

		Context("L.13 User with long position can place conditional limit and market order in the same block (limit first)", func() {
			BeforeEach(func() {
				runTest("l13", true)
			})
			It("both orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 10, nil, 0, 9, false)
				verifyConditionalOrder(condOrders, 1, 10, nil, 0, 9, false)
			})
		})

		Context("L.13 User with short position can place conditional limit and market order in the same block (limit first)", func() {
			BeforeEach(func() {
				runTest("l13-short", true)
			})
			It("both orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 12, nil, 0, 13, false)
				verifyConditionalOrder(condOrders, 1, 12, nil, 0, 13, false)
			})
		})

		Context("L.13A User with long position can place conditional limit and market order in the same block (market first)", func() {
			BeforeEach(func() {
				runTest("l13a", true)
			})
			It("both orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 10, nil, 0, 9, false)
				verifyConditionalOrder(condOrders, 1, 10, nil, 0, 9, false)
			})
		})
		Context("L.13A User with short position can place conditional limit and market order in the same block (market first)", func() {
			BeforeEach(func() {
				runTest("l13a-short", true)
			})
			It("both orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 12, nil, 0, 14, false)
				verifyConditionalOrder(condOrders, 1, 12, nil, 0, 14, false)
			})
		})

		Context("L.14 conditional RO can be placed in same block as vanilla as long as vanilla is sent first", func() {
			BeforeEach(func() {
				runTest("l14", true)
			})
			It("both orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
			})
		})

		//shouldn't vanilla be placed anyway?
		Context("L.15 conditional RO cannot be placed in same block as vanilla if RO is sent first", func() {
			BeforeEach(func() {
				runTest("l15", false)
			})
			It("both orders are rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.16 Stop buy order is rejected if trigger price is below mark price", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("l16", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("L.17 Stop sell order is rejected if trigger price is above mark price", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("l17", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("L.18 Take buy order is rejected if trigger price is above mark price", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("l18", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("L.19 Take sell order is rejected if trigger price is below mark price", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("l19", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("L.20 Two market sell orders in different direction can be placed", func() {
			BeforeEach(func() {
				runTest("l20", true)
			})
			It("orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 9, &expectFalse, 5, 8, false)
				verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 5, 12, false)
			})
		})

		Context("L.21 Two market buy orders in different direction can be placed", func() {
			BeforeEach(func() {
				runTest("l21", true)
			})
			It("orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 9, &expectFalse, 5, 8, false)
				verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 5, 12, false)
			})
		})

		Context("L.22 User cannot place more than 1 market order above mark price", func() {
			BeforeEach(func() {
				runTest("l22", false)
			})
			It("second MO order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 1, 12, false)
			})
		})

		Context("L.22A User cannot place more than 1 market order above mark price", func() {
			BeforeEach(func() {
				runTest("l22-resting", false)
			})
			It("new MO order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 1, 12, false)
			})
		})

		Context("L.23 User cannot place more than 1 market order below mark price", func() {
			BeforeEach(func() {
				runTest("l23", false)
			})
			It("second MO order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 1, 10, false)
			})
		})

		Context("L.23A User cannot place more than 1 market order below mark price", func() {
			BeforeEach(func() {
				runTest("l23-resting", false)
			})
			It("new MO order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 1, 10, false)
			})
		})

		Context("L.24 User can place 2 market order above mark price on two different markets", func() {
			BeforeEach(func() {
				runTest("l24", true)
			})
			It("both MO orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 1, 12, false)

				condOrders = getAllConditionalOrdersSorted(mainSubaccountId, testInput.Perps[1].MarketID)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 1, 12, false)
			})
		})

		Context("L.25 User can place 2 market order below mark price on two different markets", func() {
			BeforeEach(func() {
				runTest("l25", true)
			})
			It("both MO orders are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 1, 10, false)

				condOrders = getAllConditionalOrdersSorted(mainSubaccountId, testInput.Perps[1].MarketID)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 10, &expectFalse, 1, 10, false)
			})
		})

		Context("L.26 User with insufficient balance cannot place a conditional order", func() {
			BeforeEach(func() {
				runTest("l26_insufficient_balance", false)
			})
			It("order is rejected", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("L.27 User with partially sufficient balance can place one conditional order", func() {
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					Expect(err).NotTo(BeNil())
				}
				runTest("l27_partially_insufficient_balance", false)
			})
			It("one order is rejected, one is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))

				verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 10, false)
			})
		})

		//one is placed one is rejected
	})

	Context("Order conversion", func() {

		Context("C.1 SL sell market order is converted into regular market order and filled if there is liquidity", func() {
			BeforeEach(func() {
				runTest("c1", true)
			})
			It("market order is matched", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.1 SL buy market order is converted into regular market order and filled if there is liquidity", func() {
			BeforeEach(func() {
				runTest("c1-short", true)
			})
			It("market order is matched", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.1a SL sell market order is converted into regular market order and filled if there is liquidity (same block version)", func() {
			BeforeEach(func() {
				runTest("c1a", true)
			})
			It("market order is matched", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.1a SL buy market order is converted into regular market order and filled if there is liquidity (same block version)", func() {
			BeforeEach(func() {
				runTest("c1a-short", true)
			})
			It("market order is matched", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.2 SL market order is converted into regular market order and rejected if there is no liquidity", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				hooks["post-setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					newOrderHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)

					expectedBalance := preSetupAvailableBalance.Sub(newOrderHold)
					Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				}
				runTest("c2", true)
			})
			It("market order is rejected matched", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("C.3 TP market order is converted into regular market order and filled if there is liquidity (long)", func() {
			BeforeEach(func() {
				runTest("c3", true)
			})
			It("market order is filled", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.3 TP market order is converted into regular market order and filled if there is liquidity (short)", func() {
			BeforeEach(func() {
				runTest("c3-short", true)
			})
			It("market order is filled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.4 TP sell market order is converted into regular market order and rejected is if there is no liquidity", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				hooks["post-setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					newOrderHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)

					expectedBalance := preSetupAvailableBalance.Sub(newOrderHold)
					Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				}
				runTest("c4", true)
			})
			It("market order is filled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				printConditionalOrders(condOrders)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("C.5 SL limit order is converted into regular limit order (long)", func() {
			var postSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("c5", true)
			})
			It("limit order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, false)
				orderFee := calculateTakerFeeForAllOrders(nil, orders)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 9, &expectFalse, 5, 8, false)

				expectedBalance := postSetupAvailableBalance.Add(orderFee)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("C.5 SL limit order is converted into regular limit order (short)", func() {
			var postSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("c5-short", true)
			})
			It("limit order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 13, false)
				orderFee := calculateTakerFeeForAllOrders(nil, orders)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 5, 14, false)

				expectedBalance := postSetupAvailableBalance.Add(orderFee)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("C.6 TP limit order is converted into regular limit order (long)", func() {
			var postSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("c6", true)
			})
			It("limit order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 13, false)
				orderFee := calculateTakerFeeForAllOrders(nil, orders)

				expectedBalance := postSetupAvailableBalance.Add(orderFee)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.6 TP limit order is converted into regular limit order (short)", func() {
			var postSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("c6-short", true)
			})
			It("limit order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 8, false)

				orderFee := calculateTakerFeeForAllOrders(nil, orders)

				expectedBalance := postSetupAvailableBalance.Add(orderFee)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.7 When conditional limit order with trigger price equal to current mark price is rejected", func() {
			BeforeEach(func() {
				runTest("c7", false)
			})
			It("limit order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.8 When conditional market order with trigger price equal to current mark price is rejected", func() {
			BeforeEach(func() {
				runTest("c8", false)
			})
			It("market order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.9 When conditional limit RO order with trigger price equal to current mark price is rejected", func() {
			BeforeEach(func() {
				runTest("c9", false)
			})
			It("limit RO order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				printConditionalOrders(condOrders)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.10 When conditional RO market order with trigger price equal to current mark price is rejected", func() {
			BeforeEach(func() {
				runTest("c10", false)
			})
			It("market RO order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.11 Conditional SL order is triggered when price goes below trigger price (long)", func() {
			BeforeEach(func() {
				runTest("c11", true)
			})
			It("order is triggered", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.11 Conditional SL order is triggered when price goes above trigger price (short)", func() {
			BeforeEach(func() {
				runTest("c11-short", true)
			})
			It("order is triggered", func() {
				verifyPosition(10, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 12, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.12 Conditional TP order is triggered when price goes above trigger price (long)", func() {
			BeforeEach(func() {
				runTest("c12", true)
			})
			It("order is triggered", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 12, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.12 Conditional TP order is triggered when price goes below trigger price (short)", func() {
			BeforeEach(func() {
				runTest("c12-short", true)
			})
			It("order is triggered", func() {
				verifyPosition(10, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.13 All matching TP/SL orders are triggered when price goes below/above trigger price", func() {
			BeforeEach(func() {
				hooks["-1"] = func(err error) {
					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(8))
				}
				hooks["1"] = func(err error) {
					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(3))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(5))
				}
				runTest("c13", true)
			})
			It("orders are triggered", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(6))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 6, &expectFalse, 5, 6, false)
				verifyConditionalOrder(condOrders, 1, 14, &expectTrue, 5, 14, false)
			})
		})

		Context("C.14 Conditional order with negative trigger price is rejected", func() {
			BeforeEach(func() {
				runTest("c14", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.15 Conditional order with zero trigger price is rejected", func() {
			BeforeEach(func() {
				runTest("c15", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("C.16 Conditional order RO order is not removed, when conditional vanilla order is triggered (long)", func() {
			BeforeEach(func() {
				runTest("c16", true)
			})
			It("RO order stays", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		Context("C.16 Conditional order RO order is not removed, when conditional vanilla order is triggered (short)", func() {
			BeforeEach(func() {
				runTest("c16-short", true)
			})
			It("RO order stays", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})
	})

	Context("Multiple orders", func() {

		Context("MO.1 Multiple market orders with the same trigger price cannot be added in the same block", func() {
			BeforeEach(func() {
				runTest("mo1", false)
			})
			It("market order is not placed", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		Context("MO.2a Multiple limit and market orders can be added in the same block with the same trigger price and when they are triggered market order is executed first (long)", func() {
			BeforeEach(func() {
				runTest("mo2a", true)
			})
			It("market order is filled, limit is partially filled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 4, 9, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.2b Multiple limit and market orders can be added in the same block with the same trigger price and when they are triggered market order is executed first (long)", func() {
			BeforeEach(func() {
				runTest("mo2b", true)
			})
			It("market order is filled, limit is partially filled", func() {
				verifyPosition(2, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 7, 10, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.2 Multiple limit and market orders can be added in the same block with the same trigger price and when they are triggered market order is executed first (long)", func() {
			BeforeEach(func() {
				runTest("mo2", true)
			})
			It("market order is filled, limit is partially filled", func() {
				verifyPosition(2, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 1, 9, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.2 Multiple limit and market orders can be added in the same block with the same trigger price and when they are triggered market order is executed first (short)", func() {
			BeforeEach(func() {
				runTest("mo2-short", true)
			})
			It("market order is filled, limit is partially filled", func() {
				verifyPosition(2, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 1, 12, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.3 Multiple limit orders can be added in the same block with the same trigger price and when they are triggered both are filled if there's liquidity (long)", func() {
			BeforeEach(func() {
				runTest("mo3", true)
			})
			It("limit orders are filled", func() {
				verifyPosition(7, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.3 Multiple limit orders can be added in the same block with the same trigger price and when they are triggered both are filled if there's liquidity (short)", func() {
			BeforeEach(func() {
				runTest("mo3-short", true)
			})
			It("limit orders are filled", func() {
				verifyPosition(7, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.4 When prices goes up and down it triggers conditional SL/TP orders as it moves (long)", func() {
			BeforeEach(func() {
				hooks["-1"] = func(err error) {
					verifyPosition(10, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(6))
				}
				hooks["1"] = func(err error) {
					verifyPosition(9, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(5))
				}
				hooks["2"] = func(err error) {
					verifyPosition(8, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(4))
				}
				hooks["3"] = func(err error) {
					verifyPosition(7, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
				}
				hooks["4"] = func(err error) {
					verifyPosition(6, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
				}
				hooks["5"] = func(err error) {
					verifyPosition(5, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["6"] = func(err error) {
					verifyPosition(4, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["7"] = func(err error) {
					verifyPosition(2, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
				}
				hooks["9"] = func(err error) {
					verifyPosition(2, true)
					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				runTest("mo4", true)
			})
			It("limit orders are filled as price changes", func() {
				verifyPosition(12, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.4 When prices goes up and down it triggers conditional SL/TP orders as it moves (short)", func() {
			BeforeEach(func() {
				hooks["-1"] = func(err error) {
					verifyPosition(10, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(6))
				}
				hooks["1"] = func(err error) {
					verifyPosition(9, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(5))
				}
				hooks["2"] = func(err error) {
					verifyPosition(8, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(4))
				}
				hooks["3"] = func(err error) {
					verifyPosition(7, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
				}
				hooks["4"] = func(err error) {
					verifyPosition(6, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
				}
				hooks["5"] = func(err error) {
					verifyPosition(5, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["6"] = func(err error) {
					verifyPosition(4, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["7"] = func(err error) {
					verifyPosition(2, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
				}
				hooks["9"] = func(err error) {
					verifyPosition(2, false)
					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				runTest("mo4-short", true)
			})
			It("limit orders are filled as price changes", func() {
				verifyPosition(12, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.4A When prices goes up and down it triggers conditional SL/TP RO/market/limit orders as it moves (long)", func() {
			BeforeEach(func() {
				hooks["-1"] = func(err error) {
					verifyPosition(10, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(6))
				}
				hooks["1"] = func(err error) {
					verifyPosition(9, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(5))
				}
				hooks["2"] = func(err error) {
					verifyPosition(8, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(4))
				}
				hooks["3"] = func(err error) {
					verifyPosition(7, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
				}
				hooks["4"] = func(err error) {
					verifyPosition(6, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
				}
				hooks["5"] = func(err error) {
					verifyPosition(5, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["6"] = func(err error) {
					verifyPosition(4, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["7"] = func(err error) {
					verifyPosition(2, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
				}
				hooks["8"] = func(err error) {
					verifyPosition(2, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				runTest("mo4a", true)
			})
			It("limit orders are filled as price changes", func() {
				verifyPosition(12, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.4A When prices goes up and down it triggers conditional SL/TP RO/market/limit orders as it moves (short)", func() {
			BeforeEach(func() {
				hooks["-1"] = func(err error) {
					verifyPosition(10, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(6))
				}
				hooks["1"] = func(err error) {
					verifyPosition(9, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(5))
				}
				hooks["2"] = func(err error) {
					verifyPosition(8, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(4))
				}
				hooks["3"] = func(err error) {
					verifyPosition(7, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
				}
				hooks["4"] = func(err error) {
					verifyPosition(6, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
				}
				hooks["5"] = func(err error) {
					verifyPosition(5, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["6"] = func(err error) {
					verifyPosition(4, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				hooks["7"] = func(err error) {
					verifyPosition(2, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
				}
				hooks["8"] = func(err error) {
					verifyPosition(2, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				runTest("mo4a-short", true)
			})
			It("limit orders are filled as price changes", func() {
				verifyPosition(12, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.5 User-initiated market order is processed and conditional MO is rejected if they were to be processed in the same block (long)", func() {
			BeforeEach(func() {
				runTest("mo5", true)
			})
			It("user's MO is filled", func() {
				verifyPosition(8, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.5 User-initiated market order is processed and conditional MO is rejected if they were to be processed in the same block (short)", func() {
			BeforeEach(func() {
				runTest("mo5-short", true)
			})
			It("user's MO is filled", func() {
				verifyPosition(8, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.6 Both User-initiated market order and conditional limit order are processed if they were to be processed in the same block (long)", func() {
			BeforeEach(func() {
				runTest("mo6", true)
			})
			It("both are filled", func() {
				verifyPosition(7, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.6 Both User-initiated market order and conditional limit order are processed if they were to be processed in the same block (short)", func() {
			BeforeEach(func() {
				runTest("mo6-short", true)
			})
			It("both are filled", func() {
				verifyPosition(7, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.7 Both User-initiated limit order and conditional market order are processed if they were to be processed in the same block (long)", func() {
			BeforeEach(func() {
				runTest("mo7", true)
			})
			It("both are filled", func() {
				verifyPosition(7, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.7 Both User-initiated limit order and conditional market order are processed if they were to be processed in the same block (short)", func() {
			BeforeEach(func() {
				runTest("mo7-short", true)
			})
			It("both are filled", func() {
				verifyPosition(7, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.8 Both User-initiated limit order and conditional limit order are processed if they were to be processed in the same block (long)", func() {
			BeforeEach(func() {
				runTest("mo8", true)
			})
			It("both are filled", func() {
				verifyPosition(7, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.8 Both User-initiated limit order and conditional limit order are processed if they were to be processed in the same block (short)", func() {
			BeforeEach(func() {
				runTest("mo8-short", true)
			})
			It("both are filled", func() {
				verifyPosition(7, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.9 User-initiated market order is processed, conditional limit is processed and conditional MO is rejected if they were to be processed in the same block (long)", func() {
			BeforeEach(func() {
				runTest("mo9", true)
			})
			It("user's MO and conditional limit is filled", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.9 User-initiated market order is processed, conditional limit is processed and conditional MO is rejected if they were to be processed in the same block (short)", func() {
			BeforeEach(func() {
				runTest("mo9-short", true)
			})
			It("user's MO and conditional limit is filled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.10 Non-conditional limit order is processed before triggered conditional order", func() {
			BeforeEach(func() {
				runTest("mo10", true)
			})
			It("regular order goes first", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyFillableAndQuantity(orders, 0, 2, 1)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MO.11 Market order margin is returned if it cannot be filled, because another order drained liquidity", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(1)
				}
				runTest("mo11_margin_returned", true)
			})
			It("margin is returned", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(preSetupAvailableBalance.String()))
			})
		})
	})

	//no PO support for now
	Context("Post-only variations", func() {
		/*Context("PO.1 Post-only conditional order can be added, but it is placed only if there's no matching opposite order once trigger price is reached", func() {
			BeforeEach(func() {
				runTest("po1", true)
			})
			It("po order is placed", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 1, 9, false)
				verifyWhetherOrderIsPostOnly(common.BytesToHash(([]byte(orders[0].OrderHash))), true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PO.2 Post-only conditional order can be added, but is not placed if it would have matched once trigger price is reached", func() {
			BeforeEach(func() {
				runTest("po2", true)
			})
			It("po order is not placed", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PO.3 Post-only conditional order is executed after non-post-only if both have the same limit and trigger price", func() {
			BeforeEach(func() {
				runTest("po3", true)
			})
			It("po order is placed after non-po is executed", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, false)
				verifyWhetherOrderIsPostOnly(common.BytesToHash(([]byte(orders[0].OrderHash))), true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})*/
	})

	Context("Reduce-only variations", func() {

		Context("ROV.1 conditional market RO order is filled if there's liquidity (long)", func() {
			var preAvailableBalance sdk.Dec
			var postAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-ro"] = func(err error) {
					preAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				// make sure no margin is locked when market RO order is placed
				hooks["post-ro"] = func(err error) {
					postAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					Expect(preAvailableBalance.String()).To(Equal(postAvailableBalance.String()))
				}
				runTest("rov1", true)
			})
			It("market order is filled", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.1 conditional market RO order is filled if there's liquidity (short)", func() {
			BeforeEach(func() {
				runTest("rov1-short", true)
			})
			It("market order is filled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.2 conditional market RO order is rejected if there is no liquidity", func() {
			BeforeEach(func() {
				runTest("rov2", true)
			})
			It("market order is rejected", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.3 conditional market RO order is partially filled if there's some liquidity (long)", func() {
			BeforeEach(func() {
				runTest("rov3", true)
			})
			It("market order is filled partially", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.3 conditional market RO order is partially filled if there's some liquidity (short)", func() {
			BeforeEach(func() {
				runTest("rov3-short", true)
			})
			It("market order is filled partially", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.4 conditional market RO order is filled before conditional limit RO order (long)", func() {
			BeforeEach(func() {
				runTest("rov4", true)
			})
			It("market order is filled, limit becomes resting", func() {
				verifyPosition(6, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 6, 9, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.4 conditional market RO order is filled before conditional limit RO order (short)", func() {
			BeforeEach(func() {
				runTest("rov4-short", true)
			})
			It("market order is filled, limit becomes resting", func() {
				verifyPosition(6, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 6, 13, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.5 conditional market RO order is always filled before conditional limit RO order even if limit order has better price (long)", func() {
			BeforeEach(func() {
				runTest("rov5", true)
			})
			It("market order and better limit order is filled", func() {
				verifyPosition(3, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 3, 8, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.5 conditional market RO order is always filled before conditional limit RO order even if limit order has better price (short)", func() {
			BeforeEach(func() {
				runTest("rov5-short", true)
			})
			It("market order and better limit order is filled", func() {
				verifyPosition(3, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 3, 13, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.6 conditional market RO order is filled before conditional limit RO order and limit orders are filled in random order", func() {
			BeforeEach(func() {
				runTest("rov6", true)
			})
			It("market order and one limit order is filled, last one is invalidated", func() {
				verifyPosition(1, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
				//verifyOrder(orders, 0, 1, 9, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))

			})
		})

		//no PO support for now
		Context("ROV.7 conditional market RO order is filled before conditional limit RO order if there's some liquidity and non-PO limit order is filled first", func() {
			BeforeEach(func() {
				runTest("rov7", true)
			})
			/*It("po limit is placed", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 8, true)
				verifyWhetherOrderIsPostOnly(common.BytesToHash(([]byte(orders[0].OrderHash))), true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})*/
		})

		//no PO support for now
		Context("ROV.8 conditional market RO order is filled before conditional limit RO order if there's some liquidity and non-PO limit order is filled first and PO is cancelled if it can be filled", func() {
			BeforeEach(func() {
				runTest("rov8", true)
			})
			/*It("po limit is canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})*/
		})

		Context("ROV.9 conditional limit RO order is resized, when it's size is above position size and trigger price has been reached (long)", func() {
			var preAvailableBalance sdk.Dec
			var postAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-ro"] = func(err error) {
					preAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				// make sure no margin is locked when market RO order is placed
				hooks["post-ro"] = func(err error) {
					postAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					Expect(preAvailableBalance.String()).To(Equal(postAvailableBalance.String()))
				}
				runTest("rov9", true)
			})
			It("RO is resized", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 10, 9, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.9 conditional limit RO order is resized, when it's size is above position size and trigger price has been reached (short)", func() {
			BeforeEach(func() {
				runTest("rov9-short", true)
			})
			It("RO is resized", func() {
				verifyPosition(10, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 10, 13, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.10 conditional limit RO with better price is filled first (long)", func() {
			BeforeEach(func() {
				runTest("rov10", true)
			})
			It("better priced limit order is filled first", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.10 conditional limit RO with better price is filled first (short)", func() {
			BeforeEach(func() {
				runTest("rov10-short", true)
			})
			It("better priced limit order is filled first", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 13, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		//no PO support for now
		Context("ROV.11 conditional limit non-PO RO order is filled first if both have same price", func() {
			BeforeEach(func() {
				runTest("rov11", true)
			})
			/*It("non PO limit order is filled first", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, true)
				verifyWhetherOrderIsPostOnly(common.BytesToHash(([]byte(orders[0].OrderHash))), true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})*/
		})

		Context("ROV.12 conditional limit RO is rejected if resting one has better price and there's no quantity left (long)", func() {
			BeforeEach(func() {
				runTest("rov12", true)
			})
			It("RO is rejected", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 7, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("ROV.12 conditional limit RO is rejected if resting one has better price and there's no quantity left (short)", func() {
			BeforeEach(func() {
				runTest("rov12-short", true)
			})
			It("RO is rejected", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 16, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})
	})

	Context("From different accounts", func() {

		Context("FDA.1 Conditional orders with the same trigger price are filled if they cross", func() {
			BeforeEach(func() {
				runTest("fda1", true)
			})
			It("crossing conditional orders are matched", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("FDA.2 Conditional limit and market orders with the same trigger price become resting even if they cross", func() {
			BeforeEach(func() {
				runTest("fda2", true)
			})
			It("crossing conditional orders are not matched", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 9, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("FDA.3 Conditional market orders with the same trigger price do not match", func() {
			BeforeEach(func() {
				runTest("fda3", true)
			})
			It("crossing conditional orders are not matched", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("FDA.4 Conditional market orders with the same trigger price do not match", func() {
			BeforeEach(func() {
				runTest("fda4", true)
			})
			It("orders match and position is opened", func() {
				verifyPosition(6, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 4, 10, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

	})

	Context("Oracle price", func() {
		Context("OP.1 Conditional SL order is matched only if committed oracle price eclipses it", func() {
			BeforeEach(func() {
				runTest("op1", true)
			})
			It("conditional order is not triggered", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 9, &expectFalse, 5, 9, true)
			})
		})

		Context("OP.2 Conditional TP order is matched only if committed oracle price eclipses it", func() {
			BeforeEach(func() {
				runTest("op2", true)
			})
			It("conditional order is not triggered", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 5, 13, true)
			})
		})
	})

	Context("Position liquidation", func() {

		Context("PL.1 When position is liquidated RO orders are canceled", func() {
			BeforeEach(func() {
				runTest("pl1", true)
			})
			It("orders are canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PL.2 When position is liquidated but have enough deposit to liquidate, still all orders are cancelled", func() {
			BeforeEach(func() {
				hooks["before-liq"] = func(err error) {
					orders := getAllOrdersSorted()
					fmt.Println("Orders: ", len(orders))
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					fmt.Println("Cond orders: ", len(condOrders))
				}
				runTest("pl2", true)
			})
			It("all orders are cancelled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PL.3 When position is liquidated RO orders are canceled even if trigger price was reached", func() {
			BeforeEach(func() {
				runTest("pl3", true)
			})
			It("RO orders are canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PL.5 When position is closed RO orders are canceled", func() {
			BeforeEach(func() {
				runTest("pl5", true)
			})
			It("orders are canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				printConditionalOrders(condOrders)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PL.6 When position is closed RO orders are not cancelled as vanilla order stays", func() {
			BeforeEach(func() {
				runTest("pl6", true)
			})
			It("orders are not canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
			})
		})

		Context("PL.7 When position is closed RO orders are canceled even if trigger price was reached", func() {
			BeforeEach(func() {
				runTest("pl7", true)
			})
			It("RO orders are canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PL.8 When position is liquidated with not enough deposit to liquidate, RO and vanilla orders are cancelled", func() {
			BeforeEach(func() {
				runTest("pl8", true)
			})
			It("all orders are canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("PL.9 When position is liquidated and insurance fund is drained all RO and vanilla orders are cancelled", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["orders_placed"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(1)
				}
				runTest("pl9", false)
			})
			It("market is wiped clean", func() {
				market := k.GetDerivativeMarketByID(ctx, market.MarketID())
				Expect(market.GetMarketStatus()).To(Equal(types.MarketStatus_Paused), "market wasn't paused")
				verifyPosition(0, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				secondAccountSubaccountId := common.HexToHash(accounts[1].SubaccountIDs[0])
				verifyOtherPosition(0, true, secondAccountSubaccountId)

				secondAccountOrders := getAllOrdersSortedForAccount(secondAccountSubaccountId)
				Expect(len(secondAccountOrders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))

				secondAccountCondOrders := getAllConditionalOrdersSorted(secondAccountSubaccountId, marketId)
				Expect(len(secondAccountCondOrders)).To(Equal(0))

				Expect(getAvailableQuoteBalancePlusBank(1).MustFloat64() > preSetupAvailableBalance.MustFloat64()).To(BeTrue())
			})
		})

		Context("PL.10 When position is liquidated and user has no funds, but has resting orders they are cancelled to provide missing funds", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["orders_placed"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(1)
				}
				runTest("pl10_cancel_conditionals_for_missing_funds", false)
			})
			It("but market remains active and other users' orders are untouched", func() {
				if testexchange.IsUsingDefaultSubaccount() {
					Skip("app doesn't work like that for default subaccount")
				}
				market := k.GetDerivativeMarketByID(ctx, market.MarketID())
				Expect(market.GetMarketStatus()).To(Equal(types.MarketStatus_Active), "market was paused")
				verifyPosition(0, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				secondAccountSubaccountId := common.HexToHash(accounts[1].SubaccountIDs[0])
				verifyOtherPosition(0, true, secondAccountSubaccountId)

				secondAccountOrders := getAllOrdersSortedForAccount(secondAccountSubaccountId)
				Expect(len(secondAccountOrders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))

				secondAccountCondOrders := getAllConditionalOrdersSorted(secondAccountSubaccountId, marketId)
				Expect(len(secondAccountCondOrders)).To(Equal(2))

				Expect(getAvailableQuoteBalancePlusBank(1).MustFloat64() > preSetupAvailableBalance.MustFloat64()).To(BeTrue())
			})
		})
	})

	Context("Order cancelling using MsgCancelDerivativeOrder", func() {

		Context("OC.1 Conditional vanilla limit order can be canceled", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("oc1", true)
			})
			It("order is canceled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("OC.1A Conditional vanilla market order can be canceled", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("oc1a", true)
			})
			It("order is canceled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("OC.2 Conditional RO order can be canceled", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("oc2", true)
			})
			It("order is canceled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("OC.3 User can cancel any conditional order she wants", func() {
			BeforeEach(func() {
				hooks["orders_created"] = func(err error) {
					verifyPosition(5, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
					verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 2, 7, true)
					verifyConditionalOrder(condOrders, 1, 11, &expectTrue, 3, 12, true)
					verifyConditionalOrder(condOrders, 2, 13, &expectTrue, 5, 14, false)
				}
				hooks["order1_canceled"] = func(err error) {
					verifyPosition(5, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
					verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 2, 7, true)
					verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 14, false)
				}
				hooks["order2_canceled"] = func(err error) {
					verifyPosition(5, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 5, 14, false)
				}
				runTest("oc3", true)
			})
			It("orders are canceled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("OC.4 User can cancel multiple conditional orders in the same block (without batch)", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("oc4", true)
			})
			It("orders are canceled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("OC.5 User will cancel a conditional order before it's converted even if trigger price is eclipsed in the same block", func() {
			BeforeEach(func() {
				hooks["beforeCancel2"] = func(err error) {
					verifyPosition(5, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 5, 14, false)
				}
				runTest("oc5", true)
			})
			It("orders are canceled", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("OC.6 Cancelling the only open vanilla order closes all open conditional RO orders", func() {
			BeforeEach(func() {
				hooks["1"] = func(err error) {
					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
				}
				runTest("oc6", true)
			})
			It("RO orders are canceled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("OC.7 Order cancelling doesn't close any open conditional RO orders if the position is still open", func() {
			BeforeEach(func() {
				runTest("oc7", true)
			})
			It("RO order is not canceled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 2, 7, true)
			})
		})

		Context("OC.8 When existing RO order is cancelled conditional one can be placed", func() {
			BeforeEach(func() {
				runTest("oc8", true)
			})
			It("RO order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 4, 8, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})
	})

	Context("Batch update", func() {
		Context("BU.1 User can create a single conditional order using batch message", func() {
			BeforeEach(func() {
				runTest("bu1", true)
			})
			It("order is placed", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("BU.2 User can create multiple conditional orders", func() {
			BeforeEach(func() {
				hooks["1"] = func(err error) {
					verifyPosition(0, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
					verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 5, 7, true)
					verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 14, false)
					verifyConditionalOrder(condOrders, 2, 19, &expectTrue, 5, 20, true)
				}
				hooks["2"] = func(err error) {
					verifyPosition(5, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
					verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 5, 7, true)
					verifyConditionalOrder(condOrders, 1, 19, &expectTrue, 5, 20, true)
				}
				runTest("bu2", true)
			})
			It("order are filled", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 7, true)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 19, &expectTrue, 5, 20, true)
			})
		})

		Context("BU.2A User can create multiple conditional RO orders if first order in batch is a vanilla order", func() {
			BeforeEach(func() {
				runTest("bu2a", true)
			})
			It("order are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 5, 14, false)

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 5, 7, true)
				verifyConditionalOrder(condOrders, 1, 19, &expectTrue, 5, 20, true)
			})
		})

		Context("BU.2B User cannot create conditional RO orders if there is no margin locked", func() {
			BeforeEach(func() {
				runTest("bu2b", true)
			})
			It("order are placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("BU.2C User can create conditional RO orders if he has an open long position", func() {
			BeforeEach(func() {
				runTest("bu2c", true)
			})
			It("order are placed", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 5, 7, true)
				verifyConditionalOrder(condOrders, 1, 19, &expectTrue, 5, 20, true)
			})
		})

		Context("BU.2D User can create conditional RO orders if he has an open short position", func() {
			BeforeEach(func() {
				runTest("bu2d", true)
			})
			It("order are placed", func() {
				verifyPosition(5, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 5, 7, true)
				verifyConditionalOrder(condOrders, 1, 19, &expectTrue, 5, 20, true)
			})
		})

		Context("BU.3 User can cancel a single conditional order", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bu3", true)
			})
			It("order is canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BU.4 User can cancel multiple conditional orders", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bu4", true)
			})
			It("some orders are canceled", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 9, &expectFalse, 3, 8, false)
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BU.4A When user cancels all existing orders inside batch then she cannot place any more conditional RO", func() {
			BeforeEach(func() {
				runTest("bu4a", true)
			})
			It("RO orders are rejected", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("BU.5 User can create a single conditional order and cancel another one", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bu5", true)
			})
			It("order is canceled and created", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 3, 5, true)

				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BU.6 User can create multiple RO conditional orders while cancelling multiple ones", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bu6", true)
			})
			It("order is canceled and created", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(3))
				verifyConditionalOrder(condOrders, 0, 3, &expectFalse, 7, 2, true)
				verifyConditionalOrder(condOrders, 1, 7, &expectFalse, 2, 5, false)
				verifyConditionalOrder(condOrders, 2, 8, &expectFalse, 3, 5, true)

				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BU.6A User can create multiple mixed conditional orders while cancelling multiple ones", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bu6a", true)
			})
			It("order is canceled and created", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(3))
				verifyConditionalOrder(condOrders, 0, 3, &expectFalse, 7, 2, false)
				verifyConditionalOrder(condOrders, 1, 7, &expectFalse, 2, 5, false)
				verifyConditionalOrder(condOrders, 2, 8, &expectFalse, 3, 5, true)

				newOrderHold := calculateMarginHoldForTakerOrder(condOrders[0].Price, condOrders[0].Quantity, condOrders[0].Margin)
				expectedBalance := preSetupAvailableBalance.Sub(newOrderHold)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BU.7 Canceled orders are not triggered, when mark price eclipses trigger price", func() {
			BeforeEach(func() {
				runTest("bu7", true)
			})
			It("old conditional orders are canceled and resting order is not filled, new conditionals created", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 3, &expectFalse, 7, 2, true)
				verifyConditionalOrder(condOrders, 1, 8, &expectTrue, 3, 5, false)
			})
		})

		Context("BU.8 Batch update with conditional RO first, skips the RO and places vanilla", func() {
			BeforeEach(func() {
				runTest("bu8", true)
			})
			It("vanilla is placed", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 3, 5, false)
			})
		})

		Context("BU.9 User can batch cancel all orders in the market and create some new ones", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				hooks["1"] = func(err error) {
					verifyPosition(10, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(2))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
				}
				runTest("bu9", true)
			})
			It("all orders are cancelled and placed", func() {
				verifyPosition(10, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(3))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 5, 7, true)
				verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 14, false)
				verifyConditionalOrder(condOrders, 2, 19, &expectTrue, 5, 20, true)

				newOrderHold := calculateMarginHoldForTakerOrder(condOrders[1].Price, condOrders[1].Quantity, condOrders[1].Margin)
				expectedBalance := preSetupAvailableBalance.Sub(newOrderHold)
				Expect(expectedBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})
	})

	Context("Batch cancel", func() {

		Context("BC.1 User can cancel a single conditional order", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bc1_single", true)
			})
			It("order is canceled", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BC.2 User can cancel a multiple conditional orders", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bc2_multiple", true)
			})
			It("some orders are canceled", func() {
				verifyPosition(5, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 8, &expectFalse, 2, 8, false)

				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})

		Context("BC.3 Canceled orders are not triggered, when mark price eclipses trigger price", func() {
			var preSetupAvailableBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(err error) {
					preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
				}
				runTest("bc3_too_late", true)
			})
			It("position is not created", func() {
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))

				Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
			})
		})
	})

	Describe("Order Mask", func() {
		countOrderTypes := func() (buyOrHigher, sellOrLower, regular, conditional, market, limit uint8) {
			markPrice := sdk.NewDec(10)

			market += uint8(len(k.GetAllSubaccountDerivativeMarketOrdersByMarketDirection(ctx, marketId, mainSubaccountId, true)))
			market += uint8(len(k.GetAllSubaccountDerivativeMarketOrdersByMarketDirection(ctx, marketId, mainSubaccountId, false)))
			for _, order := range getAllOrdersSorted() {
				if order.IsBuy {
					buyOrHigher++
				} else {
					sellOrLower++
				}
				regular++
				limit++
			}
			for _, order := range getAllConditionalOrdersSorted(mainSubaccountId, marketId) {
				if order.TriggerPrice.GT(markPrice) {
					buyOrHigher++
				} else {
					sellOrLower++
				}
				if order.IsLimit {
					limit++
				} else {
					market++
				}
				conditional++
			}
			return
		}
		Context("OM.1 Different kind of order masks work as expected", func() {
			BeforeEach(func() {
				hooks["nBRL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(3)))
					Expect(sell).To(Equal(uint8(3)))
					Expect(regular).To(Equal(uint8(2)))
					Expect(conditional).To(Equal(uint8(4)))
					Expect(market).To(Equal(uint8(2)))
					Expect(limit).To(Equal(uint8(4)))
				}
				hooks["yBRL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(3)))
					Expect(regular).To(Equal(uint8(1)))
					Expect(conditional).To(Equal(uint8(4)))
					Expect(market).To(Equal(uint8(2)))
					Expect(limit).To(Equal(uint8(3)))
				}
				hooks["nSRL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(3)))
					Expect(regular).To(Equal(uint8(1)))
					Expect(conditional).To(Equal(uint8(4)))
					Expect(market).To(Equal(uint8(2)))
					Expect(limit).To(Equal(uint8(3)))
				}
				hooks["ySRL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(2)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(4)))
					Expect(market).To(Equal(uint8(2)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["nLCM"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(2)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(4)))
					Expect(market).To(Equal(uint8(2)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["yLCM"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(1)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(3)))
					Expect(market).To(Equal(uint8(1)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["nHCM"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(1)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(3)))
					Expect(market).To(Equal(uint8(1)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["yHCM"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(1)))
					Expect(sell).To(Equal(uint8(1)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(2)))
					Expect(market).To(Equal(uint8(0)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["nLCL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(1)))
					Expect(sell).To(Equal(uint8(1)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(2)))
					Expect(market).To(Equal(uint8(0)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["yLCL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(1)))
					Expect(sell).To(Equal(uint8(0)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(1)))
					Expect(market).To(Equal(uint8(0)))
					Expect(limit).To(Equal(uint8(1)))
				}
				runTest("om1", true)
			})
			It("cancellation happens correctly with different order masks", func() {
				buy, sell, regular, conditional, market, limit := countOrderTypes()
				Expect(buy).To(Equal(uint8(0)))
				Expect(sell).To(Equal(uint8(0)))
				Expect(regular).To(Equal(uint8(0)))
				Expect(conditional).To(Equal(uint8(0)))
				Expect(market).To(Equal(uint8(0)))
				Expect(limit).To(Equal(uint8(0)))
			})
		})

		Context("OM.2 Conditional orders can be cancelled with any mask", func() {
			BeforeEach(func() {
				hooks["LCM"] = func(err error) {
					printConditionalOrders(getAllConditionalOrdersSorted(mainSubaccountId, marketId))
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(2)))
					Expect(sell).To(Equal(uint8(1)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(3)))
					Expect(market).To(Equal(uint8(1)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["HCM"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(1)))
					Expect(sell).To(Equal(uint8(1)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(2)))
					Expect(market).To(Equal(uint8(0)))
					Expect(limit).To(Equal(uint8(2)))
				}
				hooks["LCL"] = func(err error) {
					buy, sell, regular, conditional, market, limit := countOrderTypes()
					Expect(buy).To(Equal(uint8(1)))
					Expect(sell).To(Equal(uint8(0)))
					Expect(regular).To(Equal(uint8(0)))
					Expect(conditional).To(Equal(uint8(1)))
					Expect(market).To(Equal(uint8(0)))
					Expect(limit).To(Equal(uint8(1)))
				}
				runTest("om2", true)
			})
			It("order is cancelled", func() {
				buy, sell, regular, conditional, market, limit := countOrderTypes()
				Expect(buy).To(Equal(uint8(0)))
				Expect(sell).To(Equal(uint8(0)))
				Expect(regular).To(Equal(uint8(0)))
				Expect(conditional).To(Equal(uint8(0)))
				Expect(market).To(Equal(uint8(0)))
				Expect(limit).To(Equal(uint8(0)))
			})
		})
	})

	Context("Market fee update", func() {
		Context("MFU.1 When taker fee increases only limit vanilla orders are impacted", func() {
			var afterSetupBalance sdk.Dec
			var initialTakerFee sdk.Dec
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					afterSetupBalance = getAvailableQuoteBalancePlusBank(0)
					initialTakerFee = getTakerFee(0)
				}
				runTest("mfu1", true)
			})
			It(" higher deposit is reserved", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))

				verifyEstimateQuoteFeeDepositChange(0, condOrders, afterSetupBalance, initialTakerFee)
			})
		})

		Context("MFU.1A When taker fee increases only vanilla market orders are impacted", func() {
			var afterSetupBalance sdk.Dec
			var initialTakerFee sdk.Dec
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					afterSetupBalance = getAvailableQuoteBalancePlusBank(0)
					initialTakerFee = getTakerFee(0)
				}
				runTest("mfu1", true)
			})
			It(" higher deposit is reserved", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))

				verifyEstimateQuoteFeeDepositChange(0, condOrders, afterSetupBalance, initialTakerFee)
			})
		})

		Context("MFU.2 When taker fee decreases only vanilla orders are impacted", func() {
			var afterSetupBalance sdk.Dec
			var initialTakerFee sdk.Dec
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					afterSetupBalance = getAvailableQuoteBalancePlusBank(0)
					initialTakerFee = getTakerFee(0)
				}
				runTest("mfu2", true)
			})
			It("part of deposit is returned", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))

				verifyEstimateQuoteFeeDepositChange(0, condOrders, afterSetupBalance, initialTakerFee)
			})
		})

		Context("MFU.3 When taker fee increases all types of conditional vanilla orders are impacted", func() {
			var afterSetupBalance sdk.Dec
			var initialTakerFee sdk.Dec
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					afterSetupBalance = getAvailableQuoteBalancePlusBank(0)
					initialTakerFee = getTakerFee(0)
				}
				runTest("mfu3", true)
			})
			It(" higher deposit is reserved", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(4))

				verifyEstimateQuoteFeeDepositChange(0, condOrders, afterSetupBalance, initialTakerFee)
			})
		})

		Context("MFU.3A When taker fee increases market orders are equally impacted", func() {
			var afterSetupBalance sdk.Dec
			var initialTakerFee sdk.Dec
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					afterSetupBalance = getAvailableQuoteBalancePlusBank(0)
					initialTakerFee = getTakerFee(0)
				}
				runTest("mfu3a", true)
			})
			It("higher deposit is reserved", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(3))

				verifyEstimateQuoteFeeDepositChange(0, condOrders, afterSetupBalance, initialTakerFee)
			})
		})

		Context("MFU.4 When taker fee increases and user doesn't have any balance left", func() {
			BeforeEach(func() {
				runTest("mfu4", true)
			})
			It("one conditional order is canceled", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(1))
			})
		})

		Context("MFU.5 When taker fee increases and user doesn't have any balance left all vanilla order is cancelled, but RO stays due to existing position", func() {
			BeforeEach(func() {
				runTest("mfu5", true)
			})
			It("RO order stays", func() {
				verifyPosition(10, true)
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				printConditionalOrders(condOrders)
				Expect(len(condOrders)).To(Equal(1))
				verifyConditionalOrder(condOrders, 0, 199, &expectTrue, 100, 200, true)
			})
		})

		Context("MFU.6 When taker fee increases and user doesn't have any balance left vanilla and RO orders are cancelled", func() {
			BeforeEach(func() {
				runTest("mfu6", true)
			})
			It("RO order stays", func() {
				verifyPosition(0, true)
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MFU.7 When taker fee increases and user doesn't have any balance left his first vanilla orders is cancelled and RO stays", func() {
			BeforeEach(func() {
				runTest("mfu7", true)
			})
			It("RO and one vanilla order stays", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 110, &expectTrue, 100, 100, false)
				verifyConditionalOrder(condOrders, 1, 199, &expectTrue, 100, 200, true)
			})
		})

		Context("MFU.8 When maker fee increases and user doesn't have any balance left no orders are canceled", func() {
			BeforeEach(func() {
				runTest("mfu8", true)
			})
			It("no order is cancelled", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(2))
				verifyConditionalOrder(condOrders, 0, 110, &expectTrue, 100, 100, false)
				verifyConditionalOrder(condOrders, 1, 199, &expectTrue, 100, 200, true)
			})
		})
	})

	Context("Inactivated market", func() {
		Context("Paused", func() {
			Context("IM.1 No conditional orders are canceled", func() {
				var postSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(10))
						postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					runTest("im1-paused", true)
				})
				It("no conditional orders are canceled", func() {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(10))
					Expect(postSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.1A No conditional vanilla orders are canceled", func() {
				var postSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(6))
						postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					runTest("im1a-paused", true)
				})
				It("all conditional orders are canceled", func() {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(6))
					Expect(postSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.1B No conditional RO orders are canceled", func() {
				var postSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["setup"] = func(err error) {
						verifyPosition(10, true)

						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(2))
						postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					runTest("im1b-paused", true)
				})
				It("no conditional orders are canceled", func() {
					verifyPosition(10, true)
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(2))
					Expect(postSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.2 No orders are canceled and positions are closed", func() {
				var postSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["setup"] = func(err error) {
						verifyPosition(10, true)

						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(3))
						postSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					runTest("im2-paused", true)
				})
				It("nothing is canceled", func() {
					verifyPosition(10, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(3))
					Expect(postSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})
		})

		Context("Demolished", func() {
			Context("IM.1 All conditional orders are canceled", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(10))

					}
					runTest("im1-demolished", true)
				})
				It("all conditional orders are canceled", func() {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.1A All conditional vanilla orders are canceled", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(6))
					}
					runTest("im1a-demolished", true)
				})
				It("all conditional orders are canceled", func() {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.1B All conditional RO orders are canceled", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						verifyPosition(10, true)
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(2))
					}
					runTest("im1b-demolished", true)
				})
				It("all conditional orders are canceled", func() {
					verifyPosition(10, true)
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)

					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.2 All orders are canceled and positions are closed", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						verifyPosition(10, true)

						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(3))
					}
					runTest("im2-demolished", true)
				})
				It("all is canceled", func() {
					verifyPosition(10, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})
		})

		Context("Expired", func() {
			Context("IM.1 All conditional orders are canceled", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(10))
					}
					runTest("im1-expired", true)
				})
				It("all conditional orders are canceled", func() {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.1A All conditional vanilla orders are canceled", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(6))
					}
					runTest("im1a-expired", true)
				})
				It("all conditional orders are canceled", func() {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.1B All conditional RO orders are canceled", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						verifyPosition(10, true)
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(2))
					}
					runTest("im1b-expired", true)
				})
				It("all conditional orders are canceled", func() {
					verifyPosition(10, true)
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})

			Context("IM.2 All orders are canceled and positions are closed", func() {
				var preSetupAvailableBalance sdk.Dec
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						preSetupAvailableBalance = getAvailableQuoteBalancePlusBank(0)
					}
					hooks["setup"] = func(err error) {
						verifyPosition(10, true)

						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(3))
					}
					runTest("im2-expired", true)
				})
				It("all is canceled", func() {
					verifyPosition(10, true)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(0))
					Expect(preSetupAvailableBalance.String()).To(Equal(getAvailableQuoteBalancePlusBank(0).String()))
				})
			})
		})
	})

	Context("Margin and mark price validations", func() {

		Context("MM.1 Trigger price is used when validating margin ratios for limit buy orders", func() {
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 5000, &expectTrue, 20, 5000, false)
				}
				runTest("mm1_limit_buy_valid", true)
			})
			It("order is placed and later triggered", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 20, 5000, false)
			})
		})

		Context("MM.1A Trigger price is used when validating margin ratios for limit buy orders", func() {
			BeforeEach(func() {
				runTest("mm1_limit_buy_invalid", false)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MM.2 Trigger price is used when validating margin ratios for limit sell orders", func() {
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 5000, &expectTrue, 20, 5000, false)
				}
				runTest("mm2_limit_sell_valid", true)
			})
			It("order is placed and later triggered", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 20, 5000, false)
			})
		})

		Context("MM.2A Trigger price is used when validating margin ratios for limit sell orders", func() {
			BeforeEach(func() {
				runTest("mm2_limit_sell_invalid", false)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MM.3 Trigger price is used when validating margin ratios for market buy orders", func() {
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 5000, &expectTrue, 20, 5000, false)
				}
				runTest("mm3_market_buy_valid", true)
			})
			It("order is placed and later triggered", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))

				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				verifyPosition(20, true)
			})
		})

		Context("MM.3A Trigger price is used when validating margin ratios for market buy orders", func() {
			BeforeEach(func() {
				runTest("mm3_market_buy_invalid", false)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})

		Context("MM.4 Trigger price is used when validating margin ratios for market sell orders", func() {
			BeforeEach(func() {
				hooks["setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 5000, &expectTrue, 20, 5000, false)
				}
				runTest("mm4_market_sell_valid", true)
			})
			It("order is placed and later triggered", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))

				verifyPosition(20, false)
			})
		})

		Context("MM.4A Trigger price is used when validating margin ratios for market sell orders", func() {
			BeforeEach(func() {
				runTest("mm4_market_sell_invalid", false)
			})
			It("order is rejected", func() {
				condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
				Expect(len(condOrders)).To(Equal(0))
			})
		})
	})

	Context("Margin freeing", func() {

		Context("Margin is freed instantly, when conditional limit order is triggered and regular is added to the orderbook", func() {
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 10, false)
					// currently we cannot withdraw everything, because withdrawal amount needs to be an int and sometimes there's some dust left
					Expect(getAvailableQuoteBalancePlusBank(0).MustFloat64() < sdk.MustNewDecFromStr("1").MustFloat64()).To(BeTrue())
				}
				runTest("mf1_limit_order", true)
			})
			It("order is placed even if there's no free margin", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 10, 10, false)
			})
		})

		Context("Margin is freed instantly, when conditional market order is triggered and regular is added to the orderbook", func() {
			BeforeEach(func() {
				hooks["post-setup"] = func(err error) {
					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 10, false)
					// currently we cannot withdraw everything, because withdrawal amount needs to be an int and sometimes there's some dust left
					Expect(getAvailableQuoteBalancePlusBank(0).MustFloat64() < sdk.MustNewDecFromStr("1").MustFloat64()).To(BeTrue())
				}
				runTest("mf2_market_order", true)
			})
			It("order is placed even if there's no free margin", func() {
				verifyPosition(10, true)
			})
		})
	})

	Describe("Manual testing", func() {
		var (
			msgServer    types.MsgServer
			triggerPrice = startingPrice.Add(sdk.NewDec(1))
			quantity     = sdk.NewDec(1)
			err          error
		)
		Context("Market orders", func() {
			var (
				order     types.MsgCreateDerivativeMarketOrder
				orderHash common.Hash
			)
			BeforeEach(func() {
				msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

				order = types.MsgCreateDerivativeMarketOrder{
					Sender: senderBuyer.String(),
					Order: types.DerivativeOrder{
						MarketId: marketId.String(),
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer.Hex(),
							FeeRecipient: senderBuyer.String(),
							Price:        triggerPrice,
							Quantity:     quantity,
						},
						OrderType:    types.OrderType_STOP_BUY,
						Margin:       sdk.ZeroDec(),
						TriggerPrice: &triggerPrice,
					},
				}
			})
			JustBeforeEach(func() {
				err = order.ValidateBasic()
				if err != nil {
					return
				}

				resp, createErr := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), &order)
				err = createErr
				if resp != nil {
					orderHash = common.HexToHash(resp.OrderHash)
				}
			})

			Describe("validity checks", func() {
				When("reduce only and no margin locked", func() {
					It("should error with no margin locked error", func() {
						Expect(err).Should(MatchError(types.ErrNoMarginLocked))
					})
				})
				When("reduce only and margin is locked", func() {
					BeforeEach(func() {
						buyOrder := testInput.NewMsgCreateDerivativeLimitOrder(triggerPrice, quantity, margin, types.OrderType_BUY, subaccountIdBuyer)
						_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), buyOrder)
						testexchange.OrFail(err)
					})
					It("should not error", func() {
						Expect(err).To(BeNil())
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.ReduceOnlyConditionalOrderCount).To(Equal(uint32(1)))
					})
				})
				When("vanilla", func() {
					BeforeEach(func() {
						order.Order.Margin = margin
					})
					It("should not error and hold supplied margin", func() {
						Expect(err).To(BeNil())
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(1)))

						depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())
						Expect(deposit.AvailableBalance.Sub(depositAfter.AvailableBalance).GTE(margin)).To(BeTrue()) // marginHold = margin + fees
					})
				})
				When("TriggerPrice is incorrect", func() {
					BeforeEach(func() {
						order.Order.TriggerPrice = nil
					})
					It("should error with incorrect trigger price", func() {
						Expect(err).Should(MatchError(types.ErrInvalidTriggerPrice))
					})
				})
				When("TriggerPrice does not match order type", func() {
					BeforeEach(func() {
						lowerTriggerPrice := startingPrice.Sub(sdk.OneDec())
						order.Order.TriggerPrice = &lowerTriggerPrice
						order.Order.OrderType = types.OrderType_TAKE_SELL
					})
					It("should error with incorrect trigger price", func() {
						Expect(err).Should(MatchError(types.ErrInvalidTriggerPrice))
					})
				})
				When("exceed conditional market orders per side limit", func() {
					BeforeEach(func() {
						order.Order.Margin = margin // allow order to go through
					})
					It("should reject second market order on same side (isHigher flag)", func() {
						Expect(err).To(BeNil())
						_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), &order)
						Expect(err).Should(MatchError(types.ErrConditionalMarketOrderAlreadyExists))
					})
					It("should allow second market order on the opposite side (isHigher flag)", func() {
						Expect(err).To(BeNil())

						lowerPrice := startingPrice.Sub(sdk.OneDec())
						order.Order.TriggerPrice = &lowerPrice
						order.Order.OrderType = types.OrderType_TAKE_BUY

						_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), &order)
						Expect(err).To(BeNil())
					})
				})
			})

			Describe("cancellation tests", func() {
				When("vanilla order cancelled", func() {
					BeforeEach(func() {
						order.Order.Margin = margin
					})
					It("should not error and release locked margin", func() {
						depositBefore := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())

						err = app.ExchangeKeeper.CancelConditionalDerivativeMarketOrder(ctx, market, subaccountIdBuyer, nil, orderHash)
						Expect(err).To(BeNil())

						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(0)))

						depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())
						Expect(depositAfter.AvailableBalance.Sub(depositBefore.AvailableBalance).GTE(margin)).To(BeTrue())
					})
				})
				When("market is settled", func() {
					var (
						depositBefore *types.Deposit
					)
					BeforeEach(func() {
						order.Order.Margin = margin
					})
					JustBeforeEach(func() {
						depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())

						order.Order.Margin = sdk.ZeroDec()
						msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), &order)

						marketSettlementInfo := &types.DerivativeMarketSettlementInfo{
							MarketId:        marketId.String(),
							SettlementPrice: startingPrice,
						}
						app.ExchangeKeeper.SetDerivativesMarketScheduledSettlementInfo(ctx, marketSettlementInfo)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx) // settlement happens here
						// ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					})
					It("should cancel both order and release margin from cancelled vanilla conditional order", func() {
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(0)))
						Expect(metadata.ReduceOnlyConditionalOrderCount).To(Equal(uint32(0)))

						depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())
						Expect(depositAfter.AvailableBalance.Sub(depositBefore.AvailableBalance).GTE(margin)).To(BeTrue())
					})
				})
			})

			Describe("triggering", func() {
				BeforeEach(func() {
					order.Order.Margin = margin // make order valid, vanilla

					// opposite side resting order to be matched
					sellerLimitOrder := testInput.NewMsgCreateDerivativeLimitOrder(triggerPrice, quantity, margin, types.OrderType_SELL, subaccountIdSeller)
					_, sellerErr := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), sellerLimitOrder)
					testexchange.OrFail(sellerErr)
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})
				When("Triggering one order on one side", func() {
					JustBeforeEach(func() {
						app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(triggerPrice, ctx.BlockTime().Unix()))
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					})
					It("should leave 0 conditional orders and 1 position", func() {
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(0)))

						position := app.ExchangeKeeper.GetPosition(ctx, marketId, subaccountIdBuyer)
						Expect(position).ToNot(BeNil())
						Expect(position.Quantity.String()).To(Equal(quantity.String()))
					})
				})
			})

			Describe("genesis export/import", func() {
				var (
					state       *types.GenesisState
					marketOrder *types.DerivativeMarketOrder
				)
				BeforeEach(func() {
					order.Order.Margin = margin // make order valid, vanilla
				})
				JustBeforeEach(func() {
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					state = app.ExchangeKeeper.ExportGenesis(ctx)
					marketOrder = state.ConditionalDerivativeOrderbooks[0].GetMarketOrders()[0]
				})
				It("should have 1 market conditional in GenesisState", func() {
					Expect(len(state.ConditionalDerivativeOrderbooks)).To(Equal(1))
					Expect(len(state.ConditionalDerivativeOrderbooks[0].GetMarketOrders())).To(Equal(1))
					derivativeOrder := marketOrder.ToDerivativeOrder(marketId.String())
					Expect(*derivativeOrder).To(Equal(order.Order))
				})
				It("should have 2 market conditionals in storage after import", func() {
					app.ExchangeKeeper.InitGenesis(ctx, *state)
					metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
					Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(2)))
				})
			})
		})

		Context("Limit orders", func() {
			var (
				order     types.MsgCreateDerivativeLimitOrder
				orderHash common.Hash
			)
			BeforeEach(func() {
				msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

				order = types.MsgCreateDerivativeLimitOrder{
					Sender: senderBuyer.String(),
					Order: types.DerivativeOrder{
						MarketId: marketId.String(),
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer.Hex(),
							FeeRecipient: senderBuyer.String(),
							Price:        triggerPrice,
							Quantity:     quantity,
						},
						OrderType:    types.OrderType_STOP_BUY,
						Margin:       sdk.ZeroDec(),
						TriggerPrice: &triggerPrice,
					},
				}
			})
			JustBeforeEach(func() {
				err = order.ValidateBasic()
				if err != nil {
					return
				}
				resp, createErr := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &order)
				err = createErr
				if resp != nil {
					orderHash = common.HexToHash(resp.OrderHash)
				}
			})

			Describe("validity checks", func() {
				When("reduce only and no margin locked", func() {
					It("should error with no margin locked error", func() {
						Expect(err).Should(MatchError(types.ErrNoMarginLocked))
					})
				})
				When("reduce only and margin is locked", func() {
					BeforeEach(func() {
						buyOrder := testInput.NewMsgCreateDerivativeLimitOrder(triggerPrice, quantity, margin, types.OrderType_BUY, subaccountIdBuyer)
						_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), buyOrder)
						testexchange.OrFail(err)
					})
					It("should not error", func() {
						Expect(err).To(BeNil())
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.ReduceOnlyConditionalOrderCount).To(Equal(uint32(1)))
					})
				})
				When("vanilla", func() {
					BeforeEach(func() {
						order.Order.Margin = margin
					})
					It("should not error and hold supplied margin", func() {
						Expect(err).To(BeNil())
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(1)))

						depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())
						Expect(deposit.AvailableBalance.Sub(depositAfter.AvailableBalance).GTE(margin)).To(BeTrue()) // marginHold = margin + fees
					})
				})
				When("TriggerPrice is incorrect", func() {
					BeforeEach(func() {
						order.Order.TriggerPrice = nil
					})
					It("should error with incorrect trigger price", func() {
						Expect(err).Should(MatchError(types.ErrInvalidTriggerPrice))
					})
				})
				When("TriggerPrice does not match order type", func() {
					BeforeEach(func() {
						order.Order.OrderType = types.OrderType_STOP_SELL
					})
					It("should error with incorrect trigger price", func() {
						Expect(err).Should(MatchError(types.ErrInvalidTriggerPrice))
					})
				})
				When("no limits on conditional limit orders per side", func() {
					BeforeEach(func() {
						order.Order.Margin = margin // allow order to go through
					})
					It("should allow second limit order", func() {
						Expect(err).To(BeNil())
						_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &order)
						Expect(err).To(BeNil())
					})
				})
			})

			Describe("cancellation tests", func() {
				When("vanilla", func() {
					BeforeEach(func() {
						order.Order.Margin = margin
					})
					It("should not error and release locked margin", func() {
						depositBefore := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())

						err = app.ExchangeKeeper.CancelConditionalDerivativeLimitOrder(ctx, market, subaccountIdBuyer, nil, orderHash)
						Expect(err).To(BeNil())

						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(0)))

						depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdBuyer, market.GetQuoteDenom())
						Expect(depositAfter.AvailableBalance.Sub(depositBefore.AvailableBalance).GTE(margin)).To(BeTrue())
					})
				})
			})

			Describe("triggering", func() {
				BeforeEach(func() {
					order.Order.Margin = margin // make order valid, vanilla
				})
				When("Triggering one order on one side", func() {
					JustBeforeEach(func() {
						app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(triggerPrice, ctx.BlockTime().Unix()))
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					})
					It("should leave 0 conditional orders and 1 vanilla limit order", func() {
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(0)))
						Expect(metadata.VanillaLimitOrderCount).To(Equal(uint32(1)))
					})
				})
				When("Triggering two orders on lower side", func() {
					JustBeforeEach(func() {
						markPrice := startingPrice.Sub(sdk.NewDec(5))
						order.Order.OrderInfo.Price = markPrice
						order.Order.OrderType = types.OrderType_TAKE_BUY

						triggerPrice1 := startingPrice.Sub(sdk.OneDec())
						order.Order.TriggerPrice = &triggerPrice1
						_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &order)
						testexchange.OrFail(err)

						triggerPrice2 := triggerPrice1.Sub(sdk.OneDec())
						order.Order.TriggerPrice = &triggerPrice2
						_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &order)
						testexchange.OrFail(err)

						triggerPrice3 := triggerPrice2.Sub(sdk.OneDec())
						order.Order.TriggerPrice = &triggerPrice3
						_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &order)
						testexchange.OrFail(err)

						app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(triggerPrice2, ctx.BlockTime().Unix()))
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					})
					It("should leave 1 isHigher and 1 isLower conditional orders and 2 triggered isLower vanilla limit orders", func() {
						metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
						Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(2)))
						Expect(metadata.VanillaLimitOrderCount).To(Equal(uint32(2)))
					})
				})
			})

			Describe("genesis export/import", func() {
				var (
					state      *types.GenesisState
					limitOrder *types.DerivativeLimitOrder
				)
				BeforeEach(func() {
					order.Order.Margin = margin // make order valid, vanilla
				})
				JustBeforeEach(func() {
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					state = app.ExchangeKeeper.ExportGenesis(ctx)
					limitOrder = state.ConditionalDerivativeOrderbooks[0].GetLimitOrders()[0]
				})
				It("should have 1 limit conditional in GenesisState", func() {
					Expect(len(state.ConditionalDerivativeOrderbooks)).To(Equal(1))
					Expect(len(state.ConditionalDerivativeOrderbooks[0].GetLimitOrders())).To(Equal(1))
					derivativeOrder := limitOrder.ToDerivativeOrder(marketId.String())
					Expect(*derivativeOrder).To(Equal(order.Order))
				})
				It("should have 2 limit conditionals in storage after import", func() {
					app.ExchangeKeeper.InitGenesis(ctx, *state)
					metadata := app.ExchangeKeeper.GetSubaccountOrderbookMetadata(ctx, marketId, subaccountIdBuyer, true)
					Expect(metadata.VanillaConditionalOrderCount).To(Equal(uint32(2)))
				})
			})
		})
	})

	//they require manually breaking the code so that it panics in specific place in "injective-chain/modules/exchange/abci.go" crash
	Context("Recovey tests", func() {

		/* add these lines to TriggerConditionalDerivativeLimitOrder()
		if marketOrder.GetQuantity().String() == sdk.MustNewDecFromStr("10").String() {
			fmt.Printf("CRASHING order: p:%v (q:%v) tp:%v isBuy: %v\n", marketOrder.Price().String(), marketOrder.GetQuantity().String(), marketOrder.TriggerPrice.String(), marketOrder.IsBuy())
			panic("mietek")
		}
		*/
		/*Context("When code panics during processing limit orders", func() {

			// make sure that order with Q=5 and P=12 and TP=12 fails
			Context("And first limit order fails, then all conditional market orders are still processed (RT.1)", func() {
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(3))
						verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 12, false)
						verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 5, 12, false)
						verifyConditionalOrder(condOrders, 2, 13, &expectTrue, 5, 13, false)
					}
					runTest("rt1_first_limit_panics", true)
				})
				It("other limit and all market orders are processed", func() {
					verifyPosition(14, false)
					verifyOtherPosition(17, true, common.HexToHash(accounts[1].SubaccountIDs[0]))
					verifyOtherPosition(3, false, common.HexToHash(accounts[2].SubaccountIDs[0]))

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					printConditionalOrders(condOrders)

					Expect(len(condOrders)).To(Equal(2))

					verifyConditionalOrder(condOrders, 0, 8, &expectTrue, 6, 8, false)
					verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 13, false)
				})
			})

			// make sure that order with Q=10 and P=12 and TP=12 fails
			Context("And second limit order fails, then all conditional market orders are still processed (RT.1)", func() {
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(3))
						verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 12, false)
						verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 5, 12, false)
						verifyConditionalOrder(condOrders, 2, 13, &expectTrue, 5, 13, false)
					}
					runTest("rt1_second_limit_panics", true)
				})
				It("other limit and all market orders are processed", func() {
					verifyPosition(9, false)
					verifyOtherPosition(12, true, common.HexToHash(accounts[1].SubaccountIDs[0]))
					verifyOtherPosition(3, false, common.HexToHash(accounts[2].SubaccountIDs[0]))

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					printConditionalOrders(condOrders)

					Expect(len(condOrders)).To(Equal(2))

					verifyConditionalOrder(condOrders, 0, 8, &expectTrue, 6, 8, false)
					verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 13, false)
				})
			})

			// make sure that order with Q=10 and P=12 and TP=12 fails
			Context("Then non-conditional orders are still processed normally (RT.2)", func() {
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(2))
						verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 12, false)
						verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 13, false)
					}
					runTest("rt2_limit_panics_again", true)
				})
				It("normal orders are processed", func() {
					verifyPosition(8, false)

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					printConditionalOrders(condOrders)

					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 5, 13, false)
				})
			})
		})

		/* add these lines to TriggerConditionalDerivativeMarketOrder()
		if marketOrder.GetQuantity().String() == sdk.MustNewDecFromStr("10").String() {
			fmt.Printf("CRASHING order: p:%v (q:%v) tp:%v isBuy: %v\n", marketOrder.Price().String(), marketOrder.GetQuantity().String(), marketOrder.TriggerPrice.String(), marketOrder.IsBuy())
			panic("mietek")
		}
		*/

		/*Context("When code panics during processing market orders", func() {

			// make sure that order with Q=4 and P=12 and TP=12 fails
			Context("And first market order fails, then all conditional market orders are still processed (RT.3)", func() {
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(5))
						verifyConditionalOrder(condOrders, 0, 8, &expectTrue, 6, 8, false)
						verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 10, 12, false)
						verifyConditionalOrder(condOrders, 2, 12, &expectTrue, 5, 12, false)
						verifyConditionalOrder(condOrders, 3, 12, &expectTrue, 4, 12, false)
						verifyConditionalOrder(condOrders, 4, 13, &expectTrue, 5, 13, false)
					}
					runTest("rt3_first_market_panics", true)
				})
				It("market orders are processed", func() {
					verifyPosition(15, false)
					verifyOtherPosition(20, true, common.HexToHash(accounts[1].SubaccountIDs[0]))
					verifyOtherPosition(5, false, common.HexToHash(accounts[2].SubaccountIDs[0]))

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					printConditionalOrders(condOrders)
					Expect(len(condOrders)).To(Equal(2))

					verifyConditionalOrder(condOrders, 0, 8, &expectTrue, 6, 8, false)
					verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 13, false)
				})
			})

			// make sure that order with Q=5 and P=12 and TP=12 fails
			Context("And second market order fails, then all conditional market orders are still processed (RT.3)", func() {
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(5))
						verifyConditionalOrder(condOrders, 0, 8, &expectTrue, 6, 8, false)
						verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 10, 12, false)
						verifyConditionalOrder(condOrders, 2, 12, &expectTrue, 5, 12, false)
						verifyConditionalOrder(condOrders, 3, 12, &expectTrue, 4, 12, false)
						verifyConditionalOrder(condOrders, 4, 13, &expectTrue, 5, 13, false)
					}
					runTest("rt3_second_market_panics", true)
				})
				It("market orders are processed", func() {
					verifyPosition(19, false)
					verifyOtherPosition(19, true, common.HexToHash(accounts[1].SubaccountIDs[0]))
					verifyOtherPosition(0, false, common.HexToHash(accounts[2].SubaccountIDs[0]))

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					printConditionalOrders(condOrders)
					Expect(len(condOrders)).To(Equal(2))

					verifyConditionalOrder(condOrders, 0, 8, &expectTrue, 6, 8, false)
					verifyConditionalOrder(condOrders, 1, 13, &expectTrue, 5, 13, false)
				})
			})

			// make sure that order with Q=7 and P=12 and TP=12 fails
			Context("Then non-conditional orders are still processed normally (RT.4)", func() {
				BeforeEach(func() {
					hooks["pre-setup"] = func(err error) {
						condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
						Expect(len(condOrders)).To(Equal(3))
						verifyConditionalOrder(condOrders, 0, 12, &expectTrue, 10, 12, false)
						verifyConditionalOrder(condOrders, 1, 12, &expectTrue, 7, 12, false)
						verifyConditionalOrder(condOrders, 2, 13, &expectTrue, 5, 13, false)
					}
					runTest("rt4_market_panics_again", true)
				})
				It("normal orders are processed", func() {
					verifyPosition(18, false)
					verifyOtherPosition(18, true, common.HexToHash(accounts[1].SubaccountIDs[0]))

					orders := getAllOrdersSorted()
					Expect(len(orders)).To(Equal(0))

					condOrders := getAllConditionalOrdersSorted(mainSubaccountId, marketId)
					printConditionalOrders(condOrders)

					Expect(len(condOrders)).To(Equal(1))
					verifyConditionalOrder(condOrders, 0, 13, &expectTrue, 5, 13, false)
				})
			})
		})*/
	})
})
