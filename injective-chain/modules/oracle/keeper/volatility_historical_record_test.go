package keeper_test

import (
	"sort"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Volatility Historical Symbol Prices Test", func() {
	// state
	var (
		app    *simapp.InjectiveApp
		ctx    sdk.Context
		oracle = types.OracleType_Band
	)

	// test fixtures
	var (
		one = sdk.MustNewDecFromStr("1000000000000000000")

		timeZero = int64(1600000000)

		injRecord = types.PriceRecord{
			Timestamp: timeZero,
			Price:     sdk.MustNewDecFromStr("5000000000000000000"),
		}

		usdtRecord = types.PriceRecord{
			Timestamp: timeZero,
			Price:     sdk.MustNewDecFromStr("990000000000000000"),
		}

		injRecord2 = types.PriceRecord{
			Timestamp: injRecord.Timestamp + 1,
			Price:     injRecord.Price.Add(one),
		}

		usdtRecord2 = types.PriceRecord{
			Timestamp: usdtRecord.Timestamp + 1,
			Price:     usdtRecord.Price.Add(one),
		}
	)

	var _ = BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(timeZero, 0)})
	})

	Describe("Persistence of Historical Price Records", func() {
		When("No historical price record persists", func() {
			It("Returns empty record", func() {
				entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "INJ", 0)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestPriceRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())
			})

			It("Returns nil mixed record", func() {
				entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", 0)
				Expect(entry).Should(BeNil())
				Expect(ok).Should(BeFalse())
			})

			It("Cleanup is a no-op", func() {
				app.OracleKeeper.CleanupHistoricalPriceRecords(ctx)
			})
		})

		When("Have price records", func() {
			BeforeEach(func() {
				app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", &injRecord)
				app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", &usdtRecord)
			})

			It("Returns records for INJ", func() {
				entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "INJ", 0)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(injRecord.Price))
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(injRecord.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())
			})

			It("Returns records for USDT", func() {
				entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "USDT", 0)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(usdtRecord.Price))
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(usdtRecord.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())
			})

			It("Returns mixed record", func() {
				entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", 0)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(injRecord.Price.Quo(usdtRecord.Price)))
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(usdtRecord.Timestamp))
					}
				}
				Expect(ok).Should(BeTrue())
			})
		})
	})

	Describe("Time filtering and cleanup of Historical Price Records", func() {
		Context("Omitting expired price records", func() {
			BeforeEach(func() {
				app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", &injRecord)
				app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", &injRecord2)
				app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", &usdtRecord)
				app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", &usdtRecord2)
			})

			When("Some records expired", func() {
				var from int64

				BeforeEach(func() {
					from = injRecord.Timestamp + 1
				})

				It("Omits expired", func() {
					entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "INJ", from)
					if Expect(entry).ShouldNot(BeNil()) {
						if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
							Expect(entry.LatestPriceRecords[0].Price).Should(Equal(injRecord2.Price))
							Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(injRecord2.Timestamp))
						}
					}
					Expect(omitted).Should(BeTrue())
				})

				It("Mixed prices start from the new offset", func() {
					entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", from)
					if Expect(entry).ShouldNot(BeNil()) {
						if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
							Expect(entry.LatestPriceRecords[0].Price).Should(Equal(injRecord2.Price.Quo(usdtRecord2.Price)))
							Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(usdtRecord2.Timestamp))
						}
					}
					Expect(ok).Should(BeTrue())
				})
			})

			When("All records expired", func() {
				var from int64

				BeforeEach(func() {
					from = injRecord.Timestamp + 2
				})

				It("Omits all", func() {
					entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "INJ", from)
					Expect(entry).ShouldNot(BeNil())
					Expect(entry.LatestPriceRecords).Should(BeEmpty())
					Expect(omitted).Should(BeTrue())
				})

				It("Returns nil mixed record", func() {
					entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", from)
					Expect(entry).Should(BeNil())
					Expect(ok).Should(BeFalse())
				})
			})
		})
	})

	Context("Cleanup expired price records", func() {
		BeforeEach(func() {
			app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", &injRecord)
			app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", &injRecord2)
			app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", &usdtRecord)
			app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", &usdtRecord2)
		})

		When("Symbol price data partially expired", func() {
			var from int64

			JustBeforeEach(func() {
				ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(injRecord2.Timestamp+types.MaxHistoricalPriceRecordAge, 0)})
				from = injRecord2.Timestamp
			})

			It("Return post-cleanup data without omitting anything", func() {
				app.OracleKeeper.CleanupHistoricalPriceRecords(ctx)

				entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "INJ", from)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(injRecord2.Price))
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(injRecord2.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())

				entry, omitted = app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "USDT", from)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(1)) {
						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(usdtRecord2.Price))
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(usdtRecord2.Timestamp))
					}
				}
				Expect(omitted).Should(BeFalse())
			})
		})

		When("Symbol price data completely expired", func() {
			var from int64

			JustBeforeEach(func() {
				ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(injRecord2.Timestamp+(types.MaxHistoricalPriceRecordAge+1), 0)})
				from = injRecord2.Timestamp
			})

			It("Post-cleanup entries doesn't have any price data", func() {
				app.OracleKeeper.CleanupHistoricalPriceRecords(ctx)

				entry, omitted := app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "INJ", from)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestPriceRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())

				entry, omitted = app.OracleKeeper.GetHistoricalPriceRecords(ctx, oracle, "USDT", from)
				Expect(entry).ShouldNot(BeNil())
				Expect(entry.LatestPriceRecords).Should(BeEmpty())
				Expect(omitted).Should(BeFalse())
			})

			It("Post-cleanup cleanup is a no-op", func() {
				app.OracleKeeper.CleanupHistoricalPriceRecords(ctx)
				app.OracleKeeper.CleanupHistoricalPriceRecords(ctx)
			})
		})
	})

	Context("Getting Mixed Historical Price Records", func() {
		When("Some price data is missing", func() {
			var (
				// Time           --- 1 ---- 2 ---- 3 ---- 4
				// INJ Price      --- 6 ---- 8 ---- 4 ----
				// USDT Price     --- 2 ----   ---- 4 ---- 2
				// INJ/USDT Price --- 3 ---- 4 ---- 1 ---- 2
				priceRecords = map[string][]*types.PriceRecord{
					"INJ": {
						{Timestamp: timeZero + 1, Price: one.Mul(sdk.NewDec(6))},
						{Timestamp: timeZero + 2, Price: one.Mul(sdk.NewDec(8))},
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						// t0 + 4 is missing price
					},
					"USDT": {
						{Timestamp: timeZero + 1, Price: one.Mul(sdk.NewDec(2))},
						// t0 + 2 is missing price
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						{Timestamp: timeZero + 4, Price: one.Mul(sdk.NewDec(2))},
					},
				}
			)

			BeforeEach(func() {
				for _, priceRecord := range priceRecords["INJ"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", priceRecord)
				}

				for _, priceRecord := range priceRecords["USDT"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", priceRecord)
				}
			})

			It("Takes price records from previous timestamps", func() {
				entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", timeZero)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(4)) {
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(timeZero + 1))
						Expect(entry.LatestPriceRecords[1].Timestamp).Should(Equal(timeZero + 2))
						Expect(entry.LatestPriceRecords[2].Timestamp).Should(Equal(timeZero + 3))
						Expect(entry.LatestPriceRecords[3].Timestamp).Should(Equal(timeZero + 4))

						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(sdk.NewDec(3)))
						Expect(entry.LatestPriceRecords[1].Price).Should(Equal(sdk.NewDec(4)))
						Expect(entry.LatestPriceRecords[2].Price).Should(Equal(sdk.NewDec(1)))
						Expect(entry.LatestPriceRecords[3].Price).Should(Equal(sdk.NewDec(2)))
					}
				}
				Expect(ok).Should(BeTrue())
			})
		})

		When("Some base price data is missing", func() {
			var (
				// Time           --- 1 ---- 2 ---- 3 ---- 4
				// INJ Price      --- 6 ---- 8 ---- 4 ----
				// USDT Price     ---   ----   ---- 4 ---- 2
				// INJ/USDT Price --- X ---- X ---- 1 ---- 2
				priceRecords = map[string][]*types.PriceRecord{
					"INJ": {
						{Timestamp: timeZero + 1, Price: one.Mul(sdk.NewDec(6))},
						{Timestamp: timeZero + 2, Price: one.Mul(sdk.NewDec(8))},
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						// t0 + 4 is missing price
					},
					"USDT": {
						// t0 + 1 is missing price
						// t0 + 2 is missing price
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						{Timestamp: timeZero + 4, Price: one.Mul(sdk.NewDec(2))},
					},
				}
			)

			BeforeEach(func() {
				for _, priceRecord := range priceRecords["INJ"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", priceRecord)
				}

				for _, priceRecord := range priceRecords["USDT"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracle, "USDT", priceRecord)
				}
			})

			It("Can only obtain last two mixed prices", func() {
				entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", timeZero)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(2)) {
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(timeZero + 3))
						Expect(entry.LatestPriceRecords[1].Timestamp).Should(Equal(timeZero + 4))

						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(sdk.NewDec(1)))
						Expect(entry.LatestPriceRecords[1].Price).Should(Equal(sdk.NewDec(2)))
					}
				}
				Expect(ok).Should(BeTrue())
			})
		})

		When("Some quote price data is missing", func() {
			var (
				// Time           --- 1 ---- 2 ---- 3 ---- 4
				// INJ Price      ---   ---- 8 ---- 4 ----
				// USDT Price     --- 2 ----   ---- 4 ---- 2
				// INJ/USDT Price --- X ---- 4 ---- 1 ---- 2
				priceRecords = map[string][]*types.PriceRecord{
					"INJ": {
						// t0 + 1 is missing price
						{Timestamp: timeZero + 2, Price: one.Mul(sdk.NewDec(8))},
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						// t0 + 4 is missing price
					},
					"USDT": {
						{Timestamp: timeZero + 1, Price: one.Mul(sdk.NewDec(2))},
						// t0 + 2 is missing price
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						{Timestamp: timeZero + 4, Price: one.Mul(sdk.NewDec(2))},
					},
				}
				oracleINJ  = types.OracleType_Band
				oracleUSDT = types.OracleType_Coinbase
			)

			BeforeEach(func() {
				for _, priceRecord := range priceRecords["INJ"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracleINJ, "INJ", priceRecord)
				}

				for _, priceRecord := range priceRecords["USDT"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracleUSDT, "USDT", priceRecord)
				}
			})

			It("Can only obtain last three mixed prices", func() {
				entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracleINJ, oracleUSDT, "INJ", "USDT", timeZero)
				if Expect(entry).ShouldNot(BeNil()) {
					if Expect(entry.LatestPriceRecords).Should(HaveLen(3)) {
						Expect(entry.LatestPriceRecords[0].Timestamp).Should(Equal(timeZero + 2))
						Expect(entry.LatestPriceRecords[1].Timestamp).Should(Equal(timeZero + 3))
						Expect(entry.LatestPriceRecords[2].Timestamp).Should(Equal(timeZero + 4))

						Expect(entry.LatestPriceRecords[0].Price).Should(Equal(sdk.NewDec(4)))
						Expect(entry.LatestPriceRecords[1].Price).Should(Equal(sdk.NewDec(1)))
						Expect(entry.LatestPriceRecords[2].Price).Should(Equal(sdk.NewDec(2)))
					}
				}
				Expect(ok).Should(BeTrue())
			})
		})

		When("All quote price data is missing", func() {
			var (
				// Time           --- 1 ---- 2 ---- 3 ---- 4
				// INJ Price      ---   ---- 8 ---- 4 ----
				// USDT Price     ---   ----   ----   ----
				// INJ/USDT Price --- X ---- X ---- X ---- X
				priceRecords = map[string][]*types.PriceRecord{
					"INJ": {
						// t0 + 1 is missing price
						{Timestamp: timeZero + 2, Price: one.Mul(sdk.NewDec(8))},
						{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
						// t0 + 4 is missing price
					},
				}
			)

			BeforeEach(func() {
				for _, priceRecord := range priceRecords["INJ"] {
					app.OracleKeeper.AppendPriceRecord(ctx, oracle, "INJ", priceRecord)
				}
			})

			It("Cannot obtain mixed prices", func() {
				entry, ok := app.OracleKeeper.GetMixedHistoricalPriceRecords(ctx, oracle, oracle, "INJ", "USDT", timeZero)
				Expect(entry).Should(BeNil())
				Expect(ok).Should(BeFalse())
			})
		})
	})

	Describe("Price volatility", func() {
		var (
			baseOracleInfo, quoteOracleInfo *types.OracleInfo
			priceRecords                    map[string][]*types.PriceRecord
		)
		BeforeEach(func() {
			baseOracleInfo = &types.OracleInfo{
				Symbol:     "INJ",
				OracleType: types.OracleType_Band,
			}
			// Time 				––– 1 –––– 2 ––––  3  –––––  4  ––––– 5
			// INJ Price 			––– 6 –––– 8 ––––  4  –––––     ––––– 7
			// INJ Volatility 		1.479019946
			priceRecords = map[string][]*types.PriceRecord{
				baseOracleInfo.Symbol: {
					{Timestamp: timeZero + 1, Price: one.Mul(sdk.NewDec(6))},
					{Timestamp: timeZero + 2, Price: one.Mul(sdk.NewDec(8))},
					{Timestamp: timeZero + 3, Price: one.Mul(sdk.NewDec(4))},
					// t0 + 4 is missing price
					{Timestamp: timeZero + 5, Price: one.Mul(sdk.NewDec(7))},
				},
			}
		})
		Context("Direct INJ/USD volatility", func() {
			BeforeEach(func() {
				quoteOracleInfo = &types.OracleInfo{
					Symbol:     "USD",
					OracleType: types.OracleType_Band,
				}
			})
			When("Single price record", func() {
				BeforeEach(func() {
					app.OracleKeeper.AppendPriceRecord(ctx, baseOracleInfo.OracleType, baseOracleInfo.Symbol, priceRecords[baseOracleInfo.Symbol][0])
				})

				It("Returns zero volatility", func() {
					volatility, points, _ := app.OracleKeeper.GetOracleVolatility(ctx, baseOracleInfo, quoteOracleInfo, &types.OracleHistoryOptions{
						MaxAge:            0,
						IncludeRawHistory: true,
						IncludeMetadata:   true,
					})
					if Expect(volatility).ShouldNot(BeNil()) {
						Expect(volatility.String()).Should(Equal(sdk.ZeroDec().String()))
					}
					if Expect(points).Should(HaveLen(1)) {
						Expect(points[0].Timestamp).Should(Equal(priceRecords[baseOracleInfo.Symbol][0].Timestamp))
						Expect(points[0].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][0].Price))
					}
				})
			})
			When("Multiple price records", func() {
				BeforeEach(func() {
					for _, priceRecord := range priceRecords[baseOracleInfo.Symbol] {
						app.OracleKeeper.AppendPriceRecord(ctx, baseOracleInfo.OracleType, baseOracleInfo.Symbol, priceRecord)
					}
				})
				It("Returns INJ/USD volatility", func() {
					volatility, points, _ := app.OracleKeeper.GetOracleVolatility(ctx, baseOracleInfo, quoteOracleInfo, &types.OracleHistoryOptions{
						MaxAge:            0,
						IncludeRawHistory: true,
						IncludeMetadata:   true,
					})
					if Expect(volatility).ShouldNot(BeNil()) {
						Expect(volatility.String()).Should(Equal(sdk.MustNewDecFromStr("1479019945774904010.641832072890404262").String()))
					}
					if Expect(points).Should(HaveLen(4)) {
						Expect(points[0].Timestamp).Should(Equal(timeZero + 1))
						Expect(points[1].Timestamp).Should(Equal(timeZero + 2))
						Expect(points[2].Timestamp).Should(Equal(timeZero + 3))
						Expect(points[3].Timestamp).Should(Equal(timeZero + 5))

						Expect(points[0].Price).Should(Equal(one.Mul(sdk.NewDec(6))))
						Expect(points[1].Price).Should(Equal(one.Mul(sdk.NewDec(8))))
						Expect(points[2].Price).Should(Equal(one.Mul(sdk.NewDec(4))))
						Expect(points[3].Price).Should(Equal(one.Mul(sdk.NewDec(7))))
					}
				})
			})
		})

		Context("Mixed INJ/UST volatility", func() {
			BeforeEach(func() {
				quoteOracleInfo = &types.OracleInfo{
					Symbol:     "UST",
					OracleType: types.OracleType_Band,
				}
				// Time 				––– 1 –––– 2 ––––  3  –––––  4  ––––– 5
				// INJ Price 			––– 6 –––– 8 ––––  4  –––––     ––––– 7
				// UST Price 			––– 1 ––––   –––– 0.95 –––– 0.91 ––– 0.78
				// INJ/UST Price 		--- 6 ---- 8 ---- 4,21 ---- 4,395 -- 8,974
				// INJ/UST Volatility 	1.903865704
				priceRecords[quoteOracleInfo.Symbol] = []*types.PriceRecord{
					{Timestamp: timeZero + 1, Price: one.Mul(sdk.MustNewDecFromStr("1"))},
					// t0 + 2 is missing price
					{Timestamp: timeZero + 3, Price: one.Mul(sdk.MustNewDecFromStr("0.95"))},
					{Timestamp: timeZero + 4, Price: one.Mul(sdk.MustNewDecFromStr("0.91"))},
					{Timestamp: timeZero + 5, Price: one.Mul(sdk.MustNewDecFromStr("0.78"))},
				}
			})
			When("Single price record", func() {
				BeforeEach(func() {
					app.OracleKeeper.AppendPriceRecord(ctx, baseOracleInfo.OracleType, baseOracleInfo.Symbol, priceRecords[baseOracleInfo.Symbol][0])
					app.OracleKeeper.AppendPriceRecord(ctx, quoteOracleInfo.OracleType, quoteOracleInfo.Symbol, priceRecords[quoteOracleInfo.Symbol][0])
				})

				It("Returns zero volatility", func() {
					volatility, points, _ := app.OracleKeeper.GetOracleVolatility(ctx, baseOracleInfo, quoteOracleInfo, &types.OracleHistoryOptions{
						MaxAge:            0,
						IncludeRawHistory: true,
						IncludeMetadata:   true,
					})
					if Expect(volatility).ShouldNot(BeNil()) {
						Expect(volatility.String()).Should(Equal(sdk.ZeroDec().String()))
					}
					if Expect(points).Should(HaveLen(1)) {
						Expect(points[0].Timestamp).Should(Equal(priceRecords[baseOracleInfo.Symbol][0].Timestamp))
						Expect(points[0].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][0].Price.Quo(priceRecords[quoteOracleInfo.Symbol][0].Price)))
					}
				})
			})
			When("Multiple price records", func() {
				BeforeEach(func() {
					for _, priceRecord := range priceRecords[baseOracleInfo.Symbol] {
						app.OracleKeeper.AppendPriceRecord(ctx, baseOracleInfo.OracleType, baseOracleInfo.Symbol, priceRecord)
					}
					for _, priceRecord := range priceRecords[quoteOracleInfo.Symbol] {
						app.OracleKeeper.AppendPriceRecord(ctx, quoteOracleInfo.OracleType, quoteOracleInfo.Symbol, priceRecord)
					}
				})
				It("Returns INJ/UST volatility", func() {
					volatility, points, _ := app.OracleKeeper.GetOracleVolatility(ctx, baseOracleInfo, quoteOracleInfo, &types.OracleHistoryOptions{
						MaxAge:            0,
						IncludeRawHistory: true,
						IncludeMetadata:   true,
					})
					if Expect(volatility).ShouldNot(BeNil()) {
						Expect(volatility.String()).Should(Equal(sdk.MustNewDecFromStr("1.903865704343742654").String()))
					}
					if Expect(points).Should(HaveLen(5)) {
						Expect(points[0].Timestamp).Should(Equal(timeZero + 1))
						Expect(points[1].Timestamp).Should(Equal(timeZero + 2))
						Expect(points[2].Timestamp).Should(Equal(timeZero + 3))
						Expect(points[3].Timestamp).Should(Equal(timeZero + 4))
						Expect(points[4].Timestamp).Should(Equal(timeZero + 5))

						Expect(points[0].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][0].Price.Quo(priceRecords[quoteOracleInfo.Symbol][0].Price)))
						Expect(points[1].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][1].Price.Quo(priceRecords[quoteOracleInfo.Symbol][0].Price)))
						Expect(points[2].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][2].Price.Quo(priceRecords[quoteOracleInfo.Symbol][1].Price)))
						Expect(points[3].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][2].Price.Quo(priceRecords[quoteOracleInfo.Symbol][2].Price)))
						Expect(points[4].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][3].Price.Quo(priceRecords[quoteOracleInfo.Symbol][3].Price)))
					}
				})
			})

			When("Multiple price records with MaxAge constraint: timeZero+2", func() {
				BeforeEach(func() {
					for _, priceRecord := range priceRecords[baseOracleInfo.Symbol] {
						app.OracleKeeper.AppendPriceRecord(ctx, baseOracleInfo.OracleType, baseOracleInfo.Symbol, priceRecord)
					}
					for _, priceRecord := range priceRecords[quoteOracleInfo.Symbol] {
						app.OracleKeeper.AppendPriceRecord(ctx, quoteOracleInfo.OracleType, quoteOracleInfo.Symbol, priceRecord)
					}
				})
				It("Returns INJ/USD volatility", func() {
					// set block time to equal timeZero + 5
					ctx = ctx.WithBlockTime(time.Unix(timeZero+5, 0))
					volatility, points, meta := app.OracleKeeper.GetOracleVolatility(ctx, baseOracleInfo, quoteOracleInfo, &types.OracleHistoryOptions{
						MaxAge:            3,
						IncludeRawHistory: true,
						IncludeMetadata:   true,
					})
					sortedPoints := make([]*types.PriceRecord, 0, len(points))
					sortedPoints = append(sortedPoints, points...)
					sort.SliceStable(sortedPoints, func(i, j int) bool {
						return sortedPoints[i].Price.LT(sortedPoints[j].Price)
					})
					if Expect(volatility).ShouldNot(BeNil()) {
						Expect(volatility.String()).Should(Equal(sdk.MustNewDecFromStr("2.121239361702429402").String()))
					}
					if Expect(points).Should(HaveLen(4)) {
						// timeZero + 2 takes UST price from timezero+1
						Expect(points[0].Timestamp).Should(Equal(timeZero + 2))
						Expect(points[1].Timestamp).Should(Equal(timeZero + 3))
						Expect(points[2].Timestamp).Should(Equal(timeZero + 4))
						Expect(points[3].Timestamp).Should(Equal(timeZero + 5))

						Expect(points[0].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][1].Price.Quo(priceRecords[quoteOracleInfo.Symbol][0].Price)))
						Expect(points[1].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][2].Price.Quo(priceRecords[quoteOracleInfo.Symbol][1].Price)))
						Expect(points[2].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][2].Price.Quo(priceRecords[quoteOracleInfo.Symbol][2].Price)))
						Expect(points[3].Price).Should(Equal(priceRecords[baseOracleInfo.Symbol][3].Price.Quo(priceRecords[quoteOracleInfo.Symbol][3].Price)))

						Expect(meta.Mean).Should(Equal(points[0].Price.Add(points[1].Price).Add(points[2].Price).Add(points[3].Price).Quo(sdk.NewDec(4))))
						Expect(meta.Twap).Should(Equal(points[1].Price.Add(points[2].Price).Add(points[3].Price).Quo(sdk.NewDec(3))))
						Expect(meta.MinPrice).Should(Equal(points[1].Price))
						Expect(meta.MaxPrice).Should(Equal(points[3].Price))
						Expect(meta.MedianPrice).Should(Equal(sortedPoints[1].Price.Add(sortedPoints[2].Price).Quo(sdk.NewDec(2))))
						Expect(meta.FirstTimestamp).Should(Equal(points[0].Timestamp))
						Expect(meta.LastTimestamp).Should(Equal(points[3].Timestamp))
						Expect(meta.GroupCount).Should(Equal(uint32(4)))
						Expect(meta.RecordsSampleSize).Should(Equal(uint32(4)))
					}
				})
			})
		})
	})
})
