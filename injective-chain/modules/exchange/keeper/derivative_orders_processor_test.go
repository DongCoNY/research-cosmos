package keeper_test

import (
	"strconv"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

type testOrder struct {
	price           string
	quantity        string
	expectedFill    string
	margin          string
	subaccountNonce string
	index           int
}

type testPosition struct {
	entryPrice      string
	quantity        string
	margin          string
	subaccountNonce string
}

type buySellOrders struct {
	buys                            []testOrder
	sells                           []testOrder
	existingBuys                    []testPosition
	existingSells                   []testPosition
	expectedMarketBuyClearingPrice  string
	expectedMarketSellClearingPrice string
}

var marketOrderHashPrefix = "aa"
var marketFeeRecipient = testexchange.DefaultAddress
var limitFeeRecipient = "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"

var buyPositionClosingMargins map[common.Hash]sdk.Dec
var sellPositionClosingMargins map[common.Hash]sdk.Dec

var existingPositionPrices map[common.Hash]sdk.Dec
var buyExistingPositionQuantities map[common.Hash]sdk.Dec
var sellExistingPositionQuantities map[common.Hash]sdk.Dec

var positionStatesMap map[common.Hash]*exchangekeeper.PositionState

func expectMarketOrderExpansion(
	orders []testOrder,
	expectedClearingPrice sdk.Dec,
	marketExpansions []*exchangekeeper.DerivativeOrderStateExpansion,
	marketOrderCancels []*exchangetypes.DerivativeMarketOrderCancel,
	derivativeMarket *exchangetypes.DerivativeMarket,
	positionClosingMargins map[common.Hash]sdk.Dec,
	isBuy bool,
) sdk.Dec {
	expectedMarketSellCancels := make([]testOrder, 0)
	totalExpectedQuantity := sdk.NewDec(0)

	Expect(len(marketExpansions)).To(Not(BeZero()))

	for i, expansion := range marketExpansions {
		expectedSubaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orders[i].subaccountNonce)
		expectedOrderMargin, _ := sdk.NewDecFromStr(orders[i].margin)
		expectedFillQuantity, _ := sdk.NewDecFromStr(orders[i].expectedFill)
		expectedQuantity, _ := sdk.NewDecFromStr(orders[i].quantity)
		expectedOrderPrice, _ := sdk.NewDecFromStr(orders[i].price)

		expectedUsedMargin := expectedOrderMargin.Mul(expectedFillQuantity).Quo(expectedQuantity)

		isReduceOnly := expectedOrderMargin.IsZero()

		totalExpectedQuantity = totalExpectedQuantity.Add(expectedFillQuantity)
		expectedRemainingOrderQuantity := expectedQuantity.Sub(expectedFillQuantity)

		if expectedRemainingOrderQuantity.IsPositive() {
			orders[i].index = i
			expectedMarketSellCancels = append(expectedMarketSellCancels, orders[i])
		}

		expectedOrderFillNotional := expectedFillQuantity.Mul(expectedClearingPrice)
		expectedTradingFee := expectedOrderFillNotional.Mul(derivativeMarket.TakerFeeRate)

		expectedPositionClosingMargin := positionClosingMargins[expectedSubaccountID]
		if expectedPositionClosingMargin.IsNil() {
			expectedPositionClosingMargin = sdk.NewDec(0)
		}

		expectedExistingPositionPrice := existingPositionPrices[expectedSubaccountID]
		if expectedExistingPositionPrice.IsNil() {
			expectedExistingPositionPrice = expectedOrderPrice
		}

		expectedPriceDelta := expectedClearingPrice.Sub(expectedExistingPositionPrice)
		expectedPNL := expectedFillQuantity.Mul(expectedPriceDelta)

		expectedPayout := expectedPositionClosingMargin.Add(expectedPNL)

		var isNewPosition bool

		if isBuy {
			isNewPosition = !sellExistingPositionQuantities[expectedSubaccountID].IsNil()
		} else {
			isNewPosition = !buyExistingPositionQuantities[expectedSubaccountID].IsNil()
		}

		if isReduceOnly {
			expectedPayout = expectedPayout.Sub(expectedTradingFee) // paying fee via payout
		} else if isNewPosition {
			expectedPayout = sdk.NewDec(0) // no payout for new position
		}

		isFullyCancellingOrder := expectedQuantity.Equal(expectedRemainingOrderQuantity)

		Expect(expansion.SubaccountID).To(Equal(expectedSubaccountID))

		if isFullyCancellingOrder {
			Expect(expansion.PositionDelta).To(BeNil())
			Expect(expansion.Payout.String()).To(Equal(expectedPayout.String()))
		} else {
			Expect(expansion.PositionDelta.ExecutionPrice.String()).To(Equal(expectedClearingPrice.String()))
			Expect(expansion.PositionDelta.ExecutionMargin.String()).To(Equal(expectedUsedMargin.String()))
			Expect(expansion.PositionDelta.ExecutionQuantity.String()).To(Equal(expectedFillQuantity.String()))
			Expect(expansion.Payout.String()).To(Equal(expectedPayout.String()))
		}

		address, _ := sdk.AccAddressFromBech32(marketFeeRecipient)
		expectedFeeRecipient := common.BytesToAddress(address.Bytes())
		Expect(expansion.FeeRecipient).To(Equal(expectedFeeRecipient))
		Expect(expansion.OrderHash).To(Equal(common.HexToHash(marketOrderHashPrefix + strconv.FormatInt(int64(i), 16))))
		// TODO: @gorgos - should this be the expectedRemainingOrderQuantity or the expected fillable amount?
		// e := expansion.MarketOrderFilledDelta
		// fmt.Printf("=======\nOrder Quantity %s, fillQuantity %s, unfilledQuantity %s expectedAmount %s isNil %v\n=======\n",e.Order.Quantity(), e.FillQuantity.String(), e.UnfilledQuantity().String(), expectedRemainingOrderQuantity.String(), expectedRemainingOrderQuantity.IsNil())
		Expect(expansion.MarketOrderFilledDelta.UnfilledQuantity().String()).To(Equal(expectedRemainingOrderQuantity.String()))

		expectedTotalBalanceDelta := expectedPayout.Sub(expectedUsedMargin.Add(expectedTradingFee))
		expectedClearingFeeChargeOrRefund := expectedFillQuantity.Mul(expectedPriceDelta).Mul(derivativeMarket.TakerFeeRate)

		unfilledQuantity := expectedQuantity.Sub(expectedFillQuantity)
		expectedUnmatchedRefund := unfilledQuantity.Mul(expectedOrderPrice).Mul(derivativeMarket.TakerFeeRate)
		unusedExecutionMarginRefund := expectedOrderMargin.Sub(expectedUsedMargin)

		if isReduceOnly {
			expectedTotalBalanceDelta = expectedTotalBalanceDelta.Add(expectedTradingFee) // already paid in payout
			expectedClearingFeeChargeOrRefund = sdk.NewDec(0)
		}

		expectedCloseExecutionMargin := sdk.NewDec(0)
		expectedAvailableBalanceDelta := expectedPayout.Add(expectedCloseExecutionMargin).Sub(expectedClearingFeeChargeOrRefund).Add(expectedUnmatchedRefund).Add(unusedExecutionMarginRefund) // .Sub(expectedUnmatchedRefund)

		Expect(expansion.AvailableBalanceDelta.String()).To(Equal(expectedAvailableBalanceDelta.String()))
		Expect(expansion.TotalBalanceDelta.String()).To(Equal(expectedTotalBalanceDelta.String()))

		expectedFeeRecipientReward := derivativeMarket.RelayerFeeShareRate.Mul(expectedTradingFee)
		expectedAuctionFeeReward := expectedTradingFee.Sub(expectedFeeRecipientReward)
		Expect(expansion.AuctionFeeReward.String()).To(Equal(expectedAuctionFeeReward.String()))
		Expect(expansion.FeeRecipientReward.String()).To(Equal(expectedFeeRecipientReward.String()))
	}

	Expect(len(marketOrderCancels)).To(Equal(len(expectedMarketSellCancels)))

	for i, marketSellCancel := range marketOrderCancels {
		expectedSubaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + expectedMarketSellCancels[i].subaccountNonce
		expectedMargin, _ := sdk.NewDecFromStr(expectedMarketSellCancels[i].margin)
		expectedFillQuantity, _ := sdk.NewDecFromStr(expectedMarketSellCancels[i].expectedFill)
		expectedQuantity, _ := sdk.NewDecFromStr(expectedMarketSellCancels[i].quantity)
		expectedOrderPrice, _ := sdk.NewDecFromStr(expectedMarketSellCancels[i].price)
		expectedCancelAmount := expectedQuantity.Sub(expectedFillQuantity)

		Expect(marketSellCancel.MarketOrder.OrderInfo.SubaccountId).To(Equal(expectedSubaccountID))
		Expect(marketSellCancel.MarketOrder.OrderInfo.Price.String()).To(Equal(expectedOrderPrice.String()))
		Expect(marketSellCancel.MarketOrder.Margin.String()).To(Equal(expectedMargin.String()))
		Expect(marketSellCancel.MarketOrder.OrderInfo.Quantity.String()).To(Equal(expectedQuantity.String()))
		Expect(marketSellCancel.MarketOrder.OrderInfo.FeeRecipient).To(Equal(marketFeeRecipient))
		Expect(marketSellCancel.MarketOrder.OrderHash).To(Equal(common.FromHex(marketOrderHashPrefix + strconv.FormatInt(int64(expectedMarketSellCancels[i].index), 16))))
		Expect(marketSellCancel.CancelQuantity.String()).To(Equal(expectedCancelAmount.String()))
	}

	return totalExpectedQuantity
}

