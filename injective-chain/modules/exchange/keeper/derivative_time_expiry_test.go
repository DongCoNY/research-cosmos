package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	oraclekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/keeper"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Time expiry market tests: non-default subaccounts", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		err              error

		buyer       = testexchange.SampleNonDefaultSubaccountAddr1
		seller      = testexchange.SampleNonDefaultSubaccountAddr2
		marketMaker = testexchange.SampleNonDefaultSubaccountAddr3
		liquidator  = testexchange.SampleNonDefaultSubaccountAddr4

		feeRecipient  = sdk.MustAccAddressFromBech32(testexchange.DefaultFeeRecipientAddress)
		startingPrice = sdk.NewDec(2000)
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote, testInput.ExpiryMarkets[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		initialInsuranceFundBalance = sdk.NewDec(44)

		coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

		err = app.InsuranceKeeper.CreateInsuranceFund(
			ctx,
			sender,
			coin,
			testInput.ExpiryMarkets[0].Ticker,
			testInput.ExpiryMarkets[0].QuoteDenom,
			oracleBase,
			oracleQuote,
			oracleType,
			testInput.ExpiryMarkets[0].Expiry,
		)
		testexchange.OrFail(err)

		_, _, err := app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
			ctx,
			testInput.ExpiryMarkets[0].Ticker,
			testInput.ExpiryMarkets[0].QuoteDenom,
			oracleBase,
			oracleQuote,
			0,
			oracleType,
			testInput.ExpiryMarkets[0].Expiry,
			testInput.ExpiryMarkets[0].InitialMarginRatio,
			testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
			testInput.ExpiryMarkets[0].MakerFeeRate,
			testInput.ExpiryMarkets[0].TakerFeeRate,
			testInput.ExpiryMarkets[0].MinPriceTickSize,
			testInput.ExpiryMarkets[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)
		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.ExpiryMarkets[0].MarketID, true)
	})

	Describe("Launching BandIBC oracle perpetual market when original Band oracle market exists", func() {
		It("should fail", func() {
			oracleBase, oracleQuote := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote
			oracleType := oracletypes.OracleType_Band
			app.OracleKeeper.SetBandPriceState(ctx, oracleQuote, &oracletypes.BandPriceState{
				Symbol:      oracleQuote,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})
			app.OracleKeeper.SetBandPriceState(ctx, oracleBase, &oracletypes.BandPriceState{
				Symbol:      oracleBase,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			initialInsuranceFundBalance = sdk.NewDec(44)

			coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

			err = app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				coin,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
			)
			testexchange.OrFail(err)

			_, _, err := app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
				ctx,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
				testInput.ExpiryMarkets[0].InitialMarginRatio,
				testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
				testInput.ExpiryMarkets[0].MakerFeeRate,
				testInput.ExpiryMarkets[0].TakerFeeRate,
				testInput.ExpiryMarkets[0].MinPriceTickSize,
				testInput.ExpiryMarkets[0].MinQuantityTickSize,
			)
			testexchange.OrFail(err)
			oracleType = oracletypes.OracleType_BandIBC
			_, _, err = app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
				ctx,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
				testInput.ExpiryMarkets[0].InitialMarginRatio,
				testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
				testInput.ExpiryMarkets[0].MakerFeeRate,
				testInput.ExpiryMarkets[0].TakerFeeRate,
				testInput.ExpiryMarkets[0].MinPriceTickSize,
				testInput.ExpiryMarkets[0].MinQuantityTickSize,
			)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("with a promoted Band IBC oracle already exists ticker"))
		})
	})

	Describe("When liquidating a position", func() {
		var (
			liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, liquidatedOwnerAvailableBalance   sdk.Dec
			marketMakerMargin, marketMakerQuantity, marketMakerDiff                                                          sdk.Dec
			newOraclePrice, missingFunds                                                                                     sdk.Dec
			auctionDepositBefore, buyerDepositBefore, sellerDepositBefore, marketMakerDepositBefore, liquidatorDepositBefore *types.Deposit
			exchangeBalanceBefore, insuranceBalanceBefore, feeRecipientBefore                                                sdk.Coin
			exchangeAddress, insuranceAddress                                                                                sdk.AccAddress
			initialCoinDeposit                                                                                               math.Int
		)

		var expectCorrectNegativeLiquidationPayout = func(addedBuyerAvailableBalanceFromOrders, addedInsuranceDeposits sdk.Dec) {
			var (
				quoteDenom = testInput.ExpiryMarkets[0].QuoteDenom

				// deposit after
				auctionDepositAfter = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, quoteDenom)

				buyerDepositAfter       = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
				sellerDepositAfter      = testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
				marketMakerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
				liquidatorDepositAfter  = testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)

				// balance after
				exchangeBalanceAfter  = app.BankKeeper.GetBalance(ctx, exchangeAddress, quoteDenom)
				feeRecipientAfter     = app.BankKeeper.GetBalance(ctx, feeRecipient, quoteDenom)
				insuranceBalanceAfter = app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)

				// deposit diff
				availableBuyerDiff      = buyerDepositAfter.AvailableBalance.Sub(buyerDepositBefore.AvailableBalance)
				totalBuyerDiff          = buyerDepositAfter.TotalBalance.Sub(buyerDepositBefore.TotalBalance)
				availableSellerDiff     = sellerDepositAfter.AvailableBalance.Sub(sellerDepositBefore.AvailableBalance)
				totalSellerDiff         = sellerDepositAfter.TotalBalance.Sub(sellerDepositBefore.TotalBalance)
				totalMarketMakerDiff    = marketMakerDepositAfter.TotalBalance.Sub(marketMakerDepositBefore.TotalBalance)
				availableLiquidatorDiff = liquidatorDepositAfter.AvailableBalance.Sub(liquidatorDepositBefore.AvailableBalance)
				totalLiquidatorDiff     = liquidatorDepositAfter.TotalBalance.Sub(liquidatorDepositBefore.TotalBalance)

				// balance diff
				exchangeBalanceDiff  = exchangeBalanceAfter.Amount.Sub(exchangeBalanceBefore.Amount)
				feeRecipientDiff     = feeRecipientAfter.Amount.Sub(feeRecipientBefore.Amount)
				insuranceBalanceDiff = insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
			)

			sellerPnlNotional := liquidatedPositionQuantity.Mul(newOraclePrice.Sub(liquidatedPositionPrice)).Neg()
			sellerPayout := sellerPnlNotional.Add(liquidatedPositionMargin)

			hasLiquidatedMarket := missingFunds.GT(liquidatedOwnerAvailableBalance.Add(addedBuyerAvailableBalanceFromOrders).Add(addedInsuranceDeposits).Add(initialInsuranceFundBalance))
			existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.ExpiryMarkets[0].MarketID)

			if hasLiquidatedMarket {
				Expect(availableBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(totalBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				deficitAmountAfterInsuranceFunds := sdk.NewDec(176)
				expectedSellerPnlPayout := sdk.NewDec(1220)
				expectedAvailableSellerDiff := expectedSellerPnlPayout.Sub(expectedSellerPnlPayout.Mul(deficitAmountAfterInsuranceFunds).Quo(expectedSellerPnlPayout)).Add(liquidatedPositionMargin)
				expectedTotalSellerDiff := expectedAvailableSellerDiff

				Expect(availableSellerDiff.String()).Should(Equal(expectedAvailableSellerDiff.String()))
				Expect(totalSellerDiff.String()).Should(Equal(expectedTotalSellerDiff.String()))

				Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(marketMakerDepositAfter.AvailableBalance.String()).Should(Equal(marketMakerDepositAfter.TotalBalance.String()))

				Expect(exchangeBalanceDiff.Add(feeRecipientDiff).String()).Should(Equal(initialInsuranceFundBalance.RoundInt().String()))
				Expect(insuranceBalanceDiff.String()).Should(Equal(initialInsuranceFundBalance.Neg().RoundInt().String()))
				Expect(len(existingPositions)).Should(Equal(0))

				Expect(auctionDepositAfter.AvailableBalance.String()).Should(Equal(auctionDepositBefore.AvailableBalance.String()))
				Expect(auctionDepositAfter.TotalBalance.String()).Should(Equal(auctionDepositBefore.TotalBalance.String()))
			} else {
				buyerPnl := liquidatedPositionPrice.Sub(newOraclePrice).Mul(liquidatedPositionQuantity).Neg()
				buyerClosingFee := liquidatedPositionQuantity.Mul(newOraclePrice).Mul(derivativeMarket.TakerFeeRate)
				expectedBuyerDiff := sdk.MaxDec(sdk.ZeroDec(), liquidatedPositionMargin.Add(buyerPnl).Sub(buyerClosingFee))

				Expect(availableBuyerDiff.String()).Should(Equal(expectedBuyerDiff.String()))
				Expect(totalBuyerDiff.String()).Should(Equal(expectedBuyerDiff.String()))

				sellerClosingFee := liquidatedPositionQuantity.Mul(newOraclePrice).Mul(derivativeMarket.TakerFeeRate)
				expectedSellerDiff := sellerPayout.Sub(sellerClosingFee)
				Expect(availableSellerDiff.String()).Should(Equal(expectedSellerDiff.String()))
				Expect(totalSellerDiff.String()).Should(Equal(expectedSellerDiff.String()))

				Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				Expect(exchangeBalanceDiff.String()).Should(Equal(missingFunds.RoundInt().String()))
				Expect(insuranceBalanceDiff.String()).Should(Equal(missingFunds.Neg().RoundInt().String()))
				Expect(len(existingPositions)).Should(Equal(0))

				Expect(auctionDepositAfter.AvailableBalance.String()).Should(Equal(auctionDepositBefore.AvailableBalance.Add(buyerClosingFee).Add(sellerClosingFee).String()))
				Expect(auctionDepositAfter.TotalBalance.String()).Should(Equal(auctionDepositBefore.TotalBalance.Add(buyerClosingFee).Add(sellerClosingFee).String()))
			}

			Expect(availableLiquidatorDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			Expect(totalLiquidatorDiff.String()).Should(Equal(sdk.ZeroDec().String()))
		}

		JustBeforeEach(func() {
			limitDerivativeBuyOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, types.OrderType_SELL, seller)

			exchangeAddress = app.AccountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()
			insuranceAddress = app.AccountKeeper.GetModuleAccount(ctx, insurancetypes.ModuleName).GetAddress()

			initialCoinDeposit = sdk.NewInt(200000)
			coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialCoinDeposit)

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, marketMaker.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, liquidator.String(), sdk.NewCoins(coin))

			marketMakerQuantity = sdk.NewDec(200)
			marketMakerMargin = sdk.NewDec(27000)
			bigLimitDerivativeBuyOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice.Sub(marketMakerDiff), marketMakerQuantity, marketMakerMargin, types.OrderType_BUY, marketMaker)
			bigLimitDerivativeSellOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice.Add(marketMakerDiff), marketMakerQuantity, marketMakerMargin, types.OrderType_SELL, marketMaker)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeBuyOrder)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeSellOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			oracleBase, oracleQuote, _ := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote, testInput.ExpiryMarkets[0].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(newOraclePrice, ctx.BlockTime().Unix()))

			auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)
			exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, testInput.ExpiryMarkets[0].QuoteDenom)
			insuranceBalanceBefore = app.BankKeeper.GetBalance(ctx, insuranceAddress, testInput.ExpiryMarkets[0].QuoteDenom)

			quoteDenom := testInput.ExpiryMarkets[0].QuoteDenom

			buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
			sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
			marketMakerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
			liquidatorDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
		})

		AfterEach(func() {
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Describe("with negative liquidation payout", func() {
			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)
				marketMakerDiff = sdk.NewDec(600)
				newOraclePrice = sdk.NewDec(1400)
				missingFunds = sdk.NewDec(200)
			})

			Describe("with insufficient user available balance to cover missing funds", func() {
				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("49.94") // so still missing 150.06
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					err = testexchange.RemoveFunds(app, ctx, buyer, sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, withdrawAmount.RoundInt()))
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.ExpiryMarkets[0].QuoteDenom)
					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, testInput.ExpiryMarkets[0].QuoteDenom)
					feeRecipientBefore = app.BankKeeper.GetBalance(ctx, feeRecipient, testInput.ExpiryMarkets[0].QuoteDenom)
				})

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidateExpiryPosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					updatedTime := ctx.BlockTime().Add(time.Second * 3600)
					ctx = ctx.WithBlockTime(time.Time(updatedTime))

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Paused))

					expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
				})
			})
		})

		Describe("with regular settlement", func() {
			Describe("with unchanged oracle price", func() {
				BeforeEach(func() {
					liquidatedPositionMargin = sdk.NewDec(1000)
					liquidatedPositionPrice = sdk.NewDec(2010)
					liquidatedPositionQuantity = sdk.NewDec(2)

					marketMakerDiff = sdk.NewDec(600)
					missingFunds = sdk.NewDec(0)
					newOraclePrice = sdk.NewDec(2000)
				})

				JustBeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.ZeroDec()
				})

				It("should settle correctly", func() {
					expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)
					updatedTimeForTwapStart := time.Unix(testInput.ExpiryMarkets[0].Expiry-int64(testexchange.ThirtyMinutesInSeconds), 0)
					ctx = ctx.WithBlockTime(time.Time(updatedTimeForTwapStart))

					for i := 0; i < 10; i++ {
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
					}

					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, testInput.ExpiryMarkets[0].QuoteDenom)
					auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

					ctx = ctx.WithBlockTime(time.Time(expiryTime))

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

					expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
				})

				Describe("with missing funds upon regular settlement", func() {
					BeforeEach(func() {
						liquidatedPositionMargin = sdk.NewDec(1000)
						liquidatedPositionPrice = sdk.NewDec(2010)
						liquidatedPositionQuantity = sdk.NewDec(2)
						marketMakerDiff = sdk.NewDec(600)
						newOraclePrice = sdk.NewDec(1500)
						missingFunds = sdk.NewDec(29)
					})

					JustBeforeEach(func() {
						liquidatedOwnerAvailableBalance = sdk.ZeroDec()
					})

					It("should settle correctly using insurance fund", func() {
						expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)
						updatedTimeForTwapStart := time.Unix(testInput.ExpiryMarkets[0].Expiry-int64(testexchange.ThirtyMinutesInSeconds), 0)
						ctx = ctx.WithBlockTime(time.Time(updatedTimeForTwapStart))

						for i := 0; i < 10; i++ {
							ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
							exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
						}

						exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, testInput.ExpiryMarkets[0].QuoteDenom)
						auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

						ctx = ctx.WithBlockTime(time.Time(expiryTime))

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
						Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

						expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
					})
				})
			})

			Describe("with changing oracle price", func() {
				BeforeEach(func() {
					liquidatedPositionMargin = sdk.NewDec(1000)
					liquidatedPositionPrice = sdk.NewDec(2010)
					liquidatedPositionQuantity = sdk.NewDec(2)

					marketMakerDiff = sdk.NewDec(600)
					missingFunds = sdk.NewDec(0)
					newOraclePrice = sdk.NewDec(1850)
				})

				JustBeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.ZeroDec()
				})

				It("should settle correctly with TWAP price", func() {
					expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)
					updatedTimeForTwapStart := time.Unix(testInput.ExpiryMarkets[0].Expiry-int64(testexchange.ThirtyMinutesInSeconds), 0)

					// break loop earlier
					const max_iterations = 10_000
					iterations := 0

					for i := 0; i <= max_iterations; i++ {
						advancedTime := time.Time(updatedTimeForTwapStart).Add(time.Minute * time.Duration(i))

						if advancedTime.Unix() >= time.Time(expiryTime).Unix() {
							break
						}

						iterations = i
						ctx = ctx.WithBlockTime(advancedTime)

						sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
						app.OracleKeeper.SetPriceFeedRelayer(ctx, testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote, sender)

						currentOraclePrice := newOraclePrice.Add(sdk.NewDec(int64(i)))
						newOraclePriceFeedMsg := oracletypes.MsgRelayPriceFeedPrice{
							Sender: sender.String(),
							Base:   []string{testInput.ExpiryMarkets[0].OracleBase},
							Quote:  []string{testInput.ExpiryMarkets[0].OracleQuote},
							Price:  []sdk.Dec{currentOraclePrice},
						}
						oracleMsgServer := oraclekeeper.NewMsgServerImpl(app.OracleKeeper)

						_, err := oracleMsgServer.RelayPriceFeedPrice(sdk.WrapSDKContext(ctx), &newOraclePriceFeedMsg)
						testexchange.OrFail(err)

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						if i == max_iterations {
							Fail("TEF tests: too many iterations", 1)
						}
					}

					exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, testInput.ExpiryMarkets[0].QuoteDenom)
					auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

					newOraclePrice = newOraclePrice.Add(sdk.NewDec(int64(iterations)).Quo(sdk.NewDec(2)))

					ctx = ctx.WithBlockTime(time.Time(expiryTime))

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

					expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
				})
			})
		})

		Describe("with a halted chain skipping right through settlement", func() {
			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)

				marketMakerDiff = sdk.NewDec(600)
				missingFunds = sdk.NewDec(0)
				newOraclePrice = sdk.NewDec(2000)
			})

			JustBeforeEach(func() {
				liquidatedOwnerAvailableBalance = sdk.ZeroDec()
			})

			It("should settle correctly", func() {
				expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)

				ctx = ctx.WithBlockTime(time.Time(expiryTime))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				exchangeBalanceBefore = app.BankKeeper.GetBalance(ctx, exchangeAddress, testInput.ExpiryMarkets[0].QuoteDenom)
				auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
				Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

				expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
			})
		})
	})
})

