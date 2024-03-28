package keeper_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
)

var _ = Describe("OCR config functions test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		feedConfig     *types.FeedConfig
		feedConfigInfo *types.FeedConfigInfo
		prevConfig     *types.FeedConfigInfo
		feedId         = "feed_hash"
		configDigest   []byte
		// feedHashHex = hex.EncodeToString()

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

	Describe("Feed config setter / getter", func() {
		Context("get feed config before initialization", func() {
			JustBeforeEach(func() {
				feedConfig = app.OcrKeeper.GetFeedConfig(ctx, feedId)
			})
			It("should be nil", func() {
				Expect(feedConfig).To(BeNil())
			})
		})
		Context("get feed config after initialization", func() {
			JustBeforeEach(func() {
				prevConfig = app.OcrKeeper.SetFeedConfig(ctx, feedId, &types.FeedConfig{
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
				feedConfig = app.OcrKeeper.GetFeedConfig(ctx, feedId)
			})
			It("should not be nil", func() {
				Expect(prevConfig).To(Not(BeNil()))
				Expect(prevConfig.F).To(BeEquivalentTo(uint32(0)))
				Expect(feedConfig).To(Not(BeNil()))
				Expect(feedConfig.ModuleParams.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedConfig.Signers).To(BeEquivalentTo([]string{signer1.String(), signer2.String()}))
				Expect(feedConfig.Transmitters).To(BeEquivalentTo([]string{transmitter.String()}))
				Expect(feedConfig.F).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OnchainConfig).To(BeEquivalentTo([]byte("onchain_config")))
				Expect(feedConfig.OffchainConfigVersion).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OffchainConfig).To(BeEquivalentTo([]byte("offchain_config")))
			})
		})
		Context("get feed config by digest", func() {
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

				configInfo := app.OcrKeeper.GetFeedConfigInfo(ctx, feedId)
				if configInfo == nil {
					configInfo = &types.FeedConfigInfo{}
				}

				cc := &types.ContractConfig{
					ConfigCount:           configInfo.ConfigCount,
					Signers:               feedConfig.Signers,
					Transmitters:          feedConfig.Transmitters,
					F:                     feedConfig.F,
					OnchainConfig:         feedConfig.OnchainConfig,
					OffchainConfigVersion: feedConfig.OffchainConfigVersion,
					OffchainConfig:        feedConfig.OffchainConfig,
				}
				configDigest = cc.Digest(testChainID, feedConfig.ModuleParams.FeedId)

				feedConfig = app.OcrKeeper.GetFeedConfigByDigest(ctx, configDigest)
			})
			It("should not be nil", func() {
				Expect(feedConfig).To(Not(BeNil()))
				Expect(feedConfig.ModuleParams.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedConfig.Signers).To(BeEquivalentTo([]string{signer1.String(), signer2.String()}))
				Expect(feedConfig.Transmitters).To(BeEquivalentTo([]string{transmitter.String()}))
				Expect(feedConfig.F).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OnchainConfig).To(BeEquivalentTo([]byte("onchain_config")))
				Expect(feedConfig.OffchainConfigVersion).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OffchainConfig).To(BeEquivalentTo([]byte("offchain_config")))
			})
			It("check Transmitter", func() {
				Expect(app.OcrKeeper.IsTransmitter(ctx, feedId, transmitter)).To(BeTrue())
				Expect(app.OcrKeeper.IsTransmitter(ctx, feedId, signer1)).To(BeFalse())
			})
			It("check Signer", func() {
				Expect(app.OcrKeeper.IsSigner(ctx, feedId, transmitter)).To(BeFalse())
				Expect(app.OcrKeeper.IsSigner(ctx, feedId, signer1)).To(BeTrue())
			})
			It("check GetAllTransmitters", func() {
				Expect(app.OcrKeeper.GetAllTransmitters(ctx, feedId)).To(BeEquivalentTo(feedConfig.Transmitters))
			})
			It("check GetAllSigners", func() {
				Expect(app.OcrKeeper.GetAllSigners(ctx, feedId)).To(BeEquivalentTo(feedConfig.Signers))
			})
			It("check GetAllFeedConfigs", func() {
				Expect(app.OcrKeeper.GetAllFeedConfigs(ctx)).To(BeEquivalentTo([]*types.FeedConfig{feedConfig}))
			})
		})
	})

	Describe("Feed config info setter / getter", func() {
		Context("get feed config before initialization", func() {
			JustBeforeEach(func() {
				feedConfigInfo = app.OcrKeeper.GetFeedConfigInfo(ctx, feedId)
			})
			It("should be nil", func() {
				Expect(feedConfigInfo).To(BeNil())
			})
		})
		Context("get feed config after initialization", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetFeedConfigInfo(ctx, feedId, &types.FeedConfigInfo{
					LatestConfigDigest:      []byte("latest_config_digest"),
					F:                       1,
					N:                       1,
					ConfigCount:             1,
					LatestConfigBlockNumber: 1,
				})
				feedConfigInfo = app.OcrKeeper.GetFeedConfigInfo(ctx, feedId)
			})
			It("should not be nil", func() {
				Expect(feedConfigInfo).To(Not(BeNil()))
				Expect(feedConfigInfo.LatestConfigDigest).To(BeEquivalentTo([]byte("latest_config_digest")))
				Expect(feedConfigInfo.F).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfigInfo.N).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfigInfo.ConfigCount).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfigInfo.LatestConfigBlockNumber).To(BeEquivalentTo(uint32(1)))
			})
		})
	})
})
