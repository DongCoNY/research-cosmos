package testexchange

import (
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (tp *TestPlayer) buildCreateDenomMsg(params *ActionCreateDenomTokenFactory) types.MsgCreateDenom {
	return *types.NewMsgCreateDenom((*tp.Accounts)[params.AccountIndex].AccAddress.String(), params.Subdenom, params.Name, params.Symbol)
}

func (tp *TestPlayer) replayCreateDenom(params *ActionCreateDenomTokenFactory) {
	msgServer := keeper.NewMsgServerImpl(tp.App.TokenFactoryKeeper)

	msg := tp.buildCreateDenomMsg(params)
	resp, err := msgServer.CreateDenom(sdk.WrapSDKContext(tp.Ctx), &msg)
	OrFail(err)

	*tp.TokenFactoryDenoms = append(*tp.TokenFactoryDenoms, resp.GetNewTokenDenom())

	tp.SuccessActions[params.ActionType]++
}

func (tp *TestPlayer) replayMintDenom(params *ActionMintTokenFactory) {
	app := tp.App
	msgServer := keeper.NewMsgServerImpl(app.TokenFactoryKeeper)

	senderAddress := (*tp.Accounts)[params.AccountIndex].AccAddress.String()
	denom, err := tp.extractSpecialValue(params.Denom)
	OrFail(err)
	msg := types.NewMsgMint(senderAddress, sdk.NewCoin(denom, sdk.NewInt(int64(params.Amount))))

	err = msg.ValidateBasic()
	OrFail(err)

	_, err = msgServer.Mint(sdk.WrapSDKContext(tp.Ctx), msg)
	OrFail(err)

	tp.SuccessActions[params.ActionType]++
}

func (tp *TestPlayer) replayBurnDenom(params *ActionBurnTokenFactory) {
	app := tp.App
	msgServer := keeper.NewMsgServerImpl(app.TokenFactoryKeeper)

	senderAddress := (*tp.Accounts)[params.AccountIndex].AccAddress.String()
	denom, err := tp.extractSpecialValue(params.Denom)
	OrFail(err)
	msg := types.NewMsgBurn(senderAddress, sdk.NewCoin(denom, sdk.NewInt(int64(params.Amount))))

	err = msg.ValidateBasic()
	OrFail(err)

	_, err = msgServer.Burn(sdk.WrapSDKContext(tp.Ctx), msg)
	OrFail(err)

	tp.SuccessActions[params.ActionType]++
}
