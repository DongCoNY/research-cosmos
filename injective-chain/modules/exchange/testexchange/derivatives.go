package testexchange

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

// utils
func (testInput *TestInput) NewMsgCreateBinaryOptionsLimitOrder(price sdk.Dec, quantity sdk.Dec, isReduceOnly bool, orderType exchangetypes.OrderType, subaccountID common.Hash, marketIdx int) *exchangetypes.MsgCreateBinaryOptionsLimitOrder {
	sender := types.SubaccountIDToSdkAddress(subaccountID)
	market := testInput.BinaryMarkets[marketIdx]
	msg := exchangetypes.NewMsgCreateBinaryOptionsLimitOrder(sender, &exchangetypes.BinaryOptionsMarket{
		OracleScaleFactor: market.OracleScaleFactor,
		MarketId:          market.MarketID.String(),
	}, subaccountID.Hex(), sender.String(), price, quantity, orderType, isReduceOnly)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgCreateDerivativeLimitOrder(price sdk.Dec, quantity sdk.Dec, margin sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash) *exchangetypes.MsgCreateDerivativeLimitOrder {
	msg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(DefaultFeeRecipientAddress, price, quantity, margin, orderType, subaccountID, 0, false)
	OrFail(msg.ValidateBasic())
	return msg
}

func NewBareDerivativeLimitOrder(price, quantity, margin sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash, isExpiry bool) *BareDerivativeLimitOrder {
	return &BareDerivativeLimitOrder{
		Price:        price,
		Quantity:     quantity,
		Margin:       margin,
		OrderType:    orderType,
		SubaccountID: subaccountID,
		IsExpiry:     isExpiry,
	}
}

func NewBareDerivativeLimitOrderFromString(price, quantity, margin string, orderType exchangetypes.OrderType, subaccountID common.Hash, isExpiry bool) *BareDerivativeLimitOrder {
	return NewBareDerivativeLimitOrder(sdk.MustNewDecFromStr(price), sdk.MustNewDecFromStr(quantity), sdk.MustNewDecFromStr(margin), orderType, subaccountID, isExpiry)
}

type BareDerivativeLimitOrder struct {
	Price        sdk.Dec
	Quantity     sdk.Dec
	Margin       sdk.Dec
	OrderType    exchangetypes.OrderType
	SubaccountID common.Hash
	IsExpiry     bool
}

func (testInput *TestInput) NewListOfMsgCreateDerivativeLimitOrderForMarketIndex(
	marketIndex int,
	bareOrders ...*BareDerivativeLimitOrder,
) []*exchangetypes.MsgCreateDerivativeLimitOrder {
	msgs := make([]*exchangetypes.MsgCreateDerivativeLimitOrder, len(bareOrders))
	for idx, order := range bareOrders {
		msgs[idx] = testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(DefaultFeeRecipientAddress, order.Price, order.Quantity, order.Margin, order.OrderType, order.SubaccountID, marketIndex, order.IsExpiry)
	}
	return msgs
}

func (testInput *TestInput) NewMsgCreateDerivativeMarketOrder(quantity, margin sdk.Dec, worstPrice sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash) *exchangetypes.MsgCreateDerivativeMarketOrder {
	msg := testInput.NewMsgCreateDerivativeMarketOrderForMarketIndex(worstPrice, quantity, orderType, margin, subaccountID, 0, false)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgCreateExpiryDerivativeLimitOrder(price sdk.Dec, quantity sdk.Dec, margin sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash) *exchangetypes.MsgCreateDerivativeLimitOrder {
	msg := testInput.NewMsgCreateDerivativeLimitOrderForMarketIndex(DefaultFeeRecipientAddress, price, quantity, margin, orderType, subaccountID, 0, true)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidatePosition(liquidableSubaccountID common.Hash) *exchangetypes.MsgLiquidatePosition {
	msg := testInput.NewMsgLiquidatePositionForMarketIndex(liquidableSubaccountID, 0)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidatePositionForMarketIndex(liquidableSubaccountID common.Hash, marketIndex int) *exchangetypes.MsgLiquidatePosition {
	msg := testInput.NewMsgLiquidatePositionWithOrderForMarketIndex(liquidableSubaccountID, common.Hash{}, marketIndex, exchangetypes.OrderType_UNSPECIFIED, sdk.Dec{}, sdk.Dec{}, sdk.Dec{}, false)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidatePositionForMarketID(liquidableSubaccountID, liquidatorSubaccountId, marketID common.Hash) *exchangetypes.MsgLiquidatePosition {
	msg := testInput.NewMsgLiquidatePositionWithOrderForMarketId(liquidableSubaccountID, liquidatorSubaccountId, marketID, exchangetypes.OrderType_UNSPECIFIED, sdk.Dec{}, sdk.Dec{}, sdk.Dec{}, false)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidateExpiryPosition(liquidableSubaccountID common.Hash) *exchangetypes.MsgLiquidatePosition {
	msg := testInput.NewMsgLiquidateExpiryPositionForMarketIndex(liquidableSubaccountID, 0)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidateExpiryPositionForMarketIndex(liquidableSubaccountID common.Hash, marketIndex int) *exchangetypes.MsgLiquidatePosition {
	msg := testInput.NewMsgLiquidatePositionWithOrderForMarketIndex(liquidableSubaccountID, common.Hash{}, marketIndex, exchangetypes.OrderType_UNSPECIFIED, sdk.Dec{}, sdk.Dec{}, sdk.Dec{}, true)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidatePositionWithOrder(liquidableSubaccountID, liquidatorSubaccountID common.Hash, orderType exchangetypes.OrderType, price, quantity, margin sdk.Dec) *exchangetypes.MsgLiquidatePosition {
	msg := testInput.NewMsgLiquidatePositionWithOrderForMarketIndex(liquidableSubaccountID, liquidatorSubaccountID, 0, orderType, price, quantity, margin, false)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgLiquidatePositionWithOrderForMarketIndex(liquidableSubaccountID, liquidatorSubaccountID common.Hash, marketIndex int, orderType exchangetypes.OrderType, price, quantity, margin sdk.Dec, isExpiry bool) *exchangetypes.MsgLiquidatePosition {
	var marketID common.Hash

	if isExpiry {
		marketID = testInput.ExpiryMarkets[marketIndex].MarketID
	} else {
		marketID = testInput.Perps[marketIndex].MarketID
	}

	return testInput.NewMsgLiquidatePositionWithOrderForMarketId(liquidableSubaccountID, liquidatorSubaccountID, marketID, orderType, price, quantity, margin, isExpiry)
}

func (testInput *TestInput) NewMsgLiquidatePositionWithOrderForMarketId(liquidableSubaccountID, liquidatorSubaccountID, marketID common.Hash, orderType exchangetypes.OrderType, price, quantity, margin sdk.Dec, isExpiry bool) *exchangetypes.MsgLiquidatePosition {
	var order *exchangetypes.DerivativeOrder

	if !price.IsNil() {
		order = &exchangetypes.DerivativeOrder{
			MarketId: marketID.Hex(),
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: liquidatorSubaccountID.Hex(),
				FeeRecipient: DefaultFeeRecipientAddress,
				Price:        price,
				Quantity:     quantity,
			},
			Margin:       margin,
			OrderType:    orderType,
			TriggerPrice: nil,
		}
	}

	msg := exchangetypes.MsgLiquidatePosition{
		Sender:       exchangetypes.SubaccountIDToSdkAddress(liquidatorSubaccountID).String(),
		SubaccountId: liquidableSubaccountID.Hex(),
		MarketId:     marketID.Hex(),
		Order:        order,
	}
	OrFail(msg.ValidateBasic())
	return &msg
}

func (testInput *TestInput) NewMsgEmergencySettleMarket(liquidableSubaccountID, liquidatorSubaccountID common.Hash) *exchangetypes.MsgEmergencySettleMarket {
	msg := testInput.NewMsgEmergencySettleMarketForMarketIndex(liquidableSubaccountID, liquidatorSubaccountID, 0, false)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgEmergencySettleMarketForMarketIndex(liquidableSubaccountID, liquidatorSubaccountID common.Hash, marketIndex int, isExpiry bool) *exchangetypes.MsgEmergencySettleMarket {
	var marketID common.Hash

	if isExpiry {
		marketID = testInput.ExpiryMarkets[marketIndex].MarketID
	} else {
		marketID = testInput.Perps[marketIndex].MarketID
	}

	msg := exchangetypes.MsgEmergencySettleMarket{
		Sender:       exchangetypes.SubaccountIDToSdkAddress(liquidatorSubaccountID).String(),
		SubaccountId: liquidableSubaccountID.Hex(),
		MarketId:     marketID.Hex(),
	}
	OrFail(msg.ValidateBasic())

	return &msg
}

func NewCreateDerivativeLimitOrderMsg(
	orderType exchangetypes.OrderType,
	marketID common.Hash,
	sender common.Hash,
	feeRecipient string,
	price,
	quantity,
	margin sdk.Dec,
) *exchangetypes.MsgCreateDerivativeLimitOrder {
	msg := &exchangetypes.MsgCreateDerivativeLimitOrder{
		Sender: exchangetypes.SubaccountIDToSdkAddress(sender).String(),
		Order: exchangetypes.DerivativeOrder{
			MarketId:  marketID.Hex(),
			OrderType: orderType,
			Margin:    margin,
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: sender.Hex(),
				FeeRecipient: feeRecipient,
				Price:        price,
				Quantity:     quantity,
			},
		},
	}

	OrFail(msg.ValidateBasic())

	return msg
}

func (testInput *TestInput) NewMsgCreateDerivativeLimitOrderForMarketIndex(
	feeRecipient string,
	price, quantity, margin sdk.Dec,
	orderType exchangetypes.OrderType,
	subaccountID common.Hash,
	marketIndex int,
	isExpiry bool,
) *exchangetypes.MsgCreateDerivativeLimitOrder {
	sender := exchangetypes.SubaccountIDToSdkAddress(subaccountID).String()
	var marketID common.Hash

	if isExpiry {
		marketID = testInput.ExpiryMarkets[marketIndex].MarketID
	} else {
		marketID = testInput.Perps[marketIndex].MarketID
	}

	msg := exchangetypes.MsgCreateDerivativeLimitOrder{
		Sender: sender,
		Order: exchangetypes.DerivativeOrder{
			MarketId: marketID.Hex(),
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				FeeRecipient: feeRecipient,
				Price:        price,
				Quantity:     quantity,
			},
			Margin:       margin,
			OrderType:    orderType,
			TriggerPrice: nil,
		},
	}
	OrFail(msg.ValidateBasic())
	return &msg
}

func (testInput *TestInput) NewMsgCreateBinaryOptionsLimitOrderForMarketIndex(
	feeRecipient string,
	price, quantity sdk.Dec,
	orderType exchangetypes.OrderType,
	subaccountID common.Hash,
	marketIndex int,
	isReduceOnly bool,
) *exchangetypes.MsgCreateBinaryOptionsLimitOrder {
	sender := exchangetypes.SubaccountIDToSdkAddress(subaccountID)
	market := testInput.BinaryMarkets[marketIndex]

	msg := exchangetypes.NewMsgCreateBinaryOptionsLimitOrder(sender, &exchangetypes.BinaryOptionsMarket{
		OracleScaleFactor: market.OracleScaleFactor,
		MarketId:          market.MarketID.String(),
	}, subaccountID.Hex(), feeRecipient, price, quantity, orderType, isReduceOnly)
	OrFail(msg.ValidateBasic())
	return msg
}

func (testInput *TestInput) NewMsgCreateDerivativeMarketOrderForMarketIndex(worstPrice sdk.Dec, quantity sdk.Dec, orderType exchangetypes.OrderType, margin sdk.Dec, subaccountID common.Hash, marketIndex int, isTimeExpiry bool) *exchangetypes.MsgCreateDerivativeMarketOrder {
	sender := exchangetypes.SubaccountIDToSdkAddress(subaccountID).String()

	var marketID common.Hash
	if isTimeExpiry {
		marketID = testInput.ExpiryMarkets[marketIndex].MarketID
	} else {
		marketID = testInput.Perps[marketIndex].MarketID
	}

	msg := exchangetypes.MsgCreateDerivativeMarketOrder{
		Sender: sender,
		Order: exchangetypes.DerivativeOrder{
			MarketId: common.Bytes2Hex(marketID.Bytes()),
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				FeeRecipient: DefaultFeeRecipientAddress,
				Price:        worstPrice,
				Quantity:     quantity,
			},
			OrderType:    orderType,
			TriggerPrice: nil,
			Margin:       margin,
		},
	}
	OrFail(msg.ValidateBasic())
	return &msg
}

func (testInput *TestInput) NewMsgCreateBinaryOptionsMarketOrderForMarketIndex(feeRecipient string, worstPrice sdk.Dec, quantity sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash, marketIndex int, isReduceOnly bool) *exchangetypes.MsgCreateBinaryOptionsMarketOrder {
	sender := exchangetypes.SubaccountIDToSdkAddress(subaccountID)
	market := testInput.BinaryMarkets[marketIndex]

	msg := exchangetypes.NewMsgCreateBinaryOptionsMarketOrder(sender, &exchangetypes.BinaryOptionsMarket{
		OracleScaleFactor: market.OracleScaleFactor,
		MarketId:          market.MarketID.String(),
	}, subaccountID.Hex(), feeRecipient, worstPrice, quantity, orderType, isReduceOnly)
	OrFail(msg.ValidateBasic())
	return msg
}

type DerivativeLimitTestOrder struct {
	Price           int    `json:"price"`
	Quantity        int    `json:"quantity"`
	SubAccountNonce string `json:"subaccountNonce"` // last two digits of subaccount
	QuantityFilled  int    `json:"quantityFilled"`
}

type DerivativeOrderbook struct {
	SellOrders            []DerivativeLimitTestOrder `json:"sell"`
	TransientSellOrders   []DerivativeLimitTestOrder `json:"transient-sells"`
	BuyOrders             []DerivativeLimitTestOrder `json:"buy"`
	TransientBuyOrders    []DerivativeLimitTestOrder `json:"transient-buys"`
	ExpectedClearingPrice float32                    `json:"expected-clearing-price"`
}
