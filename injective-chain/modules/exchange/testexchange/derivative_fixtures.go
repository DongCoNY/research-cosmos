package testexchange

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	json "github.com/InjectiveLabs/jsonc"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"

	//lint:ignore ST1001 allow dot import for convenience
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var resultingDerivativeDeposits []map[common.Hash]derivativeDeposit

type derivativesLimitTestOrder struct {
	Price                                 float32 `json:"price"`
	Quantity                              int     `json:"quantity"`
	Margin                                int     `json:"margin"`
	SubAccountNonce                       int     `json:"subaccountNonce"` // last one digit of subaccount
	ExpectedQuantityFilledViaMatching     int     `json:"expectedQuantityFilledViaMatching"`
	ExpectedQuantityFilledViaMarketOrders int     `json:"expectedQuantityFilledViaMarketOrders"`
	IsPostOnly                            bool    `json:"isPostOnly"`
	OrderHashes                           []string
}

type derivativesMarketTestOrder struct {
	Quantity        int `json:"quantity"`
	Margin          int `json:"margin"`
	SubAccountNonce int `json:"subaccountNonce"` // last one digit of subaccount
	QuantityFilled  int `json:"expectedQuantityFilled"`
	WorstPrice      int `json:"worstPrice"`
	OrderHashes     []string
}

type derivativeDeposit struct {
	QuoteAvailableBalance sdk.Dec
	QuoteTotalBalance     sdk.Dec
}

type testDerivativesOrderbook struct {
	ExistingPositionBuys             []derivativesLimitTestOrder  `json:"existing-position-buys"`
	ExistingPositionSells            []derivativesLimitTestOrder  `json:"existing-position-sells"`
	SellOrders                       []derivativesLimitTestOrder  `json:"resting-sells"`
	TransientSellOrders              []derivativesLimitTestOrder  `json:"transient-sells"`
	MarketSellOrders                 []derivativesMarketTestOrder `json:"market-sells"`
	BuyOrders                        []derivativesLimitTestOrder  `json:"resting-buys"`
	TransientBuyOrders               []derivativesLimitTestOrder  `json:"transient-buys"`
	MarketBuyOrders                  []derivativesMarketTestOrder `json:"market-buys"`
	ExpectedClearingPriceMatching    float32                      `json:"expected-clearing-price-matching"`
	ExpectedClearingPriceMarketSells float32                      `json:"expected-clearing-price-market-sells"`
	ExpectedClearingPriceMarketBuys  float32                      `json:"expected-clearing-price-market-buys"`
}

type subaccountPosition struct {
	SubaccountId string
	Position     *exchangetypes.Position
}

type tradeEvents struct {
	limitMatches               []*exchangetypes.DerivativeTradeLog
	limitFillsFromBuys         []*exchangetypes.DerivativeTradeLog
	limitFillsFromSells        []*exchangetypes.DerivativeTradeLog
	marketBuys                 []*exchangetypes.DerivativeTradeLog
	marketSells                []*exchangetypes.DerivativeTradeLog
	newBuyOrders               []*exchangetypes.DerivativeLimitOrder
	newSellOrders              []*exchangetypes.DerivativeLimitOrder
	marketUpdates              []*exchangetypes.EventPerpetualMarketUpdate
	marketFundingUpdates       []*exchangetypes.EventPerpetualMarketFundingUpdate
	expiryMarketUpdates        []*exchangetypes.EventExpiryFuturesMarketUpdate
	binaryOptionsMarketUpdates []*exchangetypes.EventBinaryOptionsMarketUpdate
}

type expectedOrderEventData struct {
	limitFilled     uint64
	marketFilled    uint64
	executionMargin sdk.Dec
	isMaker         bool
	isLong          bool
	subaccountId    common.Hash
}

var existingPreviousPositions map[int]map[common.Hash]subaccountPosition
var originalPositions map[int]map[common.Hash]subaccountPosition
var orderNoncesPerTrader map[common.Hash]uint32

func expectCorrectOrderEventValues(
	emittedEventData []*exchangetypes.DerivativeTradeLog,
	expectedPrices []float32,
	marketIndex int,
	expectedOrderMatchEventData map[string]expectedOrderEventData,
	derivativesMarket keeper.DerivativeMarketI,
	isMarketFilled, isDiscountedFeeRate bool,
) {
	for i := range emittedEventData {
		expectedPriceString := fmt.Sprintf("%f000000000000", float32(expectedPrices[i]))
		orderHash := common.BytesToHash(emittedEventData[i].OrderHash).Hex()
		positionDelta := emittedEventData[i].PositionDelta

		var expectedPayout, pnlNotional sdk.Dec
		subaccountId := common.BytesToHash(emittedEventData[i].SubaccountId)
		position := originalPositions[marketIndex][subaccountId].Position
		isClosingViaNetting := position != nil && position.Quantity.IsPositive() && positionDelta.IsLong == !position.IsLong

		if isClosingViaNetting {
			closingQuantity := sdk.MinDec(position.Quantity, positionDelta.ExecutionQuantity)
			if position.IsLong {
				pnlNotional = closingQuantity.Mul(positionDelta.ExecutionPrice.Sub(position.EntryPrice))
			} else {
				pnlNotional = closingQuantity.Mul(positionDelta.ExecutionPrice.Sub(position.EntryPrice)).Neg()
			}
			positionClosingMargin := position.Margin.Mul(closingQuantity).Quo(position.Quantity)
			expectedPayout = pnlNotional.Add(positionClosingMargin)
		} else {
			expectedPayout = sdk.ZeroDec()
		}

		var feeRate sdk.Dec
		if expectedOrderMatchEventData[orderHash].isMaker {
			feeRate = derivativesMarket.GetMakerFeeRate()
		} else {
			feeRate = derivativesMarket.GetTakerFeeRate()
		}

		if isDiscountedFeeRate && (feeRate.IsPositive() || !expectedOrderMatchEventData[orderHash].isMaker) {
			feeRate = feeRate.Quo(sdk.NewDec(10)).Mul(sdk.NewDec(9))
		}

		expectedFee := positionDelta.ExecutionPrice.Mul(positionDelta.ExecutionQuantity).Mul(feeRate)

		if feeRate.IsNegative() {
			expectedFee = expectedFee.Mul(sdk.NewDec(1).Sub(derivativesMarket.GetRelayerFeeShareRate()))
		}

		isReduceOnly := positionDelta.ExecutionMargin.IsZero()
		if isReduceOnly {
			expectedPayout = expectedPayout.Sub(expectedFee)
		}

		expectedFeeLog := positionDelta.ExecutionPrice.Mul(positionDelta.ExecutionQuantity).Mul(feeRate)

		isSelfRelayedTrade := exchangetypes.SubaccountIDToEthAddress(subaccountId) == common.BytesToAddress(emittedEventData[i].FeeRecipientAddress)
		if isSelfRelayedTrade {
			if feeRate.IsPositive() {
				expectedFeeLog = expectedFeeLog.Mul(sdk.NewDec(1).Sub(derivativesMarket.GetRelayerFeeShareRate()))
			}
		}

		Expect(positionDelta.ExecutionPrice.String()).Should(Equal(expectedPriceString))

		if isMarketFilled {
			Expect(positionDelta.ExecutionQuantity.TruncateInt().Uint64()).Should(Equal(expectedOrderMatchEventData[orderHash].marketFilled))
		} else {
			Expect(positionDelta.ExecutionQuantity.TruncateInt().Uint64()).Should(Equal(expectedOrderMatchEventData[orderHash].limitFilled))
		}

		if derivativesMarket.GetMarketType() == exchangetypes.MarketType_BinaryOption {
			orderType := exchangetypes.OrderType_BUY
			if !positionDelta.IsLong {
				orderType = exchangetypes.OrderType_SELL
			}
			expectedMargin := exchangetypes.GetRequiredBinaryOptionsOrderMargin(positionDelta.ExecutionPrice, positionDelta.ExecutionQuantity, derivativesMarket.GetOracleScaleFactor(), orderType, isReduceOnly)
			Expect(positionDelta.ExecutionMargin.String()).Should(Equal(expectedMargin.String()))
			// Expect(positionDelta.ExecutionMargin.String()).Should(Equal(expectedOrderMatchEventData[orderHash].executionMargin.String()))
		} else {
			Expect(positionDelta.ExecutionMargin.String()).Should(Equal(expectedOrderMatchEventData[orderHash].executionMargin.String()))
		}
		Expect(positionDelta.IsLong).Should(Equal(expectedOrderMatchEventData[orderHash].isLong))
		Expect(emittedEventData[i].Fee.String()).Should(Equal(expectedFeeLog.String()))
		Expect(emittedEventData[i].Payout.String()).Should(Equal(expectedPayout.String()))
	}
}

