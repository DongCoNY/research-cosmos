package cli_test

import (
	"fmt"
	"testing"
	"time"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	dbm "github.com/cometbft/cometbft-db"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	pruningtypes "github.com/cosmos/cosmos-sdk/store/pruning/types"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govcli "github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	govtestutil "github.com/cosmos/cosmos-sdk/x/gov/client/testutil"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/crypto/hd"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/client/cli"
	exchangetestutil "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/client/testutil"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancetestutil "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/client/testutil"
	oracletestutil "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/client/testutil"
)

var (
	// spot market params
	spotTicker     = "stake/node0token"
	spotBaseDenom  = "stake"
	spotQuoteDenom = "node0token"
	// perpetual market params
	perpetualTicker        = "STAKE"
	perpetualQuoteDenom    = "stake"
	perpetualOracleBase    = "oraclestake"
	perpetualOracleQuote   = "oracleusdt"
	perpetualOracleType    = oracletypes.OracleType_PriceFeed
	perpetualOracleTypeStr = oracletypes.OracleType_name[int32(perpetualOracleType)]
	// expiry futures market params
	futuresTicker        = "stake expiry futures market"
	futuresQuoteDenom    = "stake"
	futuresOracleBase    = "oracleBase"
	futuresOracleQuote   = "oracleQuote"
	futuresExpiry        = 1718568853
	futuresExpiryStr     = "1718568853"
	futuresOracleType    = oracletypes.OracleType_PriceFeed
	futuresOracleTypeStr = oracletypes.OracleType_name[int32(futuresOracleType)]
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
	govGenesis.Params.Quorum = "0"
	govGenesis.Params.Threshold = "0.1"
	govGenesisRaw := encCfg.Marshaler.MustMarshalJSON(govGenesis)
	genesisState["gov"] = govGenesisRaw
	xchgGenesis := exchangetypes.DefaultGenesisState()
	xchgGenesis.Params.DerivativeMarketInstantListingFee.Amount = sdk.NewInt(10000000)
	xchgGenesis.Params.DerivativeMarketInstantListingFee.Denom = sdk.DefaultBondDenom
	xchgGenesis.Params.SpotMarketInstantListingFee.Amount = sdk.NewInt(10000000)
	xchgGenesis.Params.SpotMarketInstantListingFee.Denom = sdk.DefaultBondDenom
	xchgGenesis.Params.IsInstantDerivativeMarketLaunchEnabled = true
	xchgGenesisRaw := encCfg.Marshaler.MustMarshalJSON(xchgGenesis)
	genesisState["exchange"] = xchgGenesisRaw

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
		NumValidators:     3,
		BondDenom:         sdk.DefaultBondDenom,
		MinGasPrices:      fmt.Sprintf("0.0000006%s", sdk.DefaultBondDenom),
		AccountTokens:     sdk.TokensFromConsensusPower(1000000000000, sdk.DefaultPowerReduction),
		StakingTokens:     sdk.TokensFromConsensusPower(5000000000000, sdk.DefaultPowerReduction),
		BondedTokens:      sdk.TokensFromConsensusPower(1000000000000, sdk.DefaultPowerReduction),
		PruningStrategy:   pruningtypes.PruningOptionNothing,
		CleanupDir:        true,
		SigningAlgo:       string(hd.Secp256k1Type),
		KeyringOptions:    []keyring.Option{},
	}

	var err error
	s.network, err = network.New(s.T(), "testrun", s.cfg)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())

	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	// 1a. Grant PriceFeeder privileges to a user with GrantPriceFeederPrivilegeProposal for a given marketID
	_, err = oracletestutil.GrantPriceFeederPrivilege(s.network, clientCtx, perpetualOracleBase, perpetualOracleQuote, val.Address.String(), val.Address)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())

	// 1b. Grant PriceFeeder Oracle privileges to a user with GrantPriceFeederPrivilegeProposal for a given marketID
	_, err = oracletestutil.GrantPriceFeederPrivilege(s.network, clientCtx, futuresOracleBase, futuresOracleQuote, val.Address.String(), val.Address)
	s.Require().NoError(err)

	// wait for proposal time
	time.Sleep(10 * time.Second)
	s.Require().NoError(s.network.WaitForNextBlock())

	// 2. Call SetPriceFeederPrice for the market with a price
	_, err = oracletestutil.MsgRelayPriceFeedPrice(s.network, clientCtx, perpetualOracleBase, perpetualOracleQuote, "25.00", val.Address)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())

	// 2. Call SetPriceFeederPrice for the market with a price
	_, err = oracletestutil.MsgRelayPriceFeedPrice(s.network, clientCtx, futuresOracleBase, futuresOracleQuote, "25.00", val.Address)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())

	_, err = exchangetestutil.MsgInstantSpotMarketLaunch(s.network, clientCtx, spotTicker, spotBaseDenom, spotQuoteDenom, val.Address)
	s.Require().NoError(err)

	// create insurance fund for perpetual market launch
	_, err = insurancetestutil.MsgCreateInsuranceFund(s.network,
		clientCtx,
		perpetualTicker,
		perpetualQuoteDenom,
		perpetualOracleBase, perpetualOracleQuote, perpetualOracleTypeStr,
		"-1",
		"10000000stake", val.Address)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())

	// create insurance fund for expiry future market launch
	_, err = insurancetestutil.MsgCreateInsuranceFund(s.network,
		clientCtx,
		futuresTicker,
		futuresQuoteDenom,
		futuresOracleBase, futuresOracleQuote, futuresOracleTypeStr,
		futuresExpiryStr,
		"10000000stake", val.Address)
	s.Require().NoError(err)

	_, err = exchangetestutil.MsgInstantPerpetualMarketLaunch(s.network, clientCtx, perpetualTicker, perpetualQuoteDenom, perpetualOracleBase, perpetualOracleQuote, perpetualOracleTypeStr, val.Address)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestSpotMarketParamsUpdateProposalTxCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.NewSpotMarketUpdateParamsProposalTxCmd()
	clientCtx := val.ClientCtx

	// spot market ID

	marketID := exchangetypes.NewSpotMarketID(spotBaseDenom, spotQuoteDenom)

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagMarketID, marketID.String()),
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagRelayerFeeShareRate, "0.4"),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagMarketStatus, "Active"),
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "spot market params update proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "Where is the title!?"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
		fmt.Sprintf("--%s=%s", flags.FlagGas, flags.GasFlagAuto),
		fmt.Sprintf("--%s=%s", flags.FlagGasAdjustment, "1.5"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, txResp.String())
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestSpotMarketLaunchProposalTxCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.NewSpotMarketLaunchProposalTxCmd()
	clientCtx := val.ClientCtx

	args := []string{
		"node0token/stake",
		"node0token",
		"stake",
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "node0token/stake spot market launch proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "Where is the title!?"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
		fmt.Sprintf("--%s=%s", flags.FlagGas, flags.GasFlagAuto),
		fmt.Sprintf("--%s=%s", flags.FlagGasAdjustment, "1.5"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, txResp.String())
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestInstantSpotMarketLaunchTxCmd_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.NewInstantSpotMarketLaunchTxCmd()
	clientCtx := val.ClientCtx

	args := []string{
		"stake/node1token",
		"stake",
		"node1token",
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err, res.String())
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestInstantPerpetualMarketLaunchTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	// create insurance fund for perpetual market launch
	_, err := insurancetestutil.MsgCreateInsuranceFund(s.network,
		clientCtx,
		"instant perpetual market for stake",
		perpetualQuoteDenom,
		"oracleBase", "oracleQuote", "pricefeed",
		"-1",
		"10000000stake", val.Address)
	s.Require().NoError(err)

	cmd := cli.NewInstantPerpetualMarketLaunchTxCmd()

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagTicker, "instant perpetual market for stake"),
		fmt.Sprintf("--%s=%s", cli.FlagQuoteDenom, perpetualQuoteDenom),
		fmt.Sprintf("--%s=%s", cli.FlagOracleBase, "oracleBase"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleQuote, "oracleQuote"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleType, "pricefeed"),
		fmt.Sprintf("--%s=%d", cli.FlagOracleScaleFactor, 0),
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagInitialMarginRatio, "0.05"),
		fmt.Sprintf("--%s=%s", cli.FlagMaintenanceMarginRatio, "0.02"),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.0001"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.001"),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err, res.String())
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestInstantExpiryFuturesMarketLaunchTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	cmd := cli.NewInstantExpiryFuturesMarketLaunchTxCmd()

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagTicker, futuresTicker),
		fmt.Sprintf("--%s=%s", cli.FlagQuoteDenom, futuresQuoteDenom),
		fmt.Sprintf("--%s=%s", cli.FlagOracleBase, futuresOracleBase),
		fmt.Sprintf("--%s=%s", cli.FlagOracleQuote, futuresOracleQuote),
		fmt.Sprintf("--%s=%s", cli.FlagOracleType, futuresOracleType),
		fmt.Sprintf("--%s=%d", cli.FlagOracleScaleFactor, 0),
		fmt.Sprintf("--%s=%d", cli.FlagExpiry, futuresExpiry), // time.Now().Unix()+86400
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagInitialMarginRatio, "0.05"),
		fmt.Sprintf("--%s=%s", cli.FlagMaintenanceMarginRatio, "0.02"),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err, res.String())
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestPerpetualMarketLaunchProposalTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	// create insurance fund for perpetual market launch
	_, err := insurancetestutil.MsgCreateInsuranceFund(s.network,
		clientCtx,
		"STAKE/NODE0TOKEN-PERPETUAL",
		"node0token",
		perpetualOracleBase, perpetualOracleQuote, "pricefeed",
		"-1",
		"10000000node0token", val.Address)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())

	cmd := cli.NewPerpetualMarketLaunchProposalTxCmd()

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagTicker, "STAKE/NODE0TOKEN-PERPETUAL"),
		fmt.Sprintf("--%s=%s", cli.FlagQuoteDenom, "node0token"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleBase, perpetualOracleBase),
		fmt.Sprintf("--%s=%s", cli.FlagOracleQuote, perpetualOracleQuote),
		fmt.Sprintf("--%s=%s", cli.FlagOracleType, "pricefeed"),
		fmt.Sprintf("--%s=%d", cli.FlagOracleScaleFactor, 1),
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagInitialMarginRatio, "0.05"),
		fmt.Sprintf("--%s=%s", cli.FlagMaintenanceMarginRatio, "0.02"),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.0001"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.001"),
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "STAKE/NODE0TOKEN-0011 perpetual market launch proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "Where is the title!?"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagGas, flags.GasFlagAuto),
		fmt.Sprintf("--%s=%s", flags.FlagGasAdjustment, "1.5"),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, string(res.Bytes()))
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestExpiryFuturesMarketLaunchProposalTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	// create insurance fund for expiry futures market launch
	_, err := insurancetestutil.MsgCreateInsuranceFund(s.network,
		clientCtx,
		"STAKE/NODE0TOKEN-EXPIRY1718568672",
		"node0token",
		perpetualOracleBase, perpetualOracleQuote, "pricefeed",
		"1718568672",
		"10000000node0token", val.Address)
	s.Require().NoError(err)

	cmd := cli.NewExpiryFuturesMarketLaunchProposalTxCmd()

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagTicker, "STAKE/NODE0TOKEN-EXPIRY1718568672"),
		fmt.Sprintf("--%s=%s", cli.FlagQuoteDenom, "node0token"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleBase, perpetualOracleBase),
		fmt.Sprintf("--%s=%s", cli.FlagOracleQuote, perpetualOracleQuote),
		fmt.Sprintf("--%s=%s", cli.FlagOracleType, "pricefeed"),
		fmt.Sprintf("--%s=%d", cli.FlagOracleScaleFactor, 0),
		fmt.Sprintf("--%s=%d", cli.FlagExpiry, 1718568672), // time.Now().Unix()+86400
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagInitialMarginRatio, "0.05"),
		fmt.Sprintf("--%s=%s", cli.FlagMaintenanceMarginRatio, "0.02"),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "expiry futures market launch proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "Where is the title!?"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagGas, flags.GasFlagAuto),
		fmt.Sprintf("--%s=%s", flags.FlagGasAdjustment, "1.5"),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, string(res.Bytes()))
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestDerivativeMarketParamUpdateProposalTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	cmd := cli.NewDerivativeMarketParamUpdateProposalTxCmd()

	// perpetual market ID
	marketID := exchangetypes.NewPerpetualMarketID(perpetualTicker, perpetualQuoteDenom, perpetualOracleBase, perpetualOracleQuote, perpetualOracleType)

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagMarketID, marketID.String()),
		fmt.Sprintf("--%s=%s", cli.FlagInitialMarginRatio, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMaintenanceMarginRatio, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagRelayerFeeShareRate, "0.01"),
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "derivative market param update proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "Where is the title!?"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", cli.FlagMinPriceTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMinQuantityTickSize, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagHourlyInterestRate, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagHourlyFundingRateCap, "0.01"),
		fmt.Sprintf("--%s=%s", cli.FlagMakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagTakerFeeRate, "0.001"),
		fmt.Sprintf("--%s=%s", cli.FlagOracleBase, perpetualOracleBase),
		fmt.Sprintf("--%s=%s", cli.FlagOracleQuote, perpetualOracleQuote),
		fmt.Sprintf("--%s=%s", cli.FlagOracleType, perpetualOracleType),
		fmt.Sprintf("--%s=%d", cli.FlagOracleScaleFactor, 0),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
		fmt.Sprintf("--%s=%s", flags.FlagGas, flags.GasFlagAuto),
		fmt.Sprintf("--%s=%s", flags.FlagGasAdjustment, "1.5"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	txResp := sdk.TxResponse{}
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &txResp), res.String())
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, txResp.String())
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestMarketForcedSettlementTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	cmd := cli.NewMarketForcedSettlementTxCmd()

	// perpetual market ID
	marketID := exchangetypes.NewPerpetualMarketID(perpetualTicker, perpetualQuoteDenom, perpetualOracleBase, perpetualOracleQuote, perpetualOracleType)

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagMarketID, marketID.String()),
		fmt.Sprintf("--%s=%s", cli.FlagSettlementPrice, "100000"),
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "market forced settlement proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "Where is the title!?"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
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
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, string(res.Bytes()))
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestUpdateDenomDecimalsTxCmd_HappyPath() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	cmd := cli.NewUpdateDenomDecimalsProposalTxCmd()

	args := []string{
		fmt.Sprintf("--%s=%s", cli.FlagDenoms, "inj,usdt"),
		fmt.Sprintf("--%s=%s", cli.FlagDecimals, "18,6"),
		fmt.Sprintf("--%s=%s", govcli.FlagTitle, "update denom decimals proposal"),
		fmt.Sprintf("--%s=%s", govcli.FlagDescription, "18 decimals for inj and 6 decimals for usdt"),
		fmt.Sprintf("--%s=%s", govcli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, govtypes.DefaultMinDepositTokens).String()),
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
	txResp, err = clitestutil.GetTxResponse(s.network, clientCtx, txResp.TxHash)
	s.Require().NoError(err)
	s.Require().True(len(txResp.Logs) > 0, string(res.Bytes()))
	s.Require().Equal(txResp.Logs[0].Events[1].Attributes[0].Key, "proposal_id")
	proposalID := txResp.Logs[0].Events[1].Attributes[0].Value

	_, err = govtestutil.MsgVote(val.ClientCtx, val.Address.String(), proposalID, "yes")
	s.Require().NoError(err)
	s.Require().NoError(s.network.WaitForNextBlock())
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TestGetAllSpotMarkets_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetAllSpotMarkets()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := exchangetypes.QuerySpotMarketsResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())

	s.Require().Greater(len(resp.Markets), 0, res.String())
}

