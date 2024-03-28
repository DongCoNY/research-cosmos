package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	expectedMinGasPrice := sdk.NewDecCoins(sdk.NewDecCoin("inj", sdk.NewInt(500000000)))
	require.True(t, cfg.GetMinGasPrices().IsEqual(expectedMinGasPrice))
}

func TestSetMinimumFees(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetMinGasPrices(sdk.DecCoins{sdk.NewInt64DecCoin("foo", 5)})
	require.Equal(t, "5.000000000000000000foo", cfg.MinGasPrices)
}
