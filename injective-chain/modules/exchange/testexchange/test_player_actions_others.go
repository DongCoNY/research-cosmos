package testexchange

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"cosmossdk.io/math"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptokeys "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/gogoproto/jsonpb"
	"github.com/cosmos/gogoproto/proto"

	injjson "github.com/InjectiveLabs/jsonc"
	"github.com/cometbft/cometbft/abci/types"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	auctionkeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/keeper"
	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	insurancekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/keeper"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	peggy "github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy"
	peggykeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/keeper"
	peggytypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"
)

func (tp *TestPlayer) replaySetOraclePriceAction(params *ActionSetPriceOracles) {
	ctx := tp.Ctx
	app := tp.App
	for idx, spot := range tp.TestInput.Spots {
		oracleBase, oracleQuote := spot.BaseDenom, spot.QuoteDenom
		price := f2d(params.SpotsPrices[idx])

		tp.App.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			oracleBase,
			oracleQuote,
			oracletypes.NewPriceState(price, ctx.BlockTime().Unix()),
		)
	}
	for idx, perp := range tp.TestInput.Perps {
		oracleBase, oracleQuote, _ := perp.OracleBase, perp.OracleQuote, perp.OracleType
		price := f2d(params.PerpsPrices[idx])
		tp.App.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			oracleBase,
			oracleQuote,
			oracletypes.NewPriceState(price, ctx.BlockTime().Unix()),
		)
	}
	if len(params.ExpiryMarkets) > 0 { // may not be set in manually created jsons
		for idx, expiryMarket := range tp.TestInput.ExpiryMarkets {
			oracleBase, oracleQuote, _ := expiryMarket.OracleBase, expiryMarket.OracleQuote, expiryMarket.OracleType
			price := f2d(params.ExpiryMarkets[idx])
			app.OracleKeeper.SetPriceFeedPriceState(
				ctx,
				oracleBase,
				oracleQuote,
				oracletypes.NewPriceState(price, ctx.BlockTime().Unix()),
			)
		}
	}

	for idx, boMarket := range tp.TestInput.BinaryMarkets {
		price := f2d(params.BinaryMarkets[idx])
		app.OracleKeeper.SetProviderPriceState(
			ctx,
			boMarket.OracleProvider,
			oracletypes.NewProviderPriceState(boMarket.OracleSymbol, price, ctx.BlockTime().Unix()),
		)
	}
	tp.SuccessActions[params.ActionType]++
}

func (tp *TestPlayer) replayLiquidatePositionAction(
	params *ActionLiquidatePosition,
) (*exchangetypes.MsgLiquidatePositionResponse, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
	marketType := tp.GetDefaultMarketType(params.MarketType)
	marketID := tp.FindMarketId(marketType, params.MarketIndex)
	subaccountID := common.HexToHash(accounts[params.AccountIndex].SubaccountIDs[0])

	liquidatorAccIdx := params.LiquidatorAccountIndex
	if liquidatorAccIdx == nil {
		liquidatorAccIdx = &params.AccountIndex
	}
	liquidatorID := common.HexToHash(accounts[*liquidatorAccIdx].SubaccountIDs[0])
	liquidateMsg := tp.TestInput.NewMsgLiquidatePositionForMarketID(
		subaccountID,
		liquidatorID,
		marketID,
	)
	if resp, err := msgServer.LiquidatePosition(sdk.WrapSDKContext(ctxCached), liquidateMsg); err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
		return resp, nil
	} else {
		tp.FailedActions[params.ActionType]++
		return nil, err
	}
}

func (tp *TestPlayer) PerformEndBlockerAction(
	blockInterval int,
	skipBeginBlock bool,
) {
	ctx := tp.Ctx
	app := tp.App
	avgBlockInterval := 1
	// funding interval is 3600s and when this action is selected for 36th, funding interval time comes
	interval := time.Second * time.Duration(avgBlockInterval+blockInterval)
	blockTime := ctx.BlockTime().Add(interval)

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	ctx = ctx.WithBlockTime(blockTime)
	ctx, _ = EndBlockerAndCommit(app, ctx)
	if !skipBeginBlock {
		oracle.NewBlockHandler(app.OracleKeeper).BeginBlocker(ctx)
		exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
		wasmx.NewBlockHandler(app.WasmxKeeper).BeginBlocker(ctx, types.RequestBeginBlock{})
		//request isn't used
	}
	tp.Ctx = ctx
}

func (tp *TestPlayer) replayEndBlockerAction(params *ActionEndBlocker) {
	tp.PerformEndBlockerAction(params.BlockInterval, params.SkipBeginBlock)
	tp.SuccessActions[params.ActionType]++
}

func (tp *TestPlayer) replayBeginBlockerAction(params *ActionBeginBlocker) {
	ctx := tp.Ctx
	app := tp.App

	exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	wasmx.NewBlockHandler(app.WasmxKeeper).
		BeginBlocker(ctx, types.RequestBeginBlock{})
		//request isn't used
	tp.SuccessActions[params.ActionType]++
}

