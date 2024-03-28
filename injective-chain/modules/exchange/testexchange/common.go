package testexchange

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ccodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	"github.com/cosmos/ibc-go/v7/modules/apps/transfer"
	ibc "github.com/cosmos/ibc-go/v7/modules/core"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/auction"
	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	. "github.com/onsi/gomega"
)

var (
	// ModuleBasics is a mock module basic manager for testing
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distribution.AppModuleBasic{},
		gov.NewAppModuleBasic(
			[]govclient.ProposalHandler{
				paramsclient.ProposalHandler,
				upgradeclient.LegacyProposalHandler,
				upgradeclient.LegacyCancelProposalHandler,
			},
		),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		ibc.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		transfer.AppModuleBasic{},
		vesting.AppModuleBasic{},
	)
)

var (

	// BuyerAccPrivKeys generate secp256k1 pubkeys to be used for account pub keys
	BuyerAccPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}

	// BuyerAccPubKeys holds the pub keys for the account keys
	BuyerAccPubKeys = []ccrypto.PubKey{
		BuyerAccPrivKeys[0].PubKey(),
		BuyerAccPrivKeys[1].PubKey(),
		BuyerAccPrivKeys[2].PubKey(),
		BuyerAccPrivKeys[3].PubKey(),
		BuyerAccPrivKeys[4].PubKey(),
	}

	// BuyerAccAddrs holds the sdk.AccAddresses
	BuyerAccAddrs = []sdk.AccAddress{
		sdk.AccAddress(BuyerAccPubKeys[0].Address()),
		sdk.AccAddress(BuyerAccPubKeys[1].Address()),
		sdk.AccAddress(BuyerAccPubKeys[2].Address()),
		sdk.AccAddress(BuyerAccPubKeys[3].Address()),
		sdk.AccAddress(BuyerAccPubKeys[4].Address()),
	}

	// SellerAccPrivKeys generate secp256k1 pubkeys to be used for account pub keys
	SellerAccPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}

	// SellerAccPubKeys holds the pub keys for the account keys
	SellerAccPubKeys = []ccrypto.PubKey{
		SellerAccPrivKeys[0].PubKey(),
		SellerAccPrivKeys[1].PubKey(),
		SellerAccPrivKeys[2].PubKey(),
		SellerAccPrivKeys[3].PubKey(),
		SellerAccPrivKeys[4].PubKey(),
	}

	// SellerAccAddrs holds the sdk.AccAddresses
	SellerAccAddrs = []sdk.AccAddress{
		sdk.AccAddress(SellerAccPubKeys[0].Address()),
		sdk.AccAddress(SellerAccPubKeys[1].Address()),
		sdk.AccAddress(SellerAccPubKeys[2].Address()),
		sdk.AccAddress(SellerAccPubKeys[3].Address()),
		sdk.AccAddress(SellerAccPubKeys[4].Address()),
	}

	// InitCoins holds the number of coins to initialize an Accounts with
	InitCoins = sdk.NewCoins(chaintypes.NewInjectiveCoin(sdk.NewInt(10)))

	// TestingExchangeParams is a set of exchange params for testing
	TestingExchangeParams = GetDefaultTestingExchangeParams()

	DefaultWithdrawalAddress = sdk.MustAccAddressFromBech32("inj1w26juqraq8x94smrfy5g7fxwr0v39nkluxrq07")
)

func GetDefaultTestingExchangeParams() exchangetypes.Params {
	return exchangetypes.Params{
		SpotMarketInstantListingFee:                 chaintypes.NewInjectiveCoin(sdkmath.NewIntWithDecimal(exchangetypes.SpotMarketInstantListingFee, chaintypes.BaseDenomUnit)),
		DerivativeMarketInstantListingFee:           chaintypes.NewInjectiveCoin(sdkmath.NewIntWithDecimal(exchangetypes.DerivativeMarketInstantListingFee, chaintypes.BaseDenomUnit)),
		DefaultSpotMakerFeeRate:                     sdk.NewDecWithPrec(1, 3), // default 0.1% fees
		DefaultSpotTakerFeeRate:                     sdk.NewDecWithPrec(3, 3), // default 0.3% fees
		DefaultDerivativeMakerFeeRate:               sdk.NewDecWithPrec(1, 3), // default 0.1% fees
		DefaultDerivativeTakerFeeRate:               sdk.NewDecWithPrec(3, 3), // default 0.3% fees
		DefaultInitialMarginRatio:                   sdk.NewDecWithPrec(5, 2), // default 5% initial margin ratio
		DefaultMaintenanceMarginRatio:               sdk.NewDecWithPrec(2, 2), // default 2% maintenance margin ratio
		DefaultFundingInterval:                      exchangetypes.DefaultFundingIntervalSeconds,
		FundingMultiple:                             exchangetypes.DefaultFundingMultipleSeconds,
		RelayerFeeShareRate:                         sdk.NewDecWithPrec(40, 2),
		DefaultHourlyFundingRateCap:                 sdk.NewDecWithPrec(625, 6),     // default 0.0625% max hourly funding rate
		DefaultHourlyInterestRate:                   sdk.NewDecWithPrec(416666, 11), // 0.01% daily interest rate = 0.0001 / 24 = 0.00000416666
		MaxDerivativeOrderSideCount:                 uint32(20),
		InjRewardStakedRequirementThreshold:         sdkmath.NewIntWithDecimal(100, chaintypes.BaseDenomUnit), // 100 INJ
		TradingRewardsVestingDuration:               15000,
		LiquidatorRewardShareRate:                   sdk.NewDecWithPrec(5, 2),
		BinaryOptionsMarketInstantListingFee:        chaintypes.NewInjectiveCoin(sdkmath.NewIntWithDecimal(exchangetypes.BinaryOptionsMarketInstantListingFee, chaintypes.BaseDenomUnit)),
		AtomicMarketOrderAccessLevel:                exchangetypes.AtomicMarketOrderAccessLevel_SmartContractsOnly,
		SpotAtomicMarketOrderFeeMultiplier:          sdk.NewDecWithPrec(25, 1),
		DerivativeAtomicMarketOrderFeeMultiplier:    sdk.NewDecWithPrec(25, 1),
		BinaryOptionsAtomicMarketOrderFeeMultiplier: sdk.NewDecWithPrec(25, 1),
		MinimalProtocolFeeRate:                      sdk.MustNewDecFromStr("0.00005"),
	}
}

var (
	startingDeposit = sdk.NewDec(1_000_000_000_000_000)
)

type Market struct {
	BaseDenom           string
	QuoteDenom          string
	Ticker              string
	MarketID            common.Hash
	MinPriceTickSize    sdk.Dec
	MinQuantityTickSize sdk.Dec
	MakerFeeRate        sdk.Dec
	TakerFeeRate        sdk.Dec
	IsActive            bool // used to optimize fuzz tests, can be ignored otherwise
}

type SpotMarket struct {
	Market
}

type PerpMarket struct {
	Market
	OracleBase             string
	OracleQuote            string
	OracleType             oracletypes.OracleType
	InitialMarginRatio     sdk.Dec
	MaintenanceMarginRatio sdk.Dec
}

