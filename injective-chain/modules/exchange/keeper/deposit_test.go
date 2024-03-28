package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

// TODO test EventBatchDepositUpdate

var _ = Describe("Deposit", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		msgServer exchangetypes.MsgServer
		ctx       sdk.Context

		depositFromStore  *exchangetypes.Deposit
		deposit           *exchangetypes.Deposit
		subaccountDeposit *exchangetypes.Deposit
		externalDeposit   *exchangetypes.Deposit
		increment         sdk.Coin
		withdraw          sdk.Coin
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1")).String()
	subaccountIDStr := "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	subaccountID2Str := "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"
	externalAccountStr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F000000000000000000000001"
	subaccountId := common.HexToHash(subaccountIDStr)
	subaccountId2 := common.HexToHash(subaccountID2Str)
	externalAccount := common.HexToHash(externalAccountStr)

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
	})

	Context("Check if deposit exist", func() {
		It("the deposit should not be present", func() {
			deposit = app.ExchangeKeeper.GetDeposit(ctx, subaccountId, testInput.Spots[0].QuoteDenom)
			Expect(deposit).To(Equal(exchangetypes.NewDeposit()))
		})
	})

	Context("when a new deposit with amount A is added", func() {
		BeforeEach(func() {
			deposit = &exchangetypes.Deposit{
				AvailableBalance: sdk.NewDec(100),
				TotalBalance:     sdk.NewDec(100),
			}
			app.ExchangeKeeper.SetDepositOrSendToBank(ctx, subaccountId, testInput.Spots[0].QuoteDenom, *deposit, false)
			depositFromStore = app.ExchangeKeeper.GetDeposit(ctx, subaccountId, testInput.Spots[0].QuoteDenom)
		})

		It("available and total deposit balance should be A", func() {
			Expect(depositFromStore.AvailableBalance).To(Equal(deposit.AvailableBalance))
			Expect(depositFromStore.TotalBalance).To(Equal(deposit.TotalBalance))
		})

		Context("when existing deposit is incremented with amount B", func() {
			JustBeforeEach(func() {
				increment = sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.NewInt(15))
				app.ExchangeKeeper.IncrementDepositWithCoinOrSendToBank(ctx, subaccountId, increment)
				depositFromStore = app.ExchangeKeeper.GetDeposit(ctx, subaccountId, testInput.Spots[0].QuoteDenom)
			})

			It("the available and total deposit balance is incremented by B", func() {
				Expect(depositFromStore.AvailableBalance).To(Equal(deposit.AvailableBalance.Add(increment.Amount.ToDec())))
				Expect(depositFromStore.TotalBalance).To(Equal(deposit.TotalBalance.Add(increment.Amount.ToDec())))
			})
		})

		Context("when withdraw more than available balance", func() {
			var err error
			JustBeforeEach(func() {
				withdraw = sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.NewInt(depositFromStore.AvailableBalance.BigInt().Int64()+1))
				err = app.ExchangeKeeper.DecrementDeposit(ctx, subaccountId, withdraw.Denom, withdraw.Amount.ToDec())
			})
			It("Insufficient Deposit Error should be thrown", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when  partial available balance C is withdrawn", func() {
			var err error
			var depositAfterWithdraw *exchangetypes.Deposit
			JustBeforeEach(func() {
				withdraw = sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.NewInt(20))
				err = app.ExchangeKeeper.DecrementDeposit(ctx, subaccountId, withdraw.Denom, withdraw.Amount.ToDec())
				depositAfterWithdraw = app.ExchangeKeeper.GetDeposit(ctx, subaccountId, testInput.Spots[0].QuoteDenom)
			})

			It("the available and total deposit balance is decremented by C", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(depositAfterWithdraw.AvailableBalance).To(Equal(depositFromStore.AvailableBalance.Sub(sdk.NewDec(20))))
				Expect(depositAfterWithdraw.TotalBalance).To(Equal(depositFromStore.TotalBalance.Sub(sdk.NewDec(20))))
			})
		})
	})

	Context("subaccount transfer", func() {
		var err error
		BeforeEach(func() {
			deposit = &exchangetypes.Deposit{
				AvailableBalance: sdk.NewDec(100),
				TotalBalance:     sdk.NewDec(100),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountId.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, externalAccount.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			subaccountDeposit = testexchange.GetBankAndDepositFunds(app, ctx, subaccountId, testInput.Spots[0].QuoteDenom)
			externalDeposit = testexchange.GetBankAndDepositFunds(app, ctx, externalAccount, testInput.Spots[0].QuoteDenom)
		})

		Context("when transfer more than src subaccount amount", func() {
			JustBeforeEach(func() {
				_, err = msgServer.SubaccountTransfer(sdk.WrapSDKContext(ctx), &exchangetypes.MsgSubaccountTransfer{
					Sender:                  sender,
					SourceSubaccountId:      subaccountIDStr,
					DestinationSubaccountId: subaccountID2Str,
					Amount:                  sdk.NewInt64Coin(testInput.Spots[0].QuoteDenom, 1000000000),
				})
			})
			It("action should fail", func() {
				Expect(err).To(HaveOccurred())
			})
			It("source and destination amount should remain same", func() {
				srcAmt := testexchange.GetBankAndDepositFunds(app, ctx, subaccountId, testInput.Spots[0].QuoteDenom)
				dstAmt := testexchange.GetBankAndDepositFunds(app, ctx, subaccountId2, testInput.Spots[0].QuoteDenom)

				Expect(srcAmt.AvailableBalance).To(Equal(subaccountDeposit.AvailableBalance))
				Expect(srcAmt.TotalBalance).To(Equal(subaccountDeposit.TotalBalance))
				Expect(dstAmt).To(Equal(exchangetypes.NewDeposit()))
			})
		})

		Context("when transfer less than src subaccount amount", func() {
			var err error
			JustBeforeEach(func() {
				_, err = msgServer.SubaccountTransfer(sdk.WrapSDKContext(ctx), &exchangetypes.MsgSubaccountTransfer{
					Sender:                  sender,
					SourceSubaccountId:      subaccountIDStr,
					DestinationSubaccountId: subaccountID2Str,
					Amount:                  sdk.NewInt64Coin(testInput.Spots[0].QuoteDenom, 10),
				})
			})
			It("action should success and source and destination amount should be modified", func() {
				Expect(err).To(BeNil())
				srcAmt := testexchange.GetBankAndDepositFunds(app, ctx, subaccountId, testInput.Spots[0].QuoteDenom)
				dstAmt := testexchange.GetBankAndDepositFunds(app, ctx, subaccountId2, testInput.Spots[0].QuoteDenom)

				Expect(srcAmt.AvailableBalance).To(Equal(subaccountDeposit.AvailableBalance.Sub(sdk.NewDec(10))))
				Expect(srcAmt.TotalBalance).To(Equal(subaccountDeposit.TotalBalance.Sub(sdk.NewDec(10))))
				Expect(dstAmt.AvailableBalance).To(Equal(sdk.NewDec(10)))
				Expect(dstAmt.TotalBalance).To(Equal(sdk.NewDec(10)))
			})
		})
	})

	Context("external transfer", func() {
		var err error
		BeforeEach(func() {
			deposit = &exchangetypes.Deposit{
				AvailableBalance: sdk.NewDec(100),
				TotalBalance:     sdk.NewDec(100),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountId.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, externalAccount.String(), sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			subaccountDeposit = testexchange.GetBankAndDepositFunds(app, ctx, subaccountId, testInput.Spots[0].QuoteDenom)
			externalDeposit = testexchange.GetBankAndDepositFunds(app, ctx, externalAccount, testInput.Spots[0].QuoteDenom)

		})

		Context("when transfer more than src subaccount amount", func() {
			JustBeforeEach(func() {
				_, err = msgServer.ExternalTransfer(sdk.WrapSDKContext(ctx), &exchangetypes.MsgExternalTransfer{
					Sender:                  sender,
					SourceSubaccountId:      subaccountIDStr,
					DestinationSubaccountId: externalAccountStr,
					Amount:                  sdk.NewInt64Coin(testInput.Spots[0].QuoteDenom, 100000000),
				})
			})
			It("action should fail", func() {
				Expect(err).To(HaveOccurred())
			})
			It("source and destination amount should remain same", func() {
				srcAmt := testexchange.GetBankAndDepositFunds(app, ctx, subaccountId, testInput.Spots[0].QuoteDenom)
				dstAmt := testexchange.GetBankAndDepositFunds(app, ctx, externalAccount, testInput.Spots[0].QuoteDenom)

				Expect(srcAmt.AvailableBalance).To(Equal(subaccountDeposit.AvailableBalance))
				Expect(srcAmt.TotalBalance).To(Equal(subaccountDeposit.TotalBalance))
				Expect(dstAmt.AvailableBalance).To(Equal(externalDeposit.AvailableBalance))
				Expect(dstAmt.TotalBalance).To(Equal(externalDeposit.TotalBalance))
			})
		})

		Context("when transfer less than src subaccount amount", func() {
			var err error
			JustBeforeEach(func() {
				_, err = msgServer.ExternalTransfer(sdk.WrapSDKContext(ctx), &exchangetypes.MsgExternalTransfer{
					Sender:                  sender,
					SourceSubaccountId:      subaccountIDStr,
					DestinationSubaccountId: externalAccountStr,
					Amount:                  sdk.NewInt64Coin(testInput.Spots[0].QuoteDenom, 10),
				})
			})
			It("action should success and source and destination amount should be modified", func() {
				Expect(err).To(BeNil())
				srcAmt := testexchange.GetBankAndDepositFunds(app, ctx, subaccountId, testInput.Spots[0].QuoteDenom)
				dstAmt := testexchange.GetBankAndDepositFunds(app, ctx, externalAccount, testInput.Spots[0].QuoteDenom)

				Expect(srcAmt.AvailableBalance).To(Equal(subaccountDeposit.AvailableBalance.Sub(sdk.NewDec(10))))
				Expect(srcAmt.TotalBalance).To(Equal(subaccountDeposit.TotalBalance.Sub(sdk.NewDec(10))))
				Expect(dstAmt.AvailableBalance).To(Equal(externalDeposit.AvailableBalance.Add(sdk.NewDec(10))))
				Expect(dstAmt.TotalBalance).To(Equal(externalDeposit.TotalBalance.Add(sdk.NewDec(10))))
			})
		})
	})
})

