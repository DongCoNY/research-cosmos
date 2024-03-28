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
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Spot Orders Revert Unit Test", func() {
	var (
		testInput    testexchange.TestInput
		app          *simapp.InjectiveApp
		ctx          sdk.Context
		subaccountID common.Hash
		msgServer    exchangetypes.MsgServer
		marketID     common.Hash
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)

		_, err := app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		testexchange.OrFail(err)

		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
		subaccountID = testexchange.SampleSubaccountAddr1
		marketID = testInput.Spots[0].MarketID

		deposit := &exchangetypes.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
	})

	Describe("When there is not enough remaining limit sell liquidity", func() {
		BeforeEach(func() {
			isBuy := true
			buyPrice1, buyQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			buyPrice2, buyQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			buyPrice3, buyQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")

			buyOrder1 := testexchange.NewSpotLimitOrder(buyPrice1, buyQuantity1, subaccountID, marketID, isBuy)
			buyOrder2 := testexchange.NewSpotLimitOrder(buyPrice2, buyQuantity2, subaccountID, marketID, isBuy)
			buyOrder3 := testexchange.NewSpotLimitOrder(buyPrice3, buyQuantity3, subaccountID, marketID, isBuy)

			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &buyOrder1, marketID, isBuy, common.BytesToHash(buyOrder1.OrderHash))
			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &buyOrder2, marketID, isBuy, common.BytesToHash(buyOrder2.OrderHash))
			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &buyOrder3, marketID, isBuy, common.BytesToHash(buyOrder3.OrderHash))
		})

		It("rejects only the market buy orders", func() {
			sellWorstPrice := sdk.NewDec(1)
			buyWorstPrice := sdk.NewDec(1000)
			quantity := sdk.NewDec(1)

			marketOrderMsgBuy := testInput.NewMsgCreateSpotMarketOrder(quantity, buyWorstPrice, exchangetypes.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)

			marketOrderMsgSell := testInput.NewMsgCreateSpotMarketOrder(quantity, sellWorstPrice, exchangetypes.OrderType_SELL, subaccountID)
			_, errMarketSell := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)

			Expect(errMarketBuy).To(Equal(exchangetypes.ErrNoLiquidity))
			Expect(errMarketSell).To(BeNil())
		})
	})

	Describe("When there is not enough remaining limit buy liquidity", func() {
		BeforeEach(func() {
			isBuy := false
			sellPrice1, sellQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			sellPrice2, sellQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			sellPrice3, sellQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")

			sellOrder1 := testexchange.NewSpotLimitOrder(sellPrice1, sellQuantity1, subaccountID, marketID, isBuy)
			sellOrder2 := testexchange.NewSpotLimitOrder(sellPrice2, sellQuantity2, subaccountID, marketID, isBuy)
			sellOrder3 := testexchange.NewSpotLimitOrder(sellPrice3, sellQuantity3, subaccountID, marketID, isBuy)

			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &sellOrder1, marketID, isBuy, common.BytesToHash(sellOrder1.OrderHash))
			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &sellOrder2, marketID, isBuy, common.BytesToHash(sellOrder2.OrderHash))
			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &sellOrder3, marketID, isBuy, common.BytesToHash(sellOrder3.OrderHash))
		})

		It("rejects only the market sell orders", func() {
			sellWorstPrice := sdk.NewDec(1)
			buyWorstPrice := sdk.NewDec(1000)
			quantity := sdk.NewDec(1)

			marketOrderMsgSell := testInput.NewMsgCreateSpotMarketOrder(quantity, sellWorstPrice, exchangetypes.OrderType_SELL, subaccountID)
			_, errMarketSell := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)

			marketOrderMsgBuy := testInput.NewMsgCreateSpotMarketOrder(quantity, buyWorstPrice, exchangetypes.OrderType_BUY, subaccountID)
			_, errMarketBuy := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgBuy)

			Expect(errMarketSell).To(Equal(exchangetypes.ErrNoLiquidity))
			Expect(errMarketBuy).To(BeNil())
		})
	})

	Describe("When spot market does not exist", func() {
		fakeMarketId := "0x0000000000000000000000000000000000000000000000000000000000000001"
		quantity := sdk.NewDec(1)

		It("rejects the market order creation", func() {
			sellWorstPrice := sdk.ZeroDec()
			orderType := exchangetypes.OrderType_SELL

			marketOrderMsgSell := &exchangetypes.MsgCreateSpotMarketOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.SpotOrder{
					MarketId: fakeMarketId,
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID.Hex(),
						FeeRecipient: testexchange.DefaultAddress,
						Price:        sellWorstPrice,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					TriggerPrice: nil,
				},
			}

			_, errMarketSell := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), marketOrderMsgSell)
			expectedError := "active spot market doesn't exist " + fakeMarketId + ": " + exchangetypes.ErrSpotMarketNotFound.Error()

			Expect(errMarketSell.Error()).To(Equal(expectedError))
		})

		It("rejects the limit order creation", func() {
			sellPrice := sdk.NewDec(1000)
			orderType := exchangetypes.OrderType_SELL

			limitOrderMsgSell := &exchangetypes.MsgCreateSpotLimitOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.SpotOrder{
					MarketId: fakeMarketId,
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID.Hex(),
						FeeRecipient: testexchange.DefaultAddress,
						Price:        sellPrice,
						Quantity:     quantity,
					},
					OrderType:    orderType,
					TriggerPrice: nil,
				},
			}

			_, errLimitSell := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderMsgSell)
			expectedError := "active spot market doesn't exist " + fakeMarketId + ": " + exchangetypes.ErrSpotMarketNotFound.Error()

			Expect(errLimitSell.Error()).To(Equal(expectedError))
		})

		// It("rejects params update", func() {
		// 	makerFeeRate := sdk.NewDec(1)
		// 	takerFeeRate := sdk.NewDec(1)
		// 	relayerFeeShareRate := sdk.NewDec(1)
		// 	_, errSetStatus := app.ExchangeKeeper.UpdateSpotMarketParam(
		// 		ctx,
		// 		common.HexToHash(fakeMarketId),
		// 		makerFeeRate,
		// 		takerFeeRate,
		// 		relayerFeeShareRate,
		// 	)
		// 	expectedError := exchangetypes.ErrSpotMarketNotFound.Error()

		// 	Expect(errSetStatus.Error()).To(Equal(expectedError))
		// })

		It("rejects status update", func() {
			baseDenom := "FETH"
			quoteDenom := "FUSDT"

			marketID := exchangetypes.NewSpotMarketID(baseDenom, quoteDenom)

			_, errSetStatus := app.ExchangeKeeper.SetSpotMarketStatus(ctx, marketID, exchangetypes.MarketStatus_Active)
			expectedError := "marketID " + marketID.Hex() + ": " + exchangetypes.ErrSpotMarketNotFound.Error()

			Expect(errSetStatus.Error()).To(Equal(expectedError))
		})
	})

	Describe("When spot market already exists", func() {
		It("rejects market creation", func() {
			baseDenom := testInput.Spots[0].BaseDenom
			quoteDenom := testInput.Spots[0].QuoteDenom
			ticker := testInput.Spots[0].Ticker
			_, errMarketLaunch := app.ExchangeKeeper.SpotMarketLaunch(ctx, ticker, baseDenom, quoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
			expectedPreError := "ticker " + ticker + " baseDenom " + baseDenom + " quoteDenom " + quoteDenom
			expectedError := expectedPreError + ": " + exchangetypes.ErrSpotMarketExists.Error()

			Expect(errMarketLaunch.Error()).To(Equal(expectedError))
		})
	})

	Describe("When there are no quote deposits into the subaccount", func() {
		subaccountID := testexchange.SampleSubaccountAddr2
		It("rejects the limit buy order creation", func() {
			orderType := exchangetypes.OrderType_BUY
			buyPrice, buyQuantity := testexchange.PriceAndQuantityFromString("100", "100")

			buyOrder := testInput.NewMsgCreateSpotLimitOrder(buyPrice, buyQuantity, orderType, subaccountID)

			_, errCreateLimitOrder := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), buyOrder)
			Expect(testexchange.IsExpectedInsufficientFundsErrorType(subaccountID, errCreateLimitOrder)).To(BeTrue(), "Incorrect error type returned")
		})

		It("rejects the market buy order creation", func() {
			isBuy := false
			sellPrice1, sellQuantity1 := testexchange.PriceAndQuantityFromString("90.1", "1.5")
			sellOrder1 := testexchange.NewSpotLimitOrder(sellPrice1, sellQuantity1, subaccountID, marketID, isBuy)
			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &sellOrder1, marketID, isBuy, common.BytesToHash(sellOrder1.OrderHash))

			orderType := exchangetypes.OrderType_BUY
			worstBuyPrice, buyQuantity := testexchange.PriceAndQuantityFromString("100", "100")

			buyOrder := testInput.NewMsgCreateSpotMarketOrder(worstBuyPrice, buyQuantity, orderType, subaccountID)

			_, errCreateMarketOrder := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), buyOrder)
			Expect(testexchange.IsExpectedInsufficientFundsErrorType(subaccountID, errCreateMarketOrder)).To(BeTrue(), "Incorrect error type returned")
		})

		It("rejects the withdrawals of quote asset", func() {
			quoteDenom := testInput.Spots[0].QuoteDenom
			amountToWithdraw := sdk.NewCoin(quoteDenom, sdk.NewInt(100))

			errWithdrawQuote := app.ExchangeKeeper.DecrementDeposit(ctx, subaccountID, amountToWithdraw.Denom, amountToWithdraw.Amount.ToDec())
			expectedError := exchangetypes.ErrInsufficientDeposit.Error()

			Expect(errWithdrawQuote.Error()).To(Equal(expectedError))
		})
	})

	Describe("When there are no base deposits into the subaccount", func() {
		subaccountID := testexchange.SampleSubaccountAddr3
		It("rejects the limit sell order creation", func() {
			orderType := exchangetypes.OrderType_SELL
			sellPrice, sellQuantity := testexchange.PriceAndQuantityFromString("100", "100")

			sellOrder := testInput.NewMsgCreateSpotLimitOrder(sellPrice, sellQuantity, orderType, subaccountID)
			_, errCreateLimitOrder := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), sellOrder)

			Expect(testexchange.IsExpectedInsufficientFundsErrorType(subaccountID, errCreateLimitOrder)).To(BeTrue(), "Incorrect error type returned")
		})

		It("rejects the market sell order creation", func() {
			isBuy := true
			sellPrice1, sellQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			sellOrder1 := testexchange.NewSpotLimitOrder(sellPrice1, sellQuantity1, subaccountID, marketID, isBuy)
			app.ExchangeKeeper.SetNewSpotLimitOrder(ctx, &sellOrder1, marketID, isBuy, common.BytesToHash(sellOrder1.OrderHash))

			orderType := exchangetypes.OrderType_SELL
			worstSellPrice, sellQuantity := testexchange.PriceAndQuantityFromString("100", "100")

			sellOrder := testInput.NewMsgCreateSpotMarketOrder(worstSellPrice, sellQuantity, orderType, subaccountID)

			_, errCreateMarketOrder := msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), sellOrder)
			Expect(testexchange.IsExpectedInsufficientFundsErrorType(subaccountID, errCreateMarketOrder)).To(BeTrue(), "Incorrect error type returned")

		})

		It("rejects the withdrawals of base asset", func() {
			baseDenom := testInput.Spots[0].BaseDenom
			amountToWithdraw := sdk.NewCoin(baseDenom, sdk.NewInt(100))

			errWithdrawBase := app.ExchangeKeeper.DecrementDeposit(ctx, subaccountID, amountToWithdraw.Denom, amountToWithdraw.Amount.ToDec())
			expectedError := exchangetypes.ErrInsufficientDeposit.Error()

			Expect(errWithdrawBase.Error()).To(Equal(expectedError))
		})
	})

	Describe("When there are not enough quote deposits into the subaccount", func() {
		subaccountID := testexchange.SampleSubaccountAddr3
		quoteDeposit := &exchangetypes.Deposit{
			AvailableBalance: sdk.NewDec(50),
			TotalBalance:     sdk.NewDec(50),
		}
		JustBeforeEach(func() {
			By("Depositing funds into subaccount")
			testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
		})

		It("rejects the limit buy order creation", func() {
			quoteDenom := testInput.Spots[0].QuoteDenom
			orderType := exchangetypes.OrderType_BUY
			buyPrice, buyQuantity := testexchange.PriceAndQuantityFromString("100", "100")

			buyOrder := testInput.NewMsgCreateSpotLimitOrder(buyPrice, buyQuantity, orderType, subaccountID)
			depositNeeded := buyOrder.Order.OrderInfo.Price.Mul(buyOrder.Order.OrderInfo.Quantity).Mul(sdk.NewDec(1).Add(sdk.NewDecWithPrec(3, 3)))

			_, errCreateLimitOrder := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), buyOrder)
			expectedError := testexchange.GetInsufficientFundsErrorMessage(subaccountID, quoteDenom, quoteDeposit.AvailableBalance, depositNeeded)

			Expect(errCreateLimitOrder.Error()).To(Equal(expectedError))
		})

		It("rejects the withdrawals of quote asset", func() {
			quoteDenom := testInput.Spots[0].QuoteDenom
			amountToWithdraw := sdk.NewCoin(quoteDenom, sdk.NewInt(100))

			errWithdrawQuote := app.ExchangeKeeper.DecrementDeposit(ctx, subaccountID, amountToWithdraw.Denom, amountToWithdraw.Amount.ToDec())
			expectedError := exchangetypes.ErrInsufficientDeposit.Error()

			Expect(errWithdrawQuote.Error()).To(Equal(expectedError))
		})
	})

	Describe("When there are not enough base deposits into the subaccount", func() {
		subaccountID := testexchange.SampleSubaccountAddr4
		baseDeposit := &exchangetypes.Deposit{
			AvailableBalance: sdk.NewDec(50),
			TotalBalance:     sdk.NewDec(50),
		}
		JustBeforeEach(func() {
			By("Depositing funds into subaccount")
			testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, baseDeposit.AvailableBalance.TruncateInt())))
		})

		It("rejects the limit sell order creation", func() {
			baseDenom := testInput.Spots[0].BaseDenom
			orderType := exchangetypes.OrderType_SELL
			sellPrice, sellQuantity := testexchange.PriceAndQuantityFromString("100", "100")

			sellOrder := testInput.NewMsgCreateSpotLimitOrder(sellPrice, sellQuantity, orderType, subaccountID)
			depositNeeded := sellOrder.Order.OrderInfo.Quantity
			_ = depositNeeded
			_ = baseDenom

			_, errCreateLimitOrder := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), sellOrder)
			expectedError := testexchange.GetInsufficientFundsErrorMessage(subaccountID, baseDenom, baseDeposit.AvailableBalance, depositNeeded)

			Expect(errCreateLimitOrder.Error()).To(Equal(expectedError))
		})

		It("rejects the withdrawals of base asset", func() {
			baseDenom := testInput.Spots[0].BaseDenom
			amountToWithdraw := sdk.NewCoin(baseDenom, sdk.NewInt(100))

			errWithdrawBase := app.ExchangeKeeper.DecrementDeposit(ctx, subaccountID, amountToWithdraw.Denom, amountToWithdraw.Amount.ToDec())
			expectedError := exchangetypes.ErrInsufficientDeposit.Error()

			Expect(errWithdrawBase.Error()).To(Equal(expectedError))
		})
	})
})
