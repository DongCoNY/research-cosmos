package keeper_test

import (
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
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Emergency Settle Market Tests - non-default subaccount", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		derivativeMarket  *types.DerivativeMarket
		msgServer         types.MsgServer
		err               error
		buyer             = testexchange.SampleNonDefaultSubaccountAddr1
		seller            = testexchange.SampleNonDefaultSubaccountAddr2
		marketMaker       = testexchange.SampleNonDefaultSubaccountAddr3
		liquidator        = testexchange.SampleNonDefaultSubaccountAddr4
		feeRecipient      = sdk.MustAccAddressFromBech32(testexchange.DefaultFeeRecipientAddress)
		buyerAccount      = types.SubaccountIDToSdkAddress(buyer)
		liquidatorAccount = types.SubaccountIDToSdkAddress(liquidator)
		startingPrice     = sdk.NewDec(2000)
		isMarketMaking    bool
		exchangeAddress   sdk.AccAddress
		insuranceAddress  sdk.AccAddress
		quoteDenom        string
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

		exchangeAddress = app.AccountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()
		insuranceAddress = app.AccountKeeper.GetModuleAccount(ctx, insurancetypes.ModuleName).GetAddress()

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		initialInsuranceFundBalance = sdk.NewDec(44)
		market := testInput.Perps[0]
		quoteDenom = market.QuoteDenom

		coin := sdk.NewCoin(quoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, quoteDenom, oracleBase, oracleQuote, oracleType, -1))

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
		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)
	})

	Describe("When liquidating a position", func() {
		var (
			liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, liquidatedOwnerAvailableBalance sdk.Dec
			marketMakerMargin, marketMakerQuantity, marketMakerDiff                                                        sdk.Dec
			newOraclePrice, missingFunds                                                                                   sdk.Dec
			buyerDepositBefore, sellerDepositBefore, marketMakerDepositBefore, liquidatorDepositBefore                     *types.Deposit
			liquidatorBalanceBefore, feeRecipientBalanceBefore                                                             math.Int
			exchangeBalanceBefore, insuranceBalanceBefore                                                                  sdk.Coin
			initialCoinDeposit                                                                                             math.Int
		)

		var expectCorrectNegativeLiquidationPayout = func(isUsingMarketMakerOrder bool, addedBuyerAvailableBalanceFromOrders, addedInsuranceDeposits sdk.Dec) {
			buyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
			sellerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
			marketMakerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
			liquidatorDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)

			liquidatorBalanceAfter := app.BankKeeper.GetBalance(ctx, liquidatorAccount, quoteDenom).Amount
			feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom).Amount
			exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
			insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)

			availableBuyerDiff := buyerDepositAfter.AvailableBalance.Sub(buyerDepositBefore.AvailableBalance)
			totalBuyerDiff := buyerDepositAfter.TotalBalance.Sub(buyerDepositBefore.TotalBalance)

			availableSellerDiff := sellerDepositAfter.AvailableBalance.Sub(sellerDepositBefore.AvailableBalance)
			totalSellerDiff := sellerDepositAfter.TotalBalance.Sub(sellerDepositBefore.TotalBalance)
			totalMarketMakerDiff := marketMakerDepositAfter.TotalBalance.Sub(marketMakerDepositBefore.TotalBalance)
			availableLiquidatorDiff := liquidatorDepositAfter.AvailableBalance.Sub(liquidatorDepositBefore.AvailableBalance)
			totalLiquidatorDiff := liquidatorDepositAfter.TotalBalance.Sub(liquidatorDepositBefore.TotalBalance)
			bankLiquidatorDiff := liquidatorBalanceAfter.Sub(liquidatorBalanceBefore)
			feeRecipientDiff := feeRecipientBalanceAfter.Sub(feeRecipientBalanceBefore)
			exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)
			insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)

			hasSettledMarket := missingFunds.GT(liquidatedOwnerAvailableBalance.Add(addedBuyerAvailableBalanceFromOrders).Add(addedInsuranceDeposits))
			existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

			if hasSettledMarket {
				Expect(availableBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(totalBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				expectedSellerPnlPayout := sdk.NewDec(1220)
				expectedAvailableSellerDiff := expectedSellerPnlPayout.Sub(expectedSellerPnlPayout.Mul(missingFunds).Quo(expectedSellerPnlPayout)).Add(liquidatedPositionMargin)
				expectedTotalSellerDiff := expectedAvailableSellerDiff

				Expect(availableSellerDiff.String()).Should(Equal(expectedAvailableSellerDiff.String()))
				Expect(totalSellerDiff.String()).Should(Equal(expectedTotalSellerDiff.String()))

				Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(marketMakerDepositAfter.AvailableBalance.String()).Should(Equal(marketMakerDepositAfter.TotalBalance.String()))

				Expect(feeRecipientDiff.String()).To(Equal(sdk.ZeroInt().String()))
				Expect(exchangeBalanceDiff.String()).Should(Equal(initialInsuranceFundBalance.RoundInt().String()))
				Expect(insuranceBalanceDiff.String()).Should(Equal(initialInsuranceFundBalance.Neg().RoundInt().String()))
				Expect(len(existingPositions)).Should(Equal(0))
			} else {
				expectedAvailableBuyerDiff := sdk.MinDec(missingFunds.Sub(addedBuyerAvailableBalanceFromOrders), liquidatedOwnerAvailableBalance).Neg()
				expectedTotalBuyerDiff := sdk.MinDec(missingFunds, liquidatedOwnerAvailableBalance.Add(addedBuyerAvailableBalanceFromOrders)).Neg()
				Expect(availableBuyerDiff.String()).Should(Equal(expectedAvailableBuyerDiff.String()))
				Expect(totalBuyerDiff.String()).Should(Equal(expectedTotalBuyerDiff.String()))

				Expect(availableSellerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(totalSellerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
				marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)
				marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
				Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()))

				expectedInsuranceWithdrawal := sdk.MaxDec(sdk.ZeroDec(), missingFunds.Sub(liquidatedOwnerAvailableBalance).Sub(addedBuyerAvailableBalanceFromOrders))

				Expect(exchangeBalanceDiff.Add(feeRecipientDiff).String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().RoundInt().String()))
				Expect(insuranceBalanceDiff.String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().Neg().RoundInt().String()))
				Expect(len(existingPositions)).Should(Equal(2))
			}

			Expect(availableLiquidatorDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			Expect(totalLiquidatorDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			Expect(bankLiquidatorDiff.ToDec().String()).Should(Equal(sdk.ZeroDec().String()))
		}

		BeforeEach(func() {
			isMarketMaking = true
		})

		JustBeforeEach(func() {
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, types.OrderType_SELL, seller)

			initialCoinDeposit = sdk.NewInt(200000)
			coin := sdk.NewCoin(quoteDenom, initialCoinDeposit)

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, marketMaker.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, liquidator.String(), sdk.NewCoins(coin))

			if isMarketMaking {
				marketMakerQuantity = sdk.NewDec(200)
				marketMakerMargin = sdk.NewDec(27000)

				bigLimitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(liquidatedPositionPrice.Sub(marketMakerDiff), marketMakerQuantity, marketMakerMargin, types.OrderType_BUY, marketMaker)
				bigLimitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(liquidatedPositionPrice.Add(marketMakerDiff), marketMakerQuantity, marketMakerMargin, types.OrderType_SELL, marketMaker)

				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeBuyOrder)
				testexchange.OrFail(err)
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeSellOrder)
				testexchange.OrFail(err)
			}

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(newOraclePrice, ctx.BlockTime().Unix()))

			buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
			sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
			marketMakerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
			liquidatorDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)

			liquidatorBalanceBefore = app.BankKeeper.GetBalance(ctx, liquidatorAccount, quoteDenom).Amount
			feeRecipientBalanceBefore = app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom).Amount
			exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
			insuranceBalanceBefore = app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
		})

		AfterEach(func() {
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Describe("with negative liquidation payout", func() {
			BeforeEach(func() {
				// liquidation payout = -200
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)
				marketMakerDiff = sdk.NewDec(600)
				newOraclePrice = sdk.NewDec(1400)

				missingFunds = sdk.NewDec(200)
			})

			Describe("with sufficient user available balance to cover missing funds", func() {
				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("200.96") // so not missing any funds after using available
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
						Sender:       buyerAccount.String(),
						SubaccountId: buyer.Hex(),
						Amount:       sdk.NewCoin(quoteDenom, withdrawAmount.RoundInt()),
					})
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				It("return invalid emergency settle error", func() {
					emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidator)

					_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
					Expect(err).To(Equal(types.ErrInvalidEmergencySettle))
				})
			})

			Describe("with sufficient user locked balances in transient order to cover missing funds", func() {
				var secondBuyerOrderMargin, secondBuyerOrderQuantity, secondBuyerOrderPrice sdk.Dec

				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("49.96") // so still missing 150.04
				})

				JustBeforeEach(func() {
					secondBuyerOrderMargin = sdk.NewDec(154)
					secondBuyerOrderQuantity = sdk.NewDec(1)
					secondBuyerOrderPrice = newOraclePrice
					limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(secondBuyerOrderPrice, secondBuyerOrderQuantity, secondBuyerOrderMargin, types.OrderType_BUY, buyer)
					_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
					testexchange.OrFail(err)

					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee).Sub(secondBuyerOrderMargin)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
						Sender:       buyerAccount.String(),
						SubaccountId: buyer.Hex(),
						Amount:       sdk.NewCoin(quoteDenom, withdrawAmount.RoundInt()),
					})
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
				})

				It("return invalid emergency settle error", func() {
					emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidator)

					_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
					Expect(err).To(Equal(types.ErrInvalidEmergencySettle))
				})
			})

			Describe("with sufficient user locked balances in resting order to cover missing funds", func() {
				var secondBuyerOrderMargin, secondBuyerOrderQuantity, secondBuyerOrderPrice sdk.Dec

				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("49.96") // so still missing 150.04
				})

				JustBeforeEach(func() {
					secondBuyerOrderMargin = sdk.NewDec(154)
					secondBuyerOrderQuantity = sdk.NewDec(1)
					secondBuyerOrderPrice = newOraclePrice
					limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(secondBuyerOrderPrice, secondBuyerOrderQuantity, secondBuyerOrderMargin, types.OrderType_BUY, buyer)
					_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee).Sub(secondBuyerOrderMargin)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
						Sender:       buyerAccount.String(),
						SubaccountId: buyer.Hex(),
						Amount:       sdk.NewCoin(quoteDenom, withdrawAmount.RoundInt()),
					})
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				It("return invalid emergency settle error", func() {
					emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidator)

					_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
					Expect(err).To(Equal(types.ErrInvalidEmergencySettle))
				})
			})

			Describe("with sufficient balances in insurance fund to cover missing funds", func() {
				var insuranceFundAddedDeposit sdk.Dec

				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("49.94") // so still missing 150.06
					insuranceFundAddedDeposit = sdk.NewDec(152)

					sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
					coin := sdk.NewCoin(quoteDenom, insuranceFundAddedDeposit.RoundInt())
					app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
					app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
					err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender, testInput.Perps[0].MarketID, coin)
					testexchange.OrFail(err)
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
						Sender:       buyerAccount.String(),
						SubaccountId: buyer.Hex(),
						Amount:       sdk.NewCoin(quoteDenom, withdrawAmount.RoundInt()),
					})
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				It("return invalid emergency settle error", func() {
					emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidator)

					_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
					Expect(err).To(Equal(types.ErrInvalidEmergencySettle))
				})
			})

			Describe("with insufficient balances to cover missing funds", func() {
				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("49.94") // so still missing 150.06
					missingFunds = sdk.NewDec(176)
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
						Sender:       buyerAccount.String(),
						SubaccountId: buyer.Hex(),
						Amount:       sdk.NewCoin(quoteDenom, withdrawAmount.RoundInt()),
					})
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				Describe("when liquidator has sufficient funds", func() {
					JustBeforeEach(func() {
						liquidatorSubaccountID := types.MustGetSubaccountIDOrDeriveFromNonce(liquidatorAccount, "0")

						app.BankKeeper.MintCoins(ctx, exchangetypes.ModuleName, sdk.NewCoins(sdk.NewCoin(quoteDenom, sdk.NewInt(1000000000000000))))
						app.ExchangeKeeper.SetDepositOrSendToBank(
							ctx,
							liquidatorSubaccountID,
							quoteDenom,
							types.Deposit{AvailableBalance: sdk.NewDec(1000000000000000), TotalBalance: sdk.NewDec(1000000000000000)},
							false,
						)

						liquidatorBalanceBefore = app.BankKeeper.GetBalance(ctx, liquidatorAccount, quoteDenom).Amount
					})

					It("should settle the market", func() {
						liquidatorSubaccountID := types.MustGetSubaccountIDOrDeriveFromNonce(liquidatorAccount, "0")

						emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidatorSubaccountID)
						_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
						testexchange.OrFail(err)

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						isUsingMarketMakerOrder := true
						expectCorrectNegativeLiquidationPayout(isUsingMarketMakerOrder, sdk.ZeroDec(), sdk.ZeroDec())

						markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
						Expect(markets[0].Status).Should(Equal(types.MarketStatus_Paused))
					})
				})

				Describe("when liquidator has insufficient funds", func() {
					It("should still settle the market", func() {
						liquidatorSubaccountID := types.MustGetSubaccountIDOrDeriveFromNonce(liquidatorAccount, "0")
						emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidatorSubaccountID)
						_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
						testexchange.OrFail(err)

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						isUsingMarketMakerOrder := true
						expectCorrectNegativeLiquidationPayout(isUsingMarketMakerOrder, sdk.ZeroDec(), sdk.ZeroDec())

						markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
						Expect(markets[0].Status).Should(Equal(types.MarketStatus_Paused))
					})
				})
			})
		})

		Describe("with positive liquidation payout", func() {
			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(500)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)
				marketMakerDiff = sdk.NewDec(200)
				newOraclePrice = sdk.NewDec(1795)
			})

			Describe("when there is not enough liquidity in orderbook", func() {
				BeforeEach(func() {
					isMarketMaking = false
				})

				It("return invalid emergency settle error", func() {
					emergencySettleMarketMsg := testInput.NewMsgEmergencySettleMarket(buyer, liquidator)

					_, err = msgServer.EmergencySettleMarket(sdk.WrapSDKContext(ctx), emergencySettleMarketMsg)
					Expect(err).To(Equal(types.ErrInvalidEmergencySettle))
				})
			})
		})
	})
})
