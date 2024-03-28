package keeper_test

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Liquidation Tests - Safer logic - non-default subaccount", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		msgServer        types.MsgServer
		err              error
		buyer            = testexchange.SampleNonDefaultSubaccountAddr1
		seller           = testexchange.SampleNonDefaultSubaccountAddr2
		marketMaker      = testexchange.SampleNonDefaultSubaccountAddr3
		liquidator       = testexchange.SampleNonDefaultSubaccountAddr4
		startingPrice    = sdk.NewDec(2000)
		quoteDenom       string
		insuranceAddress sdk.AccAddress
	)

	BeforeEach(func() {
		if testexchange.IsUsingDefaultSubaccount() {
			Skip("only works with non-default subaccount")
		}
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			oracleBase,
			oracleQuote,
			oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
		)

		insuranceAddress = app.AccountKeeper.GetModuleAccount(ctx, insurancetypes.ModuleName).
			GetAddress()

		insuranceSender := sdk.AccAddress(
			common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"),
		)
		initialInsuranceFundBalance = sdk.NewDec(1000)
		market := testInput.Perps[0]
		quoteDenom = market.QuoteDenom

		coin := sdk.NewCoin(quoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			minttypes.ModuleName,
			insuranceSender,
			sdk.NewCoins(coin),
		)
		testexchange.OrFail(
			app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				insuranceSender,
				coin,
				testInput.Perps[0].Ticker,
				quoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				-1,
			),
		)

		_, _, err := app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			testInput.Perps[0].Ticker,
			quoteDenom,
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
		app.ExchangeKeeper.GetDerivativeMarket(
			ctx,
			testInput.Perps[0].MarketID,
			true,
		)
	})

	var assertMarketMakerHasXOpenOrders = func(expectedCount int) {
		openOrders := app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(
			ctx,
			testInput.Perps[0].MarketID,
			marketMaker,
		)
		Expect(
			len(openOrders),
		).Should(Equal(expectedCount), fmt.Sprintf("market maker should have %d open orders", expectedCount))
	}
	_ = assertMarketMakerHasXOpenOrders

	var assertLiquidatorHasXOpenOrders = func(expectedCount int) {
		openOrders := app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(
			ctx,
			testInput.Perps[0].MarketID,
			liquidator,
		)
		Expect(
			len(openOrders),
		).Should(Equal(expectedCount), fmt.Sprintf("liquidator should have %d open orders", expectedCount))
	}
	_ = assertLiquidatorHasXOpenOrders

	type PositionState bool
	const (
		PositionOpen   PositionState = true
		PositionClosed PositionState = false
	)

	var assertBuyerHasPositionInState = func(expectedOpen PositionState) {
		openPosition := app.ExchangeKeeper.GetPosition(
			ctx,
			testInput.Perps[0].MarketID,
			buyer,
		)

		hasOpenPosition := openPosition != nil

		Expect(
			hasOpenPosition,
		).Should(Equal(bool(expectedOpen)), fmt.Sprintf("buyer's position should be open? %v", expectedOpen))
	}
	_ = assertBuyerHasPositionInState

	var assertMarketHasStatus = func(expectedStatus types.MarketStatus) {
		markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
		Expect(
			markets[0].Status,
		).Should(Equal(expectedStatus), fmt.Sprintf("market didn't have %s status", expectedStatus.String()))
	}
	_ = assertMarketHasStatus

	var assertInsuranceFundBalanceIs = func(expectedBalance sdk.Dec) {
		actualBalance := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)

		Expect(
			actualBalance.Amount.ToDec().String()).
			To(Equal(expectedBalance.String()), "insurnace fund balance was incorrect")
	}

	_ = assertInsuranceFundBalanceIs

	var processBlock = func() {
		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}
	_ = processBlock

	Describe("When liquidating a position", func() {
		var (
			liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin sdk.Dec
			newOraclePrice                                                                sdk.Dec
			initialCoinDeposit                                                            math.Int
		)

		JustBeforeEach(func() {
			// mint quote denom to all market participants
			initialCoinDeposit = sdk.NewInt(200000)
			coin := sdk.NewCoin(quoteDenom, initialCoinDeposit)

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, marketMaker.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, liquidator.String(), sdk.NewCoins(coin))

			// open the position
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(
				liquidatedPositionPrice,
				liquidatedPositionQuantity,
				liquidatedPositionMargin,
				types.OrderType_BUY,
				buyer,
			)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
				liquidatedPositionPrice,
				liquidatedPositionQuantity,
				liquidatedPositionMargin,
				types.OrderType_SELL,
				seller,
			)

			_, err = msgServer.CreateDerivativeLimitOrder(
				sdk.WrapSDKContext(ctx),
				limitDerivativeBuyOrder,
			)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(
				sdk.WrapSDKContext(ctx),
				limitDerivativeSellOrder,
			)
			testexchange.OrFail(err)

			processBlock()

			oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(
				ctx,
				oracleBase,
				oracleQuote,
				oracletypes.NewPriceState(newOraclePrice, ctx.BlockTime().Unix()),
			)
		})

		//this fails!
		AfterEach(func() {
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Describe("with positive liquidation payout", func() {
			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2000)
				liquidatedPositionQuantity = sdk.NewDec(2)
				newOraclePrice = sdk.NewDec(1500) //equal to banruptcy price
			})

			Describe("with sufficient liquidity >= bankruptcy price", func() {
				When("liquidator doesn't provider any order", func() {
					It("should liquidate correctly", func() {
						banruptcyPrice := newOraclePrice
						// post order with price equal to bankruptcy price which will be used for liquidation
						// (this is the worst price possible for market order used to liquidate position)
						sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							banruptcyPrice,
							liquidatedPositionQuantity,
							banruptcyPrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							marketMaker,
						)

						_, err = msgServer.CreateDerivativeLimitOrder(
							sdk.WrapSDKContext(ctx),
							sellOrder,
						)
						testexchange.OrFail(err)

						processBlock()

						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						testexchange.OrFail(err)

						processBlock()

						assertBuyerHasPositionInState(PositionClosed)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
					})
				})

				When("liquidator provides incorrectly priced order", func() {
					It("should still liquidate correctly", func() {
						banruptcyPrice := newOraclePrice

						// post order with price equal to bankruptcy price which will be used for liquidation
						// (this is the worst price possible for market order used to liquidate position)
						sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							banruptcyPrice,
							liquidatedPositionQuantity,
							banruptcyPrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							marketMaker,
						)

						_, err = msgServer.CreateDerivativeLimitOrder(
							sdk.WrapSDKContext(ctx),
							sellOrder,
						)

						testexchange.OrFail(err)
						processBlock()

						// build order with price below bankruptcy price which will be send together with liquidation message
						invalidOrderPrice := banruptcyPrice.Sub(sdk.NewDec(1))
						liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							invalidOrderPrice,
							liquidatedPositionQuantity,
							invalidOrderPrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							liquidator,
						)

						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
						liquidateMsg.Order = &liquidationOrder.Order

						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						Expect(err).To(BeNil(), "should liquidate position")

						processBlock()

						assertBuyerHasPositionInState(PositionClosed)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
					})
				})

				When(
					"liquidator provides better priced order with extra quantity",
					func() {
						It("should liquidate correctly using liquidator's order", func() {
							banruptcyPrice := newOraclePrice

							// post order with price equal to bankruptcy price which will be used for liquidation
							// (this is the worst price possible for market order used to liquidate position)
							sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								banruptcyPrice,
								liquidatedPositionQuantity,
								banruptcyPrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								marketMaker,
							)

							_, err = msgServer.CreateDerivativeLimitOrder(
								sdk.WrapSDKContext(ctx),
								sellOrder,
							)
							testexchange.OrFail(err)

							processBlock()

							// build order with price better than bankruptcy price which will be send together with liquidation message
							betterPrice := banruptcyPrice.Add(sdk.NewDec(1))
							liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								betterPrice,
								liquidatedPositionQuantity.Mul(sdk.NewDec(2)),
								betterPrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								liquidator,
							)

							liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
							liquidateMsg.Order = &liquidationOrder.Order

							_, err = msgServer.LiquidatePosition(
								sdk.WrapSDKContext(ctx),
								liquidateMsg,
							)
							Expect(err).To(BeNil(), "should liquidate position")

							processBlock()

							assertBuyerHasPositionInState(PositionClosed)
							assertMarketMakerHasXOpenOrders(1)
							// liquidator's order was cancelled after filling
							assertLiquidatorHasXOpenOrders(0)
							assertMarketHasStatus(types.MarketStatus_Active)
							// account for better price
							assertInsuranceFundBalanceIs(
								initialInsuranceFundBalance.Add(sdk.NewDec(1)),
							)
						})
					},
				)
			})

			Describe("with insufficient liquidity", func() {

				When("liquidator doesn't provide any order", func() {

					It("should fail to liquidate correctly", func() {
						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

						_, err = msgServer.LiquidatePosition(
							sdk.WrapSDKContext(ctx),
							liquidateMsg,
						)
						Expect(err).To(Not(BeNil()), "should fail to liquidate position")
						Expect(
							err.Error(),
						).To(ContainSubstring("no liquidity on the orderbook"), "wrong error message")

						processBlock()

						assertBuyerHasPositionInState(PositionOpen)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
					})
				})

				When("liquidator provides incorrect order", func() {

					It("should fail to liquidate correctly", func() {
						banruptcyPrice := newOraclePrice
						invalidOrderPrice := banruptcyPrice.Sub(sdk.NewDec(1))

						// build order with price below bankruptcy price which will be send together with liquidation message
						sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							invalidOrderPrice,
							liquidatedPositionQuantity,
							invalidOrderPrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							liquidator,
						)
						testexchange.OrFail(err)

						processBlock()

						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
						liquidateMsg.Order = &sellOrder.Order
						_, err = msgServer.LiquidatePosition(
							sdk.WrapSDKContext(ctx),
							liquidateMsg,
						)
						Expect(err).To(Not(BeNil()), "should fail to liquidate position")
						Expect(
							err.Error(),
						).To(ContainSubstring("no liquidity on the orderbook"), "wrong error message")

						processBlock()

						assertBuyerHasPositionInState(PositionOpen)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
					})
				})

				When(
					"liquidator provides correctly priced order that together with existing orders provides enough liquidity",
					func() {
						It("should liquidate correctly", func() {
							banruptcyPrice := newOraclePrice

							// post order with price equal to bankruptcy price which will be used for liquidation
							// (this is the worst price possible for market order used to liquidate position)
							sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								banruptcyPrice,
								liquidatedPositionQuantity.Quo(sdk.NewDec(2)),
								banruptcyPrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								marketMaker,
							)

							_, err = msgServer.CreateDerivativeLimitOrder(
								sdk.WrapSDKContext(ctx),
								sellOrder,
							)
							testexchange.OrFail(err)

							processBlock()

							// build order with price equal to bankruptcy price which will be send together with liquidation message
							// this order together with resting order should be enough to liquidate the position
							liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								banruptcyPrice,
								liquidatedPositionQuantity.Quo(sdk.NewDec(2)),
								banruptcyPrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								liquidator,
							)

							liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
							liquidateMsg.Order = &liquidationOrder.Order

							_, err = msgServer.LiquidatePosition(
								sdk.WrapSDKContext(ctx),
								liquidateMsg,
							)
							Expect(err).To(BeNil(), "should liquidate position")

							processBlock()

							assertBuyerHasPositionInState(PositionClosed)
							assertMarketMakerHasXOpenOrders(0)
							assertLiquidatorHasXOpenOrders(0)
							assertMarketHasStatus(types.MarketStatus_Active)
							assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
						})
					},
				)

				When(
					"liquidator provides correctly priced order",
					func() {
						It("should liquidate correctly", func() {
							banruptcyPrice := newOraclePrice

							// build order with price equal to bankruptcy price which will be send together with liquidation message
							liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								banruptcyPrice,
								liquidatedPositionQuantity,
								banruptcyPrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								liquidator,
							)

							liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
							liquidateMsg.Order = &liquidationOrder.Order
							_, err = msgServer.LiquidatePosition(
								sdk.WrapSDKContext(ctx),
								liquidateMsg,
							)
							Expect(err).To(BeNil(), "should liquidate position")

							processBlock()

							assertBuyerHasPositionInState(PositionClosed)
							assertMarketMakerHasXOpenOrders(0)
							assertLiquidatorHasXOpenOrders(0)
							assertMarketHasStatus(types.MarketStatus_Active)
							assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
						})
					},
				)
			})
		})

		Describe("with negative liquidation payout", func() {
			var missingFunds sdk.Dec
			var withdrawBuyersBalance = func() {
				buyerLeftBalance := app.ExchangeKeeper.GetDeposit(ctx, buyer, quoteDenom)
				if buyerLeftBalance == nil {
					panic("buyerLeftBalance is nil")
				}

				buyerAccount := types.SubaccountIDToSdkAddress(buyer)
				_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
					Sender:       buyerAccount.String(),
					SubaccountId: buyer.Hex(),
					Amount: sdk.NewCoin(
						quoteDenom,
						buyerLeftBalance.AvailableBalance.RoundInt(),
					),
				})
				testexchange.OrFail(err)
			}

			_ = withdrawBuyersBalance

			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2000)
				liquidatedPositionQuantity = sdk.NewDec(2)
				//100 less than banruptcy price
				newOraclePrice = sdk.NewDec(1400)
				bankruptcyPrice := sdk.NewDec(1500)
				missingFunds = bankruptcyPrice.Sub(newOraclePrice).Mul(liquidatedPositionQuantity)
			})

			Describe("with sufficient liquidity >= oracle price", func() {
				When("liquidator doesn't provider any order", func() {
					It("should liquidate correctly", func() {
						withdrawBuyersBalance()

						processBlock()

						// post order with price equal to oracle price which will be used for liquidation
						// (this is the worst price possible for market order used to liquidate position)
						sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							newOraclePrice,
							liquidatedPositionQuantity,
							newOraclePrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							marketMaker,
						)

						_, err = msgServer.CreateDerivativeLimitOrder(
							sdk.WrapSDKContext(ctx),
							sellOrder,
						)
						testexchange.OrFail(err)

						processBlock()

						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						testexchange.OrFail(err)

						processBlock()

						assertBuyerHasPositionInState(PositionClosed)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance.Sub(missingFunds))
					})
				})

				When("liquidator provides incorrectly priced order", func() {
					It("should still liquidate correctly", func() {
						withdrawBuyersBalance()

						// post order with price equal to oracle price which will be used for liquidation
						// (this is the worst price possible for market order used to liquidate position)
						sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							newOraclePrice,
							liquidatedPositionQuantity,
							newOraclePrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							marketMaker,
						)

						_, err = msgServer.CreateDerivativeLimitOrder(
							sdk.WrapSDKContext(ctx),
							sellOrder,
						)

						testexchange.OrFail(err)
						processBlock()

						// build order with price below oracle price which will be send together with liquidation message
						invalidOrderPrice := newOraclePrice.Sub(sdk.NewDec(1))
						liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							invalidOrderPrice,
							liquidatedPositionQuantity,
							invalidOrderPrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							liquidator,
						)

						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
						liquidateMsg.Order = &liquidationOrder.Order

						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						Expect(err).To(BeNil(), "should liquidate position")

						processBlock()

						assertBuyerHasPositionInState(PositionClosed)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance.Sub(missingFunds))
					})
				})

				When(
					"liquidator provides better priced order with extra quantity",
					func() {
						It("should liquidate correctly using liquidator's order", func() {
							withdrawBuyersBalance()

							// post order with price equal to oracle price which will be used for liquidation
							// (this is the worst price possible for market order used to liquidate position)
							sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								newOraclePrice,
								liquidatedPositionQuantity,
								newOraclePrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								marketMaker,
							)

							_, err = msgServer.CreateDerivativeLimitOrder(
								sdk.WrapSDKContext(ctx),
								sellOrder,
							)
							testexchange.OrFail(err)

							processBlock()

							// build order with price better than oracle price which will be send together with liquidation message
							betterPrice := newOraclePrice.Add(sdk.NewDec(1))
							liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								betterPrice,
								liquidatedPositionQuantity.Mul(sdk.NewDec(2)),
								betterPrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								liquidator,
							)

							liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
							liquidateMsg.Order = &liquidationOrder.Order

							_, err = msgServer.LiquidatePosition(
								sdk.WrapSDKContext(ctx),
								liquidateMsg,
							)
							Expect(err).To(BeNil(), "should liquidate position")

							processBlock()

							assertBuyerHasPositionInState(PositionClosed)
							assertMarketMakerHasXOpenOrders(1)
							// liquidator's order was cancelled after filling
							assertLiquidatorHasXOpenOrders(0)
							assertMarketHasStatus(types.MarketStatus_Active)
							// account for better price
							assertInsuranceFundBalanceIs(
								initialInsuranceFundBalance.Sub(missingFunds).Add(sdk.NewDec(2)),
							)
						})
					},
				)
			})

			Describe("with insufficient liquidity", func() {

				When("liquidator doesn't provide any order", func() {

					It("should fail to liquidate correctly", func() {
						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

						_, err = msgServer.LiquidatePosition(
							sdk.WrapSDKContext(ctx),
							liquidateMsg,
						)
						Expect(err).To(Not(BeNil()), "should fail to liquidate position")
						Expect(
							err.Error(),
						).To(ContainSubstring("no liquidity on the orderbook"), "wrong error message")

						processBlock()

						assertBuyerHasPositionInState(PositionOpen)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
					})
				})

				When("liquidator provides incorrect order", func() {

					It("should fail to liquidate correctly", func() {
						invalidOrderPrice := newOraclePrice.Sub(sdk.NewDec(1))

						// build order with price below oracle price which will be send together with liquidation message
						sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
							invalidOrderPrice,
							liquidatedPositionQuantity,
							invalidOrderPrice.Mul(liquidatedPositionQuantity),
							types.OrderType_BUY,
							liquidator,
						)
						testexchange.OrFail(err)

						processBlock()

						liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
						liquidateMsg.Order = &sellOrder.Order
						_, err = msgServer.LiquidatePosition(
							sdk.WrapSDKContext(ctx),
							liquidateMsg,
						)
						Expect(err).To(Not(BeNil()), "should fail to liquidate position")
						Expect(
							err.Error(),
						).To(ContainSubstring("no liquidity on the orderbook"), "wrong error message")

						processBlock()

						assertBuyerHasPositionInState(PositionOpen)
						assertMarketMakerHasXOpenOrders(0)
						assertLiquidatorHasXOpenOrders(0)
						assertMarketHasStatus(types.MarketStatus_Active)
						assertInsuranceFundBalanceIs(initialInsuranceFundBalance)
					})
				})

				When(
					"liquidator provides correctly priced order that together with existing orders provides enough liquidity",
					func() {
						It("should liquidate correctly", func() {
							withdrawBuyersBalance()

							// post order with price equal to oracle price which will be used for liquidation
							// (this is the worst price possible for market order used to liquidate position)
							sellOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								newOraclePrice,
								liquidatedPositionQuantity.Quo(sdk.NewDec(2)),
								newOraclePrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								marketMaker,
							)

							_, err = msgServer.CreateDerivativeLimitOrder(
								sdk.WrapSDKContext(ctx),
								sellOrder,
							)
							testexchange.OrFail(err)

							processBlock()

							// build order with price equal to oracle price which will be send together with liquidation message
							// this order together with resting order should be enough to liquidate the position
							liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								newOraclePrice,
								liquidatedPositionQuantity.Quo(sdk.NewDec(2)),
								newOraclePrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								liquidator,
							)

							liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
							liquidateMsg.Order = &liquidationOrder.Order

							_, err = msgServer.LiquidatePosition(
								sdk.WrapSDKContext(ctx),
								liquidateMsg,
							)
							Expect(err).To(BeNil(), "should liquidate position")

							processBlock()

							assertBuyerHasPositionInState(PositionClosed)
							assertMarketMakerHasXOpenOrders(0)
							assertLiquidatorHasXOpenOrders(0)
							assertMarketHasStatus(types.MarketStatus_Active)
							assertInsuranceFundBalanceIs(
								initialInsuranceFundBalance.Sub(missingFunds),
							)
						})
					},
				)

				When(
					"liquidator provides correctly priced order",
					func() {
						It("should liquidate correctly", func() {
							withdrawBuyersBalance()

							// build order with price equal to oracle price which will be send together with liquidation message
							liquidationOrder := testInput.NewMsgCreateDerivativeLimitOrder(
								newOraclePrice,
								liquidatedPositionQuantity,
								newOraclePrice.Mul(liquidatedPositionQuantity),
								types.OrderType_BUY,
								liquidator,
							)

							liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
							liquidateMsg.Order = &liquidationOrder.Order
							_, err = msgServer.LiquidatePosition(
								sdk.WrapSDKContext(ctx),
								liquidateMsg,
							)
							Expect(err).To(BeNil(), "should liquidate position")

							processBlock()

							assertBuyerHasPositionInState(PositionClosed)
							assertMarketMakerHasXOpenOrders(0)
							assertLiquidatorHasXOpenOrders(0)
							assertMarketHasStatus(types.MarketStatus_Active)
							assertInsuranceFundBalanceIs(
								initialInsuranceFundBalance.Sub(missingFunds),
							)
						})
					},
				)
			})
		})
	})
})