type BinaryMarket struct {
	Market
	OracleSymbol        string
	OracleProvider      string
	OracleScaleFactor   uint32
	ExpirationTimestamp int64
	SettlementTimestamp int64
	Admin               string
	SettlementPrice     *sdk.Dec
}

type ExpiryMarket struct {
	Market
	OracleBase             string
	OracleQuote            string
	OracleType             oracletypes.OracleType
	InitialMarginRatio     sdk.Dec
	MaintenanceMarginRatio sdk.Dec
	Expiry                 int64
}

// TestInput stores the various keepers required to test peggy
type TestInput struct {
	Spots         []SpotMarket
	Perps         []PerpMarket
	BinaryMarkets []BinaryMarket
	ExpiryMarkets []ExpiryMarket

	MarketIDToSpotMarket   map[common.Hash]*SpotMarket
	MarketIDToPerpMarket   map[common.Hash]*PerpMarket
	MarketIDToExpiryMarket map[common.Hash]*ExpiryMarket
	MarketIDToBinaryMarket map[common.Hash]*BinaryMarket

	InitialBankSupply *sdk.Coins
}

const ThirtyMinutesInSeconds = 60 * 30
const FourWeeksInSeconds = ThirtyMinutesInSeconds * 2 * 24 * 7 * 4
const DefaultAddress = "inj1wfawuv6fslzjlfa4v7exv27mk6rpfeyvhvxchc"
const DefaultFeeRecipientAddress = "inj1jfawuv6fslzjlfa4v7exv27mk6rpfeyvzsj987"
const DefaultValidatorAddress = "injvaloper1w6upuzptkvlpugp0zg6mlr76sxctkey6vk8dsu"
const DefaultValidatorDelegatorAddress = "inj1w6upuzptkvlpugp0zg6mlr76sxctkey6msjg3c"

const SampleAccountAddrStr1 = "inj1p7z8p649xspcey7wp5e4leqf7wa39kjjj6wja8"
const SampleAccountAddrStr2 = "inj1ve5ux2vmmgnk98gg4fyhtml8yfvy28pxhzvgz4"
const SampleAccountAddrStr3 = "inj19y6ev2fymrnmqgarveyxqgj02t6r09l0gjxyj9"
const SampleAccountAddrStr4 = "inj1enur5aqtnl0fda362yjx8dr75v5fl9qta9tcxz"
const SampleAccountAddrStr5 = "inj1qtxe8dl3hhjjepst3zx96m04lpxftlzfst7k48"
const SampleAccountAddrStr6 = "inj16sv0ptr00m333mlut2et7htxtgz3kq5s9e3adl"
const SampleAccountAddrStr7 = "inj1m4sxt3wj3m4hfh7lv7aemtac606wletpn2shau"
const SampleAccountAddrStr8 = "inj1u3e9xlq2c8nr3cknhxv44w958tqdp0fwfquzj5"
const SampleAccountAddrStr9 = "inj1yay9xerfd3gmqp5vwm2zl9ymztjhtcljtuz4yx"
const SampleAccountAddrStr10 = "inj15y7dhyl76xesp0jk8pwyxt3javfdrh8d389x5m"

var (
	SampleAccountAddr1  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr1)
	SampleAccountAddr2  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr2)
	SampleAccountAddr3  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr3)
	SampleAccountAddr4  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr4)
	SampleAccountAddr5  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr5)
	SampleAccountAddr6  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr6)
	SampleAccountAddr7  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr7)
	SampleAccountAddr8  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr8)
	SampleAccountAddr9  = sdk.MustAccAddressFromBech32(SampleAccountAddrStr9)
	SampleAccountAddr10 = sdk.MustAccAddressFromBech32(SampleAccountAddrStr10)
)

var (
	SampleSubaccountAddr1  = GetSubaccountId(SampleAccountAddrStr1)
	SampleSubaccountAddr2  = GetSubaccountId(SampleAccountAddrStr2)
	SampleSubaccountAddr3  = GetSubaccountId(SampleAccountAddrStr3)
	SampleSubaccountAddr4  = GetSubaccountId(SampleAccountAddrStr4)
	SampleSubaccountAddr5  = GetSubaccountId(SampleAccountAddrStr5)
	SampleSubaccountAddr6  = GetSubaccountId(SampleAccountAddrStr6)
	SampleSubaccountAddr7  = GetSubaccountId(SampleAccountAddrStr7)
	SampleSubaccountAddr8  = GetSubaccountId(SampleAccountAddrStr8)
	SampleSubaccountAddr9  = GetSubaccountId(SampleAccountAddrStr9)
	SampleSubaccountAddr10 = GetSubaccountId(SampleAccountAddrStr10)
)

var (
	SampleDefaultSubaccountAddr1  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr1).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr2  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr2).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr3  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr3).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr4  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr4).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr5  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr5).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr6  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr6).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr7  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr7).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr8  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr8).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr9  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr9).Bytes(), uint32(0)))
	SampleDefaultSubaccountAddr10 = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr10).Bytes(), uint32(0)))
)

var (
	SampleNonDefaultSubaccountAddr1  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr1).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr2  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr2).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr3  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr3).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr4  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr4).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr5  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr5).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr6  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr6).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr7  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr7).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr8  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr8).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr9  = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr9).Bytes(), uint32(1)))
	SampleNonDefaultSubaccountAddr10 = *MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(sdk.MustAccAddressFromBech32(SampleAccountAddrStr10).Bytes(), uint32(1)))
)

func GetSubaccountId(address string) common.Hash {
	startIndex := uint32(getSubaccountStartingIndex())

	sdkAddress := sdk.MustAccAddressFromBech32(address)
	subaccountId, err := exchangetypes.SdkAddressWithNonceToSubaccountID(sdkAddress.Bytes(), uint32(startIndex))
	if err != nil {
		panic(err)
	}

	return *subaccountId
}

var (
	subaccountStartingIndex *int
)

func IsUsingDefaultSubaccount() bool {
	return getSubaccountStartingIndex() == 0
}

