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
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/client/cli"
	oracletestutil "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/client/testutil"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
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

	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	// grant price feeder privilege, params are (base, quote, relayers, from)
	_, err = oracletestutil.GrantPriceFeederPrivilege(s.network, clientCtx, "inj", "usdt", val.Address.String(), val.Address)
	s.Require().NoError(err)

	// TODO: vote with other 2 validators to pass the test
	// permission grant is not working as only one is voting - could modify vote quorum if it's easier

	// grant band oracle privilege
	// _, err = oracletestutil.GrantBandOraclePrivilege(clientCtx, val.Address.String(), val.Address)
	// s.Require().NoError(err)

	time.Sleep(6 * time.Second)
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	_, err = oracletestutil.MsgRelayPriceFeedPrice(s.network, clientCtx, "inj", "usdt", "25.00", val.Address)
	s.Require().NoError(err)

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

func (s *IntegrationTestSuite) TestGetPriceFeedsCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetPriceFeedsCmd()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	// check price feeds
	resp := types.QueryPriceFeedPriceStatesResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())
	s.Require().GreaterOrEqual(len(resp.PriceStates), 1, res.String())
}
