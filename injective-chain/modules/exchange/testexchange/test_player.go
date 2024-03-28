package testexchange

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"

	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

type TestFlags byte

const (
	DefaultFlags     TestFlags = 0
	DoNotStopOnError TestFlags = 1 << iota
	DoNotCheckInvariants
)

type InvariantChecks struct {
	EachActionCheck func(action TestAction, actionIndex int, tp *TestPlayer)
	FinalCheck      func(tp *TestPlayer)
}

func (tp *TestPlayer) ReplayTestWithLegacyHooks(testflags TestFlags, legacyHooks *map[string]func(error), invariantChecks *InvariantChecks) (err error) {
	hooks := make(map[string]TestPlayerHook)
	for key, legacyHook := range *legacyHooks {
		hook := WrapErrorOnlyHook(legacyHook)
		hooks[key] = hook
	}
	return tp.ReplayTest(testflags, &hooks)
}

func (tp *TestPlayer) ReplayTest(testflags TestFlags, hooks *map[string]TestPlayerHook) (err error) {
	if testflags&DoNotStopOnError > 0 {
		tp.stopOnError = false
	} else {
		tp.stopOnError = true
	}

	for _, acc := range *tp.Accounts {
		subaccountID := common.HexToHash(acc.SubaccountIDs[0])
		deposits := tp.App.ExchangeKeeper.GetDeposits(tp.Ctx, subaccountID)

		bankBalances := sdk.NewCoins()
		if exchangetypes.IsDefaultSubaccountID(subaccountID) {
			bankBalances = tp.App.BankKeeper.GetAllBalances(tp.Ctx, exchangetypes.SubaccountIDToSdkAddress(subaccountID))
		}
		tp.BalancesTracker.SetBankBalancesAndSubaccountDeposits(subaccountID, bankBalances, deposits)
	}
	if hooks != nil {
		if hook := (*hooks)["init"]; hook != nil {
			hook(nil)
		}
	}

	for idx, action := range tp.Recording.Actions {
		actionId := action.GetActionId()
		if actionId == "128" {
			actionId = fmt.Sprintf("action_%d", idx)
		}
		if action.shouldLog() {
			fmt.Fprintf(GinkgoWriter, "Action %v (%v) \n", actionId, action.GetActionType())
		}
		for i := 0; i < action.GetRepeatTimes(); i++ {
			var resp any
			switch action.GetActionType() {
			case ActionType_setPriceOracle:
				tp.replaySetOraclePriceAction(action.(*ActionSetPriceOracles))
			case ActionType_positionBinary, ActionType_positionDerivative:
				err = tp.replayCreatePosition(action.(*ActionCreatePosition))
			case ActionType_spotLimitOrder, ActionType_spotMarketOrder:
				resp, err = tp.ReplayCreateSpotOrderAction(action.(*ActionCreateOrder))
			case ActionType_boLimitOrder, ActionType_boMarketOrder:
				resp, err = tp.replayCreateBinaryOptionsOrderAction(action.(*ActionCreateOrder))
			case ActionType_derivativeLimitOrder, ActionType_expiryLimitOrder, ActionType_derivativeMarketOrder:
				resp, err = tp.ReplayCreateDerivativeOrderAction(action.(*ActionCreateOrder), action.GetActionType() == ActionType_expiryLimitOrder)
			case ActionType_endblocker:
				tp.replayEndBlockerAction(action.(*ActionEndBlocker))
			case ActionType_beginblocker:
				tp.replayBeginBlockerAction(action.(*ActionBeginBlocker))
			case ActionType_cancelOrder:
				resp, err = tp.replayCancelOrderAction(action.(*ActionCancelOrder))
			case ActionType_liquidatePostion:
				resp, err = tp.replayLiquidatePositionAction(action.(*ActionLiquidatePosition))
			case ActionType_batchUpdate:
				err = tp.replayBatchUpdateAction(action.(*ActionBatchOrderUpdate))
			case ActionType_updateMarket:
				err = tp.replayUpdateMarketProposal(action.(*ActionUpdateMarketParams))
			case ActionType_withdrawal:
				resp, err = tp.replayWithdrawal(action.(*ActionWithdrawal))
			case ActionType_deposit:
				resp, err = tp.replayDeposit(action.(*ActionDeposit))
			case ActionType_removeFunds:
				err = tp.replayRemoveFunds(action.(*ActionWithdrawal))
			case ActionType_sendFunds:
				err = tp.replaySendFunds(action.(*ActionSend))
			case ActionType_forcedSettlement:
				err = tp.replayForcedMarketSettlementProposal(action.(*ActionForcedMarketSettlementParams))
			case ActionType_marketFunding:
				tp.replayPerpetualMarketFunding(action.(*ActionPerpetualMarketFundingParams))
			case ActionType_registerAndInitContract:
				resp, err = tp.ReplayRegisterAndInitializeContract(action.(*ActionRegisterAndInitializeContract))
			case ActionType_executeContract:
				err = tp.ReplayExecuteContract(action.(*ActionExecuteContract))
			case ActionType_createDenom:
				tp.replayCreateDenom(action.(*ActionCreateDenomTokenFactory))
			case ActionType_burnDenom:
				tp.replayBurnDenom(action.(*ActionBurnTokenFactory))
			case ActionType_mintDenom:
				tp.replayMintDenom(action.(*ActionMintTokenFactory))
			case ActionType_launchMarket:
				err = tp.replayLaunchMarket(action.(*ActionLaunchMarket))
			case ActionType_send:
				resp, err = tp.replaySend(action.(*ActionSend))
			case ActionType_batchDeregisterContracts:
				err = tp.ReplayDeregisterContracts(action.(*ActionBatchDeregisterContracts))
			case ActionType_registerContracts:
				err = tp.replayRegisterContracts(action.(*ActionBatchRegisterContracts))
			case ActionType_registerVault:
				err = tp.replayRegisterVault(action.(*ActionRegisterMitoVault))
			case ActionType_storeContractCode:
				resp, err = tp.ReplayStoreContractCode(action.(*ActionStoreContractCode))
			case ActionType_registerContractForBB:
				err = tp.ReplayRegisterContractForBB(action.(*ActionRegisterContractForBB))
			case ActionType_underwrite:
				err = tp.replayUnderwrite(action.(*ActionUnderwrite))
			case ActionType_placeBid:
				err = tp.replayPlaceBid(action.(*ActionPlaceBid))
			case ActionType_sendToEth:
				err = tp.replaySendToEth(action.(*ActionSendToEth))
			case ActionType_delegate:
				err = tp.replayDelegate(action.(*ActionDelegate))
			case ActionType_mint:
				err = tp.ReplayMint(action.(*ActionMintCoinParams))
			case ActionType_peggyDepositClaim:
				err = tp.replayPeggyDepositClaim(action.(*ActionPeggyDepositClaim))
			default:
				panic(fmt.Sprintf("Action type: %v is not supported (yet). Check if there's no error or implement it", action.GetActionType()))
			}
			if _, found := tp.NumOperationsByAction[action.GetActionType()]; found == false {
				tp.NumOperationsByAction[action.GetActionType()] = 0
			}
			tp.NumOperationsByAction[action.GetActionType()]++
			if hooks != nil {
				hook := (*hooks)[actionId]
				if hook != nil {
					params := TestPlayerHookParams{resp, err}

					hook(&params)
					err = nil
				}
			}
			if err != nil && tp.stopOnError {
				return
			} else {
				err = nil
			}
		} // for aciton.repeat times
	} // for actions

	if testflags&DoNotCheckInvariants == 0 {
		DefaultInvariantChecks.FinalCheck(tp)
	}

	return
}