func getSubaccountStartingIndex() int {
	if subaccountStartingIndex != nil {
		return *subaccountStartingIndex
	}

	// try properties file first
	// working directory depends on the location of test file that's calling the function, not file containting it
	// so we need to adjust properties file location accordingly
	workindDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	var joinByOsPathSeparator = func(elements []string) string {
		result := ""
		for idx := range elements {
			if idx != len(elements)-1 {
				result = result + elements[idx] + string(os.PathSeparator)
			} else {
				result = result + elements[idx]
			}
		}

		return result
	}

	rootRelativeLocation := ""
	filename := "test.properties"
	injectiveChainIndex := strings.Index(workindDir, "injective-chain")
	if injectiveChainIndex != -1 {
		childFolderCount := strings.Count(workindDir[injectiveChainIndex:], string(os.PathSeparator))
		nestedLocation := ""
		for i := 0; i < childFolderCount; i++ {
			nestedLocation = nestedLocation + ".." + string(os.PathSeparator)
		}
		rootRelativeLocation = nestedLocation + joinByOsPathSeparator([]string{"modules", "exchange", "testexchange", filename})
	}

	if rootRelativeLocation != "" {
		propertiesBytes, err := ReadFileWithError(rootRelativeLocation)
		if err == nil && len(propertiesBytes) > 0 {
			propertiesString := string(propertiesBytes)
			subexpressionName := "subaccount_start_index"
			r := regexp.MustCompile(fmt.Sprintf("%s=(?P<%s>.*$)", subexpressionName, subexpressionName))
			matches := r.FindStringSubmatch(propertiesString)
			if idx := r.SubexpIndex(subexpressionName); idx >= 0 {
				asInt, err := strconv.Atoi(matches[idx])
				if err != nil {
					panic(err)
				}
				subaccountStartingIndex = &asInt
				return asInt
			}
		}
	}

	// from env
	envValue := os.Getenv("TEST_EXCHANGE_USE_DEFAULT_SUBACCOUNT")
	if envValue != "" {
		text := strings.ToLower(envValue)
		startIndex := 1
		if text == "true" || text == "yes" || text == "1" {
			startIndex = 0
		}

		subaccountStartingIndex = &startIndex

		return startIndex
	}

	// return default
	defaultIndex := 0
	subaccountStartingIndex = &defaultIndex
	return defaultIndex
}

func SetupPerpetualMarket(
	baseDenom,
	quoteDenom string,
	oracleBase string,
	oracleQuote string,
	oracleType oracletypes.OracleType,
) PerpMarket {
	var (
		ticker   = baseDenom + "/" + quoteDenom
		marketID = exchangetypes.NewPerpetualMarketID(ticker, quoteDenom, oracleBase, oracleQuote, oracleType)
	)
	return PerpMarket{
		Market: Market{
			BaseDenom:           baseDenom,
			QuoteDenom:          quoteDenom,
			Ticker:              ticker,
			MarketID:            marketID,
			MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
			MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
			MakerFeeRate:        sdk.NewDecWithPrec(1, 3), // default 0.1% maker fees
			TakerFeeRate:        sdk.NewDecWithPrec(3, 3), // default 0.3% taker fees
			IsActive:            true,
		},
		OracleBase:             oracleBase,
		OracleQuote:            oracleQuote,
		OracleType:             oracleType,
		InitialMarginRatio:     sdk.NewDecWithPrec(5, 2), // default 5% initial margin ratio
		MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2), // default 2% maintenance margin ratio
	}
}

