package fuzztesting

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
)

type Account struct {
	AccAddress    sdk.AccAddress
	SubaccountIDs []string
}

type Action struct {
	name   string
	weight int
	f      func(int)
}

var numSubaccounts = 3
var FeeRecipient = "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"

// Use higher dust for longer fuzz test runs, e.g. 1 USD
// var dust = sdk.MustNewDecFromStr("1000000")
var dust = sdk.MustNewDecFromStr("1")

var (
	masterContractAddress, vaultSubaccountId string
)
var VaultContractAddress string

func matchTick(base, minTick sdk.Dec) sdk.Dec {
	rounded := base.Quo(minTick).Ceil().Mul(minTick)
	return rounded
}

func extractBytes(data *[]byte, nBytes int) []byte {
	dataBytes := nBytes
	if dataBytes > len(*data) {
		dataBytes = len(*data)
	}
	bytes := (*data)[:dataBytes]
	*data = (*data)[dataBytes:]
	for len(bytes) < nBytes {
		bytes = append(bytes, 0)
	}
	return bytes
}

func randInt(data *[]byte) int {
	bytes := extractBytes(data, 4)
	return int(binary.LittleEndian.Uint32(bytes))
}

func randIntn(data *[]byte, n int) int {
	x := randInt(data)
	if x < 0 {
		x = -x
	}
	return x % n
}

func randInt63(data *[]byte) int64 {
	bytes := extractBytes(data, 8)
	return int64(binary.LittleEndian.Uint64(bytes))
}

func randInt63n(data *[]byte, n int64) int64 {
	x := randInt63(data)
	if x < 0 {
		x = -x
	}
	return x % n
}

func genNullableInitialMarginRatio(data *[]byte, maintenanceMarginRatio *sdk.Dec) *sdk.Dec {
	if maintenanceMarginRatio == nil {
		return nil
	}

	switch randIntn(data, 40) {
	case 0: // 2.5% of nil
		return nil
	case 1: // 2.5% of > 1 value
		rate := maintenanceMarginRatio.Mul(sdk.MustNewDecFromStr("2.1"))
		return &rate
	case 2: // 2.5% of < 0 value
		rate := maintenanceMarginRatio.Mul(sdk.MustNewDecFromStr("0.9"))
		return &rate
	default: // 92.5% of correct range
		rate := maintenanceMarginRatio.Mul(sdk.MustNewDecFromStr("1.1"))
		return &rate
	}
}

func genNullableMakerFeeRate(data *[]byte) *sdk.Dec {
	r := randIntn(data, 40)

	if r == 0 {
		return nil
	} else if r == 1 {
		rate := sdk.NewDec(randInt63n(data, 10000) + 101).QuoInt64(100)
		return &rate
	} else if r >= 2 && r <= 20 {
		rate := sdk.NewDec(randInt63n(data, 15)).QuoInt64(100).Neg()
		return &rate
	}

	rate := sdk.NewDec(randInt63n(data, 15)).QuoInt64(100)
	return &rate
}

func genNullableTakerFeeRate(data *[]byte, makerFeeRate *sdk.Dec) *sdk.Dec {
	if makerFeeRate == nil {
		return nil
	}

	if makerFeeRate.Equal(sdk.ZeroDec()) {
		rate := sdk.ZeroDec()
		return &rate
	}

	switch randIntn(data, 40) {
	case 0: // 2.5% of nil
		return nil
	default: // 97.5% of correct range
		makerFeeRateScaled := makerFeeRate.MulInt64(10000).Abs().RoundInt64()

		minRateScaled := makerFeeRateScaled
		maxRateScaled := makerFeeRateScaled * 3

		takerFeeRateScaled := sdk.NewDec(randInt63n(data, maxRateScaled-minRateScaled+1) + minRateScaled)

		rate := takerFeeRateScaled.QuoInt64(10000)
		return &rate
	}
}

func genNullableFeeRate(data *[]byte) *sdk.Dec {
	switch randIntn(data, 40) {
	case 0: // 2.5% of nil
		return nil
	case 1: // 2.5% of > 1 value
		rate := sdk.NewDec(randInt63n(data, 10000) + 101).QuoInt64(100)
		return &rate
	case 2: // 2.5% of < 0 value
		rate := sdk.NewDec(randInt63n(data, 15)).QuoInt64(100).Neg()
		return &rate
	default: // 92.5% of correct range
		rate := sdk.NewDec(randInt63n(data, 15)).QuoInt64(100)
		return &rate
	}
}

func genNullableHourlyInterestRate(data *[]byte) *sdk.Dec {
	switch randIntn(data, 40) {
	case 0: // 2.5% of nil
		return nil
	case 1: // 2.5% of > 1 value
		rate := sdk.NewDec(randInt63n(data, 10000) + 101).QuoInt64(100)
		return &rate
	case 2: // 2.5% of < 0 value
		rate := sdk.NewDec(randInt63n(data, 100)).QuoInt64(100).Neg()
		return &rate
	default: // 92.5% of correct range
		rate := sdk.NewDec(randInt63n(data, 100)).QuoInt64(10000)
		return &rate
	}
}

func genNullableTickSize(data *[]byte) *sdk.Dec {
	switch randIntn(data, 5) {
	case 0: // 20% of nil
		return nil
	case 1:
		rate := sdk.MustNewDecFromStr("0.000001")
		return &rate
	case 2:
		rate := sdk.MustNewDecFromStr("0.001")
		return &rate
	case 3:
		rate := sdk.MustNewDecFromStr("0.1")
		return &rate
	default:
		rate := sdk.MustNewDecFromStr("1")
		return &rate
	}
}

func genNullableFundingCap(data *[]byte) *sdk.Dec {
	switch randIntn(data, 5) {
	case 0: // 20% of nil
		return nil
	default: // 80% of non-nil
		rate := sdk.NewDec(randInt63n(data, 300)).QuoInt64(10000)
		return &rate
	}
}

func genMarketStatus(data *[]byte, r int) exchangetypes.MarketStatus {
	switch randIntn(data, r) {
	case 0:
		return exchangetypes.MarketStatus_Active
	case 1:
		return exchangetypes.MarketStatus_Expired
	case 2:
		return exchangetypes.MarketStatus_Paused
	case 3:
		return exchangetypes.MarketStatus_Demolished
	default:
		return exchangetypes.MarketStatus_Unspecified
	}
}

/*
*
This method will try to find some random active market - will retry until it finds one
*/
func findRandomActiveMarketId(data *[]byte, marketType exchangetypes.MarketType, testInput testexchange.TestInput, marketsNumber int) (*common.Hash, int) {
	max := 20
	if marketsNumber == 0 {
		return nil, -1
	}
	for i := 0; i < max; i++ {
		marketIndex := randIntn(data, marketsNumber)

		switch marketType {
		case exchangetypes.MarketType_Spot:
			if testInput.Spots[marketIndex].IsActive {
				return &testInput.Spots[marketIndex].MarketID, marketIndex
			}
		case exchangetypes.MarketType_Perpetual:
			if testInput.Perps[marketIndex].IsActive {
				return &testInput.Perps[marketIndex].MarketID, marketIndex
			}
		case exchangetypes.MarketType_Expiry:
			if testInput.ExpiryMarkets[marketIndex].IsActive {
				return &testInput.ExpiryMarkets[marketIndex].MarketID, marketIndex
			}
		case exchangetypes.MarketType_BinaryOption:
			if testInput.BinaryMarkets[marketIndex].IsActive {
				return &testInput.BinaryMarkets[marketIndex].MarketID, marketIndex
			}
		}
	}

	return nil, -1
}

func saveFuzzTestActions(replayFilePath string, history *testexchange.RecordedTest) {
	bytes, err := json.Marshal(history)
	if err != nil {
		return
	}
	err = os.WriteFile(replayFilePath, bytes, 0755)
	if err != nil {
		return
	}
}

func ReplayFuzzTest(replayFilePath string, testflags testexchange.TestFlags) error {
	testSetup := testexchange.LoadReplayableTest(replayFilePath)
	return testSetup.ReplayTest(testflags, nil)
}

func fuzzTestAsGinkgo(t *testing.T, numAccounts, numSpotMarkets, numDerivativeMarkets, numExpiryMarkets, numBinaryOptionsMarkets, numOperations int, dataOrig []byte) {
	RegisterTestingT(t)
	fuzzTest(numAccounts, numSpotMarkets, numDerivativeMarkets, numExpiryMarkets, numBinaryOptionsMarkets, numOperations, dataOrig)
}