func (testInput *TestInput) calculateExpectedDerivativeDepositsForLimitOrders(app *simapp.InjectiveApp, ctx sdk.Context, orders []derivativesLimitTestOrder, isResting bool, marketIndex int, clearingPrice sdk.Dec, isBuy bool, marketId common.Hash, isDiscountedFeeRate, isBinaryOptions bool) {
	for i := 0; i < len(orders); i++ {
		order := orders[i]
		margin := sdk.NewDec(int64(order.Margin))
		quantity := sdk.NewDec(int64(order.Quantity))
		price := sdk.NewDec(int64(order.Price))

		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(order.SubAccountNonce), 16))
		expectedQuantityFilledViaMatching := sdk.NewDec(int64(order.ExpectedQuantityFilledViaMatching))
		expectedQuantityFilledViaMarketOrders := sdk.NewDec(int64(order.ExpectedQuantityFilledViaMarketOrders))

		executionPrice := clearingPrice

		if expectedQuantityFilledViaMatching.IsPositive() && expectedQuantityFilledViaMarketOrders.IsNegative() {
			// needed due to assumptions we make on the execution price
			panic("fixtures don't support limit orders being partially filled by both matching and market orders")
		}

		if clearingPrice.IsZero() || expectedQuantityFilledViaMarketOrders.IsPositive() {
			executionPrice = price
		}

		var market keeper.DerivativeMarketI
		if isBinaryOptions {
			market = app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, marketId, true)
		} else {
			market = app.ExchangeKeeper.GetDerivativeMarket(ctx, marketId, true)
		}

		var feeForFilled sdk.Dec
		if isResting {
			feeForFilled = market.GetMakerFeeRate()
		} else {
			feeForFilled = market.GetTakerFeeRate()
		}

		if isDiscountedFeeRate && (!feeForFilled.IsNegative() || !isResting) {
			feeForFilled = feeForFilled.Quo(sdk.NewDec(10)).Mul(sdk.NewDec(9))
		}

		if feeForFilled.IsNegative() {
			feeForFilled = feeForFilled.Mul(sdk.NewDec(1).Sub(market.GetRelayerFeeShareRate()))
		}

		notionalMarket := price.Mul(quantity)
		feePaidMarket := notionalMarket.Mul(sdk.ZeroDec().Add(feeForFilled)).Mul(expectedQuantityFilledViaMarketOrders).Quo(quantity)

		notionalLimit := executionPrice.Mul(quantity)
		feePaidLimit := notionalLimit.Mul(sdk.ZeroDec().Add(feeForFilled)).Mul(expectedQuantityFilledViaMatching).Quo(quantity)

		feePaid := feePaidMarket.Add(feePaidLimit)
		positiveMakerFeePart := sdk.MaxDec(sdk.ZeroDec(), market.GetMakerFeeRate())
		feePaidForRemainingQuantity := price.Mul(quantity.Sub(expectedQuantityFilledViaMarketOrders).Sub(expectedQuantityFilledViaMatching)).Mul(positiveMakerFeePart)

		startingDeposits := derivativeDeposit{
			QuoteAvailableBalance: startingDeposit,
			QuoteTotalBalance:     startingDeposit,
		}

		if resultingDerivativeDeposits[marketIndex][trader] != (derivativeDeposit{}) {
			startingDeposits = resultingDerivativeDeposits[marketIndex][trader]
		}

		quantityFilled := expectedQuantityFilledViaMatching.Add(expectedQuantityFilledViaMarketOrders)
		existingPosition := existingPreviousPositions[marketIndex][trader]

		executionMargin := margin.Mul(quantityFilled).Quo(quantity)
		if isBinaryOptions {
			orderType := exchangetypes.OrderType_BUY
			if !isBuy {
				orderType = exchangetypes.OrderType_SELL
			}
			isReduceOnly := order.Margin == 0

			correctExecutionMargin := exchangetypes.GetRequiredBinaryOptionsOrderMargin(
				executionPrice,
				quantityFilled,
				market.GetOracleScaleFactor(),
				orderType,
				isReduceOnly,
			)
			executionMargin = correctExecutionMargin
		}

		if existingPosition.Position == nil {
			existingPosition = subaccountPosition{
				trader.String(),
				&exchangetypes.Position{
					IsLong:     true,
					Quantity:   sdk.NewDec(0),
					Margin:     sdk.NewDec(0),
					EntryPrice: sdk.NewDec(0),
				},
			}
		}

		isAddingToPosition := quantityFilled.IsPositive() && (existingPosition.Position.IsLong == isBuy || existingPosition.Position.Quantity.IsZero())
		isClosingPosition := !isAddingToPosition

		oldPositionMargin := existingPosition.Position.Margin
		oldPositionQuantity := existingPosition.Position.Quantity
		oldPositionEntryPrice := existingPosition.Position.EntryPrice
		oldPositionIsLong := existingPosition.Position.IsLong

		var isFlipping bool

		if isAddingToPosition {
			existingPosition.Position.Quantity = existingPosition.Position.Quantity.Add(quantityFilled)
			existingPosition.Position.Margin = existingPosition.Position.Margin.Add(executionMargin)
			previousNotional := existingPosition.Position.EntryPrice.Mul(existingPosition.Position.Quantity)
			newNotional := price.Mul(expectedQuantityFilledViaMarketOrders).Add(executionPrice.Mul(expectedQuantityFilledViaMatching))
			existingPosition.Position.EntryPrice = (previousNotional.Add(newNotional)).Quo(existingPosition.Position.Quantity)
			existingPosition.Position.IsLong = isBuy
		} else if quantityFilled.IsPositive() {
			isFlipping = quantityFilled.GT(existingPosition.Position.Quantity)
			existingPosition.Position.Quantity = existingPosition.Position.Quantity.Sub(quantityFilled).Abs()

			if quantityFilled.IsZero() {
				existingPosition.Position.EntryPrice = sdk.ZeroDec()
			}

			if isFlipping {
				existingPosition.Position.IsLong = !existingPosition.Position.IsLong
				existingPosition.Position.Margin = executionMargin.Mul(existingPosition.Position.Quantity).Quo(quantityFilled)
				notionalForQuantityFilledViaMarketOrders := price.Mul(expectedQuantityFilledViaMarketOrders)
				notionalForQuantityFilledViaMatching := executionPrice.Mul(expectedQuantityFilledViaMatching)
				totalNotional := notionalForQuantityFilledViaMarketOrders.Add(notionalForQuantityFilledViaMatching)
				existingPosition.Position.EntryPrice = totalNotional.Quo(quantityFilled)
			} else {
				existingPosition.Position.Margin = existingPosition.Position.Margin.Mul(existingPosition.Position.Quantity).Quo(oldPositionQuantity)
			}
		}

		if existingPosition.Position.Quantity.IsZero() {
			existingPosition.Position.IsLong = true
		}

		_, ok := existingPreviousPositions[marketIndex]
		if !ok {
			existingPreviousPositions[marketIndex] = make(map[common.Hash]subaccountPosition)
		}
		existingPreviousPositions[marketIndex][trader] = existingPosition

		var availableQuoteBalanceSpent, totalQuoteBalanceSpent sdk.Dec

		if isClosingPosition && quantityFilled.IsPositive() {
			closingQuantity := sdk.MinDec(oldPositionQuantity, quantityFilled)
			marginRefunded := oldPositionMargin.Mul(closingQuantity).Quo(oldPositionQuantity)
			marginUsedForFlipping := sdk.NewDec(0)

			if isFlipping {
				marginUsedForFlipping = executionMargin.Mul(existingPosition.Position.Quantity).Quo(quantity)
			}

			var pnlNotional sdk.Dec

			if oldPositionIsLong {
				pnlNotional = closingQuantity.Mul(executionPrice.Sub(oldPositionEntryPrice)).Neg()
			} else {
				pnlNotional = closingQuantity.Mul(executionPrice.Sub(oldPositionEntryPrice))
			}

			availableQuoteBalanceSpent = marginRefunded.Neg().Add(marginUsedForFlipping.Add(feePaid)).Add(pnlNotional)
			totalQuoteBalanceSpent = availableQuoteBalanceSpent
		} else if quantityFilled.IsPositive() {
			totalQuoteBalanceSpent = executionMargin.Add(feePaid)
			remainingMargin := margin.Mul(quantity.Sub(quantityFilled)).Quo(quantity)
			availableQuoteBalanceSpent = remainingMargin.Add(executionMargin).Add(feePaid).Add(feePaidForRemainingQuantity)
		} else {
			totalQuoteBalanceSpent = sdk.ZeroDec()
			availableQuoteBalanceSpent = margin.Add(feePaidForRemainingQuantity)
		}

		resultingDerivativeDeposits[marketIndex][trader] = derivativeDeposit{
			QuoteAvailableBalance: startingDeposits.QuoteAvailableBalance.Sub(availableQuoteBalanceSpent),
			QuoteTotalBalance:     startingDeposits.QuoteTotalBalance.Sub(totalQuoteBalanceSpent),
		}
	}
}

