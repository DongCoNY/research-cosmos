package keeper_test

import (
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("GRPC query test", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
	})

	Context("DerivativeMarketAddress check", func() {
		It("check value", func() {
			res, err := app.ExchangeKeeper.DerivativeMarketAddress(sdk.WrapSDKContext(ctx), &exchangetypes.QueryDerivativeMarketAddressRequest{
				MarketId: testInput.Perps[0].MarketID.Hex(),
			})
			Expect(err).To(BeNil())
			Expect(res.Address).To(BeEquivalentTo(exchangetypes.SubaccountIDToSdkAddress(testInput.Perps[0].MarketID).String()))
			Expect(res.SubaccountId).To(BeEquivalentTo(exchangetypes.SdkAddressToSubaccountID(exchangetypes.SubaccountIDToSdkAddress(testInput.Perps[0].MarketID)).String()))
		})
	})
})
