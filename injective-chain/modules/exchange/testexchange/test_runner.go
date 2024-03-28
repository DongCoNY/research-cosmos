package testexchange

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cometbft/cometbft/crypto"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptokeys "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	auctiontypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	tokenfactorytypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

type Account struct {
	AccAddress    sdk.AccAddress
	SubaccountIDs []string
}

type Action struct {
	name   string
	weight int
	f      func(int)
}

var FeeRecipient = "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"
var ValidatorAddress []byte

var (
	masterContractAddress, vaultSubaccountId string
)
var VaultContractAddress string

func matchTick(base, minTick sdk.Dec) sdk.Dec {
	rounded := base.Quo(minTick).Ceil().Mul(minTick)
	return rounded
}

func initTokenFactory(
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	tfParams *TestPlayerTokenFactoryParams,
) {
	initParams := tokenfactorytypes.Params{
		DenomCreationFee: nil,
	}
	if tfParams != nil && tfParams.DenomCreationFee != nil {
		initParams.DenomCreationFee = sdk.Coins{(*tfParams.DenomCreationFee).toSdkCoin()}
	}

	app.TokenFactoryKeeper.SetParams(ctx, initParams)
}

func initAuctionModule(app *simapp.InjectiveApp, ctx sdk.Context) {
	auctionParams := auctiontypes.Params{
		AuctionPeriod:           5,
		MinNextBidIncrementRate: sdk.NewDecWithPrec(25, 4),
	}
	app.AuctionKeeper.SetParams(ctx, auctionParams)
	app.AuctionKeeper.InitEndingTimeStamp(ctx)
	app.AuctionKeeper.DeleteBid(ctx)
	app.AuctionKeeper.SetAuctionRound(ctx, 0)
}

func initTradingRewards(
	_ TestInput,
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	rewardCoins sdk.Coins,
) {
	funder, _ := sdk.AccAddressFromBech32("inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt")

	app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewardCoins)
	app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, funder, rewardCoins)

	err := app.DistrKeeper.FundCommunityPool(ctx, rewardCoins, funder)
	OrFail(err)
}

func initStaking(_ TestInput, app *simapp.InjectiveApp, ctx sdk.Context, accounts []Account) {
	pubKey, err := codectypes.NewAnyWithValue(cryptokeys.GenPrivKey().PubKey())
	OrFail(err)

	val_account_1, err := sdk.AccAddressFromBech32(DefaultValidatorDelegatorAddress)
	OrFail(err)

	mintCoins := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(1000)))
	stakingCoins := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(100)))
	err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, mintCoins)
	OrFail(err)
	err = app.BankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		minttypes.ModuleName,
		val_account_1,
		stakingCoins,
	)
	OrFail(err)

	msg := &stakingtypes.MsgCreateValidator{
		Description: stakingtypes.Description{Moniker: "just a validator"},
		Commission: stakingtypes.NewCommissionRates(
			sdk.MustNewDecFromStr("0.1"),
			sdk.MustNewDecFromStr("0.1"),
			sdk.MustNewDecFromStr("0.1"),
		),
		MinSelfDelegation: sdk.NewInt(1),
		DelegatorAddress:  DefaultValidatorDelegatorAddress,
		ValidatorAddress:  DefaultValidatorAddress,
		Value:             sdk.NewCoin("inj", sdk.NewInt(100)),
		Pubkey:            pubKey,
	}

	err = msg.ValidateBasic()
	OrFail(err)

	msgServer := stakingkeeper.NewMsgServerImpl(app.StakingKeeper)
	_, err = msgServer.CreateValidator(sdk.WrapSDKContext(ctx), msg)
	OrFail(err)

	stakingMintTraderCoins := sdk.NewCoins(
		sdk.NewCoin("inj", sdk.NewInt(10_000_000_000).MulRaw(int64(len(accounts)))),
	)
	err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, stakingMintTraderCoins)
	OrFail(err)
}