func (testInput *TestInput) calculateExpectedDepositsDerivativesForMarketOrders(app *simapp.InjectiveApp, ctx sdk.Context, orders []derivativesMarketTestOrder, marketIndex int, clearingPrice sdk.Dec, isBuy bool, marketId common.Hash, isDiscountedFeeRate, isBinaryOptions bool) {
	for i := 0; i < len(orders); i++ {
		order := orders[i]
		margin := sdk.NewDec(int64(order.Margin))
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(order.SubAccountNonce), 16))
		quantityFilled := sdk.NewDec(int64(order.QuantityFilled))
		quantity := sdk.NewDec(int64(order.Quantity))

		var market keeper.DerivativeMarketI
		if isBinaryOptions {
			market = app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, marketId, true)
		} else {
			market = app.ExchangeKeeper.GetDerivativeMarket(ctx, marketId, true)
		}

		feeForFilled := market.GetTakerFeeRate()
		if feeForFilled.IsPositive() && isDiscountedFeeRate {
			feeForFilled = feeForFilled.Quo(sdk.NewDec(10)).Mul(sdk.NewDec(9))
		}

		notional := clearingPrice.Mul(quantityFilled)
		feePaid := notional.Mul(sdk.ZeroDec().Add(feeForFilled))

		existingPosition := existingPreviousPositions[marketIndex][trader]

		executionMargin := margin.Mul(quantityFilled).Quo(quantity)
		if isBinaryOptions {
			orderType := exchangetypes.OrderType_BUY
			if !isBuy {
				orderType = exchangetypes.OrderType_SELL
			}
			isReduceOnly := order.Margin == 0

			executionMargin = exchangetypes.GetRequiredBinaryOptionsOrderMargin(
				clearingPrice,
				quantityFilled,
				market.GetOracleScaleFactor(),
				orderType,
				isReduceOnly,
			)
		}

		if existingPosition.Position == nil {
			existingPosition = subaccountPosition{
				trader.String(),
				&exchangetypes.Position{
					IsLong:     true,
					Quantity:   sdk.NewDec(0),
					Margin:     sdk.NewDec(0),
					EntryPrice: sdk.NewDec(0),
				},
			}
		}

		isAddingToPosition := quantityFilled.IsPositive() && (existingPosition.Position.IsLong == isBuy || existingPosition.Position.Quantity.IsZero())
		isClosingPosition := !isAddingToPosition
		oldPositionMargin := existingPosition.Position.Margin
		oldPositionQuantity := existingPosition.Position.Quantity

		var isFlipping bool

		if isAddingToPosition {
			existingPosition.Position.Quantity = existingPosition.Position.Quantity.Add(quantityFilled)
			existingPosition.Position.Margin = existingPosition.Position.Margin.Add(executionMargin)
			previousNotional := existingPosition.Position.EntryPrice.Mul(existingPosition.Position.Quantity)
			newNotional := clearingPrice.Mul(quantityFilled)
			existingPosition.Position.EntryPrice = (previousNotional.Add(newNotional)).Quo(existingPosition.Position.Quantity)
			existingPosition.Position.IsLong = isBuy
		} else if quantityFilled.IsPositive() {
			isFlipping = quantityFilled.GT(existingPosition.Position.Quantity)
			existingPosition.Position.Quantity = existingPosition.Position.Quantity.Sub(quantityFilled).Abs()
			existingPosition.Position.EntryPrice = clearingPrice

			if isFlipping {
				existingPosition.Position.IsLong = !existingPosition.Position.IsLong
				existingPosition.Position.Margin = executionMargin.Mul(existingPosition.Position.Quantity).Quo(quantityFilled)
			} else {
				existingPosition.Position.Margin = existingPosition.Position.Margin.Mul(existingPosition.Position.Quantity).Quo(oldPositionQuantity)
			}
		}

		if existingPosition.Position.Quantity.IsZero() {
			existingPosition.Position.IsLong = true
		}

		_, ok := existingPreviousPositions[marketIndex]
		if !ok {
			existingPreviousPositions[marketIndex] = make(map[common.Hash]subaccountPosition)
		}
		existingPreviousPositions[marketIndex][trader] = existingPosition

		var availableQuoteBalanceSpent, totalQuoteBalanceSpent sdk.Dec

		if isClosingPosition && quantityFilled.IsPositive() {
			closingQuantity := sdk.MinDec(oldPositionQuantity, quantityFilled)
			marginRefunded := oldPositionMargin.Mul(closingQuantity).Quo(oldPositionQuantity)
			marginUsedForFlipping := sdk.NewDec(0)

			if isFlipping {
				marginUsedForFlipping = executionMargin.Mul(existingPosition.Position.Quantity).Quo(quantity)
			}

			availableQuoteBalanceSpent = marginRefunded.Neg().Add(marginUsedForFlipping.Add(feePaid))
			totalQuoteBalanceSpent = availableQuoteBalanceSpent
		} else {
			availableQuoteBalanceSpent = executionMargin.Add(feePaid)
			totalQuoteBalanceSpent = availableQuoteBalanceSpent
		}

		startingDeposits := derivativeDeposit{
			QuoteAvailableBalance: startingDeposit,
			QuoteTotalBalance:     startingDeposit,
		}

		if resultingDerivativeDeposits[marketIndex][trader] != (derivativeDeposit{}) {
			startingDeposits = resultingDerivativeDeposits[marketIndex][trader]
		}

		resultingDerivativeDeposits[marketIndex][trader] = derivativeDeposit{
			QuoteAvailableBalance: startingDeposits.QuoteAvailableBalance.Sub(availableQuoteBalanceSpent),
			QuoteTotalBalance:     startingDeposits.QuoteTotalBalance.Sub(totalQuoteBalanceSpent),
		}
	}
}

