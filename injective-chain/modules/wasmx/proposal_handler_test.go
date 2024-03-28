package wasmx_test

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	wasmdkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx"
	. "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/test"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

const (
	CONTRACT_ADDRESS   = testexchange.SampleAccountAddrStr1
	CONTRACT_ADDRESS_2 = testexchange.SampleAccountAddrStr2
)

func getValidWasmByteCode() []byte {
	dummyContractRelativePathPrefix := "./"
	return TestWasmX{}.GetValidWasmByteCode(&dummyContractRelativePathPrefix)
}

func getValidRegistrationProposal(contractAddress string) types.ContractRegistrationRequestProposal {
	return types.ContractRegistrationRequestProposal{
		Title:       "title",
		Description: "desc",
		ContractRegistrationRequest: types.ContractRegistrationRequest{
			ContractAddress:   contractAddress,
			GasLimit:          1000000,
			GasPrice:          1000000001,
			ShouldPinContract: true,
			CodeId:            5,
			FundingMode:       types.FundingMode_SelfFunded,
		},
	}
}

var _ = Describe("Wasmx proposal handler tests", func() {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	var (
		app     *simapp.InjectiveApp
		ctx     sdk.Context
		handler govtypes.Handler
	)

	topupAddress := func(addr string, amount sdk.Coin) {
		accAddr, _ := sdk.AccAddressFromBech32(addr)
		amounts := sdk.Coins{amount}
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, accAddr, amounts)
	}

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
	})

	Context("contract registration proposal", func() {

		Context("valid proposal", func() {
			pinCodeCallCount := 0
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					HasContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) bool { return true },
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				pinCodeFunc := func(ctx sdk.Context, codeID uint64) error {
					pinCodeCallCount++
					return nil
				}
				mockedOpsKeeper := WasmOpsKeeperMock{PinCodeHandler: &pinCodeFunc}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				params.IsExecutionEnabled = true
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("shouldn't return any errors", func() {
				proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err := handler(ctx, &proposal)
				Expect(err).To(BeNil(), "proposal handling failed")
				Expect(pinCodeCallCount).To(Equal(1), "contract wasn't pinned")
			})
		})

		Context("contract address is validated", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should fail, when contract address is empty", func() {
				proposal := types.ContractRegistrationRequestProposal{
					Title:       "title",
					Description: "desc",
					ContractRegistrationRequest: types.ContractRegistrationRequest{
						ContractAddress:   "",
						GasLimit:          1000000,
						GasPrice:          1000000001,
						ShouldPinContract: true,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				}
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("invalid contract address"), "invalid error returned")
			})

			It("it should fail, when contract address is invalid", func() {
				proposal := types.ContractRegistrationRequestProposal{
					Title:       "title",
					Description: "desc",
					ContractRegistrationRequest: types.ContractRegistrationRequest{
						ContractAddress:   "bla bla",
						GasLimit:          1000000,
						GasPrice:          1000000001,
						ShouldPinContract: true,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				}
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("invalid contract address"), "invalid error returned")
			})

		})

		Context("gas limit is validated", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should fail, when gas limit is below minimum", func() {
				proposal := types.ContractRegistrationRequestProposal{
					Title:       "title",
					Description: "desc",
					ContractRegistrationRequest: types.ContractRegistrationRequest{
						ContractAddress:   CONTRACT_ADDRESS,
						GasLimit:          1,
						GasPrice:          1000000001,
						ShouldPinContract: true,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				}
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("ContractRegistrationRequestProposal: The gasLimit"), "invalid error returned")
			})

			It("it should fail, when gas limit is above maximum", func() {
				proposal := types.ContractRegistrationRequestProposal{
					Title:       "title",
					Description: "desc",
					ContractRegistrationRequest: types.ContractRegistrationRequest{
						ContractAddress:   CONTRACT_ADDRESS,
						GasLimit:          100000000,
						GasPrice:          1000000001,
						ShouldPinContract: true,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				}
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("ContractRegistrationRequestProposal: The gasLimit"), "invalid error returned")
			})
		})

		Context("gas price is validated", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				params.MinGasPrice = 2
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should fail, when gas price is below minimum", func() {
				proposal := types.ContractRegistrationRequestProposal{
					Title:       "title",
					Description: "desc",
					ContractRegistrationRequest: types.ContractRegistrationRequest{
						ContractAddress:   CONTRACT_ADDRESS,
						GasLimit:          1000000,
						GasPrice:          1,
						ShouldPinContract: true,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				}
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("ContractRegistrationRequestProposal: The gasPrice"), "invalid error returned")
			})
		})

		Context("contract doesn't exist", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						return nil
					},
				}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should fail, when contract doesn't exist", func() {
				proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("ContractRegistrationRequestProposal: The contract address"), "invalid error returned")
			})
		})

		Context("contract already registered", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should fail if contract is already registered", func() {
				proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err := handler(ctx, &proposal)
				Expect(err).To(BeNil()) //first registration should pass

				proposal2 := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err2 := handler(ctx, &proposal2)
				Expect(err2).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err2.Error()).To(ContainSubstring("ContractRegistrationRequestProposal: contract"), "invalid error returned")
			})
		})

		Context("contract is not registered", func() {
			pinCodeCallCount := 0
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				pinCodeFunc := func(ctx sdk.Context, codeID uint64) error {
					pinCodeCallCount++
					return nil
				}
				mockedOpsKeeper := WasmOpsKeeperMock{PinCodeHandler: &pinCodeFunc}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should skip registered contracts with incorrect addresses and succeed", func() {
				proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err := handler(ctx, &proposal)
				Expect(err).To(BeNil(), "proposal handling failed")
				Expect(pinCodeCallCount).To(Equal(1), "contract wasn't pinned")
			})
		})

		Context("contract pinning fails", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				errShouldPinContractFunc := func(ctx sdk.Context, codeID uint64) error {
					return fmt.Errorf("glorious error")
				}
				mockedOpsKeeper := WasmOpsKeeperMock{
					PinCodeHandler: &errShouldPinContractFunc,
				}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)

				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("it should fail", func() {
				proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
				Expect(err.Error()).To(ContainSubstring("ContractRegistrationRequestProposal: Error while pinning the contract"), "invalid error returned")
			})
		})
	})

	Context("batch contract registration proposal", func() {

		Context("two valid proposals", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("shouldn't return any errors", func() {
				request1 := types.ContractRegistrationRequest{
					ContractAddress:   CONTRACT_ADDRESS,
					GasLimit:          1000000,
					GasPrice:          1000000001,
					ShouldPinContract: true,
					CodeId:            5,
					FundingMode:       types.FundingMode_SelfFunded,
				}
				request2 := types.ContractRegistrationRequest{
					ContractAddress:   CONTRACT_ADDRESS_2,
					GasLimit:          1000000,
					GasPrice:          1000000001,
					ShouldPinContract: true,
					CodeId:            5,
					FundingMode:       types.FundingMode_SelfFunded,
				}

				proposal := types.BatchContractRegistrationRequestProposal{
					Title:                        "title",
					Description:                  "desc",
					ContractRegistrationRequests: []types.ContractRegistrationRequest{request1, request2},
				}

				err := handler(ctx, &proposal)
				Expect(err).To(BeNil(), "proposal handling failed")
			})
		})

		Context("one of two proposals is invalid", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("should fail if first proposal is invalid", func() {
				request1 := types.ContractRegistrationRequest{
					ContractAddress:   "",
					GasLimit:          1000000,
					GasPrice:          1000000001,
					ShouldPinContract: true,
					FundingMode:       types.FundingMode_SelfFunded,
				}
				request2 := types.ContractRegistrationRequest{
					ContractAddress:   "inj1tlatz9jq7s34y6zurg67zfgqajvrjg9wsprqyu",
					GasLimit:          1000000,
					GasPrice:          1000000001,
					ShouldPinContract: true,
					FundingMode:       types.FundingMode_SelfFunded,
				}

				proposal := types.BatchContractRegistrationRequestProposal{
					Title:                        "title",
					Description:                  "desc",
					ContractRegistrationRequests: []types.ContractRegistrationRequest{request1, request2},
				}

				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
			})

			It("should fail if second proposal is invalid", func() {
				request1 := types.ContractRegistrationRequest{
					ContractAddress:   "inj1tlatz9jq7s34y6zurg67zfgqajvrjg9wsprqyu",
					GasLimit:          1000000,
					GasPrice:          1000000001,
					ShouldPinContract: true,
					FundingMode:       types.FundingMode_SelfFunded,
				}
				request2 := types.ContractRegistrationRequest{
					ContractAddress:   "",
					GasLimit:          1000000,
					GasPrice:          1000000001,
					ShouldPinContract: true,
					FundingMode:       types.FundingMode_SelfFunded,
				}

				proposal := types.BatchContractRegistrationRequestProposal{
					Title:                        "title",
					Description:                  "desc",
					ContractRegistrationRequests: []types.ContractRegistrationRequest{request1, request2},
				}

				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling succeeded")
			})
		})
	})

	Context("batch contract deregistration proposal", func() {

		Context("two valid proposals", func() {
			unpinCallCount := 0
			JustBeforeEach(func() {

				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					},
				}
				unpinFunc := func(ctx sdk.Context, codeID uint64) error {
					unpinCallCount++
					return nil
				}
				mockedOpsKeeper := WasmOpsKeeperMock{UnpinCodeHandler: &unpinFunc}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)
				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

				testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS).ContractRegistrationRequest))
				testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS_2).ContractRegistrationRequest))

				topupAddress(CONTRACT_ADDRESS, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})
				topupAddress(CONTRACT_ADDRESS_2, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})
			})

			It("shouldn't return any errors", func() {
				proposal := types.BatchContractDeregistrationProposal{
					Title:       "title",
					Description: "desc",
					Contracts:   []string{CONTRACT_ADDRESS, CONTRACT_ADDRESS_2},
				}

				err := handler(ctx, &proposal)
				Expect(err).To(BeNil(), "proposal handling failed")
				Expect(unpinCallCount).To(Equal(2), "unpin function wasn't called 2 times")
			})
		})

	})

	Context("registered contract fetching", func() {
		JustBeforeEach(func() {
			params := types.DefaultParams()
			app.WasmxKeeper.SetParams(ctx, params)
		})

		It("fails if registered contract list cannot be fetched", func() {
			mockedViewKeeper := WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
					resp := wasmdtypes.ContractInfo{
						CodeID: uint64(5),
					}
					return &resp
				},
			}
			mockedOpsKeeper := WasmOpsKeeperMock{}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(Not(BeNil()), "proposal handling succeeded")
		})

		It("fails if registered contract address is empty", func() {
			mockedViewKeeper := WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
					resp := wasmdtypes.ContractInfo{
						CodeID: uint64(5),
					}
					return &resp
				},
			}
			mockedOpsKeeper := WasmOpsKeeperMock{}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(Not(BeNil()), "proposal handling succeeded")
		})

		It("fails if registered contract address is invalid", func() {
			mockedViewKeeper := WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
					resp := wasmdtypes.ContractInfo{
						CodeID: uint64(5),
					}
					return &resp
				},
			}
			mockedOpsKeeper := WasmOpsKeeperMock{}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(Not(BeNil()), "proposal handling succeeded")
		})
	})

	Context("one of two contracts is not registered", func() {

		It("succeeds if first contract was never registered", func() {
			unpinCallCount := 0
			unpinFunc := func(ctx sdk.Context, codeID uint64) error {
				unpinCallCount++
				return nil
			}
			mockedViewKeeper := WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
					if contractAddress.String() == CONTRACT_ADDRESS {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					} else {
						return nil
					}
				},
			}
			mockedOpsKeeper := WasmOpsKeeperMock{UnpinCodeHandler: &unpinFunc}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
			testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS).ContractRegistrationRequest))
			topupAddress(CONTRACT_ADDRESS, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})

			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{CONTRACT_ADDRESS, CONTRACT_ADDRESS_2},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(BeNil(), "proposal handling failed")
			Expect(unpinCallCount).To(Equal(1), "incorrect number of contracts was unpinned")
		})
		It("succeeds if second contract was never registered", func() {
			unpinCallCount := 0
			unpinFunc := func(ctx sdk.Context, codeID uint64) error {
				unpinCallCount++
				return nil
			}
			mockedViewKeeper := WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
					if contractAddress.String() == CONTRACT_ADDRESS_2 {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(5),
						}
						return &resp
					} else {
						return nil
					}
				},
			}
			mockedOpsKeeper := WasmOpsKeeperMock{UnpinCodeHandler: &unpinFunc}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
			testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS_2).ContractRegistrationRequest))
			topupAddress(CONTRACT_ADDRESS_2, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})

			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{CONTRACT_ADDRESS, CONTRACT_ADDRESS_2},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(BeNil(), "proposal handling failed")
			Expect(unpinCallCount).To(Equal(1), "incorrect number of contracts was unpinned")
		})
	})

	Context("contract unpinning fails", func() {
		var mockedViewKeeper types.WasmViewKeeper
		firstContractCodeID := uint64(1)
		secondContractCodeID := uint64(2)
		unpinCalledForFirst := false
		unpinCalledForSecond := false

		JustBeforeEach(func() {
			mockedViewKeeper = WasmViewKeeperMock{
				GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
					addr := sdk.MustAccAddressFromBech32(CONTRACT_ADDRESS)

					codeId := secondContractCodeID
					if addr.Equals(contractAddress) {
						codeId = firstContractCodeID
					}

					resp := wasmdtypes.ContractInfo{
						CodeID: codeId,
					}

					return &resp
				},
			}
			params := types.DefaultParams()
			app.WasmxKeeper.SetParams(ctx, params)
		})

		It("skips deregistering second contract if first fails", func() {
			unpinFunc := func(ctx sdk.Context, codeID uint64) error {
				if codeID == firstContractCodeID {
					unpinCalledForFirst = true
					return fmt.Errorf("ohmyzosh")
				} else {
					unpinCalledForSecond = true
				}

				return nil
			}
			mockedOpsKeeper := WasmOpsKeeperMock{
				UnpinCodeHandler: &unpinFunc,
			}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)

			testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS).ContractRegistrationRequest))
			testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS_2).ContractRegistrationRequest))
			topupAddress(CONTRACT_ADDRESS, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})
			topupAddress(CONTRACT_ADDRESS_2, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})

			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{CONTRACT_ADDRESS, CONTRACT_ADDRESS_2},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(Not(BeNil()), "proposal handling succeeded")
			Expect(err.Error()).To(Equal("ohmyzosh"), "wrong error returned")

			Expect(unpinCalledForFirst).To(BeTrue(), "first contract's unpin wasn't called")

			Expect(unpinCalledForSecond).To(BeFalse(), "second contract unpin was called")
			contract2 := app.WasmxKeeper.GetContractByAddress(ctx, sdk.MustAccAddressFromBech32(CONTRACT_ADDRESS))
			Expect(contract2).ToNot(BeNil())
		})

		It("second contract is still deregistered even if it's unpinning fails", func() {
			unpinFunc := func(ctx sdk.Context, codeID uint64) error {
				if codeID == firstContractCodeID {
					unpinCalledForFirst = true
				} else {
					unpinCalledForSecond = true
					return fmt.Errorf("hastabambista")
				}

				return nil
			}
			mockedOpsKeeper := WasmOpsKeeperMock{
				UnpinCodeHandler: &unpinFunc,
			}
			app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)

			testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS).ContractRegistrationRequest))
			testexchange.OrFail(app.WasmxKeeper.RegisterContract(ctx, getValidRegistrationProposal(CONTRACT_ADDRESS_2).ContractRegistrationRequest))
			topupAddress(CONTRACT_ADDRESS, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})
			topupAddress(CONTRACT_ADDRESS_2, sdk.Coin{Denom: "inj", Amount: govtypes.DefaultMinDepositTokens})

			wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
			handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)

			proposal := types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{CONTRACT_ADDRESS, CONTRACT_ADDRESS_2},
			}

			err := handler(ctx, &proposal)
			Expect(err).To(Not(BeNil()), "proposal handling succeeded")
			Expect(err.Error()).To(Equal("hastabambista"), "wrong error returned")
			Expect(unpinCalledForFirst).To(BeTrue(), "first contract's unpin wasn't called")
			Expect(unpinCalledForSecond).To(BeTrue(), "second contract unpin wasn't called")
		})
	})

	Context("batch store code proposal", func() {

		Context("two valid proposals", func() {
			createCallCount := 0
			pinCallCount := 0
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{}
				createFunc := func(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte, instantiateAccess *wasmdtypes.AccessConfig) (codeID uint64, checksum []byte, err error) {
					createCallCount++
					return 0, make([]byte, 0), nil
				}

				pinFunc := func(ctx sdk.Context, codeID uint64) error {
					pinCallCount++
					return nil
				}
				mockedOpsKeeper := WasmOpsKeeperMock{CreateHandler: &createFunc, PinCodeHandler: &pinFunc}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				/*params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)*/

				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandlerX(mockedOpsKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("shouldn't return any errors", func() {
				proposal := types.BatchStoreCodeProposal{
					Title:       "title",
					Description: "desc",
					Proposals: []wasmdtypes.StoreCodeProposal{
						{
							Title:                 "title",
							Description:           "desc",
							RunAs:                 CONTRACT_ADDRESS,
							WASMByteCode:          getValidWasmByteCode(),
							InstantiatePermission: nil,
						},
						{
							Title:                 "title",
							Description:           "desc",
							RunAs:                 CONTRACT_ADDRESS,
							WASMByteCode:          getValidWasmByteCode(),
							InstantiatePermission: nil,
						},
					},
				}

				err := handler(ctx, &proposal)
				Expect(err).To(BeNil(), "proposal handling failed")
				Expect(createCallCount).To(Equal(2), "create function wasn't called 2 times")
				Expect(pinCallCount).To(Equal(2), "pin code function wasn't called 2 times")
			})
		})
	})

	Context("verifying code id", func() {

		Context("code-id doesn't match registration proposal", func() {
			JustBeforeEach(func() {
				mockedViewKeeper := WasmViewKeeperMock{
					GetContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
						resp := wasmdtypes.ContractInfo{
							CodeID: uint64(4),
						}
						return &resp
					},
				}
				mockedOpsKeeper := WasmOpsKeeperMock{}
				app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
				params := types.DefaultParams()
				app.WasmxKeeper.SetParams(ctx, params)

				wasmGovHandler := wasmdkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmdtypes.EnableAllProposals)
				handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
			})

			It("should fail with wrong code-id message", func() {
				proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
				err := handler(ctx, &proposal)
				Expect(err).To(Not(BeNil()), "proposal handling should fail due to wrong code-id")
				Expect(err.Error()).To(ContainSubstring("does not match codeId from the proposal"))
			})
		})

		//Context("code-id doesn't match registered code-id and migrations are not allowed", func() {
		//	var returnCodeId = 5
		//	JustBeforeEach(func() {
		//		mockedViewKeeper := WasmViewKeeperMock{
		//			getContractInfoHandler: func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
		//				resp := wasmdtypes.ContractInfo{
		//					CodeID: uint64(returnCodeId),
		//				}
		//				return &resp
		//			},
		//		}
		//		mockedOpsKeeper := WasmOpsKeeperMock{}
		//		app.WasmxKeeper.SetWasmKeepers(mockedViewKeeper, mockedOpsKeeper)
		//		params := types.DefaultParams()
		//		app.WasmxKeeper.SetParams(ctx, params)
		//		handler = wasmx.NewWasmxProposalHandler(app.WasmxKeeper)
		//	})
		//
		//	It("shouldn't return any errors", func() {
		//		proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
		//		err := handler(ctx, &proposal)
		//		Expect(err).To(BeNil()) //first registration should pass
		//
		//		returnCodeId = 4
		//
		//		proposal := getValidRegistrationProposal(CONTRACT_ADDRESS)
		//		err := handler(ctx, &proposal)
		//		Expect(err).To(Not(BeNil()), "proposal handling should fail due to wrong code-id")
		//		Expect(err).To(ContainSubstring("does not match codeId from the proposal"))
		//	})
		//})
	})

})