// SetupTest does all the initialization of data for exchange test
func SetupTest(app *simapp.InjectiveApp, ctx sdk.Context, spotMarketCount int, derivativeMarketCount int, binaryMarketCount int) (TestInput, sdk.Context) {
	app.ExchangeKeeper.SetParams(ctx, TestingExchangeParams)
	app.ExchangeKeeper.SetSpotExchangeEnabled(ctx)
	app.ExchangeKeeper.SetDerivativesExchangeEnabled(ctx)

	input := TestInput{
		Spots:                  make([]SpotMarket, 0),
		Perps:                  make([]PerpMarket, 0),
		BinaryMarkets:          make([]BinaryMarket, 0),
		ExpiryMarkets:          make([]ExpiryMarket, 0),
		MarketIDToSpotMarket:   make(map[common.Hash]*SpotMarket),
		MarketIDToPerpMarket:   make(map[common.Hash]*PerpMarket),
		MarketIDToExpiryMarket: make(map[common.Hash]*ExpiryMarket),
		MarketIDToBinaryMarket: make(map[common.Hash]*BinaryMarket),
		InitialBankSupply:      nil,
	}
	quoteDenomSupplies := sdk.Coins{}
	baseDenomSupplies := sdk.Coins{}

	//Initialize global constants
	for marketIndex := 0; marketIndex < spotMarketCount; marketIndex++ {
		baseDenom := "ETH" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT" + strconv.Itoa(marketIndex)
		marketID := exchangetypes.NewSpotMarketID(baseDenom, quoteDenom)

		spotMarket := SpotMarket{
			Market: Market{
				BaseDenom:           baseDenom,
				QuoteDenom:          quoteDenom,
				Ticker:              baseDenom + "/" + quoteDenom,
				MarketID:            marketID,
				MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
				MakerFeeRate:        sdk.NewDecWithPrec(1, 3), // default 0.1% maker fees
				TakerFeeRate:        sdk.NewDecWithPrec(3, 3), // default 0.3% taker fees
				IsActive:            true,
			},
		}

		input.MarketIDToSpotMarket[marketID] = &spotMarket
		input.Spots = append(input.Spots, spotMarket)
		quoteDenomSupplies = quoteDenomSupplies.Add(sdk.NewInt64Coin(quoteDenom, 400000000))
		baseDenomSupplies = baseDenomSupplies.Add(sdk.NewInt64Coin(baseDenom, 400000000))
	}

	for marketIndex := 0; marketIndex < derivativeMarketCount; marketIndex++ {
		baseDenom := "ETH" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT" + strconv.Itoa(marketIndex)
		ticker := baseDenom + "/" + quoteDenom
		oracleBase := "OracleBase for " + ticker
		oracleQuote := "OracleQuote for " + ticker
		oracleType := oracletypes.OracleType_PriceFeed
		marketID := exchangetypes.NewPerpetualMarketID(ticker, quoteDenom, oracleBase, oracleQuote, oracleType)

		perpMarket := PerpMarket{
			Market: Market{
				BaseDenom:           baseDenom,
				QuoteDenom:          quoteDenom,
				Ticker:              ticker,
				MarketID:            marketID,
				MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
				MakerFeeRate:        sdk.NewDecWithPrec(1, 3), // default 0.1% maker fees
				TakerFeeRate:        sdk.NewDecWithPrec(3, 3), // default 0.3% taker fees
				IsActive:            true,
			},
			OracleBase:             oracleBase,
			OracleQuote:            oracleQuote,
			OracleType:             oracleType,
			InitialMarginRatio:     sdk.NewDecWithPrec(5, 2), // default 5% initial margin ratio
			MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2), // default 2% maintenance margin ratio
		}

		input.MarketIDToPerpMarket[marketID] = &perpMarket
		input.Perps = append(input.Perps, perpMarket)
		quoteDenomSupplies = quoteDenomSupplies.Add(sdk.NewInt64Coin(quoteDenom, 400000000))
		baseDenomSupplies = baseDenomSupplies.Add(sdk.NewInt64Coin(baseDenom, 400000000))
	}

	for marketIndex := 0; marketIndex < derivativeMarketCount; marketIndex++ {
		baseDenom := "ETH/TEF" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT/TEF" + strconv.Itoa(marketIndex)
		ticker := baseDenom + "/" + quoteDenom
		oracleBase := "OracleBase for " + ticker
		oracleQuote := "OracleQuote for " + ticker
		oracleType := oracletypes.OracleType_PriceFeed
		expiry := ctx.BlockTime().Unix() + int64(FourWeeksInSeconds)
		marketID := exchangetypes.NewExpiryFuturesMarketID(ticker, quoteDenom, oracleBase, oracleQuote, oracleType, expiry)

		expiryMarket := ExpiryMarket{
			Market: Market{
				BaseDenom:           baseDenom,
				QuoteDenom:          quoteDenom,
				Ticker:              ticker,
				MarketID:            marketID,
				MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
				MakerFeeRate:        sdk.NewDecWithPrec(1, 3), // default 0.1% maker fees
				TakerFeeRate:        sdk.NewDecWithPrec(3, 3), // default 0.3% taker fees
				IsActive:            true,
			},
			OracleBase:             oracleBase,
			OracleQuote:            oracleQuote,
			OracleType:             oracleType,
			InitialMarginRatio:     sdk.NewDecWithPrec(5, 2), // default 5% initial margin ratio
			MaintenanceMarginRatio: sdk.NewDecWithPrec(2, 2), // default 2% maintenance margin ratio
			Expiry:                 expiry,
		}

		input.MarketIDToExpiryMarket[marketID] = &expiryMarket
		input.ExpiryMarkets = append(input.ExpiryMarkets, expiryMarket)
		quoteDenomSupplies = quoteDenomSupplies.Add(sdk.NewInt64Coin(quoteDenom, 400000000))
		baseDenomSupplies = baseDenomSupplies.Add(sdk.NewInt64Coin(baseDenom, 400000000))
	}

	for marketIndex := 0; marketIndex < binaryMarketCount; marketIndex++ {
		baseDenom := "ETH" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT" + strconv.Itoa(marketIndex)
		ticker := baseDenom + "/" + quoteDenom
		oracleSymbol := "oracleSymbol for " + ticker
		oracleProvider := "provider misha"
		oracleType := oracletypes.OracleType_Provider
		marketID := exchangetypes.NewBinaryOptionsMarketID(ticker, quoteDenom, oracleSymbol, oracleProvider, oracleType)
		expiry := ctx.BlockTime().Unix() + int64(FourWeeksInSeconds)
		settlement := expiry + 69

		binaryMarket := BinaryMarket{
			Market: Market{
				BaseDenom:           baseDenom,
				QuoteDenom:          quoteDenom,
				Ticker:              ticker,
				MarketID:            marketID,
				MinPriceTickSize:    sdk.NewDecWithPrec(1, 4),
				MinQuantityTickSize: sdk.NewDecWithPrec(1, 4),
				MakerFeeRate:        sdk.NewDecWithPrec(1, 3), // default 0.1% maker fees
				TakerFeeRate:        sdk.NewDecWithPrec(3, 3), // default 0.3% taker fees
				IsActive:            true,
			},
			OracleSymbol:        oracleSymbol,
			OracleProvider:      oracleProvider,
			OracleScaleFactor:   6,
			ExpirationTimestamp: expiry,
			SettlementTimestamp: settlement,
			Admin:               DefaultAddress,
			SettlementPrice:     nil,
		}

		input.MarketIDToBinaryMarket[marketID] = &binaryMarket
		input.BinaryMarkets = append(input.BinaryMarkets, binaryMarket)
		quoteDenomSupplies = quoteDenomSupplies.Add(sdk.NewInt64Coin(quoteDenom, 400000000))
	}

	coinSupplies := quoteDenomSupplies.Add(baseDenomSupplies...)
	OrFail(app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coinSupplies...)))

	// Initialize each of the buyer Accounts
	for i := range []int{0, 1, 2, 3, 4} {
		acc := app.AccountKeeper.NewAccount(
			ctx,
			authtypes.NewBaseAccount(BuyerAccAddrs[i], BuyerAccPubKeys[i], uint64(i), 0),
		)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, acc.GetAddress(), InitCoins)
		//input.BankKeeper.SetBalances(Ctx, acc.GetAddress(), InitCoins)
		app.AccountKeeper.SetAccount(ctx, acc)
	}

	// Initialize each of the seller Accounts
	for i := range []int{0, 1, 2, 3, 4} {
		acc := app.AccountKeeper.NewAccount(
			ctx,
			authtypes.NewBaseAccount(SellerAccAddrs[i], SellerAccPubKeys[i], uint64(i), 0),
		)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, acc.GetAddress(), InitCoins)
		//input.BankKeeper.SetBalances(Ctx, acc.GetAddress(), InitCoins)
		app.AccountKeeper.SetAccount(ctx, acc)
	}

	ctx, _ = EndBlockerAndCommit(app, ctx)

	exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
	app.DistrKeeper.SetFeePool(ctx, distrtypes.InitialFeePool())

	auction.InitGenesis(ctx, app.AuctionKeeper, *auctiontypes.DefaultGenesisState())

	initialBankSupply := sdk.Coins{}
	app.BankKeeper.IterateTotalSupply(ctx, func(coin sdk.Coin) bool {
		initialBankSupply = initialBankSupply.Add(coin)
		return false
	})
	input.InitialBankSupply = &initialBankSupply

	// Return the test input
	return input, ctx
}

func (testInput *TestInput) GetMarketIndexFromID(marketID string) int {
	for marketIndex, market := range testInput.Perps {
		if market.MarketID.Hex() == marketID {
			return marketIndex
		}
	}

	for marketIndex, market := range testInput.ExpiryMarkets {
		if market.MarketID.Hex() == marketID {
			return marketIndex
		}
	}

	for marketIndex, market := range testInput.BinaryMarkets {
		if market.MarketID.Hex() == marketID {
			return marketIndex
		}
	}

	return -1
}

func (testInput *TestInput) MintDerivativeDeposits(app *simapp.InjectiveApp, ctx sdk.Context, marketIndex int, trader common.Hash, amount *sdk.Dec, isTimeExpiry bool) {
	var market Market
	if isTimeExpiry {
		market = testInput.ExpiryMarkets[marketIndex].Market
	} else {
		market = testInput.Perps[marketIndex].Market
	}

	mintAmount := startingDeposit
	denom := market.QuoteDenom
	if amount != nil {
		mintAmount = *amount
	}

	if exchangetypes.IsDefaultSubaccountID(trader) {
		funds := sdk.NewCoins(sdk.NewCoin(denom, mintAmount.TruncateInt()))
		OrFail(app.BankKeeper.MintCoins(ctx, exchangetypes.ModuleName, funds))

		address := exchangetypes.SubaccountIDToSdkAddress(trader)
		OrFail(app.BankKeeper.SendCoinsFromModuleToAccount(ctx, exchangetypes.ModuleName, address, funds))
		return
	}

	deposit := app.ExchangeKeeper.GetDeposit(ctx, trader, denom)
	deposit.AvailableBalance = deposit.AvailableBalance.Add(mintAmount)
	deposit.TotalBalance = deposit.TotalBalance.Add(mintAmount)

	app.ExchangeKeeper.SetDepositOrSendToBank(ctx, trader, denom, *deposit, false)
}

