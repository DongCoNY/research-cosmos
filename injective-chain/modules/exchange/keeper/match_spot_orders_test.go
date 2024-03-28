package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Order Matching Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		spotMarket *types.SpotMarket
		msgServer  types.MsgServer
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)

	})

	Describe("Succeeds", func() {
		Describe("when a spot limit buy order", func() {
			var (
				startingQuoteDepositBuyer  *types.Deposit
				startingQuoteDepositSeller *types.Deposit
				startingBaseDepositBuyer   *types.Deposit
				startingBaseDepositSeller  *types.Deposit
				limitBuyOrderMsg           *types.MsgCreateSpotLimitOrder
				limitSellOrderMsg          *types.MsgCreateSpotLimitOrder
			)

			Describe("with equal price and quantity and different subaccounts", func() {
				limitQuantity := sdk.NewDec(1)
				limitPrice := sdk.NewDec(2000)
				buyer := testexchange.SampleSubaccountAddr1
				seller := testexchange.SampleSubaccountAddr2

				BeforeEach(func() {
					By("Constructing the limit buy order")
					limitBuyOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitPrice, limitQuantity, types.OrderType_BUY, buyer)

					By("Depositing funds into subaccount of buyer")
					startingQuoteDepositBuyer = &types.Deposit{
						AvailableBalance: sdk.NewDec(10000),
						TotalBalance:     sdk.NewDec(10000),
					}
					testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, startingQuoteDepositBuyer.AvailableBalance.TruncateInt())))

					startingBaseDepositBuyer = &types.Deposit{
						AvailableBalance: sdk.ZeroDec(),
						TotalBalance:     sdk.ZeroDec(),
					}

					By("Creating the spot limit buy order")
					msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsg)
				})

				Describe("in the same block", func() {
					JustBeforeEach(func() {
						By("Constructing the limit sell order from different subaccount")
						limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitPrice, limitQuantity, types.OrderType_SELL, seller)

						By("Depositing funds into subaccount of seller")
						startingBaseDepositSeller = &types.Deposit{
							AvailableBalance: sdk.NewDec(2),
							TotalBalance:     sdk.NewDec(2),
						}
						testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositSeller.AvailableBalance.TruncateInt())))

						startingQuoteDepositSeller = &types.Deposit{
							AvailableBalance: sdk.ZeroDec(),
							TotalBalance:     sdk.ZeroDec(),
						}

						By("Creating the spot limit sell order")
						_, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)
						testexchange.OrFail(err)

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("is getting matched with a spot limit sell order", func() {
						It("has correct balances for buyer for base and quote asset", func() {
							depositBuyerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].BaseDenom)

							expectedBuyerBaseTotalBalance := startingBaseDepositBuyer.TotalBalance.Add(limitQuantity)
							expectedBuyerBaseAvailableBalance := startingBaseDepositBuyer.AvailableBalance.Add(limitQuantity)

							Expect(depositBuyerBaseAsset.TotalBalance).To(Equal(expectedBuyerBaseTotalBalance))
							Expect(depositBuyerBaseAsset.AvailableBalance).To(Equal(expectedBuyerBaseAvailableBalance))

							notional := limitPrice.Mul(limitQuantity)
							balancePaid := notional.Add(notional.Mul(spotMarket.TakerFeeRate))
							depositBuyerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].QuoteDenom)

							expectedBuyerQuoteTotalBalance := startingQuoteDepositBuyer.TotalBalance.Sub(balancePaid)
							expectedBuyerQuoteAvailableBalance := startingQuoteDepositBuyer.AvailableBalance.Sub(balancePaid)

							Expect(depositBuyerQuoteAsset.TotalBalance).To(Equal(expectedBuyerQuoteTotalBalance))
							Expect(depositBuyerQuoteAsset.AvailableBalance).To(Equal(expectedBuyerQuoteAvailableBalance))
						})

						It("has correct balances for seller for base and quote asset", func() {
							depositSellerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Spots[0].BaseDenom)

							expectedSellerBaseTotalBalance := startingBaseDepositBuyer.TotalBalance.Add(limitQuantity)
							expectedSellerBaseAvailableBalance := startingBaseDepositBuyer.AvailableBalance.Add(limitQuantity)

							Expect(depositSellerBaseAsset.TotalBalance).To(Equal(expectedSellerBaseTotalBalance))
							Expect(depositSellerBaseAsset.AvailableBalance).To(Equal(expectedSellerBaseAvailableBalance))

							notional := limitPrice.Mul(limitQuantity)
							balanceGathered := notional.Sub(notional.Mul(spotMarket.TakerFeeRate))
							depositSellerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Spots[0].QuoteDenom)

							expectedSellerQuoteTotalBalance := startingQuoteDepositSeller.TotalBalance.Add(balanceGathered)
							expectedSellerQuoteAvailableBalance := startingQuoteDepositSeller.AvailableBalance.Add(balanceGathered)

							Expect(depositSellerQuoteAsset.TotalBalance).To(Equal(expectedSellerQuoteTotalBalance))
							Expect(depositSellerQuoteAsset.AvailableBalance).To(Equal(expectedSellerQuoteAvailableBalance))
						})
					})
				})

				Describe("in different blocks", func() {
					BeforeEach(func() {
						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					JustBeforeEach(func() {
						By("Constructing the limit sell order from different subaccount")
						limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitPrice, limitQuantity, types.OrderType_SELL, seller)

						By("Depositing funds into subaccount of seller")
						startingBaseDepositSeller = &types.Deposit{
							AvailableBalance: sdk.NewDec(2),
							TotalBalance:     sdk.NewDec(2),
						}
						testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositSeller.AvailableBalance.TruncateInt())))
						startingQuoteDepositSeller = &types.Deposit{
							AvailableBalance: sdk.ZeroDec(),
							TotalBalance:     sdk.ZeroDec(),
						}

						By("Creating the spot limit sell order")
						_, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)
						testexchange.OrFail(err)

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("is getting matched with a spot limit sell order", func() {
						It("has correct balances for buyer for base and quote asset", func() {
							depositBuyerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].BaseDenom)

							expectedBuyerBaseTotalBalance := startingBaseDepositBuyer.TotalBalance.Add(limitQuantity)
							expectedBuyerBaseAvailableBalance := startingBaseDepositBuyer.AvailableBalance.Add(limitQuantity)

							Expect(depositBuyerBaseAsset.TotalBalance).To(Equal(expectedBuyerBaseTotalBalance))
							Expect(depositBuyerBaseAsset.AvailableBalance).To(Equal(expectedBuyerBaseAvailableBalance))

							notional := limitPrice.Mul(limitQuantity)
							balancePaid := notional.Add(notional.Mul(spotMarket.MakerFeeRate))
							depositBuyerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].QuoteDenom)

							expectedBuyerQuoteTotalBalance := startingQuoteDepositBuyer.TotalBalance.Sub(balancePaid)
							expectedBuyerQuoteAvailableBalance := startingQuoteDepositBuyer.AvailableBalance.Sub(balancePaid)

							Expect(depositBuyerQuoteAsset.TotalBalance).To(Equal(expectedBuyerQuoteTotalBalance))
							Expect(depositBuyerQuoteAsset.AvailableBalance).To(Equal(expectedBuyerQuoteAvailableBalance))
						})

						It("has correct balances for seller for base and quote asset", func() {
							depositSellerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Spots[0].BaseDenom)

							expectedSellerBaseTotalBalance := startingBaseDepositBuyer.TotalBalance.Add(limitQuantity)
							expectedSellerBaseAvailableBalance := startingBaseDepositBuyer.AvailableBalance.Add(limitQuantity)

							Expect(depositSellerBaseAsset.TotalBalance).To(Equal(expectedSellerBaseTotalBalance))
							Expect(depositSellerBaseAsset.AvailableBalance).To(Equal(expectedSellerBaseAvailableBalance))

							notional := limitPrice.Mul(limitQuantity)
							balanceGathered := notional.Sub(notional.Mul(spotMarket.TakerFeeRate))
							depositSellerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Spots[0].QuoteDenom)

							expectedSellerQuoteTotalBalance := startingQuoteDepositSeller.TotalBalance.Add(balanceGathered)
							expectedSellerQuoteAvailableBalance := startingQuoteDepositSeller.AvailableBalance.Add(balanceGathered)

							Expect(depositSellerQuoteAsset.TotalBalance).To(Equal(expectedSellerQuoteTotalBalance))
							Expect(depositSellerQuoteAsset.AvailableBalance).To(Equal(expectedSellerQuoteAvailableBalance))
						})
					})
				})
			})

			Describe("with equal price and quantity and same subaccount", func() {
				limitQuantity := sdk.NewDec(1)
				limitPrice := sdk.NewDec(2000)
				trader := testexchange.SampleSubaccountAddr1
				var (
					startingQuoteDepositTrader *types.Deposit
					startingBaseDepositTrader  *types.Deposit
				)

				BeforeEach(func() {
					By("Constructing the limit buy order")
					limitBuyOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitPrice, limitQuantity, types.OrderType_BUY, trader)

					By("Depositing funds into subaccount of trader (buyer and seller in the same time)")
					startingQuoteDepositTrader = &types.Deposit{
						AvailableBalance: sdk.NewDec(10000),
						TotalBalance:     sdk.NewDec(10000),
					}
					testexchange.MintAndDeposit(app, ctx, trader.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, startingQuoteDepositTrader.AvailableBalance.TruncateInt())))

					startingBaseDepositTrader = &types.Deposit{
						AvailableBalance: sdk.NewDec(6),
						TotalBalance:     sdk.NewDec(6),
					}
					testexchange.MintAndDeposit(app, ctx, trader.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositTrader.AvailableBalance.TruncateInt())))

					By("Creating the spot limit buy order")
					msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsg)
				})

				Describe("in the same block", func() {
					JustBeforeEach(func() {
						By("Constructing the limit sell order")
						limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitPrice, limitQuantity, types.OrderType_SELL, trader)

						By("Creating the spot limit sell order")
						msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("is getting matched with a spot limit sell order", func() {
						It("has correct balances for trader", func() {
							depositTraderBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].BaseDenom)

							Expect(depositTraderBaseAsset.TotalBalance).To(Equal(startingBaseDepositTrader.TotalBalance))
							Expect(depositTraderBaseAsset.AvailableBalance).To(Equal(startingBaseDepositTrader.AvailableBalance))

							notional := limitPrice.Mul(limitQuantity)
							// When self matching, only fees are paid.
							balancePaid := notional.Mul(spotMarket.TakerFeeRate).Mul(sdk.NewDec(2))
							depositTraderQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].QuoteDenom)

							expectedTraderQuoteTotalBalance := startingQuoteDepositTrader.TotalBalance.Sub(balancePaid)
							expectedTraderQuoteAvailableBalance := startingQuoteDepositTrader.AvailableBalance.Sub(balancePaid)

							Expect(depositTraderQuoteAsset.TotalBalance).To(Equal(expectedTraderQuoteTotalBalance))
							Expect(depositTraderQuoteAsset.AvailableBalance).To(Equal(expectedTraderQuoteAvailableBalance))
						})
					})
				})

				Describe("in different blocks", func() {
					BeforeEach(func() {
						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					JustBeforeEach(func() {
						By("Constructing the limit sell order")
						limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitPrice, limitQuantity, types.OrderType_SELL, trader)

						By("Creating the spot limit sell order")
						msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("is getting matched with a spot limit sell order", func() {
						It("has correct balances for trader for base and quote asset", func() {
							depositTraderBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].BaseDenom)

							Expect(depositTraderBaseAsset.TotalBalance).To(Equal(startingBaseDepositTrader.TotalBalance))
							Expect(depositTraderBaseAsset.AvailableBalance).To(Equal(startingBaseDepositTrader.AvailableBalance))

							notional := limitPrice.Mul(limitQuantity)
							// When self matching, only fees are paid.
							balancePaidForBuyOrder := notional.Mul(spotMarket.MakerFeeRate)
							balancePaidForSellOrder := notional.Mul(spotMarket.TakerFeeRate)
							balancePaid := balancePaidForBuyOrder.Add(balancePaidForSellOrder)
							depositTraderQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].QuoteDenom)

							expectedTraderQuoteTotalBalance := startingQuoteDepositTrader.TotalBalance.Sub(balancePaid)
							expectedTraderQuoteAvailableBalance := startingQuoteDepositTrader.AvailableBalance.Sub(balancePaid)

							Expect(depositTraderQuoteAsset.TotalBalance).To(Equal(expectedTraderQuoteTotalBalance))
							Expect(depositTraderQuoteAsset.AvailableBalance).To(Equal(expectedTraderQuoteAvailableBalance))
						})
					})
				})
			})

			Describe("with different price, quantity and same subaccount", func() {
				limitBuyQuantity := sdk.NewDec(1)
				limitBuyPrice := sdk.NewDec(2000)
				limitSellQuantity := sdk.NewDec(3)
				limitSellPrice := sdk.NewDec(1900)
				clearingPriceSameBlock := sdk.NewDec(1950)
				clearingPriceDiffBlocks := sdk.NewDec(2000)
				trader := testexchange.SampleSubaccountAddr1
				var (
					startingQuoteDepositTrader *types.Deposit
					startingBaseDepositTrader  *types.Deposit
				)

				BeforeEach(func() {
					By("Constructing the limit buy order")
					limitBuyOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitBuyPrice, limitBuyQuantity, types.OrderType_BUY, trader)

					By("Depositing funds into subaccount of trader")
					startingQuoteDepositTrader = &types.Deposit{
						AvailableBalance: sdk.NewDec(10000),
						TotalBalance:     sdk.NewDec(10000),
					}

					testexchange.MintAndDeposit(app, ctx, trader.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, startingQuoteDepositTrader.AvailableBalance.TruncateInt())))

					startingBaseDepositTrader = &types.Deposit{
						AvailableBalance: sdk.NewDec(6),
						TotalBalance:     sdk.NewDec(6),
					}
					testexchange.MintAndDeposit(app, ctx, trader.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositTrader.AvailableBalance.TruncateInt())))

					By("Creating the spot limit buy order")
					msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsg)
				})

				Describe("in the same block", func() {
					JustBeforeEach(func() {
						By("Constructing the limit sell order")
						limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitSellPrice, limitSellQuantity, types.OrderType_SELL, trader)

						By("Creating the spot limit sell order")
						msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("is getting matched with a spot limit sell order", func() {
						It("has correct balances for trader", func() {
							depositTraderBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].BaseDenom)

							expectedTraderBaseAvailableBalance := startingBaseDepositTrader.AvailableBalance.Add(limitBuyQuantity).Sub(limitSellQuantity)

							Expect(depositTraderBaseAsset.TotalBalance).To(Equal(startingBaseDepositTrader.TotalBalance))
							Expect(depositTraderBaseAsset.AvailableBalance).To(Equal(expectedTraderBaseAvailableBalance))

							notional := clearingPriceSameBlock.Mul(limitBuyQuantity)
							// When self matching, only fees are paid.
							balancePaid := notional.Mul(spotMarket.TakerFeeRate).Mul(sdk.NewDec(2))
							depositTraderQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].QuoteDenom)

							expectedTraderQuoteTotalBalance := startingQuoteDepositTrader.TotalBalance.Sub(balancePaid)
							expectedTraderQuoteAvailableBalance := startingQuoteDepositTrader.AvailableBalance.Sub(balancePaid)

							Expect(depositTraderQuoteAsset.TotalBalance).To(Equal(expectedTraderQuoteTotalBalance))
							Expect(depositTraderQuoteAsset.AvailableBalance).To(Equal(expectedTraderQuoteAvailableBalance))
						})
					})
				})

				Describe("in different blocks", func() {
					BeforeEach(func() {
						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					JustBeforeEach(func() {
						By("Constructing the limit sell order from different subaccount")
						limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(limitSellPrice, limitSellQuantity, types.OrderType_SELL, trader)

						By("Creating the spot limit sell order")
						msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("is getting matched with a spot limit sell order", func() {
						It("has correct order states for buyer", func() {
							limitOrders, clearingPrice, clearingQuantity := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(
								ctx,
								testInput.Spots[0].MarketID,
								true,
								sdk.NewDec(10),
							)

							Expect(len(limitOrders)).To(Equal(0))
							Expect(clearingPrice).To(Equal(sdk.Dec{}))
							Expect(clearingQuantity).To(Equal(sdk.ZeroDec()))
						})

						It("has correct order states for seller", func() {
							limitOrders, clearingPrice, clearingQuantity := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(
								ctx,
								testInput.Spots[0].MarketID,
								false,
								sdk.NewDec(10),
							)

							expectedClearingQuantity := limitSellQuantity.Sub(limitBuyQuantity)

							Expect(len(limitOrders)).To(Equal(1))
							Expect(clearingPrice).To(Equal(clearingPrice))
							Expect(clearingQuantity).To(Equal(expectedClearingQuantity))
						})

						It("has correct balances for trader", func() {
							depositTraderBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].BaseDenom)
							expectedTraderBaseAvailableBalance := startingBaseDepositTrader.AvailableBalance.Add(limitBuyQuantity).Sub(limitSellQuantity)

							Expect(depositTraderBaseAsset.TotalBalance).To(Equal(startingBaseDepositTrader.TotalBalance))
							Expect(depositTraderBaseAsset.AvailableBalance).To(Equal(expectedTraderBaseAvailableBalance))

							notional := clearingPriceDiffBlocks.Mul(limitBuyQuantity)
							// When self matching, only fees are paid.
							balancePaidForBuyOrder := notional.Mul(spotMarket.MakerFeeRate)
							balancePaidForSellOrder := notional.Mul(spotMarket.TakerFeeRate)
							balancePaid := balancePaidForBuyOrder.Add(balancePaidForSellOrder)
							depositTraderQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, trader, testInput.Spots[0].QuoteDenom)

							expectedTraderQuoteTotalBalance := startingQuoteDepositTrader.TotalBalance.Sub(balancePaid)
							expectedTraderQuoteAvailableBalance := startingQuoteDepositTrader.AvailableBalance.Sub(balancePaid)

							Expect(depositTraderQuoteAsset.TotalBalance.String()).To(Equal(expectedTraderQuoteTotalBalance.String()))
							Expect(depositTraderQuoteAsset.AvailableBalance.String()).To(Equal(expectedTraderQuoteAvailableBalance.String()))
						})
					})
				})
			})
		})

		Describe("when multiple spot limit orders", func() {
			var (
				startingQuoteDepositBuyers  *types.Deposit
				startingQuoteDepositSellers *types.Deposit
				startingBaseDepositBuyers   *types.Deposit
				startingBaseDepositSellers  *types.Deposit
				limitBuyOrderMsgs           [3]*types.MsgCreateSpotLimitOrder
				limitSellOrderMsgs          [3]*types.MsgCreateSpotLimitOrder
				buyers                      []common.Hash
				sellers                     []common.Hash
			)

			Describe("with a variety of prices, quantities and subaccounts", func() {
				// Orders that will result in clearing price of 1950.
				limitBuyQuantities := []sdk.Dec{sdk.NewDec(2), sdk.NewDec(3), sdk.NewDec(2)}
				limitBuyPrices := []sdk.Dec{sdk.NewDec(1967), sdk.NewDec(1985), sdk.NewDec(2000)}
				limitSellQuantities := []sdk.Dec{sdk.NewDec(2), sdk.NewDec(1), sdk.NewDec(3)}
				limitSellPrices := []sdk.Dec{sdk.NewDec(1930), sdk.NewDec(1918), sdk.NewDec(1900)}

				expectedBuyFilledQuantities := []sdk.Dec{sdk.NewDec(1), sdk.NewDec(3), sdk.NewDec(2)}

				clearingPriceSameBlock := sdk.MustNewDecFromStr("1948.5")
				clearingPriceDiffBlocks := sdk.NewDec(1967)

				// All different buyers and sellers.
				buyers = []common.Hash{
					testexchange.SampleSubaccountAddr1,
					testexchange.SampleSubaccountAddr2,
					testexchange.SampleSubaccountAddr3,
				}

				sellers = []common.Hash{
					testexchange.SampleSubaccountAddr4,
					testexchange.SampleSubaccountAddr5,
					testexchange.SampleSubaccountAddr6,
				}

				BeforeEach(func() {
					By("Constructing the limit buy orders")
					for i := range buyers {
						limitBuyOrderMsgs[i] = testInput.NewMsgCreateSpotLimitOrder(limitBuyPrices[i], limitBuyQuantities[i], types.OrderType_BUY, buyers[i])
					}

					By("Depositing funds into subaccounts of buyers")
					startingQuoteDepositBuyers = &types.Deposit{
						AvailableBalance: sdk.NewDec(10000),
						TotalBalance:     sdk.NewDec(10000),
					}

					startingBaseDepositBuyers = &types.Deposit{
						AvailableBalance: sdk.ZeroDec(),
						TotalBalance:     sdk.ZeroDec(),
					}

					for i := range buyers {
						testexchange.MintAndDeposit(app, ctx, buyers[i].String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, startingQuoteDepositBuyers.AvailableBalance.TruncateInt())))
					}

					By("Creating the spot limit buy orders")
					for i := range limitBuyOrderMsgs {
						msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsgs[i])
					}
				})

				Describe("in the same block", func() {
					JustBeforeEach(func() {
						By("Constructing the limit sell orders")
						for i := range sellers {
							limitSellOrderMsgs[i] = testInput.NewMsgCreateSpotLimitOrder(limitSellPrices[i], limitSellQuantities[i], types.OrderType_SELL, sellers[i])
						}

						By("Depositing funds into subaccount of seller")
						startingBaseDepositSellers = &types.Deposit{
							AvailableBalance: sdk.NewDec(6),
							TotalBalance:     sdk.NewDec(6),
						}

						startingQuoteDepositSellers = &types.Deposit{
							AvailableBalance: sdk.ZeroDec(),
							TotalBalance:     sdk.ZeroDec(),
						}

						for i := range sellers {
							testexchange.MintAndDeposit(app, ctx, sellers[i].String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositSellers.AvailableBalance.TruncateInt())))
						}

						By("Creating the spot limit sell order")
						for i := range limitSellOrderMsgs {
							msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsgs[i])
						}

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("are getting matched with multiple spot limit sell orders", func() {
						It("has correct balances for buyers for base and quote asset", func() {
							for i := range buyers {
								depositBuyerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].BaseDenom)

								expectedBuyerBaseTotalBalance := startingBaseDepositBuyers.TotalBalance.Add(expectedBuyFilledQuantities[i])
								expectedBuyerBaseAvailableBalance := startingBaseDepositBuyers.AvailableBalance.Add(expectedBuyFilledQuantities[i])

								Expect(depositBuyerBaseAsset.TotalBalance.String()).Should(Equal(expectedBuyerBaseTotalBalance.String()))
								Expect(depositBuyerBaseAsset.AvailableBalance.String()).Should(Equal(expectedBuyerBaseAvailableBalance.String()))

								notionalFilled := clearingPriceSameBlock.Mul(expectedBuyFilledQuantities[i])

								balancePaidFilled := notionalFilled.Add(notionalFilled.Mul(spotMarket.TakerFeeRate))
								unfilledNotional := limitBuyPrices[i].Mul(limitBuyQuantities[i].Sub(expectedBuyFilledQuantities[i]))
								balancePaidUnfilled := unfilledNotional.Add(unfilledNotional.Mul(spotMarket.MakerFeeRate))
								balancePaidAvailable := balancePaidFilled.Add(balancePaidUnfilled)

								depositBuyerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].QuoteDenom)

								expectedBuyerQuoteTotalBalance := startingQuoteDepositBuyers.TotalBalance.Sub(balancePaidFilled)
								expectedBuyerQuoteAvailableBalance := startingQuoteDepositBuyers.AvailableBalance.Sub(balancePaidAvailable)

								Expect(depositBuyerQuoteAsset.TotalBalance.String()).Should(Equal(expectedBuyerQuoteTotalBalance.String()))
								Expect(depositBuyerQuoteAsset.AvailableBalance.String()).Should(Equal(expectedBuyerQuoteAvailableBalance.String()))
							}
						})

						It("has correct balances for sellers for base and quote asset", func() {
							for i := range sellers {
								depositSellerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].BaseDenom)

								expectedSellerBaseTotalBalance := startingBaseDepositSellers.TotalBalance.Sub(limitSellQuantities[i])
								expectedSellerBaseAvailableBalance := startingBaseDepositSellers.AvailableBalance.Sub(limitSellQuantities[i])

								Expect(depositSellerBaseAsset.TotalBalance.String()).Should(Equal(expectedSellerBaseTotalBalance.String()))
								Expect(depositSellerBaseAsset.AvailableBalance.String()).Should(Equal(expectedSellerBaseAvailableBalance.String()))

								notional := clearingPriceSameBlock.Mul(limitSellQuantities[i])
								balanceGathered := notional.Sub(notional.Mul(spotMarket.TakerFeeRate))
								depositSellerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].QuoteDenom)

								expectedSellerQuoteTotalBalance := startingQuoteDepositSellers.TotalBalance.Add(balanceGathered)
								expectedSellerQuoteAvailableBalance := startingQuoteDepositSellers.AvailableBalance.Add(balanceGathered)

								Expect(depositSellerQuoteAsset.TotalBalance).To(Equal(expectedSellerQuoteTotalBalance))
								Expect(depositSellerQuoteAsset.AvailableBalance).To(Equal(expectedSellerQuoteAvailableBalance))
							}
						})
					})
				})

				Describe("in different block for buys and sells", func() {
					BeforeEach(func() {
						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					JustBeforeEach(func() {
						By("Constructing the limit sell orders")
						for i := range sellers {
							limitSellOrderMsgs[i] = testInput.NewMsgCreateSpotLimitOrder(limitSellPrices[i], limitSellQuantities[i], types.OrderType_SELL, sellers[i])
						}

						By("Depositing funds into subaccount of seller")
						startingBaseDepositSellers = &types.Deposit{
							AvailableBalance: sdk.NewDec(6),
							TotalBalance:     sdk.NewDec(6),
						}

						startingQuoteDepositSellers = &types.Deposit{
							AvailableBalance: sdk.ZeroDec(),
							TotalBalance:     sdk.ZeroDec(),
						}

						for i := range sellers {
							testexchange.MintAndDeposit(app, ctx, sellers[i].String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositSellers.AvailableBalance.TruncateInt())))
						}

						By("Creating the spot limit sell order")
						for i := range limitSellOrderMsgs {
							msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsgs[i])
						}

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("are getting matched with multiple spot limit sell orders", func() {
						It("has correct balances for buyers for base and quote asset", func() {
							for i := range buyers {
								depositBuyerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].BaseDenom)

								expectedBuyerBaseTotalBalance := startingBaseDepositBuyers.TotalBalance.Add(expectedBuyFilledQuantities[i])
								expectedBuyerBaseAvailableBalance := startingBaseDepositBuyers.AvailableBalance.Add(expectedBuyFilledQuantities[i])

								Expect(depositBuyerBaseAsset.TotalBalance.String()).Should(Equal(expectedBuyerBaseTotalBalance.String()))
								Expect(depositBuyerBaseAsset.AvailableBalance.String()).Should(Equal(expectedBuyerBaseAvailableBalance.String()))

								notionalFilled := clearingPriceDiffBlocks.Mul(expectedBuyFilledQuantities[i])

								balancePaidFilled := notionalFilled.Add(notionalFilled.Mul(spotMarket.MakerFeeRate))
								unfilledNotional := limitBuyPrices[i].Mul(limitBuyQuantities[i].Sub(expectedBuyFilledQuantities[i]))
								balancePaidUnfilled := unfilledNotional.Add(unfilledNotional.Mul(spotMarket.MakerFeeRate))
								balancePaidAvailable := balancePaidFilled.Add(balancePaidUnfilled)

								depositBuyerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].QuoteDenom)

								expectedBuyerQuoteTotalBalance := startingQuoteDepositBuyers.TotalBalance.Sub(balancePaidFilled)
								expectedBuyerQuoteAvailableBalance := startingQuoteDepositBuyers.AvailableBalance.Sub(balancePaidAvailable)

								Expect(depositBuyerQuoteAsset.TotalBalance.String()).Should(Equal(expectedBuyerQuoteTotalBalance.String()))
								Expect(depositBuyerQuoteAsset.AvailableBalance.String()).Should(Equal(expectedBuyerQuoteAvailableBalance.String()))
							}
						})

						It("has correct balances for sellers for base and quote asset", func() {
							for i := range sellers {
								depositSellerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].BaseDenom)

								expectedSellerBaseTotalBalance := startingBaseDepositSellers.TotalBalance.Sub(limitSellQuantities[i])
								expectedSellerBaseAvailableBalance := startingBaseDepositSellers.AvailableBalance.Sub(limitSellQuantities[i])

								Expect(depositSellerBaseAsset.TotalBalance).To(Equal(expectedSellerBaseTotalBalance))
								Expect(depositSellerBaseAsset.AvailableBalance).To(Equal(expectedSellerBaseAvailableBalance))

								notional := clearingPriceDiffBlocks.Mul(limitSellQuantities[i])
								balanceGathered := notional.Sub(notional.Mul(spotMarket.TakerFeeRate))
								depositSellerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].QuoteDenom)

								expectedSellerQuoteTotalBalance := startingQuoteDepositSellers.TotalBalance.Add(balanceGathered)
								expectedSellerQuoteAvailableBalance := startingQuoteDepositSellers.AvailableBalance.Add(balanceGathered)

								Expect(depositSellerQuoteAsset.TotalBalance).To(Equal(expectedSellerQuoteTotalBalance))
								Expect(depositSellerQuoteAsset.AvailableBalance).To(Equal(expectedSellerQuoteAvailableBalance))
							}
						})
					})
				})
			})
		})

		Describe("when multiple spot limit orders", func() {
			var (
				startingQuoteDepositBuyers  *types.Deposit
				startingQuoteDepositSellers *types.Deposit
				startingBaseDepositBuyers   *types.Deposit
				startingBaseDepositSellers  *types.Deposit
				limitBuyOrderMsgs           [3]*types.MsgCreateSpotLimitOrder
				limitSellOrderMsgs          [3]*types.MsgCreateSpotLimitOrder
				buyers                      []common.Hash
				sellers                     []common.Hash
			)

			Describe("with a variety of prices, quantities and subaccounts", func() {
				// Orders that will result in clearing price of 1950.
				limitBuyQuantities := []sdk.Dec{sdk.NewDec(2), sdk.NewDec(3), sdk.NewDec(2)}
				limitBuyPrices := []sdk.Dec{sdk.NewDec(1967), sdk.NewDec(1985), sdk.NewDec(2000)}
				limitSellQuantities := []sdk.Dec{sdk.NewDec(2), sdk.NewDec(1), sdk.NewDec(3)}
				limitSellPrices := []sdk.Dec{sdk.NewDec(1930), sdk.NewDec(1918), sdk.NewDec(1900)}

				expectedBuyFilledQuantities := []sdk.Dec{sdk.NewDec(1), sdk.NewDec(3), sdk.NewDec(2)}

				clearingPriceSameBlock := sdk.MustNewDecFromStr("1948.5")
				clearingPriceDiffBlocks := sdk.NewDec(1967)

				// All different buyers and sellers.
				buyers = []common.Hash{
					testexchange.SampleSubaccountAddr1,
					testexchange.SampleSubaccountAddr2,
					testexchange.SampleSubaccountAddr3,
				}

				sellers = []common.Hash{
					testexchange.SampleSubaccountAddr4,
					testexchange.SampleSubaccountAddr5,
					testexchange.SampleSubaccountAddr6,
				}

				BeforeEach(func() {
					By("Constructing the limit buy orders")
					for i := range buyers {
						limitBuyOrderMsgs[i] = testInput.NewMsgCreateSpotLimitOrder(limitBuyPrices[i], limitBuyQuantities[i], types.OrderType_BUY, buyers[i])
					}

					By("Depositing funds into subaccounts of buyers")
					startingQuoteDepositBuyers = &types.Deposit{
						AvailableBalance: sdk.NewDec(10000),
						TotalBalance:     sdk.NewDec(10000),
					}

					startingBaseDepositBuyers = &types.Deposit{
						AvailableBalance: sdk.ZeroDec(),
						TotalBalance:     sdk.ZeroDec(),
					}

					for i := range buyers {
						testexchange.MintAndDeposit(app, ctx, buyers[i].String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, startingQuoteDepositBuyers.AvailableBalance.TruncateInt())))
					}

					By("Creating the spot limit buy orders")
					for i := range limitBuyOrderMsgs {
						msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsgs[i])
					}
				})

				Describe("in the same block", func() {
					JustBeforeEach(func() {
						By("Constructing the limit sell orders")
						for i := range sellers {
							limitSellOrderMsgs[i] = testInput.NewMsgCreateSpotLimitOrder(limitSellPrices[i], limitSellQuantities[i], types.OrderType_SELL, sellers[i])
						}

						By("Depositing funds into subaccount of seller")
						startingBaseDepositSellers = &types.Deposit{
							AvailableBalance: sdk.NewDec(6),
							TotalBalance:     sdk.NewDec(6),
						}

						startingQuoteDepositSellers = &types.Deposit{
							AvailableBalance: sdk.ZeroDec(),
							TotalBalance:     sdk.ZeroDec(),
						}

						for i := range sellers {
							testexchange.MintAndDeposit(app, ctx, sellers[i].String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositSellers.AvailableBalance.TruncateInt())))
						}

						By("Creating the spot limit sell order")
						for i := range limitSellOrderMsgs {
							msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsgs[i])
						}

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("are getting matched with multiple spot limit sell orders", func() {
						It("has correct balances for buyers for base and quote asset", func() {
							for i := range buyers {
								depositBuyerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].BaseDenom)

								expectedBuyerBaseTotalBalance := startingBaseDepositBuyers.TotalBalance.Add(expectedBuyFilledQuantities[i])
								expectedBuyerBaseAvailableBalance := startingBaseDepositBuyers.AvailableBalance.Add(expectedBuyFilledQuantities[i])

								Expect(depositBuyerBaseAsset.TotalBalance.String()).Should(Equal(expectedBuyerBaseTotalBalance.String()))
								Expect(depositBuyerBaseAsset.AvailableBalance.String()).Should(Equal(expectedBuyerBaseAvailableBalance.String()))

								notionalFilled := clearingPriceSameBlock.Mul(expectedBuyFilledQuantities[i])

								balancePaidFilled := notionalFilled.Add(notionalFilled.Mul(spotMarket.TakerFeeRate))
								unfilledNotional := limitBuyPrices[i].Mul(limitBuyQuantities[i].Sub(expectedBuyFilledQuantities[i]))
								balancePaidUnfilled := unfilledNotional.Add(unfilledNotional.Mul(spotMarket.MakerFeeRate))
								balancePaidAvailable := balancePaidFilled.Add(balancePaidUnfilled)

								depositBuyerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].QuoteDenom)

								expectedBuyerQuoteTotalBalance := startingQuoteDepositBuyers.TotalBalance.Sub(balancePaidFilled)
								expectedBuyerQuoteAvailableBalance := startingQuoteDepositBuyers.AvailableBalance.Sub(balancePaidAvailable)

								Expect(depositBuyerQuoteAsset.TotalBalance.String()).Should(Equal(expectedBuyerQuoteTotalBalance.String()))
								Expect(depositBuyerQuoteAsset.AvailableBalance.String()).Should(Equal(expectedBuyerQuoteAvailableBalance.String()))
							}
						})

						It("has correct balances for sellers for base and quote asset", func() {
							for i := range sellers {
								depositSellerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].BaseDenom)

								expectedSellerBaseTotalBalance := startingBaseDepositSellers.TotalBalance.Sub(limitSellQuantities[i])
								expectedSellerBaseAvailableBalance := startingBaseDepositSellers.AvailableBalance.Sub(limitSellQuantities[i])

								Expect(depositSellerBaseAsset.TotalBalance.String()).Should(Equal(expectedSellerBaseTotalBalance.String()))
								Expect(depositSellerBaseAsset.AvailableBalance.String()).Should(Equal(expectedSellerBaseAvailableBalance.String()))

								notional := clearingPriceSameBlock.Mul(limitSellQuantities[i])
								balanceGathered := notional.Sub(notional.Mul(spotMarket.TakerFeeRate))
								depositSellerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].QuoteDenom)

								expectedSellerQuoteTotalBalance := startingQuoteDepositSellers.TotalBalance.Add(balanceGathered)
								expectedSellerQuoteAvailableBalance := startingQuoteDepositSellers.AvailableBalance.Add(balanceGathered)

								Expect(depositSellerQuoteAsset.TotalBalance).To(Equal(expectedSellerQuoteTotalBalance))
								Expect(depositSellerQuoteAsset.AvailableBalance).To(Equal(expectedSellerQuoteAvailableBalance))
							}
						})
					})
				})

				Describe("in different block for buys and sells", func() {
					BeforeEach(func() {
						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					JustBeforeEach(func() {
						By("Constructing the limit sell orders")
						for i := range sellers {
							limitSellOrderMsgs[i] = testInput.NewMsgCreateSpotLimitOrder(limitSellPrices[i], limitSellQuantities[i], types.OrderType_SELL, sellers[i])
						}

						By("Depositing funds into subaccount of seller")
						startingBaseDepositSellers = &types.Deposit{
							AvailableBalance: sdk.NewDec(6),
							TotalBalance:     sdk.NewDec(6),
						}

						startingQuoteDepositSellers = &types.Deposit{
							AvailableBalance: sdk.ZeroDec(),
							TotalBalance:     sdk.ZeroDec(),
						}

						for i := range sellers {
							testexchange.MintAndDeposit(app, ctx, sellers[i].String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, startingBaseDepositSellers.AvailableBalance.TruncateInt())))
						}

						By("Creating the spot limit sell order")
						for i := range limitSellOrderMsgs {
							msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsgs[i])
						}

						By("Calling the end blocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					Context("are getting matched with multiple spot limit sell orders", func() {
						It("has correct balances for buyers for base and quote asset", func() {
							for i := range buyers {
								depositBuyerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].BaseDenom)

								expectedBuyerBaseTotalBalance := startingBaseDepositBuyers.TotalBalance.Add(expectedBuyFilledQuantities[i])
								expectedBuyerBaseAvailableBalance := startingBaseDepositBuyers.AvailableBalance.Add(expectedBuyFilledQuantities[i])

								Expect(depositBuyerBaseAsset.TotalBalance.String()).Should(Equal(expectedBuyerBaseTotalBalance.String()))
								Expect(depositBuyerBaseAsset.AvailableBalance.String()).Should(Equal(expectedBuyerBaseAvailableBalance.String()))

								notionalFilled := clearingPriceDiffBlocks.Mul(expectedBuyFilledQuantities[i])

								balancePaidFilled := notionalFilled.Add(notionalFilled.Mul(spotMarket.MakerFeeRate))
								unfilledNotional := limitBuyPrices[i].Mul(limitBuyQuantities[i].Sub(expectedBuyFilledQuantities[i]))
								balancePaidUnfilled := unfilledNotional.Add(unfilledNotional.Mul(spotMarket.MakerFeeRate))
								balancePaidAvailable := balancePaidFilled.Add(balancePaidUnfilled)

								depositBuyerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, buyers[i], testInput.Spots[0].QuoteDenom)

								expectedBuyerQuoteTotalBalance := startingQuoteDepositBuyers.TotalBalance.Sub(balancePaidFilled)
								expectedBuyerQuoteAvailableBalance := startingQuoteDepositBuyers.AvailableBalance.Sub(balancePaidAvailable)

								Expect(depositBuyerQuoteAsset.TotalBalance.String()).Should(Equal(expectedBuyerQuoteTotalBalance.String()))
								Expect(depositBuyerQuoteAsset.AvailableBalance.String()).Should(Equal(expectedBuyerQuoteAvailableBalance.String()))
							}
						})

						It("has correct balances for sellers for base and quote asset", func() {
							for i := range sellers {
								depositSellerBaseAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].BaseDenom)

								expectedSellerBaseTotalBalance := startingBaseDepositSellers.TotalBalance.Sub(limitSellQuantities[i])
								expectedSellerBaseAvailableBalance := startingBaseDepositSellers.AvailableBalance.Sub(limitSellQuantities[i])

								Expect(depositSellerBaseAsset.TotalBalance).To(Equal(expectedSellerBaseTotalBalance))
								Expect(depositSellerBaseAsset.AvailableBalance).To(Equal(expectedSellerBaseAvailableBalance))

								notional := clearingPriceDiffBlocks.Mul(limitSellQuantities[i])
								balanceGathered := notional.Sub(notional.Mul(spotMarket.TakerFeeRate))
								depositSellerQuoteAsset := testexchange.GetBankAndDepositFunds(app, ctx, sellers[i], testInput.Spots[0].QuoteDenom)

								expectedSellerQuoteTotalBalance := startingQuoteDepositSellers.TotalBalance.Add(balanceGathered)
								expectedSellerQuoteAvailableBalance := startingQuoteDepositSellers.AvailableBalance.Add(balanceGathered)

								Expect(depositSellerQuoteAsset.TotalBalance).To(Equal(expectedSellerQuoteTotalBalance))
								Expect(depositSellerQuoteAsset.AvailableBalance).To(Equal(expectedSellerQuoteAvailableBalance))
							}
						})
					})
				})
			})
		})
	})
})
