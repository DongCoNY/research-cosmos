package keeper_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/types"
)

var _ = Describe("OCR report functions test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		feedConfig *types.FeedConfig
		feedId     = "feed_hash"
		// feedHashHex = hex.EncodeToString()
		transmission      *types.Transmission
		aggregatorRoundId uint64
		epochAndRound     *types.EpochAndRound
		err               error

		AccPrivKeys = []ccrypto.PrivKey{
			secp256k1.GenPrivKey(),
			secp256k1.GenPrivKey(),
			secp256k1.GenPrivKey(),
		}

		AccPubKeys = []ccrypto.PubKey{
			AccPrivKeys[0].PubKey(),
			AccPrivKeys[1].PubKey(),
			AccPrivKeys[2].PubKey(),
		}

		AccAddrs = []sdk.AccAddress{
			sdk.AccAddress(AccPubKeys[0].Address()),
			sdk.AccAddress(AccPubKeys[1].Address()),
			sdk.AccAddress(AccPubKeys[2].Address()),
		}

		signer1     = AccAddrs[0]
		signer2     = AccAddrs[1]
		transmitter = AccAddrs[2]
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: testChainID, Time: time.Unix(1618997040, 0)})
	})

	Describe("transmission test", func() {
		Context("transmission before initialization", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId: feedId,
					},
				}
				app.OcrKeeper.SetFeedConfig(ctx, feedId, feedConfig)
				transmission = app.OcrKeeper.GetTransmission(ctx, feedId)
			})
			It("should be nil", func() {
				Expect(transmission).To(BeNil())
			})
		})
		Context("transmission after set", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetFeedConfig(ctx, feedId, &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId: feedId,
					},
				})
				app.OcrKeeper.SetTransmission(ctx, feedId, &types.Transmission{
					Answer:                sdk.NewDec(1),
					ObservationsTimestamp: 1555000,
					TransmissionTimestamp: 2555000,
				})
				transmission = app.OcrKeeper.GetTransmission(ctx, feedId)
			})
			It("should not be nil", func() {
				Expect(transmission).To(Not(BeNil()))
				Expect(transmission.Answer).To(BeEquivalentTo(sdk.NewDec(1)))
				Expect(transmission.ObservationsTimestamp).To(BeEquivalentTo(uint32(1555000)))
				Expect(transmission.TransmissionTimestamp).To(BeEquivalentTo(uint32(2555000)))
			})
			It("check get all transmissions", func() {
				transmissions := app.OcrKeeper.GetAllFeedTransmissions(ctx)
				Expect(len(transmissions)).To(BeEquivalentTo(1))
				Expect(transmissions[0]).To(Not(BeNil()))
				Expect(transmissions[0].FeedId).To(BeEquivalentTo("feed_hash"))
				Expect(transmissions[0].Transmission.Answer).To(BeEquivalentTo(sdk.NewDec(1)))
				Expect(transmissions[0].Transmission.ObservationsTimestamp).To(BeEquivalentTo(uint32(1555000)))
				Expect(transmissions[0].Transmission.TransmissionTimestamp).To(BeEquivalentTo(uint32(2555000)))
			})
		})
		Context("transmission set using TransmitterReport", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:              feedId,
						MinAnswer:           sdk.NewDecWithPrec(1, 1), // 0.1
						MaxAnswer:           sdk.NewDec(100),          // 100
						LinkPerObservation:  sdk.NewInt(4),
						LinkPerTransmission: sdk.NewInt(7),
						LinkDenom:           "link",
						UniqueReports:       false,
						Description:         "tatrnsmissions",
					},
				}
				app.OcrKeeper.SetFeedConfig(ctx, feedId, feedConfig)
				feedConfigInfo := app.OcrKeeper.GetFeedConfigInfo(ctx, feedId)
				err = app.OcrKeeper.TransmitterReport(
					ctx,
					transmitter,
					feedId,
					feedConfig,
					feedConfigInfo,
					1, 1,
					types.Report{
						ObservationsTimestamp: 1555000,
						Observers:             []byte("Observers"),
						Observations:          []sdk.Dec{sdk.NewDec(1), sdk.NewDec(1), sdk.NewDec(1)},
					})
				aggregatorRoundId = app.OcrKeeper.LatestAggregatorRoundID(ctx, feedId)
				transmission = app.OcrKeeper.GetTransmission(ctx, feedId)
			})
			It("values should be updated correctly", func() {
				Expect(err).To(BeNil())
				Expect(aggregatorRoundId).To(BeEquivalentTo(uint64(1)))
				Expect(transmission).To(Not(BeNil()))
				Expect(transmission.Answer).To(BeEquivalentTo(sdk.NewDec(1)))
				Expect(transmission.ObservationsTimestamp).To(BeEquivalentTo(uint32(1555000)))
				Expect(transmission.TransmissionTimestamp).To(BeEquivalentTo(uint32(1618997040)))
			})
		})
	})

	Describe("aggregator round id test", func() {
		Context("aggregator round id before set", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId: feedId,
					},
				}
				app.OcrKeeper.SetFeedConfig(ctx, feedId, feedConfig)
				aggregatorRoundId = app.OcrKeeper.LatestAggregatorRoundID(ctx, feedId)
			})
			It("should be 0", func() {
				Expect(aggregatorRoundId).To(BeEquivalentTo(uint64(0)))
			})
		})
		Context("aggregator round id after increase", func() {
			JustBeforeEach(func() {
				aggregatorRoundId = app.OcrKeeper.IncreaseAggregatorRoundID(ctx, feedId)
			})
			It("should be increased", func() {
				Expect(aggregatorRoundId).To(BeEquivalentTo(uint64(1)))
				aggregatorRoundId = app.OcrKeeper.LatestAggregatorRoundID(ctx, feedId)
				Expect(aggregatorRoundId).To(BeEquivalentTo(uint64(1)))
			})
		})
		Context("aggregator round id after set", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetAggregatorRoundID(ctx, feedId, 100)
			})
			It("should be set correctly", func() {
				aggregatorRoundId = app.OcrKeeper.LatestAggregatorRoundID(ctx, feedId)
				Expect(aggregatorRoundId).To(BeEquivalentTo(uint64(100)))
			})
			It("check all latest aggregator round ids", func() {
				aggregatorRoundIds := app.OcrKeeper.GetAllLatestAggregatorRoundIDs(ctx)
				Expect(aggregatorRoundIds).To(BeEquivalentTo([]*types.FeedLatestAggregatorRoundIDs{{
					FeedId:            feedId,
					AggregatorRoundId: 100,
				}}))
			})
		})
	})

	Describe("epoch and round setter and getter", func() {
		Context("get latest epoch and round before set", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),

					ModuleParams: &types.ModuleParams{
						FeedId: feedId,
					},
				}
				app.OcrKeeper.SetFeedConfig(ctx, feedId, feedConfig)
				epochAndRound = app.OcrKeeper.GetLatestEpochAndRound(ctx, feedId)
			})
			It("should be 0,0 value", func() {
				Expect(epochAndRound).To(Not(BeNil()))
				Expect(epochAndRound.Epoch).To(BeEquivalentTo(uint64(0)))
				Expect(epochAndRound.Round).To(BeEquivalentTo(uint64(0)))
			})
		})
		Context("aggregator round id after set", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetLatestEpochAndRound(ctx, feedId, &types.EpochAndRound{
					Epoch: 1,
					Round: 2,
				})
				epochAndRound = app.OcrKeeper.GetLatestEpochAndRound(ctx, feedId)
			})
			It("should be set", func() {
				Expect(epochAndRound).To(Not(BeNil()))
				Expect(epochAndRound.Epoch).To(BeEquivalentTo(uint64(1)))
				Expect(epochAndRound.Round).To(BeEquivalentTo(uint64(2)))
			})
		})
		Context("get all aggregator round ids", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetLatestEpochAndRound(ctx, feedId, &types.EpochAndRound{
					Epoch: 1,
					Round: 2,
				})
			})
			It("should be set", func() {
				epochAndRounds := app.OcrKeeper.GetAllLatestEpochAndRounds(ctx)
				Expect(len(epochAndRounds)).To(BeEquivalentTo(1))
				Expect(epochAndRounds[0].FeedId).To(BeEquivalentTo(feedId))
				Expect(epochAndRounds[0].EpochAndRound).To(Not(BeNil()))
				Expect(epochAndRounds[0].EpochAndRound.Epoch).To(BeEquivalentTo(uint64(1)))
				Expect(epochAndRounds[0].EpochAndRound.Round).To(BeEquivalentTo(uint64(2)))
			})
		})
	})
})
