package keeper_test

import (
	"reflect"
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

var _ = Describe("Order Invalidation Tests", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		positionsBefore      []*types.DerivativePosition
		msgServer            types.MsgServer
		err                  error
		buyer                = testexchange.SampleSubaccountAddr1
		seller               = testexchange.SampleSubaccountAddr2
		bankruptcyPrice      sdk.Dec
		startingPrice        = sdk.NewDec(2000)
		margin               = sdk.NewDec(1800)
		quantity             = sdk.NewDec(1)
		newCumulativeFunding = sdk.MustNewDecFromStr("200.03")
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
		initialInsuranceFundBalance = sdk.NewDec(44)
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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
		testexchange.OrFail(err)

		limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(startingPrice, quantity, margin, types.OrderType_BUY, buyer)
		limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(startingPrice, quantity, margin, types.OrderType_SELL, seller)

		initialCoinDeposit := sdk.NewInt(200000)
		coin = sdk.NewCoin(testInput.Perps[0].QuoteDenom, initialCoinDeposit)

		testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(coin))
		testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(coin))

		funding := types.PerpetualMarketFunding{
			CumulativeFunding: sdk.NewDec(100),
			CumulativePrice:   sdk.NewDec(10),
			LastTimestamp:     ctx.BlockTime().Unix(),
		}
		app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
		testexchange.OrFail(err)
		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
		testexchange.OrFail(err)

		bankruptcyPrice = sdk.NewDec(201)

		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	})

	AfterEach(func() {
		Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
	})

	Describe("when using limit order", func() {
		Describe("when reduce only becomes invalid due to bankruptcy price", func() {
			BeforeEach(func() {
				bankruptcyPrice = sdk.NewDec(201)

				reduceOnlyOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(0), types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), reduceOnlyOrder)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				funding := types.PerpetualMarketFunding{
					CumulativeFunding: newCumulativeFunding,
					CumulativePrice:   sdk.NewDec(10),
					LastTimestamp:     ctx.BlockTime().Unix(),
				}
				app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

				matchingTheReduceOnlyOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(1000), types.OrderType_BUY, seller)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), matchingTheReduceOnlyOrder)
				testexchange.OrFail(err)

				positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("invalidates the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(1))
				Expect(len(sellOrders)).Should(Equal(0))

				positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding
				positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding

				Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
				Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeTrue())
			})
		})

		Describe("when limit order becomes invalid due to bankruptcy price of existing position", func() {
			var addedMargin sdk.Dec

			JustBeforeEach(func() {
				bankruptcyPrice = sdk.NewDec(201)

				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(bankruptcyPrice, ctx.BlockTime().Unix()))

				limitOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(4), addedMargin, types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrder)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				funding := types.PerpetualMarketFunding{
					CumulativeFunding: newCumulativeFunding,
					CumulativePrice:   sdk.NewDec(10),
					LastTimestamp:     ctx.BlockTime().Unix(),
				}
				app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

				matchingTheLimitOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(4), sdk.NewDec(1000), types.OrderType_BUY, seller)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), matchingTheLimitOrder)
				testexchange.OrFail(err)

				positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			Describe("when added margin is sufficient to pay beyond bankruptcy price", func() {
				BeforeEach(func() {
					addedMargin = sdk.NewDec(475)
				})

				It("does not invalidate the order", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
					Expect(len(buyOrders)).Should(Equal(0))
					Expect(len(sellOrders)).Should(Equal(0))

					positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

					positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding
					positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding
					Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
					Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeFalse())

					Expect(positionsAfter[0].Position.IsLong).Should(Equal(!positionsBefore[0].Position.IsLong))
					Expect(positionsAfter[0].Position.Quantity.String()).Should(Equal(sdk.NewDec(3).String()))
					Expect(positionsAfter[0].Position.Margin.String()).Should(Equal(addedMargin.Sub(addedMargin.Quo(sdk.NewDec(4))).String()))
				})
			})

			Describe("when added margin is not sufficient to pay beyond bankruptcy price", func() {
				BeforeEach(func() {
					addedMargin = sdk.NewDec(236)
				})

				It("invalidates the order", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
					Expect(len(buyOrders)).Should(Equal(1))
					Expect(len(sellOrders)).Should(Equal(0))

					positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

					positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding
					positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding

					Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
					Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeTrue())
				})
			})
		})

		Describe("when reduce only becomes invalid due to direction", func() {
			BeforeEach(func() {
				reduceOnlyOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(0), types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), reduceOnlyOrder)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				secondOrderQuantity := quantity.Mul(sdk.NewDec(2))
				margin2 := sdk.NewDec(4000)
				limitDerivativeBuyOrder1 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice.Sub(sdk.NewDec(1)), secondOrderQuantity, margin2, types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder1)
				testexchange.OrFail(err)
				limitDerivativeSellOrder1 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice.Sub(sdk.NewDec(1)), secondOrderQuantity, margin2, types.OrderType_BUY, seller)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder1)
				testexchange.OrFail(err)
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("cancels the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(0))
				Expect(len(sellOrders)).Should(Equal(0))
			})
		})

		Describe("when reduce only becomes invalid due to oracle price", func() {
			BeforeEach(func() {
				positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				order2 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(100), types.OrderType_BUY, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), order2)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				order1 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(10000), types.OrderType_SELL, seller)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), order1)
				testexchange.OrFail(err)

				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(sdk.NewDec(100), ctx.BlockTime().Unix()))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("invalidates the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(0))
				Expect(len(sellOrders)).Should(Equal(1))

				positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
				Expect(reflect.DeepEqual(positionsBefore, positionsAfter)).Should(BeTrue())
			})
		})
	})

	Describe("when using transient limit order", func() {
		BeforeEach(func() {
			positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

			bankruptcyPrice = sdk.NewDec(201)
			toBeFilledOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(1500), types.OrderType_BUY, seller)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), toBeFilledOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
		})

		Describe("when reduce only becomes invalid due to bankruptcy price", func() {
			BeforeEach(func() {
				reduceOnlyOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(0), types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), reduceOnlyOrder)
				testexchange.OrFail(err)

				funding := types.PerpetualMarketFunding{
					CumulativeFunding: newCumulativeFunding,
					CumulativePrice:   sdk.NewDec(10),
					LastTimestamp:     ctx.BlockTime().Unix(),
				}
				app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("invalidates the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(1))
				Expect(len(sellOrders)).Should(Equal(0))

				positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
				positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding
				positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding

				Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
				Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeTrue())
			})
		})

		Describe("when transient limit order becomes invalid due to bankruptcy price of existing position", func() {
			var addedMargin sdk.Dec

			JustBeforeEach(func() {
				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(bankruptcyPrice, ctx.BlockTime().Unix()))
				limitOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(4), addedMargin, types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrder)
				testexchange.OrFail(err)

				funding := types.PerpetualMarketFunding{
					CumulativeFunding: newCumulativeFunding,
					CumulativePrice:   sdk.NewDec(10),
					LastTimestamp:     ctx.BlockTime().Unix(),
				}
				app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			Describe("when added margin is sufficient to pay beyond bankruptcy price", func() {
				BeforeEach(func() {
					addedMargin = sdk.NewDec(480)
				})

				It("does not invalidate the order", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
					Expect(len(buyOrders)).Should(Equal(0))
					Expect(len(sellOrders)).Should(Equal(1))

					positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(positionsAfter)).Should(BeZero())
				})
			})

			Describe("when added margin is not sufficient to pay beyond bankruptcy price", func() {
				BeforeEach(func() {
					addedMargin = sdk.NewDec(238)
				})

				It("invalidates the order", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
					Expect(len(buyOrders)).Should(Equal(1))
					Expect(len(sellOrders)).Should(Equal(0))

					positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding
					positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding

					Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
					Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeTrue())
				})
			})
		})

		Describe("when reduce only becomes invalid due to oracle price", func() {
			BeforeEach(func() {
				order1 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(2), sdk.NewDec(10000), types.OrderType_SELL, seller)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), order1)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				order2 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(100), types.OrderType_BUY, buyer)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), order2)
				testexchange.OrFail(err)

				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(sdk.NewDec(100), ctx.BlockTime().Unix()))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("invalidates the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(0))
				Expect(len(sellOrders)).Should(Equal(1))

				positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
				Expect(reflect.DeepEqual(positionsBefore, positionsAfter)).Should(BeTrue())
			})
		})
	})

	Describe("when using market order", func() {
		BeforeEach(func() {
			positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

			bankruptcyPrice = sdk.NewDec(201)
			toBeFilledOrder := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(1), sdk.NewDec(1500), types.OrderType_BUY, seller)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), toBeFilledOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
		})

		Describe("when reduce only becomes invalid due to bankruptcy price", func() {
			BeforeEach(func() {
				reduceOnlyOrder := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(0), bankruptcyPrice, types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), reduceOnlyOrder)
				testexchange.OrFail(err)

				funding := types.PerpetualMarketFunding{
					CumulativeFunding: newCumulativeFunding,
					CumulativePrice:   sdk.NewDec(10),
					LastTimestamp:     ctx.BlockTime().Unix(),
				}
				app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("invalidates the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(1))
				Expect(len(sellOrders)).Should(Equal(0))

				positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
				positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding
				positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding

				Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
				Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeTrue())
			})
		})

		Describe("when market order becomes invalid due to bankruptcy price of existing position", func() {
			var addedMargin sdk.Dec

			JustBeforeEach(func() {
				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(bankruptcyPrice, ctx.BlockTime().Unix()))
				marketOrder := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(4), addedMargin, bankruptcyPrice, types.OrderType_SELL, buyer)
				_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), marketOrder)
				testexchange.OrFail(err)

				funding := types.PerpetualMarketFunding{
					CumulativeFunding: newCumulativeFunding,
					CumulativePrice:   sdk.NewDec(10),
					LastTimestamp:     ctx.BlockTime().Unix(),
				}
				app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, testInput.Perps[0].MarketID, &funding)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			Describe("when added margin is sufficient to pay beyond bankruptcy price", func() {
				BeforeEach(func() {
					addedMargin = sdk.NewDec(480)
				})

				It("does not invalidate the order", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
					Expect(len(buyOrders)).Should(Equal(0))
					Expect(len(sellOrders)).Should(Equal(0))

					positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(positionsAfter)).Should(BeZero())
				})
			})

			Describe("when added margin is not sufficient to pay beyond bankruptcy price", func() {
				BeforeEach(func() {
					addedMargin = sdk.NewDec(238)
				})

				It("invalidates the order", func() {
					buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
					sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
					Expect(len(buyOrders)).Should(Equal(1))
					Expect(len(sellOrders)).Should(Equal(0))

					positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					positionsBefore[1].Position.CumulativeFundingEntry = newCumulativeFunding
					positionsAfter[1].Position.Margin = positionsAfter[1].Position.Margin.Sub(sdk.MustNewDecFromStr("100.03")) // from funding

					Expect(reflect.DeepEqual(positionsBefore[0], positionsAfter[0])).Should(BeFalse())
					Expect(reflect.DeepEqual(positionsBefore[1], positionsAfter[1])).Should(BeTrue())
				})
			})
		})

		Describe("when reduce only becomes invalid due to oracle price", func() {
			BeforeEach(func() {
				order1 := testInput.NewMsgCreateDerivativeLimitOrder(bankruptcyPrice, sdk.NewDec(2), sdk.NewDec(10000), types.OrderType_SELL, seller)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), order1)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				positionsBefore = app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				order2 := testInput.NewMsgCreateDerivativeMarketOrder(sdk.NewDec(1), sdk.NewDec(100), bankruptcyPrice, types.OrderType_BUY, buyer)
				_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), order2)
				testexchange.OrFail(err)

				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(sdk.NewDec(100), ctx.BlockTime().Unix()))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("invalidates the order", func() {
				buyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				sellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(buyOrders)).Should(Equal(0))
				Expect(len(sellOrders)).Should(Equal(1))

				positionsAfter := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
				Expect(reflect.DeepEqual(positionsBefore, positionsAfter)).Should(BeTrue())
			})
		})
	})
})
