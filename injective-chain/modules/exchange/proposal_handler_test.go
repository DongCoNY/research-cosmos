package exchange_test

import (
	"fmt"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("Proposal handler tests", func() {
	var (
		testInput                   testexchange.TestInput
		app                         *simapp.InjectiveApp
		ctx                         sdk.Context
		initialInsuranceFundBalance sdk.Dec

		err           error
		startingPrice = sdk.NewDec(2000)
	)

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
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 1, 1)
	})

	Describe("Upgrading band oracle market to bandIBC", func() {
		It("should success", func() {
			oracleBase, oracleQuote := testInput.ExpiryMarkets[0].OracleBase, testInput.ExpiryMarkets[0].OracleQuote
			oracleType := oracletypes.OracleType_Band
			app.OracleKeeper.SetBandPriceState(ctx, oracleQuote, &oracletypes.BandPriceState{
				Symbol:      oracleQuote,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})
			app.OracleKeeper.SetBandPriceState(ctx, oracleBase, &oracletypes.BandPriceState{
				Symbol:      oracleBase,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			initialInsuranceFundBalance = sdk.NewDec(44)

			coin := sdk.NewCoin(testInput.ExpiryMarkets[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))

			err = app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				coin,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
			)
			testexchange.OrFail(err)

			_, _, err := app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
				ctx,
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				0,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
				testInput.ExpiryMarkets[0].InitialMarginRatio,
				testInput.ExpiryMarkets[0].MaintenanceMarginRatio,
				testInput.ExpiryMarkets[0].MakerFeeRate,
				testInput.ExpiryMarkets[0].TakerFeeRate,
				testInput.ExpiryMarkets[0].MinPriceTickSize,
				testInput.ExpiryMarkets[0].MinQuantityTickSize,
			)
			testexchange.OrFail(err)
			marketID := types.NewExpiryFuturesMarketID(
				testInput.ExpiryMarkets[0].Ticker,
				testInput.ExpiryMarkets[0].QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				testInput.ExpiryMarkets[0].Expiry,
			)
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)

			app.OracleKeeper.SetBandIBCPriceState(ctx, oracleQuote, &oracletypes.BandPriceState{
				Symbol:      oracleQuote,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})

			app.OracleKeeper.SetBandIBCPriceState(ctx, oracleBase, &oracletypes.BandPriceState{
				Symbol:      oracleBase,
				Rate:        sdk.NewInt(1),
				ResolveTime: uint64(ctx.BlockTime().Unix()),
				Request_ID:  1,
				PriceState:  *oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()),
			})

			err = handler(ctx, &types.DerivativeMarketParamUpdateProposal{
				Title:       "Derivative market band oracle to IBC",
				Description: "Upgrade derivative market band oracle to IBC.",
				MarketId:    marketID.String(),
				OracleParams: &types.OracleParams{
					OracleBase:        oracleBase,
					OracleQuote:       oracleQuote,
					OracleScaleFactor: 0,
					OracleType:        oracletypes.OracleType_BandIBC,
				},
			})
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

			market := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, marketID)
			Expect(market).ToNot(BeNil())
			Expect(market.OracleType).To(BeEquivalentTo(oracletypes.OracleType_BandIBC))
			insuranceFund := app.InsuranceKeeper.GetInsuranceFund(ctx, marketID)
			Expect(insuranceFund).ToNot(BeNil())
			Expect(insuranceFund.OracleType).To(BeEquivalentTo(market.OracleType))
			Expect(insuranceFund.OracleBase).To(BeEquivalentTo(market.OracleBase))
			Expect(insuranceFund.OracleQuote).To(BeEquivalentTo(market.OracleQuote))
		})
	})

	Describe("Batch community pool spend proposal", func() {
		It("should success with enough balance", func() {
			app := simapp.Setup(false)
			ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)

			coins := sdk.NewCoins(sdk.NewInt64Coin("inj", 1000))
			app.MintKeeper.MintCoins(ctx, coins)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, testexchange.BuyerAccAddrs[0], coins)
			app.DistrKeeper.FundCommunityPool(ctx, coins, testexchange.BuyerAccAddrs[0])

			err = handler(ctx, &types.BatchCommunityPoolSpendProposal{
				Title:       "Batch community pool spend proposal",
				Description: "Batch community pool spend proposal.",
				Proposals: []*distrtypes.CommunityPoolSpendProposal{
					{
						Title:       "community pool spend proposal",
						Description: "community pool spend proposal.",
						Recipient:   testexchange.BuyerAccAddrs[0].String(),
						Amount:      sdk.NewCoins(sdk.NewInt64Coin("inj", 1)),
					},
					{
						Title:       "community pool spend proposal",
						Description: "community pool spend proposal.",
						Recipient:   testexchange.BuyerAccAddrs[1].String(),
						Amount:      sdk.NewCoins(sdk.NewInt64Coin("inj", 2)),
					},
				},
			})
			Expect(err).To(BeNil())
			balance1 := app.BankKeeper.GetBalance(ctx, testexchange.BuyerAccAddrs[0], "inj")
			Expect(balance1.String()).To(BeEquivalentTo("1inj"))
			balance2 := app.BankKeeper.GetBalance(ctx, testexchange.BuyerAccAddrs[1], "inj")
			Expect(balance2.String()).To(BeEquivalentTo("2inj"))
		})

		It("should fail with not enough balance", func() {
			app := simapp.Setup(false)
			ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
			handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)

			coins := sdk.NewCoins(sdk.NewInt64Coin("inj", 1000))
			app.MintKeeper.MintCoins(ctx, coins)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, testexchange.BuyerAccAddrs[0], coins)
			app.DistrKeeper.FundCommunityPool(ctx, coins, testexchange.BuyerAccAddrs[0])

			err = handler(ctx, &types.BatchCommunityPoolSpendProposal{
				Title:       "Batch community pool spend proposal",
				Description: "Batch community pool spend proposal.",
				Proposals: []*distrtypes.CommunityPoolSpendProposal{
					{
						Title:       "community pool spend proposal",
						Description: "community pool spend proposal.",
						Recipient:   testexchange.BuyerAccAddrs[0].String(),
						Amount:      sdk.NewCoins(sdk.NewInt64Coin("inj", 1000000000000000000)),
					},
					{
						Title:       "community pool spend proposal",
						Description: "community pool spend proposal.",
						Recipient:   testexchange.BuyerAccAddrs[1].String(),
						Amount:      sdk.NewCoins(sdk.NewInt64Coin("inj", 1000000000000000000)),
					},
				},
			})
			Expect(err).To(Not(BeNil()))
		})
	})

	Describe("Binary Options market param update proposal tests", func() {
		var (
			market             *types.BinaryOptionsMarket
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller = testexchange.SampleSubaccountAddr2.String()
			handler            govtypes.Handler
			proposal           *types.BinaryOptionsMarketParamUpdateProposal
		)
		BeforeEach(func() {
			oracleSymbol, oracleProvider, admin := testInput.BinaryMarkets[0].OracleSymbol, testInput.BinaryMarkets[0].OracleProvider, testInput.BinaryMarkets[0].Admin
			startingPrice := sdk.NewDec(2000)

			app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
				Provider: oracleProvider,
				Relayers: []string{admin},
			})
			app.OracleKeeper.SetProviderPriceState(ctx, oracleProvider, oracletypes.NewProviderPriceState(oracleSymbol, startingPrice, ctx.BlockTime().Unix()))

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			coin := sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(coin))
			adminAccount, _ := sdk.AccAddressFromBech32(testInput.BinaryMarkets[0].Admin)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, adminAccount, sdk.NewCoins(coin))

			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, sdk.NewCoins(coin))

			_, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(
				ctx,
				testInput.BinaryMarkets[0].Ticker,
				testInput.BinaryMarkets[0].OracleSymbol,
				testInput.BinaryMarkets[0].OracleProvider,
				oracletypes.OracleType_Provider,
				testInput.BinaryMarkets[0].OracleScaleFactor,
				testInput.BinaryMarkets[0].MakerFeeRate,
				testInput.BinaryMarkets[0].TakerFeeRate,
				testInput.BinaryMarkets[0].ExpirationTimestamp,
				testInput.BinaryMarkets[0].SettlementTimestamp,
				testInput.BinaryMarkets[0].Admin,
				testInput.BinaryMarkets[0].QuoteDenom,
				testInput.BinaryMarkets[0].MinPriceTickSize,
				testInput.BinaryMarkets[0].MinQuantityTickSize,
			)
			testexchange.OrFail(err)
			market = app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, testInput.BinaryMarkets[0].MarketID, true)

			deposit := &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(market.QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(market.QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			app.ExchangeKeeper.SetFeeDiscountSchedule(ctx, &types.FeeDiscountSchedule{
				TierInfos: []*types.FeeDiscountTierInfo{
					{TakerDiscountRate: sdk.MustNewDecFromStr("0.2")},
					{TakerDiscountRate: sdk.MustNewDecFromStr("0.8")},
				},
			})

			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)

			proposal = types.NewBinaryOptionsMarketParamUpdateProposal(
				"binary options proposal title",
				"binary options proposal Description",
				market.MarketId,
				&market.MakerFeeRate,
				&market.TakerFeeRate,
				&market.RelayerFeeShareRate,
				&market.MinPriceTickSize,
				&market.MinQuantityTickSize,
				market.ExpirationTimestamp,
				market.SettlementTimestamp,
				market.Admin,
				types.MarketStatus_Unspecified,
				nil,
			)
		})

		JustBeforeEach(func() {
			err = handler(ctx, proposal)
		})

		It("should return without errors", func() {
			Expect(proposal.ValidateBasic()).To(BeNil())
			Expect(err).To(BeNil())
		})

		When("market does no exist", func() {
			BeforeEach(func() {
				proposal.MarketId = types.NewBinaryOptionsMarketID("awd", market.QuoteDenom, market.OracleSymbol, market.OracleProvider, market.OracleType).String()
			})
			It("should return error", func() {
				Expect(err).To(Equal(types.ErrBinaryOptionsMarketNotFound))
			})
		})

		When("market was demolished already", func() {
			BeforeEach(func() {
				settlementTime := time.Unix(market.SettlementTimestamp+1, 0)
				ctx = ctx.WithBlockTime(settlementTime)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})
			It("should return error", func() {
				Expect(proposal.ValidateBasic()).To(BeNil())
				Expect(err).To(Equal(types.ErrInvalidMarketStatus))
			})
		})

		When("ExpirationTimestamp is wrong", func() {
			It("Expiration timestamp should be > 0", func() {
				proposal.ExpirationTimestamp = -1
				Expect(proposal.ValidateBasic()).To(Equal(types.ErrInvalidExpiry))
			})
			It("Expiration timestamp should be < SettlementTimestamp", func() {
				proposal.ExpirationTimestamp = proposal.SettlementTimestamp + 1
				Expect(proposal.ValidateBasic()).To(Equal(types.ErrInvalidExpiry))
			})
			When("Expiration timestamp < market.SettlementTimestamp", func() {
				BeforeEach(func() {
					proposal.ExpirationTimestamp = proposal.SettlementTimestamp + 1
					proposal.SettlementTimestamp = 0
				})
				It("should return error", func() {
					expError := sdkerrors.Wrap(types.ErrInvalidExpiry, "expiration timestamp should be prior to settlement timestamp").Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
			When("Expiration timestamp is in the past", func() {
				BeforeEach(func() {
					proposal.ExpirationTimestamp = ctx.BlockTime().Unix() - 1
				})
				It("should return error", func() {
					expError := sdkerrors.Wrapf(types.ErrInvalidExpiry, "expiration timestamp %d is in the past", proposal.ExpirationTimestamp).Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
			When("Market is expired already", func() {
				BeforeEach(func() {
					expiryTime := time.Unix(market.ExpirationTimestamp+1, 0)
					ctx = ctx.WithBlockTime(expiryTime)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					proposal.ExpirationTimestamp += 2
				})
				It("should return error", func() {
					expError := sdkerrors.Wrap(types.ErrInvalidExpiry, "cannot change expiration time of an expired market").Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
		})

		When("SettlementTimestamp is wrong", func() {
			BeforeEach(func() {
				proposal.ExpirationTimestamp = 0
			})
			It("Settlement timestamp should be > 0", func() {
				proposal.SettlementTimestamp = -1
				Expect(proposal.ValidateBasic()).To(Equal(types.ErrInvalidSettlement))
			})
			When("Settlement timestamp is in the past", func() {
				BeforeEach(func() {
					proposal.SettlementTimestamp = ctx.BlockTime().Unix() - 1
				})
				It("should return error", func() {
					expError := sdkerrors.Wrapf(types.ErrInvalidSettlement, "expiration timestamp %d is in the past", proposal.SettlementTimestamp).Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
		})

		When("SettlementPrice is set", func() {
			It("Settlement price should not be negative (except -1)", func() {
				sp := sdk.NewDec(-100)
				proposal.SettlementPrice = &sp
				expError := sdkerrors.Wrap(types.ErrInvalidPrice, proposal.SettlementPrice.String()).Error()
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

				*proposal.SettlementPrice = sdk.NewDecWithPrec(-2, 1)
				expError = sdkerrors.Wrap(types.ErrInvalidPrice, proposal.SettlementPrice.String()).Error()
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))
			})
			It("Market Status should be set to Demolished when settlement price is set", func() {
				sp := sdk.NewDec(-1)
				proposal.SettlementPrice = &sp
				expError := sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", proposal.Status.String()).Error()
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

				*proposal.SettlementPrice = sdk.ZeroDec()
				expError = sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", proposal.Status.String()).Error()
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

				*proposal.SettlementPrice = sdk.NewDecWithPrec(2, 1)
				expError = sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", proposal.Status.String()).Error()
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

				*proposal.SettlementPrice = sdk.OneDec()
				expError = sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", proposal.Status.String()).Error()
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

				proposal.Status = types.MarketStatus_Demolished
				Expect(proposal.ValidateBasic()).To(BeNil())
			})
		})

		It("Market status should be either Unspecified or Demolished", func() {
			proposal.Status = types.MarketStatus_Demolished
			Expect(proposal.ValidateBasic()).To(BeNil())

			proposal.Status = types.MarketStatus_Paused
			expError := sdkerrors.Wrap(types.ErrInvalidMarketStatus, proposal.Status.String()).Error()
			Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

			proposal.Status = types.MarketStatus_Active
			expError = sdkerrors.Wrap(types.ErrInvalidMarketStatus, proposal.Status.String()).Error()
			Expect(proposal.ValidateBasic().Error()).To(Equal(expError))

			proposal.Status = types.MarketStatus_Expired
			expError = sdkerrors.Wrap(types.ErrInvalidMarketStatus, proposal.Status.String()).Error()
			Expect(proposal.ValidateBasic().Error()).To(Equal(expError))
		})

		When("Admin address is set", func() {
			It("should be valid", func() {
				proposal.Admin = "awd"
				expError := "decoding bech32 failed: invalid bech32 string length 3"
				Expect(proposal.ValidateBasic().Error()).To(Equal(expError))
			})

			When("admin does not exist (not funded)", func() {
				BeforeEach(func() {
					proposal.Admin = "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"
				})
				It("should return error", func() {
					expError := sdkerrors.Wrapf(types.ErrAccountDoesntExist, "admin %s", proposal.Admin).Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
		})

		When("Oracle info is set", func() {
			BeforeEach(func() {
				proposal.OracleParams = types.NewProviderOracleParams(
					market.OracleSymbol,
					market.OracleProvider,
					market.OracleScaleFactor,
					market.OracleType,
				)
			})
			It("should be valid", func() {
				Expect(proposal.ValidateBasic()).To(BeNil())
			})

			When("oracle provider is non-existent", func() {
				BeforeEach(func() {
					proposal.OracleParams.Provider = "awd"
				})
				It("should return error", func() {
					expError := sdkerrors.Wrapf(types.ErrInvalidOracle, "oracle provider %s does not exist", proposal.OracleParams.Provider).Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
		})

		When("Fee rates are set", func() {
			BeforeEach(func() {
				takerFeeRate := sdk.MustNewDecFromStr("0.5")        // max taker discount is 80%, means takerFeeRate can go to 0.1
				relayerFeeShareRate := sdk.MustNewDecFromStr("0.6") // takerFeeRate of 0.1 * 0.6 = 0.06, means 0.04 left for maker fees
				makerFeeRate := sdk.MustNewDecFromStr("-0.035")
				proposal.TakerFeeRate = &takerFeeRate
				proposal.RelayerFeeShareRate = &relayerFeeShareRate
				proposal.MakerFeeRate = &makerFeeRate
			})

			It("should be valid", func() {
				Expect(proposal.ValidateBasic()).To(BeNil())
				Expect(err).To(BeNil())
			})

			It("taker fee rate should be less than 100%", func() {
				*proposal.TakerFeeRate = sdk.MustNewDecFromStr("1.01")
				Expect(proposal.ValidateBasic()).To(Equal(fmt.Errorf("exchange fee cannot be greater than 1: %s", proposal.TakerFeeRate)))
			})

			When("maker fee rate should be less than discounted taker fee", func() {
				BeforeEach(func() {
					*proposal.MakerFeeRate = sdk.MustNewDecFromStr("-0.05")
				})
				It("should return error", func() {
					Expect(err.Error()).To(ContainSubstring("makerFeeRate"))
				})
			})

			When("maker fee rate should be less than discounted taker fee even when not all fees are present in a proposal", func() {
				BeforeEach(func() {
					*proposal.MakerFeeRate = sdk.MustNewDecFromStr("-0.08")
					proposal.RelayerFeeShareRate = nil // keep
				})
				It("should return error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("makerFeeRate"))
				})
			})
		})
	})

	Describe("Forced settlement of derivative market proposal tests", func() {
		var (
			market          *types.DerivativeMarket
			handler         govtypes.Handler
			proposal        *types.MarketForcedSettlementProposal
			settlementPrice *sdk.Dec
		)
		BeforeEach(func() {
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

			market, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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

			settlementPriceValue := sdk.NewDec(3000)
			settlementPrice = &settlementPriceValue

			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			proposal = types.NewMarketForcedSettlementProposal(
				"market forced settlement title",
				"market forced settlement description",
				market.MarketId,
				settlementPrice,
			)
		})

		When("All is fine", func() {
			JustBeforeEach(func() {
				err = handler(ctx, proposal)
			})

			It("should return without errors", func() {
				Expect(proposal.ValidateBasic()).To(BeNil())
				Expect(err).To(BeNil())
			})
		})

		When("There are errors", func() {
			It("Should return error when settlement price is negative", func() {
				settlementPriceValue := sdk.NewDec(-3000)
				settlementPrice = &settlementPriceValue
				proposal.SettlementPrice = settlementPrice

				Expect(proposal.ValidateBasic()).To(Not(BeNil()))

				err = handler(ctx, proposal)
				Expect(err).To(Not(BeNil()))
			})

			It("Should return error when posting same proposal twice", func() {
				err = handler(ctx, proposal)
				Expect(err).To(BeNil())

				err = handler(ctx, proposal)
				Expect(err).To(Not(BeNil()))
			})

			It("Market not found will cause error", func() {
				proposal.MarketId = "0x1e11532fc29f1bc3eb75f6fddf4997e904c780ddf155ecb58bc89bf723e1ba56"
				err = handler(ctx, proposal)
				Expect(err).To(Not(BeNil()))
			})
		})
	})

	Describe("Forced settlement of spot market proposal tests", func() {
		var (
			market   *types.SpotMarket
			handler  govtypes.Handler
			proposal *types.MarketForcedSettlementProposal
		)
		BeforeEach(func() {
			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			spotCoin := sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(spotCoin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(spotCoin))

			market, err = app.ExchangeKeeper.SpotMarketLaunch(
				ctx,
				testInput.Spots[0].Ticker,
				testInput.Spots[0].BaseDenom,
				testInput.Spots[0].QuoteDenom,
				testInput.Spots[0].MinPriceTickSize,
				testInput.Spots[0].MinQuantityTickSize,
			)
			testexchange.OrFail(err)

			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			proposal = types.NewMarketForcedSettlementProposal(
				"market forced settlement title",
				"market forced settlement description",
				market.MarketId,
				nil,
			)
		})

		When("All is fine", func() {
			JustBeforeEach(func() {
				err = handler(ctx, proposal)
			})

			It("should return without errors", func() {
				Expect(proposal.ValidateBasic()).To(BeNil())
				Expect(err).To(BeNil())
			})

			It("should force close the market in the next block", func() {
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

				spotMarketForceClose := app.ExchangeKeeper.GetSpotMarketForceCloseInfo(ctx, common.HexToHash(market.MarketId))
				Expect(spotMarketForceClose).To(BeNil())

				spotMarket := app.ExchangeKeeper.GetSpotMarketByID(ctx, common.HexToHash(market.MarketId))
				Expect(spotMarket.Status).To(Equal(types.MarketStatus_Paused))
			})
		})

		When("There are errors", func() {
			It("Should return error when settlement price is non nil", func() {
				settlementPriceValue := sdk.NewDec(3000)
				settlementPrice := &settlementPriceValue
				proposal.SettlementPrice = settlementPrice

				Expect(proposal.ValidateBasic()).To(BeNil())

				err = handler(ctx, proposal)
				Expect(err).To(Not(BeNil()))
			})

			It("Should return error when posting same proposal twice", func() {
				err = handler(ctx, proposal)
				Expect(err).To(BeNil())

				err = handler(ctx, proposal)
				Expect(err).To(Not(BeNil()))
			})

			It("Market not found will cause error", func() {
				proposal.MarketId = "0x1e11532fc29f1bc3eb75f6fddf4997e904c780ddf155ecb58bc89bf723e1ba56"
				err = handler(ctx, proposal)
				Expect(err).To(Not(BeNil()))
			})
		})
	})

	Describe("Update derivative market params proposal tests", func() {
		var (
			market   *types.DerivativeMarket
			handler  govtypes.Handler
			proposal *types.DerivativeMarketParamUpdateProposal
		)
		BeforeEach(func() {
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

			market, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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

			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			proposal = types.NewDerivativeMarketParamUpdateProposal(
				"market params update proposal title",
				"market params update proposal description",
				market.MarketId,
				&testInput.Perps[0].InitialMarginRatio,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				types.MarketStatus_Unspecified,
				nil,
			)
		})

		When("Nothing is changed", func() {
			JustBeforeEach(func() {
				err = handler(ctx, proposal)
			})

			It("Updated market should equal pre-proposal", func() {
				Expect(proposal.ValidateBasic()).To(BeNil())
				Expect(err).To(BeNil())

				updatedMarket := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, market.MarketID())
				Expect(updatedMarket).To(Equal(market))
			})
		})

		When("Modifying fees", func() {

			It("Should return error when maker fee > taker fee", func() {
				makerFee := sdk.NewDecWithPrec(4, 3)
				proposal.MakerFeeRate = &makerFee // 0.4% maker fees
				takerFee := sdk.NewDecWithPrec(2, 3)
				proposal.TakerFeeRate = &takerFee
				err = handler(ctx, proposal)

				Expect(err.Error()).To(Equal(types.ErrFeeRatesRelation.Error()))
			})

			It("Should return error when negative maker fee > exchange part of taker fee ", func() {
				relayerFee := sdk.NewDecWithPrec(4, 1) // 0.4
				proposal.RelayerFeeShareRate = &relayerFee
				makerFee := sdk.NewDecWithPrec(-2, 3) // -0.002
				proposal.MakerFeeRate = &makerFee
				takerFee := sdk.NewDecWithPrec(3, 3) // 0.003
				proposal.TakerFeeRate = &takerFee
				err = handler(ctx, proposal)

				Expect(err.Error()).To(ContainSubstring("if makerFeeRate"))
			})
		})

		When("Setting discounts", func() {

			JustBeforeEach(func() {
				relayerFee := sdk.MustNewDecFromStr("0.4")
				proposal.RelayerFeeShareRate = &relayerFee
				makerFee := sdk.MustNewDecFromStr("-0.002")
				proposal.MakerFeeRate = &makerFee
				takerFee := sdk.MustNewDecFromStr("0.005")
				proposal.TakerFeeRate = &takerFee
				handler(ctx, proposal)
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})

			It("After discount takerFee wouldn't cover negative makerFee", func() {
				discountProposal := &types.FeeDiscountProposal{
					Title:       "Fee Discount",
					Description: "Fee Discount",
					Schedule: &types.FeeDiscountSchedule{
						BucketCount:    10,
						BucketDuration: 1000,
						QuoteDenoms:    []string{testInput.Perps[0].QuoteDenom},
						TierInfos: []*types.FeeDiscountTierInfo{
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.02"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.02"),
								StakedAmount:      sdk.NewInt(100),
								Volume:            sdk.MustNewDecFromStr("0.3"),
							},
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.05"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.05"),
								StakedAmount:      sdk.NewInt(1000),
								Volume:            sdk.MustNewDecFromStr("3"),
							},
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.35"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.35"),
								StakedAmount:      sdk.NewInt(3000),
								Volume:            sdk.MustNewDecFromStr("10"),
							},
						},
						DisqualifiedMarketIds: make([]string, 0),
					},
				}

				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				ctxCached, _ := ctx.CacheContext()
				errFees := handler(ctxCached, discountProposal)

				Expect(errFees).To(HaveOccurred())
				Expect(errFees.Error()).To(ContainSubstring("if makerFeeRate"))
			})

			It("Could not error with when set disqualified market", func() {
				relayerFeeShareRate := sdk.MustNewDecFromStr("0.6")
				makerFeeRate := sdk.MustNewDecFromStr("-0.02")

				spotMarket, _ := app.ExchangeKeeper.SpotMarketLaunchWithCustomFees(
					ctx,
					testInput.Spots[0].Ticker,
					testInput.Spots[0].BaseDenom,
					testInput.Spots[0].QuoteDenom,
					testInput.Spots[0].MinPriceTickSize,
					testInput.Spots[0].MinQuantityTickSize,
					makerFeeRate,
					testInput.Spots[0].TakerFeeRate,
					relayerFeeShareRate,
				)

				discountProposal := &types.FeeDiscountProposal{
					Title:       "Fee Discount",
					Description: "Fee Discount",
					Schedule: &types.FeeDiscountSchedule{
						BucketCount:    10,
						BucketDuration: 1000,
						QuoteDenoms:    []string{testInput.Perps[0].QuoteDenom},
						TierInfos: []*types.FeeDiscountTierInfo{
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.000000000000000001"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.05"),
								StakedAmount:      sdk.NewInt(1000),
								Volume:            sdk.MustNewDecFromStr("1000000000"),
							},
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.000000000000000001"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.075"),
								StakedAmount:      sdk.NewInt(2000),
								Volume:            sdk.MustNewDecFromStr("1000000000"),
							},
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.000000000000000001"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.125"),
								StakedAmount:      sdk.NewInt(3000),
								Volume:            sdk.MustNewDecFromStr("1000000000"),
							},
							{
								MakerDiscountRate: sdk.MustNewDecFromStr("0.000000000000000001"),
								TakerDiscountRate: sdk.MustNewDecFromStr("0.4"),
								StakedAmount:      sdk.NewInt(4000),
								Volume:            sdk.MustNewDecFromStr("1000000000"),
							},
						},
						DisqualifiedMarketIds: []string{spotMarket.MarketId, market.MarketId},
					},
				}

				handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				ctxCached, _ := ctx.CacheContext()
				errFees := handler(ctxCached, discountProposal)
				Expect(errFees).To(BeNil())
			})
		})
	})

})

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}