func expectLimitOrderExpansion(
	orders []testOrder,
	limitExpansions []*exchangekeeper.DerivativeOrderStateExpansion,
	limitOrderCancels []*exchangetypes.DerivativeLimitOrder,
	derivativeMarket *exchangetypes.DerivativeMarket,
	positionClosingMargins map[common.Hash]sdk.Dec,
	existingPositionQuantities map[common.Hash]sdk.Dec,
	isBuy bool,
) sdk.Dec {
	totalExpectedQuantity := sdk.NewDec(0)

	Expect(len(limitExpansions)).To(Not(BeZero()))

	for i, expansion := range limitExpansions {
		expectedSubaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orders[i].subaccountNonce)
		expectedQuantity, _ := sdk.NewDecFromStr(orders[i].quantity)
		expectedOrderPrice, _ := sdk.NewDecFromStr(orders[i].price)
		expectedMargin, _ := sdk.NewDecFromStr(orders[i].margin)
		expectedFillQuantity, _ := sdk.NewDecFromStr(orders[i].expectedFill)
		expectedExecutionMargin := expectedMargin.Mul(expectedFillQuantity).Quo(expectedQuantity)

		totalExpectedQuantity = totalExpectedQuantity.Add(expectedFillQuantity)
		expectedRemainingOrderQuantity := expectedQuantity.Sub(expectedFillQuantity)

		expectedPositionClosingMargin := positionClosingMargins[expectedSubaccountID]
		if expectedPositionClosingMargin.IsNil() {
			expectedPositionClosingMargin = sdk.NewDec(0)
		}

		expectedExistingPositionPrice := existingPositionPrices[expectedSubaccountID]

		if expectedExistingPositionPrice.IsNil() {
			expectedExistingPositionPrice = expectedOrderPrice
		}

		expectedExistingPositionQuantity := existingPositionQuantities[expectedSubaccountID]
		if expectedExistingPositionQuantity.IsNil() {
			expectedExistingPositionQuantity = sdk.NewDec(0)
		}

		expectedClosingQuantity := sdk.MinDec(expectedExistingPositionQuantity, expectedFillQuantity)
		expectedCloseExecutionMargin := expectedExecutionMargin.Mul(expectedClosingQuantity).Quo(expectedFillQuantity)

		expectedOrderFillNotional := expectedFillQuantity.Mul(expectedOrderPrice)
		expectedTradingFee := expectedOrderFillNotional.Mul(derivativeMarket.MakerFeeRate)
		expectedTraderFee := expectedTradingFee

		if derivativeMarket.MakerFeeRate.IsNegative() {
			expectedTraderFee = expectedTradingFee.Mul(sdk.NewDec(1).Sub(derivativeMarket.RelayerFeeShareRate))
		}

		expectedPriceDelta := expectedOrderPrice.Sub(expectedExistingPositionPrice)
		expectedPNL := expectedFillQuantity.Mul(expectedPriceDelta)

		if !isBuy {
			expectedPNL = expectedPNL.Neg()
		}

		expectedPayout := expectedPositionClosingMargin.Add(expectedPNL)
		expectedCollateralizationMargin := expectedExecutionMargin.Sub(expectedCloseExecutionMargin)

		isReduceOnly := expectedMargin.IsZero()

		if isReduceOnly {
			expectedPayout = expectedPayout.Sub(expectedTraderFee)
		}

		Expect(expansion.SubaccountID).To(Equal(expectedSubaccountID))
		Expect(expansion.PositionDelta.ExecutionPrice.String()).To(Equal(expectedOrderPrice.String()))
		Expect(expansion.PositionDelta.ExecutionMargin.String()).To(Equal(expectedExecutionMargin.String()))
		Expect(expansion.PositionDelta.ExecutionQuantity.String()).To(Equal(expectedFillQuantity.String()))
		Expect(expansion.Payout.String()).To(Equal(expectedPayout.String()))
		address, _ := sdk.AccAddressFromBech32(limitFeeRecipient)
		expectedFeeRecipient := common.BytesToAddress(address.Bytes())
		Expect(expansion.FeeRecipient).To(Equal(expectedFeeRecipient))
		// TODO - Check order hash
		// removed orderhash check for now
		// Expect(expansion.OrderHash).To(Equal(common.HexToHash(... + strconv.FormatInt(int64(i), 16)).Bytes()))

		// TODO: @gorgos - should this be the expectedRemainingOrderQuantity or the expected fillable amount?
		Expect(expansion.LimitOrderFilledDelta.FillableQuantity().String()).To(Equal(expectedRemainingOrderQuantity.String()))

		expectedTotalBalanceDelta := expectedPayout.Sub(expectedCollateralizationMargin).Sub(expectedTraderFee)
		expectedAvailableBalanceDelta := expectedCloseExecutionMargin.Add(expectedPayout)

		if derivativeMarket.MakerFeeRate.IsNegative() && !isReduceOnly {
			expectedAvailableBalanceDelta = expectedAvailableBalanceDelta.Add(expectedTraderFee.Abs())
		}

		if isReduceOnly {
			expectedTotalBalanceDelta = expectedTotalBalanceDelta.Add(expectedTraderFee) // already paid in PNL
		}

		Expect(expansion.AvailableBalanceDelta.String()).To(Equal(expectedAvailableBalanceDelta.String()))
		Expect(expansion.TotalBalanceDelta.String()).To(Equal(expectedTotalBalanceDelta.String()))

		expectedFeeRecipientReward := derivativeMarket.RelayerFeeShareRate.Mul(expectedTradingFee).Abs()
		expectedAuctionFeeReward := expectedTradingFee.Sub(expectedFeeRecipientReward)

		if derivativeMarket.MakerFeeRate.IsNegative() {
			expectedAuctionFeeReward = expectedTradingFee
		}

		Expect(expansion.AuctionFeeReward.String()).To(Equal(expectedAuctionFeeReward.String()))
		Expect(expansion.FeeRecipientReward.String()).To(Equal(expectedFeeRecipientReward.String()))
	}

	for i, expansion := range limitOrderCancels {
		expectedSubaccountID := "727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orders[i].subaccountNonce
		expectedQuantity, _ := sdk.NewDecFromStr(orders[i].quantity)
		expectedOrderPrice, _ := sdk.NewDecFromStr(orders[i].price)

		expectedFillQuantity, _ := sdk.NewDecFromStr(orders[i].expectedFill)
		expectedRemainingOrderQuantity := expectedQuantity.Sub(expectedFillQuantity)

		// removed orderhash check for now
		// Expect(expansion.OrderHash).To(Equal(common.HexToHash(... + strconv.FormatInt(int64(i), 16)).Bytes()))
		Expect(expansion.Fillable.String()).To(Equal(expectedRemainingOrderQuantity.String()))
		Expect(expansion.OrderInfo.Price.String()).To(Equal(expectedOrderPrice.String()))
		Expect(expansion.OrderInfo.Quantity.String()).To(Equal(expectedQuantity.String()))
		Expect(expansion.OrderInfo.SubaccountId).To(Equal(expectedSubaccountID))

		address, _ := sdk.AccAddressFromBech32(limitFeeRecipient)
		expectedFeeRecipient := common.BytesToAddress(address.Bytes())
		Expect(expansion.OrderInfo.FeeRecipient).To(Equal(expectedFeeRecipient))
	}

	return totalExpectedQuantity
}

