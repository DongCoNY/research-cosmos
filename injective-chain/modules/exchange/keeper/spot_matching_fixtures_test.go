package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = Describe("Spot Matching Fixture Tests", func() {
	var (
		testInput testexchange.TestInput
		app       simapp.InjectiveApp
		ctx       sdk.Context

		spotMarket *exchangetypes.SpotMarket
		err        error
	)

	var runMatchingFixtures = func(hasNegativeMakerFee, isDiscountedFeeRate bool) {
		JustBeforeEach(func() {
			marketIds := make([]string, 0)
			quoteDenoms := make([]string, 0)
			marketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

			marketCount := 3

			for i := 0; i < marketCount; i++ {
				spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[i].MarketID, true)
				negativeMakerFee := sdk.NewDecWithPrec(-3, 1)

				if hasNegativeMakerFee {
					app.ExchangeKeeper.ScheduleSpotMarketParamUpdate(ctx, &exchangetypes.SpotMarketParamUpdateProposal{
						Title:               "Update spot market param",
						Description:         "Update spot market description",
						MarketId:            spotMarket.MarketId,
						MakerFeeRate:        &negativeMakerFee,
						TakerFeeRate:        &spotMarket.TakerFeeRate,
						RelayerFeeShareRate: &spotMarket.RelayerFeeShareRate,
						MinPriceTickSize:    &spotMarket.MinPriceTickSize,
						MinQuantityTickSize: &spotMarket.MinQuantityTickSize,
						Status:              exchangetypes.MarketStatus_Active,
					})
				}

				marketIds = append(marketIds, testInput.Spots[i].MarketID.Hex())
				quoteDenoms = append(quoteDenoms, testInput.Spots[i].QuoteDenom)
				marketMultipliers = append(marketMultipliers, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(1),
					TakerPointsMultiplier: sdk.NewDec(1),
				})
			}

			if hasNegativeMakerFee {
				proposals := make([]exchangetypes.SpotMarketParamUpdateProposal, 0)
				app.ExchangeKeeper.IterateSpotMarketParamUpdates(ctx, func(p *exchangetypes.SpotMarketParamUpdateProposal) (stop bool) {
					proposals = append(proposals, *p)
					return false
				})

				for i := 0; i < marketCount; i++ {
					app.ExchangeKeeper.ExecuteSpotMarketParamUpdateProposal(ctx, &proposals[i])
				}
			}

			testexchange.AddTradeRewardCampaign(testInput, &app, ctx, marketIds, quoteDenoms, marketMultipliers, false, true)

			if isDiscountedFeeRate {
				testexchange.AddFeeDiscount(testInput, &app, ctx, marketIds, quoteDenoms)
			}
		})

		Describe("spot markets limit orders execution", func() {
			DescribeTable("When resting orderbook is empty",
				testexchange.ExpectCorrectSpotOrderbookMatching,
				Entry("Matches all transient correctly", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/empty-resting-orderbook/matching-all.json", 3, isDiscountedFeeRate),
				Entry("Matches partial transient correctly", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/empty-resting-orderbook/matching-partial.json", 3, isDiscountedFeeRate),
				Entry("Matches no transient correctly", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/empty-resting-orderbook/matching-none.json", 3, isDiscountedFeeRate),
			)

			Context("When resting orderbook is not empty", func() {
				// pro-rata choosing random orders to fill based on orderhash which changes when a subaccount creates multiple orders
				// so for simplicity we test pro-rata orders only with the same subaccount
				DescribeTable("When resting buy orders all have same price and transient sell orders' quantity not enough to fill them all",
					testexchange.ExpectCorrectSpotOrderbookMatching,
					Entry("Matches all transient correctly", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/no-pro-rata-filling.json", 3, isDiscountedFeeRate),
				)
				DescribeTable("When transient buy orders all have same price and resting sell orders' quantity not enough to fill them all",
					testexchange.ExpectCorrectSpotOrderbookMatching,
					Entry("Matches only some transient fully", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/no-pro-rata-filling-2.json", 3, isDiscountedFeeRate),
				)

				DescribeTable("When resulting orderbook stays blank",
					testexchange.ExpectCorrectSpotOrderbookMatching,
					Entry(
						"Matches all transient orders fully",
						&testInput,
						&app,
						&ctx,
						"./fixtures/spots/limit-matching/non-empty-resting-orderbook/all-matched.json",
						3,
						isDiscountedFeeRate,
					),
				)

				DescribeTable("When resulting orderbook is not blank",
					testexchange.ExpectCorrectSpotOrderbookMatching,
					Entry("Matches some transient orders fully", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/matching-partial.json", 3, isDiscountedFeeRate),
					Entry("Matches some transient orders fully and some partially", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/matching-partial-2.json", 3, isDiscountedFeeRate),
					Entry("Matches some transient orders fully and some partially", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/matching-partial-3.json", 3, isDiscountedFeeRate),
					Entry("Matches some transient orders fully and some partially", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/matching-partial-4.json", 3, isDiscountedFeeRate),
					Entry("Matches some transient orders fully and some partially", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/matching-partial-5.json", 3, isDiscountedFeeRate),
					Entry("Matches transient orders partially having resting orders on both sides", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/matching-partial-6.json", 3, isDiscountedFeeRate),
				)

				DescribeTable("when integration testing",
					testexchange.ExpectCorrectSpotOrderbookMatching,
					Entry("Matches all correctly", &testInput, &app, &ctx, "./fixtures/spots/integration-tests/all-matched.json", 3, isDiscountedFeeRate),
				)

				DescribeTable("When resting orders are post only",
					testexchange.ExpectCorrectSpotOrderbookMatching,
					Entry("Sell post only orders", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/post-only-sells.json", 3, isDiscountedFeeRate),
					Entry("Buy post only orders", &testInput, &app, &ctx, "./fixtures/spots/limit-matching/non-empty-resting-orderbook/post-only-buys.json", 3, isDiscountedFeeRate),
				)

			})
		})

		Describe("spot markets market orders execution", func() {
			DescribeTable("When resting buy orders all have same price and transient sell orders' quantity not enough to fill them all",
				testexchange.ExpectCorrectSpotOrderbookMatching,
				Entry("Matches all correctly", &testInput, &app, &ctx, "./fixtures/spots/market-matching/all-matched.json", 3, isDiscountedFeeRate),
			)
			DescribeTable("buy market orders are partially matched",
				testexchange.ExpectCorrectSpotOrderbookMatching,
				Entry("when one's worst price is not supported",
					&testInput,
					&app,
					&ctx,
					"./fixtures/spots/market-matching/partial-market-matching/matching-partial-buys.json",
					3,
					isDiscountedFeeRate,
				),
			)

			DescribeTable("sell market orders are partially matched",
				testexchange.ExpectCorrectSpotOrderbookMatching,
				Entry("when one's worst price is not supported",
					&testInput,
					&app,
					&ctx,
					"./fixtures/spots/market-matching/partial-market-matching/matching-partial-sells.json",
					3,
					isDiscountedFeeRate,
				),
			)

			DescribeTable("when there is enough buy liquidity in orderbook",
				testexchange.ExpectCorrectSpotOrderbookMatching,
				Entry("when matching one market sell with one limit buy",
					&testInput,
					&app,
					&ctx,
					"./fixtures/spots/market-matching/one-buy-to-sell.json",
					3,
					isDiscountedFeeRate,
				),

				Entry("when matching one market buy with one limit sell",
					&testInput,
					&app,
					&ctx,
					"./fixtures/spots/market-matching/one-sell-to-buy.json",
					3,
					isDiscountedFeeRate,
				),

				Entry("when matching one market buy with one limit sell",
					&testInput,
					&app,
					&ctx,
					"./fixtures/spots/market-matching/one-buy-to-sell-same-subaccount.json",
					3,
					isDiscountedFeeRate,
				),
			)
		})
	}

	BeforeEach(func() {
		marketCount := 3
		app = *simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})

		testInput, ctx = testexchange.SetupTest(&app, ctx, marketCount, 0, 0)

		marketIds := make([]string, 0)
		quoteDenoms := make([]string, 0)
		marketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

		for i := 0; i < marketCount; i++ {
			_, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[i].Ticker, testInput.Spots[i].BaseDenom, testInput.Spots[i].QuoteDenom, testInput.Spots[i].MinPriceTickSize, testInput.Spots[i].MinQuantityTickSize)
			spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[i].MarketID, true)

			testexchange.OrFail(err)

			marketIds = append(marketIds, testInput.Spots[i].MarketID.Hex())
			quoteDenoms = append(quoteDenoms, testInput.Spots[i].QuoteDenom)
			marketMultipliers = append(marketMultipliers, exchangetypes.PointsMultiplier{
				MakerPointsMultiplier: sdk.NewDec(2),
				TakerPointsMultiplier: sdk.NewDec(2),
			})
		}

		testexchange.AddTradeRewardCampaign(testInput, &app, ctx, marketIds, quoteDenoms, marketMultipliers, true, true)

		funder, _ := sdk.AccAddressFromBech32("inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt")
		rewardTokensCoin := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(500000)))
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewardTokensCoin)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, funder, rewardTokensCoin)
		err = app.DistrKeeper.FundCommunityPool(ctx, rewardTokensCoin, funder)
		testexchange.OrFail(err)

		timestamp := ctx.BlockTime().Unix() + 100 + 10000
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	})

	Describe("when maker fee is positive", func() {
		hasNegativeMakerFee := false

		Describe("when the fees are discounted", func() {
			isDiscountedFeeRate := true
			runMatchingFixtures(hasNegativeMakerFee, isDiscountedFeeRate)
		})

		Describe("when the fees are not discounted", func() {
			isDiscountedFeeRate := false
			runMatchingFixtures(hasNegativeMakerFee, isDiscountedFeeRate)
		})
	})

	Describe("when maker fee is negative", func() {
		hasNegativeMakerFee := true

		// currently markets with negative maker fee do not support discounted fees
		// Describe("when the fees are discounted", func() {
		// 	isDiscountedFeeRate := true
		// 	runMatchingFixtures(hasNegativeMakerFee, isDiscountedFeeRate)
		// })

		Describe("when the fees are not discounted", func() {
			isDiscountedFeeRate := false
			runMatchingFixtures(hasNegativeMakerFee, isDiscountedFeeRate)
		})
	})
})
