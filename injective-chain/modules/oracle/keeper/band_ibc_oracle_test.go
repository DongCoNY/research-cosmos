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

var _ = Describe("Band IBC Oracle Test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context
	)

	var _ = BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
	})

	Describe("Band IBC Price States Test", func() {
		Context("Band IBC price", func() {
			It("Band IBC price state initial", func() {
				// assert price
				data := app.OracleKeeper.GetBandIBCPriceState(ctx, "INJ")
				Expect(data).To(BeNil())

				states := app.OracleKeeper.GetAllBandIBCPriceStates(ctx)
				Expect(len(states)).To(BeEquivalentTo(0))

				refPrice := app.OracleKeeper.GetBandIBCReferencePrice(ctx, "INJ", "USD")
				Expect(refPrice).To(BeNil())

				refPrice = app.OracleKeeper.GetBandIBCReferencePrice(ctx, "INJ", "INJ")
				Expect(refPrice).To(BeNil())
			})

			It("Band IBC price state after Band IBC relay", func() {
				bandPriceState := &types.BandPriceState{
					Symbol:      "INJ",
					Rate:        sdk.NewInt(20),
					ResolveTime: 1,
					Request_ID:  1,
					PriceState:  *types.NewPriceState(sdk.NewDec(20), 1),
				}

				app.OracleKeeper.SetBandIBCPriceState(ctx, "INJ", bandPriceState)

				// assert price
				data := app.OracleKeeper.GetBandIBCPriceState(ctx, "INJ")
				Expect(data).To(Not(BeNil()))
				Expect(data).To(BeEquivalentTo(bandPriceState))

				states := app.OracleKeeper.GetAllBandIBCPriceStates(ctx)
				Expect(len(states)).To(BeEquivalentTo(1))

				refPrice := app.OracleKeeper.GetBandIBCReferencePrice(ctx, "INJ", "USD")
				Expect(refPrice).To(Not(BeNil()))
				Expect(*refPrice).To(BeEquivalentTo(sdk.NewDec(20)))

				refPrice = app.OracleKeeper.GetBandIBCReferencePrice(ctx, "INJ", "INJ")
				Expect(refPrice).To(Not(BeNil()))
				Expect(*refPrice).To(BeEquivalentTo(sdk.NewDec(1)))
			})
		})
	})

	Describe("Band IBC Oracle Requests Test", func() {
		Context("Band IBC price", func() {
			It("Band IBC oracle requests at initial", func() {
				// assert getting request
				req := app.OracleKeeper.GetBandIBCOracleRequest(ctx, 1)
				Expect(req).To(BeNil())
				reqs := app.OracleKeeper.GetAllBandIBCOracleRequests(ctx)
				Expect(len(reqs)).To(BeZero())
			})

			It("Band IBC oracle requests after setting requests", func() {
				bandOracleRequest := types.BandOracleRequest{
					RequestId:      1,
					OracleScriptId: 1,
					Symbols:        []string{"INJ"},
					AskCount:       1,
					MinCount:       1,
					FeeLimit:       sdk.Coins{sdk.NewInt64Coin("INJ", 1)},
					PrepareGas:     100,
					ExecuteGas:     200,
				}

				app.OracleKeeper.SetBandIBCOracleRequest(ctx, bandOracleRequest)

				// assert getting request
				req := app.OracleKeeper.GetBandIBCOracleRequest(ctx, 1)
				Expect(req).To(Not(BeNil()))
				Expect(*req).To(BeEquivalentTo(bandOracleRequest))
				reqs := app.OracleKeeper.GetAllBandIBCOracleRequests(ctx)
				Expect(len(reqs)).To(BeEquivalentTo(1))

				// delete request and try again
				app.OracleKeeper.DeleteBandIBCOracleRequest(ctx, 1)
				req = app.OracleKeeper.GetBandIBCOracleRequest(ctx, 1)
				Expect(req).To(BeNil())
				reqs = app.OracleKeeper.GetAllBandIBCOracleRequests(ctx)
				Expect(len(reqs)).To(BeZero())
			})
		})
	})

	Describe("Band IBC Latest ClientID Test", func() {
		Context("Band IBC latest clientID", func() {
			It("Band IBC latest clientID at initial", func() {
				id := app.OracleKeeper.GetBandIBCLatestClientID(ctx)
				Expect(id).To(BeEquivalentTo(0))
			})

			It("Band IBC latest clientID after set", func() {
				app.OracleKeeper.SetBandIBCLatestClientID(ctx, 10)
				id := app.OracleKeeper.GetBandIBCLatestClientID(ctx)
				Expect(id).To(BeEquivalentTo(10))
			})
		})
	})

	Describe("Band IBC Call Data Record Test", func() {
		Context("Band IBC call data record", func() {
			It("Band IBC call data record at initial", func() {
				record := app.OracleKeeper.GetBandIBCCallDataRecord(ctx, 1)
				Expect(record).To(BeNil())
			})

			It("Band IBC call data record after set", func() {
				recordA := &types.CalldataRecord{
					ClientId: 1,
					Calldata: []byte("123"),
				}
				app.OracleKeeper.SetBandIBCCallDataRecord(ctx, recordA)
				record := app.OracleKeeper.GetBandIBCCallDataRecord(ctx, 1)
				Expect(record).To(Not(BeNil()))
				Expect(record).To(BeEquivalentTo(recordA))
			})
		})
	})
})
