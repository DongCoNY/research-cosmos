package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Derivatives Orders Keeper Unit Test", func() {
	var (
		testInput          testexchange.TestInput
		app                *simapp.InjectiveApp
		ctx                sdk.Context
		market             keeper.MarketI
		marketID           common.Hash
		subaccountIdBuyer  = testexchange.SampleSubaccountAddr1
		subaccountIdSeller = testexchange.SampleSubaccountAddr2
		senderBuyer        = types.SubaccountIDToSdkAddress(subaccountIdBuyer)
		deposit            = &types.Deposit{
			AvailableBalance: sdk.NewDec(100000),
			TotalBalance:     sdk.NewDec(100000),
		}
		startingPrice           = sdk.NewDec(2000)
		margin                  = sdk.NewDec(1000)
		oracleBase, oracleQuote string
		oracleType              oracletypes.OracleType
		err                     error
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		oracleBase, oracleQuote, oracleType = testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, senderBuyer, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, senderBuyer, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		market, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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

		marketID = market.MarketID()

		testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
		testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

	})

	Describe("Launching BandIBC oracle TEF market when original Band oracle market exists", func() {
		It("should fail", func() {
			oracleBase, oracleQuote := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote
			oracleType := oracletypes.OracleType_Band
			app.OracleKeeper.SetBandPriceState(ctx, oracleQuote, &oracletypes.BandPriceState{
				Symbol:      oracleQuote,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})
			app.OracleKeeper.SetBandPriceState(ctx, oracleBase, &oracletypes.BandPriceState{
				Symbol:      oracleBase,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})

			initialInsuranceFundBalance := sdk.NewDec(44)

			coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, senderBuyer, sdk.NewCoins(coin))

			err := app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				senderBuyer,
				coin,
				testInput.Perps[0].Ticker,
				testInput.Perps[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				-1,
			)
			testexchange.OrFail(err)

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
			oracleType = oracletypes.OracleType_BandIBC
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
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("with a promoted Band IBC oracle already exists ticker"))
		})
	})

	Describe("For a derivative limit orderbook", func() {
		var (
			buyOrder1  types.DerivativeLimitOrder
			buyOrder2  types.DerivativeLimitOrder
			buyOrder3  types.DerivativeLimitOrder
			sellOrder1 types.DerivativeLimitOrder
			sellOrder2 types.DerivativeLimitOrder
			sellOrder3 types.DerivativeLimitOrder
		)

		BeforeEach(func() {
			isBuy := true

			buyPrice1, buyQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			buyPrice2, buyQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			buyPrice3, buyQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")

			buyOrder1 = testexchange.NewDerivativeLimitOrder(buyPrice1, buyQuantity1, margin, subaccountIdBuyer, marketID.Hex(), "", isBuy)
			buyOrder2 = testexchange.NewDerivativeLimitOrder(buyPrice2, buyQuantity2, margin, subaccountIdBuyer, marketID.Hex(), "", isBuy)
			buyOrder3 = testexchange.NewDerivativeLimitOrder(buyPrice3, buyQuantity3, margin, subaccountIdBuyer, marketID.Hex(), "", isBuy)

			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &buyOrder1, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &buyOrder2, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &buyOrder3, nil, marketID)

			isBuy = false
			sellPrice1, sellQuantity1 := testexchange.PriceAndQuantityFromString("100.1", "1.5")
			sellPrice2, sellQuantity2 := testexchange.PriceAndQuantityFromString("300.1", "3.5")
			sellPrice3, sellQuantity3 := testexchange.PriceAndQuantityFromString("200.1", "100.531")

			sellOrder1 = testexchange.NewDerivativeLimitOrder(sellPrice1, sellQuantity1, margin, subaccountIdSeller, marketID.Hex(), "", isBuy)
			sellOrder2 = testexchange.NewDerivativeLimitOrder(sellPrice2, sellQuantity2, margin, subaccountIdSeller, marketID.Hex(), "", isBuy)
			sellOrder3 = testexchange.NewDerivativeLimitOrder(sellPrice3, sellQuantity3, margin, subaccountIdSeller, marketID.Hex(), "", isBuy)

			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &sellOrder1, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &sellOrder2, nil, marketID)
			app.ExchangeKeeper.SetNewDerivativeLimitOrderWithMetadata(ctx, &sellOrder3, nil, marketID)
		})

		It("should have buys in descending order", func() {
			buyOrderbook := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, true)
			Expect(len(buyOrderbook)).To(Equal(3))

			for idx, buyOrder := range buyOrderbook {
				if idx == len(buyOrderbook)-1 {
					break
				}

				nextOrder := buyOrderbook[idx+1]
				Expect(buyOrder.OrderInfo.Price.GT(nextOrder.OrderInfo.Price)).To(BeTrue())
			}

			Expect(buyOrderbook[0]).To(Equal(&buyOrder2))
			Expect(buyOrderbook[1]).To(Equal(&buyOrder3))
			Expect(buyOrderbook[2]).To(Equal(&buyOrder1))
		})

		It("should have sells in ascending order", func() {
			sellOrderbook := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, false)
			Expect(len(sellOrderbook)).To(Equal(3))

			for idx, sellOrder := range sellOrderbook {
				if idx == len(sellOrderbook)-1 {
					break
				}

				nextOrder := sellOrderbook[idx+1]
				Expect(sellOrder.OrderInfo.Price.LT(nextOrder.OrderInfo.Price)).To(BeTrue())
			}

			Expect(sellOrderbook[0]).To(Equal(&sellOrder1))
			Expect(sellOrderbook[1]).To(Equal(&sellOrder3))
			Expect(sellOrderbook[2]).To(Equal(&sellOrder2))
		})

		It("should have highest buy as best prices", func() {
			bestPrice := app.ExchangeKeeper.GetBestDerivativeLimitOrderPrice(ctx, marketID, true)
			expectedBestPrice, _ := sdk.NewDecFromStr("300.1")

			Expect(*bestPrice).To(Equal(expectedBestPrice))
		})

		It("should have lowest sell as best price", func() {
			bestPrice := app.ExchangeKeeper.GetBestDerivativeLimitOrderPrice(ctx, marketID, false)
			expectedBestPrice, _ := sdk.NewDecFromStr("100.1")

			Expect(*bestPrice).To(Equal(expectedBestPrice))
		})
	})
})
