package types_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"

	. "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Proposals Spots", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context
		handler   govtypes.Handler
		marketId  string
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 2, 0, 0)

		app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		spotMarket := app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
		marketId = spotMarket.MarketId
	})

	Describe("Validate SpotMarketParamUpdateProposal", func() {
		var proposal SpotMarketParamUpdateProposal
		var err error
		var expectedError string
		var stringOfLength100 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

		BeforeEach(func() {
			makerFeeRate := sdk.NewDecWithPrec(1, 3)
			takerFeeRate := sdk.NewDecWithPrec(15, 4)
			relayerFeeShareRate := sdk.NewDecWithPrec(40, 2)
			minPriceTickSize := sdk.NewDecWithPrec(1, 4)
			minQuantityTickSize := sdk.NewDecWithPrec(1, 4)

			proposal = SpotMarketParamUpdateProposal{
				Title:               "Just a Title",
				Description:         "Just a Description",
				MarketId:            marketId,
				MakerFeeRate:        &makerFeeRate,
				TakerFeeRate:        &takerFeeRate,
				RelayerFeeShareRate: &relayerFeeShareRate,
				MinPriceTickSize:    &minPriceTickSize,
				MinQuantityTickSize: &minQuantityTickSize,
				Status:              MarketStatus_Active,
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
					proposal.Status = MarketStatus_Unspecified
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
				proposal.Status = MarketStatus_Unspecified

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

				expectedError = proposal.MarketId + ": " + ErrMarketInvalid.Error()
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
					expectedError = expectedError1 + ErrInvalidPriceTickSize.Error()
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
					expectedError = expectedError1 + ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					minPriceTickSize := MaxOrderPrice.Add(sdk.OneDec())
					proposal.MinPriceTickSize = &minPriceTickSize

					expectedError = "unsupported tick size amount: " + ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					minPriceTickSize := sdk.NewDecWithPrec(8, 18)
					proposal.MinPriceTickSize = &minPriceTickSize

					expectedError = "unsupported tick size: " + ErrInvalidPriceTickSize.Error()
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
					expectedError = expectedError1 + ErrInvalidQuantityTickSize.Error()
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
					expectedError = expectedError1 + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					minQuantityTickSize := MaxOrderPrice.Add(sdk.OneDec())
					proposal.MinQuantityTickSize = &minQuantityTickSize

					expectedError = "unsupported tick size amount: " + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is unsupported", func() {
				BeforeEach(func() {
					minQuantityTickSize := sdk.NewDecWithPrec(8, 18)
					proposal.MinQuantityTickSize = &minQuantityTickSize

					expectedError = "unsupported tick size: " + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
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
					Expect(ErrFeeRatesRelation.Is(err)).To(BeTrue())
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
					Expect(ErrFeeRatesRelation.Is(err)).To(BeTrue())
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

					expectedError = ErrFeeRatesRelation.Error()
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

					expectedError = ErrFeeRatesRelation.Error()
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

				expectedError = ErrFeeRatesRelation.Error()
			})

			It("should be invalid with MakerFeeRate cannot be greater than TakerFeeRate error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When updating MakerFeeRate, but not TakerFeeRate field, which is greater than existing TakerFeeRate", func() {
			BeforeEach(func() {
				proposal.TakerFeeRate = nil
				makerFeeRate := sdk.NewDecWithPrec(5, 3)
				proposal.MakerFeeRate = &makerFeeRate

				expectedError = ErrFeeRatesRelation.Error()
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

					expectedError = proposal.Status.String() + ": " + ErrInvalidMarketStatus.Error()
				})

				It("should be invalid with invalid market status error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Status is greater than 5", func() {
				BeforeEach(func() {
					proposal.Status = 6

					expectedError = proposal.Status.String() + ": " + ErrInvalidMarketStatus.Error()
				})

				It("should be invalid with invalid market status error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})
	})

	Describe("Validate SpotMarketLaunchProposal", func() {
		var proposal SpotMarketLaunchProposal
		var err error
		var expectedError string
		var stringOfLength100 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

		BeforeEach(func() {
			proposal = SpotMarketLaunchProposal{
				Title:               "Just a Title",
				Description:         "Just a Description",
				Ticker:              testInput.Spots[1].Ticker,
				BaseDenom:           testInput.Spots[1].BaseDenom,
				QuoteDenom:          testInput.Spots[1].QuoteDenom,
				MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
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

			Context("But with same market existing", func() {
				BeforeEach(func() {
					proposal.BaseDenom = testInput.Spots[0].BaseDenom
					proposal.QuoteDenom = testInput.Spots[0].QuoteDenom

					expectedError1 := "ticker " + proposal.Ticker + " baseDenom " + proposal.BaseDenom + " quoteDenom " + proposal.QuoteDenom
					expectedError = expectedError1 + ": " + ErrSpotMarketExists.Error()
				})

				It("should be valid", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
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

		Context("With invalid MinPriceTickSize field", func() {
			Context("When MinPriceTickSize is smaller than 0", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is equal to 0", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + proposal.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = MaxOrderPrice.Add(sdk.OneDec())

					expectedError = "unsupported tick size amount: " + ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					proposal.MinPriceTickSize = sdk.NewDecWithPrec(8, 18)

					expectedError = "unsupported tick size: " + ErrInvalidPriceTickSize.Error()
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
					expectedError = expectedError1 + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is equal to 0", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + proposal.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = MaxOrderPrice.Add(sdk.OneDec())

					expectedError = "unsupported tick size amount: " + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is unsupported", func() {
				BeforeEach(func() {
					proposal.MinQuantityTickSize = sdk.NewDecWithPrec(8, 18)

					expectedError = "unsupported tick size: " + ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("Without Ticker field", func() {
			BeforeEach(func() {
				proposal.Ticker = ""

				expectedError = "ticker should not be empty or exceed 30 characters: " + ErrInvalidTicker.Error()
			})

			It("should be invalid with invalid ticker error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong BaseDenom field", func() {
			Context("When BaseDenom field is empty", func() {
				BeforeEach(func() {
					proposal.BaseDenom = ""

					expectedError = "base denom should not be empty: " + ErrInvalidBaseDenom.Error()
				})

				It("should be invalid with invalid base denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))

				})
			})

			Context("When BaseDenom does not exist", func() {
				BeforeEach(func() {
					proposal.BaseDenom = "wrongBase"

					expectedError = "denom " + proposal.BaseDenom + " does not exist in supply: " + ErrInvalidBaseDenom.Error()
				})

				It("should be invalid with invalid base denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))

				})
			})
		})

		Context("With wrong QuoteDenom field", func() {
			Context("When QuoteDenom field is empty", func() {
				BeforeEach(func() {
					proposal.QuoteDenom = ""

					expectedError = "quote denom should not be empty: " + ErrInvalidQuoteDenom.Error()
				})

				It("should be invalid with invalid quote denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))

				})
			})

			Context("When QuoteDenom does not exist", func() {
				BeforeEach(func() {
					proposal.QuoteDenom = "wrongQuote"

					expectedError = "denom " + proposal.QuoteDenom + " does not exist in supply: " + ErrInvalidQuoteDenom.Error()
				})

				It("should be invalid with invalid base denom error", func() {
					Expect(err.Error()).To(Equal(expectedError))

				})
			})
		})

		Context("With QuoteDenom field being the same with BaseDenom field", func() {
			BeforeEach(func() {
				proposal.QuoteDenom = proposal.BaseDenom

				expectedError = "base denom cannot be same with quote denom"
			})

			It("should be invalid with base denom cannot be same with quote denom error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})
})
