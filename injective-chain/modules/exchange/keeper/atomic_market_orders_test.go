package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Atomic market orders", func() {
	var (
		injectiveApp     *simapp.InjectiveApp
		keeper           exchangekeeper.Keeper
		bankKeeper       bankkeeper.Keeper
		ctx              sdk.Context
		accounts         []te.Account
		mainSubaccountId common.Hash
		marketId         common.Hash
		testInput        te.TestInput
		simulationError  error
		hooks            map[string]te.TestPlayerHook
		balancesTracker  *te.BalancesTracker
		USDT0            string = "USDT0"
		tp               *te.TestPlayer
	)

	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
		balancesTracker = te.NewBalancesTracker()
		hooks["init"] = func(*te.TestPlayerHookParams) {
			for _, acc := range accounts {
				subaccountID := common.HexToHash(acc.SubaccountIDs[0])
				bankBalances := bankKeeper.GetAllBalances(ctx, acc.AccAddress)
				deposits := keeper.GetDeposits(ctx, subaccountID)
				balancesTracker.SetBankBalancesAndSubaccountDeposits(subaccountID, bankBalances, deposits)
			}
		}
	})

	var setup = func(testSetup *te.TestPlayer) {
		tp = testSetup
		injectiveApp = testSetup.App
		keeper = injectiveApp.ExchangeKeeper
		bankKeeper = injectiveApp.BankKeeper
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

	var verifyDerivativePosition = func(accountIdx, quantity int64, isLong bool) {
		subaccountId := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		te.VerifyPosition(injectiveApp, ctx, subaccountId, marketId, quantity, isLong)
	}

	var getAllDerivativeOrdersSorted = func() []*types.TrimmedDerivativeLimitOrder {
		return te.GetAllDerivativeOrdersSorted(injectiveApp, ctx, mainSubaccountId, marketId)
	}

	var getAllSpotOrdersSorted = func() []*types.TrimmedSpotLimitOrder {
		return te.GetAllSpotOrdersSorted(injectiveApp, ctx, mainSubaccountId, marketId)
	}

	var getAvailableUSDTBalancePlusBank = func(accountIdx int) sdk.Dec {
		denom := "USDT0"
		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		balancesQuote := keeper.GetDeposit(ctx, subaccountID, denom).AvailableBalance
		bankBalance := sdk.ZeroInt()
		if types.IsDefaultSubaccountID(subaccountID) {
			accountAddr := types.SubaccountIDToSdkAddress(subaccountID)
			bankBalance = bankKeeper.GetBalance(ctx, accountAddr, denom).Amount
		}

		return balancesQuote.Add(bankBalance.ToDec())
	}

	var getAvailableBaseAssetBalancePlusBankForMarket = func(accountIdx int, marketId common.Hash, marketType te.MarketType) sdk.Dec {
		subaccountID := common.HexToHash(accounts[accountIdx].SubaccountIDs[0])
		var baseDenom string
		if marketType == te.MarketType_spot {
			market := keeper.GetSpotMarketByID(ctx, marketId)
			baseDenom = market.BaseDenom

		} else {
			market := keeper.GetDerivativeMarketByID(ctx, marketId)
			baseDenom = market.OracleBase
		}

		balancesBase := keeper.GetDeposit(ctx, subaccountID, baseDenom).AvailableBalance
		bankBalance := sdk.ZeroInt()
		if types.IsDefaultSubaccountID(subaccountID) {
			accountAddr := types.SubaccountIDToSdkAddress(subaccountID)
			bankBalance = bankKeeper.GetBalance(ctx, accountAddr, baseDenom).Amount
		}

		return balancesBase.Add(bankBalance.ToDec())
	}

	var DEFAULT_ATOMIC_MULTILIER = f2d(2.5)

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

	var calculateMarginHoldForTakerOrder = func(price sdk.Dec, quantity sdk.Dec, margin sdk.Dec, marketType te.MarketType, isAtomic bool, atomicMultiplier *sdk.Dec) sdk.Dec {
		takerFeeRate := getTakerFeeRateForMarket(marketType)
		fee := price.Mul(quantity).Mul(takerFeeRate)

		if isAtomic {
			multiplierToUse := DEFAULT_ATOMIC_MULTILIER
			if atomicMultiplier != nil {
				multiplierToUse = *atomicMultiplier
			}

			fee = fee.Mul(multiplierToUse)
		}

		return margin.Add(fee)
	}

	var calculateBalanceChangeForMatchedTakerOrder = func(price sdk.Dec, quantity sdk.Dec, margin sdk.Dec, marketType te.MarketType, isBuy, isAtomic bool, atomicMultiplier *sdk.Dec) sdk.Dec {
		takerFeeRate := getTakerFeeRateForMarket(marketType)
		fee := price.Mul(quantity).Mul(takerFeeRate)

		if isAtomic {
			multiplierToUse := DEFAULT_ATOMIC_MULTILIER
			if atomicMultiplier != nil {
				multiplierToUse = *atomicMultiplier
			}

			fee = fee.Mul(multiplierToUse)
		}

		switch marketType {
		case te.MarketType_spot:
			if isBuy {
				return margin.Add(fee)
			}

			return margin.Sub(fee)
		default:
			return margin.Add(fee)
		}
	}

	var calculateExpectedValueForMakerOrder = func(price, quantity, margin sdk.Dec, marketType te.MarketType, isBuy bool) sdk.Dec {
		makerFeeRate := getMakerFeeRateForMarket(marketType)
		// fmt.Printf("Fee rate: %v\n", makerFeeRate.String())
		fee := price.Mul(quantity).Mul(makerFeeRate)

		switch marketType {
		case te.MarketType_spot:
			if isBuy {
				if makerFeeRate.IsNegative() {
					return margin.Mul(f2d(-1)).Add(fee.Abs().Mul(f2d(0.6))) //as by default 40% goes to the relayer
				}
				return margin.Mul(f2d(-1)).Sub(fee)
			}

			if makerFeeRate.IsNegative() {
				return margin.Add(fee.Abs().Mul(f2d(0.6))) //as by default 40% goes to the relayer
			}
			return margin.Sub(fee)
		default:
			// fmt.Printf("Fee: %v\n", fee.String())
			// fmt.Printf("Margin: %v\n", margin.String())
			if makerFeeRate.IsNegative() {
				toReturn := margin.Sub(fee.Abs().Mul(f2d(0.6)))
				// fmt.Printf("Margin hold [N]: %v\n", toReturn.String())

				return toReturn //as by default 40% goes to the relayer
			}

			toReturn := margin.Add(fee)
			// fmt.Printf("Margin hold [P]: %v\n", toReturn.String())

			return toReturn
		}
	}

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/atomic", file)
		test := te.LoadReplayableTest(filepath)
		setup(&test)
		simulationError = test.ReplayTest(testexchange.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	printDerivativeOrders := func(orders []*types.TrimmedDerivativeLimitOrder) {
		fmt.Println(te.GetReadableSlice(orders, "-", func(ord *types.TrimmedDerivativeLimitOrder) string {
			ro := ""
			if ord.Margin.IsZero() {
				ro = " ro"
			}
			return fmt.Sprintf("p:%v(q:%v%v)", ord.Price.TruncateInt(), ord.Quantity.TruncateInt(), ro)
		}))
	}
	_ = printDerivativeOrders

	printSpotOrders := func(orders []*types.TrimmedSpotLimitOrder) {
		fmt.Println(te.GetReadableSlice(orders, "-", func(ord *types.TrimmedSpotLimitOrder) string {
			side := "sell"
			if ord.IsBuy {
				side = "buy"
			}
			return fmt.Sprintf("p:%v(q:%v [f:%v] %v)", ord.Price.TruncateInt(), ord.Quantity.TruncateInt(), ord.Fillable.TruncateInt(), side)
		}))
	}
	_ = printSpotOrders

	Context("Access control", func() {

		Context("AC.6 Atomic DERIVATIVE order cannot be placed by user if access level is set to none", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error was not thrown")
					Expect(types.ErrInvalidAccessLevel.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				runTest("ac6_noone_user_perp", true)
			})

			It("atomic spot market order should be rejected", func() {
				Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), "USDT0").String()).To(Equal(f2d(0).String()), "USDT0 balance of user 0 should not change")
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0))
			})
		})

		Context("AC.6 Atomic SPOT order cannot be placed by user if access level is set to noone", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error was not thrown")
					Expect(types.ErrInvalidAccessLevel.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				runTest("ac6_noone_user_spot", true)
			})

			It("atomic spot market order should be rejected", func() {
				Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), "USDT0").String()).To(Equal(f2d(0).String()), "USDT0 balance of user 0 should not change")
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0))
			})
		})

		Context("AC.7 Atomic DERIVATIVE order cannot be placed by user if access level is set to smart contract", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error was not thrown")
					Expect(types.ErrInvalidAccessLevel.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				runTest("ac7_sc_user_perp", true)
			})

			It("atomic spot market order should be rejected", func() {
				Expect(balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), "USDT0").String()).To(Equal(f2d(0).String()), "USDT0 balance of user 0 should not change")
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0))
			})
		})

		//not implemented yet
		Context("AC.8 Atomic DERIVATIVE order cannot be placed by user if access level is set to smart contract's BB", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error was not thrown")
					Expect(types.ErrInvalidAccessLevel.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				runTest("ac8_sc_bb_user_perp", true)
			})

			/*It("atomic spot market order should be rejected", func() {
				Expect(getDepositChange(0, "USDT0").String()).To(Equal(f2d(0).String()), "USDT0 balance of user 0 should not change")
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0))
			})*/
		})
	})

	Context("Spot markets", func() {

		Context("AS.1 Atomic order cannot be placed without margin", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrInsufficientDeposit.Is(p.Error) || types.ErrInsufficientFunds.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
					Expect(p.Response).To(BeNil(), "nil order response should have been returned")
				}

				runTest("as1_needs_margin", true)
			})

			It("atomic spot market order should be rejected", func() {
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(f2d(0).String()))
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0))
			})
		})

		Context("AS.2 Atomic order matches with new-post only order (front-running is possible)", func() {
			var preBaseBalance sdk.Dec
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_spot).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.Price.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				runTest("as2_front_running", true)
			})

			It("atomic spot market order can be front-run", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance.Add(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AS.3 Atomic order doesn't match with market orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["market"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				runTest("as3_no_market_no_cry", true)
			})

			It("atomic spot market and market order to not cross", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AS.3 Atomic order doesn't match with market orders (reverse order)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["market"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				runTest("as3_no_market_no_cry_2", true)
			})

			It("atomic spot market and market order to not cross", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AS.4 Atomic orders do not match with other atomic orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				runTest("as4_atomics_no_no", true)
			})

			It("atomic spot market and market order to not cross", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AS.5 Atomic order is executed before market orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var pretUsdtBalance2 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					pretUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_spot).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.Price.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(10).String()), "filled quantity for atomic order should be 10")
					Expect(response.Results.Price.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("as5_atomic_before_market", true)
			})

			It("atomic spot market is executed before market", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Sub(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should decrease")

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Add(atomicOrderValue).String()), "quote asset balance for user 0 should increase")

				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true)
				expectedUsdtBalance2 := pretUsdtBalance2.Add(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("AS.6 Market order is rejected if atomic order would eat all liquidity", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var pretUsdtBalance2 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					pretUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["market"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				runTest("as6_market_order_is_rejected", true)
			})

			It("market order is rejected", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Sub(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should decrease")

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Add(atomicOrderValue).String()), "quote asset balance for user 0 should increase")

				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true)
				expectedUsdtBalance2 := pretUsdtBalance2.Add(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("AS.7 Atomic order is executed before limit orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var pretUsdtBalance2 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					pretUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_spot).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.Price.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(10).String()), "filled quantity for atomic order should be 10")
					Expect(response.Results.Price.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("as7_atomic_before_limit", true)
			})

			It("atomic spot market is executed before limit", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Sub(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should decrease")

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Add(atomicOrderValue).String()), "quote asset balance for user 0 should increase")

				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true)
				expectedUsdtBalance2 := pretUsdtBalance2.Add(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("AS.8 Atomic order can be partially filled", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_spot).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.Price.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(5).String()), "filled quantity for atomic order should be 5")
					Expect(response.Results.Price.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("as8_partial_fill", true)
			})

			It("atomic spot market is executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Sub(f2d(5))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should decrease")

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(5), f2d(50), te.MarketType_spot, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Add(atomicOrderValue).String()), "quote asset balance for user 0 should increase")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(5), f2d(50), te.MarketType_spot, true)
				expectedUsdtBalance2 := preUsdtBalance1.Add(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("AS.9 Multiple atomic orders from the same account", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				runTest("as9_multiple_atomics", true)
			})

			It("2 atomic spot market orders are executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Add(f2d(4))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should increase")

				atomicOrderValue1 := calculateBalanceChangeForMatchedTakerOrder(f2d(11), f2d(2), f2d(22), te.MarketType_spot, true, true, nil)
				atomicOrderValue2 := calculateBalanceChangeForMatchedTakerOrder(f2d(11), f2d(2), f2d(22), te.MarketType_spot, true, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue1).Sub(atomicOrderValue2).String()), "quote asset balance for user 0 should decrease")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(11), f2d(4), f2d(44), te.MarketType_spot, false)
				expectedUsdtBalance2 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 1 should increase")
			})
		})

		Context("AS.10 Multiple atomic orders from the same account with opposing direction", func() {
			var preUsdtBalance0 sdk.Dec
			var postUsdtBalance1 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["post-setup"] = func(p *te.TestPlayerHookParams) {
					postUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_spot).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.Price.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_spot).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.Price.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				runTest("as10_multiple_atomics_opposing", true)
			})

			It("2 atomic spot market orders are executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Add(f2d(1))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should increase")

				atomicOrderValue1 := calculateBalanceChangeForMatchedTakerOrder(f2d(11), f2d(2), f2d(22), te.MarketType_spot, true, true, nil)
				atomicOrderValue2 := calculateBalanceChangeForMatchedTakerOrder(f2d(3), f2d(1), f2d(3), te.MarketType_spot, false, true, nil)
				expectedUsdtBalance0 := preUsdtBalance0.Sub(atomicOrderValue1).Add(atomicOrderValue2)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(expectedUsdtBalance0.String()), "quote asset balance for user 0 should decrease")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(11), f2d(2), f2d(22), te.MarketType_spot, false)

				expectedUsdtBalance1 := postUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should increase")
			})
		})

		Context("AS.11 Multiple atomic orders and normal orders from the same account", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var preBaseBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				runTest("as11_different_orders", true)
			})

			It("atomic spot market orders are executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance0.Add(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0 should increase")

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(3), f2d(30), te.MarketType_spot, true, true, nil)
				marketOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(2), f2d(20), te.MarketType_spot, true, false, nil)
				limitOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(5), f2d(50), te.MarketType_spot, true, false, nil)
				expectedUsdtBalance0 := preUsdtBalance0.Sub(atomicOrderValue).Sub(marketOrderValue).Sub(limitOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(expectedUsdtBalance0.String()), "quote asset balance for user 0 should decrease")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should increase")
			})
		})

		Context("AS.12 Multiple atomic orders from different accounts", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var preBaseBalance0 sdk.Dec
			var preBaseBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					preBaseBalance0 = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preBaseBalance1 = getAvailableBaseAssetBalancePlusBankForMarket(1, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				runTest("as12_different_orders_diff_accounts", true)
			})

			It("atomic spot market orders are executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance0 := preBaseBalance0.Add(f2d(3))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance0.String()), "base asset balance for user 0 should increase")

				expectedBaseBalance1 := preBaseBalance1.Add(f2d(3))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance1.String()), "base asset balance for user 1 should increase")

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(3), f2d(30), te.MarketType_spot, true, true, nil)
				expectedUsdtBalance0 := preUsdtBalance0.Sub(atomicOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(expectedUsdtBalance0.String()), "quote asset balance for user 0 should decrease")

				expectedUsdtBalance1 := preUsdtBalance1.Sub(atomicOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("AS.13 Atomic order doesn't match with transient limit orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue())
				}
				runTest("as13_no_limit_no_cry", true)
			})

			It("atomic spot market and limit order to not cross", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AS.14 Atomic order matches with new post-only order (no batch)", func() {
			var preBaseBalance sdk.Dec
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				runTest("as14_post-only_no_batch", true)
			})

			It("atomic spot market order matches a transient po", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance.Add(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AS.15 Atomic order cannot be sent as part of batch update", func() {
			var preBaseBalance sdk.Dec
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["batch"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrInvalidOrderTypeForMessage.Is(p.Error)).To(BeTrue(), p.Error.Error())
				}
				runTest("as15_no_atom_no_batch", true)
			})

			It("atomic spot market order is rejected", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(preBaseBalance.String()), "base asset balance for user 0")

				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 should not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")
			})
		})

		Context("AS.16 Atomic order cannot be cancelled (MsgSpotCancel)", func() {
			var preBaseBalance sdk.Dec
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("as16_no_atom_cancel_single", true)
			})

			It("atomic spot market order is executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance.Add(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		//test support for that message not implemented
		Context("AS.17 Atomic order cannot be cancelled (MsgSpotBatchCancel)", func() {
			/*var (
				preBaseBalance  sdk.Dec
				preUsdtBalance0 sdk.Dec
				preUsdtBalance1 sdk.Dec
			)
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalanceForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalance(0)
					preUsdtBalance1 = getAvailableUSDTBalance(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("as17_no_atom_cancel_batch", true)
			})

			It("atomic spot market order is executedexecuted", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance.Add(f2d(10))
				Expect(getAvailableBaseAssetBalanceForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil)
				Expect(getAvailableUSDTBalance(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalance(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})*/
		})

		Context("AS.18 Atomic order cannot be cancelled (MsgBatchUpdate)", func() {
			var preBaseBalance sdk.Dec
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("as18_no_atom_batch_update_cancel", true)
			})

			It("atomic spot market order is executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance.Add(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AS.18A Atomic order cannot be cancelled (MsgBatchUpdate cancel all)", func() {
			var preBaseBalance sdk.Dec
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("as18a_no_atom_batch_update_cancel_all", true)
			})

			It("atomic spot market order is executed", func() {
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0), "there should be no spot orders")
				expectedBaseBalance := preBaseBalance.Add(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, false)
				expectedUsdtBalance1 := preUsdtBalance1.Add(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AS.19 Atomic order fee can be set for each market independently", func() {
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					multipliers := make([]*types.MarketFeeMultiplier, 1)
					multiplier1 := types.MarketFeeMultiplier{
						MarketId:      testInput.Spots[1].MarketID.String(),
						FeeMultiplier: f2d(4.0),
					}
					multipliers[0] = &multiplier1
					keeper.SetAtomicMarketOrderFeeMultipliers(ctx, multipliers)
				}
				runTest("as19_update_fee_proposal", true)
			})

			It("Fee-multiplier for a specific market should be updated", func() {
				balanceChange1 := balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), USDT0)
				order1Value := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, nil).Mul(f2d(-1))
				Expect(balanceChange1.String()).To(Equal(order1Value.String()))
				m := f2d(4)
				balanceChange2 := balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), "USDT1")
				order2Value := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_spot, true, &m).Mul(f2d(-1))
				Expect(balanceChange2.String()).To(Equal(order2Value.String()))
			})
		})

		Context("AS.20 If first atomic order fails second is still processed", func() {
			var preBaseBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).To(BeNil(), "error should NOT have been thrown")
				}
				runTest("as20_first_fails_second_succeeds", true)
			})

			It("second atomic wins", func() {
				expectedBaseBalance := preBaseBalance.Sub(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")
			})
		})

		Context("AS.21 If second atomic order fails first is still processed", func() {
			var preBaseBalance sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preBaseBalance = getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot)
				}
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).To(BeNil(), "error should NOT have been thrown")
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrNoLiquidity.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				runTest("as21_first_succeeds_second_fails", true)
			})

			It("first atomic wins", func() {
				expectedBaseBalance := preBaseBalance.Sub(f2d(10))
				Expect(getAvailableBaseAssetBalancePlusBankForMarket(0, testInput.Spots[0].MarketID, te.MarketType_spot).String()).To(Equal(expectedBaseBalance.String()), "base asset balance for user 0")
			})
		})

		Context("AS.22 If there's less than requested amount in orderbook", func() {

			BeforeEach(func() {
				hooks["atomicOp"] = func(p *te.TestPlayerHookParams) {
					r := p.Response.(*types.MsgCreateSpotMarketOrderResponse)
					te.VerifyEqualDecs(r.Results.Quantity, f2d(8.0), "atomicOp - verify bought quantity")
					te.VerifyEqualDecs(r.Results.Price, f2d(1000.0), "atomicOp - verify bought price")
					te.VerifyEqualDecs(r.Results.Fee, f2d(8000.0*0.0075), "atomicOp - verify bought fees")
				}
				runTest("as22_check_order_response", true)
			})

			It("bought amount and fee in response should reflect it correctly", func() {
				te.VerifyEqualDecs(tp.GetAvailableDepositPlusBankChange(0, "ETH0"), f2d(8.0), "Should have bought 8 units")
				te.VerifyEqualDecs(tp.GetAvailableDepositPlusBankChange(0, "USDT0"), f2d(-1007.5*8), "Should have cost that much")
			})
		})
	})

	Context("Derivative markets", func() {

		Context("AD.1 Atomic order cannot be placed without margin", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrInsufficientDeposit.Is(p.Error) || types.ErrInsufficientFunds.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
					Expect(p.Response).To(BeNil(), "nil order response should have been returned")
				}
				runTest("ad1_needs_margin_perp", true)
			})

			It("atomic perp market order should be rejected", func() {
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(f2d(0).String()))
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0))
			})
		})

		Context("AD.2 Atomic order matches with new-post only order (front-running is possible)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				runTest("ad2_front_running_perp", true)
			})

			It("atomic perp market order can be front-run", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)
				verifyDerivativePosition(1, 10, false)

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AD.3 Atomic order doesn't match with market orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["market"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "market error should have thrown error")
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "atomic market error should have thrown error")
				}
				runTest("ad3_no_market_no_cry_perp", true)
			})

			It("atomic perp market and market order to not cross", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 0, true)
				verifyDerivativePosition(1, 0, true)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AD.3 Atomic order doesn't match with market orders (reversed)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["market"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "market error should have thrown error")
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "atomic market error should have thrown error")
				}
				runTest("ad3_no_market_no_cry_perp_2", true)
			})

			It("atomic perp market and market order to not cross", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 0, true)
				verifyDerivativePosition(1, 0, true)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AD.4 Atomic orders do not match with other atomic orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue())
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue())
				}
				runTest("ad4_atomics_no_no_perp", true)
			})

			It("atomic perp market and market order to not cross", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 0, true)
				verifyDerivativePosition(1, 0, true)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AD.5 Atomic order is executed before market orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var pretUsdtBalance2 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					pretUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).To(BeNil(), "atomic order wasn't executed")
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(10).String()), "filled quantity for atomic order should be 10")
					Expect(response.Results.PositionDelta.ExecutionPrice.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("ad5_atomic_before_market_perp", false)
			})

			It("atomic perp market is executed before market", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, false)

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0 should decrease")

				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true)
				expectedUsdtBalance2 := pretUsdtBalance2.Sub(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("AD.6 Market order is rejected if atomic order would eat all liquidity", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var pretUsdtBalance2 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					pretUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
				}
				hooks["market"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "market order should be rejected")
				}
				runTest("ad6_market_order_is_rejected_perp", true)
			})

			It("market order is rejected", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, false)

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0 should decrease")

				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true)
				expectedUsdtBalance2 := pretUsdtBalance2.Sub(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("AD.7 Atomic order is executed before limit orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var preUsdtBalance2 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					preUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(10).String()), "filled quantity for atomic order should be 10")
					Expect(response.Results.PositionDelta.ExecutionPrice.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("ad7_atomic_before_limit_perp", true)
			})

			It("atomic spot market is executed before limit", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, false)

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0 should decrease")

				expectedUsdtBalance1 := preUsdtBalance1.Sub(f2d(100)) //only margin, with no fee
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true)
				expectedUsdtBalance2 := preUsdtBalance2.Sub(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("AD.8 Atomic order can be partially filled", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(5).String()), "filled quantity for atomic order should be 5")
					Expect(response.Results.PositionDelta.ExecutionPrice.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")

				}
				runTest("ad8_partial_fill_perp", true)
			})

			It("atomic spot market is executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 5, false)

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(5), f2d(50), te.MarketType_derivative, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0 should decrease")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(5), f2d(50), te.MarketType_derivative, true)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("AD.9 Multiple atomic orders from the same account", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				runTest("ad9_multiple_atomics_perp", true)
			})

			It("2 atomic perp market orders are executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 4, true)

				atomicOrderValue1 := calculateBalanceChangeForMatchedTakerOrder(f2d(11), f2d(2), f2d(22), te.MarketType_derivative, true, true, nil)
				atomicOrderValue2 := calculateBalanceChangeForMatchedTakerOrder(f2d(11), f2d(2), f2d(22), te.MarketType_derivative, true, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue1).Sub(atomicOrderValue2).String()), "quote asset balance for user 0 should decrease")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(11), f2d(4), f2d(100), te.MarketType_derivative, false)
				expectedUsdtBalance2 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("AD.10 Multiple atomic orders from the same account with opposing direction", func() {
			BeforeEach(func() {
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				runTest("ad10_multiple_atomics_opposing_perp", true)
			})

			It("2 atomic perp market orders are executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 1, true)
			})
		})

		Context("AD.11 Multiple atomic orders and normal orders from the same account", func() {
			BeforeEach(func() {
				runTest("ad11_different_orders_perp", true)
			})

			It("atomic perp market orders are executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)
			})
		})

		Context("AD.12 Multiple atomic orders from different accounts", func() {
			BeforeEach(func() {
				runTest("ad12_different_orders_diff_accounts_perp", true)
			})

			It("atomic perp market orders are executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 3, true)
				verifyDerivativePosition(1, 3, true)
			})
		})

		Context("AD.13 Atomic order doesn't match with transient limit orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue())
				}
				runTest("ad13_no_limit_no_cry_perp", true)
			})

			It("atomic perp market and limit order to not cross", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")

				atomicOrderValue := f2d(100) //no fee is retained, when taker order becomes a maker order
				expectedUsdtBalance1 := preUsdtBalance1.Sub(atomicOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("AD.14 Atomic order matches with new post-only order (no batch)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				runTest("ad14_post-only_no_batch_perp", true)
			})

			It("atomic spot market order matches a transient po", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AD.15 Atomic RO order cannot be placed without open position in opposite direction", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrPositionNotFound.Is(p.Error)).To(BeTrue(), "atomic order should be rejected")
				}
				runTest("ad15_ro_no_position", true)
			})

			It("atomic perp market order matches a transient po", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 0, true)

				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				atomicOrderValue := f2d(100) //no fee is retained, when taker order becomes a maker order
				expectedUsdtBalance1 := preUsdtBalance1.Sub(atomicOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("AD.16 Atomic RO order cannot be placed with open position in the same direction", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "no error was thrown")
					Expect(types.ErrInvalidReduceOnlyPositionDirection.Is(p.Error)).To(BeTrue(), "wrong error was thrown", p.Error)
				}
				runTest("ad16_ro_same_direction_position", true)
			})

			It("atomic RO market order is rejected", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)

				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AD.17 Atomic RO order can be placed with open position in opposite direction", func() {
			var preUsdtBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateDerivativeMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				runTest("ad17_ro_opposite_position", true)
			})

			It("atomic perp market order matches a transient po", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 0, true)

				exepctedAtomicRoOrderFee := f2d(3).Mul(f2d(10)).Mul(getTakerFeeRateForMarket(te.MarketType_derivative).Mul(DEFAULT_ATOMIC_MULTILIER))
				expectedPositionOpeningFee := f2d(3).Mul(f2d(10)).Mul(getTakerFeeRateForMarket(te.MarketType_derivative))
				exepctedUsdtBalance0change := exepctedAtomicRoOrderFee.Add(expectedPositionOpeningFee)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(exepctedUsdtBalance0change).String()), "quote asset balance for user 0 did not change")
			})
		})

		Context("AD.18 Atomic order cannot be sent as part of batch update", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["batch"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "Error was not thrown")
					Expect(types.ErrInvalidOrderTypeForMessage.Is(p.Error)).To(BeTrue(), "Wrong error was thrown")
				}
				runTest("ad18_no_atom_no_batch_perp", true)
			})

			It("atomic perp market order is rejected", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 0, true)

				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 did not change")
			})
		})

		Context("AD.19 Atomic order cannot be cancelled (MsgDerivativeCancel)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("ad19_no_atom_cancel_single_perp", true)
			})

			It("atomic perp market order is executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AD.20 Atomic order cannot be cancelled (MsgDerivativeBatchCancel)", func() {
			BeforeEach(func() {
				runTest("ad20_no_atom_cancel_batch_perp", true)
			})

			It("atomic perp market order is executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)
			})
		})

		Context("AD.21 Atomic order cannot be cancelled (MsgBatchUpdate)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("ad21_no_atom_batch_update_cancel_perp", true)
			})

			It("atomic perp market order is executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AD.21A Atomic order cannot be cancelled (MsgBatchUpdate cancel all)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["cancel"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrOrderDoesntExist.Is(p.Error)).To(BeTrue())
				}
				runTest("ad21a_no_atom_batch_update_cancel_all_perp", true)
			})

			It("atomic perp market order is executed", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				verifyDerivativePosition(0, 10, true)

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, false)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("AD.22 Atomic order fee can be set for each market independently", func() {
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					multipliers := make([]*types.MarketFeeMultiplier, 1)
					multiplier1 := types.MarketFeeMultiplier{
						MarketId:      testInput.Perps[1].MarketID.String(),
						FeeMultiplier: f2d(5.0),
					}
					multipliers[0] = &multiplier1
					keeper.SetAtomicMarketOrderFeeMultipliers(ctx, multipliers)
				}
				runTest("ad22_update_fee_proposal_perp", true)
			})

			It("Fee-multiplier for a specific market should be updated", func() {
				balanceChange1 := balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[0].SubaccountIDs[0]), USDT0)
				order1Value := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, nil).Mul(f2d(-1))
				Expect(balanceChange1.String()).To(Equal(order1Value.String()))
				m := f2d(5)
				balanceChange2 := balancesTracker.GetTotalDepositPlusBankChange(injectiveApp, ctx, common.HexToHash(accounts[1].SubaccountIDs[0]), "USDT1")
				order2Value := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_derivative, true, &m).Mul(f2d(-1))
				Expect(balanceChange2.String()).To(Equal(order2Value.String()))
			})
		})

		Context("AD.23 If first atomic order fails second is still processed", func() {
			BeforeEach(func() {
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).To(BeNil(), "error should NOT have been thrown")
				}
				runTest("ad23_first_fails_second_succeeds_perp", true)
			})

			It("second atomic wins", func() {
				verifyDerivativePosition(0, 10, false)
			})
		})

		Context("AD.24 If second atomic order fails first is still processed", func() {
			BeforeEach(func() {
				hooks["atomic1"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).To(BeNil(), "error should NOT have been thrown")
				}
				hooks["atomic2"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
				}
				runTest("ad24_first_succeeds_second_fails_perp", true)
			})

			It("first atomic wins", func() {
				verifyDerivativePosition(0, 10, false)
			})
		})
	})

	Context("Binary options markets", func() {

		Context("ABO.1 Atomic order cannot be placed without margin", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil(), "error should have been thrown")
					Expect(types.ErrInsufficientDeposit.Is(p.Error) || types.ErrInsufficientFunds.Is(p.Error)).To(BeTrue(), "wrong error was thrown")
					Expect(p.Response).To(BeNil(), "nil order response should have been returned")
				}
				runTest("abo1_needs_margin_bo", true)
			})

			It("atomic bo market order should be rejected", func() {
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(f2d(0).String()))
				Expect(len(getAllSpotOrdersSorted())).To(Equal(0))
			})
		})

		Context("ABO.5 Atomic order is executed before market orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var pretUsdtBalance2 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					pretUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateBinaryOptionsMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_binary).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(10).String()), "filled quantity for atomic order should be 10")
					Expect(response.Results.PositionDelta.ExecutionPrice.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("abo5_atomic_before_market_bo", true)
			})

			It("atomic bo market is executed before market", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no bo orders")
				verifyDerivativePosition(0, 10, false)

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(9999900), te.MarketType_binary, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0 should decrease")

				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(preUsdtBalance1.String()), "quote asset balance for user 1 should not change")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_binary, true)
				expectedUsdtBalance2 := pretUsdtBalance2.Sub(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("ABO.7 Atomic order is executed before limit orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			var preUsdtBalance2 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
					preUsdtBalance2 = getAvailableUSDTBalancePlusBank(2)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateBinaryOptionsMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_binary).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
					Expect(response.Results.Quantity.String()).To(Equal(f2d(10).String()), "filled quantity for atomic order should be 10")
					Expect(response.Results.PositionDelta.ExecutionPrice.String()).To(Equal(f2d(10).String()), "execution price for atomic order should be 10")
				}
				runTest("abo7_atomic_before_limit_bo", true)
			})

			It("atomic bo market is executed before limit", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no bo orders")
				verifyDerivativePosition(0, 10, false)

				atomicOrderValue := calculateBalanceChangeForMatchedTakerOrder(f2d(10), f2d(10), f2d(9999900), te.MarketType_binary, false, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0 should decrease")

				expectedUsdtBalance1 := preUsdtBalance1.Sub(f2d(9999900)) //only margin, with no fee
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")

				buyOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_binary, true)
				expectedUsdtBalance2 := preUsdtBalance2.Sub(buyOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(2).String()).To(Equal(expectedUsdtBalance2.String()), "quote asset balance for user 2 should decrease")
			})
		})

		Context("ABO.13 Atomic order doesn't match with transient limit orders", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(p.Error)).To(BeTrue())
				}
				runTest("abo13_no_limit_no_cry_bo", true)
			})

			It("atomic perp market and limit order to not cross", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no perp orders")
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")

				atomicOrderValue := f2d(9999900) //no fee is retained, when taker order becomes a maker order
				expectedUsdtBalance1 := preUsdtBalance1.Sub(atomicOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("ABO.14 Atomic order matches with new post-only order (no batch)", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				runTest("abo14_post-only_no_batch_bo", true)
			})

			It("atomic bo market order matches a transient po", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no bo orders")
				verifyDerivativePosition(0, 10, true)

				atomicOrderValue := calculateMarginHoldForTakerOrder(f2d(10), f2d(10), f2d(100), te.MarketType_binary, true, nil)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(atomicOrderValue).String()), "quote asset balance for user 0")

				sellOrderExpectedValue := calculateExpectedValueForMakerOrder(f2d(10), f2d(10), f2d(9999900), te.MarketType_binary, false)
				expectedUsdtBalance1 := preUsdtBalance1.Sub(sellOrderExpectedValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1")
			})
		})

		Context("ABO.15 Atomic RO order cannot be placed without open position in opposite direction", func() {
			var preUsdtBalance0 sdk.Dec
			var preUsdtBalance1 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
					preUsdtBalance1 = getAvailableUSDTBalancePlusBank(1)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					Expect(p.Error).ToNot(BeNil())
					Expect(types.ErrPositionNotFound.Is(p.Error)).To(BeTrue(), "atomic order should be rejected")
				}
				runTest("abo15_ro_no_position_bo", true)
			})

			It("atomic bo market order matches a transient po", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no bo orders")
				verifyDerivativePosition(0, 0, true)

				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.String()), "quote asset balance for user 0 did not change")
				atomicOrderValue := f2d(9999900) //no fee is retained, when taker order becomes a maker order
				expectedUsdtBalance1 := preUsdtBalance1.Sub(atomicOrderValue)
				Expect(getAvailableUSDTBalancePlusBank(1).String()).To(Equal(expectedUsdtBalance1.String()), "quote asset balance for user 1 should decrease")
			})
		})

		Context("ABO.17 Atomic RO order can be placed with open position in opposite direction", func() {
			var preUsdtBalance0 sdk.Dec
			BeforeEach(func() {
				hooks["pre-setup"] = func(p *te.TestPlayerHookParams) {
					preUsdtBalance0 = getAvailableUSDTBalancePlusBank(0)
				}
				hooks["atomic"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateBinaryOptionsMarketOrderResponse)
					expectedFeeRate := getTakerFeeRateForMarket(te.MarketType_binary).Mul(DEFAULT_ATOMIC_MULTILIER)
					expectedFee := response.Results.PositionDelta.ExecutionPrice.Mul(response.Results.Quantity).Mul(expectedFeeRate)
					Expect(response.Results.Fee.String()).To(Equal(expectedFee.String()), "fee for atomic order should be correct")
				}
				runTest("abo17_ro_opposite_position_bo", true)
			})

			It("atomic perp market order matches a transient po", func() {
				Expect(len(getAllDerivativeOrdersSorted())).To(Equal(0), "there should be no bo orders")
				verifyDerivativePosition(0, 0, true)

				exepctedAtomicRoOrderFee := f2d(3).Mul(f2d(10)).Mul(getTakerFeeRateForMarket(te.MarketType_binary).Mul(DEFAULT_ATOMIC_MULTILIER))
				expectedPositionOpeningFee := f2d(3).Mul(f2d(10)).Mul(getTakerFeeRateForMarket(te.MarketType_binary))
				exepctedUsdtBalance0change := exepctedAtomicRoOrderFee.Add(expectedPositionOpeningFee)
				Expect(getAvailableUSDTBalancePlusBank(0).String()).To(Equal(preUsdtBalance0.Sub(exepctedUsdtBalance0change).String()), "quote asset balance for user 0 did not change")
			})
		})

		Context("there's an atomic binary order in the block", func() {
			BeforeEach(func() {
				hooks["atomicOrder"] = func(p *te.TestPlayerHookParams) {
					response := p.Response.(*types.MsgCreateBinaryOptionsMarketOrderResponse)
					Expect(response.Results.Quantity.Float64()).To(Equal(8.0))
					Expect(response.Results.Fee.Float64()).To(Equal(120.0))
					Expect(response.Results.PositionDelta.ExecutionPrice.Float64()).To(Equal(1000.0))
					Expect(response.Results.Payout.Float64()).To(Equal(0.0))
				}
				hooks["otherMarketOrder"] = func(p *te.TestPlayerHookParams) {
					err := p.Error
					Expect(err).ToNot(BeNil())
					Expect(types.ErrSlippageExceedsWorstPrice.Is(err)).To(BeTrue())
				}
				runTest("test_atomic_binary_market_01", true)
			})

			It("atomic should be executed before others", func() {
				verifyDerivativePosition(0, 8, true)
				verifyDerivativePosition(1, 8, false)
				verifyDerivativePosition(2, 0, true)
			})
		})
	})

	Context("Test atomic-orders smart-contract", func() {

		BeforeEach(func() {
			runTest("test_spot_sc_atomic", true)
		})
		It("should execute", func() {
			if !testexchange.IsUsingDefaultSubaccount() {
				Skip("only makes sense with default subaccount")
			}
			market := injectiveApp.ExchangeKeeper.GetSpotMarket(ctx, marketId, true)
			takerRate := market.TakerFeeRate
			multiplier := injectiveApp.ExchangeKeeper.GetMarketAtomicExecutionFeeMultiplier(ctx, marketId, types.MarketType_Spot)
			feeRate := takerRate.Mul(multiplier)
			fmt.Printf("Taker: %v, Multiplier: %v, Fee: %v, expectedFee: %v \n", takerRate.MustFloat64(), multiplier.MustFloat64(), feeRate, f2d(10000).Mul(feeRate))
			te.VerifyEqualDecs(tp.GetAvailableDepositPlusBankChange(0, "ETH0"), f2d(8.0), "Should have bought 8 units")
			te.VerifyEqualDecs(tp.GetAvailableDepositPlusBankChange(0, "USDT0"), sdk.MustNewDecFromStr("-8036"), "Should have cost that much")
		})

	})
})
