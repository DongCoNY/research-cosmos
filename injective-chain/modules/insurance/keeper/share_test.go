package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Share Token Mint/Burn Tests", func() {
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

		sender sdk.AccAddress

		fund *types.InsuranceFund
		err  error
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
		sender = InsuranceAccAddrs[0]

		fund = types.NewInsuranceFund(
			marketID, depositDenom, "share1", time.Minute, ticker, oracleBase, oracleQuote, oracleType, expiry,
		)
	})

	Describe("Mint test", func() {
		var amount sdkmath.Int
		JustBeforeEach(func() {
			fund, err = app.InsuranceKeeper.MintShareTokens(ctx, fund, sender, sdk.NewInt(1000000))
			amount = app.BankKeeper.GetBalance(ctx, sender, fund.ShareDenom()).Amount
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
		It("should mint correct amount", func() {
			Expect(amount).To(Equal(sdk.NewInt(1000000)))
		})
	})

	Describe("Burn test", func() {
		JustBeforeEach(func() {
			_, err = app.InsuranceKeeper.MintShareTokens(ctx, fund, sender, sdk.NewInt(1000000))
			Expect(err).To(BeNil())
			err = app.BankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.Coins{sdk.NewInt64Coin(fund.ShareDenom(), 300000)})
			Expect(err).To(BeNil())
			fund, err = app.InsuranceKeeper.BurnShareTokens(ctx, fund, sdk.NewInt(300000))
		})
		It("should not have error", func() {
			Expect(err).To(BeNil())
		})
		It("should burn correct amount", func() {
			Expect(fund.TotalShare).To(Equal(sdk.NewInt(700000)))
		})
	})
})
