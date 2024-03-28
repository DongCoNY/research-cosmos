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

func (testInput *TestInput) addBinaryOptionsLimitTestOrdersForMarketIndex(ctx sdk.Context, msgServer exchangetypes.MsgServer, orders []derivativesLimitTestOrder, isBuy bool, marketIndex int) {
	for i := 0; i < len(orders); i++ {
		price := sdk.NewDec(int64(orders[i].Price))
		quantity := sdk.NewDec(int64(orders[i].Quantity))
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))

		var orderType exchangetypes.OrderType

		if isBuy {
			orderType = exchangetypes.OrderType_BUY
		} else {
			orderType = exchangetypes.OrderType_SELL
		}

		isReduceOnly := orders[i].Margin == 0

		limitOrderMsg := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(DefaultAddress, price, quantity, orderType, trader, marketIndex, isReduceOnly)
		_, err := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), limitOrderMsg)
		OrFail(err)

		orderNoncesPerTrader[trader]++
		orderHash, err := limitOrderMsg.Order.ComputeOrderHash(orderNoncesPerTrader[trader])
		OrFail(err)
		orders[i].OrderHashes[marketIndex] = orderHash.Hex()
	}
}

func (testInput *TestInput) addBinaryOptionsMarketTestOrdersForMarketIndex(ctx sdk.Context, msgServer exchangetypes.MsgServer, orders []derivativesMarketTestOrder, isBuy bool, marketIndex int) {
	for i := 0; i < len(orders); i++ {
		worstPrice := sdk.NewDec(int64(orders[i].WorstPrice))
		quantity := sdk.NewDec(int64(orders[i].Quantity))
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))

		var orderType exchangetypes.OrderType

		if isBuy {
			orderType = exchangetypes.OrderType_BUY
		} else {
			orderType = exchangetypes.OrderType_SELL
		}

		isReduceOnly := orders[i].Margin == 0

		marketOrderMsg := testInput.NewMsgCreateBinaryOptionsMarketOrderForMarketIndex(DefaultAddress, worstPrice, quantity, orderType, trader, marketIndex, isReduceOnly)
		_, err := msgServer.CreateBinaryOptionsMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsg)
		OrFail(err)

		orderNoncesPerTrader[trader]++
		orderHash, err := marketOrderMsg.Order.ComputeOrderHash(orderNoncesPerTrader[trader])
		OrFail(err)
		orders[i].OrderHashes[marketIndex] = orderHash.Hex()
	}
}