func expectCorrectExpansionValues(
	buySellOrdersTest buySellOrders,
	derivativeMarketOrderExpansions *exchangekeeper.DerivativeMarketOrderExpansionData,
	derivativeMarket *exchangetypes.DerivativeMarket,
	isRestingBuy bool,
) {
	totalExpectedQuantity := sdk.NewDec(0)

	if isRestingBuy {
		expectedClearingPrice, _ := sdk.NewDecFromStr(buySellOrdersTest.expectedMarketSellClearingPrice)
		marketOrdersTotalQuantity := expectMarketOrderExpansion(
			buySellOrdersTest.sells, expectedClearingPrice,
			derivativeMarketOrderExpansions.MarketSellExpansions,
			derivativeMarketOrderExpansions.MarketSellOrderCancels,
			derivativeMarket,
			buyPositionClosingMargins,
			true,
		)
		totalExpectedQuantity = totalExpectedQuantity.Add(marketOrdersTotalQuantity)

		limitOrdersTotalQuantity := expectLimitOrderExpansion(
			buySellOrdersTest.buys,
			derivativeMarketOrderExpansions.LimitBuyExpansions,
			derivativeMarketOrderExpansions.RestingLimitBuyOrderCancels,
			derivativeMarket,
			sellPositionClosingMargins,
			sellExistingPositionQuantities,
			false,
		)
		totalExpectedQuantity = totalExpectedQuantity.Add(limitOrdersTotalQuantity)

		Expect(derivativeMarketOrderExpansions.MarketSellClearingPrice.String()).To(Equal(expectedClearingPrice.String()))
		Expect(derivativeMarketOrderExpansions.MarketSellClearingQuantity.String()).To(Equal(totalExpectedQuantity.Quo(sdk.NewDec(2)).String()))

		Expect(len(derivativeMarketOrderExpansions.MarketBuyExpansions)).To(BeZero())
		Expect(len(derivativeMarketOrderExpansions.LimitSellExpansions)).To(BeZero())
		Expect(len(derivativeMarketOrderExpansions.RestingLimitSellOrderCancels)).To(BeZero())
		Expect(len(derivativeMarketOrderExpansions.MarketBuyOrderCancels)).To(BeZero())

		Expect(derivativeMarketOrderExpansions.MarketBuyClearingPrice).To(Equal(sdk.Dec{}))
		Expect(derivativeMarketOrderExpansions.MarketBuyClearingQuantity).To(Equal(sdk.Dec{}))
	} else {
		expectedClearingPrice, _ := sdk.NewDecFromStr(buySellOrdersTest.expectedMarketBuyClearingPrice)
		marketOrdersTotalQuantity := expectMarketOrderExpansion(
			buySellOrdersTest.buys,
			expectedClearingPrice,
			derivativeMarketOrderExpansions.MarketBuyExpansions,
			derivativeMarketOrderExpansions.MarketBuyOrderCancels,
			derivativeMarket,
			sellPositionClosingMargins,
			false,
		)
		totalExpectedQuantity = totalExpectedQuantity.Add(marketOrdersTotalQuantity)

		limitOrdersTotalQuantity := expectLimitOrderExpansion(
			buySellOrdersTest.sells,
			derivativeMarketOrderExpansions.LimitSellExpansions,
			derivativeMarketOrderExpansions.RestingLimitSellOrderCancels,
			derivativeMarket,
			buyPositionClosingMargins,
			buyExistingPositionQuantities,
			true,
		)
		totalExpectedQuantity = totalExpectedQuantity.Add(limitOrdersTotalQuantity)

		Expect(derivativeMarketOrderExpansions.MarketBuyClearingPrice.String()).To(Equal(expectedClearingPrice.String()))
		Expect(derivativeMarketOrderExpansions.MarketBuyClearingQuantity.String()).To(Equal(totalExpectedQuantity.Quo(sdk.NewDec(2)).String()))

		Expect(len(derivativeMarketOrderExpansions.MarketSellExpansions)).To(BeZero())
		Expect(len(derivativeMarketOrderExpansions.LimitBuyExpansions)).To(BeZero())
		Expect(len(derivativeMarketOrderExpansions.RestingLimitBuyOrderCancels)).To(BeZero())
		Expect(len(derivativeMarketOrderExpansions.MarketSellOrderCancels)).To(BeZero())

		Expect(derivativeMarketOrderExpansions.MarketSellClearingPrice).To(Equal(sdk.Dec{}))
		Expect(derivativeMarketOrderExpansions.MarketSellClearingQuantity).To(Equal(sdk.Dec{}))
	}
}

