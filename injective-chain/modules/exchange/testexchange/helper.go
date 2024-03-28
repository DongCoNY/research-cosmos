package testexchange

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"cosmossdk.io/math"
	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func NewDecFromFloat(f64Value float64) sdk.Dec {
	return sdk.MustNewDecFromStr(fmt.Sprintf("%f", f64Value))
}

func NewDerivativeLimitOrder(price, quantity, margin sdk.Dec, subaccountID common.Hash, feeRecipient string, orderHashString string, isBuy bool) types.DerivativeLimitOrder {
	orderType := types.OrderType_BUY

	if !isBuy {
		orderType = types.OrderType_SELL
	}

	orderHash := common.HexToHash(orderHashString)

	if orderHashString == "" {
		orderHash = common.HexToHash(strconv.Itoa(rand.Intn(1000000000000)))
	}

	order := types.DerivativeLimitOrder{
		OrderInfo: types.OrderInfo{
			SubaccountId: subaccountID.Hex(),
			FeeRecipient: feeRecipient,
			Price:        price,
			Quantity:     quantity,
		},
		OrderType:    orderType,
		Fillable:     quantity,
		Margin:       margin,
		TriggerPrice: nil,
		OrderHash:    orderHash.Bytes(),
	}
	return order
}

func NewSpotLimitOrder(price, quantity sdk.Dec, subaccountID, marketID common.Hash, isBuy bool) types.SpotLimitOrder {
	orderType := types.OrderType_BUY

	if !isBuy {
		orderType = types.OrderType_SELL
	}
	randomHash := common.HexToHash(strconv.Itoa(rand.Intn(1000000000000)))

	order := types.SpotLimitOrder{
		OrderInfo: types.OrderInfo{
			SubaccountId: subaccountID.Hex(),
			FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
			Price:        price,
			Quantity:     quantity,
		},
		OrderType:    orderType,
		Fillable:     quantity,
		TriggerPrice: nil,
		OrderHash:    randomHash.Bytes(),
	}
	return order
}

func PriceAndQuantityFromString(priceString string, quantityString string) (price sdk.Dec, quantity sdk.Dec) {
	price = sdk.MustNewDecFromStr(priceString)
	quantity = sdk.MustNewDecFromStr(quantityString)
	return price, quantity
}

func PriceQuantityAndMarginFromString(priceString, quantityString, marginString string) (price, quantity, margin sdk.Dec) {
	price = sdk.MustNewDecFromStr(priceString)
	quantity = sdk.MustNewDecFromStr(quantityString)
	margin = sdk.MustNewDecFromStr(marginString)
	return price, quantity, margin
}

func PrintOrderbook(buyOrderbook []*types.SpotLimitOrder, sellOrderbook []*types.SpotLimitOrder) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Buy Price", "Buy Quantity", "Sell Price", "Sell Quantity"})

	maxLength := len(buyOrderbook)
	if len(sellOrderbook) > maxLength {
		maxLength = len(sellOrderbook)
	}
	precision := 6

	for idx := 0; idx < maxLength; idx++ {
		row := make([]string, 0)
		if idx < len(buyOrderbook) {
			buyOrder := buyOrderbook[idx]
			row = append(row, buyOrder.OrderInfo.Price.String()[:precision])
			row = append(row, buyOrder.Fillable.String()[:precision])
		} else {
			row = append(row, "-", "-")
		}
		if idx < len(sellOrderbook) {
			sellOrder := sellOrderbook[idx]
			row = append(row, sellOrder.OrderInfo.Price.String()[:precision])
			row = append(row, sellOrder.Fillable.String()[:precision])
		} else {
			row = append(row, "-", "-")
		}
		table.Append(row)
	}
	table.Render()
}

func OrFail(err error) {
	if err != nil {
		Fail(err.Error(), 1)
	}
}

func ReturnOrFail(_ any, err error) {
	if err != nil {
		Fail(err.Error(), 1)
	}
}

func Grant(ctx sdk.Context, keeper authzkeeper.Keeper, granter, grantee string, grant interface{}) (*authz.MsgGrantResponse, error) {
	expiration := time.Now().Add(100 * time.Second)
	return keeper.Grant(sdk.WrapSDKContext(ctx), &authz.MsgGrant{
		Granter: granter,
		Grantee: grantee,
		Grant: authz.Grant{
			Authorization: cdctypes.UnsafePackAny(grant),
			Expiration:    &expiration,
		},
	})
}