func InitContractRegistry(app *simapp.InjectiveApp, ctx sdk.Context) {
	app.WasmxKeeper.SetParams(ctx, wasmxtypes.Params{
		IsExecutionEnabled:    true,
		MaxBeginBlockTotalGas: 42000000,
		MaxContractGasLimit:   15000000,
		MinGasPrice:           1000,
	})
	//res := make([]paramstypes.ParamChange, 4)
	//res[0] = paramstypes.NewParamChange("xwasm", "MinGasPrice", string(`"1000"`))
	//res[1] = paramstypes.NewParamChange("xwasm", "IsExecutionEnabled", string(`true`))
	//res[2] = paramstypes.NewParamChange("xwasm", "MaxBeginBlockTotalGas", string(`"42000000"`))
	//res[3] = paramstypes.NewParamChange("xwasm", "MaxContractGasLimit", string(`"15000000"`))
	//addRegistryAddressParamProposal := &paramstypes.ParameterChangeProposal{
	//	Title:       "Init registry contract",
	//	Description: "Init registry contract",
	//	Changes:     res,
	//}
	//
	//handler := params.NewParamChangeProposalHandler(app.ParamsKeeper)
	//err := handler(ctx, addRegistryAddressParamProposal)
	//OrFail(err)
}

func (tp *TestPlayer) initMito(
	_ TestInput,
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	accounts []Account,
	marketID common.Hash,
) {
	sender := accounts[0].AccAddress.String()
	contractAddr := accounts[0].AccAddress.String()

	var (
		//registryCodeID uint64 = 1
		masterCodeID uint64 = 2
		vaultCodeID  uint64 = 3
	)
	_, file, _, _ := runtime.Caller(0)
	currentDir := filepath.Dir(file)

	storeMasterCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender,
		WASMByteCode: keeper.ReadFile(currentDir + "/../wasm/mito_master.wasm"),
	}
	_, err := tp.MsgServerWasm.StoreCode(
		sdk.WrapSDKContext(ctx),
		storeMasterCodeMsg,
	)
	OrFail(err)

	storeDerivativeVaultCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender,
		WASMByteCode: keeper.ReadFile(currentDir + "/../wasm/mito_vault_derivatives.wasm"),
	}
	_, err = tp.MsgServerWasm.StoreCode(
		sdk.WrapSDKContext(ctx),
		storeDerivativeVaultCodeMsg,
	)
	OrFail(err)

	storeSpotVaultCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender,
		WASMByteCode: keeper.ReadFile(currentDir + "/../wasm/mito_vault_spot.wasm"),
	}
	_, err = tp.MsgServerWasm.StoreCode(
		sdk.WrapSDKContext(ctx),
		storeSpotVaultCodeMsg,
	)
	OrFail(err)

	initMasterMessage := InstantiateMasterMsg{
		Owner:                sender,
		DistributionContract: contractAddr, // TODO
		MitoToken:            contractAddr, // TODO
	}
	byteMaster, err := json.Marshal(initMasterMessage)
	OrFail(err)
	instantiateMasterMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender,
		Admin:  sender,
		CodeID: masterCodeID,
		Label:  "master",
		Msg:    wasmtypes.RawContractMessage(string(byteMaster)),
		Funds:  sdk.NewCoins(),
	}

	_, err = tp.MsgServerWasm.InstantiateContract(
		sdk.WrapSDKContext(ctx),
		instantiateMasterMsg,
	)
	OrFail(err)
	// fmt.Println("👉 Instantiated contract: ", resp.Address)

	app.WasmKeeper.IterateContractsByCode(ctx, masterCodeID, func(addr sdk.AccAddress) bool {
		masterContractAddress = addr.String()
		// fmt.Println("👉 masterContractAddress: ", masterContractAddress)
		return true
	})

	var masterAcc sdk.AccAddress
	masterAcc, err = sdk.AccAddressFromBech32(masterContractAddress)
	OrFail(err)
	var vaultSubaccountIdHash *common.Hash
	vaultSubaccountIdHash, err = exchangetypes.SdkAddressWithNonceToSubaccountID(masterAcc, 1)
	OrFail(err)
	vaultSubaccountId = vaultSubaccountIdHash.Hex()
	// fmt.Println("👉 VaultSubaccountId: ", VaultSubaccountId)

	initVaultMessage := DerivativeVaultInstantiateMsg{
		ConfigOwner:                         sender,
		MasterAddress:                       masterContractAddress,
		MarketId:                            marketID.Hex(),
		Leverage:                            "5.0",
		OrderDensity:                        10,
		ReservationPriceSensitivityRatio:    "1.0",
		ReservationSpreadSensitivityRatio:   "0.5",
		MaxActiveCapitalUtilizationRatio:    "0.2",
		HeadChangeToleranceRatio:            "0.005",
		HeadToTailDeviationRatio:            "0.02",
		MinProximityToLiquidation:           "0.05",
		PostReductionPercOfMaxPosition:      "0.75",
		MinOracleVolatilitySampleSize:       5,
		EmergencyOracleVolatilitySampleSize: 2,
		DefaultMidPriceVolatilityRatio:      "0.005",
		LastValidMarkPrice:                  "2200000",
		MinVolatilityRatio:                  "0.5",
		AllowedSubscriptionTypes:            3,
		AllowedRedemptionTypes:              7,
		PositionPnlPenalty:                  "0.1",
		NotionalValueCap:                    "100000000000000000000000000000",
		OracleStaleTime:                     1800,
		OracleVolatilityMaxAge:              100,
	}

	var vaultInitMsgBz []byte
	vaultInitMsgBz, err = json.Marshal(initVaultMessage)
	OrFail(err)

	//var registryContract string
	//app.WasmKeeper.IterateContractsByCode(ctx, registryCodeID, func(addr sdk.AccAddress) bool {
	//	registryContract = addr.String()
	//	// fmt.Println("👉 registryContractAddress: ", registryContract)
	//	return true
	//})

	gasCoins := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(10_000_000_000)))
	OrFail(app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, gasCoins))
	OrFail(
		app.BankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			minttypes.ModuleName,
			MustNotErr(sdk.AccAddressFromBech32(sender)),
			gasCoins,
		),
	)

	registerVaultMsgBz := []byte(
		`{"register_vault":{"vault_code_id": ` + strconv.Itoa(
			int(vaultCodeID),
		) + `, "vault_label": "Derivative Vault", "instantiate_vault_msg": {"Derivative":` + string(
			vaultInitMsgBz,
		) + `}}}`,
	)

	registerVaultMsg := &wasmtypes.MsgExecuteContract{
		Sender:   sender,
		Contract: masterContractAddress,
		Msg:      wasmtypes.RawContractMessage(string(registerVaultMsgBz)),
		Funds:    gasCoins,
	}
	_, err = tp.MsgServerWasm.ExecuteContract(
		sdk.WrapSDKContext(ctx),
		registerVaultMsg,
	)
	OrFail(err)

	app.WasmKeeper.IterateContractsByCode(ctx, vaultCodeID, func(addr sdk.AccAddress) bool {
		VaultContractAddress = addr.String()
		// fmt.Println("👉 VaultContractAddress: ", VaultContractAddress)
		return true
	})

	err = app.WasmxKeeper.RegisterContract(ctx, wasmxtypes.ContractRegistrationRequest{
		ContractAddress: VaultContractAddress,
		GasLimit:        100_000,
		GasPrice:        1000,
		FundingMode:     wasmxtypes.FundingMode_SelfFunded,
	})
	OrFail(err)
}

