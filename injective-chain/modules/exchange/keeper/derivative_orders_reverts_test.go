package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Derivatives Orders Keeper Unit Test", func() {
	var (
		testInput    testexchange.TestInput
		app          *simapp.InjectiveApp
		ctx          sdk.Context
		msgServer    exchangetypes.MsgServer
		marketID     common.Hash
		subaccountID common.Hash
	)

	BeforeEach(func() {
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

		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
		subaccountID = testexchange.SampleSubaccountAddr1
		marketID = testInput.Perps[0].MarketID

		deposit := &exchangetypes.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
	})

	Describe("When there is not enough remaining buy liquidity", func() {
		BeforeEach(func() {
			isBuy := true
			buyPrice1, buyQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			buyPrice2, buyQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			buyPrice3, buyQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")
			margin := sdk.NewDec(1000)

			buyOrder1 := testexchange.NewDerivativeLimitOrder(buyPrice1, buyQuantity1, margin, subaccountID, marketID.Hex(), "", isBuy)
			buyOrder2 := testexchange.NewDerivativeLimitOrder(buyPrice2, buyQuantity2, margin, subaccountID, marketID.Hex(), "", isBuy)
			buyOrder3 := testexchange.NewDerivativeLimitOrder(buyPrice3, buyQuantity3, margin, subaccountID, marketID.Hex(), "", isBuy)

			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &buyOrder1, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &buyOrder2, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &buyOrder3, nil, marketID)
		})

		It("rejects only the market order buys", func() {
			marketOrderMsgBuy := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(500), sdk.NewDec(2000), exchangetypes.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)
			Expect(errMarketBuy).To(Equal(exchangetypes.ErrSlippageExceedsWorstPrice))

			marketOrderMsgSell := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(3000), sdk.NewDec(100), exchangetypes.OrderType_SELL, subaccountID)
			_, errMarketSell := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)
			Expect(errMarketSell).To(BeNil())
		})

		It("rejects sell orders not within valid price range", func() {
			marketOrderMsgSell := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(3000), sdk.NewDec(500), exchangetypes.OrderType_SELL, subaccountID)
			_, errMarketSell := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)
			Expect(errMarketSell).To(Equal(exchangetypes.ErrSlippageExceedsWorstPrice))
		})
	})

	Describe("When there is not enough remaining sell liquidity", func() {
		BeforeEach(func() {
			isBuy := false
			sellPrice1, sellQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			sellPrice2, sellQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			sellPrice3, sellQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")
			margin := sdk.NewDec(1000)

			sellOrder1 := testexchange.NewDerivativeLimitOrder(sellPrice1, sellQuantity1, margin, subaccountID, marketID.Hex(), "", isBuy)
			sellOrder2 := testexchange.NewDerivativeLimitOrder(sellPrice2, sellQuantity2, margin, subaccountID, marketID.Hex(), "", isBuy)
			sellOrder3 := testexchange.NewDerivativeLimitOrder(sellPrice3, sellQuantity3, margin, subaccountID, marketID.Hex(), "", isBuy)

			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &sellOrder1, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &sellOrder2, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &sellOrder3, nil, marketID)
		})

		It("rejects only the market order sells", func() {
			marketOrderMsgSell := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(500), sdk.NewDec(2000), exchangetypes.OrderType_SELL, subaccountID)
			_, errMarketSell := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)
			Expect(errMarketSell).To(Equal(exchangetypes.ErrSlippageExceedsWorstPrice))

			marketOrderMsgBuy := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(1000), sdk.NewDec(500), exchangetypes.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)
			Expect(errMarketBuy).To(BeNil())
		})

		It("rejects buy orders not within valid price range", func() {
			marketOrderMsgBuy := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(3000), sdk.NewDec(100), exchangetypes.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)
			Expect(errMarketBuy).To(Equal(exchangetypes.ErrSlippageExceedsWorstPrice))
		})
	})
})