// mints bank balances if default subaccount, otherwise mints to subaccount deposits
func (testInput *TestInput) mintSpotDeposits(app *simapp.InjectiveApp, ctx sdk.Context, marketIndex int, trader common.Hash, amount *sdk.Dec) {
	market := testInput.Spots[marketIndex].Market

	mintAmount := startingDeposit
	if amount != nil {
		mintAmount = *amount
	}

	if exchangetypes.IsDefaultSubaccountID(trader) {
		quoteFunds := sdk.NewCoins(sdk.NewCoin(market.QuoteDenom, mintAmount.TruncateInt()))
		baseFunds := sdk.NewCoins(sdk.NewCoin(market.BaseDenom, mintAmount.TruncateInt()))
		OrFail(app.BankKeeper.MintCoins(ctx, exchangetypes.ModuleName, quoteFunds))
		OrFail(app.BankKeeper.MintCoins(ctx, exchangetypes.ModuleName, baseFunds))

		address := exchangetypes.SubaccountIDToSdkAddress(trader)
		OrFail(app.BankKeeper.SendCoinsFromModuleToAccount(ctx, exchangetypes.ModuleName, address, quoteFunds))
		OrFail(app.BankKeeper.SendCoinsFromModuleToAccount(ctx, exchangetypes.ModuleName, address, baseFunds))
		return
	}

	quoteDeposit := &exchangetypes.Deposit{
		AvailableBalance: mintAmount,
		TotalBalance:     mintAmount,
	}

	baseDeposit := &exchangetypes.Deposit{
		AvailableBalance: mintAmount,
		TotalBalance:     mintAmount,
	}

	app.ExchangeKeeper.SetDeposit(ctx, trader, market.QuoteDenom, quoteDeposit)
	app.ExchangeKeeper.SetDeposit(ctx, trader, market.BaseDenom, baseDeposit)
}

func (testInput *TestInput) nukeAvailableDeposits(app *simapp.InjectiveApp, ctx sdk.Context, denom string, trader common.Hash) {
	deposit := app.ExchangeKeeper.GetDeposit(ctx, trader, denom)

	if deposit.AvailableBalance.IsPositive() {
		decreaseAmount := deposit.AvailableBalance
		deposit.TotalBalance = deposit.TotalBalance.Sub(decreaseAmount)
		deposit.AvailableBalance = sdk.ZeroDec()

		// send to a random account so the accounting is still legit
		randomSubaccountID := common.HexToHash("0xd2fe5d33615a1c52c08018c47e8bc53646a0e101000000011100000000001111")
		OrFail(app.ExchangeKeeper.IncrementDepositForNonDefaultSubaccount(ctx, randomSubaccountID, denom, decreaseAmount))
		app.ExchangeKeeper.SetDeposit(ctx, trader, denom, deposit)
	}
}

// zeroes out balances if default subaccount, otherwise withdraws the available subaccount deposits
func (testInput *TestInput) withdrawDeposits(app *simapp.InjectiveApp, ctx sdk.Context, trader common.Hash, denom string) {

	testInput.nukeAvailableDeposits(app, ctx, denom, trader)

	if exchangetypes.IsDefaultSubaccountID(trader) {
		address := exchangetypes.SubaccountIDToSdkAddress(trader)

		balance := app.BankKeeper.GetBalance(ctx, address, denom)

		// burn the coins to keep legit accounting
		OrFail(app.BankKeeper.SendCoinsFromAccountToModule(ctx, address, exchangetypes.ModuleName, sdk.NewCoins(balance)))
		OrFail(app.BankKeeper.BurnCoins(ctx, exchangetypes.ModuleName, sdk.NewCoins(balance)))
	}
}

// func (testInput *TestInput) AddSpotDepositsForSubaccounts(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int, subaccounts []common.Hash) {
// 	for i := 0; i < 16; i++ {
// 		for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
// 			for _, subaccount := range subaccounts {
// 				testInput.mintSpotDeposits(app, ctx, marketIndex, subaccount, nil)
// 			}
// 		}
// 	}
// }

func (testInput *TestInput) AddSpotDeposits(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int) {
	for i := 0; i < 16; i++ {
		for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
			for j := 0; j < 16; j++ {
				trader := common.HexToHash(strconv.FormatInt(int64(i), 16) + "27aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(j), 16))
				testInput.mintSpotDeposits(app, ctx, marketIndex, trader, nil)
			}
		}
	}
}

// AddSpotDepositsForSubaccounts only mints deposits for explicit input subaccounts with an optional mint amount (uses default test value if nil)
func (testInput *TestInput) AddSpotDepositsForSubaccounts(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int, amount *sdk.Dec, subaccounts []common.Hash) {
	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		for _, subaccount := range subaccounts {
			testInput.mintSpotDeposits(app, ctx, marketIndex, subaccount, amount)
		}
	}
}

// WithdrawSpotDepositsForSubaccounts clears out the available or bank balances for the subaccounts
func (testInput *TestInput) WithdrawSpotDepositsForSubaccounts(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int, subaccounts []common.Hash) {
	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		market := testInput.Spots[marketIndex].Market
		for _, subaccount := range subaccounts {
			testInput.withdrawDeposits(app, ctx, subaccount, market.BaseDenom)
			testInput.withdrawDeposits(app, ctx, subaccount, market.QuoteDenom)
		}
	}
}

// WithdrawDerivativeDepositsForSubaccounts clears out the available or bank balances for the subaccounts
func (testInput *TestInput) WithdrawDerivativeDepositsForSubaccounts(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int, isExpiry bool, subaccounts []common.Hash) {
	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		var market Market
		if isExpiry {
			market = testInput.ExpiryMarkets[marketIndex].Market
		} else {
			market = testInput.Perps[marketIndex].Market
		}

		for _, subaccount := range subaccounts {
			testInput.withdrawDeposits(app, ctx, subaccount, market.QuoteDenom)
		}
	}
}

func (testInput *TestInput) AddDerivativeDeposits(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int, isTimeExpiry bool) {
	for i := 0; i < 16; i++ {
		for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
			for j := 0; j < 16; j++ {
				trader := common.HexToHash(strconv.FormatInt(int64(i), 16) + "27aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(j), 16))
				testInput.MintDerivativeDeposits(app, ctx, marketIndex, trader, nil, isTimeExpiry)
			}
		}
	}
}

// AddDerivativeDepositsForSubaccounts only mints deposits for explicit input subaccounts with an optional mint amount (uses default test value if nil)
func (testInput *TestInput) AddDerivativeDepositsForSubaccounts(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int, amount *sdk.Dec, isTimeExpiry bool, subaccounts []common.Hash) {
	for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
		for _, subaccount := range subaccounts {
			testInput.MintDerivativeDeposits(app, ctx, marketIndex, subaccount, amount, isTimeExpiry)
		}
	}
}

func (testInput *TestInput) mintBinaryOptionsDeposits(app *simapp.InjectiveApp, ctx sdk.Context, marketIndex int, trader common.Hash) {
	market := testInput.BinaryMarkets[marketIndex].Market

	quoteDeposit := &exchangetypes.Deposit{
		AvailableBalance: startingDeposit,
		TotalBalance:     startingDeposit,
	}

	app.ExchangeKeeper.SetDeposit(ctx, trader, market.QuoteDenom, quoteDeposit)
}

