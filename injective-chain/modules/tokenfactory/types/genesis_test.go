package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func TestGenesisState_Validate(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
						},
					},
				},
			},
			valid: true,
		},
		{
			desc: "different admin from creator",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "inj1gaac6hhhctzzmwzdad24gxwds97rx6m0p8j0vu",
						},
					},
				},
			},
			valid: true,
		},
		{
			desc: "empty admin",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "",
						},
					},
				},
			},
			valid: true,
		},
		{
			desc: "no admin",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
					},
				},
			},
			valid: true,
		},
		{
			desc: "invalid admin",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "moose",
						},
					},
				},
			},
			valid: false,
		},
		{
			desc: "multiple denoms",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "",
						},
					},
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/litecoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "",
						},
					},
				},
			},
			valid: true,
		},
		{
			desc: "duplicate denoms",
			genState: &types.GenesisState{
				FactoryDenoms: []types.GenesisDenom{
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "",
						},
					},
					{
						Denom: "factory/inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt/bitcoin",
						AuthorityMetadata: types.DenomAuthorityMetadata{
							Admin: "",
						},
					},
				},
			},
			valid: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
