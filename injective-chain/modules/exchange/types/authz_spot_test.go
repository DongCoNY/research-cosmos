package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestSpotAuthz", func() {
	validSubaccountID := "ab8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
	validSubaccountID2 := "ab8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"
	invalidSubaccountID := "kb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
	Describe("Test Create Spot Limit Order Authz", func() {
		Describe("Test CreateSpotLimitOrderAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := CreateSpotLimitOrderAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := CreateSpotLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := CreateSpotLimitOrderAuthz{
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
				msg := CreateSpotLimitOrderAuthz{
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
				msg := CreateSpotLimitOrderAuthz{
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

		Describe("Test CreateSpotLimitOrderAuth Accept", func() {
			It("should not accept message type MsgCreateSpotMarketOrder", func() {
				authzMsg := CreateSpotLimitOrderAuthz{}
				msg := &MsgCreateSpotMarketOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order != subaccountID", func() {
				authzMsg := CreateSpotLimitOrderAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgCreateSpotLimitOrder{
					Order: SpotOrder{
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
				authzMsg := CreateSpotLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateSpotLimitOrder{
					Order: SpotOrder{
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
			It("should accept message MsgCreateSpotLimitOrder", func() {
				authzMsg := CreateSpotLimitOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateSpotLimitOrder{
					Order: SpotOrder{
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
	Describe("Test Create Spot Market Order Authz", func() {
		Describe("Test CreateSpotMarketOrderAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := CreateSpotMarketOrderAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := CreateSpotMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := CreateSpotMarketOrderAuthz{
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
				msg := CreateSpotMarketOrderAuthz{
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
				msg := CreateSpotMarketOrderAuthz{
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

		Describe("Test CreateSpotLimitOrderAuth Accept", func() {
			It("should not accept message type MsgCreateSpotMarketOrder", func() {
				authzMsg := CreateSpotMarketOrderAuthz{}
				msg := &MsgCreateSpotLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order != subaccountID", func() {
				authzMsg := CreateSpotMarketOrderAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgCreateSpotMarketOrder{
					Order: SpotOrder{
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
				authzMsg := CreateSpotMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateSpotMarketOrder{
					Order: SpotOrder{
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
			It("should accept message MsgCreateSpotMarketOrder", func() {
				authzMsg := CreateSpotMarketOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCreateSpotMarketOrder{
					Order: SpotOrder{
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
	Describe("Test Batch Create Spot Limit Order Authz", func() {
		Describe("Test BatchCreateSpotLimitOrdersAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := BatchCreateSpotLimitOrdersAuthz{
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
				msg := BatchCreateSpotLimitOrdersAuthz{
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
				msg := BatchCreateSpotLimitOrdersAuthz{
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

		Describe("Test BatchCreateSpotLimitOrdersAuthz Accept", func() {
			It("should not accept message type MsgCreateSpotMarketOrder", func() {
				authzMsg := BatchCreateSpotLimitOrdersAuthz{}
				msg := &MsgCreateSpotLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order having subaccount != granted subaccountID", func() {
				authzMsg := BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgBatchCreateSpotLimitOrders{
					Orders: []SpotOrder{
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
				authzMsg := BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCreateSpotLimitOrders{
					Orders: []SpotOrder{
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
			It("should accept message MsgBatchCreateSpotLimitOrders", func() {
				authzMsg := BatchCreateSpotLimitOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCreateSpotLimitOrders{
					Orders: []SpotOrder{
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
	Describe("Test Cancel Spot Market Order Authz", func() {
		Describe("Test CancelSpotOrderAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := CancelSpotOrderAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := CancelSpotOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := CancelSpotOrderAuthz{
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
				msg := CancelSpotOrderAuthz{
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
				msg := CreateSpotMarketOrderAuthz{
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

		Describe("Test CreateSpotLimitOrderAuth Accept", func() {
			It("should not accept message type MsgCreateSpotMarketOrder", func() {
				authzMsg := CancelSpotOrderAuthz{}
				msg := &MsgCreateSpotLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept canceling order subaccountID != granted subaccountID", func() {
				authzMsg := CancelSpotOrderAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgCancelSpotOrder{
					SubaccountId: validSubaccountID2,
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			})
			It("should not accept when marketId is not in accepted list", func() {
				authzMsg := CancelSpotOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCancelSpotOrder{
					SubaccountId: validSubaccountID,
					MarketId:     "0x54d4505adef6a5cef26bc403a33d595620ded4e15b9e2bc3dd489b714813366a",
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("requested market is unauthorized: unauthorized"))
			})
			It("should accept message MsgCancelSpotOrder", func() {
				authzMsg := CancelSpotOrderAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgCancelSpotOrder{
					SubaccountId: validSubaccountID,
					MarketId:     "0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
				}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(authz.AcceptResponse{Accept: true, Delete: false, Updated: nil}))
			})
		})
	})
	Describe("Test Batch Cancel Spot Orders Authz", func() {
		Describe("Test BatchCancelSpotOrdersAuthz validate basic", func() {
			It("Should return error when subaccount is not a valid hash", func() {
				msg := BatchCancelSpotOrdersAuthz{
					SubaccountId: invalidSubaccountID,
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
			})
			It("Should return error when there is no marketID supplied", func() {
				msg := BatchCancelSpotOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds:    []string{},
				}
				err := msg.ValidateBasic()
				Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
			})
			// TODO: It should handle case-sensitive marketIDs
			It("Should return error when there is more than AuthorizedMarketsLimit", func() {
				AuthorizedMarketsLimit = 3
				msg := BatchCancelSpotOrdersAuthz{
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
				msg := BatchCancelSpotOrdersAuthz{
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
				msg := BatchCancelSpotOrdersAuthz{
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

		Describe("Test BatchCreateSpotLimitOrdersAuthz Accept", func() {
			It("should not accept message type MsgCreateSpotMarketOrder", func() {
				authzMsg := BatchCancelSpotOrdersAuthz{}
				msg := &MsgCreateSpotLimitOrder{}
				res, err := authzMsg.Accept(sdk.Context{}, msg)
				Expect(res).To(Equal(authz.AcceptResponse{}))
				Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			})
			It("should not accept order having subaccount != granted subaccountID", func() {
				authzMsg := BatchCancelSpotOrdersAuthz{
					SubaccountId: validSubaccountID,
				}
				msg := &MsgBatchCancelSpotOrders{
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
				authzMsg := BatchCancelSpotOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCancelSpotOrders{
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
			It("should accept message BatchCancelSpotOrdersAuthz", func() {
				authzMsg := BatchCancelSpotOrdersAuthz{
					SubaccountId: validSubaccountID,
					MarketIds: []string{
						"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
						"0x1c79dac019f73e4060494ab1b4fcba734350656d6fc4d474f6a238c13c6f9ced",
					},
				}
				msg := &MsgBatchCancelSpotOrders{
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
