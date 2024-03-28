package testexchange

import (
	"encoding/json"
	"regexp"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
)

type ActionType string

// nolint:all
const (
	ActionType_include                  ActionType = "include"
	ActionType_setPriceOracle           ActionType = "priceOracle"
	ActionType_positionDerivative       ActionType = "positionDerivative"
	ActionType_positionBinary           ActionType = "positionBinary"
	ActionType_spotLimitOrder           ActionType = "spotLimitOrder"
	ActionType_spotMarketOrder          ActionType = "spotMarketOrder"
	ActionType_derivativeLimitOrder     ActionType = "derivativeLimitOrder"
	ActionType_derivativeMarketOrder    ActionType = "derivativeMarketOrder"
	ActionType_expiryLimitOrder         ActionType = "expiryLimitOrder"
	ActionType_boLimitOrder             ActionType = "boLimitOrder"
	ActionType_boMarketOrder            ActionType = "boMarketOrder"
	ActionType_endblocker               ActionType = "endblocker"
	ActionType_beginblocker             ActionType = "beginblocker"
	ActionType_cancelOrder              ActionType = "cancelOrder"
	ActionType_liquidatePostion         ActionType = "liquidatePosition"
	ActionType_batchUpdate              ActionType = "batchUpdate"
	ActionType_updateMarket             ActionType = "updateMarket"
	ActionType_forcedSettlement         ActionType = "forcedSettlement"
	ActionType_deposit                  ActionType = "deposit"
	ActionType_withdrawal               ActionType = "withdrawal"
	ActionType_marketFunding            ActionType = "marketFunding"
	ActionType_registerAndInitContract  ActionType = "registerAndInitContract"
	ActionType_executeContract          ActionType = "executeContract"
	ActionType_createDenom              ActionType = "createTfDenom"
	ActionType_mintDenom                ActionType = "mintTfDenom"
	ActionType_burnDenom                ActionType = "burnTfDenom"
	ActionType_launchMarket             ActionType = "launchMarket"
	ActionType_send                     ActionType = "send"
	ActionType_batchDeregisterContracts ActionType = "deregisterContracts"
	ActionType_registerContracts        ActionType = "registerContracts"
	ActionType_storeContractCode        ActionType = "storeContractCode"
	ActionType_registerContractForBB    ActionType = "registerForBB"
	ActionType_registerVault            ActionType = "registerVault"
	ActionType_underwrite               ActionType = "underwrite"
	ActionType_placeBid                 ActionType = "placeBid"
	ActionType_sendToEth                ActionType = "sendToEth"
	ActionType_delegate                 ActionType = "delegate"
	ActionType_mint                     ActionType = "mintCoin"
	ActionType_removeFunds              ActionType = "removeFunds"
	ActionType_sendFunds                ActionType = "sendFunds"
	ActionType_peggyDepositClaim        ActionType = "peggyDepositClaim"
)

type MarketType string

// nolint:all
const (
	MarketType_spot       MarketType = "spot"
	MarketType_derivative MarketType = "derivative"
	MarketType_expiry     MarketType = "expiry"
	MarketType_binary     MarketType = "binary"
)

func (mt MarketType) Ptr() *MarketType {
	return &mt
}

type OrderType string

// nolint:all
const (
	OrderType_BUY         OrderType = "buy"
	OrderType_SELL        OrderType = "sell"
	OrderType_STOP_BUY    OrderType = "stopBuy"
	OrderType_STOP_SELL   OrderType = "stopSell"
	OrderType_TAKE_BUY    OrderType = "takeProfitBuy"
	OrderType_TAKE_SELL   OrderType = "takeProfitSell"
	OrderType_BUY_PO      OrderType = "poBuy"
	OrderType_SELL_PO     OrderType = "poSell"
	OrderType_BUY_ATOMIC  OrderType = "buyAtomic"
	OrderType_SELL_ATOMIC OrderType = "sellAtomic"
)

func (pt OrderType) Ptr() *OrderType {
	return &pt
}

