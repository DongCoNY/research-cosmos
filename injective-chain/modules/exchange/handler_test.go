package exchange_test

import (
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
)

type HandlerTestSuite struct {
	suite.Suite

	app *app.InjectiveApp
	ctx sdk.Context

	handler sdk.Handler
}

func (suite *HandlerTestSuite) SetupTest() {
	suite.app = app.Setup(false)
	suite.ctx = suite.app.NewContext(true, tmproto.Header{Height: suite.app.LastBlockHeight()})
	suite.handler = exchange.NewHandler(suite.app.ExchangeKeeper)
}
