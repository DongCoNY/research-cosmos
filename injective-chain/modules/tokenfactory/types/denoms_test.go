package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func TestDeconstructDenom(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	for _, tc := range []struct {
		desc             string
		denom            string
		expectedSubdenom string
		err              error
	}{
		{
			desc:  "empty is invalid",
			denom: "",
			err:   types.ErrInvalidDenom,
		},
		{
			desc:  "inj is invalid",
			denom: "inj",
			err:   types.ErrInvalidDenom,
		},
		{
			desc:             "normal",
			denom:            "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
			expectedSubdenom: "bitcoin",
		},
		{
			desc:             "multiple slashes in subdenom",
			denom:            "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin/1/abc/a",
			expectedSubdenom: "bitcoin/1/abc/a",
		},
		{
			desc:  "empty subdenom",
			denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/",
			err:   types.ErrSubdenomTooShort,
		},
		{
			desc:  "whitespace subdenom",
			denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/ ",
			err:   types.ErrInvalidDenom,
		},
		{
			desc:  "incorrect prefix",
			denom: "ibc/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
			err:   types.ErrInvalidDenom,
		},
		{
			desc:  "subdenom of only slashes",
			denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/////",
			err:   types.ErrSubdenomTooShort,
		},
		{
			desc:  "invalid nested subdenom",
			denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/a/b/c//e",
			err:   types.ErrSubdenomNestedTooShort,
		},
		{
			desc:  "nested subdenom with whitespace",
			denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/a/b/c/ /e",
			err:   types.ErrInvalidDenom,
		},
		{
			desc:  "too long name",
			denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/adsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsf",
			err:   types.ErrInvalidDenom,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			expectedCreator := "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt"
			creator, subdenom, err := types.DeconstructDenom(tc.denom)
			if tc.err != nil {
				require.ErrorContains(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, expectedCreator, creator)
				require.Equal(t, tc.expectedSubdenom, subdenom)
			}
		})
	}
}

func TestGetTokenDenom(t *testing.T) {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	for _, tc := range []struct {
		desc     string
		creator  string
		subdenom string
		valid    bool
	}{
		{
			desc:     "normal",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "bitcoin",
			valid:    true,
		},
		{
			desc:     "multiple slashes in subdenom",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "bitcoin/1",
			valid:    true,
		},
		{
			desc:     "no subdenom",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "",
			valid:    false,
		},
		{
			desc:     "subdenom of only slashes",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "/////",
			valid:    false,
		},
		{
			desc:     "subdenom with many valid slashes",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "a/b/c/d/e",
			valid:    true,
		},
		{
			desc:     "subdenom of with one incorrect slash",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "/a/b//d/e",
			valid:    false,
		},
		{
			desc:     "too long name",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "adsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsfadsf",
			valid:    false,
		},
		{
			desc:     "subdenom is exactly max length",
			creator:  "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
			subdenom: "bitcoinfsadfsdfeadfsafwefsefsefsdfsdafasefsf",
			valid:    true,
		},
		{
			desc:     "creator is empty",
			creator:  "",
			subdenom: "bitcoin",
			valid:    false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := types.GetTokenDenom(tc.creator, tc.subdenom)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestNewTokenFactoryDenomMintCoinsRestriction(t *testing.T) {
	ctx := sdk.Context{} // Create a mock context for testing
	coinsToMint := sdk.Coins{
		{Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin"},
		{Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/ethereum"},
	}

	err := types.NewTokenFactoryDenomMintCoinsRestriction()(ctx, coinsToMint)
	require.Nil(t, err)

	invalidCoins := sdk.Coins{{Denom: "invalid/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/invalid"}}

	err = types.NewTokenFactoryDenomMintCoinsRestriction()(ctx, invalidCoins)
	require.NotNil(t, err)
	require.Equal(t, "does not have permission to mint invalid/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/invalid", err.Error())
}