func (tp *TestPlayer) replayCreatePosition(params *ActionCreatePosition) error {
	ctx := tp.Ctx
	app := tp.App
	entryPrice := params.EntryPrice
	var marketType MarketType
	if params.MarketType != nil {
		marketType = *params.MarketType
	} else {
		switch params.ActionType {
		case ActionType_positionDerivative:
			marketType = MarketType_derivative
		case ActionType_positionBinary:
			marketType = MarketType_binary
		}
	}
	marketId := tp.FindMarketId(marketType, params.MarketIndex)
	if entryPrice == 0 {
		if marketType != MarketType_spot {
			if market, markPrice := app.ExchangeKeeper.GetDerivativeOrBinaryOptionsMarketWithMarkPrice(ctx, marketId, true); market != nil {
				entryPrice = markPrice.MustFloat64()
			}
		} else {
			entryPrice = 1000 // doesn't matter for spot
		}
	}
	var createActionType ActionType
	switch params.ActionType {
	case ActionType_positionDerivative:
		createActionType = ActionType_derivativeLimitOrder
	case ActionType_positionBinary:
		createActionType = ActionType_boLimitOrder
	}

	side1 := ActionCreateOrder{
		TestActionBase: TestActionBase{
			ActionType: createActionType,
		},
		TestActionWithMarketAndAccount: TestActionWithMarketAndAccount{
			TestActionWithMarket: params.TestActionWithMarket,
			TestActionWithAccount: TestActionWithAccount{
				AccountIndex: params.LongAccountIndex,
			},
		},
		Price:    entryPrice,
		Quantity: params.Quantity,
		IsLong:   true,
		Leverage: params.LeverageLong,
		Margin:   params.MarginLong,
	}
	side2 := ActionCreateOrder{
		TestActionBase: TestActionBase{
			ActionType: createActionType,
		},
		TestActionWithMarketAndAccount: TestActionWithMarketAndAccount{
			TestActionWithMarket: params.TestActionWithMarket,
			TestActionWithAccount: TestActionWithAccount{
				AccountIndex: params.ShortAccountIndex,
			},
		},
		Price:    entryPrice,
		Quantity: params.Quantity,
		IsLong:   false,
		Leverage: params.LeverageShort,
		Margin:   params.MarginShort,
	}
	var err error = nil
	switch params.ActionType {
	case ActionType_positionDerivative:
		_, err = tp.ReplayCreateDerivativeOrderAction(&side1, false)
		if err == nil || !tp.stopOnError {
			_, err = tp.ReplayCreateDerivativeOrderAction(&side2, false)
		}
	case ActionType_positionBinary:
		_, err = tp.replayCreateBinaryOptionsOrderAction(&side1)
		if err == nil || !tp.stopOnError {
			_, err = tp.replayCreateBinaryOptionsOrderAction(&side2)
		}
	}

	if err == nil || !tp.stopOnError {
		endBlocker := ActionEndBlocker{
			TestActionBase: TestActionBase{
				ActionType: ActionType_endblocker,
			},
		}
		tp.replayEndBlockerAction(&endBlocker)
	}
	return err
}

func (tp *TestPlayer) replayUpdateMarketProposal(params *ActionUpdateMarketParams) error {
	ctx := tp.Ctx
	app := tp.App
	marketType := tp.GetDefaultMarketType(params.MarketType)
	marketID := tp.FindMarketId(marketType, params.MarketIndex)

	makerFeeRate := fn2d(params.MakerFeeRate)
	takerFeeRate := fn2d(params.TakerFeeRate)

	relayerFeeShareRate := fn2d(params.RelayerFeeShareRate)
	minPriceTickSize := fn2d(params.MinPriceTickSize)
	ninQuantityTickSize := fn2d(params.MinQuantityTickSize)

	status := exchangetypes.MarketStatus_Unspecified
	if params.MarketStatus != nil {
		status = *params.MarketStatus
	}

	var proposal govtypes.Content
	switch marketType {
	case MarketType_spot:
		proposal = &exchangetypes.SpotMarketParamUpdateProposal{
			Title:               "Spot market param update",
			Description:         "Spot market param update",
			MarketId:            marketID.String(),
			MakerFeeRate:        makerFeeRate,
			TakerFeeRate:        takerFeeRate,
			RelayerFeeShareRate: relayerFeeShareRate,
			MinPriceTickSize:    minPriceTickSize,
			MinQuantityTickSize: ninQuantityTickSize,
			Status:              status,
		}
	case MarketType_derivative:
		proposal = &exchangetypes.DerivativeMarketParamUpdateProposal{
			Title:                  "Derivative market param update",
			Description:            "Derivative market param update",
			MarketId:               marketID.String(),
			InitialMarginRatio:     fn2d(params.InitialMarginRatio),
			MaintenanceMarginRatio: fn2d(params.MaintenanceMarginRatio),
			MakerFeeRate:           makerFeeRate,
			TakerFeeRate:           takerFeeRate,
			RelayerFeeShareRate:    relayerFeeShareRate,
			MinPriceTickSize:       minPriceTickSize,
			MinQuantityTickSize:    ninQuantityTickSize,
			HourlyInterestRate:     fn2d(params.HourlyInterestRate),
			HourlyFundingRateCap:   fn2d(params.HourlyFundingRateCap),
			Status:                 status,
		}
	case MarketType_binary:
		proposal = &exchangetypes.BinaryOptionsMarketParamUpdateProposal{
			Title:               "Derivative market param update",
			Description:         "Derivative market param update",
			MarketId:            marketID.String(),
			MakerFeeRate:        makerFeeRate,
			TakerFeeRate:        takerFeeRate,
			RelayerFeeShareRate: relayerFeeShareRate,
			MinPriceTickSize:    minPriceTickSize,
			MinQuantityTickSize: ninQuantityTickSize,
			ExpirationTimestamp: params.ExpirationTimestamp,
			SettlementTimestamp: params.SettlementTimestamp,
			SettlementPrice:     fn2d(params.SettlementPrice),
			Admin:               params.Admin,
			Status:              status,
		}
	}
	handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
	ctxCached, writeCache := ctx.CacheContext()
	err := handler(ctxCached, proposal)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}
	return nil
}

