package keeper_test

import (
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	testifysuite "github.com/stretchr/testify/suite"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
)

type KeeperTestSuite struct {
	testifysuite.Suite

	ctx sdk.Context
	app *app.InjectiveApp
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.app = app.Setup(false)
	suite.ctx = suite.app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
}

func TestKeeperTestSuite(t *testing.T) {
	testifysuite.Run(t, new(KeeperTestSuite))
}
