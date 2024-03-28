package stream

import (
	"encoding/base64"
	"errors"
	"fmt"
	exchangeTypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/json"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestABCIToSubaccountDeposit(t *testing.T) {

	tests := []struct {
		name        string
		inputEvent  abci.Event
		expected    []*types.SubaccountDeposits
		expectedErr error
	}{
		{
			name: "Valid event with deposit updates",
			inputEvent: abci.Event{
				Type: string(SubaccountDeposit),
				Attributes: []abci.EventAttribute{
					{Key: "deposit_updates",
						Value: `[
						{
							"denom": "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7",
							"deposits": [
								{
									"subaccount_id": "Gg0k57Eis+m9IxUgT5DzoLn+oJEAAAAAAAAAAAAAAAA=",
									"deposit": {
										"available_balance": "100.0",
										"total_balance": "200.0"
									}
								}
							]
						}
					]`},
				},
			},
			expected: []*types.SubaccountDeposits{
				{
					SubaccountId: "0x1a0d24e7b122b3e9bd2315204f90f3a0b9fea091000000000000000000000000",
					Deposits: []types.SubaccountDeposit{
						{
							Denom: "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7",
							Deposit: exchangeTypes.Deposit{
								AvailableBalance: sdk.MustNewDecFromStr("100.0"),
								TotalBalance:     sdk.MustNewDecFromStr("200.0"),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "Valid event with multi deposit updates",
			inputEvent: abci.Event{
				Type: string(SubaccountDeposit),
				Attributes: []abci.EventAttribute{
					{Key: "deposit_updates",
						Value: `[
						{
							"denom": "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7",
							"deposits": [
								{
									"subaccount_id": "Gg0k57Eis+m9IxUgT5DzoLn+oJEAAAAAAAAAAAAAAAA=",
									"deposit": {
										"available_balance": "100.0",
										"total_balance": "200.0"
									}
								},
								{
								"subaccount_id": "ERERERERERERERERERERERERERERERERERERERERERE=",
									"deposit": {
										"available_balance": "77.0",
										"total_balance": "111.0"
									}
								}

							]
						},
					{
							"denom": "inj",
							"deposits": [
								{
									"subaccount_id": "Gg0k57Eis+m9IxUgT5DzoLn+oJEAAAAAAAAAAAAAAAA=",
									"deposit": {
										"available_balance": "55.0",
										"total_balance": "10.0"
									}
								}
							]
						}
					]`},
				},
			},
			expected: []*types.SubaccountDeposits{
				{
					SubaccountId: "0x1a0d24e7b122b3e9bd2315204f90f3a0b9fea091000000000000000000000000",
					Deposits: []types.SubaccountDeposit{
						{
							Denom: "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7",
							Deposit: exchangeTypes.Deposit{
								AvailableBalance: sdk.MustNewDecFromStr("100.0"),
								TotalBalance:     sdk.MustNewDecFromStr("200.0"),
							},
						},
						{
							Denom: "inj",
							Deposit: exchangeTypes.Deposit{
								AvailableBalance: sdk.MustNewDecFromStr("55.0"),
								TotalBalance:     sdk.MustNewDecFromStr("10.0"),
							},
						},
					},
				},
				{
					SubaccountId: "0x1111111111111111111111111111111111111111111111111111111111111111",
					Deposits: []types.SubaccountDeposit{
						{
							Denom: "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7",
							Deposit: exchangeTypes.Deposit{
								AvailableBalance: sdk.MustNewDecFromStr("77.0"),
								TotalBalance:     sdk.MustNewDecFromStr("111.0"),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "Empty event",
			inputEvent: abci.Event{
				Type:       string(SubaccountDeposit),
				Attributes: []abci.EventAttribute{},
			},
			expected:    []*types.SubaccountDeposits{},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getDeposits, err := ABCIToSubaccountDeposit(test.inputEvent)
			require.Equal(t, test.expectedErr, err)
			for i, expectedDeposit := range test.expected {
				require.Equal(t, expectedDeposit.SubaccountId, getDeposits[i].SubaccountId)

				for j, expectedSubaccountDeposit := range expectedDeposit.Deposits {
					actualSubaccountDeposit := getDeposits[i].Deposits[j]
					require.Equal(t, expectedSubaccountDeposit.Denom, actualSubaccountDeposit.Denom)
					require.Equal(t, expectedSubaccountDeposit.Deposit.AvailableBalance, actualSubaccountDeposit.Deposit.AvailableBalance)
					require.Equal(t, expectedSubaccountDeposit.Deposit.TotalBalance, actualSubaccountDeposit.Deposit.TotalBalance)
				}
			}

			require.Len(t, getDeposits, len(test.expected))
		})
	}
}

func TestABCITOEventSetPythPrices(t *testing.T) {

	mockData := `{
		"type": "injective.oracle.v1beta1.EventSetPythPrices",
		"attributes": [
			{
				"key": "prices",
           		 "value": "[{\"price_id\":\"0xb962539d0fcb272a494d65ea56f94851c2bcf8823935da05bd628916e2e9edbf\",\"ema_price\":\"56.037461000000000000\",\"ema_conf\":\"0.034704570000000000\",\"conf\":\"0.039043100000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"56.010000000000000000\",\"cumulative_price\":\"305435915.315517110000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x7a5bc1d2b56ad029048cd63964b3ad2776eadf812edc1a43a31406cb54bff592\",\"ema_price\":\"9.441150800000000000\",\"ema_conf\":\"0.006795720000000000\",\"conf\":\"0.006106640000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"9.474000000000000000\",\"cumulative_price\":\"49408504.193217280000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0xc63e2a7f37a04e5e614c07238bedb25dcc38927fba8fe890597a593c0b2fa4ad\",\"ema_price\":\"2.156915370000000000\",\"ema_conf\":\"0.001798550000000000\",\"conf\":\"0.002004180000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"2.161500000000000000\",\"cumulative_price\":\"11201599.356491570000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x8ac0c70fff57e9aefdf5edf44b51d62c2d433653cbb2cf5cc06bb115af04d221\",\"ema_price\":\"8.075388800000000000\",\"ema_conf\":\"0.005992650000000000\",\"conf\":\"0.005889990000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"8.292890000000000000\",\"cumulative_price\":\"34059097.619944110000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x6e3f3fa8253588df9326580180233eb791e03b443a3ba7a1d892e73874e19a54\",\"ema_price\":\"94.394144000000000000\",\"ema_conf\":\"0.043525260000000000\",\"conf\":\"0.038319320000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"94.380000000000000000\",\"cumulative_price\":\"499563683.310592100000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x2b89b9dc8fdf9f34709a5b106b472f0f39bb6ca9ce04b0fd7f2e971688e2e53b\",\"ema_price\":\"1.000027340000000000\",\"ema_conf\":\"0.000286290000000000\",\"conf\":\"0.000356280000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"1.000010780000000000\",\"cumulative_price\":\"6186998.068531810000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x67a6f93030420c1c9e3fe37c1ab6b77966af82f995944a9fefce357a22854a80\",\"ema_price\":\"0.683280000000000000\",\"ema_conf\":\"0.000070000000000000\",\"conf\":\"0.000070000000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"0.682730000000000000\",\"cumulative_price\":\"3695667.249550000000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0xa995d00bb36a63cef7fd2c287dc105fc8f3d93779f062f09551b0af3e81ec30b\",\"ema_price\":\"1.120780000000000000\",\"ema_conf\":\"0.000070000000000000\",\"conf\":\"0.000100000000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"1.120760000000000000\",\"cumulative_price\":\"6011594.202590000000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0xf0d57deca57b3da2fe63a493f4c25925fdfd8edf834b20f93e1f84dbd1504d4a\",\"ema_price\":\"0.000007856500000000\",\"ema_conf\":\"0.000000011800000000\",\"conf\":\"0.000000012900000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"0.000007855000000000\",\"cumulative_price\":\"53.065606327600000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0xef0d8b6fda2ceba41da15d4095d1da392a0d2f8ed0c6c7bc0f4cfac8c280b56d\",\"ema_price\":\"27.233751000000000000\",\"ema_conf\":\"0.019417710000000000\",\"conf\":\"0.015679990000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"27.163179990000000000\",\"cumulative_price\":\"130590191.382414600000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x23d7315113f5b1d3ba7a83604c44b94d79f4fd69af77f804fc7f920a6dc65744\",\"ema_price\":\"0.717753160000000000\",\"ema_conf\":\"0.000526680000000000\",\"conf\":\"0.000597260000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"0.714159630000000000\",\"cumulative_price\":\"6811844.793750650000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x78d185a741d07edb3412b09008b7c5cfb9bbbd7d568bf00ba737b456ba171501\",\"ema_price\":\"6.082594400000000000\",\"ema_conf\":\"0.004275450000000000\",\"conf\":\"0.003994290000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"6.093500000000000000\",\"cumulative_price\":\"32135627.244935160000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a\",\"ema_price\":\"0.999995240000000000\",\"ema_conf\":\"0.000204220000000000\",\"conf\":\"0.000204850000000000\",\"publish_time\":\"1689854329\",\"price_state\":{\"price\":\"0.999982500000000000\",\"cumulative_price\":\"6186035.618776860000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x67aed5a24fdad045475e7195c98a98aea119c763f272d4523f5bac93a4f33c2b\",\"ema_price\":\"0.081000340000000000\",\"ema_conf\":\"0.000077220000000000\",\"conf\":\"0.000070620000000000\",\"publish_time\":\"1689854328\",\"price_state\":{\"price\":\"0.081040700000000000\",\"cumulative_price\":\"46546.557175270000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x70dddcb074263ce201ea9a1be5b3537e59ed5b9060d309e12d61762cfe59fb7e\",\"ema_price\":\"1.985903000000000000\",\"ema_conf\":\"0.001667460000000000\",\"conf\":\"0.001844430000000000\",\"publish_time\":\"1689854328\",\"price_state\":{\"price\":\"1.983639070000000000\",\"cumulative_price\":\"1151112.907161960000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0x46b8cc9347f04391764a0361e0b17c3ba394b001e7c304f7650f6376e37c321d\",\"ema_price\":\"167.779143000000000000\",\"ema_conf\":\"0.191197800000000000\",\"conf\":\"0.199844690000000000\",\"publish_time\":\"1689854328\",\"price_state\":{\"price\":\"167.979311230000000000\",\"cumulative_price\":\"95704455.581574720000000000\",\"timestamp\":\"1689854331\"}},{\"price_id\":\"0xec5d399846a9209f3fe5881d70aae9268c94339ff9817e8d18ff19fa05eea1c8\",\"ema_price\":\"0.812392410000000000\",\"ema_conf\":\"0.000407230000000000\",\"conf\":\"0.000407470000000000\",\"publish_time\":\"1689854328\",\"price_state\":{\"price\":\"0.808365000000000000\",\"cumulative_price\":\"439907.328932840000000000\",\"timestamp\":\"1689854331\"}}]"
			}
		]
		}`

	var mockEvent abci.Event
	err := json.Unmarshal([]byte(mockData), &mockEvent)
	if err != nil {
		t.Fatal(err)
	}

	pythPrices, err := ABCITOEventSetPythPrices(mockEvent)

	require.NoError(t, err)
	require.NotNil(t, pythPrices)
	require.Equal(t, 17, len(pythPrices))

	firstPrice := pythPrices[0]
	expectedPrice := types.OraclePrice{
		Symbol: "0xb962539d0fcb272a494d65ea56f94851c2bcf8823935da05bd628916e2e9edbf",
		Price:  sdk.MustNewDecFromStr("56.037461000000000000"),
		Type:   "pyth",
	}

	require.Equal(t, expectedPrice, *firstPrice)
}

func TestABCITOEventSetBandIBCPriceEvent(t *testing.T) {

	mockData := `{
  "type": "injective.oracle.v1beta1.SetBandIBCPriceEvent",
  "attributes": [
    {
      "key": "client_id",
      "value": "\"8016534\"",
      "index": true
    },
    {
      "key": "prices",
      "value": "[\"9.222000000000000000\",\"241.810000000000000000\",\"29770.860000000000000000\",\"1871.900000000000000000\",\"8.369499999000000000\",\"0.999970000000000000\",\"0.507401112000000000\",\"0.621814000000000000\",\"24.509264700000000000\"]",
      "index": true
    },
    {
      "key": "relayer",
      "value": "\"inj1xn5035tcml863n9nvw4tvu2p0qs4hp3206v28y\"",
      "index": true
    },
    {
      "key": "request_id",
      "value": "\"20062149\"",
      "index": true
    },
    {
      "key": "resolve_time",
      "value": "\"1690188177\"",
      "index": true
    },
    {
      "key": "symbols",
      "value": "[\"ATOM\",\"BNB\",\"BTC\",\"ETH\",\"INJ\",\"USDT\",\"OSMO\",\"STX\",\"SOL\"]",
      "index": true
    }
  ]
}`

	var mockEvent abci.Event
	err := json.Unmarshal([]byte(mockData), &mockEvent)
	if err != nil {
		t.Fatal(err)
	}

	bandPrices, err := ABCITOEventSetBandIBCPrice(mockEvent)

	require.NoError(t, err)
	require.NotNil(t, bandPrices)
	require.Equal(t, 9, len(bandPrices))

	firstPrice := bandPrices[0]
	expectedPrice := types.OraclePrice{
		Symbol: "ATOM",
		Price:  sdk.MustNewDecFromStr("9.222000000000000000"),
		Type:   "bandibc",
	}

	require.Equal(t, expectedPrice, *firstPrice)
}

func TestABCITOEventSetProviderPrice(t *testing.T) {

	mockData := `      {
        "type": "injective.oracle.v1beta1.SetProviderPriceEvent",
        "attributes": [
          {
            "key": "price",
            "value": "\"0.500000000000000000\""
          },
          {
            "key": "provider",
            "value": "\"val_provider\""
          },
          {
            "key": "relayer",
            "value": "\"inj15gnk95hvqrsr343ecqjuv7yf2af9rkdqeax52d\""
          },
          {
            "key": "symbol",
            "value": "\"mietek\""
          }
        ]
      }`

	var mockEvent abci.Event
	err := json.Unmarshal([]byte(mockData), &mockEvent)
	if err != nil {
		t.Fatal(err)
	}

	providerPrice, err := ABCITOEventSetProviderPrice(mockEvent)

	require.NoError(t, err)
	require.NotNil(t, providerPrice)

	firstPrice := providerPrice
	expectedPrice := types.OraclePrice{
		Symbol: "mietek",
		Price:  sdk.MustNewDecFromStr("0.500000000000000000"),
		Type:   "provider",
	}

	require.Equal(t, expectedPrice, *firstPrice)
}

func TestABCITOEventSetPricefeedPrice(t *testing.T) {

	mockData := `{
  "type": "injective.oracle.v1beta1.SetPriceFeedPriceEvent",
  "attributes": [
    {
      "key": "base",
      "value": "\"BONK\"",
      "index": true
    },
    {
      "key": "price",
      "value": "\"0.000000334588000000\"",
      "index": true
    },
    {
      "key": "quote",
      "value": "\"USDT\"",
      "index": true
    },
    {
      "key": "relayer",
      "value": "\"inj1cy8rp4e2czmf7nrsz7ekqffyztvud9vyrndzmv\"",
      "index": true
    }
  ]
}`

	var mockEvent abci.Event
	err := json.Unmarshal([]byte(mockData), &mockEvent)
	if err != nil {
		t.Fatal(err)
	}

	priceFeedPrice, err := ABCITOEventSetPricefeedPrice(mockEvent)

	require.NoError(t, err)
	require.NotNil(t, priceFeedPrice)

	firstPrice := priceFeedPrice
	expectedPrice := types.OraclePrice{
		Symbol: "BONK",
		Price:  sdk.MustNewDecFromStr("0.000000334588000000"),
		Type:   "pricefeed",
	}

	require.Equal(t, expectedPrice, *firstPrice)
}

func TestABCIToSpotOrders(t *testing.T) {
	tests := []struct {
		name       string
		inputEvent abci.Event
		wantMsg    []*types.SpotOrderUpdate
		wantErr    error
	}{
		{
			name: "Happy case - One sell order",
			inputEvent: abci.Event{
				Type: string(SpotOrders),
				Attributes: []abci.EventAttribute{
					{
						Key:   "buy_orders",
						Value: "[]",
						Index: true,
					},
					{
						Key:   "market_id",
						Value: "\"0x0686357b934c761784d58a2b8b12618dfe557de108a220e06f8f6580abb83aab\"",
						Index: true,
					},
					{
						Key:   "sell_orders",
						Value: "[{\"order_info\":{\"subaccount_id\":\"0xb31629e5789a69ba6e613369ec965e8ec19e57cf000000000000000000000001\",\"fee_recipient\":\"inj1kvtznetcnf5m5mnpxd57e9j73mqeu470k7lj6w\",\"price\":\"0.147000000000000000\",\"quantity\":\"2550000000.000000000000000000\"},\"order_type\":\"SELL_PO\",\"fillable\":\"2550000000.000000000000000000\",\"trigger_price\":null,\"order_hash\":\"hCxZYt6cJRP8ut4Ju0GrtHbHHSds0fQwCTYs5L1IVyk=\"}]",
						Index: true,
					},
				},
			},
			wantMsg: []*types.SpotOrderUpdate{
				{
					Status:    types.OrderUpdateStatus_Booked,
					OrderHash: mustB64DecodeString("hCxZYt6cJRP8ut4Ju0GrtHbHHSds0fQwCTYs5L1IVyk="),
					Order: &types.SpotOrder{
						MarketId: "0x0686357b934c761784d58a2b8b12618dfe557de108a220e06f8f6580abb83aab",
						Order: exchangeTypes.SpotLimitOrder{
							OrderInfo: exchangeTypes.OrderInfo{
								SubaccountId: "0xb31629e5789a69ba6e613369ec965e8ec19e57cf000000000000000000000001",
								FeeRecipient: "inj1kvtznetcnf5m5mnpxd57e9j73mqeu470k7lj6w",
								Price:        sdk.MustNewDecFromStr("0.147000000000000000"),
								Quantity:     sdk.MustNewDecFromStr("2550000000.000000000000000000"),
							},
							OrderType:    exchangeTypes.OrderType_SELL_PO,
							Fillable:     sdk.MustNewDecFromStr("2550000000.000000000000000000"),
							TriggerPrice: nil,
							OrderHash:    mustB64DecodeString("hCxZYt6cJRP8ut4Ju0GrtHbHHSds0fQwCTYs5L1IVyk="),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "Error case - Unexpected topic",
			inputEvent: abci.Event{
				Type: "some_unexpected_topic",
			},
			wantMsg: nil,
			wantErr: fmt.Errorf("unexpected topic: %s", "some_unexpected_topic"),
		},
		{
			name: "Error case - Invalid JSON in sell_orders attribute",
			inputEvent: abci.Event{
				Type: string(SpotOrders),
				Attributes: []abci.EventAttribute{
					{
						Key:   "buy_orders",
						Value: "[]",
						Index: true,
					},
					{
						Key:   "market_id",
						Value: "\"0x0686357b934c761784d58a2b8b12618dfe557de108a220e06f8f6580abb83aab\"",
						Index: true,
					},
					{
						Key: "sell_orders",
						// Invalid JSON data (missing closing brace '}')
						Value: "[{\"order_info\":{\"subaccount_id\":\"0xb31629e5789a69ba6e613369ec965e8ec19e57cf000000000000000000000001\",\"fee_recipient\":\"inj1kvtznetcnf5m5mnpxd57e9j73mqeu470k7lj6w\",\"price\":\"0.147000000000000000\",\"quantity\":\"2550000000.000000000000000000\"},\"order_type\":\"SELL_PO\",\"fillable\":\"2550000000.000000000000000000\",\"trigger_price\":null,\"order_hash\":\"hCxZYt6cJRP8ut4Ju0GrtHbHHSds0fQwCTYs5L1IVyk=\"]",
						Index: true,
					},
				},
			},
			wantMsg: nil,
			wantErr: fmt.Errorf("failed to unmarshal ABCI event to SpotOrder: invalid character ']' after object key:value pair"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, gotErr := ABCIToSpotOrderUpdates(tt.inputEvent)
			require.EqualValues(t, tt.wantMsg, gotMsg)
			if !errorsEqual(gotErr, tt.wantErr) {
				t.Errorf("got error: %v, want error: %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestABCIToDerivativeOrders(t *testing.T) {
	zeroDec := sdk.MustNewDecFromStr("0.000000000000000000")
	tests := []struct {
		name       string
		inputEvent abci.Event
		wantMsg    []*types.DerivativeOrderUpdate
		wantErr    error
	}{
		// Test case 1: Happy case - One buy order
		{
			name: "Happy case - One buy order",
			inputEvent: abci.Event{
				Type: string(DerivativeOrders),
				Attributes: []abci.EventAttribute{
					{
						Key:   "buy_orders",
						Value: "[{\"order_info\":{\"subaccount_id\":\"0x31307ef22c77cc89cd7e61246608f15b82d09fed000000000000000000000001\",\"fee_recipient\":\"inj1xyc8au3vwlxgnnt7vyjxvz83twpdp8lde3qmkg\",\"price\":\"241480000.000000000000000000\",\"quantity\":\"567.890000000000000000\"},\"order_type\":\"BUY_PO\",\"margin\":\"13713407720.000000000000000000\",\"fillable\":\"567.890000000000000000\",\"trigger_price\":\"0.000000000000000000\",\"order_hash\":\"yM2ZwdHYvJA6BKENqCIBUwqKj35se4LWQRmY9FWzAb4=\"}]",
						Index: true,
					},
					{
						Key:   "market_id",
						Value: "\"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced\"",
						Index: true,
					},
					{
						Key:   "sell_orders",
						Value: "[]",
						Index: true,
					},
				},
			},
			wantMsg: []*types.DerivativeOrderUpdate{
				{
					Status:    types.OrderUpdateStatus_Booked,
					OrderHash: mustB64DecodeString("yM2ZwdHYvJA6BKENqCIBUwqKj35se4LWQRmY9FWzAb4="),
					Order: &types.DerivativeOrder{
						MarketId: "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						Order: exchangeTypes.DerivativeLimitOrder{
							OrderInfo: exchangeTypes.OrderInfo{
								SubaccountId: "0x31307ef22c77cc89cd7e61246608f15b82d09fed000000000000000000000001",
								FeeRecipient: "inj1xyc8au3vwlxgnnt7vyjxvz83twpdp8lde3qmkg",
								Price:        sdk.MustNewDecFromStr("241480000.000000000000000000"),
								Quantity:     sdk.MustNewDecFromStr("567.890000000000000000"),
							},
							OrderType:    exchangeTypes.OrderType_BUY_PO,
							Margin:       sdk.MustNewDecFromStr("13713407720.000000000000000000"),
							Fillable:     sdk.MustNewDecFromStr("567.890000000000000000"),
							OrderHash:    mustB64DecodeString("yM2ZwdHYvJA6BKENqCIBUwqKj35se4LWQRmY9FWzAb4="),
							TriggerPrice: &zeroDec,
						},
					},
				},
			},
			wantErr: nil,
		},
		// Test case 2: Error case - Unexpected topic
		{
			name: "Error case - Unexpected topic",
			inputEvent: abci.Event{
				Type: "some_unexpected_topic",
			},
			wantMsg: nil,
			wantErr: fmt.Errorf("unexpected topic: %s", "some_unexpected_topic"),
		},
		// Test case 3: Error case - Invalid JSON in buy_orders attribute
		{
			name: "Error case - Invalid JSON in buy_orders attribute",
			inputEvent: abci.Event{
				Type: string(DerivativeOrders),
				Attributes: []abci.EventAttribute{
					{
						Key: "buy_orders",
						// Invalid JSON data (missing closing brace '}')
						Value: "[{\"order_info\":{\"subaccount_id\":\"0x31307ef22c77cc89cd7e61246608f15b82d09fed000000000000000000000001\",\"fee_recipient\":\"inj1xyc8au3vwlxgnnt7vyjxvz83twpdp8lde3qmkg\",\"price\":\"241480000.000000000000000000\",\"quantity\":\"567.890000000000000000\"},\"order_type\":\"BUY_PO\",\"margin\":\"13713407720.000000000000000000\",\"fillable\":\"567.890000000000000000\",\"trigger_price\":\"0.000000000000000000\",\"order_hash\":\"yM2ZwdHYvJA6BKENqCIBUwqKj35se4LWQRmY9FWzAb4=\"]",
						Index: true,
					},
					{
						Key:   "market_id",
						Value: "\"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced\"",
						Index: true,
					},
					{
						Key:   "sell_orders",
						Value: "[]",
						Index: true,
					},
				},
			},
			wantMsg: nil,
			wantErr: fmt.Errorf("failed to unmarshal ABCI event to DerivativeOrder: invalid character ']' after object key:value pair"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, gotErr := ABCIToDerivativeOrderUpdates(tt.inputEvent)
			require.EqualValues(t, tt.wantMsg, gotMsg)
			if !errorsEqual(gotErr, tt.wantErr) {
				t.Errorf("got error: %v, want error: %v", gotErr, tt.wantErr)
			}
		})
	}
}

var zeroDec = sdk.MustNewDecFromStr("0.000000000000000000")

func TestABCICancelDerivativeOrderToDerivativeOrderUpdates(t *testing.T) {
	tests := []struct {
		name                 string
		mockData             string
		expectedError        error
		expectedOrderUpdates []*types.DerivativeOrderUpdate
	}{
		{
			name: "happy case",
			mockData: `{
				"type": "injective.exchange.v1beta1.EventCancelDerivativeOrder",
				"attributes": [
					{
						"key": "isLimitCancel",
						"value": "true"
					},
					{
						"key": "limit_order",
						"value": "{\"order_info\":{\"subaccount_id\":\"0x13414b047539298d5aed429722211681eaab43b7000000000000000000000000\",\"fee_recipient\":\"inj1zdq5kpr48y5c6khdg2tjyggks842ksahapmy47\",\"price\":\"1672600000.000000000000000000\",\"quantity\":\"0.020000000000000000\"},\"order_type\":\"SELL\",\"margin\":\"33452000.000000000000000000\",\"fillable\":\"0.020000000000000000\",\"trigger_price\":\"0.000000000000000000\",\"order_hash\":\"yYSr6XaynSn+AgmEIT+oekgP/mEoaQiI7/K1GnwAeJA=\"}"
					},
					{
						"key": "market_id",
						"value": "\"0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a\""
					},
					{
						"key": "market_order_cancel",
						"value": "null"
					}
				]
			}`,
			expectedError: nil,
			expectedOrderUpdates: []*types.DerivativeOrderUpdate{
				{
					Status:    types.OrderUpdateStatus_Cancelled,
					OrderHash: mustB64DecodeString("yYSr6XaynSn+AgmEIT+oekgP/mEoaQiI7/K1GnwAeJA="),
					Order: &types.DerivativeOrder{
						MarketId: "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
						Order: exchangeTypes.DerivativeLimitOrder{
							OrderInfo: exchangeTypes.OrderInfo{
								SubaccountId: "0x13414b047539298d5aed429722211681eaab43b7000000000000000000000000",
								FeeRecipient: "inj1zdq5kpr48y5c6khdg2tjyggks842ksahapmy47",
								Price:        sdk.MustNewDecFromStr("1672600000.000000000000000000"),
								Quantity:     sdk.MustNewDecFromStr("0.020000000000000000"),
							},
							OrderType:    exchangeTypes.OrderType_SELL,
							Margin:       sdk.MustNewDecFromStr("33452000.000000000000000000"),
							Fillable:     sdk.MustNewDecFromStr("0.020000000000000000"),
							TriggerPrice: &zeroDec,
							OrderHash:    mustB64DecodeString("yYSr6XaynSn+AgmEIT+oekgP/mEoaQiI7/K1GnwAeJA="),
						},
					},
				},
			},
		},
		{
			name: "wrong type",
			mockData: `{
				"type": "injective.exchange.v1beta1.WrongType",
				"attributes": []
			}`,
			expectedError: errors.New("unexpected topic: injective.exchange.v1beta1.WrongType"), // Adjust this based on your actual error message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockEvent abci.Event
			err := json.Unmarshal([]byte(tt.mockData), &mockEvent)
			if err != nil {
				t.Fatal(err)
			}

			orderUpdates, err := ABCICancelDerivativeOrderToDerivativeOrderUpdates(mockEvent)
			require.Equal(t, tt.expectedError, err)
			if tt.expectedError == nil {
				require.Equal(t, tt.expectedOrderUpdates, orderUpdates)
			}
		})
	}
}

func TestABCICancelSpotOrderToSpotOrderUpdates(t *testing.T) {
	tests := []struct {
		name                 string
		mockData             string
		expectedError        error
		expectedOrderUpdates []*types.SpotOrderUpdate
	}{
		{
			name: "happy case",
			mockData: `{
				"type": "injective.exchange.v1beta1.EventCancelSpotOrder",
				"attributes": [
					{
						"key": "market_id",
						"value": "\"0xb9a07515a5c239fcbfa3e25eaa829a03d46c4b52b9ab8ee6be471e9eb0e9ea31\""
					},
					{
						"key": "order",
						"value": "{\"order_info\":{\"subaccount_id\":\"0xf9c0cf9ee2e4816fab75c261ab16e83305c3f3fd000000000000000000000001\",\"fee_recipient\":\"inj1l8qvl8hzujqkl2m4cfs6k9hgxvzu8ularqrx8w\",\"price\":\"0.005191000000000000\",\"quantity\":\"7010000000.000000000000000000\"},\"order_type\":\"SELL_PO\",\"fillable\":\"7010000000.000000000000000000\",\"trigger_price\":null,\"order_hash\":\"8hit7VcwjzBbZWX56Fb9QNsELm2GghqbKa+ZjZDkYyo=\"}"
					}
				]
			}`,
			expectedError: nil,
			expectedOrderUpdates: []*types.SpotOrderUpdate{
				{
					Status:    types.OrderUpdateStatus_Cancelled,
					OrderHash: mustB64DecodeString("8hit7VcwjzBbZWX56Fb9QNsELm2GghqbKa+ZjZDkYyo="),
					Order: &types.SpotOrder{
						MarketId: "0xb9a07515a5c239fcbfa3e25eaa829a03d46c4b52b9ab8ee6be471e9eb0e9ea31",
						Order: exchangeTypes.SpotLimitOrder{
							OrderInfo: exchangeTypes.OrderInfo{
								SubaccountId: "0xf9c0cf9ee2e4816fab75c261ab16e83305c3f3fd000000000000000000000001",
								FeeRecipient: "inj1l8qvl8hzujqkl2m4cfs6k9hgxvzu8ularqrx8w",
								Price:        sdk.MustNewDecFromStr("0.005191000000000000"),
								Quantity:     sdk.MustNewDecFromStr("7010000000.000000000000000000"),
							},
							OrderType:    exchangeTypes.OrderType_SELL_PO,
							Fillable:     sdk.MustNewDecFromStr("7010000000.000000000000000000"),
							TriggerPrice: nil,
							OrderHash:    mustB64DecodeString("8hit7VcwjzBbZWX56Fb9QNsELm2GghqbKa+ZjZDkYyo="),
						},
					},
				},
			},
		},
		{
			name: "wrong type",
			mockData: `{
				"type": "injective.exchange.v1beta1.WrongType",
				"attributes": []
			}`,
			expectedError: errors.New("unexpected topic: injective.exchange.v1beta1.WrongType"), // Adjust this based on your actual error message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockEvent abci.Event
			err := json.Unmarshal([]byte(tt.mockData), &mockEvent)
			if err != nil {
				t.Fatal(err)
			}

			orderUpdates, err := ABCICancelSpotOrderToSpotOrderUpdates(mockEvent)
			require.Equal(t, tt.expectedError, err)
			if tt.expectedError == nil {
				require.Equal(t, tt.expectedOrderUpdates, orderUpdates)
			}
		})
	}
}

// Helper function to check if two errors are equal
func errorsEqual(err1, err2 error) bool {
	if (err1 == nil && err2 != nil) || (err1 != nil && err2 == nil) {
		return false
	}

	if err1 == nil && err2 == nil {
		return true
	}

	return err1.Error() == err2.Error()
}

func mustB64DecodeString(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}
