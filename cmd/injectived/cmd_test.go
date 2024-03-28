package main_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	injectived "github.com/InjectiveLabs/injective-core/cmd/injectived"
)

func TestInitCmd(t *testing.T) {
	rootCmd, _ := injectived.NewRootCmd()
	rootCmd.SetArgs([]string{
		"init",           // Test the init cmd
		"injective-test", // Moniker
		fmt.Sprintf("--%s=%s", cli.FlagOverwrite, "true"), // Overwrite genesis.json, in case it already exists
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	})

	err := injectived.Execute(rootCmd)
	require.NoError(t, err)
}
