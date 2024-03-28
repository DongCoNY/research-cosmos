package keeper_test

import (
	"strconv"
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

type testSpotOrder struct {
	price           string
	quantity        string
	expectedFill    string
	subaccountNonce string
	index           int
}

type spotBuySellOrders struct {
	buys                            []testSpotOrder
	sells                           []testSpotOrder
	expectedMarketBuyClearingPrice  string
	expectedMarketSellClearingPrice string
	expectedLimitClearingPrice      string
}

func expectSpotMarketOrderExpansion(
	orders []testSpotOrder,
	expectedClearingPrice sdk.Dec,
	marketExpansions *exchangetypes.EventBatchSpotExecution,
	subaccountIDs []common.Hash,
	baseBalanceDeltas exchangetypes.DepositDeltas,
	quoteBalanceDeltas exchangetypes.DepositDeltas,
	spotMarket exchangetypes.SpotMarket,
) sdk.Dec {
	Expect(len(marketExpansions.Trades)).To(Not(BeZero()))

	totalAuctionFees := sdk.ZeroDec()

	for i, trade := range marketExpansions.Trades {
		expectedSubaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orders[i].subaccountNonce)
		expectedQuantity, _ := sdk.NewDecFromStr(orders[i].quantity)
		expectedFillQuantity, _ := sdk.NewDecFromStr(orders[i].expectedFill)
		receivedSubaccountID := common.BytesToHash(trade.SubaccountId)

		Expect(trade.Price.String()).To(Equal(expectedClearingPrice.String()))
		Expect(trade.Quantity.String()).To(Equal(expectedFillQuantity.String()))
		Expect(receivedSubaccountID.Hex()).To(Equal(expectedSubaccountID.String()))

		baseDelta := baseBalanceDeltas[receivedSubaccountID]
		quoteDelta := quoteBalanceDeltas[receivedSubaccountID]

		var expectedBaseAvailableBalanceDelta, expectedBaseTotalBalanceDelta, expectedQuoteAvailableBalanceDelta, expectedQuoteTotalBalanceDelta sdk.Dec

		if marketExpansions.IsBuy {
			expectedBaseAvailableBalanceDelta = expectedFillQuantity
			expectedBaseTotalBalanceDelta = expectedFillQuantity

			bought := expectedFillQuantity.Mul(expectedClearingPrice)
			fees := bought.Mul(spotMarket.TakerFeeRate)
			totalSpent := bought.Add(fees).Neg()

			totalAuctionFees = totalAuctionFees.Add(fees.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate)))

			expectedQuoteAvailableBalanceDelta = totalSpent
			expectedQuoteTotalBalanceDelta = totalSpent
		} else {
			expectedBaseAvailableBalanceDelta = expectedQuantity.Sub(expectedFillQuantity)
			expectedBaseTotalBalanceDelta = expectedFillQuantity.Neg()

			bought := expectedFillQuantity.Mul(expectedClearingPrice)
			fees := bought.Mul(spotMarket.TakerFeeRate)
			totalReceived := bought.Sub(fees)

			totalAuctionFees = totalAuctionFees.Add(fees.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate)))

			expectedQuoteAvailableBalanceDelta = totalReceived
			expectedQuoteTotalBalanceDelta = totalReceived
		}

		Expect(baseDelta.AvailableBalanceDelta.String()).To(Equal(expectedBaseAvailableBalanceDelta.String()))
		Expect(baseDelta.TotalBalanceDelta.String()).To(Equal(expectedBaseTotalBalanceDelta.String()))
		Expect(quoteDelta.AvailableBalanceDelta.String()).To(Equal(expectedQuoteAvailableBalanceDelta.String()))
		Expect(quoteDelta.TotalBalanceDelta.String()).To(Equal(expectedQuoteTotalBalanceDelta.String()))
	}

	Expect(marketExpansions.ExecutionType).To(Equal(exchangetypes.ExecutionType_Market))
	Expect(marketExpansions.MarketId).To(Equal(spotMarket.MarketId))

	return totalAuctionFees
}

