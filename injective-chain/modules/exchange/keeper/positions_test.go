package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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

func matchBuyerAndSeller(testInput testexchange.TestInput, app *simapp.InjectiveApp, ctx sdk.Context, msgServer types.MsgServer, marketType types.MarketType, margin, quantity, price sdk.Dec, isLong bool, buyer, seller common.Hash) {
	var err error

	var buyerOrderType, sellerOrderType types.OrderType

	if isLong {
		buyerOrderType = types.OrderType_BUY
		sellerOrderType = types.OrderType_SELL
	} else {
		buyerOrderType = types.OrderType_SELL
		sellerOrderType = types.OrderType_BUY
	}

	if marketType == types.MarketType_BinaryOption {
		limitBinaryOptionsBuyOrder := testInput.NewMsgCreateBinaryOptionsLimitOrder(price, quantity, false, buyerOrderType, buyer, 0)
		limitBinaryOptionsSellOrder := testInput.NewMsgCreateBinaryOptionsLimitOrder(price, quantity, false, sellerOrderType, seller, 0)
		_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), limitBinaryOptionsBuyOrder)
		testexchange.OrFail(err)
		_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), limitBinaryOptionsSellOrder)
		testexchange.OrFail(err)
	} else {
		limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, buyerOrderType, buyer)
		limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, sellerOrderType, seller)
		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
		testexchange.OrFail(err)
		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
		testexchange.OrFail(err)
	}

	ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
	exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
}

