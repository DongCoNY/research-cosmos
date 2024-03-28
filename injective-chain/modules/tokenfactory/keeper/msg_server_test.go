package keeper_test

import (
	"fmt"
	"strings"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// TestMintDenomMsg tests TypeMsgMint message is emitted on a successful mint
func (suite *KeeperTestSuite) TestMintDenomMsg() {
	// Create a denom
	suite.CreateDefaultDenom()

	for _, tc := range []struct {
		desc                  string
		amount                int64
		mintDenom             string
		admin                 string
		valid                 bool
		expectedMessageEvents int
	}{
		{
			desc:      "denom does not exist",
			amount:    10,
			mintDenom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/evmos",
			admin:     suite.TestAccs[0].String(),
			valid:     false,
		},
		{
			desc:                  "success case",
			amount:                10,
			mintDenom:             suite.defaultDenom,
			admin:                 suite.TestAccs[0].String(),
			valid:                 true,
			expectedMessageEvents: 1,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
			suite.Require().Equal(0, len(ctx.EventManager().Events()))
			// Test mint message
			suite.msgServer.Mint(sdk.WrapSDKContext(ctx), types.NewMsgMint(tc.admin, sdk.NewInt64Coin(tc.mintDenom, 10)))
			// Ensure current number and type of event is emitted
			suite.AssertTypedEventEmitted(ctx, &types.EventMintTFDenom{}, tc.expectedMessageEvents)
		})
	}
}

// TestBurnDenomMsg tests TypeMsgBurn message is emitted on a successful burn
func (suite *KeeperTestSuite) TestBurnDenomMsg() {
	// Create a denom.
	suite.CreateDefaultDenom()
	// mint 10 default token for testAcc[0]
	suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))

	for _, tc := range []struct {
		desc                  string
		amount                int64
		burnDenom             string
		admin                 string
		valid                 bool
		expectedMessageEvents int
	}{
		{
			desc:      "denom does not exist",
			burnDenom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/evmos",
			admin:     suite.TestAccs[0].String(),
			valid:     false,
		},
		{
			desc:                  "can burn inj denom",
			burnDenom:             "inj",
			admin:                 suite.TestAccs[0].String(),
			valid:                 true,
			expectedMessageEvents: 1,
		},
		{
			desc:                  "success case",
			burnDenom:             suite.defaultDenom,
			admin:                 suite.TestAccs[0].String(),
			valid:                 true,
			expectedMessageEvents: 1,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
			suite.Require().Equal(0, len(ctx.EventManager().Events()))
			// Test burn message
			suite.msgServer.Burn(sdk.WrapSDKContext(ctx), types.NewMsgBurn(tc.admin, sdk.NewInt64Coin(tc.burnDenom, 10)))
			// Ensure current number and type of event is emitted
			suite.AssertTypedEventEmitted(ctx, &types.EventBurnDenom{}, tc.expectedMessageEvents)
		})
	}
}

func (suite *KeeperTestSuite) TestCreateDenomWithAnotherDenomFeeMsg() {
	suite.SetupTest()
	suite.Run("pay for creation with another token factory denom", func() {
		// Create a denom.
		suite.CreateDefaultDenom()

		_, err := suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))
		suite.Require().NoError(err)

		tokenFactoryKeeper := suite.App.TokenFactoryKeeper
		ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
		suite.Require().Equal(0, len(ctx.EventManager().Events()))

		// Set denom creation fee in params
		denomCreationFee := types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin(suite.defaultDenom, sdk.NewInt(10)))}
		tokenFactoryKeeper.SetParams(suite.Ctx, denomCreationFee)

		// Test create denom message
		_, err = suite.msgServer.CreateDenom(sdk.WrapSDKContext(ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "subdenom", "Subdenom", "SBD"))
		suite.Require().NoError(err)

		// Ensure current number and type of event is emitted
		suite.AssertTypedEventEmitted(ctx, &types.EventCreateTFDenom{}, 1)
	})
}