func expectSpotLimitOrderExpansion(
	orders []testSpotOrder,
	limitExpansions []*exchangetypes.SpotLimitOrderDelta,
	subaccountIDs []common.Hash,
	baseBalanceDeltas exchangetypes.DepositDeltas,
	quoteBalanceDeltas exchangetypes.DepositDeltas,
	spotMarket exchangetypes.SpotMarket,
	clearingPrice, tradingFeeRate sdk.Dec,
) sdk.Dec {
	Expect(len(limitExpansions)).To(Not(BeZero()))
	totalAuctionFees := sdk.ZeroDec()

	for i, expansion := range limitExpansions {
		expectedSubaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orders[i].subaccountNonce)
		expectedQuantity, _ := sdk.NewDecFromStr(orders[i].quantity)
		expectedOrderPrice, _ := sdk.NewDecFromStr(orders[i].price)
		expectedFillQuantity, _ := sdk.NewDecFromStr(orders[i].expectedFill)
		receivedSubaccountID := common.HexToHash(expansion.Order.OrderInfo.SubaccountId)

		Expect(expansion.FillQuantity.String()).To(Equal(expectedFillQuantity.String()))
		Expect(expansion.Order.OrderInfo.Quantity.String()).To(Equal(expectedQuantity.String()))
		Expect(expansion.Order.OrderInfo.Price.String()).To(Equal(expectedOrderPrice.String()))
		Expect(expansion.Order.OrderInfo.SubaccountId).To(Equal(expectedSubaccountID.String()))

		baseDelta := baseBalanceDeltas[receivedSubaccountID]
		quoteDelta := quoteBalanceDeltas[receivedSubaccountID]

		var expectedBaseAvailableBalanceDelta, expectedBaseTotalBalanceDelta, expectedQuoteAvailableBalanceDelta, expectedQuoteTotalBalanceDelta sdk.Dec

		if expansion.Order.OrderType.IsBuy() {
			expectedBaseAvailableBalanceDelta = expectedFillQuantity
			expectedBaseTotalBalanceDelta = expectedFillQuantity

			bought := expectedFillQuantity.Mul(expectedOrderPrice)
			totalTradeFees := bought.Mul(tradingFeeRate)

			if tradingFeeRate.IsNegative() {
				feesPaidToMaker := totalTradeFees.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate)).Abs()

				totalSpent := bought.Sub(feesPaidToMaker).Neg()
				totalAuctionFees = totalAuctionFees.Add(totalTradeFees)

				expectedQuoteAvailableBalanceDelta = feesPaidToMaker
				expectedQuoteTotalBalanceDelta = totalSpent
			} else {
				totalSpent := bought.Add(totalTradeFees).Neg()
				totalAuctionFees = totalAuctionFees.Add(totalTradeFees.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate)))

				expectedQuoteAvailableBalanceDelta = sdk.ZeroDec()

				if clearingPrice.IsPositive() {
					bought := expectedFillQuantity.Mul(clearingPrice)
					totalTradeFees := bought.Mul(tradingFeeRate)
					totalSpent = bought.Add(totalTradeFees).Neg()

					priceDelta := clearingPrice.Sub(expectedOrderPrice)
					refundNotional := priceDelta.Mul(expectedFillQuantity)
					matchedFeeRefund := sdk.MaxDec(spotMarket.MakerFeeRate, sdk.ZeroDec()).Mul(refundNotional)

					expectedQuoteAvailableBalanceDelta = refundNotional.Add(matchedFeeRefund).Neg()
				}

				expectedQuoteTotalBalanceDelta = totalSpent
			}
		} else {
			expectedBaseAvailableBalanceDelta = sdk.ZeroDec()
			expectedBaseTotalBalanceDelta = expectedFillQuantity.Neg()

			bought := expectedFillQuantity.Mul(expectedOrderPrice)
			totalTradeFees := bought.Mul(tradingFeeRate)

			var totalReceived sdk.Dec

			if tradingFeeRate.IsNegative() {
				feesPaidToMaker := totalTradeFees.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate)).Abs()
				totalReceived = bought.Add(feesPaidToMaker)
				totalAuctionFees = totalAuctionFees.Add(totalTradeFees)
			} else {
				totalReceived = bought.Sub(totalTradeFees)
				totalAuctionFees = totalAuctionFees.Add(totalTradeFees.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate)))
			}

			expectedQuoteAvailableBalanceDelta = totalReceived
			expectedQuoteTotalBalanceDelta = totalReceived

			if clearingPrice.IsPositive() {
				bought := expectedFillQuantity.Mul(clearingPrice)
				totalTradeFees := bought.Mul(tradingFeeRate)
				totalReceived = bought.Sub(totalTradeFees)

				expectedQuoteAvailableBalanceDelta = totalReceived
			}

			expectedQuoteTotalBalanceDelta = totalReceived
		}

		Expect(baseDelta.AvailableBalanceDelta.String()).To(Equal(expectedBaseAvailableBalanceDelta.String()))
		Expect(baseDelta.TotalBalanceDelta.String()).To(Equal(expectedBaseTotalBalanceDelta.String()))
		Expect(quoteDelta.AvailableBalanceDelta.String()).To(Equal(expectedQuoteAvailableBalanceDelta.String()))
		Expect(quoteDelta.TotalBalanceDelta.String()).To(Equal(expectedQuoteTotalBalanceDelta.String()))
	}

	return totalAuctionFees
}

