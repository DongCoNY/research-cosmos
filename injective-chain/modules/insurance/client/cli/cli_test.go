package cli_test

import (
	"fmt"
	"testing"
	"time"

	pruningtypes "github.com/cosmos/cosmos-sdk/store/pruning/types"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	dbm "github.com/cometbft/cometbft-db"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/suite"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/client/cli"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

// NewAppConstructor returns a new AppConstructor
func NewAppConstructor(encodingCfg app.EncodingConfig) network.AppConstructor {
	return func(val network.ValidatorI) servertypes.Application {
		return app.NewInjectiveApp(
			val.GetCtx().Logger,
			dbm.NewMemDB(),
			nil,
			true,
			make(map[int64]bool),
			val.GetCtx().Config.RootDir,
			0,
			encodingCfg,
			simtestutil.EmptyAppOptions{},
			baseapp.SetMinGasPrices(val.GetAppConfig().MinGasPrices),
			baseapp.SetChainID("injective-1"),
		)
	}
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	encCfg := app.MakeEncodingConfig()

	// customize voting period for testing
	genesisState := app.ModuleBasics.DefaultGenesis(encCfg.Marshaler)
	govGenesis := govtypes.DefaultGenesisState()
	votingPeriod := time.Second * 6
	govGenesis.Params.VotingPeriod = &votingPeriod
	govGenesisRaw := encCfg.Marshaler.MustMarshalJSON(govGenesis)
	genesisState["gov"] = govGenesisRaw

	s.cfg = network.Config{
		Codec:             encCfg.Marshaler,
		TxConfig:          encCfg.TxConfig,
		LegacyAmino:       encCfg.Amino,
		InterfaceRegistry: encCfg.InterfaceRegistry,
		AccountRetriever:  authtypes.AccountRetriever{},
		AppConstructor:    NewAppConstructor(encCfg),
		GenesisState:      genesisState,
		TimeoutCommit:     2 * time.Second,
		ChainID:           "injective-1",
		NumValidators:     1,
		BondDenom:         sdk.DefaultBondDenom,
		MinGasPrices:      fmt.Sprintf("0.000006%s", sdk.DefaultBondDenom),
		AccountTokens:     sdk.TokensFromConsensusPower(1000, sdk.DefaultPowerReduction),
		StakingTokens:     sdk.TokensFromConsensusPower(500, sdk.DefaultPowerReduction),
		BondedTokens:      sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction),
		PruningStrategy:   pruningtypes.PruningOptionNothing,
		CleanupDir:        true,
		SigningAlgo:       string(hd.Secp256k1Type),
		KeyringOptions:    []keyring.Option{},
	}

	n, err := network.New(s.T(), "testrun", s.cfg)
	s.Require().NoError(err)

	s.network = n

	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

// TODO: for CLI tests completion, we need oracle integrated here

func (s *IntegrationTestSuite) TestCreateInsuranceFundTxCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.NewCreateInsuranceFundTxCmd()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagTicker, "ticker"),
		fmt.Sprintf("--%s=%s", cli.FlagQuoteDenom, "inj"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleBase, "oracle-base"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleQuote, "oracle-quote"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleType, "pricefeed"),
		fmt.Sprintf("--%s=%d", cli.FlagExpiry, 1619181341),
		fmt.Sprintf("--%s=%s", cli.FlagInitialDeposit, "1000inj"),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
}

func (s *IntegrationTestSuite) TestUnderwriteInsuranceFundTxCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.NewUnderwriteInsuranceFundTxCmd()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagMarketId, "0x000001"),
		fmt.Sprintf("--%s=%s", cli.FlagDeposit, "1000inj"),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
}

func (s *IntegrationTestSuite) TestRequestRedemptionTxCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.NewRequestRedemptionTxCmd()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagMarketId, "0x000001"),
		fmt.Sprintf("--%s=%s", cli.FlagShareToken, "1000000share1"),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
}

func (s *IntegrationTestSuite) TestGetInsuranceParamsCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetInsuranceParamsCmd()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryInsuranceParamsResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())
}

func (s *IntegrationTestSuite) TestGetEstimatedRedemptionsCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetEstimatedRedemptionsCmd()
	clientCtx := val.ClientCtx

	args := []string{
		"0x0001", val.Address.String(),
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryEstimatedRedemptionsResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())
}

func (s *IntegrationTestSuite) TestGetPendingRedemptionsCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetEstimatedRedemptionsCmd()
	clientCtx := val.ClientCtx

	args := []string{
		"0x0001", val.Address.String(),
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryPendingRedemptionsResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())
}
