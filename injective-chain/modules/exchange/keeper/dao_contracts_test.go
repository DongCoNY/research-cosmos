package keeper_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
)

var _ = Describe("DAO contracts test", func() {
	var (
		err                                                        error
		player                                                     te.TestPlayer
		app                                                        *app.InjectiveApp
		ctx                                                        sdk.Context
		coreAddr, preProposeAddr, votingAddr, proposalAddr, cfAddr sdk.AccAddress
		sender                                                     string
		nativeToken                                                = "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/mito"
	)

	executeContract := func(sender, contractAddr, msg string, funds sdk.Coins) *types.MsgExecuteContractResponse {
		msgServer := wasmkeeper.NewMsgServerImpl(&app.WasmKeeper)
		res, err := msgServer.ExecuteContract(
			sdk.WrapSDKContext(ctx),
			&types.MsgExecuteContract{Sender: sender, Contract: contractAddr, Msg: types.RawContractMessage(msg), Funds: funds},
		)
		if err != nil {
			te.OrFail(fmt.Errorf("error during contract execution: %w (msg: %s)", err, msg))
		}
		return res
	}

	setupDao := func(votingCodeId uint64, votingParams, proposalDenom string) {
		adminFactoryAddress, _, _ := player.RegisterAndInitializeContract("../wasm/cw_admin_factory.wasm", "admin-factory", sender, map[string]any{}, nil)
		preProposeCodeId := te.MustNotErr(player.StoreContract("../wasm/dao_pre_propose_single.wasm", sender))
		proposalCodeId := te.MustNotErr(player.StoreContract("../wasm/dao_proposal_single.wasm", sender))
		coreCodeId := te.MustNotErr(player.StoreContract("../wasm/dao_core.wasm", sender))

		preProposeParams := fmt.Sprintf(`{
			"deposit_info": {
				"denom": {"token": {"denom": {"native": "%s"}}},
				"amount": "10",
				"refund_policy": "never"
			},
			"open_proposal_submission": false,
			"extension": {}
		}`, proposalDenom)
		proposalParams := fmt.Sprintf(`{
				"threshold": {"absolute_percentage": {"percentage": {"majority": {}}}},
				"max_voting_period": {"height": 10},
				"only_members_execute": false,
				"allow_revoting": false,
				"pre_propose_info": {"module_may_propose": {
					"info": {
						"code_id": %d,
						"msg": "%s",
						"admin": {"core_module": {}},
						"label": "DAO pre-propose module"
					}
				}},
				"close_proposal_on_execution_failure": true
			}`, preProposeCodeId, base64.StdEncoding.EncodeToString([]byte(preProposeParams)))
		proposalInstantiationMsg := fmt.Sprintf(`{
				"code_id": %d,
				"msg": "%s",
				"admin": {"core_module": {}},
				"label": "dao-proposal"
			}`, proposalCodeId, base64.StdEncoding.EncodeToString([]byte(proposalParams)))

		votingInstantiationMsg := fmt.Sprintf(`{
				"code_id": %d,
				"msg": "%s",
				"label": "dao-voting-power"
			}`, votingCodeId, base64.StdEncoding.EncodeToString([]byte(votingParams)))

		coreInstantiationMsg := fmt.Sprintf(`{
				"name": "DAO",
				"description": "DAO governance",
				"voting_module_instantiate_info": %s,
				"proposal_modules_instantiate_info": [%s],
				"automatically_add_cw20s": false,
				"automatically_add_cw721s": false

			}`, votingInstantiationMsg, proposalInstantiationMsg)

		// instantiate DAO core through admin factory
		factoryExecutionMsg := fmt.Sprintf(`{
			"instantiate_contract_with_self_admin": {
				"instantiate_msg": "%s",
				"code_id": %d,
				"label": "dao-core"
			}
		}`, base64.StdEncoding.EncodeToString([]byte(coreInstantiationMsg)), coreCodeId)

		executeContract(sender, adminFactoryAddress.String(), factoryExecutionMsg, sdk.Coins{})

		app.WasmKeeper.IterateContractsByCode(ctx, coreCodeId, func(addr sdk.AccAddress) bool {
			coreAddr = addr
			return true
		})
		app.WasmKeeper.IterateContractsByCode(ctx, preProposeCodeId, func(addr sdk.AccAddress) bool {
			preProposeAddr = addr
			return true
		})
		app.WasmKeeper.IterateContractsByCode(ctx, votingCodeId, func(addr sdk.AccAddress) bool {
			votingAddr = addr
			return true
		})
		app.WasmKeeper.IterateContractsByCode(ctx, proposalCodeId, func(addr sdk.AccAddress) bool {
			proposalAddr = addr
			return true
		})

		// mint native tokens
		coin := sdk.NewCoin(nativeToken, sdk.NewInt(20))
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sdk.MustAccAddressFromBech32(sender), sdk.NewCoins(coin))

		// init community fund
		cfAddr, _, err = player.RegisterAndInitializeContract("../wasm/community_fund.wasm", "community-fund", coreAddr.String(), map[string]any{"owner": coreAddr.String()}, nil)
		te.OrFail(err)

		// send some MITO otkens to community fund
		// mint native tokens
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sdk.MustAccAddressFromBech32(cfAddr.String()), sdk.NewCoins(coin))
	}

	BeforeEach(func() {
		config := te.TestPlayerConfig{NumAccounts: 2, NumSpotMarkets: 1, InitContractRegistry: true}
		player = te.InitTest(config, nil)
		app = player.App
		ctx = player.Ctx
		sender = (*player.Accounts)[0].AccAddress.String()
	})

	Context("proposal using dao contracts", func() {
		JustBeforeEach(func() {
			votingCodeId := te.MustNotErr(player.StoreContract("../wasm/dao_voting_native_staked.wasm", sender))
			votingParams := fmt.Sprintf(`{"denom": "%s"}`, nativeToken)
			setupDao(votingCodeId, votingParams, nativeToken)
		})

		It("should instantiate all modules with DAO as an admin", func() {
			coreAdminBz := te.MustNotErr(app.WasmKeeper.QuerySmart(ctx, coreAddr, []byte(`{"admin":{}}`)))
			var coreAdmin string
			te.OrFail(json.Unmarshal(coreAdminBz, &coreAdmin))
			Expect(coreAddr.String()).To(Equal(coreAdmin))
		})

		It("should create proposal successfully, vote on it and execute it", func() {
			senderAddr, _ := sdk.AccAddressFromBech32(sender)
			initBalance := app.BankKeeper.GetBalance(ctx, senderAddr, nativeToken)
			Expect(initBalance.Amount.String()).To(Equal("20"))
			// first become a DAO member by staking to voting module
			executeContract(sender, votingAddr.String(), `{"stake": {}}`, sdk.Coins{sdk.Coin{Denom: nativeToken, Amount: sdk.NewInt(10)}})
			// stakes only counted starting the next block
			ctx, _ = te.EndBlockerAndCommit(app, ctx)

			// then create a proposal
			// on success, it will:
			// 1. withdraw all deposit funds from pre-propose module to DAO module (10 tokens)
			// 2. send all funds from community fund to sender (20 tokens)
			// 3. send all funds from DAO to sender address (10 tokens)
			cfMsg := fmt.Sprintf(`{"execute_msgs": {"msgs": [{"bank": {
				"send": {
					"to_address": "%s",
					"amount": [{"denom": "%s", "amount": "20"}]
				}
			}}]}}`, sender, nativeToken)
			proposalMsg := fmt.Sprintf(`{"propose": {
				"msg": { 
					"propose": {
						"title": "My first proposal",
						"description": "Who needs this?",
						"msgs": [
							{"wasm": {
								"execute": {
									"contract_addr": "%s",
									"msg": "%s",
									"funds": []
								}
							}},
							{"wasm": {
								"execute": {
									"contract_addr": "%s",
									"msg": "%s",
									"funds": []
								}
							}},
							{"bank": {
								"send": {
									"to_address": "%s",
									"amount": [{"denom": "%s", "amount": "10"}]
								}
							}}
						]
					}
				}
			}}`, cfAddr, base64.StdEncoding.EncodeToString([]byte(cfMsg)), preProposeAddr, base64.StdEncoding.EncodeToString([]byte(`{"withdraw": {}}`)), sender, nativeToken)
			executeContract(sender, preProposeAddr.String(), proposalMsg, sdk.Coins{sdk.Coin{Denom: nativeToken, Amount: sdk.NewInt(10)}})

			// get last proposal id
			proposalIdBz := te.MustNotErr(app.WasmKeeper.QuerySmart(ctx, proposalAddr, []byte(`{"next_proposal_id":{}}`)))
			var proposalId uint64
			te.OrFail(json.Unmarshal(proposalIdBz, &proposalId))
			proposalId--

			// then vote for it
			voteMsg := fmt.Sprintf(`{"vote": {
				"proposal_id": %d,
				"vote": "yes",
				"rationale": "Because I can?"
			}}`, proposalId)
			executeContract(sender, proposalAddr.String(), voteMsg, sdk.Coins{})

			// now check proposal status
			proposalRespBz := te.MustNotErr(app.WasmKeeper.QuerySmart(ctx, proposalAddr, []byte(fmt.Sprintf(`{"proposal":{"proposal_id": %d}}`, proposalId))))
			var proposalResp map[string]any
			te.OrFail(json.Unmarshal(proposalRespBz, &proposalResp))
			proposal := proposalResp["proposal"].(map[string]any)
			votes := proposal["votes"].(map[string]any)

			Expect(proposal["status"]).To(Equal("passed"))
			Expect(proposal["total_power"]).To(Equal("10"))
			Expect(votes["yes"]).To(Equal("10"))

			// now execute passed proposal
			beforeBalance := app.BankKeeper.GetBalance(ctx, senderAddr, nativeToken)
			Expect(beforeBalance.Amount.String()).To(Equal("0"))
			executeContract(sender, proposalAddr.String(), fmt.Sprintf(`{"execute":{"proposal_id": %d}}`, proposalId), sdk.Coins{})
			afterBalance := app.BankKeeper.GetBalance(ctx, senderAddr, nativeToken)
			Expect(afterBalance.Amount.String()).To(Equal("30"))
		})
	})

	Context("proposal using mito staking contract", func() {
		var allocatorContractAddress, stakingContractAddress, lpDenom string
		JustBeforeEach(func() {
			allocatorContractAddress, stakingContractAddress, lpDenom = player.MitoStakingSetup(player.TestInput.Spots[0].QuoteDenom, sdk.MustAccAddressFromBech32(sender), 0)

			votingCodeId := te.MustNotErr(player.StoreContract("../wasm/staking_voting.wasm", sender))
			votingInstantiationMsg := fmt.Sprintf(`{
				"lp_denom": "%s",
				"staking_contract_addr": "%s"
			}`, lpDenom, stakingContractAddress)
			setupDao(votingCodeId, votingInstantiationMsg, nativeToken)

			te.OrFail(player.PerformMintAction(sdk.Coin{Denom: lpDenom, Amount: sdk.NewInt(100)}, sdk.MustAccAddressFromBech32(sender), nil))
			player.MitoStakingAllocate(sdk.MustAccAddressFromBech32(sender), allocatorContractAddress, lpDenom, 1, 101, []sdk.Coin{{Denom: "inj", Amount: sdk.NewInt(100)}})
		})

		It("should instantiate all modules with DAO as an admin", func() {
			coreAdminBz := te.MustNotErr(app.WasmKeeper.QuerySmart(ctx, coreAddr, []byte(`{"admin":{}}`)))
			var coreAdmin string
			te.OrFail(json.Unmarshal(coreAdminBz, &coreAdmin))
			Expect(coreAddr.String()).To(Equal(coreAdmin))
		})

		It("should create proposal successfully, vote on it and execute it", func() {
			senderAddr := te.MustNotErr(sdk.AccAddressFromBech32(sender))

			// first become a DAO member by staking to voting module
			player.MitoStakingStake(stakingContractAddress, lpDenom, sender, 5, 10, true)
			//executeContract(sender, stakingContractAddress, `{"stake": {}}`, sdk.Coins{sdk.Coin{Denom: lpDenom, Amount: sdk.NewInt(10)}})
			// stakes only counted starting the next block
			ctx, _ = te.EndBlockerAndCommit(app, player.Ctx)
			// then create a proposal
			// on success, it will:
			// 1. withdraw all deposit funds from pre-propose module to DAO module (10 tokens)
			// 2. send all funds from community fund to sender (20 tokens)
			// 3. send all funds from DAO to sender address (10 tokens)
			cfMsg := fmt.Sprintf(`{"execute_msgs": {"msgs": [{"bank": {
				"send": {
					"to_address": "%s",
					"amount": [{"denom": "%s", "amount": "7"}]
				}
			}}]}}`, sender, nativeToken)
			proposalMsg := fmt.Sprintf(`{"propose": {
				"msg": { 
					"propose": {
						"title": "My first proposal",
						"description": "Who needs this?",
						"msgs": [
							{"wasm": {
								"execute": {
									"contract_addr": "%s",
									"msg": "%s",
									"funds": []
								}
							}}
						]
					}
				}
			}}`, cfAddr, base64.StdEncoding.EncodeToString([]byte(cfMsg)))
			executeContract(sender, preProposeAddr.String(), proposalMsg, sdk.Coins{sdk.Coin{Denom: nativeToken, Amount: sdk.NewInt(10)}})

			afterProposeBalance := app.BankKeeper.GetBalance(ctx, senderAddr, nativeToken)
			Expect(afterProposeBalance.Amount.String()).To(Equal("10"))

			// get last proposal id
			proposalIdBz := te.MustNotErr(app.WasmKeeper.QuerySmart(ctx, proposalAddr, []byte(`{"next_proposal_id":{}}`)))
			var proposalId uint64
			te.OrFail(json.Unmarshal(proposalIdBz, &proposalId))
			proposalId--

			// then vote for it
			voteMsg := fmt.Sprintf(`{"vote": {
				"proposal_id": %d,
				"vote": "yes",
				"rationale": "Because I can?"
			}}`, proposalId)
			executeContract(sender, proposalAddr.String(), voteMsg, sdk.Coins{})

			// now check proposal status
			proposalRespBz := te.MustNotErr(app.WasmKeeper.QuerySmart(ctx, proposalAddr, []byte(fmt.Sprintf(`{"proposal":{"proposal_id": %d}}`, proposalId))))
			var proposalResp map[string]any
			te.OrFail(json.Unmarshal(proposalRespBz, &proposalResp))
			proposal := proposalResp["proposal"].(map[string]any)
			votes := proposal["votes"].(map[string]any)
			Expect(proposal["status"]).To(Equal("passed"))
			Expect(proposal["total_power"]).To(Equal("10"))
			Expect(votes["yes"]).To(Equal("10"))

			// now execute passed proposal

			beforeBalance := app.BankKeeper.GetBalance(ctx, senderAddr, nativeToken)
			Expect(beforeBalance.Amount.String()).To(Equal("10"))
			commFundBalance := app.BankKeeper.GetBalance(ctx, cfAddr, nativeToken)
			_ = commFundBalance

			executeContract(sender, proposalAddr.String(), fmt.Sprintf(`{"execute":{"proposal_id": %d}}`, proposalId), sdk.Coins{})
			afterBalance := app.BankKeeper.GetBalance(ctx, senderAddr, nativeToken)
			Expect(afterBalance.Amount.String()).To(Equal("17")) // 7 back from executing a proposal
		})
	})

})