func (testInput *TestInput) addPositions(app *simapp.InjectiveApp, ctx sdk.Context, msgServer exchangetypes.MsgServer, positions []derivativesLimitTestOrder, isBuy bool, marketId common.Hash, marketIndex int) {
	for i, positionData := range positions {
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(positions[i].SubAccountNonce), 16))
		quantity := sdk.NewDec(int64(positionData.Quantity))
		entryPrice := sdk.NewDec(int64(positionData.Price))
		margin := sdk.NewDec(int64(positionData.Margin))
		existingPosition := exchangetypes.Position{
			IsLong:     isBuy,
			Quantity:   quantity,
			EntryPrice: entryPrice,
			Margin:     margin,
		}
		originalPosition := exchangetypes.Position{
			IsLong:     isBuy,
			Quantity:   quantity,
			EntryPrice: entryPrice,
			Margin:     margin,
		}

		app.ExchangeKeeper.SetPosition(ctx, marketId, trader, &existingPosition)

		_, ok := existingPreviousPositions[marketIndex]
		if !ok {
			existingPreviousPositions[marketIndex] = make(map[common.Hash]subaccountPosition)
		}
		_, ok = originalPositions[marketIndex]
		if !ok {
			originalPositions[marketIndex] = make(map[common.Hash]subaccountPosition)
		}

		existingSubaccountPosition := subaccountPosition{
			trader.String(),
			&existingPosition,
		}
		originalSubaccountPosition := subaccountPosition{
			trader.String(),
			&originalPosition,
		}
		existingPreviousPositions[marketIndex][trader] = existingSubaccountPosition
		originalPositions[marketIndex][trader] = originalSubaccountPosition
	}
}

func (testInput *TestInput) addDerivativesLimitTestOrdersForMarketIndex(ctx sdk.Context, msgServer exchangetypes.MsgServer, orders []derivativesLimitTestOrder, isBuy, isTimeExpiry bool, marketIndex int) {
	for i := 0; i < len(orders); i++ {
		price := sdk.NewDec(int64(orders[i].Price))
		quantity := sdk.NewDec(int64(orders[i].Quantity))
		margin := sdk.NewDec(int64(orders[i].Margin))
		isPostOnly := orders[i].IsPostOnly
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))

		var orderType exchangetypes.OrderType

		if isBuy {
			if isPostOnly {
				orderType = exchangetypes.OrderType_BUY_PO
			} else {
				orderType = exchangetypes.OrderType_BUY
			}
		} else {
			if isPostOnly {
				orderType = exchangetypes.OrderType_SELL_PO
			} else {
				orderType = exchangetypes.OrderType_SELL
			}
		}

		limitOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(DefaultAddress, price, quantity, margin, orderType, trader, marketIndex, isTimeExpiry)
		_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderMsg)
		OrFail(err)

		orderNoncesPerTrader[trader]++
		orderHash, err := limitOrderMsg.Order.ComputeOrderHash(orderNoncesPerTrader[trader])
		OrFail(err)
		orders[i].OrderHashes[marketIndex] = orderHash.Hex()
	}
}

func (testInput *TestInput) addDerivativesMarketTestOrdersForMarketIndex(ctx sdk.Context, msgServer exchangetypes.MsgServer, orders []derivativesMarketTestOrder, isBuy, isTimeExpiry bool, marketIndex int) {
	for i := 0; i < len(orders); i++ {
		worstPrice := sdk.NewDec(int64(orders[i].WorstPrice))
		quantity := sdk.NewDec(int64(orders[i].Quantity))
		margin := sdk.NewDec(int64(orders[i].Margin))
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))

		var orderType exchangetypes.OrderType

		if isBuy {
			orderType = exchangetypes.OrderType_BUY
		} else {
			orderType = exchangetypes.OrderType_SELL
		}

		marketOrderMsg := testInput.NewMsgCreateDerivativeMarketOrderForMarketIndex(worstPrice, quantity, orderType, margin, trader, marketIndex, isTimeExpiry)
		_, err := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsg)
		OrFail(err)

		orderNoncesPerTrader[trader]++
		orderHash, err := marketOrderMsg.Order.ComputeOrderHash(orderNoncesPerTrader[trader])
		OrFail(err)
		orders[i].OrderHashes[marketIndex] = orderHash.Hex()
	}
}

