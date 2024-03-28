package testexchange

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
)

// Use higher dust for longer fuzz test runs, e.g. 1 USD
var dust = sdk.MustNewDecFromStr("1")

var DefaultInvariantChecks = InvariantChecks{
	EachActionCheck: func(action TestAction, actionIndex int, tp *TestPlayer) {
		tp.InvariantChecker2(action.GetActionId())
		tp.InvariantChecker3(action.GetActionId())
		if action.GetActionType() == ActionType_endblocker {
			tp.InvariantChecker1(action.GetActionId())
			tp.InvariantChecker4(action.GetActionId())
			tp.InvariantChecker6(action.GetActionId())
		}
		tp.InvariantChecker5(action.GetActionId())
		tp.InvariantChecker6("")
		tp.InvariantChecker6("")
		if actionIndex == len(tp.Recording.Actions)-1 {
			tp.InvariantChecker6(action.GetActionId())
		}
		tp.InvariantCheckOrderbookLevels(action.GetActionId())
	},
	FinalCheck: func(tp *TestPlayer) {
		fmt.Fprintf(GinkgoWriter, "\n\n\nPerformed actions report:\n\n\n")

		for key, number := range tp.NumOperationsByAction {
			fmt.Fprintf(GinkgoWriter, "%s: %d (failed: %d, attempts: %d) \n", key, tp.SuccessActions[key], tp.FailedActions[key], number)
		}

		fmt.Fprintln(GinkgoWriter, "\ninitialBlockTime", tp.InitialBlockTime)
		fmt.Fprintln(GinkgoWriter, "initialBlockHeight", tp.InitialBlockHeight)
		fmt.Fprintln(GinkgoWriter, "finalBlockTime", tp.Ctx.BlockTime())
		fmt.Fprintln(GinkgoWriter, "finalBlockHeight", tp.Ctx.BlockHeight())

		tp.InvariantChecker1("")
		tp.InvariantChecker2("")
		tp.InvariantChecker3("")
		tp.InvariantChecker4("")
		tp.InvariantChecker5("")
		tp.InvariantChecker6("")
		tp.InvariantCheckOrderbookLevels("")

		TestingExchangeParams.DefaultHourlyFundingRateCap = sdk.NewDecWithPrec(625, 6)
	},
}

func (tp *TestPlayer) InvariantChecker1(actionId string) {
	// if actionId != "" {
	// 	fmt.Println("executed action: ", actionId)
	// }
	// tp.Ctx, _ = EndBlockerAndCommit(tp.App, tp.Ctx)
	Expect(tp.App.ExchangeKeeper.IsMetadataInvariantValid(tp.Ctx)).To(BeTrue())
}

func (tp *TestPlayer) InvariantChecker2(actionId string) {
	ctx := tp.Ctx
	app := tp.App
	// if actionId != "" {
	// 	fmt.Println("executed action: ", actionId)
	// }
	currentBankSupply := sdk.Coins{}
	app.BankKeeper.IterateTotalSupply(ctx, func(coin sdk.Coin) bool {
		if coin.Denom != "inj" {
			currentBankSupply = currentBankSupply.Add(coin)
		}
		return false
	})

	// ensure Funds Total Supply = sum_of(individual_accounts) + exchange module account + insurance fund account + auction module account
	individualAccountsBalance := sdk.Coins{}
	bankGenesis := app.BankKeeper.ExportGenesis(ctx)
	moduleAddresses := app.ModuleAccountAddrs()

	for _, balance := range bankGenesis.Balances {
		// skip module accounts
		if _, ok := moduleAddresses[balance.Address]; ok {
			continue
		}
		individualAccountsBalance = individualAccountsBalance.Add(balance.Coins...)

	}

	tmpRewardsSenderCoins := app.BankKeeper.GetAllBalances(ctx, exchangetypes.TempRewardsSenderAddress)

	// auction balances can be high for long runs, especially given high trading fees after param updates
	// for _, coin := range tmpRewardsSenderCoins {
	// 	maxValueFromRounding := sdk.NewInt(100_000)
	// 	Expect(coin.Amount.Sub(maxValueFromRounding).IsNegative()).To(BeTrue())
	// }

	individualAccountsBalance = individualAccountsBalance.Add(tmpRewardsSenderCoins...)

	exchangeModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(exchangetypes.ModuleName))
	insuranceModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(insurancetypes.ModuleName))
	distributionModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(distributiontypes.ModuleName))
	auctionModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(auctiontypes.ModuleName))
	sumBalances := exchangeModuleAccountBalance.Add(insuranceModuleAccountBalance...).Add(individualAccountsBalance...).Add(distributionModuleAccountBalance...).Add(auctionModuleAccountBalance...)

	allBalances := tp.InitialBankSupply.Add(sumBalances...)
	injAmount := allBalances.AmountOf("inj")
	injCoins := sdk.NewCoin("inj", injAmount)
	allBalances = allBalances.Sub(injCoins)
	Expect(currentBankSupply.String()).To(BeEquivalentTo(allBalances.String()))
}

