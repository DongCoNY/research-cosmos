package testexchange

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func NewBalancesTracker() *BalancesTracker {
	return &BalancesTracker{
		Funds:             make(map[common.Hash]map[string]*BankOrSubaccountDeposits),
		subaccountToIndex: make(map[common.Hash]int),
		indexToSubaccount: make(map[int]common.Hash),
	}
}

type BalancesTracker struct {
	// subaccountID => denom => funds
	Funds             map[common.Hash]map[string]*BankOrSubaccountDeposits
	subaccountToIndex map[common.Hash]int
	indexToSubaccount map[int]common.Hash
}

func (b *BalancesTracker) GetAllDenomsForAccountIdx(accountIdx int) []string {
	subaccountID := b.indexToSubaccount[accountIdx]
	subaccountFunds := b.Funds[subaccountID]

	denoms := make([]string, 0, len(subaccountFunds))
	for denom := range subaccountFunds {
		denoms = append(denoms, denom)
	}

	return denoms
}

func (b *BalancesTracker) GetTotalBalancePlusBank(accountIdx int, denom string) sdk.Dec {
	if !b.hasFunds(accountIdx, denom) {
		return sdk.ZeroDec()
	}

	subaccountID := b.indexToSubaccount[accountIdx]
	funds := b.Funds[subaccountID][denom]
	return funds.BankBalance.ToDec().Add(funds.Deposits.TotalBalance)
}

func (b *BalancesTracker) GetSpendableBalance(accountIdx int, denom string) sdk.Dec {
	if !b.hasFunds(accountIdx, denom) {
		return sdk.ZeroDec()
	}

	subaccountID := b.indexToSubaccount[accountIdx]
	funds := b.Funds[subaccountID][denom]
	return funds.BankBalance.ToDec().Add(funds.Deposits.AvailableBalance)
}

func (b *BalancesTracker) GetAvailableBalance(accountIdx int, denom string) sdk.Dec {
	if !b.hasFunds(accountIdx, denom) {
		return sdk.ZeroDec()
	}

	subaccountID := b.indexToSubaccount[accountIdx]
	funds := b.Funds[subaccountID][denom]
	return funds.Deposits.AvailableBalance
}

func (b *BalancesTracker) GetTotalBalance(accountIdx int, denom string) sdk.Dec {
	if !b.hasFunds(accountIdx, denom) {
		return sdk.ZeroDec()
	}

	subaccountID := b.indexToSubaccount[accountIdx]
	funds := b.Funds[subaccountID][denom]
	return funds.Deposits.TotalBalance
}

func (b *BalancesTracker) hasFunds(accountIdx int, denom string) bool {
	subaccountID := b.indexToSubaccount[accountIdx]
	subaccountFunds := b.Funds[subaccountID]
	if subaccountFunds == nil {
		return false
	}

	funds := subaccountFunds[denom]

	return funds != nil
}

func (b *BalancesTracker) initializeSubaccount(subaccountID common.Hash) {
	b.Funds[subaccountID] = make(map[string]*BankOrSubaccountDeposits)
	if len(b.indexToSubaccount) != len(b.subaccountToIndex) {
		panic("bad initialization")
	}

	index := len(b.indexToSubaccount)
	b.indexToSubaccount[index] = subaccountID
	b.subaccountToIndex[subaccountID] = index
}

func (b *BalancesTracker) SetBankBalancesAndSubaccountDeposits(
	subaccountID common.Hash,
	bankBalances sdk.Coins,
	deposits map[string]*types.Deposit,
) {
	for _, balance := range bankBalances {
		funds := NewBankOrSubaccountDeposits(balance.Amount, nil)
		b.AddFunds(subaccountID, balance.Denom, funds)
	}

	for denom, deposit := range deposits {
		funds := NewBankOrSubaccountDeposits(sdk.ZeroInt(), deposit)
		b.AddFunds(subaccountID, denom, funds)
	}
}

