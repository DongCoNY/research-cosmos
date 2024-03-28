package types_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	. "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

type MockMessageWithMultipleSigners struct {
	signers []sdk.AccAddress
}

func (m MockMessageWithMultipleSigners) ValidateBasic() error {
	return nil
}

func (m MockMessageWithMultipleSigners) GetSigners() []sdk.AccAddress {
	return m.signers
}

func (m MockMessageWithMultipleSigners) Reset() {}

func (m MockMessageWithMultipleSigners) String() string {
	return "mockMessage"
}

func (m MockMessageWithMultipleSigners) ProtoMessage() {}

// Route implements the sdk.Msg interface. It should return the name of the module
func (m MockMessageWithMultipleSigners) Route() string { return "exchange" }

// Type implements the sdk.Msg interface. It should return the action.
func (m MockMessageWithMultipleSigners) Type() string { return "MockMessage" }

// GetSignBytes implements the sdk.Msg interface. It encodes the message for signing
func (m *MockMessageWithMultipleSigners) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

var _ = Describe("Msgs Accounting", func() {
	Describe("Validate BatchUpdateOrders", func() {
		var msg MsgBatchUpdateOrders
		var err error
		var expectedError string

		BeforeEach(func() {
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			msg = MsgBatchUpdateOrders{
				Sender:       testexchange.DefaultAddress,
				SubaccountId: subaccountID,
				SpotMarketIdsToCancelAll: []string{
					"0x10057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					"0xc0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
				},
				DerivativeMarketIdsToCancelAll: []string{
					"0xd0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
					"0xe0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
				},
				SpotOrdersToCancel: []*OrderData{
					{
						MarketId:     "0x00057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002",
						OrderHash:    "0x5cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989",
					}, {
						MarketId:     "0x20057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000",
						OrderHash:    "0x4cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989",
					},
				},
				DerivativeOrdersToCancel: []*OrderData{
					{
						MarketId:     "0xa0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002",
						OrderHash:    "0x6cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989",
					}, {
						MarketId:     "0xf0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000",
						OrderHash:    "0x2cf90f9026695a5650035f8a6c92c5294787b18032f08ce45460ee9b6bc63989",
					},
				},
				SpotOrdersToCreate: []*SpotOrder{
					{
						MarketId: "0xa0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: OrderInfo{
							SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000",
							FeeRecipient: testexchange.DefaultAddress,
							Price:        sdk.NewDec(1000),
							Quantity:     sdk.NewDec(1000),
						},
						OrderType:    OrderType_BUY,
						TriggerPrice: nil,
					},
				},
				DerivativeOrdersToCreate: []*DerivativeOrder{
					{
						MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: OrderInfo{
							SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000",
							FeeRecipient: testexchange.DefaultAddress,
							Price:        sdk.NewDec(1000),
							Quantity:     sdk.NewDec(1000),
						},
						OrderType:    OrderType_BUY,
						Margin:       sdk.NewDec(100000),
						TriggerPrice: nil,
					},
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

		Context("With empty subaccount of one order, but the other with full subaccount id", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCreate =
					[]*SpotOrder{{
						MarketId: "0xa0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: OrderInfo{
							SubaccountId: "",
							FeeRecipient: testexchange.DefaultAddress,
							Price:        sdk.NewDec(1000),
							Quantity:     sdk.NewDec(1000),
						},
						OrderType:    OrderType_BUY,
						TriggerPrice: nil,
					},
					}
				msg.DerivativeOrdersToCreate =
					[]*DerivativeOrder{
						{
							MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
							OrderInfo: OrderInfo{
								SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000",
								FeeRecipient: testexchange.DefaultAddress,
								Price:        sdk.NewDec(1000),
								Quantity:     sdk.NewDec(1000),
							},
							OrderType:    OrderType_BUY,
							Margin:       sdk.NewDec(100000),
							TriggerPrice: nil,
						},
					}
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplifed subaccount ids for orders", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCreate =
					[]*SpotOrder{{
						MarketId: "0xa0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: OrderInfo{
							SubaccountId: "0",
							FeeRecipient: testexchange.DefaultAddress,
							Price:        sdk.NewDec(1000),
							Quantity:     sdk.NewDec(1000),
						},
						OrderType:    OrderType_BUY,
						TriggerPrice: nil,
					},
					}
				msg.DerivativeOrdersToCreate =
					[]*DerivativeOrder{
						{
							MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
							OrderInfo: OrderInfo{
								SubaccountId: "1",
								FeeRecipient: testexchange.DefaultAddress,
								Price:        sdk.NewDec(1000),
								Quantity:     sdk.NewDec(1000),
							},
							OrderType:    OrderType_BUY,
							Margin:       sdk.NewDec(100000),
							TriggerPrice: nil,
						},
					}
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified subaccount of one order, but the other with full subaccount id", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCreate =
					[]*SpotOrder{{
						MarketId: "0xa0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: OrderInfo{
							SubaccountId: "0",
							FeeRecipient: testexchange.DefaultAddress,
							Price:        sdk.NewDec(1000),
							Quantity:     sdk.NewDec(1000),
						},
						OrderType:    OrderType_BUY,
						TriggerPrice: nil,
					},
					}
				msg.DerivativeOrdersToCreate =
					[]*DerivativeOrder{
						{
							MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
							OrderInfo: OrderInfo{
								SubaccountId: "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000",
								FeeRecipient: testexchange.DefaultAddress,
								Price:        sdk.NewDec(1000),
								Quantity:     sdk.NewDec(1000),
							},
							OrderType:    OrderType_BUY,
							Margin:       sdk.NewDec(100000),
							TriggerPrice: nil,
						},
					}
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With bad subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SubaccountId = badSubaccountId
				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With negative simplified subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "-1"
				msg.SubaccountId = badSubaccountId
				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With too long simplified subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "1111"
				msg.SubaccountId = badSubaccountId
				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "a2"
				msg.SubaccountId = badSubaccountId
				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With simplified main subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With simplified non-default main subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With empty fee recipient and simplified subaccount ids for orders", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCreate =
					[]*SpotOrder{{
						MarketId: "0xa0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
						OrderInfo: OrderInfo{
							SubaccountId: "0",
							FeeRecipient: "",
							Price:        sdk.NewDec(1000),
							Quantity:     sdk.NewDec(1000),
						},
						OrderType:    OrderType_BUY,
						TriggerPrice: nil,
					},
					}
				msg.DerivativeOrdersToCreate =
					[]*DerivativeOrder{
						{
							MarketId: "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
							OrderInfo: OrderInfo{
								SubaccountId: "1",
								FeeRecipient: "",
								Price:        sdk.NewDec(1000),
								Quantity:     sdk.NewDec(1000),
							},
							OrderType:    OrderType_BUY,
							Margin:       sdk.NewDec(100000),
							TriggerPrice: nil,
						},
					}
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With missing subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = ""
				expectedError = "msg contains cancel all marketIDs but no subaccountID" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with missing subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With subaccountID but missing cancelAll marketIDs", func() {
			BeforeEach(func() {
				msg.SpotMarketIdsToCancelAll = []string{}
				msg.DerivativeMarketIdsToCancelAll = []string{}

				expectedError = "msg contains subaccountID but no cancel all marketIDs" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with missing subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With duplicate cancelAll spot marketID in spot orders to cancel", func() {
			BeforeEach(func() {
				msg.SpotMarketIdsToCancelAll[0] = msg.SpotOrdersToCancel[0].MarketId

				expectedError = "msg contains order to cancel in a spot market that is also in cancel all" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with duplicate marketID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With duplicate cancelAll derivative marketID in derivative orders to cancel", func() {
			BeforeEach(func() {
				msg.DerivativeMarketIdsToCancelAll[0] = msg.DerivativeOrdersToCancel[0].MarketId

				expectedError = "msg contains order to cancel in a derivative market that is also in cancel all" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with duplicate marketID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With duplicate cancelAll spot marketIDs", func() {
			BeforeEach(func() {
				msg.SpotMarketIdsToCancelAll[0] = msg.SpotMarketIdsToCancelAll[1]

				expectedError = "msg contains duplicate cancel all spot market ids" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with duplicate marketID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With duplicate cancelAll derivative marketIDs", func() {
			BeforeEach(func() {
				msg.DerivativeMarketIdsToCancelAll[0] = msg.DerivativeMarketIdsToCancelAll[1]

				expectedError = "msg contains duplicate cancel all derivative market ids" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with duplicate marketID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With duplicate spot order to cancel", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCancel[0] = msg.SpotOrdersToCancel[1]

				expectedError = "msg contains duplicate spot order to cancel" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with duplicate order error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With duplicate derivative order to cancel", func() {
			BeforeEach(func() {
				msg.DerivativeOrdersToCancel[0] = msg.DerivativeOrdersToCancel[1]

				expectedError = "msg contains duplicate derivative order to cancel" + ": " + ErrInvalidBatchMsgUpdate.Error()
			})

			It("should be invalid with duplicate order error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid spot order to cancel", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCancel[0].MarketId = ""

				expectedError = msg.SpotOrdersToCancel[0].MarketId + ": " + ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid spot order error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid derivative order to cancel", func() {
			BeforeEach(func() {
				msg.DerivativeOrdersToCancel[0].MarketId = ""
				expectedError = msg.DerivativeOrdersToCancel[0].MarketId + ": " + ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid derivative order error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid spot order to create", func() {
			BeforeEach(func() {
				msg.SpotOrdersToCreate[0].MarketId = ""

				expectedError = msg.SpotOrdersToCreate[0].MarketId + ": " + ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid spot order error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With invalid derivative order to create", func() {
			BeforeEach(func() {
				msg.DerivativeOrdersToCreate[0].MarketId = "hey"
				expectedError = msg.DerivativeOrdersToCreate[0].MarketId + ": " + ErrMarketInvalid.Error()
			})

			It("should be invalid with invalid derivative order error", func() {
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
	})

	Describe("Validate Deposit", func() {
		var msg MsgDeposit
		var err error
		var expectedError string
		baseDenom := "INJ"

		BeforeEach(func() {
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			msg = MsgDeposit{
				Sender:       testexchange.DefaultAddress,
				SubaccountId: subaccountID,
				Amount:       amountToDeposit,
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

		Context("With non-default simplified subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "1"
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With default simplified subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0"
			})
			It("should be invalid", func() {
				Expect(err.Error()).To(Equal("0: " + ErrBadSubaccountID.Error()))
			})
		})

		Context("With default external subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0x45f1410c0716a5658525498721c981d251ed1a21000000000000000000000000"
			})
			It("should be invalid", func() {
				Expect(err.Error()).To(Equal(msg.SubaccountId + ": " + ErrBadSubaccountID.Error()))
			})
		})

		Context("With non-default external subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0x45f1410c0716a5658525498721c981d251ed1a21000000000000000000000001"
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With non-numeric simplified subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "7sa"
			})
			It("should be invalid", func() {
				Expect(err.Error()).To(Equal(msg.SubaccountId + ": " + ErrBadSubaccountID.Error()))
			})
		})

		Context("With too long simplified subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0982"
			})
			It("should be invalid", func() {
				Expect(err.Error()).To(Equal(msg.SubaccountId + ": " + ErrBadSubaccountID.Error()))
			})
		})

		Context("With empty subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = ""
			})

			It("should be invalid", func() {
				Expect(err.Error()).To(Equal(": " + ErrBadSubaccountID.Error()))
			})
		})

		Context("With bad subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c10000000000000000000000000"

				expectedError = msg.SubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
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

		Context("With zero amount", func() {
			BeforeEach(func() {
				amountToDeposit := sdk.NewCoin(baseDenom, sdk.ZeroInt())
				msg.Amount = amountToDeposit

				expectedError = sdk.ZeroInt().String() + baseDenom + ": " + sdkerrors.ErrInvalidCoins.Error()
			})

			It("should be invalid with invalid coins error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		// For negative deposit amounts or length of two characters or less for coin test is panicking.
	})

	Describe("Validate Withdraw", func() {
		var msg MsgWithdraw
		var err error
		var expectedError string
		baseDenom := "INJ"

		BeforeEach(func() {
			amountToWithdraw := sdk.NewCoin(baseDenom, sdk.NewInt(50))
			subaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"

			msg = MsgWithdraw{
				Sender:       testexchange.DefaultAddress,
				SubaccountId: subaccountID,
				Amount:       amountToWithdraw,
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

		Context("With simplified non-default subaccountID", func() {
			BeforeEach(func() {
				msg.SubaccountId = "1"
			})

			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With sender being different from owner of subaccount", func() {
			BeforeEach(func() {
				wrongSender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7934ea8c9c1"))
				msg.Sender = wrongSender.String()

				expectedError = msg.SubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With bad subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("For default subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c10000000000000000000000000"
				msg.SubaccountId = badSubaccountId

				expectedError = msg.SubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With simplified default subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0"
				msg.SubaccountId = badSubaccountId

				expectedError = msg.SubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With too long simplified subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "1111"
				msg.SubaccountId = badSubaccountId

				expectedError = msg.SubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified subaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "a2"
				msg.SubaccountId = badSubaccountId

				expectedError = msg.SubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
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

		Context("With zero amount", func() {
			BeforeEach(func() {
				amountToWithdraw := sdk.NewCoin(baseDenom, sdk.ZeroInt())
				msg.Amount = amountToWithdraw

				expectedError = sdk.ZeroInt().String() + baseDenom + ": " + sdkerrors.ErrInvalidCoins.Error()
			})

			It("should be invalid with invalid coins error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate SubaccountTransfer", func() {
		var msg MsgSubaccountTransfer
		var err error
		var expectedError string
		baseDenom := "INJ"

		BeforeEach(func() {
			amountToTransfer := sdk.NewCoin(baseDenom, sdk.NewInt(50))
			sourceSubaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
			destinationSubaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"

			msg = MsgSubaccountTransfer{
				Sender:                  testexchange.DefaultAddress,
				SourceSubaccountId:      sourceSubaccountID,
				DestinationSubaccountId: destinationSubaccountID,
				Amount:                  amountToTransfer,
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

		Context("With simplfied non-default subaccount ids", func() {
			BeforeEach(func() {
				msg.SourceSubaccountId = "1"
				msg.DestinationSubaccountId = "2"
			})
			It("should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("With bad sourceSubaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SourceSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default simplified sourceSubaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0"
				msg.SourceSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified SourceSubaccountId", func() {
			BeforeEach(func() {
				badSubaccountId := "j81"
				msg.SourceSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With too long simplified SourceSubaccountId", func() {
			BeforeEach(func() {
				badSubaccountId := "2212"
				msg.SourceSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default simplified DestinationSubaccountId", func() {
			BeforeEach(func() {
				badSubaccountId := "0"
				msg.DestinationSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With non-numeric simplified DestinationSubaccountId", func() {
			BeforeEach(func() {
				badSubaccountId := "j81"
				msg.DestinationSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With too long simplified DestinationSubaccountId", func() {
			BeforeEach(func() {
				badSubaccountId := "2212"
				msg.DestinationSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not owned by sender sourceSubaccountID", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a479f320ead074411a4b0e7944ea7c9c1000000000000000000000001"
				msg.SourceSubaccountId = notOwnedSubaccountId
				expectedError = msg.SourceSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With bad destinationSubaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.DestinationSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not owned by sender destinationSubaccountID", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a479f320ead074411a4b0e7944ea7c9c1000000000000000000000002"
				msg.DestinationSubaccountId = notOwnedSubaccountId
				expectedError = msg.DestinationSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default subaccount as sourceSubaccountID", func() {
			BeforeEach(func() {
				msg.SourceSubaccountId = "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000"

				expectedError = msg.SourceSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default subaccount as destinationSubaccountID", func() {
			BeforeEach(func() {
				msg.DestinationSubaccountId = "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000000"

				expectedError = msg.DestinationSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With subaccounts of the same address that is not the sender", func() {
			BeforeEach(func() {
				notOwnedSourceSubaccountId := "0x90f8bf6a479f320ead074411a4b0e7944ea7c9c1000000000000000000000001"
				notOwnedDestinationSubaccountId := "0x90f8bf6a479f320ead074411a4b0e7944ea7c9c1000000000000000000000002"
				msg.SourceSubaccountId = notOwnedSourceSubaccountId
				msg.DestinationSubaccountId = notOwnedDestinationSubaccountId

				expectedError = notOwnedSourceSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
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

		Context("With zero amount", func() {
			BeforeEach(func() {
				amountToTransfer := sdk.NewCoin(baseDenom, sdk.ZeroInt())
				msg.Amount = amountToTransfer

				expectedError = sdk.ZeroInt().String() + baseDenom + ": " + sdkerrors.ErrInvalidCoins.Error()
			})

			It("should be invalid with invalid coins error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})

	Describe("Validate ExternalTransfer", func() {
		var msg MsgExternalTransfer
		var err error
		var expectedError string
		baseDenom := "INJ"

		BeforeEach(func() {
			amountToTransfer := sdk.NewCoin(baseDenom, sdk.NewInt(50))
			sourceSubaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
			destinationSubaccountID := "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"

			msg = MsgExternalTransfer{
				Sender:                  testexchange.DefaultAddress,
				SourceSubaccountId:      sourceSubaccountID,
				DestinationSubaccountId: destinationSubaccountID,
				Amount:                  amountToTransfer,
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

		Context("With bad sourceSubaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.SourceSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With not owned by sender sourceSubaccountID", func() {
			BeforeEach(func() {
				notOwnedSubaccountId := "0x90f8bf6a479f320ead074411a4b0e7944ea7c9c1000000000000000000000001"
				msg.SourceSubaccountId = notOwnedSubaccountId
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Not(BeNil()))
			})
		})

		Context("With bad destinationSubaccountID", func() {
			BeforeEach(func() {
				badSubaccountId := "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c1000000000000000000000001" // one less character
				msg.DestinationSubaccountId = badSubaccountId

				expectedError = badSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default subaccount in sourceSubaccountID", func() {
			BeforeEach(func() {
				msg.SourceSubaccountId = "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c10000000000000000000000000"

				expectedError = msg.SourceSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("With default subaccount in destinationSubaccountID", func() {
			BeforeEach(func() {
				msg.DestinationSubaccountId = "0x90f8bf6a47f320ead074411a4b0e7944ea8c9c10000000000000000000000000"

				expectedError = msg.DestinationSubaccountId + ": " + ErrBadSubaccountID.Error()
			})

			It("should be invalid with bad subaccountID error", func() {
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

		Context("With zero amount", func() {
			BeforeEach(func() {
				amountToTransfer := sdk.NewCoin(baseDenom, sdk.ZeroInt())
				msg.Amount = amountToTransfer

				expectedError = sdk.ZeroInt().String() + baseDenom + ": " + sdkerrors.ErrInvalidCoins.Error()
			})

			It("should be invalid with invalid coins error", func() {
				Expect(err.Error()).To(Equal(expectedError))
			})
		})
	})
})
