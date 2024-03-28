package keeper_test

import (
	"errors"
	"fmt"
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Derivatives MsgServer Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		derivativeMarket *types.DerivativeMarket
		msgServer        types.MsgServer
		err              error
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 3, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		startingPrice := sdk.NewDec(2000)

		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		_, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
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
		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)

		testexchange.OrFail(err)
	})

	Describe("InstantPerpetualMarketLaunch", func() {
		var (
			err          error
			message      *types.MsgInstantPerpetualMarketLaunch
			amountMinted *sdk.Coin
			amountNeeded math.Int
			marketID     common.Hash
		)
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))

		BeforeEach(func() {
			params := app.ExchangeKeeper.GetParams(ctx)
			params.IsInstantDerivativeMarketLaunchEnabled = true
			app.ExchangeKeeper.SetParams(ctx, params)

			oracleBase, oracleQuote, oracleType := testInput.Perps[1].OracleBase, testInput.Perps[1].OracleQuote, testInput.Perps[1].OracleType
			startingPrice := sdk.NewDec(2000)
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
			message = &types.MsgInstantPerpetualMarketLaunch{
				Sender:                 sender.String(),
				Ticker:                 testInput.Perps[1].Ticker,
				QuoteDenom:             testInput.Perps[1].QuoteDenom,
				OracleBase:             oracleBase,
				OracleQuote:            oracleQuote,
				OracleScaleFactor:      0,
				OracleType:             oracleType,
				MakerFeeRate:           testInput.Perps[1].MakerFeeRate,
				TakerFeeRate:           testInput.Perps[1].TakerFeeRate,
				InitialMarginRatio:     testInput.Perps[1].InitialMarginRatio,
				MaintenanceMarginRatio: testInput.Perps[1].MaintenanceMarginRatio,
				MinPriceTickSize:       testInput.Perps[1].MinPriceTickSize,
				MinQuantityTickSize:    testInput.Perps[1].MinQuantityTickSize,
			}

			coin := sdk.NewCoin(testInput.Perps[1].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[1].Ticker, testInput.Perps[1].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

			amountNeeded = math.Int(sdk.NewDec(1000))

			marketID = types.NewPerpetualMarketID(testInput.Perps[1].Ticker, testInput.Perps[1].QuoteDenom, oracleBase, oracleQuote, oracleType)
		})

		JustBeforeEach(func() {
			_, err = msgServer.InstantPerpetualMarketLaunch(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount.Sub(amountNeeded)))
			})

			It("Should have funded community pool", func() {
				poolBalanceAfter := app.DistrKeeper.GetFeePool(ctx)

				Expect(len(poolBalanceAfter.CommunityPool)).NotTo(BeZero())

				for _, coin := range poolBalanceAfter.CommunityPool {
					Expect(coin.Denom).To(Equal("inj"))
					Expect(coin.Amount.String()).To(Equal(amountNeeded.ToDec().String()))
				}
			})

			It("Should have correct market fields", func() {
				marketInfo := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, marketID)
				expectedRelayerFeeShareRate := sdk.NewDecWithPrec(40, 2)

				Expect(marketInfo.Ticker).To(Equal(testInput.Perps[1].Ticker))
				Expect(marketInfo.QuoteDenom).To(Equal(testInput.Perps[1].QuoteDenom))
				Expect(marketInfo.MakerFeeRate).To(Equal(testInput.Perps[1].MakerFeeRate))
				Expect(marketInfo.TakerFeeRate).To(Equal(testInput.Perps[1].TakerFeeRate))
				Expect(marketInfo.InitialMarginRatio).To(Equal(testInput.Perps[1].InitialMarginRatio))
				Expect(marketInfo.MaintenanceMarginRatio).To(Equal(testInput.Perps[1].MaintenanceMarginRatio))
				Expect(marketInfo.MinPriceTickSize).To(Equal(testInput.Perps[1].MinPriceTickSize))
				Expect(marketInfo.MinQuantityTickSize).To(Equal(testInput.Perps[1].MinQuantityTickSize))
				Expect(marketInfo.OracleBase).To(Equal(testInput.Perps[1].OracleBase))
				Expect(marketInfo.OracleQuote).To(Equal(testInput.Perps[1].OracleQuote))
				Expect(marketInfo.OracleType).To(Equal(testInput.Perps[1].OracleType))
				Expect(marketInfo.Status).To(Equal(types.MarketStatus_Active))
				Expect(marketInfo.RelayerFeeShareRate).To(Equal(expectedRelayerFeeShareRate))
			})
		})

		Context("With sender not having enough balance", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(500)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with insufficient funds error", func() {
				errorMessage := amountMinted.String() + " is smaller than " + amountNeeded.String() + "inj: "
				expectedError := "spendable balance " + errorMessage + sdkerrors.ErrInsufficientFunds.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When quoteDenom is invalid", func() {
			BeforeEach(func() {
				message.QuoteDenom = "SMTH"

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with invalid quote denom error", func() {
				errorMessage := "denom " + message.QuoteDenom + " does not exist in supply: "
				expectedError := errorMessage + types.ErrInvalidQuoteDenom.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When perpetual market already exists", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				msgServer.InstantPerpetualMarketLaunch(sdk.WrapSDKContext(ctx), message)
			})

			It("Should be invalid with perpetual market exists error", func() {
				errorMessagePart1 := "ticker " + message.Ticker + " quoteDenom " + message.QuoteDenom + ": "
				expectedError := errorMessagePart1 + types.ErrPerpetualMarketExists.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When oracle for perpetual market is invalid", func() {
			BeforeEach(func() {
				message.OracleType = oracletypes.OracleType_Uma

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with invalid oracle error", func() {
				errorMessagePart1 := "type Uma base " + message.OracleBase + " quote " + message.OracleQuote + ": "
				expectedError := errorMessagePart1 + types.ErrInvalidOracle.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Prevent conditions", func() {
			It("Prevent perpetual market launch when an equivalent market launch proposal already exists", func() {
				message.Ticker = testInput.Perps[2].Ticker
				message.QuoteDenom = testInput.Perps[2].QuoteDenom

				// create insurance fund for perpetual market
				depositCoin := sdk.NewInt64Coin(message.QuoteDenom, 100)
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{depositCoin})
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.Coins{depositCoin})
				err := app.InsuranceKeeper.CreateInsuranceFund(
					ctx,
					sender,
					depositCoin,
					message.Ticker,
					message.QuoteDenom,
					message.OracleBase,
					message.OracleQuote,
					message.OracleType,
					-1,
				)
				Expect(err).To(BeNil())

				// submit perpetual market launch proposal
				content := types.NewPerpetualMarketLaunchProposal(
					"perpetual market launch proposal",
					"perpetual market launch proposal for specific market",
					message.Ticker,
					message.QuoteDenom,
					message.OracleBase,
					message.OracleQuote,
					message.OracleScaleFactor,
					message.OracleType,
					message.InitialMarginRatio,
					message.MaintenanceMarginRatio,
					message.MakerFeeRate,
					message.TakerFeeRate,
					message.MinPriceTickSize,
					message.MinQuantityTickSize,
				)
				proposalMsg, err := govtypesv1.NewLegacyContent(content, app.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String())
				Expect(err).To(BeNil())
				proposal, err := app.GovKeeper.SubmitProposal(ctx, []sdk.Msg{proposalMsg}, "", content.Title, content.Description, sender)
				Expect(err).To(BeNil())

				// proposal to active queue
				app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

				// prepare coins for instant market launch
				amount := sdk.Coins{sdk.NewCoin("inj", math.Int(sdk.NewDec(10000)))}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				// try execution of instant perpetual market
				_, err = msgServer.InstantPerpetualMarketLaunch(sdk.WrapSDKContext(ctx), message)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(types.ErrMarketLaunchProposalAlreadyExists.Error()))
			})
		})
	})

	Describe("InstantExpiryFuturesMarketLaunch", func() {
		var (
			err          error
			message      *types.MsgInstantExpiryFuturesMarketLaunch
			amountMinted *sdk.Coin
			amountNeeded math.Int
			marketID     common.Hash
		)
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))

		BeforeEach(func() {
			params := app.ExchangeKeeper.GetParams(ctx)
			params.IsInstantDerivativeMarketLaunchEnabled = true
			app.ExchangeKeeper.SetParams(ctx, params)

			expiry := ctx.BlockTime().Add(time.Hour * 24 * 7).Unix()
			oracleBase, oracleQuote, oracleType := testInput.ExpiryMarkets[1].OracleBase, testInput.ExpiryMarkets[1].OracleQuote, testInput.ExpiryMarkets[1].OracleType
			startingPrice := sdk.NewDec(2000)
			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))
			message = &types.MsgInstantExpiryFuturesMarketLaunch{
				Sender:                 sender.String(),
				Ticker:                 testInput.ExpiryMarkets[1].Ticker,
				QuoteDenom:             testInput.ExpiryMarkets[1].QuoteDenom,
				OracleBase:             oracleBase,
				OracleQuote:            oracleQuote,
				OracleScaleFactor:      0,
				OracleType:             oracleType,
				MakerFeeRate:           testInput.ExpiryMarkets[1].MakerFeeRate,
				TakerFeeRate:           testInput.ExpiryMarkets[1].TakerFeeRate,
				InitialMarginRatio:     testInput.ExpiryMarkets[1].InitialMarginRatio,
				MaintenanceMarginRatio: testInput.ExpiryMarkets[1].MaintenanceMarginRatio,
				MinPriceTickSize:       testInput.ExpiryMarkets[1].MinPriceTickSize,
				MinQuantityTickSize:    testInput.ExpiryMarkets[1].MinQuantityTickSize,
				Expiry:                 expiry,
			}

			coin := sdk.NewCoin(testInput.ExpiryMarkets[1].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.ExpiryMarkets[1].Ticker, testInput.ExpiryMarkets[1].QuoteDenom, oracleBase, oracleQuote, oracleType, expiry))

			amountNeeded = math.Int(sdk.NewDec(1000))

			marketID = types.NewExpiryFuturesMarketID(testInput.ExpiryMarkets[1].Ticker, testInput.ExpiryMarkets[1].QuoteDenom, oracleBase, oracleQuote, oracleType, expiry)
		})

		JustBeforeEach(func() {
			_, err = msgServer.InstantExpiryFuturesMarketLaunch(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount.Sub(amountNeeded)))
			})

			It("Should have funded community pool", func() {
				poolBalanceAfter := app.DistrKeeper.GetFeePool(ctx)

				Expect(len(poolBalanceAfter.CommunityPool)).NotTo(BeZero())

				for _, coin := range poolBalanceAfter.CommunityPool {
					Expect(coin.Denom).To(Equal("inj"))
					Expect(coin.Amount.String()).To(Equal(amountNeeded.ToDec().String()))
				}
			})

			It("Should have correct market fields", func() {
				marketInfo := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, marketID)
				expectedRelayerFeeShareRate := sdk.NewDecWithPrec(40, 2)

				Expect(marketInfo.Ticker).To(Equal(testInput.ExpiryMarkets[1].Ticker))
				Expect(marketInfo.QuoteDenom).To(Equal(testInput.ExpiryMarkets[1].QuoteDenom))
				Expect(marketInfo.MakerFeeRate).To(Equal(testInput.ExpiryMarkets[1].MakerFeeRate))
				Expect(marketInfo.TakerFeeRate).To(Equal(testInput.ExpiryMarkets[1].TakerFeeRate))
				Expect(marketInfo.InitialMarginRatio).To(Equal(testInput.ExpiryMarkets[1].InitialMarginRatio))
				Expect(marketInfo.MaintenanceMarginRatio).To(Equal(testInput.ExpiryMarkets[1].MaintenanceMarginRatio))
				Expect(marketInfo.MinPriceTickSize).To(Equal(testInput.ExpiryMarkets[1].MinPriceTickSize))
				Expect(marketInfo.MinQuantityTickSize).To(Equal(testInput.ExpiryMarkets[1].MinQuantityTickSize))
				Expect(marketInfo.OracleBase).To(Equal(testInput.ExpiryMarkets[1].OracleBase))
				Expect(marketInfo.OracleQuote).To(Equal(testInput.ExpiryMarkets[1].OracleQuote))
				Expect(marketInfo.OracleType).To(Equal(testInput.ExpiryMarkets[1].OracleType))
				Expect(marketInfo.Status).To(Equal(types.MarketStatus_Active))
				Expect(marketInfo.RelayerFeeShareRate).To(Equal(expectedRelayerFeeShareRate))
			})
		})

		Context("With sender not having enough balance", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(500)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with insufficient funds error", func() {
				errorMessage := "spendable balance " + amountMinted.String() + " is smaller than " + amountNeeded.String() + "inj: "
				expectedError := errorMessage + sdkerrors.ErrInsufficientFunds.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When quoteDenom is invalid", func() {
			BeforeEach(func() {
				message.QuoteDenom = "SMTH"

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with invalid quote denom error", func() {
				errorMessage := "denom " + message.QuoteDenom + " does not exist in supply: "
				expectedError := errorMessage + types.ErrInvalidQuoteDenom.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When expiry market already exists", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				msgServer.InstantExpiryFuturesMarketLaunch(sdk.WrapSDKContext(ctx), message)
			})

			It("Should be invalid with expiry market exists error", func() {
				errorMessagePart1 := fmt.Sprintf("ticker %s quoteDenom %s oracle base %s quote %s expiry %d: ", message.Ticker, message.QuoteDenom, message.OracleBase, message.OracleQuote, message.Expiry)
				expectedError := errorMessagePart1 + types.ErrExpiryFuturesMarketExists.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When oracle for expiry market is invalid", func() {
			BeforeEach(func() {
				message.OracleType = oracletypes.OracleType_Uma

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with invalid oracle error", func() {
				errorMessagePart1 := "type Uma base " + message.OracleBase + " quote " + message.OracleQuote + ": "
				expectedError := errorMessagePart1 + types.ErrInvalidOracle.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Prevent conditions", func() {
			It("Prevent expiry market launch when an equivalent market launch proposal already exists", func() {
				message.Ticker = testInput.ExpiryMarkets[2].Ticker
				message.QuoteDenom = testInput.ExpiryMarkets[2].QuoteDenom

				// create insurance fund for expiry market
				depositCoin := sdk.NewInt64Coin(message.QuoteDenom, 100)
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{depositCoin})
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.Coins{depositCoin})
				err := app.InsuranceKeeper.CreateInsuranceFund(
					ctx,
					sender,
					depositCoin,
					message.Ticker,
					message.QuoteDenom,
					message.OracleBase,
					message.OracleQuote,
					message.OracleType,
					message.Expiry,
				)
				Expect(err).To(BeNil())

				// submit expiry market launch proposal
				content := types.NewExpiryFuturesMarketLaunchProposal(
					"expiry market launch proposal",
					"expiry market launch proposal for specific market",
					message.Ticker,
					message.QuoteDenom,
					message.OracleBase,
					message.OracleQuote,
					message.OracleScaleFactor,
					message.OracleType,
					message.Expiry,
					message.InitialMarginRatio,
					message.MaintenanceMarginRatio,
					message.MakerFeeRate,
					message.TakerFeeRate,
					message.MinPriceTickSize,
					message.MinQuantityTickSize,
				)
				proposalMsg, err := govtypesv1.NewLegacyContent(content, app.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String())
				Expect(err).To(BeNil())
				proposal, err := app.GovKeeper.SubmitProposal(ctx, []sdk.Msg{proposalMsg}, "", content.Title, content.Description, sender)
				Expect(err).To(BeNil())

				// proposal to active queue
				app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

				// prepare coins for instant market launch
				amount := sdk.Coins{sdk.NewCoin("inj", math.Int(sdk.NewDec(10000)))}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				// try execution of instant expiry market
				_, err = msgServer.InstantExpiryFuturesMarketLaunch(sdk.WrapSDKContext(ctx), message)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(types.ErrMarketLaunchProposalAlreadyExists.Error()))
			})
		})
	})

	Describe("CreateDerivativeLimitOrder for vanilla orders", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCreateDerivativeLimitOrder
			balanceUsed  sdk.Dec
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(message.Order.Margin)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.Order.OrderInfo.SubaccountId = ""
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.Order.OrderInfo.SubaccountId = simpleSubaccountId
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.FeeRecipient = ""
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have deposited relayer fee share back to sender", func() {
				counterPartyAddress := testexchange.SampleAccountAddr2
				counterPartySubaccountId := testexchange.SampleDefaultSubaccountAddr2.String()
				testexchange.MintAndDeposit(app, ctx, counterPartySubaccountId, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				sellOrder := &types.MsgCreateDerivativeLimitOrder{
					Sender: counterPartyAddress.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: counterPartySubaccountId,
							FeeRecipient: testexchange.SampleAccountAddr3.String(),
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						Margin:       sdk.NewDec(2000),
						OrderType:    types.OrderType_SELL,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), sellOrder)
				testexchange.OrFail(err)
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)
				if !testexchange.IsUsingDefaultSubaccount() {
					defaultSubaccount := types.MustSdkAddressWithNonceToSubaccountID(types.SubaccountIDToSdkAddress(common.HexToHash(subaccountID)), 0)
					depositDefaultAfter := testexchange.GetBankAndDepositFunds(app, ctx, defaultSubaccount, testInput.Perps[0].QuoteDenom)
					depositAfter.AvailableBalance = depositAfter.AvailableBalance.Add(depositDefaultAfter.AvailableBalance)
					depositAfter.TotalBalance = depositAfter.TotalBalance.Add(depositDefaultAfter.TotalBalance)
				}

				expectedFeeReturn := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity).Mul(derivativeMarket.RelayerFeeShareRate)
				expectedChange := balanceUsed.Sub(expectedFeeReturn)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(expectedChange).String()), "incorrect available balance")
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.Sub(expectedChange).String()), "incorrect total balance")
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with derivative market not found error", func() {
				errorMessage := "active derivative market for marketID " + message.Order.MarketId + " not found: "
				expectedError := errorMessage + types.ErrDerivativeMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When price tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDecWithPrec(2001, 5)
			})

			It("Should be invalid with invalid price error", func() {
				errorMessage1 := "price " + message.Order.OrderInfo.Price.String()
				errorMessage2 := " must be a multiple of the minimum price tick size " + derivativeMarket.MinPriceTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When quantity tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(2, 5)
			})

			It("Should be invalid with invalid quantity error", func() {
				errorMessage1 := "quantity " + message.Order.OrderInfo.Quantity.String()
				errorMessage2 := " must be a multiple of the minimum quantity tick size " + derivativeMarket.MinQuantityTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidQuantity.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When deposits of subaccount less than needed", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(100000)
			})

			It("Should be invalid with insufficient deposit error", func() {
				fee := message.Order.OrderInfo.Quantity.Mul(message.Order.OrderInfo.Price).Mul(derivativeMarket.TakerFeeRate)
				marginHold := message.Order.Margin.Add(fee)
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), derivativeMarket.QuoteDenom, sdk.NewDec(100000), marginHold)
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When deposits of subaccount are not existing", func() {
			var anotherSubaccount = testexchange.SampleSubaccountAddr3.String()
			BeforeEach(func() {
				anotherAccount := testexchange.SampleAccountAddr3
				message.Order.OrderInfo.SubaccountId = anotherSubaccount
				message.Sender = anotherAccount.String()
			})

			It("Should be invalid with insufficient deposit error", func() {
				Expect(testexchange.IsExpectedInsufficientFundsErrorType(common.HexToHash(anotherSubaccount), err)).To(BeTrue())
			})
		})

		Context("When margin is below InitialMarginRatio requirement", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(199)
			})

			It("Should be invalid with insufficient order margin error", func() {
				marginRequirement := message.Order.OrderInfo.Quantity.Mul(message.Order.OrderInfo.Price).Mul(derivativeMarket.InitialMarginRatio)
				errorMessage1 := "InitialMarginRatio Check: need at least " + marginRequirement.String()
				errorMessage2 := " but got " + message.Order.Margin.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInsufficientOrderMargin.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When margin is below MarkPriceThreshold requirement", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(220)
				message.Order.OrderInfo.Price = sdk.NewDec(2100)
			})

			It("Should be invalid with insufficient order margin error", func() {
				markPriceThreshold := message.Order.ComputeInitialMarginRequirementMarkPriceThreshold(derivativeMarket.InitialMarginRatio)
				errorMessage1 := "Buy MarkPriceThreshold Check: mark/trigger price " + sdk.NewDec(2000).String()
				errorMessage2 := " must be GTE " + markPriceThreshold.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInsufficientOrderMargin.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When already placed a market order", func() {
			BeforeEach(func() {
				subaccountIdSeller := types.MustSdkAddressWithNonceToSubaccountID(testexchange.SampleAccountAddr1, 3)

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdSeller.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				derivativeMarketBuyOrderMessage := &types.MsgCreateDerivativeMarketOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), derivativeMarketBuyOrderMessage)
				testexchange.OrFail(err2)
			})

			It("Should be allowed to mix limit and market orders", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When there are existing reduce-only orders", func() {
			BeforeEach(func() {
				subaccountIdBuyer := types.MustSdkAddressWithNonceToSubaccountID(testexchange.SampleAccountAddr1, 3)

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("which are better priced than vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {
					existingReduceOnlyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: sender.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: subaccountID,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2010),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_BUY,
							Margin:       sdk.NewDec(0),
							TriggerPrice: nil,
						},
					}

					// Create two reduce only orders.
					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						true,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						true,
					)

					Expect(metadataBefore.ReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(metadataBefore.AggregateReduceOnlyQuantity).To(Equal(metadataAfter.AggregateReduceOnlyQuantity))
				})
			})

			Context("which are equally priced to vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {
					existingReduceOnlyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: sender.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: subaccountID,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2000),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_BUY,
							Margin:       sdk.NewDec(0),
							TriggerPrice: nil,
						},
					}

					// Create two reduce only orders.
					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						true,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						true,
					)

					expectedReduceOnlyLimitOrderCount := metadataBefore.ReduceOnlyLimitOrderCount - 2
					expectedAggregateReduceOnlyQuantity := metadataBefore.AggregateReduceOnlyQuantity.Sub(sdk.NewDec(2))

					Expect(expectedReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(expectedAggregateReduceOnlyQuantity.String()).To(Equal(metadataAfter.AggregateReduceOnlyQuantity.String()))
				})
			})

			Context("which are worst priced than vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {
					existingReduceOnlyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: sender.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: subaccountID,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(1990),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_BUY,
							Margin:       sdk.NewDec(0),
							TriggerPrice: nil,
						},
					}

					// Create two reduce only orders.
					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						true,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						true,
					)

					expectedReduceOnlyLimitOrderCount := metadataBefore.ReduceOnlyLimitOrderCount - 2
					expectedAggregateReduceOnlyQuantity := metadataBefore.AggregateReduceOnlyQuantity.Sub(sdk.NewDec(2))

					Expect(expectedReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(expectedAggregateReduceOnlyQuantity.String()).To(Equal(metadataAfter.AggregateReduceOnlyQuantity.String()))
				})
			})
		})

		Context("When there are already max allowed vanilla only orders", func() {
			BeforeEach(func() {
				for i := 0; i < 20; i++ {
					derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: sender.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: subaccountID,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2000),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_BUY,
							Margin:       sdk.NewDec(200),
							TriggerPrice: nil,
						},
					}

					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
					testexchange.OrFail(err2)
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				}
			})

			It("Should be invalid with max orders per side error", func() {
				Expect(err).Should(MatchError(types.ErrExceedsOrderSideCount))
			})
		})
	})

	Describe("CreateDerivativeLimitOrder for reduce-only orders", func() {
		var (
			err     error
			message *types.MsgCreateDerivativeLimitOrder
			deposit *types.Deposit
		)
		buyer := testexchange.SampleAccountAddr1
		seller := testexchange.SampleAccountAddr2

		BeforeEach(func() {
			subaccountIdBuyer := testexchange.SampleSubaccountAddr1
			subaccountIdSeller := testexchange.SampleSubaccountAddr2

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer.String(),
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller.String(),
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(1000),
					TriggerPrice: nil,
				},
			}

			_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer.String(),
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(0),
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(21, 1)
				message.Order.OrderInfo.SubaccountId = ""
			})

			It("Should be resized", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.Order.OrderInfo.SubaccountId = simpleSubaccountId
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(21, 1)
			})

			It("Should be resized", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When quantity is greater than position quantity", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(21, 1)
			})

			It("Should be resized", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When direction of the order is same with direction of existing position", func() {
			BeforeEach(func() {
				message.Order.OrderType = types.OrderType_BUY
			})

			It("Should be invalid with invalid reduce only position direction error", func() {
				expectedError := types.ErrInvalidReduceOnlyPositionDirection.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When price is bankrupting the position", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(1000)
			})

			It("Should be invalid with price surpasses bankruptcy price error", func() {
				expectedError := types.ErrPriceSurpassesBankruptcyPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When position does not exist", func() {
			BeforeEach(func() {
				invalidSubaccount := types.MustSdkAddressWithNonceToSubaccountID(testexchange.SampleAccountAddr1, 3)
				message.Order.OrderInfo.SubaccountId = invalidSubaccount.String()
			})

			It("Should be invalid with position not found error", func() {
				errorMessage1 := "Position for marketID " + common.HexToHash(message.Order.MarketId).Hex()
				errorMessage2 := " subaccountID " + common.HexToHash(message.Order.OrderInfo.SubaccountId).Hex() + " not found: "
				expectedError := errorMessage1 + errorMessage2 + types.ErrPositionNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When reduce-only order's quantity is greater than position's quantity", func() {
			Context("while there is no existing reduce-only order", func() {
				BeforeEach(func() {
					message.Order.OrderInfo.Quantity = sdk.NewDec(3)
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("while there is existing reduce-only order", func() {
				BeforeEach(func() {
					existingReduceOnlyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: buyer.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: message.Order.OrderInfo.SubaccountId,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2000),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(0),
							TriggerPrice: nil,
						},
					}

					_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		Context("When there is an existing vanilla order", func() {
			Context("which is better priced than reduce-only order", func() {
				BeforeEach(func() {
					existingVanillaOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: buyer.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: message.Order.OrderInfo.SubaccountId,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(1900),
								Quantity:     sdk.NewDec(2),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(2000),
							TriggerPrice: nil,
						},
					}

					_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be rejected because existing order >= position.size", func() {
					Expect(errors.Is(err, types.ErrInsufficientPositionQuantity))
				})
			})

			Context("which is equally priced with reduce-only order", func() {
				BeforeEach(func() {
					existingVanillaOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: buyer.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: message.Order.OrderInfo.SubaccountId,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2000),
								Quantity:     sdk.NewDec(2),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(2000),
							TriggerPrice: nil,
						},
					}

					_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)
					testexchange.OrFail(err)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be rejected because existing order >= position.size", func() {
					Expect(errors.Is(err, types.ErrInsufficientPositionQuantity))
				})
			})

			Context("which is worst priced than reduce-only order", func() {
				BeforeEach(func() {
					existingVanillaOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: buyer.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: message.Order.OrderInfo.SubaccountId,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2001),
								Quantity:     sdk.NewDec(2),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(2000),
							TriggerPrice: nil,
						},
					}

					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("CreateDerivativeLimitOrder in PostOnly mode", func() {
		var (
			err                       error
			subaccountID              string
			counterparty_subaccountID string
			message                   *types.MsgCreateDerivativeLimitOrder
			counterpartyOrders        []*types.MsgCreateDerivativeLimitOrder
			balanceUsed               sdk.Dec
			deposit                   *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1
		counterparty := testexchange.SampleAccountAddr2

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			counterparty_subaccountID = testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			exchangeParams := app.ExchangeKeeper.GetParams(ctx)
			exchangeParams.PostOnlyModeHeightThreshold = ctx.BlockHeight() + 2000
			app.ExchangeKeeper.SetParams(ctx, exchangeParams)

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, counterparty_subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			counterpartyOrders = createCounterpartyDerivativeOrders(
				counterparty.String(),
				counterparty_subaccountID,
				derivativeMarket.MarketId)
		})

		sendOrders := func() {
			for _, counterpartyOrder := range counterpartyOrders {
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), counterpartyOrder)
			}
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(message.Order.Margin)
		}

		Context("With all valid fields", func() {
			BeforeEach(sendOrders)
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("Crossing Top of the Book", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(2105)
				sendOrders()
			})
			It("Should throw post-only mode error", func() {
				expectedError := "Post-only order exceeds top of book price"
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateDerivativeMarketOrder for vanilla orders", func() {
		var (
			err                            error
			subaccountID                   string
			message                        *types.MsgCreateDerivativeMarketOrder
			derivativeLimitBuyOrderMessage *types.MsgCreateDerivativeLimitOrder
			balanceUsed                    sdk.Dec
			deposit                        *types.Deposit
			buyer                          = testexchange.SampleAccountAddr1
			seller                         = testexchange.SampleAccountAddr2
		)

		BeforeEach(func() {
			subaccountIdBuyer := testexchange.SampleSubaccountAddr1
			subaccountIdSeller := testexchange.SampleSubaccountAddr2
			subaccountID = subaccountIdSeller.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage = &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer.String(),
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2100),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(3000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = &types.MsgCreateDerivativeMarketOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), message)
			feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(message.Order.Margin)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.Order.OrderInfo.SubaccountId = ""
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.Order.OrderInfo.SubaccountId = simpleSubaccountId
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.FeeRecipient = ""
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have deposited relayer fee share back to sender", func() {
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)
				if !testexchange.IsUsingDefaultSubaccount() {
					defaultSubaccount := types.MustSdkAddressWithNonceToSubaccountID(types.SubaccountIDToSdkAddress(common.HexToHash(subaccountID)), 0)
					depositDefaultAfter := testexchange.GetBankAndDepositFunds(app, ctx, defaultSubaccount, testInput.Perps[0].QuoteDenom)
					depositAfter.AvailableBalance = depositAfter.AvailableBalance.Add(depositDefaultAfter.AvailableBalance)
					depositAfter.TotalBalance = depositAfter.TotalBalance.Add(depositDefaultAfter.TotalBalance)
				}

				notional := derivativeLimitBuyOrderMessage.Order.OrderInfo.Price.Mul(message.Order.OrderInfo.Quantity)
				reducedFee := notional.Mul(derivativeMarket.TakerFeeRate).Mul(sdk.NewDec(1).Sub(derivativeMarket.RelayerFeeShareRate))
				balanceNeeded := message.Order.Margin.Add(reducedFee)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).String()), "incorrect available balance")
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.Sub(balanceNeeded).String()), "incorrect total balance")
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with derivative market not found error", func() {
				errorMessage := "active derivative market for marketID " + message.Order.MarketId + " not found: "
				expectedError := errorMessage + types.ErrDerivativeMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When price tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDecWithPrec(2001, 5)
			})

			It("Should be invalid with invalid price error", func() {
				errorMessage1 := "price " + message.Order.OrderInfo.Price.String()
				errorMessage2 := " must be a multiple of the minimum price tick size " + derivativeMarket.MinPriceTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When quantity tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(2, 5)
			})

			It("Should be invalid with invalid quantity error", func() {
				errorMessage1 := "quantity " + message.Order.OrderInfo.Quantity.String()
				errorMessage2 := " must be a multiple of the minimum quantity tick size " + derivativeMarket.MinQuantityTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidQuantity.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When deposits of subaccount less than needed", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(100000)
			})

			It("Should be invalid with insufficient depposit error", func() {
				fee := message.Order.OrderInfo.Quantity.Mul(message.Order.OrderInfo.Price).Mul(derivativeMarket.TakerFeeRate)
				marginHold := message.Order.Margin.Add(fee)
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), derivativeMarket.QuoteDenom, sdk.NewDec(100000), marginHold)
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When deposits of subaccount are not existing", func() {
			BeforeEach(func() {
				invalidSubaccount := types.MustSdkAddressWithNonceToSubaccountID(seller, 3)
				message.Order.OrderInfo.SubaccountId = invalidSubaccount.String()
			})

			It("Should be invalid with insufficient depposit error", func() {
				Expect(err).Should(MatchError(types.ErrInsufficientDeposit))
			})
		})

		Context("When margin is below InitialMarginRatio requirement", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(199)
			})

			It("Should be invalid with insufficient order margin error", func() {
				marginRequirement := message.Order.OrderInfo.Quantity.Mul(message.Order.OrderInfo.Price).Mul(derivativeMarket.InitialMarginRatio)
				errorMessage1 := "InitialMarginRatio Check: need at least " + marginRequirement.String()
				errorMessage2 := " but got " + message.Order.Margin.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInsufficientOrderMargin.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When margin is below MarkPriceThreshold requirement", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(190)
				message.Order.OrderInfo.Price = sdk.NewDec(1900)
			})

			It("Should be invalid with insufficient order margin error", func() {
				markPriceThreshold := message.Order.ComputeInitialMarginRequirementMarkPriceThreshold(derivativeMarket.InitialMarginRatio)
				errorMessage1 := "Sell MarkPriceThreshold Check: mark/trigger price " + sdk.NewDec(2000).String()
				errorMessage2 := " must be LTE " + markPriceThreshold.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInsufficientOrderMargin.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When there is no liquidity in the orderbook", func() {
			BeforeEach(func() {
				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When worst price is not valid", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(2200)
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When already placed another market order", func() {
			BeforeEach(func() {
				subaccountIdSeller := types.MustSdkAddressWithNonceToSubaccountID(seller, 3)

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdSeller.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				derivativeMarketBuyOrderMessage := &types.MsgCreateDerivativeMarketOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), derivativeMarketBuyOrderMessage)
				testexchange.OrFail(err2)
			})

			It("Should be invalid with derivative market order already exists error", func() {
				expectedError := types.ErrMarketOrderAlreadyExists.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When already placed a limit order", func() {
			BeforeEach(func() {
				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err2)
			})

			It("Should be allowed", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When there are existing reduce-only orders", func() {
			BeforeEach(func() {
				// switch seller and buyer
				subaccountIdSeller := testexchange.SampleSubaccountAddr1
				subaccountIdBuyer := testexchange.SampleSubaccountAddr2

				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: buyer.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdSeller.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(4),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err2)

				derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				derivativeLimitBuyOrderMessage1 := &types.MsgCreateDerivativeLimitOrder{
					Sender: buyer.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdSeller.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(10),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage1)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("which are worst priced than vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {
					existingReduceOnlyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: seller.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: subaccountID,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2010),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(0),
							TriggerPrice: nil,
						},
					}

					// Create two reduce only orders.
					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						false,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(derivativeMarket.MarketId),
						common.HexToHash(subaccountID),
						false,
					)

					expectedReduceOnlyLimitOrderCount := metadataBefore.ReduceOnlyLimitOrderCount - 2
					expectedAggregateReduceOnlyQuantity := metadataBefore.AggregateReduceOnlyQuantity.Sub(sdk.NewDec(2))

					Expect(expectedReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(expectedAggregateReduceOnlyQuantity.String()).To(Equal(metadataAfter.AggregateReduceOnlyQuantity.String()))
				})
			})
		})
	})

	Describe("CreateDerivativeMarketOrder for reduce-only orders", func() {
		var (
			err     error
			message *types.MsgCreateDerivativeMarketOrder
			deposit *types.Deposit
			buyer   = testexchange.SampleAccountAddr1
			seller  = testexchange.SampleAccountAddr2
		)

		BeforeEach(func() {
			subaccountIdBuyer := testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller := testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(1000),
					TriggerPrice: nil,
				},
			}

			_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			derivativeLimitBuyOrderMessage2 := &types.MsgCreateDerivativeLimitOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage2)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = &types.MsgCreateDerivativeMarketOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(0),
					TriggerPrice: nil,
				},
			}

		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When quantity is greater than position quantity", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(21, 1)
			})

			It("Should be resized", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When direction of the order is same with direction of existing position", func() {
			BeforeEach(func() {
				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: testexchange.SampleSubaccountAddr2.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(4),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				message.Order.OrderType = types.OrderType_BUY
			})

			It("Should be invalid with invalid reduce only position direction error", func() {
				expectedError := types.ErrInvalidReduceOnlyPositionDirection.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When price is bankrupting the position", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(1000)
			})

			It("Should be invalid with price surpasses bankruptcy price error", func() {
				expectedError := types.ErrPriceSurpassesBankruptcyPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When there is no liquidity in the orderbook", func() {
			BeforeEach(func() {
				derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: testexchange.SampleSubaccountAddr2.String(),
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When worst price is not valid", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(2200)
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When position does not exist", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.SubaccountId = testexchange.SampleSubaccountAddr3.String()
			})

			It("Should be invalid with position not found error", func() {
				errorMessage1 := "Position for marketID " + common.HexToHash(message.Order.MarketId).Hex()
				errorMessage2 := " subaccountID " + common.HexToHash(message.Order.OrderInfo.SubaccountId).Hex() + " not found: "
				expectedError := errorMessage1 + errorMessage2 + types.ErrPositionNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When reduce-only order's quantity is greater than position's quantity", func() {
			Context("while there is no existing reduce-only order", func() {
				BeforeEach(func() {
					message.Order.OrderInfo.Quantity = sdk.NewDec(3)
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("while there is existing reduce-only order", func() {
				BeforeEach(func() {
					existingReduceOnlyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: buyer.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: message.Order.OrderInfo.SubaccountId,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2100),
								Quantity:     sdk.NewDec(1),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(0),
							TriggerPrice: nil,
						},
					}

					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		Context("When there is an existing vanilla order", func() {
			// TODO - Check if can reproduce with better priced than reduce-only
			Context("which is worst priced than reduce-only order", func() {
				BeforeEach(func() {
					existingVanillaOrderMessage := &types.MsgCreateDerivativeLimitOrder{
						Sender: buyer.String(),
						Order: types.DerivativeOrder{
							MarketId: derivativeMarket.MarketId,
							OrderInfo: types.OrderInfo{
								SubaccountId: message.Order.OrderInfo.SubaccountId,
								FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
								Price:        sdk.NewDec(2001),
								Quantity:     sdk.NewDec(2),
							},
							OrderType:    types.OrderType_SELL,
							Margin:       sdk.NewDec(2000),
							TriggerPrice: nil,
						},
					}

					_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("CreateDerivativeMarketOrder for vanilla orders special cases", func() {
		var (
			err                            error
			subaccountIdBuyer              = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller             = testexchange.SampleSubaccountAddr2.String()
			message                        *types.MsgCreateDerivativeMarketOrder
			balanceUsed                    sdk.Dec
			deposit                        *types.Deposit
			balanceBefore                  *types.Deposit
			balanceAfterCreation           *types.Deposit
			balanceAfterEndBlocker         *types.Deposit
			derivativeLimitBuyOrderMessage *types.MsgCreateDerivativeLimitOrder
			buyer                          = testexchange.SampleAccountAddr1
			seller                         = testexchange.SampleAccountAddr2
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage = &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(500),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = &types.MsgCreateDerivativeMarketOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}
		})

		Context("When the order to be executed against market order is getting canceled", func() {
			BeforeEach(func() {
				balanceBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)
				_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), message)

				feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
				balanceUsed = feePaid.Add(message.Order.Margin)

				balanceAfterCreation = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)

				derivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					common.HexToHash(derivativeMarket.MarketId),
					true,
				)

				restingOrder := derivativeOrders[0]
				cancelMessage := &types.MsgCancelDerivativeOrder{
					Sender:       buyer.String(),
					MarketId:     derivativeMarket.MarketId,
					SubaccountId: subaccountIdBuyer,
					OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
				}

				_, err2 := msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), cancelMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				balanceAfterEndBlocker = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should not be executed and have correct balances", func() {
				availableBalanceAfterCreation := deposit.AvailableBalance.Sub(balanceUsed)

				Expect(balanceBefore.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(balanceBefore.TotalBalance).To(Equal(deposit.TotalBalance))

				Expect(balanceAfterCreation.AvailableBalance).To(Equal(availableBalanceAfterCreation))
				Expect(balanceAfterCreation.TotalBalance).To(Equal(deposit.TotalBalance))

				Expect(balanceAfterEndBlocker.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(balanceAfterEndBlocker.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When the order to be executed against market order becomes invalid", func() {
			BeforeEach(func() {
				balanceBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)
				_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), message)

				feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
				balanceUsed = feePaid.Add(message.Order.Margin)

				balanceAfterCreation = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)

				oracleBase, oracleQuote, _ := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				invalidationPrice := sdk.NewDec(1750)

				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(invalidationPrice, ctx.BlockTime().Unix()))

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				balanceAfterEndBlocker = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should not be executed and have correct balances", func() {
				availableBalanceAfterCreation := deposit.AvailableBalance.Sub(balanceUsed)

				Expect(balanceBefore.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(balanceBefore.TotalBalance).To(Equal(deposit.TotalBalance))

				Expect(balanceAfterCreation.AvailableBalance).To(Equal(availableBalanceAfterCreation))
				Expect(balanceAfterCreation.TotalBalance).To(Equal(deposit.TotalBalance))

				Expect(balanceAfterEndBlocker.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(balanceAfterEndBlocker.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateDerivativeMarketOrder in PostOnly mode", func() {
		var (
			err                            error
			subaccountID                   string
			message                        *types.MsgCreateDerivativeMarketOrder
			derivativeLimitBuyOrderMessage *types.MsgCreateDerivativeLimitOrder
			deposit                        *types.Deposit
			buyer                          = testexchange.SampleAccountAddr1
			seller                         = testexchange.SampleAccountAddr2
		)

		BeforeEach(func() {
			subaccountIdBuyer := testexchange.SampleSubaccountAddr1
			subaccountIdSeller := testexchange.SampleSubaccountAddr2
			subaccountID = subaccountIdSeller.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			exchangeParams := app.ExchangeKeeper.GetParams(ctx)
			exchangeParams.PostOnlyModeHeightThreshold = ctx.BlockHeight() + 2000
			app.ExchangeKeeper.SetParams(ctx, exchangeParams)

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller.String(), sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage = &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer.String(),
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY_PO,
					Margin:       sdk.NewDec(3000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			//ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			//Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = &types.MsgCreateDerivativeMarketOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(1900),
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeMarketOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should throw post-only mode error", func() {
				expectedError := fmt.Sprintf("cannot create market orders in post only mode until height %d: exchange is in post-only mode", ctx.BlockHeight()+2000)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

	})

	Describe("CancelDerivativeOrder for vanilla orders", func() {
		var (
			err                         error
			message                     *types.MsgCancelDerivativeOrder
			balanceUsed                 sdk.Dec
			deposit                     *types.Deposit
			derivativeLimitOrderMessage *types.MsgCreateDerivativeLimitOrder
			buyer                       = testexchange.SampleAccountAddr1
			subaccountIdBuyer           = testexchange.SampleSubaccountAddr1.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitOrderMessage = &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
			testexchange.OrFail(err2)
			feePaid := derivativeMarket.MakerFeeRate.Mul(derivativeLimitOrderMessage.Order.OrderInfo.Price).Mul(derivativeLimitOrderMessage.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(derivativeLimitOrderMessage.Order.Margin)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			derivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
				ctx,
				common.HexToHash(derivativeMarket.MarketId),
				true,
			)

			restingOrder := derivativeOrders[0]

			message = &types.MsgCancelDerivativeOrder{
				Sender:       buyer.String(),
				MarketId:     derivativeMarket.MarketId,
				SubaccountId: subaccountIdBuyer,
				OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
		})

		Describe("When they are not filled", func() {
			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
				})

				It("Should have correct event", func() {
					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Margin).To(Equal(derivativeLimitOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(derivativeLimitOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})

			Context("With empty subaccount id", func() {
				BeforeEach(func() {
					if !testexchange.IsUsingDefaultSubaccount() {
						Skip("only makes sense with default subaccount")
					}
					message.SubaccountId = ""
				})
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
				})
			})

			Context("With simplified subaccount id", func() {
				BeforeEach(func() {
					simpleSubaccountId := "1"
					if testexchange.IsUsingDefaultSubaccount() {
						simpleSubaccountId = "0"
					}
					message.SubaccountId = simpleSubaccountId
				})
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
				})
			})

			Context("When market does not exist", func() {
				BeforeEach(func() {
					message.MarketId = common.HexToHash("0x9").Hex()
				})

				It("Should be invalid with derivative market not found error", func() {
					errorMessage := "active derivative market doesn't exist " + message.MarketId + ": "
					expectedError := errorMessage + types.ErrDerivativeMarketNotFound.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have not updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
				})
			})

			Context("When order does not exist", func() {
				BeforeEach(func() {
					message.SubaccountId = types.MustSdkAddressWithNonceToSubaccountID(buyer, 2).String()
				})

				It("Should be invalid with order does not exist error", func() {
					expectedError := "Derivative Limit Order doesn't exist: " + types.ErrOrderDoesntExist.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have not updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
				})
			})
		})

		Describe("When they are partially filled", func() {
			BeforeEach(func() {
				seller := testexchange.SampleAccountAddr2
				sellerSubaccountId := testexchange.SampleSubaccountAddr2.String()

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, sellerSubaccountId, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: sellerSubaccountId,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(1),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed.Quo(sdk.NewDec(2)))))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance.Sub(balanceUsed.Quo(sdk.NewDec(2)))))
				})

				It("Should have correct event", func() {
					expectedFillable := derivativeLimitOrderMessage.Order.OrderInfo.Quantity.Sub(sdk.OneDec())

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(expectedFillable))
							Expect(event.LimitOrder.Margin).To(Equal(derivativeLimitOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(derivativeLimitOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(derivativeLimitOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})
		})

		Describe("When they are fully filled", func() {
			BeforeEach(func() {
				seller := testexchange.SampleAccountAddr2
				sellerSubaccountId := testexchange.SampleSubaccountAddr2.String()

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, sellerSubaccountId, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: sellerSubaccountId,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be invalid with order does not exist error", func() {
					expectedError := "Derivative Limit Order doesn't exist: " + types.ErrOrderDoesntExist.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have updated balances for all filled quantity", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance.Sub(balanceUsed)))
				})
			})
		})
	})

	Describe("CancelDerivativeOrder for transient buy orders", func() {
		var (
			err          error
			message      *types.MsgCancelDerivativeOrder
			deposit      *types.Deposit
			sender       = testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(1121),
				TotalBalance:     sdk.NewDec(1121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: testInput.Perps[0].MarketID.Hex(),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(40),
					},
					Margin:       sdk.NewDec(1000),
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.Perps[0].MarketID,
				true,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelDerivativeOrder{
				Sender:       sender.String(),
				MarketId:     testInput.Perps[0].MarketID.Hex(),
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					true,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					true,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelDerivativeOrder for transient sell orders", func() {
		var (
			err          error
			message      *types.MsgCancelDerivativeOrder
			deposit      *types.Deposit
			sender       = testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()
		)

		BeforeEach(func() {

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(5121),
				TotalBalance:     sdk.NewDec(5121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: testInput.Perps[0].MarketID.Hex(),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(1000),
						Quantity:     sdk.NewDec(3),
					},
					Margin:       sdk.NewDec(4000),
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.Perps[0].MarketID,
				false,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelDerivativeOrder{
				Sender:       sender.String(),
				MarketId:     testInput.Perps[0].MarketID.Hex(),
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					false,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					false,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelDerivativeOrder for transient reduce-only buy orders", func() {
		var (
			err                error
			message            *types.MsgCancelDerivativeOrder
			deposit            *types.Deposit
			depositBefore      *types.Deposit
			buyer              = testexchange.SampleAccountAddr1
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(7121),
				TotalBalance:     sdk.NewDec(7121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			matchBuyerAndSeller(
				testInput,
				app,
				ctx,
				msgServer,
				types.MarketType_Perpetual,
				sdk.NewDec(2000),
				sdk.NewDec(2),
				sdk.NewDec(2000),
				false,
				common.HexToHash(subaccountIdBuyer),
				common.HexToHash(subaccountIdSeller),
			)

			depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

			derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: testInput.Perps[0].MarketID.Hex(),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(1800),
						Quantity:     sdk.NewDec(2),
					},
					Margin:       sdk.ZeroDec(),
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.Perps[0].MarketID,
				true,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelDerivativeOrder{
				Sender:       buyer.String(),
				MarketId:     testInput.Perps[0].MarketID.Hex(),
				SubaccountId: subaccountIdBuyer,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdBuyer), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(depositBefore.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(depositBefore.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					true,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					true,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelDerivativeOrder for transient reduce-only sell orders", func() {
		var (
			err                error
			message            *types.MsgCancelDerivativeOrder
			deposit            *types.Deposit
			depositBefore      *types.Deposit
			seller             = testexchange.SampleAccountAddr1
			subaccountIdSeller = testexchange.SampleSubaccountAddr1.String()
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(7121),
				TotalBalance:     sdk.NewDec(7121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			matchBuyerAndSeller(
				testInput,
				app,
				ctx,
				msgServer,
				types.MarketType_Perpetual,
				sdk.NewDec(2000),
				sdk.NewDec(2),
				sdk.NewDec(2000),
				true,
				common.HexToHash(subaccountIdSeller), // super clever trick, to have reducable position
				common.HexToHash(subaccountIdBuyer),
			)

			depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)

			derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: testInput.Perps[0].MarketID.Hex(),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(1800),
						Quantity:     sdk.NewDec(2),
					},
					Margin:       sdk.ZeroDec(),
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}

			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.Perps[0].MarketID,
				false,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelDerivativeOrder{
				Sender:       seller.String(),
				MarketId:     testInput.Perps[0].MarketID.Hex(),
				SubaccountId: subaccountIdSeller,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(depositBefore.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(depositBefore.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					false,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.Perps[0].MarketID,
					false,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelDerivativeOrder for reduce-only orders", func() {
		var (
			err                                   error
			message                               *types.MsgCancelDerivativeOrder
			deposit                               *types.Deposit
			derivativeLimitReduceOnlyOrderMessage *types.MsgCreateDerivativeLimitOrder
			buyer                                 = testexchange.SampleAccountAddr1
			seller                                = testexchange.SampleAccountAddr2
			subaccountIdBuyer                     = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller                    = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(1000),
					TriggerPrice: nil,
				},
			}

			_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			derivativeLimitReduceOnlyOrderMessage = &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(0),
					TriggerPrice: nil,
				},
			}

			_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitReduceOnlyOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			derivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
				ctx,
				common.HexToHash(derivativeMarket.MarketId),
				false,
			)

			restingOrder := derivativeOrders[0]

			message = &types.MsgCancelDerivativeOrder{
				Sender:       buyer.String(),
				MarketId:     derivativeMarket.MarketId,
				SubaccountId: subaccountIdBuyer,
				OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
		})

		Describe("When they are not filled", func() {
			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have correct event", func() {
					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Margin).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})
		})

		Describe("When they are partially filled", func() {
			BeforeEach(func() {
				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: buyer.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(1),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have correct event", func() {
					expectedFillable := derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity.Sub(sdk.OneDec())

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.Perps[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(expectedFillable))
							Expect(event.LimitOrder.Margin).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(derivativeLimitReduceOnlyOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})
		})

		Describe("When they are fully filled", func() {
			BeforeEach(func() {
				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: buyer.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be invalid with order does not exist error", func() {
					expectedError := "Derivative Limit Order doesn't exist: " + types.ErrOrderDoesntExist.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Describe("When position has been netted out", func() {
			BeforeEach(func() {
				derivativeLimitNettingOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: buyer.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(1900),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitNettingOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(100000),
					TotalBalance:     sdk.NewDec(100000),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
					Sender: buyer.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountIdBuyer,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(1000),
						TriggerPrice: nil,
					},
				}

				_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be invalid with derivative position does not exist error", func() {
					Expect(err).NotTo(BeNil())
				})
			})
		})
	})

	Describe("IncreasePositionMargin", func() {
		var (
			err                     error
			sender                  = testexchange.SampleAccountAddr3
			destinationSubaccountId string
			sourceSubaccountId      string
			message                 *types.MsgIncreasePositionMargin
			marginBefore            sdk.Dec
			marginToAdd             sdk.Dec
			deposit                 *types.Deposit
			buyer                   = testexchange.SampleAccountAddr1
			seller                  = testexchange.SampleAccountAddr2
			subaccountIdBuyer       = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller      = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			sourceSubaccountId = testexchange.SampleSubaccountAddr3.String()
			destinationSubaccountId = subaccountIdBuyer

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, sourceSubaccountId, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitBuyOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: buyer.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdBuyer,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			derivativeLimitSellOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: seller.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIdSeller,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}

			_, err2 = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			marginToAdd = sdk.NewDec(1000)

			message = &types.MsgIncreasePositionMargin{
				Sender:                  sender.String(),
				MarketId:                derivativeMarket.MarketId,
				SourceSubaccountId:      sourceSubaccountId,
				DestinationSubaccountId: destinationSubaccountId,
				Amount:                  marginToAdd,
			}

			positionBefore := app.ExchangeKeeper.GetPosition(ctx, common.HexToHash(derivativeMarket.MarketId), common.HexToHash(destinationSubaccountId))
			marginBefore = positionBefore.Margin
		})

		JustBeforeEach(func() {
			_, err = msgServer.IncreasePositionMargin(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balance and position", func() {
				positionAfter := app.ExchangeKeeper.GetPosition(ctx, common.HexToHash(derivativeMarket.MarketId), common.HexToHash(destinationSubaccountId))
				marginAfter := positionAfter.Margin
				expectedMarginAfter := marginBefore.Add(marginToAdd)

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(sourceSubaccountId), testInput.Perps[0].QuoteDenom)
				expectedBalanceAfter := deposit.AvailableBalance.Sub(marginToAdd)

				Expect(marginAfter).To(Equal(expectedMarginAfter))
				Expect(depositAfter.AvailableBalance.String()).To(Equal(expectedBalanceAfter.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(expectedBalanceAfter.String()))
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.SourceSubaccountId = ""
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balance and position", func() {
				positionAfter := app.ExchangeKeeper.GetPosition(ctx, common.HexToHash(derivativeMarket.MarketId), common.HexToHash(destinationSubaccountId))
				marginAfter := positionAfter.Margin
				expectedMarginAfter := marginBefore.Add(marginToAdd)

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(sourceSubaccountId), testInput.Perps[0].QuoteDenom)
				expectedBalanceAfter := deposit.AvailableBalance.Sub(marginToAdd)

				Expect(marginAfter).To(Equal(expectedMarginAfter))
				Expect(depositAfter.AvailableBalance.String()).To(Equal(expectedBalanceAfter.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(expectedBalanceAfter.String()))
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.SourceSubaccountId = simpleSubaccountId
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balance and position", func() {
				positionAfter := app.ExchangeKeeper.GetPosition(ctx, common.HexToHash(derivativeMarket.MarketId), common.HexToHash(destinationSubaccountId))
				marginAfter := positionAfter.Margin
				expectedMarginAfter := marginBefore.Add(marginToAdd)

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(sourceSubaccountId), testInput.Perps[0].QuoteDenom)
				expectedBalanceAfter := deposit.AvailableBalance.Sub(marginToAdd)

				Expect(marginAfter).To(Equal(expectedMarginAfter))
				Expect(depositAfter.AvailableBalance.String()).To(Equal(expectedBalanceAfter.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(expectedBalanceAfter.String()))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.MarketId = "0x9"
			})

			It("Should be invalid with derivative market not found error", func() {
				errorMessage := "active derivative market for marketID " + common.HexToHash(message.MarketId).Hex() + " not found: "
				expectedError := errorMessage + types.ErrDerivativeMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When position does not exist", func() {
			BeforeEach(func() {
				message.DestinationSubaccountId = sourceSubaccountId
			})

			It("Should be invalid with position not found error", func() {
				errorMessage := "subaccountID " + common.HexToHash(sourceSubaccountId).Hex() + " marketID " + common.HexToHash(message.MarketId).Hex() + ": "
				expectedError := errorMessage + types.ErrPositionNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When sourceSubaccountId does not have enough funds", func() {
			BeforeEach(func() {
				marginToAdd = sdk.NewDec(1000000)

				message = &types.MsgIncreasePositionMargin{
					Sender:                  sender.String(),
					MarketId:                derivativeMarket.MarketId,
					SourceSubaccountId:      sourceSubaccountId,
					DestinationSubaccountId: destinationSubaccountId,
					Amount:                  marginToAdd,
				}

				positionBefore := app.ExchangeKeeper.GetPosition(ctx, common.HexToHash(derivativeMarket.MarketId), common.HexToHash(destinationSubaccountId))
				marginBefore = positionBefore.Margin
			})

			It("Should be invalid with insufficient deposit error", func() {
				Expect(testexchange.IsExpectedInsufficientFundsErrorType(common.HexToHash(sourceSubaccountId), err)).To(BeTrue())
			})
		})
	})

	Describe("Create orders with client order id", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCreateDerivativeLimitOrder
			balanceUsed  sdk.Dec
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100000),
				TotalBalance:     sdk.NewDec(100000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Perps[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
						Cid:          "my_great_order_1",
					},
					OrderType:    types.OrderType_BUY,
					Margin:       sdk.NewDec(2000),
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(message.Order.Margin)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("Using cid to cancel", func() {
			It("Should be cancellable", func() {
				var message = &types.MsgCancelDerivativeOrder{
					Sender:       sender.String(),
					MarketId:     derivativeMarket.MarketId,
					SubaccountId: subaccountID,
					OrderHash:    "",
					Cid:          "my_great_order_1",
				}
				_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), message)
				Expect(err).To(BeNil())

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

		})

		Context("Creating another vanilla order with the same cid", func() {
			It("Should fail", func() {
				var message = &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
							Cid:          "my_great_order_1",
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}

				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
				Expect(err.Error()).To(Equal(types.ErrClientOrderIdAlreadyExists.Error()))

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)
				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))

			})

		})

		Context("Creating another order vanilla with different cid", func() {
			It("Should be valid", func() {
				var message = &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
							Cid:          "my_great_order_2",
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
				feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
				var balanceNeededForSecondOrder = feePaid.Add(message.Order.Margin)

				Expect(err).To(BeNil())

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)
				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed).Sub(balanceNeededForSecondOrder)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

		})

		Context("If cancelled, cid should be reusable", func() {
			JustBeforeEach(func() {
				var cancelMessage = &types.MsgCancelDerivativeOrder{
					Sender:       sender.String(),
					MarketId:     derivativeMarket.MarketId,
					SubaccountId: subaccountID,
					OrderHash:    "",
					Cid:          "my_great_order_1",
				}
				_, err = msgServer.CancelDerivativeOrder(sdk.WrapSDKContext(ctx), cancelMessage)

				var message = &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
							Cid:          "my_great_order_2",
						},
						OrderType:    types.OrderType_BUY,
						Margin:       sdk.NewDec(2000),
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
				feePaid := derivativeMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
				balanceUsed = feePaid.Add(message.Order.Margin)
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.Perps[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

			It("Should have updated balances based only on last order", func() {

			})

		})

	})
})

func createCounterpartyDerivativeOrders(
	counterparty_address string,
	counterparty_subaccountID string,
	marketId string,
) (counterpartyOrders []*types.MsgCreateDerivativeLimitOrder) {
	counterpartyOrders = append(counterpartyOrders,
		&types.MsgCreateDerivativeLimitOrder{
			Sender: counterparty_address,
			Order: types.DerivativeOrder{
				MarketId: marketId,
				OrderInfo: types.OrderInfo{
					SubaccountId: counterparty_subaccountID,
					FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					Price:        sdk.NewDec(1900),
					Quantity:     sdk.NewDec(2),
				},
				OrderType:    types.OrderType_BUY_PO,
				Margin:       sdk.NewDec(2000),
				TriggerPrice: nil,
			},
		})
	counterpartyOrders = append(counterpartyOrders,
		&types.MsgCreateDerivativeLimitOrder{
			Sender: counterparty_address,
			Order: types.DerivativeOrder{
				MarketId: marketId,
				OrderInfo: types.OrderInfo{
					SubaccountId: counterparty_subaccountID,
					FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					Price:        sdk.NewDec(2100),
					Quantity:     sdk.NewDec(2),
				},
				OrderType:    types.OrderType_SELL_PO,
				Margin:       sdk.NewDec(2000),
				TriggerPrice: nil,
			},
		})
	return counterpartyOrders
}
