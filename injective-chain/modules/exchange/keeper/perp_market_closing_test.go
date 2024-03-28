package keeper_test

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Derivative markets closing", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		bankKeeper       bankkeeper.Keeper
		ctx              sdk.Context
		accounts         []te.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		testInput        te.TestInput
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		balancesTracker  *te.BalancesTracker
		tp               te.TestPlayer
		USDT             string = "USDT0"
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		hooks["setup-position"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				subaccountID := common.HexToHash(acc.SubaccountIDs[0])
				bankBalances := bankKeeper.GetAllBalances(
					ctx,
					types.SubaccountIDToSdkAddress(subaccountID),
				)
				deposits := keeper.GetDeposits(ctx, subaccountID)

				balancesTracker.SetBankBalancesAndSubaccountDeposits(
					subaccountID,
					bankBalances,
					deposits,
				)
			}
		}
	})

	var getTotalDepositPlusBankChange = func(accountIdx int, denom string) sdk.Dec {
		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		deposit := keeper.GetDeposit(ctx, subaccountID, denom)
		bankBalance := sdk.ZeroInt()
		if types.IsDefaultSubaccountID(subaccountID) {
			accountAddr := types.SubaccountIDToSdkAddress(subaccountID)
			bankBalance = bankKeeper.GetBalance(ctx, accountAddr, denom).Amount
		}

		funds := balancesTracker.GetTotalBalancePlusBank(accountIdx, denom)
		balanceDelta := deposit.TotalBalance.Add(bankBalance.ToDec()).Sub(funds)
		return balanceDelta
	}

	var setup = func(testSetup te.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		bankKeeper = injectiveApp.BankKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		if len(testInput.Spots) > 0 {
			marketId = testInput.Spots[0].MarketID
		}
		if len(testInput.Perps) > 0 {
			marketId = testInput.Perps[0].MarketID
		}
		if len(testInput.BinaryMarkets) > 0 {
			marketId = testInput.BinaryMarkets[0].MarketID
		}
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/perp/market_closing", file)
		tp = te.LoadReplayableTest(filepath)
		setup(tp)
		simulationError = tp.ReplayTest(testexchange.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printOrders := func(orders []*types.TrimmedDerivativeLimitOrder) {
		fmt.Println(
			"Orders: ",
			testexchange.GetReadableSlice(
				orders,
				" | ",
				func(ord *types.TrimmedDerivativeLimitOrder) string {
					ro := ""
					if ord.Margin.IsZero() {
						ro = " ro"
					}
					side := "sell"
					if ord.IsBuy {
						side = "buy"
					}
					return fmt.Sprintf(
						"p:%v(q:%v%v) side:%v",
						ord.Price.TruncateInt(),
						ord.Fillable.TruncateInt(),
						ro,
						side,
					)
				},
			),
		)
	}
	_ = printOrders

	var verifyPosition = func(quantity int64, isLong bool) {
		testexchange.VerifyPosition(injectiveApp, ctx, mainSubaccountId, marketId, quantity, isLong)
	}
	_ = verifyPosition

	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedDerivativeLimitOrder {
		return testexchange.GetAllDerivativeOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	var getAllOrdersSorted = func() []*types.TrimmedDerivativeLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	var verifyOrder = testexchange.VerifyDerivativeOrder
	_ = verifyOrder

	var getAllConditionalOrdersSorted = func(subaccountId common.Hash) []*types.TrimmedDerivativeConditionalOrder {
		orders := keeper.GetAllSubaccountConditionalOrders(ctx, marketId, subaccountId)
		sort.SliceStable(orders, func(i, j int) bool {
			if orders[i].TriggerPrice.Equal(orders[j].TriggerPrice) {
				if orders[i].Quantity.Equal(orders[j].Quantity) {
					return !orders[i].IsBuy
				}
				return orders[i].Quantity.GT(orders[j].Quantity)
			}
			return orders[i].TriggerPrice.LT(orders[j].TriggerPrice)
		})
		return orders
	}
	_ = getAllConditionalOrdersSorted

	var getAllConditionalOrdersSortedForMainAccount = func() []*types.TrimmedDerivativeConditionalOrder {
		return getAllConditionalOrdersSorted(mainSubaccountId)
	}

	_ = getAllConditionalOrdersSortedForMainAccount

	var calculateReturn = func(settlementPrice, positionQuantity, positionEntryPrice, positionMargin sdk.Dec, isLong bool, funding *types.PerpetualMarketFunding) sdk.Dec {
		var pnlNotional sdk.Dec
		if isLong {
			pnlNotional = positionQuantity.Mul(settlementPrice.Sub(positionEntryPrice))
		} else {
			pnlNotional = positionQuantity.Mul(settlementPrice.Sub(positionEntryPrice)).Neg()
		}

		if funding != nil {
			unrealizedFundingPayment := positionQuantity.Mul(funding.CumulativeFunding)

			if isLong {
				positionMargin = positionMargin.Sub(unrealizedFundingPayment)
			} else {
				positionMargin = positionMargin.Add(unrealizedFundingPayment)
			}
		}

		return positionMargin.Add(pnlNotional)
	}

	_ = calculateReturn

	Context("When market is force-settled", func() {

		Context("And settlement price was provided", func() {

			Context("And there was no socialised loss", func() {

				Context("And there were no funding payments", func() {

					When("And user has a long position at loss", func() {
						BeforeEach(func() {
							runTest("mc1_forced_no_funding_long_at_loss", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								nil,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a long position at gain", func() {
						BeforeEach(func() {
							runTest("mc1_forced_no_funding_long_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position
							expectedReturn := calculateReturn(
								f2d(120),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								nil,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a short position at loss", func() {
						BeforeEach(func() {
							runTest("mc1_forced_no_funding_short_at_loss", true)
						})
						It(
							"all orders are canceled and long position is settled correctly",
							func() {
								orders := getAllOrdersSorted()
								Expect(len(orders)).To(Equal(0), "there were resting orders")

								condOrders := getAllConditionalOrdersSortedForMainAccount()
								Expect(
									len(condOrders),
								).To(Equal(0), "there were resting conditional orders")

								//we expect the user to get all margin hold from resting orders plus some deposit from closed position
								expectedReturn := calculateReturn(
									f2d(120),
									f2d(10),
									f2d(100),
									f2d(1000),
									false,
									nil,
								)
								Expect(
									getTotalDepositPlusBankChange(0, USDT).String(),
								).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
							},
						)
					})

					When("And user has a short position at gain", func() {
						BeforeEach(func() {
							runTest("mc1_forced_no_funding_short_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								false,
								nil,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})
				})

				Context("And there were funding payments", func() {

					When("And user has a long position at loss", func() {
						BeforeEach(func() {
							runTest("mc2_forced_with_funding_long_at_loss", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(10),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								&funding,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a long position at gain", func() {
						BeforeEach(func() {
							runTest("mc2_forced_with_funding_long_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(-10),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(120),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								&funding,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a short position at loss", func() {
						BeforeEach(func() {
							runTest("mc2_forced_with_funding_short_at_loss", true)
						})
						It(
							"all orders are canceled and long position is settled correctly",
							func() {
								orders := getAllOrdersSorted()
								Expect(len(orders)).To(Equal(0), "there were resting orders")

								condOrders := getAllConditionalOrdersSortedForMainAccount()
								Expect(
									len(condOrders),
								).To(Equal(0), "there were resting conditional orders")

								funding := types.PerpetualMarketFunding{
									CumulativeFunding: f2d(-12),
									CumulativePrice:   f2d(110),
									LastTimestamp:     5,
								}

								//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
								expectedReturn := calculateReturn(
									f2d(120),
									f2d(10),
									f2d(100),
									f2d(1000),
									false,
									&funding,
								)
								Expect(
									getTotalDepositPlusBankChange(0, USDT).String(),
								).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
							},
						)
					})

					When("And user has a short position at gain", func() {
						BeforeEach(func() {
							runTest("mc2_forced_with_funding_short_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")
							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(20),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								false,
								&funding,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})
				})
			})

			PContext("And there was socialised loss", func() {
				When("And user has a short position at gain", func() {
					BeforeEach(func() {
						runTest("mc3_forced_with_funding_short_at_gain_with_haircut", true)
					})
					It(
						"all orders are canceled and position is settled correctly after haircut",
						func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")
							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(200),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//todo figure out how to calculate expected loss and haircut

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								false,
								&funding,
							)
							fmt.Printf("expectedReturn: %v\n", expectedReturn)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						},
					)
				})
			})
		})

		Context("And settlement price was not provided", func() {

			Context("And there was no socialised loss", func() {

				Context("And there were no funding payments", func() {

					When("And user has a long position at loss", func() {
						BeforeEach(func() {
							runTest("mc1_forced_oracle_price_no_funding_long_at_loss", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								nil,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a long position at gain", func() {
						BeforeEach(func() {
							runTest("mc1_forced_oracle_price_no_funding_long_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position
							expectedReturn := calculateReturn(
								f2d(120),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								nil,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a short position at loss", func() {
						BeforeEach(func() {
							runTest("mc1_forced_oracle_price_no_funding_short_at_loss", true)
						})
						It(
							"all orders are canceled and long position is settled correctly",
							func() {
								orders := getAllOrdersSorted()
								Expect(len(orders)).To(Equal(0), "there were resting orders")

								condOrders := getAllConditionalOrdersSortedForMainAccount()
								Expect(
									len(condOrders),
								).To(Equal(0), "there were resting conditional orders")

								//we expect the user to get all margin hold from resting orders plus some deposit from closed position
								expectedReturn := calculateReturn(
									f2d(120),
									f2d(10),
									f2d(100),
									f2d(1000),
									false,
									nil,
								)
								Expect(
									getTotalDepositPlusBankChange(0, USDT).String(),
								).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
							},
						)
					})

					When("And user has a short position at gain", func() {
						BeforeEach(func() {
							runTest("mc1_forced_oracle_price_no_funding_short_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								false,
								nil,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})
				})

				Context("And there were funding payments", func() {

					When("And user has a long position at loss", func() {
						BeforeEach(func() {
							runTest("mc2_forced_oracle_price_with_funding_long_at_loss", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(10),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								&funding,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a long position at gain", func() {
						BeforeEach(func() {
							runTest("mc2_forced_oracle_price_with_funding_long_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")

							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(-10),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(120),
								f2d(10),
								f2d(100),
								f2d(1000),
								true,
								&funding,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})

					When("And user has a short position at loss", func() {
						BeforeEach(func() {
							runTest("mc2_forced_oracle_price_with_funding_short_at_loss", true)
						})
						It(
							"all orders are canceled and long position is settled correctly",
							func() {
								orders := getAllOrdersSorted()
								Expect(len(orders)).To(Equal(0), "there were resting orders")

								condOrders := getAllConditionalOrdersSortedForMainAccount()
								Expect(
									len(condOrders),
								).To(Equal(0), "there were resting conditional orders")

								funding := types.PerpetualMarketFunding{
									CumulativeFunding: f2d(-12),
									CumulativePrice:   f2d(110),
									LastTimestamp:     5,
								}

								//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
								expectedReturn := calculateReturn(
									f2d(120),
									f2d(10),
									f2d(100),
									f2d(1000),
									false,
									&funding,
								)
								Expect(
									getTotalDepositPlusBankChange(0, USDT).String(),
								).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
							},
						)
					})

					When("And user has a short position at gain", func() {
						BeforeEach(func() {
							runTest("mc2_forced_oracle_price_with_funding_short_at_gain", true)
						})
						It("all orders are canceled and position is settled correctly", func() {
							orders := getAllOrdersSorted()
							Expect(len(orders)).To(Equal(0), "there were resting orders")

							condOrders := getAllConditionalOrdersSortedForMainAccount()
							Expect(
								len(condOrders),
							).To(Equal(0), "there were resting conditional orders")
							funding := types.PerpetualMarketFunding{
								CumulativeFunding: f2d(20),
								CumulativePrice:   f2d(110),
								LastTimestamp:     5,
							}

							//we expect the user to get all margin hold from resting orders plus some deposit from closed position modified by funding payments
							expectedReturn := calculateReturn(
								f2d(90),
								f2d(10),
								f2d(100),
								f2d(1000),
								false,
								&funding,
							)
							Expect(
								getTotalDepositPlusBankChange(0, USDT).String(),
							).To(Equal(expectedReturn.String()), "USDT deposit was incorrect after force-settling market")
						})
					})
				})
			})

			PContext("And there was socialised loss", func() {
				When("And user has a short position at gain", func() {
					//	todo: add once mc3_forced_with_funding_short_at_gain_with_haircut is implemented
				})
			})
		})

	})

	Context("When market params are updated with both change fee and market is closed", func() {

		Context("market is derivative", func() {
			BeforeEach(func() {
				runTest("mc4_derivative_update_fees_demolish", true)
			})
			It("all orders are canceled and long position is settled correctly", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0), "there were resting orders")
				Expect(tp.GetAvailableDepositChange(0, "USDT0").MustFloat64()).To(Equal(0.0))
				Expect(tp.GetTotalDepositChange(0, "USDT0").MustFloat64()).To(Equal(0.0))
			})
		})

		Context("market is spot", func() {
			BeforeEach(func() {
				runTest("mc4_spot_update_fees_demolish", true)
			})
			It("all orders are canceled and long position is settled correctly", func() {
				Expect(tp.GetAvailableDepositChange(0, "USDT0").MustFloat64()).To(Equal(0.0))
				Expect(tp.GetTotalDepositChange(0, "USDT0").MustFloat64()).To(Equal(0.0))
			})
		})

		Context("market is binary", func() {
			BeforeEach(func() {
				runTest("mc4_binary_update_fees_demolish", true)
			})
			It("all orders are canceled and long position is settled correctly", func() {
				Expect(tp.GetAvailableDepositChange(0, "USDT0").MustFloat64()).To(Equal(0.0))
				Expect(tp.GetTotalDepositChange(0, "USDT0").MustFloat64()).To(Equal(0.0))
			})
		})
	})

	Context("When market is paused", func() {

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc5_perp_paused", true)
			})
			It("they are not cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(2), "there were no resting orders")
			})
		})

		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc5_perp_paused_cancel", false)
			})
			It("they can be cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1), "resting order was not canceled")
			})
		})
	})

	Context("When market is expired", func() {
		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc6_perp_expired", true)
			})
			It("they are cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0), "there were resting orders")
			})
		})
	})

	Context("When market is demolished", func() {
		Context("And there are resting limit orders", func() {
			BeforeEach(func() {
				runTest("mc7_perp_demolished", true)
			})
			It("they are cancelled", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0), "there were resting orders")
			})
		})
	})
})