func (tp *TestPlayer) buildDepositMsg(params *ActionDeposit) exchangetypes.MsgDeposit {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	srcAccIndex := params.AccountIndex
	address := accounts[srcAccIndex].AccAddress

	var amount cosmostypes.Coin
	denom, err := tp.extractSpecialValue(params.Denom)
	OrFail(err)

	if params.Amount != nil {
		amount = cosmostypes.Coin{
			Denom:  denom,
			Amount: cosmostypes.NewInt(int64(*params.Amount)),
		}
	}

	if params.ToHave != nil {
		balancesDenom := app.ExchangeKeeper.GetDeposit(
			ctx,
			common.HexToHash(accounts[srcAccIndex].SubaccountIDs[0]),
			denom,
		)
		toDeposit := f2d(*params.ToHave).Sub(balancesDenom.AvailableBalance)
		amount = cosmostypes.Coin{
			Denom:  denom,
			Amount: toDeposit.TruncateInt(),
		}
	}

	if params.Amount == nil && params.ToHave == nil {
		panic("Either 'amount' or 'toHave' has to be set for action 'deposit'")
	}

	return exchangetypes.MsgDeposit{
		Sender:       address.String(),
		SubaccountId: accounts[srcAccIndex].SubaccountIDs[0],
		Amount:       amount,
	}
}

func (tp *TestPlayer) replayDeposit(
	params *ActionDeposit,
) (*exchangetypes.MsgDepositResponse, error) {
	ctx := tp.Ctx
	app := tp.App
	msg := tp.buildDepositMsg(params)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	ctxCached, writeCache := ctx.CacheContext()
	msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
	if resp, err := msgServer.Deposit(sdk.WrapSDKContext(ctxCached), &msg); err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
		return resp, nil
	} else {
		tp.FailedActions[params.ActionType]++
		return nil, err
	}
}

func (tp *TestPlayer) buildWithdrawalMsg(params *ActionWithdrawal) exchangetypes.MsgWithdraw {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	var amount cosmostypes.Coin

	if params.Amount != nil {
		amount = cosmostypes.Coin{
			Denom:  params.Denom,
			Amount: cosmostypes.NewInt(int64(*params.Amount)),
		}
	}

	if params.ToLeave != nil {
		balancesQuote := app.ExchangeKeeper.GetDeposit(
			ctx,
			common.HexToHash(accounts[accountIndex].SubaccountIDs[0]),
			params.Denom,
		)
		toWithdraw := balancesQuote.AvailableBalance.Sub(f2d(*params.ToLeave))
		amount = cosmostypes.Coin{
			Denom:  params.Denom,
			Amount: toWithdraw.TruncateInt(),
		}
	}

	if params.Amount == nil && params.ToLeave == nil {
		panic("Either 'amount' or 'toLeave' has to be set for action 'withdrawal'")
	}

	return exchangetypes.MsgWithdraw{
		Sender:       accounts[accountIndex].AccAddress.String(),
		SubaccountId: common.HexToHash(accounts[accountIndex].SubaccountIDs[0]).String(),
		Amount:       amount,
	}
}

func (tp *TestPlayer) replayWithdrawal(
	params *ActionWithdrawal,
) (*exchangetypes.MsgWithdrawResponse, error) {
	msg := tp.buildWithdrawalMsg(params)

	msgServer := keeper.AccountsMsgServerImpl(tp.App.ExchangeKeeper)

	resp, err := msgServer.Withdraw(sdk.WrapSDKContext(tp.Ctx), &msg)
	OrFail(err)

	if err == nil {
		tp.SuccessActions[params.ActionType]++
		return resp, nil
	} else {
		tp.FailedActions[params.ActionType]++
		return nil, err
	}
}

func (tp *TestPlayer) replaySend(params *ActionSend) (*cosmostypes.Result, error) {
	ctx := tp.Ctx
	app := tp.App

	msg := tp.buildSendMsg(params)
	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}

	handler := app.MsgServiceRouter().Handler(&msg)
	resp, err := handler(ctx, &msg)

	if err == nil {
		tp.SuccessActions[params.ActionType]++
		return resp, nil
	} else {
		tp.FailedActions[params.ActionType]++
		return nil, err
	}
}

func (tp *TestPlayer) buildSendMsg(params *ActionSend) banktypes.MsgSend {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	var amount cosmostypes.Coin
	denom, err := tp.extractSpecialValue(params.Denom)
	OrFail(err)

	if params.Amount != nil {
		amount = cosmostypes.Coin{
			Denom:  denom,
			Amount: cosmostypes.NewInt(int64(*params.Amount)),
		}
	}

	if params.ToLeave != nil {
		balance := app.BankKeeper.GetBalance(ctx, accounts[accountIndex].AccAddress, denom)
		toWithdraw := balance.Amount.Sub(sdk.NewInt(*params.ToLeave))
		amount = cosmostypes.Coin{
			Denom:  denom,
			Amount: toWithdraw,
		}
	}

	if params.Amount == nil && params.ToLeave == nil {
		panic("Either 'amount' or 'toLeave' has to be set for action 'send'")
	}

	return banktypes.MsgSend{
		FromAddress: accounts[params.AccountIndex].AccAddress.String(),
		ToAddress:   accounts[params.RecipientIndex].AccAddress.String(),
		Amount:      sdk.Coins{amount},
	}
}