func (ot OrderType) getExchangeOrderType() types.OrderType {
	switch ot {
	case OrderType_BUY:
		return types.OrderType_BUY
	case OrderType_BUY_PO:
		return types.OrderType_BUY_PO
	case OrderType_SELL:
		return types.OrderType_SELL
	case OrderType_SELL_PO:
		return types.OrderType_SELL_PO
	case OrderType_STOP_BUY:
		return types.OrderType_STOP_BUY
	case OrderType_STOP_SELL:
		return types.OrderType_STOP_SELL
	case OrderType_TAKE_BUY:
		return types.OrderType_TAKE_BUY
	case OrderType_TAKE_SELL:
		return types.OrderType_TAKE_SELL
	case OrderType_BUY_ATOMIC:
		return types.OrderType_BUY_ATOMIC
	case OrderType_SELL_ATOMIC:
		return types.OrderType_SELL_ATOMIC
	}
	return types.OrderType_UNSPECIFIED
}

func GetTestOrderType(ot types.OrderType) OrderType {
	switch ot {
	case types.OrderType_BUY:
		return OrderType_BUY
	case types.OrderType_BUY_PO:
		return OrderType_BUY_PO
	case types.OrderType_SELL:
		return OrderType_SELL
	case types.OrderType_SELL_PO:
		return OrderType_SELL_PO
	case types.OrderType_STOP_BUY:
		return OrderType_STOP_BUY
	case types.OrderType_STOP_SELL:
		return OrderType_STOP_SELL
	case types.OrderType_TAKE_BUY:
		return OrderType_TAKE_BUY
	case types.OrderType_TAKE_SELL:
		return OrderType_TAKE_SELL
	case types.OrderType_BUY_ATOMIC:
		return OrderType_BUY_ATOMIC
	case types.OrderType_SELL_ATOMIC:
		return OrderType_SELL_ATOMIC
	}
	return ""
}

type TestAction interface {
	GetActionType() ActionType
	GetActionId() string
	SetActionId(id string)
	GetRepeatTimes() int
	shouldLog() bool
}

type HasAccountIndex interface {
	getAccountIndex() int
}

type RecordedTest struct {
	NumAccounts             int               `json:"numAccounts"`
	NumSpotMarkets          int               `json:"numSpotMarkets"`
	NumDerivativeMarkets    int               `json:"numDerivativeMarkets"`
	NumExpiryMarkets        int               `json:"numExpiryMarkets"`
	NumBinaryOptionsMarkets int               `json:"numBinaryOptionsMarkets"`
	Seed                    int64             `json:"seed"`
	Config                  *TestPlayerConfig `json:"config,omitempty"`
	Actions                 []TestAction      `json:"-"`
	RawActions              []json.RawMessage `json:"actions"`
	Accounts                [][]byte          `json:"accounts"`
}

// extra config for tests
type TestPlayerConfig struct {
	NumAccounts               int
	NumSpotMarkets            int
	NumDerivativeMarkets      int
	NumExpiryMarkets          int
	NumBinaryOptionsMarkets   int
	BankParams                *TestPlayerBankParams          `json:"bankParams,omitempty"`
	ExchangeParams            *TestPlayerExchangeParams      `json:"exchangeParams,omitempty"`
	TokenFactoryParams        *TestPlayerTokenFactoryParams  `json:"tokenFactoryParams,omitempty"`
	PerpMarkets               *[]*TestPlayerMarketPerpConfig `json:"perpMarkets,omitempty"`
	InitMarketMaking          bool                           `json:"initMarketMaking"`
	InitContractRegistry      bool                           `json:"initContractRegistry"`
	InitAuctionModule         bool                           `json:"initAuctionModule"`
	RegistryOwnerAccountIndex int                            `json:"registryOwnerAccountIndex"`
	RandSeed                  int64
	NumSubaccounts            int `json:"numSubaccounts"` /// how many account should be created (default will be 1)
}

