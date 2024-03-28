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

var _ = Describe("SpotMarkets", func() {
	var (
		testInput  testexchange.TestInput
		app        *simapp.InjectiveApp
		ctx        sdk.Context
		spotMarket *types.SpotMarket
	)

	BeforeEach(func() {
		// Create Injective App
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
	})

	Context("Check if spot market exist", func() {
		It("the spot market should not be present", func() {
			spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
			Expect(spotMarket).To(BeNil())
		})
	})

	Context("Launch a spot market", func() {
		var err error
		JustBeforeEach(func() {
			marketInput := testInput.Spots[0]
			_, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, marketInput.Ticker, marketInput.BaseDenom, marketInput.QuoteDenom, marketInput.MinPriceTickSize, marketInput.MinQuantityTickSize)
		})
		It("the spot market should be  added", func() {
			Expect(err).NotTo(HaveOccurred())
			spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
			Expect(spotMarket).NotTo(BeNil())

		})
	})

	Context("Spot market param increase fee rates by governance", func() {
		var proposals []types.SpotMarketParamUpdateProposal
		var oldSpotMarketMakerFee, newSpotMarketMakerFee sdk.Dec
		var err error

		// Note that subaccountID1 is a default subaccount
		var subaccountID1 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000")
		var subaccountID2 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001")
		var subaccountID3 = common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002")

		var subaccountsList = []common.Hash{subaccountID1, subaccountID2, subaccountID3}

		var buyPrice1, buyQuantity1, buyPrice2, buyQuantity2, buyPrice3, buyQuantity3, addedBalance sdk.Dec
		var isAddingBalanceToSubaccounts = false

		var setSpotMarketToMakerFee = func() {
			spotMarket.MakerFeeRate = oldSpotMarketMakerFee
			app.ExchangeKeeper.SetSpotMarket(ctx, spotMarket)
		}

		var addMakerFeeProposal = func() {
			setSpotMarketToMakerFee()

			makerFeeRate := newSpotMarketMakerFee
			takerFeeRate := sdk.NewDecWithPrec(2, 2)
			relayerFeeShareRate := sdk.NewDecWithPrec(5, 2)
			minPriceTickSize := sdk.NewDecWithPrec(1, 5)
			minQuantityTickSize := sdk.NewDecWithPrec(1, 5)

			app.ExchangeKeeper.ScheduleSpotMarketParamUpdate(ctx, &types.SpotMarketParamUpdateProposal{
				Title:               "Update spot market param",
				Description:         "Update spot market description",
				MarketId:            spotMarket.MarketId,
				MakerFeeRate:        &makerFeeRate,        // 1% <= 0.1%
				TakerFeeRate:        &takerFeeRate,        // 2% <= 0.2%
				RelayerFeeShareRate: &relayerFeeShareRate, // 5% <= 0.5%
				MinPriceTickSize:    &minPriceTickSize,
				MinQuantityTickSize: &minQuantityTickSize,
				Status:              0,
			})
			proposals = make([]types.SpotMarketParamUpdateProposal, 0)
			app.ExchangeKeeper.IterateSpotMarketParamUpdates(ctx, func(p *types.SpotMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})
		}

		JustBeforeEach(func() {
			marketInput := testInput.Spots[0]
			spotMarket, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, marketInput.Ticker, marketInput.BaseDenom, marketInput.QuoteDenom, marketInput.MinPriceTickSize, marketInput.MinQuantityTickSize)
			testexchange.OrFail(err)

			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
			testInput.AddSpotDepositsForSubaccounts(app, ctx, 1, nil, subaccountsList)

			// Set Orders
			buyPrice1, buyQuantity1 = testexchange.PriceAndQuantityFromString("10.1", "1.5")
			buyPrice2, buyQuantity2 = testexchange.PriceAndQuantityFromString("30.1", "3.5")
			buyPrice3, buyQuantity3 = testexchange.PriceAndQuantityFromString("20.1", "100.531")

			sellPrice1, sellQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			sellPrice2, sellQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			sellPrice3, sellQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")

			msgs := testInput.NewListOfMsgCreateSpotLimitOrderForMarketIndex(0,
				// buy orders
				testexchange.NewBareSpotLimitOrder(buyPrice1, buyQuantity1, types.OrderType_BUY_PO, subaccountID1),
				testexchange.NewBareSpotLimitOrder(buyPrice2, buyQuantity2, types.OrderType_BUY_PO, subaccountID2),
				testexchange.NewBareSpotLimitOrder(buyPrice3, buyQuantity3, types.OrderType_BUY_PO, subaccountID3),
				// sell orders
				testexchange.NewBareSpotLimitOrder(sellPrice1, sellQuantity1, types.OrderType_SELL_PO, subaccountID1),
				testexchange.NewBareSpotLimitOrder(sellPrice2, sellQuantity2, types.OrderType_SELL_PO, subaccountID2),
				testexchange.NewBareSpotLimitOrder(sellPrice3, sellQuantity3, types.OrderType_SELL_PO, subaccountID3),
			)

			for _, msg := range msgs {
				testexchange.ReturnOrFail(msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), msg))
			}

			spotOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, spotMarket.MarketID(), true)
			Expect(spotOrders).To(HaveLen(3))

			// withdraw balance for testing purposes
			testInput.WithdrawSpotDepositsForSubaccounts(app, ctx, 1, subaccountsList)

			addedBalance = sdk.NewDec(1000)
			if isAddingBalanceToSubaccounts {
				testInput.AddSpotDepositsForSubaccounts(app, ctx, 1, &addedBalance, subaccountsList)
			}

			addMakerFeeProposal()
		})

		var expectCorrectParamUpdate = func() {
			It("spot market update proposal should be found", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(len(proposals)).Should(BeEquivalentTo(1))
				Expect(proposals[0].MarketId).Should(BeEquivalentTo(spotMarket.MarketId))
				Expect(*proposals[0].MakerFeeRate).Should(BeEquivalentTo(newSpotMarketMakerFee))
				Expect(*proposals[0].TakerFeeRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(2, 2)))
				Expect(*proposals[0].RelayerFeeShareRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(5, 2)))
				Expect(*proposals[0].MinPriceTickSize).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
				Expect(*proposals[0].MinQuantityTickSize).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
			})

			It("proposal execution should pass and give fee refunds/charges", func() {
				err = app.ExchangeKeeper.ExecuteSpotMarketParamUpdateProposal(ctx, &proposals[0])
				testexchange.OrFail(err)

				spotOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(ctx, spotMarket.MarketID(), true)
				deposit1 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID1, spotMarket.QuoteDenom)
				deposit2 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID2, spotMarket.QuoteDenom)
				deposit3 := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID3, spotMarket.QuoteDenom)

				isChargingExtraFee := newSpotMarketMakerFee.IsPositive() && newSpotMarketMakerFee.GT(oldSpotMarketMakerFee)
				isRefundingFee := oldSpotMarketMakerFee.IsPositive() && newSpotMarketMakerFee.LT(oldSpotMarketMakerFee)
				hasSufficientBalanceToPayExtraFee := isAddingBalanceToSubaccounts

				if isChargingExtraFee && !hasSufficientBalanceToPayExtraFee {
					orderCancelRefundRate := sdk.MaxDec(sdk.ZeroDec(), oldSpotMarketMakerFee)

					expectedAvailableBalance1 := buyQuantity1.Mul(buyPrice1).Mul(sdk.NewDec(1).Add(orderCancelRefundRate))
					Expect(deposit1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := buyQuantity2.Mul(buyPrice2).Mul(sdk.NewDec(1).Add(orderCancelRefundRate))
					Expect(deposit2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := buyQuantity3.Mul(buyPrice3).Mul(sdk.NewDec(1).Add(orderCancelRefundRate))
					Expect(deposit3.String()).To(Equal(expectedAvailableBalance3.String()))

					Expect(len(spotOrders)).To(Equal(0))
				}

				if isChargingExtraFee && hasSufficientBalanceToPayExtraFee {
					feeChargeRate := sdk.MinDec(newSpotMarketMakerFee, newSpotMarketMakerFee.Sub(oldSpotMarketMakerFee))

					expectedAvailableBalance1 := addedBalance.Sub(buyQuantity1.Mul(buyPrice1).Mul(feeChargeRate))
					Expect(deposit1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := addedBalance.Sub(buyQuantity2.Mul(buyPrice2).Mul(feeChargeRate))
					Expect(deposit2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := addedBalance.Sub(buyQuantity3.Mul(buyPrice3).Mul(feeChargeRate))
					Expect(deposit3.String()).To(Equal(expectedAvailableBalance3.String()))

					Expect(len(spotOrders)).To(Equal(3))
				}

				if isRefundingFee {
					refundRate := sdk.MinDec(oldSpotMarketMakerFee, oldSpotMarketMakerFee.Sub(newSpotMarketMakerFee))

					expectedAvailableBalance1 := buyQuantity1.Mul(buyPrice1).Mul(refundRate)
					Expect(deposit1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := buyQuantity2.Mul(buyPrice2).Mul(refundRate)
					Expect(deposit2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := buyQuantity3.Mul(buyPrice3).Mul(refundRate)
					Expect(deposit3.String()).To(Equal(expectedAvailableBalance3.String()))

					Expect(len(spotOrders)).To(Equal(3))
				}

				if newSpotMarketMakerFee.Equal(oldSpotMarketMakerFee) {
					expectedAvailableBalance1 := sdk.ZeroDec()
					Expect(deposit1.String()).To(Equal(expectedAvailableBalance1.String()))
					expectedAvailableBalance2 := sdk.ZeroDec()
					Expect(deposit2.String()).To(Equal(expectedAvailableBalance2.String()))
					expectedAvailableBalance3 := sdk.ZeroDec()
					Expect(deposit3.String()).To(Equal(expectedAvailableBalance3.String()))

					Expect(len(spotOrders)).To(Equal(3))
				}
			})
		}

		Describe("when maker fee stays unchanged", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(2, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(2, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee decreases from positive to still positive", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(2, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(1, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee increases from positive to positive", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(1, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(4, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee increases from negative to still negative", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(-4, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(-1, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee decreases from negative to negative", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(-2, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(-5, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee decreases from positive to negative", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(2, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(-1, 3)
			})

			expectCorrectParamUpdate()
		})

		Describe("when maker fee increases from negative to positive", func() {
			BeforeEach(func() {
				oldSpotMarketMakerFee = sdk.NewDecWithPrec(-2, 3)
				newSpotMarketMakerFee = sdk.NewDecWithPrec(1, 3)
			})

			Describe("when subaccounts do not have sufficient available balance to pay extra fee", func() {
				BeforeEach(func() {
					isAddingBalanceToSubaccounts = false
				})

				expectCorrectParamUpdate()
			})

			Describe("when subaccounts have sufficient available balance to pay extra fee", func() {
				BeforeEach(func() {
					isAddingBalanceToSubaccounts = true
				})

				expectCorrectParamUpdate()
			})
		})
	})

	Context("Spot market param decrease fee rates by governance", func() {
		var proposals []types.SpotMarketParamUpdateProposal
		var market *types.SpotMarket
		var err error
		JustBeforeEach(func() {
			marketInput := testInput.Spots[0]
			market, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, marketInput.Ticker, marketInput.BaseDenom, marketInput.QuoteDenom, marketInput.MinPriceTickSize, marketInput.MinQuantityTickSize)
			testexchange.OrFail(err)

			msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

			subaccountID := common.HexToHash("0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000")
			subaccountsList := []common.Hash{subaccountID}

			testInput.AddSpotDepositsForSubaccounts(app, ctx, 1, nil, subaccountsList)

			// Set Orders
			msgs := testInput.NewListOfMsgCreateSpotLimitOrderForMarketIndex(0,
				// buy orders
				testexchange.NewBareSpotLimitOrderFromString("10.1", "1.5", types.OrderType_BUY_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("30.1", "3.5", types.OrderType_BUY_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("20.1", "100.531", types.OrderType_BUY_PO, subaccountID),
				// sell orders
				testexchange.NewBareSpotLimitOrderFromString("100.1", "1.5", types.OrderType_SELL_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("300.1", "3.5", types.OrderType_SELL_PO, subaccountID),
				testexchange.NewBareSpotLimitOrderFromString("200.1", "100.531", types.OrderType_SELL_PO, subaccountID),
			)

			for _, msg := range msgs {
				testexchange.ReturnOrFail(msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), msg))
			}

			// withdraw balance for testing purposes
			testInput.WithdrawSpotDepositsForSubaccounts(app, ctx, 1, subaccountsList)

			makerFeeRate := sdk.NewDecWithPrec(1, 4)
			takerFeeRate := sdk.NewDecWithPrec(2, 4)
			relayerFeeShareRate := sdk.NewDecWithPrec(5, 4)
			minPriceTickSize := sdk.NewDecWithPrec(1, 5)
			minQuantityTickSize := sdk.NewDecWithPrec(1, 5)

			app.ExchangeKeeper.ScheduleSpotMarketParamUpdate(ctx, &types.SpotMarketParamUpdateProposal{
				Title:               "Update spot market param",
				Description:         "Update spot market description",
				MarketId:            market.MarketId,
				MakerFeeRate:        &makerFeeRate,        // 1% <= 0.1%
				TakerFeeRate:        &takerFeeRate,        // 2% <= 0.2%
				RelayerFeeShareRate: &relayerFeeShareRate, // 5% <= 0.5%
				MinPriceTickSize:    &minPriceTickSize,
				MinQuantityTickSize: &minQuantityTickSize,
			})
			app.ExchangeKeeper.IterateSpotMarketParamUpdates(ctx, func(p *types.SpotMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})
		})

		It("spot market update proposal should be found", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(len(proposals)).Should(BeEquivalentTo(1))
			Expect(proposals[0].MarketId).Should(BeEquivalentTo(market.MarketId))
			Expect(*proposals[0].MakerFeeRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 4)))
			Expect(*proposals[0].TakerFeeRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(2, 4)))
			Expect(*proposals[0].RelayerFeeShareRate).Should(BeEquivalentTo(sdk.NewDecWithPrec(5, 4)))
			Expect(*proposals[0].MinPriceTickSize).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
			Expect(*proposals[0].MinQuantityTickSize).Should(BeEquivalentTo(sdk.NewDecWithPrec(1, 5)))
		})

		It("proposal execution should pass", func() {
			err = app.ExchangeKeeper.ExecuteSpotMarketParamUpdateProposal(ctx, &proposals[0])
			testexchange.OrFail(err)
		})
	})
})
