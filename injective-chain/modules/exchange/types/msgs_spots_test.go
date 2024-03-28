package types_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Msgs Spots", func() {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	Describe("Validate MsgInstantSpotMarketLaunch", func() {
		var msg exchangetypes.MsgInstantSpotMarketLaunch
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgInstantSpotMarketLaunch{
				Sender:              testexchange.DefaultAddress,
				Ticker:              "INJ / ATOM",
				BaseDenom:           "inj",
				QuoteDenom:          "atom",
				MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
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

		Context("Without BaseDenom field", func() {
			BeforeEach(func() {
				msg.BaseDenom = ""
				expectedError = "base denom should not be empty: " + exchangetypes.ErrInvalidBaseDenom.Error()
			})

			It("should be invalid with invalid base denom error", func() {
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

		Context("With QuoteDenom field being the same with BaseDenom field", func() {
			BeforeEach(func() {
				msg.QuoteDenom = msg.BaseDenom
				expectedError = "base denom cannot be same with quote denom"
			})

			It("should be invalid with base denom cannot be same with quote denom error", func() {
				Expect(err.Error()).To(Equal(expectedError))
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

				It("should be invalid with invalid price tick size error", func() {
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

	Describe("Validate MsgCreateSpotLimitOrder", func() {
		var msg exchangetypes.MsgCreateSpotLimitOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			dec := sdk.OneDec()
			msg = exchangetypes.MsgCreateSpotLimitOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.SpotOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.NewDec(137),
						Quantity:     sdk.NewDec(24),
					},
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

		Context("With all valid fields and price below MinDerivativeOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With all valid fields and trigger price below MinDerivativeOrderPrice", func() {
			BeforeEach(func() {
				belowMinDerivativePrice := types.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
				msg.Order.TriggerPrice = &belowMinDerivativePrice
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

		Context("With non-positive Quantity field", func() {
			Context("When Quantity is smaller than 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.NewDec(-1)

					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid coins error", func() {
					Expect(err.Error()).To(Equal(expectedError))
				})
			})

			Context("When Quantity is equal to 0", func() {
				BeforeEach(func() {
					msg.Order.OrderInfo.Quantity = sdk.ZeroDec()
					expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
				})

				It("should be invalid with invalid coins error", func() {
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

		Context("With greater Quantity than MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater TriggerPrice than MaxOrderPrice field", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
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

	Describe("Validate MsgCreateSpotMarketOrder", func() {
		var msg exchangetypes.MsgCreateSpotMarketOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			dec := sdk.OneDec()
			msg = exchangetypes.MsgCreateSpotMarketOrder{
				Sender: testexchange.DefaultAddress,
				Order: exchangetypes.SpotOrder{
					MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					OrderInfo: exchangetypes.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1cyztgl4rmxu9fns0xc508f9q5x7dfqka5mhula",
						Price:        sdk.OneDec(),
						Quantity:     sdk.NewDec(24),
					},
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

		Context("With all valid fields and price below MinDerivativeOrderPrice", func() {
			BeforeEach(func() {
				msg.Order.OrderInfo.Price = types.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With all valid fields and trigger price below MinDerivativeOrderPrice", func() {
			BeforeEach(func() {
				belowMinDerivativePrice := types.MinDerivativeOrderPrice.Sub(sdk.SmallestDec())
				msg.Order.TriggerPrice = &belowMinDerivativePrice
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
				msg = exchangetypes.MsgCreateSpotMarketOrder{
					Sender: testexchange.DefaultAddress,
					Order: exchangetypes.SpotOrder{
						MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: exchangetypes.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Quantity:     sdk.NewDec(24),
						},
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

		Context("With greater Quantity than MaxOrderQuantity field", func() {
			BeforeEach(func() {
				quantity := exchangetypes.MaxOrderQuantity.Add(sdk.OneDec())
				msg.Order.OrderInfo.Quantity = quantity

				expectedError = msg.Order.OrderInfo.Quantity.String() + ": " + exchangetypes.ErrInvalidQuantity.Error()
			})

			It("should be invalid with invalid quantity error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater Price than MaxOrderPrice field", func() {
			BeforeEach(func() {
				price := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.OrderInfo.Price = price

				expectedError = msg.Order.OrderInfo.Price.String() + ": " + exchangetypes.ErrInvalidPrice.Error()
			})

			It("should be invalid with invalid price error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With greater TriggerPrice than MaxOrderPrice field", func() {
			BeforeEach(func() {
				triggerPrice := exchangetypes.MaxOrderPrice.Add(sdk.OneDec())
				msg.Order.TriggerPrice = &triggerPrice

				expectedError = exchangetypes.ErrInvalidTriggerPrice.Error()
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

	Describe("Validate CancelSpotOrder", func() {
		var msg exchangetypes.MsgCancelSpotOrder
		var err error
		var expectedError string

		BeforeEach(func() {
			msg = exchangetypes.MsgCancelSpotOrder{
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
})
