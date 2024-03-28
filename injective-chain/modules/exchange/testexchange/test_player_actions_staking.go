package testexchange

import (
	"encoding/json"
	"fmt"
	"time"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

func (tp *TestPlayer) MitoStakingAllocate(
	sender sdk.AccAddress,
	allocatorContractAddress, lpDenom string,
	blockStartInc, blockEndInc int64,
	rewards sdk.Coins,
) {
	tp.App.BankKeeper.MintCoins(tp.Ctx, minttypes.ModuleName, rewards)
	tp.App.BankKeeper.SendCoinsFromModuleToAccount(tp.Ctx, minttypes.ModuleName, sender, rewards)

	rewardsStr, _ := rewards.MarshalJSON()

	allocateMsg := &wasmtypes.MsgExecuteContract{
		Sender:   sender.String(),
		Contract: allocatorContractAddress,
		Msg: wasmtypes.RawContractMessage(fmt.Sprintf(`{"allocate_reward_gauges": {"gauges": [{
						"lp_token": "%s",
						"start_timestamp": "%d",
						"end_timestamp": "%d",
						"reward_tokens": %s
					}]}}`, lpDenom, tp.Ctx.BlockTime().UnixNano()+blockStartInc*time.Second.Nanoseconds(), tp.Ctx.BlockTime().UnixNano()+blockEndInc*time.Second.Nanoseconds(), rewardsStr)),
		Funds: rewards,
	}
	_, err := tp.MsgServerWasm.ExecuteContract(sdk.WrapSDKContext(tp.Ctx), allocateMsg)
	OrFail(err)
}

func (tp *TestPlayer) MitoStakingStake(
	stakingContractAddress, lpDenom string,
	staker string,
	blockInc, stake int64,
	mintStakedCoins bool,
) {
	stakeCoins := sdk.Coins{{Denom: lpDenom, Amount: sdk.NewInt(stake)}}
	if mintStakedCoins {
		err := tp.App.BankKeeper.MintCoins(tp.Ctx, minttypes.ModuleName, stakeCoins)
		OrFail(err)
		err = tp.App.BankKeeper.SendCoinsFromModuleToAccount(tp.Ctx, minttypes.ModuleName, sdk.MustAccAddressFromBech32(staker), stakeCoins)
		OrFail(err)
	}

	stakeMsg := &wasmtypes.MsgExecuteContract{
		Sender:   staker,
		Contract: stakingContractAddress,
		Msg:      wasmtypes.RawContractMessage(`{"stake": {}}`),
		Funds:    stakeCoins,
	}
	tp.Ctx = tp.Ctx.WithBlockHeight(tp.Ctx.BlockHeight() + 1)
	_, err := tp.MsgServerWasm.ExecuteContract(sdk.WrapSDKContext(tp.Ctx.WithBlockTime(tp.Ctx.BlockTime().Add(time.Duration(time.Second.Nanoseconds()*blockInc)))), stakeMsg)
	OrFail(err)
}

func (tp *TestPlayer) MitoStakingUnstake(
	stakingContractAddress, lpDenom, staker string,
	blockInc, stake int64,
) error {
	stakeCoin, _ := json.Marshal(sdk.Coin{Denom: lpDenom, Amount: sdk.NewInt(stake)})
	unstakeMsg := &wasmtypes.MsgExecuteContract{
		Sender:   staker,
		Contract: stakingContractAddress,
		Msg:      wasmtypes.RawContractMessage(fmt.Sprintf(`{"unstake": {"coin": %s}}`, stakeCoin)),
	}
	tp.Ctx = tp.Ctx.WithBlockHeight(tp.Ctx.BlockHeight() + 1)
	_, err := tp.MsgServerWasm.ExecuteContract(sdk.WrapSDKContext(tp.Ctx.WithBlockTime(tp.Ctx.BlockTime().Add(time.Duration(time.Second.Nanoseconds()*blockInc)))), unstakeMsg)
	return err
}

func (tp *TestPlayer) MitoStakingClaimStake(stakingContractAddress, lpDenom, staker string, blockInc int64) error {
	unstakeMsg := &wasmtypes.MsgExecuteContract{
		Sender:   staker,
		Contract: stakingContractAddress,
		Msg:      wasmtypes.RawContractMessage(fmt.Sprintf(`{"claim_stake": {"lp_token": "%s"}}`, lpDenom)),
	}
	tp.Ctx = tp.Ctx.WithBlockHeight(tp.Ctx.BlockHeight() + 1)
	_, err := tp.MsgServerWasm.ExecuteContract(sdk.WrapSDKContext(tp.Ctx.WithBlockTime(tp.Ctx.BlockTime().Add(time.Duration(time.Second.Nanoseconds()*blockInc)))), unstakeMsg)
	return err
}

func (tp *TestPlayer) MitoStakingClaim(
	stakingContractAddress, lpDenom, staker string,
	blockInc int64,
) error {
	claimMsg := &wasmtypes.MsgExecuteContract{
		Sender:   staker,
		Contract: stakingContractAddress,
		Msg:      wasmtypes.RawContractMessage(fmt.Sprintf(`{"claim_rewards": {"lp_token": "%s"}}`, lpDenom)),
	}
	tp.Ctx = tp.Ctx.WithBlockHeight(tp.Ctx.BlockHeight() + 1)
	_, err := tp.MsgServerWasm.ExecuteContract(sdk.WrapSDKContext(tp.Ctx.WithBlockTime(tp.Ctx.BlockTime().Add(time.Duration(time.Second.Nanoseconds()*blockInc)))), claimMsg)
	return err
}

func (tp *TestPlayer) MitoStakingSetup(
	quoteDenom string,
	sender sdk.AccAddress,
	lockupPeriod uint64,
) (allocatorContractAddress, stakingContractAddress, lpDenom string) {
	coin := sdk.NewCoin(quoteDenom, sdk.NewInt(44))
	tp.App.BankKeeper.MintCoins(tp.Ctx, minttypes.ModuleName, sdk.NewCoins(coin))
	tp.App.BankKeeper.SendCoinsFromModuleToAccount(tp.Ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

	var (
		allocatorCodeID uint64
		stakingCodeID   uint64
	)

	// Contract 1: Allocator Contract
	storeAllocatorCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender.String(),
		WASMByteCode: ReadFile("../wasm/staking_allocator.wasm"),
	}
	resp, err := tp.MsgServerWasm.StoreCode(sdk.WrapSDKContext(tp.Ctx), storeAllocatorCodeMsg)
	OrFail(err)
	allocatorCodeID = resp.CodeID

	// Contract 2: Staking Contract
	storeStakingCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender.String(),
		WASMByteCode: ReadFile("../wasm/staking_contract.wasm"),
	}
	resp, err = tp.MsgServerWasm.StoreCode(sdk.WrapSDKContext(tp.Ctx), storeStakingCodeMsg)
	OrFail(err)
	stakingCodeID = resp.CodeID

	instantiateAllocatorMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender.String(),
		Admin:  sender.String(),
		CodeID: allocatorCodeID,
		Label:  "staking-allocator",
		Msg:    wasmtypes.RawContractMessage(fmt.Sprintf(`{"owner": "%s"}`, sender)),
		Funds:  sdk.NewCoins(),
	}
	err = instantiateAllocatorMsg.ValidateBasic()
	OrFail(err)

	_, err = tp.MsgServerWasm.InstantiateContract(sdk.WrapSDKContext(tp.Ctx), instantiateAllocatorMsg)
	OrFail(err)

	tp.App.WasmKeeper.IterateContractsByCode(tp.Ctx, allocatorCodeID, func(addr sdk.AccAddress) bool {
		allocatorContractAddress = addr.String()
		return true
	})

	instantiateStakingMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender.String(),
		Admin:  sender.String(),
		CodeID: stakingCodeID,
		Label:  "staking-contract",
		Msg:    wasmtypes.RawContractMessage(fmt.Sprintf(`{"owner":"%s","allocator_contract_address":"%s","lockup_period":"%d"}`, sender, allocatorContractAddress, lockupPeriod)),
		Funds:  sdk.NewCoins(),
	}
	err = instantiateStakingMsg.ValidateBasic()
	OrFail(err)

	_, err = tp.MsgServerWasm.InstantiateContract(sdk.WrapSDKContext(tp.Ctx), instantiateStakingMsg)
	OrFail(err)

	tp.App.WasmKeeper.IterateContractsByCode(tp.Ctx, stakingCodeID, func(addr sdk.AccAddress) bool {
		stakingContractAddress = addr.String()
		return true
	})

	updateAllocatorConfigMsg := &wasmtypes.MsgExecuteContract{
		Sender:   sender.String(),
		Contract: allocatorContractAddress,
		Msg:      wasmtypes.RawContractMessage(fmt.Sprintf(`{"update_config": {"staking_contract_address": "%s"}}`, stakingContractAddress)),
	}
	_, err = tp.MsgServerWasm.ExecuteContract(sdk.WrapSDKContext(tp.Ctx), updateAllocatorConfigMsg)
	OrFail(err)

	lpDenom = fmt.Sprintf("factory/%v/lp%v", allocatorContractAddress, stakingContractAddress)
	return
}
