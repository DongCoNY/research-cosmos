package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func (suite *KeeperTestSuite) TestGenesis() {
	genesisState := types.GenesisState{
		FactoryDenoms: []types.GenesisDenom{
			{
				Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
				AuthorityMetadata: types.DenomAuthorityMetadata{
					Admin: "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
				},
				Name:   "Bitcoin",
				Symbol: "BTC",
			},
			{
				Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/diff-admin",
				AuthorityMetadata: types.DenomAuthorityMetadata{
					Admin: "inj1rv886hm7zfhqpxnaa83mwgue0lw2j3y7ef0kdr",
				},
			},
			{
				Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/litecoin",
				AuthorityMetadata: types.DenomAuthorityMetadata{
					Admin: "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
				},
				Name:   "Litecoin",
				Symbol: "LTC",
			},
		},
	}

	suite.SetupTestForInitGenesis()
	app := suite.App

	// Test both with bank denom metadata set, and not set.
	for i, denom := range genesisState.FactoryDenoms {
		// hacky, sets bank metadata to exist if i != 0, to cover both cases.
		if i != 0 {
			app.BankKeeper.SetDenomMetaData(suite.Ctx, banktypes.Metadata{Base: denom.GetDenom(), Name: denom.GetName(), Symbol: denom.GetSymbol()})
		}
	}

	// check before initGenesis that the module account is nil
	tokenfactoryModuleAccount := app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	suite.Require().Nil(tokenfactoryModuleAccount)

	app.TokenFactoryKeeper.SetParams(suite.Ctx, types.Params{DenomCreationFee: sdk.Coins{sdk.NewInt64Coin("inj", 100)}})
	app.TokenFactoryKeeper.InitGenesis(suite.Ctx, genesisState)

	// check that the module account is now initialized
	tokenfactoryModuleAccount = app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	suite.Require().NotNil(tokenfactoryModuleAccount)

	exportedGenesis := app.TokenFactoryKeeper.ExportGenesis(suite.Ctx)
	suite.Require().NotNil(exportedGenesis)
	suite.Require().Equal(genesisState, *exportedGenesis)
}

func (suite *KeeperTestSuite) TestInvalidGenesis() {
	for _, genesisState := range []types.GenesisState{
		{
			FactoryDenoms: []types.GenesisDenom{
				{
					Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
					AuthorityMetadata: types.DenomAuthorityMetadata{
						Admin: "what a wrong address",
					},
				},
				{
					Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/admin",
					AuthorityMetadata: types.DenomAuthorityMetadata{
						Admin: "inj1rv886hm7zfhqpxnaa83mwgue0lw2j3y7ef0kdr",
					},
				},
			},
		},
		{
			FactoryDenoms: []types.GenesisDenom{
				{
					Denom: "incorrect denom",
					AuthorityMetadata: types.DenomAuthorityMetadata{
						Admin: "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
					},
				},
				{
					Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/admin",
					AuthorityMetadata: types.DenomAuthorityMetadata{
						Admin: "inj1rv886hm7zfhqpxnaa83mwgue0lw2j3y7ef0kdr",
					},
				},
			},
		},
	} {
		suite.SetupTestForInitGenesis()
		app := suite.App

		// Test both with bank denom metadata set, and not set.
		for i, denom := range genesisState.FactoryDenoms {
			// hacky, sets bank metadata to exist if i != 0, to cover both cases.
			if i != 0 {
				app.BankKeeper.SetDenomMetaData(suite.Ctx, banktypes.Metadata{Base: denom.GetDenom()})
			}
		}

		// check before initGenesis that the module account is nil
		tokenfactoryModuleAccount := app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
		suite.Require().Nil(tokenfactoryModuleAccount)

		app.TokenFactoryKeeper.SetParams(suite.Ctx, types.Params{DenomCreationFee: sdk.Coins{sdk.NewInt64Coin("inj", 100)}})
		suite.Require().Panics(func() { app.TokenFactoryKeeper.InitGenesis(suite.Ctx, genesisState) })
	}
}

func (suite *KeeperTestSuite) TestEmptyGenesis() {
	suite.SetupTestForInitGenesis()
	app := suite.App

	// check before initGenesis that the module account is nil
	tokenfactoryModuleAccount := app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	suite.Require().Nil(tokenfactoryModuleAccount)

	app.TokenFactoryKeeper.SetParams(suite.Ctx, types.Params{DenomCreationFee: sdk.Coins{sdk.NewInt64Coin("inj", 100)}})
	app.TokenFactoryKeeper.InitGenesis(suite.Ctx, types.GenesisState{})

	// check that the module account is now initialized
	tokenfactoryModuleAccount = app.AccountKeeper.GetAccount(suite.Ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName))
	suite.Require().NotNil(tokenfactoryModuleAccount)

	exportedGenesis := app.TokenFactoryKeeper.ExportGenesis(suite.Ctx)
	suite.Require().NotNil(exportedGenesis)
	suite.Require().Equal(types.GenesisState{Params: types.Params{DenomCreationFee: sdk.Coins(nil)}, FactoryDenoms: []types.GenesisDenom{}}, *exportedGenesis)
}
