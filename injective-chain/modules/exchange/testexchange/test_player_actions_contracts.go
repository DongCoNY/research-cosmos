package testexchange

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	wasmxkeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/keeper"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"

	. "github.com/onsi/ginkgo"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
)

// ReplayRegisterAndInitializeContract will store, initialise and optionally register contract. Returns contract address
func (tp *TestPlayer) ReplayRegisterAndInitializeContract(params *ActionRegisterAndInitializeContract) (string, error) {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts

	codeId, err := tp.ReplayStoreContractCode(&params.ActionStoreContractCode)
	if err != nil {
		return "", err
	}
	msgServer := wasmkeeper.NewMsgServerImpl(&app.WasmKeeper)
	sender := accounts[params.AccountIndex].AccAddress.String()

	label := params.Label
	if label == "" {
		label = params.Filename
	}

	message := params.Message
	err = tp.substituteParams(message, &params.TestActionWithMarketAndAccount)
	if err != nil {
		return "", err
	}

	messageDataBytes, err := json.Marshal(message)
	if err != nil {
		return "", err
	}
	instantiateContractMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender,
		Admin:  sender,
		CodeID: codeId,
		Label:  label,
		Msg:    wasmtypes.RawContractMessage(messageDataBytes),
		Funds:  sdk.NewCoins(),
	}

	resp, err := msgServer.InstantiateContract(
		sdk.WrapSDKContext(ctx),
		instantiateContractMsg,
	)

	if err != nil {
		return "", err
	}

	_ = resp
	var contractId = params.ContractId
	if contractId == "" {
		contractId = fmt.Sprintf("contract_%v", codeId)
	}
	var contractAddress string
	app.WasmKeeper.IterateContractsByCode(ctx, codeId, func(addr sdk.AccAddress) bool {
		contractAddress = addr.String()
		return true
	})
	tp.ContractsById[contractId] = contractAddress
	tp.codeIdByContractId[contractId] = int(codeId)

	contractAddr, err := sdk.AccAddressFromBech32(contractAddress)
	if err != nil {
		return "", err
	}
	subaccountIdHash, err := exchangetypes.SdkAddressWithNonceToSubaccountID(contractAddr, uint32(1))
	OrFail(err)

	account := Account{
		AccAddress:    contractAddr,
		SubaccountIDs: []string{subaccountIdHash.Hex()},
	}

	// fmt.Printf("👉 contract id: %s | contract account: %v\n", contractId, account)

	*tp.Accounts = append(*tp.Accounts, account)

	if params.RegisterForBB || params.RegisterForBBParams != nil {
		var newContract string
		app.WasmKeeper.IterateContractsByCode(ctx, codeId, func(addr sdk.AccAddress) bool {
			newContract = addr.String()
			return true
		})
		registerRequest := params.RegisterForBBParams
		if registerRequest == nil {
			registerRequest = &wasmxtypes.ContractRegistrationRequest{
				GasLimit:    1000000,
				GasPrice:    1,
				FundingMode: wasmxtypes.FundingMode_SelfFunded,
			}
		}
		registerRequest.ContractAddress = newContract
		registerRequest.CodeId = codeId

		err = app.WasmxKeeper.RegisterContract(ctx, *registerRequest)
		OrFail(err)
	}

	if len(params.Funds) > 0 {
		gasCoins := testCoinsToSdkCoins(params.Funds)
		OrFail(app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, gasCoins))
		vaultAddr, err := sdk.AccAddressFromBech32(contractAddress)
		if err != nil {
			return "", err
		}

		OrFail(app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, vaultAddr, gasCoins))
		OrFail(err)
	}

	if err != nil {
		return "", err
	}
	return contractAddress, nil
}