func (tp *TestPlayer) InvariantChecker3(actionId string) {
	ctx := tp.Ctx
	app := tp.App
	// if actionId != "" {
	// 	fmt.Println("executed action: ", actionId)
	// }
	// ensure insurance fund module account balance = sum_of(individual_insurance_fund_records)
	insuranceFundSum := sdk.NewCoins()
	insuranceFunds := app.InsuranceKeeper.GetAllInsuranceFunds(ctx)
	for _, fund := range insuranceFunds {
		insuranceFundSum = insuranceFundSum.Add(sdk.NewCoin(fund.DepositDenom, fund.Balance))
	}
	insuranceRedemptions := app.InsuranceKeeper.GetAllInsuranceFundRedemptions(ctx)
	for _, redemption := range insuranceRedemptions {
		insuranceFundSum = insuranceFundSum.Add(redemption.RedemptionAmount)
	}
	insuranceModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(insurancetypes.ModuleName))

	insuranceModuleAccountBalance = removeLPTokens(insuranceModuleAccountBalance)

	if len(insuranceModuleAccountBalance) > 0 {
		Expect(insuranceModuleAccountBalance).To(BeEquivalentTo(insuranceFundSum))
	} else {
		Expect(insuranceFundSum).To(BeEmpty())
	}
}

// removes LP tokens generated by the insurance module with share denom: (i.e. "...shareX")
func removeLPTokens(coins sdk.Coins) sdk.Coins {
	nonLpTokens := sdk.NewCoins()
	for _, coin := range coins {
		if !strings.Contains(coin.String(), "share") {
			nonLpTokens = nonLpTokens.Add(coin)
		}
	}

	return nonLpTokens
}

