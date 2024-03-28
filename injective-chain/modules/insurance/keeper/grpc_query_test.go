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
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Query GRPC Endpoints test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		marketID common.Hash

		depositDenom string
		ticker       string
		oracleBase   string
		oracleQuote  string
		oracleType   oracletypes.OracleType
		expiry       int64

		sender  sdk.AccAddress
		sender2 sdk.AccAddress

		err error
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Now()})
		app.InsuranceKeeper.SetParams(ctx, TestingInsuranceParams)

		depositDenom = "usdt"
		ticker = "inj/usdt"
		oracleBase = "inj"
		oracleQuote = "usdt"
		oracleType = oracletypes.OracleType_PriceFeed
		expiry = -1
		marketID = exchangetypes.NewDerivativesMarketID(ticker, depositDenom, oracleBase, oracleQuote, oracleType, expiry)

		sender = InsuranceAccAddrs[0]
		sender2 = InsuranceAccAddrs[1]

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
		shareCoin := app.BankKeeper.GetBalance(ctx, sender2, fund.ShareDenom())
		err = app.InsuranceKeeper.RequestInsuranceFundRedemption(ctx, sender2, marketID, sdk.NewCoin(fund.ShareDenom(), shareCoin.Amount.Quo(sdk.NewInt(2))))
		Expect(err).To(BeNil())
	})

	Describe("Insurance params", func() { // InsuranceParams
		var queryResponse *types.QueryInsuranceParamsResponse
		JustBeforeEach(func() {
			goCtx := sdk.WrapSDKContext(ctx)
			queryResponse, err = app.InsuranceKeeper.InsuranceParams(goCtx, &types.QueryInsuranceParamsRequest{})
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
		It("should have correct return value", func() {
			Expect(queryResponse).ToNot(BeNil())
			Expect(queryResponse.Params).To(Equal(types.Params{
				DefaultRedemptionNoticePeriodDuration: time.Minute,
			}))
		})
	})
	Describe("Estimated redemptions", func() {
		var queryResponse *types.QueryEstimatedRedemptionsResponse
		JustBeforeEach(func() {
			goCtx := sdk.WrapSDKContext(ctx)
			queryResponse, err = app.InsuranceKeeper.EstimatedRedemptions(goCtx, &types.QueryEstimatedRedemptionsRequest{
				MarketId: marketID.String(),
				Address:  sender2.String(),
			})
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
		It("should have correct return value", func() {
			Expect(queryResponse).ToNot(BeNil())
			Expect(queryResponse.Amount).To(Equal([]sdk.Coin{sdk.NewInt64Coin(depositDenom, 2500)}))
		})
	})
	Describe("Pending redemptions", func() {
		var queryResponse *types.QueryPendingRedemptionsResponse
		JustBeforeEach(func() {
			goCtx := sdk.WrapSDKContext(ctx)
			queryResponse, err = app.InsuranceKeeper.PendingRedemptions(goCtx, &types.QueryPendingRedemptionsRequest{
				MarketId: marketID.String(),
				Address:  sender2.String(),
			})
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
		It("should have correct return value", func() {
			Expect(queryResponse).ToNot(BeNil())
			Expect(queryResponse.Amount).To(Equal([]sdk.Coin{sdk.NewInt64Coin(depositDenom, 2500)}))
		})
	})
})