func (tp *TestPlayer) ReplayRegisterContractForBB(regParams *ActionRegisterContractForBB) error {
	gasLimit := 1000000
	if regParams.GasLimit != nil {
		gasLimit = *regParams.GasLimit
	}

	gasPrice := 1
	if regParams.GasPrice != nil {
		gasPrice = *regParams.GasPrice
	}

	contractAddress, err := tp.extractSpecialValue(regParams.ContractAddress)
	if err != nil {
		return err
	}

	granterAddress := sdk.AccAddress{}
	if regParams.GranterAddress != nil {
		granterAddress = sdk.MustAccAddressFromBech32(*regParams.GranterAddress)
	}

	fundingMode := wasmxtypes.FundingMode_SelfFunded
	if regParams.FundingMode != nil {
		fundingMode = *regParams.FundingMode
	}

	err = tp.App.WasmxKeeper.RegisterContract(tp.Ctx, wasmxtypes.ContractRegistrationRequest{
		ContractAddress: contractAddress,
		GasLimit:        uint64(gasLimit),
		GasPrice:        uint64(gasPrice),
		GranterAddress:  granterAddress.String(),
		FundingMode:     fundingMode,
	})

	if err != nil {
		return err
	}

	if len(regParams.Funds) > 0 {
		gasCoins := testCoinsToSdkCoins(regParams.Funds)
		err = tp.App.BankKeeper.MintCoins(tp.Ctx, minttypes.ModuleName, gasCoins)
		if err != nil {
			return err
		}

		vaultAddr, err := sdk.AccAddressFromBech32(contractAddress)
		if err != nil {
			return err
		}

		err = tp.App.BankKeeper.SendCoinsFromModuleToAccount(tp.Ctx, minttypes.ModuleName, vaultAddr, gasCoins)
		if err != nil {
			return err
		}
	}

	return nil
}

func (tp *TestPlayer) ReplayUpdateContractRegistryParams(contractAddress, adminAddress string, gasLimit, gasPrice uint64) error {
	msgServerWasm := wasmxkeeper.NewMsgServerImpl(tp.App.WasmxKeeper)
	msg := wasmxtypes.MsgUpdateContract{
		Sender:          contractAddress,
		ContractAddress: contractAddress,
		GasLimit:        gasLimit,
		GasPrice:        gasPrice,
		AdminAddress:    adminAddress,
	}
	_, err := msgServerWasm.UpdateRegistryContractParams(sdk.WrapSDKContext(tp.Ctx), &msg)
	return err
}

func (tp *TestPlayer) buildRegisterVaultMessage(message map[string]any) map[string]any {

	if registerMsg, ok := message["register_vault"]; ok {
		if registerMsgAsMap, ok := registerMsg.(map[string]any); ok {
			if instantiateMsg, ok := registerMsgAsMap["instantiate_vault_msg"]; ok {
				if instantiateMsgAsMap, ok := instantiateMsg.(map[string]any); ok {
					mapToUse := make(map[string]any, 0)
					if spotMsg, ok := instantiateMsgAsMap["Spot"]; ok {
						defaultInstantiateMessage := tp.getDefaultVaultInstantiateMessage(VaultType_Spot)
						if spotMsgAsMap, ok := spotMsg.(map[string]any); ok {
							for k, v := range spotMsgAsMap {
								defaultInstantiateMessage[k] = v
							}
						}
						mapToUse["Spot"] = defaultInstantiateMessage
					} else if derivativeMsg, ok := instantiateMsgAsMap["Derivative"]; ok {
						defaultInstantiateMessage := tp.getDefaultVaultInstantiateMessage(VaultType_Derivative)
						if derivativeMsgAsMap, ok := derivativeMsg.(map[string]any); ok {
							for k, v := range derivativeMsgAsMap {
								defaultInstantiateMessage[k] = v
							}
						}
						mapToUse["Derivative"] = defaultInstantiateMessage
					}
					registerMsgAsMap["instantiate_vault_msg"] = mapToUse
				}
			}
			message["register_vault"] = registerMsgAsMap
		}
	}

	return message
}