func (tp *TestPlayer) InvariantChecker4(actionId string) {
	ctx := tp.Ctx
	app := tp.App
	//if actionId != "" {
	//	fmt.Println("executed action: ", actionId)
	//}

	// "right" side of invariant check
	exchangeModuleAccountDecBalance := make(map[string]math.Int)

	// check sum of total balance of deposits of individual subaccounts + margin of all positions == exchange module account balance
	exchangeModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(exchangetypes.ModuleName))

	for _, coin := range exchangeModuleAccountBalance {
		if exchangeModuleAccountDecBalance[coin.Denom].IsNil() {
			exchangeModuleAccountDecBalance[coin.Denom] = sdk.ZeroInt()
		}

		exchangeModuleAccountDecBalance[coin.Denom] = exchangeModuleAccountDecBalance[coin.Denom].Add(coin.Amount)
	}

	// check total_balance >= available_balance
	totalDeposit := make(map[string]sdk.Dec)

	balances := app.ExchangeKeeper.GetAllExchangeBalances(ctx)
	for _, balance := range balances {
		Expect(balance.Deposits.TotalBalance.Add(dust).IsNegative()).To(BeFalse(), fmt.Sprintf("subaccount %s had negative total balance of %v", balance.SubaccountId, balance.Deposits.TotalBalance.String()))

		// comment out when running long fuzz tests as it can become negative due to matchedFeePriceDeltaRefundOr*Charge*
		Expect(balance.Deposits.AvailableBalance.IsNegative()).To(BeFalse(), balance.Deposits.AvailableBalance.String())
		// allow dust amount up than available balance
		Expect(
			balance.Deposits.TotalBalance.Add(dust).GTE(balance.Deposits.AvailableBalance),
		).To(BeTrue(), "diff of "+balance.Deposits.TotalBalance.Sub(balance.Deposits.AvailableBalance).String()+" | "+balance.Deposits.TotalBalance.String()+" "+balance.Deposits.AvailableBalance.String(), balance.Denom, balance.SubaccountId)

		if totalDeposit[balance.Denom].IsNil() {
			totalDeposit[balance.Denom] = sdk.ZeroDec()
		}

		totalDeposit[balance.Denom] = totalDeposit[balance.Denom].Add(balance.Deposits.TotalBalance)
	}

	positions := app.ExchangeKeeper.GetAllPositions(ctx)
	totalMargin := make(map[string]sdk.Dec)

	positionDirectionQuantities := make(map[common.Hash]sdk.Dec)

	for _, p := range positions {
		referencePrice := positions[0].Position.EntryPrice
		marketID := common.HexToHash(p.MarketId)

		var quoteDenom string
		//isBinaryMarket := false
		market := app.ExchangeKeeper.GetDerivativeMarket(ctx, marketID, true)
		if market != nil {
			quoteDenom = market.GetQuoteDenom()
		} else if tp.TestInput.MarketIDToPerpMarket[marketID] != nil {
			quoteDenom = tp.TestInput.MarketIDToPerpMarket[marketID].QuoteDenom
		} else if tp.TestInput.MarketIDToExpiryMarket[marketID] != nil {
			quoteDenom = tp.TestInput.MarketIDToExpiryMarket[marketID].QuoteDenom
		} else {
			quoteDenom = tp.TestInput.MarketIDToBinaryMarket[marketID].QuoteDenom
			//isBinaryMarket = true
		}

		if d := positionDirectionQuantities[marketID]; d.IsNil() {
			positionDirectionQuantities[marketID] = sdk.ZeroDec()
		}
		if p.Position.IsLong {
			positionDirectionQuantities[marketID] = positionDirectionQuantities[marketID].Add(p.Position.Quantity)
		} else {
			positionDirectionQuantities[marketID] = positionDirectionQuantities[marketID].Sub(p.Position.Quantity)
		}
		if p.Position.Margin.IsNegative() {
			// Can happen with funding payments over time
			fmt.Fprintln(GinkgoWriter, "[WARN]: Negative Margin", p.String())
		}

		funding := app.ExchangeKeeper.GetPerpetualMarketFunding(ctx, marketID)
		payout := p.Position.Margin

		//if !isBinaryMarket {
		if p.Position.IsLong {
			payout = payout.Add(p.Position.Quantity.Mul(referencePrice.Sub(p.Position.EntryPrice)))
		} else {
			payout = payout.Sub(p.Position.Quantity.Mul(referencePrice.Sub(p.Position.EntryPrice)))
		}

		if funding != nil {
			fundingPayment := p.Position.Quantity.Mul(funding.CumulativeFunding.Sub(p.Position.CumulativeFundingEntry))

			if p.Position.IsLong {
				fundingPayment = fundingPayment.Neg()
			}

			payout = payout.Add(fundingPayment)
		}
		//}

		//market := app.ExchangeKeeper.GetDerivativeMarket(Ctx, marketID, true)
		//if market != nil {
		//	quoteDenom = market.QuoteDenom
		//} else if TestInput.MarketIDToPerpMarket[marketID] != nil {
		//	quoteDenom = TestInput.MarketIDToPerpMarket[marketID].QuoteDenom
		//} else if TestInput.MarketIDToExpiryMarket[marketID] != nil {
		//	quoteDenom = TestInput.MarketIDToExpiryMarket[marketID].QuoteDenom
		//} else {
		//	quoteDenom = TestInput.MarketIDToBinaryMarket[marketID].QuoteDenom
		//}

		if totalMargin[quoteDenom].IsNil() {
			totalMargin[quoteDenom] = sdk.ZeroDec()
		}

		totalMargin[quoteDenom] = totalMargin[quoteDenom].Add(payout)

		// expectedMargin := GetRequiredBinaryOptionsMargin(p.Position, TestInput.BinaryMarkets[0].OracleScaleFactor)
		// fmt.Printf("Position  denom: %v isLong: %v entry price: %v\n", quoteDenom, p.Position.IsLong, p.Position.EntryPrice)
		// fmt.Printf("{\"price\": %v, \"quantity\": %d, \"margin\": %v, \"subaccountNonce\": %v}\n",
		//	p.Position.EntryPrice, p.Position.Quantity.RoundInt(), p.Position.Margin.RoundInt(), p.SubaccountId[2:7])

		// fmt.Printf("Position (%v): long: %v, margin: %v, expected margin: %v, quantity: %v, entryPrice: %v, \n",
		//	p.SubaccountId[2:7], p.Position.IsLong, p.Position.Margin.RoundInt().QuoRaw(100), expectedMargin.RoundInt().QuoRaw(100),
		//	p.Position.Quantity.RoundInt(), p.Position.EntryPrice.RoundInt().QuoRaw(100))
	}

	for _, v := range positionDirectionQuantities {
		Expect(v.IsZero(), v.String())
	}

	exchangeApplicationBalances := make(map[string]sdk.Dec)

	for denom, amount := range totalDeposit {
		marginAmount, found := totalMargin[denom]
		if !found {
			marginAmount = sdk.ZeroDec()
		}
		totalDenomBalance := amount.Add(marginAmount)
		exchangeApplicationBalances[denom] = totalDenomBalance
	}

	// exchangeApplicationBalances := totalDeposit
	//
	//for denom, amount := range totalMargin {
	//	exchangeApplicationBalances[denom] = exchangeApplicationBalances[denom].Add(amount)
	//}
	//
	for denom, logicAmount := range exchangeApplicationBalances {
		decBalance := exchangeModuleAccountDecBalance[denom]

		if decBalance.IsNil() {
			continue
		}
		moduleAmount := decBalance.ToDec()

		// arrives from rounding errors when adding up balances inside fuzz test
		// also possibly due to some unknown issues, for long runs should stay below 10000 USDT
		// dustFactor := int64(10000)
		dustFactor := int64(100)
		// dustFactor := int64(1)

		if logicAmount.Sub(moduleAmount).Abs().GT(sdk.NewDec(dustFactor)) {
			errMessage := fmt.Sprintf("🥶 (dep: %v + mar: %v == %s) != %s. Diff of %s %s", totalDeposit[denom], totalMargin[denom], logicAmount.String(), moduleAmount.String(), logicAmount.Sub(moduleAmount).String(), denom)
			fmt.Println(errMessage)
			Expect(!logicAmount.Sub(moduleAmount).Abs().GT(sdk.SmallestDec().MulInt64(dustFactor))).To(BeTrue(), errMessage)
			// fmt.Println(errMessage)
		}
		//else {
		//	if denom == "USDT0" {
		//		errMessage := fmt.Sprintf("🥶 (dep: %v + mar: %v == %s) != %s. Diff of %s %s", totalDeposit[denom], totalMargin[denom], logicAmount.String(), moduleAmount.String(), logicAmount.Sub(moduleAmount).String(), denom)
		//		fmt.Println(errMessage)
		//	}
		//}
	}
}

