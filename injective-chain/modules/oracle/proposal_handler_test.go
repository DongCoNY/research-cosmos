package oracle_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

var _ = Describe("Proposal handler tests", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context
		err error
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
	})

	Describe("Enable band IBC oracle proposal", func() {
		It("should success", func() {

			handler := oracle.NewOracleProposalHandler(app.OracleKeeper)
			bandIBCParams := types.DefaultTestBandIbcParams()
			err = handler(ctx, &types.EnableBandIBCProposal{
				Title:         "Band oracle IBC enable proposal",
				Description:   "Enable band oracle IBC.",
				BandIbcParams: *bandIBCParams,
			})
			Expect(err).To(BeNil())

			portID := app.OracleKeeper.GetPort(ctx)
			Expect(portID).To(BeEquivalentTo(bandIBCParams.IbcPortId))

			isBound := app.OracleKeeper.IsBound(ctx, portID)
			Expect(isBound).To(BeTrue())

			onchainParams := app.OracleKeeper.GetBandIBCParams(ctx)
			Expect(onchainParams).To(BeEquivalentTo(bandIBCParams))
		})
	})
})