func (tp *TestPlayer) replayRegisterVault(regParams *ActionRegisterMitoVault) error {
	app := tp.App
	ctx := tp.Ctx
	rawRegisterMsg := tp.buildRegisterVaultMessage(regParams.RegistrationInfo.Message)
	regInfo := regParams.RegistrationInfo.Info
	executeParams := ActionExecuteContract{
		TestActionBase:                 regParams.TestActionBase,
		TestActionWithMarketAndAccount: regParams.TestActionWithMarketAndAccount,
		ContractId:                     regParams.MasterContractId,
		Message:                        rawRegisterMsg,
		Funds:                          []Coin{},
		WithdrawFunds:                  false,
		ExecutionType:                  "wasm",
		ExtraParams:                    nil,
	}

	msg := tp.buildWasmExecuteContractMsg(&executeParams)

	msgServer := wasmkeeper.NewMsgServerImpl(&app.WasmKeeper)

	_, err := msgServer.ExecuteContract(
		sdk.WrapSDKContext(ctx),
		&msg,
	)

	if err != nil {
		return err
	}

	codeId := regInfo.CodeId
	codeId, err = tp.extractSpecialValue(codeId)
	if err != nil {
		return err
	}

	codeIdAsInt, err := strconv.Atoi(codeId)
	if err != nil {
		return err
	}

	var contractId = regInfo.ContractId
	var contractAddress string
	app.WasmKeeper.IterateContractsByCode(ctx, uint64(codeIdAsInt), func(addr sdk.AccAddress) bool {
		contractAddress = addr.String()
		return true
	})
	tp.ContractsById[contractId] = contractAddress
	tp.codeIdByContractId[contractId] = codeIdAsInt

	contractAddr, err := sdk.AccAddressFromBech32(contractAddress)
	if err != nil {
		return err
	}
	subaccountIdHash, err := exchangetypes.SdkAddressWithNonceToSubaccountID(contractAddr, uint32(0))
	OrFail(err)

	account := Account{
		AccAddress:    contractAddr,
		SubaccountIDs: []string{subaccountIdHash.Hex()},
	}
	*tp.Accounts = append(*tp.Accounts, account)

	if regInfo.RegisterForBB {
		defGasPrice := 1000 // set 1000 by default, so contract is executed
		bbAction := ActionRegisterContractForBB{
			TestActionBase:  regParams.TestActionBase,
			ContractAddress: contractAddress,
			GasPrice:        &defGasPrice,
			Funds: []Coin{
				{
					Denom:  "inj",
					Amount: 100000000000},
			},
		}
		if regInfo.RegistryParams != nil {
			bbAction.GasLimit = regInfo.RegistryParams.GasLimit
			bbAction.GasPrice = regInfo.RegistryParams.GasPrice
			if len(regInfo.RegistryParams.Funds) > 0 {
				bbAction.Funds = regInfo.RegistryParams.Funds
			}
		}
		err = tp.ReplayRegisterContractForBB(&bbAction)
		if err != nil {
			return err
		}
	}

	return nil
}

func (tp *TestPlayer) replayRegisterContracts(batchParams *ActionBatchRegisterContracts) error {
	ctx := tp.Ctx
	app := tp.App

	for i := range batchParams.Requests {
		address, err := tp.extractSpecialValue(batchParams.Requests[i].ContractAddress)
		OrFail(err)
		batchParams.Requests[i].ContractAddress = address

		if batchParams.Requests[i].FundingMode == wasmxtypes.FundingMode_Unspecified {
			batchParams.Requests[i].FundingMode = wasmxtypes.FundingMode_SelfFunded
		}
	}

	registerContractsMsg := &wasmxtypes.BatchContractRegistrationRequestProposal{
		Title:                        batchParams.Title,
		Description:                  batchParams.Description,
		ContractRegistrationRequests: batchParams.Requests,
	}

	wasmGovHandler := wasmkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmtypes.EnableAllProposals)
	handler := wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
	err := handler(ctx, registerContractsMsg)

	fmt.Printf("err: %+v\n", err)

	if err == nil {
		tp.SuccessActions[batchParams.ActionType]++
	} else {
		tp.FailedActions[batchParams.ActionType]++
		return err
	}

	return nil
}