// TestCreateDenomMsg tests TypeMsgCreateDenom message is emitted on a successful denom creation
func (suite *KeeperTestSuite) TestCreateDenomMsg() {
	defaultDenomCreationFee := types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(50000000)))}
	for _, tc := range []struct {
		desc                  string
		denomCreationFee      types.Params
		subdenom              string
		name                  string
		symbol                string
		valid                 bool
		expectedMessageEvents int
	}{
		{
			desc:             "subdenom too long",
			denomCreationFee: defaultDenomCreationFee,
			subdenom:         "assadsadsadasdasdsadsadsadsadsadsadsklkadaskkkdasdasedskhanhassyeunganassfnlksdflksafjlkasd",
			name:             "AASAKSNDHDIAKLASLSLALS",
			symbol:           "SAKSAKASKSAK",
			valid:            false,
		},
		{
			desc:             "insufficient funds",
			denomCreationFee: types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("eth", sdk.NewInt(10)))},
			subdenom:         "taktotoshi",
			name:             "Taktotoshi",
			symbol:           "TKTTS",
			valid:            false,
		},
		{
			desc:                  "success case: defaultDenomCreationFee",
			denomCreationFee:      defaultDenomCreationFee,
			subdenom:              "evmos",
			name:                  "Evmos",
			symbol:                "EVMOS",
			valid:                 true,
			expectedMessageEvents: 1,
		},
	} {
		suite.SetupTest()
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			tokenFactoryKeeper := suite.App.TokenFactoryKeeper
			ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
			suite.Require().Equal(0, len(ctx.EventManager().Events()))
			// Set denom creation fee in params
			tokenFactoryKeeper.SetParams(suite.Ctx, tc.denomCreationFee)
			// Test create denom message
			_, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), tc.subdenom, tc.name, tc.symbol))
			if tc.valid {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
			// Ensure current number and type of event is emitted
			suite.AssertTypedEventEmitted(ctx, &types.EventCreateTFDenom{}, tc.expectedMessageEvents)
		})
	}
}

// TestEmptyMetadataCreateDenomMsg tests TypeMsgCreateDenom message is emitted with missing name and symbol
func (suite *KeeperTestSuite) TestEmptyMetadataCreateDenomMsg() {
	defaultDenomCreationFee := types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(50000000)))}
	for _, tc := range []struct {
		desc                  string
		denomCreationFee      types.Params
		subdenom              string
		valid                 bool
		expectedMessageEvents int
	}{
		{
			desc:             "subdenom too long",
			denomCreationFee: defaultDenomCreationFee,
			subdenom:         "assadsadsadasdasdsadsadsadsadsadsadsklkadaskkkdasdasedskhanhassyeunganassfnlksdflksafjlkasd",
			valid:            false,
		},
		{
			desc:                  "success case",
			denomCreationFee:      defaultDenomCreationFee,
			subdenom:              "taktotoshi",
			valid:                 true,
			expectedMessageEvents: 1,
		},
		{
			desc:                  "success case: defaultDenomCreationFee",
			denomCreationFee:      defaultDenomCreationFee,
			subdenom:              "evmos",
			valid:                 true,
			expectedMessageEvents: 1,
		},
	} {
		suite.SetupTest()
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			tokenFactoryKeeper := suite.App.TokenFactoryKeeper
			ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
			suite.Require().Equal(0, len(ctx.EventManager().Events()))
			// Set denom creation fee in params
			tokenFactoryKeeper.SetParams(suite.Ctx, tc.denomCreationFee)
			// Test create denom message
			msgCreateDenom := types.MsgCreateDenom{
				Sender:   suite.TestAccs[0].String(),
				Subdenom: tc.subdenom,
			}
			_, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(ctx), &msgCreateDenom)
			if tc.valid {
				suite.Require().NoError(err)
				tokenDenom := fmt.Sprintf("factory/%s/%s", suite.TestAccs[0].String(), tc.subdenom)
				tokenMetadata, _ := suite.App.BankKeeper.GetDenomMetaData(ctx, tokenDenom)
				expectedDenom := strings.Split(tokenMetadata.DenomUnits[0].Denom, "/")[2]
				suite.Require().Equal(tc.subdenom, expectedDenom)
				suite.Require().Equal("", tokenMetadata.Name)
				suite.Require().Equal("", tokenMetadata.Symbol)
			} else {
				suite.Require().Error(err)
			}
			// Ensure current number and type of event is emitted
			suite.AssertTypedEventEmitted(ctx, &types.EventCreateTFDenom{}, tc.expectedMessageEvents)
		})
	}
}