func (testInput *TestInput) AddBinaryOptionsDeposits(app *simapp.InjectiveApp, ctx sdk.Context, marketCount int) {
	for i := 0; i < 16; i++ {
		for marketIndex := 0; marketIndex < marketCount; marketIndex++ {
			for j := 0; j < 16; j++ {
				trader := common.HexToHash(strconv.FormatInt(int64(i), 16) + "27aee334987c52fa7b567b2662bdbb68614e48c00000000000000000000000" + strconv.FormatInt(int64(j), 16))
				testInput.mintBinaryOptionsDeposits(app, ctx, marketIndex, trader)
			}
		}
	}
}

// MakeTestCodec creates a legacy amino codec for testing
func MakeTestCodec() *codec.LegacyAmino {
	var cdc = codec.NewLegacyAmino()
	auth.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	bank.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	staking.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	distribution.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	sdk.RegisterLegacyAminoCodec(cdc)
	ccodec.RegisterCrypto(cdc)
	params.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	types.RegisterLegacyAminoCodec(cdc)
	return cdc
}

// MakeTestMarshaler creates a proto codec for use in testing
func MakeTestMarshaler() codec.Codec {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)

	return codec.NewProtoCodec(interfaceRegistry)
}

func EndBlockerAndCommit(app *simapp.InjectiveApp, ctx sdk.Context) (sdk.Context, []abci.Event) {
	response := app.EndBlocker(ctx, abci.RequestEndBlock{
		Height: ctx.BlockHeight(),
	})

	exchangeTStore := ctx.TransientStore(app.ExchangeKeeper.GetTransientStoreKey())
	iterator := exchangeTStore.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		exchangeTStore.Delete(iterator.Key())
	}

	ocrTStore := ctx.TransientStore(app.OcrKeeper.GetTransientStoreKey())
	iterator = ocrTStore.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		ocrTStore.Delete(iterator.Key())
	}

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	return ctx, response.Events
}

// utils
func (testInput *TestInput) NewMsgCreateSpotLimitOrder(price sdk.Dec, quantity sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash) *exchangetypes.MsgCreateSpotLimitOrder {
	return testInput.NewMsgCreateSpotLimitOrderForMarketIndex(price, quantity, orderType, subaccountID, 0)
}

func NewBareSpotLimitOrder(price, quantity sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash) *BareSpotLimitOrder {
	return &BareSpotLimitOrder{
		Price:        price,
		Quantity:     quantity,
		OrderType:    orderType,
		SubaccountID: subaccountID,
	}
}

func NewBareSpotLimitOrderFromString(price string, quantity string, orderType exchangetypes.OrderType, subaccountID common.Hash) *BareSpotLimitOrder {
	return NewBareSpotLimitOrder(sdk.MustNewDecFromStr(price), sdk.MustNewDecFromStr(quantity), orderType, subaccountID)
}

type BareSpotLimitOrder struct {
	Price        sdk.Dec
	Quantity     sdk.Dec
	OrderType    exchangetypes.OrderType
	SubaccountID common.Hash
}

func (testInput *TestInput) NewListOfMsgCreateSpotLimitOrderForMarketIndex(
	marketIndex int,
	bareOrders ...*BareSpotLimitOrder,
) []*exchangetypes.MsgCreateSpotLimitOrder {
	msgs := make([]*exchangetypes.MsgCreateSpotLimitOrder, len(bareOrders))
	for idx, order := range bareOrders {
		msgs[idx] = testInput.NewMsgCreateSpotLimitOrderForMarketIndex(order.Price, order.Quantity, order.OrderType, order.SubaccountID, marketIndex)
	}
	return msgs
}

func (testInput *TestInput) NewMsgCreateSpotLimitOrderForMarketIndex(price sdk.Dec, quantity sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash, marketIndex int) *exchangetypes.MsgCreateSpotLimitOrder {
	sender := exchangetypes.SubaccountIDToSdkAddress(subaccountID).String()

	msg := exchangetypes.MsgCreateSpotLimitOrder{
		Sender: sender,
		Order: exchangetypes.SpotOrder{
			MarketId: testInput.Spots[marketIndex].MarketID.Hex(),
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				FeeRecipient: DefaultAddress,
				Price:        price,
				Quantity:     quantity,
			},
			OrderType:    orderType,
			TriggerPrice: nil,
		},
	}
	OrFail(msg.ValidateBasic())
	return &msg
}

func (testInput *TestInput) NewMsgCreateSpotMarketOrder(quantity, worstPrice sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash) *exchangetypes.MsgCreateSpotMarketOrder {
	return testInput.NewMsgCreateSpotMarketOrderForMarketIndex(worstPrice, quantity, orderType, subaccountID, 0)
}

func (testInput *TestInput) NewMsgCreateSpotMarketOrderForMarketIndex(worstPrice sdk.Dec, quantity sdk.Dec, orderType exchangetypes.OrderType, subaccountID common.Hash, marketIndex int) *exchangetypes.MsgCreateSpotMarketOrder {
	sender := exchangetypes.SubaccountIDToSdkAddress(subaccountID).String()

	msg := exchangetypes.MsgCreateSpotMarketOrder{
		Sender: sender,
		Order: exchangetypes.SpotOrder{
			MarketId: common.Bytes2Hex(testInput.Spots[marketIndex].MarketID.Bytes()),
			OrderInfo: exchangetypes.OrderInfo{
				SubaccountId: subaccountID.Hex(),
				FeeRecipient: DefaultAddress,
				Price:        worstPrice,
				Quantity:     quantity,
			},
			OrderType:    orderType,
			TriggerPrice: nil,
		},
	}
	OrFail(msg.ValidateBasic())
	return &msg
}

func AddTradeRewardCampaign(
	testInput TestInput,
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	marketIds,
	quoteDenoms []string,
	marketMultipliers []exchangetypes.PointsMultiplier,
	isNew, isSpot bool,
) {
	handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
	title := "Trade Reward Campaign"
	description := "Trade Reward Campaign"

	var tradingRewardBoostInfo exchangetypes.TradingRewardCampaignBoostInfo

	if isSpot {
		tradingRewardBoostInfo = exchangetypes.TradingRewardCampaignBoostInfo{
			BoostedSpotMarketIds:  marketIds,
			SpotMarketMultipliers: marketMultipliers,
		}
	} else {
		tradingRewardBoostInfo = exchangetypes.TradingRewardCampaignBoostInfo{
			BoostedDerivativeMarketIds:  marketIds,
			DerivativeMarketMultipliers: marketMultipliers,
		}
	}

	campaignInfo := &exchangetypes.TradingRewardCampaignInfo{
		CampaignDurationSeconds: 30000,
		QuoteDenoms:             quoteDenoms,
		TradingRewardBoostInfo:  &tradingRewardBoostInfo,
		DisqualifiedMarketIds:   nil,
	}

	var proposal govtypes.Content

	if !isNew {
		proposal = (govtypes.Content)(&exchangetypes.TradingRewardCampaignUpdateProposal{
			Title:        title,
			Description:  description,
			CampaignInfo: campaignInfo,
		})
	} else {
		proposal = (govtypes.Content)(&exchangetypes.TradingRewardCampaignLaunchProposal{
			Title:        title,
			Description:  description,
			CampaignInfo: campaignInfo,
			CampaignRewardPools: []*exchangetypes.CampaignRewardPool{{
				MaxCampaignRewards: sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100000))),
				StartTimestamp:     ctx.BlockTime().Unix() + 100,
			}},
		})
	}

	err := handler(ctx, proposal)
	OrFail(err)
}

