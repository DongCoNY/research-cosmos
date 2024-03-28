package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
)

var _ = Describe("Contract can be removed from registry", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		ctx              sdk.Context
		mainSubaccountId func() common.Hash
		marketId         common.Hash
		testInput        testexchange.TestInput
		simulationError  error
		hooks            map[string]testexchange.TestPlayerHook
		tp               *testexchange.TestPlayer
	)
	BeforeEach(func() {
		hooks = make(map[string]testexchange.TestPlayerHook)
	})

	var setup = func(testSetup *testexchange.TestPlayer) {
		tp = testSetup
		injectiveApp = testSetup.App
		ctx = testSetup.Ctx
		mainSubaccountId = func() common.Hash {
			return common.HexToHash((*tp.Accounts)[3].SubaccountIDs[0])
		}
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

	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.SpotLimitOrder {
		return testexchange.GetAllLimitSpotOrdersForMarket(injectiveApp, ctx, subaccountId, marketId)
	}

	var getAllOrdersSortedForAccountIndex = func(accountIndex int) []*types.SpotLimitOrder {
		return getAllOrdersSortedForAccount(common.HexToHash((*tp.Accounts)[accountIndex].SubaccountIDs[0]))
	}

	var getAllOrdersSortedForMainContract = func() []*types.SpotLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId())
	}

	var verifyContractOrders = func(expectedBuyCount, expectedSellCount int) {
		expectedTotal := expectedBuyCount + expectedSellCount
		contractOrders := getAllOrdersSortedForMainContract()
		Expect(len(contractOrders)).To(Equal(expectedTotal), "incorrect number of spot orders was posted by the contract")
		buyOrders := 0
		sellOrders := 0
		for _, ord := range contractOrders {
			if ord.IsBuy() {
				buyOrders++
				Expect(ord.OrderType).To(Equal(types.OrderType_BUY_PO), "Buy order wasn't a PO order")
			} else {
				sellOrders++
				Expect(ord.OrderType).To(Equal(types.OrderType_SELL_PO), "Sell order wasn't a PO order")
			}
		}
		Expect(buyOrders).To(Equal(expectedBuyCount), "incorrect number of buy orders was posted by the contract")
		Expect(sellOrders).To(Equal(expectedSellCount), "incorrect number of sell orders was posted by the contract")
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/registry", file)
		player := testexchange.LoadReplayableTest(filepath)
		setup(&player)
		simulationError = tp.ReplayTest(testexchange.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printOrders := func(orders []*types.SpotLimitOrder) {
		fmt.Println(testexchange.GetReadableSlice(orders, "-", func(ord *types.SpotLimitOrder) string {
			direction := "sell"
			if ord.IsBuy() {
				direction = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v %v)", ord.OrderInfo.Price.TruncateInt(), ord.OrderInfo.Quantity.TruncateInt(), direction)
		}))
	}
	_ = printOrders

	var getRegisteredContracts = func() map[string]struct{} {
		activeContracts := make(map[string]struct{})
		injectiveApp.WasmxKeeper.IterateContractsByGasPrice(ctx, 0, func(contractAddress sdk.AccAddress, contractInfo wasmxtypes.RegisteredContract) (stop bool) {
			activeContracts[contractAddress.String()] = struct{}{}
			return false
		})
		return activeContracts
	}

	When("contract is deregistered", func() {
		BeforeEach(func() {
			hooks["post-execution"] = func(params *testexchange.TestPlayerHookParams) {
				verifyContractOrders(6, 6)
				Expect(len(getRegisteredContracts())).To(Equal(1), "no contract was registered")
			}
			hooks["deregistration"] = func(params *testexchange.TestPlayerHookParams) {
				// contract should have canceled it's orders
				verifyContractOrders(0, 0)
				Expect(len(getRegisteredContracts())).To(Equal(0), "contract was still registered")
			}
			runTest("mito_deregistered", true)
		})
		It("it doesn't execute in begin blocker any more", func() {
			// contract should have canceled it's orders
			verifyContractOrders(0, 0)
			Expect(len(getRegisteredContracts())).To(Equal(0), "contract was still registered")

			accountOrders := getAllOrdersSortedForAccountIndex(0)
			//user's order should be left untouched
			Expect(len(accountOrders)).To(Equal(4), "maybe the contract was executed in begin blocker?")
		})
	})

	When("contract address is already deregistered and we try to deregister it again", func() {
		BeforeEach(func() {
			hooks["post-execution"] = func(params *testexchange.TestPlayerHookParams) {
				verifyContractOrders(6, 6)
				Expect(len(getRegisteredContracts())).To(Equal(1), "no contract was registered")
			}
			hooks["deregistration-1"] = func(params *testexchange.TestPlayerHookParams) {
				// contract should have canceled it's orders
				verifyContractOrders(0, 0)
				Expect(len(getRegisteredContracts())).To(Equal(0), "contract was still registered")
			}
			runTest("mito_deregistered_again", true)
		})
		It("it skips it silently", func() {
			// contract should have canceled it's orders
			verifyContractOrders(0, 0)
			Expect(len(getRegisteredContracts())).To(Equal(0), "contract was still registered")

			accountOrders := getAllOrdersSortedForAccountIndex(0)
			//order should be left untouched
			Expect(len(accountOrders)).To(Equal(4), "maybe the contract was executed in begin blocker?")
		})
	})

	When("contract has been deregistered", func() {
		BeforeEach(func() {
			hooks["post-execution"] = func(params *testexchange.TestPlayerHookParams) {
				// contract should post orders after registration and BB execution
				verifyContractOrders(6, 6)
				Expect(len(getRegisteredContracts())).To(Equal(1), "no contract was registered")
			}
			hooks["deregistration"] = func(params *testexchange.TestPlayerHookParams) {
				// contract should have canceled it's orders
				verifyContractOrders(0, 0)
				Expect(len(getRegisteredContracts())).To(Equal(0), "contract was still registered")
			}
			hooks["registration"] = func(params *testexchange.TestPlayerHookParams) {
				Expect(len(getRegisteredContracts())).To(Equal(1), "contract was registered again")
			}
			runTest("mito_deregistered_registered", true)
		})
		It("it can be registered again and works like a charm", func() {
			// contract should post orders again after registration and BB execution
			verifyContractOrders(6, 6)
		})
	})
})