func (tp *TestPlayer) replayRemoveFunds(params *ActionWithdrawal) error {
	accounts := *tp.Accounts
	denom := params.Denom
	accountIndex := params.AccountIndex
	subaccount := common.HexToHash(accounts[accountIndex].SubaccountIDs[params.SubaccountIndex])
	var amount cosmostypes.Coin
	if params.Amount != nil {
		amount = cosmostypes.Coin{
			Denom:  denom,
			Amount: f2d(*params.Amount).TruncateInt(),
		}
	}

	if params.ToLeave != nil {
		balance := GetBankAndDepositFunds(tp.App, tp.Ctx, subaccount, denom)
		toWithdraw := balance.AvailableBalance.Sub(f2d(*params.ToLeave))
		amount = cosmostypes.Coin{
			Denom:  denom,
			Amount: toWithdraw.TruncateInt(),
		}
	}
	var err error
	if exchangetypes.IsDefaultSubaccountID(subaccount) {
		err = tp.App.BankKeeper.SendCoins(
			tp.Ctx,
			accounts[accountIndex].AccAddress,
			DefaultWithdrawalAddress,
			sdk.NewCoins(amount),
		)
	} else {
		msg := tp.buildWithdrawalMsg(params)
		msgServer := keeper.AccountsMsgServerImpl(tp.App.ExchangeKeeper)
		_, err = msgServer.Withdraw(sdk.WrapSDKContext(tp.Ctx), &msg)
	}
	return err
}

func (tp *TestPlayer) replaySendFunds(params *ActionSend) error {
	_, err := tp.replaySend(params)
	if err != nil {
		return err
	}

	accounts := *tp.Accounts
	denom := params.Denom
	accountIndex := params.AccountIndex
	subaccount := common.HexToHash(accounts[accountIndex].SubaccountIDs[params.SubaccountIndex])

	if !exchangetypes.IsDefaultSubaccountID(subaccount) {
		var amount math.Int
		if params.Amount != nil {
			amount = sdk.NewInt(int64(*params.Amount))
		}

		if params.ToLeave != nil {
			balance := tp.App.BankKeeper.GetBalance(
				tp.Ctx,
				accounts[accountIndex].AccAddress,
				denom,
			)
			amount = balance.Amount.Sub(sdk.NewInt(*params.ToLeave))
		}

		amountF64, _ := amount.ToDec().Float64()
		depositParams := &ActionDeposit{
			TestActionBase: params.TestActionBase,
			TestActionWithAccount: TestActionWithAccount{
				AccountIndex:    params.RecipientIndex,
				SubaccountIndex: params.TestActionWithAccount.SubaccountIndex,
			},
			Amount: &amountF64,
			Denom:  denom,
		}

		_, err = tp.replayDeposit(depositParams)
		return err
	}

	return err
}

func (tp *TestPlayer) replayForcedMarketSettlementProposal(
	params *ActionForcedMarketSettlementParams,
) error {
	ctx := tp.Ctx
	app := tp.App
	marketType := tp.GetDefaultMarketType(params.MarketType)
	marketIndex := 0
	if params.MarketIndex != nil {
		marketIndex = *params.MarketIndex
	}
	marketID := tp.FindMarketId(marketType, marketIndex)

	title := "Forced market settlement proposal"
	description := "Forced market settlement proposal"

	if params.Title != nil {
		title = *params.Title
	}

	if params.Description != nil {
		description = *params.Description
	}

	var price *github_com_cosmos_cosmos_sdk_types.Dec

	if params.SettlementPrice != nil {
		p := github_com_cosmos_cosmos_sdk_types.MustNewDecFromStr(
			fmt.Sprintf("%v", *params.SettlementPrice),
		)
		price = &p
	}

	proposal := &exchangetypes.MarketForcedSettlementProposal{
		Title:           title,
		Description:     description,
		MarketId:        marketID.String(),
		SettlementPrice: price,
	}

	handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
	ctxCached, writeCache := ctx.CacheContext()
	err := handler(ctxCached, proposal)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}
	return nil
}

func (tp *TestPlayer) replayPerpetualMarketFunding(params *ActionPerpetualMarketFundingParams) {
	ctx := tp.Ctx
	app := tp.App
	marketType := tp.GetDefaultMarketType(params.MarketType)
	if marketType == MarketType_spot {
		panic("Cannot set perpetual market funding for spot market")
	}

	marketIndex := 0
	if params.MarketIndex != nil {
		marketIndex = *params.MarketIndex
	}
	marketID := tp.FindMarketId(marketType, marketIndex)

	funding := exchangetypes.PerpetualMarketFunding{
		CumulativeFunding: github_com_cosmos_cosmos_sdk_types.MustNewDecFromStr(
			fmt.Sprintf("%v", params.CumulativeFunding),
		),
		CumulativePrice: github_com_cosmos_cosmos_sdk_types.MustNewDecFromStr(
			fmt.Sprintf("%v", params.CumulativePrice),
		),
		LastTimestamp: ctx.BlockTime().Unix(),
	}
	app.ExchangeKeeper.SetPerpetualMarketFunding(ctx, marketID, &funding)

	ctxCached, writeCache := ctx.CacheContext()
	writeCache()
	tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
	tp.SuccessActions[params.ActionType]++
}

