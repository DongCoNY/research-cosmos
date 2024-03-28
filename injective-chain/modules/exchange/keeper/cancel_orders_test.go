package keeper_test

import (
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

// TODO test EventCancelDerivativeOrder

var _ = Describe("Order Canceling Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		spotMarket *types.SpotMarket
		msgServer  types.MsgServer
		err        error
		buyer      = testexchange.SampleSubaccountAddr1
		seller     = testexchange.SampleSubaccountAddr2
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		msgServer = exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)
		msgServer = exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)

		_, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)

		Expect(err).To(BeNil())
		Expect(common.HexToHash(spotMarket.MarketId)).To(BeEquivalentTo(testInput.Spots[0].MarketID))
	})

	Describe("Succeeds", func() {
		Describe("when spot limit buy order", func() {
			var (
				quoteDeposit     *types.Deposit
				limitBuyOrderMsg *types.MsgCreateSpotLimitOrder
			)

			BeforeEach(func() {
				By("Constructing the limit order")
				price, quantity, orderType := sdk.NewDec(2000), sdk.NewDec(1), types.OrderType_BUY
				limitBuyOrderMsg = testInput.NewMsgCreateSpotLimitOrder(price, quantity, orderType, buyer)

				By("Depositing funds into subaccount")
				quoteDeposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(2015),
					TotalBalance:     sdk.NewDec(2015),
				}
				testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))

				By("Creating the spot limit order")
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsg)
				Expect(err).To(BeNil())

				By("Calling the end blocker")
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			})

			JustBeforeEach(func() {
				By("Canceling the spot order")
				limitOrders, _, _ := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(
					ctx,
					testInput.Spots[0].MarketID,
					true,
					sdk.NewDec(10),
				)

				restingOrder := limitOrders[0]
				_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), &types.MsgCancelSpotOrder{
					Sender:       limitBuyOrderMsg.Sender,
					MarketId:     testInput.Spots[0].MarketID.Hex(),
					SubaccountId: restingOrder.OrderInfo.SubaccountId,
					OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
				})

				Expect(err).To(BeNil())
			})

			Context("is getting canceled before filled", func() {
				It("Checking if order was deleted from the orderbook", func() {
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

				It("Checking if balances have been updated successfully", func() {
					depositPostCancel := testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].QuoteDenom)
					Expect(depositPostCancel.AvailableBalance).To(Equal(depositPostCancel.TotalBalance))
					Expect(depositPostCancel.TotalBalance).To(Equal(quoteDeposit.TotalBalance))
					Expect(depositPostCancel.AvailableBalance).To(Equal(quoteDeposit.AvailableBalance))
				})
			})
		})

		Describe("when spot limit sell order", func() {
			var (
				baseDeposit       *types.Deposit
				limitSellOrderMsg *types.MsgCreateSpotLimitOrder
			)

			BeforeEach(func() {
				By("Constructing the limit order")
				price, quantity, orderType := sdk.NewDec(2000), sdk.NewDec(1), types.OrderType_SELL
				limitSellOrderMsg = testInput.NewMsgCreateSpotLimitOrder(price, quantity, orderType, seller)

				By("Depositing funds into subaccount")

				baseDeposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(2),
					TotalBalance:     sdk.NewDec(2),
				}
				testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, baseDeposit.AvailableBalance.TruncateInt())))

				By("Creating the spot limit order")
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitSellOrderMsg)
				Expect(err).To(BeNil())

				By("Calling the end blocker")
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			})

			JustBeforeEach(func() {
				By("Canceling the spot order")
				limitOrders, _, _ := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(
					ctx,
					testInput.Spots[0].MarketID,
					false,
					sdk.NewDec(10),
				)

				restingOrder := limitOrders[0]
				_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), &types.MsgCancelSpotOrder{
					Sender:       limitSellOrderMsg.Sender,
					MarketId:     testInput.Spots[0].MarketID.Hex(),
					SubaccountId: restingOrder.OrderInfo.SubaccountId,
					OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
				})

				Expect(err).To(BeNil())
			})

			Context("is getting canceled before filled", func() {
				It("Checking if order was deleted from the orderbook", func() {
					limitOrders, clearingPrice, clearingQuantity := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(
						ctx,
						testInput.Spots[0].MarketID,
						false,
						sdk.NewDec(10),
					)
					Expect(limitOrders).Should(BeEmpty())
					Expect(clearingPrice).To(Equal(sdk.Dec{}))
					Expect(clearingQuantity).To(Equal(sdk.ZeroDec()))
				})

				It("Checking if balances have been updated successfully", func() {
					depositPostCancel := testexchange.GetBankAndDepositFunds(app, ctx, seller, testInput.Spots[0].BaseDenom)
					Expect(depositPostCancel.AvailableBalance).To(Equal(depositPostCancel.TotalBalance))
					Expect(depositPostCancel.TotalBalance).To(Equal(baseDeposit.TotalBalance))
					Expect(depositPostCancel.AvailableBalance).To(Equal(baseDeposit.AvailableBalance))
				})
			})
		})
	})
})
