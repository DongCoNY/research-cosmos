package keeper_test

import (
	"time"

	sdkmath "cosmossdk.io/math"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var (
	// InsuranceAccPrivKeys generate secp256k1 pubkeys to be used for account pub keys
	InsuranceAccPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}

	// InsuranceAccPubKeys holds the pub keys for the account keys
	InsuranceAccPubKeys = []ccrypto.PubKey{
		InsuranceAccPrivKeys[0].PubKey(),
		InsuranceAccPrivKeys[1].PubKey(),
		InsuranceAccPrivKeys[2].PubKey(),
	}

	// InsuranceAccAddrs holds the sdk.AccAddresses
	InsuranceAccAddrs = []sdk.AccAddress{
		sdk.AccAddress(InsuranceAccPubKeys[0].Address()),
		sdk.AccAddress(InsuranceAccPubKeys[1].Address()),
		sdk.AccAddress(InsuranceAccPubKeys[2].Address()),
	}

	// TestingPeggyParams is a set of exchange params for testing
	TestingInsuranceParams = types.Params{
		DefaultRedemptionNoticePeriodDuration: time.Minute,
	}
)

var _ = Describe("Insurance keeper functions test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		marketID common.Hash

		depositDenom1 string
		ticker1       string
		oracleBase1   string
		oracleQuote1  string
		oracleType1   oracletypes.OracleType
		expiry1       int64

		depositDenom2 string
		ticker2       string
		oracleBase2   string
		oracleQuote2  string
		oracleType2   oracletypes.OracleType
		expiry2       int64

		// nolint:all
		// marketID2 common.Hash

		sender  sdk.AccAddress
		sender2 sdk.AccAddress

		fund  *types.InsuranceFund
		coins sdk.Coins
		err   error
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Now()})
		app.InsuranceKeeper.SetParams(ctx, TestingInsuranceParams)

		depositDenom1 = "usdt"
		ticker1 = "inj/usdt"
		oracleBase1 = "inj"
		oracleQuote1 = "usdt"
		oracleType1 = oracletypes.OracleType_PriceFeed
		expiry1 = -1

		marketID = exchangetypes.NewDerivativesMarketID(ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)

		depositDenom2 = "usdt2"
		ticker2 = "inj2/usdt2"
		oracleBase2 = "inj2"
		oracleQuote2 = "usdt2"
		oracleType2 = oracletypes.OracleType_PriceFeed
		expiry2 = -1

		// nolint:all
		// marketID2 = exchangetypes.NewDerivativesMarketID(ticker2, depositDenom2, oracleBase2, oracleQuote2, oracleType2, expiry2)

		sender = InsuranceAccAddrs[0]
		sender2 = InsuranceAccAddrs[1]
	})

	// TODO: add edge cases to check more failure cases by function
	// TODO: check precision loss handling here and implement https://www.notion.so/injective/Implementation-Details-of-Insurance-Fund-Manager-86b4adcda989400f8308d04e7a5fa88e

	Describe("Insurance fund setter and getter", func() {
		Context("insurance fund get for non-existent", func() {
			JustBeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			})
			It("should have nil", func() {
				Expect(fund).To(BeNil())
			})
		})

		Context("insurance fund get after set", func() {
			JustBeforeEach(func() {
				app.InsuranceKeeper.SetInsuranceFund(ctx, types.NewInsuranceFund(
					marketID, depositDenom1, "share1", time.Minute, ticker1, oracleBase1, oracleQuote1, oracleType1, expiry1,
				))
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			})
			It("should have correct insurance fund data", func() {
				Expect(fund).ToNot(BeNil())
				Expect(fund.DepositDenom).To(Equal(depositDenom1))
				Expect(fund.InsurancePoolTokenDenom).To(Equal("share1"))
				Expect(fund.RedemptionNoticePeriodDuration).To(Equal(time.Minute))
				Expect(fund.MarketId).To(Equal(marketID.Hex()))
				Expect(fund.Balance).To(Equal(sdk.ZeroInt()))
				Expect(fund.TotalShare).To(Equal(sdk.ZeroInt()))
				Expect(fund.MarketTicker).To(Equal(ticker1))
				Expect(fund.OracleBase).To(Equal(oracleBase1))
				Expect(fund.OracleQuote).To(Equal(oracleQuote1))
				Expect(fund.OracleType).To(Equal(oracleType1))
				Expect(fund.Expiry).To(Equal(expiry1))
			})
		})
	})

	When("insurance fund is created", func() {
		var (
			userBalance, userDeposit sdk.Coin
		)

		BeforeEach(func() {
			userBalance = sdk.NewInt64Coin(depositDenom1, 10000)
			userDeposit = sdk.NewInt64Coin(depositDenom1, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(userBalance))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(userDeposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, userDeposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			Expect(err).To(BeNil())
		})

		Context("share tokens of insurance fund", func() {
			It("are minted properly", func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				Expect(fund).To(Not(BeNil()))
				insuranceShare := app.BankKeeper.GetBalance(ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName), fund.ShareDenom()).Amount
				userShare := app.BankKeeper.GetBalance(ctx, sender, fund.ShareDenom()).Amount
				Expect(insuranceShare).To(Equal(types.InsuranceFundProtocolOwnedLiquiditySupply))
				Expect(userShare).To(Equal(types.InsuranceFundInitialSupply.Sub(types.InsuranceFundProtocolOwnedLiquiditySupply)))
			})
		})

		Context("and the insurance fund has no balance left", func() {
			BeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				Expect(fund).To(Not(BeNil()))
				err = app.InsuranceKeeper.WithdrawFromInsuranceFund(ctx, marketID, fund.Balance)
				Expect(err).To(BeNil())
			})

			Context("and a someone underwrites to the fund", func() {
				BeforeEach(func() {
					balance := sdk.NewInt64Coin(depositDenom1, 10000)
					err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(balance))
					Expect(err).To(BeNil())
					err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(balance))
					Expect(err).To(BeNil())
					err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, balance)
					Expect(err).To(BeNil())
				})

				It("a new supply of share tokens is minted", func() {
					fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
					Expect(fund).To(Not(BeNil()))
					insuranceShare := app.BankKeeper.GetBalance(ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName), fund.ShareDenom()).Amount
					userShare := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount
					Expect(insuranceShare).To(Equal(types.InsuranceFundProtocolOwnedLiquiditySupply))
					Expect(userShare).To(Equal(types.InsuranceFundInitialSupply.Sub(types.InsuranceFundProtocolOwnedLiquiditySupply)))
				})
			})
		})

		Context("and the fund creator withdraws all his deposit", func() {
			BeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				Expect(fund).To(Not(BeNil()))
				withdrawAmount := app.BankKeeper.GetBalance(ctx, sender, fund.ShareDenom())
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender, marketID, withdrawAmount)
				Expect(err).To(BeNil())
				ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Minute + time.Second))
				err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(ctx)
				Expect(err).To(BeNil())
			})

			Context("fund total share", func() {
				It("is equal to protocol owned liquidity", func() {
					fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
					Expect(fund.TotalShare).To(Equal(types.InsuranceFundProtocolOwnedLiquiditySupply))
					Expect(app.BankKeeper.GetBalance(ctx, sender, fund.ShareDenom()).Amount).To(Equal(sdk.ZeroInt()))
				})
			})

			Context("and a someone underwrites to the fund", func() {
				BeforeEach(func() {
					balance := sdk.NewInt64Coin(depositDenom1, 10000)
					err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(balance))
					Expect(err).To(BeNil())
					err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(balance))
					Expect(err).To(BeNil())
					err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, balance)
					Expect(err).To(BeNil())
				})

				It("a new supply of share tokens is minted", func() {
					fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
					Expect(fund).To(Not(BeNil()))
					insuranceShare := app.BankKeeper.GetBalance(ctx, app.AccountKeeper.GetModuleAddress(types.ModuleName), fund.ShareDenom()).Amount
					userShare := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount
					Expect(insuranceShare).To(Equal(types.InsuranceFundProtocolOwnedLiquiditySupply))
					Expect(userShare).To(Equal(types.InsuranceFundInitialSupply.Sub(types.InsuranceFundProtocolOwnedLiquiditySupply)))
				})
			})
		})
	})

	Describe("Insurance fund create and getter", func() {
		Context("insurance fund creation put more deposit than balance", func() {
			JustBeforeEach(func() {
				balance := sdk.NewInt64Coin(depositDenom1, 1000)
				deposit := sdk.NewInt64Coin(depositDenom1, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(balance))
				Expect(err).To(BeNil())

				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			})
			It("failure in creation", func() {
				Expect(err).ToNot(BeNil())
			})
		})

		Context("insurance fund creation with zero deposit", func() {
			var originShareDenomId uint64
			var nextShareDenomId uint64

			JustBeforeEach(func() {
				originShareDenomId = app.InsuranceKeeper.ExportNextShareDenomId(ctx)
				balance := sdk.NewInt64Coin(depositDenom1, 1000)
				deposit := sdk.NewInt64Coin(depositDenom1, 0)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
				nextShareDenomId = app.InsuranceKeeper.ExportNextShareDenomId(ctx)
			})
			It("failure in creation", func() {
				Expect(err).ToNot(BeNil())
			})
			It("share denom global id should be increased", func() {
				Expect(nextShareDenomId).To(Equal(originShareDenomId + 1))
			})
		})

		Context("insurance fund create", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())

				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("insurance fund multiple creation with same marketID", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
				Expect(err).To(BeNil())
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			})
			It("should have error", func() {
				Expect(err).ToNot(BeNil())
			})
		})

		Context("insurance fund multiple creation with different marketID", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 10000)
				deposit2 := sdk.NewInt64Coin(depositDenom2, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
				Expect(err).To(BeNil())
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit2))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit2))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit2, ticker2, depositDenom2, oracleBase2, oracleQuote2, oracleType2, expiry2)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("insurance fund get after set", func() {
			var shareAmount sdkmath.Int
			var totalShare sdkmath.Int
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
				Expect(err).To(BeNil())
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount = app.BankKeeper.GetBalance(ctx, sender, fund.ShareDenom()).Amount
				totalShare = fund.TotalShare
			})
			It("should have correct share token amount", func() {
				Expect(totalShare).To(Equal(types.InsuranceFundInitialSupply))
				Expect(shareAmount).To(Equal(types.InsuranceFundInitialSupply.Sub(types.InsuranceFundProtocolOwnedLiquiditySupply)))
			})
			It("should have correct insurance fund data", func() {
				Expect(fund).ToNot(BeNil())
				Expect(fund.DepositDenom).To(Equal(depositDenom1))
				Expect(fund.InsurancePoolTokenDenom).To(Equal(fund.ShareDenom()))
				Expect(fund.RedemptionNoticePeriodDuration).To(Equal(time.Minute))
				Expect(fund.MarketId).To(Equal(marketID.Hex()))
				Expect(fund.Balance).To(Equal(sdk.NewInt(10000)))
				Expect(fund.TotalShare).To(Equal(types.InsuranceFundInitialSupply))
			})
		})
	})

	Describe("Insurance fund underwrite and getter", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom1, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			Expect(err).To(BeNil())
		})

		Context("estimated redemptions before underwrite", func() {
			JustBeforeEach(func() {
				coins = app.InsuranceKeeper.GetEstimatedRedemptions(ctx, sender2, marketID)
			})
			It("should be empty", func() {
				Expect(coins.AmountOf(depositDenom1)).To(Equal(sdk.ZeroInt()))
			})
		})

		Context("underwrite more than balance", func() {
			JustBeforeEach(func() {
				balance := sdk.NewInt64Coin(depositDenom1, 1000)
				deposit := sdk.NewInt64Coin(depositDenom1, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
			})
			It("failure in underwriting", func() {
				Expect(err).ToNot(BeNil())
			})
		})

		Context("underwrite zero balance", func() {
			JustBeforeEach(func() {
				balance := sdk.NewInt64Coin(depositDenom1, 1000)
				deposit := sdk.NewInt64Coin(depositDenom1, 0)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(balance))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
			})
			It("failure in underwriting", func() {
				Expect(err).ToNot(BeNil())
			})
		})

		Context("underwriting insurance fund", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
			})
			It("work without issue", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("underwriting insurance fund two times", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
				Expect(err).To(BeNil())
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
			})
			It("work without issue", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("share token amount after underwrite", func() {
			var shareAmount sdkmath.Int
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
				Expect(err).To(BeNil())
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount = app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount
			})
			It("should have increased value", func() {
				Expect(shareAmount).To(Equal(types.InsuranceFundInitialSupply.Quo(sdk.NewInt(2))))
			})
		})
		Context("estimated redemptions after underwrite", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom1, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, deposit)
				Expect(err).To(BeNil())
				coins = app.InsuranceKeeper.GetEstimatedRedemptions(ctx, sender2, marketID)
			})
			It("should have increased value", func() {
				Expect(coins).To(Equal(sdk.Coins{sdk.NewInt64Coin(depositDenom1, 5000)}))
			})
		})
	})

	Describe("Insurance fund request redemption & estimated redemptions & pending redemptions getter", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom1, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			Expect(err).To(BeNil())
			underwriteAmount := sdk.NewInt64Coin(depositDenom1, 5000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, underwriteAmount)
			Expect(err).To(BeNil())
		})

		Context("pending redemptions at initial", func() {
			JustBeforeEach(func() {
				coins = app.InsuranceKeeper.GetPendingRedemptions(ctx, sender2, marketID)
			})
			It("should be empty", func() {
				Expect(coins.AmountOf(depositDenom1)).To(Equal(sdk.ZeroInt()))
			})
		})

		Context("request more redemption then share token balance", func() {
			JustBeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount.Add(sdk.NewInt(2))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
			})
			It("should have an error", func() {
				Expect(err).ToNot(BeNil())
			})
		})

		Context("request more redemption with zero share token", func() {
			JustBeforeEach(func() {
				shareAmount := sdk.ZeroInt()
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
			})
			It("should have an error", func() {
				Expect(err).ToNot(BeNil())
			})
		})

		Context("request redemption", func() {
			var originRedemptionId uint64
			var nextRedemptionId uint64
			JustBeforeEach(func() {
				originRedemptionId = app.InsuranceKeeper.ExportNextRedemptionScheduleId(ctx)
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
				nextRedemptionId = app.InsuranceKeeper.ExportNextRedemptionScheduleId(ctx)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
			It("redemption schedule global id should be increased", func() {
				Expect(nextRedemptionId).To(Equal(originRedemptionId + 1))
			})
		})

		Context("request redemption two times in different time", func() {
			JustBeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount.Quo(sdk.NewInt(2))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
				Expect(err).To(BeNil())
				futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(time.Second))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(futureCtx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("pending redemptions after redemption", func() {
			JustBeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
				Expect(err).To(BeNil())
				coins = app.InsuranceKeeper.GetPendingRedemptions(ctx, sender2, marketID)
			})
			It("should be correct value", func() {
				Expect(coins).To(Equal(sdk.Coins{sdk.NewInt64Coin(depositDenom1, 5000)}))
			})
		})

		Context("pending redemptions after redemption two times in different time", func() {
			JustBeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareAmount := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount.Quo(sdk.NewInt(2))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
				Expect(err).To(BeNil())
				futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(time.Second))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(futureCtx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
				Expect(err).To(BeNil())
				coins = app.InsuranceKeeper.GetPendingRedemptions(futureCtx, sender2, marketID)
			})
			It("should be correct value", func() {
				Expect(coins).To(Equal(sdk.Coins{sdk.NewInt64Coin(depositDenom1, 5000)}))
			})
		})
	})

	Describe("Insurance fund withdraw all matured redemptions & pending redemptions getter", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom1, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			Expect(err).To(BeNil())
			underwriteAmount := sdk.NewInt64Coin(depositDenom1, 5000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, underwriteAmount)
			Expect(err).To(BeNil())
			fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			shareAmount := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount
			err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount))
			Expect(err).To(BeNil())
		})

		Context("try withdrawing once", func() {
			var withdrawnCoins sdk.Coin
			JustBeforeEach(func() {
				futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(2 * time.Minute))
				err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(futureCtx)
				withdrawnCoins = app.BankKeeper.GetBalance(futureCtx, sender2, depositDenom1)
				fund = app.InsuranceKeeper.GetInsuranceFund(futureCtx, marketID)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
			It("should withdrawn correct amount", func() {
				Expect(withdrawnCoins).To(Equal(sdk.NewInt64Coin(depositDenom1, 5000)))
			})
			It("insurance fund should have updated correctly", func() {
				Expect(fund.Balance).To(Equal(sdk.NewInt(10000)))
				Expect(fund.TotalShare).To(Equal(types.InsuranceFundInitialSupply))
			})
		})

		Context("try withdrawing twice", func() {
			var withdrawnCoins sdk.Coin
			JustBeforeEach(func() {
				futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(2 * time.Minute))
				err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(futureCtx)
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(futureCtx)
				withdrawnCoins = app.BankKeeper.GetBalance(futureCtx, sender2, depositDenom1)
				fund = app.InsuranceKeeper.GetInsuranceFund(futureCtx, marketID)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
			It("should withdraw correct amount", func() {
				Expect(withdrawnCoins).To(Equal(sdk.NewInt64Coin(depositDenom1, 5000)))
			})
			It("insurance fund should have updated correctly", func() {
				Expect(fund.Balance).To(Equal(sdk.NewInt(10000)))
				Expect(fund.TotalShare).To(Equal(types.InsuranceFundInitialSupply))
			})
		})

		Context("matured redemption withdraw for zero balance insurance fund", func() {
			var withdrawnCoins sdk.Coin
			JustBeforeEach(func() {
				// let the insurance fund to have zero balance
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				Expect(fund).To(Not(BeNil()))
				err = app.InsuranceKeeper.WithdrawFromInsuranceFund(ctx, marketID, fund.Balance)
				Expect(err).To(BeNil())

				// withdraw matured redemption schedule
				futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(2 * time.Minute))
				err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(futureCtx)
				withdrawnCoins = app.BankKeeper.GetBalance(futureCtx, sender2, depositDenom1)
				fund = app.InsuranceKeeper.GetInsuranceFund(futureCtx, marketID)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
			It("should not have withdrawn any amount", func() {
				Expect(withdrawnCoins).To(Equal(sdk.NewInt64Coin(depositDenom1, 0)))
			})
			It("insurance fund should have updated correctly", func() {
				Expect(fund.Balance).To(Equal(sdk.NewInt(0)))
				Expect(fund.TotalShare).To(Equal(types.InsuranceFundInitialSupply))
			})
		})

	})

	Describe("Insurance fund withdraw all matured redemptions & check not matured as it is when there are two redemptions request", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom1, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1) // 9900
			Expect(err).To(BeNil())
			underwriteAmount := sdk.NewInt64Coin(depositDenom1, 5000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, underwriteAmount) // 14900
			Expect(err).To(BeNil())
			fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			shareAmount := app.BankKeeper.GetBalance(ctx, sender, fund.ShareDenom()).Amount
			err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount)) // gets back 9900
			Expect(err).To(BeNil())
			shareAmount = app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount.Quo(sdk.NewInt(2))
			err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount)) // gets back 2500
			Expect(err).To(BeNil())
			futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(time.Minute))
			err = app.InsuranceKeeper.RequestInsuranceFundRedemption(futureCtx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareAmount)) // gets back 2500
			Expect(err).To(BeNil())
		})

		Context("try withdrawing after only one is matured", func() {
			var withdrawnCoins sdk.Coin
			var withdrawnCoinsInit sdk.Coin
			var withdrawnCoinsLater sdk.Coin
			JustBeforeEach(func() {
				futureCtx := ctx.WithBlockTime(ctx.BlockTime().Add(time.Minute + time.Second))
				err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(futureCtx)
				withdrawnCoins = app.BankKeeper.GetBalance(futureCtx, sender, depositDenom1)
				withdrawnCoinsInit = app.BankKeeper.GetBalance(futureCtx, sender2, depositDenom1)
				futureCtx = ctx.WithBlockTime(ctx.BlockTime().Add(2 * time.Minute))
				withdrawnCoinsLater = app.BankKeeper.GetBalance(futureCtx, sender2, depositDenom1)
				fund = app.InsuranceKeeper.GetInsuranceFund(futureCtx, marketID)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
			It("should withdrawn correct amount", func() {
				Expect(withdrawnCoins).To(Equal(sdk.NewInt64Coin(depositDenom1, 9900)))
				Expect(withdrawnCoinsInit).To(Equal(sdk.NewInt64Coin(depositDenom1, 2500)))
				Expect(withdrawnCoinsLater).To(Equal(sdk.NewInt64Coin(depositDenom1, 2500)))
			})
			It("insurance fund should have updated correctly", func() {
				Expect(fund.Balance).To(Equal(sdk.NewInt(2600)))
				Expect(fund.TotalShare).To(Equal(types.InsuranceFundInitialSupply.Quo(sdk.NewInt(4)).Add(types.InsuranceFundProtocolOwnedLiquiditySupply)))
			})
		})
	})

	Describe("Insurance fund deposit edge cases", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom1, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker1, depositDenom1, oracleBase1, oracleQuote1, oracleType1, expiry1)
			Expect(err).To(BeNil())
		})

		Context("try to deposit normal case", func() {
			var shareAmount sdkmath.Int
			var shareDenom string
			JustBeforeEach(func() {
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				underwriteAmount := sdk.NewInt64Coin(depositDenom1, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(underwriteAmount))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(underwriteAmount))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, underwriteAmount)
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
				shareDenom = fund.ShareDenom()
				shareAmount = app.BankKeeper.GetBalance(ctx, sender2, shareDenom).Amount
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			})
			It("should not have error", func() {
				Expect(err).To(BeNil())
			})
			It("should have correct correct amount", func() {
				Expect(shareAmount).To(Equal(types.InsuranceFundInitialSupply.Quo(sdk.NewInt(2))))
			})
			It("share denom should not be changed", func() {
				Expect(shareDenom).To(Equal("share1"))
			})
			It("insurance fund should have updated correctly", func() {
				Expect(fund.Balance).To(Equal(sdk.NewInt(15000)))
				Expect(fund.TotalShare).To(Equal(types.InsuranceFundInitialSupply.Mul(sdk.NewInt(3)).Quo(sdk.NewInt(2))))
			})
		})
	})

	Describe("Immediate insurance fund redemptions", func() {
		var (
			testInput           testexchange.TestInput
			perpMarket          *exchangetypes.DerivativeMarket
			binaryOptionsMarket *exchangetypes.BinaryOptionsMarket
		)

		BeforeEach(func() {
			testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 1)
		})

		Context("perpetual markets", func() {
			BeforeEach(func() {
				perpInput := testInput.Perps[0]
				// Create insurance fund
				deposit := sdk.NewInt64Coin(perpInput.QuoteDenom, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(
					ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(
					ctx, sender, deposit, perpInput.Ticker, perpInput.QuoteDenom, perpInput.OracleBase,
					perpInput.OracleQuote, perpInput.OracleType, types.PerpetualExpiryFlag)
				Expect(err).To(BeNil())
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, perpInput.MarketID)
				Expect(fund).NotTo(BeNil())

				oracleBase, oracleQuote := perpInput.OracleBase, perpInput.OracleQuote
				app.OracleKeeper.SetPriceFeedPriceState(
					ctx, oracleBase, oracleQuote,
					oracletypes.NewPriceState(sdk.NewDec(2000), ctx.BlockTime().Unix()))

				perpMarket, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
					ctx, perpInput.Ticker, perpInput.QuoteDenom, perpInput.OracleBase, perpInput.OracleQuote,
					0, perpInput.OracleType, perpInput.InitialMarginRatio, perpInput.MaintenanceMarginRatio,
					perpInput.MakerFeeRate, perpInput.TakerFeeRate, perpInput.MinPriceTickSize,
					perpInput.MinQuantityTickSize,
				)
				Expect(err).To(BeNil())

				// Underwrite
				deposit = sdk.NewInt64Coin(fund.DepositDenom, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, perpMarket.MarketID(), deposit)
				Expect(err).To(BeNil())

				// redeem half of insurance fund
				share := sdk.NewCoin(
					fund.ShareDenom(),
					app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount.QuoRaw(2))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(
					ctx, sender2, perpMarket.MarketID(), share)
				Expect(err).To(BeNil())
			})

			When("market is demolished", func() {
				var depositBalance sdk.Coin

				BeforeEach(func() {
					err = app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &exchangetypes.DerivativeMarketParamUpdateProposal{
						Title:                  "Demolish perpetual market",
						Description:            "Demolish perpetual market",
						MarketId:               perpMarket.MarketId,
						InitialMarginRatio:     &perpMarket.InitialMarginRatio,
						MaintenanceMarginRatio: &perpMarket.MaintenanceMarginRatio,
						MakerFeeRate:           &perpMarket.MakerFeeRate,
						TakerFeeRate:           &perpMarket.TakerFeeRate,
						RelayerFeeShareRate:    &perpMarket.RelayerFeeShareRate,
						MinPriceTickSize:       &perpMarket.MinPriceTickSize,
						MinQuantityTickSize:    &perpMarket.MinQuantityTickSize,
						Status:                 exchangetypes.MarketStatus_Demolished,
						OracleParams: exchangetypes.NewOracleParams(
							perpMarket.OracleBase,
							perpMarket.OracleQuote,
							perpMarket.OracleScaleFactor,
							perpMarket.OracleType),
					})
					Expect(err).To(BeNil())
					err = app.InsuranceKeeper.WithdrawAllMaturedRedemptions(ctx)
					Expect(err).To(BeNil())
				})

				It("should withdraw insurance fund redemptions for the market", func() {
					depositBalance = app.BankKeeper.GetBalance(ctx, sender2, fund.DepositDenom)
					Expect(depositBalance).To(Equal(sdk.NewInt64Coin(fund.DepositDenom, 2500)))
				})

				When("request insurance fund redemption after market being demolished", func() {
					JustBeforeEach(func() {
						// redeem the rest
						share := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom())
						err = app.InsuranceKeeper.RequestInsuranceFundRedemption(
							ctx, sender2, perpMarket.MarketID(), share)
						Expect(err).To(BeNil())
					})

					It("should withdraw immediately without waiting for the next block", func() {
						depositBalance = app.BankKeeper.GetBalance(ctx, sender2, fund.DepositDenom)
						Expect(depositBalance).To(Equal(sdk.NewInt64Coin(fund.DepositDenom, 5000)))
					})
				})
			})
		})

		Context("binary options market", func() {
			BeforeEach(func() {
				binaryInput := testInput.BinaryMarkets[0]
				// Create insurance fund
				deposit := sdk.NewInt64Coin(binaryInput.QuoteDenom, 10000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(
					ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.CreateInsuranceFund(
					ctx, sender, deposit, binaryInput.Ticker, binaryInput.QuoteDenom, binaryInput.OracleSymbol,
					binaryInput.OracleProvider, oracletypes.OracleType_Provider, types.BinaryOptionsExpiryFlag)
				Expect(err).To(BeNil())
				fund = app.InsuranceKeeper.GetInsuranceFund(ctx, binaryInput.MarketID)
				Expect(fund).NotTo(BeNil())

				err = app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
					Provider: binaryInput.OracleProvider,
					Relayers: []string{binaryInput.Admin},
				})
				Expect(err).To(BeNil())

				coin := sdk.NewCoin(binaryInput.QuoteDenom, sdk.OneInt())
				Expect(err).To(BeNil())
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
				adminAccount, _ := sdk.AccAddressFromBech32(binaryInput.Admin)
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, adminAccount, sdk.NewCoins(coin))
				Expect(err).To(BeNil())

				binaryOptionsMarket, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(
					ctx, binaryInput.Ticker, binaryInput.OracleSymbol, binaryInput.OracleProvider,
					oracletypes.OracleType_Provider, binaryInput.OracleScaleFactor, binaryInput.MakerFeeRate,
					binaryInput.TakerFeeRate, binaryInput.ExpirationTimestamp, binaryInput.SettlementTimestamp,
					binaryInput.Admin, binaryInput.QuoteDenom, binaryInput.MinPriceTickSize,
					binaryInput.MinQuantityTickSize,
				)
				Expect(err).To(BeNil())

				// Underwrite
				deposit = sdk.NewInt64Coin(fund.DepositDenom, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, binaryOptionsMarket.MarketID(), deposit)
				Expect(err).To(BeNil())

				// redeem half of insurance fund
				share := sdk.NewCoin(
					fund.ShareDenom(),
					app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom()).Amount.QuoRaw(2))
				err = app.InsuranceKeeper.RequestInsuranceFundRedemption(
					ctx, sender2, binaryOptionsMarket.MarketID(), share)
				Expect(err).To(BeNil())
			})

			When("market is demolished", func() {
				var depositBalance sdk.Coin

				BeforeEach(func() {
					zero := sdk.ZeroDec()
					proposal := &exchangetypes.BinaryOptionsMarketParamUpdateProposal{
						Title:               "Demolish binary options market",
						Description:         "Demolish binary options market",
						MarketId:            binaryOptionsMarket.MarketId,
						MakerFeeRate:        &binaryOptionsMarket.MakerFeeRate,
						TakerFeeRate:        &binaryOptionsMarket.TakerFeeRate,
						RelayerFeeShareRate: &binaryOptionsMarket.RelayerFeeShareRate,
						MinPriceTickSize:    &binaryOptionsMarket.MinPriceTickSize,
						MinQuantityTickSize: &binaryOptionsMarket.MinQuantityTickSize,
						ExpirationTimestamp: binaryOptionsMarket.ExpirationTimestamp,
						SettlementTimestamp: binaryOptionsMarket.SettlementTimestamp,
						SettlementPrice:     &zero,
						Admin:               binaryOptionsMarket.Admin,
						Status:              exchangetypes.MarketStatus_Demolished,
						OracleParams: exchangetypes.NewProviderOracleParams(
							binaryOptionsMarket.OracleSymbol,
							binaryOptionsMarket.OracleProvider,
							binaryOptionsMarket.OracleScaleFactor,
							binaryOptionsMarket.OracleType),
					}
					err = proposal.ValidateBasic()
					Expect(err).To(BeNil())
					err = app.ExchangeKeeper.ExecuteBinaryOptionsMarketParamUpdateProposal(ctx, proposal)
					Expect(err).To(BeNil())
					// call begin blocker to settle binary options market
					app.BeginBlocker(ctx, abcitypes.RequestBeginBlock{})
					// call end blocker to withdraw redemptions
					app.EndBlocker(ctx, abcitypes.RequestEndBlock{})
				})

				It("should withdraw insurance fund redemptions for the market", func() {
					depositBalance = app.BankKeeper.GetBalance(ctx, sender2, fund.DepositDenom)
					Expect(depositBalance).To(Equal(sdk.NewInt64Coin(fund.DepositDenom, 2500)))
				})

				When("request insurance fund redemption after market being demolished", func() {
					JustBeforeEach(func() {
						// redeem the rest
						share := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom())
						err = app.InsuranceKeeper.RequestInsuranceFundRedemption(
							ctx, sender2, binaryOptionsMarket.MarketID(), share)
						Expect(err).To(BeNil())
					})

					It("should withdraw immediately without waiting for the next block", func() {
						depositBalance = app.BankKeeper.GetBalance(ctx, sender2, fund.DepositDenom)
						Expect(depositBalance).To(Equal(sdk.NewInt64Coin(fund.DepositDenom, 5000)))
					})
				})
			})

			When("market is expired", func() {
				var depositBalance sdk.Coin
				BeforeEach(func() {
					// expire the market
					expiryTime := time.Unix(binaryOptionsMarket.ExpirationTimestamp+1, 0)
					ctx = ctx.WithBlockTime(expiryTime)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					// verify expired
					market := app.ExchangeKeeper.GetBinaryOptionsMarketByID(ctx, binaryOptionsMarket.MarketID())
					Expect(market.Status).Should(Equal(exchangetypes.MarketStatus_Expired))

					// call begin blocker to settle binary options market
					app.BeginBlocker(ctx, abcitypes.RequestBeginBlock{})
					// call end blocker to withdraw redemptions
					app.EndBlocker(ctx, abcitypes.RequestEndBlock{})
				})

				It("should withdraw insurance fund redemptions for the market", func() {
					depositBalance = app.BankKeeper.GetBalance(ctx, sender2, fund.DepositDenom)
					Expect(depositBalance).To(Equal(sdk.NewInt64Coin(fund.DepositDenom, 2500)))
				})

				When("request insurance fund redemption after market being expired", func() {
					JustBeforeEach(func() {
						// redeem the rest
						share := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom())
						err = app.InsuranceKeeper.RequestInsuranceFundRedemption(
							ctx, sender2, binaryOptionsMarket.MarketID(), share)
						Expect(err).To(BeNil())
					})
					It("should withdraw immediately without waiting for the next block", func() {
						depositBalance = app.BankKeeper.GetBalance(ctx, sender2, fund.DepositDenom)
						Expect(depositBalance).To(Equal(sdk.NewInt64Coin(fund.DepositDenom, 5000)))
					})
				})
			})
		})
	})
})