func (tp *TestPlayer) replayLaunchMarket(params *ActionLaunchMarket) error {
	switch params.MarketType {
	case "spot":
		minPriceTickSize := sdk.NewDecWithPrec(1, 4)
		if params.MinPriceTickSize != nil {
			minPriceTickSize = sdk.MustNewDecFromStr(*params.MinPriceTickSize)
		}
		minQuantityTickSize := sdk.NewDecWithPrec(1, 4)
		if params.MinQuantityTickSize != nil {
			minQuantityTickSize = sdk.MustNewDecFromStr(*params.MinQuantityTickSize)
		}
		baseDenom, err := tp.extractSpecialValue(params.BaseDenom)
		OrFail(err)

		quoteDenom, err := tp.extractSpecialValue(params.QuoteDenom)
		OrFail(err)

		resp, err := tp.App.ExchangeKeeper.SpotMarketLaunch(
			tp.Ctx,
			params.Ticker,
			baseDenom,
			quoteDenom,
			minPriceTickSize,
			minQuantityTickSize,
		)
		OrFail(err)

		spotMarket := SpotMarket{
			Market: Market{
				BaseDenom:           resp.BaseDenom,
				QuoteDenom:          resp.QuoteDenom,
				Ticker:              resp.Ticker,
				MarketID:            common.HexToHash(resp.MarketId),
				MinPriceTickSize:    resp.MinPriceTickSize,
				MinQuantityTickSize: resp.MinQuantityTickSize,
				MakerFeeRate:        resp.MakerFeeRate,
				TakerFeeRate:        resp.TakerFeeRate,
				IsActive:            true,
			},
		}
		tp.TestInput.Spots = append(tp.TestInput.Spots, spotMarket)
	default:
		panic("unsupported market type")
	}

	return nil
}

func (tp *TestPlayer) buildUnderwriteMsg(params *ActionUnderwrite) insurancetypes.MsgUnderwrite {
	marketType := tp.GetDefaultMarketType(nil)
	marketID := tp.FindMarketId(marketType, params.MarketIndex)

	return insurancetypes.MsgUnderwrite{
		Sender:   (*tp.Accounts)[params.AccountIndex].AccAddress.String(),
		MarketId: marketID.Hex(),
		Deposit:  params.Deposit.toSdkCoin(),
	}
}

func (tp *TestPlayer) replayUnderwrite(params *ActionUnderwrite) error {
	msg := tp.buildUnderwriteMsg(params)

	ctxCached, writeCache := tp.Ctx.CacheContext()
	msgServer := insurancekeeper.NewMsgServerImpl(tp.App.InsuranceKeeper)

	_, err := msgServer.Underwrite(sdk.WrapSDKContext(ctxCached), &msg)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}

	return nil
}

func (tp *TestPlayer) buildSendToEthMsg(params *ActionSendToEth) peggytypes.MsgSendToEth {
	return peggytypes.MsgSendToEth{
		Sender:    (*tp.Accounts)[params.AccountIndex].AccAddress.String(),
		EthDest:   params.RecipientEthAddress,
		Amount:    params.Amount.toSdkCoin(),
		BridgeFee: params.BridgeFee.toSdkCoin(),
	}
}

func (tp *TestPlayer) replaySendToEth(params *ActionSendToEth) error {
	msg := tp.buildSendToEthMsg(params)

	ctxCached, writeCache := tp.Ctx.CacheContext()
	msgServer := peggykeeper.NewMsgServerImpl(tp.App.PeggyKeeper)

	_, err := msgServer.SendToEth(sdk.WrapSDKContext(ctxCached), &msg)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}

	return nil
}

func (tp *TestPlayer) replayPeggyDepositClaim(params *ActionPeggyDepositClaim) error {
	msg := tp.buildPeggyDepositClaimMsg(params)

	ctxCached, writeCache := tp.Ctx.CacheContext()
	handler := peggy.NewHandler(tp.App.PeggyKeeper)

	_, err := handler(ctxCached, &msg)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}

	return nil
}

func jsonStringToAny[MsgType proto.Message](arbitraryData string, m MsgType) *codectypes.Any {
	var message MsgType
	err := json.Unmarshal([]byte(arbitraryData), &message)
	OrFail(err)

	any, err := codectypes.NewAnyWithValue(message)
	OrFail(err)

	return any
}

