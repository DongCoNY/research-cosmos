package keeper_test

import (
	"testing"
	"time"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle"
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

func EndBlockerAndCommit(ctx sdk.Context, counter int) sdk.Context {
	for i := 0; i < counter; i++ {
		endBlockReq := abci.RequestEndBlock{Height: ctx.BlockHeight()}
		oracle.EndBlocker(ctx, endBlockReq)

		// build new context with height and time
		height := ctx.BlockHeight() + 1
		offset := time.Duration((height) * 1000000000)
		header := tmproto.Header{Height: height, Time: time.Unix(1618997040, 0).Add(offset)}
		ctx = ctx.WithBlockHeader(header)
	}
	return ctx
}