func SetPriceOracles(
	testInput TestInput,
	app *simapp.InjectiveApp,
	ctx sdk.Context,
	seed int64,
) *ActionSetPriceOracles {
	if seed < 1 {
		seed = time.Now().UnixNano()
	}
	s1 := rand.NewSource(seed)
	r1 := rand.New(s1)

	spotsPrices := make([]float64, 0)
	for _, spot := range testInput.Spots {
		oracleBase, oracleQuote := spot.BaseDenom, spot.QuoteDenom

		minPrice := int64(10)
		maxPrice := int64(1000)
		superLarge := (r1.Intn(10) == 0)
		superSmall := (r1.Intn(10) == 0)

		if superLarge {
			maxPrice = int64(10000)
		} else if superSmall {
			maxPrice = int64(100)
		}

		price := sdk.NewDec(r1.Int63n(maxPrice-minPrice+1) + minPrice)
		app.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			oracleBase,
			oracleQuote,
			oracletypes.NewPriceState(price, ctx.BlockTime().Unix()),
		)
		spotsPrices = append(spotsPrices, price.MustFloat64())
	}
	perpPrices := make([]float64, 0)
	for _, perp := range testInput.Perps {
		oracleBase, oracleQuote, _ := perp.OracleBase, perp.OracleQuote, perp.OracleType

		minPrice := int64(10)
		maxPrice := int64(1000)
		superLarge := (r1.Intn(10) == 0)
		superSmall := (r1.Intn(10) == 0)

		if superLarge {
			maxPrice = int64(10000)
		} else if superSmall {
			maxPrice = int64(100)
		}

		price := sdk.NewDec(r1.Int63n(maxPrice-minPrice+1) + minPrice)
		app.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			oracleBase,
			oracleQuote,
			oracletypes.NewPriceState(price, ctx.BlockTime().Unix()),
		)
		perpPrices = append(perpPrices, price.MustFloat64())
	}
	expiryPrices := make([]float64, 0)
	for _, expiryMarket := range testInput.ExpiryMarkets {
		oracleBase, oracleQuote, _ := expiryMarket.OracleBase, expiryMarket.OracleQuote, expiryMarket.OracleType

		minPrice := int64(10)
		maxPrice := int64(1000)
		superLarge := (r1.Intn(10) == 0)
		superSmall := (r1.Intn(10) == 0)

		if superLarge {
			maxPrice = int64(10000)
		} else if superSmall {
			maxPrice = int64(100)
		}

		price := sdk.NewDec(r1.Int63n(maxPrice-minPrice+1) + minPrice)
		app.OracleKeeper.SetPriceFeedPriceState(
			ctx,
			oracleBase,
			oracleQuote,
			oracletypes.NewPriceState(price, ctx.BlockTime().Unix()),
		)
		expiryPrices = append(expiryPrices, price.MustFloat64())
	}

	binaryPrices := make([]float64, 0)
	for _, boMarket := range testInput.BinaryMarkets {
		minPrice := int64(0)
		maxPrice := int64(
			exchangetypes.GetScaledPrice(sdk.OneDec(), boMarket.OracleScaleFactor).MustFloat64(),
		)
		price := sdk.NewDec(r1.Int63n(maxPrice-minPrice+1) + minPrice)

		if refund := (r1.Intn(10) == 0); refund {
			price = sdk.NewDec(-1)
		}

		app.OracleKeeper.SetProviderPriceState(
			ctx,
			boMarket.OracleProvider,
			oracletypes.NewProviderPriceState(boMarket.OracleSymbol, price, ctx.BlockTime().Unix()),
		)
		binaryPrices = append(binaryPrices, price.MustFloat64())
	}

	actionHistory := ActionSetPriceOracles{}
	actionHistory.ActionType = ActionType_setPriceOracle
	actionHistory.SpotsPrices = spotsPrices
	actionHistory.PerpsPrices = perpPrices
	actionHistory.ExpiryMarkets = expiryPrices
	actionHistory.BinaryMarkets = binaryPrices
	return &actionHistory
}