func (testInput *TestInput) LoadDerivativeOrderbookFixture(app *simapp.InjectiveApp, ctx sdk.Context, orderbookFixturePath string, marketCount int, isTimeExpiry, isDiscountedFeeRate bool) ([]derivativesLimitTestOrder, []derivativesLimitTestOrder, testDerivativesOrderbook, []float32, []float32, []tradeEvents) {
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	jsonFile, err := os.Open(orderbookFixturePath)
	OrFail(err)
	defer jsonFile.Close()

	var orderbook testDerivativesOrderbook
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &orderbook)
	OrFail(err)

	expectedClearingPriceMatchingString := fmt.Sprintf("%f000000000000", orderbook.ExpectedClearingPriceMatching)
	expectedClearingPriceMatching, _ := sdk.NewDecFromStr(expectedClearingPriceMatchingString)
	expectedClearingPriceMarketSellsString := fmt.Sprintf("%f000000000000", orderbook.ExpectedClearingPriceMarketSells)
	expectedClearingPriceMarketSells, _ := sdk.NewDecFromStr(expectedClearingPriceMarketSellsString)
	expectedClearingPriceMarketBuysString := fmt.Sprintf("%f000000000000", orderbook.ExpectedClearingPriceMarketBuys)
	expectedClearingPriceMarketBuys, _ := sdk.NewDecFromStr(expectedClearingPriceMarketBuysString)

	for i := range orderbook.BuyOrders {
		orderbook.BuyOrders[i].OrderHashes = make([]string, marketCount)
	}
	for i := range orderbook.SellOrders {
		orderbook.SellOrders[i].OrderHashes = make([]string, marketCount)
	}
	for i := range orderbook.TransientBuyOrders {
		orderbook.TransientBuyOrders[i].OrderHashes = make([]string, marketCount)
	}
	for i := range orderbook.TransientSellOrders {
		orderbook.TransientSellOrders[i].OrderHashes = make([]string, marketCount)
	}
	for i := range orderbook.MarketBuyOrders {
		orderbook.MarketBuyOrders[i].OrderHashes = make([]string, marketCount)
	}
	for i := range orderbook.MarketSellOrders {
		orderbook.MarketSellOrders[i].OrderHashes = make([]string, marketCount)
	}

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		var market Market
		if isTimeExpiry {
			market = testInput.ExpiryMarkets[marketIndex].Market
		} else {
			market = testInput.Perps[marketIndex].Market
		}
		testInput.addPositions(app, ctx, msgServer, orderbook.ExistingPositionBuys, true, market.MarketID, marketIndex)
		testInput.addPositions(app, ctx, msgServer, orderbook.ExistingPositionSells, false, market.MarketID, marketIndex)
		testInput.addDerivativesLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.BuyOrders, true, isTimeExpiry, marketIndex)
		testInput.addDerivativesLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.SellOrders, false, isTimeExpiry, marketIndex)

		ctx, _ = EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.BuyOrders, true, marketIndex, expectedClearingPriceMatching, true, market.MarketID, isDiscountedFeeRate, false)
		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.SellOrders, true, marketIndex, expectedClearingPriceMatching, false, market.MarketID, isDiscountedFeeRate, false)
	}

	emittedABCIEvents := make([]abci.Event, 0)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		var market Market
		if isTimeExpiry {
			market = testInput.ExpiryMarkets[marketIndex].Market
		} else {
			market = testInput.Perps[marketIndex].Market
		}
		testInput.addDerivativesLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.TransientBuyOrders, true, isTimeExpiry, marketIndex)
		testInput.addDerivativesLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.TransientSellOrders, false, isTimeExpiry, marketIndex)
		testInput.addDerivativesMarketTestOrdersForMarketIndex(ctx, msgServer, orderbook.MarketBuyOrders, true, isTimeExpiry, marketIndex)
		testInput.addDerivativesMarketTestOrdersForMarketIndex(ctx, msgServer, orderbook.MarketSellOrders, false, isTimeExpiry, marketIndex)

		timestamp := ctx.BlockTime().Unix() + (35000 * (int64(marketIndex) + 1))
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))

		var newMarketEmittedABCIEvents []abci.Event
		ctx, newMarketEmittedABCIEvents = EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		emittedABCIEvents = append(emittedABCIEvents, newMarketEmittedABCIEvents...)

		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.TransientBuyOrders, false, marketIndex, expectedClearingPriceMatching, true, market.MarketID, isDiscountedFeeRate, false)
		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.TransientSellOrders, false, marketIndex, expectedClearingPriceMatching, false, market.MarketID, isDiscountedFeeRate, false)
		testInput.calculateExpectedDepositsDerivativesForMarketOrders(app, ctx, orderbook.MarketBuyOrders, marketIndex, expectedClearingPriceMarketBuys, true, market.MarketID, isDiscountedFeeRate, false)
		testInput.calculateExpectedDepositsDerivativesForMarketOrders(app, ctx, orderbook.MarketSellOrders, marketIndex, expectedClearingPriceMarketSells, false, market.MarketID, isDiscountedFeeRate, false)

		timestamp = ctx.BlockTime().Unix() + (50000 * (int64(marketIndex) + 1))
		ctx := ctx.WithBlockTime(time.Unix(timestamp, 0))

		EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}

	expectedBuyOrdersAfterMatching := make([]derivativesLimitTestOrder, 0)
	expectedSellOrdersAfterMatching := make([]derivativesLimitTestOrder, 0)

	buyOrders := append(orderbook.TransientBuyOrders, orderbook.BuyOrders...)
	sellOrders := append(orderbook.TransientSellOrders, orderbook.SellOrders...)

	for _, order := range buyOrders {
		order.Quantity = order.Quantity - order.ExpectedQuantityFilledViaMatching - order.ExpectedQuantityFilledViaMarketOrders
		if order.Quantity != 0 {
			expectedBuyOrdersAfterMatching = append(expectedBuyOrdersAfterMatching, order)
		}
	}

	for _, order := range sellOrders {
		order.Quantity = order.Quantity - order.ExpectedQuantityFilledViaMatching - order.ExpectedQuantityFilledViaMarketOrders
		if order.Quantity != 0 {
			expectedSellOrdersAfterMatching = append(expectedSellOrdersAfterMatching, order)
		}
	}

	expectedLimitFillFromBuysPrices := make([]float32, 0)
	expectedLimitFillFromSellsPrices := make([]float32, 0)

	allEmittedTradeEvents := make([]tradeEvents, marketCount)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		emittedTradeEvents := tradeEvents{
			limitMatches:         make([]*exchangetypes.DerivativeTradeLog, 0),
			limitFillsFromBuys:   make([]*exchangetypes.DerivativeTradeLog, 0),
			limitFillsFromSells:  make([]*exchangetypes.DerivativeTradeLog, 0),
			marketBuys:           make([]*exchangetypes.DerivativeTradeLog, 0),
			marketSells:          make([]*exchangetypes.DerivativeTradeLog, 0),
			newBuyOrders:         make([]*exchangetypes.DerivativeLimitOrder, 0),
			newSellOrders:        make([]*exchangetypes.DerivativeLimitOrder, 0),
			marketUpdates:        make([]*exchangetypes.EventPerpetualMarketUpdate, 0),
			marketFundingUpdates: make([]*exchangetypes.EventPerpetualMarketFundingUpdate, 0),
			expiryMarketUpdates:  make([]*exchangetypes.EventExpiryFuturesMarketUpdate, 0),
		}
		allEmittedTradeEvents[marketIndex] = emittedTradeEvents
	}

	for _, emittedABCIEvent := range emittedABCIEvents {
		switch emittedABCIEvent.Type {
		case banktypes.EventTypeCoinReceived, banktypes.EventTypeCoinMint, banktypes.EventTypeCoinSpent, banktypes.EventTypeTransfer, sdk.EventTypeMessage:
			continue
		}
		parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
		OrFail(err1)

		switch event := parsedEvent.(type) {
		case *exchangetypes.EventBatchDerivativeExecution:
			marketIndex := testInput.GetMarketIndexFromID(event.MarketId)

			if event.ExecutionType == exchangetypes.ExecutionType_LimitFill && event.IsBuy {
				allEmittedTradeEvents[marketIndex].limitFillsFromBuys = append(allEmittedTradeEvents[marketIndex].limitFillsFromBuys, event.Trades...)
				break
			} else if event.ExecutionType == exchangetypes.ExecutionType_LimitFill {
				allEmittedTradeEvents[marketIndex].limitFillsFromSells = append(allEmittedTradeEvents[marketIndex].limitFillsFromSells, event.Trades...)
				break
			}

			if event.ExecutionType == exchangetypes.ExecutionType_LimitMatchNewOrder || event.ExecutionType == exchangetypes.ExecutionType_LimitMatchRestingOrder {
				allEmittedTradeEvents[marketIndex].limitMatches = append(allEmittedTradeEvents[marketIndex].limitMatches, event.Trades...)
				break
			}

			if event.ExecutionType == exchangetypes.ExecutionType_Market && event.IsBuy {
				allEmittedTradeEvents[marketIndex].marketBuys = append(allEmittedTradeEvents[marketIndex].marketBuys, event.Trades...)
			} else if event.ExecutionType == exchangetypes.ExecutionType_Market {
				allEmittedTradeEvents[marketIndex].marketSells = append(allEmittedTradeEvents[marketIndex].marketSells, event.Trades...)
			}

		case *exchangetypes.EventNewDerivativeOrders:
			marketIndex := testInput.GetMarketIndexFromID(event.MarketId)

			allEmittedTradeEvents[marketIndex].newBuyOrders = append(allEmittedTradeEvents[marketIndex].newBuyOrders, event.BuyOrders...)
			allEmittedTradeEvents[marketIndex].newSellOrders = append(allEmittedTradeEvents[marketIndex].newSellOrders, event.SellOrders...)
		case *exchangetypes.EventPerpetualMarketUpdate:
			if !isTimeExpiry {
				marketIndex := testInput.GetMarketIndexFromID(event.Market.MarketId)
				if marketIndex < marketCount {
					allEmittedTradeEvents[marketIndex].marketUpdates = append(allEmittedTradeEvents[marketIndex].marketUpdates, event)
				}
			}
		case *exchangetypes.EventPerpetualMarketFundingUpdate:
			if !isTimeExpiry {
				marketIndex := testInput.GetMarketIndexFromID(event.MarketId)
				allEmittedTradeEvents[marketIndex].marketFundingUpdates = append(allEmittedTradeEvents[marketIndex].marketFundingUpdates, event)
			}
		case *exchangetypes.EventExpiryFuturesMarketUpdate:
			if isTimeExpiry {
				marketIndex := testInput.GetMarketIndexFromID(event.Market.MarketId)
				if marketIndex < marketCount {
					allEmittedTradeEvents[marketIndex].expiryMarketUpdates = append(allEmittedTradeEvents[marketIndex].expiryMarketUpdates, event)
				}
			}
		}
	}

	for _, buyOrder := range orderbook.BuyOrders {
		if buyOrder.ExpectedQuantityFilledViaMarketOrders > 0 {
			expectedLimitFillFromBuysPrices = append(expectedLimitFillFromBuysPrices, buyOrder.Price)
		}
	}
	for _, sellOrder := range orderbook.SellOrders {
		if sellOrder.ExpectedQuantityFilledViaMarketOrders > 0 {
			expectedLimitFillFromSellsPrices = append([]float32{sellOrder.Price}, expectedLimitFillFromSellsPrices...)
		}
	}

	return expectedBuyOrdersAfterMatching,
		expectedSellOrdersAfterMatching,
		orderbook,
		expectedLimitFillFromBuysPrices,
		expectedLimitFillFromSellsPrices,
		allEmittedTradeEvents
}

