package testexchange

import (
	"fmt"
	"reflect"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func getOrderTypeForTriggerPrice(triggerPrice, markPrice sdk.Dec, isBuy bool) exchangetypes.OrderType {
	var orderType exchangetypes.OrderType
	if isBuy {
		if triggerPrice.GT(markPrice) {
			orderType = exchangetypes.OrderType_STOP_BUY
		} else {
			orderType = exchangetypes.OrderType_TAKE_BUY
		}
	} else {
		if triggerPrice.GT(markPrice) {
			orderType = exchangetypes.OrderType_TAKE_SELL
		} else {
			orderType = exchangetypes.OrderType_STOP_SELL
		}
	}
	return orderType
}

func (tp *TestPlayer) createSpotOrderFromJson(params *ActionCreateOrder) (*types.SpotOrder, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	subaccountID := common.HexToHash(accounts[accountIndex].SubaccountIDs[params.SubaccountIndex])
	marketID := tp.FindMarketId(MarketType_spot, params.MarketIndex)
	isBuy := params.IsLong

	var orderType = exchangetypes.OrderType_UNSPECIFIED
	if params.OrderType != nil {
		orderType = params.OrderType.getExchangeOrderType()
		isBuy = orderType.IsBuy()
	}

	price := f2d(params.Price)
	quantity := f2d(params.Quantity)

	var triggerPrice *sdk.Dec = nil
	if params.TriggerPrice != nil {
		p := f2d(*params.TriggerPrice)
		triggerPrice = &p
		if !orderType.IsConditional() { // we need to fix the type
			markPrice := app.ExchangeKeeper.GetSpotMidPriceOrBestPrice(ctx, marketID)
			if markPrice == nil {
				return nil, exchangetypes.ErrInvalidMarketStatus.Wrap("Cannot determine conditional order type if mid price is not available")
			}

			if orderType == exchangetypes.OrderType_UNSPECIFIED {
				orderType = getOrderTypeForTriggerPrice(*triggerPrice, *markPrice, isBuy)
			}
		}
	}

	if orderType == exchangetypes.OrderType_UNSPECIFIED {
		if isBuy {
			orderType = exchangetypes.OrderType_BUY
		} else {
			orderType = exchangetypes.OrderType_SELL
		}
	}

	feeRecipient := FeeRecipient
	if params.FeeRecipient != nil {
		feeRecipient = accounts[accountIndex].AccAddress.String()
	}

	order := exchangetypes.SpotOrder{
		MarketId: marketID.String(),
		OrderInfo: exchangetypes.OrderInfo{
			SubaccountId: subaccountID.String(),
			FeeRecipient: feeRecipient,
			Price:        price,
			Quantity:     quantity,
		},
		OrderType:    orderType,
		TriggerPrice: triggerPrice,
	}
	return &order, nil
}

// create derivative limit/market order
func (tp *TestPlayer) ReplayCreateSpotOrderAction(params *ActionCreateOrder) (any, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex

	order, err := tp.createSpotOrderFromJson(params)
	if err != nil {
		return nil, err
	}

	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	sender := accounts[accountIndex].AccAddress.String()
	switch params.ActionType {
	case ActionType_spotLimitOrder:
		msg := &exchangetypes.MsgCreateSpotLimitOrder{
			Sender: sender,
			Order:  *order,
		}
		if err := msg.ValidateBasic(); err != nil {
			return nil, err
		}
		var repl *exchangetypes.MsgCreateSpotLimitOrderResponse
		repl, err = msgServer.CreateSpotLimitOrder(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++
			var orderId = params.OrderId
			if orderId == "" {
				orderId = repl.OrderHash
			}
			tp.ordersById[orderId] = newOrderId(repl.OrderHash, msg.Order.MarketId, MarketType_spot)
			return repl, nil
		} else {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
	case ActionType_spotMarketOrder:
		msg := &exchangetypes.MsgCreateSpotMarketOrder{
			Sender: sender,
			Order:  *order,
		}
		if err = msg.ValidateBasic(); err != nil {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
		var repl *exchangetypes.MsgCreateSpotMarketOrderResponse
		repl, err = msgServer.CreateSpotMarketOrder(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++
			var orderId = params.OrderId
			if orderId == "" {
				orderId = repl.OrderHash
			}
			tp.ordersById[orderId] = newOrderId(repl.OrderHash, msg.Order.MarketId, MarketType_spot)
			return repl, nil
		} else {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
	}
	return nil, err
}

func (tp *TestPlayer) createDerivativeOrderFromJson(params *ActionCreateOrder) (*types.DerivativeOrder, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	subaccountID := common.HexToHash(accounts[accountIndex].SubaccountIDs[params.SubaccountIndex])
	marketType := tp.GetDefaultMarketType(params.MarketType)
	marketID := tp.FindMarketId(marketType, params.MarketIndex)
	var market keeper.DerivativeMarketI
	var markPrice sdk.Dec
	switch marketType {
	case MarketType_binary:
		market = app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, marketID, true)
	case MarketType_derivative, MarketType_expiry:
		market, markPrice = app.ExchangeKeeper.GetDerivativeMarketWithMarkPrice(ctx, marketID, true)
	}
	if market == nil || reflect.ValueOf(market).IsNil() || market.GetMarketStatus() != exchangetypes.MarketStatus_Active {
		return nil, exchangetypes.ErrMarketInvalid
	}

	isBuy := params.IsLong
	isReduceOnly := params.IsReduceOnly

	var orderType = exchangetypes.OrderType_UNSPECIFIED
	if params.OrderType != nil {
		orderType = params.OrderType.getExchangeOrderType()
		isBuy = orderType.IsBuy()
	}

	price := f2d(params.Price)
	quantity := f2d(params.Quantity)
	notional := price.Mul(quantity)

	var triggerPrice *sdk.Dec = nil
	if params.TriggerPrice != nil {
		p := f2d(*params.TriggerPrice)
		triggerPrice = &p
		if orderType == exchangetypes.OrderType_UNSPECIFIED {
			if isBuy {
				if triggerPrice.GT(markPrice) {
					orderType = exchangetypes.OrderType_STOP_BUY
				} else {
					orderType = exchangetypes.OrderType_TAKE_BUY
				}
			} else {
				if triggerPrice.GT(markPrice) {
					orderType = exchangetypes.OrderType_TAKE_SELL
				} else {
					orderType = exchangetypes.OrderType_STOP_SELL
				}
			}
		}
	}

	if orderType == exchangetypes.OrderType_UNSPECIFIED {
		if isBuy {
			orderType = exchangetypes.OrderType_BUY
		} else {
			orderType = exchangetypes.OrderType_SELL
		}
	}

	margin := sdk.Dec{}
	leverage := f2d(params.Leverage)
	if leverage.IsZero() {
		leverage = f2d(1.0)
	}

	if params.Margin != nil {
		margin = f2d(*params.Margin)
	} else {
		if isReduceOnly {
			margin = sdk.ZeroDec()
		} else {
			switch params.ActionType {
			case ActionType_boMarketOrder, ActionType_boLimitOrder:
				if isBuy {
					// For Buys: Margin = price * quantity
					margin = notional
				} else {
					// For Sells: Margin = (1*scaleFactor - price) * quantity
					margin = (exchangetypes.GetScaledPrice(sdk.OneDec(), market.GetOracleScaleFactor()).Sub(price)).Mul(quantity)
				}
			default: // perps and expiries
				useMarkPrice := markPrice
				if orderType.IsConditional() && triggerPrice != nil {
					useMarkPrice = *triggerPrice
				}
				if isBuy {
					// For Buys: Margin >= MarkPrice * Quantity * (1 - InitialMarginRatio) && Margin >= Price * Quantity
					// multiplier := sdk.MaxDec(sdk.OneDec().Quo(leverage), market.InitialMarginRatio)
					// margin = matchTick(notional.Mul(multiplier), market.MinQuantityTickSize)
					margin = matchTick(useMarkPrice.Mul(quantity).Mul(sdk.OneDec().Sub(market.GetInitialMarginRatio())).
						Add(notional).
						Add(sdk.NewDec(tp.rand.Int63n(10000)).QuoInt64(100)), market.GetMinQuantityTickSize())
				} else {
					// For Sells: Margin >= MarkPrice * Quantity * (1 + InitialMarginRatio) && Margin >= Price * Quantity
					// multiplier := sdk.MaxDec(sdk.OneDec().Quo(leverage), market.InitialMarginRatio)
					// margin = matchTick(notional.Mul(multiplier), market.MinQuantityTickSize)
					margin = matchTick(useMarkPrice.Mul(quantity).Mul(sdk.OneDec().Add(market.GetInitialMarginRatio())).
						Add(notional).
						Add(sdk.NewDec(tp.rand.Int63n(10000)).QuoInt64(100)), market.GetMinQuantityTickSize())

				}
			}
		}
	}

	feeRecipient := FeeRecipient
	if params.FeeRecipient != nil {
		feeRecipient = accounts[accountIndex].AccAddress.String()
	}

	order := exchangetypes.DerivativeOrder{
		MarketId: marketID.String(),
		OrderInfo: exchangetypes.OrderInfo{
			SubaccountId: subaccountID.String(),
			FeeRecipient: feeRecipient,
			Price:        price,
			Quantity:     quantity,
		},
		OrderType:    orderType,
		Margin:       margin,
		TriggerPrice: triggerPrice,
	}

	return &order, nil
}

// create derivative limit/market order
func (tp *TestPlayer) ReplayCreateDerivativeOrderAction(params *ActionCreateOrder, isExpiryMarket bool) (any, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	marketType := tp.GetDefaultMarketType(params.MarketType)
	order, err := tp.createDerivativeOrderFromJson(params)
	if err != nil {
		return nil, err
	}
	if order.SubaccountID().String() == "0x198c4ca029bdfa95d98673862a04dae4ed0d6b98000000000000000000000000" && tp.TestInput.Perps[params.MarketIndex].QuoteDenom == "USDT7" {
		fmt.Fprintf(GinkgoWriter, "Order: %v\n", order)
	}

	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	switch params.ActionType {
	case ActionType_derivativeLimitOrder:
		msg := &exchangetypes.MsgCreateDerivativeLimitOrder{
			Sender: accounts[accountIndex].AccAddress.String(),
			Order:  *order,
		}
		if err := msg.ValidateBasic(); err != nil {
			return nil, err
		}
		var repl *exchangetypes.MsgCreateDerivativeLimitOrderResponse
		repl, err = msgServer.CreateDerivativeLimitOrder(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++
			var orderId = params.OrderId
			if orderId == "" {
				orderId = repl.OrderHash
			}
			tp.ordersById[orderId] = newOrderId(repl.OrderHash, msg.Order.MarketId, marketType)
			return repl, nil
		} else {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
	case ActionType_derivativeMarketOrder:
		msg := &exchangetypes.MsgCreateDerivativeMarketOrder{
			Sender: accounts[accountIndex].AccAddress.String(),
			Order:  *order,
		}
		if err = msg.ValidateBasic(); err != nil {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
		var repl *exchangetypes.MsgCreateDerivativeMarketOrderResponse
		repl, err = msgServer.CreateDerivativeMarketOrder(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++
			var orderId = params.OrderId
			if orderId == "" {
				orderId = repl.OrderHash
			}
			tp.ordersById[orderId] = newOrderId(repl.OrderHash, msg.Order.MarketId, marketType)
			return repl, nil
		} else {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
	}
	return nil, err
}

// create binaryOptions limit order
func (tp *TestPlayer) replayCreateBinaryOptionsOrderAction(params *ActionCreateOrder) (any, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	order, err := tp.createDerivativeOrderFromJson(params)
	if err != nil {
		return nil, err
	}

	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
	switch params.ActionType {
	case ActionType_boMarketOrder:
		msg := &exchangetypes.MsgCreateBinaryOptionsMarketOrder{
			Sender: accounts[accountIndex].AccAddress.String(),
			Order:  *order,
		}

		if err := msg.ValidateBasic(); err != nil {
			fmt.Fprintf(GinkgoWriter, "Action %v, Error validating order: (%v): %v\n", params.ActionId, msg, err.Error())
			return nil, err
		}

		var resp *exchangetypes.MsgCreateBinaryOptionsMarketOrderResponse
		resp, err := msgServer.CreateBinaryOptionsMarketOrder(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)

		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++

			var orderId = params.OrderId
			if orderId == "" {
				orderId = resp.OrderHash
			}
			tp.ordersById[orderId] = newOrderId(resp.OrderHash, msg.Order.MarketId, MarketType_binary)
			return resp, nil
		} else {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
	case ActionType_boLimitOrder:
		msg := &exchangetypes.MsgCreateBinaryOptionsLimitOrder{
			Sender: accounts[accountIndex].AccAddress.String(),
			Order:  *order,
		}

		if err := msg.ValidateBasic(); err != nil {
			fmt.Fprintf(GinkgoWriter, "Action %v, Error validating order: (%v): %v\n", params.ActionId, msg, err.Error())
			return nil, err
		}

		resp, err := msgServer.CreateBinaryOptionsLimitOrder(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++

			var orderId = params.OrderId
			if orderId == "" {
				orderId = resp.OrderHash
			}
			tp.ordersById[orderId] = newOrderId(resp.OrderHash, msg.Order.MarketId, MarketType_binary)
			return resp, nil
		} else {
			tp.FailedActions[params.ActionType]++
			return nil, err
		}
	}
	return nil, err
}

func (tp *TestPlayer) replayCancelOrderAction(params *ActionCancelOrder) (any, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	orderId := tp.ordersById[params.OrderId]
	if orderId == nil {
		return nil, types.ErrOrderInvalid.Wrapf(fmt.Sprintf("Order with id: %v not found", params.OrderId))
	}
	marketID := orderId.marketId
	marketType := tp.GetDefaultMarketType(params.MarketType)
	account := accounts[params.AccountIndex].AccAddress
	subaccountId := strings.ToLower(accounts[params.AccountIndex].SubaccountIDs[0])

	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
	orderMask := params.OrderMask
	var response any
	var err error
	switch marketType {
	case MarketType_derivative, MarketType_expiry:
		{
			msg := &exchangetypes.MsgCancelDerivativeOrder{
				Sender:       account.String(),
				MarketId:     marketID,
				SubaccountId: subaccountId,
				OrderHash:    orderId.orderHash,
				OrderMask:    orderMask,
			}
			if err := msg.ValidateBasic(); err != nil {
				return nil, err
			}
			response, err = msgServer.CancelDerivativeOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err != nil {
				return nil, err
			}
		}
	case MarketType_binary:
		{
			msg := &exchangetypes.MsgCancelBinaryOptionsOrder{
				Sender:       account.String(),
				MarketId:     marketID,
				SubaccountId: subaccountId,
				OrderHash:    orderId.orderHash,
			}
			if err = msg.ValidateBasic(); err != nil {
				return nil, err
			}
			response, err = msgServer.CancelBinaryOptionsOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err != nil {
				return nil, err
			}
		}
	case MarketType_spot:
		{
			msg := &exchangetypes.MsgCancelSpotOrder{
				Sender:       account.String(),
				MarketId:     marketID,
				SubaccountId: subaccountId,
				OrderHash:    orderId.orderHash,
			}
			if err = msg.ValidateBasic(); err != nil {
				return nil, err
			}
			response, err = msgServer.CancelSpotOrder(
				sdk.WrapSDKContext(ctxCached),
				msg,
			)
			if err != nil {
				return nil, err
			}
		}
	}
	writeCache()
	tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
	tp.SuccessActions[params.ActionType]++
	return response, nil
}

func (tp *TestPlayer) replayBatchUpdateAction(params *ActionBatchOrderUpdate) error {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	subaccountID := common.HexToHash(accounts[params.AccountIndex].SubaccountIDs[0])

	// spot
	var spotOrders []*exchangetypes.SpotOrder
	if params.SpotOrdersToCreate != nil {
		spotOrders = make([]*exchangetypes.SpotOrder, 0)
		for _, action := range params.SpotOrdersToCreate {
			order, err := tp.createSpotOrderFromJson(&action)
			if err != nil {
				return err
			}
			spotOrders = append(spotOrders, order)
		}
	}

	var spotCancels []*exchangetypes.OrderData
	if params.SpotOrdersToCancel != nil {
		spotCancels = make([]*exchangetypes.OrderData, 0)
		for _, orderId := range params.SpotOrdersToCancel {
			orderInfo := tp.ordersById[orderId]
			orderData := &exchangetypes.OrderData{
				MarketId:     orderInfo.marketId,
				SubaccountId: subaccountID.String(),
				OrderHash:    orderInfo.orderHash,
			}
			spotCancels = append(spotCancels, orderData)
		}
	}

	var spotMarketsToCancel []string
	if params.SpotMarketsToCancelAll != nil {
		spotMarketsToCancel = make([]string, 0)
		for _, marketIdx := range params.SpotMarketsToCancelAll {
			marketID := tp.FindMarketId(MarketType_spot, marketIdx)
			spotMarketsToCancel = append(spotMarketsToCancel, marketID.String())
		}
	}

	// derivatives

	var derivativeOrders []*exchangetypes.DerivativeOrder
	if params.DerivativeOrdersToCreate != nil {
		derivativeOrders = make([]*exchangetypes.DerivativeOrder, 0)
		for _, action := range params.DerivativeOrdersToCreate {
			order, err := tp.createDerivativeOrderFromJson(&action)
			if err != nil {
				return err
			}
			derivativeOrders = append(derivativeOrders, order)
		}
	}

	var derivativeCancels []*exchangetypes.OrderData
	if params.DerivativeOrdersToCancel != nil {
		derivativeCancels = make([]*exchangetypes.OrderData, 0)
		for _, orderId := range params.DerivativeOrdersToCancel {
			orderInfo := tp.ordersById[orderId]
			orderData := &exchangetypes.OrderData{
				MarketId:     orderInfo.marketId,
				SubaccountId: subaccountID.String(),
				OrderHash:    orderInfo.orderHash,
			}
			derivativeCancels = append(derivativeCancels, orderData)
		}
	}
	var derivativeMarketsToCancel []string
	if params.DerivativeMarketsToCancelAll != nil {
		derivativeMarketsToCancel = make([]string, 0)
		for _, marketIdx := range params.DerivativeMarketsToCancelAll {
			marketID := tp.FindMarketId(MarketType_derivative, marketIdx)
			derivativeMarketsToCancel = append(derivativeMarketsToCancel, marketID.String())
		}
	}

	// binaries
	var binaryOrders []*exchangetypes.DerivativeOrder
	if params.BinaryOrdersToCreate != nil {
		derivativeOrders = make([]*exchangetypes.DerivativeOrder, 0)
		for _, action := range params.BinaryOrdersToCreate {
			order, err := tp.createDerivativeOrderFromJson(&action) // TODO: handle proper binary order creation
			if err != nil {
				return err
			}
			derivativeOrders = append(derivativeOrders, order)
		}
	}

	var binaryCancels []*exchangetypes.OrderData
	if params.BinaryOrdersToCancel != nil {
		binaryCancels = make([]*exchangetypes.OrderData, 0)
		for _, orderId := range params.BinaryOrdersToCancel {
			orderInfo := tp.ordersById[orderId]
			orderData := &exchangetypes.OrderData{
				MarketId:     orderInfo.marketId,
				SubaccountId: subaccountID.String(),
				OrderHash:    orderInfo.orderHash,
			}
			binaryCancels = append(binaryCancels, orderData)
		}
	}
	var binaryMarketsToCancel []string
	if params.BinaryMarketsToCancelAll != nil {
		binaryMarketsToCancel = make([]string, 0)
		for _, marketIdx := range params.BinaryMarketsToCancelAll {
			marketID := tp.FindMarketId(MarketType_binary, marketIdx)
			binaryMarketsToCancel = append(binaryMarketsToCancel, marketID.String())
		}
	}

	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)

	subaccountIdToUse := ""
	if len(spotMarketsToCancel) > 0 || len(derivativeMarketsToCancel) > 0 || len(binaryMarketsToCancel) > 0 {
		subaccountIdToUse = subaccountID.String()
	}

	switch params.BatchType {
	case BatchActionType_cancelDerivatives:
		msg := &types.MsgBatchCancelDerivativeOrders{
			Sender: accounts[params.AccountIndex].AccAddress.String(),
			Data:   DerefSlice(derivativeCancels),
		}
		if err := msg.ValidateBasic(); err != nil {
			tp.FailedActions[params.ActionType]++
			return err
		}
		_, err := msgServer.BatchCancelDerivativeOrders(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++
		} else {
			tp.FailedActions[params.ActionType]++
			return err
		}
	default:
		msg := &types.MsgBatchUpdateOrders{
			Sender:                            accounts[params.AccountIndex].AccAddress.String(),
			SubaccountId:                      subaccountIdToUse,
			SpotOrdersToCreate:                spotOrders,
			SpotOrdersToCancel:                spotCancels,
			SpotMarketIdsToCancelAll:          spotMarketsToCancel,
			DerivativeOrdersToCreate:          derivativeOrders,
			DerivativeOrdersToCancel:          derivativeCancels,
			DerivativeMarketIdsToCancelAll:    derivativeMarketsToCancel,
			BinaryOptionsOrdersToCancel:       binaryCancels,
			BinaryOptionsMarketIdsToCancelAll: binaryMarketsToCancel,
			BinaryOptionsOrdersToCreate:       binaryOrders,
		}
		if err := msg.ValidateBasic(); err != nil {
			tp.FailedActions[params.ActionType]++
			return err
		}
		response, err := msgServer.BatchUpdateOrders(
			sdk.WrapSDKContext(ctxCached),
			msg,
		)
		if err == nil {
			writeCache()
			tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
			tp.SuccessActions[params.ActionType]++

			for i, hash := range response.SpotOrderHashes {
				var orderId = params.SpotOrdersToCreate[i].OrderId
				if orderId == "" {
					orderId = hash
				}
				tp.ordersById[orderId] = newOrderId(hash, msg.SpotOrdersToCreate[i].MarketId, MarketType_spot)
			}
			for i, hash := range response.DerivativeOrderHashes {
				var orderId = params.DerivativeOrdersToCreate[i].OrderId
				if orderId == "" {
					orderId = hash
				}
				tp.ordersById[orderId] = newOrderId(hash, msg.DerivativeOrdersToCreate[i].MarketId, MarketType_derivative)
			}
			for i, hash := range response.BinaryOptionsOrderHashes {
				var orderId = params.BinaryOrdersToCreate[i].OrderId
				if orderId == "" {
					orderId = hash
				}
				tp.ordersById[orderId] = newOrderId(hash, msg.BinaryOptionsOrdersToCreate[i].MarketId, MarketType_binary)
			}
		} else {
			tp.FailedActions[params.ActionType]++
			return err
		}
	}

	return nil
}