func AddFeeDiscount(testInput TestInput, app *simapp.InjectiveApp, ctx sdk.Context, marketIds, quoteDenoms []string) {
	bucketCount := uint64(5)
	bucketDuration := int64(100)
	hasFullPastBuckets := true

	tierInfos := []*exchangetypes.FeeDiscountTierInfo{{
		MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
		TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
		StakedAmount:      sdk.NewInt(100),
		Volume:            sdk.MustNewDecFromStr("0.3"),
	}}

	defaultAddress, err := sdk.AccAddressFromBech32(DefaultAddress)
	OrFail(err)
	AddFeeDiscountWithSettingPastBuckets(testInput, app, ctx, marketIds, quoteDenoms, hasFullPastBuckets, tierInfos, bucketCount, bucketDuration, defaultAddress)
}

func AddFeeDiscountForAddress(testInput TestInput, app *simapp.InjectiveApp, ctx sdk.Context, marketIds, quoteDenoms []string, address sdk.AccAddress) {
	bucketCount := uint64(5)
	bucketDuration := int64(100)
	hasFullPastBuckets := true

	tierInfos := []*exchangetypes.FeeDiscountTierInfo{{
		MakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
		TakerDiscountRate: sdk.MustNewDecFromStr("0.1"),
		StakedAmount:      sdk.NewInt(100),
		Volume:            sdk.MustNewDecFromStr("0.3"),
	}}

	AddFeeDiscountWithSettingPastBuckets(testInput, app, ctx, marketIds, quoteDenoms, hasFullPastBuckets, tierInfos, bucketCount, bucketDuration, address)
}

func DelegateStake(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	traderAddress, stakeDenom string,
	stakingAmount sdkmath.Int,
) {
	msgServer := stakingkeeper.NewMsgServerImpl(app.StakingKeeper)

	privKey := secp256k1.GenPrivKey()
	pubKey, err := codectypes.NewAnyWithValue(privKey.PubKey())
	OrFail(err)
	account, err := sdk.AccAddressFromBech32(DefaultValidatorDelegatorAddress)
	OrFail(err)

	stakingCoins := sdk.NewCoins(sdk.NewCoin(stakeDenom, stakingAmount))
	err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, stakingCoins)
	OrFail(err)
	err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, account, stakingCoins)
	OrFail(err)

	valAddressAcc, err := sdk.ValAddressFromBech32(DefaultValidatorAddress)
	OrFail(err)
	_, alreadyExists := app.StakingKeeper.GetValidator(ctx, valAddressAcc)

	if !alreadyExists {

		createValidatorMsg := &stakingtypes.MsgCreateValidator{
			Description:       stakingtypes.Description{Moniker: "just a validator"},
			Commission:        stakingtypes.NewCommissionRates(sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.1")),
			MinSelfDelegation: sdk.NewInt(1),
			DelegatorAddress:  DefaultValidatorDelegatorAddress,
			ValidatorAddress:  DefaultValidatorAddress,
			Value:             sdk.NewCoin(stakeDenom, stakingAmount),
			Pubkey:            pubKey,
		}

		_, err = msgServer.CreateValidator(sdk.WrapSDKContext(ctx), createValidatorMsg)
		OrFail(err)
	}

	trader, err := sdk.AccAddressFromBech32(traderAddress)
	OrFail(err)

	stakedCoins := sdk.NewCoins(sdk.NewCoin(stakeDenom, stakingAmount))
	err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, stakedCoins)
	OrFail(err)
	err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, trader, stakedCoins)
	OrFail(err)

	delegateMsg := &stakingtypes.MsgDelegate{
		DelegatorAddress: traderAddress,
		ValidatorAddress: DefaultValidatorAddress,
		Amount:           sdk.NewCoin(stakeDenom, stakingAmount),
	}
	_, err = msgServer.Delegate(sdk.WrapSDKContext(ctx), delegateMsg)
	OrFail(err)
}

func AddFeeDiscountWithSettingPastBuckets(
	testInput TestInput,
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	marketIds,
	quoteDenoms []string,
	hasFullPastBuckets bool,
	tierInfos []*exchangetypes.FeeDiscountTierInfo,
	bucketCount uint64,
	bucketDuration int64,
	address sdk.AccAddress,
) {
	proposal := exchangetypes.FeeDiscountProposal{
		Title:       "Fee Discount",
		Description: "Fee Discount",
		Schedule: &exchangetypes.FeeDiscountSchedule{
			BucketCount:           bucketCount,
			BucketDuration:        bucketDuration,
			QuoteDenoms:           quoteDenoms,
			TierInfos:             tierInfos,
			DisqualifiedMarketIds: nil,
		},
	}
	handler := exchange.NewExchangeProposalHandler(app.ExchangeKeeper)
	err := handler(ctx, (govtypes.Content)(&proposal))
	OrFail(err)

	DelegateStake(app, ctx, DefaultAddress, "inj", sdk.NewInt(1000))

	if hasFullPastBuckets {
		pastVolumeBuckets := make([]sdk.Dec, 0)
		for i := 0; i < int(bucketCount); i++ {
			feeAmount := sdk.NewDec(int64(i + 1))
			pastVolumeBuckets = append(pastVolumeBuckets, feeAmount)
		}

		SetPastBucketTotalVolumeForTrader(testInput, app, ctx, address, bucketDuration, pastVolumeBuckets)
	}
}

func SetPastBucketTotalVolumeForTrader(
	testInput TestInput,
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	trader sdk.AccAddress,
	bucketDuration int64,
	pastVolumeBuckets []sdk.Dec,
) {
	bucketStartTimestamp := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	totalPastFees := sdk.ZeroDec()

	for i := 0; i < len(pastVolumeBuckets); i++ {
		feeAmount := pastVolumeBuckets[i]
		app.ExchangeKeeper.UpdateFeeDiscountAccountVolumeInBucket(ctx, trader, bucketStartTimestamp, feeAmount)

		totalPastFees = totalPastFees.Add(feeAmount)
		bucketStartTimestamp = bucketStartTimestamp - bucketDuration
	}

	app.ExchangeKeeper.SetPastBucketTotalVolume(ctx, trader, totalPastFees)
}

var GetRequiredBinaryOptionsMargin = func(position *exchangetypes.Position, oracleScaleFactor uint32) sdk.Dec {
	// Margin = Price * Quantity for buys
	if position.IsLong {
		notional := position.EntryPrice.Mul(position.Quantity)
		return notional
	}
	// Margin = (scaled(1) - Price) * Quantity for sells
	return position.Quantity.Mul(exchangetypes.GetScaledPrice(sdk.OneDec(), oracleScaleFactor).Sub(position.EntryPrice))
}

