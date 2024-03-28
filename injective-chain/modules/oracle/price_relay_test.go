package oracle_test

import (
	"encoding/json"
	"testing"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	testifysuite "github.com/stretchr/testify/suite"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"

	injectiveapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	bandapp "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/bandtesting/app"
	bandoracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/bandtesting/x/oracle/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

type PriceRelayTestSuite struct {
	testifysuite.Suite

	coordinator *ibctesting.Coordinator

	// testing chains used for convenience and readability
	chainI *ibctesting.TestChain
	chainB *ibctesting.TestChain
}

func (suite *PriceRelayTestSuite) SetupTest() {
	suite.coordinator = ibctesting.NewCoordinator(suite.T(), 0)

	// setup injective chain
	chainID := ibctesting.GetChainID(0)
	ibctesting.DefaultTestingAppInit = func() (ibctesting.TestingApp, map[string]json.RawMessage) {
		db := dbm.NewMemDB()
		encCdc := injectiveapp.MakeEncodingConfig()
		app := injectiveapp.NewInjectiveApp(log.NewNopLogger(), db, nil, true, map[int64]bool{}, injectiveapp.DefaultNodeHome, 5, encCdc, simtestutil.EmptyAppOptions{})
		genesisState := injectiveapp.NewDefaultGenesisState()
		oracleGenesis := oracletypes.DefaultGenesisState()
		oracleGenesis.BandIbcParams = *oracletypes.DefaultTestBandIbcParams()
		oracleGenesisRaw := encCdc.Marshaler.MustMarshalJSON(oracleGenesis)
		genesisState[oracletypes.ModuleName] = oracleGenesisRaw
		return app, genesisState
	}
	suite.coordinator.Chains[chainID] = ibctesting.NewTestChain(suite.T(), suite.coordinator, chainID)

	// setup band chain
	chainID = ibctesting.GetChainID(1)
	ibctesting.DefaultTestingAppInit = func() (ibctesting.TestingApp, map[string]json.RawMessage) {
		db := dbm.NewMemDB()
		encCdc := bandapp.MakeEncodingConfig()
		app := bandapp.NewBandApp(log.NewNopLogger(), db, nil, true, map[int64]bool{}, bandapp.DefaultNodeHome, 5, encCdc, simtestutil.EmptyAppOptions{})
		return app, bandapp.NewDefaultGenesisState()
	}
	suite.coordinator.Chains[chainID] = ibctesting.NewTestChain(suite.T(), suite.coordinator, chainID)

	suite.chainI = suite.coordinator.GetChain(ibctesting.GetChainID(0))
	suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(1))
}

func NewPriceRelayPath(chainI, chainB *ibctesting.TestChain) *ibctesting.Path {
	path := ibctesting.NewPath(chainI, chainB)
	path.EndpointA.ChannelConfig.Version = oracletypes.DefaultTestBandIbcParams().IbcVersion
	path.EndpointA.ChannelConfig.PortID = oracletypes.DefaultTestBandIbcParams().IbcPortId
	path.EndpointB.ChannelConfig.Version = oracletypes.DefaultTestBandIbcParams().IbcVersion
	path.EndpointB.ChannelConfig.PortID = oracletypes.ModuleName

	return path
}

// constructs a send from chainA to chainB on the established channel/connection
// and sends the same coin back from chainB to chainA.
func (suite *PriceRelayTestSuite) TestHandlePriceRelay() {
	// setup between chainA and chainB
	path := NewPriceRelayPath(suite.chainI, suite.chainB)
	suite.coordinator.Setup(path)

	timeoutHeight := clienttypes.NewHeight(1, 110)

	// relay send
	bandOracleReq := oracletypes.BandOracleRequest{
		OracleScriptId: 1,
		Symbols:        []string{"inj", "btc"},
		AskCount:       1,
		MinCount:       1,
		FeeLimit:       sdk.Coins{sdk.NewInt64Coin("inj", 1)},
		PrepareGas:     100,
		ExecuteGas:     200,
	}

	priceRelayPacket := oracletypes.NewOracleRequestPacketData("11", bandOracleReq.GetCalldata(true), &bandOracleReq)
	packet := channeltypes.NewPacket(priceRelayPacket.GetBytes(), 1, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, timeoutHeight, 0)
	_, err := path.EndpointA.SendPacket(packet.TimeoutHeight, packet.TimeoutTimestamp, packet.Data)
	suite.Require().NoError(err)

	// nolint:all
	// ack := channeltypes.NewResultAcknowledgement(types.ModuleCdc.MustMarshalJSON(bandoracletypes.NewOracleRequestPacketAcknowledgement(1)))
	err = path.RelayPacket(packet)
	suite.Require().NoError(err) // relay committed

	suite.chainB.NextBlock()

	oracleResponsePacket := bandoracletypes.NewOracleResponsePacketData("11", 1, 0, 1577923380, 1577923405, 1, []byte("beeb"))
	responsePacket := channeltypes.NewPacket(
		oracleResponsePacket.GetBytes(),
		1,
		path.EndpointB.ChannelConfig.PortID,
		path.EndpointB.ChannelID,
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		clienttypes.ZeroHeight(),
		1577924005000000000,
	)

	expectCommitment := channeltypes.CommitPacket(suite.chainB.Codec, responsePacket)
	commitment := suite.chainB.App.GetIBCKeeper().ChannelKeeper.GetPacketCommitment(suite.chainB.GetContext(), path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, 1)
	suite.Equal(expectCommitment, commitment)

	injectiveApp := suite.chainI.App.(*injectiveapp.InjectiveApp)
	injectiveApp.OracleKeeper.SetBandIBCOracleRequest(suite.chainI.GetContext(), oracletypes.BandOracleRequest{
		RequestId:      1,
		OracleScriptId: 1,
		Symbols:        []string{"A"},
		AskCount:       1,
		MinCount:       1,
		FeeLimit:       sdk.Coins{},
		PrepareGas:     100,
		ExecuteGas:     200,
	})

	// send from chainI to chainB
	msg := oracletypes.NewMsgRequestBandIBCRates(suite.chainI.SenderAccount.GetAddress(), 1)

	_, err = suite.chainI.SendMsgs(msg)
	suite.Require().NoError(err) // message committed
}

func TestPriceRelayTestSuite(t *testing.T) {
	testifysuite.Run(t, new(PriceRelayTestSuite))
}
