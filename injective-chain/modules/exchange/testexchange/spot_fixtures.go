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

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"

	//lint:ignore ST1001 allow dot import for convenience
	. "github.com/onsi/gomega"
)

var resultingSpotDeposits []map[common.Hash]spotDeposit

type spotLimitTestOrder struct {
	Price                                 int  `json:"price"`
	Quantity                              int  `json:"quantity"`
	SubAccountNonce                       int  `json:"subaccountNonce"` // last one digit of subaccount
	ExpectedQuantityFilledViaMatching     int  `json:"expectedQuantityFilledViaMatching"`
	ExpectedQuantityFilledViaMarketOrders int  `json:"expectedQuantityFilledViaMarketOrders"`
	IsPostOnly                            bool `json:"isPostOnly"`
}

type expectedSpotLimitTestOrder struct {
	Price    int `json:"price"`
	Quantity int `json:"quantity"`
}

type spotMarketTestOrder struct {
	Quantity        int `json:"quantity"`
	SubAccountNonce int `json:"subaccountNonce"` // last one digit of subaccount
	QuantityFilled  int `json:"expectedQuantityFilled"`
	WorstPrice      int `json:"worstPrice"`
}

type testOrderbook struct {
	SellOrders                       []spotLimitTestOrder         `json:"resting-sells"`
	TransientSellOrders              []spotLimitTestOrder         `json:"transient-sells"`
	MarketSellOrders                 []spotMarketTestOrder        `json:"market-sells"`
	BuyOrders                        []spotLimitTestOrder         `json:"resting-buys"`
	TransientBuyOrders               []spotLimitTestOrder         `json:"transient-buys"`
	MarketBuyOrders                  []spotMarketTestOrder        `json:"market-buys"`
	ExpectedBuyOrdersAfterMatching   []expectedSpotLimitTestOrder `json:"expected-buys-after-matching"`
	ExpectedSellOrdersAfterMatching  []expectedSpotLimitTestOrder `json:"expected-sells-after-matching"`
	ExpectedClearingPriceMatching    float32                      `json:"expected-clearing-price-matching"`
	ExpectedClearingPriceMarketSells float32                      `json:"expected-clearing-price-market-sells"`
	ExpectedClearingPriceMarketBuys  float32                      `json:"expected-clearing-price-market-buys"`
}

type spotDeposit struct {
	BaseAvailableBalance  sdk.Dec
	BaseTotalBalance      sdk.Dec
	QuoteAvailableBalance sdk.Dec
	QuoteTotalBalance     sdk.Dec
}

type tradePrices struct {
	limitMatches        []sdk.Dec
	limitFillsFromBuys  []sdk.Dec
	limitFillsFromSells []sdk.Dec
	marketBuys          []sdk.Dec
	marketSells         []sdk.Dec
}