var _ = Describe("WithdrawAllAuctionBalances", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec
		startingPrice               = sdk.NewDec(2000)

		coinBasket                sdk.Coins
		balances                  []exchangekeeper.SendToAuctionCoin
		prevAuctionModuleBalances sdk.Coins
		prevInsuranceFundAmount   math.Int
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)

		app.ExchangeKeeper.SetDenomDecimals(ctx, "bnb", 18)
		app.ExchangeKeeper.SetDenomDecimals(ctx, "cax", 18)
		app.ExchangeKeeper.SetDenomDecimals(ctx, "ecc", 18)
	})

	Context("Auction exchange account withdrawal check", func() {
		BeforeEach(func() {
			// init total supply
			coinBasket = sdk.NewCoins(
				sdk.NewCoin("bnb", sdk.NewInt(79)),
				sdk.NewCoin("cax", sdk.NewInt(245)),
				sdk.NewCoin("ecc", sdk.NewInt(137)),
			)

			// mint coin basket directly for auction module
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
			app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

			// mint coin basket for exchange module
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
			app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

			// set deposits for AuctionSubaccountID
			testexchange.MintAndDeposit(app, ctx, exchangetypes.AuctionSubaccountID.String(), coinBasket)
			moduleAcc := app.AccountKeeper.GetModuleAccount(ctx, auctiontypes.ModuleName)
			prevAuctionModuleBalances = app.BankKeeper.GetAllBalances(ctx, moduleAcc.GetAddress())
			balances = app.ExchangeKeeper.WithdrawAllAuctionBalances(ctx)
		})
		It("return value should be accurate", func() {
			expBalances := []exchangekeeper.SendToAuctionCoin{}
			for _, coin := range coinBasket {
				expBalances = append(expBalances, exchangekeeper.SendToAuctionCoin{
					SubaccountId: exchangetypes.AuctionSubaccountID.Hex(),
					Denom:        coin.Denom,
					Amount:       coin.Amount,
				})
			}
			Expect(balances).To(BeEquivalentTo(expBalances))
		})
		It("auction module bank balance should be increased", func() {
			moduleAcc := app.AccountKeeper.GetModuleAccount(ctx, auctiontypes.ModuleName)
			auctionModuleBalances := app.BankKeeper.GetAllBalances(ctx, moduleAcc.GetAddress())

			Expect(auctionModuleBalances).To(BeEquivalentTo(prevAuctionModuleBalances.Add(coinBasket...)))
		})
	})

	Context("Distribution module account withdrawal check", func() {
		BeforeEach(func() {
			// init total supply
			coinBasket = sdk.NewCoins(
				sdk.NewCoin("bnb", sdk.NewInt(79)),
				sdk.NewCoin("cax", sdk.NewInt(245)),
				sdk.NewCoin("ecc", sdk.NewInt(137)),
			)

			// mint coin basket directly for auction module
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
			app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

			// mint coin basket for exchange module
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, types.AuctionFeesAddress, coinBasket)

			moduleAcc := app.AccountKeeper.GetModuleAccount(ctx, auctiontypes.ModuleName)
			prevAuctionModuleBalances = app.BankKeeper.GetAllBalances(ctx, moduleAcc.GetAddress())
			balances = app.ExchangeKeeper.WithdrawAllAuctionBalances(ctx)
		})

		It("return value should be accurate", func() {
			expBalances := []exchangekeeper.SendToAuctionCoin{}
			for _, coin := range coinBasket {
				expBalances = append(expBalances, exchangekeeper.SendToAuctionCoin{
					SubaccountId: "",
					Denom:        coin.Denom,
					Amount:       coin.Amount,
				})
			}
			Expect(balances).To(BeEquivalentTo(expBalances))
		})

		It("auction module bank balance should be increased", func() {
			moduleAcc := app.AccountKeeper.GetModuleAccount(ctx, auctiontypes.ModuleName)
			auctionModuleBalances := app.BankKeeper.GetAllBalances(ctx, moduleAcc.GetAddress())

			Expect(auctionModuleBalances).To(BeEquivalentTo(prevAuctionModuleBalances.Add(coinBasket...)))
		})
	})

	Context("Insurance fund withdrawal check", func() {
		BeforeEach(func() {
			oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			initialInsuranceFundBalance = sdk.NewDec(44)
			coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

			_, _, err := app.ExchangeKeeper.PerpetualMarketLaunch(
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

			// init total supply
			coinBasket = sdk.NewCoins(
				sdk.NewCoin("bnb", sdk.NewInt(79)),
				sdk.NewCoin("cax", sdk.NewInt(245)),
				sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.NewInt(137)),
			)

			// mint coin basket directly for auction module
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
			app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

			// mint coin basket for exchange module
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinBasket)
			app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, coinBasket)

			fund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.Perps[0].MarketID)
			Expect(fund).ToNot(BeNil())
			prevInsuranceFundAmount = fund.Balance
			balances = app.ExchangeKeeper.WithdrawAllAuctionBalances(ctx)
		})

		It("check insurance fund doesnt change", func() {
			fund := app.InsuranceKeeper.GetInsuranceFund(ctx, testInput.Perps[0].MarketID)
			Expect(fund).ToNot(BeNil())
			Expect(fund.Balance.String()).To(Equal(prevInsuranceFundAmount.String()))
		})
	})
})
