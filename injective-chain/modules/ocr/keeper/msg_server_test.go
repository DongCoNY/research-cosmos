package keeper_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/gogoproto/proto"
	ethsecp256k1 "github.com/ethereum/go-ethereum/crypto/secp256k1"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/types"
)

const testChainID = "3"

var _ = Describe("MsgServer test", func() {
	var (
		app       *simapp.InjectiveApp
		ctx       sdk.Context
		msgServer types.MsgServer

		feedConfig *types.FeedConfig
		feedId     = "feed_hash"
		err        error

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
		moduleAdmin = AccAddrs[3]
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: testChainID, Time: time.Unix(1618997040, 0)})
		msgServer = keeper.NewMsgServerImpl(app.OcrKeeper)
	})

	Describe("Feed test", func() {
		Context("CreateFeed happy path test", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, types.Params{
					LinkDenom:           "link",
					PayoutBlockInterval: 2,
					ModuleAdmin:         moduleAdmin.String(),
				})
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:    feedId,
						LinkDenom: "link",
					},
				}
				_, err = msgServer.CreateFeed(sdk.WrapSDKContext(ctx), &types.MsgCreateFeed{
					Sender: moduleAdmin.String(),
					Config: feedConfig,
				})
			})
			It("error should be nil", func() {
				Expect(err).To(BeNil())
			})
			It("check feedConfig changes", func() {
				feedConfig = app.OcrKeeper.GetFeedConfig(ctx, feedId)
				Expect(feedConfig).To(Not(BeNil()))
				Expect(feedConfig.ModuleParams.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedConfig.Signers).To(BeEquivalentTo([]string{signer1.String(), signer2.String()}))
				Expect(feedConfig.Transmitters).To(BeEquivalentTo([]string{transmitter.String()}))
				Expect(feedConfig.F).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OffchainConfigVersion).To(BeEquivalentTo(uint32(1)))
				Expect(feedConfig.OffchainConfig).To(BeEquivalentTo([]byte("offchain_config")))
			})

			It("check transmissions count set", func() {
				feedCounts := app.OcrKeeper.GetFeedTransmissionCounts(ctx, feedId)
				Expect(len(feedCounts.Counts)).To(BeEquivalentTo(1))
				Expect(feedCounts.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedCounts.Counts[0].Address).To(BeEquivalentTo(transmitter.String()))
				Expect(feedCounts.Counts[0].Count).To(BeEquivalentTo(1))
			})

			It("check observations count set", func() {
				obsCounts := app.OcrKeeper.GetFeedObservationCounts(ctx, feedId)
				Expect(len(obsCounts.Counts)).To(BeEquivalentTo(1))
				Expect(obsCounts.FeedId).To(BeEquivalentTo(feedId))
				Expect(obsCounts.Counts[0].Address).To(BeEquivalentTo(transmitter.String()))
				Expect(obsCounts.Counts[0].Count).To(BeEquivalentTo(1))
			})
		})

		Context("UpdateFeed happy path test", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, types.Params{
					LinkDenom:           "link",
					PayoutBlockInterval: 2,
					ModuleAdmin:         moduleAdmin.String(),
				})
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String()},
					F:                     1,
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:       feedId,
						LinkDenom:    "link",
						FeedAdmin:    transmitter.String(),
						BillingAdmin: transmitter.String(),
					},
				}
				_, err = msgServer.CreateFeed(sdk.WrapSDKContext(ctx), &types.MsgCreateFeed{
					Sender: moduleAdmin.String(),
					Config: feedConfig,
				})
				Expect(err).To(BeNil())

				linkPerObservation := sdk.NewInt(2)
				linkPerTransmission := sdk.NewInt(2)
				_, err = msgServer.UpdateFeed(sdk.WrapSDKContext(ctx), &types.MsgUpdateFeed{
					Sender:              transmitter.String(),
					FeedId:              feedId,
					Signers:             []string{signer2.String()},
					Transmitters:        []string{signer1.String()},
					LinkPerObservation:  &linkPerObservation,
					LinkPerTransmission: &linkPerTransmission,
					LinkDenom:           "link2",
					FeedAdmin:           signer1.String(),
					BillingAdmin:        signer1.String(),
				})
				Expect(err).To(BeNil())
			})
			It("check feedConfig changes", func() {
				feedConfig = app.OcrKeeper.GetFeedConfig(ctx, feedId)
				Expect(feedConfig).To(Not(BeNil()))
				Expect(feedConfig.ModuleParams.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedConfig.Signers).To(BeEquivalentTo([]string{signer2.String()}))
				Expect(feedConfig.Transmitters).To(BeEquivalentTo([]string{signer1.String()}))
				Expect(feedConfig.ModuleParams.LinkPerObservation).To(BeEquivalentTo(sdk.NewInt(2)))
				Expect(feedConfig.ModuleParams.LinkPerTransmission).To(BeEquivalentTo(sdk.NewInt(2)))
				Expect(feedConfig.ModuleParams.LinkDenom).To(BeEquivalentTo("link2"))
				Expect(feedConfig.ModuleParams.FeedAdmin).To(BeEquivalentTo(signer1.String()))
				Expect(feedConfig.ModuleParams.BillingAdmin).To(BeEquivalentTo(signer1.String()))
			})

			It("check transmissions count set", func() {
				feedCounts := app.OcrKeeper.GetFeedTransmissionCounts(ctx, feedId)
				Expect(len(feedCounts.Counts)).To(BeEquivalentTo(1))
				Expect(feedCounts.FeedId).To(BeEquivalentTo(feedId))
				Expect(feedCounts.Counts[0].Address).To(BeEquivalentTo(signer1.String()))
				Expect(feedCounts.Counts[0].Count).To(BeEquivalentTo(1))
			})

			It("check observations count set", func() {
				obsCounts := app.OcrKeeper.GetFeedObservationCounts(ctx, feedId)
				Expect(len(obsCounts.Counts)).To(BeEquivalentTo(1))
				Expect(obsCounts.FeedId).To(BeEquivalentTo(feedId))
				Expect(obsCounts.Counts[0].Address).To(BeEquivalentTo(signer1.String()))
				Expect(obsCounts.Counts[0].Count).To(BeEquivalentTo(1))
			})
		})

		Context("SetPayees happy path test", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, types.Params{
					LinkDenom:           "link",
					PayoutBlockInterval: 2,
					ModuleAdmin:         moduleAdmin.String(),
				})
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String(), signer2.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:       feedId,
						LinkDenom:    "link",
						FeedAdmin:    transmitter.String(),
						BillingAdmin: transmitter.String(),
					},
				}
				_, err = msgServer.CreateFeed(sdk.WrapSDKContext(ctx), &types.MsgCreateFeed{
					Sender: moduleAdmin.String(),
					Config: feedConfig,
				})
				Expect(err).To(BeNil())

				_, err = msgServer.SetPayees(sdk.WrapSDKContext(ctx), &types.MsgSetPayees{
					Sender:       transmitter.String(),
					FeedId:       feedId,
					Transmitters: []string{signer1.String()},
					Payees:       []string{signer2.String()},
				})
				Expect(err).To(BeNil())
			})
			It("check payee changes", func() {
				payee := app.OcrKeeper.GetPayee(ctx, feedId, signer1)
				Expect(payee).To(Not(BeNil()))
				Expect(payee.String()).To(BeEquivalentTo(signer2.String()))
			})
			It("try second edition for payee", func() {
				_, err = msgServer.SetPayees(sdk.WrapSDKContext(ctx), &types.MsgSetPayees{
					Sender:       transmitter.String(),
					FeedId:       feedId,
					Transmitters: []string{signer1.String()},
					Payees:       []string{transmitter.String()},
				})
				Expect(err).To(Not(BeNil()))
			})
		})

		Context("TransferPayeeship & AcceptPayeeship happy path test", func() {
			// TransferPayeeship
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, types.Params{
					LinkDenom:           "link",
					PayoutBlockInterval: 2,
					ModuleAdmin:         moduleAdmin.String(),
				})
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String(), signer2.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:       feedId,
						LinkDenom:    "link",
						FeedAdmin:    transmitter.String(),
						BillingAdmin: transmitter.String(),
					},
				}
				_, err = msgServer.CreateFeed(sdk.WrapSDKContext(ctx), &types.MsgCreateFeed{
					Sender: moduleAdmin.String(),
					Config: feedConfig,
				})
				Expect(err).To(BeNil())

				_, err = msgServer.SetPayees(sdk.WrapSDKContext(ctx), &types.MsgSetPayees{
					Sender:       transmitter.String(),
					FeedId:       feedId,
					Transmitters: []string{signer1.String()},
					Payees:       []string{signer2.String()},
				})
				Expect(err).To(BeNil())

				_, err = msgServer.TransferPayeeship(sdk.WrapSDKContext(ctx), &types.MsgTransferPayeeship{
					Sender:      signer2.String(),
					FeedId:      feedId,
					Transmitter: signer1.String(),
					Proposed:    transmitter.String(),
				})
			})
			It("check pending payee transfer record", func() {
				pendingPayee := app.OcrKeeper.GetPendingPayeeshipTransfer(ctx, feedId, signer1)
				Expect(pendingPayee).To(Not(BeNil()))
				Expect(pendingPayee.String()).To(BeEquivalentTo(transmitter.String()))
			})
			It("check payee is not changed already", func() {
				payee := app.OcrKeeper.GetPayee(ctx, feedId, signer1)
				Expect(payee).To(Not(BeNil()))
				Expect(payee.String()).To(BeEquivalentTo(signer2.String()))
			})
			It("AcceptPayeeship and check changes", func() {
				_, err = msgServer.AcceptPayeeship(sdk.WrapSDKContext(ctx), &types.MsgAcceptPayeeship{
					FeedId:      feedId,
					Transmitter: signer1.String(),
					Payee:       transmitter.String(),
				})
				Expect(err).To(BeNil())
				payee := app.OcrKeeper.GetPayee(ctx, feedId, signer1)
				Expect(payee).To(Not(BeNil()))
				Expect(payee.String()).To(BeEquivalentTo(transmitter.String()))
			})
		})

		Context("FundFeedRewardPool & WithdrawFeedRewardPool happy path test", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, types.Params{
					LinkDenom:           "link",
					PayoutBlockInterval: 2,
					ModuleAdmin:         moduleAdmin.String(),
				})
				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String(), signer2.String()},
					F:                     1,
					OnchainConfig:         []byte("onchain_config"),
					OffchainConfigVersion: 1,
					OffchainConfig:        []byte("offchain_config"),
					ModuleParams: &types.ModuleParams{
						FeedId:       feedId,
						LinkDenom:    "link",
						FeedAdmin:    transmitter.String(),
						BillingAdmin: transmitter.String(),
					},
				}
				_, err = msgServer.CreateFeed(sdk.WrapSDKContext(ctx), &types.MsgCreateFeed{
					Sender: moduleAdmin.String(),
					Config: feedConfig,
				})
				Expect(err).To(BeNil())

				rewardCoin := sdk.NewInt64Coin(app.OcrKeeper.LinkDenom(ctx), 1000000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{rewardCoin})
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, moduleAdmin, sdk.Coins{rewardCoin})
				Expect(err).To(BeNil())
				_, err = msgServer.FundFeedRewardPool(sdk.WrapSDKContext(ctx), &types.MsgFundFeedRewardPool{
					Sender: moduleAdmin.String(),
					FeedId: feedId,
					Amount: rewardCoin,
				})
				Expect(err).To(BeNil())
			})

			It("withdraw should not success by module admin", func() {
				withdrawCoin := sdk.NewInt64Coin(app.OcrKeeper.LinkDenom(ctx), 1000000)
				_, err = msgServer.WithdrawFeedRewardPool(sdk.WrapSDKContext(ctx), &types.MsgWithdrawFeedRewardPool{
					Sender: moduleAdmin.String(),
					FeedId: feedId,
					Amount: withdrawCoin,
				})
				Expect(err).To(Not(BeNil()))
			})

			It("withdraw should success by feed admin", func() {
				withdrawCoin := sdk.NewInt64Coin(app.OcrKeeper.LinkDenom(ctx), 1000000)
				_, err = msgServer.WithdrawFeedRewardPool(sdk.WrapSDKContext(ctx), &types.MsgWithdrawFeedRewardPool{
					Sender: transmitter.String(),
					FeedId: feedId,
					Amount: withdrawCoin,
				})
				Expect(err).To(BeNil())
				adminBalance := app.BankKeeper.GetBalance(ctx, transmitter, app.OcrKeeper.LinkDenom(ctx))
				Expect(adminBalance.String()).To(BeEquivalentTo("1000000link"))
			})
		})
	})

	Describe("Reward payout test", func() {
		Context("ProcessRewardPayout test", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, types.Params{
					LinkDenom:           "link",
					PayoutBlockInterval: 2,
					ModuleAdmin:         moduleAdmin.String(),
				})

				feedConfig = &types.FeedConfig{
					Signers:               []string{signer1.String(), signer2.String()},
					Transmitters:          []string{transmitter.String(), moduleAdmin.String()},
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
						LinkDenom:           app.OcrKeeper.LinkDenom(ctx),
						FeedAdmin:           moduleAdmin.String(),
						BillingAdmin:        moduleAdmin.String(),
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
						Observations:          []sdk.Dec{sdk.NewDec(1), sdk.NewDec(1), sdk.NewDec(1), sdk.NewDec(1), sdk.NewDec(1)},
					},
					Signatures: [][]byte{{}},
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
				sig2, err := ethsecp256k1.Sign(sigData, AccPrivKeys[1].Bytes())
				Expect(err).To(BeNil())
				msg.Signatures = append([][]byte{}, sig1, sig2)

				fmt.Println("addresses check signer1", signer1.String())
				fmt.Println("addresses check signer2", signer2.String())
				fmt.Println("addresses check transmitter", transmitter.String())
				fmt.Println("addresses check moduleAdmin", moduleAdmin.String())

				for i := 0; i < 100; i++ {
					// nolint:all
					// TODO: mock transmission accurately rather than manual increase of observation and transmission counts
					// _, err = msgServer.Transmit(sdk.WrapSDKContext(ctx), msg)
					// Expect(err).To(BeNil())
					app.OcrKeeper.IncrementFeedObservationCount(ctx, msg.FeedId, signer1)
					app.OcrKeeper.IncrementFeedObservationCount(ctx, msg.FeedId, signer2)
					app.OcrKeeper.IncrementFeedObservationCount(ctx, msg.FeedId, transmitter)
					app.OcrKeeper.IncrementFeedTransmissionCount(ctx, msg.FeedId, transmitter)
				}
			})

			It("should be able to distribute based on transmission and observation count", func() {
				linkDenom := app.OcrKeeper.LinkDenom(ctx)
				transmitterPrevBalance := app.BankKeeper.GetBalance(ctx, transmitter, linkDenom)
				signer1PrevBalance := app.BankKeeper.GetBalance(ctx, signer1, linkDenom)
				signer2PrevBalance := app.BankKeeper.GetBalance(ctx, signer2, linkDenom)

				rewardCoin := sdk.NewInt64Coin(linkDenom, 1000000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{rewardCoin})
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, moduleAdmin, sdk.Coins{rewardCoin})
				Expect(err).To(BeNil())
				_, err = msgServer.FundFeedRewardPool(sdk.WrapSDKContext(ctx), &types.MsgFundFeedRewardPool{
					Sender: moduleAdmin.String(),
					FeedId: feedId,
					Amount: rewardCoin,
				})
				Expect(err).To(BeNil())
				app.OcrKeeper.ProcessRewardPayout(ctx, feedConfig)

				transmitterNextBalance := app.BankKeeper.GetBalance(ctx, transmitter, linkDenom)
				signer1NextBalance := app.BankKeeper.GetBalance(ctx, signer1, linkDenom)
				signer2NextBalance := app.BankKeeper.GetBalance(ctx, signer2, linkDenom)
				// observation fee + transmission fee
				Expect(transmitterNextBalance.Sub(transmitterPrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 3000)))
				// observation fee
				Expect(signer2NextBalance.Sub(signer2PrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 1000)))
				// observation fee
				Expect(signer1NextBalance.Sub(signer1PrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 1000)))

				// check transmission and observation count reset
				Expect(app.OcrKeeper.GetFeedTransmissionsCount(ctx, feedId, transmitter)).To(BeEquivalentTo(uint64(1)))
				Expect(app.OcrKeeper.GetFeedTransmissionsCount(ctx, feedId, signer1)).To(BeEquivalentTo(uint64(1)))
				Expect(app.OcrKeeper.GetFeedTransmissionsCount(ctx, feedId, signer2)).To(BeEquivalentTo(uint64(1)))
				Expect(app.OcrKeeper.GetFeedObservationsCount(ctx, feedId, transmitter)).To(BeEquivalentTo(uint64(1)))
				Expect(app.OcrKeeper.GetFeedObservationsCount(ctx, feedId, signer1)).To(BeEquivalentTo(uint64(1)))
				Expect(app.OcrKeeper.GetFeedObservationsCount(ctx, feedId, signer2)).To(BeEquivalentTo(uint64(1)))

				// try second payout without more transmission
				app.OcrKeeper.ProcessRewardPayout(ctx, feedConfig)

				transmitterNextBalance = app.BankKeeper.GetBalance(ctx, transmitter, linkDenom)
				signer1NextBalance = app.BankKeeper.GetBalance(ctx, signer1, linkDenom)
				signer2NextBalance = app.BankKeeper.GetBalance(ctx, signer2, linkDenom)
				// observation fee + transmission fee
				Expect(transmitterNextBalance.Sub(transmitterPrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 3030)))
				// observation fee
				Expect(signer2NextBalance.Sub(signer2PrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 1030)))
				// observation fee
				Expect(signer1NextBalance.Sub(signer1PrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 1030)))
			})

			It("should be able to distribute to payee if it is set for transmitter", func() {
				linkDenom := app.OcrKeeper.LinkDenom(ctx)
				transmitterPrevBalance := app.BankKeeper.GetBalance(ctx, transmitter, linkDenom)
				signer1PrevBalance := app.BankKeeper.GetBalance(ctx, signer1, linkDenom)
				signer2PrevBalance := app.BankKeeper.GetBalance(ctx, signer2, linkDenom)

				_, err = msgServer.SetPayees(sdk.WrapSDKContext(ctx), &types.MsgSetPayees{
					Sender:       moduleAdmin.String(),
					FeedId:       feedId,
					Transmitters: []string{transmitter.String()},
					Payees:       []string{signer2.String()},
				})
				Expect(err).To(BeNil())

				rewardCoin := sdk.NewInt64Coin(linkDenom, 1000000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{rewardCoin})
				Expect(err).To(BeNil())

				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, moduleAdmin, sdk.Coins{rewardCoin})
				Expect(err).To(BeNil())
				_, err = msgServer.FundFeedRewardPool(sdk.WrapSDKContext(ctx), &types.MsgFundFeedRewardPool{
					Sender: moduleAdmin.String(),
					FeedId: feedId,
					Amount: rewardCoin,
				})
				Expect(err).To(BeNil())
				app.OcrKeeper.ProcessRewardPayout(ctx, feedConfig)

				transmitterNextBalance := app.BankKeeper.GetBalance(ctx, transmitter, linkDenom)
				signer1NextBalance := app.BankKeeper.GetBalance(ctx, signer1, linkDenom)
				signer2NextBalance := app.BankKeeper.GetBalance(ctx, signer2, linkDenom)
				// observation fee
				Expect(transmitterNextBalance.Sub(transmitterPrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 1000)))
				// observation fee + transmission fee by being payee of transmitter
				Expect(signer2NextBalance.Sub(signer2PrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 3000)))
				// observation fee
				Expect(signer1NextBalance.Sub(signer1PrevBalance)).To(BeEquivalentTo(sdk.NewInt64Coin(linkDenom, 1000)))
			})
		})
	})
})
