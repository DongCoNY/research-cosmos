package types_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Trading Rewards Proposal Tests", func() {
	var (
		testInput           testexchange.TestInput
		app                 *simapp.InjectiveApp
		ctx                 sdk.Context
		err                 error
		quoteDenoms         []string
		marketCount         int
		newCampaignProposal exchangetypes.TradingRewardCampaignLaunchProposal
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

	BeforeEach(func() {
		marketCount = 3
		quoteDenoms = make([]string, 0)

		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, marketCount, marketCount, 0)

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

	Describe("when creating a new campaign", func() {
		var proposal exchangetypes.TradingRewardCampaignLaunchProposal

		BeforeEach(func() {
			proposal = exchangetypes.TradingRewardCampaignLaunchProposal{
				Title:       "Trade Reward Campaign",
				Description: "Trade Reward Campaign",
				CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				},
				CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
					StartTimestamp:     ctx.BlockTime().Unix() + 30000,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj0", sdk.NewInt(100000))),
				}},
			}
		})

		It("accepts proposal", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			Expect(err).ToNot(HaveOccurred())
		})

		It("sets campaign info", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			testexchange.OrFail(err)

			campaignInfo := app.ExchangeKeeper.GetCampaignInfo(ctx)

			Expect(campaignInfo.CampaignDurationSeconds).To(Equal(proposal.CampaignInfo.CampaignDurationSeconds))
			Expect(campaignInfo.QuoteDenoms).To(Equal(proposal.CampaignInfo.QuoteDenoms))
			Expect(campaignInfo.TradingRewardBoostInfo).To(Equal(proposal.CampaignInfo.TradingRewardBoostInfo))
			Expect(campaignInfo.DisqualifiedMarketIds).To(Equal(proposal.CampaignInfo.DisqualifiedMarketIds))
		})

		It("sets campaign reward pools", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			testexchange.OrFail(err)

			campaignRewardSchedules := app.ExchangeKeeper.GetAllCampaignRewardPools(ctx)
			for idx, campaignRewardSchedule := range campaignRewardSchedules {
				Expect(campaignRewardSchedule).To(Equal(proposal.CampaignRewardPools[idx]))
			}
		})

		It("sets market qualifications", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			testexchange.OrFail(err)

			marketIDs, areQualified := app.ExchangeKeeper.GetAllTradingRewardsMarketQualification(ctx)

			for i, marketID := range marketIDs {
				if marketID == testInput.Perps[0].MarketID {
					Expect(areQualified[i]).To(BeTrue())
					continue
				}

				Expect(areQualified[i]).To(BeFalse())
			}
		})

		Context("when the start timestamp is in the past", func() {
			JustBeforeEach(func() {
				proposal.CampaignRewardPools[0].StartTimestamp = ctx.BlockTime().Unix()
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign start timestamp has already passed: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when a campaign already exists", func() {
			BeforeEach(func() {
				newCampaignProposal = exchangetypes.TradingRewardCampaignLaunchProposal{
					Title:       "Trade Reward Campaign",
					Description: "Trade Reward Campaign",
					CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
						CampaignDurationSeconds: 30000,
						QuoteDenoms:             quoteDenoms,
						TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
							BoostedSpotMarketIds:       nil,
							SpotMarketMultipliers:      nil,
							BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
							DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
								MakerPointsMultiplier: sdk.NewDec(1),
								TakerPointsMultiplier: sdk.NewDec(3),
							}},
						},
						DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
					},
					CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj0", sdk.NewInt(100000))),
					}},
				}
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&newCampaignProposal))
				testexchange.OrFail(err)
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("already existing trading reward campaign: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains invalid quote denoms", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             []string{"wrongBase"},
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPools = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: []sdk.Coin{sdk.NewCoin("inj", sdk.NewInt(100000))},
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("denom wrongBase does not exist in supply: " + exchangetypes.ErrInvalidBaseDenom.Error()))
			})
		})

		Context("when the proposal contains invalid quote denoms", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = nil
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new campaign info cannot be nil: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains invalid quote denoms", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo.CampaignDurationSeconds = 0
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign duration cannot be zero: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains invalid quote denoms", func() {
			JustBeforeEach(func() {
				proposal.CampaignRewardPools = []*exchangetypes.CampaignRewardPool{}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new campaign reward pools cannot be nil: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})
	})

	Describe("when modifying an existing campaign", func() {
		var proposal exchangetypes.TradingRewardCampaignUpdateProposal

		BeforeEach(func() {
			newCampaignProposal = exchangetypes.TradingRewardCampaignLaunchProposal{
				Title:       "Trade Reward Campaign",
				Description: "Trade Reward Campaign",
				CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				},
				CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
					StartTimestamp:     ctx.BlockTime().Unix() + 30000,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj0", sdk.NewInt(100000))),
				}, {
					StartTimestamp:     ctx.BlockTime().Unix() + 60000,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj1", sdk.NewInt(200000))),
				}, {
					StartTimestamp:     ctx.BlockTime().Unix() + 90000,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj2", sdk.NewInt(300000))),
				}},
			}
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&newCampaignProposal))
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			proposal = exchangetypes.TradingRewardCampaignUpdateProposal{
				Title:       "Trade Reward Campaign",
				Description: "Trade Reward Campaign",
				CampaignInfo: &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				},
			}
		})

		Describe("when adding new reward pools", func() {
			BeforeEach(func() {
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{{
					StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj1", sdk.NewInt(100000))),
				}}
			})

			Context("when the proposal is valid with values", func() {
				JustBeforeEach(func() {
					proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
						CampaignDurationSeconds: 30000,
						QuoteDenoms:             quoteDenoms,
						TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
							BoostedSpotMarketIds:       nil,
							SpotMarketMultipliers:      nil,
							BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
							DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
								MakerPointsMultiplier: sdk.NewDec(1),
								TakerPointsMultiplier: sdk.NewDec(3),
							}},
						},
						DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
					}
				})

				It("accepts proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err).ToNot(HaveOccurred())
				})

				It("sets campaign info", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					campaignInfo := app.ExchangeKeeper.GetCampaignInfo(ctx)

					Expect(campaignInfo.CampaignDurationSeconds).To(Equal(proposal.CampaignInfo.CampaignDurationSeconds))
					Expect(campaignInfo.QuoteDenoms).To(Equal(proposal.CampaignInfo.QuoteDenoms))
					Expect(campaignInfo.TradingRewardBoostInfo).To(Equal(proposal.CampaignInfo.TradingRewardBoostInfo))
					Expect(campaignInfo.DisqualifiedMarketIds).To(Equal(proposal.CampaignInfo.DisqualifiedMarketIds))
				})

				It("sets campaign reward pools", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					combinedCampaignRewardPools := append(newCampaignProposal.CampaignRewardPools, proposal.CampaignRewardPoolsAdditions...)

					campaignRewardSchedules := app.ExchangeKeeper.GetAllCampaignRewardPools(ctx)
					for idx, campaignRewardSchedule := range campaignRewardSchedules {
						Expect(campaignRewardSchedule).To(Equal(combinedCampaignRewardPools[idx]))
					}
				})

				It("sets market qualifications", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					marketIDs, areQualified := app.ExchangeKeeper.GetAllTradingRewardsMarketQualification(ctx)

					for i, marketID := range marketIDs {
						if marketID == testInput.Perps[0].MarketID {
							Expect(areQualified[i]).To(BeTrue())
							continue
						}

						Expect(areQualified[i]).To(BeFalse())
					}
				})
			})

			Context("when the campaign start time is not matching", func() {
				JustBeforeEach(func() {
					proposal.CampaignRewardPoolsAdditions[0].StartTimestamp = ctx.BlockTime().Unix() + 30000*2 - 1
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("reward pool addition start timestamp not matching campaign duration: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
				})
			})
		})

		Describe("when modifying existing reward pools", func() {
			BeforeEach(func() {
				proposal.CampaignRewardPoolsUpdates = []*exchangetypes.CampaignRewardPool{{
					StartTimestamp:     ctx.BlockTime().Unix() + 60000,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj1", sdk.NewInt(200000))),
				}, {
					StartTimestamp:     ctx.BlockTime().Unix() + 90000,
					MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj2", sdk.NewInt(100000))),
				}}
			})

			Context("when the proposal is valid with values", func() {
				JustBeforeEach(func() {
					proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
						CampaignDurationSeconds: 30000,
						QuoteDenoms:             quoteDenoms,
						TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
							BoostedSpotMarketIds:       nil,
							SpotMarketMultipliers:      nil,
							BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
							DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
								MakerPointsMultiplier: sdk.NewDec(1),
								TakerPointsMultiplier: sdk.NewDec(3),
							}},
						},
						DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
					}
				})

				It("accepts proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err).ToNot(HaveOccurred())
				})

				It("sets campaign info", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					campaignInfo := app.ExchangeKeeper.GetCampaignInfo(ctx)

					Expect(campaignInfo.CampaignDurationSeconds).To(Equal(proposal.CampaignInfo.CampaignDurationSeconds))
					Expect(campaignInfo.QuoteDenoms).To(Equal(proposal.CampaignInfo.QuoteDenoms))
					Expect(campaignInfo.TradingRewardBoostInfo).To(Equal(proposal.CampaignInfo.TradingRewardBoostInfo))
					Expect(campaignInfo.DisqualifiedMarketIds).To(Equal(proposal.CampaignInfo.DisqualifiedMarketIds))
				})

				It("sets campaign reward pools", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					combinedCampaignRewardPools := append(
						[]*exchangetypes.CampaignRewardPool{newCampaignProposal.CampaignRewardPools[0]},
						proposal.CampaignRewardPoolsUpdates...,
					)

					campaignRewardSchedules := app.ExchangeKeeper.GetAllCampaignRewardPools(ctx)
					for idx, campaignRewardSchedule := range campaignRewardSchedules {
						Expect(campaignRewardSchedule).To(Equal(combinedCampaignRewardPools[idx]))
					}
				})

				It("sets market qualifications", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					marketIDs, areQualified := app.ExchangeKeeper.GetAllTradingRewardsMarketQualification(ctx)

					for i, marketID := range marketIDs {
						if marketID == testInput.Perps[0].MarketID {
							Expect(areQualified[i]).To(BeTrue())
							continue
						}

						Expect(areQualified[i]).To(BeFalse())
					}
				})
			})
		})

		Context("when the proposal is valid with values for existing campaign", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj1", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*5,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj2", sdk.NewInt(200000))),
					},
				}
			})

			It("accepts proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err).ToNot(HaveOccurred())
			})

			It("sets campaign info", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				testexchange.OrFail(err)

				campaignInfo := app.ExchangeKeeper.GetCampaignInfo(ctx)
				Expect(campaignInfo.CampaignDurationSeconds).To(Equal(proposal.CampaignInfo.CampaignDurationSeconds))
				Expect(campaignInfo.QuoteDenoms).To(Equal(proposal.CampaignInfo.QuoteDenoms))
				Expect(campaignInfo.TradingRewardBoostInfo).To(Equal(proposal.CampaignInfo.TradingRewardBoostInfo))
				Expect(campaignInfo.DisqualifiedMarketIds).To(Equal(proposal.CampaignInfo.DisqualifiedMarketIds))
			})

			It("sets campaign reward pools", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				testexchange.OrFail(err)

				combinedCampaignRewardPools := append(newCampaignProposal.CampaignRewardPools, proposal.CampaignRewardPoolsAdditions...)
				campaignRewardSchedules := app.ExchangeKeeper.GetAllCampaignRewardPools(ctx)
				for idx, campaignRewardSchedule := range campaignRewardSchedules {
					Expect(campaignRewardSchedule).To(Equal(combinedCampaignRewardPools[idx]))
				}
			})

			It("sets market qualifications", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				testexchange.OrFail(err)

				marketIDs, areQualified := app.ExchangeKeeper.GetAllTradingRewardsMarketQualification(ctx)

				for i, marketID := range marketIDs {
					if marketID == testInput.Perps[0].MarketID {
						Expect(areQualified[i]).To(BeTrue())
						continue
					}

					Expect(areQualified[i]).To(BeFalse())
				}
			})
		})

		Context("when the campaigns are nil", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = nil
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign info cannot be nil: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the campaigns are nil", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 2,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo:  nil,
					DisqualifiedMarketIds:   nil,
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign duration does not match existing campaign: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the campaigns are nil", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 3000,
					QuoteDenoms:             nil,
					TradingRewardBoostInfo:  nil,
					DisqualifiedMarketIds:   nil,
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign quote denoms cannot be nil: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the campaigns are nil", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 3000,
					QuoteDenoms:             []string{},
					TradingRewardBoostInfo:  nil,
					DisqualifiedMarketIds:   nil,
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign quote denoms cannot be nil: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the campaign timestamps are not multiples of the interval", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 200 + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 200 + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("reward pool addition start timestamp not matching campaign duration: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the campaign timestamps are not multiples of the interval", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*2,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*2 + 1000,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("start timestamps not matching campaign duration: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the campaign timestamps are not in ascending order", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 100,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("reward pool start timestamps must be in ascending order: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the spot maker multiplier is negative", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex()},
						SpotMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(-1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
						BoostedDerivativeMarketIds:  nil,
						DerivativeMarketMultipliers: nil,
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("spot market maker multiplier cannot be negative: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the spot taker multiplier is negative", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex()},
						SpotMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(-3),
						}},
						BoostedDerivativeMarketIds:  nil,
						DerivativeMarketMultipliers: nil,
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("spot market taker multiplier cannot be negative: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the derivative maker multiplier is negative", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(-1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("derivative market maker multiplier cannot be negative: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the derivative taker multiplier is negative", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(-3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("derivative market taker multiplier cannot be negative: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains duplicate spot market ids", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds: []string{testInput.Spots[0].MarketID.Hex(), testInput.Spots[0].MarketID.Hex()},
						SpotMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}, {
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
						BoostedDerivativeMarketIds:  nil,
						DerivativeMarketMultipliers: nil,
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign contains duplicate boosted market ids: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains duplicate derivative market ids", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}, {
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign contains duplicate boosted market ids: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains duplicate derivative market ids", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: []string{testInput.Perps[0].MarketID.Hex(), testInput.Perps[0].MarketID.Hex()},
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("campaign contains duplicate disqualified market ids: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains duplicate reward denoms", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: []sdk.Coin{sdk.NewCoin("inj", sdk.NewInt(100000)), sdk.NewCoin("inj", sdk.NewInt(100000))},
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("reward pool campaign contains duplicate market coins: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains zero reward amount", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             quoteDenoms,
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: []sdk.Coin{sdk.NewCoin("inj", sdk.NewInt(100000)), sdk.NewCoin("inj2", sdk.NewInt(0))},
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("reward pool contains zero or nil reward amount: " + exchangetypes.ErrInvalidTradingRewardCampaign.Error()))
			})
		})

		Context("when the proposal contains invalid quote denoms", func() {
			JustBeforeEach(func() {
				proposal.CampaignInfo = &exchangetypes.TradingRewardCampaignInfo{
					CampaignDurationSeconds: 30000,
					QuoteDenoms:             []string{"wrongBase"},
					TradingRewardBoostInfo: &exchangetypes.TradingRewardCampaignBoostInfo{
						BoostedSpotMarketIds:       nil,
						SpotMarketMultipliers:      nil,
						BoostedDerivativeMarketIds: []string{testInput.Perps[0].MarketID.Hex()},
						DerivativeMarketMultipliers: []exchangetypes.PointsMultiplier{{
							MakerPointsMultiplier: sdk.NewDec(1),
							TakerPointsMultiplier: sdk.NewDec(3),
						}},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				}
				proposal.CampaignRewardPoolsAdditions = []*exchangetypes.CampaignRewardPool{
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*3,
						MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(200000))),
					},
					{
						StartTimestamp:     ctx.BlockTime().Unix() + 30000*4,
						MaxCampaignRewards: []sdk.Coin{sdk.NewCoin("inj", sdk.NewInt(100000))},
					},
				}
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("denom wrongBase does not exist in supply: " + exchangetypes.ErrInvalidBaseDenom.Error()))
			})
		})

		Describe("when updating reward pool points", func() {
			var proposal exchangetypes.TradingRewardPendingPointsUpdateProposal

			BeforeEach(func() {
				proposal = exchangetypes.TradingRewardPendingPointsUpdateProposal{
					Title:       "Trade Reward Points Update",
					Description: "Trade Reward Points Update",
					RewardPointUpdates: []*exchangetypes.RewardPointUpdate{{
						AccountAddress: testexchange.DefaultAddress,
						NewPoints:      sdk.NewDec(100),
					}, {
						AccountAddress: testexchange.DefaultValidatorDelegatorAddress,
						NewPoints:      sdk.NewDec(200),
					}},
					PendingPoolTimestamp: ctx.BlockTime().Unix() + 50000,
				}
				app.ExchangeKeeper.SetCampaignRewardPendingPool(ctx, &exchangetypes.CampaignRewardPool{
					StartTimestamp: proposal.PendingPoolTimestamp,
				})
			})

			Context("when the proposal is valid with values", func() {
				It("accepts proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err).ToNot(HaveOccurred())
				})

				It("changes the points", func() {
					tradingRewardPoints := exchangetypes.TradingRewardPoints{
						testexchange.DefaultAddress:                   sdk.NewDec(800),
						testexchange.DefaultValidatorDelegatorAddress: sdk.NewDec(1000),
					}
					app.ExchangeKeeper.PersistTradingRewardPendingPoints(ctx, tradingRewardPoints, proposal.PendingPoolTimestamp)

					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					testexchange.OrFail(err)

					totalPoints := app.ExchangeKeeper.GetTotalTradingRewardPendingPoints(ctx, proposal.PendingPoolTimestamp)

					account1, err1 := sdk.AccAddressFromBech32(testexchange.DefaultAddress)
					testexchange.OrFail(err1)
					account2, err2 := sdk.AccAddressFromBech32(testexchange.DefaultValidatorDelegatorAddress)
					testexchange.OrFail(err2)

					currentPoints1 := app.ExchangeKeeper.GetCampaignTradingRewardPendingPoints(ctx, account1, proposal.PendingPoolTimestamp)
					currentPoints2 := app.ExchangeKeeper.GetCampaignTradingRewardPendingPoints(ctx, account2, proposal.PendingPoolTimestamp)

					Expect(totalPoints.String()).To(Equal((sdk.NewDec(300).String())))
					Expect(currentPoints1.String()).To(Equal((sdk.NewDec(100).String())))
					Expect(currentPoints2.String()).To(Equal((sdk.NewDec(200).String())))
				})

				Context("when trading points already exist", func() {
					BeforeEach(func() {
						handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
						err = handler(ctx, (govtypes.Content)(&proposal))
						testexchange.OrFail(err)

						proposal = exchangetypes.TradingRewardPendingPointsUpdateProposal{
							Title:       "Trade Reward Points Update",
							Description: "Trade Reward Points Update",
							RewardPointUpdates: []*exchangetypes.RewardPointUpdate{{
								AccountAddress: testexchange.DefaultAddress,
								NewPoints:      sdk.NewDec(0),
							}, {
								AccountAddress: testexchange.DefaultValidatorDelegatorAddress,
								NewPoints:      sdk.NewDec(0),
							}},
							PendingPoolTimestamp: ctx.BlockTime().Unix() + 50000,
						}
					})

					It("changes the points", func() {
						handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
						err = handler(ctx, (govtypes.Content)(&proposal))

						totalPoints := app.ExchangeKeeper.GetTotalTradingRewardPendingPoints(ctx, proposal.PendingPoolTimestamp)

						account1, err1 := sdk.AccAddressFromBech32(testexchange.DefaultAddress)
						testexchange.OrFail(err1)
						account2, err2 := sdk.AccAddressFromBech32(testexchange.DefaultValidatorDelegatorAddress)
						testexchange.OrFail(err2)

						currentPoints1 := app.ExchangeKeeper.GetCampaignTradingRewardPendingPoints(ctx, account1, proposal.PendingPoolTimestamp)
						currentPoints2 := app.ExchangeKeeper.GetCampaignTradingRewardPendingPoints(ctx, account2, proposal.PendingPoolTimestamp)

						Expect(totalPoints.String()).To(Equal((sdk.NewDec(0).String())))
						Expect(currentPoints1.String()).To(Equal((sdk.NewDec(0).String())))
						Expect(currentPoints2.String()).To(Equal((sdk.NewDec(0).String())))
					})
				})
			})
		})
	})
})