type TestPlayerMarketPerpConfig struct {
	TakerFeeRate      *float64 `json:"takerFeeRate,omitempty"`
	MakerFeeRate      *float64 `json:"makerFeeRate,omitempty"`
	InsuranceFund     *int64   `json:"insuranceFund,omitempty"`
	OracleScaleFactor *uint32  `json:"oracleScaleFactor,omitempty"`
}

type TestPlayerTokenFactoryParams struct {
	DenomCreationFee *Coin `json:"denomCreationFee"`
}

type MintDestination string

// nolint: all
const (
	MintDestination_bank       MintDestination = "bank"
	MintDestination_subaccount MintDestination = "subaccount"
)

type MintCoins struct {
	Coin
	MintTo MintDestination `json:"mintTo"`
}

type TestPlayerBankParams struct {
	ExtraCoins *[]MintCoins `json:"extraCoins,omitempty"`
}

type TestPlayerExchangeParams struct {
	AtomicMarketOrderAccessLevel   *AtomicMarketOrderAccessLevel `json:"atomicMarketOrderAccessLevel,omitempty"`
	AtomicMarketOrderFeeMultiplier *float64                      `json:"atomicMarketOrderFeeMultiplier"`
}

type AtomicMarketOrderAccessLevel string

// nolint:all
const (
	AtomicMarketOrderAccessLevel_nobody                    AtomicMarketOrderAccessLevel = "nobody"
	AtomicMarketOrderAccessLevel_smartContract             AtomicMarketOrderAccessLevel = "smartContract"
	AtomicMarketOrderAccessLevel_smartContractBeginBlocker AtomicMarketOrderAccessLevel = "smartContractBeginBlocker"
	AtomicMarketOrderAccessLevel_everyone                  AtomicMarketOrderAccessLevel = "everyone"
)

func (al AtomicMarketOrderAccessLevel) getExchangeAccessLevel() types.AtomicMarketOrderAccessLevel {
	switch al {
	case AtomicMarketOrderAccessLevel_nobody:
		return types.AtomicMarketOrderAccessLevel_Nobody
	case AtomicMarketOrderAccessLevel_smartContract:
		return types.AtomicMarketOrderAccessLevel_SmartContractsOnly
	case AtomicMarketOrderAccessLevel_smartContractBeginBlocker:
		return types.AtomicMarketOrderAccessLevel_BeginBlockerSmartContractsOnly
	case AtomicMarketOrderAccessLevel_everyone:
		return types.AtomicMarketOrderAccessLevel_Everyone
	}
	return types.AtomicMarketOrderAccessLevel_Nobody
}

type ActionsInclude struct {
	TestActionBase
	File string `json:"file"`
}

type TestActionBase struct {
	ActionType  ActionType `json:"actionType"`
	ActionId    string     `json:"actionId"`
	RepeatTimes int        `json:"repeatTimes"` // if > 0 it will be replayed this many times, default 1 time
	ShouldLog   bool       `json:"shouldLog"`   // if true, will perform some basic log of the action in console (for debugging only)
}

func (f *TestActionBase) GetActionType() ActionType { return f.ActionType }
func (f *TestActionBase) GetActionId() string       { return f.ActionId }
func (f *TestActionBase) SetActionId(id string)     { f.ActionId = id }
func (f *TestActionBase) GetRepeatTimes() int {
	if f.RepeatTimes > 0 {
		return f.RepeatTimes
	}
	return 1
}
func (f *TestActionBase) shouldLog() bool { return f.ShouldLog }
func NewActionBaseWithType(actionType ActionType) TestActionBase {
	return TestActionBase{
		ActionType: actionType,
	}
}

type TestActionWithMarket struct {
	MarketIndex int         `json:"marketIndex"`          // default 0
	MarketType  *MarketType `json:"marketType,omitempty"` // default context dependent
}

type TestActionWithAccount struct {
	AccountIndex    int `json:"accountIndex"`
	SubaccountIndex int `json:"subaccountIndex"`
}