func (testInput *TestInput) calculateExpectedDepositsAfterMatching(app *simapp.InjectiveApp, ctx sdk.Context, orders []spotLimitTestOrder, isBuy bool, isResting bool, marketIndex int, clearingPrice sdk.Dec, isDiscountedFeeRate bool) {
	for i := 0; i < len(orders); i++ {
		price := sdk.NewDec(int64(orders[i].Price))
		quantity := sdk.NewDec(int64(orders[i].Quantity))
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))
		expectedQuantityFilledViaMatching := sdk.NewDec(int64(orders[i].ExpectedQuantityFilledViaMatching))
		expectedQuantityFilledViaMarketOrders := sdk.NewDec(int64(orders[i].ExpectedQuantityFilledViaMarketOrders))

		spotMarket := app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[marketIndex].MarketID, true)
		feeRateForFilled := spotMarket.TakerFeeRate

		if isResting {
			feeRateForFilled = spotMarket.MakerFeeRate
		}

		usedMakerFeeRate := spotMarket.MakerFeeRate

		if feeRateForFilled.IsPositive() && isDiscountedFeeRate {
			feeRateForFilled = feeRateForFilled.Quo(sdk.NewDec(10)).Mul(sdk.NewDec(9))
			usedMakerFeeRate = spotMarket.MakerFeeRate.Quo(sdk.NewDec(10)).Mul(sdk.NewDec(9))
		}

		if isBuy {
			availableQuoteBalanceSpentForMarketOrders := expectedQuantityFilledViaMarketOrders.Mul(price).Mul(sdk.NewDec(1).Add(usedMakerFeeRate))
			totalQuoteBalanceSpentForMarketOrders := availableQuoteBalanceSpentForMarketOrders

			if feeRateForFilled.IsNegative() {
				feeRateForFilled = feeRateForFilled.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate))
				availableQuoteBalanceSpentForMarketOrders = expectedQuantityFilledViaMarketOrders.Mul(price).Mul(sdk.NewDec(1).Add(feeRateForFilled))
				totalQuoteBalanceSpentForMarketOrders = expectedQuantityFilledViaMarketOrders.Mul(price).Mul(sdk.NewDec(1).Add(feeRateForFilled))
			}

			availableQuoteBalanceSpentForRemaining := (quantity.Sub(expectedQuantityFilledViaMatching).Sub(expectedQuantityFilledViaMarketOrders)).Mul(
				price)

			if spotMarket.MakerFeeRate.IsPositive() {
				availableQuoteBalanceSpentForRemaining = availableQuoteBalanceSpentForRemaining.Mul(sdk.NewDec(1).Add(spotMarket.MakerFeeRate))
			}

			availableQuoteBalanceSpentForMatching := expectedQuantityFilledViaMatching.Mul(clearingPrice).Mul(
				sdk.NewDec(1).Add(feeRateForFilled))
			availableQuoteBalanceSpent := availableQuoteBalanceSpentForMatching.Add(availableQuoteBalanceSpentForRemaining).Add(
				availableQuoteBalanceSpentForMarketOrders)

			totalQuoteBalanceSpent := availableQuoteBalanceSpentForMatching.Add(totalQuoteBalanceSpentForMarketOrders)

			availableBaseBalanceEarned := expectedQuantityFilledViaMatching.Add(expectedQuantityFilledViaMarketOrders)
			totalBaseBalanceEarned := availableBaseBalanceEarned

			startingDeposits := spotDeposit{
				BaseAvailableBalance:  startingDeposit,
				BaseTotalBalance:      startingDeposit,
				QuoteAvailableBalance: startingDeposit,
				QuoteTotalBalance:     startingDeposit,
			}

			if resultingSpotDeposits[marketIndex][trader] != (spotDeposit{}) {
				startingDeposits = resultingSpotDeposits[marketIndex][trader]
			}

			resultingSpotDeposits[marketIndex][trader] = spotDeposit{
				BaseAvailableBalance:  startingDeposits.BaseAvailableBalance.Add(availableBaseBalanceEarned),
				BaseTotalBalance:      startingDeposits.BaseTotalBalance.Add(totalBaseBalanceEarned),
				QuoteAvailableBalance: startingDeposits.QuoteAvailableBalance.Sub(availableQuoteBalanceSpent),
				QuoteTotalBalance:     startingDeposits.QuoteTotalBalance.Sub(totalQuoteBalanceSpent),
			}
		} else {
			var availableQuoteBalanceEarnedFromMarketOrders sdk.Dec

			if feeRateForFilled.IsNegative() {
				feeRateForFilled = feeRateForFilled.Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate))
				availableQuoteBalanceEarnedFromMarketOrders = expectedQuantityFilledViaMarketOrders.Mul(price).Mul(sdk.NewDec(1).Sub(feeRateForFilled))
			} else {
				availableQuoteBalanceEarnedFromMarketOrders = expectedQuantityFilledViaMarketOrders.Mul(price).Mul(sdk.NewDec(1).Sub(usedMakerFeeRate))
			}

			availableQuoteBalanceEarnedFromMatching := expectedQuantityFilledViaMatching.Mul(clearingPrice).Mul(sdk.NewDec(1).Sub(feeRateForFilled))
			availableQuoteBalanceEarned := availableQuoteBalanceEarnedFromMatching.Add(availableQuoteBalanceEarnedFromMarketOrders)
			totalQuoteBalanceEarned := availableQuoteBalanceEarned

			availableBaseBalanceSpent := quantity
			totalBaseBalanceSpent := expectedQuantityFilledViaMatching.Add(expectedQuantityFilledViaMarketOrders)

			startingDeposits := spotDeposit{
				BaseAvailableBalance:  startingDeposit,
				BaseTotalBalance:      startingDeposit,
				QuoteAvailableBalance: startingDeposit,
				QuoteTotalBalance:     startingDeposit,
			}

			if resultingSpotDeposits[marketIndex][trader] != (spotDeposit{}) {
				startingDeposits = resultingSpotDeposits[marketIndex][trader]
			}

			resultingSpotDeposits[marketIndex][trader] = spotDeposit{
				BaseAvailableBalance:  startingDeposits.BaseAvailableBalance.Sub(availableBaseBalanceSpent),
				BaseTotalBalance:      startingDeposits.BaseTotalBalance.Sub(totalBaseBalanceSpent),
				QuoteAvailableBalance: startingDeposits.QuoteAvailableBalance.Add(availableQuoteBalanceEarned),
				QuoteTotalBalance:     startingDeposits.QuoteTotalBalance.Add(totalQuoteBalanceEarned),
			}
		}
	}
}

