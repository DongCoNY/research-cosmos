package peggy_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/testpeggy"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"

	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Have the validators put in a erc20<>denom relation with ERC20DeployedEvent
// Send some coins of that denom into the cosmos module
// Check that the coins are locked, not burned
// Have the validators put in a deposit event for that ERC20
// Check that the coins are unlocked and sent to the right account

func TestCosmosOriginated(t *testing.T) {
	tv := initializeTestingVars(t)
	addDenomToERC20Relation(tv)
	lockCoinsInModule(tv)
}

type testingVars struct {
	myOrchestratorAddr sdk.AccAddress
	myValAddr          sdk.ValAddress
	erc20              common.Address
	denom              string
	input              testpeggy.TestInput
	ctx                sdk.Context
	h                  sdk.Handler
	t                  *testing.T
}

func initializeTestingVars(t *testing.T) *testingVars {
	var tv testingVars

	tv.input = testpeggy.CreateTestEnv(t)
	tv.ctx = tv.input.Context

	tv.t = t

	tv.myOrchestratorAddr, _ = sdk.AccAddressFromBech32("inj1f2kdg34689x93cvw2y59z7y46dvz2fk8g3cggx")
	tv.myValAddr = sdk.ValAddress(tv.myOrchestratorAddr) // revisit when proper mapping is impl in keeper

	tv.erc20 = common.HexToAddress(testpeggy.TestingPeggyParams.CosmosCoinErc20Contract)
	tv.denom = "inj"

	tv.input.PeggyKeeper.StakingKeeper = testpeggy.NewStakingKeeperMock(tv.myValAddr)
	tv.input.PeggyKeeper.SetOrchestratorValidator(tv.ctx, tv.myValAddr, tv.myOrchestratorAddr)
	tv.h = NewHandler(tv.input.PeggyKeeper)

	return &tv
}

func addDenomToERC20Relation(tv *testingVars) {
	tv.input.BankKeeper.SetDenomMetaData(tv.ctx, bank.Metadata{
		Description: "The native staking token of the Injective chain.",
		DenomUnits: []*bank.DenomUnit{
			{Denom: "inj", Exponent: uint32(18), Aliases: []string{}},
		},
		Base:    "inj",
		Display: "inj",
	})

	var (
		myNonce = uint64(1)
	)

	ethClaim := types.MsgERC20DeployedClaim{
		CosmosDenom:   tv.denom,
		TokenContract: tv.erc20.Hex(),
		Name:          "inj",
		Symbol:        "inj",
		Decimals:      18,
		EventNonce:    myNonce,
		Orchestrator:  tv.myOrchestratorAddr.String(),
	}

	_, err := tv.h(tv.ctx, &ethClaim)
	require.NoError(tv.t, err)

	NewBlockHandler(tv.input.PeggyKeeper).EndBlocker(tv.ctx)

	// check if attestation persisted
	attestation := tv.input.PeggyKeeper.GetAttestation(tv.ctx, myNonce, ethClaim.ClaimHash())
	require.NotNil(tv.t, attestation)

	// check if erc20<>denom relation added to db
	isCosmosOriginated, gotERC20, err := tv.input.PeggyKeeper.DenomToERC20Lookup(tv.ctx, tv.denom)

	require.NoError(tv.t, err)
	assert.False(tv.t, isCosmosOriginated)

	isCosmosOriginated, gotDenom := tv.input.PeggyKeeper.ERC20ToDenomLookup(tv.ctx, tv.erc20)
	assert.True(tv.t, isCosmosOriginated)

	assert.Equal(tv.t, tv.denom, gotDenom)
	assert.Equal(tv.t, tv.erc20, gotERC20)
}

