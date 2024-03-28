package ocr_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/types"
)

var _ = Describe("Proposal handler tests", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		feedConfig *types.FeedConfig
		feedId     = "feed_hash"
		// feedHashHex = hex.EncodeToString()
		proposalHandler govtypes.Handler
		err             error

		AccPrivKeys = []ccrypto.PrivKey{
			secp256k1.GenPrivKey(),
			secp256k1.GenPrivKey(),
			secp256k1.GenPrivKey(),
			secp256k1.GenPrivKey(),
		}

		AccPubKeys = []ccrypto.PubKey{
			AccPrivKeys[0].PubKey(),
			AccPrivKeys[1].PubKey(),
			AccPrivKeys[2].PubKey(),
			AccPrivKeys[3].PubKey(),
		}

		AccAddrs = []sdk.AccAddress{
			sdk.AccAddress(AccPubKeys[0].Address()),
			sdk.AccAddress(AccPubKeys[1].Address()),
			sdk.AccAddress(AccPubKeys[2].Address()),
			sdk.AccAddress(AccPubKeys[3].Address()),
		}
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: testChainID, Time: time.Unix(1618997040, 0)})
		proposalHandler = ocr.NewOcrProposalHandler(app.OcrKeeper)
	})

	Describe("proposal handler tests", func() {
		Context("Set feed config proposal handler", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()},
					Transmitters:          []string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:              feedId,
						MinAnswer:           sdk.NewDecWithPrec(1, 1), // 0.1
						MaxAnswer:           sdk.NewDec(100),          // 100
						LinkPerObservation:  sdk.NewInt(10),
						LinkPerTransmission: sdk.NewInt(20),
						LinkDenom:           app.OcrKeeper.LinkDenom(ctx),
						Description:         "",
					},
				}
				err = proposalHandler(ctx, &types.SetConfigProposal{
					Title:       "Set feed config proposal",
					Description: "Set feed config proposal :)",
					Config:      feedConfig,
				})

				feedConfig = app.OcrKeeper.GetFeedConfig(ctx, feedId)
			})
			It("should be nil", func() {
				Expect(err).To(BeNil())
				Expect(feedConfig).To(Not(BeNil()))
				Expect(feedConfig.ModuleParams.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedConfig.Signers).To(BeEquivalentTo([]string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()}))
				Expect(feedConfig.Transmitters).To(BeEquivalentTo([]string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()}))
				Expect(feedConfig.F).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OnchainConfig).To(BeEquivalentTo([]byte("onchain_config")))
				Expect(feedConfig.OffchainConfigVersion).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OffchainConfig).To(BeEquivalentTo([]byte("offchain_config")))
			})
		})

		Context("Set batch feed config proposal handler", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()},
					Transmitters:          []string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:    feedId,
						MinAnswer: sdk.NewDecWithPrec(1, 1), // 0.1
						MaxAnswer: sdk.NewDec(100),          // 100
					},
				}
				err = proposalHandler(ctx, &types.SetBatchConfigProposal{
					Title:        "Set feed config proposal",
					Description:  "Set feed config proposal :)",
					LinkDenom:    app.OcrKeeper.LinkDenom(ctx),
					Signers:      []string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()},
					Transmitters: []string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()},
					FeedProperties: []*types.FeedProperties{{
						FeedId:                feedId,
						F:                     1,
						OnchainConfig:         []byte("onchain_config"),
						OffchainConfigVersion: 1,
						OffchainConfig:        []byte("offchain_config"),
						MinAnswer:             sdk.NewDecWithPrec(1, 1), // 0.1
						MaxAnswer:             sdk.NewDec(100),          // 100
						LinkPerObservation:    sdk.NewInt(10),
						LinkPerTransmission:   sdk.NewInt(20),
						Description:           "",
					}},
				})

				feedConfig = app.OcrKeeper.GetFeedConfig(ctx, feedId)
			})
			It("should be nil", func() {
				Expect(err).To(BeNil())
				Expect(feedConfig).To(Not(BeNil()))
				Expect(feedConfig.ModuleParams.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedConfig.Signers).To(BeEquivalentTo([]string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()}))
				Expect(feedConfig.Transmitters).To(BeEquivalentTo([]string{AccAddrs[0].String(), AccAddrs[1].String(), AccAddrs[2].String(), AccAddrs[3].String()}))
				Expect(feedConfig.F).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OnchainConfig).To(BeEquivalentTo([]byte("onchain_config")))
				Expect(feedConfig.OffchainConfigVersion).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OffchainConfig).To(BeEquivalentTo([]byte("offchain_config")))
			})
		})
	})
})
