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

var _ = Describe("Transmissions test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		feedConfig *types.FeedConfig
		feedId     = "feed_hash"

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

		signer1     = AccAddrs[0]
		signer2     = AccAddrs[1]
		transmitter = AccAddrs[2]
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: testChainID, Time: time.Unix(1618997040, 0)})
	})

	Describe("Transmission test", func() {
		Context("Transmission count test", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:    feedId,
						LinkDenom: "link",
					},
				}
				app.OcrKeeper.SetFeedConfig(ctx, feedId, feedConfig)
				app.OcrKeeper.SetFeedTransmissionsCount(ctx, feedId, transmitter, 100)
			})
			It("check transmission count", func() {
				feedCounts := app.OcrKeeper.GetFeedTransmissionCounts(ctx, feedId)
				Expect(len(feedCounts.Counts)).To(BeEquivalentTo(1))
				Expect(feedCounts.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedCounts.Counts[0].Address).To(BeEquivalentTo(transmitter.String()))
				Expect(feedCounts.Counts[0].Count).To(BeEquivalentTo(100))
			})
			It("check transmission count after single deletion", func() {
				app.OcrKeeper.DeleteFeedTransmissionCounts(ctx, feedId)
				feedCounts := app.OcrKeeper.GetFeedTransmissionCounts(ctx, feedId)
				Expect(len(feedCounts.Counts)).To(BeEquivalentTo(0))
			})
			It("check transmission count after all deletion", func() {
				app.OcrKeeper.DeleteAllFeedTransmissionCounts(ctx)
				feedCounts := app.OcrKeeper.GetFeedTransmissionCounts(ctx, feedId)
				Expect(len(feedCounts.Counts)).To(BeEquivalentTo(0))
			})
		})
	})
})