func ExpectCorrectDerivativeOrderbookMatching(testInput *TestInput, app **simapp.InjectiveApp, ctx *sdk.Context, orderbookFixturePath string, marketCount int, isTimeExpiry, isDiscountedFeeRate bool) {
	resultingDerivativeDeposits = make([]map[common.Hash]derivativeDeposit, marketCount)
	existingPreviousPositions = make(map[int]map[common.Hash]subaccountPosition)
	originalPositions = make(map[int]map[common.Hash]subaccountPosition)
	orderNoncesPerTrader = make(map[common.Hash]uint32)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		resultingDerivativeDeposits[marketIndex] = make(map[common.Hash]derivativeDeposit)
	}
	testInput.AddDerivativeDeposits(*app, *ctx, marketCount, isTimeExpiry)

	expectedBuyOrdersAfterMatching,
		expectedSellOrdersAfterMatching,
		orderbook,
		expectedLimitFillFromBuysPrices,
		expectedLimitFillFromSellsPrices,
		allEmittedTradeEvents := testInput.LoadDerivativeOrderbookFixture(*app, *ctx, orderbookFixturePath, marketCount, isTimeExpiry, isDiscountedFeeRate)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		emittedTradeEvents := allEmittedTradeEvents[marketIndex]

		var market Market
		if isTimeExpiry {
			market = testInput.ExpiryMarkets[marketIndex].Market
		} else {
			market = testInput.Perps[marketIndex].Market
		}

		limitBuyOrders := (*app).ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(*ctx, market.MarketID, true)
		limitSellOrders := (*app).ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(*ctx, market.MarketID, false)

		for i := 0; i < len(expectedBuyOrdersAfterMatching); i++ {
			Expect(limitBuyOrders[i].OrderInfo.Price.String()).Should(Equal(sdk.NewDec(int64(expectedBuyOrdersAfterMatching[i].Price)).String()))
			Expect(limitBuyOrders[i].Fillable.String()).Should(Equal(sdk.NewDec(int64(expectedBuyOrdersAfterMatching[i].Quantity)).String()))
		}

		for i := 0; i < len(expectedSellOrdersAfterMatching); i++ {
			Expect(limitSellOrders[i].OrderInfo.Price.String()).Should(Equal(sdk.NewDec(int64(expectedSellOrdersAfterMatching[i].Price)).String()))
			Expect(limitSellOrders[i].Fillable.String()).Should(Equal(sdk.NewDec(int64(expectedSellOrdersAfterMatching[i].Quantity)).String()))
		}

		if orderbook.ExpectedClearingPriceMatching > 0 {
			Expect(emittedTradeEvents.limitMatches).Should(Not(BeEmpty()))
		}
		if orderbook.ExpectedClearingPriceMarketBuys > 0 {
			Expect(emittedTradeEvents.marketBuys).Should(Not(BeEmpty()))
		}
		if orderbook.ExpectedClearingPriceMarketSells > 0 {
			Expect(emittedTradeEvents.marketSells).Should(Not(BeEmpty()))
		}

		limitMatchPrices := make([]float32, 0)
		marketBuyPrices := make([]float32, 0)
		marketSellPrices := make([]float32, 0)

		for range emittedTradeEvents.limitMatches {
			limitMatchPrices = append(limitMatchPrices, orderbook.ExpectedClearingPriceMatching)
		}
		for range emittedTradeEvents.marketBuys {
			marketBuyPrices = append(marketBuyPrices, orderbook.ExpectedClearingPriceMarketBuys)
		}
		for range emittedTradeEvents.marketSells {
			marketSellPrices = append(marketSellPrices, orderbook.ExpectedClearingPriceMarketSells)
		}

		derivativesMarket := (*app).ExchangeKeeper.GetDerivativeMarket(*ctx, market.MarketID, true)
		expectedOrderMatchEventData := make(map[string]expectedOrderEventData)

		for _, buyOrder := range orderbook.BuyOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(buyOrder.SubAccountNonce), 16))
			totalFilled := buyOrder.ExpectedQuantityFilledViaMatching + buyOrder.ExpectedQuantityFilledViaMarketOrders
			expectedExecutionMargin := sdk.NewDec(int64(buyOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(buyOrder.Quantity)))

			expectedOrderMatchEventData[buyOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(buyOrder.ExpectedQuantityFilledViaMatching),
				marketFilled:    uint64(buyOrder.ExpectedQuantityFilledViaMarketOrders),
				executionMargin: expectedExecutionMargin,
				isLong:          true,
				isMaker:         true,
				subaccountId:    trader,
			}
		}
		for _, sellOrder := range orderbook.SellOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(sellOrder.SubAccountNonce), 16))
			totalFilled := sellOrder.ExpectedQuantityFilledViaMatching + sellOrder.ExpectedQuantityFilledViaMarketOrders
			expectedExecutionMargin := sdk.NewDec(int64(sellOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(sellOrder.Quantity)))
			expectedOrderMatchEventData[sellOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(sellOrder.ExpectedQuantityFilledViaMatching),
				marketFilled:    uint64(sellOrder.ExpectedQuantityFilledViaMarketOrders),
				executionMargin: expectedExecutionMargin,
				isLong:          false,
				isMaker:         true,
				subaccountId:    trader,
			}
		}
		for _, buyOrder := range orderbook.TransientBuyOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(buyOrder.SubAccountNonce), 16))
			totalFilled := buyOrder.ExpectedQuantityFilledViaMatching + buyOrder.ExpectedQuantityFilledViaMarketOrders
			expectedExecutionMargin := sdk.NewDec(int64(buyOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(buyOrder.Quantity)))
			expectedOrderMatchEventData[buyOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(buyOrder.ExpectedQuantityFilledViaMatching),
				marketFilled:    uint64(buyOrder.ExpectedQuantityFilledViaMarketOrders),
				executionMargin: expectedExecutionMargin,
				isLong:          true,
				isMaker:         false,
				subaccountId:    trader,
			}
		}
		for _, sellOrder := range orderbook.TransientSellOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(sellOrder.SubAccountNonce), 16))
			totalFilled := sellOrder.ExpectedQuantityFilledViaMatching + sellOrder.ExpectedQuantityFilledViaMarketOrders
			expectedExecutionMargin := sdk.NewDec(int64(sellOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(sellOrder.Quantity)))
			expectedOrderMatchEventData[sellOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(sellOrder.ExpectedQuantityFilledViaMatching),
				marketFilled:    uint64(sellOrder.ExpectedQuantityFilledViaMarketOrders),
				executionMargin: expectedExecutionMargin,
				isLong:          false,
				isMaker:         false,
				subaccountId:    trader,
			}
		}
		for _, buyOrder := range orderbook.MarketBuyOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(buyOrder.SubAccountNonce), 16))
			totalFilled := buyOrder.QuantityFilled
			expectedExecutionMargin := sdk.NewDec(int64(buyOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(buyOrder.Quantity)))
			expectedOrderMatchEventData[buyOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(0),
				marketFilled:    uint64(buyOrder.QuantityFilled),
				executionMargin: expectedExecutionMargin,
				isLong:          true,
				isMaker:         false,
				subaccountId:    trader,
			}
		}
		for _, sellOrder := range orderbook.MarketSellOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(sellOrder.SubAccountNonce), 16))
			totalFilled := sellOrder.QuantityFilled
			expectedExecutionMargin := sdk.NewDec(int64(sellOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(sellOrder.Quantity)))
			expectedOrderMatchEventData[sellOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(0),
				marketFilled:    uint64(sellOrder.QuantityFilled),
				executionMargin: expectedExecutionMargin,
				isLong:          false,
				isMaker:         false,
				subaccountId:    trader,
			}
		}

		for _, marketUpdate := range emittedTradeEvents.marketUpdates {
			Expect(marketUpdate.Funding.CumulativeFunding.String()).Should(Equal(sdk.ZeroDec().String()))
			Expect(marketUpdate.Funding.CumulativePrice.String()).Should(Equal(sdk.ZeroDec().String()))
			// Expect(marketUpdate.Funding.LastTimestamp).Should(Equal(TestInput.Context.BlockTime().Unix()))

			Expect(marketUpdate.Market.InitialMarginRatio.String()).Should(Equal(derivativesMarket.InitialMarginRatio.String()))
			Expect(marketUpdate.Market.MaintenanceMarginRatio.String()).Should(Equal(derivativesMarket.MaintenanceMarginRatio.String()))
			// Expect(marketUpdate.Market.MakerFeeRate.String()).Should(Equal(derivativesMarket.MakerFeeRate.String()))
			Expect(marketUpdate.Market.TakerFeeRate.String()).Should(Equal(derivativesMarket.TakerFeeRate.String()))
			Expect(marketUpdate.Market.RelayerFeeShareRate.String()).Should(Equal(derivativesMarket.RelayerFeeShareRate.String()))
			Expect(marketUpdate.Market.MinPriceTickSize.String()).Should(Equal(derivativesMarket.MinPriceTickSize.String()))
			Expect(marketUpdate.Market.MinQuantityTickSize.String()).Should(Equal(derivativesMarket.MinQuantityTickSize.String()))
			Expect(marketUpdate.Market.IsPerpetual).Should(Equal(derivativesMarket.IsPerpetual))
			Expect(marketUpdate.Market.Status).Should(Equal(derivativesMarket.Status))
			Expect(marketUpdate.Market.MarketId).Should(Equal(derivativesMarket.MarketId))
			Expect(marketUpdate.Market.Ticker).Should(Equal(derivativesMarket.Ticker))
			Expect(marketUpdate.Market.OracleBase).Should(Equal(derivativesMarket.OracleBase))
			Expect(marketUpdate.Market.OracleQuote).Should(Equal(derivativesMarket.OracleQuote))
			Expect(marketUpdate.Market.OracleType).Should(Equal(derivativesMarket.OracleType))
			Expect(marketUpdate.Market.OracleScaleFactor).Should(Equal(derivativesMarket.OracleScaleFactor))
			Expect(marketUpdate.Market.QuoteDenom).Should(Equal(derivativesMarket.QuoteDenom))

			Expect(marketUpdate.PerpetualMarketInfo.FundingInterval).Should(Equal(exchangetypes.DefaultFundingIntervalSeconds))
			Expect(marketUpdate.PerpetualMarketInfo.HourlyFundingRateCap.String()).Should(Equal(TestingExchangeParams.DefaultHourlyFundingRateCap.String()))
			// Expect(marketUpdate.PerpetualMarketInfo.HourlyInterestRate.String()).Should(Equal(TestingExchangeParams.DefaultHourlyInterestRate.String()))
			Expect(marketUpdate.PerpetualMarketInfo.MarketId).Should(Equal(derivativesMarket.MarketId))
			// Expect(marketUpdate.PerpetualMarketInfo.NextFundingTimestamp).Should(Equal(TestInput.Context.BlockTime().Unix() + exchangetypes.DefaultFundingIntervalSeconds))
		}

		for _, marketUpdate := range emittedTradeEvents.expiryMarketUpdates {
			Expect(marketUpdate.Market.InitialMarginRatio.String()).Should(Equal(derivativesMarket.InitialMarginRatio.String()))
			Expect(marketUpdate.Market.MaintenanceMarginRatio.String()).Should(Equal(derivativesMarket.MaintenanceMarginRatio.String()))
			// Expect(marketUpdate.Market.MakerFeeRate.String()).Should(Equal(derivativesMarket.MakerFeeRate.String()))
			Expect(marketUpdate.Market.TakerFeeRate.String()).Should(Equal(derivativesMarket.TakerFeeRate.String()))
			Expect(marketUpdate.Market.RelayerFeeShareRate.String()).Should(Equal(derivativesMarket.RelayerFeeShareRate.String()))
			Expect(marketUpdate.Market.MinPriceTickSize.String()).Should(Equal(derivativesMarket.MinPriceTickSize.String()))
			Expect(marketUpdate.Market.MinQuantityTickSize.String()).Should(Equal(derivativesMarket.MinQuantityTickSize.String()))
			Expect(marketUpdate.Market.IsPerpetual).Should(Equal(derivativesMarket.IsPerpetual))
			Expect(marketUpdate.Market.Status).Should(Equal(derivativesMarket.Status))
			Expect(marketUpdate.Market.MarketId).Should(Equal(derivativesMarket.MarketId))
			Expect(marketUpdate.Market.Ticker).Should(Equal(derivativesMarket.Ticker))
			Expect(marketUpdate.Market.OracleBase).Should(Equal(derivativesMarket.OracleBase))
			Expect(marketUpdate.Market.OracleQuote).Should(Equal(derivativesMarket.OracleQuote))
			Expect(marketUpdate.Market.OracleType).Should(Equal(derivativesMarket.OracleType))
			Expect(marketUpdate.Market.OracleScaleFactor).Should(Equal(derivativesMarket.OracleScaleFactor))
			Expect(marketUpdate.Market.QuoteDenom).Should(Equal(derivativesMarket.QuoteDenom))

			Expect(marketUpdate.ExpiryFuturesMarketInfo.MarketId).Should(Equal(derivativesMarket.MarketId))
			Expect(marketUpdate.ExpiryFuturesMarketInfo.TwapStartTimestamp).Should(Equal(ctx.BlockTime().Unix() + FourWeeksInSeconds - ThirtyMinutesInSeconds))
			Expect(marketUpdate.ExpiryFuturesMarketInfo.ExpirationTwapStartPriceCumulative).Should(Equal(sdk.ZeroDec()))
			Expect(marketUpdate.ExpiryFuturesMarketInfo.SettlementPrice).Should(Equal(sdk.ZeroDec()))
			Expect(marketUpdate.ExpiryFuturesMarketInfo.ExpirationTimestamp).Should(Equal(ctx.BlockTime().Unix() + FourWeeksInSeconds))
		}

		for _, marketFundingUpdate := range emittedTradeEvents.marketFundingUpdates {
			if marketFundingUpdate.IsHourlyFunding {
				Expect(marketFundingUpdate.Funding.CumulativePrice.String()).Should(Equal(sdk.ZeroDec().String()))
			}
			// Expect(marketFundingUpdate.Funding.CumulativeFunding.String()).Should(Equal(sdk.ZeroDec().String()))

			// Expect(marketFundingUpdate.Funding.LastTimestamp % 3600).Should(BeZero())
			Expect(marketFundingUpdate.MarketId).Should(Equal(derivativesMarket.MarketId))
		}

		expectCorrectOrderEventValues(emittedTradeEvents.marketBuys, marketBuyPrices, marketIndex, expectedOrderMatchEventData, derivativesMarket, true, isDiscountedFeeRate)
		expectCorrectOrderEventValues(emittedTradeEvents.marketSells, marketSellPrices, marketIndex, expectedOrderMatchEventData, derivativesMarket, true, isDiscountedFeeRate)
		expectCorrectOrderEventValues(emittedTradeEvents.limitFillsFromBuys, expectedLimitFillFromBuysPrices, marketIndex, expectedOrderMatchEventData, derivativesMarket, true, isDiscountedFeeRate)
		expectCorrectOrderEventValues(emittedTradeEvents.limitFillsFromSells, expectedLimitFillFromSellsPrices, marketIndex, expectedOrderMatchEventData, derivativesMarket, true, isDiscountedFeeRate)
		expectCorrectOrderEventValues(emittedTradeEvents.limitMatches, limitMatchPrices, marketIndex, expectedOrderMatchEventData, derivativesMarket, false, isDiscountedFeeRate)

		allNewBuyOrders := make([]derivativesLimitTestOrder, 0)  // orderbook.BuyOrders
		allNewSellOrders := make([]derivativesLimitTestOrder, 0) // orderbook.SellOrders

		for _, buyOrder := range orderbook.TransientBuyOrders {
			if buyOrder.Quantity > buyOrder.ExpectedQuantityFilledViaMarketOrders+buyOrder.ExpectedQuantityFilledViaMatching {
				allNewBuyOrders = append(allNewBuyOrders, buyOrder)
			}
		}
		for _, sellOrder := range orderbook.TransientSellOrders {
			if sellOrder.Quantity > sellOrder.ExpectedQuantityFilledViaMarketOrders+sellOrder.ExpectedQuantityFilledViaMatching {
				allNewSellOrders = append(allNewSellOrders, sellOrder)
			}
		}
		for i, buyOrder := range emittedTradeEvents.newBuyOrders {
			expectedPrice := sdk.NewDec(int64(allNewBuyOrders[i].Price))
			Expect(buyOrder.OrderInfo.Price.String()).Should(Equal(expectedPrice.String()))
		}
		for i, newSellOrder := range emittedTradeEvents.newSellOrders {
			expectedPrice := sdk.NewDec(int64(allNewSellOrders[len(allNewSellOrders)-1-i].Price))
			Expect(newSellOrder.OrderInfo.Price.String()).Should(Equal(expectedPrice.String()))
		}

		for key, value := range resultingDerivativeDeposits[marketIndex] {
			depositQuoteAsset := (*app).ExchangeKeeper.GetDeposit(*ctx, key, market.QuoteDenom)

			Expect(depositQuoteAsset.AvailableBalance.String()).Should(Equal(value.QuoteAvailableBalance.String()))
			Expect(depositQuoteAsset.TotalBalance.String()).Should(Equal(value.QuoteTotalBalance.String()))
		}

		positionsForSpecificMarket := (*app).ExchangeKeeper.GetAllPositionsByMarket(*ctx, market.MarketID)

		for i := range positionsForSpecificMarket {
			subaccountId := common.HexToHash(positionsForSpecificMarket[i].SubaccountId)
			position := existingPreviousPositions[marketIndex][subaccountId]

			Expect(positionsForSpecificMarket[i].Position.Quantity.String()).Should(Equal(position.Position.Quantity.String()))
			Expect(positionsForSpecificMarket[i].Position.Margin.String()).Should(Equal(position.Position.Margin.String()))
			Expect(positionsForSpecificMarket[i].Position.EntryPrice.String()).Should(Equal(position.Position.EntryPrice.String()))
			Expect(positionsForSpecificMarket[i].Position.IsLong).Should(Equal(position.Position.IsLong))
		}
	}

	sender, err := sdk.AccAddressFromBech32(DefaultAddress)
	OrFail(err)

	hasReceivedINJ := false
	receivedTokens := (*app).BankKeeper.GetAllBalances(*ctx, sender)

	for _, receivedToken := range receivedTokens {
		if receivedToken.Denom == "inj" {
			hasReceivedINJ = true
			expectedRewardTokens := sdk.NewInt(100000)
			Expect(receivedToken.Amount.String()).Should(Equal(expectedRewardTokens.String()))
		}
	}

	Expect(hasReceivedINJ).Should(BeTrue())

	Expect((*app).ExchangeKeeper.IsMetadataInvariantValid(*ctx)).To(BeTrue())
	InvariantCheckBalanceAndSupply(*app, *ctx, testInput)
	InvariantCheckInsuranceModuleBalance(*app, *ctx)
	InvariantCheckAccountFees(*app, *ctx)
}