func Revoke(ctx sdk.Context, keeper authzkeeper.Keeper, granter, grantee string, msg sdk.Msg) (*authz.MsgRevokeResponse, error) {
	return keeper.Revoke(sdk.WrapSDKContext(ctx), &authz.MsgRevoke{
		Granter:    granter,
		Grantee:    grantee,
		MsgTypeUrl: sdk.MsgTypeURL(msg),
	})
}

func Exec(ctx sdk.Context, keeper authzkeeper.Keeper, grantee string, msgs ...interface{}) (*authz.MsgExecResponse, error) {
	msgToExec := []*cdctypes.Any{}
	for _, m := range msgs {
		msgToExec = append(msgToExec, cdctypes.UnsafePackAny(m))
	}
	return keeper.Exec(sdk.WrapSDKContext(ctx), &authz.MsgExec{
		Grantee: grantee,
		Msgs:    msgToExec,
	})
}

func RandomUniqueNumber(exclude int, max int) int {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for {
		r := r1.Intn(max)
		if r != exclude {
			return r
		}
	}
}

// GetReadableSlice is a test utility function to return a readable represenation of any arbitrary slice, by applying formatter function to each slice element
func GetReadableSlice[T any](slice []T, sep string, formatter func(T) string) string {
	stringsArr := make([]string, len(slice))
	for i, t := range slice {
		stringsArr[i] = formatter(t)
	}
	return strings.Join(stringsArr, sep)
}