func (testInput *TestInput) LoadBinaryOptionsOrderbookFixture(app *simapp.InjectiveApp, ctx sdk.Context, orderbookFixturePath string, marketCount int, isDiscountedFeeRate bool) ([]derivativesLimitTestOrder, []derivativesLimitTestOrder, testDerivativesOrderbook, []float32, []float32, []tradeEvents) {
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
		market := testInput.BinaryMarkets[marketIndex].Market
		testInput.addPositions(app, ctx, msgServer, orderbook.ExistingPositionBuys, true, market.MarketID, marketIndex)
		testInput.addPositions(app, ctx, msgServer, orderbook.ExistingPositionSells, false, market.MarketID, marketIndex)
		testInput.addBinaryOptionsLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.BuyOrders, true, marketIndex)
		testInput.addBinaryOptionsLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.SellOrders, false, marketIndex)

		ctx, _ = EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.BuyOrders, true, marketIndex, expectedClearingPriceMatching, true, market.MarketID, isDiscountedFeeRate, true)
		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.SellOrders, true, marketIndex, expectedClearingPriceMatching, false, market.MarketID, isDiscountedFeeRate, true)
	}

	emittedABCIEvents := make([]abci.Event, 0)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		market := testInput.BinaryMarkets[marketIndex].Market
		testInput.addBinaryOptionsLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.TransientBuyOrders, true, marketIndex)
		testInput.addBinaryOptionsLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.TransientSellOrders, false, marketIndex)
		testInput.addBinaryOptionsMarketTestOrdersForMarketIndex(ctx, msgServer, orderbook.MarketBuyOrders, true, marketIndex)
		testInput.addBinaryOptionsMarketTestOrdersForMarketIndex(ctx, msgServer, orderbook.MarketSellOrders, false, marketIndex)

		timestamp := ctx.BlockTime().Unix() + (35000 * (int64(marketIndex) + 1))
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))

		var newMarketEmittedABCIEvents []abci.Event
		ctx, newMarketEmittedABCIEvents = EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		emittedABCIEvents = append(emittedABCIEvents, newMarketEmittedABCIEvents...)

		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.TransientBuyOrders, false, marketIndex, expectedClearingPriceMatching, true, market.MarketID, isDiscountedFeeRate, true)
		testInput.calculateExpectedDerivativeDepositsForLimitOrders(app, ctx, orderbook.TransientSellOrders, false, marketIndex, expectedClearingPriceMatching, false, market.MarketID, isDiscountedFeeRate, true)
		testInput.calculateExpectedDepositsDerivativesForMarketOrders(app, ctx, orderbook.MarketBuyOrders, marketIndex, expectedClearingPriceMarketBuys, true, market.MarketID, isDiscountedFeeRate, true)
		testInput.calculateExpectedDepositsDerivativesForMarketOrders(app, ctx, orderbook.MarketSellOrders, marketIndex, expectedClearingPriceMarketSells, false, market.MarketID, isDiscountedFeeRate, true)

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
			limitMatches:               make([]*exchangetypes.DerivativeTradeLog, 0),
			limitFillsFromBuys:         make([]*exchangetypes.DerivativeTradeLog, 0),
			limitFillsFromSells:        make([]*exchangetypes.DerivativeTradeLog, 0),
			marketBuys:                 make([]*exchangetypes.DerivativeTradeLog, 0),
			marketSells:                make([]*exchangetypes.DerivativeTradeLog, 0),
			newBuyOrders:               make([]*exchangetypes.DerivativeLimitOrder, 0),
			newSellOrders:              make([]*exchangetypes.DerivativeLimitOrder, 0),
			marketUpdates:              make([]*exchangetypes.EventPerpetualMarketUpdate, 0),
			marketFundingUpdates:       make([]*exchangetypes.EventPerpetualMarketFundingUpdate, 0),
			expiryMarketUpdates:        make([]*exchangetypes.EventExpiryFuturesMarketUpdate, 0),
			binaryOptionsMarketUpdates: make([]*exchangetypes.EventBinaryOptionsMarketUpdate, 0),
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
		case *exchangetypes.EventBinaryOptionsMarketUpdate:
			marketIndex := testInput.GetMarketIndexFromID(event.Market.MarketId)
			if marketIndex < marketCount {
				allEmittedTradeEvents[marketIndex].binaryOptionsMarketUpdates = append(allEmittedTradeEvents[marketIndex].binaryOptionsMarketUpdates, event)
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

func ExpectCorrectBinaryOptionsOrderbookMatching(testInput *TestInput, app **simapp.InjectiveApp, ctx *sdk.Context, orderbookFixturePath string, marketCount int, isDiscountedFeeRate bool) {
	resultingDerivativeDeposits = make([]map[common.Hash]derivativeDeposit, marketCount)
	existingPreviousPositions = make(map[int]map[common.Hash]subaccountPosition)
	originalPositions = make(map[int]map[common.Hash]subaccountPosition)
	orderNoncesPerTrader = make(map[common.Hash]uint32)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		resultingDerivativeDeposits[marketIndex] = make(map[common.Hash]derivativeDeposit)
	}
	testInput.AddBinaryOptionsDeposits(*app, *ctx, marketCount)

	expectedBuyOrdersAfterMatching,
		expectedSellOrdersAfterMatching,
		orderbook,
		expectedLimitFillFromBuysPrices,
		expectedLimitFillFromSellsPrices,
		allEmittedTradeEvents := testInput.LoadBinaryOptionsOrderbookFixture(*app, *ctx, orderbookFixturePath, marketCount, isDiscountedFeeRate)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		emittedTradeEvents := allEmittedTradeEvents[marketIndex]
		market := testInput.BinaryMarkets[marketIndex].Market

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

		derivativesMarket := (*app).ExchangeKeeper.GetBinaryOptionsMarket(*ctx, market.MarketID, true)
		expectedOrderMatchEventData := make(map[string]expectedOrderEventData)

		for _, buyOrder := range orderbook.BuyOrders {
			trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(buyOrder.SubAccountNonce), 16))
			totalFilled := buyOrder.ExpectedQuantityFilledViaMatching + buyOrder.ExpectedQuantityFilledViaMarketOrders
			expectedExecutionMargin := sdk.NewDec(int64(buyOrder.Margin)).Mul(sdk.NewDec(int64(totalFilled))).Quo(sdk.NewDec(int64(buyOrder.Quantity)))

			expectedOrderMatchEventData[buyOrder.OrderHashes[marketIndex]] = expectedOrderEventData{
				limitFilled:     uint64(buyOrder.ExpectedQuantityFilledViaMatching),
				marketFilled:    uint64(buyOrder.ExpectedQuantityFilledViaMarketOrders),
				executionMargin: expectedExecutionMargin, // note: this is ignored/overwritten for binary options
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
				executionMargin: expectedExecutionMargin, // note: this is ignored/overwritten for binary options
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
				executionMargin: expectedExecutionMargin, // note: this is ignored/overwritten for binary options
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
				executionMargin: expectedExecutionMargin, // note: this is ignored/overwritten for binary options
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
				executionMargin: expectedExecutionMargin, // note: this is ignored/overwritten for binary options
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
				executionMargin: expectedExecutionMargin, // note: this is ignored/overwritten for binary options
				isLong:          false,
				isMaker:         false,
				subaccountId:    trader,
			}
		}

		for _, marketUpdate := range emittedTradeEvents.binaryOptionsMarketUpdates {
			Expect(marketUpdate.Market.TakerFeeRate.String()).Should(Equal(derivativesMarket.TakerFeeRate.String()))
			Expect(marketUpdate.Market.RelayerFeeShareRate.String()).Should(Equal(derivativesMarket.RelayerFeeShareRate.String()))
			Expect(marketUpdate.Market.MinPriceTickSize.String()).Should(Equal(derivativesMarket.MinPriceTickSize.String()))
			Expect(marketUpdate.Market.MinQuantityTickSize.String()).Should(Equal(derivativesMarket.MinQuantityTickSize.String()))
			Expect(marketUpdate.Market.Status).Should(Equal(derivativesMarket.Status))
			Expect(marketUpdate.Market.MarketId).Should(Equal(derivativesMarket.MarketId))
			Expect(marketUpdate.Market.Ticker).Should(Equal(derivativesMarket.Ticker))
			Expect(marketUpdate.Market.OracleType).Should(Equal(derivativesMarket.OracleType))
			Expect(marketUpdate.Market.OracleScaleFactor).Should(Equal(derivativesMarket.OracleScaleFactor))
			Expect(marketUpdate.Market.QuoteDenom).Should(Equal(derivativesMarket.QuoteDenom))
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
			// diff := depositQuoteAsset.AvailableBalance.Sub(value.QuoteAvailableBalance)
			//if !diff.IsZero() {
			//	comparison := "more"
			//	if diff.IsNegative() {
			//		comparison = "less"
			//	}
			//	fmt.Println("🦀 subaccount", key.String()[len(key.String())-2:], "has", getReadableDec(diff), comparison, "available balance than expected from", getReadableDec(depositQuoteAsset.AvailableBalance), "-", getReadableDec(value.QuoteAvailableBalance))
			//}
			// diff = depositQuoteAsset.TotalBalance.Sub(value.QuoteTotalBalance)
			//if !diff.IsZero() {
			//	comparison := "more"
			//	if diff.IsNegative() {
			//		comparison = "less"
			//	}
			//	fmt.Println("🦀 subaccount", key.String()[len(key.String())-2:], "has", getReadableDec(diff), comparison, "total balance than expected from", getReadableDec(depositQuoteAsset.TotalBalance), "-", getReadableDec(value.QuoteTotalBalance))
			//}
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

			expectedMargin := GetRequiredBinaryOptionsMargin(positionsForSpecificMarket[i].Position, testInput.BinaryMarkets[marketIndex].OracleScaleFactor)
			Expect(positionsForSpecificMarket[i].Position.Margin.String()).To(Equal(expectedMargin.String()))
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