func (b *BalancesTracker) AddFunds(
	subaccountID common.Hash,
	denom string,
	funds *BankOrSubaccountDeposits,
) {
	if _, found := b.subaccountToIndex[subaccountID]; !found {
		b.initializeSubaccount(subaccountID)
	}

	if b.Funds[subaccountID][denom] == nil {
		b.Funds[subaccountID][denom] = NewBankOrSubaccountDeposits(
			sdk.ZeroInt(),
			types.NewDeposit(),
		)
	}

	if !types.IsDefaultSubaccountID(subaccountID) {
		funds.BankBalance = sdk.NewInt(0)
	}
	b.Funds[subaccountID][denom].Add(funds)

	//if !types.IsDefaultSubaccountID(subaccountID) && !funds.BankBalance.IsZero() {
	//	// TODO: consider removing the panic to be more lenient, or actually just support adding bank balance support
	//	// for non-default subaccounts since why not.
	//	panic("bank balances should only be set for default subaccounts")
	//}

	//b.Funds[subaccountID][denom].Add(funds)
}

func (b *BalancesTracker) GetTotalDepositPlusBankChange(
	injectiveApp *simapp.InjectiveApp,
	ctx sdk.Context,
	subaccountID common.Hash,
	denom string,
) sdk.Dec {
	deposit := injectiveApp.ExchangeKeeper.GetDeposit(ctx, subaccountID, denom)
	bankBalance := sdk.ZeroInt()
	if types.IsDefaultSubaccountID(subaccountID) {
		accountAddr := types.SubaccountIDToSdkAddress(subaccountID)
		bankBalance = injectiveApp.BankKeeper.GetBalance(ctx, accountAddr, denom).Amount
	}

	initialFunds := b.Funds[subaccountID][denom]
	totalFunds := initialFunds.BankBalance.ToDec().Add(initialFunds.Deposits.TotalBalance)
	balanceDelta := deposit.TotalBalance.Add(bankBalance.ToDec()).Sub(totalFunds)
	return balanceDelta
}

func (b *BalancesTracker) GetAvailableDepositPlusBankChange(
	injectiveApp *simapp.InjectiveApp,
	ctx sdk.Context,
	subaccountID common.Hash,
	denom string,
) sdk.Dec {
	deposit := injectiveApp.ExchangeKeeper.GetDeposit(ctx, subaccountID, denom)
	bankBalance := sdk.ZeroInt()
	if types.IsDefaultSubaccountID(subaccountID) {
		accountAddr := types.SubaccountIDToSdkAddress(subaccountID)
		bankBalance = injectiveApp.BankKeeper.GetBalance(ctx, accountAddr, denom).Amount
	}

	initialFunds := b.Funds[subaccountID][denom]
	if initialFunds == nil {
		initialFunds = NewBankOrSubaccountDeposits(sdk.ZeroInt(), types.NewDeposit())
	}
	totalFunds := initialFunds.BankBalance.ToDec().Add(initialFunds.Deposits.AvailableBalance)
	balanceDelta := deposit.AvailableBalance.Add(bankBalance.ToDec()).Sub(totalFunds)
	return balanceDelta
}

func NewBankOrSubaccountDeposits(
	bankBalance math.Int,
	deposits *types.Deposit,
) *BankOrSubaccountDeposits {
	if deposits == nil {
		deposits = types.NewDeposit()
	}
	return &BankOrSubaccountDeposits{
		BankBalance: bankBalance,
		Deposits:    deposits,
	}
}

type BankOrSubaccountDeposits struct {
	Deposits    *types.Deposit
	BankBalance math.Int
}

func (b *BankOrSubaccountDeposits) Add(balance *BankOrSubaccountDeposits) {
	b.BankBalance = b.BankBalance.Add(balance.BankBalance)
	b.Deposits.AvailableBalance = b.Deposits.AvailableBalance.Add(balance.Deposits.AvailableBalance)
	b.Deposits.TotalBalance = b.Deposits.TotalBalance.Add(balance.Deposits.TotalBalance)
}
