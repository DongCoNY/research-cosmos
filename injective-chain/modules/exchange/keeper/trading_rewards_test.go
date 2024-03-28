package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var (
	spotBuyer      = testexchange.SampleSubaccountAddr1
	spotSeller     = testexchange.SampleSubaccountAddr2
	perpBuyer      = testexchange.SampleSubaccountAddr3
	perpSeller     = testexchange.SampleSubaccountAddr4
	expiryBuyer    = testexchange.SampleSubaccountAddr5
	expirySeller   = testexchange.SampleSubaccountAddr6
	allSubaccounts = []common.Hash{spotBuyer, spotSeller, perpBuyer, perpSeller, expiryBuyer, expirySeller}
	// spotBuyer    = common.HexToHash("127aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
	// spotSeller   = common.HexToHash("227aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
	// perpBuyer    = common.HexToHash("327aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
	// perpSeller   = common.HexToHash("427aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
	// expiryBuyer  = common.HexToHash("527aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
	// expirySeller = common.HexToHash("627aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
)

func fundCommunityPool(app *simapp.InjectiveApp, ctx sdk.Context, testInput testexchange.TestInput, injReward, inj2Reward math.Int) {
	funder, _ := sdk.AccAddressFromBech32("inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt")

	rewardTokensCoin1 := sdk.NewCoins(sdk.NewCoin("inj", injReward))
	app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewardTokensCoin1)
	app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, funder, rewardTokensCoin1)
	err := app.DistrKeeper.FundCommunityPool(ctx, rewardTokensCoin1, funder)
	testexchange.OrFail(err)

	rewardTokensCoin2 := sdk.NewCoins(sdk.NewCoin("inj2", inj2Reward))
	app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewardTokensCoin2)
	app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, funder, rewardTokensCoin2)
	err = app.DistrKeeper.FundCommunityPool(ctx, rewardTokensCoin2, funder)
	testexchange.OrFail(err)
}

func expectCorrectRewardTokenPayoutsForSubaccount(
	ctx sdk.Context,
	app *simapp.InjectiveApp,
	testInput testexchange.TestInput,
	subaccount common.Hash,
	injReward, inj2Reward sdk.Dec,
	rewardRate sdk.Dec,
	injRewardFromPreviousCampaign,
	stakedAmount math.Int,
) {
	coins := app.BankKeeper.GetAllBalances(ctx, sdk.AccAddress(common.FromHex(subaccount.String()[2:42])))
	hasReceivedINJ := false
	hasReceivedINJ2 := false

	expectedReceivedInj := injReward.Mul(rewardRate).TruncateInt().Add(injRewardFromPreviousCampaign)
	expectedReceivedInj2 := inj2Reward.Mul(rewardRate).TruncateInt()

	maxInjReward := math.NewIntWithDecimal(100, 18) // 100 INJ

	if expectedReceivedInj.GT(maxInjReward) {
		expectedReceivedInj = sdk.MaxInt(maxInjReward, sdk.MinInt(expectedReceivedInj, stakedAmount))
	}

	for _, coin := range coins {
		if coin.Denom == "inj" {
			hasReceivedINJ = true
			Expect(coin.Amount.String()).To(Equal(expectedReceivedInj.String()))
		}

		if coin.Denom == "inj2" {
			hasReceivedINJ2 = true
			Expect(coin.Amount.String()).To(Equal(expectedReceivedInj2.String()))
		}
	}

	if expectedReceivedInj.IsPositive() {
		Expect(hasReceivedINJ).To(BeTrue())
	}

	if expectedReceivedInj2.IsPositive() {
		Expect(hasReceivedINJ2).To(BeTrue())
	}
}

type expectedRewards struct {
	shareRate               sdk.Dec
	injFromPreviousCampaign math.Int
}

func expectCorrectRewardTokenPayouts(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	testInput testexchange.TestInput,
	spotBuyerRewards, spotSellerRewards expectedRewards,
	perpBuyerRewards, perpSellerRewards expectedRewards,
	expiryBuyerRewards, expirySellerRewards expectedRewards,
	injReward, inj2Reward sdk.Dec,
	stakedAmount math.Int,
) {
	expectCorrectRewardTokenPayoutsForSubaccount(ctx, app, testInput, spotBuyer, injReward, inj2Reward, spotBuyerRewards.shareRate, spotBuyerRewards.injFromPreviousCampaign, sdk.ZeroInt())
	expectCorrectRewardTokenPayoutsForSubaccount(ctx, app, testInput, spotSeller, injReward, inj2Reward, spotSellerRewards.shareRate, spotSellerRewards.injFromPreviousCampaign, sdk.ZeroInt())
	expectCorrectRewardTokenPayoutsForSubaccount(ctx, app, testInput, perpBuyer, injReward, inj2Reward, perpBuyerRewards.shareRate, perpBuyerRewards.injFromPreviousCampaign, sdk.ZeroInt())
	expectCorrectRewardTokenPayoutsForSubaccount(ctx, app, testInput, perpSeller, injReward, inj2Reward, perpSellerRewards.shareRate, perpSellerRewards.injFromPreviousCampaign, stakedAmount)
	expectCorrectRewardTokenPayoutsForSubaccount(ctx, app, testInput, expiryBuyer, injReward, inj2Reward, expiryBuyerRewards.shareRate, expiryBuyerRewards.injFromPreviousCampaign, sdk.ZeroInt())
	expectCorrectRewardTokenPayoutsForSubaccount(ctx, app, testInput, expirySeller, injReward, inj2Reward, expirySellerRewards.shareRate, expirySellerRewards.injFromPreviousCampaign, sdk.ZeroInt())
}

