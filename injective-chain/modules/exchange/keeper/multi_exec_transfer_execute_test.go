package keeper_test

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

// TODO: deprecate this since we no longer have MulitExec
var _ = Describe("MultiExec tests", func() {
	var (
		tp                           *te.TestPlayer
		injectiveApp                 *simapp.InjectiveApp
		keeper                       exchangekeeper.Keeper
		ctx                          sdk.Context
		accounts                     []te.Account
		mainSubaccountId             common.Hash
		marketId                     common.Hash
		testInput                    te.TestInput
		simulationError              error
		hooks                        map[string]te.TestPlayerHook
		initialInsuranceFundsBalance map[string]math.Int
		balancesTracker              *te.BalancesTracker
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		initialInsuranceFundsBalance = make(map[string]math.Int, 0)
		hooks["post-setup"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				subaccountID := common.HexToHash(acc.SubaccountIDs[0])
				bankBalances := injectiveApp.BankKeeper.GetAllBalances(ctx, types.SubaccountIDToSdkAddress(subaccountID))
				balancesTracker.SetBankBalancesAndSubaccountDeposits(subaccountID, bankBalances, keeper.GetDeposits(ctx, subaccountID))
			}

			for _, fund := range tp.App.InsuranceKeeper.GetAllInsuranceFunds(tp.Ctx) {
				initialInsuranceFundsBalance[fund.MarketId] = fund.Balance
			}
		}
	})

	var getInsuraceFundBalanceChange = func(marketIndex int) math.Int {
		marketType := tp.GetDefaultMarketType(nil)
		marketID := tp.FindMarketId(marketType, marketIndex)
		fund := injectiveApp.InsuranceKeeper.GetInsuranceFund(tp.Ctx, marketID)

		oldBalance := sdk.NewInt(0)
		initialFundBalance, ok := initialInsuranceFundsBalance[marketID.Hex()]

		if ok {
			oldBalance = initialFundBalance
		}

		return fund.Balance.Sub(oldBalance)
	}

	_ = getInsuraceFundBalanceChange

	var setup = func(testSetup *te.TestPlayer) {
		tp = testSetup
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		ctx = testSetup.Ctx
		accounts = *testSetup.Accounts
		mainSubaccountId = common.HexToHash(accounts[0].SubaccountIDs[0])
		testInput = testSetup.TestInput
		if len(testInput.Spots) > 0 {
			marketId = testInput.Spots[0].MarketID
		}
		if len(testInput.Perps) > 0 {
			marketId = testInput.Perps[0].MarketID
		}
		if len(testInput.BinaryMarkets) > 0 {
			marketId = testInput.BinaryMarkets[0].MarketID
		}
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/wrapped_message", file)
		test := te.LoadReplayableTest(filepath)
		setup(&test)
		simulationError = tp.ReplayTest(te.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}
	_ = runTest

	// getAllOrdersSorted returns all current orders sorted by best price, quantity, vanilla /reduce only
	var getAllOrdersSortedForAccount = func(subaccountId common.Hash) []*types.TrimmedSpotLimitOrder {
		return te.GetAllSpotOrdersSorted(injectiveApp, tp.Ctx, subaccountId, marketId)
	}

	var getAllOrdersSortedInMarket = func(marketId common.Hash) []*types.TrimmedSpotLimitOrder {
		return te.GetAllSpotOrdersSorted(injectiveApp, tp.Ctx, mainSubaccountId, marketId)
	}

	_ = getAllOrdersSortedInMarket

	var getAllOrdersSorted = func() []*types.TrimmedSpotLimitOrder {
		return getAllOrdersSortedForAccount(mainSubaccountId)
	}
	_ = getAllOrdersSorted

	printOrders := func(orders []*types.TrimmedSpotLimitOrder) {
		fmt.Println("Orders: ", te.GetReadableSlice(orders, " | ", func(ord *types.TrimmedSpotLimitOrder) string {
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v) side:%v", ord.Price.TruncateInt(), ord.Fillable.TruncateInt(), side)
		}))
	}
	_ = printOrders

	var verifySpotOrder = te.VerifySpotOrder

	_ = verifySpotOrder

	var getMarketId = func(marketType te.MarketType) common.Hash {
		switch marketType {
		case te.MarketType_spot:
			return testInput.Spots[0].Market.MarketID
		case te.MarketType_derivative:
			return testInput.Perps[0].Market.MarketID
		case te.MarketType_expiry:
			return testInput.ExpiryMarkets[0].Market.MarketID
		case te.MarketType_binary:
			return testInput.BinaryMarkets[0].Market.MarketID
		}

		panic(fmt.Sprintf("Unknown market type: %v", marketType))
	}

	var getTakerFeeRateForMarket = func(marketType te.MarketType) sdk.Dec {
		marketId := getMarketId(marketType)
		switch marketType {
		case te.MarketType_spot:
			return keeper.GetSpotMarketByID(ctx, marketId).TakerFeeRate
		case te.MarketType_derivative:
			return keeper.GetDerivativeMarketByID(ctx, marketId).TakerFeeRate
		case te.MarketType_binary:
			return keeper.GetBinaryOptionsMarketByID(ctx, marketId).TakerFeeRate
		}

		panic("unsupported market type")
	}

	_ = getTakerFeeRateForMarket

	var getMakerFeeRateForMarket = func(marketType te.MarketType) sdk.Dec {
		marketId := getMarketId(marketType)
		switch marketType {
		case te.MarketType_spot:
			return keeper.GetSpotMarketByID(ctx, marketId).MakerFeeRate
		case te.MarketType_derivative:
			return keeper.GetDerivativeMarketByID(ctx, marketId).MakerFeeRate
		case te.MarketType_binary:
			return keeper.GetBinaryOptionsMarketByID(ctx, marketId).MakerFeeRate
		}

		panic("unsupported market type")
	}

	_ = getMakerFeeRateForMarket

	var assertEventWasEmitted = func(eventType string, attributes []sdk.Attribute) {
		eventFound := false
		matchingAttribues := 0
		totalMatchesFound := 0

		for _, e := range tp.Ctx.EventManager().Events() {
			if e.Type == eventType {
				eventFound = true
				for _, expectedAttr := range attributes {
					for _, actualAttr := range e.Attributes {
						if string(actualAttr.Key) == expectedAttr.GetKey() && string(actualAttr.Value) == expectedAttr.GetValue() {
							matchingAttribues++
							break
						}
					}

					if matchingAttribues == len(attributes) {
						break
					}
				}
			}
			if (matchingAttribues == len(attributes)) && eventFound {
				totalMatchesFound++
				break
			}
		}
		fmt.Printf("Event found? %v Attributed expected/found: %d/%d\n", eventFound, matchingAttribues, len(attributes))
		Expect(eventFound).To(BeTrue(), "No event with type '%s' found", eventType)
		Expect(matchingAttribues).To(Equal(len(attributes)), "incorrect number of matching attributes found")
		Expect(totalMatchesFound).To(Equal(1), "incorrect number of events emitted found")
	}

	_ = assertEventWasEmitted

})
