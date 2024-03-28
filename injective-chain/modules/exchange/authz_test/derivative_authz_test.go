package authztest

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Derivative Exchange Authz Tests", func() {
	var (
		app               *simapp.InjectiveApp
		testInput         testexchange.TestInput
		msgServer         exchangetypes.MsgServer
		grantee1          = testexchange.DefaultAddress
		grantee2          = "inj1fpmlw98jka5dc9cjrwurvutz87n87y45skvqkv"
		granter1          = "inj1dye2gg272p7hjqlsavdaacg8n55jsh8mk70hxt"
		validSubaccountID string

		derivativeMarket *exchangetypes.DerivativeMarket
		ctx              sdk.Context
		limitOrderSell   *exchangetypes.MsgCreateDerivativeLimitOrder
		limitOrderSell2  *exchangetypes.MsgCreateDerivativeLimitOrder
		limitOrderBuy    *exchangetypes.MsgCreateDerivativeLimitOrder
		limitOrderBuy2   *exchangetypes.MsgCreateDerivativeLimitOrder
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
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
		// init oracle price
		startingPrice := sdk.NewDec(2000)
		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		// init insurance fund
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		initialInsuranceFundBalance := sdk.NewDec(44)
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		// deposit for subaccount
		deposit := &exchangetypes.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		testexchange.MintAndDeposit(app, ctx, validSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

		_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			testInput.Perps[0].Ticker,
			testInput.Perps[0].QuoteDenom,
			oracleBase, oracleQuote,
			0,
			oracleType,
			testInput.Perps[0].InitialMarginRatio,
			testInput.Perps[0].MaintenanceMarginRatio,
			testInput.Perps[0].MakerFeeRate,
			testInput.Perps[0].TakerFeeRate,
			testInput.Perps[0].MinPriceTickSize,
			testInput.Perps[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)

		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)
		price := sdk.NewDec(int64(2000))
		quantity := sdk.NewDec(2)
		margin := sdk.NewDec(int64(1500))
		limitOrderSell = testInput.NewMsgCreateDerivativeLimitOrder(
			price,
			quantity,
			margin,
			exchangetypes.OrderType_SELL,
			common.HexToHash(validSubaccountID),
		)
		limitOrderSell2 = testInput.NewMsgCreateDerivativeLimitOrder(
			price,
			quantity.Add(sdk.NewDec(1)),
			margin,
			exchangetypes.OrderType_SELL,
			common.HexToHash(validSubaccountID),
		)
		limitOrderBuy = testInput.NewMsgCreateDerivativeLimitOrder(
			sdk.NewDec(int64(2000)),
			sdk.NewDec(1),
			sdk.NewDec(int64(1500)),
			exchangetypes.OrderType_BUY,
			common.HexToHash(validSubaccountID),
		)
		limitOrderBuy2 = testInput.NewMsgCreateDerivativeLimitOrder(
			sdk.NewDec(int64(2000)),
			sdk.NewDec(3),
			sdk.NewDec(int64(1700)),
			exchangetypes.OrderType_BUY,
			common.HexToHash(validSubaccountID),
		)
	})

	Context("Test Create Derivative Limit Order Authz", func() {
		It("should allow grantee1 to create derivative limit orders for granter1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
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
				&exchangetypes.CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// increase block height
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// then revoke it
			testexchange.Revoke(ctx, app.AuthzKeeper, granter1, grantee1, &exchangetypes.MsgCreateDerivativeLimitOrder{})

			// grantee1 try to exec derivative message for granter1
			msg := *limitOrderBuy
			msg.Sender = granter1
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not let grantee 2 create derivative limit orders for granter 1", func() {
			// grantee1 try to exec derivative message for granter1
			msg := *limitOrderBuy
			msg.Sender = granter1
			_, err := testexchange.Exec(ctx, app.AuthzKeeper, grantee2, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Context("Test Create Derivative Market Orders Authz", func() {
		It("should allow grantee1 to create derivative market orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// grantee1 try to exec derivative message for granter1
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// grantee1 try to exec derivative message for granter1
			msg := testInput.NewMsgCreateDerivativeMarketOrder(
				sdk.NewDec(1),
				sdk.NewDec(int64(700)),
				sdk.NewDec(int64(2005)),
				exchangetypes.OrderType_BUY,
				common.HexToHash(validSubaccountID),
			)
			msg.Sender = granter1
			msg.Order.MarketId = derivativeMarket.MarketId

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, msg)
			Expect(err).To(BeNil())
		})
		It("should let granter 1 revoke grant and grantee1 can't create market orders", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// then revoke it
			_, err = testexchange.Revoke(ctx, app.AuthzKeeper, granter1, grantee1, &exchangetypes.MsgCreateDerivativeMarketOrder{})
			testexchange.OrFail(err)

			// grantee1 try to exec derivative message for granter1
			msg := testInput.NewMsgCreateDerivativeMarketOrder(
				sdk.NewDec(1),
				sdk.NewDec(int64(700)),
				sdk.NewDec(int64(2005)),
				exchangetypes.OrderType_BUY,
				common.HexToHash(validSubaccountID),
			)
			msg.Sender = granter1
			msg.Order.MarketId = derivativeMarket.MarketId

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not let grantee 2 create derivative limit orders for granter 1", func() {
			// grantee1 try to exec derivative message for granter1
			msg := testInput.NewMsgCreateDerivativeMarketOrder(
				sdk.NewDec(1),
				sdk.NewDec(int64(700)),
				sdk.NewDec(int64(2005)),
				exchangetypes.OrderType_BUY,
				common.HexToHash(validSubaccountID),
			)
			msg.Sender = granter1
			msg.Order.MarketId = derivativeMarket.MarketId
			_, err := testexchange.Exec(ctx, app.AuthzKeeper, grantee2, msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Context("Test Batch Create Derivative Limit Order Authz", func() {
		It("should allow grantee1 to create derivative 2 limit orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchCreateDerivativeLimitOrders{
				Sender: granter1,
				Orders: []exchangetypes.DerivativeOrder{
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
				&exchangetypes.BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// then revoke it
			_, err = testexchange.Revoke(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.MsgBatchCreateDerivativeLimitOrders{},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchCreateDerivativeLimitOrders{
				Sender: granter1,
				Orders: []exchangetypes.DerivativeOrder{
					limitOrderBuy.Order,
					limitOrderBuy2.Order,
				},
			}
			msg.Sender = granter1

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not allow grantee2 to create 2 derivative limit orders for granter1", func() {
			msg := exchangetypes.MsgBatchCreateDerivativeLimitOrders{
				Sender: granter1,
				Orders: []exchangetypes.DerivativeOrder{
					limitOrderBuy.Order,
					limitOrderBuy2.Order,
				},
			}
			msg.Sender = granter1
			_, err := testexchange.Exec(ctx, app.AuthzKeeper, grantee2, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Context("Test Cancel Derivative Limit Order Authz", func() {
		It("should allow grantee1 to cancel 1 derivative limit orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			msg := exchangetypes.MsgCancelDerivativeOrder{
				Sender:       granter1,
				MarketId:     derivativeMarket.MarketId,
				SubaccountId: validSubaccountID,
				OrderHash:    resp.OrderHash,
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to cancel 1 derivative limit orders for granter1 once granter1 revokes it", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// then revoke it
			_, err = testexchange.Revoke(ctx, app.AuthzKeeper, granter1, grantee1, &exchangetypes.MsgCancelDerivativeOrder{})
			testexchange.OrFail(err)
			msg := exchangetypes.MsgCancelDerivativeOrder{
				Sender:       granter1,
				MarketId:     derivativeMarket.MarketId,
				SubaccountId: validSubaccountID,
				OrderHash:    resp.OrderHash,
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not allow grantee2 to cancel 1 derivative limit orders", func() {
			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgCancelDerivativeOrder{
				Sender:       granter1,
				MarketId:     derivativeMarket.MarketId,
				SubaccountId: validSubaccountID,
				OrderHash:    resp.OrderHash,
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
	})

	Context("Test Batch Cancel Derivative Limit Order Authz", func() {
		It("should allow grantee1 to cancel 2 derivative limit orders for granter1", func() {
			// granter1 gives grantee1 permission
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			resp2, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			msg := exchangetypes.MsgBatchCancelDerivativeOrders{
				Sender: granter1,
				Data: []exchangetypes.OrderData{
					{
						MarketId:     derivativeMarket.MarketId,
						SubaccountId: validSubaccountID,
						OrderHash:    resp.OrderHash,
					},
					{
						MarketId:     derivativeMarket.MarketId,
						SubaccountId: validSubaccountID,
						OrderHash:    resp2.OrderHash,
					},
				},
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to cancel 2 derivative orders for granter1 since granter1 revokes permission", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			resp2, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			// then revoke it
			_, err = app.AuthzKeeper.Revoke(sdk.WrapSDKContext(ctx), &authz.MsgRevoke{
				Granter:    granter1,
				Grantee:    grantee1,
				MsgTypeUrl: sdk.MsgTypeURL(&exchangetypes.MsgBatchCancelDerivativeOrders{}),
			})
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchCancelDerivativeOrders{
				Sender: granter1,
				Data: []exchangetypes.OrderData{
					{
						MarketId:     derivativeMarket.MarketId,
						SubaccountId: validSubaccountID,
						OrderHash:    resp.OrderHash,
					},
					{
						MarketId:     derivativeMarket.MarketId,
						SubaccountId: validSubaccountID,
						OrderHash:    resp2.OrderHash,
					},
				},
			}

			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should not allow grantee2 to cancel 2 derivative orders for granter1 since grantee2 doesnot have permission", func() {
			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell)
			testexchange.OrFail(err)

			resp2, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitOrderSell2)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			msg := exchangetypes.MsgBatchCancelDerivativeOrders{
				Sender: granter1,
				Data: []exchangetypes.OrderData{
					{
						MarketId:     derivativeMarket.MarketId,
						SubaccountId: validSubaccountID,
						OrderHash:    resp.OrderHash,
					},
					{
						MarketId:     derivativeMarket.MarketId,
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
