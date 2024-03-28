package keeper_test

import (
	"time"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Msgs execution test", func() {
	var (
		app       *simapp.InjectiveApp
		msgServer types.MsgServer
		ctx       sdk.Context

		marketID common.Hash

		depositDenom string
		ticker       string
		oracleBase   string
		oracleQuote  string
		oracleType   oracletypes.OracleType
		expiry       int64

		sender  sdk.AccAddress
		sender2 sdk.AccAddress

		coins sdk.Coins
		err   error
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Now()})
		app.InsuranceKeeper.SetParams(ctx, TestingInsuranceParams)

		msgServer = insurancekeeper.NewMsgServerImpl(app.InsuranceKeeper)

		depositDenom = "usdt"
		ticker = "inj/usdt"
		oracleBase = "inj"
		oracleQuote = "usdt"
		oracleType = oracletypes.OracleType_PriceFeed
		expiry = -1

		marketID = exchangetypes.NewDerivativesMarketID(ticker, depositDenom, oracleBase, oracleQuote, oracleType, expiry)

		sender = InsuranceAccAddrs[0]
		sender2 = InsuranceAccAddrs[1]
	})

	Describe("Create insurance fund", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			_, err = msgServer.CreateInsuranceFund(sdk.WrapSDKContext(ctx), &types.MsgCreateInsuranceFund{
				Sender:         sender.String(),
				Ticker:         ticker,
				QuoteDenom:     depositDenom,
				OracleBase:     oracleBase,
				OracleQuote:    oracleQuote,
				OracleType:     oracleType,
				Expiry:         expiry,
				InitialDeposit: deposit,
			})
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
	})
	Describe("Underwrite insurance fund", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker, depositDenom, oracleBase, oracleQuote, oracleType, expiry)
		})

		Context("estimated redemptions before underwrite", func() {
			JustBeforeEach(func() {
				coins = app.InsuranceKeeper.GetEstimatedRedemptions(ctx, sender2, marketID)
			})
			It("should be empty", func() {
				Expect(coins.AmountOf(depositDenom)).To(Equal(sdk.ZeroInt()))
			})
		})

		Context("underwriting insurance fund", func() {
			JustBeforeEach(func() {
				deposit := sdk.NewInt64Coin(depositDenom, 5000)
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(deposit))
				Expect(err).To(BeNil())
				_, err = msgServer.Underwrite(sdk.WrapSDKContext(ctx), &types.MsgUnderwrite{
					Sender:   sender2.String(),
					MarketId: marketID.Hex(),
					Deposit:  deposit,
				})
			})
			It("work without issue", func() {
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("Request redemption", func() {
		JustBeforeEach(func() {
			deposit := sdk.NewInt64Coin(depositDenom, 10000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(deposit))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, deposit, ticker, depositDenom, oracleBase, oracleQuote, oracleType, expiry)
			Expect(err).To(BeNil())
			underwriteAmount := sdk.NewInt64Coin(depositDenom, 5000)
			err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender2, sdk.NewCoins(underwriteAmount))
			Expect(err).To(BeNil())
			err = app.InsuranceKeeper.UnderwriteInsuranceFund(ctx, sender2, marketID, underwriteAmount)
			Expect(err).To(BeNil())

			fund := app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			shareCoins := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom())
			_, err = msgServer.RequestRedemption(sdk.WrapSDKContext(ctx), &types.MsgRequestRedemption{
				Sender:   sender.String(),
				MarketId: marketID.Hex(),
				Amount:   shareCoins,
			})
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
	})
})