// reverseSlice will reverse slice contents (in place)
func reverseSlice[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func Count[T any](slice []T, predicate func(T) bool) int {
	var result = 0
	for _, v := range slice {
		if predicate(v) {
			result++
		}
	}
	return result
}

func FindFirst[T any](slice []*T, predicate func(*T) bool) *T {
	for _, v := range slice {
		if predicate(v) {
			return v
		}
	}
	return nil
}

func DerefSlice[T any](slice []*T) []T {
	newSlice := make([]T, len(slice))
	for i, t := range slice {
		newSlice[i] = *t
	}
	return newSlice
}

func MaxInt(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func VerifyDerivativeOrder(orders []*types.TrimmedDerivativeLimitOrder, orderIdx int, quantity float64, price float64, isReduceOnly bool) {
	Expect(orders[orderIdx].Price.TruncateInt().String()).To(Equal(NewDecFromFloat(price).TruncateInt().String()), fmt.Sprintf("Price for order %d", orderIdx))
	Expect(orders[orderIdx].Fillable.TruncateInt().String()).To(Equal(NewDecFromFloat(quantity).TruncateInt().String()), fmt.Sprintf("Quantity for order %d", orderIdx))
	if isReduceOnly {
		Expect(orders[orderIdx].IsReduceOnly()).To(BeTrue(), fmt.Sprintf("Order %d should be reduce-only", orderIdx))
	} else {
		Expect(orders[orderIdx].IsReduceOnly()).To(BeFalse(), fmt.Sprintf("Order %d should be vanilla", orderIdx))
	}
}

var VerifySellOrder = func(orders []*types.TrimmedDerivativeLimitOrder, orderIdx int, quantity float64, price float64, isReduceOnly bool) {
	Expect(orders[orderIdx].IsBuy).To(BeFalse(), fmt.Sprintf("Order %d should be a sell/short order", orderIdx))
	VerifyDerivativeOrder(orders, orderIdx, quantity, price, isReduceOnly)
}

var VerifyBuyOrder = func(orders []*types.TrimmedDerivativeLimitOrder, orderIdx int, quantity float64, price float64, isReduceOnly bool) {
	Expect(orders[orderIdx].IsBuy).To(BeTrue(), fmt.Sprintf("Order %d should be a buy/long order", orderIdx))
	VerifyDerivativeOrder(orders, orderIdx, quantity, price, isReduceOnly)
}

func VerifySpotOrder(orders []*types.TrimmedSpotLimitOrder, orderIdx int, quantity float64, fillable float64, price float64, isBuy bool) {
	Expect(orders[orderIdx].Price.TruncateInt().String()).To(Equal(NewDecFromFloat(price).TruncateInt().String()), fmt.Sprintf("Price for order %d", orderIdx))
	Expect(orders[orderIdx].Quantity.TruncateInt().String()).To(Equal(NewDecFromFloat(quantity).TruncateInt().String()), fmt.Sprintf("Quantity for order %d", orderIdx))
	Expect(orders[orderIdx].Fillable.TruncateInt().String()).To(Equal(NewDecFromFloat(fillable).TruncateInt().String()), fmt.Sprintf("Fillable quantity for order %d", orderIdx))
	if isBuy {
		Expect(orders[orderIdx].IsBuy).To(BeTrue(), fmt.Sprintf("Order %d should be a buy order", orderIdx))
	} else {
		Expect(orders[orderIdx].IsBuy).To(BeFalse(), fmt.Sprintf("Order %d should be a sell order", orderIdx))
	}
}

func GetAllLimitDerivativeOrdersSorted(app *app.InjectiveApp, ctx sdk.Context, subaccountId common.Hash, marketId common.Hash) []*types.DerivativeLimitOrder {
	var orders []*types.DerivativeLimitOrder = app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketID(ctx, marketId)

	ownOrders := make([]*types.DerivativeLimitOrder, 0)
	for idx := range orders {
		if (*orders[idx]).SubaccountID().String() == subaccountId.String() {
			ownOrders = append(ownOrders, orders[idx])
		}
	}

	sort.SliceStable(orders, func(i, j int) bool {
		if orders[i].OrderInfo.Price.Equal(orders[j].OrderInfo.Price) {
			if orders[i].OrderInfo.Quantity.Equal(orders[j].OrderInfo.Quantity) {
				return !orders[i].IsReduceOnly()
			}
			return orders[i].OrderInfo.Quantity.GT(orders[j].OrderInfo.Quantity)
		}
		if orders[i].IsBuy() {
			return orders[i].OrderInfo.Price.GT(orders[j].OrderInfo.Price)
		} else {
			return orders[i].OrderInfo.Price.LT(orders[j].OrderInfo.Price)
		}
	})
	return ownOrders
}

func GetAllDerivativeOrdersSorted(app *app.InjectiveApp, ctx sdk.Context, subaccountId common.Hash, marketId common.Hash) []*types.TrimmedDerivativeLimitOrder {
	orders := app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, subaccountId)
	sort.SliceStable(orders, func(i, j int) bool {
		if orders[i].Price.Equal(orders[j].Price) {
			if orders[i].Quantity.Equal(orders[j].Quantity) {
				return !orders[i].IsReduceOnly()
			}
			return orders[i].Quantity.GT(orders[j].Quantity)
		}
		if orders[i].IsBuy {
			return orders[i].Price.GT(orders[j].Price)
		} else {
			return orders[i].Price.LT(orders[j].Price)
		}
	})
	return orders
}

func GetAllSpotOrdersSorted(app *app.InjectiveApp, ctx sdk.Context, subaccountId common.Hash, marketId common.Hash) []*types.TrimmedSpotLimitOrder {
	orders := app.ExchangeKeeper.GetAllTraderSpotLimitOrders(ctx, marketId, subaccountId)
	sort.SliceStable(orders, func(i, j int) bool {
		if orders[i].Price.Equal(orders[j].Price) {
			return orders[i].Quantity.GT(orders[j].Quantity)
		}
		if orders[i].IsBuy {
			return orders[i].Price.GT(orders[j].Price)
		} else {
			return orders[i].Price.LT(orders[j].Price)
		}
	})
	return orders
}

func GetAllLimitSpotOrdersForMarket(app *app.InjectiveApp, ctx sdk.Context, subaccountId common.Hash, marketId common.Hash) []*types.SpotLimitOrder {
	orders := app.ExchangeKeeper.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, marketId, true, subaccountId)
	orders = append(orders, app.ExchangeKeeper.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, marketId, false, subaccountId)...)
	sort.SliceStable(orders, func(i, j int) bool {
		if orders[i].OrderInfo.Price.Equal(orders[j].OrderInfo.Price) {
			return orders[i].OrderInfo.Quantity.GT(orders[j].OrderInfo.Quantity)
		}
		if orders[i].IsBuy() {
			return orders[i].OrderInfo.Price.GT(orders[j].OrderInfo.Price)
		} else {
			return orders[i].OrderInfo.Price.LT(orders[j].OrderInfo.Price)
		}
	})
	return orders
}

func VerifyPosition(app *app.InjectiveApp, ctx sdk.Context, subaccountID common.Hash, marketId common.Hash, quantity int64, isLong bool) {
	position := app.ExchangeKeeper.GetPosition(ctx, marketId, subaccountID)
	if quantity == 0 {
		Expect(position).To(BeNil(), "Position should not exist")
	} else {
		Expect(position).To(Not(BeNil()), "Position should exist")
		Expect(position.Quantity.String()).To(Equal(sdk.NewDec(quantity).String()), "Position quantity was incorrect")
		Expect(position.IsLong).To(Equal(isLong), fmt.Sprintf("Position should be long: %v", isLong))
	}
}