func (testInput *TestInput) calculateExpectedDepositsAfterMarketFilling(app *simapp.InjectiveApp, ctx sdk.Context, orders []spotMarketTestOrder, isBuy bool, marketIndex int, clearingPrice sdk.Dec, isDiscountedFeeRate bool) {
	for i := 0; i < len(orders); i++ {
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))
		quantityFilled := sdk.NewDec(int64(orders[i].QuantityFilled))

		spotMarket := app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[marketIndex].MarketID, true)
		feeForFilled := spotMarket.TakerFeeRate

		if feeForFilled.IsPositive() && isDiscountedFeeRate {
			feeForFilled = feeForFilled.Quo(sdk.NewDec(10)).Mul(sdk.NewDec(9))
		}

		if isBuy {
			availableQuoteBalanceSpent := quantityFilled.Mul(clearingPrice).Mul(sdk.NewDec(1).Add(feeForFilled))
			totalQuoteBalanceSpent := availableQuoteBalanceSpent

			availableBaseBalanceEarned := quantityFilled
			totalBaseBalanceEarned := quantityFilled

			startingDeposits := spotDeposit{
				BaseAvailableBalance:  startingDeposit,
				BaseTotalBalance:      startingDeposit,
				QuoteAvailableBalance: startingDeposit,
				QuoteTotalBalance:     startingDeposit,
			}

			if resultingSpotDeposits[marketIndex][trader] != (spotDeposit{}) {
				startingDeposits = resultingSpotDeposits[marketIndex][trader]
			}

			resultingSpotDeposits[marketIndex][trader] = spotDeposit{
				BaseAvailableBalance:  startingDeposits.BaseAvailableBalance.Add(availableBaseBalanceEarned),
				BaseTotalBalance:      startingDeposits.BaseTotalBalance.Add(totalBaseBalanceEarned),
				QuoteAvailableBalance: startingDeposits.QuoteAvailableBalance.Sub(availableQuoteBalanceSpent),
				QuoteTotalBalance:     startingDeposits.QuoteTotalBalance.Sub(totalQuoteBalanceSpent),
			}
		} else {
			availableQuoteBalanceEarned := quantityFilled.Mul(clearingPrice).Mul(sdk.NewDec(1).Sub(feeForFilled))
			totalQuoteBalanceEarned := availableQuoteBalanceEarned

			availableBaseBalanceSpent := quantityFilled
			totalBaseBalanceSpent := quantityFilled

			startingDeposits := spotDeposit{
				BaseAvailableBalance:  startingDeposit,
				BaseTotalBalance:      startingDeposit,
				QuoteAvailableBalance: startingDeposit,
				QuoteTotalBalance:     startingDeposit,
			}

			if resultingSpotDeposits[marketIndex][trader] != (spotDeposit{}) {
				startingDeposits = resultingSpotDeposits[marketIndex][trader]
			}

			resultingSpotDeposits[marketIndex][trader] = spotDeposit{
				BaseAvailableBalance:  startingDeposits.BaseAvailableBalance.Sub(availableBaseBalanceSpent),
				BaseTotalBalance:      startingDeposits.BaseTotalBalance.Sub(totalBaseBalanceSpent),
				QuoteAvailableBalance: startingDeposits.QuoteAvailableBalance.Add(availableQuoteBalanceEarned),
				QuoteTotalBalance:     startingDeposits.QuoteTotalBalance.Add(totalQuoteBalanceEarned),
			}
		}
	}
}

