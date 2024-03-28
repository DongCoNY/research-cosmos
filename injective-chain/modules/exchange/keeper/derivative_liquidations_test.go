package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Liquidation Tests - non-default subaccount", func() {
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
			liquidationOrderPrice, liquidationOrderQuantity, liquidationOrderMargin                                        sdk.Dec
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

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					isUsingMarketMakerOrder := true
					expectCorrectNegativeLiquidationPayout(isUsingMarketMakerOrder, sdk.ZeroDec(), sdk.ZeroDec())

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active))
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
				})

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					isUsingMarketMakerOrder := true
					tradingFee := secondBuyerOrderPrice.Mul(secondBuyerOrderQuantity).Mul(derivativeMarket.TakerFeeRate)
					addedBuyerAvailableBalanceFromOrders := secondBuyerOrderMargin.Add(tradingFee)
					expectCorrectNegativeLiquidationPayout(isUsingMarketMakerOrder, addedBuyerAvailableBalanceFromOrders, sdk.ZeroDec())

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active))
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

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					isUsingMarketMakerOrder := true
					tradingFee := secondBuyerOrderPrice.Mul(secondBuyerOrderQuantity).Mul(derivativeMarket.MakerFeeRate)
					addedBuyerAvailableBalanceFromOrders := secondBuyerOrderMargin.Add(tradingFee)
					expectCorrectNegativeLiquidationPayout(isUsingMarketMakerOrder, addedBuyerAvailableBalanceFromOrders, sdk.ZeroDec())

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active))
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

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)

					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					isUsingMarketMakerOrder := true
					expectCorrectNegativeLiquidationPayout(isUsingMarketMakerOrder, sdk.ZeroDec(), insuranceFundAddedDeposit)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active))
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

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
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

		Describe("with positive liquidation payout", func() {
			var expectCorrectPositiveLiquidationPayout = func(isUsingMarketMakerOrder bool) {
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
				availableMarketMakerDiff := marketMakerDepositAfter.AvailableBalance.Sub(marketMakerDepositBefore.AvailableBalance)
				totalMarketMakerDiff := marketMakerDepositAfter.TotalBalance.Sub(marketMakerDepositBefore.TotalBalance)
				availableLiquidatorDiff := liquidatorDepositAfter.AvailableBalance.Sub(liquidatorDepositBefore.AvailableBalance)

				totalLiquidatorDiff := liquidatorDepositAfter.TotalBalance.Sub(liquidatorDepositBefore.TotalBalance)
				bankLiquidatorDiff := liquidatorBalanceAfter.Sub(liquidatorBalanceBefore)
				feeRecipientDiff := feeRecipientBalanceAfter.Sub(feeRecipientBalanceBefore)
				exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)
				insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)

				Expect(availableBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(totalBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(availableSellerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(totalSellerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				Expect(availableMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
				marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)

				var expectedLiquidatorPayout, expectedInsuranceFundPayout, liquidatorPositionMargin sdk.Dec

				if isUsingMarketMakerOrder {
					marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
					Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()))

					pnlNotionalLiquidatedPosition := liquidatedPositionQuantity.Mul(liquidatedPositionPrice.Sub(marketMakerPositionEntryPrice))
					expectedPayout := liquidatedPositionMargin.Sub(pnlNotionalLiquidatedPosition)

					liquidatorRewardShareRate := app.ExchangeKeeper.GetLiquidatorRewardShareRate(ctx)
					expectedLiquidatorPayout = expectedPayout.Mul(liquidatorRewardShareRate)
					expectedInsuranceFundPayout = expectedPayout.Sub(expectedLiquidatorPayout)
					Expect(availableLiquidatorDiff.Add(bankLiquidatorDiff.ToDec()).String()).Should(Equal(expectedLiquidatorPayout.String()))
					Expect(totalLiquidatorDiff.Add(bankLiquidatorDiff.ToDec()).String()).Should(Equal(expectedLiquidatorPayout.String()))
				} else {
					Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

					liquidatorPositionMargin = liquidationOrderMargin.Quo(liquidationOrderQuantity).Mul(liquidatedPositionQuantity)
					liquidatorTradingFee := liquidatedPositionQuantity.Mul(liquidationOrderPrice).Mul(derivativeMarket.MakerFeeRate)
					pnlNotionalLiquidatedPosition := liquidatedPositionQuantity.Mul(liquidatedPositionPrice.Sub(liquidationOrderPrice))
					expectedPayout := liquidatedPositionMargin.Sub(pnlNotionalLiquidatedPosition)
					liquidatorRewardShareRate := app.ExchangeKeeper.GetLiquidatorRewardShareRate(ctx)
					expectedLiquidatorPayout = expectedPayout.Mul(liquidatorRewardShareRate)
					expectedInsuranceFundPayout = expectedPayout.Sub(expectedLiquidatorPayout)
					expectedLiquidatorPositionBalanceDiff := liquidatorPositionMargin.Add(liquidatorTradingFee).Neg()
					expectedLiquidatorBalanceDiff := expectedLiquidatorPayout.Add(expectedLiquidatorPositionBalanceDiff)

					Expect(bankLiquidatorDiff.ToDec().Add(availableLiquidatorDiff).String()).Should(Equal(expectedLiquidatorBalanceDiff.String()))
				}

				Expect(sdk.NewDecFromInt(exchangeBalanceDiff.Add(bankLiquidatorDiff).Add(feeRecipientDiff)).String()).Should(Equal(expectedInsuranceFundPayout.Neg().String()))
				Expect(sdk.NewDecFromInt(insuranceBalanceDiff).String()).Should(Equal(expectedInsuranceFundPayout.String()))

				existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)

				for _, existingPosition := range existingPositions {
					Expect(existingPosition.MarketId).Should(Equal(derivativeMarket.MarketId))

					switch existingPosition.SubaccountId {
					case buyer.Hex():
						Fail("Liquidated position was not removed as expected")
					case seller.Hex():
						Expect(existingPosition.Position.EntryPrice.String()).Should(Equal(liquidatedPositionPrice.String()))
						Expect(existingPosition.Position.IsLong).Should(BeFalse())
						Expect(existingPosition.Position.Margin.String()).Should(Equal(liquidatedPositionMargin.String()))
						Expect(existingPosition.Position.Quantity.String()).Should(Equal(liquidatedPositionQuantity.String()))
						Expect(existingPosition.Position.CumulativeFundingEntry.String()).Should(Equal(sdk.ZeroDec().String()))
					case marketMaker.Hex():
						if !isUsingMarketMakerOrder {
							Fail("Position with unexpected subaccount id encountered")
						}

						Expect(existingPosition.Position.EntryPrice.String()).Should(Equal(marketMakerPositionEntryPrice.String()))
						Expect(existingPosition.Position.IsLong).Should(BeTrue())
						Expect(existingPosition.Position.Margin.String()).Should(
							Equal(marketMakerPositionMargin.String()),
						)
						Expect(existingPosition.Position.Quantity.String()).Should(Equal(liquidatedPositionQuantity.String()))
						Expect(existingPosition.Position.CumulativeFundingEntry.String()).Should(Equal(sdk.ZeroDec().String()))
					case liquidator.Hex():
						if isUsingMarketMakerOrder {
							Fail("Position with unexpected subaccount id encountered")
						}

						Expect(existingPosition.Position.EntryPrice.String()).Should(Equal(liquidationOrderPrice.String()))
						Expect(existingPosition.Position.IsLong).Should(BeTrue())
						Expect(existingPosition.Position.Margin.String()).Should(Equal(liquidatorPositionMargin.String()))
						Expect(existingPosition.Position.Quantity.String()).Should(Equal(liquidatedPositionQuantity.String()))
						Expect(existingPosition.Position.CumulativeFundingEntry.String()).Should(Equal(sdk.ZeroDec().String()))
					default:
						Fail("Position with unexpected subaccount id encountered")
					}
				}

				Expect(len(existingPositions)).Should(Equal(2))

				limitBuyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
				limitSellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
				Expect(len(limitBuyOrders)).Should(Equal(1))
				Expect(len(limitSellOrders)).Should(Equal(1))

				marketMakerBuyOrder := limitBuyOrders[0]
				marketMakerSellOrder := limitSellOrders[0]
				Expect(marketMakerSellOrder.Margin.String()).Should(Equal(marketMakerMargin.String()))
				Expect(marketMakerSellOrder.OrderInfo.Price.String()).Should(Equal(liquidatedPositionPrice.Add(marketMakerDiff).String()))
				Expect(marketMakerSellOrder.OrderInfo.Quantity.String()).Should(Equal(marketMakerQuantity.String()))
				Expect(marketMakerSellOrder.OrderInfo.SubaccountId).Should(Equal(marketMaker.Hex()))
				Expect(marketMakerSellOrder.Fillable.String()).Should(Equal(marketMakerQuantity.String()))

				Expect(marketMakerBuyOrder.Margin.String()).Should(Equal(marketMakerMargin.String()))
				Expect(marketMakerBuyOrder.OrderInfo.Price.String()).Should(Equal(liquidatedPositionPrice.Sub(marketMakerDiff).String()))
				Expect(marketMakerBuyOrder.OrderInfo.Quantity.String()).Should(Equal(marketMakerQuantity.String()))
				Expect(marketMakerBuyOrder.OrderInfo.SubaccountId).Should(Equal(marketMaker.Hex()))

				if isUsingMarketMakerOrder {
					Expect(marketMakerBuyOrder.Fillable.String()).Should(Equal(marketMakerQuantity.Sub(liquidatedPositionQuantity).String()))
				} else {
					Expect(marketMakerBuyOrder.Fillable.String()).Should(Equal(marketMakerQuantity.String()))
				}

				markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
				Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active))
			}

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

				It("should reject liquidation", func() {
					liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					expectedError := "no liquidity on the orderbook!"
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Describe("when there is enough liquidity in orderbook", func() {
				Describe("without providing order", func() {
					It("should liquidate correctly", func() {
						liquidateMsg := testInput.NewMsgLiquidatePositionWithOrder(buyer, liquidator, types.OrderType_UNSPECIFIED, sdk.Dec{}, sdk.Dec{}, sdk.Dec{})

						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						testexchange.OrFail(err)

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						isUsingMarketMakerOrder := true
						expectCorrectPositiveLiquidationPayout(isUsingMarketMakerOrder)
					})
				})
			})

			Describe("when providing an order", func() {
				Describe("using full liquidator's subaccount id", func() {
					It("should liquidate correctly using this order in liquidity", func() {
						liquidationOrderPrice = liquidatedPositionPrice.Add(marketMakerDiff)
						liquidationOrderQuantity = sdk.NewDec(3)
						liquidationOrderMargin = sdk.NewDec(6300)

						liquidateMsg := testInput.NewMsgLiquidatePositionWithOrder(
							buyer,
							liquidator,
							types.OrderType_BUY,
							liquidationOrderPrice,
							liquidationOrderQuantity,
							liquidationOrderMargin,
						)

						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						testexchange.OrFail(err)
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						isUsingMarketMakerOrder := false
						expectCorrectPositiveLiquidationPayout(isUsingMarketMakerOrder)
					})
				})

				Describe("using simplified liquidator's subaccount id", func() {
					It("should liquidate correctly using this order in liquidity", func() {
						liquidationOrderPrice = liquidatedPositionPrice.Add(marketMakerDiff)
						liquidationOrderQuantity = sdk.NewDec(3)
						liquidationOrderMargin = sdk.NewDec(6300)

						liquidateMsg := testInput.NewMsgLiquidatePositionWithOrder(
							buyer,
							liquidator,
							types.OrderType_BUY,
							liquidationOrderPrice,
							liquidationOrderQuantity,
							liquidationOrderMargin,
						)
						liquidateMsg.Order.OrderInfo.SubaccountId = "1"

						_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
						testexchange.OrFail(err)
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						isUsingMarketMakerOrder := false
						expectCorrectPositiveLiquidationPayout(isUsingMarketMakerOrder)
					})
				})
			})
		})
	})
})

