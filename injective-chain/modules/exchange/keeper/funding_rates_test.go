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

var _ = Describe("Funding rate Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket                   *types.DerivativeMarket
		msgServer                          types.MsgServer
		err                                error
		buyer                              = testexchange.SampleSubaccountAddr1
		seller                             = testexchange.SampleSubaccountAddr2
		buyer2                             = testexchange.SampleSubaccountAddr3
		seller2                            = testexchange.SampleSubaccountAddr4
		fundingBeforeBeginBlockerExecution *types.PerpetualMarketFunding
		fundingAfterBeginBlockerExecution  *types.PerpetualMarketFunding
		timeInterval                       = int64(3600)
		startingPrice                      = sdk.NewDec(2000)
		hoursPerDay                        = sdk.NewDec(24)
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

	Describe("Using only limit orders execution", func() {
		var (
			price    sdk.Dec
			quantity sdk.Dec
		)

		JustBeforeEach(func() {
			margin := sdk.NewDec(5000)
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_SELL, seller)

			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(20000),
				TotalBalance:     sdk.NewDec(20000),
			}
			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			fundingBeforeBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			fundingAfterBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Describe("with clearing price bigger than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2050)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})

		Describe("with clearing price smaller than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1990)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1950)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice).Neg()

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})
	})

	Describe("Using only market sell orders execution", func() {
		var (
			price    sdk.Dec
			quantity sdk.Dec
		)

		JustBeforeEach(func() {
			margin := sdk.NewDec(5000)
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_BUY, buyer)
			marketDerivativeSellOrder := testInput.NewMsgCreateDerivativeMarketOrder(quantity, margin, price, types.OrderType_SELL, seller)

			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(20000),
				TotalBalance:     sdk.NewDec(20000),
			}
			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketDerivativeSellOrder)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			fundingBeforeBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			fundingAfterBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())
		})

		Describe("with clearing price bigger than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2050)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})

		Describe("with clearing price smaller than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1990)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1950)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice).Neg()

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})
	})

	Describe("Using only market buy orders execution", func() {
		var (
			price    sdk.Dec
			quantity sdk.Dec
		)

		JustBeforeEach(func() {
			margin := sdk.NewDec(5000)
			marketDerivativeBuyOrder := testInput.NewMsgCreateDerivativeMarketOrder(quantity, margin, price, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_SELL, seller)

			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(20000),
				TotalBalance:     sdk.NewDec(20000),
			}
			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketDerivativeBuyOrder)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			fundingBeforeBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			fundingAfterBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())
		})

		Describe("with clearing price bigger than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2050)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})

		Describe("with clearing price smaller than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1990)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					price = sdk.NewDec(1950)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (price.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice).Neg()

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})
	})

	Describe("Using all kind of order execution", func() {
		var (
			priceForMarketBuy     sdk.Dec
			priceForMarketSell    sdk.Dec
			priceForLimitMatching sdk.Dec
			clearingPrice         sdk.Dec
			quantity              sdk.Dec
		)

		JustBeforeEach(func() {
			margin := sdk.NewDec(5000)
			limitDerivativeBuyOrderForMatching := testInput.NewMsgCreateDerivativeLimitOrder(priceForLimitMatching, quantity, margin, types.OrderType_BUY, buyer2)
			limitDerivativeSellOrderForMatching := testInput.NewMsgCreateDerivativeLimitOrder(priceForLimitMatching, quantity, margin, types.OrderType_SELL, seller2)

			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(priceForMarketSell, quantity, margin, types.OrderType_BUY, buyer)
			marketDerivativeSellOrder := testInput.NewMsgCreateDerivativeMarketOrder(quantity, margin, priceForMarketSell, types.OrderType_SELL, seller)

			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(priceForMarketBuy, quantity, margin, types.OrderType_SELL, seller)
			marketDerivativeBuyOrder := testInput.NewMsgCreateDerivativeMarketOrder(quantity, margin, priceForMarketBuy, types.OrderType_BUY, buyer)

			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(20000),
				TotalBalance:     sdk.NewDec(20000),
			}
			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, buyer2.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller2.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketDerivativeBuyOrder)
			msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketDerivativeSellOrder)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrderForMatching)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrderForMatching)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			fundingBeforeBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			fundingAfterBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())
		})

		Describe("with clearing price bigger than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					priceForMarketBuy = sdk.NewDec(2017)
					priceForMarketSell = sdk.NewDec(2004)
					priceForLimitMatching = sdk.NewDec(2009)
					clearingPrice = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (clearingPrice.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					priceForMarketBuy = sdk.NewDec(2080)
					priceForMarketSell = sdk.NewDec(2025)
					priceForLimitMatching = sdk.NewDec(2045)
					clearingPrice = sdk.NewDec(2050)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (clearingPrice.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})

		Describe("with clearing price smaller than mark price", func() {
			Context("but not enough to reach cap", func() {
				BeforeEach(func() {
					priceForMarketBuy = sdk.NewDec(1997)
					priceForMarketSell = sdk.NewDec(1984)
					priceForLimitMatching = sdk.NewDec(1989)
					clearingPrice = sdk.NewDec(1990)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (clearingPrice.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					hourlyInterestRate := types.DefaultParams().DefaultHourlyInterestRate
					expectedCumulativeFunding := expectedCumulativePrice.Quo(sdk.NewDec(timeInterval).Mul(hoursPerDay)).Add(hourlyInterestRate).Mul(startingPrice)

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})

			Context("and enough to reach cap", func() {
				BeforeEach(func() {
					priceForMarketBuy = sdk.NewDec(1990)
					priceForMarketSell = sdk.NewDec(1920)
					priceForLimitMatching = sdk.NewDec(1940)
					clearingPrice = sdk.NewDec(1950)
					quantity = sdk.NewDec(1)
				})

				It("should have correct funding", func() {
					expectedCumulativePrice := (clearingPrice.Sub(startingPrice)).Quo(startingPrice).Mul(sdk.NewDec(timeInterval))
					expectedCumulativeFunding := types.DefaultParams().DefaultHourlyFundingRateCap.Mul(startingPrice).Neg()

					Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(expectedCumulativePrice.String()))
					Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
				})

				It("should have correct funding events", func() {
					expectedLastFundingTimestamp := ctx.BlockTime().Unix()
					expectedMarketCreationTimestamp := expectedLastFundingTimestamp - timeInterval
					isFirstPerpetualFundingEvent := true

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventPerpetualMarketUpdate:
							Expect(common.HexToHash(event.PerpetualMarketInfo.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.PerpetualMarketInfo.HourlyFundingRateCap.String()).To(Equal(types.DefaultParams().DefaultHourlyFundingRateCap.String()))
							Expect(event.PerpetualMarketInfo.HourlyInterestRate.String()).To(Equal(types.DefaultParams().DefaultHourlyInterestRate.String()))
							Expect(event.PerpetualMarketInfo.NextFundingTimestamp).To(Equal(expectedLastFundingTimestamp))
							Expect(event.PerpetualMarketInfo.FundingInterval).To(Equal(timeInterval))
							Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.CumulativeFunding.String()).To(Equal(sdk.ZeroDec().String()))
							Expect(event.Funding.LastTimestamp).To(Equal(expectedMarketCreationTimestamp))

						case *types.EventPerpetualMarketFundingUpdate:
							if isFirstPerpetualFundingEvent {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))

								isFirstPerpetualFundingEvent = false
							} else {
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(event.Funding.CumulativeFunding).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding))
								Expect(event.Funding.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
								Expect(event.Funding.LastTimestamp).To(Equal(expectedLastFundingTimestamp))
								Expect(event.FundingRate.String()).To(Equal(fundingAfterBeginBlockerExecution.CumulativeFunding.String()))
								Expect(event.IsHourlyFunding).To(Equal(true))
							}
						}
					}
				})
			})
		})
	})

	Describe("Using no orders execution (empty epoch)", func() {
		JustBeforeEach(func() {
			// Updates time but also the funding timestamps
			updatedTime := ctx.BlockTime().Add(time.Second * 3600)
			ctx = ctx.WithBlockTime(time.Time(updatedTime))

			fundingBeforeBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())

			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			fundingAfterBeginBlockerExecution = app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, derivativeMarket.MarketID())
		})

		It("should have correct funding", func() {
			expectedCumulativeFunding := types.DefaultParams().DefaultHourlyInterestRate.Mul(startingPrice)

			Expect(fundingBeforeBeginBlockerExecution.CumulativePrice.String()).To(Equal(sdk.ZeroDec().String()))
			Expect(fundingAfterBeginBlockerExecution.CumulativeFunding.String()).To(Equal(expectedCumulativeFunding.String()))
		})
	})
})
