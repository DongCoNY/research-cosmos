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

var _ = Describe("Spot Orders Keeper Unit Test", func() {
	var (
		testInput    testexchange.TestInput
		app          *simapp.InjectiveApp
		ctx          sdk.Context
		msgServer    types.MsgServer
		market       *types.SpotMarket
		subaccountID common.Hash
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)

		var err error
		market, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		testexchange.OrFail(err)

		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
		subaccountID = testexchange.SampleSubaccountAddr1

		deposit := &types.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
	})

	Describe("When there is not enough remaining buy liquidity", func() {
		BeforeEach(func() {
			msgs := testInput.NewListOfMsgCreateSpotLimitOrderForMarketIndex(0,
				// buy orders
				testexchange.NewBareSpotLimitOrderFromString("100.1", "1.5", types.OrderType_BUY_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("300.1", "3.5", types.OrderType_BUY_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("200.1", "100.531", types.OrderType_BUY_PO, subaccountID),
			)

			for _, msg := range msgs {
				testexchange.ReturnOrFail(msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), msg))
			}
		})

		It("rejects only the market order buys", func() {
			marketOrderMsgBuy := testInput.NewMsgCreateSpotMarketOrder(sdk.NewDec(1), sdk.NewDec(1000), types.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)
			Expect(errMarketBuy).To(Equal(types.ErrNoLiquidity))

			marketOrderMsgSell := testInput.NewMsgCreateSpotMarketOrder(sdk.NewDec(1), sdk.NewDec(1), types.OrderType_SELL, subaccountID)
			_, errMarketSell := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)
			Expect(errMarketSell).To(BeNil())
		})
	})

	Describe("When there is not enough remaining buy liquidity", func() {
		BeforeEach(func() {
			msgs := testInput.NewListOfMsgCreateSpotLimitOrderForMarketIndex(0,
				// sell orders
				testexchange.NewBareSpotLimitOrderFromString("100.1", "1.5", types.OrderType_SELL_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("300.1", "3.5", types.OrderType_SELL_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("200.1", "100.531", types.OrderType_SELL_PO, subaccountID),
			)

			for _, msg := range msgs {
				testexchange.ReturnOrFail(msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), msg))
			}
		})

		It("rejects only the market order buys", func() {
			marketOrderMsgBuy := testInput.NewMsgCreateSpotMarketOrder(sdk.NewDec(1), sdk.NewDec(1000), types.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)
			Expect(errMarketBuy).To(BeNil())
		})
	})

	Describe("For a spot limit buy orderbook", func() {
		It("should be in sorted order", func() {
			subaccountID := testexchange.SampleSubaccountAddr1
			marketID := common.HexToHash("0x1e11532fc29f1bc3eb75f6fddf4997e904c780ddf155ecb58bc89bf723e1ba56")

			// Set Buy Orders
			isBuy := true
			buyOrder1 := addSpotLimitOrder(app, ctx, "100.1", "1.5", subaccountID, marketID, isBuy)
			buyOrder2 := addSpotLimitOrder(app, ctx, "300.1", "3.5", subaccountID, marketID, isBuy)
			buyOrder3 := addSpotLimitOrder(app, ctx, "200.1", "100.531", subaccountID, marketID, isBuy)

			// Set Sell Orders
			isBuy = false
			sellOrder1 := addSpotLimitOrder(app, ctx, "100.1", "1.5", subaccountID, marketID, isBuy)
			sellOrder2 := addSpotLimitOrder(app, ctx, "300.1", "3.5", subaccountID, marketID, isBuy)
			sellOrder3 := addSpotLimitOrder(app, ctx, "200.1", "100.531", subaccountID, marketID, isBuy)

			// Check Buy orderbook
			buyOrderbook := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, true)
			Expect(len(buyOrderbook)).To(Equal(3))

			// check order prices are descending
			for idx, buyOrder := range buyOrderbook {
				if idx == len(buyOrderbook)-1 {
					break
				}

				nextOrder := buyOrderbook[idx+1]
				Expect(buyOrder.OrderInfo.Price.GT(nextOrder.OrderInfo.Price)).To(BeTrue())
			}

			// check order states are equal as expected
			Expect(buyOrderbook[0]).To(Equal(&buyOrder2))
			Expect(buyOrderbook[1]).To(Equal(&buyOrder3))
			Expect(buyOrderbook[2]).To(Equal(&buyOrder1))

			// Check Sell orderbook
			sellOrderbook := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, false)

			// length sanity check
			Expect(len(sellOrderbook)).To(Equal(3))

			// check order prices are descending
			for idx, sellOrder := range sellOrderbook {
				if idx == len(sellOrderbook)-1 {
					break
				}
				nextOrder := sellOrderbook[idx+1]
				Expect(sellOrder.OrderInfo.Price.LT(nextOrder.OrderInfo.Price)).To(BeTrue())
			}

			// check order states are equal as expected
			Expect(sellOrderbook[0]).To(Equal(&sellOrder1))
			Expect(sellOrderbook[1]).To(Equal(&sellOrder3))
			Expect(sellOrderbook[2]).To(Equal(&sellOrder2))
		})
	})

	Describe("When force closing a market", func() {

		It("All orders should be cancelled", func() {
			// for some reason setting orders doesn't work if it's in BeforeEach block instead of "it" block, so all setup is here

			subaccountBuyID := testexchange.SampleSubaccountAddr1
			subaccountSellID := testexchange.SampleSubaccountAddr2
			subaccountsList := []common.Hash{subaccountBuyID, subaccountSellID}

			testInput.AddSpotDepositsForSubaccounts(app, ctx, 1, nil, subaccountsList)

			marketID := market.MarketID()

			msgs := testInput.NewListOfMsgCreateSpotLimitOrderForMarketIndex(0,
				// buy orders
				testexchange.NewBareSpotLimitOrderFromString("100.1", "1.5", types.OrderType_BUY_PO, subaccountBuyID),
				testexchange.NewBareSpotLimitOrderFromString("300.1", "3.5", types.OrderType_BUY_PO, subaccountBuyID),
				testexchange.NewBareSpotLimitOrderFromString("200.1", "100.531", types.OrderType_BUY_PO, subaccountBuyID),
				// sell orders
				testexchange.NewBareSpotLimitOrderFromString("400.1", "1.5", types.OrderType_SELL_PO, subaccountSellID),
				testexchange.NewBareSpotLimitOrderFromString("500.1", "3.5", types.OrderType_SELL_PO, subaccountSellID),
				testexchange.NewBareSpotLimitOrderFromString("600.1", "100.531", types.OrderType_SELL_PO, subaccountSellID),
			)

			for _, msg := range msgs {
				testexchange.ReturnOrFail(msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), msg))
			}

			buyOrderbook := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, true)
			sellOrderbook := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, false)

			Expect(len(buyOrderbook)).To(Equal(3))
			Expect(len(sellOrderbook)).To(Equal(3))

			app.ExchangeKeeper.CancelAllRestingLimitOrdersFromSpotMarket(ctx, market, marketID)

			buyOrderbook2 := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, true)
			sellOrderbook2 := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, marketID, false)

			Expect(len(buyOrderbook2)).To(Equal(0))
			Expect(len(sellOrderbook2)).To(Equal(0))
		})
	})
})

func addSpotLimitOrder(app *simapp.InjectiveApp, ctx sdk.Context, priceStr string, quantityStr string, subaccountID common.Hash, marketID common.Hash, isBuy bool) types.SpotLimitOrder {
	price, quantity := testexchange.PriceAndQuantityFromString(priceStr, quantityStr)
	order := testexchange.NewSpotLimitOrder(price, quantity, subaccountID, marketID, isBuy)
	app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &order, marketID, isBuy, common.BytesToHash(order.OrderHash))
	return order
}
