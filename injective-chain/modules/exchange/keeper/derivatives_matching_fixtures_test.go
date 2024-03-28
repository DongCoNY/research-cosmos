package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Derivatives Matching Fixture Tests", func() {
	var (
		testInput   testexchange.TestInput
		app         *simapp.InjectiveApp
		ctx         sdk.Context
		err         error
		marketCount int
	)

	var runMatchingFixtures = func(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate bool) {
		JustBeforeEach(func() {
			marketIds := make([]string, 0)
			quoteDenoms := make([]string, 0)
			marketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

			marketCount := 3

			for i := 0; i < marketCount; i++ {
				var marketID common.Hash

				if isTimeExpiry {
					marketID = testInput.ExpiryMarkets[i].MarketID
				} else {
					marketID = testInput.Perps[i].MarketID
				}

				derivativeMarket := app.ExchangeKeeper.GetDerivativeMarket(ctx, marketID, true)
				negativeMakerFee := sdk.NewDecWithPrec(-1, 3)

				if hasNegativeMakerFee {
					app.ExchangeKeeper.ScheduleDerivativeMarketParamUpdate(ctx, &exchangetypes.DerivativeMarketParamUpdateProposal{
						Title:                  "Update Derivative market param",
						Description:            "Update Derivative market description",
						MarketId:               derivativeMarket.MarketId,
						MakerFeeRate:           &negativeMakerFee,
						TakerFeeRate:           &derivativeMarket.TakerFeeRate,
						RelayerFeeShareRate:    &derivativeMarket.RelayerFeeShareRate,
						MinPriceTickSize:       &derivativeMarket.MinPriceTickSize,
						MinQuantityTickSize:    &derivativeMarket.MinQuantityTickSize,
						InitialMarginRatio:     &derivativeMarket.InitialMarginRatio,
						MaintenanceMarginRatio: &derivativeMarket.MaintenanceMarginRatio,
						Status:                 exchangetypes.MarketStatus_Active,
					})
				}

				marketIds = append(marketIds, testInput.Perps[i].MarketID.Hex(), testInput.ExpiryMarkets[i].MarketID.Hex())
				quoteDenoms = append(quoteDenoms, testInput.Perps[i].QuoteDenom, testInput.ExpiryMarkets[i].QuoteDenom)
				marketMultipliers = append(marketMultipliers, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(1),
					TakerPointsMultiplier: sdk.NewDec(1),
				}, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(2),
					TakerPointsMultiplier: sdk.NewDec(2),
				})
			}

			if hasNegativeMakerFee {
				proposals := make([]exchangetypes.DerivativeMarketParamUpdateProposal, 0)
				app.ExchangeKeeper.IterateDerivativeMarketParamUpdates(ctx, func(p *exchangetypes.DerivativeMarketParamUpdateProposal) (stop bool) {
					proposals = append(proposals, *p)
					return false
				})

				for i := 0; i < marketCount; i++ {
					app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &proposals[i])
				}
			}

			testexchange.AddTradeRewardCampaign(testInput, app, ctx, marketIds, quoteDenoms, marketMultipliers, false, false)

			if isDiscountedFeeRate {
				testexchange.AddFeeDiscount(testInput, app, ctx, marketIds, quoteDenoms)
			}
		})

		DescribeTable("When matching derivatives market orders",
			testexchange.ExpectCorrectDerivativeOrderbookMatching,
			Entry("Matches one buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/one-buy-to-one-sell.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches one sell to one buy correctly", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/one-sell-to-one-buy.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches many orders correctly", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/all-matched.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches many orders with partial market buy order fill correctly", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/partial-matched-buys.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches many orders with partial market sell order fill correctly", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/partial-matched-sells.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches via netting and partial close", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/netting-partial-close.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches via netting and full close", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/netting-full-close.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches via netting and partial position reduce", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/netting-partial-reduce.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches via netting with flip", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/netting-with-flip.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches via reduce-only", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/reduce-only.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches one sell to one buy partially correctly", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/partial-matching-netting-reduce-only.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches one sell to one buy partially correctly with flip", &testInput, &app, &ctx, "./fixtures/derivatives/market-matching/partial-matching-netting-with-flip.json", 3, isTimeExpiry, isDiscountedFeeRate),
		)

		DescribeTable("When matching derivatives limit orders",
			testexchange.ExpectCorrectDerivativeOrderbookMatching,
			Entry("Matches one buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/one-buy-to-one-sell.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches one post only buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/one-post-only-buy-to-one-sell.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches one sell to one buy correctly", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/one-sell-to-one-buy.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches many orders correctly", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/all-matched.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches many orders correctly with out of range clearing price", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/all-matched-2.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches many orders partially correctly", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/partial-matched.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches via netting", &testInput, &app, &ctx, "./fixtures/derivatives/limit-matching/netting.json", 3, isTimeExpiry, isDiscountedFeeRate),
		)

		DescribeTable("When matching derivatives limit and market orders",
			testexchange.ExpectCorrectDerivativeOrderbookMatching,
			Entry("Matches one buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/derivatives/integration-tests/all-matched.json", 3, isTimeExpiry, isDiscountedFeeRate),
			Entry("Matches market orders only against resting limit", &testInput, &app, &ctx, "./fixtures/derivatives/integration-tests/match-market-only-against-resting.json", 3, isTimeExpiry, isDiscountedFeeRate),
		)
	}

	BeforeEach(func() {
		marketCount = 3
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, marketCount, 0)

		marketIds := make([]string, 0)
		quoteDenoms := make([]string, 0)
		marketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

		for i := 0; i < marketCount; i++ {
			startingPrice := sdk.NewDec(2000)
			app.OracleKeeper.SetPriceFeedPriceState(ctx, testInput.Perps[i].OracleBase, testInput.Perps[i].OracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			perpCoin := sdk.NewCoin(testInput.Perps[i].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(perpCoin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(perpCoin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				perpCoin,
				testInput.Perps[i].Ticker,
				testInput.Perps[i].QuoteDenom,
				testInput.Perps[i].OracleBase,
				testInput.Perps[i].OracleQuote,
				testInput.Perps[i].OracleType,
				-1,
			))

			_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
				ctx,
				testInput.Perps[i].Ticker,
				testInput.Perps[i].QuoteDenom,
				testInput.Perps[i].OracleBase,
				testInput.Perps[i].OracleQuote,
				0,
				testInput.Perps[i].OracleType,
				testInput.Perps[i].InitialMarginRatio,
				testInput.Perps[i].MaintenanceMarginRatio,
				testInput.Perps[i].MakerFeeRate,
				testInput.Perps[i].TakerFeeRate,
				testInput.Perps[i].MinPriceTickSize,
				testInput.Perps[i].MinQuantityTickSize,
			)
			testexchange.OrFail(err)

			app.OracleKeeper.SetPriceFeedPriceState(ctx, testInput.ExpiryMarkets[i].OracleBase, testInput.ExpiryMarkets[i].OracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			oracleBase, oracleQuote, oracleType := testInput.ExpiryMarkets[i].OracleBase, testInput.ExpiryMarkets[i].OracleQuote, testInput.ExpiryMarkets[i].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			expiryCoin := sdk.NewCoin(testInput.ExpiryMarkets[i].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(expiryCoin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(expiryCoin))

			err = app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				expiryCoin,
				testInput.ExpiryMarkets[i].Ticker,
				testInput.ExpiryMarkets[i].QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				testInput.ExpiryMarkets[i].Expiry,
			)
			testexchange.OrFail(err)

			_, _, err = app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
				ctx,
				testInput.ExpiryMarkets[i].Ticker,
				testInput.ExpiryMarkets[i].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.ExpiryMarkets[i].Expiry,
				testInput.ExpiryMarkets[i].InitialMarginRatio,
				testInput.ExpiryMarkets[i].MaintenanceMarginRatio,
				testInput.ExpiryMarkets[i].MakerFeeRate,
				testInput.ExpiryMarkets[i].TakerFeeRate,
				testInput.ExpiryMarkets[i].MinPriceTickSize,
				testInput.ExpiryMarkets[i].MinQuantityTickSize,
			)
			testexchange.OrFail(err)

			marketIds = append(marketIds, testInput.Perps[i].MarketID.Hex(), testInput.ExpiryMarkets[i].MarketID.Hex())
			quoteDenoms = append(quoteDenoms, testInput.Perps[i].QuoteDenom, testInput.ExpiryMarkets[i].QuoteDenom)
			marketMultipliers = append(marketMultipliers, exchangetypes.PointsMultiplier{
				MakerPointsMultiplier: sdk.NewDec(1),
				TakerPointsMultiplier: sdk.NewDec(1),
			}, exchangetypes.PointsMultiplier{
				MakerPointsMultiplier: sdk.NewDec(2),
				TakerPointsMultiplier: sdk.NewDec(2),
			})
		}

		testexchange.AddTradeRewardCampaign(testInput, app, ctx, marketIds, quoteDenoms, marketMultipliers, true, false)

		marketInfos := app.ExchangeKeeper.GetAllPerpetualMarketInfoStates(ctx)
		for _, marketInfo := range marketInfos {
			marketInfo.HourlyInterestRate = sdk.ZeroDec()
			app.ExchangeKeeper.SetPerpetualMarketInfo(ctx, common.HexToHash(marketInfo.MarketId), &marketInfo)
		}

		funder, _ := sdk.AccAddressFromBech32("inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt")
		rewardTokensCoin := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(500000)))
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewardTokensCoin)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, funder, rewardTokensCoin)
		err = app.DistrKeeper.FundCommunityPool(ctx, rewardTokensCoin, funder)
		testexchange.OrFail(err)

		timestamp := ctx.BlockTime().Unix() + 100 + 10000
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		timestamp = ctx.BlockTime().Unix() + 10000
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	})

	AfterEach(func() {
		Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
	})

	Describe("when maker fee is positive", func() {
		hasNegativeMakerFee := false

		Describe("when its a perp market", func() {
			isTimeExpiry := false

			Describe("when the fees are discounted", func() {
				isDiscountedFeeRate := true
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})

			Describe("when the fees are not discounted", func() {
				isDiscountedFeeRate := false
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})
		})

		Describe("when its a time expiry market", func() {
			isTimeExpiry := true

			Describe("when the fees are discounted", func() {
				isDiscountedFeeRate := true
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})

			Describe("when the fees are not discounted", func() {
				isDiscountedFeeRate := false
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})
		})
	})

	Describe("when maker fee is negative", func() {
		hasNegativeMakerFee := true

		Describe("when its a perp market", func() {
			isTimeExpiry := false

			Describe("when the fees are discounted", func() {
				isDiscountedFeeRate := true
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})

			Describe("when the fees are not discounted", func() {
				isDiscountedFeeRate := false
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})
		})

		Describe("when its a time expiry market", func() {
			isTimeExpiry := true

			Describe("when the fees are discounted", func() {
				isDiscountedFeeRate := true
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})

			Describe("when the fees are not discounted", func() {
				isDiscountedFeeRate := false
				runMatchingFixtures(hasNegativeMakerFee, isTimeExpiry, isDiscountedFeeRate)
			})
		})
	})
})
