package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
)

var _ = Describe("Wasmx storage tests", func() {
	var ctx sdk.Context
	var keeper keeper.Keeper
	var contract1addr = sdk.MustAccAddressFromBech32(te.SampleAccountAddrStr1)
	var contract2addr = sdk.MustAccAddressFromBech32(te.SampleAccountAddrStr2)
	var contract3addr = sdk.MustAccAddressFromBech32(te.SampleAccountAddrStr3)
	var contract4addr = sdk.MustAccAddressFromBech32(te.SampleAccountAddrStr4)

	BeforeEach(func() {
		config := te.TestPlayerConfig{NumAccounts: 2, NumSpotMarkets: 1}
		player := te.InitTest(config, nil)
		ctx = player.Ctx
		keeper = player.App.WasmxKeeper
	})

	When("one contract is stored", func() {
		It("it's possible to retrieve it", func() {
			contract := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     10000,
				IsExecutable: false,
			}
			keeper.SetContract(ctx, contract1addr, contract)

			storedContract := keeper.GetContractByAddress(ctx, contract1addr)
			Expect(*storedContract).To(Equal(contract))

			keeper.DeleteContract(ctx, contract1addr)

			deletedContract := keeper.GetContractByAddress(ctx, contract1addr)
			Expect(deletedContract).To(BeNil())

			keeper.DeleteContract(ctx, contract1addr)
		})
	})

	When("multiple contracts are stored", func() {
		It("it's possible to iterate over them in gas price order", func() {
			contract1 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1499,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract1addr, contract1)

			contract2 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     2000,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract2addr, contract2)

			contract3 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1500,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract3addr, contract3)

			contract4 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1800,
				IsExecutable: false,
			}
			keeper.SetContract(ctx, contract4addr, contract4)

			found := make([]types.RegisteredContract, 0)
			keeper.IterateContractsByGasPrice(ctx, 1500, func(contractAddress sdk.AccAddress, contractInfo types.RegisteredContract) bool {
				found = append(found, contractInfo)
				return false
			})

			Expect(len(found)).To(Equal(2))
			Expect(found[0].GasPrice).To(Equal(uint64(2000)))
			Expect(found[1].GasPrice).To(Equal(uint64(1500)))
		})
	})

	When("Exporting and importing genesis state", func() {
		BeforeEach(func() {
			contract1 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1499,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract1addr, contract1)

			contract2 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     2000,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract2addr, contract2)

			contract3 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1500,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract3addr, contract3)

			contract4 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1800,
				IsExecutable: false,
			}
			keeper.SetContract(ctx, contract4addr, contract4)
		})

		It("it's possible to export and import genesis state", func() {
			state := keeper.ExportGenesis(ctx)
			Expect(len(state.RegisteredContracts)).To(Equal(4))

			for _, c := range state.RegisteredContracts {
				keeper.DeleteContract(ctx, sdk.MustAccAddressFromBech32(c.Address))
			}
			keeper.IterateContractsByGasPrice(ctx, 0, func(contractAddress sdk.AccAddress, contractInfo types.RegisteredContract) (stop bool) {
				panic("Shouldn't be called as there should be no contracts")
			})

			keeper.InitGenesis(ctx, *state)
			contracts := make([]types.RegisteredContract, 0)
			keeper.IterateContractsByGasPrice(ctx, 0, func(contractAddress sdk.AccAddress, contractInfo types.RegisteredContract) (stop bool) {
				contracts = append(contracts, contractInfo)
				return false
			})
			Expect(len(contracts)).To(Equal(4))
			Expect(contracts[0].GasPrice).To(Equal(uint64(2000)))
			Expect(contracts[1].GasPrice).To(Equal(uint64(1500)))
			Expect(contracts[2].GasPrice).To(Equal(uint64(1499)))
			Expect(contracts[3].GasPrice).To(Equal(uint64(1800)))
		})
	})

	When("Code id was updated", func() {
		BeforeEach(func() {
			contract1 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1499,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract1addr, contract1)

			contract2 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     2000,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract2addr, contract2)

			contract3 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1500,
				IsExecutable: true,
			}
			keeper.SetContract(ctx, contract3addr, contract3)

			contract4 := types.RegisteredContract{
				GasLimit:     1000,
				GasPrice:     1800,
				IsExecutable: false,
			}
			keeper.SetContract(ctx, contract4addr, contract4)
		})

		It("it's possible to export and import genesis state", func() {
			state := keeper.ExportGenesis(ctx)
			Expect(len(state.RegisteredContracts)).To(Equal(4))

			for _, c := range state.RegisteredContracts {
				keeper.DeleteContract(ctx, sdk.MustAccAddressFromBech32(c.Address))
			}
			keeper.IterateContractsByGasPrice(ctx, 0, func(contractAddress sdk.AccAddress, contractInfo types.RegisteredContract) (stop bool) {
				panic("Shouldn't be called as there should be no contracts")
			})

			keeper.InitGenesis(ctx, *state)
			contracts := make([]types.RegisteredContract, 0)
			keeper.IterateContractsByGasPrice(ctx, 0, func(contractAddress sdk.AccAddress, contractInfo types.RegisteredContract) (stop bool) {
				contracts = append(contracts, contractInfo)
				return false
			})
			Expect(len(contracts)).To(Equal(4))
			Expect(contracts[0].GasPrice).To(Equal(uint64(2000)))
			Expect(contracts[1].GasPrice).To(Equal(uint64(1500)))
			Expect(contracts[2].GasPrice).To(Equal(uint64(1499)))
			Expect(contracts[3].GasPrice).To(Equal(uint64(1800)))
		})
	})
})
