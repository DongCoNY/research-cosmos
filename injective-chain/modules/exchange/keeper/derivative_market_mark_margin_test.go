package keeper_test

import (
	"fmt"
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

// Implementation of tests specified in https://injectivelabs.ontestpad.com/script/15#1274/2/

var _ = Describe("Derivative markets mark price and margin", func() {
	var (
		injectiveApp            *simapp.InjectiveApp
		k                       keeper.Keeper
		ctx                     sdk.Context
		accounts                []testexchange.Account
		mainSubaccountId        common.Hash
		marketId                common.Hash
		testInput               testexchange.TestInput
		simulationError         error
		hooks                   map[string]func(error)
		app                     *simapp.InjectiveApp
		market                  keeper.MarketI
		startingPrice           = sdk.NewDec(2000)
		oracleBase, oracleQuote string
		oracleType              oracletypes.OracleType
		err                     error
		tp                      testexchange.TestPlayer
	)

	BeforeEach(func() {
		hooks = make(map[string]func(error))

		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)
		oracleBase, oracleQuote, oracleType = testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

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

		marketId = market.MarketID()
	})

	var setup = func(testSetup testexchange.TestPlayer) {
		injectiveApp = testSetup.App
		k = injectiveApp.ExchangeKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		marketId = testInput.Perps[0].MarketID
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/mark_margin", file)
		tp = testexchange.LoadReplayableTest(filepath)
		setup(tp)
		simulationError = tp.ReplayTestWithLegacyHooks(testexchange.DefaultFlags, &hooks, nil)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	var getAvailableQuoteBalance = func(accountIdx int) sdk.Dec {
		marketId := testInput.Perps[0].MarketID
		market := k.GetDerivativeMarketByID(ctx, marketId)
		balancesQuote := testexchange.GetBankAndDepositFunds(injectiveApp, ctx, common.HexToHash(accounts[accountIdx].SubaccountIDs[0]), market.QuoteDenom)
		return balancesQuote.AvailableBalance
	}

	_ = getAvailableQuoteBalance

	printOrders := func(orders []*types.TrimmedDerivativeLimitOrder) {
		fmt.Fprintln(GinkgoWriter, "Orders: ", testexchange.GetReadableSlice(orders, " | ", func(ord *types.TrimmedDerivativeLimitOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v%v) side:%v", ord.Price.TruncateInt(), ord.Fillable.TruncateInt(), ro, side)
		}))
	}
	_ = printOrders

	var verifyPosition = func(quantity int64, isLong bool) {
		testexchange.VerifyPosition(injectiveApp, ctx, mainSubaccountId, marketId, quantity, isLong)
	}
	_ = verifyPosition

	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedDerivativeLimitOrder {
		return testexchange.GetAllDerivativeOrdersSorted(injectiveApp, ctx, subaccountId, marketId)
	}

	var getAllOrdersSorted = func() []*types.TrimmedDerivativeLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	var verifyOrder = testexchange.VerifyDerivativeOrder
	_ = verifyOrder

	Context("When mark price and margin constrain holds", func() {

		Context("CH.1 Limit buy order can be placed", func() {
			BeforeEach(func() {
				runTest("ch1_limit_buy", true)
			})
			It("order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 20, 5000, false)
			})
		})

		Context("CH.2 Limit sell order can be placed", func() {
			BeforeEach(func() {
				runTest("ch2_limit_sell", true)
			})
			It("order is placed", func() {
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(1))
				verifyOrder(orders, 0, 20, 5000, false)
			})
		})

		Context("CH.3 Market buy order can be placed", func() {
			BeforeEach(func() {
				runTest("ch3_market_buy", true)
			})
			It("order is placed", func() {
				verifyPosition(20, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})

		Context("CH.4 Market sell order can be placed", func() {
			BeforeEach(func() {
				runTest("ch4_market_sell", true)
			})
			It("order is placed", func() {
				verifyPosition(20, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})
	})

	Context("When mark price and margin constrain doesn't hold", func() {

		Context("CH.5 Limit buy order cannot be placed", func() {
			BeforeEach(func() {
				runTest("ch5_limit_buy", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})

		Context("CH.6 Limit sell order cannot be placed", func() {
			BeforeEach(func() {
				runTest("ch6_limit_sell", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})

		Context("CH.7 Market buy order cannot be placed", func() {
			BeforeEach(func() {
				runTest("ch7_market_buy", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(0, true)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})

		Context("CH.8 Market sell order cannot be placed", func() {
			BeforeEach(func() {
				runTest("ch8_market_sell", false)
			})
			It("order is rejected", func() {
				Expect(simulationError).NotTo(BeNil())
				verifyPosition(0, false)
				orders := getAllOrdersSorted()
				Expect(len(orders)).To(Equal(0))
			})
		})
	})
})