func (tp *TestPlayer) ReplayDeregisterContracts(batchParams *ActionBatchDeregisterContracts) error {
	ctx := tp.Ctx
	app := tp.App

	addresses := make([]string, 0)
	for _, adr := range batchParams.Contracts {
		address, err := tp.extractSpecialValue(adr)
		OrFail(err)
		addresses = append(addresses, address)
	}

	deregisterContractsMsg := &wasmxtypes.BatchContractDeregistrationProposal{
		Title:       batchParams.Title,
		Description: batchParams.Description,
		Contracts:   addresses,
	}

	wasmGovHandler := wasmkeeper.NewLegacyWasmProposalHandler(app.WasmKeeper, wasmtypes.EnableAllProposals)
	handler := wasmx.NewWasmxProposalHandler(app.WasmxKeeper, wasmGovHandler)
	err := handler(ctx, deregisterContractsMsg)

	if err == nil {
		tp.SuccessActions[batchParams.ActionType]++
	} else {
		tp.FailedActions[batchParams.ActionType]++
		return err
	}

	return nil
}

func (tp *TestPlayer) RegisterAndInitializeContract(
	wasmFilePath string,
	label string,
	sender string,
	message map[string]any,
	registerForBeginBlocker *BeginBlockerRegistryParams,
) (sdk.AccAddress, uint64, error) {
	codeId := MustNotErr(tp.StoreContract(wasmFilePath, sender))
	messageDataBytes := MustNotErr(json.Marshal(message))
	return tp.InitialiseContract(codeId, label, sender, messageDataBytes, registerForBeginBlocker)
}

func (tp *TestPlayer) StoreContract(
	wasmFilePath string,
	sender string,
) (uint64, error) {
	msgServer := wasmkeeper.NewMsgServerImpl(&tp.App.WasmKeeper)

	storeContractCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender,
		WASMByteCode: keeper.ReadFile(wasmFilePath),
	}
	respStore, err := msgServer.StoreCode(
		sdk.WrapSDKContext(tp.Ctx),
		storeContractCodeMsg,
	)
	if err != nil {
		return 0, err
	}
	return respStore.CodeID, nil
}

func (tp *TestPlayer) InitialiseContract(
	codeId uint64,
	label string,
	sender string,
	message []byte,
	registerForBeginBlocker *BeginBlockerRegistryParams,
) (sdk.AccAddress, uint64, error) {
	ctx := tp.Ctx
	app := tp.App
	msgServer := wasmkeeper.NewMsgServerImpl(&tp.App.WasmKeeper)

	instantiateContractMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender,
		Admin:  sender,
		CodeID: codeId,
		Label:  label,
		Msg:    wasmtypes.RawContractMessage(message),
		Funds:  sdk.NewCoins(),
	}

	_, err := msgServer.InstantiateContract(
		sdk.WrapSDKContext(ctx),
		instantiateContractMsg,
	)

	if err != nil {
		return nil, 0, err
	}

	var contractAddress string
	app.WasmKeeper.IterateContractsByCode(ctx, codeId, func(addr sdk.AccAddress) bool {
		contractAddress = addr.String()
		return true
	})

	contractAddr, err := sdk.AccAddressFromBech32(contractAddress)
	if err != nil {
		return nil, 0, err
	}
	subaccountIdHash, err := exchangetypes.SdkAddressWithNonceToSubaccountID(contractAddr, uint32(1))
	OrFail(err)

	account := Account{
		AccAddress:    contractAddr,
		SubaccountIDs: []string{subaccountIdHash.Hex()},
	}

	// fmt.Printf("👉 contract id: %s | contract account: %v\n", contractId, account)

	*tp.Accounts = append(*tp.Accounts, account)

	if registerForBeginBlocker != nil {
		var newContract string
		app.WasmKeeper.IterateContractsByCode(ctx, codeId, func(addr sdk.AccAddress) bool {
			newContract = addr.String()
			return true
		})

		err = app.WasmxKeeper.RegisterContract(ctx, wasmxtypes.ContractRegistrationRequest{
			ContractAddress: newContract,
			GasLimit:        registerForBeginBlocker.GasLimit,
			GasPrice:        registerForBeginBlocker.GasPrice,
			CodeId:          codeId,
			FundingMode:     wasmxtypes.FundingMode_SelfFunded,
		})
		OrFail(err)
	}

	if err != nil {
		return nil, 0, err
	}
	return contractAddr, codeId, nil
}