func (tp *TestPlayer) InvariantChecker5(actionId string) {
	ctx := tp.Ctx
	app := tp.App
	// if actionId != "" {
	// 	fmt.Println("executed action: ", actionId)
	// }

	allAccountVolume := app.ExchangeKeeper.GetAllAccountVolumeInAllBuckets(ctx)

	allAccounts := make([]string, 0)
	accountFees := make(map[string]sdk.Dec)

	for _, accountVolume := range allAccountVolume {
		for _, feesInBucket := range accountVolume.AccountVolume {
			if _, ok := accountFees[feesInBucket.Account]; !ok {
				accountFees[feesInBucket.Account] = sdk.ZeroDec()
			} else {
				allAccounts = append(allAccounts, feesInBucket.Account)
			}

			if accountFees[feesInBucket.Account].IsNil() {
				accountFees[feesInBucket.Account] = sdk.ZeroDec()
			}

			accountFees[feesInBucket.Account] = accountFees[feesInBucket.Account].Add(feesInBucket.Volume)
		}
	}

	currBucketStartTimestamp := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	for _, account := range allAccounts {
		accAccount, err := sdk.AccAddressFromBech32(account)
		OrFail(err)

		accountFeesInStore := app.ExchangeKeeper.GetFeeDiscountTotalAccountVolume(ctx, accAccount, currBucketStartTimestamp)
		Expect(accountFeesInStore.String()).Should(Equal(accountFees[account].String()))
	}
}

