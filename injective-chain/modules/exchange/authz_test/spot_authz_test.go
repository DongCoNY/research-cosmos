package authztest

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Spot Exchange Authz Tests", func() {
	var (
		app               *simapp.InjectiveApp
		testInput         testexchange.TestInput
		msgServer         exchangetypes.MsgServer
		grantee1          = testexchange.DefaultAddress
		grantee2          = "inj1fpmlw98jka5dc9cjrwurvutz87n87y45skvqkv"
		granter1          = "inj1dye2gg272p7hjqlsavdaacg8n55jsh8mk70hxt"
		validSubaccountID string

		SpotMarket      *exchangetypes.SpotMarket
		ctx             sdk.Context
		limitOrderSell  *exchangetypes.MsgCreateSpotLimitOrder
		limitOrderSell2 *exchangetypes.MsgCreateSpotLimitOrder
		limitOrderBuy   *exchangetypes.MsgCreateSpotLimitOrder
		limitOrderBuy2  *exchangetypes.MsgCreateSpotLimitOrder
	)
	BeforeEach(func() {
		subaccountIdHash, err := exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(granter1), 1)
		validSubaccountID = subaccountIdHash.String() // "0x6932a4215e507d7903f0eb1bdee1079d29285cfb000000000000000000000000"
		testexchange.OrFail(err)

		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 16, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		// deposit for subaccount
		deposit := &exchangetypes.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		testexchange.MintAndDeposit(app, ctx, validSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))

		_, err = app.ExchangeKeeper.SpotMarketLaunch(
			ctx,
			testInput.Spots[0].Ticker,
			testInput.Spots[0].BaseDenom,
			testInput.Spots[0].QuoteDenom,
			testInput.Spots[0].MinPriceTickSize,
			testInput.Spots[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)

		SpotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
		price := sdk.NewDec(int64(2000))
		quantity := sdk.NewDec(2)
		limitOrderSell = testInput.NewMsgCreateSpotLimitOrder(
			price,
			quantity,
			exchangetypes.OrderType_SELL,
			common.HexToHash(validSubaccountID),
		)
		limitOrderSell2 = testInput.NewMsgCreateSpotLimitOrder(
			price,
			quantity.Add(sdk.NewDec(1)),
			exchangetypes.OrderType_SELL,
			common.HexToHash(validSubaccountID),
		)
		limitOrderBuy = testInput.NewMsgCreateSpotLimitOrder(
			sdk.NewDec(int64(2000)),
			sdk.NewDec(1),
			exchangetypes.OrderType_BUY,
			common.HexToHash(validSubaccountID),
		)
		limitOrderBuy2 = testInput.NewMsgCreateSpotLimitOrder(
			sdk.NewDec(int64(2000)),
			sdk.NewDec(3),
			exchangetypes.OrderType_BUY,
			common.HexToHash(validSubaccountID),
		)
	})

	Describe("Test Create Spot Limit Order Authz", func() {
		It("should allow grantee1 to create Spot limit orders for granter1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateSpotLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{testInput.Spots[0].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// increase block height
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := *limitOrderBuy
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should let granter 1 revoke grant and grantee1 can't create limit orders", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateSpotLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{testInput.Spots[0].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// increase block height
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// then revoke it
			testexchange.Revoke(ctx, app.AuthzKeeper, granter1, grantee1, &exchangetypes.MsgCreateSpotLimitOrder{})

			// grantee1 try to exec Spot message for granter1
			msg := *limitOrderBuy
			msg.Sender = granter1
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not let grantee 2 create Spot limit orders for granter 1", func() {
			// grantee1 try to exec Spot message for granter1
			msg := *limitOrderBuy
			msg.Sender = granter1
			_, err := testexchange.Exec(ctx, app.AuthzKeeper, grantee2, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Describe("Test Create Spot Market Orders Authz", func() {
		It("should allow grantee1 to create Spot market orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateSpotMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{common.Bytes2Hex(testInput.Spots[0].MarketID.Bytes())},
				},
			)
			testexchange.OrFail(err)

			//post limit sell order to match with market order that will be submitted later
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// grantee1 try to exec Spot message for granter1
			msg := testInput.NewMsgCreateSpotMarketOrder(
				sdk.NewDec(1),
				sdk.NewDec(int64(2005)),
				exchangetypes.OrderType_BUY,
				common.HexToHash(validSubaccountID),
			)
			msg.Sender = granter1

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, msg)
			Expect(err).To(BeNil())
		})
		It("should let granter 1 revoke grant and grantee1 can't create market orders", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateSpotMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{common.Bytes2Hex(testInput.Spots[0].MarketID.Bytes())},
				},
			)
			testexchange.OrFail(err)

			// then revoke it
			_, err = testexchange.Revoke(ctx, app.AuthzKeeper, granter1, grantee1, &exchangetypes.MsgCreateSpotMarketOrder{})
			testexchange.OrFail(err)

			// grantee1 try to exec Spot message for granter1
			msg := testInput.NewMsgCreateSpotMarketOrder(
				sdk.NewDec(1),
				sdk.NewDec(int64(700)),
				exchangetypes.OrderType_BUY,
				common.HexToHash(validSubaccountID),
			)
			msg.Sender = granter1

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not let grantee 2 create Spot limit orders for granter 1", func() {
			// grantee1 try to exec Spot message for granter1
			msg := testInput.NewMsgCreateSpotMarketOrder(
				sdk.NewDec(1),
				sdk.NewDec(int64(2005)),
				exchangetypes.OrderType_BUY,
				common.HexToHash(validSubaccountID),
			)
			msg.Sender = granter1
			msg.Order.MarketId = SpotMarket.MarketId
			_, err := testexchange.Exec(ctx, app.AuthzKeeper, grantee2, msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Describe("Test Batch Create Spot Limit Order Authz", func() {
		It("should allow grantee1 to create Spot 2 limit orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{SpotMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchCreateSpotLimitOrders{
				Sender: granter1,
				Orders: []exchangetypes.SpotOrder{
					limitOrderBuy.Order,
					limitOrderBuy2.Order,
				},
			}
			msg.Sender = granter1

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should let granter 1 revoke grant and grantee1 can't create market orders", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{SpotMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// then revoke it
			_, err = testexchange.Revoke(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.MsgBatchCreateSpotLimitOrders{},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchCreateSpotLimitOrders{
				Sender: granter1,
				Orders: []exchangetypes.SpotOrder{
					limitOrderBuy.Order,
					limitOrderBuy2.Order,
				},
			}
			msg.Sender = granter1

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not allow grantee2 to create 2 Spot limit orders for granter1", func() {
			msg := exchangetypes.MsgBatchCreateSpotLimitOrders{
				Sender: granter1,
				Orders: []exchangetypes.SpotOrder{
					limitOrderBuy.Order,
					limitOrderBuy2.Order,
				},
			}
			msg.Sender = granter1
			_, err := testexchange.Exec(ctx, app.AuthzKeeper, grantee2, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Describe("Test Cancel Spot Limit Order Authz", func() {
		It("should allow grantee1 to cancel 1 Spot limit orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CancelSpotOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{SpotMarket.MarketId[2:]},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgCancelSpotOrder{
				Sender:       granter1,
				MarketId:     SpotMarket.MarketId[2:],
				SubaccountId: validSubaccountID,
				OrderHash:    resp.OrderHash,
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to cancel 1 Spot limit orders for granter1 once granter1 revokes it", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CancelSpotOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{SpotMarket.MarketId[2:]},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// then revoke it
			_, err = testexchange.Revoke(ctx, app.AuthzKeeper, granter1, grantee1, &exchangetypes.MsgCancelSpotOrder{})
			testexchange.OrFail(err)
			msg := exchangetypes.MsgCancelSpotOrder{
				Sender:       granter1,
				MarketId:     SpotMarket.MarketId[2:],
				SubaccountId: validSubaccountID,
				OrderHash:    resp.OrderHash,
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not allow grantee2 to cancel 1 Spot limit orders", func() {
			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgCancelSpotOrder{
				Sender:       granter1,
				MarketId:     SpotMarket.MarketId[2:],
				SubaccountId: validSubaccountID,
				OrderHash:    resp.OrderHash,
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Describe("Test Batch Cancel Spot Limit Order Authz", func() {
		It("should allow grantee1 to cancel 2 Spot limit orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCancelSpotOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{SpotMarket.MarketId[2:]},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			resp2, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			msg := exchangetypes.MsgBatchCancelSpotOrders{
				Sender: granter1,
				Data: []exchangetypes.OrderData{
					{
						MarketId:     SpotMarket.MarketId[2:],
						SubaccountId: validSubaccountID,
						OrderHash:    resp.OrderHash,
					},
					{
						MarketId:     SpotMarket.MarketId[2:],
						SubaccountId: validSubaccountID,
						OrderHash:    resp2.OrderHash,
					},
				},
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to cancel 2 Spot orders for granter1 since granter1 revokes permission", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCancelSpotOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{SpotMarket.MarketId[2:]},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			resp2, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			// then revoke it
			_, err = app.AuthzKeeper.Revoke(sdk.WrapSDKContext(ctx), &authz.MsgRevoke{
				Granter:    granter1,
				Grantee:    grantee1,
				MsgTypeUrl: sdk.MsgTypeURL(&exchangetypes.MsgBatchCancelSpotOrders{}),
			})
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchCancelSpotOrders{
				Sender: granter1,
				Data: []exchangetypes.OrderData{
					{
						MarketId:     SpotMarket.MarketId[2:],
						SubaccountId: validSubaccountID,
						OrderHash:    resp.OrderHash,
					},
					{
						MarketId:     SpotMarket.MarketId[2:],
						SubaccountId: validSubaccountID,
						OrderHash:    resp2.OrderHash,
					},
				},
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not allow grantee2 to cancel 2 Spot orders for granter1 since grantee2 doesnot have permission", func() {
			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			resp2, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			msg := exchangetypes.MsgBatchCancelSpotOrders{
				Sender: granter1,
				Data: []exchangetypes.OrderData{
					{
						MarketId:     SpotMarket.MarketId[2:],
						SubaccountId: validSubaccountID,
						OrderHash:    resp.OrderHash,
					},
					{
						MarketId:     SpotMarket.MarketId[2:],
						SubaccountId: validSubaccountID,
						OrderHash:    resp2.OrderHash,
					},
				},
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})
})