func GetAvailableBalance(app *app.InjectiveApp, ctx sdk.Context, subaccountID common.Hash, denom string) sdk.Dec {
	balancesQuote := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, denom)
	return balancesQuote.AvailableBalance
}

func GetTakerFeeRateForSpotMarket(app *app.InjectiveApp, ctx sdk.Context, marketID common.Hash) sdk.Dec {
	market := app.ExchangeKeeper.GetSpotMarketByID(ctx, marketID)

	return market.TakerFeeRate
}

func GetTakerFeeRateForDerivativeMarket(app *app.InjectiveApp, ctx sdk.Context, marketID common.Hash) sdk.Dec {
	market := app.ExchangeKeeper.GetDerivativeMarketByID(ctx, marketID)

	return market.TakerFeeRate
}

func VerifyEqualDecs(tested, expected sdk.Dec, message string) {
	Expect(tested.String()).To(Equal(expected.String()), message)
}

func MustNotErr[V any](value V, err error) V {
	if err != nil {
		panic(err)
	} else {
		return value
	}
}

func MustOk[V any](value V, ok bool) V {
	if !ok {
		panic("Can't unwrap value")
	} else {
		return value
	}
}

type BalanceType string

const (
	BalanceType_total     BalanceType = "total"
	BalanceType_available BalanceType = "available"
)

func GetSubaccountDepositChange(tp *TestPlayer, accountIdx int, denom string, balanceType BalanceType, initialDeposits []map[string]*types.Deposit) sdk.Dec {
	balances := tp.App.ExchangeKeeper.GetDeposit(tp.Ctx, common.HexToHash((*tp.Accounts)[accountIdx].SubaccountIDs[0]), denom)
	balance := balances.TotalBalance
	oldDeposit, ok := initialDeposits[accountIdx][denom]
	if !ok {
		tmp := types.Deposit{
			AvailableBalance: f2d(0),
			TotalBalance:     f2d(0),
		}
		oldDeposit = &tmp
	}

	oldBalance := oldDeposit.TotalBalance
	if balanceType == BalanceType_available {
		balance = balances.AvailableBalance
		oldBalance = oldDeposit.AvailableBalance
	}

	balanceDelta := balance.Sub(oldBalance)
	return balanceDelta
}

func GetBankDepositChange(tp *TestPlayer, accountIdx int, denom string, initialBankBalance map[string]sdk.Coins) math.Int {
	address := (*tp.Accounts)[accountIdx].AccAddress
	newBalance := tp.App.BankKeeper.GetBalance(tp.Ctx, address, denom).Amount
	oldBalnaces, ok := initialBankBalance[address.String()]
	oldBalance := sdk.NewInt(0)

	if ok {
		for _, bal := range oldBalnaces {
			if bal.Denom == denom {
				oldBalance = bal.Amount
				break
			}
		}
	}

	balanceDelta := newBalance.Sub(oldBalance)
	fmt.Fprintf(GinkgoWriter, "old balance: %v | new balance: %v | delta: %v", oldBalance, newBalance, balanceDelta)

	return balanceDelta
}

type ChargeType string

const (
	ChargeType_Credit ChargeType = "credit"
	ChargeType_Debit  ChargeType = "debit"
)

func Decimal2IntBankChange(value sdk.Dec, chargeType ChargeType) (intValue math.Int, reminder sdk.Dec) {
	// round down on credit, round up on debit
	if chargeType == ChargeType_Credit {
		intValue = value.TruncateInt()
	} else {
		intValue = value.Abs().Ceil().TruncateInt()
	}
	reminder = value.Abs().Sub(intValue.ToDec()).Abs()

	if value.IsNegative() {
		intValue = intValue.Neg()
	}

	return intValue, reminder
}

func AdjustBankIntAndDustReminder(bankChange math.Int, dustReminder sdk.Dec) (newBankChange math.Int, newDustReminder sdk.Dec) {
	newBankChange = bankChange
	newDustReminder = dustReminder

	if newDustReminder.TruncateInt().GT(sdk.NewInt(0)) {
		newBankChange = bankChange.Add(newDustReminder.TruncateInt())
		newDustReminder = dustReminder.Sub(newDustReminder.TruncateInt().ToDec())
	}

	return newBankChange, newDustReminder
}
