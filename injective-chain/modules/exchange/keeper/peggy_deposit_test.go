package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Peggy Deposits", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		bankKeeper       bankkeeper.Keeper
		ctx              sdk.Context
		accounts         []te.Account
		_                te.TestInput
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		balancesTracker  *te.BalancesTracker
		tp               te.TestPlayer
		mainSubaccountId common.Hash
	)

	BeforeEach(func() {
		if !te.IsUsingDefaultSubaccount() {
			Skip("only makes sense with default subaccount [as that where peggy deposits go]")
		}
		mainSubaccountId = te.GetSubaccountId(
			"inj14eutluwnxq3uztlv00kzv5f0e6w8c84p0l3ufw",
		)
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		hooks["setup"] = func(*te.TestPlayerHookParams) {
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

	var setup = func(testSetup te.TestPlayer) {
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		bankKeeper = injectiveApp.BankKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/peggy", file)
		tp = te.LoadReplayableTest(filepath)
		setup(tp)
		simulationError = tp.ReplayTest(te.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	var (
		DEFAULT_PEGGY_DENOM = "peggy0x11A227e1aAb93Ad76646663381A94Fdaa3256FCF"
	)

	When("peggy deposit claim is received", func() {
		Context("with incorrect data", func() {
			When("Ethereum address is used as MsgDeposit sender", func() {
				BeforeEach(func() {
					runTest("p1_deposit_invalid_msg_deposit_sender", true)
				})
				It("processes the deposit and ignores invalid arbitraty data", func() {
					Expect(
						simulationError,
					).To(BeNil(), "error returned when processing deposit claim with invalid sender for the inner MsgDeposit")

					delta := balancesTracker.GetAvailableDepositPlusBankChange(
						tp.App,
						tp.Ctx,
						mainSubaccountId,
						DEFAULT_PEGGY_DENOM,
					)
					Expect(
						delta.String(),
					).To(Equal(sdk.MustNewDecFromStr("123456789").String()), "peggy deposit was not processed")
				})
			})

			When("MsgDeposit signer is different than Deposit sender", func() {
				BeforeEach(func() {
					runTest("p3_deposit_invalid_msg_deposit_signer", true)
				})
				It("processes the deposit and ignores invalid arbitraty data", func() {
					Expect(
						simulationError,
					).To(BeNil(), "error returned when processing deposit claim with differnt sender for the inner MsgDeposit")

					delta := balancesTracker.GetAvailableDepositPlusBankChange(
						tp.App,
						tp.Ctx,
						mainSubaccountId,
						DEFAULT_PEGGY_DENOM,
					)
					Expect(
						delta.String(),
					).To(Equal(sdk.MustNewDecFromStr("123456787").String()), "peggy deposit was not processed")
				})
			})

			When("MsgCreateSpotMarketOrder is incorrect", func() {
				BeforeEach(func() {
					runTest("p6_deposit_invalid_msg_create_spot_market_order", true)
				})
				It(
					"processes the deposit and ignores market order for non-existent market",
					func() {
						Expect(
							simulationError,
						).To(BeNil(), "error returned when processing deposit claim with valid msg")

						delta := balancesTracker.GetAvailableDepositPlusBankChange(
							tp.App,
							tp.Ctx,
							mainSubaccountId,
							DEFAULT_PEGGY_DENOM,
						)
						Expect(
							delta.String(),
						).To(Equal(sdk.MustNewDecFromStr("123456789").String()), "peggy deposit was not processed")

						eth0Delta := balancesTracker.GetAvailableDepositPlusBankChange(
							tp.App,
							tp.Ctx,
							mainSubaccountId,
							"ETH0",
						)

						Expect(
							eth0Delta.String(),
						).To(Equal(sdk.MustNewDecFromStr("0").String()), "msg create market order was processed")
					},
				)
			})

			When("MsgCreateSpotMarketOrder is incorrect", func() {
				BeforeEach(func() {
					runTest(
						"p8_deposit_invalid_msg_create_spot_market_order_incorrect_data_types",
						true,
					)
				})
				It(
					"processes the deposit and ignores invalid market order msg",
					func() {
						Expect(
							simulationError,
						).To(BeNil(), "error returned when processing deposit claim with valid msg")

						delta := balancesTracker.GetAvailableDepositPlusBankChange(
							tp.App,
							tp.Ctx,
							mainSubaccountId,
							DEFAULT_PEGGY_DENOM,
						)
						Expect(
							delta.String(),
						).To(Equal(sdk.MustNewDecFromStr("123456789").String()), "peggy deposit was not processed")

						eth0Delta := balancesTracker.GetAvailableDepositPlusBankChange(
							tp.App,
							tp.Ctx,
							mainSubaccountId,
							"ETH0",
						)

						Expect(
							eth0Delta.String(),
						).To(Equal(sdk.MustNewDecFromStr("0").String()), "msg create market order was processed")
					},
				)
			})
		})

		Context("with correct data", func() {
			When("MsgDeposit is correct", func() {
				BeforeEach(func() {
					runTest("p4_deposit_valid_msg_deposit", true)
				})
				It("processes the deposit and processes subaccount deposit data", func() {
					Expect(
						simulationError,
					).To(BeNil(), "error returned when processing deposit claim with valid msg")

					secondSub, err := types.GetSubaccountIDOrDeriveFromNonce(
						sdk.MustAccAddressFromBech32(
							"inj14eutluwnxq3uztlv00kzv5f0e6w8c84p0l3ufw",
						),
						"1",
					)
					te.OrFail(err)

					secondSubaccountDelta := balancesTracker.GetAvailableDepositPlusBankChange(
						tp.App,
						tp.Ctx,
						secondSub,
						DEFAULT_PEGGY_DENOM,
					)
					Expect(
						secondSubaccountDelta.String(),
					).To(Equal(sdk.MustNewDecFromStr("123456789").String()), "attached MsgDeposit was not processed")
				})
			})

			When("MsgCreateSpotMarketOrder is correct", func() {
				BeforeEach(func() {
					runTest("p5_deposit_valid_msg_create_spot_market_order", true)
				})
				It("processes the deposit and places market order", func() {
					Expect(
						simulationError,
					).To(BeNil(), "error returned when processing deposit claim with valid msg")

					peggyDelta := balancesTracker.GetAvailableDepositPlusBankChange(
						tp.App,
						tp.Ctx,
						mainSubaccountId,
						DEFAULT_PEGGY_DENOM,
					)

					// should be 789 lower than what was deposited
					Expect(
						peggyDelta.String(),
					).To(Equal(sdk.MustNewDecFromStr("123456000").String()), "msg create market order was not processed")
				})
			})

			When("unsupported MsgCreateSpotLimitOrder message is correct", func() {
				BeforeEach(func() {
					runTest("p7_deposit_valid_msg_create_spot_limit_order", true)
				})
				It("processes the deposit and arbitraty data is ignored", func() {
					Expect(
						simulationError,
					).To(BeNil(), "error returned when processing deposit claim with valid msg")

					delta := balancesTracker.GetAvailableDepositPlusBankChange(
						tp.App,
						tp.Ctx,
						mainSubaccountId,
						DEFAULT_PEGGY_DENOM,
					)
					Expect(
						delta.String(),
					).To(Equal(sdk.MustNewDecFromStr("123456789").String()), "peggy deposit was not processed")

					spotOrders := te.GetAllLimitSpotOrdersForMarket(
						tp.App,
						tp.Ctx,
						mainSubaccountId,
						tp.TestInput.Spots[0].MarketID,
					)

					Expect(len(spotOrders)).To(Equal(0), "msg create limit order was processed")
				})
			})
		})

		When("with arbitrary data contains random content", func() {
			BeforeEach(func() {
				runTest("p2_deposit_invalid_arbitrary_data", true)
			})
			It("processes the deposit and ignores invalid arbitraty data", func() {
				Expect(
					simulationError,
				).To(BeNil(), "error returned when processing deposit claim with senseless data")

				delta := balancesTracker.GetAvailableDepositPlusBankChange(
					tp.App,
					tp.Ctx,
					mainSubaccountId,
					DEFAULT_PEGGY_DENOM,
				)
				Expect(
					delta.String(),
				).To(Equal(sdk.MustNewDecFromStr("123456788").String()), "peggy deposit was not processed")
			})
		})
	})
})
