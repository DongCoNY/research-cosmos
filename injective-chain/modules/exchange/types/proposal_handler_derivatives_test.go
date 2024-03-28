package types_test

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Proposals Derivatives", func() {
	var (
		testInput        testexchange.TestInput
		app              *simapp.InjectiveApp
		ctx              sdk.Context
		handler          govtypes.Handler
		derivativeMarket *exchangetypes.DerivativeMarket
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 2, 0)

		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		startingPrice := sdk.NewDec(2000)

		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
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
		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)

		testexchange.OrFail(err)
	})

	Describe("Validate exchangetypes.PerpetualMarketLaunchProposal", func() {
		var proposal exchangetypes.PerpetualMarketLaunchProposal
		var err error
		var expectedError string
		var stringOfLength100 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

		BeforeEach(func() {
			oracleBase, oracleQuote, oracleType := testInput.Perps[1].OracleBase, testInput.Perps[1].OracleQuote, testInput.Perps[1].OracleType
			startingPrice := sdk.NewDec(2000)

			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			coin := sdk.NewCoin(testInput.Perps[1].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[1].Ticker, testInput.Perps[1].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

			proposal = exchangetypes.PerpetualMarketLaunchProposal{
				Title:                  "Just a Title",
				Description:            "Just a Description",
				Ticker:                 testInput.Perps[1].Ticker,
				QuoteDenom:             testInput.Perps[1].QuoteDenom,
				OracleBase:             oracleBase,
				OracleQuote:            oracleQuote,
				OracleType:             oracleType,
				OracleScaleFactor:      0,
				InitialMarginRatio:     sdk.NewDecWithPrec(5, 2),
				MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2),
				MakerFeeRate:           sdk.NewDecWithPrec(1, 3),
				TakerFeeRate:           sdk.NewDecWithPrec(15, 4),
				MinPriceTickSize:       sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize:    sdk.NewDecWithPrec(1, 4),
			}
		})

		JustBeforeEach(func() {
			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Title field", func() {
			BeforeEach(func() {
				proposal.Title = ""

				expectedError = "proposal title cannot be blank: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Title field longer than allowed", func() {
			BeforeEach(func() {
				proposal.Title = ""
				for i := 0; i < 3; i++ {
					proposal.Title += stringOfLength100
				}

				expectedError = "proposal title is longer than max length of 140: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Description field", func() {
			BeforeEach(func() {
				proposal.Description = ""

				expectedError = "proposal description cannot be blank: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Description field longer than allowed", func() {
			BeforeEach(func() {
				proposal.Description = ""
				for i := 0; i < 101; i++ {
					proposal.Description += stringOfLength100
				}

				expectedError = "proposal description is longer than max length of 10000: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Ticker field", func() {
			BeforeEach(func() {
				proposal.Ticker = ""

				expectedError = "ticker should not be empty or exceed 30 characters: " + exchangetypes.ErrInvalidTicker.Error()
			})

			It("should be invalid with invalid ticker error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid QuoteDenom field", func() {
			Context("Without QuoteDenom field", func() {
				BeforeEach(func() {
					proposal.QuoteDenom = ""

					expectedError = "quote denom should not be empty: " + exchangetypes.ErrInvalidQuoteDenom.Error()
				})

				It("should be invalid with invalid quote denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("With QuoteDenom field not existing", func() {
				BeforeEach(func() {
					proposal.QuoteDenom = "SMTH"

					expectedError = "denom " + proposal.QuoteDenom + " does not exist in supply: " + exchangetypes.ErrInvalidQuoteDenom.Error()
				})

				It("should be invalid with invalid quote denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("Without Oracle Base field", func() {
			BeforeEach(func() {
				proposal.OracleBase = ""

				expectedError = "oracle base should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Oracle Quote field", func() {
			BeforeEach(func() {
				proposal.OracleQuote = ""

				expectedError = "oracle quote should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Oracle Quote field being the same with Oracle Base field", func() {
			BeforeEach(func() {
				proposal.OracleQuote = proposal.OracleBase

				expectedError = exchangetypes.ErrSameOracles.Error()
			})

			It("should be invalid with oracle base cannot be same with oracle quote error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-correct OracleType field", func() {
			Context("When OracleType is equal to 0", func() {
				BeforeEach(func() {
					proposal.OracleType = 0

					expectedError = proposal.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OracleType is greater than 11", func() {
				BeforeEach(func() {
					proposal.OracleType = 12

					expectedError = proposal.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MakerFeeRate field", func() {
			Context("When MakerFeeRate is negative and takerFeeRate * (1 - relayerFeeShareRate) + makerFeeRate < 0", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(-2, 3)
					proposal.TakerFeeRate = sdk.NewDecWithPrec(32, 4)
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(exchangetypes.ErrFeeRatesRelation.Is(err)).To(BeTrue())
				})
			})

			Context("When MakerFeeRate is negative and takerFeeRate * (1 - relayerFeeShareRate) + makerFeeRate < minimalProtocolFeeRate", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(-2, 3)
					proposal.TakerFeeRate = sdk.NewDecWithPrec(34, 4)
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(exchangetypes.ErrFeeRatesRelation.Is(err)).To(BeTrue())
				})
			})

			Context("When MakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(1001, 3)

					expectedError = "exchange fee cannot be greater than 1: " + proposal.MakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MakerFeeRate is greater than TakerFeeRate", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(3, 3)

					expectedError = exchangetypes.ErrFeeRatesRelation.Error()
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid TakerFeeRate field", func() {
			Context("When TakerFeeRate is smaller than 0", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = sdk.NewDec(-1)

					expectedError = "exchange fee cannot be negative: " + proposal.TakerFeeRate.String()
				})

				It("should be invalid with cannot be negative error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = sdk.NewDecWithPrec(1001, 3)

					expectedError = "exchange fee cannot be greater than 1: " + proposal.TakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is smaller than MakerFeeRate", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = sdk.NewDecWithPrec(1, 4)

					expectedError = exchangetypes.ErrFeeRatesRelation.Error()
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid InitialMarginRatio field", func() {
			Context("When InitialMarginRatio is equal to 0", func() {
				BeforeEach(func() {
					proposal.InitialMarginRatio = sdk.NewDec(0)

					expectedError = "margin ratio cannot be less than minimum: " + proposal.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					proposal.InitialMarginRatio = sdk.NewDecWithPrec(1000, 3)

					expectedError = "margin ratio cannot be greater than or equal to 1: " + proposal.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is smaller than MaintenanceMarginRatio", func() {
				BeforeEach(func() {
					proposal.InitialMarginRatio = sdk.NewDecWithPrec(1, 2)

					expectedError = exchangetypes.ErrMarginsRelation.Error()
				})

				It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MaintenanceMarginRatio field", func() {
			Context("When MaintenanceMarginRatio is equal to 0", func() {
				BeforeEach(func() {
					proposal.MaintenanceMarginRatio = sdk.NewDec(0)

					expectedError = "margin ratio cannot be less than minimum: " + proposal.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					proposal.MaintenanceMarginRatio = sdk.NewDecWithPrec(1000, 3)

					expectedError = "margin ratio cannot be greater than or equal to 1: " + proposal.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is greater than InitialMarginRatio", func() {
				BeforeEach(func() {
					proposal.MaintenanceMarginRatio = sdk.NewDecWithPrec(6, 2)

					expectedError = exchangetypes.ErrMarginsRelation.Error()
				})

				It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MinPriceTickSize field", func() {
			Context("When MinPriceTickSize is smaller than 0", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is equal to 0", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())

					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.NewDecWithPrec(8, 18)

					expectedError = "unsupported tick size: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MinQuantityTickSize field", func() {
			Context("When MinQuantityTickSize is smaller than 0", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is equal to 0", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())

					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("When trying to create duplicate perpetual market", func() {
			BeforeEach(func() {
				proposal.Ticker = testInput.Perps[0].Ticker
				proposal.QuoteDenom = testInput.Perps[0].QuoteDenom
				proposal.OracleBase = testInput.Perps[0].OracleBase
				proposal.OracleQuote = testInput.Perps[0].OracleQuote
				proposal.OracleType = testInput.Perps[0].OracleType

				expectedError = "ticker " + proposal.Ticker + " quoteDenom " + proposal.QuoteDenom + ": " + exchangetypes.ErrPerpetualMarketExists.Error()
			})

			It("should be invalid with perpetual market exists error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When using invalid oracle", func() {
			BeforeEach(func() {
				proposal.OracleBase = testInput.Perps[0].OracleBase

				expectedError1 := "type " + proposal.OracleType.String() + " base " + proposal.OracleBase + " quote " + proposal.OracleQuote
				expectedError = expectedError1 + ": " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When using invalid oracle scale factor", func() {
			BeforeEach(func() {
				proposal.OracleScaleFactor = exchangetypes.MaxOracleScaleFactor + 1

				expectedError = exchangetypes.ErrExceedsMaxOracleScaleFactor.Error()
			})

			It("should be invalid with exceeds MaxOracleScaleFactor error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With no insurance fund", func() {
			BeforeEach(func() {
				proposal.OracleBase = testInput.Perps[0].OracleBase
				startingPrice := sdk.NewDec(2000)
				marketID := exchangetypes.NewPerpetualMarketID(proposal.Ticker, proposal.QuoteDenom, proposal.OracleBase, proposal.OracleQuote, proposal.OracleType)

				app.OracleKeeper.SetPriceFeedPriceState(ctx, testInput.Perps[0].OracleBase, testInput.Perps[1].OracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

				expectedError1 := "ticker " + proposal.Ticker + " marketID " + marketID.Hex()
				expectedError = expectedError1 + ": " + insurancetypes.ErrInsuranceFundNotFound.Error()
			})

			It("should be invalid with insurnace fund not found error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate ExpiryFuturesMarketLaunchProposal", func() {
		var proposal exchangetypes.ExpiryFuturesMarketLaunchProposal
		var err error
		var expectedError string
		var stringOfLength100 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

		BeforeEach(func() {
			oracleBase, oracleQuote, oracleType := testInput.Perps[1].OracleBase, testInput.Perps[1].OracleQuote, testInput.Perps[1].OracleType
			startingPrice := sdk.NewDec(2000)

			app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

			sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
			coin := sdk.NewCoin(testInput.Perps[1].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[1].Ticker, testInput.Perps[1].QuoteDenom, oracleBase, oracleQuote, oracleType, ctx.BlockTime().Unix()+1000))

			proposal = exchangetypes.ExpiryFuturesMarketLaunchProposal{
				Title:                  "Just a Title",
				Description:            "Just a Description",
				Ticker:                 testInput.Perps[1].Ticker,
				QuoteDenom:             testInput.Perps[1].QuoteDenom,
				OracleBase:             oracleBase,
				OracleQuote:            oracleQuote,
				OracleType:             oracleType,
				OracleScaleFactor:      0,
				Expiry:                 ctx.BlockTime().Unix() + 1000,
				InitialMarginRatio:     sdk.NewDecWithPrec(5, 2),
				MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2),
				MakerFeeRate:           sdk.NewDecWithPrec(1, 3),
				TakerFeeRate:           sdk.NewDecWithPrec(15, 4),
				MinPriceTickSize:       sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize:    sdk.NewDecWithPrec(1, 4),
			}
		})

		JustBeforeEach(func() {
			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Title field", func() {
			BeforeEach(func() {
				proposal.Title = ""

				expectedError = "proposal title cannot be blank: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Title field longer than allowed", func() {
			BeforeEach(func() {
				proposal.Title = ""
				for i := 0; i < 3; i++ {
					proposal.Title += stringOfLength100
				}

				expectedError = "proposal title is longer than max length of 140: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Description field", func() {
			BeforeEach(func() {
				proposal.Description = ""

				expectedError = "proposal description cannot be blank: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Description field longer than allowed", func() {
			BeforeEach(func() {
				proposal.Description = ""
				for i := 0; i < 101; i++ {
					proposal.Description += stringOfLength100
				}

				expectedError = "proposal description is longer than max length of 10000: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Ticker field", func() {
			BeforeEach(func() {
				proposal.Ticker = ""

				expectedError = "ticker should not be empty or exceed 30 characters: " + exchangetypes.ErrInvalidTicker.Error()
			})

			It("should be invalid with invalid ticker error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid QuoteDenom field", func() {
			Context("Without QuoteDenom field", func() {
				BeforeEach(func() {
					proposal.QuoteDenom = ""

					expectedError = "quote denom should not be empty: " + exchangetypes.ErrInvalidQuoteDenom.Error()
				})

				It("should be invalid with invalid quote denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("With QuoteDenom field not existing", func() {
				BeforeEach(func() {
					proposal.QuoteDenom = "SMTH"

					expectedError = "denom " + proposal.QuoteDenom + " does not exist in supply: " + exchangetypes.ErrInvalidQuoteDenom.Error()
				})

				It("should be invalid with invalid quote denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("Without Oracle Base field", func() {
			BeforeEach(func() {
				proposal.OracleBase = ""

				expectedError = "oracle base should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Oracle Quote field", func() {
			BeforeEach(func() {
				proposal.OracleQuote = ""

				expectedError = "oracle quote should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Oracle Quote field being the same with Oracle Base field", func() {
			BeforeEach(func() {
				proposal.OracleQuote = proposal.OracleBase

				expectedError = exchangetypes.ErrSameOracles.Error()
			})

			It("should be invalid with oracle base cannot be same with oracle quote error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-correct OracleType field", func() {
			Context("When OracleType is equal to 0", func() {
				BeforeEach(func() {
					proposal.OracleType = 0

					expectedError = proposal.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OracleType is greater than 11", func() {
				BeforeEach(func() {
					proposal.OracleType = 12

					expectedError = proposal.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MakerFeeRate field", func() {
			Context("When MakerFeeRate is negative and takerFeeRate * (1 - relayerFeeShareRate) + makerFeeRate < 0", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(-2, 3)
					proposal.TakerFeeRate = sdk.NewDecWithPrec(32, 4)
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(exchangetypes.ErrFeeRatesRelation.Is(err)).To(BeTrue())
				})
			})

			Context("When MakerFeeRate is negative and takerFeeRate * (1 - relayerFeeShareRate) + makerFeeRate < minimalProtocolFeeRate", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(-2, 3)
					proposal.TakerFeeRate = sdk.NewDecWithPrec(34, 4)
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(exchangetypes.ErrFeeRatesRelation.Is(err)).To(BeTrue())
				})
			})

			Context("When MakerFeeRate is greater than TakerFeeRate", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = sdk.NewDecWithPrec(3, 3)

					expectedError = exchangetypes.ErrFeeRatesRelation.Error()
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid TakerFeeRate field", func() {
			Context("When TakerFeeRate is smaller than 0", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = sdk.NewDec(-1)

					expectedError = "exchange fee cannot be negative: " + proposal.TakerFeeRate.String()
				})

				It("should be invalid with cannot be negative error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = sdk.NewDecWithPrec(1001, 3)

					expectedError = "exchange fee cannot be greater than 1: " + proposal.TakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is smaller than MakerFeeRate", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = sdk.NewDecWithPrec(1, 4)

					expectedError = exchangetypes.ErrFeeRatesRelation.Error()
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid InitialMarginRatio field", func() {
			Context("When InitialMarginRatio is equal to 0", func() {
				BeforeEach(func() {
					proposal.InitialMarginRatio = sdk.NewDec(0)

					expectedError = "margin ratio cannot be less than minimum: " + proposal.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					proposal.InitialMarginRatio = sdk.NewDecWithPrec(1000, 3)

					expectedError = "margin ratio cannot be greater than or equal to 1: " + proposal.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is smaller than MaintenanceMarginRatio", func() {
				BeforeEach(func() {
					proposal.InitialMarginRatio = sdk.NewDecWithPrec(1, 2)

					expectedError = exchangetypes.ErrMarginsRelation.Error()
				})

				It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MaintenanceMarginRatio field", func() {
			Context("When MaintenanceMarginRatio is equal to 0", func() {
				BeforeEach(func() {
					proposal.MaintenanceMarginRatio = sdk.NewDec(0)

					expectedError = "margin ratio cannot be less than minimum: " + proposal.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					proposal.MaintenanceMarginRatio = sdk.NewDecWithPrec(1000, 3)

					expectedError = "margin ratio cannot be greater than or equal to 1: " + proposal.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is greater than InitialMarginRatio", func() {
				BeforeEach(func() {
					proposal.MaintenanceMarginRatio = sdk.NewDecWithPrec(6, 2)

					expectedError = exchangetypes.ErrMarginsRelation.Error()
				})

				It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MinPriceTickSize field", func() {
			Context("When MinPriceTickSize is smaller than 0", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is equal to 0", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())

					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.NewDecWithPrec(8, 18)

					expectedError = "unsupported tick size: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MinQuantityTickSize field", func() {
			Context("When MinQuantityTickSize is smaller than 0", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is equal to 0", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())

					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("When trying to create duplicate expiry market", func() {
			BeforeEach(func() {
				handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
				handler(ctx, (govtypes.Content)(&proposal))

				expectedError1 := "ticker " + proposal.Ticker + " quoteDenom " + proposal.QuoteDenom
				expectedError2 := " oracle base " + proposal.OracleBase + " quote " + proposal.OracleQuote + " expiry " + fmt.Sprint(proposal.Expiry)
				expectedError = expectedError1 + expectedError2 + ": " + exchangetypes.ErrExpiryFuturesMarketExists.Error()
			})

			It("should be invalid with expiry futures market exists error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When using invalid oracle", func() {
			BeforeEach(func() {
				proposal.OracleBase = testInput.Perps[0].OracleBase

				expectedError1 := "type " + proposal.OracleType.String() + " base " + proposal.OracleBase + " quote " + proposal.OracleQuote
				expectedError = expectedError1 + ": " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When using invalid oracle scale factor", func() {
			BeforeEach(func() {
				proposal.OracleScaleFactor = exchangetypes.MaxOracleScaleFactor + 1

				expectedError = exchangetypes.ErrExceedsMaxOracleScaleFactor.Error()
			})

			It("should be invalid with exceeds MaxOracleScaleFactor error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With no insurance fund", func() {
			BeforeEach(func() {
				proposal.OracleBase = testInput.Perps[0].OracleBase
				startingPrice := sdk.NewDec(2000)
				marketID := exchangetypes.NewExpiryFuturesMarketID(proposal.Ticker, proposal.QuoteDenom, proposal.OracleBase, proposal.OracleQuote, proposal.OracleType, proposal.Expiry)

				app.OracleKeeper.SetPriceFeedPriceState(ctx, testInput.Perps[0].OracleBase, testInput.Perps[1].OracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

				expectedError1 := "ticker " + proposal.Ticker + " marketID " + marketID.Hex()
				expectedError = expectedError1 + ": " + insurancetypes.ErrInsuranceFundNotFound.Error()
			})

			It("should be invalid with insurnace fund not found error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong expiry field", func() {
			Context("Without Expiry field", func() {
				BeforeEach(func() {
					proposal.Expiry = 0

					expectedError = "expiry should not be empty: " + exchangetypes.ErrInvalidExpiry.Error()
				})

				It("should be invalid with invalid expiry date error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("With Expiry field being an old timestamp", func() {
				BeforeEach(func() {
					proposal.Expiry = 10

					expectedError1 := "ticker " + proposal.Ticker + " quoteDenom " + proposal.QuoteDenom + " oracleBase " + proposal.OracleBase
					expectedError2 := " oracleQuote " + proposal.OracleQuote + " expiry " + fmt.Sprint(proposal.Expiry) + " expired. Current blocktime "
					expectedError = expectedError1 + expectedError2 + fmt.Sprint(ctx.BlockTime().Unix()) + ": " + exchangetypes.ErrExpiryFuturesMarketExpired.Error()
				})

				It("should be invalid with expiry futures market expired error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})
	})

	Describe("Validate DerivativeMarketParamUpdateProposal", func() {
		var proposal exchangetypes.DerivativeMarketParamUpdateProposal
		var err error
		var expectedError string
		var stringOfLength100 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

		BeforeEach(func() {
			initialMarginRatio := sdk.NewDecWithPrec(5, 2)
			maintenanceMarginRatio := sdk.NewDecWithPrec(2, 2)
			makerFeeRate := sdk.NewDecWithPrec(1, 3)
			takerFeeRate := sdk.NewDecWithPrec(15, 4)
			relayerFeeShareRate := sdk.NewDecWithPrec(40, 2)
			minPriceTickSize := sdk.NewDecWithPrec(1, 4)
			minQuantityTickSize := sdk.NewDecWithPrec(1, 4)

			proposal = exchangetypes.DerivativeMarketParamUpdateProposal{
				Title:                  "Just a Title",
				Description:            "Just a Description",
				MarketId:               derivativeMarket.MarketId,
				InitialMarginRatio:     &initialMarginRatio,
				MaintenanceMarginRatio: &maintenanceMarginRatio,
				MakerFeeRate:           &makerFeeRate,
				TakerFeeRate:           &takerFeeRate,
				RelayerFeeShareRate:    &relayerFeeShareRate,
				MinPriceTickSize:       &minPriceTickSize,
				MinQuantityTickSize:    &minQuantityTickSize,
				Status:                 exchangetypes.MarketStatus_Active,
			}
		})

		JustBeforeEach(func() {
			handler = exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
			err = handler(ctx, (govtypes.Content)(&proposal))
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With some nil fields", func() {
			Context("When MakerFeeRate is nil", func() {
				BeforeEach(func() {
					proposal.MakerFeeRate = nil
				})
				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
			Context("When TakerFeeRate is nil", func() {
				BeforeEach(func() {
					proposal.TakerFeeRate = nil
				})
				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
			Context("When RelayerFeeShareRate is nil", func() {
				BeforeEach(func() {
					proposal.RelayerFeeShareRate = nil
				})
				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
			Context("When MinPriceTickSize is nil", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = nil
				})
				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
			Context("When MinQuantityTickSize is nil", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = nil
				})
				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
			Context("When Status is unspecified", func() {
				BeforeEach(func() {
					proposal.Status = exchangetypes.MarketStatus_Unspecified
				})
				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		Context("When all valid fields are nil", func() {
			BeforeEach(func() {
				proposal.MakerFeeRate = nil
				proposal.TakerFeeRate = nil
				proposal.RelayerFeeShareRate = nil
				proposal.MinPriceTickSize = nil
				proposal.MinQuantityTickSize = nil
				proposal.InitialMarginRatio = nil
				proposal.MaintenanceMarginRatio = nil
				proposal.Status = exchangetypes.MarketStatus_Unspecified

				expectedError = "At least one field should not be nil: invalid proposal content"
			})

			It("should be invalid with invalid proposal error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Title field", func() {
			BeforeEach(func() {
				proposal.Title = ""

				expectedError = "proposal title cannot be blank: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Title field longer than allowed", func() {
			BeforeEach(func() {
				proposal.Title = ""
				for i := 0; i < 3; i++ {
					proposal.Title += stringOfLength100
				}

				expectedError = "proposal title is longer than max length of 140: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Description field", func() {
			BeforeEach(func() {
				proposal.Description = ""

				expectedError = "proposal description cannot be blank: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Description field longer than allowed", func() {
			BeforeEach(func() {
				proposal.Description = ""
				for i := 0; i < 101; i++ {
					proposal.Description += stringOfLength100
				}

				expectedError = "proposal description is longer than max length of 10000: invalid proposal content"
			})

			It("should be invalid with invalid proposal content error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				proposal.MarketId = ""

				expectedError = proposal.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid MinPriceTickSize field", func() {
			Context("When MinPriceTickSize is smaller than 0", func() {
				BeforeEach(func() {
					minPriceTickSize := sdk.NewDec(-1)
					proposal.MinPriceTickSize = &minPriceTickSize

					expectedError1 := "tick size cannot be negative: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is equal to 0", func() {
				BeforeEach(func() {
					minPriceTickSize := sdk.ZeroDec()
					proposal.MinPriceTickSize = &minPriceTickSize

					expectedError1 := "tick size cannot be zero: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					minPriceTickSize := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
					proposal.MinPriceTickSize = &minPriceTickSize

					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					minPriceTickSize := sdk.NewDecWithPrec(8, 18)
					proposal.MinPriceTickSize = &minPriceTickSize

					expectedError = "unsupported tick size: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MinQuantityTickSize field", func() {
			Context("When MinQuantityTickSize is smaller than 0", func() {
				BeforeEach(func() {
					minQuantityTickSize := sdk.NewDec(-1)
					proposal.MinQuantityTickSize = &minQuantityTickSize

					expectedError1 := "tick size cannot be negative: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is equal to 0", func() {
				BeforeEach(func() {
					minQuantityTickSize := sdk.ZeroDec()
					proposal.MinQuantityTickSize = &minQuantityTickSize

					expectedError1 := "tick size cannot be zero: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					minQuantityTickSize := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
					proposal.MinQuantityTickSize = &minQuantityTickSize

					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is unsupported", func() {
				BeforeEach(func() {
					minQuantityTickSize := sdk.NewDecWithPrec(8, 18)
					proposal.MinQuantityTickSize = &minQuantityTickSize

					expectedError = "unsupported tick size: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid InitialMarginRatio field", func() {
			Context("When InitialMarginRatio is equal to 0", func() {
				BeforeEach(func() {
					initialMarginRatio := sdk.ZeroDec()
					proposal.InitialMarginRatio = &initialMarginRatio

					expectedError = "margin ratio cannot be less than minimum: " + proposal.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					initialMarginRatio := sdk.NewDecWithPrec(1000, 3)
					proposal.InitialMarginRatio = &initialMarginRatio

					expectedError = "margin ratio cannot be greater than or equal to 1: " + proposal.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is smaller than MaintenanceMarginRatio", func() {
				BeforeEach(func() {
					initialMarginRatio := sdk.NewDecWithPrec(1, 2)
					proposal.InitialMarginRatio = &initialMarginRatio

					expectedError = exchangetypes.ErrMarginsRelation.Error()
				})

				It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid MaintenanceMarginRatio field", func() {
			Context("When MaintenanceMarginRatio is equal to 0", func() {
				BeforeEach(func() {
					maintenanceMarginRatio := sdk.ZeroDec()
					proposal.MaintenanceMarginRatio = &maintenanceMarginRatio

					expectedError = "margin ratio cannot be less than minimum: " + proposal.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					maintenanceMarginRatio := sdk.NewDecWithPrec(1000, 3)
					proposal.MaintenanceMarginRatio = &maintenanceMarginRatio

					expectedError = "margin ratio cannot be greater than or equal to 1: " + proposal.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is greater than InitialMarginRatio", func() {
				BeforeEach(func() {
					maintenanceMarginRatio := sdk.NewDecWithPrec(6, 2)
					proposal.MaintenanceMarginRatio = &maintenanceMarginRatio

					expectedError = exchangetypes.ErrMarginsRelation.Error()
				})

				It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("When updating InitialMarginRatio, but not MaintenanceMarginRatio field, which is smaller than existing MaintenanceMarginRatio", func() {
			BeforeEach(func() {
				proposal.MaintenanceMarginRatio = nil
				initialMarginRatio := sdk.NewDecWithPrec(1, 2)
				proposal.InitialMarginRatio = &initialMarginRatio

				expectedError = exchangetypes.ErrMarginsRelation.Error()
			})

			It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When updating MaintenanceMarginRatio, but not InitialMarginRatio field, which is greater than existing InitialMarginRatio", func() {
			BeforeEach(func() {
				proposal.InitialMarginRatio = nil
				maintenanceMarginRatio := sdk.NewDecWithPrec(6, 2)
				proposal.MaintenanceMarginRatio = &maintenanceMarginRatio

				expectedError = exchangetypes.ErrMarginsRelation.Error()
			})

			It("should be invalid with MaintenanceMarginRatio cannot be greater than InitialMarginRatio error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid MakerFeeRate field", func() {
			Context("When MakerFeeRate is negative and takerFeeRate * (1 - relayerFeeShareRate) + makerFeeRate < 0", func() {
				BeforeEach(func() {
					makerFeeRate := sdk.NewDecWithPrec(-1, 3)
					takerFeeRate := sdk.NewDecWithPrec(26, 4)
					relayerFeeShareRate := sdk.NewDecWithPrec(6, 1)

					proposal.MakerFeeRate = &makerFeeRate
					proposal.TakerFeeRate = &takerFeeRate
					proposal.RelayerFeeShareRate = &relayerFeeShareRate
				})

				It("should be invalid with maker does not match taker fee error", func() {
					Expect(exchangetypes.ErrFeeRatesRelation.Is(err)).To(BeTrue())
				})
			})

			Context("When MakerFeeRate is negative and takerFeeRate * (1 - relayerFeeShareRate) + makerFeeRate < minimalProtocolFeeRate", func() {
				BeforeEach(func() {
					makerFeeRate := sdk.NewDecWithPrec(-1, 3)
					takerFeeRate := sdk.NewDecWithPrec(262, 5)
					relayerFeeShareRate := sdk.NewDecWithPrec(6, 1)

					proposal.MakerFeeRate = &makerFeeRate
					proposal.TakerFeeRate = &takerFeeRate
					proposal.RelayerFeeShareRate = &relayerFeeShareRate
				})

				It("should be invalid with maker does not match taker fee error", func() {
					Expect(exchangetypes.ErrFeeRatesRelation.Is(err)).To(BeTrue())
				})
			})

			Context("When MakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					makerFeeRate := sdk.NewDecWithPrec(1001, 3)
					proposal.MakerFeeRate = &makerFeeRate

					expectedError = "exchange fee cannot be greater than 1: " + proposal.MakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MakerFeeRate is greater than TakerFeeRate", func() {
				BeforeEach(func() {
					makerFeeRate := sdk.NewDecWithPrec(3, 3)
					proposal.MakerFeeRate = &makerFeeRate

					expectedError = exchangetypes.ErrFeeRatesRelation.Error()
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid TakerFeeRate field", func() {
			Context("When TakerFeeRate is smaller than 0", func() {
				BeforeEach(func() {
					takerFeeRate := sdk.NewDec(-1)
					proposal.TakerFeeRate = &takerFeeRate

					expectedError = "exchange fee cannot be negative: " + proposal.TakerFeeRate.String()
				})

				It("should be invalid with cannot be negative error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					takerFeeRate := sdk.NewDecWithPrec(1001, 3)
					proposal.TakerFeeRate = &takerFeeRate

					expectedError = "exchange fee cannot be greater than 1: " + proposal.TakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is smaller than MakerFeeRate", func() {
				BeforeEach(func() {
					takerFeeRate := sdk.NewDecWithPrec(1, 4)
					proposal.TakerFeeRate = &takerFeeRate

					expectedError = exchangetypes.ErrFeeRatesRelation.Error()
				})

				It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("When updating TakerFeeRate, but not MakerFeeRate field, which is smaller than existing MakerFeeRate", func() {
			BeforeEach(func() {
				proposal.MakerFeeRate = nil
				takerFeeRate := sdk.NewDecWithPrec(1, 4)
				proposal.TakerFeeRate = &takerFeeRate

				expectedError = exchangetypes.ErrFeeRatesRelation.Error()
			})

			It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When updating MakerFeeRate, but not TakerFeeRate field, which is greater than existing TakerFeeRate", func() {
			BeforeEach(func() {
				proposal.TakerFeeRate = nil
				makerFeeRate := sdk.NewDecWithPrec(4, 3)
				proposal.MakerFeeRate = &makerFeeRate

				expectedError = exchangetypes.ErrFeeRatesRelation.Error()
			})

			It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid RelayerFeeShareRate field", func() {
			Context("When RelayerFeeShareRate is smaller than 0", func() {
				BeforeEach(func() {
					relayerFeeShareRate := sdk.NewDec(-1)
					proposal.RelayerFeeShareRate = &relayerFeeShareRate

					expectedError = "exchange fee cannot be negative: " + proposal.RelayerFeeShareRate.String()
				})

				It("should be invalid with cannot be negative error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When RelayerFeeShareRate is greater than 1", func() {
				BeforeEach(func() {
					relayerFeeShareRate := sdk.NewDecWithPrec(1001, 3)
					proposal.RelayerFeeShareRate = &relayerFeeShareRate

					expectedError = "exchange fee cannot be greater than 1: " + proposal.RelayerFeeShareRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With non-correct Status field", func() {
			Context("When Status is smaller than 0", func() {
				BeforeEach(func() {
					proposal.Status = -1

					expectedError = proposal.Status.String() + ": " + exchangetypes.ErrInvalidMarketStatus.Error()
				})

				It("should be invalid with invalid market status error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Status is greater than 5", func() {
				BeforeEach(func() {
					proposal.Status = 6

					expectedError = proposal.Status.String() + ": " + exchangetypes.ErrInvalidMarketStatus.Error()
				})

				It("should be invalid with invalid market status error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})
	})
})
