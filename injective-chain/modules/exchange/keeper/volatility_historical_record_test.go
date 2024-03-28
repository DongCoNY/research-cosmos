package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Volatility Historical Market Trades Test", func() {
	// state
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		testInput testexchange.TestInput

		spotMarket testexchange.SpotMarket
		perpMarket testexchange.PerpMarket
	)

	// test fixtures
	var (
		one = sdk.MustNewDecFromStr("1000000000000000000")

		timeZero = time.Unix(1600000000, 0).UTC().Unix()

		spotTradeRecord = types.TradeRecord{
			Timestamp: timeZero,
			Price:     sdk.MustNewDecFromStr("5000000000000000000"),
			Quantity:  one,
		}

		perpTradeRecord = types.TradeRecord{
			Timestamp: timeZero,
			Price:     sdk.MustNewDecFromStr("990000000000000000"),
			Quantity:  one,
		}

		spotTradeRecord2 = types.TradeRecord{
			Timestamp: spotTradeRecord.Timestamp + 1,
			Price:     spotTradeRecord.Price.Add(one),
		}

		perpTradeRecord2 = types.TradeRecord{
			Timestamp: perpTradeRecord.Timestamp + 1,
			Price:     perpTradeRecord.Price.Add(one),
		}
	)

	var _ = BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(timeZero+30, 0)})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 1, 0)

		spotMarket = testInput.Spots[0]
		perpMarket = testInput.Perps[0]

		_, err := app.ExchangeKeeper.SpotMarketLaunch(ctx, spotMarket.Ticker, spotMarket.BaseDenom, spotMarket.QuoteDenom, spotMarket.MinPriceTickSize, spotMarket.MinQuantityTickSize)
		testexchange.OrFail(err)

		app.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			perpMarket.OracleBase,
			perpMarket.OracleQuote,
			oracletypes.NewPriceState(sdk.NewDec(2000), ctx.BlockTime().Unix()),
		)

		sender, _ := sdk.AccAddressFromBech32("inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt")
		coin := sdk.NewCoin(perpMarket.QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

		err = app.InsuranceKeeper.CreateInsuranceFund(
			ctx,
			sender,
			coin,
			perpMarket.Ticker,
			perpMarket.QuoteDenom,
			perpMarket.OracleBase,
			perpMarket.OracleQuote,
			perpMarket.OracleType,
			-1,
		)
		testexchange.OrFail(err)

		_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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
		testexchange.OrFail(err)
	})

	Describe("Persistence of Historical Price Records", func() {
		When("No historical price record persists", func() {
			It("Returns empty record", func() {
				entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, spotMarket.MarketID, 0)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestTradeRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())

				entry, omitted = app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, perpMarket.MarketID, 0)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestTradeRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())
			})

			It("Cleanup is a no-op", func() {
				app.ExchangeKeeper.CleanupHistoricalTradeRecords(ctx)
			})
		})

		When("Have price records", func() {
			BeforeEach(func() {
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord)
				app.ExchangeKeeper.AppendTradeRecord(ctx, perpMarket.MarketID, &perpTradeRecord)
			})

			It("Returns records for a Spot Market", func() {
				entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, spotMarket.MarketID, 0)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestTradeRecords).Should(HaveLen(1)) {
						Expect(entry.LatestTradeRecords[0].Price).Should(Equal(spotTradeRecord.Price))
						Expect(entry.LatestTradeRecords[0].Timestamp).Should(Equal(spotTradeRecord.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())
			})

			It("Returns records for a Perp Market", func() {
				entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, perpMarket.MarketID, 0)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestTradeRecords).Should(HaveLen(1)) {
						Expect(entry.LatestTradeRecords[0].Price).Should(Equal(perpTradeRecord.Price))
						Expect(entry.LatestTradeRecords[0].Timestamp).Should(Equal(perpTradeRecord.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())
			})
		})
	})

	Describe("Price volatility", func() {
		When("Have one price record", func() {
			BeforeEach(func() {
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord)
			})

			It("Returns zero volatility", func() {
				vol, rawHistory, meta := app.ExchangeKeeper.GetMarketVolatility(ctx, spotMarket.MarketID, &types.TradeHistoryOptions{
					MaxAge:            0,
					IncludeRawHistory: true,
					IncludeMetadata:   true,
				})
				Expect(vol.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(len(rawHistory)).Should(Equal(1))
				Expect(meta.FirstTimestamp).Should(Equal(spotTradeRecord.Timestamp))
				Expect(meta.LastTimestamp).Should(Equal(spotTradeRecord.Timestamp))
				Expect(meta.GroupCount).Should(Equal(uint32(1)))
				Expect(meta.RecordsSampleSize).Should(Equal(uint32(1)))
				Expect(meta.MinPrice.String()).Should(Equal(spotTradeRecord.Price.String()))
				Expect(meta.MaxPrice.String()).Should(Equal(spotTradeRecord.Price.String()))
				Expect(meta.Mean.String()).Should(Equal(spotTradeRecord.Price.String()))
				Expect(meta.Twap.String()).Should(Equal(sdk.ZeroDec().String())) // twap is 0 when only 1 record
			})
		})

		When("Have multiple price records", func() {
			var (
				spotTradeRecord3 = types.TradeRecord{
					Timestamp: timeZero,
					Price:     sdk.MustNewDecFromStr("3000000000000000000"),
					Quantity:  one,
				}
				spotTradeRecord4 = types.TradeRecord{
					Timestamp: timeZero + 15,
					Price:     sdk.MustNewDecFromStr("4500000000000000000"),
					Quantity:  one,
				}
			)

			BeforeEach(func() {
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord)
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord3)
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord4)
			})

			It("Returns expected volatility for a Spot Market", func() {
				vol, rawTrades, meta := app.ExchangeKeeper.GetMarketVolatility(ctx, spotMarket.MarketID, &types.TradeHistoryOptions{
					TradeGroupingSec:  5,
					IncludeMetadata:   true,
					IncludeRawHistory: true,
				})
				Expect(vol.String()).Should(Equal(sdk.MustNewDecFromStr("235702260395515841.466948120701616346").String()))
				Expect(len(rawTrades)).Should(Equal(3))
				Expect(meta.Mean.String()).Should(Equal(sdk.MustNewDecFromStr("4166666666666666666.666666666666666667").String()))
				Expect(meta.MinPrice.String()).Should(Equal(spotTradeRecord3.Price.String()))
				Expect(meta.MaxPrice.String()).Should(Equal(spotTradeRecord.Price.String()))
				Expect(meta.Twap.String()).Should(Equal(spotTradeRecord4.Price.String())) // since we have only two groups, twap == second group price
			})

			When("No HistoryOptions provided", func() {
				It("Returns only volatility", func() {
					vol, rawTrades, meta := app.ExchangeKeeper.GetMarketVolatility(ctx, spotMarket.MarketID, nil)
					Expect(vol.String()).Should(Equal(sdk.MustNewDecFromStr("235702260395515841.466948120701616346").String()))
					Expect(len(rawTrades)).Should(Equal(0))
					Expect(meta).Should(BeNil())
				})
			})

			It("MaxAge allows only last trade", func() {
				vol, rawTrades, _ := app.ExchangeKeeper.GetMarketVolatility(ctx, spotMarket.MarketID, &types.TradeHistoryOptions{
					MaxAge:            15,
					IncludeRawHistory: true,
				})
				Expect(vol.String()).Should(Equal(sdk.ZeroDec().String()))
				if Expect(len(rawTrades)).Should(Equal(1)) {
					Expect(rawTrades[0].Price.String()).Should(Equal(spotTradeRecord4.Price.String()))
				}
			})
		})
	})

	Describe("Time filtering and cleanup of Historical Price Records", func() {
		Context("Omitting expired trade records", func() {
			BeforeEach(func() {
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord)
				app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord2)
				app.ExchangeKeeper.AppendTradeRecord(ctx, perpMarket.MarketID, &perpTradeRecord)
				app.ExchangeKeeper.AppendTradeRecord(ctx, perpMarket.MarketID, &perpTradeRecord2)
			})

			When("Some records expired", func() {
				var from int64

				BeforeEach(func() {
					from = spotTradeRecord.Timestamp + 1
				})

				It("Omits expired", func() {
					entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, spotMarket.MarketID, from)
					if Expect(entry).ShouldNot(BeNil()) {
						if Expect(entry.LatestTradeRecords).Should(HaveLen(1)) {
							Expect(entry.LatestTradeRecords[0].Price).Should(Equal(spotTradeRecord2.Price))
							Expect(entry.LatestTradeRecords[0].Timestamp).Should(Equal(spotTradeRecord2.Timestamp))
						}
					}
					Expect(omitted).Should(BeTrue())

					entry, omitted = app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, perpMarket.MarketID, from)
					if Expect(entry).ShouldNot(BeNil()) {
						if Expect(entry.LatestTradeRecords).Should(HaveLen(1)) {
							Expect(entry.LatestTradeRecords[0].Price).Should(Equal(perpTradeRecord2.Price))
							Expect(entry.LatestTradeRecords[0].Timestamp).Should(Equal(perpTradeRecord2.Timestamp))
						}
					}
					Expect(omitted).Should(BeTrue())
				})
			})

			When("All records expired", func() {
				var from int64

				BeforeEach(func() {
					from = spotTradeRecord.Timestamp + 2
				})

				It("Omits all", func() {
					entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, spotMarket.MarketID, from)
					Expect(entry).ShouldNot(BeNil())
					Expect(entry.LatestTradeRecords).Should(BeEmpty())
					Expect(omitted).Should(BeTrue())

					entry, omitted = app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, perpMarket.MarketID, from)
					Expect(entry).ShouldNot(BeNil())
					Expect(entry.LatestTradeRecords).Should(BeEmpty())
					Expect(omitted).Should(BeTrue())
				})
			})
		})
	})

	Context("Cleanup expired price records", func() {
		BeforeEach(func() {
			app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord)
			app.ExchangeKeeper.AppendTradeRecord(ctx, spotMarket.MarketID, &spotTradeRecord2)
			app.ExchangeKeeper.AppendTradeRecord(ctx, perpMarket.MarketID, &perpTradeRecord)
			app.ExchangeKeeper.AppendTradeRecord(ctx, perpMarket.MarketID, &perpTradeRecord2)
		})

		When("Symbol price data partially expired", func() {
			var from int64

			JustBeforeEach(func() {
				ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(spotTradeRecord2.Timestamp+types.MaxHistoricalTradeRecordAge, 0)})
				from = spotTradeRecord2.Timestamp
			})

			It("Return post-cleanup data without omitting anything", func() {
				app.ExchangeKeeper.CleanupHistoricalTradeRecords(ctx)

				entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, spotMarket.MarketID, from)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestTradeRecords).Should(HaveLen(1)) {
						Expect(entry.LatestTradeRecords[0].Price).Should(Equal(spotTradeRecord2.Price))
						Expect(entry.LatestTradeRecords[0].Timestamp).Should(Equal(spotTradeRecord2.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())

				entry, omitted = app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, perpMarket.MarketID, from)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestTradeRecords).Should(HaveLen(1)) {
						Expect(entry.LatestTradeRecords[0].Price).Should(Equal(perpTradeRecord2.Price))
						Expect(entry.LatestTradeRecords[0].Timestamp).Should(Equal(perpTradeRecord2.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())
			})
		})

		When("Market trade data completely expired", func() {
			var from int64

			JustBeforeEach(func() {
				ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(spotTradeRecord2.Timestamp+types.MaxHistoricalTradeRecordAge+1, 0)})
				from = spotTradeRecord2.Timestamp
			})

			It("Post-cleanup entries doesn't have any trade data", func() {
				app.ExchangeKeeper.CleanupHistoricalTradeRecords(ctx)

				entry, omitted := app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, spotMarket.MarketID, from)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestTradeRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())

				entry, omitted = app.ExchangeKeeper.GetHistoricalTradeRecords(ctx, perpMarket.MarketID, from)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestTradeRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())
			})

			It("Post-cleanup cleanup is a no-op", func() {
				app.ExchangeKeeper.CleanupHistoricalTradeRecords(ctx)
				app.ExchangeKeeper.CleanupHistoricalTradeRecords(ctx)
			})
		})
	})
})
