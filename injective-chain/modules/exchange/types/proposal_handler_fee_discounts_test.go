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

var _ = Describe("Fee Discount Proposal Tests", func() {
	var (
		testInput   testexchange.TestInput
		app         *simapp.InjectiveApp
		ctx         sdk.Context
		err         error
		quoteDenoms []string
		marketCount int
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

			quoteDenoms = append(quoteDenoms, testInput.Perps[i].QuoteDenom)
			quoteDenoms = append(quoteDenoms, testInput.ExpiryMarkets[i].QuoteDenom)
		}
	})

	Describe("when creating a new campaign", func() {
		var proposal exchangetypes.FeeDiscountProposal

		BeforeEach(func() {
			proposal = exchangetypes.FeeDiscountProposal{
				Title:       "Fee Discount",
				Description: "Fee Discount",
				Schedule: &exchangetypes.FeeDiscountSchedule{
					BucketCount:    2,
					BucketDuration: 30,
					QuoteDenoms:    quoteDenoms,
					TierInfos: []*exchangetypes.FeeDiscountTierInfo{
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							StakedAmount:      sdk.NewInt(100),
							Volume:            sdk.MustNewDecFromStr("0.3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							StakedAmount:      sdk.NewInt(1000),
							Volume:            sdk.MustNewDecFromStr("3"),
						},
					},
					DisqualifiedMarketIds: getAllMarketIDsExcept(&[]string{testInput.Perps[0].MarketID.Hex()}),
				},
			}
		})

		It("accepts proposal", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts proposal", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			testexchange.OrFail(err)

			marketIDs, qualifications := app.ExchangeKeeper.GetAllFeeDiscountMarketQualification(ctx)
			qualifiedMarketID := testInput.Perps[0].MarketID

			for i, marketID := range marketIDs {
				if marketID.Hex() == qualifiedMarketID.Hex() {
					Expect(qualifications[i]).To(BeTrue())
					continue
				}

				Expect(qualifications[i]).To(BeFalse())
			}
		})

		It("accepts proposal", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			testexchange.OrFail(err)

			feeDiscountCurrentBucketStartTimestamp := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
			expectedStartTimestamp := ctx.BlockTime().Unix()

			Expect(feeDiscountCurrentBucketStartTimestamp).To(Equal(expectedStartTimestamp))
		})

		It("accepts proposal", func() {
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
			testexchange.OrFail(err)

			feeDiscountSchedule := app.ExchangeKeeper.GetFeeDiscountSchedule(ctx)
			Expect(feeDiscountSchedule).To(Equal(proposal.Schedule))
		})

		Context("when the proposal contains nil schedule", func() {
			JustBeforeEach(func() {
				proposal.Schedule = nil
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new fee discount schedule cannot be nil: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
			})
		})

		Context("when the proposal contains less than two buckets", func() {
			JustBeforeEach(func() {
				proposal.Schedule.BucketCount = 1
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new fee discount schedule must have at least 2 buckets: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
			})
		})

		Context("when the proposal's duration is less than 10 seconds", func() {
			JustBeforeEach(func() {
				proposal.Schedule.BucketDuration = 9
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new fee discount schedule must have have bucket durations of at least 10 seconds: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
			})
		})

		Context("when the proposal contains duplicate quote denoms", func() {
			JustBeforeEach(func() {
				quoteDenoms = append(quoteDenoms, testInput.Perps[0].QuoteDenom)
				proposal.Schedule.QuoteDenoms = quoteDenoms
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new fee discount schedule cannot have duplicate quote denoms: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
			})
		})

		Context("when the proposal contains duplicate disqualified market ids", func() {
			JustBeforeEach(func() {
				disqualifiedMarketIds := append(proposal.Schedule.DisqualifiedMarketIds, testInput.Spots[0].MarketID.Hex())
				proposal.Schedule.DisqualifiedMarketIds = disqualifiedMarketIds
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new fee discount schedule cannot have duplicate disqualified market ids: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
			})
		})

		Context("when the proposal has less no tiers", func() {
			JustBeforeEach(func() {
				proposal.Schedule.TierInfos = nil
			})

			It("does not accept proposal", func() {
				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				err = handler(ctx, (govtypes.Content)(&proposal))
				Expect(err.Error()).To(Equal("new fee discount schedule must have at least one discount tier: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
			})
		})

		Context("when validating the tier infos", func() {
			Context("when the proposal has less no tiers", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos = nil
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("new fee discount schedule must have at least one discount tier: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal has negative maker fee discount rate", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos[0].MakerDiscountRate = sdk.NewDec(-1)
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("MakerDiscountRate must be between 0 and 1: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal has maker fee above 1", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos[0].MakerDiscountRate = sdk.MustNewDecFromStr("1.01")
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("MakerDiscountRate must be between 0 and 1: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal has negative taker fee discount rate", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos[0].TakerDiscountRate = sdk.NewDec(-1)
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("TakerDiscountRate must be between 0 and 1: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal has taker fee above 1", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos[0].TakerDiscountRate = sdk.MustNewDecFromStr("1.01")
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("TakerDiscountRate must be between 0 and 1: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal has negative staked amount", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos[0].StakedAmount = sdk.NewInt(-1)
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("StakedAmount must be non-negative: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal has negative fee paid amount", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos[0].Volume = sdk.NewDec(-1)
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("Volume must be non-negative: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal maker fee discount rates are not in ascending order", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos = []*exchangetypes.FeeDiscountTierInfo{
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							StakedAmount:      sdk.NewInt(100),
							Volume:            sdk.MustNewDecFromStr("0.3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.05"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							StakedAmount:      sdk.NewInt(1000),
							Volume:            sdk.MustNewDecFromStr("3"),
						},
					}
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("successive MakerDiscountRates must be equal or larger than those of lower tiers: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal taker fee discount rates are not in ascending order", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos = []*exchangetypes.FeeDiscountTierInfo{
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							StakedAmount:      sdk.NewInt(100),
							Volume:            sdk.MustNewDecFromStr("0.3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.05"),
							StakedAmount:      sdk.NewInt(1000),
							Volume:            sdk.MustNewDecFromStr("3"),
						},
					}
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("successive TakerDiscountRates must be equal or larger than those of lower tiers: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal staked amounts are not in ascending order", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos = []*exchangetypes.FeeDiscountTierInfo{
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							StakedAmount:      sdk.NewInt(100),
							Volume:            sdk.MustNewDecFromStr("0.3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							StakedAmount:      sdk.NewInt(50),
							Volume:            sdk.MustNewDecFromStr("3"),
						},
					}
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("successive StakedAmount must be equal or larger than those of lower tiers: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})

			Context("when the proposal fee paid amounts are not in ascending order", func() {
				JustBeforeEach(func() {
					proposal.Schedule.TierInfos = []*exchangetypes.FeeDiscountTierInfo{
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
							StakedAmount:      sdk.NewInt(100),
							Volume:            sdk.MustNewDecFromStr("0.3"),
						},
						{
							MakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							TakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
							StakedAmount:      sdk.NewInt(1000),
							Volume:            sdk.MustNewDecFromStr("0.1"),
						},
					}
				})

				It("does not accept proposal", func() {
					handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
					err = handler(ctx, (govtypes.Content)(&proposal))
					Expect(err.Error()).To(Equal("successive Volume must be equal or larger than those of lower tiers: " + exchangetypes.ErrInvalidFeeDiscountSchedule.Error()))
				})
			})
		})
	})
})
