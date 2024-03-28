package keeper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
)

var _ = Describe("Aggregate volumes metadata tests", func() {
	var (
		player te.TestPlayer
	)

	type orderParams struct {
		acc, subacc, marketIdx int
		isLong                 bool
		price, quantity        float64
	}

	var newSpot = func(p orderParams) {
		orderType := te.OrderType_BUY
		if !p.isLong {
			orderType = te.OrderType_SELL
		}
		te.MustNotErr(player.ReplayCreateSpotOrderAction(&te.ActionCreateOrder{
			TestActionBase: te.NewActionBaseWithType(te.ActionType_spotLimitOrder),
			TestActionWithMarketAndAccount: te.TestActionWithMarketAndAccount{
				TestActionWithAccount: te.TestActionWithAccount{
					AccountIndex:    p.acc,
					SubaccountIndex: p.subacc,
				},
				TestActionWithMarket: te.TestActionWithMarket{
					MarketType:  te.MarketType_spot.Ptr(),
					MarketIndex: p.marketIdx,
				},
			},
			Price:     p.price,
			Quantity:  p.quantity,
			OrderType: &orderType,
		}))
	}

	var newDerivative = func(p orderParams) {
		orderType := te.OrderType_BUY
		if !p.isLong {
			orderType = te.OrderType_SELL
		}
		te.MustNotErr(player.ReplayCreateDerivativeOrderAction(&te.ActionCreateOrder{
			TestActionBase: te.NewActionBaseWithType(te.ActionType_derivativeLimitOrder),
			TestActionWithMarketAndAccount: te.TestActionWithMarketAndAccount{
				TestActionWithAccount: te.TestActionWithAccount{
					AccountIndex:    p.acc,
					SubaccountIndex: p.subacc,
				},
				TestActionWithMarket: te.TestActionWithMarket{
					MarketType:  te.MarketType_derivative.Ptr(),
					MarketIndex: p.marketIdx,
				},
			},
			Price:     p.price,
			Quantity:  p.quantity,
			OrderType: &orderType,
		}, false))
	}

	Context("Spot Markets", func() {
		var marketType te.MarketType

		BeforeEach(func() {
			marketType = te.MarketType_spot
			config := te.TestPlayerConfig{NumAccounts: 4, NumSpotMarkets: 1, NumSubaccounts: 2}
			player = te.InitTest(config, nil)
		})

		When("One order is matched", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, isLong: true, price: 90, quantity: 10})
				newSpot(orderParams{acc: 0, isLong: true, price: 70, quantity: 20}) // unimportant, just to check it's ignored
				newSpot(orderParams{acc: 1, isLong: false, price: 90, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched order notional", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(1800)))

				volumesSub1 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0., 0))
				Expect(volumesSub1[0].Volume.Total().TruncateInt64()).To(Equal(int64(900)))

				volumesSub2 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(1., 0))
				Expect(volumesSub2[0].Volume.Total().TruncateInt64()).To(Equal(int64(900)))
			})
		})

		When("Two order are matched", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, isLong: true, price: 110, quantity: 10})
				newSpot(orderParams{acc: 0, isLong: true, price: 105, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: false, price: 105, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(4200)))
			})
		})

		When("Two order are matched in subsequent blocks", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, isLong: true, price: 90, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: false, price: 90, quantity: 10})
				player.PerformEndBlockerAction(0, false)
				newSpot(orderParams{acc: 0, isLong: true, price: 100, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(3800)))
			})
		})

		When("Two order by different accounts are matched", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, isLong: true, price: 100, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: true, price: 100, quantity: 10})
				newSpot(orderParams{acc: 2, isLong: false, price: 100, quantity: 10})
				newSpot(orderParams{acc: 3, isLong: false, price: 100, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(4000)))
			})
		})

		When("Mixed maker and taker volumes", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, isLong: true, price: 110, quantity: 10})
				newSpot(orderParams{acc: 0, isLong: true, price: 105, quantity: 10})
				player.PerformEndBlockerAction(0, false)
				newSpot(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: false, price: 105, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(4200)))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(2100)))
				Expect(volume.MakerVolume.TruncateInt64()).To(Equal(int64(2100)))
			})
		})

		When("User has 2 subaccounts", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, subacc: 0, isLong: true, price: 100, quantity: 10})
				newSpot(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				player.PerformEndBlockerAction(0, false)
				newSpot(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				newSpot(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				newSpot(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(4000)))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(2500)))
				Expect(volume.MakerVolume.TruncateInt64()).To(Equal(int64(1500)))

				volume00 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0, 0))[0]
				Expect(volume00.Volume.Total().TruncateInt64()).To(Equal(int64(1000)))

				volume01 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0, 1))[0]
				Expect(volume01.Volume.Total().TruncateInt64()).To(Equal(int64(1000)))
				Expect(volume01.Volume.MakerVolume.TruncateInt64()).To(Equal(int64(500)))
				Expect(volume01.Volume.TakerVolume.TruncateInt64()).To(Equal(int64(500)))

				volume0 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesByAccAddress(player.Ctx, (*player.Accounts)[0].AccAddress)[0]
				Expect(volume0.Volume.Total().TruncateInt64()).To(Equal(int64(2000)))
				Expect(volume0.Volume.MakerVolume.TruncateInt64()).To(Equal(int64(1500)))
				Expect(volume0.Volume.TakerVolume.TruncateInt64()).To(Equal(int64(500)))
			})
		})

	})

	Context("Derivative Markets", func() {
		var marketType te.MarketType

		BeforeEach(func() {
			marketType = te.MarketType_derivative
			config := te.TestPlayerConfig{NumAccounts: 4, NumDerivativeMarkets: 1, NumSubaccounts: 2}
			player = te.InitTest(config, nil)
		})

		When("One order is matched", func() {
			BeforeEach(func() {
				newDerivative(orderParams{acc: 0, isLong: true, price: 90, quantity: 10})
				newDerivative(orderParams{acc: 0, isLong: true, price: 70, quantity: 10}) // unimportant, just to check it's ignored
				newDerivative(orderParams{acc: 1, isLong: false, price: 90, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched order notional", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(1800)))

				volumesSub1 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0., 0))
				Expect(volumesSub1[0].Volume.Total().TruncateInt64()).To(Equal(int64(900)))

				volumesSub2 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(1., 0))
				Expect(volumesSub2[0].Volume.Total().TruncateInt64()).To(Equal(int64(900)))
			})
		})

		When("Two order are matched", func() {
			BeforeEach(func() {
				newDerivative(orderParams{acc: 0, isLong: true, price: 110, quantity: 10})
				newDerivative(orderParams{acc: 0, isLong: true, price: 105, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: false, price: 105, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(4200)))
			})
		})

		When("Two order are matched in subsequent blocks", func() {
			BeforeEach(func() {
				newDerivative(orderParams{acc: 0, isLong: true, price: 90, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: false, price: 90, quantity: 10})
				player.PerformEndBlockerAction(0, false)
				newDerivative(orderParams{acc: 0, isLong: true, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(3800)))
			})
		})

		When("Two order by different accounts are matched", func() {
			BeforeEach(func() {
				newDerivative(orderParams{acc: 0, isLong: true, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: true, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 2, isLong: false, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 3, isLong: false, price: 100, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(4000)))
			})
		})

		When("Mixed maker and taker volumes", func() {
			BeforeEach(func() {
				newDerivative(orderParams{acc: 0, isLong: true, price: 110, quantity: 10})
				newDerivative(orderParams{acc: 0, isLong: true, price: 105, quantity: 10})
				player.PerformEndBlockerAction(0, false)
				newDerivative(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: false, price: 105, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(4200)))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(2100)))
				Expect(volume.MakerVolume.TruncateInt64()).To(Equal(int64(2100)))
			})
		})

		When("User has 2 subaccounts", func() {
			BeforeEach(func() {
				newDerivative(orderParams{acc: 0, subacc: 0, isLong: true, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				player.PerformEndBlockerAction(0, false)
				newDerivative(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				newDerivative(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 1, isLong: false, price: 100, quantity: 10})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(marketType, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(4000)))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(2500)))
				Expect(volume.MakerVolume.TruncateInt64()).To(Equal(int64(1500)))

				volume00 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0, 0))[0]
				Expect(volume00.Volume.Total().TruncateInt64()).To(Equal(int64(1000)))

				volume01 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0, 1))[0]
				Expect(volume01.Volume.Total().TruncateInt64()).To(Equal(int64(1000)))
				Expect(volume01.Volume.MakerVolume.TruncateInt64()).To(Equal(int64(500)))
				Expect(volume01.Volume.TakerVolume.TruncateInt64()).To(Equal(int64(500)))

				volume0 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesByAccAddress(player.Ctx, (*player.Accounts)[0].AccAddress)[0]
				Expect(volume0.Volume.Total().TruncateInt64()).To(Equal(int64(2000)))
				Expect(volume0.Volume.MakerVolume.TruncateInt64()).To(Equal(int64(1500)))
				Expect(volume0.Volume.TakerVolume.TruncateInt64()).To(Equal(int64(500)))
			})
		})

	})

	Context("Mixed spot and derivative markets", func() {

		BeforeEach(func() {
			config := te.TestPlayerConfig{NumAccounts: 4, NumSpotMarkets: 1, NumDerivativeMarkets: 1, NumSubaccounts: 2}
			player = te.InitTest(config, nil)
		})

		When("User buys spot and derivatives", func() {
			BeforeEach(func() {
				newSpot(orderParams{acc: 0, subacc: 0, isLong: true, price: 100, quantity: 10})
				newDerivative(orderParams{acc: 0, subacc: 0, isLong: true, price: 100, quantity: 10})
				newSpot(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				newDerivative(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				player.PerformEndBlockerAction(0, false)
				newSpot(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				newDerivative(orderParams{acc: 0, subacc: 1, isLong: true, price: 100, quantity: 5})
				newSpot(orderParams{acc: 1, isLong: false, price: 100, quantity: 20})
				newDerivative(orderParams{acc: 1, isLong: false, price: 100, quantity: 20})
				player.PerformEndBlockerAction(0, false)
			})

			It("Total volume should equal matched orders notionals", func() {
				volume := player.App.ExchangeKeeper.GetMarketAggregateVolume(player.Ctx, player.FindMarketId(te.MarketType_spot, 0))
				Expect(volume.Total().TruncateInt64()).To(Equal(int64(4000)))
				Expect(volume.TakerVolume.TruncateInt64()).To(Equal(int64(2500)))
				Expect(volume.MakerVolume.TruncateInt64()).To(Equal(int64(1500)))

				volume00 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0, 0))[0]
				Expect(volume00.Volume.Total().TruncateInt64()).To(Equal(int64(1000)))

				volume01 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesBySubaccount(player.Ctx, player.GetSubaccountId(0, 1))[0]
				Expect(volume01.Volume.Total().TruncateInt64()).To(Equal(int64(1000)))
				Expect(volume01.Volume.MakerVolume.TruncateInt64()).To(Equal(int64(500)))
				Expect(volume01.Volume.TakerVolume.TruncateInt64()).To(Equal(int64(500)))

				volume0 := player.App.ExchangeKeeper.GetAllSubaccountMarketAggregateVolumesByAccAddress(player.Ctx, (*player.Accounts)[0].AccAddress)[0]
				Expect(volume0.Volume.Total().TruncateInt64()).To(Equal(int64(2000)))
				Expect(volume0.Volume.MakerVolume.TruncateInt64()).To(Equal(int64(1500)))
				Expect(volume0.Volume.TakerVolume.TruncateInt64()).To(Equal(int64(500)))
			})
		})
	})
})
