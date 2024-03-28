package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestAuthzCommon", func() {
	validSubaccontID := "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
	validSubaccontID2 := "ef8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"

	Describe("Test batchUpdateOrdersAuthz validate basic", func() {
		It("should return errors when supplied account is not hash", func() {
			msg := BatchUpdateOrdersAuthz{
				SubaccountId: "0xafbacdefffn",
			}
			err := msg.ValidateBasic()
			Expect(err.Error()).To(Equal("invalid subaccount id to authorize: internal logic error"))
		})
		It("should return errors when both kind of markets are empty", func() {
			msg := BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				SpotMarkets:       []string{},
				DerivativeMarkets: []string{},
			}
			err := msg.ValidateBasic()
			Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
		})
		It("should return errors when derivative market length > 3", func() {
			AuthorizedMarketsLimit = 2
			msg := BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				SpotMarkets:       []string{"market-id-1", "market-id-2", "market-id-2"},
				DerivativeMarkets: []string{"market-id-1", "market-id-2"},
			}
			err := msg.ValidateBasic()
			Expect(err.Error()).To(Equal("invalid markets array length: internal logic error"))
		})
		It("should return errors when markets are not unique", func() {
			AuthorizedMarketsLimit = 10
			msg := BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				SpotMarkets:       []string{"market-id-1", "market-id-2", "market-id-2"},
				DerivativeMarkets: []string{"market-id-1", "market-id-2"},
			}
			err := msg.ValidateBasic()
			Expect(err.Error()).To(Equal("cannot have duplicate markets: internal logic error"))
		})
		It("should return spot markets are not hex", func() {
			AuthorizedMarketsLimit = 10
			msg := BatchUpdateOrdersAuthz{
				SubaccountId: validSubaccontID,
				SpotMarkets:  []string{"market-id-1", "market-id-2"},
			}
			err := msg.ValidateBasic()
			Expect(err.Error()).To(Equal("invalid spot market id to authorize: internal logic error"))
		})
		It("should return derivative markets are not hex", func() {
			AuthorizedMarketsLimit = 10
			msg := BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				DerivativeMarkets: []string{"market-id-1"},
			}
			err := msg.ValidateBasic()
			Expect(err.Error()).To(Equal("invalid derivative market id to authorize: internal logic error"))
		})
		It("should pass basic validation", func() {
			AuthorizedMarketsLimit = 10
			msg := BatchUpdateOrdersAuthz{
				SubaccountId: validSubaccontID,
				DerivativeMarkets: []string{
					"0xc559df216747fc11540e638646c384ad977617d6d8f0ea5ffdfc18d52e58ab01",
					"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
				},
				SpotMarkets: []string{
					"0xfbc729e93b05b4c48916c1433c9f9c2ddb24605a73483303ea0f87a8886b52af",
				},
			}
			err := msg.ValidateBasic()
			Expect(err).To(BeNil())
		})

		It("should not pass basic validation with empty subaccount_id", func() {
			AuthorizedMarketsLimit = 10
			msg := BatchUpdateOrdersAuthz{
				SubaccountId: "",
				DerivativeMarkets: []string{
					"0xc559df216747fc11540e638646c384ad977617d6d8f0ea5ffdfc18d52e58ab01",
					"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e",
				},
				SpotMarkets: []string{
					"0xfbc729e93b05b4c48916c1433c9f9c2ddb24605a73483303ea0f87a8886b52af",
				},
			}
			err := msg.ValidateBasic()
			Expect(err).To(Not(BeNil()))
		})
	})
	Describe("Test batchUpdateOrdersAuthz Accept", func() {
		It("should accept batchUPdateOrderAuthz", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				SpotMarkets:       []string{},
				DerivativeMarkets: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId:                   validSubaccontID,
				DerivativeMarketIdsToCancelAll: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}

			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err).To(BeNil())
			Expect(resp).To(Equal(authz.AcceptResponse{
				Accept: true, Delete: false, Updated: nil,
			}))
		})
		It("should not accept when message is not MsgBatchUpdateOrders", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				SpotMarkets:       []string{},
				DerivativeMarkets: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchCancelSpotOrders{}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("type mismatch: invalid type"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		It("should not accept when subaccount not match", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				SpotMarkets:       []string{},
				DerivativeMarkets: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId:                   validSubaccontID2,
				DerivativeMarketIdsToCancelAll: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested subaccount is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		It("should not accept when spot order to create not in authz list", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId: validSubaccontID,
				SpotMarkets:  []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId: validSubaccontID,
				SpotOrdersToCreate: []*SpotOrder{
					{
						MarketId: "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0f",
					},
				},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested spot market to create orders is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		It("should not accept when marketIDs in SpotOrdersToCancel not authz", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId: validSubaccontID,
				SpotMarkets:  []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId: validSubaccontID,
				SpotOrdersToCancel: []*OrderData{
					{
						MarketId: "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0f",
					},
				},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested spot market to cancel orders is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		It("should not accept when marketIDs in SpotMarketIdsToCancelAll not in authz spot market list", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId: validSubaccontID,
				SpotMarkets:  []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId: validSubaccontID,
				SpotMarketIdsToCancelAll: []string{
					"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0f",
				},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested spot market to cancel all orders is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		// --
		It("should not accept when derivative order to create not in authz list", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				DerivativeMarkets: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId: validSubaccontID,
				DerivativeOrdersToCreate: []*DerivativeOrder{
					{
						MarketId: "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0f",
					},
				},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested derivative market to create orders is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		It("should not accept when marketIDs in DerivativeOrdersToCancel not authz", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				DerivativeMarkets: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId: validSubaccontID,
				DerivativeOrdersToCancel: []*OrderData{
					{
						MarketId: "0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0f",
					},
				},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested derivative market to cancel orders is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
		It("should not accept when marketIDs in DerivativeMarketIdsToCancelAll not in authz", func() {
			permission := &BatchUpdateOrdersAuthz{
				SubaccountId:      validSubaccontID,
				DerivativeMarkets: []string{"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0e"},
			}
			msg := &MsgBatchUpdateOrders{
				SubaccountId: validSubaccontID,
				DerivativeMarketIdsToCancelAll: []string{
					"0x01e920e081b6f3b2e5183399d5b6733bb6f80319e6be3805b95cb7236910ff0f",
				},
			}
			resp, err := permission.Accept(sdk.Context{}, msg)
			Expect(err.Error()).To(Equal("requested derivative market to cancel all orders is unauthorized: unauthorized"))
			Expect(resp).To(Equal(authz.AcceptResponse{}))
		})
	})
})
