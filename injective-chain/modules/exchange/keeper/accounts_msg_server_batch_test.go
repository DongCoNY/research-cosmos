package keeper_test

import (
	"io/ioutil"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	log "github.com/xlab/suplog"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
)

var _ = Describe("Accounts MsgServer Batch Tests", func() {
	log.DefaultLogger.SetOutput(ioutil.Discard)

	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context
		msgServer exchangetypes.MsgServer
		err       error

		buyer  = testexchange.SampleSubaccountAddr1
		seller = testexchange.SampleSubaccountAddr2
		admin  = common.HexToHash("inj1wfawuv6fslzjlfa4v7exv27mk6rpfeyvhvxchc")
	)

	var (
		res          *exchangetypes.MsgBatchUpdateOrdersResponse
		subaccountID string
		message      *exchangetypes.MsgBatchUpdateOrders
		balancesUsed map[string]sdk.Dec
		deposits     map[string]*exchangetypes.Deposit
	)

	var addNewOrders = func() {
		for i := 0; i < 5; i++ {
			initialCoinDeposit := sdk.NewInt(50000000)
			spotCoinQuote := sdk.NewCoin(testInput.Spots[i].QuoteDenom, initialCoinDeposit)
			spotCoinBase := sdk.NewCoin(testInput.Spots[i].BaseDenom, initialCoinDeposit)
			derivativeCoin := sdk.NewCoin(testInput.Perps[i].QuoteDenom, initialCoinDeposit)
			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(spotCoinQuote.Add(derivativeCoin), spotCoinBase))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(spotCoinQuote.Add(derivativeCoin), spotCoinBase))
			testexchange.MintAndDeposit(app, ctx, admin.String(), sdk.NewCoins(spotCoinQuote.Add(derivativeCoin)))

			price := sdk.NewDec(int64(2000))
			quantity := sdk.NewDec(int64(i + 1))
			margin := sdk.NewDec(int64(1500))

			buyPrice1 := price.Sub(sdk.NewDec(int64((i + 1) * 10)))
			buyPrice2 := price.Sub(sdk.NewDec(int64((i + 2) * 10)))
			buyPrice3 := price.Sub(sdk.NewDec(int64((i + 3) * 10)))
			sellPrice1 := price.Add(sdk.NewDec(int64((i + 1) * 10)))
			sellPrice2 := price.Add(sdk.NewDec(int64((i + 2) * 10)))
			sellPrice3 := price.Add(sdk.NewDec(int64((i + 3) * 10)))

			limitSpotBuyOrder1 := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(buyPrice1, quantity, exchangetypes.OrderType_BUY, buyer, i)
			limitSpotBuyOrder2 := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(buyPrice2, quantity, exchangetypes.OrderType_BUY, buyer, i)
			limitSpotBuyOrder3 := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(buyPrice3, quantity, exchangetypes.OrderType_BUY, buyer, i)
			limitSpotSellOrder1 := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(sellPrice1, quantity, exchangetypes.OrderType_SELL, seller, i)
			limitSpotSellOrder2 := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(sellPrice2, quantity, exchangetypes.OrderType_SELL, seller, i)
			limitSpotSellOrder3 := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(sellPrice3, quantity, exchangetypes.OrderType_SELL, seller, i)

			limitDerivativeBuyOrder1 := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, buyPrice1, quantity, margin, exchangetypes.OrderType_BUY, buyer, i, false)
			limitDerivativeBuyOrder2 := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, buyPrice2, quantity, margin, exchangetypes.OrderType_BUY, buyer, i, false)
			limitDerivativeBuyOrder3 := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, buyPrice3, quantity, margin, exchangetypes.OrderType_BUY, buyer, i, false)
			limitDerivativeSellOrder1 := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, sellPrice1, quantity, margin, exchangetypes.OrderType_SELL, seller, i, false)
			limitDerivativeSellOrder2 := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, sellPrice2, quantity, margin, exchangetypes.OrderType_SELL, seller, i, false)
			limitDerivativeSellOrder3 := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, sellPrice3, quantity, margin, exchangetypes.OrderType_SELL, seller, i, false)

			binaryOptionBuyOrder1 := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(testexchange.DefaultAddress, buyPrice1, quantity, exchangetypes.OrderType_BUY, buyer, i, false)
			binaryOptionBuyOrder2 := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(testexchange.DefaultAddress, buyPrice2, quantity, exchangetypes.OrderType_BUY, buyer, i, false)
			binaryOptionBuyOrder3 := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(testexchange.DefaultAddress, buyPrice3, quantity, exchangetypes.OrderType_BUY, buyer, i, false)
			binaryOptionSellOrder1 := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(testexchange.DefaultAddress, sellPrice1, quantity, exchangetypes.OrderType_SELL, seller, i, false)
			binaryOptionSellOrder2 := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(testexchange.DefaultAddress, sellPrice2, quantity, exchangetypes.OrderType_SELL, seller, i, false)
			binaryOptionSellOrder3 := testInput.NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(testexchange.DefaultAddress, sellPrice3, quantity, exchangetypes.OrderType_SELL, seller, i, false)

			for _, msg := range []*exchangetypes.MsgCreateSpotLimitOrder{limitSpotBuyOrder1, limitSpotBuyOrder2, limitSpotBuyOrder3, limitSpotSellOrder1, limitSpotSellOrder2, limitSpotSellOrder3} {
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), msg)
				testexchange.OrFail(err)
			}
			for _, msg := range []*exchangetypes.MsgCreateDerivativeLimitOrder{limitDerivativeBuyOrder1, limitDerivativeBuyOrder2, limitDerivativeBuyOrder3, limitDerivativeSellOrder1, limitDerivativeSellOrder2, limitDerivativeSellOrder3} {
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), msg)
				testexchange.OrFail(err)
			}
			for _, msg := range []*exchangetypes.MsgCreateBinaryOptionsLimitOrder{binaryOptionBuyOrder1, binaryOptionBuyOrder2, binaryOptionBuyOrder3, binaryOptionSellOrder1, binaryOptionSellOrder2, binaryOptionSellOrder3} {
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), msg)
				testexchange.OrFail(err)
			}
		}
	}

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		spotMarketsCount := 5
		derivativeMarketsCount := 5
		binaryOptionsMarketsCount := 5

		testInput, ctx = testexchange.SetupTest(app, ctx, spotMarketsCount, derivativeMarketsCount, binaryOptionsMarketsCount)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		for idx := 0; idx < spotMarketsCount; idx++ {
			_, err := app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[idx].Ticker, testInput.Spots[idx].BaseDenom, testInput.Spots[idx].QuoteDenom, testInput.Spots[idx].MinPriceTickSize, testInput.Spots[idx].MinQuantityTickSize)
			testexchange.OrFail(err)
		}

		//** derivative markets
		for idx := 0; idx < derivativeMarketsCount; idx++ {
			oracleBase, oracleQuote, oracleType := testInput.Perps[idx].OracleBase, testInput.Perps[idx].OracleQuote, testInput.Perps[idx].OracleType
			startingPrice := sdk.NewDec(2000)

			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			sender := sdk.AccAddress(common.FromHex("727aee334987c52fa7b567b2662bdbb68614e48c"))
			coin := sdk.NewCoin(testInput.Perps[idx].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[idx].Ticker, testInput.Perps[idx].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

			_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
				ctx,
				testInput.Perps[idx].Ticker,
				testInput.Perps[idx].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.Perps[idx].InitialMarginRatio,
				testInput.Perps[idx].MaintenanceMarginRatio,
				testInput.Perps[idx].MakerFeeRate,
				testInput.Perps[idx].TakerFeeRate,
				testInput.Perps[idx].MinPriceTickSize,
				testInput.Perps[idx].MinQuantityTickSize,
			)
		}
		testexchange.OrFail(err)

		//** binary markets
		for idx := 0; idx < binaryOptionsMarketsCount; idx++ {
			market := testInput.BinaryMarkets[idx]
			oracleProvider, admin := market.OracleProvider, market.Admin

			err := app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
				Provider: oracleProvider,
				Relayers: []string{admin},
			})

			adminAddress, _ := sdk.AccAddressFromBech32(admin)

			initialCoinDeposit := sdk.NewInt(200000000)
			spotCoinQuote := sdk.NewCoin(market.QuoteDenom, initialCoinDeposit)
			spotTotalCoin := sdk.NewCoin(market.QuoteDenom, initialCoinDeposit.Mul(sdk.NewInt(2)))

			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(spotTotalCoin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, adminAddress, sdk.NewCoins(spotCoinQuote))

			app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
				Provider: oracleProvider,
			})
			_, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(ctx, market.Ticker, market.OracleSymbol, market.OracleProvider,
				oracletypes.OracleType_Provider, market.OracleScaleFactor, market.MakerFeeRate, market.TakerFeeRate,
				market.ExpirationTimestamp, market.SettlementTimestamp, admin, market.QuoteDenom, market.MinPriceTickSize, market.MinQuantityTickSize)

			testexchange.OrFail(err)
			//
		}
		testexchange.OrFail(err)

		// resting orders
		addNewOrders()
		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		// transient orders
		addNewOrders()
	})

	Describe("Batch Update Orders for cancel all", func() {
		sender := testexchange.SampleAccountAddr1 // buyer - cancelling all buy orders
		BeforeEach(func() {
			message = &exchangetypes.MsgBatchUpdateOrders{
				Sender:                            sender.String(),
				SubaccountId:                      buyer.Hex(),
				SpotMarketIdsToCancelAll:          []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[1].MarketID.Hex(), testInput.Spots[2].MarketID.Hex(), testInput.Spots[3].MarketID.Hex(), testInput.Spots[4].MarketID.Hex()},
				DerivativeMarketIdsToCancelAll:    []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[1].MarketID.Hex(), testInput.Perps[2].MarketID.Hex(), testInput.Perps[3].MarketID.Hex(), testInput.Perps[4].MarketID.Hex()},
				BinaryOptionsMarketIdsToCancelAll: []string{testInput.BinaryMarkets[0].MarketID.Hex(), testInput.BinaryMarkets[1].MarketID.Hex(), testInput.BinaryMarkets[2].MarketID.Hex(), testInput.BinaryMarkets[3].MarketID.Hex(), testInput.BinaryMarkets[4].MarketID.Hex()},
				SpotOrdersToCancel:                nil,
				DerivativeOrdersToCancel:          nil,
				SpotOrdersToCreate:                nil,
				DerivativeOrdersToCreate:          nil,
			}
			err = message.ValidateBasic()
			testexchange.OrFail(err)
		})

		JustBeforeEach(func() {
			res, err = msgServer.BatchUpdateOrders(sdk.WrapSDKContext(ctx), message)
		})

		It("Should be valid", func() {
			Expect(err).To(BeNil())
		})

		It("Should have empty response", func() {
			Expect(*res).To(Equal(exchangetypes.MsgBatchUpdateOrdersResponse{
				SpotCancelSuccess:          make([]bool, 0),
				DerivativeCancelSuccess:    make([]bool, 0),
				SpotOrderHashes:            make([]string, 0),
				DerivativeOrderHashes:      make([]string, 0),
				BinaryOptionsCancelSuccess: make([]bool, 0),
				BinaryOptionsOrderHashes:   make([]string, 0),
			}))
		})

		It("Should have cancelled orders", func() {
			restingSpotOrders := app.ExchangeKeeper.GetAllSpotLimitOrderbook(ctx)
			transientSpotOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrderbook(ctx)
			restingDerivativeOrders := app.ExchangeKeeper.GetAllDerivativeAndBinaryOptionsLimitOrderbook(ctx)
			transientDerivativeOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrderbook(ctx)

			var expectSpotCancelledOrders = func(orders []*exchangetypes.SpotLimitOrder, isBuy bool) {
				if isBuy {
					Expect(len(orders)).To(BeZero())
				} else {
					Expect(len(orders)).To(Equal(3))
				}
			}

			for _, orderbook := range restingSpotOrders {
				expectSpotCancelledOrders(orderbook.Orders, orderbook.IsBuySide)
			}
			for _, orderbook := range transientSpotOrders {
				expectSpotCancelledOrders(orderbook.Orders, orderbook.IsBuySide)
			}

			var expectDerivativeCancelledOrders = func(orders []*exchangetypes.DerivativeLimitOrder, isBuy bool) {
				if isBuy {
					Expect(len(orders)).To(BeZero())
				} else {
					Expect(len(orders)).To(Equal(3))
				}
			}

			for _, orderbook := range restingDerivativeOrders {
				expectDerivativeCancelledOrders(orderbook.Orders, orderbook.IsBuySide)
			}
			for _, orderbook := range transientDerivativeOrders {
				expectDerivativeCancelledOrders(orderbook.Orders, orderbook.IsBuySide)
			}
		})
	})

	Describe("Batch Update Orders for cancelling specific orders", func() {
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			spotOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, testInput.Spots[0].MarketID, true, buyer)
			derivativeOrders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, testInput.Perps[0].MarketID, true, buyer)
			binaryOrders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, testInput.BinaryMarkets[0].MarketID, true, buyer)
			message = &exchangetypes.MsgBatchUpdateOrders{
				Sender:                         sender.String(),
				SubaccountId:                   "",
				SpotMarketIdsToCancelAll:       nil,
				DerivativeMarketIdsToCancelAll: nil,
				SpotOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    spotOrders[0].Hash().Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0x1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212abab",
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    spotOrders[1].Hash().Hex(),
					},
				},
				DerivativeOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0xabab1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212",
					},
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[0].Hex(),
					},
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[1].Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(), // invalid derivative market id
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[2].Hex(),
					},
				},
				BinaryOptionsOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    binaryOrders[0].Hex(),
					},
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0x1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212abab",
					},
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    binaryOrders[1].Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(), // invalid binary opt market id
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[2].Hex(),
					},
				},
				SpotOrdersToCreate:       nil,
				DerivativeOrdersToCreate: nil,
			}
			err = message.ValidateBasic()
			testexchange.OrFail(err)
		})

		JustBeforeEach(func() {
			res, err = msgServer.BatchUpdateOrders(sdk.WrapSDKContext(ctx), message)
		})

		It("Should be valid", func() {
			Expect(err).To(BeNil())
		})

		It("Should have empty response", func() {
			Expect(*res).To(Equal(exchangetypes.MsgBatchUpdateOrdersResponse{
				SpotCancelSuccess:          []bool{true, false, true},
				DerivativeCancelSuccess:    []bool{false, true, true, false},
				BinaryOptionsCancelSuccess: []bool{true, false, true, false},
				SpotOrderHashes:            make([]string, 0),
				DerivativeOrderHashes:      make([]string, 0),
				BinaryOptionsOrderHashes:   []string{},
			}))
		})

		It("Should have cancelled orders", func() {
			spotOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, testInput.Spots[0].MarketID, true, buyer)
			derivativeOrders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, testInput.Perps[0].MarketID, true, buyer)
			binaryOrders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, testInput.BinaryMarkets[0].MarketID, true, buyer)

			Expect(len(spotOrders)).To(Equal(1))
			Expect(len(derivativeOrders)).To(Equal(1))
			Expect(len(binaryOrders)).To(Equal(1))
		})
	})

	Describe("Batch Update Orders for cancelling specific transient orders", func() {
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			spotOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersBySubaccountAndMarket(ctx, testInput.Spots[0].MarketID, true, buyer)
			derivativeOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, testInput.Perps[0].MarketID, &buyer, true)
			binaryOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, testInput.BinaryMarkets[0].MarketID, &buyer, true)
			message = &exchangetypes.MsgBatchUpdateOrders{
				Sender:                         sender.String(),
				SubaccountId:                   "",
				SpotMarketIdsToCancelAll:       nil,
				DerivativeMarketIdsToCancelAll: nil,
				SpotOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    spotOrders[0].Hash().Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0x1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212abab",
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    spotOrders[1].Hash().Hex(),
					},
				},
				DerivativeOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0xabab1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212",
					},
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    common.Bytes2Hex(derivativeOrders[0].GetOrderHash()),
					},
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    common.Bytes2Hex(derivativeOrders[1].GetOrderHash()),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(), // invalid derivative market id
						SubaccountId: buyer.Hex(),
						OrderHash:    common.Bytes2Hex(derivativeOrders[2].GetOrderHash()),
					},
				},
				BinaryOptionsOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    common.Bytes2Hex(binaryOrders[0].GetOrderHash()),
					},
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0x1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212abab",
					},
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    common.Bytes2Hex(binaryOrders[1].GetOrderHash()),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(), // invalid binary opt market id
						SubaccountId: buyer.Hex(),
						OrderHash:    common.Bytes2Hex(binaryOrders[2].GetOrderHash()),
					},
				},
				SpotOrdersToCreate:       nil,
				DerivativeOrdersToCreate: nil,
			}
			err = message.ValidateBasic()
			testexchange.OrFail(err)
		})

		JustBeforeEach(func() {
			res, err = msgServer.BatchUpdateOrders(sdk.WrapSDKContext(ctx), message)
		})

		It("Should be valid", func() {
			Expect(err).To(BeNil())
		})

		It("Should have cancel successes as response", func() {
			Expect(*res).To(Equal(exchangetypes.MsgBatchUpdateOrdersResponse{
				SpotCancelSuccess:          []bool{true, false, true},
				DerivativeCancelSuccess:    []bool{false, true, true, false},
				SpotOrderHashes:            make([]string, 0),
				DerivativeOrderHashes:      make([]string, 0),
				BinaryOptionsCancelSuccess: []bool{true, false, true, false},
				BinaryOptionsOrderHashes:   make([]string, 0),
			}))
		})

		It("Should have cancelled orders", func() {
			spotOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersBySubaccountAndMarket(ctx, testInput.Spots[0].MarketID, true, buyer)
			derivativeOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, testInput.Perps[0].MarketID, &buyer, true)
			binaryOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, testInput.BinaryMarkets[0].MarketID, &buyer, true)

			Expect(len(spotOrders)).To(Equal(1))
			Expect(len(derivativeOrders)).To(Equal(1))
			Expect(len(binaryOrders)).To(Equal(1))
		})
	})

	Describe("Batch Update Orders for creating new orders", func() {
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			subaccountsList := []common.Hash{common.HexToHash(subaccountID)}

			depositAmount := sdk.NewDec(100000)

			testInput.AddDerivativeDepositsForSubaccounts(app, ctx, 2, &depositAmount, false, subaccountsList)

			deposits = map[string]*exchangetypes.Deposit{
				testInput.Perps[0].QuoteDenom: testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom),
				testInput.Perps[1].QuoteDenom: testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[1].QuoteDenom),
			}

			message = &exchangetypes.MsgBatchUpdateOrders{
				Sender:                         sender.String(),
				SubaccountId:                   "",
				SpotMarketIdsToCancelAll:       nil,
				DerivativeMarketIdsToCancelAll: nil,
				SpotOrdersToCancel:             nil,
				DerivativeOrdersToCancel:       nil,
				SpotOrdersToCreate: []*exchangetypes.SpotOrder{
					{
						MarketId: testInput.Spots[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1500),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					}, {
						MarketId: testInput.Spots[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1400),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					}, {
						MarketId: testInput.Spots[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1300),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					}, {
						MarketId: testInput.Spots[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1200),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					},
				},
				DerivativeOrdersToCreate: []*exchangetypes.DerivativeOrder{
					{
						MarketId: testInput.Perps[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					}, {
						MarketId: testInput.Perps[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					}, {
						MarketId: testInput.Perps[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					}, {
						MarketId: testInput.Perps[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					},
				},
				BinaryOptionsOrdersToCreate: []*exchangetypes.DerivativeOrder{
					{
						MarketId: testInput.BinaryMarkets[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(4000),
					}, {
						MarketId: testInput.BinaryMarkets[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(3998),
					}, {
						MarketId: testInput.BinaryMarkets[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(4000),
					}, {
						MarketId: testInput.BinaryMarkets[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(3998),
					},
				},
			}

			err = message.ValidateBasic()
			testexchange.OrFail(err)
		})

		JustBeforeEach(func() {
			res, err = msgServer.BatchUpdateOrders(sdk.WrapSDKContext(ctx), message)
			balancesUsed = make(map[string]sdk.Dec)
			for _, order := range message.SpotOrdersToCreate {
				market := testInput.MarketIDToSpotMarket[common.HexToHash(order.MarketId)]
				feePaid := market.TakerFeeRate.Mul(order.OrderInfo.Price).Mul(order.OrderInfo.Quantity)
				if _, ok := balancesUsed[market.QuoteDenom]; !ok {
					balancesUsed[market.QuoteDenom] = sdk.ZeroDec()
				}

				balancesUsed[market.QuoteDenom] = balancesUsed[market.QuoteDenom].Add((order.OrderInfo.Price).Mul(order.OrderInfo.Quantity)).Add(feePaid)
			}
			for _, order := range message.DerivativeOrdersToCreate {
				market := testInput.MarketIDToPerpMarket[order.MarketID()]
				feePaid := market.TakerFeeRate.Mul(order.Price()).Mul(order.OrderInfo.Quantity)
				if _, ok := balancesUsed[market.QuoteDenom]; !ok {
					balancesUsed[market.QuoteDenom] = sdk.ZeroDec()
				}

				balancesUsed[market.QuoteDenom] = balancesUsed[market.QuoteDenom].Add(feePaid.Add(order.Margin))
			}
			for _, order := range message.BinaryOptionsOrdersToCreate {
				market := testInput.MarketIDToBinaryMarket[order.MarketID()]
				feePaid := market.TakerFeeRate.Mul(order.Price()).Mul(order.OrderInfo.Quantity)
				if _, ok := balancesUsed[market.QuoteDenom]; !ok {
					balancesUsed[market.QuoteDenom] = sdk.ZeroDec()
				}

				balancesUsed[market.QuoteDenom] = balancesUsed[market.QuoteDenom].Add(feePaid).Add(order.Margin)
			}
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have valid result", func() {
				Expect(len(res.SpotOrderHashes)).To(Equal(4))
				for _, hash := range res.SpotOrderHashes {
					Expect(hash != "").To(BeTrue())
				}

				Expect(len(res.DerivativeOrderHashes)).To(Equal(4))
				for _, hash := range res.DerivativeOrderHashes {
					Expect(hash != "").To(BeTrue())
				}

				Expect(len(res.BinaryOptionsOrderHashes)).To(Equal(4))
				for _, hash := range res.DerivativeOrderHashes {
					Expect(hash != "").To(BeTrue())
				}
			})

			It("Should have updated balances", func() {
				for quoteDenom, balance := range balancesUsed {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)
					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposits[quoteDenom].AvailableBalance.Sub(balance).String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposits[quoteDenom].TotalBalance.String()))
				}
			})
		})
	})

	Describe("Batch Update Orders all in one", func() {
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = buyer.Hex()
			subaccountsList := []common.Hash{common.HexToHash(subaccountID)}

			depositAmount := sdk.NewDec(100000)

			testInput.AddDerivativeDepositsForSubaccounts(app, ctx, 2, &depositAmount, false, subaccountsList)

			spotOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, testInput.Spots[0].MarketID, true, buyer)
			derivativeOrders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, testInput.Perps[0].MarketID, true, buyer)
			binaryOrders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, testInput.BinaryMarkets[0].MarketID, true, buyer)

			message = &exchangetypes.MsgBatchUpdateOrders{
				Sender:                            sender.String(),
				SubaccountId:                      buyer.Hex(),
				SpotMarketIdsToCancelAll:          []string{testInput.Spots[1].MarketID.Hex(), testInput.Spots[3].MarketID.Hex(), testInput.Spots[4].MarketID.Hex()},
				DerivativeMarketIdsToCancelAll:    []string{testInput.Perps[1].MarketID.Hex(), testInput.Perps[3].MarketID.Hex(), testInput.Perps[4].MarketID.Hex()},
				BinaryOptionsMarketIdsToCancelAll: []string{testInput.BinaryMarkets[1].MarketID.Hex(), testInput.BinaryMarkets[3].MarketID.Hex(), testInput.BinaryMarkets[4].MarketID.Hex()},
				SpotOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    spotOrders[0].Hash().Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0x1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212abab", // invalid
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    spotOrders[1].Hash().Hex(),
					},
				},
				DerivativeOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0xabab1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212", // invalid
					},
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[0].Hex(),
					},
					{
						MarketId:     testInput.Perps[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[1].Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(), // invalid derivative market id
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[2].Hex(),
					},
				},
				BinaryOptionsOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    binaryOrders[0].Hex(),
					},
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    "0x1212abab1212abab1212abab1212abab1212abab1212abab1212abab1212abab",
					},
					{
						MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
						SubaccountId: buyer.Hex(),
						OrderHash:    binaryOrders[1].Hex(),
					},
					{
						MarketId:     testInput.Spots[0].MarketID.Hex(), // invalid binary opt market id
						SubaccountId: buyer.Hex(),
						OrderHash:    derivativeOrders[2].Hex(),
					},
				},
				SpotOrdersToCreate: []*exchangetypes.SpotOrder{
					{
						MarketId: testInput.Spots[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1500),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					}, {
						MarketId: testInput.Spots[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1400),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					}, {
						MarketId: testInput.Spots[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1300),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					}, {
						MarketId: testInput.Spots[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1200),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
					},
				},
				DerivativeOrdersToCreate: []*exchangetypes.DerivativeOrder{
					{
						MarketId: testInput.Perps[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					}, {
						MarketId: testInput.Perps[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					}, {
						MarketId: testInput.Perps[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					}, {
						MarketId: testInput.Perps[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(2000),
					},
				},
				BinaryOptionsOrdersToCreate: []*exchangetypes.DerivativeOrder{
					{
						MarketId: testInput.BinaryMarkets[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(4000),
					}, {
						MarketId: testInput.BinaryMarkets[0].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(3998),
					}, {
						MarketId: testInput.BinaryMarkets[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(4000),
					}, {
						MarketId: testInput.BinaryMarkets[1].MarketID.Hex(),
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1999),
							Quantity:     sdk.NewDec(2),
						},
						OrderType: exchangetypes.OrderType_BUY,
						Margin:    sdk.NewDec(3998),
					},
				},
			}
			err = message.ValidateBasic()
			testexchange.OrFail(err)
		})

		JustBeforeEach(func() {
			res, err = msgServer.BatchUpdateOrders(sdk.WrapSDKContext(ctx), message)
			balancesUsed = make(map[string]sdk.Dec)
			for _, order := range message.SpotOrdersToCreate {
				market := testInput.MarketIDToSpotMarket[common.HexToHash(order.MarketId)]
				feePaid := market.TakerFeeRate.Mul(order.OrderInfo.Price).Mul(order.OrderInfo.Quantity)
				if _, ok := balancesUsed[market.QuoteDenom]; !ok {
					balancesUsed[market.QuoteDenom] = sdk.ZeroDec()
				}

				balancesUsed[market.QuoteDenom] = balancesUsed[market.QuoteDenom].Add((order.OrderInfo.Price).Mul(order.OrderInfo.Quantity)).Add(feePaid)
			}
			for _, order := range message.DerivativeOrdersToCreate {
				market := testInput.MarketIDToPerpMarket[order.MarketID()]
				feePaid := market.TakerFeeRate.Mul(order.Price()).Mul(order.OrderInfo.Quantity)
				if _, ok := balancesUsed[market.QuoteDenom]; !ok {
					balancesUsed[market.QuoteDenom] = sdk.ZeroDec()
				}

				balancesUsed[market.QuoteDenom] = balancesUsed[market.QuoteDenom].Add(feePaid.Add(order.Margin))
			}
			for _, order := range message.BinaryOptionsOrdersToCreate {
				market := testInput.MarketIDToBinaryMarket[order.MarketID()]
				feePaid := market.TakerFeeRate.Mul(order.Price()).Mul(order.OrderInfo.Quantity)
				if _, ok := balancesUsed[market.QuoteDenom]; !ok {
					balancesUsed[market.QuoteDenom] = sdk.ZeroDec()
				}

				balancesUsed[market.QuoteDenom] = balancesUsed[market.QuoteDenom].Add(feePaid).Add(order.Margin)
			}
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have correct response", func() {
				Expect(len(res.SpotOrderHashes)).To(Equal(4))
				for _, hash := range res.SpotOrderHashes {
					Expect(hash != "").To(BeTrue())
				}

				Expect(len(res.DerivativeOrderHashes)).To(Equal(4))
				for _, hash := range res.DerivativeOrderHashes {
					Expect(hash != "").To(BeTrue())
				}

				Expect(res.SpotCancelSuccess).To(Equal([]bool{true, false, true}))
				Expect(res.DerivativeCancelSuccess).To(Equal([]bool{false, true, true, false}))
				Expect(res.BinaryOptionsCancelSuccess).To(Equal([]bool{true, false, true, false}))
			})

			It("Should have cancelled orders", func() {
				var expectSpotCancelledOrders = func(orders []*exchangetypes.SpotLimitOrder, idx int, isTransient bool) {
					if idx >= 3 {
						Expect(len(orders)).To(BeZero())
						return
					}

					// idx = 0: 3 resting - 2 cancelled = 1 (3 + 2 new transient orders)
					// idx = 1: 3 resting - 3 (all) cancelled = 0 (2 new transient orders)
					// idx = 2: 3 resting = 3

					if idx == 0 && isTransient {
						Expect(len(orders)).To(Equal(5))
					} else if idx == 0 {
						Expect(len(orders)).To(Equal(1))
					} else if idx == 1 && isTransient {
						Expect(len(orders)).To(Equal(2))
					} else if idx == 1 {
						Expect(len(orders)).To(BeZero())
					} else {
						Expect(len(orders)).To(Equal(3))
					}
				}

				var expectDerivativeCancelledOrders = func(orders []*exchangetypes.DerivativeLimitOrder, idx int, isTransient bool) {
					if idx >= 3 {
						Expect(len(orders)).To(BeZero())
						return
					}

					// idx = 0: 3 resting - 2 cancelled = 1 (3 + 2 new transient orders)
					// idx = 1: 3 resting - 3 (all) cancelled = 0 (2 new transient orders)
					// idx = 2: 3 resting = 3

					if idx == 0 && isTransient {
						Expect(len(orders)).To(Equal(5))
					} else if idx == 0 {
						Expect(len(orders)).To(Equal(1))
					} else if idx == 1 && isTransient {
						Expect(len(orders)).To(Equal(2))
					} else if idx == 1 {
						Expect(len(orders)).To(BeZero())
					} else {
						Expect(len(orders)).To(Equal(3))
					}
				}

				var expectBinaryCancelledOrders = func(orders []*exchangetypes.DerivativeLimitOrder, idx int, isTransient bool) {
					if idx >= 3 {
						Expect(len(orders)).To(BeZero())
						return
					}

					// idx = 0: 3 resting - 2 cancelled = 1 (3 + 2 new transient orders)
					// idx = 1: 3 resting - 3 (all) cancelled = 0 (2 new transient orders)
					// idx = 2: 3 resting = 3

					if idx == 0 && isTransient {
						Expect(len(orders)).To(Equal(5))
					} else if idx == 0 {
						Expect(len(orders)).To(Equal(1))
					} else if idx == 1 && isTransient {
						Expect(len(orders)).To(Equal(2))
					} else if idx == 1 {
						Expect(len(orders)).To(BeZero())
					} else {
						Expect(len(orders)).To(Equal(3))
					}
				}

				for idx := 0; idx < 5; idx++ {
					spotMarketId := testInput.Spots[idx].MarketID
					restingSpotOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, spotMarketId, true)
					transientSpotOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersByMarketDirection(ctx, spotMarketId, true)

					derivativeMarketId := testInput.Perps[idx].MarketID
					restingDerivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, derivativeMarketId, true)
					transientDerivativeOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(ctx, derivativeMarketId, true)

					binaryMarketId := testInput.BinaryMarkets[idx].MarketID
					restingBinaryOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, binaryMarketId, true)
					transientBinaryOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(ctx, binaryMarketId, true)

					expectSpotCancelledOrders(restingSpotOrders, idx, false)
					expectSpotCancelledOrders(transientSpotOrders, idx, true)
					expectDerivativeCancelledOrders(restingDerivativeOrders, idx, false)
					expectDerivativeCancelledOrders(transientDerivativeOrders, idx, true)
					expectBinaryCancelledOrders(restingBinaryOrders, idx, false)
					expectBinaryCancelledOrders(transientBinaryOrders, idx, true)
				}
			})
		})
	})
})