func (tp *TestPlayer) ReplayStoreContractCode(params *ActionStoreContractCode) (uint64, error) {
	sender := (*tp.Accounts)[params.AccountIndex].AccAddress.String()
	path := params.Path
	if path == "" {
		path = "../wasm"
	}

	codeId, err := tp.StoreContract(fmt.Sprintf("%s/%s", path, params.Filename), sender)
	OrFail(err)

	contractId := params.ContractId
	if contractId == "" {
		contractId = params.Filename
	}
	tp.codeIdByContractId[contractId] = int(codeId)
	return codeId, nil
}

func (tp *TestPlayer) doesExecuteActionContainRegistrationMessage(params *ActionExecuteContract) bool {
	if params.ExtraParams != nil && params.ExtraParams.Registration != nil {
		return true
	}

	return false
}

func (tp *TestPlayer) ReplayExecuteContract(params *ActionExecuteContract) error {
	ctx := tp.Ctx
	app := tp.App
	accounts := *tp.Accounts
	accountIndex := params.AccountIndex
	handler := exchange.NewHandler(app.ExchangeKeeper)

	if params.WithdrawFunds || !exchangetypes.IsDefaultSubaccountID(common.HexToHash(accounts[accountIndex].SubaccountIDs[params.SubaccountIndex])) {
		for _, fund := range params.Funds {
			requiredAmount := fund.toSdkCoinWithSubstitution(tp, &params.TestActionWithMarketAndAccount)
			bankBalance := app.BankKeeper.GetBalance(ctx, accounts[accountIndex].AccAddress, requiredAmount.Denom)
			if bankBalance.Amount.GTE(requiredAmount.Amount) {
				continue
			}

			p := &exchangetypes.MsgWithdraw{
				Sender:       accounts[accountIndex].AccAddress.String(),
				SubaccountId: common.HexToHash(accounts[accountIndex].SubaccountIDs[params.SubaccountIndex]).String(),
				Amount:       requiredAmount,
			}
			_, err := handler(ctx, p)
			if err != nil {
				return err
			}
		}
	}

	ctx, writeCache := ctx.CacheContext()
	var err error

	switch params.ExecutionType {
	case "injective":
		msg := tp.buildInjectiveExecMsg(params)

		errValidateBasic := msg.ValidateBasic()
		if errValidateBasic != nil {
			return errValidateBasic
		}

		msgServer := keeper.NewMsgServerImpl(app.ExchangeKeeper)
		_, err := msgServer.PrivilegedExecuteContract(sdk.WrapSDKContext(ctx), &msg)

		if err != nil {
			return err
		}

	case "wasm":
		message := params.Message
		OrFail(tp.substituteParams(message, &params.TestActionWithMarketAndAccount))
		fmt.Fprintln(GinkgoWriter, "Block height:", tp.Ctx.BlockHeight(), "message:", message)

		msg := tp.buildWasmExecuteContractMsg(params)

		msgServer := wasmkeeper.NewMsgServerImpl(&app.WasmKeeper)

		_, err = msgServer.ExecuteContract(
			sdk.WrapSDKContext(ctx),
			&msg,
		)
	default:
		return fmt.Errorf("unknown execution type: %s", params.ExecutionType)
	}

	if err == nil {
		writeCache()
		tp.Ctx.EventManager().EmitEvents(ctx.EventManager().Events())
		tp.SuccessActions[params.ActionType]++
	} else {
		tp.FailedActions[params.ActionType]++
		return err
	}

	return nil
}

// helpers