func (f *TestActionWithAccount) getAccountIndex() int { return f.AccountIndex }

type TestActionWithMarketAndAccount struct {
	TestActionWithMarket
	TestActionWithAccount
}

func NewTestActionWithDefaultMarketAndAccount(
	accountIndex int,
	subaccountIndex int,
) TestActionWithMarketAndAccount {
	return TestActionWithMarketAndAccount{
		TestActionWithAccount: TestActionWithAccount{
			AccountIndex:    accountIndex,
			SubaccountIndex: subaccountIndex,
		},
	}
}

// ActionCreateOrder Supports types: "derivativeLimitOrder", "expiryLimitOrder", "boLimitOrder"
type ActionCreateOrder struct {
	TestActionBase
	TestActionWithMarketAndAccount
	Price        float64    `json:"price"`
	Quantity     float64    `json:"quantity"`
	Leverage     float64    `json:"leverage"` // how leveraged position is - if unset will use x10 when applicable
	Margin       *float64   `json:"margin"`   // if unset will calculate automatically - overrides leverage if set
	IsLong       bool       `json:"isLong"`
	IsReduceOnly bool       `json:"isReduceOnly"`
	OrderId      string     `json:"orderId"`                // identificator of the order, will be order hash for fuzz tests
	TriggerPrice *float64   `json:"triggerPrice,omitempty"` // default nil - no trigger price
	OrderType    *OrderType `json:"orderType"`
	FeeRecipient *string    `json:"feeRecipientAccountIndex"` // defaults to "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"
}

type ActionCancelOrder struct {
	TestActionBase
	TestActionWithMarketAndAccount
	IsLong    bool   `json:"isLong"`
	OrderId   string `json:"orderId"` // this is id of order (by default order hash)
	OrderMask int32  `json:"orderMask"`
}

type ActionEndBlocker struct {
	TestActionBase
	SkipBeginBlock bool `json:"skipBeginBlock"`
	BlockInterval  int  `json:"blockInterval"`
	//BlockTime      *string `json:"blockTime"`
}

type ActionBeginBlocker struct {
	TestActionBase
}

type ActionLiquidatePosition struct {
	TestActionBase
	TestActionWithMarketAndAccount
	LiquidatorAccountIndex *int `json:"liquidatorAccountIndex,omitempty"`
}

type ActionSetPriceOracles struct {
	TestActionBase
	SpotsPrices   []float64 `json:"spotsPrices"`
	PerpsPrices   []float64 `json:"perpsPrices"`
	ExpiryMarkets []float64 `json:"expiryMarkets"`
	BinaryMarkets []float64 `json:"binaryMarkets"`
}

type ActionLaunchMarket struct {
	TestActionBase
	MarketType          string  `json:"marketType"`
	Ticker              string  `json:"ticker"`
	BaseDenom           string  `json:"baseDenom"`
	QuoteDenom          string  `json:"quoteDenom"`
	MinPriceTickSize    *string `json:"minPriceTickSize"`
	MinQuantityTickSize *string `json:"minQuantityTickSize"`
}

type Coin struct {
	Amount int64  `json:"amount"`
	Denom  string `json:"denom"`
}

func (c Coin) toSdkCoin() sdk.Coin {
	return sdk.Coin{
		Denom:  c.Denom,
		Amount: sdk.NewInt(c.Amount),
	}
}

func (c Coin) toSdkCoinWithSubstitution(
	tp *TestPlayer,
	params *TestActionWithMarketAndAccount,
) sdk.Coin {
	denom := MustNotErr(tp.substituteParamsInString(c.Denom, params)).(string)
	return sdk.Coin{
		Denom:  denom,
		Amount: sdk.NewInt(c.Amount),
	}
}

func testCoinsToSdkCoins(cc []Coin) sdk.Coins {
	coins := make([]sdk.Coin, len(cc))
	for i, coin := range cc {
		coins[i] = coin.toSdkCoin()
	}
	return coins
}

