package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestDerivativeAuthz", func() {
	validSubaccountID := "ab8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
	validSubaccountID2 := "ab8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"
	invalidSubaccountID := "kb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
	Describe("Test Create Derivative Limit Order Authz", func() {
		Describe("Test CreateDerivativeLimitOrderAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						"0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce",
						"0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			It("Should return error when market id are not valid hash", func() {
				msg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x0ke920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid market id to authorize: internal logic error"))
			})
			It("should pass validate basic", func() {
				msg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err).To(BeNil())
			})
		})

		Describe("Test CreateDerivativeLimitOrderAuth Accept", func() {
			It("should not accept message type MsgCreateDerivativeMarketOrder", func() {
				authzMsg := CreateDerivativeLimitOrderAuthz{}
				msg := &MsgCreateDerivativeMarketOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order != subaccountID", func() {
				authzMsg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgCreateDerivativeLimitOrder{
					Order: DerivativeOrder{
						OrderInfo: OrderInfo{
							SubaccountId: validSubaccountID2,
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			})
			It("should not accept when marketId is not in accepted list", func() {
				authzMsg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateDerivativeLimitOrder{
					Order: DerivativeOrder{
						OrderInfo: OrderInfo{
							SubaccountId: validSubaccountID,
						},
						MarketId: "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested market is unauthorized: unauthorized"))
			})
			It("should accept message MsgCreateDerivativeLimitOrder", func() {
				authzMsg := CreateDerivativeLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateDerivativeLimitOrder{
					Order: DerivativeOrder{
						OrderInfo: OrderInfo{
							SubaccountId: validSubaccountID,
						},
						MarketId: "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(authz.AcceptResponse{Accept: true, Delete: false, Updated: nil}))
			})
		})
	})
	Describe("Test Create Derivative Market Order Authz", func() {
		Describe("Test CreateDerivativeMarketOrderAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						"0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce",
						"0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			It("Should return error when market id are not valid hash", func() {
				msg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x0ke920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid market id to authorize: internal logic error"))
			})
			It("should pass validate basic", func() {
				msg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err).To(BeNil())
			})
		})

		Describe("Test CreateDerivativeLimitOrderAuth Accept", func() {
			It("should not accept message type MsgCreateDerivativeMarketOrder", func() {
				authzMsg := CreateDerivativeMarketOrderAuthz{}
				msg := &MsgCreateDerivativeLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order != subaccountID", func() {
				authzMsg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgCreateDerivativeMarketOrder{
					Order: DerivativeOrder{
						OrderInfo: OrderInfo{
							SubaccountId: validSubaccountID2,
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			})
			It("should not accept when marketId is not in accepted list", func() {
				authzMsg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateDerivativeMarketOrder{
					Order: DerivativeOrder{
						OrderInfo: OrderInfo{
							SubaccountId: validSubaccountID,
						},
						MarketId: "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested market is unauthorized: unauthorized"))
			})
			It("should accept message MsgCreateDerivativeMarketOrder", func() {
				authzMsg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateDerivativeMarketOrder{
					Order: DerivativeOrder{
						OrderInfo: OrderInfo{
							SubaccountId: validSubaccountID,
						},
						MarketId: "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(authz.AcceptResponse{Accept: true, Delete: false, Updated: nil}))
			})
		})
	})

	Describe("Test Batch Create Derivative Limit Order Authz", func() {
		Describe("Test BatchCreateDerivativeLimitOrdersAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						"0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce",
						"0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			It("Should return error when market id are not valid hash", func() {
				msg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x0ke920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid market id to authorize: internal logic error"))
			})
			It("should pass validate basic", func() {
				msg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err).To(BeNil())
			})
		})

		Describe("Test BatchCreateDerivativeLimitOrdersAuthz Accept", func() {
			It("should not accept message type MsgCreateDerivativeMarketOrder", func() {
				authzMsg := BatchCreateDerivativeLimitOrdersAuthz{}
				msg := &MsgCreateDerivativeLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order having subaccount != granted subaccountID", func() {
				authzMsg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgBatchCreateDerivativeLimitOrders{
					Orders: []DerivativeOrder{
						{
							OrderInfo: OrderInfo{
								SubaccountId: validSubaccountID2,
							},
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			})
			It("should not accept message when marketId is not in accepted list", func() {
				authzMsg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCreateDerivativeLimitOrders{
					Orders: []DerivativeOrder{
						{
							OrderInfo: OrderInfo{
								SubaccountId: validSubaccountID,
							},
							MarketId: "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested market is unauthorized: unauthorized"))
			})
			It("should accept message MsgBatchCreateDerivativeLimitOrders", func() {
				authzMsg := BatchCreateDerivativeLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCreateDerivativeLimitOrders{
					Orders: []DerivativeOrder{
						{
							OrderInfo: OrderInfo{
								SubaccountId: validSubaccountID,
							},
							MarketId: "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						},
						{
							OrderInfo: OrderInfo{
								SubaccountId: validSubaccountID,
							},
							MarketId: "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(authz.AcceptResponse{Accept: true, Delete: false, Updated: nil}))
			})
		})
	})
	Describe("Test Cancel Derivative Market Order Authz", func() {
		Describe("Test CancelDerivativeOrderAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := CancelDerivativeOrderAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						"0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce",
						"0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			It("Should return error when market id are not valid hash", func() {
				msg := CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x0ke920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid market id to authorize: internal logic error"))
			})
			It("should pass validate basic", func() {
				msg := CreateDerivativeMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err).To(BeNil())
			})
		})

		Describe("Test CreateDerivativeLimitOrderAuth Accept", func() {
			It("should not accept message type MsgCreateDerivativeMarketOrder", func() {
				authzMsg := CancelDerivativeOrderAuthz{}
				msg := &MsgCreateDerivativeLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept canceling order subaccountID != granted subaccountID", func() {
				authzMsg := CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgCancelDerivativeOrder{
					SubaccountId: validSubaccountID2,
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			})
			It("should not accept when marketId is not in accepted list", func() {
				authzMsg := CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCancelDerivativeOrder{
					SubaccountId: validSubaccountID,
					MarketId:     "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested market is unauthorized: unauthorized"))
			})
			It("should accept message MsgCancelDerivativeOrder", func() {
				authzMsg := CancelDerivativeOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCancelDerivativeOrder{
					SubaccountId: validSubaccountID,
					MarketId:     "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(authz.AcceptResponse{Accept: true, Delete: false, Updated: nil}))
			})
		})
	})

	Describe("Test Batch Cancel Derivative Orders Authz", func() {
		Describe("Test BatchCancelDerivativeOrdersAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						"0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce",
						"0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			It("Should return error when market ids are not valid hash", func() {
				msg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x0ke920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid market id to authorize: internal logic error"))
			})
			It("should pass validate basic", func() {
				msg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				err := msg.ValidateBasic()
				Expect(err).To(BeNil())
			})
		})

		Describe("Test BatchCreateDerivativeLimitOrdersAuthz Accept", func() {
			It("should not accept message type MsgCreateDerivativeMarketOrder", func() {
				authzMsg := BatchCancelDerivativeOrdersAuthz{}
				msg := &MsgCreateDerivativeLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order having subaccount != granted subaccountID", func() {
				authzMsg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgBatchCancelDerivativeOrders{
					Data: []OrderData{
						{
							SubaccountId: validSubaccountID2,
						},
						{
							SubaccountId: validSubaccountID,
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			})
			It("should not accept message when marketId is not in accepted list", func() {
				authzMsg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCancelDerivativeOrders{
					Data: []OrderData{
						{
							MarketId:     "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
							SubaccountId: validSubaccountID,
						},
						{
							MarketId:     "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
							SubaccountId: validSubaccountID,
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested market is unauthorized: unauthorized"))
			})
			It("should accept message BatchCancelDerivativeOrdersAuthz", func() {
				authzMsg := BatchCancelDerivativeOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCancelDerivativeOrders{
					Data: []OrderData{
						{
							SubaccountId: validSubaccountID,
							MarketId:     "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
						},
						{
							SubaccountId: validSubaccountID,
							MarketId:     "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						},
					},
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(authz.AcceptResponse{Accept: true, Delete: false, Updated: nil}))
			})
		})

	})
})
