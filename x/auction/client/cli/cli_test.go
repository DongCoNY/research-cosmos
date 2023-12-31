package cli_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"

	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/client/testutil"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
	tmcli "github.com/tendermint/tendermint/libs/cli"

	bankcli "github.com/cosmos/cosmos-sdk/x/bank/client/cli"

	gcli "github.com/DongCoNY/research-cosmos/testutil/cli"
	gnetwork "github.com/DongCoNY/research-cosmos/testutil/network"

	cmdcfg "github.com/DongCoNY/research-cosmos/cmd/dongtramcamd/config"
	"github.com/DongCoNY/research-cosmos/x/auction/client/cli"

	auctiontypes "github.com/DongCoNY/research-cosmos/x/auction/types"

	"github.com/DongCoNY/research-cosmos/app"
)

var addr1 sdk.AccAddress
var addr2 sdk.AccAddress
var distributorMnemonics []string
var distributorAddrs []string

func init() {
	cmdcfg.SetupConfig()
	addr1 = ed25519.GenPrivKey().PubKey().Address().Bytes()
	addr2 = ed25519.GenPrivKey().PubKey().Address().Bytes()
	distributorMnemonics = []string{
		"chronic learn inflict great answer reward evidence stool open moon skate resource arch raccoon decade tell improve stay onion section blouse carry primary fabric",
		"catalog govern other escape eye resemble dirt hundred birth build dirt jacket network blame credit palace similar carry knock auction exotic bus business machine",
	}
	distributorAddrs = []string{
		"dongtramcamrf2nmxsg0u728ga7665fmlfguqxcd8e36vf",
		"dongtramcamk3q70ranm3han4lvutvcvetncxg82zfh6me",
	}
}

type IntegrationTestSuite struct {
	suite.Suite

	cfg     gnetwork.Config
	network *gnetwork.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	s.cfg = gnetwork.DefaultConfig()

	auctionGenState := auctiontypes.DefaultGenesis()

	// Set param
	allowTokens := map[string]bool{
		"atom": true,
	}
	params := auctiontypes.NewParams(uint64(4), uint64(2), uint64(1000), uint64(100), sdk.NewDecWithPrec(2, 1), allowTokens)

	genState := app.ModuleBasics.DefaultGenesis(s.cfg.Codec)
	auctionGenState.Params = params
	auctionGenStateBz := s.cfg.Codec.MustMarshalJSON(auctionGenState)
	genState[auctiontypes.ModuleName] = auctionGenStateBz

	s.cfg.GenesisState = genState
	s.network = gnetwork.New(s.T(), s.cfg)

	a, _ := s.network.LatestHeight()
	println("ssss:", a)
	startTime := time.Now()

	// err := s.network.WaitForNextBlock()
	// s.Require().NoError(err)

	time.Sleep(5 * time.Second)
	time.Sleep(5 * time.Second)
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	fmt.Printf("Thời gian đã trôi qua: %s\n", elapsedTime)

	a, _ = s.network.LatestHeight()
	println("ssss22:", a)

	// err := s.network.WaitForNextBlock()
	// s.Require().NoError(err)
	// cofig := s.network.Config.AppConstructor
	val := s.network.Validators[0]

	// appe := cofig(*val)

	// println(appe.Commit())

	a, _ = s.network.LatestHeight()
	println("kkkkkkk:", a)

	// Initiate distributor accounts

	for idx := range distributorMnemonics {
		info, _ := val.ClientCtx.Keyring.NewAccount("distributor"+strconv.Itoa(idx), distributorMnemonics[idx], keyring.DefaultBIP39Passphrase, sdk.FullFundraiserPath, hd.Secp256k1)
		distributorAddr := sdk.AccAddress(info.PubKey.Value)
		_, err := banktestutil.MsgSendExec(
			val.ClientCtx,
			val.Address,
			distributorAddr,
			sdk.NewCoins(sdk.NewInt64Coin(s.cfg.BondDenom, 1020)), fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
			fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
			gcli.DefaultFeeString(s.cfg),
		)
		s.Require().NoError(err)
	}
	fmt.Printf("okoko\n")
}

func (s *IntegrationTestSuite) TearDownSuite() {
	// s.T().Log("tearing down integration test suite")
	// s.network.Cleanup()
}
func (s *IntegrationTestSuite) TestGetCmdQueryParams() {
	val := s.network.Validators[0]

	cmd_blance := bankcli.GetBalancesCmd()
	args := []string{distributorAddrs[1]}
	clientCtxx := val.ClientCtx
	out, err := clitestutil.ExecTestCLICmd(clientCtxx, cmd_blance, args)
	s.Require().NoError(err)
	fmt.Printf("kq:%v \n", out)
	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"json output",
			[]string{fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			`{"auction_epoch":"100","auction_period":"1209600s","min_bid_amount":"10000","bid_gap":"50","auction_rate":"0.020000000000000000","allow_tokens":""}`,
		},
		{
			"text output",
			[]string{fmt.Sprintf("--%s=1", flags.FlagHeight), fmt.Sprintf("--%s=text", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryParams()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestGetCmdHighestBid() {
	val := s.network.Validators[0]

	cmd_blance := bankcli.GetBalancesCmd()
	args := []string{distributorAddrs[1]}
	clientCtxx := val.ClientCtx
	out, err := clitestutil.ExecTestCLICmd(clientCtxx, cmd_blance, args)
	s.Require().NoError(err)
	fmt.Printf("kq:%v \n", out)
	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"json output",
			[]string{fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			`{"auction_epoch":"100","auction_period":"1209600s","min_bid_amount":"10000","bid_gap":"50","auction_rate":"0.020000000000000000","allow_tokens":""}`,
		},
		{
			"text output",
			[]string{fmt.Sprintf("--%s=1", flags.FlagHeight), fmt.Sprintf("--%s=text", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryParams()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}
func (s *IntegrationTestSuite) TestGetCmdAuctionPeriods() {
	val := s.network.Validators[0]
	id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(3, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{id, fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdAuctionPeriods()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestGetCmdAuction() {
	s.SetupSuite()
	val := s.network.Validators[0]
	auction_id := "1"
	period_id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(30, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{auction_id, period_id, fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdAuction()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}
func (s *IntegrationTestSuite) TestGetCmdAllAuction() {
	s.SetupSuite()
	val := s.network.Validators[0]
	address := s.network.Validators[0].Address
	period_id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(30, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{address.String(), period_id, fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdAllAuction()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestTxCmdMsgBid() {
	s.SetupSuite()
	val := s.network.Validators[0]

	auction_id := "1"
	period_id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(30, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{
				auction_id, period_id,
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, distributorAddrs[1]),
			},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.CmdMsgBid()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}
func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