func (tp *TestPlayer) buildInjectiveExecMsg(msgParams *ActionExecuteContract) exchangetypes.MsgPrivilegedExecuteContract {
	accounts := *tp.Accounts

	accountIndex := msgParams.AccountIndex
	sender := accounts[accountIndex].AccAddress.String()
	contractAddress := tp.ContractsById[msgParams.ContractId]
	funds := testCoinsToSdkCoinsWithSubstitution(msgParams.Funds, tp, &msgParams.TestActionWithMarketAndAccount)

	message := msgParams.Message
	err := tp.substituteParams(message, &msgParams.TestActionWithMarketAndAccount)
	OrFail(err)

	execData := wasmxtypes.ExecutionData{
		Origin: sender,
		Name:   "Injective Exec Message",
		Args:   message,
	}
	var execDataBytes []byte
	execDataBytes, err = json.Marshal(execData)
	OrFail(err)

	return exchangetypes.MsgPrivilegedExecuteContract{
		Sender:          sender,
		Funds:           funds.String(),
		ContractAddress: contractAddress,
		Data:            string(execDataBytes),
	}
}

type VaultType string

// nolint: all
const (
	VaultType_Spot       VaultType = "spot"
	VaultType_Derivative VaultType = "derivative"
)

func (tp *TestPlayer) buildWasmExecuteContractMsg(msgParams *ActionExecuteContract) wasmtypes.MsgExecuteContract {
	accounts := *tp.Accounts

	accountIndex := msgParams.AccountIndex
	sender := accounts[accountIndex].AccAddress.String()
	contractAddress := tp.ContractsById[msgParams.ContractId]
	funds := testCoinsToSdkCoinsWithSubstitution(msgParams.Funds, tp, &msgParams.TestActionWithMarketAndAccount)

	message := msgParams.Message
	err := tp.substituteParams(message, &msgParams.TestActionWithMarketAndAccount)
	OrFail(err)

	messageDataBytes, err := json.Marshal(message)
	OrFail(err)

	return wasmtypes.MsgExecuteContract{
		Sender:   sender,
		Contract: contractAddress,
		Msg:      wasmtypes.RawContractMessage(messageDataBytes),
		Funds:    funds,
	}
}

