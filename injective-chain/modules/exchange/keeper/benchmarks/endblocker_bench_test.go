package keeper_bench

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func BenchmarkEndblocker(b *testing.B) {
	var err error

	state := prepareEndblockerState(100000)
	for i := 0; i < b.N; i++ {
		isLong := i%2 == 0
		state.singleTrade(state, isLong)
		state.commitBlock()
	}

	b.ResetTimer()
	b.ReportAllocs()

	_ = err
}

const batchStepSize int = 1

type benchEndblockerState struct {
	testInput testexchange.TestInput
	app       *simapp.InjectiveApp
	ctx       sdk.Context

	derivativeMarket *types.DerivativeMarket
	msgServer        types.MsgServer
	startingPrice    sdk.Dec
}

func (*benchEndblockerState) singleTrade(state *benchEndblockerState, isLong bool) {
	var (
		price     = sdk.NewDec(2010)
		quantity  = sdk.NewDec(1)
		margin    = sdk.NewDec(2010)
		batchSize = 1
	)

	state.addToLimitOrderbook(batchSize, margin, quantity, price, isLong)
}

func prepareEndblockerState(batchSteps int) *benchEndblockerState {
	state := &benchEndblockerState{
		startingPrice: sdk.NewDec(2000),
	}

	state.app = simapp.Setup(false)
	state.app.BeginBlock(abci.RequestBeginBlock{
		Header: tmproto.Header{
			Height:  state.app.LastBlockHeight() + 1,
			AppHash: state.app.LastCommitID().Hash,
		}})
	state.ctx = state.app.BaseApp.NewContext(false, tmproto.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	})
	state.testInput, state.ctx = testexchange.SetupTest(state.app, state.ctx, 0, 1, 0)
	state.msgServer = keeper.NewMsgServerImpl(state.app.ExchangeKeeper)

	oracleBase := state.testInput.Perps[0].OracleBase
	oracleQuote := state.testInput.Perps[0].OracleQuote
	oracleType := state.testInput.Perps[0].OracleType

	state.app.OracleKeeper.SetPriceFeedPriceState(
		state.ctx,
		oracleBase,
		oracleQuote,
		oracletypes.NewPriceState(
			state.startingPrice,
			state.ctx.BlockTime().Unix(),
		),
	)

	startingPrice := sdk.NewDec(2000)
	state.app.OracleKeeper.SetPriceFeedPriceState(state.ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, state.ctx.BlockTime().Unix()))

	sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
	initialInsuranceFundBalance := sdk.NewDec(44)
	coin := sdk.NewCoin(state.testInput.Perps[0].QuoteDenom, initialInsuranceFundBalance.RoundInt())
	state.app.BankKeeper.MintCoins(state.ctx, minttypes.ModuleName, sdk.NewCoins(coin))
	state.app.BankKeeper.SendCoinsFromModuleToAccount(state.ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
	testexchange.OrFail(state.app.InsuranceKeeper.CreateInsuranceFund(state.ctx, sender, coin, state.testInput.Perps[0].Ticker, state.testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

	_, _, err := state.app.ExchangeKeeper.PerpetualMarketLaunch(
		state.ctx,
		state.testInput.Perps[0].Ticker,
		state.testInput.Perps[0].QuoteDenom,
		oracleBase,
		oracleQuote,
		0,
		oracleType,
		state.testInput.Perps[0].InitialMarginRatio,
		state.testInput.Perps[0].MaintenanceMarginRatio,
		state.testInput.Perps[0].MakerFeeRate,
		state.testInput.Perps[0].TakerFeeRate,
		state.testInput.Perps[0].MinPriceTickSize,
		state.testInput.Perps[0].MinQuantityTickSize,
	)
	testexchange.OrFail(err)

	state.derivativeMarket = state.app.ExchangeKeeper.GetDerivativeMarket(
		state.ctx,
		state.testInput.Perps[0].MarketID,
		true,
	)

	var (
		price     = sdk.NewDec(2010)
		quantity  = sdk.NewDec(1)
		margin    = sdk.NewDec(2010)
		batchSize = batchSteps * batchStepSize
	)

	quoteDeposit := &types.Deposit{
		AvailableBalance: sdk.NewDec(20000000),
		TotalBalance:     sdk.NewDec(20000000),
	}

	for i := 1; i < batchSize+2; i++ {
		traderIndex := i
		traderIndexString := padLeft(traderIndex, 24)
		subaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c" + traderIndexString)
		testexchange.MintAndDeposit(state.app, state.ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(state.testInput.Perps[0].QuoteDenom, quoteDeposit.AvailableBalance.TruncateInt())))
	}

	state.addToLimitOrderbook(batchSize, margin, quantity, price, true)
	state.ctx, _ = testexchange.EndBlockerAndCommit(state.app, state.ctx)
	exchange.NewBlockHandler(state.app.ExchangeKeeper).BeginBlocker(state.ctx)

	return state
}

func (s *benchEndblockerState) addToLimitOrderbook(batchSize int, margin, quantity, price sdk.Dec, isLong bool) {
	var err error

	for i := 1; i < batchSize+1; i += 2 {
		buyTraderIndex := i
		if isLong {
			buyTraderIndex += 1
		}

		buyTraderIndexString := padLeft(buyTraderIndex, 24)
		buySubaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c" + buyTraderIndexString)

		limitBuyDerivativeOrder := s.testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_BUY, buySubaccountID)
		_, err = s.msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(s.ctx), limitBuyDerivativeOrder)

		testexchange.OrFail(err)

		sellTraderIndex := i + 1
		if isLong {
			sellTraderIndex -= 1
		}

		sellTraderIndexString := padLeft(sellTraderIndex, 24)
		sellSubaccountID := common.HexToHash("727aee334987c52fa7b567b2662bdbb68614e48c" + sellTraderIndexString)

		limitSellDerivativeOrder := s.testInput.NewMsgCreateDerivativeLimitOrder(price, quantity, margin, types.OrderType_SELL, sellSubaccountID)
		_, err = s.msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(s.ctx), limitSellDerivativeOrder)
		testexchange.OrFail(err)
	}
}

func (s *benchEndblockerState) commitBlock() (err error) {
	s.ctx, _ = testexchange.EndBlockerAndCommit(s.app, s.ctx)

	updatedTime := s.ctx.BlockTime().Add(time.Hour * 1)
	s.ctx = s.ctx.WithBlockTime(updatedTime)
	exchange.NewBlockHandler(s.app.ExchangeKeeper).BeginBlocker(s.ctx)

	return nil
}

func padLeft(v int, length int) string {
	abs := math.Abs(float64(v))
	var padding int
	if v != 0 {
		min := math.Pow10(length - 1)
		if min-abs > 0 {
			l := math.Log10(abs)
			if l == float64(int64(l)) {
				l++
			}
			padding = length - int(math.Ceil(l))
		}
	} else {
		padding = length - 1
	}
	builder := strings.Builder{}
	if v < 0 {
		length = length + 1
	}
	builder.Grow(length * 4)
	if v < 0 {
		builder.WriteRune('-')
	}
	for i := 0; i < padding; i++ {
		builder.WriteRune('0')
	}
	builder.WriteString(strconv.FormatInt(int64(abs), 10))
	return builder.String()
}
