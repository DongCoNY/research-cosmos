package testwasmx

import (
	"fmt"
	"os"

	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestWasmX struct{}

func (t TestWasmX) GetValidWasmByteCode(relativePath *string) []byte {
	rp := "./"
	if relativePath != nil {
		rp = *relativePath
	}
	bytes, err := os.ReadFile(fmt.Sprintf("%v../exchange/wasm/dummy.wasm", rp))
	if err != nil {
		panic(err)
	}

	return bytes
}

func StoreTestContract(player *te.TestPlayer, contractFile string, gasPrice, gasLimit uint64, mintCoins ...int64) sdk.AccAddress {
	dummyAddr := sdk.MustAccAddressFromBech32(te.MustNotErr(
		player.ReplayRegisterAndInitializeContract(&te.ActionRegisterAndInitializeContract{
			ActionStoreContractCode: te.ActionStoreContractCode{
				Path:     "../../exchange/wasm",
				Filename: contractFile,
			},
			Message: make(map[string]any, 0),
		})))
	if len(mintCoins) == 0 {
		mintCoins = []int64{int64(2 * gasPrice * gasLimit * 10)}
	}

	te.OrFail(player.PerformMintAction(sdk.NewInt64Coin("inj", mintCoins[0]), dummyAddr, nil))
	return dummyAddr
}

func StoreTestContractAndRegisterInBB(player *te.TestPlayer, contractFile string, gasPrice, gasLimit uint64, mintCoins ...int64) sdk.AccAddress {
	dummyAddr := sdk.MustAccAddressFromBech32(te.MustNotErr(
		player.ReplayRegisterAndInitializeContract(&te.ActionRegisterAndInitializeContract{
			ActionStoreContractCode: te.ActionStoreContractCode{
				Path:     "../../exchange/wasm",
				Filename: contractFile,
			},
			Message: make(map[string]any, 0),
			RegisterForBBParams: &wasmxtypes.ContractRegistrationRequest{
				GasLimit:    gasLimit,
				GasPrice:    gasPrice,
				FundingMode: wasmxtypes.FundingMode_SelfFunded,
			},
		})))
	if len(mintCoins) == 0 {
		mintCoins = []int64{int64(2 * gasPrice * gasLimit * 10)}
	}

	te.OrFail(player.PerformMintAction(sdk.NewInt64Coin("inj", mintCoins[0]), dummyAddr, nil))
	return dummyAddr
}

func StoreDummyContract(player *te.TestPlayer, gasPrice, gasLimit uint64, mintCoins ...int64) sdk.AccAddress {
	return StoreTestContract(player, "dummy.wasm", gasPrice, gasLimit, mintCoins...)
}

func StoreAndRegisterInBBDummyContract(player *te.TestPlayer, gasPrice, gasLimit uint64, mintCoins ...int64) sdk.AccAddress {
	return StoreTestContractAndRegisterInBB(player, "dummy.wasm", gasPrice, gasLimit, mintCoins...)
}

func RegisterInBB(
	player *te.TestPlayer,
	gasPrice,
	gasLimit int,
	fundMode wasmxtypes.FundingMode,
	contractAddr,
	granterAddr sdk.AccAddress,
) {
	address := granterAddr.String()
	err := player.ReplayRegisterContractForBB(&te.ActionRegisterContractForBB{
		TestActionBase:  te.TestActionBase{},
		ContractAddress: contractAddr.String(),
		GasLimit:        &gasLimit,
		GasPrice:        &gasPrice,
		Funds:           []te.Coin{},
		FundingMode:     &fundMode,
		GranterAddress:  &address,
	})
	te.OrFail(err)
}

func StoreAndRegisterInBBDummyContract2(
	player *te.TestPlayer,
	gasPrice,
	gasLimit uint64,
	fundMode wasmxtypes.FundingMode,
	granterAddr sdk.AccAddress,
	mintCoins ...int64,
) sdk.AccAddress {
	return StoreAndRegisterInBBTestContract2(player, "dummy.wasm", gasPrice, gasLimit, fundMode, granterAddr, mintCoins...)
}

func StoreAndRegisterInBBTestContract2(
	player *te.TestPlayer,
	contractFile string,
	gasPrice,
	gasLimit uint64,
	fundMode wasmxtypes.FundingMode,
	granterAddr sdk.AccAddress,
	mintCoins ...int64,
) sdk.AccAddress {
	dummyAddr := sdk.MustAccAddressFromBech32(te.MustNotErr(
		player.ReplayRegisterAndInitializeContract(&te.ActionRegisterAndInitializeContract{
			ActionStoreContractCode: te.ActionStoreContractCode{
				Path:     "../../exchange/wasm",
				Filename: contractFile,
			},
			Message: make(map[string]any, 0),
			RegisterForBBParams: &wasmxtypes.ContractRegistrationRequest{
				GasLimit:       gasLimit,
				GasPrice:       gasPrice,
				FundingMode:    fundMode,
				GranterAddress: granterAddr.String(),
			},
		})))
	if len(mintCoins) == 0 {
		mintCoins = []int64{int64(2 * gasPrice * gasLimit * 10)}
	}

	te.OrFail(player.PerformMintAction(sdk.NewInt64Coin("inj", mintCoins[0]), dummyAddr, nil))
	return dummyAddr
}