func (tp *TestPlayer) getDefaultVaultInstantiateMessage(vaultType VaultType) map[string]any {
	message := make(map[string]any, 0)

	if vaultType == VaultType_Spot {
		message["master_address"] = "$contractAddress(scMaster)"
		message["config_owner"] = "$account(0)"
		message["market_id"] = "$marketId"
		message["order_density"] = 8
		message["market_buy_mid_price_deviation_percent"] = "0.8"
		message["market_sell_mid_price_deviation_percent"] = "0.8"
		message["reservation_price_sensitivity_ratio"] = "0.5"
		message["reservation_spread_sensitivity_ratio"] = "0.5"
		message["max_active_capital_utilization_ratio"] = "0.5"
		message["mid_price_tail_deviation_ratio"] = "0.5"
		message["head_to_tail_deviation_ratio"] = "0.7"
		message["head_change_tolerance_ratio"] = "0"
		message["target_base_weight"] = "1"
		message["emergency_oracle_volatility_sample_size"] = 1
		message["min_oracle_volatility_sample_size"] = 1
		message["default_mid_price_volatility_ratio"] = "0.005"
		message["inventory_imbalance_market_order_threshold"] = "0.7"
		message["cw20_code_id"] = 2
		message["lp_name"] = "MarketVaultLPTokens"
		message["lp_symbol"] = "MMLP"
		message["cw20_label"] = "Vault"
		message["reservation_price_tail_deviation_ratio"] = "0.5"
		message["allowed_subscription_types"] = 1 << 0 // 2 * 0 = 0
		message["allowed_redemption_types"] = 1<<4 - 1 // 2 * 4 - 1 = 15 as 15 allows to redeem in all supported ways
		message["min_volatility_ratio"] = "0"
		message["signed_min_head_to_fair_price_deviation_ratio"] = "0"
		message["signed_min_head_to_tob_deviation_ratio"] = "0"
		message["imbalance_adjustment_exponent"] = "2"
		message["reward_diminishing_factor"] = "0.5"
		message["base_decimals"] = 18
		message["quote_decimals"] = 6
		message["base_oracle_symbol"] = "ETH0"
		message["quote_oracle_symbol"] = "USDT0"
		message["notional_value_cap"] = "1000000000000000000000000"
		message["oracle_type"] = 2
		message["oracle_stale_time"] = 1800        // 30 minutes
		message["oracle_volatility_max_age"] = 100 // 100 seconds
	} else {
		message["master_address"] = "$contractAddress(scMaster)"
		message["config_owner"] = "$account(0)"
		message["market_id"] = "$marketId"
		message["order_density"] = 2
		message["market_buy_mid_price_deviation_percent"] = "0.5"
		message["market_sell_mid_price_deviation_percent"] = "0.5"
		message["reservation_price_sensitivity_ratio"] = "0.5"
		message["reservation_spread_sensitivity_ratio"] = "0.5"
		message["max_active_capital_utilization_ratio"] = "0.5"
		message["mid_price_tail_deviation_ratio"] = "0.5"
		message["head_to_tail_deviation_ratio"] = "0.7"
		message["head_change_tolerance_ratio"] = "0"
		message["default_mid_price_volatility_ratio"] = "0.005"
		message["inventory_imbalance_market_order_threshold"] = "0.7"
		message["cw20_code_id"] = 2
		message["lp_name"] = "MarketVaultLPTokens"
		message["lp_symbol"] = "MMLP"
		message["cw20_label"] = "Vault"
		message["emergency_oracle_volatility_sample_size"] = 1
		message["min_oracle_volatility_sample_size"] = 1
		message["oracle_volatility_group_sec"] = 1
		message["last_valid_mark_price"] = "100000"
		message["leverage"] = "2"
		message["min_proximity_to_liquidation"] = "1"
		message["min_volatility_ratio"] = "0.5"
		message["post_reduction_perc_of_max_position"] = "0.5"
		message["leveraged_active_capital_to_max_position_exposure_ratio"] = "0.1"
		message["reservation_price_tail_deviation_ratio"] = "0.5"
		message["allowed_subscription_types"] = 1<<1 + 1 // 2 * 1 + 1 = 3 as 3 allows to subscribe with and without position
		message["allowed_redemption_types"] = 1<<3 - 1   // 2 * 3 - 1 = 7 as 7 allows to redeem in all supported ways
		message["min_volatility_ratio"] = "0"
		message["signed_min_head_to_fair_price_deviation_ratio"] = "0.1"
		message["signed_min_head_to_tob_deviation_ratio"] = "0.1"
		message["position_pnl_penalty"] = "0.01"
		message["notional_value_cap"] = "1000000000000000000000000"
		message["oracle_stale_time"] = 1800        // 30 minutes
		message["oracle_volatility_max_age"] = 100 // 100 seconds
	}

	return message
}