func addExistingPositions(testInput *testexchange.TestInput, app *simapp.InjectiveApp, ctx *sdk.Context, positions []testPosition, isBuy bool) {
	positionClosingMargins := buyPositionClosingMargins

	if !isBuy {
		positionClosingMargins = sellPositionClosingMargins
	}

	for _, positionData := range positions {
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + positionData.subaccountNonce)
		quantity, _ := sdk.NewDecFromStr(positionData.quantity)
		entryPrice, _ := sdk.NewDecFromStr(positionData.entryPrice)
		margin, _ := sdk.NewDecFromStr(positionData.margin)

		position := exchangetypes.Position{
			IsLong:     isBuy,
			Quantity:   quantity,
			EntryPrice: entryPrice,
			Margin:     margin,
		}

		app.ExchangeKeeper.SetPosition(*ctx, testInput.Perps[0].MarketID, subaccountID, &position)
		positionStatesMap[subaccountID] = &exchangekeeper.PositionState{
			Position: &position,
		}

		if _, ok := positionClosingMargins[subaccountID]; !ok {
			positionClosingMargins[subaccountID] = sdk.NewDec(0)
		}
		positionClosingMargins[subaccountID] = positionClosingMargins[subaccountID].Add(margin)
		existingPositionPrices[subaccountID] = entryPrice
	}
}

