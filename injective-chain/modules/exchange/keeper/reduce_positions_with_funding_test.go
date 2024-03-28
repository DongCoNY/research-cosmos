package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

// TODO funding event testing

var _ = Describe("Reducing position when funding is applied test", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket          *types.DerivativeMarket
		msgServer                 types.MsgServer
		err                       error
		buyer                     = testexchange.SampleSubaccountAddr1
		seller                    = testexchange.SampleSubaccountAddr2
		buyer2                    = testexchange.SampleSubaccountAddr3
		seller2                   = testexchange.SampleSubaccountAddr4
		depositBefore             *types.Deposit
		depositAfter              *types.Deposit
		expectedCumulativeFunding sdk.Dec
		timeInterval              = int64(3600)
		startingPrice             = sdk.NewDec(2000)
		hoursPerDay               = sdk.NewDec(24)
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
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
		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)

		testexchange.OrFail(err)
		Expect(common.HexToHash(derivativeMarket.MarketId)).To(BeEquivalentTo(testInput.Perps[0].MarketID))
	})

	Describe("When reducing long positions", func() {
		var (
			price    sdk.Dec
			quantity sdk.Dec
		)

		JustBeforeEach(func() {
			margin := sdk.NewDec(5000)
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_SELL, seller)
			reduceOnlyDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, sdk.NewDec(0), types.OrderType_SELL, buyer)
			limitDerivativeBuyOrder2 := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_BUY, buyer2)

			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(20000),
				TotalBalance:     sdk.NewDec(20000),
			}

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, buyer2.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Perps[0].QuoteDenom)

			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), reduceOnlyDerivativeSellOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			depositAfter = testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Perps[0].QuoteDenom)
		})

		Describe("with positive funding rate", func() {
			Context("not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)

					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding = expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
				})
			})

			Context("enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2050)
					quantity = sdk.NewDec(1)

					expectedCumulativeFunding = types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
				})
			})
		})

		Describe("with negative funding rate", func() {
			Context("not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1990)
					quantity = sdk.NewDec(1)

					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding = expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
				})
			})

			Context("enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1950)
					quantity = sdk.NewDec(1)

					expectedCumulativeFunding = types.DefaultParams().DefaultHourlyFundingRateCap.Neg().Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Add(expectedCumulativeFunding).String()))
				})
			})
		})
	})

	Describe("When reducing short positions", func() {
		var (
			price    sdk.Dec
			quantity sdk.Dec
		)

		JustBeforeEach(func() {
			margin := sdk.NewDec(5000)
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_SELL, seller)
			reduceOnlyDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, sdk.NewDec(0), types.OrderType_BUY, seller)
			limitDerivativeSellOrder2 := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_SELL, seller2)

			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(20000),
				TotalBalance:     sdk.NewDec(20000),
			}

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller2.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Perps[0].QuoteDenom)

			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), reduceOnlyDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			depositAfter = testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Perps[0].QuoteDenom)
		})

		Describe("with positive funding rate", func() {
			Context("not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)

					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding = expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
				})
			})

			Context("enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2050)
					quantity = sdk.NewDec(1)

					expectedCumulativeFunding = types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
				})
			})
		})

		Describe("with negative funding rate", func() {
			Context("not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1990)
					quantity = sdk.NewDec(1)

					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding = expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
				})
			})

			Context("enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1950)
					quantity = sdk.NewDec(1)

					expectedCumulativeFunding = types.DefaultParams().DefaultHourlyFundingRateCap.Neg().Mul(startingPrice)
				})

				It("should have correct payout", func() {
					feePaid := derivativeMarket.TakerFeeRate.Mul(price).Mul(quantity).Mul(sdk.NewDec(2))

					Expect(depositBefore.AvailableBalance.String()).To(Equal(depositAfter.AvailableBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
					Expect(depositBefore.TotalBalance.String()).To(Equal(depositAfter.TotalBalance.Add(feePaid).Sub(expectedCumulativeFunding).String()))
				})
			})
		})
	})
})
