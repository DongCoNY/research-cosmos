package keeper_test

import (
	"fmt"
	"testing"

	"github.com/cosmos/gogoproto/jsonpb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/testpeggy"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	exchangeTypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func TestDepositClaimData(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "0x727AEE334987c52fA7b567b2662BDbb68614e48C"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, amountToDeposit.Amount.ToDec())

}

func TestDepositClaimData_RandomSubaccountID(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, amountToDeposit.Amount.ToDec())

}

func TestInvalidDepositClaimData_InvalidFormat(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		subaccountID      = "abcdefg"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())

}

func TestInvalidDepositClaimData_NonJsonData(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           "random data",
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())

}

func TestInvalidDepositClaimData_NegativeAmount(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	negativeAmount := sdk.Coin{
		Denom:  types.PeggyDenomString(common.HexToAddress(tokenContractAddr.Hex())),
		Amount: sdk.NewInt(-100),
	}

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       negativeAmount,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())

}

func TestInvalidDepositClaimData_MissingAmount(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		// Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())
}

func TestInvalidDepositClaimData_MissingSubaccountID(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr       = "0x727AEE334987c52fA7b567b2662BDbb68614e48C"
		senderInjAccAddr    = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr   = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit     = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		depositSubaccountID = "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: depositSubaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	// If sub account id is missing in MsgDeposit, the deposit is added to the default sub account
	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(depositSubaccountID), amountToDeposit.GetDenom())
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, amountToDeposit.Amount.ToDec())
}

func TestInvalidDepositClaimData_WithdrawClaim(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr               = "0x727AEE334987c52fA7b567b2662BDbb68614e48C"
		senderInjAccAddr            = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr           = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		amountToDeposit             = types.NewERC20Token(414, tokenContractAddr).PeggyCoin() // Pickle
		amountToDepositInSubaccount = types.NewERC20Token(30, tokenContractAddr).PeggyCoin()
		subaccountID                = "0x727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// Add deposit to sub account to check withdraw
	input.ExchangeKeeper.IncrementDepositWithCoinOrSendToBank(ctx, common.HexToHash(subaccountID), amountToDepositInSubaccount)

	// create msg deposit
	msgWithdraw := exchangeTypes.MsgWithdraw{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgWithdraw)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgWithdrawStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgWithdrawStr:", msgWithdrawStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(500),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgWithdrawStr,
	}

	fmt.Println("msgDepositClaim:", msgDepositClaim)

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())

	// Withdrawal is not successful. So the balance of subaccount should remain same.
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, amountToDepositInSubaccount.Amount.ToDec())
}

func TestDepositClaimAmountExceedsMsgAmount(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5") // Pickle
		amountToDeposit   = types.NewERC20Token(414, tokenContractAddr).PeggyCoin()
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(400),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())
}

func TestDepositInvalidSigner(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr     = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr  = sdk.AccAddress(common.FromHex(senderEthAddr))
		depositSender, _  = sdk.AccAddressFromBech32("inj1ee7xnhczhvu064utmdn48sh0wx7zq3pgnuuxfl")
		tokenContractAddr = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5") // Pickle
		amountToDeposit   = types.NewERC20Token(44, tokenContractAddr).PeggyCoin()
		subaccountID      = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       depositSender.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(400),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())
}

func TestDepositInvalidDenom(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	var (
		senderEthAddr          = "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
		senderInjAccAddr       = sdk.AccAddress(common.FromHex(senderEthAddr))
		tokenContractAddr      = common.HexToAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5") // Pickle
		wrongTokenContractAddr = common.HexToAddress("0x2339490999C7574E3bd46DEbC603fADa5EeE2CE8") // Pickle
		amountToDeposit        = types.NewERC20Token(44, wrongTokenContractAddr).PeggyCoin()
		subaccountID           = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
	)

	allVouchers := sdk.Coins{amountToDeposit}

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, senderInjAccAddr)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderInjAccAddr, allVouchers))

	// create msg deposit
	msgDeposit := exchangeTypes.MsgDeposit{
		Sender:       senderInjAccAddr.String(),
		SubaccountId: subaccountID,
		Amount:       amountToDeposit,
	}

	any, err := codectypes.NewAnyWithValue(&msgDeposit)
	assert.Nil(t, err)

	jm := &jsonpb.Marshaler{}
	msgDepositStr, err := jm.MarshalToString(any)
	assert.Nil(t, err)

	fmt.Println("msgDepositStr:", msgDepositStr)
	// create a deposit claim
	msgDepositClaim := types.MsgDepositClaim{
		EthereumSender: senderEthAddr,
		Amount:         sdk.NewInt(400),
		TokenContract:  tokenContractAddr.Hex(),
		Data:           msgDepositStr,
	}

	input.PeggyKeeper.ProcessClaimData(ctx, &msgDepositClaim)

	subAccountDepositBalance := input.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), amountToDeposit.GetDenom())
	// Deposit is not successful. So the balance should be zero
	assert.Equal(t, subAccountDepositBalance.AvailableBalance, sdk.ZeroDec())
}