func (tp *TestPlayer) buildPeggyDepositClaimMsg(
	params *ActionPeggyDepositClaim,
) peggytypes.MsgDepositClaim {
	blockHeight := tp.App.PeggyKeeper.GetLastObservedEthereumBlockHeight(tp.Ctx).EthereumBlockHeight
	if params.BlockHeight != 0 {
		blockHeight = params.BlockHeight
	}

	eventNonce := tp.App.PeggyKeeper.GetLastObservedEventNonce(tp.Ctx)
	if params.Nonce != 0 {
		eventNonce = params.Nonce
	}

	// 0 is not a valid nonce
	if eventNonce == 0 {
		eventNonce++
	}

	tokenContract := "0xAE78bfF1d33023c12Fec7bEC26512fCE9c7c1ea1" //random Ethereum address
	if params.TokenContract != nil {
		tokenContract = *params.TokenContract
	}

	var ethereumSender string
	if params.EthereumSender != nil {
		ethereumSender = *params.EthereumSender
	} else {
		ethereumSender = common.BytesToAddress((*tp.Accounts)[params.AccountIndex].AccAddress.Bytes()).String()
	}

	var cosmosReceiver string
	if params.CosmosReceiver != nil {
		cosmosReceiver = *params.CosmosReceiver
	} else {
		cosmosReceiver = (*tp.Accounts)[params.AccountIndex].AccAddress.String()
	}

	var orchestrator string
	if params.Orchestrator != nil {
		orchestrator = *params.Orchestrator
	} else {
		// try to use existing orchestrator, if exists otherwise create and register a new one
		orchestrators := tp.App.PeggyKeeper.GetOrchestratorAddresses(tp.Ctx)
		if len(orchestrators) == 0 {
			valPub := cryptokeys.GenPrivKey().PubKey()
			valAddr := sdk.ValAddress(valPub.Address())
			selfBond := sdk.NewCoins(sdk.Coin{Amount: sdk.NewInt(100), Denom: "inj"})

			err := tp.App.BankKeeper.MintCoins(tp.Ctx, minttypes.ModuleName, selfBond)
			OrFail(err)
			err = tp.App.BankKeeper.SendCoinsFromModuleToAccount(
				tp.Ctx,
				minttypes.ModuleName,
				sdk.AccAddress(valAddr),
				selfBond,
			)
			OrFail(err)

			msgServer := stakingkeeper.NewMsgServerImpl(tp.App.StakingKeeper)

			coin := sdk.NewCoin("inj", selfBond[0].Amount)
			msg, err := stakingtypes.NewMsgCreateValidator(
				valAddr,
				valPub,
				coin,
				stakingtypes.Description{},
				stakingtypes.NewCommissionRates(
					math.LegacyOneDec(),
					math.LegacyOneDec(),
					math.LegacyZeroDec(),
				),
				math.OneInt(),
			)
			OrFail(err)
			_, err = msgServer.CreateValidator(sdk.WrapSDKContext(tp.Ctx), msg)
			OrFail(err)

			var bondedValAddress sdk.ValAddress

			// to be honest I don't know why my validator is not bounded after creation
			// and why bonding it manually results in staking invariant violation
			// so I'm just iterating over all validators and taking the first one that is bonded
			for _, val := range tp.App.StakingKeeper.GetAllValidators(tp.Ctx) {
				if val.GetStatus() == stakingtypes.Bonded {
					bondedValAddress = val.GetOperator()
				}
			}

			if bondedValAddress == nil {
				panic("no bonded validators found")
			}

			_, found := tp.App.StakingKeeper.GetValidator(tp.Ctx, bondedValAddress)
			if !found {
				panic("validator not found in staking keeper after creation. this should not happen")
			}

			orchestrator = sdk.AccAddress(bondedValAddress).String()
			tp.App.PeggyKeeper.SetOrchestratorValidator(tp.Ctx, bondedValAddress, sdk.MustAccAddressFromBech32(orchestrator))
		} else {
			orchestrator = orchestrators[0].Orchestrator
		}
	}

	orchestratorAddr, _ := sdk.AccAddressFromBech32(orchestrator)
	validator, found := tp.App.PeggyKeeper.GetOrchestratorValidator(tp.Ctx, orchestratorAddr)
	if !found {
		panic("peggy orchestrator has no validator")
	}

	valid := tp.App.StakingKeeper.Validator(tp.Ctx, validator)
	if valid == nil {
		panic("validator is not present in validator set")
	}

	data := ""
	if params.ArbitraryData != nil {
		var any *codectypes.Any
		if params.ArbitraryDataType == nil {
			panic(
				"arbitrary data type must be specified if arbitrary data is specified, e.g. 'exchangetypes.MsgDeposit'",
			)
		}

		dataToUse := *params.ArbitraryData

		marketId, err := tp.extractSpecialValue(*params.ArbitraryData)
		OrFail(err)
		//substitution has to be done manually
		regex := regexp.MustCompile(`\$market.spot\[\d+\]\.id`)
		dataToUse = regex.ReplaceAllString(dataToUse, marketId)

		switch *params.ArbitraryDataType {
		case "exchangetypes.MsgDeposit":
			any = jsonStringToAny(dataToUse, &exchangetypes.MsgDeposit{})
		case "exchangetypes.MsgCreateSpotMarketOrder":
			any = jsonStringToAny(dataToUse, &exchangetypes.MsgCreateSpotMarketOrder{})
		case "exchangetypes.MsgCreateSpotLimitOrder":
			any = jsonStringToAny(dataToUse, &exchangetypes.MsgCreateSpotLimitOrder{})
		case "blob":
		default:
			panic("unsupported arbitrary data type")
		}

		if *params.ArbitraryDataType != "blob" {
			jm := &jsonpb.Marshaler{}
			var err error
			data, err = jm.MarshalToString(any)
			OrFail(err)
		} else {
			data = *params.ArbitraryData
		}
	}

	return peggytypes.MsgDepositClaim{
		EventNonce:     eventNonce,
		BlockHeight:    blockHeight,
		TokenContract:  tokenContract,
		Amount:         sdk.NewIntFromUint64(params.Amount),
		EthereumSender: ethereumSender,
		CosmosReceiver: cosmosReceiver,
		Orchestrator:   orchestrator,
		Data:           data,
	}
}

func (tp *TestPlayer) replayPlaceBid(params *ActionPlaceBid) error {
	msg := tp.buildBidMessage(params)

	ctxCached, writeCache := tp.Ctx.CacheContext()
	msgServer := auctionkeeper.NewMsgServerImpl(tp.App.AuctionKeeper)

	_, err := msgServer.Bid(sdk.WrapSDKContext(ctxCached), &msg)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}

	return nil
}

func (tp *TestPlayer) buildBidMessage(params *ActionPlaceBid) auctiontypes.MsgBid {
	return auctiontypes.MsgBid{
		Sender:    (*tp.Accounts)[params.AccountIndex].AccAddress.String(),
		BidAmount: params.Deposit.toSdkCoin(),
		Round:     uint64(params.Round),
	}
}

