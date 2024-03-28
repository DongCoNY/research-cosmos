package keeper_test

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func (suite *KeeperTestSuite) TestAdminMsgs() {
	addr0bal := int64(0)
	addr1bal := int64(0)

	bankKeeper := suite.App.BankKeeper

	suite.CreateDefaultDenom()
	// Make sure that the admin is set correctly
	queryRes, err := suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Creator:  suite.defaultCreator,
		SubDenom: suite.defaultSubdenom,
	})
	suite.Require().NoError(err)
	suite.Require().Equal(suite.TestAccs[0].String(), queryRes.AuthorityMetadata.Admin)

	// Test minting to admins own account
	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
	addr0bal += 10
	suite.Require().NoError(err)
	suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.Int64() == addr0bal, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom))

	// // Test force transferring
	// _, err = suite.msgServer.ForceTransfer(sdk.WrapSDKContext(suite.Ctx), types.NewMsgForceTransfer(suite.TestAccs[0].String(), sdk.NewInt64Coin(denom, 5), suite.TestAccs[1].String(), suite.TestAccs[0].String()))
	// suite.Require().NoError(err)
	// suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], denom).IsEqual(sdk.NewInt64Coin(denom, 15)))
	// suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], denom).IsEqual(sdk.NewInt64Coin(denom, 5)))

	// Test burning from own account
	_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))
	addr0bal -= 5
	suite.Require().NoError(err)
	suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.Int64() == addr1bal)

	// Test Change Admin
	_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), types.NewMsgChangeAdmin(suite.TestAccs[0].String(), suite.defaultDenom, suite.TestAccs[1].String()))
	queryRes, err = suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Creator:  suite.defaultCreator,
		SubDenom: suite.defaultSubdenom,
	})
	suite.Require().NoError(err)
	suite.Require().Equal(suite.TestAccs[1].String(), queryRes.AuthorityMetadata.Admin)

	// Admin can still burn tokens in balance
	_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))
	suite.Require().NoError(err)
	addr0bal -= 5

	// Make sure the new admin works
	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))
	addr1bal += 5
	suite.Require().NoError(err)
	suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.Int64() == addr1bal)

	// Try setting admin to empty
	_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), types.NewMsgChangeAdmin(suite.TestAccs[1].String(), suite.defaultDenom, ""))
	suite.Require().NoError(err)
	queryRes, err = suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Creator:  suite.defaultCreator,
		SubDenom: suite.defaultSubdenom,
	})
	suite.Require().NoError(err)
	suite.Require().Equal("", queryRes.AuthorityMetadata.Admin)

	// Make sure no admin can no longer mint
	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))
	suite.Require().Error(err)

	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))
	suite.Require().Error(err)

	// Make sure tokenholders can still burn their tokens
	_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))
	suite.Require().NoError(err)
	addr1bal -= 5

	// Make sure empty admin cannot burn
	_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn("", sdk.NewInt64Coin(suite.defaultDenom, 5)))
	suite.Require().Error(err)
}

// TestMintDenom ensures the following properties of the MintMessage:
// * Noone can mint tokens for a denom that doesn't exist
// * Only the admin of a denom can mint tokens for it
// * The admin of a denom can mint tokens for it
// * Cannot mint negative amount
func (suite *KeeperTestSuite) TestMintDenom() {
	var addr0bal int64

	// Create a denom
	suite.CreateDefaultDenom()

	for _, tc := range []struct {
		desc      string
		amount    int64
		mintDenom string
		admin     string
		valid     bool
		panics    bool
	}{
		{
			desc:      "denom does not exist",
			amount:    10,
			mintDenom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/evmos",
			admin:     suite.TestAccs[0].String(),
			valid:     false,
			panics:    false,
		},
		{
			desc:      "mint is not by the admin",
			amount:    10,
			mintDenom: suite.defaultDenom,
			admin:     suite.TestAccs[1].String(),
			valid:     false,
			panics:    false,
		},
		{
			desc:      "success case",
			amount:    10,
			mintDenom: suite.defaultDenom,
			admin:     suite.TestAccs[0].String(),
			valid:     true,
			panics:    false,
		},
		{
			desc:      "mint amount is negative",
			amount:    -10,
			mintDenom: suite.defaultDenom,
			admin:     suite.TestAccs[0].String(),
			valid:     false,
			panics:    true,
		},
		{
			desc:      "mint amount is zero",
			amount:    0,
			mintDenom: suite.defaultDenom,
			admin:     suite.TestAccs[0].String(),
			valid:     false,
			panics:    false,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			if tc.panics {
				suite.Panics(func() {
					suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(tc.admin, sdk.NewInt64Coin(tc.mintDenom, tc.amount)))
				})
				return
			}
			// Test minting to admins own account
			bankKeeper := suite.App.BankKeeper
			_, err := suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(tc.admin, sdk.NewInt64Coin(tc.mintDenom, tc.amount)))
			if tc.valid {
				addr0bal += 10
				suite.Require().NoError(err)
				suite.Require().Equal(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.Int64(), addr0bal, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom))
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestBurnDenom() {
	var addr0bal int64

	// Create a denom.
	suite.CreateDefaultDenom()

	// mint 10 default token for testAcc[0]
	suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
	addr0bal += 10

	for _, tc := range []struct {
		desc      string
		amount    int64
		burnDenom string
		admin     string
		valid     bool
		panics    bool
	}{
		{
			desc:      "denom does not exist",
			amount:    10,
			burnDenom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/evmos",
			admin:     suite.TestAccs[0].String(),
			valid:     false,
			panics:    false,
		},
		{
			desc:      "burn is not by the admin",
			amount:    10,
			burnDenom: suite.defaultDenom,
			admin:     suite.TestAccs[1].String(),
			valid:     false,
			panics:    false,
		},
		{
			desc:      "burn amount is negative",
			amount:    -1,
			burnDenom: suite.defaultDenom,
			admin:     suite.TestAccs[0].String(),
			valid:     false,
			panics:    true,
		},
		{
			desc:      "burn amount is zero",
			amount:    0,
			burnDenom: suite.defaultDenom,
			admin:     suite.TestAccs[0].String(),
			valid:     false,
			panics:    false,
		},
		{
			desc:      "success case",
			amount:    10,
			burnDenom: suite.defaultDenom,
			admin:     suite.TestAccs[0].String(),
			valid:     true,
			panics:    false,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			if tc.panics {
				suite.Panics(func() {
					suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(tc.admin, sdk.NewInt64Coin(tc.burnDenom, tc.amount)))
				})
				return
			}

			// Test minting to admins own account
			bankKeeper := suite.App.BankKeeper
			_, err := suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(tc.admin, sdk.NewInt64Coin(tc.burnDenom, tc.amount)))
			if tc.valid {
				addr0bal -= 10
				suite.Require().NoError(err)
				suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.Int64() == addr0bal, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom))
			} else {
				suite.Require().Error(err)
				suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.Int64() == addr0bal, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom))
			}
		})
	}
}