func expectSpotCorrectMarketOrderExpansionValues(
	buySellOrdersTest spotBuySellOrders,
	spotMarketOrderExpansions *keeper.SpotBatchExecutionData,
	isRestingBuy bool,
	spotMarket exchangetypes.SpotMarket,
) {
	totalAuctionFees := sdk.ZeroDec()

	if isRestingBuy {
		expectedClearingPrice, _ := sdk.NewDecFromStr(buySellOrdersTest.expectedMarketSellClearingPrice)
		marketOrderFees := expectSpotMarketOrderExpansion(
			buySellOrdersTest.sells,
			expectedClearingPrice,
			spotMarketOrderExpansions.MarketOrderExecutionEvent,
			spotMarketOrderExpansions.BaseDenomDepositSubaccountIDs,
			spotMarketOrderExpansions.BaseDenomDepositDeltas,
			spotMarketOrderExpansions.QuoteDenomDepositDeltas,
			spotMarket,
		)
		limitOrderFees := expectSpotLimitOrderExpansion(
			buySellOrdersTest.buys,
			spotMarketOrderExpansions.LimitOrderFilledDeltas,
			spotMarketOrderExpansions.BaseDenomDepositSubaccountIDs,
			spotMarketOrderExpansions.BaseDenomDepositDeltas,
			spotMarketOrderExpansions.QuoteDenomDepositDeltas,
			spotMarket,
			sdk.ZeroDec(),
			spotMarket.MakerFeeRate,
		)
		totalAuctionFees = marketOrderFees.Add(limitOrderFees)
	} else {
		expectedClearingPrice, _ := sdk.NewDecFromStr(buySellOrdersTest.expectedMarketBuyClearingPrice)
		marketOrderFees := expectSpotMarketOrderExpansion(
			buySellOrdersTest.buys,
			expectedClearingPrice,
			spotMarketOrderExpansions.MarketOrderExecutionEvent,
			spotMarketOrderExpansions.BaseDenomDepositSubaccountIDs,
			spotMarketOrderExpansions.BaseDenomDepositDeltas,
			spotMarketOrderExpansions.QuoteDenomDepositDeltas,
			spotMarket,
		)
		limitOrderFees := expectSpotLimitOrderExpansion(
			buySellOrdersTest.sells,
			spotMarketOrderExpansions.LimitOrderFilledDeltas,
			spotMarketOrderExpansions.BaseDenomDepositSubaccountIDs,
			spotMarketOrderExpansions.BaseDenomDepositDeltas,
			spotMarketOrderExpansions.QuoteDenomDepositDeltas,
			spotMarket,
			sdk.ZeroDec(),
			spotMarket.MakerFeeRate,
		)
		totalAuctionFees = marketOrderFees.Add(limitOrderFees)
	}

	auctionQuoteDelta := spotMarketOrderExpansions.QuoteDenomDepositDeltas[exchangetypes.AuctionSubaccountID]
	expectedAuctionDelta := totalAuctionFees
	Expect(auctionQuoteDelta.AvailableBalanceDelta.String()).To(Equal(expectedAuctionDelta.String()))
	Expect(auctionQuoteDelta.TotalBalanceDelta.String()).To(Equal(expectedAuctionDelta.String()))
}