// TestChangeAdminDenomMsg tests TypeMsgChangeAdmin message is emitted on a successful admin change
func (suite *KeeperTestSuite) TestChangeAdminDenomMsg() {
	for _, tc := range []struct {
		desc                    string
		msgChangeAdmin          func(denom string) *types.MsgChangeAdmin
		expectedChangeAdminPass bool
		expectedAdminIndex      int
		msgMint                 func(denom string) *types.MsgMint
		expectedMintPass        bool
		expectedMessageEvents   int
	}{
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
			expectedMessageEvents:   1,
			msgMint: func(denom string) *types.MsgMint {
				return types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(denom, 5))
			},
			expectedMintPass: true,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			// setup test
			suite.SetupTest()
			ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
			suite.Require().Equal(0, len(ctx.EventManager().Events()))
			// Create a denom and mint
			res, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "bitcoin", "Bitcoin", "BTC"))
			suite.Require().NoError(err)
			testDenom := res.GetNewTokenDenom()
			suite.msgServer.Mint(sdk.WrapSDKContext(ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(testDenom, 10)))
			// Test change admin message
			suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(ctx), tc.msgChangeAdmin(testDenom))
			// Ensure current number and type of event is emitted
			suite.AssertTypedEventEmitted(ctx, &types.EventChangeTFAdmin{}, tc.expectedMessageEvents)
		})
	}
}

// TestSetDenomMetaDataMsg tests TypeMsgSetDenomMetadata message is emitted on a successful denom metadata change
func (suite *KeeperTestSuite) TestSetDenomMetaDataMsg() {
	// setup test
	suite.SetupTest()
	suite.CreateDefaultDenom()

	for _, tc := range []struct {
		desc                  string
		msgSetDenomMetadata   types.MsgSetDenomMetadata
		expectedPass          bool
		expectedMessageEvents int
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
			expectedPass:          true,
			expectedMessageEvents: 1,
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
			desc: "incorrectly first denom",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "ouch",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    "inj",
						Exponent: 18,
					},
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
				},
				Base:    suite.defaultDenom,
				Display: "dob",
				Name:    "DOB",
				Symbol:  "DOB",
			}),
			expectedPass: false,
		},
		{
			desc: "incorrectly sorted denom units",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "ouch",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
					{
						Denom:    "inj",
						Exponent: 0,
					},
				},
				Base:    suite.defaultDenom,
				Display: "dob",
				Name:    "DOB",
				Symbol:  "DOB",
			}),
			expectedPass: false,
		},
		{
			desc: "non-zero expontent",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "ouch",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 1,
					},
				},
				Base:    suite.defaultDenom,
				Display: "dob",
				Name:    "DOB",
				Symbol:  "DOB",
			}),
			expectedPass: false,
		},
		{
			desc: "duplicated data in denom unit",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "ouch",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
					{
						Denom:    suite.defaultDenom,
						Exponent: 1,
					},
				},
				Base:    suite.defaultDenom,
				Display: "dob",
				Name:    "DOB",
				Symbol:  "DOB",
			}),
			expectedPass: false,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			ctx := suite.Ctx.WithEventManager(sdk.NewEventManager())
			suite.Require().Equal(0, len(ctx.EventManager().Events()))

			// Test set denom metadata message
			_, err := suite.msgServer.SetDenomMetadata(sdk.WrapSDKContext(ctx), &tc.msgSetDenomMetadata)
			if tc.expectedPass {
				suite.Require().NoError(err)
			} else {
				fmt.Printf("error: %v\n", err.Error())
				suite.Require().Error(err)
			}

			// Ensure current number and type of event is emitted
			suite.AssertTypedEventEmitted(ctx, &types.EventSetTFDenomMetadata{}, tc.expectedMessageEvents)
			fmt.Printf("expected events: %v\n", tc.expectedMessageEvents)
		})
	}
}
