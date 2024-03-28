package keeper_test

import (
	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	"testing"
	"time"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/auction"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKeeper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keeper Suite")
}

func EndBlockerAndCommit(app *simapp.InjectiveApp, ctx sdk.Context, counter int) sdk.Context {
	appModule := auction.NewAppModule(
		app.AuctionKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		app.ExchangeKeeper,
		app.GetSubspace(auctiontypes.ModuleName),
	)

	for i := 0; i < counter; i++ {
		endBlockReq := abci.RequestEndBlock{Height: ctx.BlockHeight()}
		appModule.EndBlocker(ctx, endBlockReq)

		// build new context with height and time
		height := ctx.BlockHeight() + 1
		offset := time.Duration((height) * 1000000000)
		header := tmproto.Header{Height: height, Time: time.Now().Add(offset)}
		ctx = ctx.WithBlockHeader(header)
	}
	return ctx
}
