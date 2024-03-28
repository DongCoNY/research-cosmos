package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("DerivativeMarkets", func() {
	var (
		testInput        testexchange.TestInput
		app              *simapp.InjectiveApp
		ctx              sdk.Context
		derivativeMarket *types.DerivativeMarket
	)

	BeforeEach(func() {
		// Create Injective App
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
	})

	var executeTest = func(param string) {
		var proposals []types.DerivativeMarketParamUpdateProposal
		var oldDerivativeMarketMakerFee, newDerivativeMarketMakerFee sdk.Dec
		var err error

		// Note that subaccountID1 is a default subaccount
		var subaccountID1 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000")
		var subaccountID2 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
		var subaccountID3 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002")
		var subaccountID4 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000004")
		var subaccountID5 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000005")
		var subaccountID6 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000006")

		var subaccountsList = []common.Hash{subaccountID1, subaccountID2, subaccountID3, subaccountID4, subaccountID5, subaccountID6}

		var buyPrice1, buyQuantity1, buyPrice2, buyQuantity2, buyPrice3, buyQuantity3, addedBalance sdk.Dec
		var sellPrice1, sellQuantity1, sellPrice2, sellQuantity2, sellPrice3, sellQuantity3 sdk.Dec
		var buyMargin1, buyMargin2, buyMargin3, sellMargin1, sellMargin2, sellMargin3 sdk.Dec
		var isAddingBalanceToSubaccounts = false

		var setDerivativeMarketToMakerFee = func() {
			derivativeMarket.MakerFeeRate = oldDerivativeMarketMakerFee
			app.ExchangeKeeper.SetDerivativeMarket(ctx, derivativeMarket)
		}

		var addMakerFeeProposal = func() {
			setDerivativeMarketToMakerFee()

			makerFeeRate := newDerivativeMarketMakerFee
			takerFeeRate := sdk.NewDecWithPrec(2, 2)
			relayerFeeShareRate := sdk.NewDecWithPrec(5, 2)
			minPriceTickSize := sdk.NewDecWithPrec(1, 5)
			minQuantityTickSize := sdk.NewDecWithPrec(1, 5)
			hourlyInterestRate := sdk.NewDecWithPrec(1, 5)
			hourlyFundingRateCap := sdk.NewDecWithPrec(1, 5)

			app.ExchangeKeeper.ScheduleDerivativeMarketParamUpdate(ctx, &types.DerivativeMarketParamUpdateProposal{
				Title:                  "Update derivative market param",
				Description:            "Update derivative market description",
				MarketId:               testInput.Perps[0].MarketID.Hex(),
				MakerFeeRate:           &makerFeeRate,        // 1% <= 0.1%
				TakerFeeRate:           &takerFeeRate,        // 2% <= 0.2%
				RelayerFeeShareRate:    &relayerFeeShareRate, // 5% <= 0.5%
				MinPriceTickSize:       &minPriceTickSize,
				MinQuantityTickSize:    &minQuantityTickSize,
				InitialMarginRatio:     &derivativeMarket.InitialMarginRatio,
				MaintenanceMarginRatio: &derivativeMarket.MaintenanceMarginRatio,
				HourlyInterestRate:     &hourlyInterestRate,
				HourlyFundingRateCap:   &hourlyFundingRateCap,
			})
			proposals = make([]types.DerivativeMarketParamUpdateProposal, 0)
			app.ExchangeKeeper.IterateDerivativeMarketParamUpdates(ctx, func(p *types.DerivativeMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})
		}

		JustBeforeEach(func() {
			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

			testInput.AddDerivativeDepositsForSubaccounts(app, ctx, 1, nil, false, subaccountsList)

			startingPrice := sdk.NewDec(300)
			app.OracleKeeper.SetPriceFeedPriceState(ctx, testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
			sender := sdk.AccAddress(common.FromHex("0x90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			perpCoin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(perpCoin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(perpCoin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				perpCoin,
				testInput.Perps[0].Ticker,
				testInput.Perps[0].QuoteDenom,
				testInput.Perps[0].OracleBase,
				testInput.Perps[0].OracleQuote,
				testInput.Perps[0].OracleType,
				-1,
			))

			derivativeMarket, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
				ctx,
				testInput.Perps[0].Ticker,
				testInput.Perps[0].QuoteDenom,
				testInput.Perps[0].OracleBase,
				testInput.Perps[0].OracleQuote,
				0,
				testInput.Perps[0].OracleType,
				testInput.Perps[0].InitialMarginRatio,
				testInput.Perps[0].MaintenanceMarginRatio,
				testInput.Perps[0].MakerFeeRate,
				testInput.Perps[0].TakerFeeRate,
				testInput.Perps[0].MinPriceTickSize,
				testInput.Perps[0].MinQuantityTickSize,
			)
			testexchange.OrFail(err)

			buyPrice1, buyQuantity1, buyMargin1 = testexchange.PriceQuantityAndMarginFromString("100.1", "1.5", "2001")
			buyPrice2, buyQuantity2, buyMargin2 = testexchange.PriceQuantityAndMarginFromString("300.1", "3.5", "2002")
			buyPrice3, buyQuantity3, buyMargin3 = testexchange.PriceQuantityAndMarginFromString("200.1", "100.531", "2003")

			sellPrice1, sellQuantity1, sellMargin1 = testexchange.PriceQuantityAndMarginFromString("500.1", "1.5", "2004")
			sellPrice2, sellQuantity2, sellMargin2 = testexchange.PriceQuantityAndMarginFromString("400.1", "3.5", "2005")
			sellPrice3, sellQuantity3, sellMargin3 = testexchange.PriceQuantityAndMarginFromString("506.1", "10.531", "2006")

			msgs := testInput.NewListOfMsgCreateDerivativeLimitOrderForMarketIndex(0,
				// buy orders
				testexchange.NewBareDerivativeLimitOrder(buyPrice1, buyQuantity1, buyMargin1, types.OrderType_BUY_PO, subaccountID1, false),
				testexchange.NewBareDerivativeLimitOrder(buyPrice2, buyQuantity2, buyMargin2, types.OrderType_BUY_PO, subaccountID2, false),
				testexchange.NewBareDerivativeLimitOrder(buyPrice3, buyQuantity3, buyMargin3, types.OrderType_BUY_PO, subaccountID3, false),
				// sell orders
				testexchange.NewBareDerivativeLimitOrder(sellPrice1, sellQuantity1, sellMargin1, types.OrderType_SELL_PO, subaccountID4, false),
				testexchange.NewBareDerivativeLimitOrder(sellPrice2, sellQuantity2, sellMargin2, types.OrderType_SELL_PO, subaccountID5, false),
				testexchange.NewBareDerivativeLimitOrder(sellPrice3, sellQuantity3, sellMargin3, types.OrderType_SELL_PO, subaccountID6, false),
			)

			for _, msg := range msgs {
				testexchange.ReturnOrFail(msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), msg))
			}

			addedBalance = sdk.NewDec(1000)
			testInput.WithdrawDerivativeDepositsForSubaccounts(app, ctx, 1, false, subaccountsList)

			if isAddingBalanceToSubaccounts {
				testInput.AddDerivativeDepositsForSubaccounts(app, ctx, 1, &addedBalance, false, subaccountsList)
			}

			addMakerFeeProposal()
		})

		var expectCorrectParamUpdate = func() {
			It("derivative market update proposal should be found", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(len(proposals)).Should(BeEquivalentTo(1))
				Expect(proposals[0].MarketId).Should(BeEquivalentTo(derivativeMarket.MarketId))
				Expect(*proposals[0].MakerFeeRate).Should(BeEquivalentTo(newDerivativeMarketMakerFee))
				Expect(*proposals[0].TakerFeeRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(2, 2)))
				Expect(*proposals[0].RelayerFeeShareRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(5, 2)))
				Expect(*proposals[0].MinPriceTickSize).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
				Expect(*proposals[0].MinQuantityTickSize).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
				Expect(*proposals[0].HourlyInterestRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
				Expect(*proposals[0].HourlyFundingRateCap).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
			})

			It("proposal execution should pass and give fee refunds/charges", func() {
				err := app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &proposals[0])
				testexchange.OrFail(err)

				derivativeBuyOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, derivativeMarket.MarketID(), true)
				derivativeSellOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, derivativeMarket.MarketID(), false)

				funds1 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID1, derivativeMarket.QuoteDenom)
				funds2 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID2, derivativeMarket.QuoteDenom)
				funds3 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID3, derivativeMarket.QuoteDenom)
				funds4 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID4, derivativeMarket.QuoteDenom)
				funds5 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID5, derivativeMarket.QuoteDenom)
				funds6 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID6, derivativeMarket.QuoteDenom)

				isChargingExtraFee := newDerivativeMarketMakerFee.IsPositive() && newDerivativeMarketMakerFee.GT(oldDerivativeMarketMakerFee)
				isRefundingFee := oldDerivativeMarketMakerFee.IsPositive() && newDerivativeMarketMakerFee.LT(oldDerivativeMarketMakerFee)
				hasSufficientBalanceToPayExtraFee := isAddingBalanceToSubaccounts

				if isChargingExtraFee && !hasSufficientBalanceToPayExtraFee {
					orderCancelRefundRate := sdk.MaxDec(sdk.ZeroDec(), oldDerivativeMarketMakerFee)

					expectedAvailableBalance1 := buyQuantity1.Mul(buyPrice1).Mul(orderCancelRefundRate).Add(buyMargin1)
					Expect(funds1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := buyQuantity2.Mul(buyPrice2).Mul(orderCancelRefundRate).Add(buyMargin2)
					Expect(funds2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := buyQuantity3.Mul(buyPrice3).Mul(orderCancelRefundRate).Add(buyMargin3)
					Expect(funds3.String()).To(Equal(expectedAvailableBalance3.String()))

					expectedAvailableBalance4 := sellQuantity1.Mul(sellPrice1).Mul(orderCancelRefundRate).Add(sellMargin1)
					Expect(funds4.String()).To(Equal(expectedAvailableBalance4.String()))
					expectedAvailableBalance5 := sellQuantity2.Mul(sellPrice2).Mul(orderCancelRefundRate).Add(sellMargin2)
					Expect(funds5.String()).To(Equal(expectedAvailableBalance5.String()))
					expectedAvailableBalance6 := sellQuantity3.Mul(sellPrice3).Mul(orderCancelRefundRate).Add(sellMargin3)
					Expect(funds6.String()).To(Equal(expectedAvailableBalance6.String()))

					Expect(len(derivativeBuyOrders)).To(Equal(0))
					Expect(len(derivativeSellOrders)).To(Equal(0))
				}

				if isChargingExtraFee && hasSufficientBalanceToPayExtraFee {
					feeChargeRate := sdk.MinDec(newDerivativeMarketMakerFee, newDerivativeMarketMakerFee.Sub(oldDerivativeMarketMakerFee))

					expectedAvailableBalance1 := addedBalance.Sub(buyQuantity1.Mul(buyPrice1).Mul(feeChargeRate))
					Expect(funds1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := addedBalance.Sub(buyQuantity2.Mul(buyPrice2).Mul(feeChargeRate))
					Expect(funds2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := addedBalance.Sub(buyQuantity3.Mul(buyPrice3).Mul(feeChargeRate))
					Expect(funds3.String()).To(Equal(expectedAvailableBalance3.String()))

					expectedAvailableBalance4 := addedBalance.Sub(sellQuantity1.Mul(sellPrice1).Mul(feeChargeRate))
					Expect(funds4.String()).To(Equal(expectedAvailableBalance4.String()))
					expectedAvailableBalance5 := addedBalance.Sub(sellQuantity2.Mul(sellPrice2).Mul(feeChargeRate))
					Expect(funds5.String()).To(Equal(expectedAvailableBalance5.String()))
					expectedAvailableBalance6 := addedBalance.Sub(sellQuantity3.Mul(sellPrice3).Mul(feeChargeRate))
					Expect(funds6.String()).To(Equal(expectedAvailableBalance6.String()))

					Expect(len(derivativeBuyOrders)).To(Equal(3))
					Expect(len(derivativeSellOrders)).To(Equal(3))
				}

				if isRefundingFee {
					refundRate := sdk.MinDec(oldDerivativeMarketMakerFee, oldDerivativeMarketMakerFee.Sub(newDerivativeMarketMakerFee))

					expectedAvailableBalance1 := buyQuantity1.Mul(buyPrice1).Mul(refundRate)
					Expect(funds1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := buyQuantity2.Mul(buyPrice2).Mul(refundRate)
					Expect(funds2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := buyQuantity3.Mul(buyPrice3).Mul(refundRate)
					Expect(funds3.String()).To(Equal(expectedAvailableBalance3.String()))

					expectedAvailableBalance4 := sellQuantity1.Mul(sellPrice1).Mul(refundRate)
					Expect(funds4.String()).To(Equal(expectedAvailableBalance4.String()))
					expectedAvailableBalance5 := sellQuantity2.Mul(sellPrice2).Mul(refundRate)
					Expect(funds5.String()).To(Equal(expectedAvailableBalance5.String()))
					expectedAvailableBalance6 := sellQuantity3.Mul(sellPrice3).Mul(refundRate)
					Expect(funds6.String()).To(Equal(expectedAvailableBalance6.String()))

					Expect(len(derivativeBuyOrders)).To(Equal(3))
					Expect(len(derivativeSellOrders)).To(Equal(3))
				}

				if newDerivativeMarketMakerFee.Equal(oldDerivativeMarketMakerFee) {
					expectedAvailableBalance1 := sdk.ZeroDec()
					Expect(funds1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := sdk.ZeroDec()
					Expect(funds2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := sdk.ZeroDec()
					Expect(funds3.String()).To(Equal(expectedAvailableBalance3.String()))

					expectedAvailableBalance4 := sdk.ZeroDec()
					Expect(funds4.String()).To(Equal(expectedAvailableBalance4.String()))
					expectedAvailableBalance5 := sdk.ZeroDec()
					Expect(funds5.String()).To(Equal(expectedAvailableBalance5.String()))
					expectedAvailableBalance6 := sdk.ZeroDec()
					Expect(funds6.String()).To(Equal(expectedAvailableBalance6.String()))

					Expect(len(derivativeBuyOrders)).To(Equal(3))
					Expect(len(derivativeSellOrders)).To(Equal(3))
				}
			})
		}

		Describe("when maker fee stays unchanged", func() {
			BeforeEach(func() {
				oldDerivativeMarketMakerFee = sdk.NewDecWithPrec(2, 3)
				newDerivativeMarketMakerFee = sdk.NewDecWithPrec(2, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee increases from positive to still positive", func() {
			BeforeEach(func() {
				oldDerivativeMarketMakerFee = sdk.NewDecWithPrec(1, 3)
				newDerivativeMarketMakerFee = sdk.NewDecWithPrec(2, 3)
			})

			Describe("when subaccounts have sufficient available balance to pay extra fee", func() {
				BeforeEach(func() {
					isAddingBalanceToSubaccounts = true
				})

				expectCorrectParamUpdate()
			})

			Describe("when subaccounts do not have sufficient available balance to pay extra fee", func() {
				BeforeEach(func() {
					isAddingBalanceToSubaccounts = false
				})

				expectCorrectParamUpdate()
			})
		})

		Describe("when maker fee increases from positive to positive", func() {
			BeforeEach(func() {
				oldDerivativeMarketMakerFee = sdk.NewDecWithPrec(1, 3)
				newDerivativeMarketMakerFee = sdk.NewDecWithPrec(4, 3)
			})

			expectCorrectParamUpdate()
		})
	}

	Context("Derivative market param increase fee rates by governance", func() {
		executeTest("xxx")

	})
	Context("Derivative market param increase fee rates by governance 2", func() {
		executeTest("yyy")

	})

})