func addRestingOrders(testInput *testexchange.TestInput, app *simapp.InjectiveApp, ctx *sdk.Context, orders []testOrder, isBuy bool) {
	msgServer := exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)

	orderType := exchangetypes.OrderType_BUY
	positionClosingMargins := buyPositionClosingMargins

	if !isBuy {
		orderType = exchangetypes.OrderType_SELL
		positionClosingMargins = sellPositionClosingMargins
	}

	for _, orderData := range orders {
		margin, _ := sdk.NewDecFromStr(orderData.margin)
		buyPrice, buyQuantity := testexchange.PriceAndQuantityFromString(orderData.price, orderData.quantity)
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orderData.subaccountNonce)

		if _, ok := positionClosingMargins[subaccountID]; !ok {
			positionClosingMargins[subaccountID] = sdk.NewDec(0)
		}

		positionClosingMargins[subaccountID] = positionClosingMargins[subaccountID].Add(margin)
		// fmt.Printf("adding new %s limit order with quantity %s margin %s from subaccount %s\n", orderType.String(), buyQuantity, margin, subaccountID.Hex())

		limitOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(limitFeeRecipient, buyPrice, buyQuantity, margin, orderType, subaccountID, 0, false)
		_, err1 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(*ctx), limitOrderMsg)
		testexchange.OrFail(err1)

		newCtx, _ := testexchange.EndBlockerAndCommit(app, *ctx)
		ctx = &newCtx
	}
}

