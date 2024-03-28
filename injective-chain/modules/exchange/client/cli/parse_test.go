package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	govcli "github.com/cosmos/cosmos-sdk/x/gov/client/cli"
)

func TestParseBatchExchangeModificationsProposalFlags(t *testing.T) {
	pflagSet := pflag.NewFlagSet("", pflag.ContinueOnError)
	pflagSet.String(govcli.FlagProposal, "proposals/batchproposal.json", "")

	result, err := parseBatchExchangeModificationsProposalFlags(pflagSet)
	require.NoError(t, err)
	require.Equal(t, "Trade Reward Campaign", result.TradingRewardCampaignUpdateProposal.Title)

	pflagSet = pflag.NewFlagSet("", pflag.ContinueOnError)
	pflagSet.String(govcli.FlagProposal, "proposals/batchproposal_wrong_key.json", "")

	_, err = parseBatchExchangeModificationsProposalFlags(pflagSet)
	require.ErrorContains(t, err, "json: unknown field \"relayer_fee_share_rate\"")
}
