package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Binary Options market tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context
		err       error

		market                              *types.BinaryOptionsMarket
		msgServer                           types.MsgServer
		senderBuyer                         = testexchange.SampleAccountAddr1
		senderSeller                        = testexchange.SampleAccountAddr2
		senderNewBuyer                      = testexchange.SampleAccountAddr3
		subaccountIdBuyer                   = testexchange.SampleSubaccountAddr1.String()
		subaccountIdSeller                  = testexchange.SampleSubaccountAddr2.String()
		subaccountIdNewBuyer                = testexchange.SampleSubaccountAddr3.String()
		feeRecipient                        = "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"
		oracleSymbol, oracleProvider, admin string
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 0, 1)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleSymbol, oracleProvider, admin = testInput.BinaryMarkets[0].OracleSymbol, testInput.BinaryMarkets[0].OracleProvider, testInput.BinaryMarkets[0].Admin

		err = app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
			Provider: oracleProvider,
			Relayers: []string{admin},
		})
		testexchange.OrFail(err)

		coin := sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(coin))
		adminAccount, _ := sdk.AccAddressFromBech32(testInput.BinaryMarkets[0].Admin)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, adminAccount, sdk.NewCoins(coin))

		_, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(
			ctx,
			testInput.BinaryMarkets[0].Ticker,
			testInput.BinaryMarkets[0].OracleSymbol,
			testInput.BinaryMarkets[0].OracleProvider,
			oracletypes.OracleType_Provider,
			testInput.BinaryMarkets[0].OracleScaleFactor,
			testInput.BinaryMarkets[0].MakerFeeRate,
			testInput.BinaryMarkets[0].TakerFeeRate,
			testInput.BinaryMarkets[0].ExpirationTimestamp,
			testInput.BinaryMarkets[0].SettlementTimestamp,
			testInput.BinaryMarkets[0].Admin,
			testInput.BinaryMarkets[0].QuoteDenom,
			testInput.BinaryMarkets[0].MinPriceTickSize,
			testInput.BinaryMarkets[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)
		market = app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, testInput.BinaryMarkets[0].MarketID, true)

		coinToDeposit := sdk.NewCoin(market.QuoteDenom, types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor).TruncateInt())
		testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(coinToDeposit))
		testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(coinToDeposit))
		testexchange.MintAndDeposit(app, ctx, subaccountIdNewBuyer, sdk.NewCoins(coinToDeposit))
	})

	createInsuranceFund := func(initialFundBalance math.Int) {
		insuranceFunderAcc := types.SubaccountIDToSdkAddress(common.HexToHash(subaccountIdBuyer))
		coin := sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, initialFundBalance)
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, insuranceFunderAcc, sdk.NewCoins(coin))

		err = app.InsuranceKeeper.CreateInsuranceFund(
			ctx,
			insuranceFunderAcc,
			coin,
			testInput.BinaryMarkets[0].Ticker,
			testInput.BinaryMarkets[0].QuoteDenom,
			testInput.BinaryMarkets[0].OracleSymbol,
			testInput.BinaryMarkets[0].OracleProvider,
			oracletypes.OracleType_Provider,
			-2,
		)
		testexchange.OrFail(err)
	}

	Describe("Expiration testing", func() {
		JustBeforeEach(func() {
			expiryTime := time.Unix(market.ExpirationTimestamp+1, 0)
			ctx = ctx.WithBlockTime(expiryTime)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
		})

		It("should have Expired status", func() {
			market := app.ExchangeKeeper.GetBinaryOptionsMarketByID(ctx, market.MarketID())
			Expect(market.Status).Should(Equal(types.MarketStatus_Expired))
		})

		It("should reject new orders", func() {
			price := types.GetScaledPrice(sdk.NewDecWithPrec(2, 1), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)
			message := types.NewMsgCreateBinaryOptionsLimitOrder(
				senderBuyer,
				market,
				subaccountIdBuyer,
				feeRecipient,
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)
			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
			expectedError := sdkerrors.Wrapf(types.ErrBinaryOptionsMarketNotFound, "marketID %s", market.MarketId)
			Expect(err.Error()).To(Equal(expectedError.Error()))
		})

		When("market had an resting order", func() {
			var (
				initialDeposit *types.Deposit
				order          *types.DerivativeLimitOrder
			)
			BeforeEach(func() {
				msg := types.NewMsgCreateBinaryOptionsLimitOrder(
					senderBuyer,
					market,
					subaccountIdBuyer,
					feeRecipient,
					types.GetScaledPrice(sdk.MustNewDecFromStr("0.5"), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					false,
				)
				testexchange.OrFail(msg.ValidateBasic())
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), msg)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				orders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, market.MarketID(), true, msg.Order.SubaccountID())
				Expect(len(orders)).To(Equal(1))

				isBuy := true
				order = app.ExchangeKeeper.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, market.MarketID(), &isBuy, msg.Order.OrderInfo.SubaccountID(), orders[0])

				initialDeposit = testexchange.GetBankAndDepositFunds(app, ctx, order.SubaccountID(), market.QuoteDenom)
			})
			It("should have no resting orders", func() {
				orders := app.ExchangeKeeper.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, market.MarketID(), true, order.SubaccountID())
				Expect(len(orders)).To(Equal(0))
			})

			It("should refund locked margin to the sender", func() {
				deposit := testexchange.GetBankAndDepositFunds(app, ctx, order.SubaccountID(), market.QuoteDenom)
				Expect(deposit.AvailableBalance.String()).To(Equal(initialDeposit.AvailableBalance.Add(order.GetCancelDepositDelta(market.MakerFeeRate).AvailableBalanceDelta).String()))
			})
		})

		When("market had open positions", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					senderBuyer,
					market,
					subaccountIdBuyer,
					feeRecipient,
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					senderSeller,
					market,
					subaccountIdSeller,
					feeRecipient,
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})
			It("should retain those open positions", func() {
				positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, market.MarketID())
				Expect(len(positions)).To(Equal(2))
			})
		})
	})

	Describe("Settlement testing", func() {
		var (
			buyerDepositBefore, buyerDepositAfter, sellerDepositBefore, sellerDepositAfter *types.Deposit
			entryPrice                                                                     = sdk.MustNewDecFromStr("0.2")
			settlementPrice                                                                sdk.Dec
			quantity                                                                       = sdk.NewDec(2)
			buyerMargin, sellerMargin                                                      sdk.Dec
		)
		// open positions before settlement
		BeforeEach(func() {
			binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				senderBuyer,
				market,
				subaccountIdBuyer,
				feeRecipient,
				types.GetScaledPrice(entryPrice, market.OracleScaleFactor),
				quantity,
				types.OrderType_BUY,
				false,
			)
			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
			testexchange.OrFail(err)

			binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				senderSeller,
				market,
				subaccountIdSeller,
				feeRecipient,
				types.GetScaledPrice(entryPrice, market.OracleScaleFactor),
				quantity,
				types.OrderType_SELL,
				false,
			)
			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
			sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)
			buyerMargin = binaryOptionsLimitBuyOrderMessage.Order.Margin
			sellerMargin = binaryOptionsLimitSellOrderMessage.Order.Margin
		})

		It("should have two open positions", func() {
			positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, market.MarketID())
			Expect(len(positions)).To(Equal(2))
		})

		testSettlementInvariants := func() {
			It("should have Demolished status", func() {
				market := app.ExchangeKeeper.GetBinaryOptionsMarketByID(ctx, market.MarketID())
				Expect(market.Status).Should(Equal(types.MarketStatus_Demolished))
			})

			It("should reject new orders", func() {
				price := types.GetScaledPrice(sdk.MustNewDecFromStr("0.3"), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)
				message := types.NewMsgCreateBinaryOptionsLimitOrder(
					senderBuyer,
					market,
					subaccountIdBuyer,
					feeRecipient,
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				expectedError := sdkerrors.Wrapf(types.ErrBinaryOptionsMarketNotFound, "marketID %s", market.MarketId)
				Expect(err.Error()).To(Equal(expectedError.Error()))
			})

			It("should have no open positions", func() {
				positions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, market.MarketID())
				Expect(len(positions)).To(Equal(0))
			})
		}

		testSettlementPrices := func() {
			When("settlement price == 0", func() {
				BeforeEach(func() {
					settlementPrice = sdk.ZeroDec()
				})

				testSettlementInvariants()

				It("buyer should lose", func() {
					buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
					sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

					expectedBuyerPayout := sdk.ZeroDec()
					expectedSellerPayout := sellerMargin.Add(buyerMargin)

					Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
					Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
				})
			})

			When("settlement price == 1", func() {
				BeforeEach(func() {
					settlementPrice = sdk.OneDec()
				})

				testSettlementInvariants()

				It("buyer should win", func() {
					buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
					sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

					expectedBuyerPayout := sellerMargin.Add(buyerMargin)
					expectedSellerPayout := sdk.ZeroDec()

					Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
					Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
				})
			})

			When("settlement price == 0.5", func() {
				BeforeEach(func() {
					settlementPrice = sdk.MustNewDecFromStr("0.5")
				})

				testSettlementInvariants()

				It("buyer should win by some fraction", func() {
					buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
					sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

					fraction := settlementPrice.Sub(entryPrice).Quo(sdk.OneDec().Sub(entryPrice)) // (0.5 - 0.2) / 0.8
					expectedBuyerPayout := buyerMargin.Add(sellerMargin.Mul(fraction))
					expectedSellerPayout := sellerMargin.Mul(sdk.OneDec().Sub(fraction))

					Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
					Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
				})
			})

			When("settlement price == 0.1", func() {
				BeforeEach(func() {
					settlementPrice = sdk.MustNewDecFromStr("0.1")
				})

				testSettlementInvariants()

				It("seller should win by some fraction", func() {
					buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
					sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

					fraction := entryPrice.Sub(settlementPrice).Quo(entryPrice) // (0.2 - 0.1) / 0.2
					expectedBuyerPayout := buyerMargin.Mul(sdk.OneDec().Sub(fraction))
					expectedSellerPayout := sellerMargin.Add(buyerMargin.Mul(fraction))

					Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
					Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
				})
			})
		}

		Context("Forcefully settled", func() {
			sellBuyersPositionAndRecalculateValues := func(newEntryPrice sdk.Dec) (newBuyerMargin sdk.Dec, marginDifference sdk.Dec) {
				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					senderNewBuyer,
					market,
					subaccountIdNewBuyer,
					feeRecipient,
					types.GetScaledPrice(newEntryPrice, market.OracleScaleFactor),
					quantity,
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					senderBuyer,
					market,
					subaccountIdBuyer,
					feeRecipient,
					types.GetScaledPrice(newEntryPrice, market.OracleScaleFactor),
					quantity,
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				newBuyerMargin = binaryOptionsLimitBuyOrderMessage.Order.Margin

				// update original buyer's deposit after he completely reduced his position
				buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)

				originalBuyMargin := types.GetRequiredBinaryOptionsOrderMargin(types.GetScaledPrice(entryPrice, market.OracleScaleFactor), quantity, market.OracleScaleFactor, types.OrderType_BUY, false)
				originalSellMargin := types.GetRequiredBinaryOptionsOrderMargin(types.GetScaledPrice(entryPrice, market.OracleScaleFactor), quantity, market.OracleScaleFactor, types.OrderType_SELL, false)

				// calculate new margin: old sell margin + new buy margin
				newBuyMargin := types.GetRequiredBinaryOptionsOrderMargin(types.GetScaledPrice(newEntryPrice, market.OracleScaleFactor), quantity, market.OracleScaleFactor, types.OrderType_BUY, false)
				originalTotalMargin := originalBuyMargin.Add(originalSellMargin)
				newTotalMargin := originalSellMargin.Add(newBuyMargin)

				return newBuyerMargin, originalTotalMargin.Sub(newTotalMargin).Abs()
			}

			postLimitOrderOrFail := func(price, quantity sdk.Dec, subaccountId string, orderType types.OrderType) *types.MsgCreateBinaryOptionsLimitOrder {
				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					types.SubaccountIDToSdkAddress(common.HexToHash(subaccountId)),
					market,
					subaccountId,
					feeRecipient,
					types.GetScaledPrice(price, market.OracleScaleFactor),
					quantity,
					orderType,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err)

				return binaryOptionsLimitOrderMessage
			}

			postReduceOnlyOrderOrFail := func(price sdk.Dec, subaccountId string, orderType types.OrderType) {
				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					types.SubaccountIDToSdkAddress(common.HexToHash(subaccountId)),
					market,
					subaccountId,
					feeRecipient,
					types.GetScaledPrice(price, market.OracleScaleFactor),
					quantity,
					orderType,
					true,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err)
			}

			_ = postReduceOnlyOrderOrFail

			When("via Admin update", func() {
				var (
					msg *types.MsgAdminUpdateBinaryOptionsMarket
				)
				BeforeEach(func() {
					msg = &types.MsgAdminUpdateBinaryOptionsMarket{
						Sender:          market.Admin,
						MarketId:        market.MarketId,
						SettlementPrice: nil,
						Status:          types.MarketStatus_Demolished,
					}
				})
				JustBeforeEach(func() {
					msg.SettlementPrice = &settlementPrice
					Expect(msg.ValidateBasic()).To(BeNil())

					_, err := msgServer.AdminUpdateBinaryOptionsMarket(sdk.WrapSDKContext(ctx), msg)
					testexchange.OrFail(err)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
				})

				testSettlementPrices()

				When("settlement price == -1", func() {

					When("there is no insurance and there's enough funds to refund the margin", func() {
						BeforeEach(func() {
							settlementPrice = sdk.MustNewDecFromStr("-1")
						})

						When("there are no RO orders", func() {
							testSettlementInvariants()

							It("everyone should be fully refunded", func() {
								buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
								sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

								expectedBuyerPayout := buyerMargin
								expectedSellerPayout := sellerMargin

								Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
								Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
							})
						})

						When("both traders have RO orders", func() {
							When("there is no insurance and there's enough funds to refund the margin", func() {
								BeforeEach(func() {
									postReduceOnlyOrderOrFail(sdk.MustNewDecFromStr("0.75"), subaccountIdBuyer, types.OrderType_SELL)
									postReduceOnlyOrderOrFail(sdk.MustNewDecFromStr("0.15"), subaccountIdSeller, types.OrderType_BUY)

									ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
									Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
								})

								testSettlementInvariants()

								It("everyone should be fully refunded", func() {
									buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

									expectedBuyerPayout := buyerMargin
									expectedSellerPayout := sellerMargin

									Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
									Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")
								})
							})
						})

						When("both traders have unfilled resting vanilla orders", func() {
							var (
								newBuyOrder, newSellOrder types.DerivativeOrder
							)
							When("there is no insurance and there's enough funds to refund the margin", func() {
								BeforeEach(func() {
									buyOrder := postLimitOrderOrFail(sdk.MustNewDecFromStr("0.2"), quantity, subaccountIdBuyer, types.OrderType_BUY)
									sellOrder := postLimitOrderOrFail(sdk.MustNewDecFromStr("0.8"), quantity, subaccountIdSeller, types.OrderType_SELL)

									newBuyOrder = buyOrder.Order
									newSellOrder = sellOrder.Order

									ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
									Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

									buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)
								})

								testSettlementInvariants()

								It("everyone should be fully refunded", func() {
									buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

									expectedBuyerPayout := buyerMargin
									expectedSellerPayout := sellerMargin

									Expect(buyerDepositAfter.AvailableBalance.LTE(buyerDepositAfter.TotalBalance)).To(BeTrue(), "buyer had more available than total")

									Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
									Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

									// simplified calulation, because we know that order wasn't filled
									feeRate := testInput.BinaryMarkets[0].MakerFeeRate

									// use notional to calculate fee, not margin!
									newBuyOrderValueWithFee := newBuyOrder.OrderInfo.Quantity.Mul(newBuyOrder.OrderInfo.Price).Mul(feeRate).Add(newBuyOrder.Margin)
									newSellOrderValueWithFee := newSellOrder.OrderInfo.Quantity.Mul(newSellOrder.OrderInfo.Price).Mul(feeRate).Add(newSellOrder.Margin)

									expectedBuyerPayout = buyerMargin.Add(newBuyOrderValueWithFee)
									expectedSellerPayout = sellerMargin.Add(newSellOrderValueWithFee)

									// resting orders decrease only available balance (not total), so we expect it to increase by resting orders margin and fee
									Expect(buyerDepositAfter.AvailableBalance.String()).To(Equal(buyerDepositBefore.AvailableBalance.Add(expectedBuyerPayout).String()), "available buyer deposit")
									Expect(sellerDepositAfter.AvailableBalance.String()).To(Equal(sellerDepositBefore.AvailableBalance.Add(expectedSellerPayout).String()), "available seller deposit")
								})
							})
						})

						When("one trader has partially filled resting vanilla order", func() {
							var (
								newBuyOrder, newSellOrder types.DerivativeOrder
							)
							When("there is no insurance and there's enough funds to refund the margin", func() {
								BeforeEach(func() {
									halfQuantity := quantity.Quo(sdk.NewDec(2))
									buyOrder := postLimitOrderOrFail(entryPrice, quantity, subaccountIdBuyer, types.OrderType_BUY)
									sellOrder := postLimitOrderOrFail(entryPrice, halfQuantity, subaccountIdSeller, types.OrderType_SELL)

									newBuyOrder = buyOrder.Order
									newSellOrder = sellOrder.Order

									// add increased position margin to expected payout
									buyerMargin = buyerMargin.Add(newBuyOrder.Margin.Quo(sdk.NewDec(2)))
									sellerMargin = sellerMargin.Add(newSellOrder.Margin)

									ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
									Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

									buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)
								})

								testSettlementInvariants()

								It("everyone should be fully refunded", func() {
									buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

									expectedBuyerPayout := buyerMargin
									expectedSellerPayout := sellerMargin

									Expect(buyerDepositAfter.AvailableBalance.LTE(buyerDepositAfter.TotalBalance)).To(BeTrue(), "buyer had more available than total")

									Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
									Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

									// simplified calulation, because we know that order wasn filled in 50%
									feeRate := testInput.BinaryMarkets[0].MakerFeeRate

									// use notional to calculate fee, not margin
									newBuyOrderValueWithFee := newBuyOrder.OrderInfo.Quantity.Quo(sdk.NewDec(2)).Mul(newBuyOrder.OrderInfo.Price).Mul(feeRate).Add(newBuyOrder.Margin.Quo(sdk.NewDec(2)))
									expectedBuyerPayout = buyerMargin.Add(newBuyOrderValueWithFee)

									// resting orders decrease only available balance (not total), so we expect it to increase by resting orders margin and fee
									Expect(buyerDepositAfter.AvailableBalance.String()).To(Equal(buyerDepositBefore.AvailableBalance.Add(expectedBuyerPayout).String()), "available buyer deposit")
									Expect(sellerDepositAfter.AvailableBalance.String()).To(Equal(sellerDepositBefore.AvailableBalance.Add(expectedSellerPayout).String()), "available seller deposit")
								})
							})
						})
					})

					When("there is no insurance and there's not enough funds to refund the margin", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							missingMargin         sdk.Dec
						)
						BeforeEach(func() {
							newBuyerMargin, missingMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.3"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded after applying haircut", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							totalQuantity := quantity.Mul(sdk.NewDec(2))
							assets := types.GetScaledPrice(totalQuantity, market.GetOracleScaleFactor()).Quo(sdk.NewDec(2))
							haircutPercentage := missingMargin.Quo(assets)

							Expect(haircutPercentage.String()).To(Equal(sdk.MustNewDecFromStr("0.1").String()))

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive payouts decresed by haircut percentage
							expectedNewBuyerPayout := newBuyerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))
							expectedSellerPayout := sellerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")
						})
					})

					When("there is insurance and there it has enough funds to refund the margin", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							missingMargin         sdk.Dec
							initialFundBalance    = sdk.NewInt(300000)
						)
						BeforeEach(func() {
							createInsuranceFund(initialFundBalance)
							newBuyerMargin, missingMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.3"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded fully and insurance fund should pay for the loss", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive full payouts
							expectedNewBuyerPayout := newBuyerMargin
							expectedSellerPayout := sellerMargin

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.BinaryMarkets[0].MarketID)
							expectedInsuranceFundBalance := initialFundBalance.Sub(missingMargin.TruncateInt())

							Expect(insuranceFund.Balance).To(Equal(expectedInsuranceFundBalance), "insurance fund balance was incorrect")
						})
					})

					When("there is insurance and there but it doesn't enough funds to refund the whole margin", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							missingMargin         sdk.Dec
							initialFundBalance    = sdk.NewInt(100000)
						)
						BeforeEach(func() {
							createInsuranceFund(initialFundBalance)
							newBuyerMargin, missingMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.3"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded partially, but insurance fund should also pay for the loss", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							missingMarginAfterInsurancePayout := missingMargin.Sub(initialFundBalance.ToDec())

							totalQuantity := quantity.Mul(sdk.NewDec(2))
							assets := types.GetScaledPrice(totalQuantity, market.GetOracleScaleFactor()).Quo(sdk.NewDec(2))
							haircutPercentage := missingMarginAfterInsurancePayout.Quo(assets)

							Expect(haircutPercentage.String()).To(Equal(sdk.MustNewDecFromStr("0.05").String()))

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive payouts decresed by haircut percentage
							expectedNewBuyerPayout := newBuyerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))
							expectedSellerPayout := sellerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.BinaryMarkets[0].MarketID)
							Expect(insuranceFund.Balance).To(Equal(sdk.NewInt(0)), "insurance fund balance was not zeroed")
						})
					})

					When("there is insurance fund and there's a surplus available", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							surplusMargin         sdk.Dec
							initialFundBalance    = sdk.NewInt(1000000)
						)
						BeforeEach(func() {
							settlementPrice = sdk.MustNewDecFromStr("-1")
							createInsuranceFund(initialFundBalance)
							newBuyerMargin, surplusMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.1"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded fully and the surplus should go to insurance fund", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive full payouts
							expectedNewBuyerPayout := newBuyerMargin
							expectedSellerPayout := sellerMargin

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.BinaryMarkets[0].MarketID)
							Expect(insuranceFund.Balance).To(Equal(initialFundBalance.Add(surplusMargin.TruncateInt())), "insurance fund balance")
						})
					})

					When("there is no insurance fund, but there's a surplus available", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							surplusMargin         sdk.Dec
							initialAuctionDeposit *types.Deposit
						)
						BeforeEach(func() {
							newBuyerMargin, surplusMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.1"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							initialAuctionDeposit = testexchange.GetBankAndDepositFunds(app, ctx, types.AuctionSubaccountID, market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded fully and the surplus should go to auction", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive fulll payouts
							expectedNewBuyerPayout := newBuyerMargin
							expectedSellerPayout := sellerMargin

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							auctionDeposit := testexchange.GetBankAndDepositFunds(app, ctx, types.AuctionSubaccountID, market.QuoteDenom)
							Expect(auctionDeposit.AvailableBalance.String()).To(Equal(initialAuctionDeposit.AvailableBalance.Add(surplusMargin).String()), "available")
							Expect(auctionDeposit.AvailableBalance.String()).To(Equal(initialAuctionDeposit.TotalBalance.Add(surplusMargin).String()), "total")
						})
					})
				})
			})

			When("via governance proposal", func() {
				var (
					msg *types.BinaryOptionsMarketParamUpdateProposal
				)
				BeforeEach(func() {
					msg = &types.BinaryOptionsMarketParamUpdateProposal{
						Title:           "binary options proposal title",
						Description:     "binary options proposal Description",
						MarketId:        market.MarketId,
						SettlementPrice: nil,
						Status:          types.MarketStatus_Demolished,
					}
				})
				JustBeforeEach(func() {
					msg.SettlementPrice = &settlementPrice
					Expect(msg.ValidateBasic()).To(BeNil())

					err := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)(ctx, msg)
					testexchange.OrFail(err)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
				})

				testSettlementPrices()

				When("settlement price == -1", func() {
					BeforeEach(func() {
						settlementPrice = sdk.MustNewDecFromStr("-1")
					})

					When("there is no insurance and there's enough funds to refund the margin", func() {
						BeforeEach(func() {
							settlementPrice = sdk.MustNewDecFromStr("-1")
						})

						When("there are no RO orders", func() {
							testSettlementInvariants()

							It("everyone should be fully refunded", func() {
								buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
								sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

								expectedBuyerPayout := buyerMargin
								expectedSellerPayout := sellerMargin

								Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
								Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
							})
						})

						When("both trades have RO orders", func() {
							When("there is no insurance and there's enough funds to refund the margin", func() {
								BeforeEach(func() {
									postReduceOnlyOrderOrFail(sdk.MustNewDecFromStr("0.75"), subaccountIdBuyer, types.OrderType_SELL)
									postReduceOnlyOrderOrFail(sdk.MustNewDecFromStr("0.15"), subaccountIdSeller, types.OrderType_BUY)

									ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
									Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
								})

								testSettlementInvariants()

								It("everyone should be fully refunded", func() {
									buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

									expectedBuyerPayout := buyerMargin
									expectedSellerPayout := sellerMargin

									Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
									Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")
								})
							})
						})

						When("both traders have unfilled resting vanilla orders", func() {
							var (
								newBuyOrder, newSellOrder types.DerivativeOrder
							)
							When("there is no insurance and there's enough funds to refund the margin", func() {
								BeforeEach(func() {
									buyOrder := postLimitOrderOrFail(sdk.MustNewDecFromStr("0.2"), quantity, subaccountIdBuyer, types.OrderType_BUY)
									sellOrder := postLimitOrderOrFail(sdk.MustNewDecFromStr("0.8"), quantity, subaccountIdSeller, types.OrderType_SELL)

									newBuyOrder = buyOrder.Order
									newSellOrder = sellOrder.Order

									ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
									Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

									buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)
								})

								testSettlementInvariants()

								It("everyone should be fully refunded", func() {
									buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

									expectedBuyerPayout := buyerMargin
									expectedSellerPayout := sellerMargin

									Expect(buyerDepositAfter.AvailableBalance.LTE(buyerDepositAfter.TotalBalance)).To(BeTrue(), "buyer had more available than total")

									Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
									Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

									// simplified calulation, because we know that order wasn't filled
									feeRate := testInput.BinaryMarkets[0].MakerFeeRate

									// use notional to calculate fee, not margin!
									newBuyOrderValueWithFee := newBuyOrder.OrderInfo.Quantity.Mul(newBuyOrder.OrderInfo.Price).Mul(feeRate).Add(newBuyOrder.Margin)
									newSellOrderValueWithFee := newSellOrder.OrderInfo.Quantity.Mul(newSellOrder.OrderInfo.Price).Mul(feeRate).Add(newSellOrder.Margin)

									expectedBuyerPayout = buyerMargin.Add(newBuyOrderValueWithFee)
									expectedSellerPayout = sellerMargin.Add(newSellOrderValueWithFee)

									// resting orders decrease only available balance (not total), so we expect it to increase by resting orders margin and fee
									Expect(buyerDepositAfter.AvailableBalance.String()).To(Equal(buyerDepositBefore.AvailableBalance.Add(expectedBuyerPayout).String()), "available buyer deposit")
									Expect(sellerDepositAfter.AvailableBalance.String()).To(Equal(sellerDepositBefore.AvailableBalance.Add(expectedSellerPayout).String()), "available seller deposit")
								})
							})
						})

						When("one trader has partially filled resting vanilla order", func() {
							var (
								newBuyOrder, newSellOrder types.DerivativeOrder
							)
							When("there is no insurance and there's enough funds to refund the margin", func() {
								BeforeEach(func() {
									halfQuantity := quantity.Quo(sdk.NewDec(2))
									buyOrder := postLimitOrderOrFail(entryPrice, quantity, subaccountIdBuyer, types.OrderType_BUY)
									sellOrder := postLimitOrderOrFail(entryPrice, halfQuantity, subaccountIdSeller, types.OrderType_SELL)

									newBuyOrder = buyOrder.Order
									newSellOrder = sellOrder.Order

									// add increased position margin to expected payout
									buyerMargin = buyerMargin.Add(newBuyOrder.Margin.Quo(sdk.NewDec(2)))
									sellerMargin = sellerMargin.Add(newSellOrder.Margin)

									ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
									Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

									buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)
								})

								testSettlementInvariants()

								It("everyone should be fully refunded", func() {
									buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
									sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

									expectedBuyerPayout := buyerMargin
									expectedSellerPayout := sellerMargin

									Expect(buyerDepositAfter.AvailableBalance.LTE(buyerDepositAfter.TotalBalance)).To(BeTrue(), "buyer had more available than total")

									Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
									Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

									// simplified calulation, because we know that order wasn filled in 50%
									feeRate := testInput.BinaryMarkets[0].MakerFeeRate

									// use notional to calculate fee, not margin
									newBuyOrderValueWithFee := newBuyOrder.OrderInfo.Quantity.Quo(sdk.NewDec(2)).Mul(newBuyOrder.OrderInfo.Price).Mul(feeRate).Add(newBuyOrder.Margin.Quo(sdk.NewDec(2)))
									expectedBuyerPayout = buyerMargin.Add(newBuyOrderValueWithFee)

									// resting orders decrease only available balance (not total), so we expect it to increase by resting orders margin and fee
									Expect(buyerDepositAfter.AvailableBalance.String()).To(Equal(buyerDepositBefore.AvailableBalance.Add(expectedBuyerPayout).String()), "available buyer deposit")
									Expect(sellerDepositAfter.AvailableBalance.String()).To(Equal(sellerDepositBefore.AvailableBalance.Add(expectedSellerPayout).String()), "available seller deposit")
								})
							})
						})
					})

					When("there is no insurance and there's not enough funds to refund the margin", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							missingMargin         sdk.Dec
						)
						BeforeEach(func() {
							newBuyerMargin, missingMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.3"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded after applying haircut", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							totalQuantity := quantity.Mul(sdk.NewDec(2))
							assets := types.GetScaledPrice(totalQuantity, market.GetOracleScaleFactor()).Quo(sdk.NewDec(2))
							haircutPercentage := missingMargin.Quo(assets)

							Expect(haircutPercentage.String()).To(Equal(sdk.MustNewDecFromStr("0.1").String()))

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive payouts decresed by haircut percentage
							expectedNewBuyerPayout := newBuyerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))
							expectedSellerPayout := sellerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")
						})
					})

					When("there is insurance and there it has enough funds to refund the margin", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							missingMargin         sdk.Dec
							initialFundBalance    = sdk.NewInt(300000)
						)
						BeforeEach(func() {
							createInsuranceFund(initialFundBalance)
							newBuyerMargin, missingMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.3"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded fully and insurance fund should pay for the loss", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive full payouts
							expectedNewBuyerPayout := newBuyerMargin
							expectedSellerPayout := sellerMargin

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.BinaryMarkets[0].MarketID)
							expectedInsuranceFundBalance := initialFundBalance.Sub(missingMargin.TruncateInt())

							Expect(insuranceFund.Balance).To(Equal(expectedInsuranceFundBalance), "insurance fund balance was incorrect")
						})
					})

					When("there is insurance and there but it doesn't enough funds to refund the whole margin", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							missingMargin         sdk.Dec
							initialFundBalance    = sdk.NewInt(100000)
						)
						BeforeEach(func() {
							createInsuranceFund(initialFundBalance)
							newBuyerMargin, missingMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.3"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded partially, but insurance fund should also pay for the loss", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							missingMarginAfterInsurancePayout := missingMargin.Sub(initialFundBalance.ToDec())

							totalQuantity := quantity.Mul(sdk.NewDec(2))
							assets := types.GetScaledPrice(totalQuantity, market.GetOracleScaleFactor()).Quo(sdk.NewDec(2))
							haircutPercentage := missingMarginAfterInsurancePayout.Quo(assets)

							Expect(haircutPercentage.String()).To(Equal(sdk.MustNewDecFromStr("0.05").String()))

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive payouts decresed by haircut percentage
							expectedNewBuyerPayout := newBuyerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))
							expectedSellerPayout := sellerMargin.Mul(sdk.OneDec().Sub(haircutPercentage))

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.BinaryMarkets[0].MarketID)
							Expect(insuranceFund.Balance).To(Equal(sdk.NewInt(0)), "insurance fund balance was not zeroed")
						})
					})

					When("there is insurance fund and there's a surplus available", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							surplusMargin         sdk.Dec
							initialFundBalance    = sdk.NewInt(1000000)
						)
						BeforeEach(func() {
							settlementPrice = sdk.MustNewDecFromStr("-1")
							createInsuranceFund(initialFundBalance)
							newBuyerMargin, surplusMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.1"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded fully and the surplus should go to insurance fund", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive full payouts
							expectedNewBuyerPayout := newBuyerMargin
							expectedSellerPayout := sellerMargin

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.BinaryMarkets[0].MarketID)
							Expect(insuranceFund.Balance).To(Equal(initialFundBalance.Add(surplusMargin.TruncateInt())), "insurance fund balance")
						})
					})

					When("there is no insurance fund, but there's a surplus available", func() {
						var (
							newBuyerDepositBefore *types.Deposit
							newBuyerMargin        sdk.Dec
							surplusMargin         sdk.Dec
							initialAuctionDeposit *types.Deposit
						)
						BeforeEach(func() {
							newBuyerMargin, surplusMargin = sellBuyersPositionAndRecalculateValues(sdk.MustNewDecFromStr("0.1"))
							newBuyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							initialAuctionDeposit = testexchange.GetBankAndDepositFunds(app, ctx, types.AuctionSubaccountID, market.QuoteDenom)
						})

						testSettlementInvariants()

						It("market participants should be refunded fully and the surplus should go to auction", func() {
							buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
							newBuyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdNewBuyer), market.QuoteDenom)
							sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

							// original buyer should not get any refund
							expectedBuyerPayout := sdk.ZeroDec()

							// traders with open positions should receive fulll payouts
							expectedNewBuyerPayout := newBuyerMargin
							expectedSellerPayout := sellerMargin

							Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()), "buyer deposit")
							Expect(newBuyerDepositAfter.TotalBalance.String()).To(Equal(newBuyerDepositBefore.TotalBalance.Add(expectedNewBuyerPayout).String()), "new buyer deposit")
							Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()), "seller deposit")

							auctionDeposit := testexchange.GetBankAndDepositFunds(app, ctx, types.AuctionSubaccountID, market.QuoteDenom)
							Expect(auctionDeposit.AvailableBalance.String()).To(Equal(initialAuctionDeposit.AvailableBalance.Add(surplusMargin).String()), "available")
							Expect(auctionDeposit.AvailableBalance.String()).To(Equal(initialAuctionDeposit.TotalBalance.Add(surplusMargin).String()), "total")
						})
					})
				})
			})
		})

		Context("Naturally settled with oracle price", func() {
			JustBeforeEach(func() {
				app.OracleKeeper.SetProviderPriceState(ctx, oracleProvider, oracletypes.NewProviderPriceState(oracleSymbol, settlementPrice, ctx.BlockTime().Unix()))

				settlementTime := time.Unix(market.SettlementTimestamp+1, 0)
				ctx = ctx.WithBlockTime(settlementTime)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			testSettlementPrices()

			When("settlement price == -1", func() {
				BeforeEach(func() {
					settlementPrice = sdk.MustNewDecFromStr("-1")
				})

				testSettlementInvariants()

				It("should settle at price == 0 (buyer loses)", func() {
					buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
					sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

					expectedBuyerPayout := sdk.ZeroDec()
					expectedSellerPayout := sellerMargin.Add(buyerMargin)

					Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
					Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
				})
			})

			When("settlement price == 2", func() {
				BeforeEach(func() {
					settlementPrice = sdk.MustNewDecFromStr("2")
				})

				testSettlementInvariants()

				It("should settle at price == 1 (seller loses)", func() {
					buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
					sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

					expectedBuyerPayout := sellerMargin.Add(buyerMargin)
					expectedSellerPayout := sdk.ZeroDec()

					Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
					Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
				})
			})
		})

		Context("Naturally settled without oracle price", func() {
			JustBeforeEach(func() {
				settlementTime := time.Unix(market.SettlementTimestamp+1, 0)
				ctx = ctx.WithBlockTime(settlementTime)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			testSettlementInvariants()

			It("should refund", func() {
				buyerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), market.QuoteDenom)
				sellerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)

				expectedBuyerPayout := buyerMargin
				expectedSellerPayout := sellerMargin

				Expect(buyerDepositAfter.TotalBalance.String()).To(Equal(buyerDepositBefore.TotalBalance.Add(expectedBuyerPayout).String()))
				Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sellerDepositBefore.TotalBalance.Add(expectedSellerPayout).String()))
			})
		})
	})
})
