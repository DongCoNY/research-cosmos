package oracle

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	bandoracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/bandchain/oracle/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/bandtesting/x/oracle/keeper"
)

// NewHandler creates the msg handler of this module, as required by Cosmos-SDK standard.
func NewHandler(k keeper.Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		switch msg := msg.(type) {
		default:
			return nil, errors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized %s message type: %T", bandoracletypes.ModuleName, msg)
		}
	}
}
