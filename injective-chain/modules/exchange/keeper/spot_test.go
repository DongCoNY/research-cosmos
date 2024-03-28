package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Spot Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		spotMarket *types.SpotMarket
		msgServer  types.MsgServer
		err        error
		buyer      = testexchange.SampleSubaccountAddr1
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		_, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)

		testexchange.OrFail(err)
		Expect(common.HexToHash(spotMarket.MarketId)).To(BeEquivalentTo(testInput.Spots[0].MarketID))
	})

	Describe("Success", func() {
		Describe("When a keys is added to transient store", func() {
			It("it should be flushed after commit", func() {
				tKey := app.ExchangeKeeper.GetTransientStoreKey()
				store := ctx.TransientStore(tKey)

				// set to transient store
				country := "serbia"
				currency := "rsd"
				key := []byte(country)
				value := []byte(currency)
				store.Set(key, value)

				// key should be present in transient store
				valueFromStore := store.Get(key)
				Expect(valueFromStore).Should(BeEquivalentTo(value))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				// key should not be present in transient store after Commit
				keyExist := store.Has(key)
				Expect(keyExist).To(BeFalse())
			})
		})
	})

	Describe("spot limit orders creation", func() {
		var (
			depositBefore    *types.Deposit
			depositAfter     *types.Deposit
			limitBuyOrderMsg *types.MsgCreateSpotLimitOrder
		)

		Describe("when creating a spot limit buy order", func() {
			BeforeEach(func() {
				By("Constructing the limit order")
				price, quantity, orderType := sdk.NewDec(2000), sdk.NewDec(1), types.OrderType_BUY
				limitBuyOrderMsg = testInput.NewMsgCreateSpotLimitOrder(price, quantity, orderType, buyer)

				By("Depositing funds into subaccount")

				quoteDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(20040),
					TotalBalance:     sdk.NewDec(20040),
				}

				baseDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(20),
					TotalBalance:     sdk.NewDec(20),
				}
				testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, math.Int(quoteDeposit.AvailableBalance.TruncateInt())), sdk.NewCoin(testInput.Spots[0].BaseDenom, math.Int(baseDeposit.AvailableBalance.TruncateInt()))))
				depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].QuoteDenom)
			})

			JustBeforeEach(func() {
				By("Creating a buy spot limit order")
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitBuyOrderMsg)
				depositAfter = testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].QuoteDenom)
			})

			Describe("when trader sends a spot limit buy order", func() {
				It("places the limit buy order successfully", func() {
					// amountDecremented := (Quantity * Price) * (1 + TakerFeeRate)
					order := limitBuyOrderMsg.Order
					amountDecremented := order.OrderInfo.Quantity.Mul(order.OrderInfo.Price).Mul(spotMarket.TakerFeeRate.Add(sdk.NewDec(1)))

					Expect(depositAfter.TotalBalance).Should(Equal(depositBefore.TotalBalance))
					Expect(depositAfter.AvailableBalance).Should(Equal(depositBefore.AvailableBalance.Sub(amountDecremented)))
				})

				Context("when EndBlocker is executed", func() {
					JustBeforeEach(func() {
						By("Running the Endblocker")
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
					})

					It("adds order to existing orderbook", func() {
						limitOrders, clearingPrice, clearingQuantity := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(ctx, testInput.Spots[0].MarketID, true, sdk.NewDec(10))

						order := limitBuyOrderMsg.Order

						// Check if order was added to resting orderbook
						Expect(len(limitOrders)).Should(Equal(1))
						Expect(limitOrders[0].OrderInfo).Should(Equal(order.OrderInfo))
						Expect(clearingPrice).Should(Equal(order.OrderInfo.Price))
						Expect(clearingQuantity).Should(Equal(order.OrderInfo.Quantity))

						depositFinal := testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.Spots[0].QuoteDenom)

						Expect(depositFinal.TotalBalance.Equal(depositAfter.TotalBalance)).To(BeTrue())
						// unmatched fee refund check: Fee Refund = Quantity * Price * (TakerFeeRate - MakerFeeRate)
						Expect(depositFinal.AvailableBalance).To(Equal(depositAfter.AvailableBalance.Add(order.OrderInfo.Quantity.Mul(order.OrderInfo.Price).Mul(spotMarket.TakerFeeRate.Sub(spotMarket.MakerFeeRate)))))
						// redundant unmatched fee refund check based on total balance
						Expect(depositFinal.AvailableBalance).To(Equal(depositFinal.TotalBalance.Sub(order.OrderInfo.Quantity.Mul(order.OrderInfo.Price).Mul(sdk.OneDec().Add(spotMarket.MakerFeeRate)))))
					})
				})

				// TODO test limitSellOrderMsg creation
			})
		})
	})
})