func (tp *TestPlayer) substituteParamsInString(message string, params *TestActionWithMarketAndAccount) (any, error) {
	if message == "$marketId" {
		marketId := tp.FindMarketId(tp.GetDefaultMarketType(params.MarketType), params.MarketIndex)
		message = marketId.String()
	}
	for found := true; found; {
		subStrAccountIdx := tp.paramParsers.AccountIdx.FindStringSubmatch(message)
		if subStrAccountIdx != nil {
			ownerAccountIdx, _ := strconv.Atoi(subStrAccountIdx[1])
			account := (*tp.Accounts)[ownerAccountIdx].AccAddress.String()
			message = strings.ReplaceAll(message, subStrAccountIdx[0], account)
		} else {
			found = false
		}
	}
	for found := true; found; {
		subStrSubaccountAddr := tp.paramParsers.ContractSubAddr.FindStringSubmatch(message)
		if subStrSubaccountAddr != nil {
			contractId := subStrSubaccountAddr[1]
			contractAddress, ok := tp.ContractsById[contractId]

			if !ok {
				return nil, fmt.Errorf("no contract address for id found: %v\n", contractId)
			}
			// if it matches "$contract(id)" skip the rest
			if subStrSubaccountAddr[2] == "" || subStrSubaccountAddr[3] == "" {
				message = strings.ReplaceAll(message, subStrSubaccountAddr[0], tp.ContractsById[contractId])
				continue
			}

			//subaccount index was provided
			subaccountIndex, err := strconv.Atoi(subStrSubaccountAddr[3])
			if err != nil {
				return nil, err
			}

			contractAddrAcc := MustNotErr(sdk.AccAddressFromBech32(contractAddress))
			subaccountIdHash := MustNotErr(exchangetypes.SdkAddressWithNonceToSubaccountID(contractAddrAcc, uint32(subaccountIndex)))
			subaccountId := subaccountIdHash.Hex()
			message = strings.ReplaceAll(message, subStrSubaccountAddr[0], subaccountId)
		} else {
			found = false
		}
	}
	for found := true; found; {
		subStrSubaccount := tp.paramParsers.SubAddr.FindStringSubmatch(message)
		if subStrSubaccount != nil {
			subaccountIdx, err := strconv.Atoi(subStrSubaccount[1])
			if err != nil {
				return nil, err
			}
			message = tp.paramParsers.SubAddr.ReplaceAllString(message, (*tp.Accounts)[params.AccountIndex].SubaccountIDs[subaccountIdx])
		} else {
			found = false
		}
	}
	for found := true; found; {
		tfSubString := tp.paramParsers.TokenFactoryIdx.FindStringSubmatch(message)
		if tfSubString != nil {
			tfIndex, err := strconv.Atoi(tfSubString[1])
			if err != nil {
				return nil, err
			}
			message = tp.paramParsers.TokenFactoryIdx.ReplaceAllString(message, (*tp.TokenFactoryDenoms)[tfIndex])
		} else {
			found = false
		}
	}
	for found := true; found; {
		subStrContractsCodeId := tp.paramParsers.ContractsCodeId.FindStringSubmatch(message)
		if subStrContractsCodeId != nil {
			codeId, ok := tp.codeIdByContractId[subStrContractsCodeId[1]]
			if !ok {
				return nil, fmt.Errorf("No contract with id '%s' was found", subStrContractsCodeId[1])
			}
			return codeId, nil
		} else {
			found = false
		}
	}
	for found := true; found; {
		blockHeightMatch := tp.paramParsers.BlockHeight.FindStringSubmatch(message)
		if blockHeightMatch != nil {
			blockHeight := int(tp.Ctx.BlockHeight())
			if blockHeightMatch[2] != "" {
				addBlocks := MustNotErr(strconv.Atoi(blockHeightMatch[2]))
				blockHeight += addBlocks
			}
			return blockHeight, nil
		} else {
			found = false
		}
	}
	for found := true; found; {
		blockTimestampMatch := tp.paramParsers.BlockTime.FindStringSubmatch(message)
		if blockTimestampMatch != nil {
			blockTimestamp := tp.Ctx.BlockTime().UnixNano()
			if blockTimestampMatch[2] != "" {
				addSeconds := MustNotErr(strconv.Atoi(blockTimestampMatch[2])) // we can add support for more units if needed
				blockTimestamp += int64(addSeconds * 1e9)
			}
			return strconv.Itoa(int(blockTimestamp)), nil
		} else {
			found = false
		}
	}

	return message, nil
}

func (tp *TestPlayer) substituteParams(message any, params *TestActionWithMarketAndAccount) error {
	if message, ok := message.(map[string]any); ok {
		for key, val := range message {
			if s, ok := val.(string); ok {
				substituted, err := tp.substituteParamsInString(s, params)
				if err != nil {
					return err
				}
				message[key] = substituted
			}
			if s, ok := val.(map[string]any); ok {
				err := tp.substituteParams(s, params)
				if err != nil {
					return err
				}
			}
			if s, ok := val.([]any); ok {
				for _, el := range s {
					err := tp.substituteParams(el, params)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
