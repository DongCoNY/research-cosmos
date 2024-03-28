package keeper_test

import (
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/testtokenfactory"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

type KeeperTestSuite struct {
	testtokenfactory.KeeperTestHelper

	queryClient types.QueryClient
	msgServer   types.MsgServer
	// defaultDenom is on the suite, as it depends on the creator test address.
	defaultDenom    string
	defaultCreator  string
	defaultSubdenom string
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.Setup()
	// Fund every TestAcc with two denoms, one of which is the denom creation fee
	fundAccsAmount := sdk.NewCoins(sdk.NewCoin(types.DefaultParams().DenomCreationFee[0].Denom, types.DefaultParams().DenomCreationFee[0].Amount.MulRaw(100)), sdk.NewCoin(testtokenfactory.SecondaryDenom, testtokenfactory.SecondaryAmount))
	for _, acc := range suite.TestAccs {
		suite.FundAcc(acc, fundAccsAmount)
	}

	var tokenKeeper keeper.Keeper = suite.App.TokenFactoryKeeper

	suite.queryClient = types.NewQueryClient(suite.QueryHelper)
	suite.msgServer = keeper.NewMsgServerImpl(tokenKeeper)
}

func (suite *KeeperTestSuite) CreateDefaultDenom() {
	creator := suite.TestAccs[0].String()
	subdenom := "bitcoin"
	name := "Bitcoin"
	symbol := "BTC"
	res, _ := suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(creator, subdenom, name, symbol))

	suite.defaultDenom = res.GetNewTokenDenom()
	suite.defaultCreator = suite.TestAccs[0].String()
	suite.defaultSubdenom = subdenom
}

func (suite *KeeperTestSuite) TestCreateModuleAccount() {
	app := suite.App

	// remove module account
	tokenfactoryModuleAccount := app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	app.AccountKeeper.RemoveAccount(suite.Ctx, tokenfactoryModuleAccount)

	// ensure module account was removed
	suite.Ctx = app.BaseApp.NewContext(false, tmproto.Header{})
	tokenfactoryModuleAccount = app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	suite.Require().Nil(tokenfactoryModuleAccount)

	// create module account
	app.TokenFactoryKeeper.CreateModuleAccount(suite.Ctx)

	// check that the module account is now initialized
	tokenfactoryModuleAccount = app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	suite.Require().NotNil(tokenfactoryModuleAccount)
}
