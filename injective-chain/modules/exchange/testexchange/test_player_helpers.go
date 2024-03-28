package testexchange

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (tp *TestPlayer) GetDefaultMarketType(paramsMarketType *MarketType) MarketType {
	if paramsMarketType != nil {
		return *paramsMarketType
	}
	if len(tp.TestInput.Spots) > 0 {
		return MarketType_spot
	}
	if len(tp.TestInput.Perps) > 0 {
		return MarketType_derivative
	}
	if len(tp.TestInput.BinaryMarkets) > 0 {
		return MarketType_binary
	}
	if len(tp.TestInput.ExpiryMarkets) > 0 {
		return MarketType_expiry
	}
	panic("No market defined in the test")
}

func (tp *TestPlayer) FindMarketId(marketType MarketType, marketIndex int) common.Hash {
	testInput := tp.TestInput
	switch marketType {
	case MarketType_spot:
		return testInput.Spots[marketIndex].MarketID
	case MarketType_derivative:
		return testInput.Perps[marketIndex].MarketID
	case MarketType_expiry:
		return testInput.ExpiryMarkets[marketIndex].MarketID
	case MarketType_binary:
		return testInput.BinaryMarkets[marketIndex].MarketID
	}
	panic("Cannot find any market, check your test definition!!")
}

func (tp *TestPlayer) GetAvailableDepositChange(accountIdx int, denom string) sdk.Dec {
	subaccountID := common.HexToHash((*tp.Accounts)[accountIdx].SubaccountIDs[0])
	balancesBase := tp.App.ExchangeKeeper.GetDeposit(tp.Ctx, subaccountID, denom)
	return balancesBase.AvailableBalance.Sub(tp.BalancesTracker.GetAvailableBalance(accountIdx, denom))
}

func (tp *TestPlayer) GetAvailableDepositPlusBankChange(accountIdx int, denom string) sdk.Dec {
	subaccountID := common.HexToHash((*tp.Accounts)[accountIdx].SubaccountIDs[0])
	return tp.BalancesTracker.GetAvailableDepositPlusBankChange(tp.App, tp.Ctx, subaccountID, denom)
}

func (tp *TestPlayer) GetTotalDepositChange(accountIdx int, denom string) sdk.Dec {
	subaccountID := common.HexToHash((*tp.Accounts)[accountIdx].SubaccountIDs[0])
	return tp.BalancesTracker.GetTotalDepositPlusBankChange(tp.App, tp.Ctx, subaccountID, denom)
}

func (tp *TestPlayer) GetAllAvailableDepositChanges(accountIdx int) map[string]sdk.Dec {
	pairs := make(map[string]sdk.Dec, 0)
	for _, denom := range tp.BalancesTracker.GetAllDenomsForAccountIdx(accountIdx) {
		bc := tp.GetAvailableDepositChange(accountIdx, denom)
		pairs[denom] = bc
	}
	return pairs
}

func (tp *TestPlayer) AccountIndexByAddr(addr sdk.AccAddress) int {
	for i, account := range *tp.Accounts {
		if account.AccAddress.Equals(addr) {
			return i
		}
	}
	panic(fmt.Sprintf("Couldn't find account %v", addr))
}

func (tp *TestPlayer) GetSubaccountId(accIdx, subaccIdx int) common.Hash {
	subaccountId := (*tp.Accounts)[accIdx].SubaccountIDs[subaccIdx]
	return common.HexToHash(subaccountId)
}

func (tp *TestPlayer) GetBankBalance(accountIdx int, denom string) sdk.Dec {
	acc := (*tp.Accounts)[accountIdx]
	coin := tp.App.BankKeeper.GetBalance(tp.Ctx, acc.AccAddress, denom)
	return sdk.NewDec(coin.Amount.Int64())
}

func f2d(f64Value float64) sdk.Dec {
	return sdk.MustNewDecFromStr(fmt.Sprintf("%f", f64Value))
}

func fn2d(nullableFloat *float64) *sdk.Dec {
	if nullableFloat != nil {
		dec := f2d(*nullableFloat)
		return &dec
	} else {
		return nil
	}
}

func d2nf(decValue sdk.Dec) *float64 {
	f := decValue.MustFloat64()
	return &f
}

func nd2nf(decValue *sdk.Dec) *float64 {
	if decValue == nil {
		return nil
	}
	f := decValue.MustFloat64()
	return &f
}
