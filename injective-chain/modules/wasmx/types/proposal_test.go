package types_test

import (
	"fmt"
	"testing"

	testwasmx "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/test"

	sdktypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

func TestValidBatchRegistrationProposal(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	for _, tc := range []struct {
		desc     string
		proposal types.BatchContractRegistrationRequestProposal
	}{
		{
			desc: "single contract",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "title",
				Description: "desc",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
		{
			desc: "two contracts",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "title",
				Description: "desc",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
					{
						ContractAddress:   "inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			require.NoError(t, err, "basic validaton failed")
		})
	}
}

func TestInvalidBatchRegistrationProposals(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	for _, tc := range []struct {
		desc     string
		proposal types.BatchContractRegistrationRequestProposal
	}{
		{
			desc: "empty title",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "",
				Description: "desc",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
		{
			desc: "empty desc",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "title",
				Description: "",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
		{
			desc: "empty contract address",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "title",
				Description: "desc",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
		{
			desc: "invalid contract address",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "title",
				Description: "desc",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "aaa",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
		{
			desc: "duplicated contract address",
			proposal: types.BatchContractRegistrationRequestProposal{
				Title:       "title",
				Description: "desc",
				ContractRegistrationRequests: []types.ContractRegistrationRequest{
					{
						ContractAddress:   "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
					{
						ContractAddress:   "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						GasLimit:          10,
						GasPrice:          10,
						ShouldPinContract: false,
						FundingMode:       types.FundingMode_SelfFunded,
					},
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			require.Error(t, err, "basic validation succeeded")
		})
	}
}

func TestValidDeregistrationProposal(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	for _, tc := range []struct {
		desc     string
		proposal types.BatchContractDeregistrationProposal
	}{
		{
			desc: "single contract",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{"inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs"},
			},
		},
		{
			desc: "two contracts",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{"inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs", "inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"},
			},
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			require.NoError(t, err, "basic validaton failed")
		})
	}
}

func TestInvalidBatchDeregistrationProposals(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	for _, tc := range []struct {
		desc     string
		proposal types.BatchContractDeregistrationProposal
	}{
		{
			desc: "empty title",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "",
				Description: "desc",
				Contracts:   []string{"inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs"},
			},
		},
		{
			desc: "empty desc",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "",
				Contracts:   []string{"inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs"},
			},
		},
		{
			desc: "empty contract list",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{},
			},
		},
		{
			desc: "invalid address",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{"omg"},
			},
		},
		{
			desc: "duplicated address",
			proposal: types.BatchContractDeregistrationProposal{
				Title:       "title",
				Description: "desc",
				Contracts:   []string{"inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs", "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs"},
			},
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			require.Error(t, err, "basic validation succeeded")
		})
	}
}

func TestValidBatchUploadProposal(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	for _, tc := range []struct {
		desc     string
		proposal types.BatchStoreCodeProposal
	}{
		{
			desc: "single code",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "desc",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "p1",
						Description:           "desc",
						RunAs:                 "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
				},
			},
		},
		{
			desc: "two codes",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "desc",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "p1",
						Description:           "desc",
						RunAs:                 "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
					{
						Title:                 "p2",
						Description:           "desc",
						RunAs:                 "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			require.NoError(t, err, "basic validaton failed")
		})
	}
}

func TestInvalidBatchUploadProposals(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	for _, tc := range []struct {
		desc     string
		proposal types.BatchStoreCodeProposal
	}{
		{
			desc: "empty title",
			proposal: types.BatchStoreCodeProposal{
				Title:       "",
				Description: "desc",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "p1",
						Description:           "desc",
						RunAs:                 "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
				},
			},
		},
		{
			desc: "empty desc",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "p1",
						Description:           "desc",
						RunAs:                 "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
				},
			},
		},
		{
			desc: "empty runAs",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "desc",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "title",
						Description:           "desc",
						RunAs:                 "",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
				},
			},
		},
		{
			desc: "invalid runAs",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "desc",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "title",
						Description:           "desc",
						RunAs:                 "hahaha",
						WASMByteCode:          getValidWasmByteCode(),
						InstantiatePermission: nil,
					},
				},
			},
		},
		{
			desc: "not wasm bytecode",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:                 "p1",
						Description:           "desc",
						RunAs:                 "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode:          []byte("bla bla bla"),
						InstantiatePermission: nil,
					},
				},
			},
		},
		{
			desc: "empty address in access config",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:        "p1",
						Description:  "desc",
						RunAs:        "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode: getValidWasmByteCode(),
						InstantiatePermission: &sdktypes.AccessConfig{
							Addresses: []string{},
						},
					},
				},
			},
		},
		{
			desc: "invalid address in access config",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:        "p1",
						Description:  "desc",
						RunAs:        "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode: getValidWasmByteCode(),
						InstantiatePermission: &sdktypes.AccessConfig{
							Addresses: []string{"bla bla"},
						},
					},
				},
			},
		},
		{
			desc: "invalid access level in access config",
			proposal: types.BatchStoreCodeProposal{
				Title:       "title",
				Description: "",
				Proposals: []sdktypes.StoreCodeProposal{
					{
						Title:        "p1",
						Description:  "desc",
						RunAs:        "inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs",
						WASMByteCode: getValidWasmByteCode(),
						InstantiatePermission: &sdktypes.AccessConfig{
							Addresses:  []string{"inj1ecdwp4z33glmwgmxlavepus0e7njldkkfyz2xs"},
							Permission: 6,
						},
					},
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			err := tc.proposal.ValidateBasic()
			require.Error(t, err, "basic validation succeeded")
		})
	}
}

func getValidWasmByteCode() []byte {
	dummyContractRelativePathPrefix := "../"
	return testwasmx.TestWasmX{}.GetValidWasmByteCode(&dummyContractRelativePathPrefix)
}