func (tp *TestPlayer) replayDelegate(params *ActionDelegate) error {
	msg := tp.buildDelegateMessage(params)

	ctxCached, writeCache := tp.Ctx.CacheContext()
	msgServer := stakingkeeper.NewMsgServerImpl(tp.App.StakingKeeper)

	_, err := msgServer.Delegate(sdk.WrapSDKContext(ctxCached), &msg)

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctxCached.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}

	return nil
}

func (tp *TestPlayer) buildDelegateMessage(params *ActionDelegate) stakingtypes.MsgDelegate {
	validator, err := tp.extractSpecialValue(params.ValidatorAddress)
	OrFail(err)
	return stakingtypes.MsgDelegate{
		DelegatorAddress: (*tp.Accounts)[params.AccountIndex].AccAddress.String(),
		ValidatorAddress: validator,
		Amount:           params.Amount.toSdkCoin(),
	}
}

func (tp *TestPlayer) convertActionToMsg(wrappedAction TestAction) sdk.Msg {
	var msg sdk.Msg

	switch wrappedAction.GetActionType() {
	case ActionType_spotLimitOrder, ActionType_spotMarketOrder:
		sender := (*tp.Accounts)[wrappedAction.(HasAccountIndex).getAccountIndex()].AccAddress.String()
		spotOrder, err := tp.createSpotOrderFromJson(wrappedAction.(*ActionCreateOrder))
		OrFail(err)
		switch wrappedAction.GetActionType() {
		case ActionType_spotLimitOrder:
			msg = &exchangetypes.MsgCreateSpotLimitOrder{
				Sender: sender,
				Order:  *spotOrder,
			}

		case ActionType_spotMarketOrder:
			msg = &exchangetypes.MsgCreateSpotMarketOrder{
				Sender: sender,
				Order:  *spotOrder,
			}
		}
	case ActionType_send:
		tmp := tp.buildSendMsg(wrappedAction.(*ActionSend))
		msg = &tmp
	case ActionType_underwrite:
		tmp := tp.buildUnderwriteMsg(wrappedAction.(*ActionUnderwrite))
		msg = &tmp
	case ActionType_placeBid:
		tmp := tp.buildBidMessage(wrappedAction.(*ActionPlaceBid))
		msg = &tmp
	case ActionType_createDenom:
		tmp := tp.buildCreateDenomMsg(wrappedAction.(*ActionCreateDenomTokenFactory))
		msg = &tmp
	case ActionType_sendToEth:
		tmp := tp.buildSendToEthMsg(wrappedAction.(*ActionSendToEth))
		msg = &tmp
	case ActionType_deposit:
		tmp := tp.buildDepositMsg(wrappedAction.(*ActionDeposit))
		msg = &tmp
	case ActionType_withdrawal:
		tmp := tp.buildWithdrawalMsg(wrappedAction.(*ActionWithdrawal))
		msg = &tmp
	case ActionType_executeContract:
		action := wrappedAction.(*ActionExecuteContract)
		switch action.ExecutionType {
		case "wasm":
			tmp := tp.buildWasmExecuteContractMsg(wrappedAction.(*ActionExecuteContract))
			msg = &tmp
		case "injective":
			tmp := tp.buildInjectiveExecMsg(wrappedAction.(*ActionExecuteContract))
			msg = &tmp
		default:
			panic("unsupported execution type:" + action.ExecutionType)
		}
	default:
		panic("unsupported action type: " + wrappedAction.GetActionType())
	}

	return msg
}

func (tp *TestPlayer) parseWrappedAction(actionString interface{}) TestAction {
	var a TestActionBase
	asByte, err := json.Marshal(actionString)
	OrFail(err)
	err = injjson.Unmarshal(asByte, &a)
	OrFail(err)

	var i TestAction
	switch a.GetActionType() {
	case ActionType_spotLimitOrder, ActionType_spotMarketOrder,
		ActionType_derivativeLimitOrder, ActionType_derivativeMarketOrder,
		ActionType_boMarketOrder, ActionType_boLimitOrder:
		i = &ActionCreateOrder{}
	case ActionType_cancelOrder:
		i = &ActionCancelOrder{}
	case ActionType_liquidatePostion:
		i = &ActionLiquidatePosition{}
	case ActionType_batchUpdate:
		i = &ActionBatchOrderUpdate{}
	case ActionType_withdrawal:
		i = &ActionWithdrawal{}
	case ActionType_deposit:
		i = &ActionDeposit{}
	case ActionType_registerAndInitContract:
		i = &ActionRegisterAndInitializeContract{}
	case ActionType_executeContract:
		i = &ActionExecuteContract{}
	case ActionType_createDenom:
		i = &ActionCreateDenomTokenFactory{}
	case ActionType_send:
		i = &ActionSend{}
	case ActionType_underwrite:
		i = &ActionUnderwrite{}
	case ActionType_placeBid:
		i = &ActionPlaceBid{}
	case ActionType_sendToEth:
		i = &ActionSendToEth{}
	case ActionType_delegate:
		i = &ActionDelegate{}
	default:
		OrFail(fmt.Errorf("unsupported action type: %v", a.GetActionType()))
	}

	err = injjson.Unmarshal(asByte, i)
	OrFail(err)
	return i
}