func (suite *KeeperTestSuite) TestAdminCannotBurnDenomTransferredToAnotherAccount() {
	var addr1bal int64

	// Create a denom.
	suite.CreateDefaultDenom()

	// mint 10 default token for testAcc[0]
	_, err := suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
	suite.Require().NoError(err)

	//send coins to testAcc[1] user
	msg := &banktypes.MsgSend{
		FromAddress: suite.TestAccs[0].String(),
		ToAddress:   suite.TestAccs[1].String(),
		Amount:      sdk.Coins{sdk.NewInt64Coin(suite.defaultDenom, 10)},
	}

	srv := bankkeeper.NewMsgServerImpl(suite.App.BankKeeper)
	_, err = srv.Send(suite.Ctx, msg)
	suite.NoError(err)
	//handler := bankmodule.NewHandler(suite.App.BankKeeper)
	//_, err = handler(suite.Ctx, msg)
	suite.Require().NoError(err)
	suite.Require().Equal(suite.App.BankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.String(), sdk.NewInt(0).String())
	suite.Require().Equal(suite.App.BankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.String(), sdk.NewInt(10).String())
	addr1bal += 10

	suite.Run("Case burn coins that were send to another address", func() {
		bankKeeper := suite.App.BankKeeper
		_, err := suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
		suite.Require().Error(err)
		_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
		suite.Require().NoError(err)
		suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.Int64() == 0, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom)) // new balance is 0 since burn is successful
	})
}

func (suite *KeeperTestSuite) TestNewAdminCannotBurnOldAdminsCoinsAfterAdminChange() {
	var addr0bal int64 = 0
	var addr1bal int64 = 0
	// Create a denom.
	suite.CreateDefaultDenom()

	// mint 10 default token for testAcc[0]
	_, err := suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
	addr0bal += 10
	suite.Require().NoError(err)

	// change admin to testAcc[1]
	_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), types.NewMsgChangeAdmin(suite.TestAccs[0].String(), suite.defaultDenom, suite.TestAccs[1].String()))
	suite.Require().NoError(err)

	// mint 10 default token for testAcc[1]
	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
	addr1bal += 10
	suite.Require().NoError(err)

	suite.Run(fmt.Sprintf("Case burn old admin's coins that after changing admin"), func() {
		bankKeeper := suite.App.BankKeeper
		_, err := suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, addr0bal+addr1bal)))
		suite.Require().Error(err)
		suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.Int64() == addr0bal, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom))
		suite.Require().True(bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.Int64() == addr1bal, bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom))
	})
}

