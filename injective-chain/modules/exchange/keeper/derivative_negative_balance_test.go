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
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Temporary negative balance on better-priced order match", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec
		msgServer                   types.MsgServer
		err                         error
		seller                      = testexchange.SampleSubaccountAddr1
		buyer                       = testexchange.SampleSubaccountAddr2
		lucas                       = testexchange.SampleSubaccountAddr3
		andrea                      = testexchange.SampleSubaccountAddr4
		marketMaker                 = testexchange.SampleSubaccountAddr5
		liquidator                  = testexchange.SampleSubaccountAddr6
		sellerAccount               = types.SubaccountIDToSdkAddress(seller)
		startingPrice               = sdk.NewDec(25000000)
		quoteDenom                  string

		positionQuantity, positionMargin       sdk.Dec
		marketMakerMargin, marketMakerQuantity sdk.Dec
		newOraclePrice                         sdk.Dec
		sellerDepositBefore                    *types.Deposit
		initialCoinDeposit                     math.Int
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

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		initialInsuranceFundBalance = sdk.NewDec(10000000000)
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
	})

	Describe("When matching derivative sell order at better price", func() {
		sellOrderPrice := sdk.NewDec(20000000)
		buyOrderPrice := sdk.NewDec(21000000)
		newOraclePrice = sdk.NewDec(20000000)
		positionQuantity = sdk.NewDec(2)
		positionMargin = sdk.NewDec(100000000)

		BeforeEach(func() {
			initialCoinDeposit = sdk.NewInt(500000000)
			coin := sdk.NewCoin(quoteDenom, initialCoinDeposit)

			testexchange.MintAndDeposit(app, ctx, buyer.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, seller.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, liquidator.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, lucas.String(), sdk.NewCoins(coin))
			testexchange.MintAndDeposit(app, ctx, andrea.String(), sdk.NewCoins(coin))
		})

		JustBeforeEach(func() {
			// post initial orders to create the position in such a way so that the seller has negative position delta
			lucasLimitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(30000000), positionQuantity.Quo(sdk.MustNewDecFromStr("4")), positionMargin, types.OrderType_SELL, lucas)
			anreaLimitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(25000000), positionQuantity.Quo(sdk.MustNewDecFromStr("4")), positionMargin, types.OrderType_BUY, andrea)

			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(buyOrderPrice, positionQuantity, positionMargin, types.OrderType_BUY, buyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(sellOrderPrice, positionQuantity, positionMargin, types.OrderType_SELL, seller)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), anreaLimitDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), lucasLimitDerivativeSellOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

			oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(newOraclePrice, ctx.BlockTime().Unix()))
		})

		AfterEach(func() {
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		It("doesn't turn seller's balance negative, because extra fee is taken from bank balance", func() {
			if !types.IsDefaultSubaccountID(seller) {
				Skip("cannot be run for non-default subaccount")
			}
			feeRate := testInput.Perps[0].TakerFeeRate
			originalfee := sellOrderPrice.Mul(positionQuantity).Mul(feeRate)
			newFee := buyOrderPrice.Mul(positionQuantity).Mul(feeRate)
			feeDiff := newFee.Sub(originalfee)

			// user should be charged: position margin + taker fee + extra taker fee for selling the order for more than initially expected
			expectedBankBalance := initialCoinDeposit.Sub(positionMargin.TruncateInt()).Sub(feeDiff.TruncateInt()).Sub(originalfee.TruncateInt())
			Expect(app.BankKeeper.GetBalance(ctx, sellerAccount, "USDT0").Amount.String()).To(Equal(expectedBankBalance.String()), "extra fee wasn't charged from bank balance")

			sellerDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, seller, quoteDenom)
			Expect(sellerDepositBefore.AvailableBalance.Equal(sdk.ZeroDec())).To(BeTrue(), "seller's available balance wasn't zero")
			Expect(sellerDepositBefore.TotalBalance.Equal(sdk.ZeroDec())).To(BeTrue(), "seller's total balance wasn't zero")
		})

		It("doesn't turn seller's balance negative, because extra fee is taken from subaccount balance", func() {
			if types.IsDefaultSubaccountID(seller) {
				Skip("cannot be run for default subaccount")
			}
			feeRate := testInput.Perps[0].TakerFeeRate
			originalfee := sellOrderPrice.Mul(positionQuantity).Mul(feeRate)
			newFee := buyOrderPrice.Mul(positionQuantity).Mul(feeRate)
			feeDiff := newFee.Sub(originalfee)

			expectedDepositBalance := initialCoinDeposit.ToDec().Sub(positionMargin).Sub(feeDiff).Sub(originalfee)
			actualDepositBalance := app.ExchangeKeeper.GetDeposit(ctx, seller, quoteDenom)
			Expect(actualDepositBalance.AvailableBalance.String()).To(Equal(expectedDepositBalance.String()), "extra fee wasn't charged from the subaccount")
			Expect(actualDepositBalance.TotalBalance.String()).To(Equal(expectedDepositBalance.String()), "extra fee wasn't charged from the subaccount")

			sellerDepositBefore = app.ExchangeKeeper.GetDeposit(ctx, seller, quoteDenom)
		})

		Context("but if user doesn't have any funds in bank", func() {
			var feeRate, originalFee sdk.Dec
			BeforeEach(func() {
				if !types.IsDefaultSubaccountID(seller) {
					Skip("cannot be run for non-default subaccount")
				}

				// let's move all funds (but what he needs to post the order) from seller's bank balance
				feeRate = testInput.Perps[0].TakerFeeRate
				originalFee = sellOrderPrice.Mul(positionQuantity).Mul(feeRate)
				toMove := initialCoinDeposit.Sub(positionMargin.TruncateInt().Add(originalFee.TruncateInt()))
				app.BankKeeper.SendCoinsFromAccountToModule(ctx, sellerAccount, minttypes.ModuleName, sdk.NewCoins(sdk.NewCoin("USDT0", toMove)))
			})

			It("takes the extra fee from seller's position margin", func() {
				newFee := buyOrderPrice.Mul(positionQuantity).Mul(feeRate)
				feeDiff := newFee.Sub(originalFee)

				expectedMargin := positionMargin.Sub(feeDiff)
				sellerPosition := app.ExchangeKeeper.GetPosition(ctx, testInput.Perps[0].MarketID, seller)
				Expect(sellerPosition.Margin.String()).To(Equal(expectedMargin.String()), "position margin wasn't decreased by extra fee")
			})
		})

		Context("but if user doesn't have any funds in subaccount", func() {
			var feeRate, originalFee sdk.Dec
			BeforeEach(func() {
				if types.IsDefaultSubaccountID(seller) {
					Skip("cannot be run for default subaccount")
				}

				// let's move all funds (but what he needs to post the order) from seller's subaccount balance
				feeRate = testInput.Perps[0].TakerFeeRate
				originalFee = sellOrderPrice.Mul(positionQuantity).Mul(feeRate)
				toMove := initialCoinDeposit.Sub(positionMargin.TruncateInt().Add(originalFee.TruncateInt()))
				_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), &types.MsgWithdraw{
					Sender:       sellerAccount.String(),
					SubaccountId: seller.Hex(),
					Amount:       sdk.NewCoin(quoteDenom, toMove),
				})
				testexchange.OrFail(err)
			})

			It("takes the extra fee from seller's position margin", func() {
				newFee := buyOrderPrice.Mul(positionQuantity).Mul(feeRate)
				feeDiff := newFee.Sub(originalFee)

				expectedMargin := positionMargin.Sub(feeDiff)
				sellerPosition := app.ExchangeKeeper.GetPosition(ctx, testInput.Perps[0].MarketID, seller)
				Expect(sellerPosition.Margin.String()).To(Equal(expectedMargin.String()), "position margin wasn't decreased by extra fee")
			})
		})

		Context("that is later liquidated with negative payout", func() {
			JustBeforeEach(func() {
				// push the price up so much as to make the position liquidable and at a huge loss
				newOraclePrice = sdk.NewDec(2000000000)
				marketMakerQuantity = sdk.NewDec(20)
				marketMakerMargin = newOraclePrice.Mul(marketMakerQuantity)
				_ = sdk.NewDec(500000)

				// add deposit for market maker to post resting orders that will be used for liquidation
				mmDeposit := marketMakerMargin.Mul(sdk.NewDec(3))
				coin := sdk.NewCoin(quoteDenom, mmDeposit.TruncateInt())
				testexchange.MintAndDeposit(app, ctx, marketMaker.String(), sdk.NewCoins(coin))

				bigLimitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(newOraclePrice, marketMakerQuantity, marketMakerMargin, types.OrderType_SELL, marketMaker)

				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), bigLimitDerivativeSellOrder)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(newOraclePrice, ctx.BlockTime().Unix()))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("get missing funds from insurance funds and zeroes seller's balance", func() {
				insuranceFundBefore := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.Perps[0].MarketID)
				liquidateMsg := testInput.NewMsgLiquidatePosition(seller)
				_, err = msgServer.LiquidatePosition(sdk.WrapSDKContext(ctx), liquidateMsg)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				insuranceFundAfter := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.Perps[0].MarketID)
				Expect(insuranceFundBefore.Balance.GT(insuranceFundAfter.Balance)).To(BeTrue(), "missing funds were not taken from insurance fund")

				sellerDepositAfter := app.ExchangeKeeper.GetDeposit(ctx, seller, quoteDenom)
				Expect(sellerDepositAfter.AvailableBalance.String()).To(Equal(sdk.ZeroDec().String()), "seller's available balance was not zeroed")
				Expect(sellerDepositAfter.TotalBalance.String()).To(Equal(sdk.ZeroDec().String()), "seller's total balance was not zeroed")
			})
		})
	})
})