func (rh *RecordedTest) UnmarshalJSON(b []byte) error {
	type history RecordedTest
	err := json.Unmarshal(b, (*history)(rh))
	if err != nil {
		return err
	}

	for _, raw := range rh.RawActions {
		var a TestActionBase
		err = json.Unmarshal(raw, &a)
		if err != nil {
			return err
		}
		var i TestAction
		switch a.GetActionType() {
		case ActionType_setPriceOracle:
			i = &ActionSetPriceOracles{}
		case ActionType_positionDerivative, ActionType_positionBinary:
			i = &ActionCreatePosition{}
		case ActionType_spotLimitOrder, ActionType_spotMarketOrder,
			ActionType_derivativeLimitOrder, ActionType_derivativeMarketOrder,
			ActionType_boMarketOrder, ActionType_boLimitOrder:
			i = &ActionCreateOrder{}
		case ActionType_endblocker:
			i = &ActionEndBlocker{}
		case ActionType_beginblocker:
			i = &ActionBeginBlocker{}
		case ActionType_cancelOrder:
			i = &ActionCancelOrder{}
		case ActionType_liquidatePostion:
			i = &ActionLiquidatePosition{}
		case ActionType_batchUpdate:
			i = &ActionBatchOrderUpdate{}
		case ActionType_updateMarket:
			i = &ActionUpdateMarketParams{}
		case ActionType_withdrawal, ActionType_removeFunds:
			i = &ActionWithdrawal{}
		case ActionType_deposit:
			i = &ActionDeposit{}
		case ActionType_forcedSettlement:
			i = &ActionForcedMarketSettlementParams{}
		case ActionType_marketFunding:
			i = &ActionPerpetualMarketFundingParams{}
		case ActionType_registerAndInitContract:
			i = &ActionRegisterAndInitializeContract{}
		case ActionType_executeContract:
			i = &ActionExecuteContract{}
		case ActionType_createDenom:
			i = &ActionCreateDenomTokenFactory{}
		case ActionType_mintDenom:
			i = &ActionMintTokenFactory{}
		case ActionType_burnDenom:
			i = &ActionBurnTokenFactory{}
		case ActionType_launchMarket:
			i = &ActionLaunchMarket{}
		case ActionType_send, ActionType_sendFunds:
			i = &ActionSend{}
		case ActionType_batchDeregisterContracts:
			i = &ActionBatchDeregisterContracts{}
		case ActionType_registerContracts:
			i = &ActionBatchRegisterContracts{}
		case ActionType_storeContractCode:
			i = &ActionStoreContractCode{}
		case ActionType_registerContractForBB:
			i = &ActionRegisterContractForBB{}
		case ActionType_registerVault:
			i = &ActionRegisterMitoVault{}
		case ActionType_underwrite:
			i = &ActionUnderwrite{}
		case ActionType_placeBid:
			i = &ActionPlaceBid{}
		case ActionType_sendToEth:
			i = &ActionSendToEth{}
		case ActionType_delegate:
			i = &ActionDelegate{}
		case ActionType_mint:
			i = &ActionMintCoinParams{}
		case ActionType_include:
			i = &ActionsInclude{}
		case ActionType_peggyDepositClaim:
			i = &ActionPeggyDepositClaim{}
		default:
			return fmt.Errorf("unsupported action type: %v", a.GetActionType())
		}
		err = json.Unmarshal(raw, i)
		if err != nil {
			return err
		}
		rh.Actions = append(rh.Actions, i)
	}
	return nil
}

