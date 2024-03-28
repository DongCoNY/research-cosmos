package oracle_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"

	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

func (suite *PriceRelayTestSuite) TestOnChanOpenInit() {
	var (
		channel *channeltypes.Channel
		path    *ibctesting.Path
		chanCap *capabilitytypes.Capability
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{

		{
			"success", func() {}, true,
		},
		{
			"invalid port ID", func() {
				path.EndpointA.ChannelConfig.PortID = ibctesting.MockPort
			}, false,
		},
		{
			"invalid version", func() {
				channel.Version = "version"
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			path = NewPriceRelayPath(suite.chainI, suite.chainB)
			suite.coordinator.SetupConnections(path)
			path.EndpointA.ChannelID = ibctesting.FirstChannelID

			counterparty := channeltypes.NewCounterparty(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			channel = &channeltypes.Channel{
				State:          channeltypes.INIT,
				Ordering:       channeltypes.UNORDERED,
				Counterparty:   counterparty,
				ConnectionHops: []string{path.EndpointA.ConnectionID},
				Version:        oracletypes.DefaultTestBandIbcParams().IbcVersion,
			}

			module, _, err := suite.chainI.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainI.GetContext(), oracletypes.DefaultTestBandIbcParams().IbcPortId)
			suite.Require().NoError(err)

			chanCap, err = suite.chainI.App.GetScopedIBCKeeper().NewCapability(suite.chainI.GetContext(), host.ChannelCapabilityPath(oracletypes.DefaultTestBandIbcParams().IbcPortId, path.EndpointA.ChannelID))
			suite.Require().NoError(err)

			cbs, ok := suite.chainI.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			tc.malleate() // explicitly change fields in channel and testChannel

			_, err = cbs.OnChanOpenInit(suite.chainI.GetContext(), channel.Ordering, channel.GetConnectionHops(),
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, chanCap, channel.Counterparty, channel.GetVersion(),
			)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

		})
	}
}

func (suite *PriceRelayTestSuite) TestOnChanOpenTry() {
	var (
		channel             *channeltypes.Channel
		chanCap             *capabilitytypes.Capability
		path                *ibctesting.Path
		counterpartyVersion string
	)

	testCases := []struct {
		name          string
		malleate      func()
		expPass       bool
		expAppVersion string
	}{

		{
			"success", func() {}, true, oracletypes.DefaultTestBandIbcParams().IbcVersion,
		},
		{
			"invalid port ID", func() {
				path.EndpointA.ChannelConfig.PortID = ibctesting.MockPort
			}, false, "",
		},
		{
			"invalid channel version", func() {
				channel.Version = "version"
			}, true, oracletypes.DefaultTestBandIbcParams().IbcVersion,
		},
		{
			"invalid counterparty version", func() {
				counterpartyVersion = "version"
			}, false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path = NewPriceRelayPath(suite.chainI, suite.chainB)
			suite.coordinator.SetupConnections(path)
			path.EndpointA.ChannelID = ibctesting.FirstChannelID

			counterparty := channeltypes.NewCounterparty(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			channel = &channeltypes.Channel{
				State:          channeltypes.TRYOPEN,
				Ordering:       channeltypes.UNORDERED,
				Counterparty:   counterparty,
				ConnectionHops: []string{path.EndpointA.ConnectionID},
				Version:        oracletypes.DefaultTestBandIbcParams().IbcVersion,
			}
			counterpartyVersion = oracletypes.DefaultTestBandIbcParams().IbcVersion

			module, _, err := suite.chainI.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainI.GetContext(), oracletypes.DefaultTestBandIbcParams().IbcPortId)
			suite.Require().NoError(err)

			chanCap, err = suite.chainI.App.GetScopedIBCKeeper().NewCapability(suite.chainI.GetContext(), host.ChannelCapabilityPath(oracletypes.DefaultTestBandIbcParams().IbcPortId, path.EndpointA.ChannelID))
			suite.Require().NoError(err)

			cbs, ok := suite.chainI.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			tc.malleate() // explicitly change fields in channel and testChannel

			appVersion, err := cbs.OnChanOpenTry(suite.chainI.GetContext(), channel.Ordering, channel.GetConnectionHops(),
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, chanCap, channel.Counterparty, counterpartyVersion,
			)

			suite.Assert().Equal(tc.expAppVersion, appVersion)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *PriceRelayTestSuite) TestOnChanOpenAck() {
	var counterpartyVersion string

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{

		{
			"success", func() {}, true,
		},
		{
			"invalid counterparty version", func() {
				counterpartyVersion = "version"
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path := NewPriceRelayPath(suite.chainI, suite.chainB)
			suite.coordinator.SetupConnections(path)
			path.EndpointA.ChannelID = ibctesting.FirstChannelID
			counterpartyVersion = oracletypes.DefaultTestBandIbcParams().IbcVersion

			module, _, err := suite.chainI.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainI.GetContext(), oracletypes.DefaultTestBandIbcParams().IbcPortId)
			suite.Require().NoError(err)

			cbs, ok := suite.chainI.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			tc.malleate() // explicitly change fields in channel and testChannel

			err = cbs.OnChanOpenAck(suite.chainI.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointA.Counterparty.ChannelID, counterpartyVersion)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *PriceRelayTestSuite) TestPriceFeedThreshold() {

	currentBTCPrice, _ := sdk.NewDecFromStr("48495.410")
	withinThresholdBTCPrice, _ := sdk.NewDecFromStr("49523.620")
	minThresholdBTCPrice, _ := sdk.NewDecFromStr("484.9540")
	maxThresholdBTCPrice, _ := sdk.NewDecFromStr("4952362.012")

	testCases := []struct {
		name         string
		lastPrice    sdk.Dec
		newPrice     sdk.Dec
		expThreshold bool
	}{
		{
			"Within Threshold", sdk.NewDec(100), sdk.NewDec(120), false,
		},
		{
			"Min Threshold", sdk.NewDec(101), sdk.NewDec(1), true,
		},
		{
			"Max Threshold", sdk.NewDec(2), sdk.NewDec(201), true,
		},
		{
			"Within Threshold BTC", currentBTCPrice, withinThresholdBTCPrice, false,
		},
		{
			"Min Threshold BTC", currentBTCPrice, minThresholdBTCPrice, true,
		},
		{
			"Max Threshold BTC", currentBTCPrice, maxThresholdBTCPrice, true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			isThresholdExceeded := oracletypes.CheckPriceFeedThreshold(tc.lastPrice, tc.newPrice)
			suite.Assert().Equal(tc.expThreshold, isThresholdExceeded)
		})
	}
}