// InvariantCheckBalanceAndSupply ensures that total supply of coins matches total balance of all accounts
var InvariantCheckBalanceAndSupply = func(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	testInput *TestInput,
) {
	currentBankSupply := sdk.Coins{}
	app.BankKeeper.IterateTotalSupply(ctx, func(coin sdk.Coin) bool {
		currentBankSupply = currentBankSupply.Add(coin)
		return false
	})

	// ensure Funds Total Supply = sum_of(individual_accounts) + exchange module account + insurance fund account + auction module account
	individualAccountsBalance := sdk.Coins{}

	allBalances := app.BankKeeper.GetAccountsBalances(ctx)
	for _, balance := range allBalances {
		coins := balance.Coins
		individualAccountsBalance = individualAccountsBalance.Add(coins...)
	}

	Expect(currentBankSupply.String()).To(BeEquivalentTo(individualAccountsBalance.String()))
}

// InvariantCheckInsuranceModuleBalance ensures insurance fund module account balance = sum_of(individual_insurance_fund_records)
var InvariantCheckInsuranceModuleBalance = func(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
) {

	insuranceFundSum := sdk.NewCoins()
	insuranceFunds := app.InsuranceKeeper.GetAllInsuranceFunds(ctx)
	for _, fund := range insuranceFunds {
		insuranceFundSum = insuranceFundSum.Add(sdk.NewCoin(fund.DepositDenom, fund.Balance))
	}
	insuranceRedemptions := app.InsuranceKeeper.GetAllInsuranceFundRedemptions(ctx)
	for _, redemption := range insuranceRedemptions {
		insuranceFundSum = insuranceFundSum.Add(redemption.RedemptionAmount)
	}
	insuranceModuleAccountBalance := app.BankKeeper.GetAllBalances(ctx, app.AccountKeeper.GetModuleAddress(insurancetypes.ModuleName))

	insuranceModuleAccountBalance = removeLPTokens(insuranceModuleAccountBalance)

	if len(insuranceModuleAccountBalance) > 0 {
		Expect(insuranceModuleAccountBalance).To(BeEquivalentTo(insuranceFundSum))
	} else {
		Expect(insuranceFundSum).To(BeEmpty())
	}
}

// InvariantCheckAccountFees verifies that total of volumes recorded in each bucket matches total volumes calculated per account
var InvariantCheckAccountFees = func(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
) {

	allAccountVolume := app.ExchangeKeeper.GetAllAccountVolumeInAllBuckets(ctx)

	allAccounts := make([]string, 0)
	accountFees := make(map[string]sdk.Dec)

	for _, accountVolume := range allAccountVolume {
		for _, feesInBucket := range accountVolume.AccountVolume {
			if _, ok := accountFees[feesInBucket.Account]; !ok {
				accountFees[feesInBucket.Account] = sdk.ZeroDec()
			} else {
				allAccounts = append(allAccounts, feesInBucket.Account)
			}

			if accountFees[feesInBucket.Account].IsNil() {
				accountFees[feesInBucket.Account] = sdk.ZeroDec()
			}

			accountFees[feesInBucket.Account] = accountFees[feesInBucket.Account].Add(feesInBucket.Volume)
		}
	}

	currBucketStartTimestamp := app.ExchangeKeeper.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	for _, account := range allAccounts {
		accAccount, err := sdk.AccAddressFromBech32(account)
		OrFail(err)

		accountFeesInStore := app.ExchangeKeeper.GetFeeDiscountTotalAccountVolume(ctx, accAccount, currBucketStartTimestamp)
		Expect(accountFeesInStore.String()).Should(Equal(accountFees[account].String()))
	}
}

func MintAndDeposit(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	subaccountID string,
	amounts sdk.Coins,
) {
	for _, amount := range amounts {
		app.BankKeeper.MintCoins(ctx, exchangetypes.ModuleName, sdk.NewCoins(amount))
		deposit := exchangetypes.Deposit{AvailableBalance: amount.Amount.ToDec(), TotalBalance: amount.Amount.ToDec()}
		app.ExchangeKeeper.SetDepositOrSendToBank(ctx, common.HexToHash(subaccountID), amount.Denom, deposit, false)
	}
}

func RemoveFunds(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	subaccount common.Hash,
	amount sdk.Coin,
) error {
	if exchangetypes.IsDefaultSubaccountID(subaccount) {
		return app.BankKeeper.SendCoinsFromAccountToModule(ctx,
			exchangetypes.SubaccountIDToSdkAddress(subaccount),
			exchangetypes.ModuleName,
			sdk.Coins{amount},
		)
	}

	return app.ExchangeKeeper.ExecuteWithdraw(ctx, &exchangetypes.MsgWithdraw{
		Sender:       sdk.AccAddress(subaccount.Bytes()).String(),
		SubaccountId: subaccount.Hex(),
		Amount:       amount,
	})
}

func GetBankAndDepositFunds(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	subaccountID common.Hash,
	denom string,
) *exchangetypes.Deposit {
	subaccountDeposits := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, denom)
	if !exchangetypes.IsDefaultSubaccountID(subaccountID) {
		return subaccountDeposits
	}

	// combine bankBalance + dust from subaccount deposits to get the total spendable funds
	bankBalance := app.BankKeeper.GetBalance(ctx, exchangetypes.SubaccountIDToSdkAddress(subaccountID), denom)
	subaccountDeposits.AvailableBalance = bankBalance.Amount.ToDec().Add(subaccountDeposits.AvailableBalance)
	subaccountDeposits.TotalBalance = bankBalance.Amount.ToDec().Add(subaccountDeposits.TotalBalance)
	return subaccountDeposits
}

func GetInsufficientFundsErrorMessage(
	subaccountID common.Hash,
	denom string,
	availableAmount sdk.Dec,
	requestedAmount sdk.Dec,
) string {
	if exchangetypes.IsDefaultSubaccountID(subaccountID) {
		return fmt.Sprintf("%s%s is smaller than %s%s: insufficient funds", availableAmount.TruncateInt().String(), denom, requestedAmount.Ceil().TruncateInt().String(), denom)
	} else {
		return fmt.Sprintf("Insufficient Deposits for subaccountID %s asset %s. Balance decrement %s exceeds Available Balance %s : %s", subaccountID, denom, requestedAmount.String(), availableAmount.String(), exchangetypes.ErrInsufficientDeposit.Error())
	}
}

func IsExpectedInsufficientFundsErrorType(
	subaccountID common.Hash,
	err error,
) bool {
	if exchangetypes.IsDefaultSubaccountID(subaccountID) {
		return exchangetypes.ErrInsufficientFunds.Is(err)
	} else {
		return exchangetypes.ErrInsufficientDeposit.Is(err)
	}
}

func ShouldIgnoreEvent(emittedABCIEvent abcitypes.Event) bool {
	switch emittedABCIEvent.Type {
	case banktypes.EventTypeCoinReceived, banktypes.EventTypeCoinMint, banktypes.EventTypeCoinSpent, banktypes.EventTypeTransfer, sdk.EventTypeMessage:
		return true
	}
	return false
}
