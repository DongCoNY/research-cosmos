package keeper_test

import (
	"fmt"
	"time"

	tmtypes "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
	tfkeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/keeper"
	tftypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

type KeeperTestSuite struct {
	App         *app.InjectiveApp
	Ctx         sdk.Context
	MsgServer   types.MsgServer
	TFMsgServer tftypes.MsgServer

	TestAccs []sdk.AccAddress
}

func setupTestSuite(t require.TestingT, numAccts int) (*KeeperTestSuite, keeper.Keeper) {
	s := &KeeperTestSuite{}
	s.App = app.Setup(false)
	s.Ctx = s.App.BaseApp.NewContext(false, tmtypes.Header{Height: 1, ChainID: "inj-test", Time: time.Now().UTC()})
	s.MsgServer = keeper.NewMsgServerImpl(s.App.PermissionsKeeper)
	s.TFMsgServer = tfkeeper.NewMsgServerImpl(s.App.TokenFactoryKeeper)

	s.TestAccs = createRandomAccounts(numAccts)

	var (
		secondaryDenom  = "usdt"
		secondaryAmount = sdk.NewInt(100000000)
	)
	// Fund every TestAcc with two denoms, one of which is the denom creation fee
	fundAccsAmount := sdk.NewCoins(sdk.NewCoin(tftypes.DefaultParams().DenomCreationFee[0].Denom, tftypes.DefaultParams().DenomCreationFee[0].Amount.MulRaw(100)), sdk.NewCoin(secondaryDenom, secondaryAmount))
	for _, acc := range s.TestAccs {
		err := banktestutil.FundAccount(s.App.BankKeeper, s.Ctx, acc, fundAccsAmount)
		require.NoError(t, err)
	}

	return s, s.App.PermissionsKeeper
}

// createRandomAccounts is a function return a list of randomly generated AccAddresses
func createRandomAccounts(numAccts int) []sdk.AccAddress {
	testAddrs := make([]sdk.AccAddress, numAccts)
	for i := 0; i < numAccts; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		testAddrs[i] = sdk.AccAddress(pk.Address())
	}

	return testAddrs
}

func formatDenom(sender, subdenom string) string {
	return fmt.Sprintf("factory/%s/%s", sender, subdenom)
}