var _ = Describe("Position Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		buyer            = testexchange.SampleSubaccountAddr1
		seller           = testexchange.SampleSubaccountAddr2
		startingPrice    = sdk.NewDec(2000)
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

	Describe("When changing a position", func() {
		var (
			price    sdk.Dec
			quantity sdk.Dec
			margin   sdk.Dec
			isLong   bool
		)

		JustBeforeEach(func() {
			quoteDeposit := &types.Deposit{
				AvailableBalance: sdk.NewDec(200000),
				TotalBalance:     sdk.NewDec(200000),
			}
			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

			matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, margin, quantity, price, isLong, buyer, seller)
		})

		Describe("when position is created", func() {
			Context("position is long", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)
					margin = sdk.NewDec(2010)
					isLong = true
				})

				It("creates position correctly", func() {
					positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					buyerPosition := positions[0]

					Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
					Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
					Expect(buyerPosition.Position.Quantity.String()).To(Equal(quantity.String()))
					Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(price.String()))
					Expect(buyerPosition.Position.Margin.String()).To(Equal(margin.String()))
					Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						switch emittedABCIEvent.Type {
						case banktypes.EventTypeCoinReceived, banktypes.EventTypeCoinMint, banktypes.EventTypeCoinSpent, banktypes.EventTypeTransfer, sdk.EventTypeMessage:
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventBatchDerivativePosition:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
							Expect(event.Positions[0].Position.Quantity.String()).To(Equal(quantity.String()))
							Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(price.String()))
							Expect(event.Positions[0].Position.Margin.String()).To(Equal(margin.String()))
							Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
						}
					}
				})
			})

			Context("position is short", func() {
				BeforeEach(func() {
					price = sdk.NewDec(2010)
					quantity = sdk.NewDec(1)
					margin = sdk.NewDec(2010)
					isLong = false
				})

				It("creates position correctly", func() {
					positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					buyerPosition := positions[0]

					Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
					Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
					Expect(buyerPosition.Position.Quantity.String()).To(Equal(quantity.String()))
					Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(price.String()))
					Expect(buyerPosition.Position.Margin.String()).To(Equal(margin.String()))
					Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						switch emittedABCIEvent.Type {
						case banktypes.EventTypeCoinReceived, banktypes.EventTypeCoinMint, banktypes.EventTypeCoinSpent, banktypes.EventTypeTransfer, sdk.EventTypeMessage:
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventBatchDerivativePosition:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
							Expect(event.Positions[0].Position.Quantity.String()).To(Equal(quantity.String()))
							Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(price.String()))
							Expect(event.Positions[0].Position.Margin.String()).To(Equal(margin.String()))
							Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
						}
					}
				})
			})
		})

		Describe("when position already exists", func() {
			var (
				newOrderPrice    sdk.Dec
				newOrderQuantity sdk.Dec
				newOrderMargin   sdk.Dec
				newOrderIsLong   bool
			)

			Describe("when existing position is long", func() {
				Context("new order is buy", func() {
					BeforeEach(func() {
						price = sdk.NewDec(2010)
						quantity = sdk.NewDec(3)
						margin = sdk.NewDec(3377)
						isLong = true

						newOrderPrice = sdk.NewDec(1651)
						newOrderQuantity = sdk.NewDec(7)
						newOrderMargin = sdk.NewDec(3170)
						newOrderIsLong = true
					})

					JustBeforeEach(func() {
						ctx = ctx.WithEventManager(sdk.NewEventManager())
						matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
					})

					It("increments position", func() {
						positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
						buyerPosition := positions[0]

						expectedPositionQuantity := quantity.Add(newOrderQuantity)
						expectedPositionPrice := price.Mul(quantity).Add(newOrderPrice.Mul(newOrderQuantity)).Quo(expectedPositionQuantity)
						expectedPositionMargin := margin.Add(newOrderMargin)

						Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
						Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
						Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
						Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
						Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
						Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

						emittedABCIEvents := ctx.EventManager().ABCIEvents()
						for _, emittedABCIEvent := range emittedABCIEvents {
							if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
								continue
							}
							parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
							testexchange.OrFail(err1)

							switch event := parsedEvent.(type) {
							case *types.EventBatchDerivativePosition:
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
								Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
								Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
								Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
								Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
							}
						}
					})
				})

				Context("new order is sell", func() {
					Context("when reducing position partially", func() {
						BeforeEach(func() {
							price = sdk.NewDec(2010)
							quantity = sdk.NewDec(5)
							margin = sdk.NewDec(3377)
							isLong = true

							newOrderPrice = sdk.NewDec(1651)
							newOrderQuantity = sdk.NewDec(3)
							newOrderMargin = sdk.NewDec(3170)
							newOrderIsLong = false
						})

						JustBeforeEach(func() {
							ctx = ctx.WithEventManager(sdk.NewEventManager())
							matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
						})

						It("decrements position", func() {
							positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
							buyerPosition := positions[0]

							expectedPositionQuantity := quantity.Sub(newOrderQuantity)
							expectedPositionPrice := price
							expectedPositionMargin := margin.Mul(expectedPositionQuantity).Quo(quantity)

							Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
							Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
							Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
							Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
							Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

							emittedABCIEvents := ctx.EventManager().ABCIEvents()
							for _, emittedABCIEvent := range emittedABCIEvents {
								if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
									continue
								}
								parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
								testexchange.OrFail(err1)

								switch event := parsedEvent.(type) {
								case *types.EventBatchDerivativePosition:
									Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
									Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
									Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
									Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
									Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
									Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
								}
							}
						})
					})

					Context("when reducing position fully", func() {
						BeforeEach(func() {
							price = sdk.NewDec(2010)
							quantity = sdk.NewDec(5)
							margin = sdk.NewDec(3377)
							isLong = true

							newOrderPrice = sdk.NewDec(1651)
							newOrderQuantity = sdk.NewDec(5)
							newOrderMargin = sdk.NewDec(3170)
							newOrderIsLong = false
						})

						JustBeforeEach(func() {
							ctx = ctx.WithEventManager(sdk.NewEventManager())
							matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
						})

						It("removes position", func() {
							positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
							Expect(len(positions)).To(Equal(0))

							emittedABCIEvents := ctx.EventManager().ABCIEvents()
							for _, emittedABCIEvent := range emittedABCIEvents {
								if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
									continue
								}
								parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
								testexchange.OrFail(err1)

								switch event := parsedEvent.(type) {
								case *types.EventBatchDerivativePosition:
									Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
									Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
									Expect(event.Positions[0].Position.Quantity.String()).To(Equal(sdk.ZeroDec().String()))
									Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(price.String()))
									Expect(event.Positions[0].Position.Margin.String()).To(Equal(sdk.ZeroDec().String()))
									Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
								}
							}
						})
					})

					Context("when flipping position", func() {
						BeforeEach(func() {
							price = sdk.NewDec(2010)
							quantity = sdk.NewDec(5)
							margin = sdk.NewDec(4377)
							isLong = true

							newOrderPrice = sdk.NewDec(1651)
							newOrderQuantity = sdk.NewDec(8)
							newOrderMargin = sdk.NewDec(4170)
							newOrderIsLong = false
						})

						JustBeforeEach(func() {
							ctx = ctx.WithEventManager(sdk.NewEventManager())
							matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
						})

						It("opens position in opposite direction", func() {
							positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
							buyerPosition := positions[0]

							expectedPositionQuantity := newOrderQuantity.Sub(quantity)
							expectedPositionPrice := newOrderPrice
							expectedPositionMargin := newOrderMargin.Mul(expectedPositionQuantity).Quo(newOrderQuantity)

							Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
							Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
							Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
							Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
							Expect(buyerPosition.Position.IsLong).To(Equal(newOrderIsLong))

							emittedABCIEvents := ctx.EventManager().ABCIEvents()
							for _, emittedABCIEvent := range emittedABCIEvents {
								if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
									continue
								}
								parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
								testexchange.OrFail(err1)

								switch event := parsedEvent.(type) {
								case *types.EventBatchDerivativePosition:
									Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
									Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
									Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
									Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
									Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
									Expect(event.Positions[0].Position.IsLong).To(Equal(newOrderIsLong))
								}
							}
						})
					})
				})
			})

			Describe("when existing position is short", func() {
				Context("new order is sell", func() {
					BeforeEach(func() {
						price = sdk.NewDec(2010)
						quantity = sdk.NewDec(3)
						margin = sdk.NewDec(3377)
						isLong = false

						newOrderPrice = sdk.NewDec(1651)
						newOrderQuantity = sdk.NewDec(7)
						newOrderMargin = sdk.NewDec(3170)
						newOrderIsLong = false
					})

					JustBeforeEach(func() {
						ctx = ctx.WithEventManager(sdk.NewEventManager())
						matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
					})

					It("increments position", func() {
						positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
						buyerPosition := positions[0]

						expectedPositionQuantity := quantity.Add(newOrderQuantity)
						expectedPositionPrice := price.Mul(quantity).Add(newOrderPrice.Mul(newOrderQuantity)).Quo(expectedPositionQuantity)
						expectedPositionMargin := margin.Add(newOrderMargin)

						Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
						Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
						Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
						Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
						Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
						Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

						emittedABCIEvents := ctx.EventManager().ABCIEvents()
						for _, emittedABCIEvent := range emittedABCIEvents {
							if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
								continue
							}
							parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
							testexchange.OrFail(err1)

							switch event := parsedEvent.(type) {
							case *types.EventBatchDerivativePosition:
								Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
								Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
								Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
								Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
								Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
							}
						}
					})
				})

				Context("new order is buy", func() {
					Context("when reducing position partially", func() {
						BeforeEach(func() {
							price = sdk.NewDec(2010)
							quantity = sdk.NewDec(5)
							margin = sdk.NewDec(3377)
							isLong = false

							newOrderPrice = sdk.NewDec(1651)
							newOrderQuantity = sdk.NewDec(3)
							newOrderMargin = sdk.NewDec(3170)
							newOrderIsLong = true
						})

						JustBeforeEach(func() {
							ctx = ctx.WithEventManager(sdk.NewEventManager())
							matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
						})

						It("decrements position", func() {
							positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
							buyerPosition := positions[0]

							expectedPositionQuantity := quantity.Sub(newOrderQuantity)
							expectedPositionPrice := price
							expectedPositionMargin := margin.Mul(expectedPositionQuantity).Quo(quantity)

							Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
							Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
							Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
							Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
							Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

							emittedABCIEvents := ctx.EventManager().ABCIEvents()
							for _, emittedABCIEvent := range emittedABCIEvents {
								if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
									continue
								}
								parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
								testexchange.OrFail(err1)

								switch event := parsedEvent.(type) {
								case *types.EventBatchDerivativePosition:
									Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
									Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
									Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
									Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
									Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
									Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
								}
							}
						})
					})

					Context("when closing via reduce-only", func() {
						Context("when reducing position partially", func() {
							BeforeEach(func() {
								price = sdk.NewDec(2010)
								quantity = sdk.NewDec(5)
								margin = sdk.NewDec(3377)
								isLong = false

								newOrderPrice = sdk.NewDec(1651)
								newOrderQuantity = sdk.NewDec(3)
								newOrderMargin = sdk.NewDec(0)
								newOrderIsLong = true
							})

							JustBeforeEach(func() {
								ctx = ctx.WithEventManager(sdk.NewEventManager())
								matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
							})

							It("decrements position", func() {
								positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
								buyerPosition := positions[0]

								expectedPositionQuantity := quantity.Sub(newOrderQuantity)
								expectedPositionPrice := price
								expectedPositionMargin := margin.Mul(expectedPositionQuantity).Quo(quantity)

								Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
								Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
								Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
								Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
								Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

								emittedABCIEvents := ctx.EventManager().ABCIEvents()
								for _, emittedABCIEvent := range emittedABCIEvents {
									if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
										continue
									}
									parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
									testexchange.OrFail(err1)

									switch event := parsedEvent.(type) {
									case *types.EventBatchDerivativePosition:
										Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
										Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
										Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
										Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
										Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
										Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
									}
								}
							})
						})

						Context("when reducing position fully", func() {
							BeforeEach(func() {
								price = sdk.NewDec(2010)
								quantity = sdk.NewDec(5)
								margin = sdk.NewDec(3377)
								isLong = false

								newOrderPrice = sdk.NewDec(1651)
								newOrderQuantity = sdk.NewDec(5)
								newOrderMargin = sdk.NewDec(3170)
								newOrderIsLong = true
							})

							JustBeforeEach(func() {
								ctx = ctx.WithEventManager(sdk.NewEventManager())
								matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
							})

							It("removes position", func() {
								positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
								Expect(len(positions)).To(Equal(0))
							})
						})
					})

					Context("when closing via netting", func() {
						Context("when reducing position partially", func() {
							BeforeEach(func() {
								price = sdk.NewDec(2010)
								quantity = sdk.NewDec(5)
								margin = sdk.NewDec(3377)
								isLong = false

								newOrderPrice = sdk.NewDec(1651)
								newOrderQuantity = sdk.NewDec(3)
								newOrderMargin = sdk.NewDec(3170)
								newOrderIsLong = true
							})

							JustBeforeEach(func() {
								ctx = ctx.WithEventManager(sdk.NewEventManager())
								matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
							})

							It("decrements position", func() {
								positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
								buyerPosition := positions[0]

								expectedPositionQuantity := quantity.Sub(newOrderQuantity)
								expectedPositionPrice := price
								expectedPositionMargin := margin.Mul(expectedPositionQuantity).Quo(quantity)

								Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
								Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
								Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
								Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
								Expect(buyerPosition.Position.IsLong).To(Equal(isLong))

								emittedABCIEvents := ctx.EventManager().ABCIEvents()
								for _, emittedABCIEvent := range emittedABCIEvents {
									if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
										continue
									}
									parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
									testexchange.OrFail(err1)

									switch event := parsedEvent.(type) {
									case *types.EventBatchDerivativePosition:
										Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
										Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
										Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
										Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
										Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
										Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
									}
								}
							})
						})

						Context("when reducing position fully", func() {
							BeforeEach(func() {
								price = sdk.NewDec(2010)
								quantity = sdk.NewDec(5)
								margin = sdk.NewDec(3377)
								isLong = false

								newOrderPrice = sdk.NewDec(1651)
								newOrderQuantity = sdk.NewDec(5)
								newOrderMargin = sdk.NewDec(3170)
								newOrderIsLong = true
							})

							JustBeforeEach(func() {
								ctx = ctx.WithEventManager(sdk.NewEventManager())
								matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
							})

							It("removes position", func() {
								positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
								Expect(len(positions)).To(Equal(0))

								emittedABCIEvents := ctx.EventManager().ABCIEvents()
								for _, emittedABCIEvent := range emittedABCIEvents {
									if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
										continue
									}
									parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
									testexchange.OrFail(err1)

									switch event := parsedEvent.(type) {
									case *types.EventBatchDerivativePosition:
										Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
										Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
										Expect(event.Positions[0].Position.Quantity.String()).To(Equal(sdk.ZeroDec().String()))
										Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(price.String()))
										Expect(event.Positions[0].Position.Margin.String()).To(Equal(sdk.ZeroDec().String()))
										Expect(event.Positions[0].Position.IsLong).To(Equal(isLong))
									}
								}
							})
						})

						Context("when flipping position", func() {
							BeforeEach(func() {
								price = sdk.NewDec(2010)
								quantity = sdk.NewDec(5)
								margin = sdk.NewDec(4377)
								isLong = false

								newOrderPrice = sdk.NewDec(1651)
								newOrderQuantity = sdk.NewDec(8)
								newOrderMargin = sdk.NewDec(4170)
								newOrderIsLong = true
							})

							JustBeforeEach(func() {
								ctx = ctx.WithEventManager(sdk.NewEventManager())
								matchBuyerAndSeller(testInput, app, ctx, msgServer, types.MarketType_Perpetual, newOrderMargin, newOrderQuantity, newOrderPrice, newOrderIsLong, buyer, seller)
							})

							It("opens position in opposite direction", func() {
								positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
								buyerPosition := positions[0]

								expectedPositionQuantity := newOrderQuantity.Sub(quantity)
								expectedPositionPrice := newOrderPrice
								expectedPositionMargin := newOrderMargin.Mul(expectedPositionQuantity).Quo(newOrderQuantity)

								Expect(common.HexToHash(buyerPosition.MarketId)).To(Equal(testInput.Perps[0].MarketID))
								Expect(common.HexToHash(buyerPosition.SubaccountId)).To(Equal(buyer))
								Expect(buyerPosition.Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
								Expect(buyerPosition.Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
								Expect(buyerPosition.Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
								Expect(buyerPosition.Position.IsLong).To(Equal(newOrderIsLong))

								emittedABCIEvents := ctx.EventManager().ABCIEvents()
								for _, emittedABCIEvent := range emittedABCIEvents {
									if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
										continue
									}
									parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
									testexchange.OrFail(err1)

									switch event := parsedEvent.(type) {
									case *types.EventBatchDerivativePosition:
										Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
										Expect(common.BytesToHash(event.Positions[0].SubaccountId)).To(Equal(buyer))
										Expect(event.Positions[0].Position.Quantity.String()).To(Equal(expectedPositionQuantity.String()))
										Expect(event.Positions[0].Position.EntryPrice.String()).To(Equal(expectedPositionPrice.String()))
										Expect(event.Positions[0].Position.Margin.String()).To(Equal(expectedPositionMargin.String()))
										Expect(event.Positions[0].Position.IsLong).To(Equal(newOrderIsLong))
									}
								}
							})
						})
					})
				})
			})
		})
	})
})
