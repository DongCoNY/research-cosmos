package keeper_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Derivative Order Self Matching Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
	)

	_ = msgServer
	_ = derivativeMarket

	Describe("When self matching", func() {
		var (
			marketIdx = 0
			err       error
		)

		BeforeEach(func() {
			app = simapp.Setup(false)
			ctx = app.BaseApp.NewContext(false, tmproto.Header{
				Height: 1234567,
				Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
			})
			testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
			msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

			oracleBase, oracleQuote, oracleType := testInput.Perps[marketIdx].OracleBase, testInput.Perps[marketIdx].OracleQuote, testInput.Perps[marketIdx].OracleType
			startingPrice := sdk.NewDec(2000)
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

			_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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
			derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[marketIdx].MarketID, true)
		})

		It("matches correctly", func() {
			limitQuantity := sdk.NewDec(1)
			limitPrice := sdk.NewDec(2000)
			limitMargin := sdk.NewDec(200)
			trader := testexchange.SampleSubaccountAddr1

			startingQuoteDepositTrader := &types.Deposit{
				AvailableBalance: sdk.NewDec(10000),
				TotalBalance:     sdk.NewDec(10000),
			}
			testexchange.MintAndDeposit(app, ctx, trader.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, startingQuoteDepositTrader.AvailableBalance.TruncateInt())))

			limitBuyOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, limitPrice, limitQuantity, limitMargin, types.OrderType_BUY, trader, marketIdx, false)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsg)
			testexchange.OrFail(err)

			limitSellOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(testexchange.DefaultAddress, limitPrice, limitQuantity, limitMargin, types.OrderType_SELL, trader, marketIdx, false)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[marketIdx].MarketID, false)
			sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[marketIdx].MarketID, true)
			positions := app.ExchangeKeeper.GetAllPositions(ctx)

			Expect(len(buyOrders)).To(Equal(0))
			Expect(len(sellOrders)).To(Equal(0))
			Expect(len(positions)).To(Equal(0))

			quoteDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Perps[marketIdx].QuoteDenom)

			notional := limitPrice.Mul(limitQuantity)
			balancePaid := notional.Mul(derivativeMarket.TakerFeeRate).Mul(sdk.NewDec(2)) // When self matching, only fees are paid.

			Expect(quoteDepositAfter.AvailableBalance.String()).Should(Equal(startingQuoteDepositTrader.AvailableBalance.Sub(balancePaid).String()))
			Expect(quoteDepositAfter.TotalBalance.String()).Should(Equal(startingQuoteDepositTrader.TotalBalance.Sub(balancePaid).String()))
		})
	})
})