func (s *IntegrationTestSuite) TestGetSpotMarket_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetSpotMarket()
	clientCtx := val.ClientCtx

	// spot market ID
	marketID := exchangetypes.NewSpotMarketID(spotBaseDenom, spotQuoteDenom)

	args := []string{
		marketID.String(),
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := exchangetypes.QuerySpotMarketResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())
	s.Require().NotNil(resp.Market, res.String())
	s.Require().Equal(resp.Market.Ticker, spotTicker, res.String())
	s.Require().Equal(resp.Market.BaseDenom, spotBaseDenom, res.String())
	s.Require().Equal(resp.Market.QuoteDenom, spotQuoteDenom, res.String())
}

func (s *IntegrationTestSuite) TestGetAllDerivativeMarkets_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetAllDerivativeMarkets()
	clientCtx := val.ClientCtx

	args := []string{
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := exchangetypes.QueryDerivativeMarketsResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())

	s.Require().Greater(len(resp.Markets), 0, res.String())
}

func (s *IntegrationTestSuite) TestGetDerivativeMarket_HappyPath() {
	val := s.network.Validators[0]

	cmd := cli.GetDerivativeMarket()
	clientCtx := val.ClientCtx

	// perpetual market ID
	marketID := exchangetypes.NewPerpetualMarketID(perpetualTicker, perpetualQuoteDenom, perpetualOracleBase, perpetualOracleQuote, perpetualOracleType)

	args := []string{
		marketID.String(),
		fmt.Sprintf("--%s=%s", tmcli.OutputFlag, "json"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, "injective-1"),
	}

	res, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := exchangetypes.QueryDerivativeMarketResponse{}
	s.Require().NoError(err, res.String())
	s.Require().NoError(clientCtx.Codec.UnmarshalJSON(res.Bytes(), &resp), res.String())
	s.Require().NotNil(resp.Market, res.String())
	s.Require().Equal(resp.Market.Market.Ticker, perpetualTicker, res.String())
	s.Require().Equal(resp.Market.Market.QuoteDenom, perpetualQuoteDenom, res.String())
	s.Require().Equal(resp.Market.Market.OracleQuote, perpetualOracleQuote, res.String())
	s.Require().Equal(resp.Market.Market.OracleBase, perpetualOracleBase, res.String())
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
