package ocr_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	ethsecp256k1 "github.com/ethereum/go-ethereum/crypto/secp256k1"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/types"
)

const testChainID = "3"

var _ = Describe("Handler tests", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		feedConfig *types.FeedConfig
		feedId     = "feed_hash"
		// feedHashHex = hex.EncodeToString()
		transmission *types.Transmission
		msgServer    types.MsgServer
		err          error

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
		msgServer = keeper.NewMsgServerImpl(app.OcrKeeper)
	})

	Describe("transmission via handler test", func() {
		Context("MsgTransmit", func() {
			JustBeforeEach(func() {
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String(), transmitter.String()},
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
						Description:         "",
					},
				}
				app.OcrKeeper.SetFeedConfig(ctx, feedId, feedConfig)
				configInfo := app.OcrKeeper.GetFeedConfigInfo(ctx, feedId)
				if configInfo == nil {
					configInfo = &types.FeedConfigInfo{}
				}

				cc := &types.ContractConfig{
					ConfigCount:  configInfo.ConfigCount,
					Signers:      feedConfig.Signers,
					Transmitters: feedConfig.Transmitters,
					F:            feedConfig.F,

					OnchainConfig:         feedConfig.OnchainConfig,
					OffchainConfigVersion: feedConfig.OffchainConfigVersion,
					OffchainConfig:        feedConfig.OffchainConfig,
				}
				configDigest := cc.Digest(testChainID, feedConfig.ModuleParams.FeedId)

				msg := &types.MsgTransmit{
					Transmitter:  transmitter.String(),
					ConfigDigest: configDigest,
					FeedId:       feedId,
					Epoch:        1,
					Round:        1,
					ExtraHash:    []byte("extra_hash"),
					Report: &types.Report{
						ObservationsTimestamp: 1555000,
						Observers:             []byte("Observers"),
						Observations:          []sdk.Dec{sdk.NewDec(1), sdk.NewDec(1), sdk.NewDec(1)},
					},
					Signatures: [][]byte{},
				}

				reportBytes, err := proto.Marshal(msg.Report)
				Expect(err).To(BeNil())

				sigData := (&types.ReportToSign{
					ConfigDigest: msg.ConfigDigest,
					Epoch:        msg.Epoch,
					Round:        msg.Round,
					ExtraHash:    msg.ExtraHash,
					Report:       reportBytes,
				}).Digest()

				sig1, err := ethsecp256k1.Sign(sigData, AccPrivKeys[0].Bytes())
				Expect(err).To(BeNil())
				msg.Signatures = append(msg.Signatures, sig1)
				sig2, err := ethsecp256k1.Sign(sigData, AccPrivKeys[1].Bytes())
				Expect(err).To(BeNil())
				msg.Signatures = append([][]byte{}, sig1, sig2)

				_, _ = msgServer.Transmit(sdk.WrapSDKContext(ctx), msg)
				transmission = app.OcrKeeper.GetTransmission(ctx, feedId)
			})
			It("should be set correctly", func() {
				Expect(err).To(BeNil())
				Expect(transmission).To(Not(BeNil()))
				Expect(transmission.Answer).To(BeEquivalentTo(sdk.NewDec(1)))
				Expect(transmission.ObservationsTimestamp).To(BeEquivalentTo(uint32(1555000)))
				Expect(transmission.TransmissionTimestamp).To(BeEquivalentTo(uint32(1618997040)))
			})
		})
	})
})

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}