func (tp *TestPlayer) InvariantChecker6(actionId string) {
	ctx := tp.Ctx
	app := tp.App
	//if actionId != "" {
	//	fmt.Println("executed action: ", actionId)
	//}

	binaryOptionsMarkets := app.ExchangeKeeper.GetAllBinaryOptionsMarkets(ctx)
	dustFactor, _ := sdk.NewDecFromStr("0.00001")

	for _, binaryOptionsMarket := range binaryOptionsMarkets {
		binaryOptionsPositions := app.ExchangeKeeper.GetAllPositionsByMarket(ctx, binaryOptionsMarket.MarketID())
		for _, binaryOptionsPosition := range binaryOptionsPositions {
			expectedMargin := GetRequiredBinaryOptionsMargin(binaryOptionsPosition.Position, binaryOptionsMarket.OracleScaleFactor)
			diff := binaryOptionsPosition.Position.Margin.Sub(expectedMargin)
			Expect(diff.Abs().LTE(dustFactor)).To(BeTrue(),
				fmt.Sprintf("Binary option position (own %v) expected margin: %v, margin: %v, diff: %v %v", binaryOptionsPosition.SubaccountId, expectedMargin, binaryOptionsPosition.Position.Margin, diff, actionId))
		}
	}
}

func (tp *TestPlayer) InvariantCheckOrderbookLevels(actionId string) {
	ctx := tp.Ctx
	app := tp.App
	keeper := app.ExchangeKeeper
	verifyOrderbooks := func(marketID common.Hash, isSpot bool) {
		var limit uint64 = 1000
		var computedBuys []*exchangetypes.Level
		var computedSells []*exchangetypes.Level
		if isSpot {
			computedBuys = keeper.GetComputedSpotLimitOrderbook(ctx, marketID, true, limit)
			computedSells = keeper.GetComputedSpotLimitOrderbook(ctx, marketID, false, limit)
		} else {
			computedBuys = keeper.GetComputedDerivativeLimitOrderbook(ctx, marketID, true, limit)
			computedSells = keeper.GetComputedDerivativeLimitOrderbook(ctx, marketID, false, limit)
		}
		metadataBuys := keeper.GetOrderbookPriceLevels(ctx, isSpot, marketID, true, &limit, nil, nil)
		metadataSells := keeper.GetOrderbookPriceLevels(ctx, isSpot, marketID, false, &limit, nil, nil)
		orderbookComputed := exchangetypes.NewOrderbookWithLevels(marketID, computedBuys, computedSells)
		orderbookMetadata := exchangetypes.NewOrderbookWithLevels(marketID, metadataBuys, metadataSells)

		if orderbookMetadata.IsCrossed() || !orderbookComputed.Equals(orderbookMetadata) {
			if orderbookMetadata.IsCrossed() {
				fmt.Fprintln(GinkgoWriter, "❌ Orderbook (metadata) is crossed")
				if orderbookComputed.IsCrossed() {
					fmt.Fprintln(GinkgoWriter, "❌ Computed orderbook is crossed!!!")
				}
			} else {
				fmt.Fprintln(GinkgoWriter, "Metadata orderbook doesn't match computed orderbook")
			}
			fmt.Fprintln(GinkgoWriter, "Orderbook metadata, marketID:", common.BytesToHash(orderbookMetadata.MarketId))
			orderbookMetadata.PrintDisplay()

			fmt.Fprintln(GinkgoWriter, "Orderbook computed")
			orderbookComputed.PrintDisplay()

			panic("shite")
		}
	}
	for _, market := range tp.TestInput.Spots {
		verifyOrderbooks(market.MarketID, true)
	}
	for _, market := range tp.TestInput.Perps {
		verifyOrderbooks(market.MarketID, false)
	}
	for _, market := range tp.TestInput.ExpiryMarkets {
		verifyOrderbooks(market.MarketID, false)
	}
	for _, market := range tp.TestInput.BinaryMarkets {
		verifyOrderbooks(market.MarketID, false)
	}
}