func getMarketOrders(testInput *testexchange.TestInput, ctx *sdk.Context, orders []testOrder, isBuy bool) []*exchangetypes.DerivativeMarketOrder {
	existingPositionQuantities := buyExistingPositionQuantities
	positionClosingMargins := buyPositionClosingMargins

	if !isBuy {
		positionClosingMargins = sellPositionClosingMargins
		existingPositionQuantities = sellExistingPositionQuantities
	}

	marketOrders := make([]*exchangetypes.DerivativeMarketOrder, 0)

	for i, orderData := range orders {
		price, quantity := testexchange.PriceAndQuantityFromString(orderData.price, orderData.quantity)
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + orderData.subaccountNonce)
		margin, _ := sdk.NewDecFromStr(orderData.margin)

		if _, ok := positionClosingMargins[subaccountID]; !ok {
			positionClosingMargins[subaccountID] = sdk.NewDec(0)
		}
		positionClosingMargins[subaccountID] = positionClosingMargins[subaccountID].Add(margin)

		if _, ok := existingPositionQuantities[subaccountID]; !ok {
			existingPositionQuantities[subaccountID] = sdk.NewDec(0)
		}
		existingPositionQuantities[subaccountID] = existingPositionQuantities[subaccountID].Add(quantity)

		var orderType exchangetypes.OrderType

		if isBuy {
			orderType = exchangetypes.OrderType_BUY
		} else {
			orderType = exchangetypes.OrderType_SELL
		}

		marketOrder := exchangetypes.DerivativeMarketOrder{
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				Price:        price,
				Quantity:     quantity,
				FeeRecipient: marketFeeRecipient,
			},
			OrderType:  orderType,
			Margin:     margin,
			MarginHold: margin,
			OrderHash:  common.FromHex(marketOrderHashPrefix + strconv.FormatInt(int64(i), 16)),
		}
		marketOrders = append(marketOrders, &marketOrder)
	}

	return marketOrders
}

func expectCorrectMatching(testInput *testexchange.TestInput, app **simapp.InjectiveApp, ctx *sdk.Context, buySellOrdersTest buySellOrders, isRestingBuy bool) {
	marketID := testInput.Perps[0].MarketID
	isMarketBuy := !isRestingBuy

	var restingOrders []testOrder
	var marketOrders []testOrder

	if isRestingBuy {
		restingOrders = buySellOrdersTest.buys
		marketOrders = buySellOrdersTest.sells
	} else {
		restingOrders = buySellOrdersTest.sells
		marketOrders = buySellOrdersTest.buys
	}

	addExistingPositions(testInput, *app, ctx, buySellOrdersTest.existingBuys, true)
	addExistingPositions(testInput, *app, ctx, buySellOrdersTest.existingSells, false)
	addRestingOrders(testInput, *app, ctx, restingOrders, isRestingBuy)

	funding := (*app).ExchangeKeeper.GetPerpetualMarketFunding(*ctx, marketID)
	derivativeMarket, markPrice := (*app).ExchangeKeeper.GetDerivativeMarketWithMarkPrice(*ctx, marketID, true)

	marketDerivativeOrders := getMarketOrders(testInput, ctx, marketOrders, isMarketBuy)
	var marketBuyOrders, marketSellOrders []*exchangetypes.DerivativeMarketOrder

	if isMarketBuy {
		marketBuyOrders = marketDerivativeOrders
		marketSellOrders = make([]*exchangetypes.DerivativeMarketOrder, 0)
	} else {
		marketBuyOrders = make([]*exchangetypes.DerivativeMarketOrder, 0)
		marketSellOrders = marketDerivativeOrders
	}

	// TODO also test GetDerivativeMatchingExecutionData

	feeDiscountConfig := exchangekeeper.NewFeeDiscountConfig(false, exchangekeeper.NewFeeDiscountStakingInfo(nil, 0, 0, 0, 0, false))
	isLiquidation := false
	derivativeMarketOrderExpansions := (*app).ExchangeKeeper.GetDerivativeMarketOrderExecutionData(
		*ctx,
		derivativeMarket,
		testInput.Perps[0].TakerFeeRate,
		markPrice,
		funding,
		marketBuyOrders,
		marketSellOrders,
		positionStatesMap,
		feeDiscountConfig,
		isLiquidation,
	)

	expectCorrectExpansionValues(buySellOrdersTest, derivativeMarketOrderExpansions, derivativeMarket, isRestingBuy)
}