func expectSpotCorrectLimitOrderExpansionValues(
	buySellOrdersTest spotBuySellOrders,
	spotLimitOrderExpansions *keeper.SpotBatchExecutionData,
	isRestingBuy bool,
	spotMarket exchangetypes.SpotMarket,
) {
	expectedClearingPrice := sdk.MustNewDecFromStr(buySellOrdersTest.expectedLimitClearingPrice)

	if isRestingBuy {
		expectSpotLimitOrderExpansion(
			buySellOrdersTest.buys,
			spotLimitOrderExpansions.LimitOrderFilledDeltas,
			spotLimitOrderExpansions.BaseDenomDepositSubaccountIDs,
			spotLimitOrderExpansions.BaseDenomDepositDeltas,
			spotLimitOrderExpansions.QuoteDenomDepositDeltas,
			spotMarket,
			expectedClearingPrice,
			spotMarket.MakerFeeRate,
		)
	} else {
		expectSpotLimitOrderExpansion(
			buySellOrdersTest.sells,
			spotLimitOrderExpansions.LimitOrderFilledDeltas,
			spotLimitOrderExpansions.BaseDenomDepositSubaccountIDs,
			spotLimitOrderExpansions.BaseDenomDepositDeltas,
			spotLimitOrderExpansions.QuoteDenomDepositDeltas,
			spotMarket,
			expectedClearingPrice,
			spotMarket.MakerFeeRate,
		)
	}
}

func addRestingSpotOrders(testInput *testexchange.TestInput, app *simapp.InjectiveApp, ctx *sdk.Context, orders []testSpotOrder, isBuy bool) {
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	orderType := exchangetypes.OrderType_BUY

	if !isBuy {
		orderType = exchangetypes.OrderType_SELL
	}

	for _, orderData := range orders {
		buyPrice, buyQuantity := testexchange.PriceAndQuantityFromString(orderData.price, orderData.quantity)
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orderData.subaccountNonce)

		limitOrderMsg := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(buyPrice, buyQuantity, orderType, subaccountID, 0)
		_, err1 := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(*ctx), limitOrderMsg)
		testexchange.OrFail(err1)

		*ctx, _ = testexchange.EndBlockerAndCommit(app, *ctx)
	}
}

func getSpotMarketOrders(testInput *testexchange.TestInput, ctx *sdk.Context, orders []testSpotOrder, isBuy bool) []*exchangetypes.SpotMarketOrder {
	marketOrders := make([]*exchangetypes.SpotMarketOrder, 0)

	for i, orderData := range orders {
		price, quantity := testexchange.PriceAndQuantityFromString(orderData.price, orderData.quantity)
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orderData.subaccountNonce)
		orderType := exchangetypes.OrderType_BUY
		if !isBuy {
			orderType = exchangetypes.OrderType_SELL
		}

		marketOrder := exchangetypes.SpotMarketOrder{
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				Price:        price,
				Quantity:     quantity,
				FeeRecipient: marketFeeRecipient,
			},
			OrderType: orderType,
			OrderHash: common.FromHex(marketOrderHashPrefix + strconv.FormatInt(int64(i), 16)),
		}
		marketOrders = append(marketOrders, &marketOrder)
	}

	return marketOrders
}

