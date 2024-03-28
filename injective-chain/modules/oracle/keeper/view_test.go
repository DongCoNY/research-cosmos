package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("View Tests", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context
	)

	var _ = BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
		// ctx = testInput.EndBlockerAndCommit(ctx, 1)
	})

	Describe("Get Price for given pair", func() {
		Context("Band IBC price", func() {
			It("Band IBC initial price", func() {
				// assert price
				data := app.OracleKeeper.GetPrice(ctx, types.OracleType_BandIBC, "INJ", "USD")
				Expect(data).To(BeNil())
			})

			It("Band IBC price after Band IBC relay", func() {
				bandPriceState := &types.BandPriceState{
					Symbol:      "INJ",
					Rate:        sdk.NewInt(20),
					ResolveTime: 1,
					Request_ID:  1,
					PriceState:  *types.NewPriceState(sdk.NewDec(20), 1),
				}

				app.OracleKeeper.SetBandIBCPriceState(ctx, "INJ", bandPriceState)

				// assert price
				data := app.OracleKeeper.GetPrice(ctx, types.OracleType_BandIBC, "INJ", "USD")
				Expect(data).To(Not(BeNil()))
				Expect(*data).To(BeEquivalentTo(sdk.NewDec(20)))
			})

			It("Band IBC price after Band IBC relay with non-USD quote", func() {
				bandPriceState := &types.BandPriceState{
					Symbol:      "INJ",
					Rate:        sdk.NewInt(20),
					ResolveTime: 1,
					Request_ID:  1,
					PriceState:  *types.NewPriceState(sdk.NewDec(20), 1),
				}

				app.OracleKeeper.SetBandIBCPriceState(ctx, "INJ", bandPriceState)

				// assert price
				data := app.OracleKeeper.GetPrice(ctx, types.OracleType_BandIBC, "INJ", "INJ")
				Expect(data).To(Not(BeNil()))
				Expect(*data).To(BeEquivalentTo(sdk.NewDec(1)))
			})
		})
	})
})