func (testInput *TestInput) addSpotLimitTestOrdersForMarketIndex(ctx sdk.Context, msgServer exchangetypes.MsgServer, orders []spotLimitTestOrder, isBuy bool, marketIndex int) {
	for i := 0; i < len(orders); i++ {
		price := sdk.NewDec(int64(orders[i].Price))
		quantity := sdk.NewDec(int64(orders[i].Quantity))
		trader := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(orders[i].SubAccountNonce), 16))
		isPostOnly := orders[i].IsPostOnly

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
		limitOrderMsg := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(price, quantity, orderType, trader, marketIndex)
		_, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderMsg)

		OrFail(err)
	}
}

func (testInput *TestInput) addSpotMarketTestOrdersForMarketIndex(ctx sdk.Context, msgServer exchangetypes.MsgServer, orders []spotMarketTestOrder, isBuy bool, marketIndex int) {
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

		marketOrderMsg := testInput.NewMsgCreateSpotMarketOrderForMarketIndex(worstPrice, quantity, orderType, trader, marketIndex)
		_, err := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsg)

		OrFail(err)
	}
}

func (testInput *TestInput) LoadOrderbookFixture(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	orderbookFixturePath string,
	marketCount int,
	isDiscountedFeeRate bool,
) ([]expectedSpotLimitTestOrder, []expectedSpotLimitTestOrder, float32, float32, float32, []int, []int, tradePrices) {
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	jsonFile, err := os.Open(orderbookFixturePath)
	OrFail(err)
	defer jsonFile.Close()

	var orderbook testOrderbook
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &orderbook)
	OrFail(err)

	expectedClearingPriceMatchingString := fmt.Sprintf("%f000000000000", orderbook.ExpectedClearingPriceMatching)
	expectedClearingPriceMatching, _ := sdk.NewDecFromStr(expectedClearingPriceMatchingString)
	expectedClearingPriceMarketSellsString := fmt.Sprintf("%f000000000000", orderbook.ExpectedClearingPriceMarketSells)
	expectedClearingPriceMarketSells, _ := sdk.NewDecFromStr(expectedClearingPriceMarketSellsString)
	expectedClearingPriceMarketBuysString := fmt.Sprintf("%f000000000000", orderbook.ExpectedClearingPriceMarketBuys)
	expectedClearingPriceMarketBuys, _ := sdk.NewDecFromStr(expectedClearingPriceMarketBuysString)

	for i := 0; i < marketCount; i++ {
		testInput.addSpotLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.BuyOrders, true, i)
		testInput.addSpotLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.SellOrders, false, i)

		ctx, _ = EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		testInput.calculateExpectedDepositsAfterMatching(app, ctx, orderbook.BuyOrders, true, true, i, expectedClearingPriceMatching, isDiscountedFeeRate)
		testInput.calculateExpectedDepositsAfterMatching(app, ctx, orderbook.SellOrders, false, true, i, expectedClearingPriceMatching, isDiscountedFeeRate)
	}

	var emittedABCIEvents []abci.Event

	for i := 0; i < marketCount; i++ {
		testInput.addSpotLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.TransientBuyOrders, true, i)
		testInput.addSpotLimitTestOrdersForMarketIndex(ctx, msgServer, orderbook.TransientSellOrders, false, i)
		testInput.addSpotMarketTestOrdersForMarketIndex(ctx, msgServer, orderbook.MarketBuyOrders, true, i)
		testInput.addSpotMarketTestOrdersForMarketIndex(ctx, msgServer, orderbook.MarketSellOrders, false, i)

		timestamp := ctx.BlockTime().Unix() + (35000 * (int64(i) + 1))
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))

		ctx, emittedABCIEvents = EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		testInput.calculateExpectedDepositsAfterMatching(app, ctx, orderbook.TransientBuyOrders, true, false, i, expectedClearingPriceMatching, isDiscountedFeeRate)
		testInput.calculateExpectedDepositsAfterMatching(app, ctx, orderbook.TransientSellOrders, false, false, i, expectedClearingPriceMatching, isDiscountedFeeRate)
		testInput.calculateExpectedDepositsAfterMarketFilling(app, ctx, orderbook.MarketBuyOrders, true, i, expectedClearingPriceMarketBuys, isDiscountedFeeRate)
		testInput.calculateExpectedDepositsAfterMarketFilling(app, ctx, orderbook.MarketSellOrders, false, i, expectedClearingPriceMarketSells, isDiscountedFeeRate)

		timestamp = ctx.BlockTime().Unix() + (50000 * (int64(i) + 1))
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))

		EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}

	limitEventBatches := make([]*exchangetypes.TradeLog, 0)
	limitFillFromBuysEventBatches := make([]*exchangetypes.TradeLog, 0)
	limitFillFromSellsEventBatches := make([]*exchangetypes.TradeLog, 0)
	marketBuyEventBatches := make([]*exchangetypes.TradeLog, 0)
	marketSellEventBatches := make([]*exchangetypes.TradeLog, 0)

	emittedTradePrices := tradePrices{
		limitMatches:        make([]sdk.Dec, 0),
		limitFillsFromBuys:  make([]sdk.Dec, 0),
		limitFillsFromSells: make([]sdk.Dec, 0),
		marketBuys:          make([]sdk.Dec, 0),
		marketSells:         make([]sdk.Dec, 0),
	}
	expectedLimitFillFromBuysPrices := make([]int, 0)
	expectedLimitFillFromSellsPrices := make([]int, 0)

	for i := 0; i < marketCount; i++ {
		for _, emittedABCIEvent := range emittedABCIEvents {
			switch emittedABCIEvent.Type {
			case banktypes.EventTypeCoinReceived, banktypes.EventTypeCoinMint, banktypes.EventTypeCoinSpent, banktypes.EventTypeTransfer, sdk.EventTypeMessage:
				continue
			}

			parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
			OrFail(err1)

			switch event := parsedEvent.(type) {
			case *exchangetypes.EventBatchSpotExecution:
				if event.ExecutionType == exchangetypes.ExecutionType_LimitFill && event.IsBuy {
					limitFillFromBuysEventBatches = append(limitFillFromBuysEventBatches, event.Trades...)
					break
				} else if event.ExecutionType == exchangetypes.ExecutionType_LimitFill {
					limitFillFromSellsEventBatches = append(limitFillFromSellsEventBatches, event.Trades...)
					break
				}

				if event.ExecutionType == exchangetypes.ExecutionType_LimitMatchNewOrder || event.ExecutionType == exchangetypes.ExecutionType_LimitMatchRestingOrder {
					limitEventBatches = append(limitEventBatches, event.Trades...)
					break
				}

				if event.ExecutionType == exchangetypes.ExecutionType_Market && event.IsBuy {
					marketBuyEventBatches = append(marketBuyEventBatches, event.Trades...)
				} else if event.ExecutionType == exchangetypes.ExecutionType_Market {
					marketSellEventBatches = append(marketSellEventBatches, event.Trades...)
				}
			}
		}

		for _, trade := range limitEventBatches {
			emittedTradePrices.limitMatches = append(emittedTradePrices.limitMatches, trade.Price)
		}
		for _, trade := range limitFillFromBuysEventBatches {
			emittedTradePrices.limitFillsFromBuys = append(emittedTradePrices.limitFillsFromBuys, trade.Price)
		}
		for _, trade := range limitFillFromSellsEventBatches {
			emittedTradePrices.limitFillsFromSells = append(emittedTradePrices.limitFillsFromSells, trade.Price)
		}
		for _, trade := range marketBuyEventBatches {
			emittedTradePrices.marketBuys = append(emittedTradePrices.marketBuys, trade.Price)
		}
		for _, trade := range marketSellEventBatches {
			emittedTradePrices.marketSells = append(emittedTradePrices.marketSells, trade.Price)
		}
		for _, buyOrder := range orderbook.BuyOrders {
			if buyOrder.ExpectedQuantityFilledViaMarketOrders > 0 {
				expectedLimitFillFromBuysPrices = append(expectedLimitFillFromBuysPrices, buyOrder.Price)
			}
		}
		for _, sellOrder := range orderbook.SellOrders {
			if sellOrder.ExpectedQuantityFilledViaMarketOrders > 0 {
				expectedLimitFillFromSellsPrices = append(expectedLimitFillFromSellsPrices, sellOrder.Price)
			}
		}
	}

	return orderbook.ExpectedBuyOrdersAfterMatching,
		orderbook.ExpectedSellOrdersAfterMatching,
		orderbook.ExpectedClearingPriceMatching,
		orderbook.ExpectedClearingPriceMarketBuys,
		orderbook.ExpectedClearingPriceMarketSells,
		expectedLimitFillFromBuysPrices,
		expectedLimitFillFromSellsPrices,
		emittedTradePrices
}