func testCoinsToSdkCoinsWithSubstitution(
	cc []Coin,
	tp *TestPlayer,
	params *TestActionWithMarketAndAccount,
) sdk.Coins {
	coins := make([]sdk.Coin, len(cc))
	for i, coin := range cc {
		coins[i] = coin.toSdkCoinWithSubstitution(tp, params)
	}
	return coins
}

type ActionStoreContractCode struct {
	TestActionBase
	TestActionWithMarketAndAccount
	Filename   string `json:"filename"`
	ContractId string `json:"contractId"`
	Path       string `json:"path"`
}

type ActionRegisterAndInitializeContract struct {
	ActionStoreContractCode
	CodeId              uint64                                  `json:"codeId"`
	Label               string                                  `json:"label"`
	Message             map[string]any                          `json:"message"`
	RegisterForBB       bool                                    `json:"registerForBB"`
	RegisterForBBParams *wasmxtypes.ContractRegistrationRequest `json:"registerForBBParams,omitempty"`
	Funds               []Coin                                  `json:"funds"`
}

type ContractBeginBlockerRegistrationParams struct {
	GasLimit uint64 `json:"gasLimit"`
	GasPrice uint64 `json:"gasPrice"`
}

type BeginBlockerRegistryParams struct {
	GasLimit uint64 `json:"gasLimit"`
	GasPrice uint64 `json:"gasPrice"`
}

type ActionRegisterMitoVault struct {
	TestActionBase
	TestActionWithMarketAndAccount
	MasterContractId string            `json:"masterContractId"`
	RegistrationInfo *RegistrationInfo `json:"registrationInfo,omitempty"`
}

type RegistrationInfo struct {
	Message map[string]any `json:"message"`
	Info    *struct {
		ContractId     string                       `json:"contractId"`
		CodeId         string                       `json:"codeId"`
		RegisterForBB  bool                         `json:"registerForBB"`
		RegistryParams *ActionRegisterContractForBB `json:"registryParams,omitempty"`
	} `json:"info,omitempty"`
}

type ActionExecuteContract struct {
	TestActionBase
	TestActionWithMarketAndAccount
	ContractId    string                `json:"contractId"`
	Message       any                   `json:"message"`
	Funds         []Coin                `json:"funds"`
	WithdrawFunds bool                  `json:"withdrawFunds"`
	ExecutionType string                `json:"executionType"`
	ExtraParams   *ExtraExecutionParams `json:"extraParams,omitempty"`
}

type ExtraExecutionParams struct {
	Registration *struct {
		ContractId string `json:"contractId"`
		CodeId     string `json:"codeId"`
	} `json:"registration,omitempty"`
}

type ActionRegisterContractForBB struct {
	TestActionBase
	ContractAddress string                  `json:"contractAddress"`
	GasLimit        *int                    `json:"gasLimit,omitempty"`
	GasPrice        *int                    `json:"gasPrice,omitempty"`
	Funds           []Coin                  `json:"funds"`
	GranterAddress  *string                 `json:"granterAddress,omitempty"`
	FundingMode     *wasmxtypes.FundingMode `json:"fundingMode,omitempty"`
}

type SwapContractParams struct {
	Quantity string `json:"quantity"`
	Price    string `json:"price"`
}

type SwapContractMsg struct {
	SwapSpot SwapSpotParams `json:"swap_spot"`
}

type SwapSpotParams struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

type BatchActionType string

// nolint:all
const (
	BatchActionType_cancelDerivatives BatchActionType = "derivativeCancels"
	BatchActionType_all               BatchActionType = "all"
)

