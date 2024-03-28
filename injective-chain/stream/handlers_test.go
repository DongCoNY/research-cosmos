package stream

import (
	"cosmossdk.io/math"
	"encoding/base64"
	"encoding/json"
	"fmt"
	exchangeTypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHandleBatchSpotExecutionEvent(t *testing.T) {
	marketId := "0x0611780ba69656949525013d947713300f56c37b6175e02f26bffa495c3208fe"
	responseMap := types.NewStreamResponseMap()
	responseMap.BlockHeight = 4321

	tradeLogs := make([]*exchangeTypes.TradeLog, 0)

	trade1 := exchangeTypes.TradeLog{
		Quantity:            math.LegacyNewDec(1),
		Price:               math.LegacyNewDec(200),
		SubaccountId:        []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           []byte("0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989"),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_1",
	}

	trade2 := exchangeTypes.TradeLog{
		Quantity:            math.LegacyNewDec(2),
		Price:               math.LegacyNewDec(210),
		SubaccountId:        []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           []byte("0x6cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989"),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_2",
	}

	tradeLogs = append(tradeLogs, &trade1)
	tradeLogs = append(tradeLogs, &trade2)

	marshalledTrades, _ := json.Marshal(tradeLogs)

	abciEvent := abci.Event{
		Type: "injective.exchange.v1beta1.EventBatchSpotExecution",
		Attributes: []abci.EventAttribute{
			{
				Key:   "market_id",
				Value: fmt.Sprintf("\"%s\"", marketId),
				Index: true,
			},
			{
				Key:   "is_buy",
				Value: "true",
				Index: true,
			},
			{
				Key:   "executionType",
				Value: "\"LimitFill\"",
				Index: true,
			},
			{
				Key:   "trades",
				Value: string(marshalledTrades),
				Index: true,
			},
		},
	}

	handleError := handleBatchSpotExecutionEvent(responseMap, abciEvent)

	require.NoError(t, handleError)
	spotTrades := responseMap.SpotTradesByMarketID[marketId]
	require.Len(t, spotTrades, 2)

	for index := 0; index < len(spotTrades); index++ {
		tradeLog := tradeLogs[index]
		spotTrade := spotTrades[index]
		expectedTradeID := fmt.Sprintf("%d_%d", responseMap.BlockHeight, index)

		require.Equal(t, marketId, spotTrade.MarketId)
		require.True(t, spotTrade.IsBuy)
		require.Equal(t, "LimitFill", spotTrade.ExecutionType)
		require.Equal(t, tradeLog.Quantity, spotTrade.Quantity)
		require.Equal(t, tradeLog.Price, spotTrade.Price)
		require.Equal(t, common.BytesToHash(tradeLog.SubaccountId).String(), spotTrade.SubaccountId)
		require.Equal(t, tradeLog.Fee, spotTrade.Fee)
		require.Equal(t, tradeLog.OrderHash, spotTrade.OrderHash)
		require.Equal(t, sdk.AccAddress(tradeLog.FeeRecipientAddress).String(), spotTrade.FeeRecipientAddress)
		require.Equal(t, tradeLog.Cid, spotTrade.Cid)
		require.Equal(t, expectedTradeID, spotTrade.TradeId)
	}
}

func TestHandleBatchDerivativeExecutionEvent(t *testing.T) {
	marketId := "0x17ef48032cb24375ba7c2e39f384e56433bcab20cbee9a7357e4cba2eb00abe6"
	responseMap := types.NewStreamResponseMap()
	responseMap.BlockHeight = 9876

	tradeLogs := make([]*exchangeTypes.DerivativeTradeLog, 0)

	trade1 := exchangeTypes.DerivativeTradeLog{
		SubaccountId: []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"),
		PositionDelta: &exchangeTypes.PositionDelta{
			IsLong:            true,
			ExecutionQuantity: math.LegacyNewDec(1),
			ExecutionMargin:   math.LegacyNewDec(200),
			ExecutionPrice:    math.LegacyNewDec(200),
		},
		Payout:              math.LegacyNewDec(1),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           common.HexToHash("0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989").Bytes(),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_1",
	}

	trade2 := exchangeTypes.DerivativeTradeLog{
		SubaccountId: []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"),
		PositionDelta: &exchangeTypes.PositionDelta{
			IsLong:            true,
			ExecutionQuantity: math.LegacyNewDec(2),
			ExecutionMargin:   math.LegacyNewDec(210),
			ExecutionPrice:    math.LegacyNewDec(210),
		},
		Payout:              math.LegacyNewDec(1),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           common.HexToHash("0x6cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989").Bytes(),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_2",
	}

	tradeLogs = append(tradeLogs, &trade1)
	tradeLogs = append(tradeLogs, &trade2)

	marshalledTrades, _ := json.Marshal(tradeLogs)

	abciEvent := abci.Event{
		Type: "injective.exchange.v1beta1.EventBatchDerivativeExecution",
		Attributes: []abci.EventAttribute{
			{
				Key:   "market_id",
				Value: fmt.Sprintf("\"%s\"", marketId),
				Index: true,
			},
			{
				Key:   "is_buy",
				Value: "true",
				Index: true,
			},
			{
				Key:   "is_liquidation",
				Value: "true",
				Index: true,
			},
			{
				Key:   "cumulative_funding",
				Value: "0",
				Index: true,
			},
			{
				Key:   "executionType",
				Value: "\"LimitFill\"",
				Index: true,
			},
			{
				Key:   "trades",
				Value: string(marshalledTrades),
				Index: true,
			},
		},
	}

	handleError := handleBatchDerivativeExecutionEvent(responseMap, abciEvent)

	require.NoError(t, handleError)
	derivativeTrades := responseMap.DerivativeTradesByMarketID[marketId]
	require.Len(t, derivativeTrades, 2)

	for index := 0; index < len(derivativeTrades); index++ {
		tradeLog := tradeLogs[index]
		derivativeTrade := derivativeTrades[index]
		expectedTradeID := fmt.Sprintf("%d_%d", responseMap.BlockHeight, index)

		require.Equal(t, marketId, derivativeTrade.MarketId)
		require.True(t, derivativeTrade.IsBuy)
		require.Equal(t, "LimitFill", derivativeTrade.ExecutionType)
		require.Equal(t, tradeLog.Payout, derivativeTrade.Payout)
		require.Equal(t, tradeLog.PositionDelta, derivativeTrade.PositionDelta)
		require.Equal(t, common.BytesToHash(tradeLog.SubaccountId).String(), derivativeTrade.SubaccountId)
		require.Equal(t, tradeLog.Fee, derivativeTrade.Fee)
		require.Equal(t, base64.StdEncoding.EncodeToString(tradeLog.OrderHash), derivativeTrade.OrderHash)
		require.Equal(t, sdk.AccAddress(tradeLog.FeeRecipientAddress).String(), derivativeTrade.FeeRecipientAddress)
		require.Equal(t, tradeLog.Cid, derivativeTrade.Cid)
		require.Equal(t, expectedTradeID, derivativeTrade.TradeId)
	}
}

func TestHandleOneBatchSpotExecutionEventAndOneBatchDerivativeExecutionEvent(t *testing.T) {
	spotMarketId := "0x0611780ba69656949525013d947713300f56c37b6175e02f26bffa495c3208fe"
	derivativeMarketId := "0x17ef48032cb24375ba7c2e39f384e56433bcab20cbee9a7357e4cba2eb00abe6"
	responseMap := types.NewStreamResponseMap()
	responseMap.BlockHeight = 4321

	spotTradeLogs := make([]*exchangeTypes.TradeLog, 0)

	spotTrade1 := exchangeTypes.TradeLog{
		Quantity:            math.LegacyNewDec(1),
		Price:               math.LegacyNewDec(200),
		SubaccountId:        []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           []byte("0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989"),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_1",
	}

	spotTrade2 := exchangeTypes.TradeLog{
		Quantity:            math.LegacyNewDec(2),
		Price:               math.LegacyNewDec(210),
		SubaccountId:        []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           []byte("0x6cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989"),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_2",
	}

	spotTradeLogs = append(spotTradeLogs, &spotTrade1)
	spotTradeLogs = append(spotTradeLogs, &spotTrade2)

	marshalledSpotTrades, _ := json.Marshal(spotTradeLogs)

	spotEvent := abci.Event{
		Type: "injective.exchange.v1beta1.EventBatchSpotExecution",
		Attributes: []abci.EventAttribute{
			{
				Key:   "market_id",
				Value: fmt.Sprintf("\"%s\"", spotMarketId),
				Index: true,
			},
			{
				Key:   "is_buy",
				Value: "true",
				Index: true,
			},
			{
				Key:   "executionType",
				Value: "\"LimitFill\"",
				Index: true,
			},
			{
				Key:   "trades",
				Value: string(marshalledSpotTrades),
				Index: true,
			},
		},
	}

	handleError := handleBatchSpotExecutionEvent(responseMap, spotEvent)

	require.NoError(t, handleError)
	spotTrades := responseMap.SpotTradesByMarketID[spotMarketId]
	require.Len(t, spotTrades, 2)

	for index := 0; index < len(spotTrades); index++ {
		tradeLog := spotTradeLogs[index]
		spotTrade := spotTrades[index]
		expectedTradeID := fmt.Sprintf("%d_%d", responseMap.BlockHeight, index)

		require.Equal(t, spotMarketId, spotTrade.MarketId)
		require.True(t, spotTrade.IsBuy)
		require.Equal(t, "LimitFill", spotTrade.ExecutionType)
		require.Equal(t, tradeLog.Quantity, spotTrade.Quantity)
		require.Equal(t, tradeLog.Price, spotTrade.Price)
		require.Equal(t, common.BytesToHash(tradeLog.SubaccountId).String(), spotTrade.SubaccountId)
		require.Equal(t, tradeLog.Fee, spotTrade.Fee)
		require.Equal(t, tradeLog.OrderHash, spotTrade.OrderHash)
		require.Equal(t, sdk.AccAddress(tradeLog.FeeRecipientAddress).String(), spotTrade.FeeRecipientAddress)
		require.Equal(t, tradeLog.Cid, spotTrade.Cid)
		require.Equal(t, expectedTradeID, spotTrade.TradeId)
	}

	// Process derivative trades

	derivativeTradeLogs := make([]*exchangeTypes.DerivativeTradeLog, 0)

	derivativeTrade1 := exchangeTypes.DerivativeTradeLog{
		SubaccountId: []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"),
		PositionDelta: &exchangeTypes.PositionDelta{
			IsLong:            true,
			ExecutionQuantity: math.LegacyNewDec(1),
			ExecutionMargin:   math.LegacyNewDec(200),
			ExecutionPrice:    math.LegacyNewDec(200),
		},
		Payout:              math.LegacyNewDec(1),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           common.HexToHash("0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989").Bytes(),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_1",
	}

	derivativeTrade2 := exchangeTypes.DerivativeTradeLog{
		SubaccountId: []byte("eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"),
		PositionDelta: &exchangeTypes.PositionDelta{
			IsLong:            true,
			ExecutionQuantity: math.LegacyNewDec(2),
			ExecutionMargin:   math.LegacyNewDec(210),
			ExecutionPrice:    math.LegacyNewDec(210),
		},
		Payout:              math.LegacyNewDec(1),
		Fee:                 math.LegacyNewDecWithPrec(1, 2),
		OrderHash:           common.HexToHash("0x6cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989").Bytes(),
		FeeRecipientAddress: []byte("inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"),
		Cid:                 "cid_order_2",
	}

	derivativeTradeLogs = append(derivativeTradeLogs, &derivativeTrade1)
	derivativeTradeLogs = append(derivativeTradeLogs, &derivativeTrade2)

	marshalledTrades, _ := json.Marshal(derivativeTradeLogs)

	derivativeEvent := abci.Event{
		Type: "injective.exchange.v1beta1.EventBatchDerivativeExecution",
		Attributes: []abci.EventAttribute{
			{
				Key:   "market_id",
				Value: fmt.Sprintf("\"%s\"", derivativeMarketId),
				Index: true,
			},
			{
				Key:   "is_buy",
				Value: "true",
				Index: true,
			},
			{
				Key:   "is_liquidation",
				Value: "true",
				Index: true,
			},
			{
				Key:   "cumulative_funding",
				Value: "0",
				Index: true,
			},
			{
				Key:   "executionType",
				Value: "\"LimitFill\"",
				Index: true,
			},
			{
				Key:   "trades",
				Value: string(marshalledTrades),
				Index: true,
			},
		},
	}

	handleError = handleBatchDerivativeExecutionEvent(responseMap, derivativeEvent)

	require.NoError(t, handleError)
	derivativeTrades := responseMap.DerivativeTradesByMarketID[derivativeMarketId]
	require.Len(t, derivativeTrades, 2)

	for index := 0; index < len(derivativeTrades); index++ {
		tradeLog := derivativeTradeLogs[index]
		derivativeTrade := derivativeTrades[index]
		expectedTradeID := fmt.Sprintf("%d_%d", responseMap.BlockHeight, len(spotTrades)+index)

		require.Equal(t, derivativeMarketId, derivativeTrade.MarketId)
		require.True(t, derivativeTrade.IsBuy)
		require.Equal(t, "LimitFill", derivativeTrade.ExecutionType)
		require.Equal(t, tradeLog.Payout, derivativeTrade.Payout)
		require.Equal(t, tradeLog.PositionDelta, derivativeTrade.PositionDelta)
		require.Equal(t, common.BytesToHash(tradeLog.SubaccountId).String(), derivativeTrade.SubaccountId)
		require.Equal(t, tradeLog.Fee, derivativeTrade.Fee)
		require.Equal(t, base64.StdEncoding.EncodeToString(tradeLog.OrderHash), derivativeTrade.OrderHash)
		require.Equal(t, sdk.AccAddress(tradeLog.FeeRecipientAddress).String(), derivativeTrade.FeeRecipientAddress)
		require.Equal(t, tradeLog.Cid, derivativeTrade.Cid)
		require.Equal(t, expectedTradeID, derivativeTrade.TradeId)
	}

}