var _ = Describe("Time expiry market tests: default subaccounts", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		err              error

		buyer       = testexchange.SampleDefaultSubaccountAddr1
		seller      = testexchange.SampleDefaultSubaccountAddr2
		marketMaker = testexchange.SampleDefaultSubaccountAddr3
		liquidator  = testexchange.SampleDefaultSubaccountAddr4

		startingPrice = sdk.NewDec(2000)
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote, testInput.ExpiryMarkets[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		initialInsuranceFundBalance = sdk.NewDec(44)

		coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

		err = app.InsuranceKeeper.CreateInsuranceFund(
			ctx,
			sender,
			coin,
			testInput.ExpiryMarkets[0].Ticker,
			testInput.ExpiryMarkets[0].QuoteDenom,
			oracleBase,
			oracleQuote,
			oracleType,
			testInput.ExpiryMarkets[0].Expiry,
		)
		testexchange.OrFail(err)

		_, _, err := app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
			ctx,
			testInput.ExpiryMarkets[0].Ticker,
			testInput.ExpiryMarkets[0].QuoteDenom,
			oracleBase,
			oracleQuote,
			0,
			oracleType,
			testInput.ExpiryMarkets[0].Expiry,
			testInput.ExpiryMarkets[0].InitialMarginRatio,
			testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
			testInput.ExpiryMarkets[0].MakerFeeRate,
			testInput.ExpiryMarkets[0].TakerFeeRate,
			testInput.ExpiryMarkets[0].MinPriceTickSize,
			testInput.ExpiryMarkets[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)
		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.ExpiryMarkets[0].MarketID, true)
	})

	Describe("Launching BandIBC oracle perpetual market when original Band oracle market exists", func() {
		It("should fail", func() {
			oracleBase, oracleQuote := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote
			oracleType := oracletypes.OracleType_Band
			app.OracleKeeper.SetBandPriceState(ctx, oracleQuote, &oracletypes.BandPriceState{
				Symbol:      oracleQuote,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})
			app.OracleKeeper.SetBandPriceState(ctx, oracleBase, &oracletypes.BandPriceState{
				Symbol:      oracleBase,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			initialInsuranceFundBalance = sdk.NewDec(44)

			coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

			err = app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				coin,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
			)
			testexchange.OrFail(err)

			_, _, err := app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
				ctx,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
				testInput.ExpiryMarkets[0].InitialMarginRatio,
				testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
				testInput.ExpiryMarkets[0].MakerFeeRate,
				testInput.ExpiryMarkets[0].TakerFeeRate,
				testInput.ExpiryMarkets[0].MinPriceTickSize,
				testInput.ExpiryMarkets[0].MinQuantityTickSize,
			)
			testexchange.OrFail(err)
			oracleType = oracletypes.OracleType_BandIBC
			_, _, err = app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
				ctx,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
				testInput.ExpiryMarkets[0].InitialMarginRatio,
				testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
				testInput.ExpiryMarkets[0].MakerFeeRate,
				testInput.ExpiryMarkets[0].TakerFeeRate,
				testInput.ExpiryMarkets[0].MinPriceTickSize,
				testInput.ExpiryMarkets[0].MinQuantityTickSize,
			)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("with a promoted Band IBC oracle already exists ticker"))
		})
	})

	Describe("When liquidating a position", func() {
		var (
			liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, liquidatedOwnerAvailableBalance   sdk.Dec
			marketMakerMargin, marketMakerQuantity, marketMakerDiff                                                          sdk.Dec
			newOraclePrice, missingFunds                                                                                     sdk.Dec
			auctionDepositBefore, buyerDepositBefore, sellerDepositBefore, marketMakerDepositBefore, liquidatorDepositBefore *types.Deposit
			insuranceBalanceBefore                                                                                           sdk.Coin
			insuranceAddress                                                                                                 sdk.AccAddress
			initialCoinDeposit                                                                                               math.Int
		)

		var expectCorrectNegativeLiquidationPayout = func(addedBuyerAvailableBalanceFromOrders, addedInsuranceDeposits sdk.Dec) {
			var (
				quoteDenom = testInput.ExpiryMarkets[0].QuoteDenom

				// deposit after
				auctionDepositAfter = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, quoteDenom)

				buyerDepositAfter       = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
				sellerDepositAfter      = testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
				marketMakerDepositAfter = testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
				liquidatorDepositAfter  = testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)

				// balance after
				insuranceBalanceAfter = app.BankKeeper.GetBalance(ctx, insuranceAddress, quoteDenom)

				// deposit diff
				availableBuyerDiff      = buyerDepositAfter.AvailableBalance.Sub(buyerDepositBefore.AvailableBalance)
				totalBuyerDiff          = buyerDepositAfter.TotalBalance.Sub(buyerDepositBefore.TotalBalance)
				availableSellerDiff     = sellerDepositAfter.AvailableBalance.Sub(sellerDepositBefore.AvailableBalance)
				totalSellerDiff         = sellerDepositAfter.TotalBalance.Sub(sellerDepositBefore.TotalBalance)
				totalMarketMakerDiff    = marketMakerDepositAfter.TotalBalance.Sub(marketMakerDepositBefore.TotalBalance)
				availableLiquidatorDiff = liquidatorDepositAfter.AvailableBalance.Sub(liquidatorDepositBefore.AvailableBalance)
				totalLiquidatorDiff     = liquidatorDepositAfter.TotalBalance.Sub(liquidatorDepositBefore.TotalBalance)

				// balance diff
				insuranceBalanceDiff = insuranceBalanceAfter.Amount.Sub(insuranceBalanceBefore.Amount)
			)

			sellerPnlNotional := liquidatedPositionQuantity.Mul(newOraclePrice.Sub(liquidatedPositionPrice)).Neg()
			sellerPayout := sellerPnlNotional.Add(liquidatedPositionMargin)

			hasLiquidatedMarket := missingFunds.GT(liquidatedOwnerAvailableBalance.Add(addedBuyerAvailableBalanceFromOrders).Add(addedInsuranceDeposits).Add(initialInsuranceFundBalance))
			existingPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, testInput.ExpiryMarkets[0].MarketID)

			if hasLiquidatedMarket {
				Expect(availableBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(totalBuyerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				deficitAmountAfterInsuranceFunds := sdk.NewDec(176)
				expectedSellerPnlPayout := sdk.NewDec(1220)
				expectedAvailableSellerDiff := expectedSellerPnlPayout.Sub(expectedSellerPnlPayout.Mul(deficitAmountAfterInsuranceFunds).Quo(expectedSellerPnlPayout)).Add(liquidatedPositionMargin)
				expectedTotalSellerDiff := expectedAvailableSellerDiff

				Expect(availableSellerDiff.String()).Should(Equal(expectedAvailableSellerDiff.String()))
				Expect(totalSellerDiff.String()).Should(Equal(expectedTotalSellerDiff.String()))

				Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(marketMakerDepositAfter.AvailableBalance.String()).Should(Equal(marketMakerDepositAfter.TotalBalance.String()))

				Expect(insuranceBalanceDiff.String()).Should(Equal(initialInsuranceFundBalance.Neg().RoundInt().String()))
				Expect(len(existingPositions)).Should(Equal(0))

				Expect(auctionDepositAfter.AvailableBalance.String()).Should(Equal(auctionDepositBefore.AvailableBalance.String()))
				Expect(auctionDepositAfter.TotalBalance.String()).Should(Equal(auctionDepositBefore.TotalBalance.String()))
			} else {
				buyerPnl := liquidatedPositionPrice.Sub(newOraclePrice).Mul(liquidatedPositionQuantity).Neg()
				buyerClosingFee := liquidatedPositionQuantity.Mul(newOraclePrice).Mul(derivativeMarket.TakerFeeRate)
				expectedBuyerDiff := sdk.MaxDec(sdk.ZeroDec(), liquidatedPositionMargin.Add(buyerPnl).Sub(buyerClosingFee))

				Expect(availableBuyerDiff.String()).Should(Equal(expectedBuyerDiff.String()))
				Expect(totalBuyerDiff.String()).Should(Equal(expectedBuyerDiff.String()))

				sellerClosingFee := liquidatedPositionQuantity.Mul(newOraclePrice).Mul(derivativeMarket.TakerFeeRate)
				expectedSellerDiff := sellerPayout.Sub(sellerClosingFee)
				Expect(availableSellerDiff.String()).Should(Equal(expectedSellerDiff.String()))
				Expect(totalSellerDiff.String()).Should(Equal(expectedSellerDiff.String()))

				Expect(totalMarketMakerDiff.String()).Should(Equal(sdk.ZeroDec().String()))

				Expect(insuranceBalanceDiff.String()).Should(Equal(missingFunds.Neg().RoundInt().String()))
				Expect(len(existingPositions)).Should(Equal(0))

				Expect(auctionDepositAfter.AvailableBalance.String()).Should(Equal(auctionDepositBefore.AvailableBalance.Add(buyerClosingFee).Add(sellerClosingFee).String()))
				Expect(auctionDepositAfter.TotalBalance.String()).Should(Equal(auctionDepositBefore.TotalBalance.Add(buyerClosingFee).Add(sellerClosingFee).String()))
			}

			Expect(availableLiquidatorDiff.String()).Should(Equal(sdk.ZeroDec().String()))
			Expect(totalLiquidatorDiff.String()).Should(Equal(sdk.ZeroDec().String()))
		}

		JustBeforeEach(func() {
			limitDerivativeBuyOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice, liquidatedPositionQuantity, liquidatedPositionMargin, types.OrderType_SELL, seller)

			insuranceAddress = app.AccountKeeper.GetModuleAccount(ctx, insurancetypes.ModuleName).GetAddress()

			initialCoinDeposit = sdk.NewInt(200000)
			coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialCoinDeposit)

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, marketMaker.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, liquidator.String(), sdk.NewCoins(coin))

			marketMakerQuantity = sdk.NewDec(200)
			marketMakerMargin = sdk.NewDec(27000)
			bigLimitDerivativeBuyOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice.Sub(marketMakerDiff), marketMakerQuantity, marketMakerMargin, types.OrderType_BUY, marketMaker)
			bigLimitDerivativeSellOrder := testInput.NewMsgCreateExpiryDerivativeLimitOrder(liquidatedPositionPrice.Add(marketMakerDiff), marketMakerQuantity, marketMakerMargin, types.OrderType_SELL, marketMaker)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeBuyOrder)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeSellOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			oracleBase, oracleQuote, _ := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote, testInput.ExpiryMarkets[0].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(newOraclePrice, ctx.BlockTime().Unix()))

			auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)
			insuranceBalanceBefore = app.BankKeeper.GetBalance(ctx, insuranceAddress, testInput.ExpiryMarkets[0].QuoteDenom)

			quoteDenom := testInput.ExpiryMarkets[0].QuoteDenom

			buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, quoteDenom)
			sellerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, seller, quoteDenom)
			marketMakerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, marketMaker, quoteDenom)
			liquidatorDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, liquidator, quoteDenom)
		})

		AfterEach(func() {
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Describe("with negative liquidation payout", func() {
			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)
				marketMakerDiff = sdk.NewDec(600)
				newOraclePrice = sdk.NewDec(1400)
				missingFunds = sdk.NewDec(200)
			})

			Describe("with insufficient user available balance to cover missing funds", func() {
				BeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.MustNewDecFromStr("49.94") // so still missing 150.06
				})

				JustBeforeEach(func() {
					tradingFee := liquidatedPositionPrice.Mul(liquidatedPositionQuantity).Mul(derivativeMarket.TakerFeeRate)
					buyerLeftBalance := sdk.NewDecFromInt(initialCoinDeposit).Sub(liquidatedPositionMargin).Sub(tradingFee)
					withdrawAmount := buyerLeftBalance.Sub(liquidatedOwnerAvailableBalance)

					err = testexchange.RemoveFunds(app, ctx, buyer, sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, withdrawAmount.RoundInt()))
					testexchange.OrFail(err)

					buyerDepositBefore = testexchange.GetBankAndDepositFunds(app, ctx, buyer, testInput.ExpiryMarkets[0].QuoteDenom)
				})

				It("should liquidate correctly", func() {
					liquidateMsg := testInput.NewMsgLiquidateExpiryPosition(buyer)

					_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					updatedTime := ctx.BlockTime().Add(time.Second * 3600)
					ctx = ctx.WithBlockTime(time.Time(updatedTime))

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Paused))

					expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
				})
			})
		})

		Describe("with regular settlement", func() {
			Describe("with unchanged oracle price", func() {
				BeforeEach(func() {
					liquidatedPositionMargin = sdk.NewDec(1000)
					liquidatedPositionPrice = sdk.NewDec(2010)
					liquidatedPositionQuantity = sdk.NewDec(2)

					marketMakerDiff = sdk.NewDec(600)
					missingFunds = sdk.NewDec(0)
					newOraclePrice = sdk.NewDec(2000)
				})

				JustBeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.ZeroDec()
				})

				It("should settle correctly", func() {
					expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)
					updatedTimeForTwapStart := time.Unix(testInput.ExpiryMarkets[0].Expiry-int64(testexchange.ThirtyMinutesInSeconds), 0)
					ctx = ctx.WithBlockTime(time.Time(updatedTimeForTwapStart))

					for i := 0; i < 10; i++ {
						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
					}

					auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

					ctx = ctx.WithBlockTime(time.Time(expiryTime))

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

					expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
				})

				Describe("with missing funds upon regular settlement", func() {
					BeforeEach(func() {
						liquidatedPositionMargin = sdk.NewDec(1000)
						liquidatedPositionPrice = sdk.NewDec(2010)
						liquidatedPositionQuantity = sdk.NewDec(2)
						marketMakerDiff = sdk.NewDec(600)
						newOraclePrice = sdk.NewDec(1500)
						missingFunds = sdk.NewDec(29)
					})

					JustBeforeEach(func() {
						liquidatedOwnerAvailableBalance = sdk.ZeroDec()
					})

					It("should settle correctly using insurance fund", func() {
						expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)
						updatedTimeForTwapStart := time.Unix(testInput.ExpiryMarkets[0].Expiry-int64(testexchange.ThirtyMinutesInSeconds), 0)
						ctx = ctx.WithBlockTime(time.Time(updatedTimeForTwapStart))

						for i := 0; i < 10; i++ {
							ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
							exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
						}

						auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

						ctx = ctx.WithBlockTime(time.Time(expiryTime))

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
						Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

						expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
					})
				})
			})

			Describe("with changing oracle price", func() {
				BeforeEach(func() {
					liquidatedPositionMargin = sdk.NewDec(1000)
					liquidatedPositionPrice = sdk.NewDec(2010)
					liquidatedPositionQuantity = sdk.NewDec(2)

					marketMakerDiff = sdk.NewDec(600)
					missingFunds = sdk.NewDec(0)
					newOraclePrice = sdk.NewDec(1850)
				})

				JustBeforeEach(func() {
					liquidatedOwnerAvailableBalance = sdk.ZeroDec()
				})

				It("should settle correctly with TWAP price", func() {
					expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)
					updatedTimeForTwapStart := time.Unix(testInput.ExpiryMarkets[0].Expiry-int64(testexchange.ThirtyMinutesInSeconds), 0)

					// break loop earlier
					const max_iterations = 10_000
					iterations := 0

					for i := 0; i <= max_iterations; i++ {
						advancedTime := time.Time(updatedTimeForTwapStart).Add(time.Minute * time.Duration(i))

						if advancedTime.Unix() >= time.Time(expiryTime).Unix() {
							break
						}

						iterations = i
						ctx = ctx.WithBlockTime(advancedTime)

						sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
						app.OracleKeeper.SetPriceFeedRelayer(ctx, testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote, sender)

						currentOraclePrice := newOraclePrice.Add(sdk.NewDec(int64(i)))
						newOraclePriceFeedMsg := oracletypes.MsgRelayPriceFeedPrice{
							Sender: sender.String(),
							Base:   []string{testInput.ExpiryMarkets[0].OracleBase},
							Quote:  []string{testInput.ExpiryMarkets[0].OracleQuote},
							Price:  []sdk.Dec{currentOraclePrice},
						}
						oracleMsgServer := oraclekeeper.NewMsgServerImpl(app.OracleKeeper)

						_, err := oracleMsgServer.RelayPriceFeedPrice(sdk.WrapSDKContext(ctx), &newOraclePriceFeedMsg)
						testexchange.OrFail(err)

						ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
						exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

						if i == max_iterations {
							Fail("TEF tests: too many iterations", 1)
						}
					}

					auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

					newOraclePrice = newOraclePrice.Add(sdk.NewDec(int64(iterations)).Quo(sdk.NewDec(2)))

					ctx = ctx.WithBlockTime(time.Time(expiryTime))

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
					Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

					expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
				})
			})
		})

		Describe("with a halted chain skipping right through settlement", func() {
			BeforeEach(func() {
				liquidatedPositionMargin = sdk.NewDec(1000)
				liquidatedPositionPrice = sdk.NewDec(2010)
				liquidatedPositionQuantity = sdk.NewDec(2)

				marketMakerDiff = sdk.NewDec(600)
				missingFunds = sdk.NewDec(0)
				newOraclePrice = sdk.NewDec(2000)
			})

			JustBeforeEach(func() {
				liquidatedOwnerAvailableBalance = sdk.ZeroDec()
			})

			It("should settle correctly", func() {
				expiryTime := time.Unix(testInput.ExpiryMarkets[0].Expiry, 0)

				ctx = ctx.WithBlockTime(time.Time(expiryTime))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				auctionDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, types.AuctionSubaccountID, testInput.ExpiryMarkets[0].QuoteDenom)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				markets := app.ExchangeKeeper.GetAllDerivativeMarkets(ctx)
				Expect(markets[0].Status).Should(Equal(types.MarketStatus_Expired))

				expectCorrectNegativeLiquidationPayout(sdk.ZeroDec(), sdk.ZeroDec())
			})
		})
	})
})
