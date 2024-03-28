package keeper_test

import (
	"fmt"
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/testtokenfactory"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func (suite *KeeperTestSuite) TestMsgCreateDenom() {
	var (
		tokenFactoryKeeper = suite.App.TokenFactoryKeeper
		bankKeeper         = suite.App.BankKeeper
		denomCreationFee   = tokenFactoryKeeper.GetParams(suite.Ctx).DenomCreationFee
	)

	// Get balance of acc 0 before creating a denom
	preCreateBalance := bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], denomCreationFee[0].Denom)

	// Creating a denom should work
	res, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "bitcoin", "Bitcoin", "BTC"))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(res.GetNewTokenDenom())

	// Make sure that the admin is set correctly
	queryRes, err := suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Creator:  suite.TestAccs[0].String(),
		SubDenom: "bitcoin",
	})
	suite.Require().NoError(err)
	suite.Require().Equal(suite.TestAccs[0].String(), queryRes.AuthorityMetadata.Admin)

	// Make sure that creation fee was deducted
	postCreateBalance := bankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], tokenFactoryKeeper.GetParams(suite.Ctx).DenomCreationFee[0].Denom)
	suite.Require().True(preCreateBalance.Sub(postCreateBalance).IsEqual(denomCreationFee[0]))

	// Make sure that a second version of the same denom can't be recreated
	res, err = suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "bitcoin", "Bitcoin", "BTC"))
	suite.Require().Error(err)

	// Creating a second denom should work
	res, err = suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "litecoin", "Litecoin", "LTC"))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(res.GetNewTokenDenom())

	// Try querying all the denoms created by suite.TestAccs[0]
	queryRes2, err := suite.queryClient.DenomsFromCreator(suite.Ctx.Context(), &types.QueryDenomsFromCreatorRequest{
		Creator: suite.TestAccs[0].String(),
	})
	suite.Require().NoError(err)
	suite.Require().Len(queryRes2.Denoms, 2)

	// Make sure that a second account can create a denom with the same subdenom
	res, err = suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[1].String(), "bitcoin", "Bitcoin", "BTC"))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(res.GetNewTokenDenom())

	// Make sure that an address with a "/" in it can't create denoms
	res, err = suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom("hello.eth/creator", "bitcoin", "Bitcoin", "BTC"))
	suite.Require().Error(err)
}

func (suite *KeeperTestSuite) TestCreateDenom() {
	var (
		primaryDenom            = types.DefaultParams().DenomCreationFee[0].Denom
		secondaryDenom          = testtokenfactory.SecondaryDenom
		defaultDenomCreationFee = types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin(primaryDenom, sdk.NewInt(50000000)))}
		twoDenomCreationFee     = types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin(primaryDenom, sdk.NewInt(50000000)), sdk.NewCoin(secondaryDenom, sdk.NewInt(50000000)))}
		nilCreationFee          = types.Params{DenomCreationFee: nil}

		largeAmount, _   = sdk.NewIntFromString("500000000000000000000000")
		largeCreationFee = types.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin(primaryDenom, largeAmount))}
		letters          = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

		generateSubdenomWithLength = func(length int) string {
			b := make([]rune, length)
			for i := range b {
				b[i] = letters[rand.Intn(len(letters))]
			}
			return string(b)
		}
	)

	for _, tc := range []struct {
		desc             string
		denomCreationFee types.Params
		setup            func()
		subdenom         string
		name             string
		symbol           string
		valid            bool
	}{
		{
			desc:             "subdenom too long by 1 character",
			denomCreationFee: defaultDenomCreationFee,
			subdenom:         generateSubdenomWithLength(types.MaxSubdenomLength + 1),
			name:             "Bitcoin",
			symbol:           "BTC",
			valid:            false,
		},
		{
			desc:             "subdenom with max length",
			denomCreationFee: defaultDenomCreationFee,
			subdenom:         generateSubdenomWithLength(types.MaxSubdenomLength),
			name:             "Bitcoin",
			symbol:           "BTC",
			valid:            true,
		},
		{
			desc:             "subdenom and creator pair already exists",
			denomCreationFee: defaultDenomCreationFee,
			setup: func() {
				_, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "bitcoin", "Bitcoin", "BTC"))
				suite.Require().NoError(err)
			},
			subdenom: "bitcoin",
			name:     "Bitcoin",
			symbol:   "BTC",
			valid:    false,
		},
		{
			desc:             "success case: defaultDenomCreationFee",
			denomCreationFee: defaultDenomCreationFee,
			subdenom:         "evmos",
			name:             "Evmos",
			symbol:           "EVMOS",
			valid:            true,
		},
		{
			desc:             "success case: twoDenomCreationFee",
			denomCreationFee: twoDenomCreationFee,
			subdenom:         "catcoin",
			name:             "Catcoin",
			symbol:           "CTC",
			valid:            true,
		},
		{
			desc:             "success case: nilCreationFee",
			denomCreationFee: nilCreationFee,
			subdenom:         "czcoin",
			name:             "CZ coin",
			symbol:           "CZC",
			valid:            true,
		},
		{
			desc:             "account doesn't have enough to pay for denom creation fee",
			denomCreationFee: largeCreationFee,
			subdenom:         "tooexpensive",
			name:             "Bitcoin",
			symbol:           "BTC",
			valid:            false,
		},
		{
			desc:             "subdenom having invalid characters",
			denomCreationFee: defaultDenomCreationFee,
			subdenom:         "bit/***///&&&/coin",
			name:             "Bitcoin",
			symbol:           "BTC",
			valid:            false,
		},
	} {
		suite.SetupTest()
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			if tc.setup != nil {
				tc.setup()
			}
			tokenFactoryKeeper := suite.App.TokenFactoryKeeper
			bankKeeper := suite.App.BankKeeper
			// Set denom creation fee in params
			tokenFactoryKeeper.SetParams(suite.Ctx, tc.denomCreationFee)
			denomCreationFee := tokenFactoryKeeper.GetParams(suite.Ctx).DenomCreationFee
			suite.Require().Equal(tc.denomCreationFee.DenomCreationFee, denomCreationFee)

			// note balance, create a tokenfactory denom, then note balance again
			preCreateBalance := bankKeeper.GetAllBalances(suite.Ctx, suite.TestAccs[0])
			_, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), tc.subdenom, tc.name, tc.symbol))
			postCreateBalance := bankKeeper.GetAllBalances(suite.Ctx, suite.TestAccs[0])
			if tc.valid {
				suite.Require().NoError(err)
				suite.Require().True(preCreateBalance.Sub(postCreateBalance...).IsEqual(denomCreationFee))

				// Make sure that the admin is set correctly
				queryRes, err := suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
					Creator:  suite.TestAccs[0].String(),
					SubDenom: tc.subdenom,
				})

				suite.Require().NoError(err)
				suite.Require().Equal(suite.TestAccs[0].String(), queryRes.AuthorityMetadata.Admin)

			} else {
				suite.Require().Error(err)
				// Ensure we don't charge if we expect an error
				suite.Require().True(preCreateBalance.IsEqual(postCreateBalance))
			}
		})
	}
}