// helpers
func (tp *TestPlayer) extractSpecialValue(value string) (string, error) {
	ptTokenFactoryIdx := regexp.MustCompile("\\$tf\\(([0-9]+)\\)")
	tfSubString := ptTokenFactoryIdx.FindStringSubmatch(value)
	if tfSubString != nil {
		tfIndex, err := strconv.Atoi(tfSubString[1])
		if err != nil {
			panic(err)
		}
		return (*tp.TokenFactoryDenoms)[tfIndex], nil
	}

	ptContractSubAddr := regexp.MustCompile(
		"\\$contractAddress\\(([A-z0-9_]+)\\)(\\.sub\\(([0-9])\\))?",
	)
	subStrContractAddr := ptContractSubAddr.FindStringSubmatch(value)
	if subStrContractAddr != nil {
		contractId := subStrContractAddr[1]
		contractAddress, ok := tp.ContractsById[contractId]

		if !ok {
			return "", fmt.Errorf("no contract address for id found: %v\n", contractId)
		}
		// if it matches "$contract(id)" skip the rest
		if subStrContractAddr[2] == "" || subStrContractAddr[3] == "" {
			return tp.ContractsById[contractId], nil
		}

		//subaccount index was provided
		subaccountIndex, err := strconv.Atoi(subStrContractAddr[3])
		if err != nil {
			return "", err
		}

		contractAddrAcc, err := sdk.AccAddressFromBech32(contractAddress)
		OrFail(err)
		subaccountIdHash, err := exchangetypes.SdkAddressWithNonceToSubaccountID(
			contractAddrAcc,
			uint32(subaccountIndex),
		)
		OrFail(err)
		subaccountId := subaccountIdHash.Hex()
		return subaccountId, nil
	}

	ptValidatorAddress := regexp.MustCompile("\\$validatorAddress")
	validatorAddressSubString := ptValidatorAddress.FindStringSubmatch(value)
	if validatorAddressSubString != nil {
		return DefaultValidatorAddress, nil
	}

	ptContractsCodeId := regexp.MustCompile("\\$contractCodeIdAddress\\((['a-zA-Z']+)\\)")
	subStrContractsCodeId := ptContractsCodeId.FindStringSubmatch(value)
	if subStrContractsCodeId != nil {
		codeId, ok := tp.codeIdByContractId[subStrContractsCodeId[1]]
		if !ok {
			return "", fmt.Errorf("No contract with id '%s' was found", subStrContractsCodeId[1])
		}
		return strconv.Itoa(codeId), nil
	}

	ptSubaccountId := regexp.MustCompile(
		"\\$account\\[([A-z0-9_])\\]\\.sub\\[([0-9])\\]",
	)
	subStrSubaccountId := ptSubaccountId.FindStringSubmatch(value)
	if subStrSubaccountId != nil {
		accountId := subStrSubaccountId[1]
		accountIdInt, err := strconv.Atoi(accountId)
		if err != nil {
			return "", err
		}

		if accountIdInt+1 > len(*tp.Accounts) {
			return "", fmt.Errorf("account with index %v does not exist", accountIdInt)
		}

		subaccountNonce := subStrSubaccountId[2]
		subaccountNonceInt, err := strconv.Atoi(subaccountNonce)
		OrFail(err)

		if subaccountNonceInt+1 > len((*tp.Accounts)[accountIdInt].SubaccountIDs) {
			return "", fmt.Errorf("subaccount with index %v does not exist", subaccountNonceInt)
		}

		subaccountId := (*tp.Accounts)[accountIdInt].SubaccountIDs[subaccountNonceInt]

		return subaccountId, nil
	}

	ptMarket := regexp.MustCompile(
		"\\$market\\.([A-z]+)\\[([0-9])\\]\\.([A-z]+)",
	)

	subStrMarket := ptMarket.FindStringSubmatch(value)
	if subStrMarket != nil {
		marketType := subStrMarket[1]

		if marketType != "spot" {
			return "", fmt.Errorf("unsupported market type: %v", marketType)
		}

		marketIdx := subStrMarket[2]
		marketIdxInt, err := strconv.Atoi(marketIdx)
		if err != nil {
			return "", err
		}

		if marketIdxInt+1 > len(tp.TestInput.Spots) {
			return "", fmt.Errorf("spot market with index %v does not exist", marketIdxInt)
		}

		property := subStrMarket[3]
		if property != "id" {
			return "", fmt.Errorf("unsupported market property: %v", property)
		}

		return tp.TestInput.Spots[marketIdxInt].Market.MarketID.Hex(), nil
	}

	return value, nil
}

func (tp *TestPlayer) PerformMintAction(
	amount sdk.Coin,
	targetAccount sdk.AccAddress,
	targetSubaccount *string,
) error {
	// set balance for all the Accounts
	ctx := tp.Ctx
	app := tp.App
	err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(amount))
	if err != nil {
		return err
	}
	address := targetAccount
	err = app.BankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		minttypes.ModuleName,
		address,
		sdk.NewCoins(amount),
	)
	if err != nil {
		return err
	}

	if targetSubaccount != nil {
		subaccountId := common.HexToHash(*targetSubaccount)
		err = tp.App.ExchangeKeeper.ExecuteWithdraw(tp.Ctx, &exchangetypes.MsgWithdraw{
			Sender:       exchangetypes.SubaccountIDToSdkAddress(subaccountId).String(),
			SubaccountId: subaccountId.Hex(),
			Amount:       amount,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (tp *TestPlayer) ReplayMint(params *ActionMintCoinParams) error {
	coin := params.Amount.toSdkCoinWithSubstitution(tp, &params.TestActionWithMarketAndAccount)
	var subaccountId *string
	if params.ToSubaccount {
		subaccountId = &(*tp.Accounts)[params.AccountIndex].SubaccountIDs[0]
	}
	return tp.PerformMintAction(coin, (*tp.Accounts)[params.AccountIndex].AccAddress, subaccountId)
}
