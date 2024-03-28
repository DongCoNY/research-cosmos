package types_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Msgs Derivatives", func() {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	Describe("Validate MsgInstantPerpetualMarketLaunch", func() {
		var msg exchangetypes.MsgInstantPerpetualMarketLaunch
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgInstantPerpetualMarketLaunch{
				Sender:                 testexchange.DefaultAddress,
				Ticker:                 "INJ / ATOM",
				QuoteDenom:             "inj",
				OracleBase:             "inj-band",
				OracleQuote:            "atom-band",
				OracleScaleFactor:      0,
				OracleType:             oracletypes.OracleType_Band,
				MakerFeeRate:           sdk.NewDecWithPrec(1, 3),
				TakerFeeRate:           sdk.NewDecWithPrec(2, 3),
				InitialMarginRatio:     sdk.NewDecWithPrec(5, 2),
				MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2),
				MinPriceTickSize:       sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize:    sdk.NewDecWithPrec(1, 4),
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Ticker field", func() {
			BeforeEach(func() {
				msg.Ticker = ""
				expectedError = "ticker should not be empty or exceed 30 characters: " + exchangetypes.ErrInvalidTicker.Error()
			})

			It("should be invalid with invalid ticker error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without QuoteDenom field", func() {
			BeforeEach(func() {
				msg.QuoteDenom = ""
				expectedError = "quote denom should not be empty: " + exchangetypes.ErrInvalidQuoteDenom.Error()
			})

			It("should be invalid with invalid quote denom error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Oracle Base field", func() {
			BeforeEach(func() {
				msg.OracleBase = ""
				expectedError = "oracle base should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Oracle Quote field", func() {
			BeforeEach(func() {
				msg.OracleQuote = ""
				expectedError = "oracle quote should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Oracle Quote field being the same with Oracle Base field", func() {
			BeforeEach(func() {
				msg.OracleQuote = msg.OracleBase
				expectedError = exchangetypes.ErrSameOracles.Error()
			})

			It("should be invalid with oracle base cannot be same with oracle quote error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When using invalid oracle scale factor", func() {
			BeforeEach(func() {
				msg.OracleScaleFactor = exchangetypes.MaxOracleScaleFactor + 1
				expectedError = exchangetypes.ErrExceedsMaxOracleScaleFactor.Error()
			})

			It("should be invalid with exceeds exchangetypes.MaxOracleScaleFactor error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-correct OracleType field", func() {
			Context("When OracleType is equal to 0", func() {
				BeforeEach(func() {
					msg.OracleType = 0
					expectedError = msg.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OracleType is greater than 11", func() {
				BeforeEach(func() {
					msg.OracleType = 12
					expectedError = msg.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With valid MakerFeeRate field", func() {
			Context("When MakerFeeRate is smaller than 0", func() {
				BeforeEach(func() {
					msg.MakerFeeRate = sdk.NewDecWithPrec(-1, 4)
				})

				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("When MakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					msg.MakerFeeRate = sdk.NewDecWithPrec(1001, 3)
					expectedError = "exchange fee cannot be greater than 1: " + msg.MakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MakerFeeRate is greater than TakerFeeRate", func() {
				BeforeEach(func() {
					msg.MakerFeeRate = sdk.NewDecWithPrec(3, 3)
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
					msg.TakerFeeRate = sdk.NewDec(-1)
					expectedError = "exchange fee cannot be negative: " + msg.TakerFeeRate.String()
				})

				It("should be invalid with cannot be negative error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					msg.TakerFeeRate = sdk.NewDecWithPrec(1001, 3)
					expectedError = "exchange fee cannot be greater than 1: " + msg.TakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is smaller than MakerFeeRate", func() {
				BeforeEach(func() {
					msg.TakerFeeRate = sdk.NewDecWithPrec(1, 4)

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
					msg.InitialMarginRatio = sdk.NewDec(0)
					expectedError = "margin ratio cannot be less than minimum: " + msg.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					msg.InitialMarginRatio = sdk.NewDecWithPrec(1000, 3)
					expectedError = "margin ratio cannot be greater than or equal to 1: " + msg.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is smaller than MaintenanceMarginRatio", func() {
				BeforeEach(func() {
					msg.InitialMarginRatio = sdk.NewDecWithPrec(1, 2)
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
					msg.MaintenanceMarginRatio = sdk.NewDec(0)
					expectedError = "margin ratio cannot be less than minimum: " + msg.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					msg.MaintenanceMarginRatio = sdk.NewDecWithPrec(1000, 3)
					expectedError = "margin ratio cannot be greater than or equal to 1: " + msg.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is greater than InitialMarginRatio", func() {
				BeforeEach(func() {
					msg.MaintenanceMarginRatio = sdk.NewDecWithPrec(6, 2)
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
					msg.MinPriceTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + msg.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is equal to 0", func() {
				BeforeEach(func() {
					msg.MinPriceTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + msg.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					msg.MinPriceTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					msg.MinPriceTickSize = sdk.NewDecWithPrec(8, 18)
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
					msg.MinQuantityTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + msg.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is equal to 0", func() {
				BeforeEach(func() {
					msg.MinQuantityTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + msg.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					msg.MinQuantityTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is unsupported", func() {
				BeforeEach(func() {
					msg.MinQuantityTickSize = sdk.NewDecWithPrec(8, 18)
					expectedError = "unsupported tick size: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})
	})

	Describe("Validate MsgInstantExpiryFuturesMarketLaunch", func() {
		var msg exchangetypes.MsgInstantExpiryFuturesMarketLaunch
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgInstantExpiryFuturesMarketLaunch{
				Sender:                 testexchange.DefaultAddress,
				Ticker:                 "INJ / ATOM",
				QuoteDenom:             "inj",
				OracleBase:             "inj-band",
				OracleQuote:            "atom-band",
				OracleType:             oracletypes.OracleType_Band,
				Expiry:                 10000,
				MakerFeeRate:           sdk.NewDecWithPrec(1, 3),
				TakerFeeRate:           sdk.NewDecWithPrec(2, 3),
				InitialMarginRatio:     sdk.NewDecWithPrec(5, 2),
				MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2),
				MinPriceTickSize:       sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize:    sdk.NewDecWithPrec(1, 4),
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Ticker field", func() {
			BeforeEach(func() {
				msg.Ticker = ""
				expectedError = "ticker should not be empty or exceed 30 characters: " + exchangetypes.ErrInvalidTicker.Error()
			})

			It("should be invalid with invalid ticker error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without QuoteDenom field", func() {
			BeforeEach(func() {
				msg.QuoteDenom = ""
				expectedError = "quote denom should not be empty: " + exchangetypes.ErrInvalidQuoteDenom.Error()
			})

			It("should be invalid with invalid quote denom error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Oracle Base field", func() {
			BeforeEach(func() {
				msg.OracleBase = ""

				expectedError = "oracle base should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Oracle Quote field", func() {
			BeforeEach(func() {
				msg.OracleQuote = ""

				expectedError = "oracle quote should not be empty: " + exchangetypes.ErrInvalidOracle.Error()
			})

			It("should be invalid with invalid oracle error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When using invalid oracle scale factor", func() {
			BeforeEach(func() {
				msg.OracleScaleFactor = exchangetypes.MaxOracleScaleFactor + 1
				expectedError = exchangetypes.ErrExceedsMaxOracleScaleFactor.Error()
			})

			It("should be invalid with exceeds exchangetypes.MaxOracleScaleFactor error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With Oracle Quote field being the same with Oracle Base field", func() {
			BeforeEach(func() {
				msg.OracleQuote = msg.OracleBase
				expectedError = exchangetypes.ErrSameOracles.Error()
			})

			It("should be invalid with oracle base cannot be same with oracle quote error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Expiry field", func() {
			BeforeEach(func() {
				msg.Expiry = 0
				expectedError = "expiry should not be empty: " + exchangetypes.ErrInvalidExpiry.Error()
			})

			It("should be invalid with invalid expiry date error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-correct OracleType field", func() {
			Context("When OracleType is equal to 0", func() {
				BeforeEach(func() {
					msg.OracleType = 0
					expectedError = msg.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OracleType is greater than 10", func() {
				BeforeEach(func() {
					msg.OracleType = 12
					expectedError = msg.OracleType.String() + ": " + exchangetypes.ErrInvalidOracleType.Error()
				})

				It("should be invalid with invalid oracle type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With valid MakerFeeRate field", func() {
			Context("When MakerFeeRate is smaller than 0", func() {
				BeforeEach(func() {
					msg.MakerFeeRate = sdk.NewDecWithPrec(-1, 4)
				})

				It("should be valid", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("When MakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					msg.MakerFeeRate = sdk.NewDecWithPrec(1001, 3)
					expectedError = "exchange fee cannot be greater than 1: " + msg.MakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MakerFeeRate is greater than TakerFeeRate", func() {
				BeforeEach(func() {
					msg.MakerFeeRate = sdk.NewDecWithPrec(3, 3)
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
					msg.TakerFeeRate = sdk.NewDec(-1)

					expectedError = "exchange fee cannot be negative: " + msg.TakerFeeRate.String()
				})

				It("should be invalid with cannot be negative error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is greater than 1", func() {
				BeforeEach(func() {
					msg.TakerFeeRate = sdk.NewDecWithPrec(1001, 3)
					expectedError = "exchange fee cannot be greater than 1: " + msg.TakerFeeRate.String()
				})

				It("should be invalid with cannot be greater than 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When TakerFeeRate is smaller than MakerFeeRate", func() {
				BeforeEach(func() {
					msg.TakerFeeRate = sdk.NewDecWithPrec(1, 4)
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
					msg.InitialMarginRatio = sdk.NewDec(0)
					expectedError = "margin ratio cannot be less than minimum: " + msg.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					msg.InitialMarginRatio = sdk.NewDecWithPrec(1000, 3)
					expectedError = "margin ratio cannot be greater than or equal to 1: " + msg.InitialMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When InitialMarginRatio is smaller than MaintenanceMarginRatio", func() {
				BeforeEach(func() {
					msg.InitialMarginRatio = sdk.NewDecWithPrec(1, 2)
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
					msg.MaintenanceMarginRatio = sdk.NewDec(0)
					expectedError = "margin ratio cannot be less than minimum: " + msg.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be less than minimum error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is equal to 1", func() {
				BeforeEach(func() {
					msg.MaintenanceMarginRatio = sdk.NewDecWithPrec(1000, 3)
					expectedError = "margin ratio cannot be greater than or equal to 1: " + msg.MaintenanceMarginRatio.String()
				})

				It("should be invalid with cannot be greater than or equal to 1 error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MaintenanceMarginRatio is greater than InitialMarginRatio", func() {
				BeforeEach(func() {
					msg.MaintenanceMarginRatio = sdk.NewDecWithPrec(6, 2)
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
					msg.MinPriceTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + msg.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is equal to 0", func() {
				BeforeEach(func() {
					msg.MinPriceTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + msg.MinPriceTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					msg.MinPriceTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidPriceTickSize.Error()
				})

				It("should be invalid with invalid price tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinPriceTickSize is unsupported", func() {
				BeforeEach(func() {
					msg.MinPriceTickSize = sdk.NewDecWithPrec(8, 18)
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
					msg.MinQuantityTickSize = sdk.NewDec(-1)

					expectedError1 := "tick size cannot be negative: " + msg.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is equal to 0", func() {
				BeforeEach(func() {
					msg.MinQuantityTickSize = sdk.ZeroDec()

					expectedError1 := "tick size cannot be zero: " + msg.MinQuantityTickSize.String() + ": "
					expectedError = expectedError1 + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is greater than max allowed", func() {
				BeforeEach(func() {
					msg.MinQuantityTickSize = exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
					expectedError = "unsupported tick size amount: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When MinQuantityTickSize is unsupported", func() {
				BeforeEach(func() {
					msg.MinQuantityTickSize = sdk.NewDecWithPrec(8, 18)
					expectedError = "unsupported tick size: " + exchangetypes.ErrInvalidQuantityTickSize.Error()
				})

				It("should be invalid with invalid quantity tick size error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})
	})

	Describe("Validate MsgCreateDerivativeLimitOrder", func() {
		var msg exchangetypes.MsgCreateDerivativeLimitOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			dec := sdk.OneDec()
			msg = exchangetypes.MsgCreateDerivativeLimitOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.DerivativeOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.NewDec(137),
						Quantity:     sdk.NewDec(24),
					},
					Margin:       sdk.NewDec(100),
					OrderType:    exchangetypes.OrderType_BUY,
					TriggerPrice: &dec,
				},
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With too long simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1111"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "11a"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With trigger price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.TriggerPrice = &types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With margin equal to MaxOrderMargin", func() {
			BeforeEach(func() {
				msg.Order.Margin = types.MaxOrderMargin
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.Order.MarketId = ""
				expectedError = msg.Order.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With empty SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0xCA6A7F8C75B5EEACFDA20430CF5823CE4185673000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not matching SubaccountId and Sender address", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0x90f8bf6a479f320ead074411a4b0e7944ea8d9c1000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = "inj1cyztgl4rmxu9fns0xc08f9q5x7dfqka5mhula"
				expectedError = msg.Order.OrderInfo.FeeRecipient + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Price field", func() {
			Context("When Price is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Price = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
				})

				It("should be invalid with invalid price error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Price is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Price = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
				})

				It("should be invalid with invalid price error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With smaller than 0 Margin field", func() {
			BeforeEach(func() {
				msg.Order.Margin = sdk.NewDec(-1)
				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrInsufficientOrderMargin.Error()
			})

			It("should be invalid with insufficient order margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With margin greater than exchangetypes.MaxOrderMargin field", func() {
			BeforeEach(func() {
				margin := exchangetypes.MaxOrderMargin.Add(sdk.SmallestDec())
				msg.Order.Margin = margin

				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrTooMuchOrderMargin.Error()
			})

			It("should be invalid with too invalid margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Quantity field", func() {
			Context("When Quantity is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Quantity is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid OrderType field", func() {
			Context("When OrderType is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 0
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OrderType is greater than 10", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 11
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With negative TriggerPrice field", func() {
			BeforeEach(func() {
				minusDec := sdk.NewDec(-1)
				msg.Order.TriggerPrice = &minusDec

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Quantity than exchangetypes.MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller Price than exchangetypes.MinDerivativeOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With TriggerPrice equal 0", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller than TriggerPrice than exchangetypes.MinDerivativeOrderPrice field", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
				msg.Order.OrderType = exchangetypes.OrderType(types.OrderType_STOP_SELL)
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = fmt.Sprintf("Mismatch between triggerPrice: %v and orderType: %v, or triggerPrice is incorrect: ", triggerPrice, msg.Order.OrderType) + exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With TriggerPrice being nil value", func() {
			BeforeEach(func() {
				msg.Order.TriggerPrice = &sdk.Dec{}

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate MsgCreateBinaryOptionsLimitOrder", func() {
		var msg exchangetypes.MsgCreateBinaryOptionsLimitOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			dec := sdk.ZeroDec()
			msg = exchangetypes.MsgCreateBinaryOptionsLimitOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.DerivativeOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.MustNewDecFromStr("0.1"),
						Quantity:     sdk.NewDec(24),
					},
					Margin:       sdk.NewDec(2),
					OrderType:    exchangetypes.OrderType_BUY,
					TriggerPrice: &dec,
				},
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With too long simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1111"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "11a"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With price equal to zero", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = sdk.ZeroDec()
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With margin equal to MaxOrderMargin", func() {
			BeforeEach(func() {
				msg.Order.Margin = types.MaxOrderMargin
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.Order.MarketId = ""
				expectedError = msg.Order.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With empty SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0xCA6A7F8C75B5EEACFDA20430CF5823CE4185673000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not matching SubaccountId and Sender address", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0x90f8bf6a479f320ead074411a4b0e7944ea8d9c1000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = "inj1cyztgl4rmxu9fns0xc08f9q5x7dfqka5mhula"
				expectedError = msg.Order.OrderInfo.FeeRecipient + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When Price is smaller than 0", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = sdk.NewDec(-1)
				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller than 0 Margin field", func() {
			BeforeEach(func() {
				msg.Order.Margin = sdk.NewDec(-1)
				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrInsufficientOrderMargin.Error()
			})

			It("should be invalid with insufficient order margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With margin greater than exchangetypes.MaxOrderMargin field", func() {
			BeforeEach(func() {
				margin := exchangetypes.MaxOrderMargin.Add(sdk.SmallestDec())
				msg.Order.Margin = margin

				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrTooMuchOrderMargin.Error()
			})

			It("should be invalid with too invalid margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Quantity field", func() {
			Context("When Quantity is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Quantity is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid OrderType field", func() {
			Context("When OrderType is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 0
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OrderType is greater than 10", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 11
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("With conditional order types", func() {
				invalidTypes := []exchangetypes.OrderType{exchangetypes.OrderType_STOP_BUY, exchangetypes.OrderType_STOP_SELL, exchangetypes.OrderType_TAKE_BUY, exchangetypes.OrderType_TAKE_SELL}

				for _, orderType := range invalidTypes {
					BeforeEach(func() {
						msg.Order.OrderType = orderType
						expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
					})

					It("should be invalid with unsupported order type", func() {
						Expect(err.Error()).To(Equal(expectedError))
					})
				}
			})
		})

		Context("With negative TriggerPrice field", func() {
			BeforeEach(func() {
				minusDec := sdk.NewDec(-1)
				msg.Order.TriggerPrice = &minusDec

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Quantity than exchangetypes.MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With TriggerPrice being nil value", func() {
			BeforeEach(func() {
				msg.Order.TriggerPrice = &sdk.Dec{}

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate MsgCreateDerivativeMarketOrder", func() {
		var msg exchangetypes.MsgCreateDerivativeMarketOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			dec := sdk.OneDec()
			msg = exchangetypes.MsgCreateDerivativeMarketOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.DerivativeOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.NewDec(137),
						Quantity:     sdk.NewDec(24),
					},
					Margin:       sdk.NewDec(100),
					OrderType:    exchangetypes.OrderType_BUY,
					TriggerPrice: &dec,
				},
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With too long simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1111"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "11a"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With trigger price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.TriggerPrice = &types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With margin equal to MaxOrderMargin", func() {
			BeforeEach(func() {
				msg.Order.Margin = types.MaxOrderMargin
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.Order.MarketId = ""
				expectedError = msg.Order.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With empty SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0xCA6A7F8C75B5EEACFDA20430CF5823CE4185673000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not matching SubaccountId and Sender address", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0x90f8bf6a479f320ead074411a4b0e7944ea8d9c1000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = "inj1cyztgl4rmxu9fns0xc08f9q5x7dfqka5mhula"
				expectedError = msg.Order.OrderInfo.FeeRecipient + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller than 0 Margin field", func() {
			BeforeEach(func() {
				msg.Order.Margin = sdk.NewDec(-1)
				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrInsufficientOrderMargin.Error()
			})

			It("should be invalid with insufficient order margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With margin greater than exchangetypes.MaxOrderMargin field", func() {
			BeforeEach(func() {
				margin := exchangetypes.MaxOrderMargin.Add(sdk.SmallestDec())
				msg.Order.Margin = margin

				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrTooMuchOrderMargin.Error()
			})

			It("should be invalid with too invalid margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Quantity field", func() {
			Context("When Quantity is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Quantity is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid OrderType field", func() {
			Context("When OrderType is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 0
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OrderType is greater than 10", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 11
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With negative TriggerPrice field", func() {
			BeforeEach(func() {
				minusDec := sdk.NewDec(-1)
				msg.Order.TriggerPrice = &minusDec

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With nil Price field", func() {
			BeforeEach(func() {
				subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

				dec := sdk.OneDec()
				msg = exchangetypes.MsgCreateDerivativeMarketOrder{
					Sender: testexchange.DefaultAddress,
					Order: exchangetypes.DerivativeOrder{
						MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Quantity:     sdk.NewDec(24),
						},
						Margin:       sdk.NewDec(50),
						OrderType:    exchangetypes.OrderType_BUY,
						TriggerPrice: &dec,
					},
				}

				expectedError = "<nil>: " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Quantity than exchangetypes.MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller Price than exchangetypes.MinDerivativeOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater TriggerPrice than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller than TriggerPrice than exchangetypes.MinDerivativeOrderPrice field", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
				msg.Order.OrderType = exchangetypes.OrderType(types.OrderType_STOP_SELL)
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = fmt.Sprintf("Mismatch between triggerPrice: %v and orderType: %v, or triggerPrice is incorrect: ", triggerPrice, msg.Order.OrderType) + exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With TriggerPrice being nil value", func() {
			BeforeEach(func() {
				msg.Order.TriggerPrice = &sdk.Dec{}

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate MsgCreateBinaryOptionsMarketOrder", func() {
		var msg exchangetypes.MsgCreateBinaryOptionsMarketOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			dec := sdk.ZeroDec()
			msg = exchangetypes.MsgCreateBinaryOptionsMarketOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.DerivativeOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.MustNewDecFromStr("0.1"),
						Quantity:     sdk.NewDec(24),
					},
					Margin:       sdk.NewDec(2),
					OrderType:    exchangetypes.OrderType_BUY,
					TriggerPrice: &dec,
				},
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With too long simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "1111"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccount", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "11a"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With price equal to MaxOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MaxOrderPrice
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With price equal to zero", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = sdk.ZeroDec()
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With margin equal to MaxOrderMargin", func() {
			BeforeEach(func() {
				msg.Order.Margin = types.MaxOrderMargin
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.Order.MarketId = ""
				expectedError = msg.Order.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With empty SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0xCA6A7F8C75B5EEACFDA20430CF5823CE4185673000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not matching SubaccountId and Sender address", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0x90f8bf6a479f320ead074411a4b0e7944ea8d9c1000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = "inj1cyztgl4rmxu9fns0xc08f9q5x7dfqka5mhula"
				expectedError = msg.Order.OrderInfo.FeeRecipient + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When Price is smaller than 0", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = sdk.NewDec(-1)
				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With smaller than 0 Margin field", func() {
			BeforeEach(func() {
				msg.Order.Margin = sdk.NewDec(-1)
				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrInsufficientOrderMargin.Error()
			})

			It("should be invalid with insufficient order margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With margin greater than exchangetypes.MaxOrderMargin field", func() {
			BeforeEach(func() {
				margin := exchangetypes.MaxOrderMargin.Add(sdk.SmallestDec())
				msg.Order.Margin = margin

				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrTooMuchOrderMargin.Error()
			})

			It("should be invalid with too invalid margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Quantity field", func() {
			Context("When Quantity is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Quantity is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid OrderType field", func() {
			Context("When OrderType is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 0
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OrderType is greater than 10", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 11
					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("With conditional order types", func() {
				invalidTypes := []exchangetypes.OrderType{exchangetypes.OrderType_STOP_BUY, exchangetypes.OrderType_STOP_SELL, exchangetypes.OrderType_TAKE_BUY, exchangetypes.OrderType_TAKE_SELL}

				for _, orderType := range invalidTypes {
					BeforeEach(func() {
						msg.Order.OrderType = orderType
						expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
					})

					It("should be invalid with unsupported order type", func() {
						Expect(err.Error()).To(Equal(expectedError))
					})
				}
			})
		})

		Context("With negative TriggerPrice field", func() {
			BeforeEach(func() {
				minusDec := sdk.NewDec(-1)
				msg.Order.TriggerPrice = &minusDec

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Quantity than exchangetypes.MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With TriggerPrice being nil value", func() {
			BeforeEach(func() {
				msg.Order.TriggerPrice = &sdk.Dec{}

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate CancelDerivativeOrder", func() {
		var msg exchangetypes.MsgCancelDerivativeOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgCancelDerivativeOrder{
				Sender:       testexchange.DefaultAddress,
				MarketId:     "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
				SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001",
				OrderHash:    "0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989",
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty subaccount", func() {
			BeforeEach(func() {
				msg.SubaccountId = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified default subaccount", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default subaccount", func() {
			BeforeEach(func() {
				msg.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With too long simplified subaccount", func() {
			BeforeEach(func() {
				msg.SubaccountId = "1111"
				expectedError = msg.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccount", func() {
			BeforeEach(func() {
				msg.SubaccountId = "11a"
				expectedError = msg.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With bad subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With subaccount not owned by the sender", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a478f320ead074411a4b0e7944ea8c9c1000000000000000000000001"
				msg.SubaccountId = notOwnedSubaccountId

				expectedError = notOwnedSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.MarketId = ""
				expectedError = msg.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Hash field", func() {
			BeforeEach(func() {
				msg.OrderHash = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.OrderHash + ": " + exchangetypes.ErrOrderHashInvalid.Error()
			})

			It("should be invalid with invalid order hash error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate CancelBinaryOptionsOrder", func() {
		var msg exchangetypes.MsgCancelBinaryOptionsOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgCancelBinaryOptionsOrder{
				Sender:       testexchange.DefaultAddress,
				MarketId:     "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
				SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001",
				OrderHash:    "0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989",
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With bad subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With subaccount not owned by the sender", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a478f320ead074411a4b0e7944ea8c9c1000000000000000000000001"
				msg.SubaccountId = notOwnedSubaccountId

				expectedError = notOwnedSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.MarketId = ""
				expectedError = msg.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Hash field", func() {
			BeforeEach(func() {
				msg.OrderHash = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.OrderHash + ": " + exchangetypes.ErrOrderHashInvalid.Error()
			})

			It("should be invalid with invalid order hash error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate IncreasePositionMargin", func() {
		var msg exchangetypes.MsgIncreasePositionMargin
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgIncreasePositionMargin{
				Sender:                  testexchange.DefaultAddress,
				MarketId:                "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
				SourceSubaccountId:      "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001",
				DestinationSubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002",
				Amount:                  sdk.NewDec(10),
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty source subaccount", func() {
			BeforeEach(func() {
				msg.SourceSubaccountId = ""
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified default source subaccount", func() {
			BeforeEach(func() {
				msg.SourceSubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default source subaccount", func() {
			BeforeEach(func() {
				msg.SourceSubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty destination subaccount", func() {
			BeforeEach(func() {
				msg.DestinationSubaccountId = ""
			})

			It("should be invalid", func() {
				Expect(err).To(Not(BeNil()))
			})
		})

		Context("With simplified default destination subaccount", func() {
			BeforeEach(func() {
				msg.DestinationSubaccountId = "0"
				expectedError = msg.DestinationSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With too long simplified destination subaccount", func() {
			BeforeEach(func() {
				msg.DestinationSubaccountId = "1111"
				expectedError = msg.DestinationSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified destination subaccount", func() {
			BeforeEach(func() {
				msg.DestinationSubaccountId = "11a"
				expectedError = msg.DestinationSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be valid", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With amount equal to max margin", func() {
			BeforeEach(func() {
				msg.Amount = exchangetypes.MaxOrderMargin
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With destination subaccountId not owned by the sender", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a478f320ead074411a4b0e7944ea8c9c1000000000000000000000001"
				msg.DestinationSubaccountId = notOwnedSubaccountId
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With source subaccountId not owned by the sender", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a478f320ead074411a4b0e7944ea8c9c1000000000000000000000001"
				msg.SourceSubaccountId = notOwnedSubaccountId
				expectedError = msg.SourceSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With bad source subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SourceSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With bad destination subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.DestinationSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.MarketId = ""
				expectedError = msg.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Amount field", func() {
			Context("When Amount is smaller than 0", func() {
				BeforeEach(func() {
					msg.Amount = sdk.NewDec(-1)
					expectedError = msg.Amount.String() + ": " + sdkerrors.ErrInvalidCoins.Error()
				})

				It("should be invalid with invalid coins error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Amount is equal to 0", func() {
				BeforeEach(func() {
					msg.Amount = sdk.ZeroDec()
					expectedError = msg.Amount.String() + ": " + sdkerrors.ErrInvalidCoins.Error()
				})

				It("should be invalid with invalid coins error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Amount is bigger than MaxOrderMargin", func() {
				BeforeEach(func() {
					msg.Amount = types.MaxOrderMargin.Add(sdk.SmallestDec())
					expectedError = msg.Amount.String() + ": " + exchangetypes.ErrTooMuchOrderMargin.Error()
				})

				It("should be invalid with invalid too much margin error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})
	})

	Describe("Validate LiquidatePosition", func() {
		var msg exchangetypes.MsgLiquidatePosition
		var err error
		var expectedError string

		BeforeEach(func() {
			dec := sdk.OneDec()

			msg = exchangetypes.MsgLiquidatePosition{
				Sender:       testexchange.DefaultAddress,
				MarketId:     "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
				SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001",
				Order: &exchangetypes.DerivativeOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002",
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.NewDec(137),
						Quantity:     sdk.NewDec(24),
					},
					Margin:       sdk.NewDec(100),
					OrderType:    exchangetypes.OrderType_BUY,
					TriggerPrice: &dec,
				},
			}
		})

		JustBeforeEach(func() {
			err = msg.ValidateBasic()
		})

		Context("With all valid fields", func() {
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With nil Order field", func() {
			BeforeEach(func() {
				msg.Order = nil
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With bad subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId field", func() {
			BeforeEach(func() {
				msg.MarketId = ""
				expectedError = msg.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without Sender field", func() {
			BeforeEach(func() {
				msg.Sender = ""
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With wrong Sender field", func() {
			BeforeEach(func() {
				msg.Sender = "0x90f8bf6a479f320ead074411a4b0e79ea8c9c1"
				expectedError = msg.Sender + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without MarketId inside order field", func() {
			BeforeEach(func() {
				msg.Order.MarketId = ""
				expectedError = msg.Order.MarketId + ": " + exchangetypes.ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid market error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With empty SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong SubaccountId field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0xCA6A7F8C75B5EEACFDA20430CF5823CE4185673000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not matching SubaccountId and Sender address", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.SubaccountId = "0x90f8bf6a479f320ead074411a4b0e7944ea8d9c1000000000000000000000001"
				expectedError = msg.Order.OrderInfo.SubaccountId + ": " + exchangetypes.ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountId error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Without FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = ""
			})

			It("should be valid with no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With wrong FeeRecipient field", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.FeeRecipient = "inj1cyztgl4rmxu9fns0xc08f9q5x7dfqka5mhula"
				expectedError = msg.Order.OrderInfo.FeeRecipient + ": " + sdkerrors.ErrInvalidAddress.Error()
			})

			It("should be invalid with invalid address error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Price field", func() {
			Context("When Price is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Price = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
				})

				It("should be invalid with invalid price error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Price is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Price = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
				})

				It("should be invalid with invalid price error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With smaller than 0 Margin field", func() {
			BeforeEach(func() {
				msg.Order.Margin = sdk.NewDec(-1)
				expectedError = msg.Order.Margin.String() + ": " + exchangetypes.ErrInsufficientOrderMargin.Error()
			})

			It("should be invalid with insufficient order margin error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-positive Quantity field", func() {
			Context("When Quantity is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.NewDec(-1)
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Quantity is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid quantity error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With invalid OrderType field", func() {
			Context("When OrderType is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 0

					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When OrderType is greater than 10", func() {
				BeforeEach(func() {
					msg.Order.OrderType = 11

					expectedError = string(msg.Order.OrderType) + ": " + exchangetypes.ErrUnrecognizedOrderType.Error()
				})

				It("should be invalid with unrecognized order type error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Context("With negative TriggerPrice field", func() {
			BeforeEach(func() {
				minusDec := sdk.NewDec(-1)
				msg.Order.TriggerPrice = &minusDec

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Quantity than exchangetypes.MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater TriggerPrice than exchangetypes.MaxOrderPrice field", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
			})

			It("should be invalid with invalid trigger price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})
})
