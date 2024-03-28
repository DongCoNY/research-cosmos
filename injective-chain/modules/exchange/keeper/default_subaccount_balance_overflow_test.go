package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Default subaccount overflow tests", func() {
	var (
		testPlayer         *te.TestPlayer
		injectiveApp       *simapp.InjectiveApp
		keeper             exchangekeeper.Keeper
		ctx                sdk.Context
		accounts           []te.Account
		mainSubaccountId   common.Hash
		marketId           common.Hash
		testInput          te.TestInput
		simulationError    error
		hooks              map[string]te.TestPlayerHook
		initialDeposits    []map[string]*types.Deposit
		initialBankBalance map[string]sdk.Coins
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		initialDeposits = make([]map[string]*types.Deposit, 0)
		initialBankBalance = make(map[string]sdk.Coins, 0)
		hooks["post-setup"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				var deposits = keeper.GetDeposits(ctx, common.HexToHash(acc.SubaccountIDs[0]))
				initialDeposits = append(initialDeposits, deposits)

				allBankBalances := injectiveApp.BankKeeper.GetAllBalances(ctx, acc.AccAddress)
				initialBankBalance[acc.AccAddress.String()] = allBankBalances
			}
		}
	})

	var setup = func(testSetup *te.TestPlayer) {
		testPlayer = testSetup
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
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
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/default_subaccount_overflow", file)
		test := te.LoadReplayableTest(filepath)
		setup(&test)
		simulationError = test.ReplayTest(te.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedSpotLimitOrder {
		return te.GetAllSpotOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	var getAllOrdersSorted = func() []*types.TrimmedSpotLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	printOrders := func(orders []*types.TrimmedSpotLimitOrder) {
		fmt.Println("Orders: ", te.GetReadableSlice(orders, " | ", func(ord *types.TrimmedSpotLimitOrder) string {
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v) side:%v", ord.Price.TruncateInt(), ord.Fillable.TruncateInt(), side)
		}))
	}
	_ = printOrders

	var verifySpotOrder = te.VerifySpotOrder

	_ = verifySpotOrder

	Context("Default subaccount availabe balance stores only dust", func() {

		When("There's more of dust than 1 ", func() {

			limitExecutionPrice := f2d(10.5)
			limitOrderValue := limitExecutionPrice.Mul(f2d(5))
			BeforeEach(func() {
				if !te.IsUsingDefaultSubaccount() {
					Skip("makes no sense for non-default subaccount")
				}
				hooks["limit-trade"] = func(params *te.TestPlayerHookParams) {
					quoteDenom := testPlayer.TestInput.Spots[0].QuoteDenom
					takerFeeRate := testPlayer.TestInput.Spots[0].TakerFeeRate

					expectedAddr0BankQuoteChange, expectedAddr0SubaccountQuoteChange := te.Decimal2IntBankChange(limitOrderValue.Add(limitOrderValue.Mul(takerFeeRate)), te.ChargeType_Debit)
					expectedAddr0BankQuoteChange, expectedAddr0SubaccountQuoteChange = te.AdjustBankIntAndDustReminder(expectedAddr0BankQuoteChange, expectedAddr0SubaccountQuoteChange)
					expectedAddr0BankQuoteChange = expectedAddr0BankQuoteChange.Neg()

					addr0SubaccountQuoteChange := te.GetSubaccountDepositChange(testPlayer, 0, quoteDenom, te.BalanceType_available, initialDeposits)
					Expect(addr0SubaccountQuoteChange).To(Equal(expectedAddr0SubaccountQuoteChange), fmt.Sprintf("'%s' subaccount balance did not change by %d for account 0", quoteDenom, expectedAddr0SubaccountQuoteChange))

					addr0QuoteBankChange := te.GetBankDepositChange(testPlayer, 0, quoteDenom, initialBankBalance)
					Expect(addr0QuoteBankChange).To(Equal(expectedAddr0BankQuoteChange), fmt.Sprintf("'%s' bank balance did change by %d for account 0", quoteDenom, addr0QuoteBankChange))

					expectedAddr1BankQuoteChange, expectedAddr1SubaccountQuoteChange := te.Decimal2IntBankChange(limitOrderValue.Sub(limitOrderValue.Mul(takerFeeRate)), te.ChargeType_Credit)
					expectedAddr1BankQuoteChange, expectedAddr1SubaccountQuoteChange = te.AdjustBankIntAndDustReminder(expectedAddr1BankQuoteChange, expectedAddr1SubaccountQuoteChange)

					addr1SubaccountQuoteChange := te.GetSubaccountDepositChange(testPlayer, 1, quoteDenom, te.BalanceType_available, initialDeposits)
					Expect(addr1SubaccountQuoteChange).To(Equal(expectedAddr1SubaccountQuoteChange), fmt.Sprintf("'%s' subaccount balance did not change by %d for account 1", quoteDenom, expectedAddr0SubaccountQuoteChange))

					addr1QuoteBankChange := te.GetBankDepositChange(testPlayer, 1, quoteDenom, initialBankBalance)
					Expect(addr1QuoteBankChange).To(Equal(expectedAddr1BankQuoteChange), fmt.Sprintf("'%s' bank balance did change by %d for account 1", quoteDenom, addr1QuoteBankChange))

					baseDenom := testPlayer.TestInput.Spots[0].BaseDenom
					addr0BaseBankChange := te.GetBankDepositChange(testPlayer, 0, baseDenom, initialBankBalance)
					Expect(addr0BaseBankChange).To(Equal(sdk.NewInt(5)), fmt.Sprintf("'%s' bank balance did not increase by 5 for account 0", baseDenom))

					addr0BaseSubaccountChange := te.GetSubaccountDepositChange(testPlayer, 0, baseDenom, te.BalanceType_available, initialDeposits)
					Expect(addr0BaseSubaccountChange.IsZero()).To(BeTrue(), fmt.Sprintf("'%s' subaccount balance changed for account 0", baseDenom))

					addr1BaseChange := te.GetBankDepositChange(testPlayer, 1, baseDenom, initialBankBalance)
					Expect(addr1BaseChange).To(Equal(sdk.NewInt(-5)), fmt.Sprintf("'%s' bank balance did not increase by 5 for account 0", baseDenom))

					addr1BaseSubaccountChange := te.GetSubaccountDepositChange(testPlayer, 1, baseDenom, te.BalanceType_available, initialDeposits)
					Expect(addr1BaseSubaccountChange.IsZero()).To(BeTrue(), fmt.Sprintf("'%s' subaccount balance changed for account 1", baseDenom))
				}
				runTest("default_subaccount_overflow", true)
			})
			It("is moved to bank leaving only dust on subaccount", func() {
				quoteDenom := testPlayer.TestInput.Spots[0].QuoteDenom
				takerFeeRate := testPlayer.TestInput.Spots[0].TakerFeeRate
				makerFeeRate := testPlayer.TestInput.Spots[0].MakerFeeRate
				marketExecutionPrice := f2d(11)
				marketOrderValue := marketExecutionPrice.Mul(f2d(5))

				expectedAddr0BankQuoteChange, expectedAddr0SubaccountQuoteChange := te.Decimal2IntBankChange(marketOrderValue.Sub(limitOrderValue).Sub(limitOrderValue.Mul(takerFeeRate).Add(marketOrderValue.Mul(takerFeeRate))), te.ChargeType_Credit)
				expectedAddr0BankQuoteChange, expectedAddr0SubaccountQuoteChange = te.AdjustBankIntAndDustReminder(expectedAddr0BankQuoteChange, expectedAddr0SubaccountQuoteChange)

				addr0SubaccountQuoteChange := te.GetSubaccountDepositChange(testPlayer, 0, quoteDenom, te.BalanceType_available, initialDeposits)
				Expect(addr0SubaccountQuoteChange).To(Equal(expectedAddr0SubaccountQuoteChange), fmt.Sprintf("'%s' subaccount balance did not change by %d for account 0", quoteDenom, expectedAddr0SubaccountQuoteChange))

				addr0QuoteBankChange := te.GetBankDepositChange(testPlayer, 0, quoteDenom, initialBankBalance)
				Expect(addr0QuoteBankChange).To(Equal(expectedAddr0BankQuoteChange), fmt.Sprintf("'%s' bank balance did change by %d for account 0", quoteDenom, addr0QuoteBankChange))

				expectedAddr1BankQuoteChange, expectedAddr1SubaccountQuoteChange := te.Decimal2IntBankChange(limitOrderValue.Sub(limitOrderValue.Mul(takerFeeRate)).Sub(marketOrderValue.Add(marketOrderValue.Mul(makerFeeRate))), te.ChargeType_Debit)
				expectedAddr1BankQuoteChange, expectedAddr1SubaccountQuoteChange = te.AdjustBankIntAndDustReminder(expectedAddr1BankQuoteChange, expectedAddr1SubaccountQuoteChange)

				addr1SubaccountQuoteChange := te.GetSubaccountDepositChange(testPlayer, 1, quoteDenom, te.BalanceType_available, initialDeposits)
				Expect(addr1SubaccountQuoteChange).To(Equal(expectedAddr1SubaccountQuoteChange), fmt.Sprintf("'%s' subaccount balance did not change by %d for account 1", quoteDenom, expectedAddr0SubaccountQuoteChange))

				addr1QuoteBankChange := te.GetBankDepositChange(testPlayer, 1, quoteDenom, initialBankBalance)
				Expect(addr1QuoteBankChange).To(Equal(expectedAddr1BankQuoteChange), fmt.Sprintf("'%s' bank balance did change by %d for account 1", quoteDenom, addr0QuoteBankChange))

				baseDenom := testPlayer.TestInput.Spots[0].BaseDenom
				addr0BaseBankChange := te.GetBankDepositChange(testPlayer, 0, baseDenom, initialBankBalance)
				Expect(addr0BaseBankChange.IsZero()).To(BeTrue(), fmt.Sprintf("'%s' bank balance changed for account 0", baseDenom))

				addr0BaseSubaccountChange := te.GetSubaccountDepositChange(testPlayer, 0, baseDenom, te.BalanceType_available, initialDeposits)
				Expect(addr0BaseSubaccountChange.IsZero()).To(BeTrue(), fmt.Sprintf("'%s' subaccount balance changed for account 0", baseDenom))

				addr1BaseChange := te.GetBankDepositChange(testPlayer, 1, baseDenom, initialBankBalance)
				Expect(addr1BaseChange.IsZero()).To(BeTrue(), fmt.Sprintf("'%s' bank balance changed for account 0", baseDenom))

				addr1BaseSubaccountChange := te.GetSubaccountDepositChange(testPlayer, 1, baseDenom, te.BalanceType_available, initialDeposits)
				Expect(addr1BaseSubaccountChange.IsZero()).To(BeTrue(), fmt.Sprintf("'%s' subaccount balance changed for account 1", baseDenom))
			})
		})
	})
})
