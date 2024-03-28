package keeper_test

import (
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
)

func TestKeeper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keeper Suite")
}

func EndBlockerAndCommit(app *simapp.InjectiveApp, ctx sdk.Context, counter int) sdk.Context {
	appModule := wasmx.NewAppModule(app.WasmxKeeper, app.AccountKeeper, app.BankKeeper, app.ExchangeKeeper, app.GetSubspace(wasmxtypes.ModuleName))
	for i := 0; i < counter; i++ {
		endBlockReq := abci.RequestEndBlock{Height: ctx.BlockHeight()}
		appModule.EndBlock(ctx, endBlockReq)

		// build new context with height and time
		height := ctx.BlockHeight() + 1
		offset := time.Duration((height) * 1000000000)
		header := tmproto.Header{Height: height, Time: time.Now().Add(offset)}
		ctx = ctx.WithBlockHeader(header)
	}
	return ctx
}
