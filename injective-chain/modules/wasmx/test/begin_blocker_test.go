package testwasmx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/InjectiveLabs/injective-core/injective-chain/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"

	"cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"

	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
)

var _ = Describe("Begin blocker registry Test", func() {
	var (
		player te.TestPlayer
	)

	queryRunsCount := func(contractAddr sdk.AccAddress) int {
		queryReplBz := te.MustNotErr(player.App.WasmKeeper.QuerySmart(player.Ctx, contractAddr, []byte(`{"runs":{}}`)))
		var countStr string
		te.OrFail(json.Unmarshal(queryReplBz, &countStr))
		count := te.MustNotErr(strconv.Atoi(countStr))
		return count
	}
	queryActive := func(contractAddr sdk.AccAddress) bool {
		queryReplBz := te.MustNotErr(player.App.WasmKeeper.QuerySmart(player.Ctx, contractAddr, []byte(`{"active":{}}`)))
		var activeStr string
		te.OrFail(json.Unmarshal(queryReplBz, &activeStr))
		isActive := activeStr == "true"
		return isActive
	}

	BeforeEach(func() {
		config := te.TestPlayerConfig{NumAccounts: 2, NumSpotMarkets: 1, InitContractRegistry: true}
		player = te.InitTest(config, nil)
	})

	Context("One contract in begin blocker", func() {
		var contr sdk.AccAddress
		BeforeEach(func() {
			contr = StoreAndRegisterInBBDummyContract(&player, 1000, 100_000)
			player.PerformEndBlockerAction(0, false)
		})

		It("should run in begin blocker", func() {
			Expect(queryRunsCount(contr)).To(Equal(1))
		})
	})

	Context("Not enough gas limit to run all in begin blocker", func() {
		var contr, contr2, contr3, contr4, contr5 sdk.AccAddress
		BeforeEach(func() {
			// dummy execution cost is around 70000, we reserve 3xgas_limit, so 200.000 total limit allows execution of 2 contracts
			player.App.WasmxKeeper.SetParams(player.Ctx, wasmxtypes.Params{
				IsExecutionEnabled:    true,
				MaxBeginBlockTotalGas: 200000,
				MaxContractGasLimit:   200000,
				MinGasPrice:           1000,
			})

			contr = StoreAndRegisterInBBDummyContract(&player, 1005, 100_000)
			contr2 = StoreAndRegisterInBBDummyContract(&player, 1004, 100_000)
			contr3 = StoreAndRegisterInBBDummyContract(&player, 1003, 100_000)
			contr4 = StoreAndRegisterInBBDummyContract(&player, 1002, 100_000)
			contr5 = StoreAndRegisterInBBDummyContract(&player, 1001, 100_000)
			player.PerformEndBlockerAction(0, false)
			// we push first to the last position to ensure that contr3 wasn't before deactivated from out of gas
			te.OrFail(player.ReplayUpdateContractRegistryParams(contr.String(), "", 100_000, 1000))
			player.PerformEndBlockerAction(0, false)
		})

		It("best prices should run in begin blocker", func() {
			Expect(queryRunsCount(contr)).To(Equal(1), "contract 1 (%v)", contr.String())
			Expect(queryRunsCount(contr2)).To(Equal(2), "contract 2 (%v)", contr2.String())
			Expect(queryRunsCount(contr3)).To(Equal(1), "contract 3 (%v)", contr3.String())
			Expect(queryRunsCount(contr4)).To(Equal(0), "contract 4 (%v)", contr4.String())
			Expect(queryRunsCount(contr5)).To(Equal(0), "contract 5 (%v)", contr5.String())
		})
	})

	Context("There are over 10 contracts in begin blocker", func() {
		contracts := make([]sdk.AccAddress, 0)
		BeforeEach(func() {
			for i := 0; i < 15; i++ {
				contracts = append(contracts, StoreAndRegisterInBBDummyContract(&player, uint64(1020-i), 100_000))
			}
			contracts = append(contracts, StoreAndRegisterInBBDummyContract(&player, 1200, 100_000))
			player.PerformEndBlockerAction(0, false)
		})

		It("all should run in begin blocker", func() {
			for i, contract := range contracts {
				Expect(queryRunsCount(contract)).To(Equal(1), "contract %v (%v)", i, contract.String())
			}
		})
	})

	Context("Contract got updated after registering", func() {
		var contract sdk.AccAddress
		BeforeEach(func() {
			contract = StoreAndRegisterInBBDummyContract(&player, 1200, 100_000)
			player.PerformEndBlockerAction(0, false)
			mockedViewKeeper := WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmtypes.ContractInfo {
					resp := wasmtypes.ContractInfo{
						CodeID: uint64(2),
					}
					return &resp
				},
			}
			player.App.WasmxKeeper.SetWasmViewKeeper(mockedViewKeeper)
			player.PerformEndBlockerAction(0, false)
		})

		It("all should run in begin blocker", func() {
			Expect(queryRunsCount(contract)).To(Equal(1), "contract shouldn't run 2nd time due to mismatche code_id")
		})
	})

	Context("One contract in begin blocker - well funded - multiple runs", func() {
		var contr sdk.AccAddress
		var fundsStart, fundsAfter1, fundsAfter2, fundsAfter3 math.Int
		BeforeEach(func() {
			contr = StoreAndRegisterInBBDummyContract(&player, 1000, 100_000, 500000000000)
			fundsStart = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter1 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter2 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter3 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
		})

		It("should run in begin blocker and consume gas", func() {
			Expect(queryRunsCount(contr)).To(Equal(3))
			Expect(fundsStart.GT(fundsAfter1)).To(BeTrue())
			Expect(fundsAfter1.GT(fundsAfter2)).To(BeTrue())
			Expect(fundsAfter2.GT(fundsAfter3)).To(BeTrue())
		})
	})

	Context("One contract in begin blocker - too small gas limit", func() {
		var contr sdk.AccAddress
		var fundsStart, fundsAfter1, fundsAfter2 math.Int
		var active1 bool
		BeforeEach(func() {
			contr = StoreAndRegisterInBBDummyContract(&player, 1000, 40_000, 200_000_000)
			fundsStart = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter1 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			active1 = queryActive(contr)
			player.PerformEndBlockerAction(0, false)
			fundsAfter2 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
		})

		It("should run in begin blocker and consume gas", func() {
			Expect(queryRunsCount(contr)).To(Equal(0))
			Expect(fundsStart.GT(fundsAfter1)).To(BeTrue(), "Gas should be consumed by BB and deactivate")
			Expect(active1).To(BeFalse())
			Expect(fundsAfter2.Equal(fundsAfter1)).To(BeTrue(), "no more gas consumed")
		})
	})

	Context("One contract in begin blocker - enough gas for 1 run - multiple runs", func() {
		var contr sdk.AccAddress
		var fundsStart, fundsAfter1, fundsAfter2, fundsAfter3 math.Int
		var active1, active2 bool
		BeforeEach(func() {
			contr = StoreAndRegisterInBBDummyContract(&player, 1000, 80_000, 240_000_000)
			fundsStart = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter1 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			active1 = queryActive(contr)
			player.PerformEndBlockerAction(0, false)
			fundsAfter2 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			active2 = queryActive(contr)
			player.PerformEndBlockerAction(0, false)
			fundsAfter3 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
		})

		It("should run in begin blocker and consume gas", func() {
			Expect(queryRunsCount(contr)).To(Equal(1))
			Expect(fundsStart.GT(fundsAfter1)).To(BeTrue(), "Gas should be consumed by BB")
			fmt.Println("Gas consumed by BB: ", fundsStart.Sub(fundsAfter1))
			Expect(active1).To(BeTrue())
			Expect(fundsAfter1.GT(fundsAfter2)).To(BeTrue(), "Gas should be consumed by deactivate hook")
			fmt.Println("Gas consumed by deactivate: ", fundsAfter1.Sub(fundsAfter2))
			Expect(active2).To(BeFalse(), "deactivate method should have been called")
			Expect(fundsAfter2.Equal(fundsAfter3)).To(BeTrue(), "no more gas consumed")
			fmt.Println("Consumed gas: ", fundsStart.Sub(fundsAfter1))
		})
	})

	Context("One contract in begin blocker - updated price", func() {
		var contr sdk.AccAddress
		var fundsStart, fundsAfter1, fundsAfter2 math.Int
		BeforeEach(func() {
			contr = StoreAndRegisterInBBDummyContract(&player, 1000, 80_000, 1_000_000_000)
			fundsStart = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter1 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount

			te.OrFail(player.ReplayUpdateContractRegistryParams(contr.String(), "", 80_000, 2000))
			player.PerformEndBlockerAction(0, false)
			fundsAfter2 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
		})

		It("should run once in each begin blocker and consume gas", func() {
			Expect(queryActive(contr)).To(BeTrue())
			Expect(queryRunsCount(contr)).To(Equal(2), "should run once per BB")
			Expect(fundsStart.GT(fundsAfter1)).To(BeTrue())
			Expect(fundsAfter1.GT(fundsAfter2)).To(BeTrue())
			gasConsumed1 := fundsStart.Sub(fundsAfter1)
			gasConsumed2 := fundsAfter1.Sub(fundsAfter2)
			Expect(gasConsumed2).To(Equal(gasConsumed1.Mul(sdk.NewInt(2))), "gas price set to twice in 2nd block")
		})
	})

	Context("One contract in begin blocker - deregistered", func() {
		var contr sdk.AccAddress
		var fundsStart, fundsAfter1, fundsAfter2, fundsAfterDeregister math.Int
		var active1, active2 bool
		BeforeEach(func() {
			contr = StoreAndRegisterInBBDummyContract(&player, 1000, 80_000, 240_000_000)
			fundsStart = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter1 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			active1 = queryActive(contr)
			te.OrFail(player.ReplayDeregisterContracts(&te.ActionBatchDeregisterContracts{
				Title:       "Deregister proposal",
				Description: "Test",
				Contracts:   []string{contr.String()},
			}))
			fundsAfterDeregister = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter2 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			active2 = queryActive(contr)
		})

		It("should run in begin blocker and consume gas", func() {
			Expect(queryRunsCount(contr)).To(Equal(1))
			Expect(fundsStart.GT(fundsAfter1)).To(BeTrue(), "Gas should be consumed by BB")
			Expect(active1).To(BeTrue())
			Expect(fundsAfter1.GT(fundsAfterDeregister)).To(BeTrue(), "Gas should be consumed by deregister hook")
			fmt.Println("Gas consumed by deregister: ", fundsAfter1.Sub(fundsAfterDeregister))
			Expect(active2).To(BeFalse(), "deactivate method should have been called")
			Expect(fundsAfter2.Equal(fundsAfterDeregister)).To(BeTrue(), "no more gas consumed")
		})
	})

	Context("One contract with no begin blocker section", func() {
		var contr sdk.AccAddress
		var fundsStart, fundsAfter1, fundsAfter2 math.Int
		BeforeEach(func() {
			contr = StoreTestContractAndRegisterInBB(&player, "dummy_no_de_hook.wasm", 1000, 40_000, 200_000_000)
			fundsStart = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter1 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
			player.PerformEndBlockerAction(0, false)
			fundsAfter2 = player.App.BankKeeper.GetBalance(player.Ctx, contr, "inj").Amount
		})

		It("should run in begin blocker and not fail on deregister", func() {
			Expect(queryRunsCount(contr)).To(Equal(0))
			Expect(fundsStart.GT(fundsAfter1)).To(BeTrue(), "Gas should be consumed by BB and deactivate")
			Expect(fundsAfter2.Equal(fundsAfter1)).To(BeTrue(), "no more gas consumed")
		})
	})

	Context("Not enough gas limit to run all in begin blocker, one contract gets deactivated", func() {
		var contr, contr2, contr3, contr4, contr5 sdk.AccAddress
		BeforeEach(func() {
			// dummy execution cost is around 70000, and deactivate is 66821, so 200.000 total limit allows execution of 2 contracts
			player.App.WasmxKeeper.SetParams(player.Ctx, wasmxtypes.Params{
				IsExecutionEnabled:    true,
				MaxBeginBlockTotalGas: 250000,
				MaxContractGasLimit:   150000,
				MinGasPrice:           1000,
			})

			contr = StoreAndRegisterInBBDummyContract(&player, 1005, 50_000) // this should get deactivated
			contr2 = StoreAndRegisterInBBDummyContract(&player, 1004, 100_000)
			contr3 = StoreAndRegisterInBBDummyContract(&player, 1003, 100_000)
			contr4 = StoreAndRegisterInBBDummyContract(&player, 1002, 100_000)
			contr5 = StoreAndRegisterInBBDummyContract(&player, 1001, 100_000)
			player.PerformEndBlockerAction(0, false)
		})

		It("best prices should run in begin blocker", func() {
			Expect(queryRunsCount(contr)).To(Equal(0), "contract 1 (%v)", contr.String())
			Expect(queryActive(contr)).To(BeFalse(), "deactivat hook not run")
			Expect(queryRunsCount(contr2)).To(Equal(1), "contract 2 (%v)", contr2.String())
			Expect(queryRunsCount(contr3)).To(Equal(1), "contract 3 (%v)", contr3.String(), "should run as deactivate hook from contr1 consume gas in separate context")
			Expect(queryRunsCount(contr4)).To(Equal(0), "contract 4 (%v)", contr4.String())
			Expect(queryRunsCount(contr5)).To(Equal(0), "contract 5 (%v)", contr5.String())
		})
	})

	Context("Granter contract pays for grantee contract execution", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds               int64
		)

		BeforeEach(func() {
			granterFunds = 10_000_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				5000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				100_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
			)
		})

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)
			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		It("grantee runs on granter's funds", func() {
			allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit

			player.PerformEndBlockerAction(0, false)
			allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit

			Expect(queryRunsCount(granterContr)).To(Equal(1))
			Expect(queryRunsCount(granteeContr)).To(Equal(1))
			Expect(spendLimitBefore.IsAllGT(spendLimitAfter))
		})
	})

	Context("Granter contract does not have enough funds for grantee contract execution", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds               int64
		)

		BeforeEach(func() {
			granterFunds = 500_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				3000,
				100_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
			)
		})

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)
			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		It("grantee does not spend granter's funds", func() {
			allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit

			player.PerformEndBlockerAction(0, false)
			allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
			Expect(queryRunsCount(granterContr)).To(Equal(1))
			Expect(queryRunsCount(granteeContr)).To(Equal(0))
			Expect(spendLimitBefore.IsEqual(spendLimitAfter))
		})
	})

	Context("Granter contract pays for grantee contract execution", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds               int64
		)

		Context("And grantee runs first, but granter doesn't have enough gas", func() {
			BeforeEach(func() {
				granterFunds = 800_000_000

				granterContr = StoreAndRegisterInBBDummyContract2(&player,
					2000,
					100_000,
					wasmxtypes.FundingMode_SelfFunded,
					sdk.AccAddress{},
					granterFunds,
				)

				granteeContr = StoreAndRegisterInBBDummyContract2(&player,
					5000,
					100_000,
					wasmxtypes.FundingMode_GrantOnly,
					granterContr,
				)
			})

			JustBeforeEach(func() {
				grantExpirationTime := time.Now().Add(3 * time.Hour)
				err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
					SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
					Expiration: &grantExpirationTime,
				})
				Expect(err).To(BeNil())
			})

			It("grantee doesn't run, but granter does", func() {
				allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit

				player.PerformEndBlockerAction(0, false)
				allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit

				Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
				Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did run")
				Expect(spendLimitBefore.IsAllGT(spendLimitAfter))
			})
		})
	})

	Context("Grantee contract uses both granter funds and its own", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		When("grantee has enough to pay for execution", func() {
			BeforeEach(func() {
				granterFunds = 500_000_000 // doesn't have enough to pay for grantee as 900_000_000 would be required
				granteeFunds = 5_000_000_000

				granterContr = StoreAndRegisterInBBDummyContract2(&player,
					1000,
					100_000,
					wasmxtypes.FundingMode_SelfFunded,
					sdk.AccAddress{},
					granterFunds,
				)

				granteeContr = StoreAndRegisterInBBDummyContract2(&player,
					3000,
					100_000,
					wasmxtypes.FundingMode_Dual,
					granterContr,
					granteeFunds,
				)
			})

			It("grantee does not spend granter's funds", func() {
				allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
				granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

				player.PerformEndBlockerAction(0, false)
				allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
				granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
				Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
				Expect(queryRunsCount(granteeContr)).To(Equal(1), "grantee contract did not run")
				Expect(spendLimitBefore.IsEqual(spendLimitAfter))
				Expect(granteeInjAfter.IsLT(granteeInjBefore)).To(BeTrue(), "grantee did not pay for it's execution")
			})
		})

		Context("granter doesn't have enough to pay for execution, but together they do", func() {
			BeforeEach(func() {
				granterFunds = 600_000_000
				granteeFunds = 500_000_000

				//runs first, because of higher gas price
				//execution requires at least 300_000 (x3)
				granterContr = StoreAndRegisterInBBDummyContract2(&player,
					2000,
					100_000,
					wasmxtypes.FundingMode_SelfFunded,
					sdk.AccAddress{},
					granterFunds,
				)

				//execution requires at least 600_000
				granteeContr = StoreAndRegisterInBBDummyContract2(&player,
					1000,
					200_000,
					wasmxtypes.FundingMode_Dual,
					granterContr,
					granteeFunds,
				)
			})

			It("grantee doesn't execute", func() {
				allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
				granterInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")
				granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

				player.PerformEndBlockerAction(0, false)
				allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
				granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
				granterInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")
				Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
				Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did not run")
				Expect(spendLimitBefore.IsEqual(spendLimitAfter))
				Expect(granterInjAfter.IsLT(granterInjBefore)).To(BeTrue(), "granter did not pay for it's execution")
				Expect(granteeInjAfter.IsLT(granteeInjBefore)).To(BeTrue(), "grantee paid for it's execution")
				Expect(queryActive(granteeContr)).To(BeFalse(), "grantee contract did not run")
			})
		})

		Context("granter doesn't have enough to pay for execution, but grantee does", func() {
			BeforeEach(func() {
				granterFunds = 600_000_000
				granteeFunds = 600_000_000

				//runs first, because of higher gas price
				//execution requires at least 300_000 (x3)
				granterContr = StoreAndRegisterInBBDummyContract2(&player,
					2000,
					100_000,
					wasmxtypes.FundingMode_SelfFunded,
					sdk.AccAddress{},
					granterFunds,
				)

				//execution requires at least 600_000
				granteeContr = StoreAndRegisterInBBDummyContract2(&player,
					1000,
					200_000,
					wasmxtypes.FundingMode_Dual,
					granterContr,
					granteeFunds,
				)
			})

			It("grantee does execute and pay for itself", func() {
				allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
				granterInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")
				granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

				player.PerformEndBlockerAction(0, false)
				allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
				Expect(err).To(BeNil())
				spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
				granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
				granterInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")
				Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
				Expect(queryRunsCount(granteeContr)).To(Equal(1), "grantee contract did not run")
				Expect(spendLimitBefore.IsEqual(spendLimitAfter))
				Expect(granterInjAfter.IsLT(granterInjBefore)).To(BeTrue(), "granter did not pay for it's execution")
				Expect(granteeInjAfter.IsLT(granteeInjBefore)).To(BeTrue(), "grantee did not pay for it's execution")
			})
		})
	})

	Context("grantee grants to regular INJ address in grant only mode", func() {
		var (
			granterAddr  = sdk.MustAccAddressFromBech32("inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz")
			granteeContr sdk.AccAddress
			granterFunds int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterAddr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterFunds = 600_000_000
			te.OrFail(player.PerformMintAction(sdk.NewInt64Coin("inj", granterFunds), granterAddr, nil))
			granteeContr = StoreDummyContract(&player,
				2000,
				100_000,
				600_000_000,
			)
			grantOnly := wasmxtypes.FundingMode_GrantOnly

			RegisterInBB(&player,
				2000,
				100000,
				grantOnly,
				granteeContr,
				granterAddr,
			)
		})

		It("grantee contract does execute", func() {
			allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterAddr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
			granterInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granterAddr, "inj")

			player.PerformEndBlockerAction(0, false)
			allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterAddr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
			granterInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granterAddr, "inj")
			Expect(queryRunsCount(granteeContr)).To(Equal(1), "grantee contract did not run")
			Expect(spendLimitBefore.IsEqual(spendLimitAfter))
			Expect(granterInjAfter.IsLT(granterInjBefore)).To(BeTrue(), "granter did not pay for it's execution")
		})
	})

	Context("grantee grants to itself in grant only mode", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds               int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterContr = StoreDummyContract(&player,
				2000,
				100_000,
				600_000_000,
			)
			granteeContr = granterContr

			grantOnly := wasmxtypes.FundingMode_GrantOnly

			RegisterInBB(&player,
				2000,
				100000,
				grantOnly,
				granterContr,
				granterContr,
			)
		})

		It("granter does execute", func() {
			allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
			granterInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")

			player.PerformEndBlockerAction(0, false)
			allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
			granterInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(spendLimitBefore.IsEqual(spendLimitAfter))
			Expect(granterInjAfter.IsLT(granterInjBefore)).To(BeTrue(), "granter did not pay for it's execution")
		})
	})

	Context("grantee grants to itself in dual mode", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds               int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterContr = StoreDummyContract(&player,
				2000,
				100_000,
				600_000_000,
			)
			granteeContr = granterContr

			grantOnly := wasmxtypes.FundingMode_Dual

			RegisterInBB(&player,
				2000,
				100000,
				grantOnly,
				granterContr,
				granterContr,
			)
		})

		It("granter does execute", func() {
			allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
			granterInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")

			player.PerformEndBlockerAction(0, false)
			allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
			granterInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granterContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(spendLimitBefore.IsEqual(spendLimitAfter))
			Expect(granterInjAfter.IsLT(granterInjBefore)).To(BeTrue(), "granter did not pay for it's execution")
		})
	})

	Context("grantee grants non-existent granter", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			contractFunds              int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(contractFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			contractFunds = 600_000_000
			granterContr = sdk.MustAccAddressFromBech32("inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz")
			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				2000,
				100_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
				contractFunds,
			)
		})

		It("grantee does not execute", func() {
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

			player.PerformEndBlockerAction(0, false)

			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did run")
			Expect(granteeInjBefore.IsEqual(granteeInjAfter)).To(BeTrue(), "grantee paid for it's execution")
		})
	})

	Context("Grantee grants to granter, granter to grantee - circular granting", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())

			err = player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granteeContr, granterContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granteeFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterFunds = 5_000_000_000
			granteeFunds = 5_000_000_000

			granterContr = StoreDummyContract(&player,
				1000,
				100_000,
				600_000_000,
			)
			granteeContr = StoreDummyContract(&player,
				1000,
				100_000,
				600_000_000,
			)
			fundingMode := wasmxtypes.FundingMode_GrantOnly

			RegisterInBB(&player,
				2000,
				100000,
				fundingMode,
				granteeContr,
				granterContr,
			)

			RegisterInBB(&player,
				2000,
				100000,
				fundingMode,
				granterContr,
				granteeContr,
			)
		})

		It("executes them both and charges them gas", func() {
			allowance, err := player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitBefore := allowance.(*feegrant.BasicAllowance).SpendLimit
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

			player.PerformEndBlockerAction(0, false)
			allowance, err = player.App.FeeGrantKeeper.GetAllowance(player.Ctx, granterContr, granteeContr)
			Expect(err).To(BeNil())
			spendLimitAfter := allowance.(*feegrant.BasicAllowance).SpendLimit
			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(queryRunsCount(granteeContr)).To(Equal(1), "grantee contract did not run")
			Expect(spendLimitBefore.IsEqual(spendLimitAfter))
			Expect(granteeInjAfter.IsLT(granteeInjBefore)).To(BeTrue(), "grantee did not pay for it's execution")
		})
	})

	Context("No allowance is set", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
		)

		BeforeEach(func() {
			granterFunds = 5_000_000_000
			granteeFunds = 5_000_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				2000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			//execution requires at least 600_000
			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				200_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
				granteeFunds,
			)
		})

		It("grantee contract is not executed nor it pays for gas", func() {
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

			player.PerformEndBlockerAction(0, false)
			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did run")
			Expect(granteeInjAfter.IsEqual(granteeInjBefore)).To(BeTrue(), "grantee did pay for it's execution")
		})
	})

	Context("Grant allowance is expired", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
			blockTime                  time.Time
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(1 * time.Second)
			//when contract runs grant will be expired
			blockTime = grantExpirationTime.Add(1 * time.Second)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterFunds = 5_000_000_000
			granteeFunds = 5_000_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				2000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				200_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
				granteeFunds,
			)
		})

		It("grantee contract is not executed nor it pays for gas", func() {
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			player.Ctx = player.Ctx.WithBlockTime(blockTime)

			player.PerformEndBlockerAction(0, false)
			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did run")
			Expect(granteeInjAfter.IsEqual(granteeInjBefore)).To(BeTrue(), "grantee did pay for it's execution")
		})
	})

	Context("Grant allowance is too low", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(1 * time.Second)

			err := player.App.FeeGrantKeeper.GrantAllowance(player.Ctx, granterContr, granteeContr, &feegrant.BasicAllowance{
				SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(300_000))),
				Expiration: &grantExpirationTime,
			})
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterFunds = 5_000_000_000
			granteeFunds = 5_000_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				2000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				200_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
				granteeFunds,
			)
		})

		It("grantee contract is not executed nor it pays for gas", func() {
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

			player.PerformEndBlockerAction(0, false)
			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did run")
			Expect(granteeInjAfter.IsEqual(granteeInjBefore)).To(BeTrue(), "grantee did pay for it's execution")
		})
	})

	Context("Grant allowance is periodic and execution exceeds its PeriodSpendLimit", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(
				player.Ctx,
				granterContr,
				granteeContr,
				&feegrant.PeriodicAllowance{
					Basic: feegrant.BasicAllowance{
						SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
						Expiration: &grantExpirationTime,
					},
					Period:           time.Hour,
					PeriodSpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(1))),
					PeriodCanSpend:   nil,
					PeriodReset:      time.Now().Add(time.Hour),
				},
			)
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterFunds = 5_000_000_000
			granteeFunds = 5_000_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				2000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				200_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
				granteeFunds,
			)
		})

		It("grantee contract is not executed nor it pays for gas", func() {
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

			player.PerformEndBlockerAction(0, false)
			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(queryRunsCount(granteeContr)).To(Equal(0), "grantee contract did run")
			Expect(granteeInjAfter.IsEqual(granteeInjBefore)).To(BeTrue(), "grantee did pay for it's execution")
		})
	})

	Context("Grant allowance is periodic and execution exceeds its PeriodSpendLimit", func() {
		var (
			granterContr, granteeContr sdk.AccAddress
			granterFunds, granteeFunds int64
		)

		JustBeforeEach(func() {
			grantExpirationTime := time.Now().Add(3 * time.Hour)

			err := player.App.FeeGrantKeeper.GrantAllowance(
				player.Ctx,
				granterContr,
				granteeContr,
				&feegrant.PeriodicAllowance{
					Basic: feegrant.BasicAllowance{
						SpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
						Expiration: &grantExpirationTime,
					},
					Period:           time.Hour,
					PeriodSpendLimit: sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
					PeriodCanSpend:   sdk.NewCoins(types.NewInjectiveCoin(sdk.NewInt(granterFunds))),
					PeriodReset:      time.Now().Add(time.Hour),
				},
			)
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			granterFunds = 5_000_000_000
			granteeFunds = 5_000_000_000

			granterContr = StoreAndRegisterInBBDummyContract2(&player,
				2000,
				100_000,
				wasmxtypes.FundingMode_SelfFunded,
				sdk.AccAddress{},
				granterFunds,
			)

			granteeContr = StoreAndRegisterInBBDummyContract2(&player,
				1000,
				200_000,
				wasmxtypes.FundingMode_GrantOnly,
				granterContr,
				granteeFunds,
			)
		})

		It("grantee contract is not executed nor it pays for gas", func() {
			granteeInjBefore := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")

			player.PerformEndBlockerAction(0, false)
			granteeInjAfter := player.App.BankKeeper.GetBalance(player.Ctx, granteeContr, "inj")
			Expect(queryRunsCount(granterContr)).To(Equal(1), "granter contract did not run")
			Expect(queryRunsCount(granteeContr)).To(Equal(1), "grantee contract did run")
			Expect(granteeInjAfter.IsEqual(granteeInjBefore)).To(BeTrue(), "grantee did pay for it's execution")
		})
	})
})