var _ = Describe("Trading Rewards Tests", func() {
	var (
		testInput    testexchange.TestInput
		app          *simapp.InjectiveApp
		ctx          sdk.Context
		err          error
		quoteDenoms  []string
		marketCount  = 3
		stakedAmount = sdk.ZeroInt()
		zeroRewards  = expectedRewards{
			shareRate:               sdk.NewDec(0),
			injFromPreviousCampaign: sdk.ZeroInt(),
		}
	)

	var getAllMarketIDsExcept = func(excludedMarketIDs *[]string) []string {
		marketIDs := make([]string, 0)

		for i := 0; i < marketCount; i++ {
			spotMarketID := testInput.Spots[i].MarketID.Hex()
			perpMarketID := testInput.Perps[i].MarketID.Hex()
			expiryMarketID := testInput.ExpiryMarkets[i].MarketID.Hex()

			if !exchangetypes.StringInSlice(spotMarketID, excludedMarketIDs) {
				marketIDs = append(marketIDs, spotMarketID)
			}

			if !exchangetypes.StringInSlice(perpMarketID, excludedMarketIDs) {
				marketIDs = append(marketIDs, perpMarketID)
			}

			if !exchangetypes.StringInSlice(expiryMarketID, excludedMarketIDs) {
				marketIDs = append(marketIDs, expiryMarketID)
			}
		}

		return marketIDs
	}

	var initAndStartCampaigns = func(content exchangetypes.TradingRewardCampaignLaunchProposal) {
		proposalMsg, err := govtypesv1.NewLegacyContent(&content, app.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String())
		Expect(err).To(BeNil())
		handler := app.MsgServiceRouter().Handler(proposalMsg)
		_, err = handler(ctx, proposalMsg)
		testexchange.OrFail(err)

		currentCampaignStartedTimestamp := ctx.BlockTime().Unix() + 200
		ctx = ctx.WithBlockTime(time.Unix(currentCampaignStartedTimestamp, 0))
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}

	var distributePendingRewards = func() {
		timestamp := ctx.BlockTime().Unix() + 16000
		ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))

		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}

	var simulateTrading = func() {
		msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

		price := sdk.NewDec(2000)
		quantity := sdk.NewDec(3)
		margin := sdk.NewDec(1000)

		// testInput.AddSpotDeposits(app, ctx, marketCount)
		testInput.AddSpotDepositsForSubaccounts(app, ctx, marketCount, nil, allSubaccounts)
		testInput.AddDerivativeDepositsForSubaccounts(app, ctx, marketCount, nil, true, allSubaccounts)
		testInput.AddDerivativeDepositsForSubaccounts(app, ctx, marketCount, nil, false, allSubaccounts)
		// testInput.AddDerivativeDeposits(app, ctx, marketCount, false)
		// testInput.AddDerivativeDeposits(app, ctx, marketCount, true)

		for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
			spotLimitBuyOrderMsg := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(price, quantity, exchangetypes.OrderType_BUY, spotBuyer, marketIndex)
			_, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitBuyOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			spotLimitSellOrderMsg := testInput.NewMsgCreateSpotLimitOrderForMarketIndex(price, quantity, exchangetypes.OrderType_SELL, spotSeller, marketIndex)
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitSellOrderMsg)
			testexchange.OrFail(err)

			perpLimitBuyOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(
				testexchange.DefaultAddress,
				price,
				quantity,
				margin,
				exchangetypes.OrderType_BUY,
				perpBuyer,
				marketIndex,
				false,
			)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), perpLimitBuyOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			perpLimilSellOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(
				testexchange.DefaultAddress,
				price,
				quantity,
				margin,
				exchangetypes.OrderType_SELL,
				perpSeller,
				marketIndex,
				false,
			)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), perpLimilSellOrderMsg)
			testexchange.OrFail(err)

			expiryLimitBuyOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(
				testexchange.DefaultAddress,
				price,
				quantity,
				margin,
				exchangetypes.OrderType_BUY,
				expiryBuyer,
				marketIndex,
				true,
			)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), expiryLimitBuyOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			expiryLimitSellOrderMsg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(
				testexchange.DefaultAddress,
				price,
				quantity,
				margin,
				exchangetypes.OrderType_SELL,
				expirySeller,
				marketIndex,
				true,
			)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), expiryLimitSellOrderMsg)
			testexchange.OrFail(err)
		}
		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		currentCampaignEndTimestamp := app.ExchangeKeeper.GetCurrentCampaignEndTimestamp(ctx)
		ctx = ctx.WithBlockTime(time.Unix(currentCampaignEndTimestamp, 0))

		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}

	BeforeEach(func() {
		marketCount = 3
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, marketCount, marketCount, marketCount)

		quoteDenoms = make([]string, 0)

		for i := 0; i < marketCount; i++ {
			_, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[i].Ticker, testInput.Spots[i].BaseDenom, testInput.Spots[i].QuoteDenom, testInput.Spots[i].MinPriceTickSize, testInput.Spots[i].MinQuantityTickSize)
			testexchange.OrFail(err)

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

			quoteDenoms = append(quoteDenoms, testInput.Spots[i].QuoteDenom)
			quoteDenoms = append(quoteDenoms, testInput.Perps[i].QuoteDenom)
			quoteDenoms = append(quoteDenoms, testInput.ExpiryMarkets[i].QuoteDenom)
		}
	})

	Describe("when a trade rewards campaign is running", func() {
		var injReward, inj2Reward, missingFundsInCommunityPool math.Int
		var proposal exchangetypes.TradingRewardCampaignLaunchProposal

		Context("when the community pool is sufficiently funded", func() {
			JustBeforeEach(func() {
				fundCommunityPool(app, ctx, testInput, injReward, inj2Reward)
				missingFundsInCommunityPool = sdk.ZeroInt()
			})

			Context("when the campaign has only one market and denom", func() {
				BeforeEach(func() {
					injReward = sdk.NewInt(100000)
					inj2Reward = sdk.NewInt(0)

					marketIds := []string{testInput.Perps[0].MarketID.Hex()}
					proposal = exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds:       nil,
								SpotMarketMultipliers:      nil,
								BoostedDerivativeMarketIds: marketIds,
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(3),
								}},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}
				})

				Context("when no traders are registered DMMs", func() {
					It("should distribute rewards based on weights", func() {
						initAndStartCampaigns(proposal)
						simulateTrading()
						distributePendingRewards()

						perpBuyerRewards := expectedRewards{
							// perp buyer rate =  makerPointsMultiplier / (makerPointsMultiplier + takerPointsMultiplier) = 1 / (1 + 3) = 0.25
							shareRate:               sdk.MustNewDecFromStr("0.25"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}
						perpSellerRewards := expectedRewards{
							// perp seller rate = takerPointsMultiplier / (takerPointsMultiplier + takerPointsMultiplier) = 3 / (1 + 3) = 0.75
							shareRate:               sdk.MustNewDecFromStr("0.75"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}

						expectCorrectRewardTokenPayouts(
							app,
							ctx,
							testInput,
							zeroRewards,
							zeroRewards,
							perpBuyerRewards,
							perpSellerRewards,
							zeroRewards,
							zeroRewards,
							injReward.ToDec(),
							inj2Reward.ToDec(),
							stakedAmount,
						)
					})
				})

				Context("when all traders are registered DMMs", func() {
					BeforeEach(func() {
						app.ExchangeKeeper.SetIsOptedOutOfRewards(ctx, exchangetypes.SubaccountIDToSdkAddress(spotBuyer), true)
						app.ExchangeKeeper.SetIsOptedOutOfRewards(ctx, exchangetypes.SubaccountIDToSdkAddress(spotSeller), true)
						app.ExchangeKeeper.SetIsOptedOutOfRewards(ctx, exchangetypes.SubaccountIDToSdkAddress(perpBuyer), true)
						app.ExchangeKeeper.SetIsOptedOutOfRewards(ctx, exchangetypes.SubaccountIDToSdkAddress(perpSeller), true)
						app.ExchangeKeeper.SetIsOptedOutOfRewards(ctx, exchangetypes.SubaccountIDToSdkAddress(expiryBuyer), true)
						app.ExchangeKeeper.SetIsOptedOutOfRewards(ctx, exchangetypes.SubaccountIDToSdkAddress(expirySeller), true)
					})

					It("should distribute no rewards", func() {
						initAndStartCampaigns(proposal)
						simulateTrading()
						distributePendingRewards()

						perpBuyerRewards := expectedRewards{
							shareRate:               sdk.ZeroDec(),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}
						perpSellerRewards := expectedRewards{
							shareRate:               sdk.ZeroDec(),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}

						expectCorrectRewardTokenPayouts(
							app,
							ctx,
							testInput,
							zeroRewards,
							zeroRewards,
							perpBuyerRewards,
							perpSellerRewards,
							zeroRewards,
							zeroRewards,
							injReward.ToDec(),
							inj2Reward.ToDec(),
							stakedAmount,
						)
					})
				})

				Context("when there are multiple reward pools pending", func() {
					BeforeEach(func() {
						proposal.CampaignRewardPools = append(
							proposal.CampaignRewardPools,
							&exchangetypes.CampaignRewardPool{
								MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
								StartTimestamp:     ctx.BlockTime().Unix() + 30100,
							},
						)
						initAndStartCampaigns(proposal)
					})

					It("should distribute rewards based on weights for both campaigns", func() {
						currentCampaignEndTimestamp := app.ExchangeKeeper.GetCurrentCampaignEndTimestamp(ctx)

						simulateTrading()

						newTimestamp := currentCampaignEndTimestamp + 100000
						ctx = ctx.WithBlockTime(time.Unix(newTimestamp, 0))

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						perpBuyerRewards := expectedRewards{
							// perp buyer rate =  makerPointsMultiplier / (makerPointsMultiplier + takerPointsMultiplier) = 1 / (1 + 3) = 0.25
							shareRate:               sdk.MustNewDecFromStr("0.25"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}
						perpSellerRewards := expectedRewards{
							// perp seller rate = takerPointsMultiplier / (takerPointsMultiplier + takerPointsMultiplier) = 3 / (1 + 3) = 0.75
							shareRate:               sdk.MustNewDecFromStr("0.75"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}

						expectCorrectRewardTokenPayouts(
							app,
							ctx,
							testInput,
							zeroRewards,
							zeroRewards,
							perpBuyerRewards,
							perpSellerRewards,
							zeroRewards,
							zeroRewards,
							injReward.ToDec(),
							inj2Reward.ToDec(),
							stakedAmount,
						)

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						expectCorrectRewardTokenPayouts(
							app,
							ctx,
							testInput,
							zeroRewards,
							zeroRewards,
							perpBuyerRewards,
							perpSellerRewards,
							zeroRewards,
							zeroRewards,
							injReward.ToDec(),
							inj2Reward.ToDec(),
							stakedAmount,
						)
					})
				})
			})

			Context("when the campaign has only market and denom with high inj rewards", func() {
				BeforeEach(func() {
					injReward = math.NewIntWithDecimal(100_000, 18) // 100,000 INJ
					inj2Reward = sdk.NewInt(0)

					marketIds := []string{testInput.Perps[0].MarketID.Hex()}
					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds:       nil,
								SpotMarketMultipliers:      nil,
								BoostedDerivativeMarketIds: marketIds,
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(3),
								}},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					perpBuyerRewards := expectedRewards{
						// perp buyer rate =  makerPointsMultiplier / (makerPointsMultiplier + takerPointsMultiplier) = 1 / (1 + 3) = 0.25
						shareRate:               sdk.MustNewDecFromStr("0.25"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = takerPointsMultiplier / (takerPointsMultiplier + takerPointsMultiplier) = 3 / (1 + 3) = 0.75
						shareRate:               sdk.MustNewDecFromStr("0.75"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						zeroRewards,
						zeroRewards,
						perpBuyerRewards,
						perpSellerRewards,
						zeroRewards,
						zeroRewards,
						injReward.ToDec(),
						inj2Reward.ToDec(),
						stakedAmount,
					)
				})
			})

			Context("when the campaign has only market and denom with high inj rewards and perp seller staked inj", func() {
				BeforeEach(func() {
					injReward = math.NewIntWithDecimal(100_000, 18) // 100,000 INJ
					inj2Reward = sdk.NewInt(0)

					marketIds := []string{testInput.Perps[0].MarketID.Hex()}
					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds:       nil,
								SpotMarketMultipliers:      nil,
								BoostedDerivativeMarketIds: marketIds,
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(3),
								}},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					stakedAmount = math.NewIntWithDecimal(40_000, 18)
					// only perp seller stakes
					testexchange.DelegateStake(app, ctx, testexchange.SampleAccountAddrStr4, "inj", stakedAmount)

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					perpBuyerRewards := expectedRewards{
						// perp buyer rate =  makerPointsMultiplier / (makerPointsMultiplier + takerPointsMultiplier) = 1 / (1 + 3) = 0.25
						shareRate:               sdk.MustNewDecFromStr("0.25"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = takerPointsMultiplier / (takerPointsMultiplier + takerPointsMultiplier) = 3 / (1 + 3) = 0.75
						shareRate:               sdk.MustNewDecFromStr("0.75"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						zeroRewards,
						zeroRewards,
						perpBuyerRewards,
						perpSellerRewards,
						zeroRewards,
						zeroRewards,
						injReward.ToDec(),
						inj2Reward.ToDec(),
						stakedAmount,
					)
				})
			})

			Context("when the campaign has a market with negative maker fee", func() {
				BeforeEach(func() {
					injReward = sdk.NewInt(100000)
					inj2Reward = sdk.NewInt(0)

					derivativeMarket := app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)
					negativeMakerFee := sdk.NewDecWithPrec(-1, 3)
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
					proposals := make([]exchangetypes.DerivativeMarketParamUpdateProposal, 0)
					app.ExchangeKeeper.IterateDerivativeMarketParamUpdates(ctx, func(p *exchangetypes.DerivativeMarketParamUpdateProposal) (stop bool) {
						proposals = append(proposals, *p)
						return false
					})
					app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &proposals[0])

					marketIds := []string{testInput.Perps[0].MarketID.Hex()}
					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds:       nil,
								SpotMarketMultipliers:      nil,
								BoostedDerivativeMarketIds: marketIds,
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(3),
								}},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					perpBuyerRewards := expectedRewards{
						// perp buyer rate =  makerPointsMultiplier / (makerPointsMultiplier + takerPointsMultiplier) = 1 / (1 + 3) = 0.25
						shareRate:               sdk.MustNewDecFromStr("0.25"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = takerPointsMultiplier / (takerPointsMultiplier + takerPointsMultiplier) = 3 / (1 + 3) = 0.75
						shareRate:               sdk.MustNewDecFromStr("0.75"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						zeroRewards,
						zeroRewards,
						perpBuyerRewards,
						perpSellerRewards,
						zeroRewards,
						zeroRewards,
						injReward.ToDec(),
						inj2Reward.ToDec(),
						stakedAmount,
					)
				})
			})

			Context("when the campaign has multiple markets with different weights and denoms", func() {
				BeforeEach(func() {
					injReward = sdk.NewInt(100000)
					inj2Reward = sdk.NewInt(2000)

					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[1].MarketID.Hex(), testInput.Spots[2].MarketID.Hex()},
								SpotMarketMultipliers: []exchangetypes.PointsMultiplier{
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
								},
								BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[1].MarketID.Hex(), testInput.ExpiryMarkets[0].MarketID.Hex()},
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
								},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{
								testInput.Spots[0].MarketID.Hex(),
								testInput.Spots[1].MarketID.Hex(),
								testInput.Spots[2].MarketID.Hex(),
								testInput.Perps[0].MarketID.Hex(),
								testInput.Perps[1].MarketID.Hex(),
								testInput.ExpiryMarkets[0].MarketID.Hex(),
							}),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward), sdk.NewCoin("inj2", inj2Reward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					spotBuyerRewards := expectedRewards{
						// spot buyer rate = 18/90 = 0.2
						shareRate:               sdk.MustNewDecFromStr("0.2"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					spotSellerRewards := expectedRewards{
						// spot seller rate = 36/90 = 0.4
						shareRate:               sdk.MustNewDecFromStr("0.4"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpBuyerRewards := expectedRewards{
						// perp buyer rate = 12/90 = 0.1333333333333
						shareRate:               sdk.MustNewDecFromStr("0.1333333333333"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = 12/90 = 0.1333333333333
						shareRate:               sdk.MustNewDecFromStr("0.1333333333333"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					expiryBuyerRewards := expectedRewards{
						// expiry buyer rate = 6/90 = 0.06666
						shareRate:               sdk.MustNewDecFromStr("0.06666"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					expirySellerRewards := expectedRewards{
						// expiry seller rate = 6/90 = 0.06666
						shareRate:               sdk.MustNewDecFromStr("0.06666"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						spotBuyerRewards,
						spotSellerRewards,
						perpBuyerRewards,
						perpSellerRewards,
						expiryBuyerRewards,
						expirySellerRewards,
						injReward.ToDec(),
						inj2Reward.ToDec(),
						stakedAmount,
					)
				})
			})

			Context("when the campaign has multiple markets with one heavy-weight market", func() {
				BeforeEach(func() {
					injReward = sdk.NewInt(100000)
					inj2Reward = sdk.NewInt(2000)
					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[1].MarketID.Hex(), testInput.Spots[2].MarketID.Hex()},
								SpotMarketMultipliers: []exchangetypes.PointsMultiplier{
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
								},
								BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[1].MarketID.Hex(), testInput.ExpiryMarkets[0].MarketID.Hex()},
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(5),
										TakerPointsMultiplier: sdk.NewDec(3),
									},
								},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{
								testInput.Spots[0].MarketID.Hex(),
								testInput.Spots[1].MarketID.Hex(),
								testInput.Spots[2].MarketID.Hex(),
								testInput.Perps[0].MarketID.Hex(),
								testInput.Perps[1].MarketID.Hex(),
								testInput.ExpiryMarkets[0].MarketID.Hex(),
							}),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward), sdk.NewCoin("inj2", inj2Reward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					spotBuyerRewards := expectedRewards{
						// spot buyer rate = 0.1428571429
						shareRate:               sdk.MustNewDecFromStr("0.1428571429"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					spotSellerRewards := expectedRewards{
						// spot seller rate = 0.2857142857
						shareRate:               sdk.MustNewDecFromStr("0.2857142857"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpBuyerRewards := expectedRewards{
						// perp buyer rate = 0.09523809524
						shareRate:               sdk.MustNewDecFromStr("0.09523809524"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = 0.09523809524
						shareRate:               sdk.MustNewDecFromStr("0.09523809524"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					expiryBuyerRewards := expectedRewards{
						// expiry buyer rate = 0.2380952381
						shareRate:               sdk.MustNewDecFromStr("0.2380952381"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					expirySellerRewards := expectedRewards{
						// expiry seller rate = 0.1428571429
						shareRate:               sdk.MustNewDecFromStr("0.1428571429"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						spotBuyerRewards,
						spotSellerRewards,
						perpBuyerRewards,
						perpSellerRewards,
						expiryBuyerRewards,
						expirySellerRewards,
						injReward.ToDec(),
						inj2Reward.ToDec(),
						stakedAmount,
					)
				})
			})
		})

		Context("when the community pool is not sufficiently funded", func() {
			JustBeforeEach(func() {
				injRewardsInPool := sdk.MaxInt(injReward.Sub(missingFundsInCommunityPool), sdk.ZeroInt())
				inj2RewardsInPool := sdk.MaxInt(inj2Reward.Sub(missingFundsInCommunityPool), sdk.ZeroInt())

				fundCommunityPool(app, ctx, testInput, injRewardsInPool, inj2RewardsInPool)
			})

			Context("when the campaign has only one market and denom and missing only one token", func() {
				BeforeEach(func() {
					injReward = sdk.NewInt(100000)
					inj2Reward = sdk.NewInt(0)
					missingFundsInCommunityPool = sdk.NewInt(1)

					marketIds := []string{testInput.Perps[0].MarketID.Hex()}
					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds:       nil,
								SpotMarketMultipliers:      nil,
								BoostedDerivativeMarketIds: marketIds,
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(3),
								}},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					perpBuyerRewards := expectedRewards{
						// perp buyer rate =  makerPointsMultiplier / (makerPointsMultiplier + takerPointsMultiplier) = 1 / (1 + 3) = 0.25
						shareRate:               sdk.MustNewDecFromStr("0.25"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = takerPointsMultiplier / (takerPointsMultiplier + takerPointsMultiplier) = 3 / (1 + 3) = 0.75
						shareRate:               sdk.MustNewDecFromStr("0.75"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						zeroRewards,
						zeroRewards,
						perpBuyerRewards,
						perpSellerRewards,
						zeroRewards,
						zeroRewards,
						injReward.Sub(missingFundsInCommunityPool).ToDec(),
						inj2Reward.Sub(missingFundsInCommunityPool).ToDec(),
						stakedAmount,
					)
				})
			})

			Context("when the campaign has multiple markets with different weights and denoms and missing almost all funds for one denom", func() {
				BeforeEach(func() {
					injReward = sdk.NewInt(100000)
					inj2Reward = sdk.NewInt(2000)
					missingFundsInCommunityPool = sdk.NewInt(1990)

					proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
						Title:       "Trade Reward Campaign",
						Description: "Trade Reward Campaign",
						CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
							CampaignDurationSeconds: 30000,
							QuoteDenoms:             quoteDenoms,
							TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
								BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[1].MarketID.Hex(), testInput.Spots[2].MarketID.Hex()},
								SpotMarketMultipliers: []exchangetypes.PointsMultiplier{
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(2),
									},
								},
								BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[1].MarketID.Hex(), testInput.ExpiryMarkets[0].MarketID.Hex()},
								DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
									{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(1),
									},
								},
							},
							DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{
								testInput.Spots[0].MarketID.Hex(),
								testInput.Spots[1].MarketID.Hex(),
								testInput.Spots[2].MarketID.Hex(),
								testInput.Perps[0].MarketID.Hex(),
								testInput.Perps[1].MarketID.Hex(),
								testInput.ExpiryMarkets[0].MarketID.Hex(),
							}),
						},
						CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
							MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward), sdk.NewCoin("inj2", inj2Reward)),
							StartTimestamp:     ctx.BlockTime().Unix() + 100,
						}},
					}

					initAndStartCampaigns(proposal)
				})

				It("should distribute rewards based on weights", func() {
					simulateTrading()
					distributePendingRewards()

					spotBuyerRewards := expectedRewards{
						// spot buyer rate = 18/90 = 0.2
						shareRate:               sdk.MustNewDecFromStr("0.2"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					spotSellerRewards := expectedRewards{
						// spot seller rate = 38/90 = 0.4
						shareRate:               sdk.MustNewDecFromStr("0.4"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpBuyerRewards := expectedRewards{
						// perp buyer rate = 12/90 = 0.1333333333333
						shareRate:               sdk.MustNewDecFromStr("0.1333333333334"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					perpSellerRewards := expectedRewards{
						// perp seller rate = 12/90 = 0.1333333333333
						shareRate:               sdk.MustNewDecFromStr("0.1333333333334"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					expiryBuyerRewards := expectedRewards{
						// expiry buyer rate = 6/90 = 0.066667
						shareRate:               sdk.MustNewDecFromStr("0.066667"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}
					expirySellerRewards := expectedRewards{
						// expiry seller rate = 6/90 = 0.066667
						shareRate:               sdk.MustNewDecFromStr("0.066667"),
						injFromPreviousCampaign: sdk.ZeroInt(),
					}

					expectCorrectRewardTokenPayouts(
						app,
						ctx,
						testInput,
						spotBuyerRewards,
						spotSellerRewards,
						perpBuyerRewards,
						perpSellerRewards,
						expiryBuyerRewards,
						expirySellerRewards,
						injReward.Sub(missingFundsInCommunityPool).ToDec(),
						inj2Reward.Sub(missingFundsInCommunityPool).ToDec(),
						stakedAmount,
					)
				})
			})

			Context("when rolling over campaigns", func() {
				Context("when the community pool is sufficiently funded", func() {
					BeforeEach(func() {
						injReward = sdk.NewInt(100000)
						inj2Reward = sdk.NewInt(2000)

						fundCommunityPool(app, ctx, testInput, injReward.MulRaw(2), inj2Reward.MulRaw(2))
						missingFundsInCommunityPool = sdk.ZeroInt()

						marketIds := []string{testInput.Perps[0].MarketID.Hex()}
						proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
							Title:       "Trade Reward Campaign",
							Description: "Trade Reward Campaign",
							CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
								CampaignDurationSeconds: 30000,

								QuoteDenoms: quoteDenoms,
								TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
									BoostedSpotMarketIds:       nil,
									SpotMarketMultipliers:      nil,
									BoostedDerivativeMarketIds: marketIds,
									DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
										MakerPointsMultiplier: sdk.NewDec(1),
										TakerPointsMultiplier: sdk.NewDec(3),
									}},
								},
								DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
							},
							CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
								MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
								StartTimestamp:     ctx.BlockTime().Unix() + 100,
							}},
						}

						initAndStartCampaigns(proposal)
						simulateTrading()
						distributePendingRewards()

						proposal = exchangetypes.TradingRewardCampaignLaunchProposal{
							Title:       "Trade Reward Campaign",
							Description: "Trade Reward Campaign",
							CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
								CampaignDurationSeconds: 30000,

								QuoteDenoms: quoteDenoms,
								TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
									BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[1].MarketID.Hex(), testInput.Spots[2].MarketID.Hex()},
									SpotMarketMultipliers: []exchangetypes.PointsMultiplier{
										{
											MakerPointsMultiplier: sdk.NewDec(1),
											TakerPointsMultiplier: sdk.NewDec(2),
										},
										{
											MakerPointsMultiplier: sdk.NewDec(1),
											TakerPointsMultiplier: sdk.NewDec(2),
										},
										{
											MakerPointsMultiplier: sdk.NewDec(1),
											TakerPointsMultiplier: sdk.NewDec(2),
										},
									},
									BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[1].MarketID.Hex(), testInput.ExpiryMarkets[0].MarketID.Hex()},
									DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{
										{
											MakerPointsMultiplier: sdk.NewDec(1),
											TakerPointsMultiplier: sdk.NewDec(1),
										},
										{
											MakerPointsMultiplier: sdk.NewDec(1),
											TakerPointsMultiplier: sdk.NewDec(1),
										},
										{
											MakerPointsMultiplier: sdk.NewDec(1),
											TakerPointsMultiplier: sdk.NewDec(1),
										},
									},
								},
								DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{
									testInput.Spots[0].MarketID.Hex(),
									testInput.Spots[1].MarketID.Hex(),
									testInput.Spots[2].MarketID.Hex(),
									testInput.Perps[0].MarketID.Hex(),
									testInput.Perps[1].MarketID.Hex(),
									testInput.ExpiryMarkets[0].MarketID.Hex(),
								}),
							},
							CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
								MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward), sdk.NewCoin("inj2", inj2Reward)),
								StartTimestamp:     ctx.BlockTime().Unix() + 100,
							}},
						}

						initAndStartCampaigns(proposal)
					})

					It("should distribute rewards based on weights for both campaigns", func() {
						simulateTrading()
						distributePendingRewards()

						perpBuyerInjRewardFromPreviousCampaign := injReward.QuoRaw(4)
						perpSellerInjRewardFromPreviousCampaign := injReward.QuoRaw(4).MulRaw(3)

						spotBuyerRewards := expectedRewards{
							// spot buyer rate = 18/90 = 0.2
							shareRate:               sdk.MustNewDecFromStr("0.2"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}
						spotSellerRewards := expectedRewards{
							// spot seller rate = 38/90 = 0.5
							shareRate:               sdk.MustNewDecFromStr("0.4"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}
						perpBuyerRewards := expectedRewards{
							// perp buyer rate = 12/90 = 0.1333333333333
							shareRate:               sdk.MustNewDecFromStr("0.1333333333333"),
							injFromPreviousCampaign: perpBuyerInjRewardFromPreviousCampaign,
						}
						perpSellerRewards := expectedRewards{
							// perp seller rate = 12/90 = 0.1333333333333
							shareRate:               sdk.MustNewDecFromStr("0.1333333333333"),
							injFromPreviousCampaign: perpSellerInjRewardFromPreviousCampaign,
						}
						expiryBuyerRewards := expectedRewards{
							// expiry buyer rate = 6/90 = 0.06666
							shareRate:               sdk.MustNewDecFromStr("0.06666"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}
						expirySellerRewards := expectedRewards{
							// expiry seller rate 6/90 = 0.06666
							shareRate:               sdk.MustNewDecFromStr("0.06666"),
							injFromPreviousCampaign: sdk.ZeroInt(),
						}

						expectCorrectRewardTokenPayouts(
							app,
							ctx,
							testInput,
							spotBuyerRewards,
							spotSellerRewards,
							perpBuyerRewards,
							perpSellerRewards,
							expiryBuyerRewards,
							expirySellerRewards,
							injReward.ToDec(),
							inj2Reward.ToDec(),
							stakedAmount,
						)
					})
				})
			})
		})

		Context("when the community pool is not sufficiently funded", func() {
			BeforeEach(func() {
				injReward = sdk.NewInt(100000)
				inj2Reward = sdk.NewInt(2000)

				missingFundsInCommunityPool = sdk.NewInt(200)
				injRewardsInPool := injReward.MulRaw(2).Sub(missingFundsInCommunityPool)
				fundCommunityPool(app, ctx, testInput, injRewardsInPool, inj2Reward.MulRaw(2))

				marketIds := []string{testInput.Perps[0].MarketID.Hex()}
				proposal := exchangetypes.TradingRewardCampaignLaunchProposal{
					Title:       "Trade Reward Campaign",
					Description: "Trade Reward Campaign",
					CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
						CampaignDurationSeconds: 30000,
						QuoteDenoms:             quoteDenoms,
						TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
							BoostedSpotMarketIds:       nil,
							SpotMarketMultipliers:      nil,
							BoostedDerivativeMarketIds: marketIds,
							DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
								MakerPointsMultiplier: sdk.NewDec(1),
								TakerPointsMultiplier: sdk.NewDec(3),
							}},
						},
						DisqualifiedMarketIds: getAllMarketIDsExcept(&marketIds),
					},
					CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward)),
						StartTimestamp:     ctx.BlockTime().Unix() + 100,
					}},
				}

				initAndStartCampaigns(proposal)
				simulateTrading()
				distributePendingRewards()

				proposal = exchangetypes.TradingRewardCampaignLaunchProposal{
					Title:       "Trade Reward Campaign",
					Description: "Trade Reward Campaign",
					CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
						CampaignDurationSeconds: 30000,
						QuoteDenoms:             quoteDenoms,
						TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
							BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[1].MarketID.Hex(), testInput.Spots[2].MarketID.Hex()},
							SpotMarketMultipliers: []exchangetypes.PointsMultiplier{
								{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(2),
								},
								{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(2),
								},
								{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(2),
								},
							},
							BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[1].MarketID.Hex(), testInput.ExpiryMarkets[0].MarketID.Hex()},
							DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{
								{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(1),
								},
								{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(1),
								},
								{
									MakerPointsMultiplier: sdk.NewDec(1),
									TakerPointsMultiplier: sdk.NewDec(1),
								},
							},
						},
						DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{
							testInput.Spots[0].MarketID.Hex(),
							testInput.Spots[1].MarketID.Hex(),
							testInput.Spots[2].MarketID.Hex(),
							testInput.Perps[0].MarketID.Hex(),
							testInput.Perps[1].MarketID.Hex(),
							testInput.ExpiryMarkets[0].MarketID.Hex(),
						}),
					},
					CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", injReward), sdk.NewCoin("inj2", inj2Reward)),
						StartTimestamp:     ctx.BlockTime().Unix() + 100,
					}},
				}

				initAndStartCampaigns(proposal)
			})

			It("should distribute rewards based on weights for both campaigns", func() {
				simulateTrading()
				distributePendingRewards()

				perpBuyerInjRewardFromPreviousCampaign := injReward.QuoRaw(4)
				perpSellerInjRewardFromPreviousCampaign := injReward.QuoRaw(4).MulRaw(3)

				spotBuyerRewards := expectedRewards{
					// spot buyer rate = 18/90 = 0.2
					shareRate:               sdk.MustNewDecFromStr("0.2"),
					injFromPreviousCampaign: sdk.ZeroInt(),
				}
				spotSellerRewards := expectedRewards{
					// spot seller rate = 38/90 = 0.5
					shareRate:               sdk.MustNewDecFromStr("0.4"),
					injFromPreviousCampaign: sdk.ZeroInt(),
				}
				perpBuyerRewards := expectedRewards{
					// perp buyer rate = 12/90 = 0.1333333333333
					shareRate:               sdk.MustNewDecFromStr("0.1333333333333"),
					injFromPreviousCampaign: perpBuyerInjRewardFromPreviousCampaign,
				}
				perpSellerRewards := expectedRewards{
					// perp seller rate = 12/90 = 0.1333333333333
					shareRate:               sdk.MustNewDecFromStr("0.1333333333333"),
					injFromPreviousCampaign: perpSellerInjRewardFromPreviousCampaign,
				}
				expiryBuyerRewards := expectedRewards{
					// expiry buyer rate = 6/90 = 0.066667
					shareRate:               sdk.MustNewDecFromStr("0.066667"),
					injFromPreviousCampaign: sdk.ZeroInt(),
				}
				expirySellerRewards := expectedRewards{
					// expiry seller rate = 6/90 = 0.066667
					shareRate:               sdk.MustNewDecFromStr("0.066667"),
					injFromPreviousCampaign: sdk.ZeroInt(),
				}

				expectCorrectRewardTokenPayouts(
					app,
					ctx,
					testInput,
					spotBuyerRewards,
					spotSellerRewards,
					perpBuyerRewards,
					perpSellerRewards,
					expiryBuyerRewards,
					expirySellerRewards,
					injReward.Sub(missingFundsInCommunityPool).ToDec(),
					inj2Reward.ToDec(),
					stakedAmount,
				)
			})
		})
	})
})