func (suite *KeeperTestSuite) TestChangeAdminDenom() {
	for _, tc := range []struct {
		desc                    string
		msgChangeAdmin          func(denom string) *types.MsgChangeAdmin
		expectedChangeAdminPass bool
		expectedAdminIndex      int
		msgMint                 func(denom string) *types.MsgMint
		expectedMintPass        bool
	}{
		{
			desc: "creator admin can't mint after setting to '' ",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[0].String(), denom, "")
			},
			expectedChangeAdminPass: true,
			expectedAdminIndex:      -1,
			msgMint: func(denom string) *types.MsgMint {
				return types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(denom, 5))
			},
			expectedMintPass: false,
		},
		{
			desc: "non-admins can't change the existing admin",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[1].String(), denom, suite.TestAccs[2].String())
			},
			expectedChangeAdminPass: false,
			expectedAdminIndex:      0,
		},
		{
			desc: "success change admin",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[0].String(), denom, suite.TestAccs[1].String())
			},
			expectedAdminIndex:      1,
			expectedChangeAdminPass: true,
			msgMint: func(denom string) *types.MsgMint {
				return types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(denom, 5))
			},
			expectedMintPass: true,
		},
		{
			desc: "change admin to oneself and mint",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[0].String(), denom, suite.TestAccs[0].String())
			},
			expectedChangeAdminPass: true,
			expectedAdminIndex:      0,
			msgMint: func(denom string) *types.MsgMint {
				return types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(denom, 5))
			},
			expectedMintPass: true,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			// setup test
			suite.SetupTest()

			// Create a denom and mint
			res, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "bitcoin", "Bitcoin", "BTC"))
			suite.Require().NoError(err)

			testDenom := res.GetNewTokenDenom()

			_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(testDenom, 10)))
			suite.Require().NoError(err)

			_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), tc.msgChangeAdmin(testDenom))
			if tc.expectedChangeAdminPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

			queryRes, err := suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
				Creator:  suite.TestAccs[0].String(),
				SubDenom: "bitcoin",
			})
			suite.Require().NoError(err)

			// expectedAdminIndex with negative value is assumed as admin with value of ""
			const emptyStringAdminIndexFlag = -1
			if tc.expectedAdminIndex == emptyStringAdminIndexFlag {
				suite.Require().Equal("", queryRes.AuthorityMetadata.Admin)
			} else {
				suite.Require().Equal(suite.TestAccs[tc.expectedAdminIndex].String(), queryRes.AuthorityMetadata.Admin)
			}

			// we test mint to test if admin authority is performed properly after admin change.
			if tc.msgMint != nil {
				_, err := suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), tc.msgMint(testDenom))
				if tc.expectedMintPass {
					suite.Require().NoError(err)
				} else {
					suite.Require().Error(err)
				}
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSetDenomMetaData() {
	// setup test
	suite.SetupTest()
	suite.CreateDefaultDenom()

	for _, tc := range []struct {
		desc                string
		msgSetDenomMetadata types.MsgSetDenomMetadata
		expectedPass        bool
	}{
		{
			desc: "successful set denom metadata",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
					{
						Denom:    "inj",
						Exponent: 18,
					},
				},
				Base:    suite.defaultDenom,
				Display: "inj",
				Name:    "INJ",
				Symbol:  "INJ",
			}),
			expectedPass: true,
		},
		{
			desc: "non existent factory denom name",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    fmt.Sprintf("factory/%s/litecoin", suite.TestAccs[0].String()),
						Exponent: 0,
					},
					{
						Denom:    "inj",
						Exponent: 18,
					},
				},
				Base:    fmt.Sprintf("factory/%s/litecoin", suite.TestAccs[0].String()),
				Display: "inj",
				Name:    "INJ",
				Symbol:  "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "non-factory denom",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    "inj",
						Exponent: 0,
					},
					{
						Denom:    "injj",
						Exponent: 18,
					},
				},
				Base:    "inj",
				Display: "injj",
				Name:    "INJ",
				Symbol:  "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "wrong admin",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[1].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
					{
						Denom:    "inj",
						Exponent: 18,
					},
				},
				Base:    suite.defaultDenom,
				Display: "inj",
				Name:    "INJ",
				Symbol:  "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "invalid metadata (missing display)",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
				},
				Base:   suite.defaultDenom,
				Name:   "INJ",
				Symbol: "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "invalid metadata (missing name)",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
				},
				Base:    suite.defaultDenom,
				Display: "inj",
				Symbol:  "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "invalid metadata (missing symbol)",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
				},
				Base:    suite.defaultDenom,
				Display: "inj",
				Name:    "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "invalid metadata (missing base)",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
				},
				Display: "inj",
				Name:    "INJ",
				Symbol:  "INJ",
			}),
			expectedPass: false,
		},
		{
			desc: "invalid metadata (missing denom units)",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "yeehaw",
				DenomUnits:  []*banktypes.DenomUnit{},
				Base:        suite.defaultDenom,
				Display:     "inj",
				Name:        "INJ",
				Symbol:      "INJ",
			}),
			expectedPass: false,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			bankKeeper := suite.App.BankKeeper
			res, err := suite.msgServer.SetDenomMetadata(sdk.WrapSDKContext(suite.Ctx), &tc.msgSetDenomMetadata)
			if tc.expectedPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)

				md, found := bankKeeper.GetDenomMetaData(suite.Ctx, suite.defaultDenom)
				suite.Require().True(found)
				suite.Require().Equal(tc.msgSetDenomMetadata.Metadata.Name, md.Name)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