func LoadReplayableTest(replayFilePath string) TestPlayer {
	replayHistory := loadSavedTestActions(replayFilePath)
	config := replayHistory.Config
	if config == nil {
		config = &TestPlayerConfig{}
	}
	if len(replayHistory.Accounts) > 0 {
		config.NumAccounts = len(replayHistory.Accounts)
	} else if replayHistory.NumAccounts > 0 {
		config.NumAccounts = replayHistory.NumAccounts
	}
	if replayHistory.NumSpotMarkets > 0 {
		config.NumSpotMarkets = replayHistory.NumSpotMarkets
	}
	if replayHistory.NumDerivativeMarkets > 0 {
		config.NumDerivativeMarkets = replayHistory.NumDerivativeMarkets
	}
	if replayHistory.NumExpiryMarkets > 0 {
		config.NumExpiryMarkets = replayHistory.NumExpiryMarkets
	}
	if replayHistory.NumBinaryOptionsMarkets > 0 {
		config.NumBinaryOptionsMarkets = replayHistory.NumBinaryOptionsMarkets
	}
	testSetup := InitTest(*config, &replayHistory)
	testSetup.Recording = &replayHistory
	return testSetup
}

func (rh *RecordedTest) MarshalJSON() ([]byte, error) {
	type history RecordedTest
	if rh.Actions != nil {
		for i, v := range rh.Actions {
			v.SetActionId(fmt.Sprintf("%d", i))
			a, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			rh.RawActions = append(rh.RawActions, a)
		}
	}
	return json.Marshal((*history)(rh))
}

func loadSavedTestActions(replayFilePath string) RecordedTest {
	jsonFile, err := os.Open(replayFilePath)
	OrFail(err)
	defer jsonFile.Close()

	var actionHistory RecordedTest
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &actionHistory)
	OrFail(err)

	var expandedActions = make([]TestAction, 0)
	for _, action := range actionHistory.Actions {
		if action.GetActionType() == ActionType_include {
			fileName := action.(*ActionsInclude).File
			includedTest := loadSavedTestActions(fmt.Sprintf("%v/%v", path.Dir(replayFilePath), fileName))
			for _, includedAction := range includedTest.Actions {
				expandedActions = append(expandedActions, includedAction)
			}
		} else {
			expandedActions = append(expandedActions, action)
		}
	}
	actionHistory.Actions = expandedActions
	return actionHistory
}

// WrapErrorOnlyHook is a helper method for legacy hooks
func WrapErrorOnlyHook(legacyHook func(error)) TestPlayerHook {
	return func(params *TestPlayerHookParams) {
		if params != nil {
			legacyHook(params.Error)
		} else {
			legacyHook(nil)
		}
	}
}
