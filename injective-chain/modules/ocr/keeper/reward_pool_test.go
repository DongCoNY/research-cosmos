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

var _ = Describe("Reward pool test", func() {
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

	Describe("Payee test", func() {
		Context("PendingPayeeshipTransfer test", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String(), signer2.String()},
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
			})

			It("check pending payeeship transfer at initial", func() {
				pendingPayeeship := app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, transmitter)
				Expect(pendingPayeeship).To(BeNil())
				pendingPayeeships := app.OcrKeeper.GetAllPendingPayeeships(ctx)
				Expect(len(pendingPayeeships)).To(BeEquivalentTo(0))
			})

			It("check pending payeeship transfer after set 1", func() {
				app.OcrKeeper.SetPendingPayeeshipTransfer(ctx, feedId, transmitter, signer2)
				pendingPayeeship := app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, transmitter)
				Expect(pendingPayeeship).To(Not(BeNil()))
				Expect(*pendingPayeeship).To(BeEquivalentTo(signer2))
				pendingPayeeships := app.OcrKeeper.GetAllPendingPayeeships(ctx)
				Expect(len(pendingPayeeships)).To(BeEquivalentTo(1))
				Expect(pendingPayeeships[0].FeedId).To(BeEquivalentTo(feedId))
				Expect(pendingPayeeships[0].ProposedPayee).To(BeEquivalentTo(signer2.String()))
				Expect(pendingPayeeships[0].Transmitter).To(BeEquivalentTo(transmitter.String()))
			})

			It("check pending payeeship transfer after set 2", func() {
				app.OcrKeeper.SetPendingPayeeshipTransfer(ctx, feedId, transmitter, signer2)
				app.OcrKeeper.SetPendingPayeeshipTransfer(ctx, feedId, signer2, transmitter)
				pendingPayeeship := app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, transmitter)
				Expect(pendingPayeeship).To(Not(BeNil()))
				Expect(*pendingPayeeship).To(BeEquivalentTo(signer2))

				pendingPayeeship = app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, signer2)
				Expect(pendingPayeeship).To(Not(BeNil()))
				Expect(*pendingPayeeship).To(BeEquivalentTo(transmitter))

				pendingPayeeships := app.OcrKeeper.GetAllPendingPayeeships(ctx)
				Expect(len(pendingPayeeships)).To(BeEquivalentTo(2))
				Expect(pendingPayeeships[0].FeedId).To(BeEquivalentTo(feedId))
				Expect(pendingPayeeships[1].FeedId).To(BeEquivalentTo(feedId))
			})

			It("check pending payeeship transfer after set 2 and delete 1", func() {
				app.OcrKeeper.SetPendingPayeeshipTransfer(ctx, feedId, transmitter, signer2)
				app.OcrKeeper.SetPendingPayeeshipTransfer(ctx, feedId, signer2, transmitter)
				app.OcrKeeper.DeletePendingPayeeshipTransfer(ctx, feedId, transmitter)
				pendingPayeeship := app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, transmitter)
				Expect(pendingPayeeship).To(BeNil())

				pendingPayeeship = app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, signer2)
				Expect(pendingPayeeship).To(Not(BeNil()))
				Expect(*pendingPayeeship).To(BeEquivalentTo(transmitter))

				pendingPayeeships := app.OcrKeeper.GetAllPendingPayeeships(ctx)
				Expect(len(pendingPayeeships)).To(BeEquivalentTo(1))
				Expect(pendingPayeeships[0].ProposedPayee).To(BeEquivalentTo(transmitter.String()))
				Expect(pendingPayeeships[0].Transmitter).To(BeEquivalentTo(signer2.String()))
			})
		})
	})
})
