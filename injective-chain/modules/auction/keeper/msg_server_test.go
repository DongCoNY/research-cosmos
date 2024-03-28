package keeper_test

import (
	"context"
	"fmt"
	"time"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/auction"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MsgServer Tests", func() {
	var (
		app               *simapp.InjectiveApp
		msgServer         types.MsgServer
		ctx               sdk.Context
		goCtx             context.Context
		coinBasket        sdk.Coins
		users             []sdk.AccAddress
		auctionModuleAddr sdk.AccAddress

		acc1, _        = sdk.AccAddressFromBech32("inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff")
		acc2, _        = sdk.AccAddressFromBech32("inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43")
		BidderAccAddrs = []sdk.AccAddress{acc1, acc2}

		// TestingAuctionParams is a set of auction params for testing
		TestingAuctionParams = types.Params{
			AuctionPeriod:           5,
			MinNextBidIncrementRate: sdk.NewDecWithPrec(25, 4),
		}
	)

	var _ = BeforeSuite(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Now()})
		msgServer = keeper.NewMsgServerImpl(app.AuctionKeeper)
		goCtx = sdk.WrapSDKContext(ctx)
		auctionModuleAddr = app.AccountKeeper.GetModuleAddress(types.ModuleName)

		// init auction state
		app.AuctionKeeper.SetParams(ctx, TestingAuctionParams)
		app.AuctionKeeper.InitEndingTimeStamp(ctx)
		app.AuctionKeeper.DeleteBid(ctx)
		app.AuctionKeeper.SetAuctionRound(ctx, 0)

		// init total supply
		coinBasket = sdk.NewCoins(
			sdk.NewCoin("bnb", sdk.NewInt(79)),
			sdk.NewCoin("cax", sdk.NewInt(245)),
			sdk.NewCoin("ecc", sdk.NewInt(137)),
		)

		// mint coin basket directly for auction module
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
		app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, types.ModuleName, coinBasket)

		// mint coin basket for exchange module
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
		app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

		// set deposits for AuctionSubaccountID
		testexchange.MintAndDeposit(app, ctx, exchangetypes.AuctionSubaccountID.String(), coinBasket)

		// mint some inj and send to users
		app.BankKeeper.MintCoins(
			ctx,
			minttypes.ModuleName,
			sdk.NewCoins(sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(1000000000000))),
		)
		users = BidderAccAddrs[:2]
		for _, user := range users {
			amount := sdk.NewCoins(sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(100000000)))
			acc := app.AccountKeeper.NewAccountWithAddress(ctx, user)
			app.AccountKeeper.SetAccount(ctx, acc)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, user, amount)
		}

		// fast forward 1 blocks
		ctx = EndBlockerAndCommit(app, ctx, 1)
		goCtx = sdk.WrapSDKContext(ctx)
	})

	Describe("Auction module test", func() {

		Context("Place bid with invalid denom", func() {
			It("Should fail", func() {
				user := BidderAccAddrs[0]
				invalidBidDenom := "inv"
				bidMsg := types.MsgBid{
					Sender:    user.String(),
					BidAmount: sdk.NewCoin(invalidBidDenom, sdk.NewInt(1000)),
					Round:     1,
				}
				err := bidMsg.ValidateBasic()
				Expect(err).ToNot(BeNil())
			})
		})

		Context("Place bid with invalid auction round", func() {
			It("Should fail", func() {
				user := BidderAccAddrs[0]
				bidMsg := &types.MsgBid{
					Sender:    user.String(),
					BidAmount: sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(1000)),
					Round:     1,
				}
				_, err := msgServer.Bid(goCtx, bidMsg)
				Expect(err).ToNot(BeNil())
			})
		})

		Context("Place valid bids", func() {
			It("No bids with valid coin basket at height = 2", func() {
				// assert initial bidder balances
				coins := app.BankKeeper.GetAllBalances(ctx, BidderAccAddrs[0])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(100000000)))

				coins = app.BankKeeper.GetAllBalances(ctx, BidderAccAddrs[1])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(100000000)))

				// assert auction subaccount balances
				for _, coin := range coinBasket {
					deposit := testexchange.GetBankAndDepositFunds(app, ctx, exchangetypes.AuctionSubaccountID, coin.Denom)
					Expect(deposit.TotalBalance).To(BeEquivalentTo(sdk.NewDec(coin.Amount.Int64())))
				}

				// assert auction module balance
				coins = app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(3))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(79)))  // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(245))) // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(137))) // eec

				// dump auction basket info
				currentBasket, _ := app.AuctionKeeper.CurrentAuctionBasket(goCtx, &types.QueryCurrentAuctionBasketRequest{})
				fmt.Println(ctx.BlockHeight(), currentBasket)

				// fast forward 5 blocks
				ctx = EndBlockerAndCommit(app, ctx, 5)
				goCtx = sdk.WrapSDKContext(ctx)
			})

			It("Valid bids with valid coin basket at height = 7", func() {
				bidMsg1 := types.MsgBid{
					Sender:    users[0].String(),
					BidAmount: sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(1000)),
					Round:     1,
				}
				bidMsg2 := types.MsgBid{
					Sender:    users[1].String(),
					BidAmount: sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(1379)),
					Round:     1,
				}

				// assert auction subaccount balances
				for _, coin := range coinBasket {
					deposit := testexchange.GetBankAndDepositFunds(app, ctx, exchangetypes.AuctionSubaccountID, coin.Denom)
					Expect(deposit.TotalBalance).To(BeEquivalentTo(sdk.NewDec(0)))
				}

				// assert auction module balances
				coins := app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(3))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(158))) // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(490))) // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(274))) // eec

				// don't mint coin basket for next auction round
				//app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
				//app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

				// send 1st bid
				msgServer.Bid(goCtx, &bidMsg1)

				// assert users balance and highestBid
				coins = app.BankKeeper.GetAllBalances(ctx, users[0])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(99999000)))

				coins = app.BankKeeper.GetAllBalances(ctx, users[1])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(100000000)))

				highestBid := app.AuctionKeeper.GetHighestBid(ctx)
				Expect(highestBid.Bidder).To(BeEquivalentTo(users[0].String()))
				Expect(highestBid.Amount.Amount).To(BeEquivalentTo(sdk.NewInt(1000)))

				coins = app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(4))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(158)))  // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(490)))  // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(274)))  // eec
				Expect(coins[3].Amount).To(BeEquivalentTo(sdk.NewInt(1000))) //inj

				// send 2nd bid
				msgServer.Bid(goCtx, &bidMsg2)

				// assert users balance and highestBid
				coins = app.BankKeeper.GetAllBalances(ctx, users[0])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(100000000)))

				coins = app.BankKeeper.GetAllBalances(ctx, users[1])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(99998621)))

				highestBid = app.AuctionKeeper.GetHighestBid(ctx)
				Expect(highestBid.Bidder).To(BeEquivalentTo(users[1].String()))
				Expect(highestBid.Amount.Amount).To(BeEquivalentTo(sdk.NewInt(1379)))

				coins = app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(4))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(158)))  // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(490)))  // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(274)))  // eec
				Expect(coins[3].Amount).To(BeEquivalentTo(sdk.NewInt(1379))) //inj

				// dump auction basket info
				currentBasket, _ := app.AuctionKeeper.CurrentAuctionBasket(goCtx, &types.QueryCurrentAuctionBasketRequest{})
				fmt.Println(ctx.BlockHeight(), currentBasket)
			})

			It("Assert state at height = 7", func() {

				genesis := auction.ExportGenesis(ctx, app.AuctionKeeper)
				Expect(*genesis).To(BeEquivalentTo(types.GenesisState{
					Params: types.Params{
						AuctionPeriod:           5,
						MinNextBidIncrementRate: sdk.NewDecWithPrec(25, 4),
					},
					AuctionRound: 1,
					HighestBid: &types.Bid{
						Bidder: users[1].String(),
						Amount: chaintypes.NewInjectiveCoin(sdk.NewInt(1379)),
					},
					AuctionEndingTimestamp: app.AuctionKeeper.GetEndingTimeStamp(ctx),
				}))

				// fast forward 5 blocks
				ctx = EndBlockerAndCommit(app, ctx, 5)
				goCtx = sdk.WrapSDKContext(ctx)
			})

			It("Valid bids with empty coin basket at height = 12", func() {
				bidMsg1 := types.MsgBid{
					Sender:    users[0].String(),
					BidAmount: sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(8000000)),
					Round:     2,
				}
				bidMsg2 := types.MsgBid{
					Sender:    users[1].String(),
					BidAmount: sdk.NewCoin(chaintypes.InjectiveCoin, sdk.NewInt(200)),
					Round:     2,
				}

				// send 1st bid
				_, err := msgServer.Bid(goCtx, &bidMsg1)
				Expect(err).To(BeNil())

				// assert users balance, module balance and highestBid
				coins := app.BankKeeper.GetAllBalances(ctx, users[0])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(92000000)))

				coins = app.BankKeeper.GetAllBalances(ctx, users[1])
				Expect(len(coins)).To(BeEquivalentTo(4))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(158)))      // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(490)))      // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(274)))      // eec
				Expect(coins[3].Amount).To(BeEquivalentTo(sdk.NewInt(99998621))) //inj

				highestBid := app.AuctionKeeper.GetHighestBid(ctx)
				Expect(highestBid.Bidder).To(BeEquivalentTo(users[0].String()))
				Expect(highestBid.Amount.Amount).To(BeEquivalentTo(sdk.NewInt(8000000)))

				coins = app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(8000000))) //inj

				// send 2nd bid with less than highest bid
				_, err = msgServer.Bid(goCtx, &bidMsg2)
				Expect(err).ToNot(BeNil())

				//assert users balance, module balance and highestBid
				coins = app.BankKeeper.GetAllBalances(ctx, users[0])
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(92000000)))

				coins = app.BankKeeper.GetAllBalances(ctx, users[1])
				Expect(len(coins)).To(BeEquivalentTo(4))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(158)))      // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(490)))      // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(274)))      // eec
				Expect(coins[3].Amount).To(BeEquivalentTo(sdk.NewInt(99998621))) //inj

				highestBid = app.AuctionKeeper.GetHighestBid(ctx)
				Expect(highestBid.Bidder).To(BeEquivalentTo(users[0].String()))

				coins = app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(1))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(8000000))) //inj

				currentBasket, _ := app.AuctionKeeper.CurrentAuctionBasket(goCtx, &types.QueryCurrentAuctionBasketRequest{})
				fmt.Println(ctx.BlockHeight(), currentBasket)
			})

			It("Try placing bid with less than min increment percentage", func() {
				lastBid := app.AuctionKeeper.GetHighestBid(ctx)
				bidMsg := &types.MsgBid{
					Sender:    users[1].String(),
					BidAmount: lastBid.Amount.Add(chaintypes.NewInjectiveCoin(sdk.NewInt(1))),
					Round:     2,
				}
				_, err := msgServer.Bid(goCtx, bidMsg)
				Expect(err).ToNot(BeNil())
			})

			It("Assert balances after settlement at height = 17", func() {
				// fast forward 5 blocks
				ctx = EndBlockerAndCommit(app, ctx, 5)

				coins := app.BankKeeper.GetAllBalances(ctx, users[0])
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(92000000)))

				coins = app.BankKeeper.GetAllBalances(ctx, users[1])
				Expect(len(coins)).To(BeEquivalentTo(4))
				Expect(coins[0].Amount).To(BeEquivalentTo(sdk.NewInt(158)))      // bnb
				Expect(coins[1].Amount).To(BeEquivalentTo(sdk.NewInt(490)))      // cac
				Expect(coins[2].Amount).To(BeEquivalentTo(sdk.NewInt(274)))      // eec
				Expect(coins[3].Amount).To(BeEquivalentTo(sdk.NewInt(99998621))) //inj

				coins = app.BankKeeper.GetAllBalances(ctx, auctionModuleAddr)
				Expect(len(coins)).To(BeEquivalentTo(0))

				currentBasket, _ := app.AuctionKeeper.CurrentAuctionBasket(goCtx, &types.QueryCurrentAuctionBasketRequest{})
				fmt.Println(ctx.BlockHeight(), currentBasket)
			})

			Describe("Module genesis tests", func() {

				Context("Assert module state", func() {
					It("Should pass", func() {
						genesis := auction.ExportGenesis(ctx, app.AuctionKeeper)
						Expect(*genesis).To(BeEquivalentTo(types.GenesisState{
							Params: types.Params{
								AuctionPeriod:           5,
								MinNextBidIncrementRate: sdk.NewDecWithPrec(25, 4),
							},
							AuctionRound: 3,
							HighestBid: &types.Bid{
								Bidder: "",
								Amount: chaintypes.NewInjectiveCoin(sdk.NewInt(0)),
							},
							AuctionEndingTimestamp: app.AuctionKeeper.GetEndingTimeStamp(ctx),
						}))
					})
				})
			})
		})
	})
})