type ActionBatchOrderUpdate struct {
	TestActionBase
	TestActionWithAccount
	SpotOrdersToCancel           []string            `json:"spotOrdersToCancel"`
	SpotOrdersToCreate           []ActionCreateOrder `json:"spotOrdersToCreate"`
	SpotMarketsToCancelAll       []int               `json:"spotMarketsToCancelAll"`
	DerivativeOrdersToCancel     []string            `json:"derivativeOrdersToCancel"`
	DerivativeOrdersToCreate     []ActionCreateOrder `json:"derivativeOrdersToCreate"`
	DerivativeMarketsToCancelAll []int               `json:"derivativeMarketsToCancelAll"`
	BinaryOrdersToCancel         []string            `json:"binaryOrdersToCancel"`
	BinaryOrdersToCreate         []ActionCreateOrder `json:"binaryOrdersToCreate"`
	BinaryMarketsToCancelAll     []int               `json:"binaryMarketsToCancelAll"`
	BatchType                    BatchActionType     `json:"batchType,omitempty"` // default all
}

// ActionCreatePosition - only for unit testing for setting up initial state. Supports types: "positionDerivative", "positionBinary"
type ActionCreatePosition struct {
	TestActionBase
	TestActionWithMarket
	Quantity          float64  `json:"quantity"`
	LongAccountIndex  int      `json:"longAccountIndex"`
	ShortAccountIndex int      `json:"shortAccountIndex"`
	EntryPrice        float64  `json:"entryPrice"`   // entry price, will use some random if not relevant
	LeverageLong      float64  `json:"LeverageLong"` // how leveraged position is - if unset will use x10 when applicable
	MarginLong        *float64 `json:"marginLong"`   // if 0/unset will calculate automatically - overrides leverage if set
	LeverageShort     float64  `json:"leverageShort"`
	MarginShort       *float64 `json:"marginShort"`
}

type ActionDeposit struct {
	TestActionBase
	TestActionWithAccount
	Amount *float64 `json:"amount,omitempty"`
	ToHave *float64 `json:"toHave,omitempty"`
	Denom  string   `json:"denom"`
}

type ActionWithdrawal struct {
	TestActionBase
	TestActionWithAccount
	Amount  *float64 `json:"amount,omitempty"`
	ToLeave *float64 `json:"toLeave,omitempty"`
	Denom   string   `json:"denom"`
}

type ActionSend struct {
	TestActionBase
	TestActionWithAccount
	Amount         *int64 `json:"amount,omitempty"`
	ToLeave        *int64 `json:"toLeave,omitempty"`
	Denom          string `json:"denom"`
	RecipientIndex int    `json:"recipientIndex"`
}

