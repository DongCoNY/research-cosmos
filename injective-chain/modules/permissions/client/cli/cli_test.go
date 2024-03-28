package cli_test

import (
	"fmt"
	"testing"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	pruningtypes "github.com/cosmos/cosmos-sdk/store/pruning/types"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/suite"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/client/cli"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
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

func (s *IntegrationTestSuite) Test_GetParams() {
	var (
		validator = s.network.Validators[0]
		clientCtx = validator.ClientCtx
		cmd       = cli.GetParams()
	)

	args := []string{
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryParamsResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp))
}

func (s *IntegrationTestSuite) Test_GetNamespaces() {
	var (
		validator = s.network.Validators[0]
		clientCtx = validator.ClientCtx
		cmd       = cli.GetNamespaces()
	)

	args := []string{
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryAllNamespacesResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp))
}

func (s *IntegrationTestSuite) Test_GetNamespaceByDenom() {
	var (
		validator = s.network.Validators[0]
		clientCtx = validator.ClientCtx
		cmd       = cli.GetNamespaceByDenom()
	)

	args := []string{
		"denom",
		"true",
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryNamespaceByDenomResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp))
}

func (s *IntegrationTestSuite) Test_GetNamespaceRoleAddresses() {
	var (
		validator = s.network.Validators[0]
		clientCtx = validator.ClientCtx
		cmd       = cli.GetNamespaceRoleAddresses()
	)

	args := []string{
		"denom",
		"role",
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	_, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().Error(err)
}

func (s *IntegrationTestSuite) Test_GetVouchersForAddress() {
	var (
		validator = s.network.Validators[0]
		clientCtx = validator.ClientCtx
		cmd       = cli.GetVouchersForAddress()
	)

	args := []string{
		validator.Address.String(),
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := types.QueryVouchersForAddressResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp))
}
