package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Batch Update Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		err              error
		subaccountID     string
		deposit          *types.Deposit
		sender           = sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 3, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		startingPrice := sdk.NewDec(2000)

		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			testInput.Perps[0].Ticker,
			testInput.Perps[0].QuoteDenom,
			oracleBase,
			oracleQuote,
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

		subaccountID = types.MustSdkAddressWithNonceToSubaccountID(sender, 0).String()

		deposit = &types.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}

		testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

		buyOrder := &types.MsgCreateDerivativeLimitOrder{
			Sender: sender.String(),
			Order: types.DerivativeOrder{
				MarketId: derivativeMarket.MarketId,
				OrderInfo: types.OrderInfo{
					SubaccountId: subaccountID,
					FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					Price:        sdk.NewDec(2000),
					Quantity:     sdk.NewDec(2),
				},
				OrderType:    types.OrderType_BUY,
				Margin:       sdk.NewDec(2000),
				TriggerPrice: nil,
			},
		}

		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), buyOrder)
		testexchange.OrFail(err)

		sellOrder := &types.MsgCreateDerivativeLimitOrder{
			Sender: sender.String(),
			Order: types.DerivativeOrder{
				MarketId: derivativeMarket.MarketId,
				OrderInfo: types.OrderInfo{
					SubaccountId: subaccountID,
					FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					Price:        sdk.NewDec(5000),
					Quantity:     sdk.NewDec(2),
				},
				OrderType:    types.OrderType_SELL,
				Margin:       sdk.NewDec(5000),
				TriggerPrice: nil,
			},
		}

		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), sellOrder)
		testexchange.OrFail(err)
	})

	Context("simplified subaccount id in main message", func() {
		It("cancels all orders on derivative market", func() {
			batchUpdateMsg := &types.MsgBatchUpdateOrders{
				Sender:                         sender.String(),
				SubaccountId:                   "0",
				DerivativeMarketIdsToCancelAll: []string{derivativeMarket.MarketId},
			}

			_, err := msgServer.BatchUpdateOrders(sdk.WrapSDKContext(ctx), batchUpdateMsg)
			testexchange.OrFail(err)

			depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

			Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()), "available balance was incorrect")
			Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()), "total balance was incorrect")
		})
	})
})