type ActionCreateDenomTokenFactory struct {
	TestActionBase
	TestActionWithAccount
	Subdenom string `json:"subdenom"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
}
type ActionMintTokenFactory struct {
	TestActionBase
	TestActionWithAccount
	Denom  string `json:"denom"`
	Amount int    `json:"amount"`
}

type ActionBurnTokenFactory struct {
	TestActionBase
	TestActionWithAccount
	Denom  string `json:"denom"`
	Amount int    `json:"amount"`
}

type ActionUpdateMarketParams struct {
	TestActionBase
	TestActionWithMarket
	MaintenanceMarginRatio *float64            `json:"maintenanceMarginRatio,omitempty"`
	InitialMarginRatio     *float64            `json:"initialMarginRatio,omitempty"`
	MakerFeeRate           *float64            `json:"makerFeeRate,omitempty"`
	TakerFeeRate           *float64            `json:"takerFeeRate,omitempty"`
	HourlyInterestRate     *float64            `json:"hourlyInterestRate,omitempty"`
	HourlyFundingRateCap   *float64            `json:"hourlyFundingRateCap,omitempty"`
	RelayerFeeShareRate    *float64            `json:"relayerFeeShareRate,omitempty"`
	MinPriceTickSize       *float64            `json:"minPriceTickSize,omitempty"`
	MinQuantityTickSize    *float64            `json:"minQuantityTickSize,omitempty"`
	ExpirationTimestamp    int64               `json:"expirationTimestamp,omitempty"`
	SettlementTimestamp    int64               `json:"settlementTimestamp,omitempty"`
	SettlementPrice        *float64            `json:"settlementPrice,omitempty"`
	Admin                  string              `json:"admin,omitempty"`
	MarketStatus           *types.MarketStatus `json:"marketStatus"`
}

type ActionForcedMarketSettlementParams struct {
	TestActionBase
	MarketIndex     *int        `json:"marketIndex,omitempty"` // default 0
	MarketType      *MarketType `json:"marketType"`
	SettlementPrice *float64    `json:"settlementPrice,omitempty"`
	Title           *string     `json:"title,omitempty"`
	Description     *string     `json:"description,omitempty"`
}

type ActionPerpetualMarketFundingParams struct {
	TestActionBase
	MarketIndex       *int        `json:"marketIndex,omitempty"` // default 0
	MarketType        *MarketType `json:"marketType"`
	CumulativePrice   float64     `json:"cumulativePrice"`
	CumulativeFunding float64     `json:"cumulativeFunding"`
}

type ActionBatchDeregisterContracts struct {
	TestActionBase
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Contracts   []string `json:"contracts"`
}

type ActionBatchRegisterContracts struct {
	TestActionBase
	Title       string                                   `json:"title"`
	Description string                                   `json:"description"`
	Requests    []wasmxtypes.ContractRegistrationRequest `json:"requests"`
}

type ActionUnderwrite struct {
	TestActionBase
	TestActionWithAccount
	MarketIndex int  `json:"marketIndex"`
	Deposit     Coin `json:"deposit"`
}

type ActionPlaceBid struct {
	TestActionBase
	TestActionWithAccount
	Deposit Coin `json:"deposit"`
	Round   int  `json:"round"`
}

type ActionSendToEth struct {
	TestActionBase
	TestActionWithAccount
	RecipientEthAddress string `json:"recipientEthAddress"`
	Amount              Coin   `json:"amount"`
	BridgeFee           Coin   `json:"bridgeFee"`
}

type ActionPeggyDepositClaim struct {
	TestActionBase
	TestActionWithAccount
	Nonce             uint64  `json:"nonce"`
	BlockHeight       uint64  `json:"block_height"`
	TokenContract     *string `json:"token_contract"`
	Amount            uint64  `json:"amount"`
	EthereumSender    *string `json:"ethereum_sender"`     //if empty defaults to Ethereum address of account specified by AccountIndex
	CosmosReceiver    *string `json:"cosmos_receiver"`     //if empty defaults to Injective address of account specified by AccountIndex
	Orchestrator      *string `json:"orchestrator"`        //use with caution, if you provide your own orchestrator make sure it's properly setup
	ArbitraryData     *string `json:"arbitrary_data"`      //JSON representation of any cosmos message, but passed as string, not as a JSON object
	ArbitraryDataType *string `json:"arbitrary_data_type"` //arbitrary value, but one for which we have added a mapping to the real type
}

type ActionDelegate struct {
	TestActionBase
	TestActionWithAccount
	ValidatorAddress string `json:"validatorAddress"`
	Amount           Coin   `json:"amount"`
}

type ActionMintCoinParams struct {
	TestActionBase
	TestActionWithMarketAndAccount
	ToSubaccount bool `json:"toSubaccount"`
	Amount       Coin `json:"amount"`
}

type orderIdentifier struct {
	orderHash  string
	marketId   string
	marketType MarketType
}

type TestPlayerHookParams struct {
	Response any
	Error    error
}

type TestPlayerHook func(*TestPlayerHookParams)

func newOrderId(orderHash, marketId string, marketType MarketType) *orderIdentifier {
	return &orderIdentifier{
		orderHash:  orderHash,
		marketId:   marketId,
		marketType: marketType,
	}
}

type ParamParsers struct {
	ContractSubAddr *regexp.Regexp
	SubAddr         *regexp.Regexp
	AccountIdx      *regexp.Regexp
	TokenFactoryIdx *regexp.Regexp
	ContractsCodeId *regexp.Regexp
	BlockHeight     *regexp.Regexp
	BlockTime       *regexp.Regexp
}
