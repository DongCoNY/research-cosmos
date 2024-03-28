package authztest

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

var _ = Describe("Exchange Authz Scenarios Tests", func() {
	var (
		app               *simapp.InjectiveApp
		testInput         testexchange.TestInput
		msgServer         exchangetypes.MsgServer
		grantee1          = testexchange.DefaultAddress
		granter1          = "inj1dye2gg272p7hjqlsavdaacg8n55jsh8mk70hxt"
		validSubaccountID string

		derivativeMarket *exchangetypes.DerivativeMarket
		spotMarket       *exchangetypes.SpotMarket
		ctx              sdk.Context
		derivLimitOrders []*exchangetypes.MsgCreateDerivativeLimitOrder
		spotOrders       []*exchangetypes.MsgCreateSpotLimitOrder
	)
	BeforeEach(func() {
		subaccountIdHash, err := exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(granter1), 1)
		validSubaccountID = subaccountIdHash.String() // "0x6932a4215e507d7903f0eb1bdee1079d29285cfb000000000000000000000001"
		testexchange.OrFail(err)

		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 16, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 2, 2, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		// init oracle price
		startingPrice := sdk.NewDec(2000)
		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		// init insurance fund
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		initialInsuranceFundBalance := sdk.NewDec(44)
		for i := 0; i < 2; i++ {
			coin := sdk.NewCoin(testInput.Perps[i].QuoteDenom, initialInsuranceFundBalance.RoundInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[i].Ticker, testInput.Perps[i].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

			// deposit for subaccount
			deposit := &exchangetypes.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}
			testexchange.MintAndDeposit(app, ctx, validSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[i].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			_, _, err := app.ExchangeKeeper.PerpetualMarketLaunch(
				ctx,
				testInput.Perps[i].Ticker,
				testInput.Perps[i].QuoteDenom,
				oracleBase, oracleQuote,
				0,
				oracleType,
				testInput.Perps[i].InitialMarginRatio,
				testInput.Perps[i].MaintenanceMarginRatio,
				testInput.Perps[i].MakerFeeRate,
				testInput.Perps[i].TakerFeeRate,
				testInput.Perps[i].MinPriceTickSize,
				testInput.Perps[i].MinQuantityTickSize,
			)
			testexchange.OrFail(err)

			_, err = app.ExchangeKeeper.SpotMarketLaunch(
				ctx,
				testInput.Spots[i].Ticker,
				testInput.Spots[i].BaseDenom,
				testInput.Spots[i].QuoteDenom,
				testInput.Spots[i].MinPriceTickSize,
				testInput.Spots[i].MinQuantityTickSize,
			)
			testexchange.OrFail(err)
		}

		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)
		spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
		spotMarket.MarketId = spotMarket.MarketId

		price := sdk.NewDec(int64(2000))
		quantity := sdk.NewDec(2)
		margin := sdk.NewDec(int64(1500))
		for i := 0; i < 4; i++ {
			orderType := exchangetypes.OrderType_SELL
			if i%2 != 0 {
				orderType = exchangetypes.OrderType_BUY
			}
			derivLimitOrders = append(derivLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(
				price,
				quantity.Add(sdk.NewDec(int64(i))),
				margin,
				orderType,
				common.HexToHash(validSubaccountID),
			))
			spotOrders = append(spotOrders, testInput.NewMsgCreateSpotLimitOrder(
				price,
				quantity.Add(sdk.NewDec(int64(i))),
				orderType,
				common.HexToHash(validSubaccountID),
			))
		}
	})

	Context("Test with derivative markets", func() {
		It("should allow grantee1 to create orders for derivative market 0", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				DerivativeOrdersToCreate: []*exchangetypes.DerivativeOrder{
					&derivLimitOrders[0].Order,
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to create orders for granter1, derivative market 0", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				DerivativeOrdersToCreate: []*exchangetypes.DerivativeOrder{
					&derivLimitOrders[0].Order,
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to create orders for derivative market 0, since granter grants market 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{testInput.Perps[1].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)
			// increase block height
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// revoke grant
			_, err = testexchange.Revoke(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.MsgBatchUpdateOrders{},
			)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				DerivativeOrdersToCreate: []*exchangetypes.DerivativeOrder{
					&derivLimitOrders[0].Order,
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should allow grantee1 to cancel derivative market orders of market 0 for granter 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{derivativeMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// create order
			createDerivOrderMsg := *derivLimitOrders[1]
			createDerivOrderMsg.Sender = granter1
			createDerivOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &createDerivOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// cancel order of market 0 should be succeeded
			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				DerivativeOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     createDerivOrderMsg.Order.MarketId,
						SubaccountId: createDerivOrderMsg.Order.OrderInfo.SubaccountId,
						OrderHash:    resp.OrderHash,
					},
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})

		It("should not allow grantee1 to orders in list for granter 1, since granter 1 grants market 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{testInput.Perps[1].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// create order
			createDerivOrderMsg := *derivLimitOrders[1]
			createDerivOrderMsg.Sender = granter1
			createDerivOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			resp, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &createDerivOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// cancel order of market 0 should NOT be succeeded
			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender:       granter1,
				SubaccountId: validSubaccountID,
				DerivativeOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     createDerivOrderMsg.Order.MarketId,
						SubaccountId: createDerivOrderMsg.Order.OrderInfo.SubaccountId,
						OrderHash:    resp.OrderHash,
					},
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(errors.Is(sdkerrors.ErrUnauthorized, err)).To(BeTrue())
		})
		It("should allow grantee1 to all orders of market 0 for granter 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{testInput.Perps[0].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// create order
			createDerivOrderMsg := *derivLimitOrders[1]
			createDerivOrderMsg.Sender = granter1
			createDerivOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &createDerivOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender:                         granter1,
				SubaccountId:                   validSubaccountID,
				DerivativeMarketIdsToCancelAll: []string{testInput.Perps[0].MarketID.Hex()},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to cancel all orders of market 1 for granter 1, since grantee1 has no grant for this market", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId:      validSubaccountID,
					DerivativeMarkets: []string{testInput.Perps[1].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// create order
			createDerivOrderMsg := *derivLimitOrders[1]
			createDerivOrderMsg.Sender = granter1
			createDerivOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), &createDerivOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// try to cancel all orders of market
			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender:                         granter1,
				SubaccountId:                   validSubaccountID,
				DerivativeMarketIdsToCancelAll: []string{testInput.Perps[0].MarketID.Hex()},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(errors.Is(sdkerrors.ErrUnauthorized, err)).To(BeTrue())
		})
	})

	Context("Test with spot markets", func() {
		It("should allow grantee1 to create orders for Spot market 0", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{spotMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				SpotOrdersToCreate: []*exchangetypes.SpotOrder{
					&spotOrders[0].Order,
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to create orders for granter1, Spot market 0", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{spotMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				SpotOrdersToCreate: []*exchangetypes.SpotOrder{
					&spotOrders[0].Order,
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to create orders for Spot market 0, since granter grants market 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{testInput.Perps[1].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)
			// increase block height
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// revoke grant
			_, err = testexchange.Revoke(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.MsgBatchUpdateOrders{},
			)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				SpotOrdersToCreate: []*exchangetypes.SpotOrder{
					&spotOrders[0].Order,
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).Should(MatchError(authz.ErrNoAuthorizationFound))
		})
		It("should allow grantee1 to cancel Spot market orders of market 0 for granter 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{spotMarket.MarketId},
				},
			)
			testexchange.OrFail(err)

			// create order
			createSpotOrderMsg := *spotOrders[1]
			createSpotOrderMsg.Sender = granter1
			createSpotOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), &createSpotOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// cancel order of market 0 should be succeeded
			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender: granter1,
				SpotOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     createSpotOrderMsg.Order.MarketId,
						SubaccountId: createSpotOrderMsg.Order.OrderInfo.SubaccountId,
						OrderHash:    resp.OrderHash,
					},
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to orders in list for granter 1, since granter 1 grants market 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{testInput.Perps[1].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// create order
			createSpotOrderMsg := *spotOrders[1]
			createSpotOrderMsg.Sender = granter1
			createSpotOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			resp, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), &createSpotOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// cancel order of market 0 should NOT be succeeded
			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender:       granter1,
				SubaccountId: validSubaccountID,
				SpotOrdersToCancel: []*exchangetypes.OrderData{
					{
						MarketId:     createSpotOrderMsg.Order.MarketId,
						SubaccountId: createSpotOrderMsg.Order.OrderInfo.SubaccountId,
						OrderHash:    resp.OrderHash,
					},
				},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(errors.Is(sdkerrors.ErrUnauthorized, err)).To(BeTrue())
		})
		It("should allow grantee1 to all orders of market 0 for granter 1", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{testInput.Perps[0].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// create order
			createSpotOrderMsg := *spotOrders[1]
			createSpotOrderMsg.Sender = granter1
			createSpotOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), &createSpotOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender:                   granter1,
				SubaccountId:             validSubaccountID,
				SpotMarketIdsToCancelAll: []string{testInput.Perps[0].MarketID.Hex()},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(err).To(BeNil())
		})
		It("should not allow grantee1 to cancel all orders of market 1 for granter 1, since grantee1 has no grant for this market", func() {
			_, err := testexchange.Grant(
				ctx, app.AuthzKeeper, granter1, grantee1,
				&exchangetypes.BatchUpdateOrdersAuthz{
					SubaccountId: validSubaccountID,
					SpotMarkets:  []string{testInput.Perps[1].MarketID.Hex()},
				},
			)
			testexchange.OrFail(err)

			// create order
			createSpotOrderMsg := *spotOrders[1]
			createSpotOrderMsg.Sender = granter1
			createSpotOrderMsg.Order.OrderInfo.SubaccountId = validSubaccountID
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), &createSpotOrderMsg)
			testexchange.OrFail(err)
			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			// try to cancel all orders of market
			msg := exchangetypes.MsgBatchUpdateOrders{
				Sender:                   granter1,
				SubaccountId:             validSubaccountID,
				SpotMarketIdsToCancelAll: []string{testInput.Perps[0].MarketID.Hex()},
			}
			_, err = testexchange.Exec(ctx, app.AuthzKeeper, grantee1, &msg)
			Expect(errors.Is(sdkerrors.ErrUnauthorized, err)).To(BeTrue())
		})
	})
})