func fuzzTest(numAccounts, numSpotMarkets, numDerivativeMarkets, numExpiryMarkets, numBinaryOptionsMarkets, numOperations int, dataOrig []byte) {
	data := make([]byte, len(dataOrig))
	copy(data, dataOrig)

	numAccounts %= 7 // <= 6
	if numAccounts < 1 {
		numAccounts = 1
	}
	numSpotMarkets %= 3 // <= 2
	if numSpotMarkets < 1 {
		numSpotMarkets = 1
	}
	numDerivativeMarkets %= 3 // <= 2
	if numDerivativeMarkets < 1 {
		numDerivativeMarkets = 1
	}
	numExpiryMarkets %= 3 // <= 2
	if numExpiryMarkets < 1 {
		numExpiryMarkets = 1
	}
	numBinaryOptionsMarkets %= 3 // <= 2
	if numBinaryOptionsMarkets < 1 {
		numBinaryOptionsMarkets = 1
	}

	var history *testexchange.RecordedTest
	config := testexchange.TestPlayerConfig{
		NumAccounts:               numAccounts,
		NumSpotMarkets:            numSpotMarkets,
		NumDerivativeMarkets:      numDerivativeMarkets,
		NumExpiryMarkets:          numExpiryMarkets,
		NumBinaryOptionsMarkets:   numBinaryOptionsMarkets,
		BankParams:                nil,
		ExchangeParams:            nil,
		TokenFactoryParams:        nil,
		PerpMarkets:               nil,
		InitMarketMaking:          false,
		InitContractRegistry:      false,
		InitAuctionModule:         false,
		RegistryOwnerAccountIndex: 0,
		RandSeed:                  randInt63n(&data, 100000),
		NumSubaccounts:            numSubaccounts,
	}
	testPlayer := testexchange.InitTest(config, nil)
	testInput := testPlayer.TestInput
	app := testPlayer.App
	ctx := testPlayer.Ctx
	accounts := *testPlayer.Accounts
	addressBySubAccount := *testPlayer.AddressBySubAccount
	initCoins := *testPlayer.InitCoins

	// ---- register actions ----
	actions := []Action{}
	numOperationsByAction := make(map[int]int)
	successActions := make(map[int]int)
	failedActions := make(map[int]int)

	// price oracle refresh action
	setOraclePricesAction := Action{
		name:   "set price oracle",
		weight: 100,
		f: func(actionId int) {
			action := testexchange.SetPriceOracles(testInput, app, ctx, randInt63n(&data, 10000))
			if history != nil {
				history.Actions = append(history.Actions, action)
			}
			successActions[actionId]++
		},
	}

	createSpotLimitOrderAction := Action{
		name:   "create spot limit order",
		weight: 250,
		f: func(actionId int) {
			accountIndex := randIntn(&data, numAccounts)
			if len(testInput.Spots) == 0 {
				return
			}
			marketID := testInput.Spots[randIntn(&data, numSpotMarkets)].MarketID
			isBuy := (randIntn(&data, 2) == 0)
			isPostOnly := (randIntn(&data, 2) == 0)

			orderType := exchangetypes.OrderType_BUY

			if isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_BUY_PO
			} else if !isBuy && !isPostOnly {
				orderType = exchangetypes.OrderType_SELL
			} else if !isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_SELL_PO
			}

			price := sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)
			quantity := sdk.NewDec(randInt63n(&data, 100000)).QuoInt64(100)

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCreateSpotLimitOrder{
				Sender: accounts[accountIndex].AccAddress.String(),
				Order: exchangetypes.SpotOrder{
					MarketId: marketID.String(),
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: accounts[accountIndex].SubaccountIDs[0],
						FeeRecipient: FeeRecipient,
						Price:        price,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					TriggerPrice: nil,
				},
			}
			if msg.ValidateBasic() != nil {
				return
			}
			_, err := msgServer.CreateSpotLimitOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
				// fmt.Println("spot-limit ", accountIndex, orderType.String(), price.String(), quantity.String())
			}
		},
	}

	createSpotMarketOrderAction := Action{
		name:   "create spot market order",
		weight: 200,
		f: func(actionId int) {
			accountIndex := randIntn(&data, numAccounts)
			if len(testInput.Spots) == 0 {
				return
			}
			marketID := testInput.Spots[randIntn(&data, numSpotMarkets)].MarketID
			isBuy := randIntn(&data, 2) == 0
			isPostOnly := randIntn(&data, 2) == 0

			orderType := exchangetypes.OrderType_BUY

			if isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_BUY_PO
			} else if !isBuy && !isPostOnly {
				orderType = exchangetypes.OrderType_SELL
			} else if !isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_SELL_PO
			}

			price := sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)
			quantity := sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCreateSpotMarketOrder{
				Sender: accounts[accountIndex].AccAddress.String(),
				Order: exchangetypes.SpotOrder{
					MarketId: marketID.String(),
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: accounts[accountIndex].SubaccountIDs[0],
						FeeRecipient: FeeRecipient,
						Price:        price,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					TriggerPrice: nil,
				},
			}
			if msg.ValidateBasic() != nil {
				return
			}
			_, err := msgServer.CreateSpotMarketOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
			}
		},
	}

	cancelSpotLimitOrderAction := Action{
		name:   "cancel spot limit order",
		weight: 100,
		f: func(actionId int) {
			if len(testInput.Spots) == 0 {
				return
			}
			marketID := testInput.Spots[randIntn(&data, numSpotMarkets)].MarketID
			isBuy := (randIntn(&data, 2) == 0)
			market := app.ExchangeKeeper.GetSpotMarket(ctx, marketID, true)
			if market == nil {
				return
			}
			orders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, isBuy)
			if len(orders) == 0 {
				return
			}
			order := orders[randIntn(&data, len(orders))]
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCancelSpotOrder{
				Sender:       addressBySubAccount[order.OrderInfo.SubaccountId].String(),
				MarketId:     marketID.String(),
				SubaccountId: order.OrderInfo.SubaccountId,
				OrderHash:    "0x" + common.Bytes2Hex(order.OrderHash),
			}
			if msg.ValidateBasic() != nil {
				return
			}
			_, err := msgServer.CancelSpotOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				panic(err)
			}
		},
	}

	updateSpotMarketProposalAction := Action{
		name:   "spot market update proposal",
		weight: 5,
		f: func(actionId int) {
			if len(testInput.Spots) == 0 {
				return
			}
			marketIndex := randIntn(&data, numSpotMarkets)
			marketID := testInput.Spots[marketIndex].MarketID

			makerFeeRate := genNullableMakerFeeRate(&data)
			marketStatus := genMarketStatus(&data, 50)

			p := &exchangetypes.SpotMarketParamUpdateProposal{
				Title:               "Spot market param update",
				Description:         "Spot market param update",
				MarketId:            marketID.String(),
				MakerFeeRate:        makerFeeRate,
				TakerFeeRate:        genNullableTakerFeeRate(&data, makerFeeRate),
				RelayerFeeShareRate: genNullableFeeRate(&data),
				MinPriceTickSize:    genNullableTickSize(&data),
				MinQuantityTickSize: genNullableTickSize(&data),
				Status:              marketStatus,
			}

			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			ctxCached, writeCache := ctx.CacheContext()
			err := handler(ctxCached, p)

			if err == nil {
				writeCache()
				successActions[actionId]++
				if marketStatus != exchangetypes.MarketStatus_Active && marketStatus != exchangetypes.MarketStatus_Unspecified {
					testInput.Spots[marketIndex].IsActive = false
				}
			}
		},
	}

	closeExpiryMarketAction := Action{
		name:   "close expiry market",
		weight: 2,
		f: func(actionId int) {
			if numExpiryMarkets == 0 {
				return
			}
			marketIndex := randIntn(&data, numExpiryMarkets)
			if marketIndex < 2 { // keep at least 2 expiry market open
				return
			}
			marketID := testInput.ExpiryMarkets[marketIndex].MarketID
			market := app.ExchangeKeeper.GetDerivativeMarket(ctx, marketID, true)
			if market == nil {
				return
			}
			ctxCached, writeCache := ctx.CacheContext()
			err := app.ExchangeKeeper.ScheduleDerivativeMarketParamUpdate(ctxCached, &exchangetypes.DerivativeMarketParamUpdateProposal{
				Title:                  "Close expiry market",
				Description:            "Close expiry market",
				MarketId:               marketID.String(),
				InitialMarginRatio:     &market.InitialMarginRatio,
				MaintenanceMarginRatio: &market.MaintenanceMarginRatio,
				MakerFeeRate:           &market.MakerFeeRate,
				TakerFeeRate:           &market.TakerFeeRate,
				RelayerFeeShareRate:    &market.RelayerFeeShareRate,
				MinPriceTickSize:       &market.MinPriceTickSize,
				MinQuantityTickSize:    &market.MinQuantityTickSize,
				Status:                 exchangetypes.MarketStatus_Paused,
			})
			if err == nil {
				writeCache()
				successActions[actionId]++
				testInput.ExpiryMarkets[marketIndex].IsActive = false
			} else {
				failedActions[actionId]++
			}
		},
	}

	// create derivative limit order
	createDerivativeLimitOrderAction := Action{
		name:   "create derivative limit order",
		weight: 300,
		f: func(actionId int) {
			accountIndex := randIntn(&data, numAccounts)
			isExpiryMarket := randIntn(&data, numDerivativeMarkets+numExpiryMarkets+1) < numDerivativeMarkets

			var marketType exchangetypes.MarketType
			if isExpiryMarket {
				marketType = exchangetypes.MarketType_Expiry
			} else {
				marketType = exchangetypes.MarketType_Perpetual
			}

			marketID, marketIdx := findRandomActiveMarketId(&data, marketType, testInput, numDerivativeMarkets)
			if marketID == nil {
				return
			}
			subaccountID := common.HexToHash(accounts[accountIndex].SubaccountIDs[0])
			market, markPrice := app.ExchangeKeeper.GetDerivativeMarketWithMarkPrice(ctx, *marketID, true)
			if market == nil || market.Status != exchangetypes.MarketStatus_Active {
				return
			}

			isBuy := randIntn(&data, 2) == 0
			isPostOnly := randIntn(&data, 5) == 0
			isConditional := !isPostOnly && randIntn(&data, 4) == 0

			var orderType = exchangetypes.OrderType_BUY
			if isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_BUY_PO
			} else if !isBuy && !isPostOnly {
				orderType = exchangetypes.OrderType_SELL
			} else if !isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_SELL_PO
			}

			minPrice := markPrice.QuoInt64(2).RoundInt64() + 1
			maxPrice := markPrice.MulInt64(2).RoundInt64()
			price := sdk.NewDec(randInt63n(&data, maxPrice-minPrice+1) + minPrice)
			var triggerPrice *sdk.Dec = nil
			if isConditional {
				minTriggerPrice := markPrice.Mul(f2d(0.9)).RoundInt64() + 1
				maxTriggerPrice := markPrice.Mul(f2d(1.1)).RoundInt64()
				tp := sdk.NewDec(randInt63n(&data, maxTriggerPrice-minTriggerPrice+1) + minTriggerPrice)
				triggerPrice = &tp
				if markPrice.GT(tp) {
					if orderType == exchangetypes.OrderType_BUY {
						orderType = exchangetypes.OrderType_STOP_BUY
					} else {
						orderType = exchangetypes.OrderType_TAKE_SELL
					}
				} else if markPrice.LT(tp) {
					if orderType == exchangetypes.OrderType_BUY {
						orderType = exchangetypes.OrderType_TAKE_BUY
					} else {
						orderType = exchangetypes.OrderType_STOP_SELL
					}
				} else {
					triggerPrice = nil
				}
			}

			maxQuantity := int64(100)
			quantity := sdk.NewDec(randInt63n(&data, maxQuantity))
			notional := price.Mul(quantity)

			positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, *marketID)
			existingPositionQuantity := sdk.ZeroDec()
			var existingPositionIsLong bool

			for _, position := range positions {
				if position.SubaccountId == subaccountID.Hex() {
					existingPositionQuantity = position.Position.Quantity
					existingPositionIsLong = position.Position.IsLong
					break
				}
			}

			margin := sdk.Dec{}
			// 30% to be reduce-only orders
			isReduceOnly := (randIntn(&data, 3) == 0) && existingPositionQuantity.IsPositive()

			if isReduceOnly {
				quantity = sdk.MinDec(quantity, existingPositionQuantity)

				if existingPositionIsLong {
					orderType = exchangetypes.OrderType_SELL
				} else {
					orderType = exchangetypes.OrderType_BUY
				}

				orders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, *marketID, isBuy)
				for _, order := range orders {
					isReduceOnlyOrder := order.Margin.IsZero()

					if order.OrderInfo.SubaccountId == subaccountID.Hex() && isReduceOnlyOrder {
						return
					}
				}
			}

			if isReduceOnly {
				margin = sdk.ZeroDec()
			} else if isBuy {
				// For Buys: Margin >= MarkPrice * Quantity * (1 - InitialMarginRatio) && Margin >= Price * Quantity
				margin = matchTick(markPrice.Mul(quantity).Mul(sdk.OneDec().Sub(market.InitialMarginRatio)).
					Add(notional).
					Add(sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)), market.MinQuantityTickSize)
			} else {
				// For Sells: Margin >= MarkPrice * Quantity * (1 + InitialMarginRatio) && Margin >= Price * Quantity
				margin = matchTick(markPrice.Mul(quantity).Mul(sdk.OneDec().Add(market.InitialMarginRatio)).
					Add(notional).
					Add(sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)), market.MinQuantityTickSize)
			}

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCreateDerivativeLimitOrder{
				Sender: accounts[accountIndex].AccAddress.String(),
				Order: exchangetypes.DerivativeOrder{
					MarketId: marketID.String(),
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID.String(),
						FeeRecipient: FeeRecipient,
						Price:        price,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					Margin:       margin,
					TriggerPrice: triggerPrice,
				},
			}
			// fmt.Println("Create limit order msg", msg)

			if msg.ValidateBasic() != nil {
				return
			}
			rep, err := msgServer.CreateDerivativeLimitOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
				// fmt.Printf("= Derivative-limit %s: [%d] %s p: %s q: %s m: %s\n", rep.OrderHash, accountIndex, orderType.String(), price.String(), quantity.String(), margin.String())
			}
			if history != nil {
				orderId := ""
				if rep != nil {
					orderId = rep.OrderHash
				}
				marginF := margin.MustFloat64()
				order := testexchange.ActionCreateOrder{}
				order.ActionType = testexchange.ActionType_derivativeLimitOrder
				order.OrderId = orderId
				ot := testexchange.GetTestOrderType(orderType)
				order.OrderType = &ot
				order.Quantity = quantity.MustFloat64()
				order.Price = price.MustFloat64()
				order.AccountIndex = accountIndex
				order.Margin = &marginF
				order.MarketIndex = marketIdx
				order.IsReduceOnly = isReduceOnly
				order.IsLong = isBuy
				history.Actions = append(history.Actions, &order)
			}
		},
	}

	createDerivativeMarketOrderAction := Action{
		name:   "create derivative market order",
		weight: 150,
		f: func(actionId int) {
			accountIndex := randIntn(&data, numAccounts)
			isExpiryMarket := randIntn(&data, numDerivativeMarkets+numExpiryMarkets+1) < numDerivativeMarkets
			subaccountID := common.HexToHash(accounts[accountIndex].SubaccountIDs[0])

			isBuy := (randIntn(&data, 2) == 0)
			isPostOnly := (randIntn(&data, 200) == 0) // post only will fail for market orders, so we don't want every 2nd order to fail, just a tiny bit

			var orderType = exchangetypes.OrderType_BUY

			if isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_BUY_PO
			} else if !isBuy && !isPostOnly {
				orderType = exchangetypes.OrderType_SELL
			} else if !isBuy && isPostOnly {
				orderType = exchangetypes.OrderType_SELL_PO
			}
			var marketType exchangetypes.MarketType
			if isExpiryMarket {
				marketType = exchangetypes.MarketType_Expiry
			} else {
				marketType = exchangetypes.MarketType_Perpetual
			}

			marketID, marketIdx := findRandomActiveMarketId(&data, marketType, testInput, numDerivativeMarkets)
			if marketID == nil {
				return
			}
			market, markPrice := app.ExchangeKeeper.GetDerivativeMarketWithMarkPrice(ctx, *marketID, true)
			if market == nil {
				return
			}

			minPriceTick := market.MinPriceTickSize
			minQuantityTick := market.MinQuantityTickSize
			price := sdk.NewDec(randInt63n(&data, 1000)).Mul(minPriceTick)

			quantity := sdk.NewDec(randInt63n(&data, 100)).Mul(minQuantityTick)
			notional := price.Mul(quantity)

			margin := sdk.Dec{}

			// 10% to be reduce-only orders
			//isReduceOnly := (r1.Intn(10) == 0)
			isReduceOnly := false
			if isReduceOnly {
				margin = sdk.ZeroDec()
			} else if isBuy {
				// For Buys: Margin >= MarkPrice * Quantity * (1 - InitialMarginRatio) && Margin >= Price * Quantity
				margin =
					matchTick(markPrice.Mul(quantity).Mul(sdk.OneDec().Sub(market.InitialMarginRatio)).
						Add(notional).
						Add(sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)), minQuantityTick)
			} else {
				// For Sells: Margin >= MarkPrice * ((InitialMarginRatio + 1) * Quantity) - Price * Quantity
				margin = matchTick(notional.
					Mul(market.InitialMarginRatio).
					Add(markPrice.Mul(market.InitialMarginRatio.Add(sdk.OneDec())).Mul(quantity)).
					Sub(notional).
					Add(sdk.NewDec(randInt63n(&data, 10000)).QuoInt64(100)), minQuantityTick)
			}
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCreateDerivativeMarketOrder{
				Sender: accounts[accountIndex].AccAddress.String(),
				Order: exchangetypes.DerivativeOrder{
					MarketId: marketID.String(),
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID.String(),
						FeeRecipient: FeeRecipient,
						Price:        price,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					Margin:       margin,
					TriggerPrice: nil,
				},
			}
			if err := msg.ValidateBasic(); err != nil {
				failedActions[actionId]++
				return
			}
			_, err := msgServer.CreateDerivativeMarketOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
				if history != nil {
					marginF := margin.MustFloat64()
					action := testexchange.ActionCreateOrder{}
					action.ActionType = testexchange.ActionType_derivativeMarketOrder
					ot := testexchange.GetTestOrderType(orderType)
					action.OrderType = &ot
					action.Quantity = quantity.MustFloat64()
					action.Price = price.MustFloat64()
					action.AccountIndex = accountIndex
					action.Margin = &marginF
					action.MarketIndex = marketIdx
					action.IsReduceOnly = isReduceOnly
					action.IsLong = orderType.IsBuy()
					history.Actions = append(history.Actions, &action)
				}
				// fmt.Println("derivative-market", accountIndex, orderType.String(), price.String(), quantity.String(), margin.String())
			} else {
				failedActions[actionId]++
			}
		},
	}

	cancelDerivativeLimitOrderAction := Action{
		name:   "cancel derivative order",
		weight: 100,
		f: func(actionId int) {
			if len(testInput.Perps) == 0 {
				return
			}
			isExpiryMarket := randIntn(&data, numDerivativeMarkets+numExpiryMarkets+1) < numDerivativeMarkets
			marketIndex := randIntn(&data, numDerivativeMarkets)
			marketID := testInput.Perps[marketIndex].MarketID
			if isExpiryMarket {
				marketID = testInput.ExpiryMarkets[marketIndex].MarketID
			}
			isBuy := randIntn(&data, 2) == 0
			market := app.ExchangeKeeper.GetDerivativeMarket(ctx, marketID, true)
			if market == nil {
				return
			}
			orders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, isBuy)
			if len(orders) == 0 {
				return
			}
			order := orders[randIntn(&data, len(orders))]
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			senderAddr := addressBySubAccount[order.OrderInfo.SubaccountId]
			msg := &exchangetypes.MsgCancelDerivativeOrder{
				Sender:       senderAddr.String(),
				MarketId:     marketID.String(),
				SubaccountId: order.OrderInfo.SubaccountId,
				OrderHash:    "0x" + common.Bytes2Hex(order.OrderHash),
			}
			if msg.ValidateBasic() != nil {
				return
			}
			_, err := msgServer.CancelDerivativeOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
				// fmt.Println("cancel-order", order.OrderType, order.Price(), order.GetFillable(), order.GetMargin())
				if history != nil {
					accountIndex := testPlayer.AccountIndexByAddr(senderAddr)
					actionCancelOrder := testexchange.ActionCancelOrder{}
					actionCancelOrder.ActionType = testexchange.ActionType_cancelOrder
					actionCancelOrder.OrderId = common.BytesToHash(order.OrderHash).String()
					actionCancelOrder.AccountIndex = accountIndex
					actionCancelOrder.MarketIndex = 0 // TODO: change
					actionCancelOrder.IsLong = isBuy
					history.Actions = append(history.Actions, &actionCancelOrder)
				}
			} else {
				panic(err)
			}
		},
	}

	// create binaryOptions limit order
	createBinaryOptionsLimitOrderAction := Action{
		name:   "create binaryOptions limit order",
		weight: 300,
		f: func(actionId int) {
			accountIndex := randIntn(&data, numAccounts)

			marketID, marketIdx := findRandomActiveMarketId(&data, exchangetypes.MarketType_BinaryOption, testInput, numBinaryOptionsMarkets)
			if marketID == nil {
				return
			}
			market := app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, *marketID, true)
			if market == nil || market.Status != exchangetypes.MarketStatus_Active {
				return
			}
			scaledOne := exchangetypes.GetScaledPrice(sdk.OneDec(), market.OracleScaleFactor)
			subaccountID := common.HexToHash(accounts[accountIndex].SubaccountIDs[0])
			markPrice := app.ExchangeKeeper.GetDerivativeMidPriceOrBestPrice(ctx, *marketID)
			if markPrice == nil {
				p := exchangetypes.GetScaledPrice(sdk.MustNewDecFromStr("0.3"), market.OracleScaleFactor)
				markPrice = &p
			}

			isBuy := (randIntn(&data, 2) == 0)
			orderType := exchangetypes.OrderType_BUY
			if !isBuy {
				orderType = exchangetypes.OrderType_SELL
			}

			deviationBase, _ := sdk.NewDecFromStr("0.2")
			priceDeviation := exchangetypes.GetScaledPrice(deviationBase, market.OracleScaleFactor)
			minPrice := sdk.MaxDec(markPrice.Sub(priceDeviation), market.MinPriceTickSize)
			maxPrice := sdk.MinDec(markPrice.Add(priceDeviation), scaledOne.Sub(market.MinPriceTickSize))
			//price := matchTick(sdk.NewDec(randX.Int63n(maxPrice.RoundInt64()-minPrice.RoundInt64())).Add(minPrice), market.MinPriceTickSize)
			price := matchTick(sdk.NewDec(randInt63n(&data, maxPrice.RoundInt64()-minPrice.RoundInt64())).Add(minPrice), sdk.NewDec(1000))

			// maxQuantity := int64(100)
			//quantity := sdk.NewDec(randInt63n(maxQuantity))
			quantity := sdk.NewDec(10)
			notional := price.Mul(quantity)

			positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, *marketID)
			existingPositionQuantity := sdk.ZeroDec()
			var existingPositionIsLong bool

			for _, position := range positions {
				if position.SubaccountId == subaccountID.Hex() {
					existingPositionQuantity = position.Position.Quantity
					existingPositionIsLong = position.Position.IsLong
					break
				}
			}

			margin := sdk.Dec{}
			// 30% to be reduce-only orders
			isReduceOnly := (randIntn(&data, 3) == 0) && existingPositionQuantity.IsPositive()

			if isReduceOnly {
				quantity = sdk.MinDec(quantity, existingPositionQuantity)

				if existingPositionIsLong {
					orderType = exchangetypes.OrderType_SELL
				} else {
					orderType = exchangetypes.OrderType_BUY
				}

				orders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, *marketID, isBuy)
				for _, order := range orders {
					isReduceOnlyOrder := order.Margin.IsZero()

					if order.OrderInfo.SubaccountId == subaccountID.Hex() && isReduceOnlyOrder {
						return
					}
				}
			}

			if isReduceOnly {
				margin = sdk.ZeroDec()
			} else if isBuy {
				// For Buys: Margin = price * quantity
				margin = notional
			} else {
				// For Sells: Margin = (1*scaleFactor - price) * quantity
				margin = (exchangetypes.GetScaledPrice(sdk.OneDec(), market.OracleScaleFactor).Sub(price)).Mul(quantity)
			}

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCreateBinaryOptionsLimitOrder{
				Sender: accounts[accountIndex].AccAddress.String(),
				Order: exchangetypes.DerivativeOrder{
					MarketId: marketID.String(),
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID.String(),
						FeeRecipient: FeeRecipient,
						Price:        price,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					Margin:       margin,
					TriggerPrice: nil,
				},
			}

			if err := msg.ValidateBasic(); err != nil {
				failedActions[actionId]++
				return
			}
			_, err := msgServer.CreateBinaryOptionsLimitOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				// fmt.Printf("Order isLong: %v reduce only: %v, denom: %v\n", market.QuoteDenom, isBuy, isReduceOnly)
				// fmt.Printf("{\"price\": %v, \"quantity\": %d, \"margin\": %v, \"subaccountNonce\": %v}\n",
				//	msg.Order.OrderInfo.Price, msg.Order.OrderInfo.Quantity.RoundInt(), msg.Order.Margin.RoundInt(), msg.Order.OrderInfo.SubaccountId[2:7])
				// fmt.Printf("Created order (%v, %v): isLong: %v reduce only: %v, quantity: %v, price: %v, margin: %v \n",
				//	msg.Order.OrderInfo.SubaccountId[2:7], market.QuoteDenom, isBuy, isReduceOnly,
				//	msg.Order.OrderInfo.Quantity.RoundInt(), msg.Order.OrderInfo.Price.RoundInt().QuoRaw(100), msg.Order.Margin.RoundInt().QuoRaw(100))
				successActions[actionId]++
				if history != nil {
					marginF := margin.MustFloat64()
					order := testexchange.ActionCreateOrder{}
					order.ActionType = testexchange.ActionType_boLimitOrder
					order.Quantity = quantity.MustFloat64()
					order.Price = price.MustFloat64()
					order.AccountIndex = accountIndex
					order.Margin = &marginF
					order.MarketIndex = marketIdx
					order.IsReduceOnly = isReduceOnly
					order.IsLong = isBuy
					history.Actions = append(history.Actions, &order)
				}
			} else {
				failedActions[actionId]++
			}
		},
	}

	createBinaryOptionsMarketOrderAction := Action{
		name:   "create binaryOptions market order",
		weight: 150,
		f: func(actionId int) {
			accountIndex := randIntn(&data, numAccounts)

			marketID, marketIdx := findRandomActiveMarketId(&data, exchangetypes.MarketType_BinaryOption, testInput, numBinaryOptionsMarkets)
			if marketID == nil {
				return
			}

			subaccountID := common.HexToHash(accounts[accountIndex].SubaccountIDs[0])
			isBuy := randIntn(&data, 2) == 0
			orderType := exchangetypes.OrderType_BUY
			if !isBuy {
				orderType = exchangetypes.OrderType_SELL
			}
			market := app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, *marketID, true)
			if market == nil {
				return
			}

			price := sdk.NewDec(randInt63n(&data, exchangetypes.GetScaledPrice(sdk.OneDec(), market.OracleScaleFactor).RoundInt64())).QuoInt64(100)
			quantity := sdk.NewDec(randInt63n(&data, 100)).QuoInt64(100)
			notional := price.Mul(quantity)

			margin := sdk.Dec{}

			// 10% to be reduce-only orders
			isReduceOnly := (randIntn(&data, 10) == 0)
			if isReduceOnly {
				margin = sdk.ZeroDec()
			} else if isBuy {
				// For Buys: Margin = price * quantity
				margin = notional
			} else {
				// For Sells: Margin = (1*scaleFactor - price) * quantity
				margin = (exchangetypes.GetScaledPrice(sdk.OneDec(), market.OracleScaleFactor).Sub(price)).Mul(quantity)
			}

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCreateBinaryOptionsMarketOrder{
				Sender: accounts[accountIndex].AccAddress.String(),
				Order: exchangetypes.DerivativeOrder{
					MarketId: marketID.String(),
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID.String(),
						FeeRecipient: FeeRecipient,
						Price:        price,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					Margin:       margin,
					TriggerPrice: nil,
				},
			}
			if err := msg.ValidateBasic(); err != nil {
				return
			}
			_, err := msgServer.CreateBinaryOptionsMarketOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
				if history != nil {
					marginF := margin.MustFloat64()
					order := testexchange.ActionCreateOrder{}
					order.ActionType = testexchange.ActionType_boMarketOrder
					order.Quantity = quantity.MustFloat64()
					order.Price = price.MustFloat64()
					order.AccountIndex = accountIndex
					order.Margin = &marginF
					order.MarketIndex = marketIdx
					order.IsReduceOnly = isReduceOnly
					order.IsLong = isBuy
					history.Actions = append(history.Actions, &order)
				}
			} else {
				failedActions[actionId]++
			}
		},
	}

	cancelBinaryOptionsLimitOrderAction := Action{
		name:   "cancel binaryOptions order",
		weight: 50,
		f: func(actionId int) {
			marketID, _ := findRandomActiveMarketId(&data, exchangetypes.MarketType_BinaryOption, testInput, numBinaryOptionsMarkets)
			if marketID == nil {
				return
			}
			isBuy := (randIntn(&data, 2) == 0)
			market := app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, *marketID, true)
			if market == nil {
				return
			}
			orders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, *marketID, isBuy)
			if len(orders) == 0 {
				return
			}
			order := orders[randIntn(&data, len(orders))]
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			msg := &exchangetypes.MsgCancelBinaryOptionsOrder{
				Sender:       addressBySubAccount[order.OrderInfo.SubaccountId].String(),
				MarketId:     marketID.String(),
				SubaccountId: order.OrderInfo.SubaccountId,
				OrderHash:    "0x" + common.Bytes2Hex(order.OrderHash),
			}
			if msg.ValidateBasic() != nil {
				return
			}
			_, err := msgServer.CancelBinaryOptionsOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				panic(err)
			}
		},
	}

	updateBinaryOptionsMarketProposalAction := Action{
		name:   "BinaryOptions market update proposal",
		weight: 5,
		f: func(actionId int) {
			marketID, _ := findRandomActiveMarketId(&data, exchangetypes.MarketType_BinaryOption, testInput, numBinaryOptionsMarkets)
			if marketID == nil {
				return
			}

			market := app.ExchangeKeeper.GetBinaryOptionsMarketByID(ctx, *marketID)
			makerFeeRate := genNullableMakerFeeRate(&data)
			takerFeeRate := genNullableTakerFeeRate(&data, makerFeeRate)

			expirationTimestamp := ctx.BlockTime().Unix() + int64(randIntn(&data, 3600*24*28))
			settlementTimestamp := expirationTimestamp + int64(randIntn(&data, 1000))
			shouldDemolishMarket := randIntn(&data, 25) == 0 // demolish in 4% of cases
			var settlementPrice *sdk.Dec = nil
			var marketStatus = exchangetypes.MarketStatus_Unspecified
			if shouldDemolishMarket {
				marketStatus = exchangetypes.MarketStatus_Demolished
				pr := sdk.NewDec(randInt63n(&data, int64(market.OracleScaleFactor)) / int64(market.OracleScaleFactor))
				settlementPrice = &pr
			}

			p := &exchangetypes.BinaryOptionsMarketParamUpdateProposal{
				Title:               "BinaryOptions market param update",
				Description:         "BinaryOptions market param update",
				MarketId:            marketID.String(),
				MakerFeeRate:        makerFeeRate,
				TakerFeeRate:        takerFeeRate,
				RelayerFeeShareRate: genNullableFeeRate(&data),
				MinPriceTickSize:    genNullableTickSize(&data),
				MinQuantityTickSize: genNullableTickSize(&data),
				SettlementPrice:     settlementPrice,
				ExpirationTimestamp: expirationTimestamp,
				SettlementTimestamp: settlementTimestamp,
				Status:              marketStatus,
			}
			err := p.ValidateBasic()
			if err != nil {
				failedActions[actionId]++
				return
			}
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			ctxCached, writeCache := ctx.CacheContext()
			err = handler(ctxCached, p)
			if err == nil {
				writeCache()
				successActions[actionId]++
				if marketStatus != exchangetypes.MarketStatus_Active && marketStatus != exchangetypes.MarketStatus_Unspecified {
					var marketIndex = -1
					for idx, mrkt := range testInput.BinaryMarkets {
						if mrkt.MarketID == *marketID {
							marketIndex = idx
							break
						}
					}
					testInput.BinaryMarkets[marketIndex].IsActive = false
				}
			} else {
				failedActions[actionId]++
			}
		},
	}

	runMsgPrivilegedExecuteContractWasmSubscribeAction := Action{
		name:   "run msg exec with subscribe",
		weight: 250,
		f: func(actionId int) {
			if len(testInput.Perps) == 0 {
				return
			}

			accountIndex := randIntn(&data, numAccounts)
			sender := accounts[accountIndex].AccAddress.String()

			subscriberSubaccountId := common.HexToHash(accounts[accountIndex].SubaccountIDs[0])
			subscriberAddr := exchangetypes.SubaccountIDToSdkAddress(subscriberSubaccountId)

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

			subscribe := testexchange.VaultSubscribe{}
			forwardMsg := testexchange.VaultSubscribeRedeem{
				Subscribe: &subscribe,
			}
			vaultInput := testexchange.VaultInput{
				VaultSubaccountId:  vaultSubaccountId,
				TraderSubaccountId: subscriberSubaccountId.Hex(),
				Msg:                forwardMsg,
			}

			execData := wasmxtypes.ExecutionData{
				Origin: sender,
				Name:   "VaultSubscribe",
				Args:   vaultInput,
			}
			execDataBytes, err := json.Marshal(execData)
			testexchange.OrFail(err)

			market := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, testInput.Perps[0].MarketID)

			if market.Status != exchangetypes.MarketStatus_Active {
				return
			}

			denom := testInput.Perps[0].QuoteDenom

			balance := app.BankKeeper.GetBalance(ctx, subscriberAddr, denom)

			// passing in too low values will result in vault query reverting
			if balance.Amount.LT(sdk.NewInt(200)) {
				return
			}

			msg := &exchangetypes.MsgPrivilegedExecuteContract{
				Sender:          sender,
				Funds:           balance.String(),
				ContractAddress: masterContractAddress,
				Data:            string(execDataBytes),
			}

			if msg.ValidateBasic() != nil {
				panic(msg.ValidateBasic())
			}
			_, err = msgServer.PrivilegedExecuteContract(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				panic(err)
			}
		},
	}

	// msg exec with position transfer
	// runMsgPrivilegedExecuteContractWasmRedeemAction := Action{
	// 	name:   "run msg exec with redeem",
	// 	weight: 120,
	// 	f: func(actionId int) {
	// 		if len(testInput.Perps) == 0 {
	// 			return
	// 		}
	// 		accountIndex := randIntn(&data, numAccounts)
	// 		sender := accounts[accountIndex].AccAddress
	// 		lpDenom := fmt.Sprintf("factory/%v/lp%v", masterContractAddress, VaultContractAddress)
	// 		lpTokenBalance := app.BankKeeper.GetBalance(ctx, sender, lpDenom)
	// 		if !lpTokenBalance.Amount.IsPositive() {
	// 			return
	// 		}

	// 		redeemerSubaccountId := common.HexToHash(accounts[accountIndex].SubaccountIDs[0])

	// 		ctxCached, writeCache := ctx.CacheContext()
	// 		msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	// 		// currently only market 0 supports wasm, so we don't take it on random
	// 		market := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, testInput.Perps[0].MarketID)

	// 		if market.Status != exchangetypes.MarketStatus_Active {
	// 			return
	// 		}

	// 		lpTokenBurnAmount := sdk.NewInt(randInt63n(&data, lpTokenBalance.Amount.Int64()))

	// 		vaultRedeemArgs := testexchange.VaultRedeemArgs{
	// 			VaultSubaccountId:    vaultSubaccountId,
	// 			RedeemerSubaccountId: redeemerSubaccountId.Hex(),
	// 		}
	// 		vaultRedeem := testexchange.VaultRedeem{
	// 			RedeemArgs: vaultRedeemArgs,
	// 		}
	// 		vaultInput := testexchange.VaultInput{
	// 			Subscribe: nil,
	// 			Redeem:    &vaultRedeem,
	// 		}

	// 		execData := wasmxtypes.ExecutionData{
	// 			Origin: sender.String(),
	// 			Name:   "VaultRedeem",
	// 			Args:   vaultInput,
	// 		}
	// 		execDataBytes, err := json.Marshal(execData)
	// 		testexchange.OrFail(err)

	// 		msg := &exchangetypes.MsgPrivilegedExecuteContract{
	// 			Sender:          sender.String(),
	// 			Funds:           sdk.NewCoins(sdk.NewCoin(lpDenom, lpTokenBurnAmount)).String(),
	// 			ContractAddress: masterContractAddress,
	// 			Data:            string(execDataBytes),
	// 		}

	// 		if msg.ValidateBasic() != nil {
	// 			panic(msg.ValidateBasic())
	// 		}
	// 		_, err = msgServer.PrivilegedExecuteContract(
	// 			sdk.WrapSDKContext(ctxCached),
	// 			msg,
	// 		)
	// 		if err == nil {
	// 			writeCache()
	// 			successActions[actionId]++
	// 		} else {
	// 			panic(err)
	// 		}
	// 	},
	// }

	addStakeAmount := Action{
		name:   "add staking amount",
		weight: 20,
		f: func(actionId int) {
			srcAccIndex := randIntn(&data, numAccounts)

			stakingTraderCoins := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100)))
			err := app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, accounts[srcAccIndex].AccAddress, stakingTraderCoins)
			testexchange.OrFail(err)

			msg := &stakingtypes.MsgDelegate{
				DelegatorAddress: accounts[srcAccIndex].AccAddress.String(),
				ValidatorAddress: testexchange.DefaultValidatorAddress,
				Amount:           sdk.NewCoin("inj", sdk.NewInt(100)),
			}

			if msg.ValidateBasic() != nil {
				return
			}

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := stakingkeeper.NewMsgServerImpl(app.StakingKeeper)
			if _, err := msgServer.Delegate(sdk.WrapSDKContext(ctxCached), msg); err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				panic(err)
			}
		},
	}

	addDepositAction := Action{
		name:   "add more deposit from account",
		weight: 75,
		f: func(actionId int) {
			srcAccIndex := randIntn(&data, numAccounts)
			address := accounts[srcAccIndex].AccAddress
			balance := app.BankKeeper.GetAllBalances(ctx, address)

			amountToDeposit, err := randomFees(&data, ctx, balance)
			if err != nil || amountToDeposit == nil || amountToDeposit.Empty() {
				return
			}
			msg := &exchangetypes.MsgDeposit{
				Sender:       address.String(),
				SubaccountId: accounts[srcAccIndex].SubaccountIDs[0],
				Amount:       amountToDeposit[randIntn(&data, len(amountToDeposit))],
			}

			if msg.ValidateBasic() != nil {
				return
			}
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			if _, err := msgServer.Deposit(sdk.WrapSDKContext(ctxCached), msg); err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				panic(err)
			}
		},
	}

	withdrawDepositAction := Action{
		name:   "try withdraw from account",
		weight: 75,
		f: func(actionId int) {
			srcAccIndex := randIntn(&data, numAccounts)
			address := accounts[srcAccIndex].AccAddress
			subAcccountID := accounts[srcAccIndex].SubaccountIDs[0]

			denom := initCoins[randIntn(&data, len(initCoins))].Denom
			deposit := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subAcccountID), denom)
			availableBalance := int64(0)
			if deposit != nil {
				availableBalance = deposit.AvailableBalance.RoundInt64()
			}

			if availableBalance == int64(0) {
				return
			}

			if availableBalance < int64(0) {
				// can happen due to matchedFeePriceDeltaRefundOr*Charge*
				fmt.Fprintln(GinkgoWriter, "[WARN] Negative availableBalance", availableBalance)
				return
			}

			amountToWithdraw := randInt63n(&data, availableBalance)
			msg := &exchangetypes.MsgWithdraw{
				Sender:       address.String(),
				SubaccountId: accounts[srcAccIndex].SubaccountIDs[0],
				Amount:       sdk.NewInt64Coin(denom, amountToWithdraw),
			}

			if msg.ValidateBasic() != nil {
				return
			}
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			if _, err := msgServer.Withdraw(sdk.WrapSDKContext(ctxCached), msg); err == nil {
				writeCache()
				successActions[actionId]++
			}
		},
	}

	externalTransferAction := Action{
		name:   "external transfer",
		weight: 30,
		f: func(actionId int) {
			srcAccIndex := randIntn(&data, numAccounts)
			destAccIndex := randIntn(&data, numAccounts)
			transferAmount := sdk.NewInt64Coin(initCoins[randIntn(&data, len(initCoins))].Denom, randInt63n(&data, 1000000))

			msg := &exchangetypes.MsgExternalTransfer{
				Sender:                  accounts[srcAccIndex].AccAddress.String(),
				SourceSubaccountId:      accounts[srcAccIndex].SubaccountIDs[0],
				DestinationSubaccountId: accounts[destAccIndex].SubaccountIDs[0],
				Amount:                  transferAmount,
			}

			if msg.ValidateBasic() != nil {
				return
			}
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			if _, err := msgServer.ExternalTransfer(sdk.WrapSDKContext(ctxCached), msg); err == nil {
				writeCache()
				successActions[actionId]++
			}
		},
	}

	subaccountTransferAction := Action{
		name:   "subaccount transfer",
		weight: 20,
		f: func(actionId int) {
			accIndex := randIntn(&data, numAccounts)
			denom := initCoins[randIntn(&data, len(initCoins))].Denom

			var (
				srcSubaccountIndex int
				availableBalance   sdk.Dec
			)

			subaccountsIdx := randPerm(&data, numSubaccounts)

			for subAccIdx := range subaccountsIdx {
				subId := common.HexToHash(accounts[accIndex].SubaccountIDs[subAccIdx])
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subId, denom)

				if deposit.AvailableBalance.IsPositive() {
					srcSubaccountIndex = subAccIdx
					availableBalance = deposit.AvailableBalance
					break
				}
			}

			if availableBalance.IsNil() || availableBalance.IsZero() { // we couldn't find account with balance
				return
			}

			destSubaccountIndex := testexchange.RandomUniqueNumber(srcSubaccountIndex, numSubaccounts)
			sourceSubaccountId := accounts[accIndex].SubaccountIDs[srcSubaccountIndex]
			senderSubaccountBalance := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(sourceSubaccountId), denom)

			maxAmount := sdk.MinInt(senderSubaccountBalance.AvailableBalance.RoundInt(), sdk.NewIntFromUint64(1000000))
			if maxAmount.Int64() <= 0 {
				return // not enough balance
			}

			transferAmount := sdk.NewInt64Coin(denom, randInt63n(&data, maxAmount.Int64()))

			msg := &exchangetypes.MsgSubaccountTransfer{
				Sender:                  accounts[accIndex].AccAddress.String(),
				SourceSubaccountId:      accounts[accIndex].SubaccountIDs[srcSubaccountIndex],
				DestinationSubaccountId: accounts[accIndex].SubaccountIDs[destSubaccountIndex],
				Amount:                  transferAmount,
			}

			if err := msg.ValidateBasic(); err != nil {
				return
			}

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			if _, err := msgServer.SubaccountTransfer(sdk.WrapSDKContext(ctxCached), msg); err == nil {
				writeCache()
				successActions[actionId]++
			}
		},
	}

	endBlockerAction := Action{
		name: "run endblocker and beginblocker",
		//weight: 30,
		weight: 250,
		f: func(actionId int) {
			// fmt.Println("Run endblocker...")
			// block time is from 250 to 250+50 to add randomness
			avgBlockInterval := 250
			intervalOffset := 50
			// funding interval is 3600s and when this action is selected for 36th, funding interval time comes
			randomInterval := randIntn(&data, intervalOffset)
			interval := time.Second * time.Duration(avgBlockInterval+randomInterval)

			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
			ctx = ctx.WithBlockTime(ctx.BlockTime().Add(interval))
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			successActions[actionId]++
			// TODO: should check expiry for TEF markets when it's merged into master
			// fmt.Println("endblocker + beginblocker")
			if history != nil {
				actionHistory := testexchange.ActionEndBlocker{}
				actionHistory.ActionType = testexchange.ActionType_endblocker
				actionHistory.BlockInterval = randomInterval
				history.Actions = append(history.Actions, &actionHistory)
			}
		},
	}

	liquidatePositionAction := Action{
		name:   "try liquidate position",
		weight: 100,
		f: func(actionId int) {
			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			isExpiryMarket := randIntn(&data, 2) == 0

			// it's very difficult to find any position suitable for liquidation, so it's necessary to check more than one random market
			for idx := 0; idx < numDerivativeMarkets; idx++ {
				var marketID common.Hash

				if isExpiryMarket {
					if !testInput.Perps[idx].IsActive {
						continue
					}
					marketID = testInput.Perps[idx].MarketID
				} else {
					if !testInput.ExpiryMarkets[idx].IsActive {
						continue
					}
					marketID = testInput.ExpiryMarkets[idx].MarketID
				}

				market, markPrice := app.ExchangeKeeper.GetDerivativeMarketWithMarkPrice(ctx, marketID, true)
				if market == nil {
					continue
				}

				positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, marketID)
				if len(positions) == 0 {
					continue
				}

				funding := app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, marketID)
				for _, position := range positions {
					p := position.Position
					liquidationPrice := p.GetLiquidationPrice(market.MaintenanceMarginRatio, funding)
					shouldLiquidate := p.IsLong && markPrice.LTE(liquidationPrice) || p.IsShort() && markPrice.GTE(liquidationPrice)

					if shouldLiquidate {
						liquidateMsg := testInput.NewMsgLiquidatePositionForMarketID(common.HexToHash(position.SubaccountId), common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000"), common.HexToHash(position.MarketId))
						if _, err := msgServer.LiquidatePosition(sdk.WrapSDKContext(ctxCached), liquidateMsg); err == nil {
							writeCache()
							successActions[actionId]++
							return // one liquidation is enough
						}
					}
				}
			}
		},
	}

	updateDerivativeMarketProposalAction := Action{
		name:   "derivative market update proposal",
		weight: 50,
		f: func(actionId int) {
			isExpiryMarket := (randIntn(&data, 2) == 0)
			if numDerivativeMarkets == 0 {
				return
			}
			marketIndex := randIntn(&data, numDerivativeMarkets)
			marketID := testInput.Perps[marketIndex].MarketID

			maintenanceMarginRatio := genNullableFeeRate(&data)
			initialMarginRatio := genNullableInitialMarginRatio(&data, maintenanceMarginRatio)

			makerFeeRate := genNullableMakerFeeRate(&data)
			// makerFee := testInput.Perps[marketIndex].MakerFeeRate
			// makerFeeRate := &makerFee
			takerFeeRate := genNullableTakerFeeRate(&data, makerFeeRate)

			hourlyInterestRate := genNullableHourlyInterestRate(&data)
			hourlyFundingRateCap := genNullableFundingCap(&data)
			if isExpiryMarket {
				hourlyInterestRate = nil
				hourlyFundingRateCap = nil
			}

			relayerFeeShareRate := genNullableFeeRate(&data)
			minPriceTickSize := genNullableTickSize(&data)
			minQuantityTickSize := genNullableTickSize(&data)

			status := genMarketStatus(&data, 50)

			p := &exchangetypes.DerivativeMarketParamUpdateProposal{
				Title:                  "Derivative market param update",
				Description:            "Derivative market param update",
				MarketId:               marketID.String(),
				InitialMarginRatio:     initialMarginRatio,
				MaintenanceMarginRatio: maintenanceMarginRatio,
				MakerFeeRate:           makerFeeRate,
				TakerFeeRate:           takerFeeRate,
				RelayerFeeShareRate:    relayerFeeShareRate,
				MinPriceTickSize:       minPriceTickSize,
				MinQuantityTickSize:    minQuantityTickSize,
				HourlyInterestRate:     hourlyInterestRate,
				HourlyFundingRateCap:   hourlyFundingRateCap,
				Status:                 status,
			}
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			ctxCached, writeCache := ctx.CacheContext()
			err := handler(ctxCached, p)

			if err == nil {
				writeCache()
				successActions[actionId]++
				if status != exchangetypes.MarketStatus_Active && status != exchangetypes.MarketStatus_Unspecified {
					if !isExpiryMarket {
						testInput.Perps[marketIndex].IsActive = false
					} else {
						testInput.ExpiryMarkets[marketIndex].IsActive = false
					}
				}
			} else {
				failedActions[actionId]++
			}
			if history != nil {
				var marketType testexchange.MarketType
				if isExpiryMarket {
					marketType = testexchange.MarketType_expiry
				} else {
					marketType = testexchange.MarketType_derivative
				}
				action := testexchange.ActionUpdateMarketParams{
					TestActionBase: testexchange.TestActionBase{
						ActionType: testexchange.ActionType_updateMarket,
					},
					TestActionWithMarket: testexchange.TestActionWithMarket{
						MarketIndex: marketIndex,
						MarketType:  &marketType,
					},
					MaintenanceMarginRatio: nd2nf(maintenanceMarginRatio),
					InitialMarginRatio:     nd2nf(initialMarginRatio),
					MakerFeeRate:           nd2nf(makerFeeRate),
					TakerFeeRate:           nd2nf(takerFeeRate),
					HourlyInterestRate:     nd2nf(hourlyInterestRate),
					HourlyFundingRateCap:   nd2nf(hourlyFundingRateCap),
					RelayerFeeShareRate:    nd2nf(relayerFeeShareRate),
					MinPriceTickSize:       nd2nf(minPriceTickSize),
					MinQuantityTickSize:    nd2nf(minQuantityTickSize),
					MarketStatus:           &status,
				}
				history.Actions = append(history.Actions, &action)
			}
		},
	}

	addTradingRewardCampaignProposal := Action{
		name:   "add trading reward campaign proposal",
		weight: 4,
		f: func(actionId int) {
			isExpiryMarket := randIntn(&data, 2) == 0
			var marketType exchangetypes.MarketType
			if isExpiryMarket {
				marketType = exchangetypes.MarketType_Expiry
			} else {
				marketType = exchangetypes.MarketType_Perpetual
			}
			disqualifiedMarketID, _ := findRandomActiveMarketId(&data, marketType, testInput, numDerivativeMarkets)
			if disqualifiedMarketID == nil {
				return
			}

			quoteDenoms := make([]string, 0)
			for i := 0; i < numDerivativeMarkets; i++ {
				quoteDenoms = append(quoteDenoms, testInput.Perps[i].QuoteDenom)
				quoteDenoms = append(quoteDenoms, testInput.ExpiryMarkets[i].QuoteDenom)
			}

			boostedSpotMarketIds := make([]string, 0)
			boostedSpotMarketMultipliers := make([]exchangetypes.PointsMultiplier, 0)
			boostedDerivativeMarketIds := make([]string, 0)
			boostedDerivativeMarketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

			for i := 0; i < len(testInput.Spots)/2; i++ {
				boostedSpotMarketIds = append(boostedSpotMarketIds, testInput.Spots[i].MarketID.Hex())
				boostedSpotMarketMultipliers = append(boostedSpotMarketMultipliers, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(-1),
					TakerPointsMultiplier: sdk.NewDec(3),
				})
			}

			for i := 0; i < len(testInput.Perps)/2; i++ {
				boostedDerivativeMarketIds = append(boostedDerivativeMarketIds, testInput.Perps[i].MarketID.Hex())
				boostedDerivativeMarketMultipliers = append(boostedDerivativeMarketMultipliers, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(2),
					TakerPointsMultiplier: sdk.NewDec(3),
				})
			}
			for i := 0; i < len(testInput.ExpiryMarkets)/2; i++ {
				boostedDerivativeMarketIds = append(boostedDerivativeMarketIds, testInput.ExpiryMarkets[i].MarketID.Hex())
				boostedDerivativeMarketMultipliers = append(boostedDerivativeMarketMultipliers, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(2),
					TakerPointsMultiplier: sdk.NewDec(3),
				})
			}

			isUsingDifferentCampaignDuration := (randIntn(&data, 2) == 0)
			campaignDurationSeconds := int64(12000)

			if isUsingDifferentCampaignDuration {
				minCampaignDurationSeconds := int64(7000)
				maxCampaignDurationSeconds := int64(31000)
				campaignDurationSeconds = randInt63n(&data, maxCampaignDurationSeconds-minCampaignDurationSeconds+1) + minCampaignDurationSeconds
			}

			proposal := &exchangetypes.TradingRewardCampaignLaunchProposal{
				Title:       "Trade Reward Campaign",
				Description: "Trade Reward Campaign",
				CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: campaignDurationSeconds,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:        boostedSpotMarketIds,
						SpotMarketMultipliers:       boostedSpotMarketMultipliers,
						BoostedDerivativeMarketIds:  boostedDerivativeMarketIds,
						DerivativeMarketMultipliers: boostedDerivativeMarketMultipliers,
					},
					DisqualifiedMarketIds: []string{disqualifiedMarketID.Hex()},
				},
				CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
					StartTimestamp: ctx.BlockTime().Unix() + 1500,
					MaxCampaignRewards: sdk.NewCoins(
						sdk.NewCoin("inj0", sdk.NewInt(100_000)),
						sdk.NewCoin("inj1", sdk.NewInt(100_000)),
						sdk.NewCoin("inj2", sdk.NewInt(100_000)),
					),
				}},
			}

			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			ctxCached, writeCache := ctx.CacheContext()
			proposal.ValidateBasic()
			err := handler(ctxCached, proposal)
			if err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				failedActions[actionId]++
				// fmt.Printf("trading reward error:  %v \n", err.Error())

			}
		},
	}

	addFeeDiscountProposal := Action{
		name:   "add fee discount proposal",
		weight: 4,
		f: func(actionId int) {
			isExpiryMarket := (randIntn(&data, 2) == 0)
			if numDerivativeMarkets == 0 {
				return
			}
			disqualifiedMarketIndex := randIntn(&data, numDerivativeMarkets)
			disqualifiedMarketID := testInput.Perps[disqualifiedMarketIndex].MarketID
			if isExpiryMarket {
				disqualifiedMarketID = testInput.ExpiryMarkets[disqualifiedMarketIndex].MarketID
			}

			quoteDenoms := make([]string, 0)
			for i := 0; i < numDerivativeMarkets; i++ {
				quoteDenoms = append(quoteDenoms, testInput.Perps[i].QuoteDenom)
				quoteDenoms = append(quoteDenoms, testInput.ExpiryMarkets[i].QuoteDenom)
			}

			isUsingDifferentBucketData := (randIntn(&data, 2) == 0)
			bucketCount := uint64(10)
			bucketDuration := int64(1500)

			if isUsingDifferentBucketData {
				minBucketCount := int64(6)
				maxBucketCount := int64(70)
				bucketCount = uint64(randInt63n(&data, maxBucketCount-minBucketCount+1)) + uint64(minBucketCount)

				minBucketDuration := int64(750)
				maxBucketDuration := int64(3100)
				bucketDuration = randInt63n(&data, maxBucketDuration-minBucketDuration+1) + minBucketDuration
			}

			proposal := &exchangetypes.FeeDiscountProposal{
				Title:       "Fee Discount",
				Description: "Fee Discount",
				Schedule: &exchangetypes.FeeDiscountSchedule{
					BucketCount:    bucketCount,
					BucketDuration: bucketDuration,
					QuoteDenoms:    quoteDenoms,
					TierInfos: []*exchangetypes.FeeDiscountTierInfo{
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.01"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.01"),
							StakedAmount:      sdk.NewInt(100),
							Volume:            sdk.MustNewDecFromStr("0.3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.02"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.02"),
							StakedAmount:      sdk.NewInt(1000),
							Volume:            sdk.MustNewDecFromStr("3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.03"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.03"),
							StakedAmount:      sdk.NewInt(3000),
							Volume:            sdk.MustNewDecFromStr("10"),
						},
					},
					DisqualifiedMarketIds: []string{disqualifiedMarketID.Hex()},
				},
			}

			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			ctxCached, writeCache := ctx.CacheContext()
			err := handler(ctxCached, proposal)
			if err == nil {
				writeCache()
				successActions[actionId]++
			} else {
				//  may fail with new fee discounts for negative maker fees
				failedActions[actionId]++
			}
		},
	}

	increaseMarginAction := Action{
		name:   "try increasing margin position",
		weight: 100,
		f: func(actionId int) {
			if len(testInput.Perps) == 0 {
				return
			}
			srcAccIndex := randIntn(&data, numAccounts)
			destAccIndex := randIntn(&data, numAccounts)
			isExpiryMarket := (randIntn(&data, 2) == 0)
			marketIndex := randIntn(&data, numDerivativeMarkets)
			marketID := testInput.Perps[marketIndex].MarketID
			if isExpiryMarket {
				marketID = testInput.ExpiryMarkets[marketIndex].MarketID
			}
			marginToAdd := sdk.NewDec(randInt63n(&data, 1000000))
			msg := &exchangetypes.MsgIncreasePositionMargin{
				Sender:                  accounts[srcAccIndex].AccAddress.String(),
				MarketId:                marketID.String(),
				SourceSubaccountId:      accounts[srcAccIndex].SubaccountIDs[0],
				DestinationSubaccountId: accounts[destAccIndex].SubaccountIDs[0],
				Amount:                  marginToAdd,
			}

			if msg.ValidateBasic() != nil {
				return
			}

			ctxCached, writeCache := ctx.CacheContext()
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			if _, err := msgServer.IncreasePositionMargin(sdk.WrapSDKContext(ctxCached), msg); err == nil {
				writeCache()
				successActions[actionId]++
			}
		},
	}

	actions = append(actions, createSpotLimitOrderAction)
	actions = append(actions, createSpotMarketOrderAction)
	actions = append(actions, cancelSpotLimitOrderAction)
	actions = append(actions, updateSpotMarketProposalAction)

	actions = append(actions, setOraclePricesAction)
	actions = append(actions, createDerivativeLimitOrderAction)
	actions = append(actions, createDerivativeMarketOrderAction)
	actions = append(actions, cancelDerivativeLimitOrderAction)
	actions = append(actions, closeExpiryMarketAction)
	actions = append(actions, liquidatePositionAction)
	actions = append(actions, updateDerivativeMarketProposalAction)

	actions = append(actions, createBinaryOptionsLimitOrderAction)
	actions = append(actions, createBinaryOptionsMarketOrderAction)
	actions = append(actions, cancelBinaryOptionsLimitOrderAction)
	actions = append(actions, updateBinaryOptionsMarketProposalAction)

	actions = append(actions, addTradingRewardCampaignProposal)
	actions = append(actions, addFeeDiscountProposal)
	actions = append(actions, addStakeAmount)
	//actions = append(actions, runMsgPrivilegedExecuteContractWasmSubscribeAction)
	//actions = append(actions, runMsgPrivilegedExecuteContractWasmRedeemAction)

	actions = append(actions, increaseMarginAction)
	actions = append(actions, addDepositAction)
	actions = append(actions, withdrawDepositAction)
	actions = append(actions, externalTransferAction)
	actions = append(actions, subaccountTransferAction)

	actions = append(actions, endBlockerAction)

	totalActionWeights := 0
	for _, action := range actions {
		if action.weight == 0 {
			panic(fmt.Sprintf("action with empty weight exists: action_name = %s", action.name))
		}
		totalActionWeights += action.weight
	}

	isEndBlocker := false
	for i := 0; i < numOperations; i++ {
		actionIndex := 0
		random := randIntn(&data, totalActionWeights)
		for index, action := range actions {
			random -= action.weight
			if random <= 0 {
				actionIndex = index
				break
			}
		}
		if actionIndex >= len(actions) {
			continue
		}

		// fmt.Println("actionIndex", actionIndex)
		// fmt.Println("action: ", actions[actionIndex].name)

		action := actions[actionIndex]
		if action.name == endBlockerAction.name {
			if isEndBlocker {
				continue
			} else {
				isEndBlocker = true
			}
		} else {
			isEndBlocker = false
		}
		// fmt.Printf("Action %d: %s\n", i, action.name)
		action.f(actionIndex)

		// fmt.Println("actionIndex Finished", actionIndex)
		numOperationsByAction[actionIndex]++
		if isEndBlocker {
			testPlayer.InvariantChecker1(action.name)
			testPlayer.InvariantChecker2(action.name)
			testPlayer.InvariantChecker3(action.name)
			testPlayer.InvariantChecker4(action.name)
			testPlayer.InvariantChecker5(action.name)
			testPlayer.InvariantChecker6(action.name)
		}
		testPlayer.InvariantCheckOrderbookLevels(action.name)
	}

	endBlockerAction.f(0)

	fmt.Fprintf(GinkgoWriter, "\n\n\nPerformed actions report:\n")
	// report operations per action
	actionIdxs := make([]int, 0)
	for k := range numOperationsByAction {
		actionIdxs = append(actionIdxs, k)
	}
	sort.Ints(actionIdxs)
	for idx := range actionIdxs {
		value := numOperationsByAction[idx]
		fmt.Fprintf(GinkgoWriter, "%s: %d (failed: %d, attempts: %d) \n", actions[idx].name, successActions[idx], failedActions[idx], value)
	}

	fmt.Fprintln(GinkgoWriter, "\ninitialBlockTime", testPlayer.InitialBlockTime)
	fmt.Fprintln(GinkgoWriter, "initialBlockHeight", testPlayer.InitialBlockHeight)
	fmt.Fprintln(GinkgoWriter, "finalBlockTime", ctx.BlockTime())
	fmt.Fprintln(GinkgoWriter, "finalBlockHeight", ctx.BlockHeight())

	// fmt.Printf("Create market order - success: %d\n", successRuns)
	// fmt.Printf("Create binary option order - validation failure: %d\n", basicValidationFailure)
	// fmt.Printf("Create binary option order - markets not found count: %d\n", marketNotFoundCounter)
	// fmt.Printf("Create market order - no liquidity: %d\n", noLiquidityCounter)
	// fmt.Printf("Create market order - other error: %d\n", otherError)
	// fmt.Printf("Create market order - total runs %d\n", createOrderRun)

	testPlayer.InvariantChecker1("-1")
	testPlayer.InvariantChecker2("-1")
	testPlayer.InvariantChecker3("-1")
	testPlayer.InvariantChecker5("-1")
	testPlayer.InvariantChecker6("-1")
	testPlayer.InvariantChecker4("-1")
	testPlayer.InvariantCheckOrderbookLevels("-1")

	testexchange.TestingExchangeParams.DefaultHourlyFundingRateCap = sdk.NewDecWithPrec(625, 6)

	_ = createSpotLimitOrderAction
	_ = createSpotMarketOrderAction
	_ = cancelSpotLimitOrderAction
	_ = updateSpotMarketProposalAction

	_ = setOraclePricesAction
	_ = createDerivativeLimitOrderAction
	_ = createDerivativeMarketOrderAction
	_ = cancelDerivativeLimitOrderAction
	_ = closeExpiryMarketAction
	_ = liquidatePositionAction
	_ = updateDerivativeMarketProposalAction

	_ = setOraclePricesAction
	_ = subaccountTransferAction
	_ = withdrawDepositAction
	_ = addDepositAction
	_ = externalTransferAction
	_ = addStakeAmount
	_ = runMsgPrivilegedExecuteContractWasmSubscribeAction
	_ = createBinaryOptionsLimitOrderAction
	_ = createBinaryOptionsMarketOrderAction
	_ = cancelBinaryOptionsLimitOrderAction
	_ = updateBinaryOptionsMarketProposalAction
	_ = addTradingRewardCampaignProposal
	_ = increaseMarginAction
	_ = addFeeDiscountProposal
}