var _ = Describe("Derivative Order Self Matching Tests with Pyth oracle", func() {
	var (
		ctx sdk.Context
		app *simapp.InjectiveApp
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
	})

	Describe("In a perpetual derivative market", func() {
		var (
			marketID         common.Hash
			derivativeMarket *types.DerivativeMarket
		)

		Context("with a Pyth oracle", func() {
			BeforeEach(func() {
				var (
					baseDenom = "0x7a5bc1d2b56ad029048cd63964b3ad2776eadf812edc1a43a31406cb54bff592" // INJ/USD

					//  (must be without 0x prefix to be a valid denom
					quoteDenom = "a4702f0f5818258783a1e47f453cb20b0fbec32ca67260e1d19dfcdd6a4d0ebb" // INTER/USD

					perpMarket = testexchange.SetupPerpetualMarket(baseDenom, quoteDenom, baseDenom, quoteDenom, oracletypes.OracleType_Pyth)
					sender     = common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1")
					coin       = sdk.NewCoin(perpMarket.QuoteDenom, sdk.OneInt())
				)

				// INJ/USD 2000
				app.OracleKeeper.SetPythPriceState(ctx, &oracletypes.PythPriceState{
					PriceId:    common.HexToHash(baseDenom).Hex(),
					PriceState: oracletypes.PriceState{Price: sdk.NewDec(2000)},
				})
				app.OracleKeeper.SetPythPriceState(ctx, &oracletypes.PythPriceState{
					PriceId:    common.HexToHash(quoteDenom).Hex(),
					PriceState: oracletypes.PriceState{Price: sdk.NewDec(1)},
				})

				Expect(app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{coin})).To(BeNil())
				Expect(app.BankKeeper.SendCoinsFromModuleToAccount(ctx,
					minttypes.ModuleName,
					sender,
					sdk.NewCoins(coin),
				)).To(BeNil())
				Expect(app.InsuranceKeeper.CreateInsuranceFund(
					ctx,
					sender,
					coin,
					perpMarket.Ticker,
					perpMarket.QuoteDenom,
					perpMarket.OracleBase,
					perpMarket.OracleQuote,
					perpMarket.OracleType,
					-1,
				)).To(BeNil())

				_, _, err := app.ExchangeKeeper.PerpetualMarketLaunch(
					ctx,
					perpMarket.Ticker,
					perpMarket.QuoteDenom,
					perpMarket.OracleBase,
					perpMarket.OracleQuote,
					0,
					perpMarket.OracleType,
					perpMarket.InitialMarginRatio,
					perpMarket.MaintenanceMarginRatio,
					perpMarket.MakerFeeRate,
					perpMarket.TakerFeeRate,
					perpMarket.MinPriceTickSize,
					perpMarket.MinQuantityTickSize,
				)
				Expect(err).To(BeNil())

				marketID = perpMarket.MarketID
				derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, perpMarket.MarketID, true)
			})

			When("there are 2 matching limit orders", func() {
				var (
					trader                     common.Hash
					startingQuoteDepositTrader *types.Deposit
					limitQuantity,
					limitPrice,
					limitMargin sdk.Dec
				)

				BeforeEach(func() {
					trader = testexchange.SampleSubaccountAddr1
					limitQuantity = sdk.NewDec(1)
					limitPrice = sdk.NewDec(2000)
					limitMargin = sdk.NewDec(200)
					startingQuoteDepositTrader = &types.Deposit{
						AvailableBalance: sdk.NewDec(10000),
						TotalBalance:     sdk.NewDec(10000),
					}

					fund := sdk.NewCoin(derivativeMarket.QuoteDenom, startingQuoteDepositTrader.AvailableBalance.TruncateInt())
					testexchange.MintAndDeposit(app, ctx, trader.String(), sdk.Coins{fund})

					msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

					_, err := msgServer.CreateDerivativeLimitOrder(
						sdk.WrapSDKContext(ctx),
						testexchange.NewCreateDerivativeLimitOrderMsg(
							types.OrderType_BUY,
							marketID,
							trader,
							testexchange.DefaultAddress,
							limitPrice,
							limitQuantity,
							limitMargin,
						),
					)
					Expect(err).To(BeNil())

					_, err = msgServer.CreateDerivativeLimitOrder(
						sdk.WrapSDKContext(ctx),
						testexchange.NewCreateDerivativeLimitOrderMsg(
							types.OrderType_SELL,
							marketID,
							trader,
							testexchange.DefaultAddress,
							limitPrice,
							limitQuantity,
							limitMargin,
						),
					)
					Expect(err).To(BeNil())

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
				})

				It("they get matched", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, false)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, true)
					positions := app.ExchangeKeeper.GetAllPositions(ctx)

					Expect(len(buyOrders)).To(Equal(0))
					Expect(len(sellOrders)).To(Equal(0))
					Expect(len(positions)).To(Equal(0))

					quoteDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, trader, derivativeMarket.QuoteDenom)

					notional := limitPrice.Mul(limitQuantity)
					balancePaid := notional.Mul(derivativeMarket.TakerFeeRate).Mul(sdk.NewDec(2)) // When self matching, only fees are paid.

					Expect(quoteDepositAfter.AvailableBalance.String()).Should(Equal(startingQuoteDepositTrader.AvailableBalance.Sub(balancePaid).String()))
					Expect(quoteDepositAfter.TotalBalance.String()).Should(Equal(startingQuoteDepositTrader.TotalBalance.Sub(balancePaid).String()))
				})
			})
		})
	})
})