type TestPlayer struct {
	TestInput             TestInput
	App                   *simapp.InjectiveApp
	Ctx                   sdk.Context
	Accounts              *[]Account
	Seed                  int64
	rand                  *rand.Rand
	AddressBySubAccount   *map[string]sdk.AccAddress
	InitCoins             *sdk.Coins
	InitialBankSupply     *sdk.Coins
	InitialBlockTime      time.Time
	InitialBlockHeight    int64
	InitialOraclePrices   *ActionSetPriceOracles
	Recording             *RecordedTest
	NumOperationsByAction map[ActionType]int
	SuccessActions        map[ActionType]int
	FailedActions         map[ActionType]int
	ordersById            map[string]*orderIdentifier
	ContractsById         map[string]string
	codeIdByContractId    map[string]int
	stopOnError           bool
	BalancesTracker       *BalancesTracker
	Validators            *[]sdk.AccAddress
	RegistryContractAddr  sdk.AccAddress
	registryCodeID        uint64
	TokenFactoryDenoms    *[]string
	paramParsers          *ParamParsers
	MsgServerWasm         wasmtypes.MsgServer
}

func InitTest(
	config TestPlayerConfig,
	recordedHistory *RecordedTest,
) TestPlayer {
	log.DefaultLogger.SetOutput(io.Discard)

	// cap funding to very low amount in fuzz test to prevent negative margins
	TestingExchangeParams.DefaultHourlyFundingRateCap = sdk.NewDecWithPrec(625, 6)

	// setup markets
	player := TestPlayer{}
	if recordedHistory != nil {
		player.Seed = recordedHistory.Seed
	} else {
		player.Seed = config.RandSeed
	}
	player.BalancesTracker = NewBalancesTracker()

	if config.ExchangeParams != nil {
		if config.ExchangeParams.AtomicMarketOrderAccessLevel != nil {
			accessLevel := config.ExchangeParams.AtomicMarketOrderAccessLevel.getExchangeAccessLevel()
			TestingExchangeParams.AtomicMarketOrderAccessLevel = accessLevel
		}
		if config.ExchangeParams.AtomicMarketOrderFeeMultiplier != nil {
			multiplier := f2d(*config.ExchangeParams.AtomicMarketOrderFeeMultiplier)
			TestingExchangeParams.SpotAtomicMarketOrderFeeMultiplier = multiplier
			TestingExchangeParams.DerivativeAtomicMarketOrderFeeMultiplier = multiplier
			TestingExchangeParams.BinaryOptionsAtomicMarketOrderFeeMultiplier = multiplier
		}
		defer func() { TestingExchangeParams = GetDefaultTestingExchangeParams() }()
	}

	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	})
	testInput, ctx := SetupTest(
		app,
		ctx,
		config.NumSpotMarkets,
		config.NumDerivativeMarkets,
		config.NumBinaryOptionsMarkets,
	)
	player.App = app
	player.Ctx = ctx
	player.TestInput = testInput
	if player.Seed == 0 { // don't override if it was set in history
		player.Seed = time.Now().UnixNano()
	}

	s1 := rand.NewSource(player.Seed)
	r1 := rand.New(s1)
	player.rand = r1

	player.registryCodeID = 1
	player.ordersById = make(map[string]*orderIdentifier, 0)
	player.ContractsById = make(map[string]string, 0)
	player.codeIdByContractId = make(map[string]int, 0)
	tfDenoms := []string{}
	player.TokenFactoryDenoms = &tfDenoms

	player.InitialBlockTime = ctx.BlockTime()
	player.InitialBlockHeight = ctx.BlockHeight()

	initialBankSupply := sdk.Coins{}
	app.BankKeeper.IterateTotalSupply(ctx, func(coin sdk.Coin) bool {
		initialBankSupply = initialBankSupply.Add(coin)
		return false
	})
	player.InitialBankSupply = &initialBankSupply

	// calculate init coins
	initCoins := sdk.Coins{}
	gasCoins, _ := sdk.NewIntFromString("2000000000000000000000000")
	initCoins.Add(sdk.NewCoin("INJ", gasCoins))
	for marketIndex := 0; marketIndex < config.NumSpotMarkets; marketIndex++ {
		baseDenom := "ETH" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT" + strconv.Itoa(marketIndex)
		initCoins = initCoins.Add(sdk.NewInt64Coin(quoteDenom, 100_000_000))
		initCoins = initCoins.Add(sdk.NewInt64Coin(baseDenom, 100_000_000))
	}

	for marketIndex := 0; marketIndex < config.NumDerivativeMarkets; marketIndex++ {
		baseDenom := "ETH" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT" + strconv.Itoa(marketIndex)
		initCoins = initCoins.Add(sdk.NewInt64Coin(quoteDenom, 100_000_000))
		initCoins = initCoins.Add(sdk.NewInt64Coin(baseDenom, 100_000_000))
	}

	for marketIndex := 0; marketIndex < config.NumExpiryMarkets; marketIndex++ {
		baseDenom := "ETH/TEF" + strconv.Itoa(marketIndex)
		quoteDenom := "USDT/TEF" + strconv.Itoa(marketIndex)
		initCoins = initCoins.Add(sdk.NewInt64Coin(quoteDenom, 100_000_000))
		initCoins = initCoins.Add(sdk.NewInt64Coin(baseDenom, 100_000_000))
	}

	for marketIndex := 0; marketIndex < config.NumBinaryOptionsMarkets; marketIndex++ {
		quoteDenom := "USDT" + strconv.Itoa(marketIndex)
		initCoins = initCoins.Add(sdk.NewInt64Coin(quoteDenom, 100_000_000))
	}
	player.InitCoins = &initCoins

	// generate Accounts
	accounts := []Account{}
	addressBySubAccount := make(map[string]sdk.AccAddress)
	for i := 0; i < config.NumAccounts; i++ {
		var address sdk.AccAddress
		if recordedHistory != nil && recordedHistory.Accounts != nil {
			address = sdk.AccAddress(crypto.AddressHash(recordedHistory.Accounts[i]))
		} else { // we don't have address stored, yet we want to have them repeatable, seed is usually stored
			secret := fmt.Sprintf("%v_account_00000%v", player.Seed, i)
			address = sdk.AccAddress(secp256k1.GenPrivKeySecp256k1([]byte(secret)).PubKey().Address())
		}

		account := Account{
			AccAddress:    address,
			SubaccountIDs: []string{},
		}

		numSubaccounts := MaxInt(1, config.NumSubaccounts)
		for j := getSubaccountStartingIndex(); j < numSubaccounts+getSubaccountStartingIndex(); j++ {
			subaccountID := common.BytesToAddress(address.Bytes()).Hex() + fmt.Sprintf("%024d", j)
			account.SubaccountIDs = append(account.SubaccountIDs, subaccountID)
			addressBySubAccount[subaccountID] = address
			addressBySubAccount[strings.ToLower(subaccountID)] = address
		}
		accounts = append(accounts, account)

		var err error

		// mint some INJ and send to bank module
		inj := chaintypes.NewInjectiveCoin(sdk.NewInt(10))
		coins := sdk.Coins{}
		coins = coins.Add(inj)
		err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins)
		OrFail(err)
		err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, address, coins)
		OrFail(err)

		for _, coin := range initCoins {
			for j := 0; j < numSubaccounts; j++ {
				// set balance for all the Accounts
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
				OrFail(err)

				err = app.BankKeeper.SendCoinsFromModuleToModule(
					ctx,
					minttypes.ModuleName,
					exchangetypes.ModuleName,
					sdk.NewCoins(coin),
				)
				OrFail(err)
				app.ExchangeKeeper.IncrementDepositOrSendToBank(
					ctx,
					common.HexToHash(account.SubaccountIDs[j]),
					coin.Denom,
					coin.Amount.ToDec(),
				)
			}
		}

		// mint any extra coins and send to subaccount or leave in bank
		if config.BankParams != nil && config.BankParams.ExtraCoins != nil && len(*config.BankParams.ExtraCoins) > 0 {
			for _, extraCoin := range *config.BankParams.ExtraCoins {
				asSdkCoins := sdk.Coins{extraCoin.Coin.toSdkCoin()}
				err = app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, asSdkCoins)
				OrFail(err)

				if extraCoin.MintTo == MintDestination_subaccount {
					app.ExchangeKeeper.IncrementDepositOrSendToBank(
						ctx,
						common.HexToHash(account.SubaccountIDs[0]),
						extraCoin.toSdkCoin().Denom,
						extraCoin.toSdkCoin().Amount.ToDec(),
					)
					err = app.BankKeeper.SendCoinsFromModuleToModule(
						ctx,
						minttypes.ModuleName,
						exchangetypes.ModuleName,
						asSdkCoins,
					)
					OrFail(err)
				} else {
					err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, address, asSdkCoins)
					OrFail(err)
				}
			}
		}
	}
	player.AddressBySubAccount = &addressBySubAccount
	player.Accounts = &accounts

	rewardCoins := sdk.NewCoins(
		sdk.NewCoin("inj0", sdk.NewInt(500_000_000)),
		sdk.NewCoin("inj1", sdk.NewInt(500_000_000)),
		sdk.NewCoin("inj2", sdk.NewInt(500_000_000)),
	)

	initTradingRewards(testInput, app, ctx, rewardCoins)
	initStaking(testInput, app, ctx, accounts)
	initTokenFactory(app, ctx, config.TokenFactoryParams)

	if config.InitAuctionModule {
		initAuctionModule(app, ctx)
	}

	// simulate initial price feed for all derivative markets
	player.InitialOraclePrices = SetPriceOracles(testInput, app, ctx, player.Seed)

	// launch spot markets
	for _, spot := range testInput.Spots {
		_, err := app.ExchangeKeeper.SpotMarketLaunch(
			ctx,
			spot.Ticker,
			spot.BaseDenom,
			spot.QuoteDenom,
			spot.MinPriceTickSize,
			spot.MinQuantityTickSize,
		)
		OrFail(err)
	}

	// launch insurance funds and derivative markets
	for idx, perp := range testInput.Perps {
		oracleBase, oracleQuote, oracleType := perp.OracleBase, perp.OracleQuote, perp.OracleType
		sender := accounts[0].AccAddress
		var marketConfig *TestPlayerMarketPerpConfig
		if config.PerpMarkets != nil {
			marketConfig = (*recordedHistory.Config.PerpMarkets)[idx]
		}

		var insuranceFund sdk.Coin
		if marketConfig != nil && marketConfig.InsuranceFund != nil {
			insuranceFund = sdk.NewCoin(perp.QuoteDenom, sdk.NewInt(*marketConfig.InsuranceFund))
		} else {
			insuranceFund = sdk.NewCoin(perp.QuoteDenom, sdk.NewInt(r1.Int63n(1000)+1))
		}
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(insuranceFund))
		app.BankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			minttypes.ModuleName,
			sender,
			sdk.NewCoins(insuranceFund),
		)
		OrFail(
			app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				insuranceFund,
				perp.Ticker,
				perp.QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				-1,
			),
		)

		makerFeeRate := perp.MakerFeeRate
		if marketConfig == nil {
			hasNegativeMakerFee := (r1.Intn(2) == 0)
			if hasNegativeMakerFee {
				makerFeeRate = makerFeeRate.Neg()
			}
		} else if marketConfig.MakerFeeRate != nil {
			makerFeeRate = f2d(*marketConfig.MakerFeeRate)
		}
		takerFeeRate := perp.TakerFeeRate
		if marketConfig != nil && marketConfig.TakerFeeRate != nil {
			takerFeeRate = f2d(*marketConfig.TakerFeeRate)
		}

		var oracleScaleFactor uint32
		if marketConfig != nil && marketConfig.OracleScaleFactor != nil {
			oracleScaleFactor = *marketConfig.OracleScaleFactor
		} else {
			oracleScaleFactor = 0
		}

		_, _, err := app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			perp.Ticker,
			perp.QuoteDenom,
			oracleBase,
			oracleQuote,
			oracleScaleFactor,
			oracleType,
			perp.InitialMarginRatio,
			perp.MaintenanceMarginRatio,
			makerFeeRate,
			takerFeeRate,
			perp.MinPriceTickSize,
			perp.MinQuantityTickSize,
		)
		OrFail(err)
	}

	player.MsgServerWasm = wasmkeeper.NewMsgServerImpl(&player.App.WasmKeeper)

	if config.InitContractRegistry {
		InitContractRegistry(app, ctx)
		//player.initContractRegistry(config.RegistryOwnerAccountIndex)
	}
	//InitContractRegistry(app, ctx)
	if config.InitMarketMaking { // init by default for fuzz test, and for other tests only when explicitly requested
		if config.NumDerivativeMarkets > 0 {
			player.initMito(testInput, app, ctx, accounts, testInput.Perps[0].MarketID)
		} else if config.NumSpotMarkets > 0 {
			player.initMito(testInput, app, ctx, accounts, testInput.Spots[0].MarketID)
		}
	}

	// launch insurance funds and expiry futures market
	for _, expiryMarket := range testInput.ExpiryMarkets {
		oracleBase, oracleQuote, oracleType := expiryMarket.OracleBase, expiryMarket.OracleQuote, expiryMarket.OracleType
		sender := accounts[0].AccAddress

		coin := sdk.NewCoin(expiryMarket.QuoteDenom, sdk.NewInt(r1.Int63n(1000)+1))
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			minttypes.ModuleName,
			sender,
			sdk.NewCoins(coin),
		)
		OrFail(
			app.InsuranceKeeper.CreateInsuranceFund(
				ctx,
				sender,
				coin,
				expiryMarket.Ticker,
				expiryMarket.QuoteDenom,
				oracleBase,
				oracleQuote,
				oracleType,
				expiryMarket.Expiry,
			),
		)

		makerFeeRate := expiryMarket.MakerFeeRate
		hasNegativeMakerFee := (r1.Intn(2) == 0)
		if hasNegativeMakerFee {
			makerFeeRate = makerFeeRate.Neg()
		}

		_, _, err := app.ExchangeKeeper.ExpiryFuturesMarketLaunch(
			ctx,
			expiryMarket.Ticker,
			expiryMarket.QuoteDenom,
			oracleBase,
			oracleQuote,
			0,
			oracleType,
			expiryMarket.Expiry,
			expiryMarket.InitialMarginRatio,
			expiryMarket.MaintenanceMarginRatio,
			makerFeeRate,
			expiryMarket.TakerFeeRate,
			expiryMarket.MinPriceTickSize,
			expiryMarket.MinQuantityTickSize,
		)
		OrFail(err)
	}

	// create oracle provider and launch binary options market
	for _, binaryOptionsMarket := range testInput.BinaryMarkets {
		coin := sdk.NewCoin(binaryOptionsMarket.QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		adminAccount := accounts[0].AccAddress
		app.BankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			minttypes.ModuleName,
			adminAccount,
			sdk.NewCoins(coin),
		)

		err := app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
			Provider: binaryOptionsMarket.OracleProvider,
			Relayers: []string{binaryOptionsMarket.Admin},
		})
		OrFail(err)

		makerFeeRate := binaryOptionsMarket.MakerFeeRate
		hasNegativeMakerFee := (r1.Intn(2) == 0)
		if hasNegativeMakerFee {
			makerFeeRate = makerFeeRate.Neg()
		}

		_, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(
			ctx,
			binaryOptionsMarket.Ticker,
			binaryOptionsMarket.OracleSymbol,
			binaryOptionsMarket.OracleProvider,
			oracletypes.OracleType_Provider,
			binaryOptionsMarket.OracleScaleFactor,
			makerFeeRate,
			binaryOptionsMarket.TakerFeeRate,
			binaryOptionsMarket.ExpirationTimestamp,
			binaryOptionsMarket.SettlementTimestamp,
			adminAccount.String(),
			binaryOptionsMarket.QuoteDenom,
			binaryOptionsMarket.MinPriceTickSize,
			binaryOptionsMarket.MinQuantityTickSize,
		)
		OrFail(err)
	}
	player.paramParsers = &ParamParsers{
		ContractSubAddr: regexp.MustCompile(
			"\\$contractAddress\\(([A-z0-9_]+)\\)(\\.sub\\(([0-9])\\))?",
		),
		SubAddr:         regexp.MustCompile("\\$subaccount\\(([0-9])\\)"),
		AccountIdx:      regexp.MustCompile("\\$account\\(([0-9]+)\\)"),
		TokenFactoryIdx: regexp.MustCompile("\\$tf\\(([0-9]+)\\)"),
		ContractsCodeId: regexp.MustCompile("\\$contractCodeIdAddress\\(([A-z0-9_]+)\\)"),
		BlockHeight:     regexp.MustCompile("\\$currentBlock(\\+([0-9]+))?"),
		BlockTime:       regexp.MustCompile("\\$blockTime(\\+([0-9]+)s)?"),
	}
	// ---- register actions ----
	player.NumOperationsByAction = make(map[ActionType]int)
	player.SuccessActions = make(map[ActionType]int)
	player.FailedActions = make(map[ActionType]int)
	return player
}
