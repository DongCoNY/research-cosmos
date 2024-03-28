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

var _ = Describe("Provider Oracle Market Tests", func() {
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

		Context("with a Provider oracle", func() {
			BeforeEach(func() {
				var (
					baseDenom = "0x7a5bc1d2b56ad029048cd63964b3ad2776eadf812edc1a43a31406cb54bff592" // INJ/USD

					//  (must be without 0x prefix to be a valid denom
					quoteDenom = "a4702f0f5818258783a1e47f453cb20b0fbec32ca67260e1d19dfcdd6a4d0ebb" // INTER/USD

					symbol   = "INJ/USDT"
					provider = "provider"

					perpMarket = testexchange.SetupPerpetualMarket(baseDenom, quoteDenom, symbol, provider, oracletypes.OracleType_Provider)
					sender     = common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1")
					coin       = sdk.NewCoin(perpMarket.QuoteDenom, sdk.OneInt())
				)

				app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
					Provider: provider,
					Relayers: []string{testexchange.FeeRecipient},
				})

				app.OracleKeeper.SetProviderPriceState(ctx, provider, oracletypes.NewProviderPriceState(symbol, sdk.NewDec(2000), ctx.BlockTime().Unix()))

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
