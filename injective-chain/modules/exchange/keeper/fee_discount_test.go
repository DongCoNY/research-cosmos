package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Fee Discount Tests", func() {
	var (
		testInput              testexchange.TestInput
		app                    *simapp.InjectiveApp
		ctx                    sdk.Context
		err                    error
		traderAddress          = testexchange.SampleAccountAddr1
		traderSubbacountBuyer  *common.Hash
		traderSubaccountSeller *common.Hash
		otherTraderAddress     = testexchange.SampleAccountAddr2
		otherTraderubaccountID = testexchange.SampleSubaccountAddr2
	)

	traderSubbacountBuyer, _ = exchangetypes.SdkAddressWithNonceToSubaccountID(traderAddress, 0)
	traderSubaccountSeller, _ = exchangetypes.SdkAddressWithNonceToSubaccountID(traderAddress, 1)

	var simulateTrading = func() {
		msgServer := exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)

		price := sdk.NewDec(2000)
		quantity := sdk.NewDec(3)
		margin := sdk.NewDec(1000)

		testInput.MintDerivativeDeposits(app, ctx, 0, *traderSubbacountBuyer, nil, false)
		testInput.MintDerivativeDeposits(app, ctx, 0, *traderSubaccountSeller, nil, false)

		matchBuyerAndSeller(testInput, app, ctx, msgServer, exchangetypes.MarketType_Perpetual, margin, quantity, price, true, *traderSubbacountBuyer, *traderSubaccountSeller)
	}

	var setDelegation = func(trader sdk.AccAddress, stakeAmount math.Int) {
		msgServer := stakingkeeper.NewMsgServerImpl(app.StakingKeeper)
		valAddr, err := sdk.ValAddressFromBech32(testexchange.DefaultValidatorAddress)
		testexchange.OrFail(err)

		// undelegate previous delegations
		delegation, found := app.StakingKeeper.GetDelegation(ctx, traderAddress, valAddr)
		if found {
			val, found := app.StakingKeeper.GetValidator(ctx, valAddr)
			if !found {
				panic(1)
			}
			staked := val.TokensFromShares(delegation.Shares).TruncateInt()
			_, err = msgServer.Undelegate(sdk.WrapSDKContext(ctx), &stakingtypes.MsgUndelegate{
				DelegatorAddress: trader.String(),
				ValidatorAddress: testexchange.DefaultValidatorAddress,
				Amount:           sdk.NewCoin("inj", staked),
			})
			testexchange.OrFail(err)
		}

		// delegate new coins
		stakedCoins := sdk.NewCoins(sdk.NewCoin("inj", stakeAmount))
		err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, stakedCoins)
		testexchange.OrFail(err)
		err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, traderAddress, stakedCoins)
		testexchange.OrFail(err)

		delegateMsg := &stakingtypes.MsgDelegate{
			DelegatorAddress: trader.String(),
			ValidatorAddress: testexchange.DefaultValidatorAddress,
			Amount:           sdk.NewCoin("inj", stakeAmount),
		}
		_, err = msgServer.Delegate(sdk.WrapSDKContext(ctx), delegateMsg)
		testexchange.OrFail(err)
	}

	var simulateTradingWithNewAccount = func() {
		msgServer := exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)

		price := sdk.NewDec(2000)
		quantity := sdk.NewDec(3)
		margin := sdk.NewDec(1000)

		testInput.MintDerivativeDeposits(app, ctx, 0, *traderSubaccountSeller, nil, false)
		testInput.MintDerivativeDeposits(app, ctx, 0, *traderSubbacountBuyer, nil, false)
		testInput.MintDerivativeDeposits(app, ctx, 0, otherTraderubaccountID, nil, false)

		limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity.MulInt64(2), margin, exchangetypes.OrderType_BUY, otherTraderubaccountID)
		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
		testexchange.OrFail(err)

		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		amountToDelegate := sdk.NewInt(100)
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(sdk.NewCoin("inj", amountToDelegate)))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, otherTraderAddress, sdk.NewCoins(sdk.NewCoin("inj", amountToDelegate)))
		setDelegation(otherTraderAddress, amountToDelegate)

		timestamp1 := ctx.BlockTime().Unix() + 110
		ctx = ctx.WithBlockTime(time.Unix(timestamp1, 0))
		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, exchangetypes.OrderType_SELL, *traderSubaccountSeller)
		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
		testexchange.OrFail(err)

		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		timestamp1 = ctx.BlockTime().Unix() + 110
		ctx = ctx.WithBlockTime(time.Unix(timestamp1, 0))
		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

		limitDerivativeSellOrder = testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, exchangetypes.OrderType_SELL, *traderSubaccountSeller)
		_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
		testexchange.OrFail(err)

		ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	}

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)

		startingPrice := sdk.NewDec(2000)
		app.OracleKeeper.SetPriceFeedPriceState(ctx, testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		perpCoin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(perpCoin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(perpCoin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(
			ctx,
			sender,
			perpCoin,
			testInput.Perps[0].Ticker,
			testInput.Perps[0].QuoteDenom,
			testInput.Perps[0].OracleBase,
			testInput.Perps[0].OracleQuote,
			testInput.Perps[0].OracleType,
			-1,
		))

		_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			testInput.Perps[0].Ticker,
			testInput.Perps[0].QuoteDenom,
			testInput.Perps[0].OracleBase,
			testInput.Perps[0].OracleQuote,
			0,
			testInput.Perps[0].OracleType,
			testInput.Perps[0].InitialMarginRatio,
			testInput.Perps[0].MaintenanceMarginRatio,
			testInput.Perps[0].MakerFeeRate,
			testInput.Perps[0].TakerFeeRate,
			testInput.Perps[0].MinPriceTickSize,
			testInput.Perps[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)
	})

	Describe("when setting past volume buckets", func() {
		Context("when there are no past buckets", func() {
			BeforeEach(func() {
				bucketCount := uint64(5)
				bucketDuration := int64(100)
				tierInfos := []*exchangetypes.FeeDiscountTierInfo{{
					MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
					TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
					StakedAmount:      sdk.NewInt(100),
					Volume:            sdk.MustNewDecFromStr("0.3"),
				}}

				testexchange.AddFeeDiscountWithSettingPastBuckets(
					testInput,
					app,
					ctx,
					[]string{testInput.Perps[0].MarketID.Hex()},
					[]string{testInput.Perps[0].QuoteDenom},
					false,
					tierInfos,
					bucketCount,
					bucketDuration,
					traderAddress,
				)
				app.ExchangeKeeper.SetFeeDiscountCurrentBucketStartTimestamp(ctx, ctx.BlockTime().Unix()+100)
			})

			It("should set and advance bucket volumes correctly", func() {
				currBucketStartTimestampBeforeTrading := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
				currBucketVolumeBeforeTrading := app.ExchangeKeeper.GetFeeDiscountAccountVolumeInBucket(ctx, currBucketStartTimestampBeforeTrading, traderAddress)
				pastBucketVolumeBeforeTrading := app.ExchangeKeeper.GetPastBucketTotalVolume(ctx, traderAddress)

				timestamp1 := ctx.BlockTime().Unix() + 110
				ctx = ctx.WithBlockTime(time.Unix(timestamp1, 0))
				simulateTrading()

				currBucketStartTimestampAfterTrading := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
				currBucketVolumeAfterTrading := app.ExchangeKeeper.GetFeeDiscountAccountVolumeInBucket(ctx, currBucketStartTimestampAfterTrading, traderAddress)
				pastBucketVolumeAfterTrading := app.ExchangeKeeper.GetPastBucketTotalVolume(ctx, traderAddress)

				allVolumesAfterTrading := app.ExchangeKeeper.GetAllAccountVolumeInAllBuckets(ctx)

				oldBucketVolume := sdk.ZeroDec()
				newBucketVolume := sdk.MustNewDecFromStr("12000")
				Expect(currBucketVolumeBeforeTrading.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(pastBucketVolumeBeforeTrading.String()).Should(Equal(oldBucketVolume.String()))
				Expect(currBucketVolumeAfterTrading.String()).Should(Equal(newBucketVolume.String()))
				Expect(pastBucketVolumeAfterTrading.String()).Should(Equal(oldBucketVolume.String()))

				expectedVolume := []sdk.Dec{newBucketVolume}
				for i, volumeAfter := range allVolumesAfterTrading {
					for _, volume := range volumeAfter.AccountVolume {
						Expect(volume.Account).Should(Equal(traderAddress.String()))
						Expect(volume.Volume.String()).Should(Equal(expectedVolume[i].String()))
					}
				}
			})

			It("should set and advance bucket volumes correctly with caching", func() {
				discountedFeeRateBefore, err := app.ExchangeKeeper.FeeDiscountAccountInfo(sdk.WrapSDKContext(ctx), &exchangetypes.QueryFeeDiscountAccountInfoRequest{
					Account: otherTraderAddress.String(),
				})
				testexchange.OrFail(err)

				timestamp1 := ctx.BlockTime().Unix() + 110
				ctx = ctx.WithBlockTime(time.Unix(timestamp1, 0))
				simulateTradingWithNewAccount()

				discountedFeeRateAfter, err := app.ExchangeKeeper.FeeDiscountAccountInfo(sdk.WrapSDKContext(ctx), &exchangetypes.QueryFeeDiscountAccountInfoRequest{
					Account: otherTraderAddress.String(),
				})
				testexchange.OrFail(err)

				Expect(discountedFeeRateBefore.TierLevel).Should(BeZero())
				Expect(discountedFeeRateBefore.AccountInfo.Volume.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(discountedFeeRateBefore.AccountInfo.StakedAmount.String()).Should(Equal(sdk.ZeroInt().String()))

				Expect(discountedFeeRateAfter.TierLevel).Should(Equal(uint64(1)))
				Expect(discountedFeeRateAfter.AccountInfo.Volume.IsPositive()).Should(BeTrue())
				Expect(discountedFeeRateAfter.AccountInfo.StakedAmount.String()).Should(Equal(sdk.NewInt(100).String()))
			})
		})

		Context("when there are full past buckets", func() {
			BeforeEach(func() {
				testexchange.AddFeeDiscountForAddress(testInput, app, ctx, []string{testInput.Perps[0].MarketID.Hex()}, []string{testInput.Perps[0].QuoteDenom}, traderAddress)
				app.ExchangeKeeper.SetFeeDiscountCurrentBucketStartTimestamp(ctx, ctx.BlockTime().Unix()+100)
			})

			It("should set and advance bucket volumes correctly", func() {
				currBucketStartTimestampBeforeTrading := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
				currBucketVolumeBeforeTrading := app.ExchangeKeeper.GetFeeDiscountAccountVolumeInBucket(ctx, currBucketStartTimestampBeforeTrading, traderAddress)
				pastBucketVolumeBeforeTrading := app.ExchangeKeeper.GetPastBucketTotalVolume(ctx, traderAddress)

				timestamp := ctx.BlockTime().Unix() + 210
				ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))
				simulateTrading()

				currBucketStartTimestampAfterTrading := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx) - 100
				currBucketVolumeAfterTrading := app.ExchangeKeeper.GetFeeDiscountAccountVolumeInBucket(ctx, currBucketStartTimestampAfterTrading, traderAddress)
				pastBucketVolumeAfterTrading := app.ExchangeKeeper.GetPastBucketTotalVolume(ctx, traderAddress)

				allVolumesAfterTrading := app.ExchangeKeeper.GetAllAccountVolumeInAllBuckets(ctx)

				oldestBucketVolume := sdk.NewDec(5)
				oldBucketVolume := sdk.NewDec(15)
				newBucketVolume := sdk.MustNewDecFromStr("12000")
				Expect(currBucketVolumeBeforeTrading.String()).Should(Equal(sdk.ZeroDec().String()))
				Expect(pastBucketVolumeBeforeTrading.String()).Should(Equal(oldBucketVolume.String()))
				Expect(currBucketVolumeAfterTrading.String()).Should(Equal(newBucketVolume.String()))
				Expect(pastBucketVolumeAfterTrading.String()).Should(Equal(oldBucketVolume.Add(newBucketVolume).Sub(oldestBucketVolume).String()))

				expectedVolume := []sdk.Dec{sdk.NewDec(4), sdk.NewDec(3), sdk.NewDec(2), sdk.NewDec(1), newBucketVolume}
				for i, volumeAfter := range allVolumesAfterTrading {
					for _, volume := range volumeAfter.AccountVolume {
						Expect(volume.Account).Should(Equal(traderAddress.String()))
						Expect(volume.Volume.String()).Should(Equal(expectedVolume[i].String()))
					}
				}
			})
		})
	})

	Describe("when getting actively traded accounts", func() {
		BeforeEach(func() {
			testexchange.AddFeeDiscountForAddress(testInput, app, ctx, []string{testInput.Perps[0].MarketID.Hex()}, []string{testInput.Perps[0].QuoteDenom}, traderAddress)
		})

		It("should get the correct accounts", func() {
			testInput.AddDerivativeDepositsForSubaccounts(app, ctx, 1, nil, false, []common.Hash{*traderSubbacountBuyer, *traderSubaccountSeller})

			price := sdk.NewDec(2000)
			quantity := sdk.NewDec(3)
			margin := sdk.NewDec(1000)
			msgServer := exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)
			limitDerivativeBuyOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, exchangetypes.OrderType_BUY, *traderSubbacountBuyer)
			limitDerivativeSellOrder := testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, exchangetypes.OrderType_SELL, *traderSubaccountSeller)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeBuyOrder)
			testexchange.OrFail(err)
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), limitDerivativeSellOrder)
			testexchange.OrFail(err)

			activeAccounts := app.ExchangeKeeper.GetAllAccountsActivelyTradingQualifiedMarketsInBlockForFeeDiscounts(ctx)
			Expect(activeAccounts).Should(HaveLen(1))
			Expect(activeAccounts[0].String()).Should(Equal(traderAddress.String()))
		})
	})

	Describe("when getting the fee discount tier", func() {
		var (
			bucketCount       uint64
			bucketDuration    int64
			stakingConfig     *exchangekeeper.FeeDiscountConfig
			isMaker           bool
			pastVolumeBuckets []sdk.Dec
			stakedAmount      math.Int
			discountedFeeRate sdk.Dec
		)

		tradingFeeRate := sdk.NewDecWithPrec(1, 3)
		tierInfos := []*exchangetypes.FeeDiscountTierInfo{{
			MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
			TakerDiscountRate: sdk.MustNewDecFromStr("0.15"),
			StakedAmount:      sdk.NewInt(100),
			Volume:            sdk.NewDec(3),
		}, {
			MakerDiscountRate: sdk.MustNewDecFromStr("0.2"),
			TakerDiscountRate: sdk.MustNewDecFromStr("0.25"),
			StakedAmount:      sdk.NewInt(200),
			Volume:            sdk.NewDec(6),
		}, {
			MakerDiscountRate: sdk.MustNewDecFromStr("0.3"),
			TakerDiscountRate: sdk.MustNewDecFromStr("0.35"),
			StakedAmount:      sdk.NewInt(300),
			Volume:            sdk.NewDec(9),
		}}

		BeforeEach(func() {
			bucketCount = uint64(5)
			bucketDuration = int64(100)
			testexchange.AddFeeDiscountWithSettingPastBuckets(
				testInput,
				app,
				ctx,
				[]string{testInput.Perps[0].MarketID.Hex()},
				[]string{testInput.Perps[0].QuoteDenom},
				false,
				tierInfos,
				bucketCount,
				bucketDuration,
				traderAddress,
			)
		})

		JustBeforeEach(func() {
			testexchange.SetPastBucketTotalVolumeForTrader(testInput, app, ctx, traderAddress, bucketDuration, pastVolumeBuckets)
			app.ExchangeKeeper.SetFeeDiscountCurrentBucketStartTimestamp(ctx, ctx.BlockTime().Unix()+100)

			currBucketStartTimestamp := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
			oldestBucketStartTimestamp := app.ExchangeKeeper.GetOldestBucketStartTimestamp(ctx)
			isFirstFeeCycleFinished := true
			maxTTLTimestamp := currBucketStartTimestamp
			nextTTLTimestamp := maxTTLTimestamp + app.ExchangeKeeper.GetFeeDiscountBucketDuration(ctx)

			stakingConfig = &exchangekeeper.FeeDiscountConfig{
				IsMarketQualified: true,
				FeeDiscountStakingInfo: exchangekeeper.NewFeeDiscountStakingInfo(
					&exchangetypes.FeeDiscountSchedule{
						TierInfos: tierInfos,
					},
					currBucketStartTimestamp,
					oldestBucketStartTimestamp,
					maxTTLTimestamp,
					nextTTLTimestamp,
					isFirstFeeCycleFinished,
				),
			}

			setDelegation(traderAddress, stakedAmount)
		})

		Describe("when its the lowest tier before all buckets are full as maker", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(99)
				isMaker = true
			})

			JustBeforeEach(func() {
				stakingConfig.IsFirstFeeCycleFinished = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when its the lowest tier before all buckets are full as taker", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(99)
				isMaker = false
			})

			JustBeforeEach(func() {
				stakingConfig.IsFirstFeeCycleFinished = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when its the highest tier before all buckets are full as maker", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(300)
				isMaker = true
			})

			JustBeforeEach(func() {
				stakingConfig.IsFirstFeeCycleFinished = false
			})

			It("should get the correct discounted fee rate", func() {
				app.ExchangeKeeper.SetIsFirstFeeCycleFinished(ctx, false)
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[2].MakerDiscountRate)).String()))
			})
		})

		Describe("when its the highest tier before all buckets are full as taker", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(300)
				isMaker = false
			})

			JustBeforeEach(func() {
				stakingConfig.IsFirstFeeCycleFinished = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[2].TakerDiscountRate)).String()))
			})
		})

		Describe("when its the lowest tier due to staked amount for makers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.MustNewDecFromStr("0.1"), sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(99)
				isMaker = true
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when its the lowest tier due to staked amount for takers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.MustNewDecFromStr("0.1"), sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(99)
				isMaker = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when its the lowest tier due to volume for makers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1"), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(100)
				isMaker = true
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when its the lowest tier due to volume for takers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1"), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(100)
				isMaker = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when its tier one due to staked amount for makers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(199)
				isMaker = true
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[0].MakerDiscountRate)).String()))
			})
		})

		Describe("when its tier one due to staked amount for takers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(199)
				isMaker = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[0].TakerDiscountRate)).String()))
			})
		})

		Describe("when its tier one due to volume for makers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(2), sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(200)
				isMaker = true
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[0].MakerDiscountRate)).String()))
			})
		})

		Describe("when its tier one due to volume for takers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(2), sdk.NewDec(1), sdk.NewDec(2), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")}
				stakedAmount = sdk.NewInt(200)
				isMaker = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[0].TakerDiscountRate)).String()))
			})
		})

		Describe("when its the highest tier for makers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.NewDec(2), sdk.NewDec(1)}
				stakedAmount = sdk.NewInt(300)
				isMaker = true
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[2].MakerDiscountRate)).String()))
			})
		})

		Describe("when its the highest tier for takers", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.NewDec(2), sdk.NewDec(1)}
				stakedAmount = sdk.NewInt(300)
				isMaker = false
			})

			It("should get the correct discounted fee rate", func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[2].TakerDiscountRate)).String()))
			})
		})

		Describe("when the tier is cached", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.NewDec(2), sdk.NewDec(1)}
				stakedAmount = sdk.NewInt(300)
				isMaker = true
			})

			JustBeforeEach(func() {
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
			})

			It("should get the correct discounted fee rate from cache", func() {
				oldDiscountedRate := discountedFeeRate
				stakingConfig.Schedule = nil
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(oldDiscountedRate.String()))
			})
		})

		Describe("when the lowest tier is stored in account tier info", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.NewDec(2), sdk.NewDec(1)}
				stakedAmount = sdk.NewInt(300)
				isMaker = true

				app.ExchangeKeeper.SetFeeDiscountAccountTierInfo(ctx, traderAddress, &exchangetypes.FeeDiscountTierTTL{
					Tier:         0,
					TtlTimestamp: ctx.BlockTime().Unix() + 110,
				})
			})

			It("should get the correct stored discounted fee rate", func() {
				stakingConfig.Schedule = nil
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.String()))
			})
		})

		Describe("when the highest tier is stored in account tier info", func() {
			BeforeEach(func() {
				pastVolumeBuckets = []sdk.Dec{sdk.NewDec(3), sdk.NewDec(1), sdk.NewDec(2), sdk.NewDec(2), sdk.NewDec(1)}
				stakedAmount = sdk.NewInt(300)
				isMaker = true

				app.ExchangeKeeper.SetFeeDiscountAccountTierInfo(ctx, traderAddress, &exchangetypes.FeeDiscountTierTTL{
					Tier:         3,
					TtlTimestamp: ctx.BlockTime().Unix() + 110,
				})
			})

			It("should get the correct stored discounted fee rate", func() {
				stakingConfig.Schedule = nil
				discountedFeeRate = app.ExchangeKeeper.FetchAndUpdateDiscountedTradingFeeRate(ctx, tradingFeeRate, isMaker, traderAddress, stakingConfig)
				Expect(discountedFeeRate.String()).Should(Equal(tradingFeeRate.Mul(sdk.OneDec().Sub(tierInfos[2].MakerDiscountRate)).String()))
			})
		})
	})
})
