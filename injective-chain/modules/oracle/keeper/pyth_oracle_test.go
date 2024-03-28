package keeper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	tendermint "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	eth "github.com/ethereum/go-ethereum/common"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Pyth Oracle Test", func() {
	var (
		app     *simapp.InjectiveApp
		ctx     sdk.Context
		priceID eth.Hash
	)

	BeforeEach(func() {
		priceID = eth.HexToHash("0x7a5bc1d2b56ad029048cd63964b3ad2776eadf812edc1a43a31406cb54bff592") // INJ/USD
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(
			false,
			tendermint.Header{
				Height:  1,
				ChainID: "3",
				Time:    time.Unix(1618997040, 0),
			},
		)
	})

	Describe("Pyth Price States Test", func() {
		var (
			priceState       *types.PythPriceState
			priceAttestation *types.PriceAttestation
		)

		When("price state is not set", func() {
			It("price state and price are nil ", func() {
				Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).To(BeNil())
				Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(0))
				Expect(app.OracleKeeper.GetPythPrice(ctx, "INJ", "USD")).To(BeNil())
				Expect(app.OracleKeeper.GetPythPrice(ctx, "INJ", "INJ")).To(BeNil())
			})
		})

		When("price state is set", func() {
			JustBeforeEach(func() {
				priceState = &types.PythPriceState{
					PriceId:    priceID.Hex(),
					PriceState: types.PriceState{Price: sdk.NewDec(1000)},
				}

				app.OracleKeeper.SetPythPriceState(ctx, priceState)
			})

			It("price state and price are not nil", func() {
				state := app.OracleKeeper.GetPythPriceState(ctx, priceID)
				Expect(state).To(Not(BeNil()))
				Expect(state).To(BeEquivalentTo(priceState))
				Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(1))

				refPrice := app.OracleKeeper.GetPythPrice(ctx, priceID.String(), "USD")
				Expect(refPrice).To(Not(BeNil()))
				Expect(*refPrice).To(BeEquivalentTo(sdk.MustNewDecFromStr("1000")))

				refPrice = app.OracleKeeper.GetPythPrice(ctx, priceID.String(), priceID.String())
				Expect(refPrice).To(Not(BeNil()))
				Expect(*refPrice).To(BeEquivalentTo(sdk.MustNewDecFromStr("1")))
			})
		})

		When("price attestation is processed", func() {
			BeforeEach(func() {
				priceAttestation = &types.PriceAttestation{
					PriceId:     priceID.Hex(),
					Price:       123456,
					Conf:        500,
					Expo:        -3,
					EmaPrice:    1000,
					EmaConf:     2000,
					EmaExpo:     -3,
					PublishTime: ctx.BlockTime().Unix(),
				}
			})

			Context("for the first time", func() {
				JustBeforeEach(func() {
					app.OracleKeeper.ProcessPythPriceAttestations(ctx, []*types.PriceAttestation{priceAttestation})
				})

				It("price state is set correctly", func() {
					priceState := app.OracleKeeper.GetPythPriceState(ctx, priceID)
					Expect(priceState).NotTo(BeNil())

					states := app.OracleKeeper.GetAllPythPriceStates(ctx)
					Expect(len(states)).To(Equal(1))
					Expect(priceState.PriceId).To(Equal(priceID.Hex()))
					Expect(priceState.Conf).To(Equal(sdk.MustNewDecFromStr("0.5")))
					Expect(priceState.EmaPrice).To(Equal(sdk.MustNewDecFromStr("1")))
					Expect(priceState.EmaConf).To(Equal(sdk.MustNewDecFromStr("2")))
					Expect(priceState.PriceState.Price).To(Equal(sdk.MustNewDecFromStr("123.456")))
					Expect(priceState.PriceState.Timestamp).To(Equal(ctx.BlockTime().Unix()))
				})
			})

			Context("and it's not the first time", func() {
				var (
					currentPriceState *types.PythPriceState
					newAttestation    *types.PriceAttestation
				)

				BeforeEach(func() {
					app.OracleKeeper.ProcessPythPriceAttestations(ctx, []*types.PriceAttestation{priceAttestation})
					currentPriceState = app.OracleKeeper.GetPythPriceState(ctx, priceID)
					Expect(currentPriceState).NotTo(BeNil())
					Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(1))
				})

				When("the new price attestation is outdated", func() {
					JustBeforeEach(func() {
						oldPublishTime := ctx.BlockTime().Add(-1 * time.Second)
						newAttestation = &types.PriceAttestation{
							PriceId:     priceID.Hex(),
							Price:       654321,
							Conf:        500,
							Expo:        -3,
							EmaPrice:    1000,
							EmaConf:     2000,
							PublishTime: oldPublishTime.Unix(),
						}

						app.OracleKeeper.ProcessPythPriceAttestations(ctx, []*types.PriceAttestation{newAttestation})
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).NotTo(BeNil())
						Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(1))
					})

					It("the price state is not updated", func() {
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).To(BeEquivalentTo(currentPriceState))
					})
				})

				When("the new price is less than 1% of the last price", func() {
					JustBeforeEach(func() {
						newAttestation = &types.PriceAttestation{
							PriceId: priceID.Hex(),
							//Price:       123),
							Price:       123,
							Conf:        500,
							Expo:        -3,
							EmaPrice:    1000,
							EmaConf:     2000,
							PublishTime: ctx.BlockTime().Unix(),
						}

						app.OracleKeeper.ProcessPythPriceAttestations(ctx, []*types.PriceAttestation{newAttestation})
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).NotTo(BeNil())
						Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(1))
					})

					It("the price state is not updated", func() {
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).To(BeEquivalentTo(currentPriceState))
					})
				})

				When("the new price is 100x greater than last price", func() {
					JustBeforeEach(func() {
						newAttestation = &types.PriceAttestation{
							PriceId:     priceID.Hex(),
							Price:       12345601,
							Conf:        500,
							Expo:        -3,
							EmaPrice:    1000,
							EmaConf:     2000,
							PublishTime: ctx.BlockTime().Unix(),
						}

						app.OracleKeeper.ProcessPythPriceAttestations(ctx, []*types.PriceAttestation{newAttestation})
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).NotTo(BeNil())
						Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(1))
					})

					It("the price state is not updated", func() {
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).To(BeEquivalentTo(currentPriceState))
					})
				})

				When("and the new price attestation has a better price", func() {
					JustBeforeEach(func() {
						newPublishTime := ctx.BlockTime().Add(time.Second)
						newAttestation = &types.PriceAttestation{
							PriceId:     priceID.Hex(),
							Price:       654321,
							Conf:        500,
							Expo:        -3,
							EmaPrice:    1000,
							EmaConf:     2000,
							EmaExpo:     -3,
							PublishTime: newPublishTime.Unix(),
						}

						ctx = ctx.WithBlockTime(newPublishTime)
						app.OracleKeeper.ProcessPythPriceAttestations(ctx, []*types.PriceAttestation{newAttestation})
						Expect(app.OracleKeeper.GetPythPriceState(ctx, priceID)).NotTo(BeNil())
						Expect(len(app.OracleKeeper.GetAllPythPriceStates(ctx))).To(BeEquivalentTo(1))
					})

					It("price state is updated correctly", func() {
						newPriceState := app.OracleKeeper.GetPythPriceState(ctx, priceID)
						Expect(newPriceState.PriceId).To(Equal(priceID.Hex()))
						Expect(newPriceState.Conf).To(Equal(sdk.MustNewDecFromStr("0.5")))
						Expect(newPriceState.EmaPrice).To(Equal(sdk.MustNewDecFromStr("1")))
						Expect(newPriceState.EmaConf).To(Equal(sdk.MustNewDecFromStr("2")))
						Expect(newPriceState.PriceState.Price).To(Equal(sdk.MustNewDecFromStr("654.321")))
						Expect(newPriceState.PriceState.Timestamp).To(Equal(ctx.BlockTime().Unix()))
					})
				})
			})
		})
	})
})