func getSpotLimitOrders(testInput *testexchange.TestInput, ctx *sdk.Context, orders []testSpotOrder, isBuy bool) []*exchangetypes.SpotLimitOrder {
	limitOrders := make([]*exchangetypes.SpotLimitOrder, 0)

	orderType := exchangetypes.OrderType_BUY

	if !isBuy {
		orderType = exchangetypes.OrderType_SELL
	}

	for i, orderData := range orders {
		price, quantity := testexchange.PriceAndQuantityFromString(orderData.price, orderData.quantity)
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orderData.subaccountNonce)

		limitOrder := exchangetypes.SpotLimitOrder{
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				Price:        price,
				Quantity:     quantity,
				FeeRecipient: marketFeeRecipient,
			},
			OrderHash: common.FromHex(marketOrderHashPrefix + strconv.FormatInt(int64(i), 16)),
			OrderType: orderType,
			Fillable:  quantity,
		}
		limitOrders = append(limitOrders, &limitOrder)
	}

	return limitOrders
}

func expectCorrectSpotMarketMatching(testInput *testexchange.TestInput, app **simapp.InjectiveApp, ctx *sdk.Context, buySellOrdersTest spotBuySellOrders, isRestingBuy bool) {
	marketID := testInput.Spots[0].MarketID
	isMarketBuy := !isRestingBuy

	var restingOrders []testSpotOrder
	var marketOrders []testSpotOrder

	if isRestingBuy {
		restingOrders = buySellOrdersTest.buys
		marketOrders = buySellOrdersTest.sells
	} else {
		restingOrders = buySellOrdersTest.sells
		marketOrders = buySellOrdersTest.buys
	}

	addRestingSpotOrders(testInput, *app, ctx, restingOrders, isRestingBuy)

	spotMarket := (*app).ExchangeKeeper.GetSpotMarket(*ctx, marketID, true)

	marketSpotOrders := getSpotMarketOrders(testInput, ctx, marketOrders, isMarketBuy)
	var marketBuyOrders, marketSellOrders []*exchangetypes.SpotMarketOrder

	if isMarketBuy {
		marketBuyOrders = marketSpotOrders
		marketSellOrders = make([]*exchangetypes.SpotMarketOrder, 0)
	} else {
		marketBuyOrders = make([]*exchangetypes.SpotMarketOrder, 0)
		marketSellOrders = marketSpotOrders
	}

	for _, order := range marketBuyOrders {
		orderType := exchangetypes.OrderType_BUY
		(*app).ExchangeKeeper.SetTransientSpotMarketOrder(*ctx, order, &exchangetypes.SpotOrder{
			OrderType: orderType,
			MarketId:  spotMarket.MarketId,
		}, common.BytesToHash(order.OrderHash))
	}

	for _, order := range marketSellOrders {
		orderType := exchangetypes.OrderType_SELL
		(*app).ExchangeKeeper.SetTransientSpotMarketOrder(*ctx, order, &exchangetypes.SpotOrder{
			OrderType: orderType,
			MarketId:  spotMarket.MarketId,
		}, common.BytesToHash(order.OrderHash))
	}

	var spotMarketOrderExpansions = (*app).ExchangeKeeper.ExecuteSpotMarketOrders(
		*ctx,
		&exchangetypes.MarketOrderIndicator{
			MarketId: spotMarket.MarketId,
			IsBuy:    isMarketBuy,
		},
		keeper.NewFeeDiscountStakingInfo(nil, 0, 0, 0, 0, false),
	)
	expectSpotCorrectMarketOrderExpansionValues(buySellOrdersTest, spotMarketOrderExpansions, isRestingBuy, *spotMarket)
}

