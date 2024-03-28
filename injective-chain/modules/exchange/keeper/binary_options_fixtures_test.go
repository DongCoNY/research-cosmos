package keeper_test

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Binary Options Matching Fixture Tests", func() {
	var (
		testInput   testexchange.TestInput
		app         *simapp.InjectiveApp
		ctx         sdk.Context
		err         error
		marketCount int
	)

	var runMatchingFixtures = func(hasNegativeMakerFee, isDiscountedFeeRate bool) {
		JustBeforeEach(func() {
			marketIds := make([]string, 0)
			quoteDenoms := make([]string, 0)
			marketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

			for i := 0; i < marketCount; i++ {
				marketID := testInput.BinaryMarkets[i].MarketID

				boMarket := app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, marketID, true)
				negativeMakerFee := sdk.NewDecWithPrec(-1, 3)

				if hasNegativeMakerFee {
					err = app.ExchangeKeeper.ScheduleBinaryOptionsMarketParamUpdate(ctx, &exchangetypes.BinaryOptionsMarketParamUpdateProposal{
						Title:               "Update binary options market param",
						Description:         "Update binary options market description",
						MarketId:            boMarket.MarketId,
						MakerFeeRate:        &negativeMakerFee,
						TakerFeeRate:        &boMarket.TakerFeeRate,
						RelayerFeeShareRate: &boMarket.RelayerFeeShareRate,
						MinPriceTickSize:    &boMarket.MinPriceTickSize,
						MinQuantityTickSize: &boMarket.MinQuantityTickSize,
						Status:              exchangetypes.MarketStatus_Active,
					})
					testexchange.OrFail(err)
				}

				marketIds = append(marketIds, marketID.Hex())
				quoteDenoms = append(quoteDenoms, testInput.BinaryMarkets[i].QuoteDenom)
				marketMultipliers = append(marketMultipliers, exchangetypes.PointsMultiplier{
					MakerPointsMultiplier: sdk.NewDec(1),
					TakerPointsMultiplier: sdk.NewDec(1),
				})
			}

			if hasNegativeMakerFee {
				proposals := make([]exchangetypes.BinaryOptionsMarketParamUpdateProposal, 0)
				app.ExchangeKeeper.IterateBinaryOptionsMarketParamUpdates(ctx, func(p *exchangetypes.BinaryOptionsMarketParamUpdateProposal) (stop bool) {
					proposals = append(proposals, *p)
					return false
				})

				for i := 0; i < marketCount; i++ {
					err = app.ExchangeKeeper.ExecuteBinaryOptionsMarketParamUpdateProposal(ctx, &proposals[i])
					testexchange.OrFail(err)
				}
			}

			testexchange.AddTradeRewardCampaign(testInput, app, ctx, marketIds, quoteDenoms, marketMultipliers, false, false)

			if isDiscountedFeeRate {
				testexchange.AddFeeDiscount(testInput, app, ctx, marketIds, quoteDenoms)
			}
		})

		DescribeTable("When matching binary options market orders",
			testexchange.ExpectCorrectBinaryOptionsOrderbookMatching,
			Entry("Matches one buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/one-buy-to-one-sell.json", 3, isDiscountedFeeRate),
			Entry("Matches one sell to one buy correctly", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/one-sell-to-one-buy.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders correctly (simple)", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/all-matched-simple.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders correctly", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/all-matched.json", 3, isDiscountedFeeRate),
			Entry("Matches one order with partial market buy order fill correctly (simple)", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/partial-matched-buys-simple.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders with partial market buy order fill correctly", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/partial-matched-buys.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders with partial market sell order fill correctly", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/partial-matched-sells.json", 3, isDiscountedFeeRate),
			Entry("Matches via netting and partial close", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/netting-partial-close.json", 3, isDiscountedFeeRate),
			Entry("Matches via netting and full close", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/netting-full-close.json", 3, isDiscountedFeeRate),
			Entry("Matches via netting and partial position reduce", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/netting-partial-reduce.json", 3, isDiscountedFeeRate),
			Entry("Matches via netting with flip", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/netting-with-flip.json", 3, isDiscountedFeeRate),
			Entry("Matches via reduce-only", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/reduce-only.json", 3, isDiscountedFeeRate),
			Entry("Matches one sell to one buy partially correctly", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/partial-matching-netting-reduce-only.json", 3, isDiscountedFeeRate),
			Entry("Matches one sell to one buy partially correctly with flip", &testInput, &app, &ctx, "./fixtures/binary_options/market-matching/partial-matching-netting-with-flip.json", 3, isDiscountedFeeRate),
		)

		DescribeTable("When matching binary options limit orders",
			testexchange.ExpectCorrectBinaryOptionsOrderbookMatching,
			Entry("Matches one buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/binary_options/limit-matching/one-buy-to-one-sell.json", 3, isDiscountedFeeRate),
			Entry("Matches one sell to one buy correctly", &testInput, &app, &ctx, "./fixtures/binary_options/limit-matching/one-sell-to-one-buy.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders correctly", &testInput, &app, &ctx, "./fixtures/binary_options/limit-matching/all-matched.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders correctly with out of range clearing price", &testInput, &app, &ctx, "./fixtures/binary_options/limit-matching/all-matched-2.json", 3, isDiscountedFeeRate),
			Entry("Matches many orders partially correctly", &testInput, &app, &ctx, "./fixtures/binary_options/limit-matching/partial-matched.json", 3, isDiscountedFeeRate),
			Entry("Matches via netting", &testInput, &app, &ctx, "./fixtures/binary_options/limit-matching/netting.json", 3, isDiscountedFeeRate),
		)

		DescribeTable("When matching binary options limit and market orders",
			testexchange.ExpectCorrectBinaryOptionsOrderbookMatching,
			Entry("Matches one buy to one sell correctly", &testInput, &app, &ctx, "./fixtures/binary_options/integration-tests/all-matched.json", 3, isDiscountedFeeRate),
			Entry("Matches market orders only against resting limit", &testInput, &app, &ctx, "./fixtures/binary_options/integration-tests/match-market-only-against-resting.json", 3, isDiscountedFeeRate),
		)
	}

	BeforeEach(func() {
		marketCount = 3
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 0, marketCount)

		marketIds := make([]string, 0)
		quoteDenoms := make([]string, 0)
		marketMultipliers := make([]exchangetypes.PointsMultiplier, 0)

		for i := 0; i < marketCount; i++ {
			err = app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
				Provider: testInput.BinaryMarkets[i].OracleProvider,
				Relayers: []string{testInput.BinaryMarkets[i].Admin},
			})
			testexchange.OrFail(err)

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			coin := sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			adminAccount, _ := sdk.AccAddressFromBech32(testInput.BinaryMarkets[i].Admin)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, adminAccount, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

			_, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(
				ctx,
				testInput.BinaryMarkets[i].Ticker,
				testInput.BinaryMarkets[i].OracleSymbol,
				testInput.BinaryMarkets[i].OracleProvider,
				oracletypes.OracleType_Provider,
				testInput.BinaryMarkets[i].OracleScaleFactor,
				testInput.BinaryMarkets[i].MakerFeeRate,
				testInput.BinaryMarkets[i].TakerFeeRate,
				testInput.BinaryMarkets[i].ExpirationTimestamp,
				testInput.BinaryMarkets[i].SettlementTimestamp,
				testInput.BinaryMarkets[i].Admin,
				testInput.BinaryMarkets[i].QuoteDenom,
				testInput.BinaryMarkets[i].MinPriceTickSize,
				testInput.BinaryMarkets[i].MinQuantityTickSize,
			)
			testexchange.OrFail(err)

			marketIds = append(marketIds, testInput.BinaryMarkets[i].MarketID.Hex())
			quoteDenoms = append(quoteDenoms, testInput.BinaryMarkets[i].QuoteDenom)
			marketMultipliers = append(marketMultipliers, exchangetypes.PointsMultiplier{
				MakerPointsMultiplier: sdk.NewDec(1),
				TakerPointsMultiplier: sdk.NewDec(1),
			})
		}

		testexchange.AddTradeRewardCampaign(testInput, app, ctx, marketIds, quoteDenoms, marketMultipliers, true, false)

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

		Describe("when the fees are discounted", func() {
			isDiscountedFeeRate := true
			runMatchingFixtures(hasNegativeMakerFee, isDiscountedFeeRate)
		})

		Describe("when the fees are not discounted", func() {
			isDiscountedFeeRate := false
			runMatchingFixtures(hasNegativeMakerFee, isDiscountedFeeRate)
		})
	})
})