var _ = Describe("Derivatives Orders Processor Unit Test", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context
	)

	buySellOrdersTests := []buySellOrders{
		{
			buys:                            []testOrder{{"2200", "4", "4", "2222", "1", 0}, {"2100", "5", "5", "3333", "2", 0}, {"2000", "3", "3", "4444", "3", 0}},
			sells:                           []testOrder{{"1800", "4", "4", "5555", "4", 0}, {"1900", "5", "5", "6666", "5", 0}, {"2000", "3", "3", "7777", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2108.333333333333333333", // (2200*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
		}, {
			buys:                            []testOrder{{"2300", "4", "4", "2222", "1", 0}, {"2100", "5", "5", "3333", "2", 0}, {"2000", "3", "3", "4444", "3", 0}},
			sells:                           []testOrder{{"1800", "4", "4", "5555", "4", 0}, {"1900", "5", "5", "6666", "5", 0}, {"2000", "3", "3", "7777", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2141.666666666666666667", // (2300*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
		}, {
			buys:                            []testOrder{{"2300", "4", "4", "2222", "1", 0}, {"2100", "5", "5", "3333", "2", 0}, {"2000", "3", "3", "4444", "3", 0}},
			sells:                           []testOrder{{"1800", "4", "4", "5555", "4", 0}, {"1900", "5", "5", "6666", "5", 0}, {"2000", "4", "3", "7777", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2141.666666666666666667", // (2300*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
		},
		{
			buys:                            []testOrder{{"2200", "4", "4", "2222", "1", 0}, {"2100", "5", "5", "3333", "2", 0}, {"2000", "3", "3", "4444", "3", 0}, {"100", "4", "0", "2222", "1", 0}},
			sells:                           []testOrder{{"1800", "4", "4", "5555", "4", 0}, {"1900", "5", "5", "6666", "5", 0}, {"2000", "3", "3", "7777", "6", 0}},
			expectedMarketBuyClearingPrice:  "1891.666666666666666667", // (1800*4 + 1900*5 + 2000*3) / (4 + 5 + 3)
			expectedMarketSellClearingPrice: "2108.333333333333333333", // (2200*4 + 2100*5 + 2000*3) / (4 + 5 + 3)
		},
		// netting
		{
			buys:                            []testOrder{{"2000", "4", "4", "2222", "1", 0}},
			sells:                           []testOrder{{"2000", "4", "4", "2222", "1", 0}},
			expectedMarketBuyClearingPrice:  "2000",
			expectedMarketSellClearingPrice: "2000",
		},
		{
			buys:                            []testOrder{{"2001", "4", "4", "2222", "1", 0}, {"2000", "1", "1", "3333", "2", 0}},
			sells:                           []testOrder{{"2000", "5", "5", "2222", "1", 0}},
			expectedMarketBuyClearingPrice:  "2000",
			expectedMarketSellClearingPrice: "unused",
		},
		{
			buys:                            []testOrder{{"2010", "5", "5", "2222", "1", 0}},
			sells:                           []testOrder{{"2005", "4", "4", "2222", "1", 0}, {"2000", "1", "1", "3333", "2", 0}},
			expectedMarketBuyClearingPrice:  "unused",
			expectedMarketSellClearingPrice: "2010",
		},
		// reduce-only
		{
			buys:                            []testOrder{{"2000", "4", "4", "2222", "2", 0}},
			sells:                           []testOrder{{"2000", "4", "4", "0", "1", 0}},
			existingBuys:                    []testPosition{{"2000", "4", "2222", "1"}},
			expectedMarketBuyClearingPrice:  "unused",
			expectedMarketSellClearingPrice: "2000",
		},
		{
			buys:                            []testOrder{{"2000", "4", "4", "0", "2", 0}},
			sells:                           []testOrder{{"2000", "4", "4", "2222", "1", 0}},
			existingSells:                   []testPosition{{"2000", "4", "2222", "2"}},
			expectedMarketBuyClearingPrice:  "2000",
			expectedMarketSellClearingPrice: "unused",
		},
		{
			buys:                            []testOrder{{"2005", "4", "4", "0", "1", 0}},
			sells:                           []testOrder{{"2005", "4", "4", "2222", "2", 0}},
			existingSells:                   []testPosition{{"2000", "4", "2222", "1"}},
			expectedMarketBuyClearingPrice:  "unused",
			expectedMarketSellClearingPrice: "2005",
		},
		{
			buys:                            []testOrder{{"2005", "4", "4", "0", "1", 0}},
			sells:                           []testOrder{{"2005", "4", "4", "2222", "2", 0}},
			existingSells:                   []testPosition{{"2010", "4", "2222", "1"}},
			expectedMarketBuyClearingPrice:  "unused",
			expectedMarketSellClearingPrice: "2005",
		},
		{
			buys:                            []testOrder{{"1995", "4", "4", "2222", "2", 0}},
			sells:                           []testOrder{{"1995", "4", "4", "0", "1", 0}},
			existingBuys:                    []testPosition{{"2000", "4", "2222", "1"}},
			expectedMarketBuyClearingPrice:  "unused",
			expectedMarketSellClearingPrice: "1995",
		},
		{
			buys:                            []testOrder{{"2005", "4", "4", "2222", "2", 0}},
			sells:                           []testOrder{{"2005", "4", "4", "0", "1", 0}},
			existingBuys:                    []testPosition{{"2000", "4", "2222", "1"}},
			expectedMarketBuyClearingPrice:  "unused",
			expectedMarketSellClearingPrice: "2005",
		},
	}

	BeforeEach(func() {
		positionStatesMap = make(map[common.Hash]*exchangekeeper.PositionState)
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
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		startingPrice := sdk.NewDec(2000)
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		_, _, err := app.ExchangeKeeper.PerpetualMarketLaunch(
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

		for i := 0; i < 10; i++ {
			subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(i), 16))
			deposit := &exchangetypes.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
		}
	})

	var runMatchingFixtures = func(hasNegativeMakerFee bool) {
		JustBeforeEach(func() {
			if !hasNegativeMakerFee {
				return
			}

			derivativeMarket := app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)
			negativeMakerFee := sdk.NewDecWithPrec(-1, 3)
			app.ExchangeKeeper.ScheduleDerivativeMarketParamUpdate(ctx, &exchangetypes.DerivativeMarketParamUpdateProposal{
				Title:                  "Update Derivative market param",
				Description:            "Update Derivative market description",
				MarketId:               derivativeMarket.MarketId,
				MakerFeeRate:           &negativeMakerFee,
				TakerFeeRate:           &derivativeMarket.TakerFeeRate,
				RelayerFeeShareRate:    &derivativeMarket.RelayerFeeShareRate,
				MinPriceTickSize:       &derivativeMarket.MinPriceTickSize,
				MinQuantityTickSize:    &derivativeMarket.MinQuantityTickSize,
				InitialMarginRatio:     &derivativeMarket.InitialMarginRatio,
				MaintenanceMarginRatio: &derivativeMarket.MaintenanceMarginRatio,
				Status:                 exchangetypes.MarketStatus_Active,
			})

			proposals := make([]exchangetypes.DerivativeMarketParamUpdateProposal, 0)
			app.ExchangeKeeper.IterateDerivativeMarketParamUpdates(ctx, func(p *exchangetypes.DerivativeMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})
			app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &proposals[0])
		})

		Describe("For a derivative limit orderbook", func() {
			DescribeTable("When matching market orders with limit orders",
				expectCorrectMatching,
				Entry("Matches market sells to limit buys correctly with perfect match #1", &testInput, &app, &ctx, buySellOrdersTests[0], true),
				Entry("Matches market sells to limit buys correctly with perfect match #2", &testInput, &app, &ctx, buySellOrdersTests[1], true),
				Entry("Matches market sells to limit buys correctly with unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[2], true),
				Entry("Matches market sells to limit buys correctly with fully unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[3], false),
				Entry("Matches market sells to limit buys correctly with netting in same subaccount to zero", &testInput, &app, &ctx, buySellOrdersTests[4], true),
				Entry("Matches market sells to limit buys correctly with netting in same subaccount with flipping", &testInput, &app, &ctx, buySellOrdersTests[6], true),
				Entry("Matches market sells to limit buys correctly with reduce-only", &testInput, &app, &ctx, buySellOrdersTests[7], true),
				Entry("Matches market sells to limit buys correctly with reduce-only with profit", &testInput, &app, &ctx, buySellOrdersTests[9], true),
				Entry("Matches market sells to limit buys correctly with reduce-only with loss", &testInput, &app, &ctx, buySellOrdersTests[10], true),
				Entry("Matches market buys to limit sells correctly with perfect match #1", &testInput, &app, &ctx, buySellOrdersTests[0], false),
				Entry("Matches market buys to limit sells correctly with perfect match #2", &testInput, &app, &ctx, buySellOrdersTests[1], false),
				Entry("Matches market buys to limit sells correctly with unfilled sell", &testInput, &app, &ctx, buySellOrdersTests[2], false),
				Entry("Matches market buys to limit sells correctly with netting in same subaccount to zero", &testInput, &app, &ctx, buySellOrdersTests[4], false),
				Entry("Matches market buys to limit sells correctly with netting in same subaccount with flipping", &testInput, &app, &ctx, buySellOrdersTests[5], false),
				Entry("Matches market buys to limit sells correctly with reduce-only", &testInput, &app, &ctx, buySellOrdersTests[8], false),
				Entry("Matches market buys to limit sells correctly with reduce-only with profit", &testInput, &app, &ctx, buySellOrdersTests[11], true),
				Entry("Matches market buys to limit sells correctly with reduce-only with loss", &testInput, &app, &ctx, buySellOrdersTests[12], true),
			)
		})
	}

	Describe("when maker fee is positive", func() {
		hasNegativeMakerFee := false
		runMatchingFixtures(hasNegativeMakerFee)
	})

	Describe("when maker fee is negative", func() {
		hasNegativeMakerFee := true
		runMatchingFixtures(hasNegativeMakerFee)
	})
})