// randomFees returns a random fee by selecting a random coin denomination and
// amount from the account's available balance. If the user doesn't have enough
// funds for paying fees, it returns empty coins.
func randomFees(data *[]byte, ctx sdk.Context, spendableCoins sdk.Coins) (sdk.Coins, error) {
	if spendableCoins.Empty() {
		return nil, nil
	}

	perm := randPerm(data, len(spendableCoins))
	var randCoin sdk.Coin
	for _, index := range perm {
		randCoin = spendableCoins[index]
		if !randCoin.Amount.IsZero() {
			break
		}
	}

	if randCoin.Amount.IsZero() {
		return nil, fmt.Errorf("no coins found for random fees")
	}

	amt := randInt63n(data, int64(randCoin.Amount.Uint64()))

	// Create a random fee and verify the fees are within the account's spendable
	// balance.
	fees := sdk.NewCoins(sdk.NewCoin(randCoin.Denom, sdk.NewInt(amt)))

	return fees, nil
}

func randPerm(data *[]byte, n int) []int {
	m := make([]int, n)
	// In the following loop, the iteration when i=0 always swaps m[0] with m[0].
	// A change to remove this useless iteration is to assign 1 to i in the init
	// statement. But Perm also effects r. Making this change will affect
	// the final state of r. So this change can't be made for compatibility
	// reasons for Go 1.
	for i := 0; i < n; i++ {
		j := randIntn(data, i+1)
		m[i] = m[j]
		m[j] = i
	}
	return m
}