func ExpectCorrectSpotOrderbookMatching(testInput *TestInput, app *simapp.InjectiveApp, ctx *sdk.Context, orderbookFixturePath string, marketCount int, isDiscountedFeeRate bool) {
	resultingSpotDeposits = make([]map[common.Hash]spotDeposit, marketCount)
	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		resultingSpotDeposits[marketIndex] = make(map[common.Hash]spotDeposit)
	}
	testInput.AddSpotDeposits(app, *ctx, marketCount)

	expectedBuyOrdersAfterMatching, expectedSellOrdersAfterMatching, expectedClearingPriceMatching, expectedClearingPriceMarketBuys,
		expectedClearingPriceMarketSells, expectedLimitFillFromBuysPrices, expectedLimitFillFromSellsPrices, emittedTradePrices := testInput.LoadOrderbookFixture(app, *ctx, orderbookFixturePath, marketCount, isDiscountedFeeRate)

	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		limitBuyOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(*ctx, testInput.Spots[marketIndex].MarketID, true)
		limitSellOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(*ctx, testInput.Spots[marketIndex].MarketID, false)

		for i := 0; i < len(expectedBuyOrdersAfterMatching); i++ {
			Expect(sdk.NewDec(int64(expectedBuyOrdersAfterMatching[i].Price)).String()).Should(Equal(limitBuyOrders[i].OrderInfo.Price.String()))
			Expect(sdk.NewDec(int64(expectedBuyOrdersAfterMatching[i].Quantity)).String()).Should(Equal(limitBuyOrders[i].Fillable.String()))
		}
		for i := 0; i < len(expectedSellOrdersAfterMatching); i++ {
			Expect(sdk.NewDec(int64(expectedSellOrdersAfterMatching[i].Price)).String()).Should(Equal(limitSellOrders[i].OrderInfo.Price.String()))
			Expect(sdk.NewDec(int64(expectedSellOrdersAfterMatching[i].Quantity)).String()).Should(Equal(limitSellOrders[i].Fillable.String()))
		}

		if expectedClearingPriceMatching > 0 {
			Expect(emittedTradePrices.limitMatches).Should(Not(BeEmpty()))
		}
		if expectedClearingPriceMarketBuys > 0 {
			Expect(emittedTradePrices.marketBuys).Should(Not(BeEmpty()))
		}
		if expectedClearingPriceMarketSells > 0 {
			Expect(emittedTradePrices.marketSells).Should(Not(BeEmpty()))
		}

		expectedClearingPriceMatchingString := fmt.Sprintf("%f000000000000", expectedClearingPriceMatching)
		expectedClearingPriceMarketBuysString := fmt.Sprintf("%f000000000000", expectedClearingPriceMarketBuys)
		expectedClearingPriceMarketSellsString := fmt.Sprintf("%f000000000000", expectedClearingPriceMarketSells)

		for i := range emittedTradePrices.limitMatches {
			Expect(emittedTradePrices.limitMatches[i].String()).Should(Equal(expectedClearingPriceMatchingString))
		}
		for i := range emittedTradePrices.limitFillsFromBuys {
			if i < len(expectedLimitFillFromBuysPrices) {
				expectedLimitFillString := fmt.Sprintf("%f000000000000", float32(expectedLimitFillFromBuysPrices[i]))
				Expect(emittedTradePrices.limitFillsFromBuys[i].String()).Should(Equal(expectedLimitFillString))
			}
		}
		for i := range emittedTradePrices.limitFillsFromSells {
			if i < len(expectedLimitFillFromBuysPrices) {
				expectedLimitFillString := fmt.Sprintf("%f000000000000", float32(expectedLimitFillFromSellsPrices[i]))
				Expect(emittedTradePrices.limitFillsFromSells[i].String()).Should(Equal(expectedLimitFillString))
			}
		}
		for i := range emittedTradePrices.marketBuys {
			Expect(emittedTradePrices.marketBuys[i].String()).Should(Equal(expectedClearingPriceMarketBuysString))
		}
		for i := range emittedTradePrices.marketSells {
			Expect(emittedTradePrices.marketSells[i].String()).Should(Equal(expectedClearingPriceMarketSellsString))
		}

		for key, value := range resultingSpotDeposits[marketIndex] {
			depositBaseAsset := app.ExchangeKeeper.GetDeposit(*ctx, key, testInput.Spots[marketIndex].BaseDenom)
			depositQuoteAsset := app.ExchangeKeeper.GetDeposit(*ctx, key, testInput.Spots[marketIndex].QuoteDenom)

			Expect(depositBaseAsset.AvailableBalance.String()).Should(Equal(value.BaseAvailableBalance.String()))
			Expect(depositBaseAsset.TotalBalance.String()).Should(Equal(value.BaseTotalBalance.String()))
			Expect(depositQuoteAsset.AvailableBalance.String()).Should(Equal(value.QuoteAvailableBalance.String()))
			Expect(depositQuoteAsset.TotalBalance.String()).Should(Equal(value.QuoteTotalBalance.String()))
		}
	}

	hadMatchedOrdersWithMaker := expectedClearingPriceMarketBuys > 0 || expectedClearingPriceMarketSells > 0

	if hadMatchedOrdersWithMaker {
		sender, err := sdk.AccAddressFromBech32(DefaultAddress)
		OrFail(err)

		hasReceivedINJ := false
		receivedTokens := app.BankKeeper.GetAllBalances(*ctx, sender)

		for _, receivedToken := range receivedTokens {
			if receivedToken.Denom == "inj" {
				hasReceivedINJ = true
				expectedRewardTokens := sdk.NewInt(100000)
				Expect(receivedToken.Amount.String()).Should(Equal(expectedRewardTokens.String()))
			}
		}

		Expect(hasReceivedINJ).Should(BeTrue())

		Expect((*app).ExchangeKeeper.IsMetadataInvariantValid(*ctx)).To(BeTrue())
		InvariantCheckBalanceAndSupply(app, *ctx, testInput)
		InvariantCheckInsuranceModuleBalance(app, *ctx)
		InvariantCheckAccountFees(app, *ctx)
	}
}
