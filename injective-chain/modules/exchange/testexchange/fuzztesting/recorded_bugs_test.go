package fuzztesting

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	testexchange "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Bugs found by fuzz tests", func() {
	var (
		accounts         []te.Account
		mainSubaccountId common.Hash
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		tp               te.TestPlayer
	)
	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
	})

	var setup = func(testSetup te.TestPlayer) {
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		_ = mainSubaccountId
	}
	var runTest = func(file string, stopOnError bool, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./recordings", file)
		tp = te.LoadReplayableTest(filepath)
		setup(tp)
		simulationError = tp.ReplayTest(testexchange.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printOrders := func(orders []*types.DerivativeLimitOrder) {
		fmt.Fprintln(GinkgoWriter, te.GetReadableSlice(orders, "-", func(ord *types.DerivativeLimitOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			return fmt.Sprintf("p:%v(q:%v%v)", ord.OrderInfo.Price.TruncateInt(), ord.OrderInfo.Quantity.TruncateInt(), ro)
		}))
	}
	_ = printOrders

	Context("Fuzz-test recording which has crossed orderbook v2", func() {
		It("should not fail", func() {
			runTest("orderbook_derivatives_crossed", false, false)
		})
	})
})