func expectCorrectSpotLimitMatching(testInput *testexchange.TestInput, app **simapp.InjectiveApp, ctx *sdk.Context, buySellOrdersTest spotBuySellOrders, isRestingBuy bool) {
	marketID := testInput.Spots[0].MarketID
	isMarketBuy := !isRestingBuy

	var restingOrders []testSpotOrder
	var transientOrders []testSpotOrder

	if isRestingBuy {
		restingOrders = buySellOrdersTest.buys
		transientOrders = buySellOrdersTest.sells
	} else {
		restingOrders = buySellOrdersTest.sells
		transientOrders = buySellOrdersTest.buys
	}

	addRestingSpotOrders(testInput, (*app), ctx, restingOrders, isRestingBuy)

	spotMarket := (*app).ExchangeKeeper.GetSpotMarket(*ctx, marketID, true)

	transientSpotOrders := getSpotLimitOrders(testInput, ctx, transientOrders, isMarketBuy)
	var transientBuyOrders, transientSellOrders []*exchangetypes.SpotLimitOrder

	if isMarketBuy {
		transientBuyOrders = transientSpotOrders
		transientSellOrders = make([]*exchangetypes.SpotLimitOrder, 0)
	} else {
		transientBuyOrders = make([]*exchangetypes.SpotLimitOrder, 0)
		transientSellOrders = transientSpotOrders
	}

	for _, order := range transientBuyOrders {
		isBuy := true
		(*app).ExchangeKeeper.SetTransientSpotLimitOrder(*ctx, order, common.HexToHash(spotMarket.MarketId), isBuy, common.BytesToHash(order.OrderHash))
	}

	for _, order := range transientSellOrders {
		isBuy := false
		(*app).ExchangeKeeper.SetTransientSpotLimitOrder(*ctx, order, common.HexToHash(spotMarket.MarketId), isBuy, common.BytesToHash(order.OrderHash))
	}

	spotLimitOrderExpansions := (*app).ExchangeKeeper.ExecuteSpotLimitOrderMatching(
		*ctx,
		&exchangetypes.MatchedMarketDirection{
			MarketId:    common.HexToHash(spotMarket.MarketId),
			BuysExists:  isRestingBuy,
			SellsExists: !isRestingBuy,
		},
		keeper.NewFeeDiscountStakingInfo(nil, 0, 0, 0, 0, false),
	)

	expectSpotCorrectLimitOrderExpansionValues(buySellOrdersTest, spotLimitOrderExpansions, isRestingBuy, *spotMarket)
}