var _ = Describe("Liquidation Tests - default subaccount", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		err              error
		buyer            = testexchange.SampleDefaultSubaccountAddr1
		seller           = testexchange.SampleDefaultSubaccountAddr2
		marketMaker      = testexchange.SampleDefaultSubaccountAddr3
		liquidator       = testexchange.SampleDefaultSubaccountAddr4
		feeRecipient     = sdk.MustAccAddressFromBech32(testexchange.DefaultFeeRecipientAddress)
		buyerAccount     = types.SubaccountIDToSdkAddress(buyer)
		underwriter      = sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		startingPrice    = sdk.NewDec(2000)
		isMarketMaking   bool
		exchangeAddress  sdk.AccAddress
		insuranceAddress sdk.AccAddress
		quoteDenom       string
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

		initialInsuranceFundBalance = sdk.NewDec(202) //enough to cover missing funds

		market := testInput.Perps[0]
		quoteDenom = market.QuoteDenom

		coin := sdk.NewCoin(quoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, underwriter, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, underwriter, coin, testInput.Perps[0].Ticker, quoteDenom, oracleBase, oracleQuote, oracleType, -1))

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
			liquidatedOwnerBankBalance                                                                                             math.Int
			liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin                                          sdk.Dec
			marketMakerMargin, marketMakerQuantity, marketMakerDiff                                                                sdk.Dec
			missingFunds                                                                                                           sdk.Dec
			newOraclePrice                                                                                                         sdk.Dec
			buyerSpendableFundsBefore, sellerSpendableFundsBefore, marketMakerSpendableFundsBefore, liquidatorSpendableFundsBefore *types.Deposit
			feeRecipientBalanceBefore, exchangeBalanceBefore, insuranceBalanceBefore                                               sdk.Coin
			initialCoinDeposit                                                                                                     math.Int
		)

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

			buyerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
			sellerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
			marketMakerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
			liquidatorSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)

			feeRecipientBalanceBefore = app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)

			insuranceBalanceBefore = app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
			exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
		})

		AfterEach(func() {
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Describe("with negative liquidation payout", func() {
			BeforeEach(func() {
				// liquidation payout = -200
				missingFunds = sdk.NewDec(200)
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)
				marketMakerDiff = sdk.NewDec(600)
				newOraclePrice = sdk.NewDec(1400)
			})

			Describe("with sufficient user bank balance to cover missing funds", func() {
				BeforeEach(func() {
					liquidatedOwnerBankBalance = sdk.NewInt(201) // so not missing any funds (but which still shouldn't, because they are in bank)
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := initialCoinDeposit.Sub(math.Int(liquidatedPositionMargin.Ceil().TruncateInt())).Sub(math.Int(tradingFee.Ceil().TruncateInt()))
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerBankBalance)
					app.BankKeeper.SendCoinsFromAccountToModule(ctx, buyerAccount, auctiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(quoteDenom, withdrawAmount)))

					buyerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
				})

				It("should take funds from insurance fund and charge the user only the dust on subaccount without touching her bank balances", func() {
					liquidateMsg := testInput.NewMsgLiquidatePositionForMarketID(buyer, liquidator, testInput.Perps[0].MarketID)
					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					// there should still be 2 positions open
					existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(existingPositions)).Should(Equal(2), "incorrect number of positions")

					// market should still be active
					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active), "market wasn't active")

					// owner of the position with negative payout should keep his bank funds intact and only lose dust from available balance
					buyerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					Expect(buyerSpendableFundsAfter.AvailableBalance.String()).To(Equal(buyerSpendableFundsBefore.AvailableBalance.TruncateInt().ToDec().String()), "wrong buyer's spendable balance")

					// balances of the counterparty should stay the same (position shouldn't have been closed)
					sellerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
					Expect(sellerSpendableFundsAfter.AvailableBalance.String()).To(Equal(sellerSpendableFundsBefore.AvailableBalance.String()), "wrong seller's spendable balance")

					// market maker should have paid the fee and margin
					marketMakerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
					totalMarketMakerDiff := marketMakerSpendableFundsAfter.TotalBalance.Sub(marketMakerSpendableFundsBefore.TotalBalance)
					marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
					marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)
					marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
					Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()), "market maker total balance did not change accordingly")

					// liquidator didn't get anything due to negative payout
					liquidatorSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
					Expect(liquidatorSpendableFundsAfter.AvailableBalance.String()).To(Equal(liquidatorSpendableFundsBefore.AvailableBalance.String()), "wrong liquidator's spendable balance")
					Expect(liquidatorSpendableFundsAfter.TotalBalance.String()).To(Equal(liquidatorSpendableFundsBefore.TotalBalance.String()), "wrong liquidator's spendable balance")

					// insurance's balance should have decresed by missing funds minus dust taken from position's owner
					insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
					insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
					dustChargedFromBuyer := buyerSpendableFundsBefore.AvailableBalance.Sub(buyerSpendableFundsBefore.AvailableBalance.TruncateDec())
					expectedInsuranceWithdrawal := sdk.MaxDec(sdk.ZeroDec(), missingFunds.Sub(dustChargedFromBuyer))

					Expect(insuranceBalanceDiff.String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().Neg().RoundInt().String()), "insurance fund balance did not change accordingly")

					// amount taken from insurance fund should have gone to exchange and fee recipient
					feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)
					feeRecipientDiff := feeRecipientBalanceAfter.Amount.Sub(feeRecipientBalanceBefore.Amount)
					exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
					exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)

					Expect(exchangeBalanceDiff.Add(feeRecipientDiff).String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().RoundInt().String()), "exchange and fee recipeint balances were different than insurance withdrawal")
				})
			})

			Describe("with sufficient user locked balances in transient order to cover missing funds", func() {
				var secondBuyerOrderMargin, secondBuyerOrderQuantity, secondBuyerOrderPrice sdk.Dec

				BeforeEach(func() {
					liquidatedOwnerBankBalance = sdk.NewInt(100) // still missing ~100
				})

				JustBeforeEach(func() {
					secondBuyerOrderMargin = sdk.NewDec(100)
					secondBuyerOrderQuantity = sdk.NewDec(1)
					secondBuyerOrderPrice = newOraclePrice
					limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(secondBuyerOrderPrice, secondBuyerOrderQuantity, secondBuyerOrderMargin, types.OrderType_BUY, buyer)
					_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
					testexchange.OrFail(err)

					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := initialCoinDeposit.Sub(math.Int(liquidatedPositionMargin.Ceil().TruncateInt())).Sub(math.Int(tradingFee.Ceil().TruncateInt()))
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerBankBalance)
					app.BankKeeper.SendCoinsFromAccountToModule(ctx, buyerAccount, auctiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(quoteDenom, withdrawAmount)))

					// since end blocker did not run yet order margin was not subtracted yet from these funds
					buyerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					// TODO not sure why it fails if we take the balance snapshot here, just like with non-default tests
					// exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				It("should take funds from insurance fund, send freed margin to bank and charge the user only the dust on subaccount without touching her bank balances", func() {
					liquidateMsg := testInput.NewMsgLiquidatePositionForMarketID(buyer, liquidator, testInput.Perps[0].MarketID)
					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					// there should still be 2 positions open
					existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(existingPositions)).Should(Equal(2), "incorrect number of positions")

					// market should still be active
					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active), "market wasn't active")

					// owner of the position with negative payout should keep his bank funds intact and only lose dust from available balance
					buyerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					Expect(buyerSpendableFundsAfter.AvailableBalance.String()).To(Equal(buyerSpendableFundsAfter.AvailableBalance.TruncateInt().ToDec().String()), "wrong buyer's spendable balance")

					// balances of the counterparty should stay the same (position shouldn't have been closed)
					sellerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
					Expect(sellerSpendableFundsAfter.AvailableBalance.String()).To(Equal(sellerSpendableFundsBefore.AvailableBalance.String()), "wrong seller's spendable balance")

					// market maker should have paid the fee and margin
					marketMakerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
					totalMarketMakerDiff := marketMakerSpendableFundsAfter.TotalBalance.Sub(marketMakerSpendableFundsBefore.TotalBalance)
					marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
					marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)
					marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
					Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()), "market maker total balance did not change accordingly")

					// liquidator didn't get anything due to negative payout
					liquidatorSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
					Expect(liquidatorSpendableFundsAfter.AvailableBalance.String()).To(Equal(liquidatorSpendableFundsBefore.AvailableBalance.String()), "wrong liquidator's spendable balance")
					Expect(liquidatorSpendableFundsAfter.TotalBalance.String()).To(Equal(liquidatorSpendableFundsBefore.TotalBalance.String()), "wrong liquidator's spendable balance")

					// insurance's balance should have decresed by missing funds minus dust taken from position's owner
					insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
					insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
					dustChargedFromBuyer := buyerSpendableFundsBefore.AvailableBalance.Sub(buyerSpendableFundsBefore.AvailableBalance.TruncateDec())
					expectedInsuranceWithdrawal := sdk.MaxDec(sdk.ZeroDec(), missingFunds.Sub(dustChargedFromBuyer))

					Expect(insuranceBalanceDiff.String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().Neg().RoundInt().String()), "insurance fund balance did not change accordingly")

					// amount taken from insurance fund should have gone to exchange and fee recipient
					feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)
					feeRecipientDiff := feeRecipientBalanceAfter.Amount.Sub(feeRecipientBalanceBefore.Amount)
					exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
					exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)

					Expect(exchangeBalanceDiff.Add(feeRecipientDiff).String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().RoundInt().String()), "exchange and fee recipeint balances were different than insurance withdrawal")
				})
			})

			Describe("with sufficient user locked balances in resting order to cover missing funds", func() {
				var secondBuyerOrderMargin, secondBuyerOrderQuantity, secondBuyerOrderPrice sdk.Dec

				BeforeEach(func() {
					liquidatedOwnerBankBalance = sdk.NewInt(100) // still missing ~100
				})

				JustBeforeEach(func() {
					secondBuyerOrderMargin = sdk.NewDec(100)
					secondBuyerOrderQuantity = sdk.NewDec(1)
					secondBuyerOrderPrice = newOraclePrice
					limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(secondBuyerOrderPrice, secondBuyerOrderQuantity, secondBuyerOrderMargin, types.OrderType_BUY, buyer)
					_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := initialCoinDeposit.Sub(math.Int(liquidatedPositionMargin.Ceil().TruncateInt())).Sub(math.Int(tradingFee.Ceil().TruncateInt()))
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerBankBalance)
					app.BankKeeper.SendCoinsFromAccountToModule(ctx, buyerAccount, auctiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(quoteDenom, withdrawAmount)))

					buyerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					// TODO not sure why it fails if we take the balance snapshot here, just like with non-default tests
					// exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				It("should take funds from insurance fund, send freed margin to bank and charge the user only the dust on subaccount without touching her bank balances", func() {
					liquidateMsg := testInput.NewMsgLiquidatePositionForMarketID(buyer, liquidator, testInput.Perps[0].MarketID)
					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					tradingFee := secondBuyerOrderPrice.Mul(secondBuyerOrderQuantity).Mul(derivativeMarket.MakerFeeRate)
					recooveredBuyerBalanceFromOrders := secondBuyerOrderMargin.Add(tradingFee)

					// there should still be 2 positions open
					existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(existingPositions)).Should(Equal(2), "incorrect number of positions")

					// market should still be active
					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active), "market wasn't active")

					// owner of the position with negative payout should keep his bank funds intact and only lose dust from available balance
					// but he also should receive margin (and fee) of the cancelled resting order
					buyerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					expectedBuyerSpendableFunds := &types.Deposit{
						AvailableBalance: buyerSpendableFundsBefore.AvailableBalance.Add(recooveredBuyerBalanceFromOrders),
						TotalBalance:     buyerSpendableFundsBefore.TotalBalance,
					}
					Expect(buyerSpendableFundsAfter.AvailableBalance.String()).To(Equal(expectedBuyerSpendableFunds.AvailableBalance.TruncateInt().ToDec().String()), "wrong buyer's spendable balance")
					Expect(buyerSpendableFundsAfter.TotalBalance.String()).To(Equal(expectedBuyerSpendableFunds.TotalBalance.TruncateInt().ToDec().String()), "wrong buyer's total balance")

					// balances of the counterparty should stay the same (position shouldn't have been closed)
					sellerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
					Expect(sellerSpendableFundsAfter.AvailableBalance.String()).To(Equal(sellerSpendableFundsBefore.AvailableBalance.String()), "wrong seller's spendable balance")

					// market maker should have paid the fee and margin
					marketMakerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
					totalMarketMakerDiff := marketMakerSpendableFundsAfter.TotalBalance.Sub(marketMakerSpendableFundsBefore.TotalBalance)
					marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
					marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)
					marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
					Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()), "market maker total balance did not change accordingly")

					// liquidator didn't get anything due to negative payout
					liquidatorSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
					Expect(liquidatorSpendableFundsAfter.AvailableBalance.String()).To(Equal(liquidatorSpendableFundsBefore.AvailableBalance.String()), "wrong liquidator's spendable balance")
					Expect(liquidatorSpendableFundsAfter.TotalBalance.String()).To(Equal(liquidatorSpendableFundsBefore.TotalBalance.String()), "wrong liquidator's spendable balance")

					// insurance's balance should have decresed by missing funds minus dust taken from position's owner
					insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
					insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
					dustChargedFromBuyer := buyerSpendableFundsBefore.AvailableBalance.Sub(buyerSpendableFundsBefore.AvailableBalance.TruncateDec())
					expectedInsuranceWithdrawal := sdk.MaxDec(sdk.ZeroDec(), missingFunds.Sub(dustChargedFromBuyer))

					Expect(insuranceBalanceDiff.String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().Neg().RoundInt().String()), "insurance fund balance did not change accordingly")

					// amount taken from insurance fund should have gone to exchange and fee recipient
					feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)
					feeRecipientDiff := feeRecipientBalanceAfter.Amount.Sub(feeRecipientBalanceBefore.Amount)
					exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
					exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)

					Expect(exchangeBalanceDiff.Add(feeRecipientDiff).String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().RoundInt().String()), "exchange and fee recipeint balances were different than insurance withdrawal")
				})
			})

			Describe("with sufficient balances in insurance fund to cover missing funds and zero bank balance", func() {
				BeforeEach(func() {
					liquidatedOwnerBankBalance = sdk.NewInt(0) // still missing ~100
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := initialCoinDeposit.Sub(math.Int(liquidatedPositionMargin.Ceil().TruncateInt())).Sub(math.Int(tradingFee.Ceil().TruncateInt()))
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerBankBalance)
					app.BankKeeper.SendCoinsFromAccountToModule(ctx, buyerAccount, auctiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(quoteDenom, withdrawAmount)))

					buyerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
				})

				It("should take funds from insurance fund and charge the user only the dust on subaccount", func() {
					liquidateMsg := testInput.NewMsgLiquidatePositionForMarketID(buyer, liquidator, testInput.Perps[0].MarketID)
					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					// there should still be 2 positions open
					existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(existingPositions)).Should(Equal(2), "incorrect number of positions")

					// market should still be active
					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active), "market wasn't active")

					// owner of the position with negative payout should keep his bank funds intact and only lose dust from available balance
					buyerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					Expect(buyerSpendableFundsAfter.AvailableBalance.String()).To(Equal(buyerSpendableFundsBefore.AvailableBalance.TruncateInt().ToDec().String()), "wrong buyer's spendable balance")
					Expect(buyerSpendableFundsAfter.TotalBalance.String()).To(Equal(buyerSpendableFundsBefore.TotalBalance.TruncateInt().ToDec().String()), "wrong buyer's total balance")

					// balances of the counterparty should stay the same (position shouldn't have been closed)
					sellerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
					Expect(sellerSpendableFundsAfter.AvailableBalance.String()).To(Equal(sellerSpendableFundsBefore.AvailableBalance.String()), "wrong seller's spendable balance")

					// market maker should have paid the fee and margin
					marketMakerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
					totalMarketMakerDiff := marketMakerSpendableFundsAfter.TotalBalance.Sub(marketMakerSpendableFundsBefore.TotalBalance)
					marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
					marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)
					marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
					Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()), "market maker total balance did not change accordingly")

					// liquidator didn't get anything due to negative payout
					liquidatorSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
					Expect(liquidatorSpendableFundsAfter.AvailableBalance.String()).To(Equal(liquidatorSpendableFundsBefore.AvailableBalance.String()), "wrong liquidator's spendable balance")
					Expect(liquidatorSpendableFundsAfter.TotalBalance.String()).To(Equal(liquidatorSpendableFundsBefore.TotalBalance.String()), "wrong liquidator's spendable balance")

					// insurance's balance should have decresed by missing funds minus dust taken from position's owner
					insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
					insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
					dustChargedFromBuyer := buyerSpendableFundsBefore.AvailableBalance.Sub(buyerSpendableFundsBefore.AvailableBalance.TruncateDec())
					expectedInsuranceWithdrawal := sdk.MaxDec(sdk.ZeroDec(), missingFunds.Sub(dustChargedFromBuyer))

					Expect(insuranceBalanceDiff.String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().Neg().RoundInt().String()), "insurance fund balance did not change accordingly")

					// amount taken from insurance fund should have gone to exchange and fee recipient
					feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)
					feeRecipientDiff := feeRecipientBalanceAfter.Amount.Sub(feeRecipientBalanceBefore.Amount)
					exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
					exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)

					Expect(exchangeBalanceDiff.Add(feeRecipientDiff).String()).Should(Equal(expectedInsuranceWithdrawal.Ceil().RoundInt().String()), "exchange and fee recipeint balances were different than insurance withdrawal")
				})
			})

			Describe("with insufficient balances to cover missing funds", func() {
				BeforeEach(func() {
					liquidatedPositionQuantity = sdk.NewDec(4) // so that insurance fund gets depleted
					liquidatedOwnerBankBalance = sdk.NewInt(0) // still missing ~200
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := initialCoinDeposit.Sub(math.Int(liquidatedPositionMargin.Ceil().TruncateInt())).Sub(math.Int(tradingFee.Ceil().TruncateInt()))
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerBankBalance)
					app.BankKeeper.SendCoinsFromAccountToModule(ctx, buyerAccount, auctiontypes.ModuleName, sdk.NewCoins(sdk.NewCoin(quoteDenom, withdrawAmount)))

					buyerSpendableFundsBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					// exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				})

				It("should liquidate correctly and pause the market", func() {
					beforeLiqPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(beforeLiqPositions)).Should(Equal(2), "incorrect number of positions")

					liquidateMsg := testInput.NewMsgLiquidatePositionForMarketID(buyer, liquidator, testInput.Perps[0].MarketID)
					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					// there should be 0 positions open
					existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
					Expect(len(existingPositions)).Should(Equal(0), "incorrect number of positions")

					// market should still be paused
					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Paused), "market wasn't paused")

					// owner of the position with negative payout should keep his bank funds intact and only lose dust from available balance
					buyerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
					Expect(buyerSpendableFundsAfter.AvailableBalance.String()).To(Equal(buyerSpendableFundsBefore.AvailableBalance.String()), "wrong buyer's spendable balance")
					Expect(buyerSpendableFundsAfter.TotalBalance.String()).To(Equal(buyerSpendableFundsBefore.TotalBalance.String()), "wrong buyer's total balance")

					// balances of the counterparty should increase (position was closed)
					expectedAvailableSellerDiff := sdk.NewDec(2202) // 2x margin + insurance fund
					sellerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
					availableSellerDiff := sellerSpendableFundsAfter.AvailableBalance.Sub(sellerSpendableFundsBefore.AvailableBalance)
					totalSellerDiff := sellerSpendableFundsAfter.TotalBalance.Sub(sellerSpendableFundsBefore.TotalBalance)
					Expect(availableSellerDiff.String()).To(Equal(expectedAvailableSellerDiff.String()), "wrong seller's spendable balance")
					Expect(totalSellerDiff.String()).To(Equal(expectedAvailableSellerDiff.String()), "wrong seller's total spendable balance")

					// market maker shouldn't have gotten anything
					marketMakerSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
					totalMarketMakerDiff := marketMakerSpendableFundsAfter.TotalBalance.Sub(marketMakerSpendableFundsBefore.TotalBalance)
					Expect(totalMarketMakerDiff.String()).Should(Equal(f2d(0).String()), "market maker total balance did not change accordingly")

					// liquidator didn't get anything due to negative payout
					liquidatorSpendableFundsAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
					Expect(liquidatorSpendableFundsAfter.AvailableBalance.String()).To(Equal(liquidatorSpendableFundsBefore.AvailableBalance.String()), "wrong liquidator's spendable balance")
					Expect(liquidatorSpendableFundsAfter.TotalBalance.String()).To(Equal(liquidatorSpendableFundsBefore.TotalBalance.String()), "wrong liquidator's spendable balance")

					// insurance's balance should have decresed by missing funds minus dust taken from position's owner
					insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
					insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
					Expect(insuranceBalanceDiff.String()).Should(Equal(initialInsuranceFundBalance.TruncateInt().Neg().String()), "insurance fund balance did not change accordingly")

					// amount taken from insurance fund should have gone to exchange and fee recipient
					feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)
					feeRecipientDiff := feeRecipientBalanceAfter.Amount.Sub(feeRecipientBalanceBefore.Amount)
					exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)

					Expect(feeRecipientDiff.IsZero()).To(BeTrue())
					// exchange balance contains only auction subaccount (~28) + 3 sum of dust. Hardcoded as it's hard and pointless to calculate it, leaving a check for possible regression detection
					Expect(exchangeBalanceAfter.Amount.String()).Should(Equal("31"), "exchange and fee recipeint balances were different than insurance withdrawal")
				})
			})

			//Describe("with positive liquidation payout", func() {
			//
			//	var expectCorrectPositiveLiquidationPayout = func(isUsingMarketMakerOrder bool) {
			//		buyerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
			//		sellerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
			//		marketMakerDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
			//		liquidatorDepositAfter := testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
			//
			//		liquidatorBalanceAfter := app.BankKeeper.GetBalance(ctx, liquidatorAccount, quoteDenom).Amount
			//		feeRecipientBalanceAfter := app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom).Amount
			//		exchangeBalanceAfter := app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
			//		insuranceBalanceAfter := app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)
			//
			//		availableBuyerDiff := buyerDepositAfter.AvailableBalance.Sub(buyerDepositBefore.AvailableBalance)
			//		totalBuyerDiff := buyerDepositAfter.TotalBalance.Sub(buyerDepositBefore.TotalBalance)
			//		availableSellerDiff := sellerDepositAfter.AvailableBalance.Sub(sellerDepositBefore.AvailableBalance)
			//		totalSellerDiff := sellerDepositAfter.TotalBalance.Sub(sellerDepositBefore.TotalBalance)
			//		availableMarketMakerDiff := marketMakerDepositAfter.AvailableBalance.Sub(marketMakerDepositBefore.AvailableBalance)
			//		totalMarketMakerDiff := marketMakerDepositAfter.TotalBalance.Sub(marketMakerDepositBefore.TotalBalance)
			//		availableLiquidatorDiff := liquidatorDepositAfter.AvailableBalance.Sub(liquidatorDepositBefore.AvailableBalance)
			//
			//		totalLiquidatorDiff := liquidatorDepositAfter.TotalBalance.Sub(liquidatorDepositBefore.TotalBalance)
			//		bankLiquidatorDiff := liquidatorBalanceAfter.Sub(liquidatorBalanceBefore)
			//		feeRecipientDiff := feeRecipientBalanceAfter.Sub(feeRecipientBalanceBefore)
			//		exchangeBalanceDiff := exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)
			//		insuranceBalanceDiff := insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
			//
			//		Expect(availableBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			//		Expect(totalBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			//		Expect(availableSellerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			//		Expect(totalSellerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			//
			//		Expect(availableMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			//		marketMakerPositionEntryPrice := liquidatedPositionPrice.Sub(marketMakerDiff)
			//		marketMakerPositionMargin := marketMakerMargin.Quo(marketMakerQuantity).Mul(liquidatedPositionQuantity)
			//
			//		var expectedLiquidatorPayout, expectedInsuranceFundPayout, liquidatorPositionMargin sdk.Dec
			//
			//		if isUsingMarketMakerOrder {
			//			marketMakerTradingFee := liquidatedPositionQuantity.Mul(marketMakerPositionEntryPrice).Mul(derivativeMarket.MakerFeeRate)
			//			Expect(totalMarketMakerDiff.String()).Should(Equal(marketMakerPositionMargin.Add(marketMakerTradingFee).Neg().String()))
			//
			//			pnlNotionalLiquidatedPosition := liquidatedPositionQuantity.Mul(liquidatedPositionPrice.Sub(marketMakerPositionEntryPrice))
			//			expectedPayout := liquidatedPositionMargin.Sub(pnlNotionalLiquidatedPosition)
			//
			//			liquidatorRewardShareRate := app.ExchangeKeeper.GetLiquidatorRewardShareRate(ctx)
			//			expectedLiquidatorPayout = expectedPayout.Mul(liquidatorRewardShareRate)
			//			expectedInsuranceFundPayout = expectedPayout.Sub(expectedLiquidatorPayout)
			//			Expect(availableLiquidatorDiff.Add(bankLiquidatorDiff.ToDec()).String()).Should(Equal(expectedLiquidatorPayout.String()))
			//			Expect(totalLiquidatorDiff.Add(bankLiquidatorDiff.ToDec()).String()).Should(Equal(expectedLiquidatorPayout.String()))
			//		} else {
			//			Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			//
			//			liquidatorPositionMargin = liquidationOrderMargin.Quo(liquidationOrderQuantity).Mul(liquidatedPositionQuantity)
			//			liquidatorTradingFee := liquidatedPositionQuantity.Mul(liquidationOrderPrice).Mul(derivativeMarket.MakerFeeRate)
			//			pnlNotionalLiquidatedPosition := liquidatedPositionQuantity.Mul(liquidatedPositionPrice.Sub(liquidationOrderPrice))
			//			expectedPayout := liquidatedPositionMargin.Sub(pnlNotionalLiquidatedPosition)
			//			liquidatorRewardShareRate := app.ExchangeKeeper.GetLiquidatorRewardShareRate(ctx)
			//			expectedLiquidatorPayout = expectedPayout.Mul(liquidatorRewardShareRate)
			//			expectedInsuranceFundPayout = expectedPayout.Sub(expectedLiquidatorPayout)
			//			expectedLiquidatorPositionBalanceDiff := liquidatorPositionMargin.Add(liquidatorTradingFee).Neg()
			//			expectedLiquidatorBalanceDiff := expectedLiquidatorPayout.Add(expectedLiquidatorPositionBalanceDiff)
			//
			//			Expect(bankLiquidatorDiff.ToDec().Add(availableLiquidatorDiff).String()).Should(Equal(expectedLiquidatorBalanceDiff.String()))
			//		}
			//
			//		Expect(sdk.NewDecFromInt(exchangeBalanceDiff.Add(bankLiquidatorDiff).Add(feeRecipientDiff)).String()).Should(Equal(expectedInsuranceFundPayout.Neg().String()))
			//		Expect(sdk.NewDecFromInt(insuranceBalanceDiff).String()).Should(Equal(expectedInsuranceFundPayout.String()))
			//
			//		existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.Perps[0].MarketID)
			//
			//		for _, existingPosition := range existingPositions {
			//			Expect(existingPosition.MarketId).Should(Equal(derivativeMarket.MarketId))
			//
			//			switch existingPosition.SubaccountId {
			//			case buyer.Hex():
			//				Fail("Liquidated position was not removed as expected")
			//			case seller.Hex():
			//				Expect(existingPosition.Position.EntryPrice.String()).Should(Equal(liquidatedPositionPrice.String()))
			//				Expect(existingPosition.Position.IsLong).Should(BeFalse())
			//				Expect(existingPosition.Position.Margin.String()).Should(Equal(liquidatedPositionMargin.String()))
			//				Expect(existingPosition.Position.Quantity.String()).Should(Equal(liquidatedPositionQuantity.String()))
			//				Expect(existingPosition.Position.CumulativeFundingEntry.String()).Should(Equal(sdk.ZeroDec().String()))
			//			case marketMaker.Hex():
			//				if !isUsingMarketMakerOrder {
			//					Fail("Position with unexpected subaccount id encountered")
			//				}
			//
			//				Expect(existingPosition.Position.EntryPrice.String()).Should(Equal(marketMakerPositionEntryPrice.String()))
			//				Expect(existingPosition.Position.IsLong).Should(BeTrue())
			//				Expect(existingPosition.Position.Margin.String()).Should(
			//					Equal(marketMakerPositionMargin.String()),
			//				)
			//				Expect(existingPosition.Position.Quantity.String()).Should(Equal(liquidatedPositionQuantity.String()))
			//				Expect(existingPosition.Position.CumulativeFundingEntry.String()).Should(Equal(sdk.ZeroDec().String()))
			//			case liquidator.Hex():
			//				if isUsingMarketMakerOrder {
			//					Fail("Position with unexpected subaccount id encountered")
			//				}
			//
			//				Expect(existingPosition.Position.EntryPrice.String()).Should(Equal(liquidationOrderPrice.String()))
			//				Expect(existingPosition.Position.IsLong).Should(BeTrue())
			//				Expect(existingPosition.Position.Margin.String()).Should(Equal(liquidatorPositionMargin.String()))
			//				Expect(existingPosition.Position.Quantity.String()).Should(Equal(liquidatedPositionQuantity.String()))
			//				Expect(existingPosition.Position.CumulativeFundingEntry.String()).Should(Equal(sdk.ZeroDec().String()))
			//			default:
			//				Fail("Position with unexpected subaccount id encountered")
			//			}
			//		}
			//
			//		Expect(len(existingPositions)).Should(Equal(2))
			//
			//		limitBuyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, true)
			//		limitSellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, testInput.Perps[0].MarketID, false)
			//		Expect(len(limitBuyOrders)).Should(Equal(1))
			//		Expect(len(limitSellOrders)).Should(Equal(1))
			//
			//		marketMakerBuyOrder := limitBuyOrders[0]
			//		marketMakerSellOrder := limitSellOrders[0]
			//		Expect(marketMakerSellOrder.Margin.String()).Should(Equal(marketMakerMargin.String()))
			//		Expect(marketMakerSellOrder.OrderInfo.Price.String()).Should(Equal(liquidatedPositionPrice.Add(marketMakerDiff).String()))
			//		Expect(marketMakerSellOrder.OrderInfo.Quantity.String()).Should(Equal(marketMakerQuantity.String()))
			//		Expect(marketMakerSellOrder.OrderInfo.SubaccountId).Should(Equal(marketMaker.Hex()))
			//		Expect(marketMakerSellOrder.Fillable.String()).Should(Equal(marketMakerQuantity.String()))
			//
			//		Expect(marketMakerBuyOrder.Margin.String()).Should(Equal(marketMakerMargin.String()))
			//		Expect(marketMakerBuyOrder.OrderInfo.Price.String()).Should(Equal(liquidatedPositionPrice.Sub(marketMakerDiff).String()))
			//		Expect(marketMakerBuyOrder.OrderInfo.Quantity.String()).Should(Equal(marketMakerQuantity.String()))
			//		Expect(marketMakerBuyOrder.OrderInfo.SubaccountId).Should(Equal(marketMaker.Hex()))
			//
			//		if isUsingMarketMakerOrder {
			//			Expect(marketMakerBuyOrder.Fillable.String()).Should(Equal(marketMakerQuantity.Sub(liquidatedPositionQuantity).String()))
			//		} else {
			//			Expect(marketMakerBuyOrder.Fillable.String()).Should(Equal(marketMakerQuantity.String()))
			//		}
			//
			//		markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
			//		Expect(markets[0].Status).Should(Equal(types.MarketStatus_Active))
			//	}
			//
			//	BeforeEach(func() {
			//		liquidatedPositionMargin = sdk.NewDec(500)
			//		liquidatedPositionPrice = sdk.NewDec(2010)
			//		liquidatedPositionQuantity = sdk.NewDec(2)
			//		marketMakerDiff = sdk.NewDec(200)
			//		newOraclePrice = sdk.NewDec(1795)
			//	})
			//
			//	Describe("when there is not enough liquidity in orderbook", func() {
			//		BeforeEach(func() {
			//			isMarketMaking = false
			//		})
			//
			//		It("should reject liquidation", func() {
			//			liquidateMsg := testInput.NewMsgLiquidatePosition(buyer)
			//
			//			_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
			//			expectedError := "no liquidity on the orderbook!"
			//			Expect(err.Error()).To(Equal(expectedError))
			//		})
			//	})
			//
			//	Describe("when there is enough liquidity in orderbook", func() {
			//		Describe("without providing order", func() {
			//			It("should liquidate correctly", func() {
			//				liquidateMsg := testInput.NewMsgLiquidatePositionWithOrder(buyer, liquidator, types.OrderType_UNSPECIFIED, sdk.Dec{}, sdk.Dec{}, sdk.Dec{})
			//
			//				_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
			//				testexchange.OrFail(err)
			//
			//				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			//				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			//
			//				isUsingMarketMakerOrder := true
			//				expectCorrectPositiveLiquidationPayout(isUsingMarketMakerOrder)
			//			})
			//		})
			//	})
			//
			//	// 		Describe("when providing an order", func() {
			//	// 			It("should liquidate correctly using this order in liquidity", func() {
			//	// 				liquidationOrderPrice = liquidatedPositionPrice.Add(marketMakerDiff)
			//	// 				liquidationOrderQuantity = sdk.NewDec(3)
			//	// 				liquidationOrderMargin = sdk.NewDec(6300)
			//
			//	// 				liquidateMsg := testInput.NewMsgLiquidatePositionWithOrder(
			//	// 					buyer,
			//	// 					liquidator,
			//	// 					types.OrderType_BUY,
			//	// 					liquidationOrderPrice,
			//	// 					liquidationOrderQuantity,
			//	// 					liquidationOrderMargin,
			//	// 				)
			//
			//	// 				_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
			//	// 				testexchange.OrFail(err)
			//	// 				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			//	// 				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			//
			//	// 				isUsingMarketMakerOrder := false
			//	// 				expectCorrectPositiveLiquidationPayout(isUsingMarketMakerOrder)
			//	// 			})
			//})
		})
	})
})