func lockCoinsInModule(tv *testingVars) {
	var (
		userCosmosAddr, _  = sdk.AccAddressFromBech32("inj1dqryh824u0w7p6ajk2gsr29tgj6d0nkfwsgs46")
		denom              = "inj"
		startingCoinAmount = sdk.NewIntFromUint64(150)
		sendAmount         = sdk.NewIntFromUint64(50)
		feeAmount          = sdk.NewIntFromUint64(5)
		startingCoins      = sdk.Coins{sdk.NewCoin(denom, startingCoinAmount)}
		sendingCoin        = sdk.NewCoin(denom, sendAmount)
		feeCoin            = sdk.NewCoin(denom, feeAmount)
		ethDestination     = common.HexToAddress("0x3c9289da00b02dC623d0D8D907619890301D26d4")
	)

	// we start by depositing some funds into the users balance to send
	tv.input.BankKeeper.MintCoins(tv.ctx, types.ModuleName, startingCoins)
	tv.input.BankKeeper.SendCoinsFromModuleToAccount(tv.ctx, types.ModuleName, userCosmosAddr, startingCoins)
	balance1 := tv.input.BankKeeper.GetAllBalances(tv.ctx, userCosmosAddr)
	assert.Equal(tv.t, sdk.Coins{sdk.NewCoin(denom, startingCoinAmount)}, balance1)

	// send some coins
	msg := &types.MsgSendToEth{
		Sender:    userCosmosAddr.String(),
		EthDest:   ethDestination.Hex(),
		Amount:    sendingCoin,
		BridgeFee: feeCoin,
	}

	peggyAddr := tv.input.AccountKeeper.GetModuleAddress(types.ModuleName)
	prevBalance := tv.input.BankKeeper.GetAllBalances(tv.ctx, peggyAddr)

	_, err := tv.h(tv.ctx, msg)
	require.NoError(tv.t, err)

	// Check that user balance has gone down
	balance2 := tv.input.BankKeeper.GetAllBalances(tv.ctx, userCosmosAddr)
	assert.Equal(tv.t, sdk.Coins{sdk.NewCoin(denom, startingCoinAmount.Sub(sendAmount).Sub(feeAmount))}, balance2)

	// Check that peggy balance has not gone up as tokens are burnt

	assert.Equal(tv.t,
		prevBalance,
		tv.input.BankKeeper.GetAllBalances(tv.ctx, peggyAddr),
	)
}

// nolint:all
func acceptDepositEvent(tv *testingVars) {
	var (
		myOrchestratorAddr, _ = sdk.AccAddressFromBech32("inj1f2kdg34689x93cvw2y59z7y46dvz2fk8g3cggx")
		myCosmosAddr, _       = sdk.AccAddressFromBech32("inj13c0m2t3hmaqp43rw0t97utvy5dc7ckte8f9tea")
		myNonce               = uint64(2)
		anyETHAddr            = common.HexToAddress("0xf9613b532673Cc223aBa451dFA8539B87e1F666D")
		token                 = common.HexToAddress("0x3c9289da00b02dC623d0D8D907619890301D26d4")
	)

	myErc20 := types.ERC20Token{
		Amount:   sdk.NewInt(12),
		Contract: token.Hex(),
	}

	ethClaim := types.MsgDepositClaim{
		EventNonce:     myNonce,
		TokenContract:  myErc20.Contract,
		Amount:         myErc20.Amount,
		EthereumSender: anyETHAddr.Hex(),
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}

	_, err := tv.h(tv.ctx, &ethClaim)
	require.NoError(tv.t, err)
	NewBlockHandler(tv.input.PeggyKeeper).EndBlocker(tv.ctx)

	// check that attestation persisted
	a := tv.input.PeggyKeeper.GetAttestation(tv.ctx, myNonce, ethClaim.ClaimHash())
	require.NotNil(tv.t, a)

	// Check that user balance has gone up
	assert.Equal(tv.t,
		sdk.Coins{sdk.NewCoin(token.Hex(), myErc20.Amount)},
		tv.input.BankKeeper.GetAllBalances(tv.ctx, myCosmosAddr))

	// Check that peggy balance has gone down
	peggyAddr := tv.input.AccountKeeper.GetModuleAddress(types.ModuleName)
	assert.Equal(tv.t,
		sdk.Coins{sdk.NewCoin(tv.denom, sdk.NewIntFromUint64(55).Sub(myErc20.Amount))},
		tv.input.BankKeeper.GetAllBalances(tv.ctx, peggyAddr),
	)
}