var _ = Describe("Spot Orders Processor Unit Test", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context
	)

	buySellOrdersTests := []spotBuySellOrders{
		{
			buys:                            []testSpotOrder{{"2200", "4", "4", "1", 0}, {"2100", "5", "5", "2", 0}, {"2000", "3", "3", "3", 0}},
			sells:                           []testSpotOrder{{"1800", "4", "4", "4", 0}, {"1900", "5", "5", "5", 0}, {"2000", "3", "3", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2108.333333333333333333", // (2200*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
			expectedLimitClearingPrice:      "2000",                    // (1800*4 + 1900*5 + 2000*3 + 2200*4 + 2100*5 + 2000*3) / (4 + 5 + 3) + (4 + 5 + 3)
		},
		{
			buys:                            []testSpotOrder{{"2300", "4", "4", "1", 0}, {"2100", "5", "5", "2", 0}, {"2000", "3", "3", "3", 0}},
			sells:                           []testSpotOrder{{"1800", "4", "4", "4", 0}, {"1900", "5", "5", "5", 0}, {"2000", "3", "3", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2141.666666666666666667", // (2300*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
			expectedLimitClearingPrice:      "2000",                    // (1800*4 + 1900*5 + 2000*3 + 2200*4 + 2100*5 + 2000*3) / (4 + 5 + 3) + (4 + 5 + 3)
		}, {
			buys:                            []testSpotOrder{{"2300", "4", "4", "1", 0}, {"2100", "5", "5", "2", 0}, {"2000", "3", "3", "3", 0}},
			sells:                           []testSpotOrder{{"1800", "4", "4", "4", 0}, {"1900", "5", "5", "5", 0}, {"2000", "4", "3", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2141.666666666666666667", // (2300*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
			expectedLimitClearingPrice:      "2000",                    // (1800*4 + 1900*5 + 2000*3 + 2200*4 + 2100*5 + 2000*3) / (4 + 5 + 3) + (4 + 5 + 3)
		},
	}

	BeforeEach(func() {
		positionStatesMap = make(map[common.Hash]*keeper.PositionState)
		buyPositionClosingMargins = make(map[common.Hash]sdk.Dec)
		sellPositionClosingMargins = make(map[common.Hash]sdk.Dec)
		existingPositionPrices = make(map[common.Hash]sdk.Dec)
		buyExistingPositionQuantities = make(map[common.Hash]sdk.Dec)
		sellExistingPositionQuantities = make(map[common.Hash]sdk.Dec)
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		_, err := app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		testexchange.OrFail(err)

		for i := 0; i < 10; i++ {
			subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(i), 16))
			deposit := &exchangetypes.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, math.Int(deposit.AvailableBalance.TruncateInt())), sdk.NewCoin(testInput.Spots[0].BaseDenom, math.Int(deposit.AvailableBalance.TruncateInt()))))
		}
	})

	var runMatchingFixtures = func(hasNegativeMakerFee bool) {
		JustBeforeEach(func() {
			if !hasNegativeMakerFee {
				return
			}

			spotMarket := app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
			negativeMakerFee := sdk.NewDecWithPrec(-1, 3)
			app.ExchangeKeeper.ScheduleSpotMarketParamUpdate(ctx, &exchangetypes.SpotMarketParamUpdateProposal{
				Title:               "Update spot market param",
				Description:         "Update spot market description",
				MarketId:            spotMarket.MarketId,
				MakerFeeRate:        &negativeMakerFee,
				TakerFeeRate:        &spotMarket.TakerFeeRate,
				RelayerFeeShareRate: &spotMarket.RelayerFeeShareRate,
				MinPriceTickSize:    &spotMarket.MinPriceTickSize,
				MinQuantityTickSize: &spotMarket.MinQuantityTickSize,
				Status:              exchangetypes.MarketStatus_Active,
			})

			proposals := make([]exchangetypes.SpotMarketParamUpdateProposal, 0)
			app.ExchangeKeeper.IterateSpotMarketParamUpdates(ctx, func(p *exchangetypes.SpotMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})
			app.ExchangeKeeper.ExecuteSpotMarketParamUpdateProposal(ctx, &proposals[0])
		})

		Describe("For a spot limit orderbook", func() {
			DescribeTable("When matching market orders with limit orders",
				expectCorrectSpotMarketMatching,
				Entry("Matches market sells to limit buys correctly with perfect match #1", &testInput, &app, &ctx, buySellOrdersTests[0], true),
				Entry("Matches market sells to limit buys correctly with perfect match #2", &testInput, &app, &ctx, buySellOrdersTests[1], true),
				Entry("Matches market sells to limit buys correctly with unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[2], true),
				Entry("Matches market buys to limit sells correctly with perfect match #1", &testInput, &app, &ctx, buySellOrdersTests[0], false),
				Entry("Matches market buys to limit sells correctly with perfect match #2", &testInput, &app, &ctx, buySellOrdersTests[1], false),
				Entry("Matches market buys to limit sells correctly with unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[2], false),
			)
		})
	}

	Describe("when maker fee is positive", func() {
		hasNegativeMakerFee := false
		runMatchingFixtures(hasNegativeMakerFee)

		DescribeTable("When matching limit orders with limit orders",
			expectCorrectSpotLimitMatching,
			Entry("Matches limit sells to limit buys correctly with perfect match #1", &testInput, &app, &ctx, buySellOrdersTests[0], true),
			Entry("Matches limit sells to limit buys correctly with perfect match #2", &testInput, &app, &ctx, buySellOrdersTests[1], true),
			Entry("Matches limit sells to limit buys correctly with unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[2], true),
			Entry("Matches limit buys to limit sells correctly with perfect match #1", &testInput, &app, &ctx, buySellOrdersTests[0], false),
			Entry("Matches limit buys to limit sells correctly with perfect match #2", &testInput, &app, &ctx, buySellOrdersTests[1], false),
			Entry("Matches limit buys to limit sells correctly with unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[2], false),
		)
	})

	Describe("when maker fee is negative", func() {
		hasNegativeMakerFee := true
		runMatchingFixtures(hasNegativeMakerFee)
	})
})
