package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"

	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func matchBuyerAndSellerWithAddedTime(testInput testexchange.TestInput, app *simapp.InjectiveApp, ctx sdk.Context, msgServer types.MsgServer, margin, quantity, price sdk.Dec, isLong bool, buyer, seller common.Hash, updatedTime time.Time) {
	var err error

	var buyerOrderType, sellerOrderType types.OrderType

	if isLong {
		buyerOrderType = types.OrderType_BUY
		sellerOrderType = types.OrderType_SELL
	} else {
		buyerOrderType = types.OrderType_SELL
		sellerOrderType = types.OrderType_BUY
	}

	limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, buyerOrderType, buyer)
	limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, sellerOrderType, seller)
	_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
	testexchange.OrFail(err)
	_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
	testexchange.OrFail(err)

	ctx = ctx.WithBlockTime(updatedTime)
	ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
	exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
}

var _ = Describe("Funding Tests on Multiple Epochs", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		startingPrice    = sdk.NewDec(2000)
		buyer            = testexchange.SampleSubaccountAddr1
		seller           = testexchange.SampleSubaccountAddr2
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

	Describe("When checking fundings for two epochs", func() {
		var (
			priceFirstEpoch                      sdk.Dec
			priceSecondEpoch                     sdk.Dec
			quantityFirstEpoch                   sdk.Dec
			quantitySecondEpoch                  sdk.Dec
			margin                               sdk.Dec
			expectedCumulativeFundingFirstEpoch  sdk.Dec
			expectedCumulativeFundingSecondEpoch sdk.Dec
			fundingFirstEpoch                    *types.PerpetualMarketFunding
			fundingSecondEpoch                   *types.PerpetualMarketFunding
			buyerPositionFirstEpoch              *types.DerivativePosition
			sellerPositionFirstEpoch             *types.DerivativePosition
			buyerPositionSecondEpoch             *types.DerivativePosition
			sellerPositionSecondEpoch            *types.DerivativePosition
		)

		JustBeforeEach(func() {
			initialDeposit := sdk.NewCoins(sdk.Coin{
				Denom:  testInput.Perps[0].QuoteDenom,
				Amount: sdk.NewInt(200000),
			})

			testexchange.MintAndDeposit(app, ctx, buyer.String(), initialDeposit)
			testexchange.MintAndDeposit(app, ctx, seller.String(), initialDeposit)

			updatedTime := ctx.BlockTime().Add(time.Hour * 1)
			matchBuyerAndSellerWithAddedTime(testInput, app, ctx, msgServer, margin, quantityFirstEpoch, priceFirstEpoch, true, buyer, seller, updatedTime)
			ctx = ctx.WithBlockTime(updatedTime)

			positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
			buyerPositionFirstEpoch = positions[0]
			sellerPositionFirstEpoch = positions[1]

			timeInterval := types.DefaultParams().DefaultFundingInterval

			expectedCumulativePriceFirstEpoch := (priceFirstEpoch.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
			expectedTwapFirstEpoch := expectedCumulativePriceFirstEpoch.Quo(sdk.NewDec(timeInterval).Mul(sdk.NewDec(24)))
			expectedCumulativeFundingFirstEpoch = expectedTwapFirstEpoch.Add(types.DefaultParams().DefaultHourlyInterestRate).Mul(startingPrice)

			fundingFirstEpoch = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			updatedTime = ctx.BlockTime().Add(time.Hour * 1)
			matchBuyerAndSellerWithAddedTime(testInput, app, ctx, msgServer, margin, quantitySecondEpoch, priceSecondEpoch, true, buyer, seller, updatedTime)
			ctx = ctx.WithBlockTime(updatedTime)

			positions = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
			buyerPositionSecondEpoch = positions[0]
			sellerPositionSecondEpoch = positions[1]

			expectedCumulativePriceSecondEpoch := (priceSecondEpoch.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
			expectedTwapSecondEpoch := expectedCumulativePriceSecondEpoch.Quo(sdk.NewDec(timeInterval).Mul(sdk.NewDec(24)))
			expectedFundingSecondEpoch := expectedTwapSecondEpoch.Add(types.DefaultParams().DefaultHourlyInterestRate).Mul(startingPrice)
			expectedCumulativeFundingSecondEpoch = expectedCumulativeFundingFirstEpoch.Add(expectedFundingSecondEpoch)

			fundingSecondEpoch = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())
		})

		Describe("when both epochs have positive funding", func() {
			BeforeEach(func() {
				priceFirstEpoch = sdk.NewDec(2010)
				priceSecondEpoch = sdk.NewDec(2020)
				quantityFirstEpoch = sdk.NewDec(1)
				quantitySecondEpoch = sdk.NewDec(1)
				margin = sdk.NewDec(5000)
			})

			It("has correct fundings", func() {
				fundingAppliedToPositions := fundingFirstEpoch.CumulativeFunding.Mul(quantityFirstEpoch)
				expectedMarginBuyerSecondEpoch := margin.Mul(sdk.NewDec(2)).Sub(fundingAppliedToPositions)
				expectedMarginSellerSecondEpoch := margin.Mul(sdk.NewDec(2)).Add(fundingAppliedToPositions)

				Expect(buyerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
				Expect(buyerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

				Expect(buyerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
				Expect(buyerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginBuyerSecondEpoch.String()))

				Expect(sellerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
				Expect(sellerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

				Expect(sellerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
				Expect(sellerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginSellerSecondEpoch.String()))

				Expect(fundingFirstEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
				Expect(fundingSecondEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingSecondEpoch.String()))
			})
		})

		Describe("when both epochs have negative funding", func() {
			BeforeEach(func() {
				priceFirstEpoch = sdk.NewDec(1990)
				priceSecondEpoch = sdk.NewDec(1980)
				quantityFirstEpoch = sdk.NewDec(1)
				quantitySecondEpoch = sdk.NewDec(1)
				margin = sdk.NewDec(5000)
			})

			It("has correct fundings", func() {
				fundingAppliedToPositions := fundingFirstEpoch.CumulativeFunding.Mul(quantityFirstEpoch)
				expectedMarginBuyerSecondEpoch := margin.Mul(sdk.NewDec(2)).Sub(fundingAppliedToPositions)
				expectedMarginSellerSecondEpoch := margin.Mul(sdk.NewDec(2)).Add(fundingAppliedToPositions)

				Expect(buyerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
				Expect(buyerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

				Expect(buyerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
				Expect(buyerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginBuyerSecondEpoch.String()))

				Expect(sellerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
				Expect(sellerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

				Expect(sellerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
				Expect(sellerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginSellerSecondEpoch.String()))

				Expect(fundingFirstEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
				Expect(fundingSecondEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingSecondEpoch.String()))
			})
		})

		Describe("when epochs have opposite funding", func() {
			Context("and resulting funding is positive", func() {
				BeforeEach(func() {
					priceFirstEpoch = sdk.NewDec(2020)
					priceSecondEpoch = sdk.NewDec(1990)
					quantityFirstEpoch = sdk.NewDec(1)
					quantitySecondEpoch = sdk.NewDec(1)
					margin = sdk.NewDec(5000)
				})

				It("has correct fundings", func() {
					fundingAppliedToPositions := fundingFirstEpoch.CumulativeFunding.Mul(quantityFirstEpoch)
					expectedMarginBuyerSecondEpoch := margin.Mul(sdk.NewDec(2)).Sub(fundingAppliedToPositions)
					expectedMarginSellerSecondEpoch := margin.Mul(sdk.NewDec(2)).Add(fundingAppliedToPositions)

					Expect(buyerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
					Expect(buyerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

					Expect(buyerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
					Expect(buyerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginBuyerSecondEpoch.String()))

					Expect(sellerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
					Expect(sellerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

					Expect(sellerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
					Expect(sellerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginSellerSecondEpoch.String()))

					Expect(fundingFirstEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
					Expect(fundingSecondEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingSecondEpoch.String()))
				})
			})

			Context("and resulting funding is negative", func() {
				BeforeEach(func() {
					priceFirstEpoch = sdk.NewDec(2010)
					priceSecondEpoch = sdk.NewDec(1980)
					quantityFirstEpoch = sdk.NewDec(1)
					quantitySecondEpoch = sdk.NewDec(1)
					margin = sdk.NewDec(5000)
				})

				It("has correct fundings", func() {
					fundingAppliedToPositions := fundingFirstEpoch.CumulativeFunding.Mul(quantityFirstEpoch)
					expectedMarginBuyerSecondEpoch := margin.Mul(sdk.NewDec(2)).Sub(fundingAppliedToPositions)
					expectedMarginSellerSecondEpoch := margin.Mul(sdk.NewDec(2)).Add(fundingAppliedToPositions)

					Expect(buyerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
					Expect(buyerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

					Expect(buyerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
					Expect(buyerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginBuyerSecondEpoch.String()))

					Expect(sellerPositionFirstEpoch.Position.CumulativeFundingEntry.String()).To(Equal(sdk.ZeroDec().String()))
					Expect(sellerPositionFirstEpoch.Position.Margin.String()).To(Equal(margin.String()))

					Expect(sellerPositionSecondEpoch.Position.CumulativeFundingEntry.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
					Expect(sellerPositionSecondEpoch.Position.Margin.String()).To(Equal(expectedMarginSellerSecondEpoch.String()))

					Expect(fundingFirstEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingFirstEpoch.String()))
					Expect(fundingSecondEpoch.CumulativeFunding.String()).To(Equal(expectedCumulativeFundingSecondEpoch.String()))
				})
			})
		})
	})
})
