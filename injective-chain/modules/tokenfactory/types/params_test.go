package types_test

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

func TestDefaultParams(t *testing.T) {
	t.Run("it returns default params", func(t *testing.T) {
		expectedCreationFee := sdk.NewCoins(sdk.NewCoin(chaintypes.InjectiveCoin, sdkmath.NewIntWithDecimal(10, 18)))
		defaultParams := types.DefaultParams()

		require.Equal(t, expectedCreationFee, defaultParams.DenomCreationFee, "incorrect default creation fee")
		err := defaultParams.Validate()
		require.NoError(t, err, "default params validation returned error")

		expectedParamSet := paramtypes.ParamSetPairs{
			paramtypes.NewParamSetPair(types.KeyDenomCreationFee, &defaultParams.DenomCreationFee, nil),
		}
		paramSet := defaultParams.ParamSetPairs()

		require.Equal(t, expectedParamSet[0].Key, paramSet[0].Key, "incorrect paramSet key returned")
		require.Equal(t, expectedParamSet[0].Value, paramSet[0].Value, "incorrect paramSet value returned")
		require.NotNil(t, paramSet[0].ValidatorFn, "nil validator function found")
	})
}

func TestConstructValidNewParams(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		amount sdkmath.Int
	}{
		{
			desc:   "normal value",
			amount: sdk.NewInt(2),
		},
		{
			desc:   "zero value",
			amount: sdk.NewInt(0),
		},
	} {
		t.Run(fmt.Sprintf("case %v", tc.desc), func(t *testing.T) {
			expectedCreationFee := sdk.NewCoins(sdk.NewCoin(chaintypes.InjectiveCoin, tc.amount))
			newParams := types.NewParams(expectedCreationFee)

			require.Equal(t, expectedCreationFee, newParams.DenomCreationFee, "incorrect creation fee")
			err := newParams.Validate()
			require.NoError(t, err, "new params validation returned error")

			expectedParamSet := paramtypes.ParamSetPairs{
				paramtypes.NewParamSetPair(types.KeyDenomCreationFee, &newParams.DenomCreationFee, nil),
			}
			paramSet := newParams.ParamSetPairs()

			require.Equal(t, expectedParamSet[0].Key, paramSet[0].Key, "incorrect paramSet key returned")
			require.Equal(t, expectedParamSet[0].Value, paramSet[0].Value, "incorrect paramSet value returned")
			require.NotNil(t, paramSet[0].ValidatorFn, "nil validator function found")
		})
	}
}
