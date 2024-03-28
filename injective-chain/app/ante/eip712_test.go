package ante_test

import (
	"encoding/hex"
	"testing"

	"github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/app/ante"
	"github.com/InjectiveLabs/injective-core/injective-chain/crypto/ethsecp256k1"
	tftypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestItShouldRejectUnsupportedSignMode(t *testing.T) {
	encodingConfig := app.MakeEncodingConfig()
	ante.GlobalCdc = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)
	txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	account := "inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z"
	msgBankSend := &banktypes.MsgSend{
		FromAddress: account,
		ToAddress:   "inj1995xnrrtnmtdgjmx0g937vf28dwefhkhy6gy5e",
		Amount: []types.Coin{
			types.NewCoin("inj", types.NewInt(10000000000000000)),
		},
	}
	pubKeyHex := "0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	feePayerSigHex := "e68ae45cfd3bb49f5e9ba96a2223ec795b44a8cc8b3036e2f5dbc04ffc05288a418276a7101c9f948b69fbae7dedb21f00e62a92e2445ff186b13cd287c71ee200"
	senderSigHex := "ee4363f86e6ec840c9d3c8ed6be763feb3c7c46bed12934515dc377df88129e503782e9728ccc6bc4a761c8a4413ac0931d4bad8457f6ba7e9b11fd8c98f87781b"

	sig, err := hex.DecodeString(senderSigHex)
	require.NoError(t, err, "unexpected decode signature")
	pubKey := &ethsecp256k1.PubKey{
		Key: common.FromHex(pubKeyHex),
	}

	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 5,
	}
	extOpts.FeePayer = "inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku"
	extOpts.FeePayerSig, _ = hex.DecodeString(feePayerSigHex)

	option, err := codectypes.NewAnyWithValue(extOpts)
	require.NoError(t, err, "unexpected marshal extOpts to Any")
	txBuilder.SetExtensionOptions(option)
	txBuilder.SetGasLimit(200000)
	txBuilder.SetMsgs(msgBankSend)

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_DIRECT, // unsupported signmode
		Signature: sig,
	}, txBuilder.GetTx())
	require.Contains(t, err.Error(), "wrong SignMode")
}

/*
	prepareTx response: {
	  data: '{"types":{"EIP712Domain":[{"name":"name","type":"string"},{"name":"version","type":"string"},{"name":"chainId","type":"uint256"},{"name":"verifyingContract","type":"string"},{"name":"salt","type":"string"}],"Tx":[{"name":"context","type":"string"},{"name":"msgs","type":"string"}]},"primaryType":"Tx","domain":{"name":"Injective Web3","version":"1.0.0","chainId":"0x5","verifyingContract":"cosmos","salt":"0"},"message":{"context":"{\\"account_number\\":2,\\"chain_id\\":\\"injective-777\\",\\"fee\\":{\\"amount\\":[{\\"denom\\":\\"inj\\",\\"amount\\":\\"100000000000000\\"}],\\"gas\\":200000,\\"payer\\":\\"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku\\"},\\"memo\\":\\"\\",\\"sequence\\":0,\\"timeout_height\\":34435}","msgs":"[{\\"@type\\":\\"/cosmos.bank.v1beta1.MsgSend\\",\\"from_address\\":\\"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z\\",\\"to_address\\":\\"inj1995xnrrtnmtdgjmx0g937vf28dwefhkhy6gy5e\\",\\"amount\\":[{\\"denom\\":\\"inj\\",\\"amount\\":\\"10000000000000000\\"}]}]"}}',
	  sequence: 0,
	  signMode: 'SIGN_MODE_EIP712_V2',
	  pubKeyType: '/injective.crypto.v1beta1.ethsecp256k1.PubKey',
	  feePayer: 'inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku',
	  feePayerSig: '0xe68ae45cfd3bb49f5e9ba96a2223ec795b44a8cc8b3036e2f5dbc04ffc05288a418276a7101c9f948b69fbae7dedb21f00e62a92e2445ff186b13cd287c71ee200'
	}

fee payer sig: 0xe68ae45cfd3bb49f5e9ba96a2223ec795b44a8cc8b3036e2f5dbc04ffc05288a418276a7101c9f948b69fbae7dedb21f00e62a92e2445ff186b13cd287c71ee200
singer signature: ee4363f86e6ec840c9d3c8ed6be763feb3c7c46bed12934515dc377df88129e503782e9728ccc6bc4a761c8a4413ac0931d4bad8457f6ba7e9b11fd8c98f87781b
signer pub key: 0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296

	tx: {
	  context: '{"account_number":2,"chain_id":"injective-777","fee":{"amount":[{"denom":"inj","amount":"100000000000000"}],"gas":200000,"payer":"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku"},"memo":"","sequence":0,"timeout_height":34435}',
	  msgs: '[{"@type":"/cosmos.bank.v1beta1.MsgSend","from_address":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","to_address":"inj1995xnrrtnmtdgjmx0g937vf28dwefhkhy6gy5e","amount":[{"denom":"inj","amount":"10000000000000000"}]}]'
	}
*/

func TestItShouldPassVerifyEIP712V2ButCannotVerifyV1(t *testing.T) {
	encodingConfig := app.MakeEncodingConfig()
	ante.GlobalCdc = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)

	txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	account := "inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z"
	msgBankSend := &banktypes.MsgSend{
		FromAddress: account,
		ToAddress:   "inj1995xnrrtnmtdgjmx0g937vf28dwefhkhy6gy5e",
		Amount: []types.Coin{
			types.NewCoin("inj", types.NewInt(10000000000000000)),
		},
	}

	pubKeyHex := "0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	feePayerSigHex := "e68ae45cfd3bb49f5e9ba96a2223ec795b44a8cc8b3036e2f5dbc04ffc05288a418276a7101c9f948b69fbae7dedb21f00e62a92e2445ff186b13cd287c71ee200"
	senderSigHex := "ee4363f86e6ec840c9d3c8ed6be763feb3c7c46bed12934515dc377df88129e503782e9728ccc6bc4a761c8a4413ac0931d4bad8457f6ba7e9b11fd8c98f87781b"

	sig, err := hex.DecodeString(senderSigHex)
	require.NoError(t, err, "unexpected decode signature")
	pubKey := &ethsecp256k1.PubKey{
		Key: common.FromHex(pubKeyHex),
	}

	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 5,
	}
	extOpts.FeePayer = "inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku"
	extOpts.FeePayerSig, _ = hex.DecodeString(feePayerSigHex)

	option, err := codectypes.NewAnyWithValue(extOpts)
	require.NoError(t, err, "unexpected marshal extOpts to Any")

	txBuilder.SetExtensionOptions(option)
	txBuilder.SetGasLimit(200000)
	txBuilder.SetMsgs(msgBankSend)
	txBuilder.SetTimeoutHeight(34435)
	txBuilder.SetFeeAmount(types.NewCoins(types.NewCoin("inj", types.NewInt(100000000000000))))

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_EIP712_V2,
		Signature: sig,
	}, txBuilder.GetTx())
	require.NoError(t, err, "VerifySignatureEIP712 should return nil")

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: sig,
	}, txBuilder.GetTx())
	require.Contains(t, err.Error(), "failed to verify delegated fee payer sig")
}

/*
	prepareTx response: {
	  data: '{"types":{"Coin":[{"name":"denom","type":"string"},{"name":"amount","type":"string"}],"EIP712Domain":[{"name":"name","type":"string"},{"name":"version","type":"string"},{"name":"chainId","type":"uint256"},{"name":"verifyingContract","type":"string"},{"name":"salt","type":"string"}],"Fee":[{"name":"feePayer","type":"string"},{"name":"amount","type":"Coin[]"},{"name":"gas","type":"string"}],"Msg":[{"name":"type","type":"string"},{"name":"value","type":"MsgValue"}],"MsgValue":[{"name":"from_address","type":"string"},{"name":"to_address","type":"string"},{"name":"amount","type":"TypeAmount[]"}],"Tx":[{"name":"account_number","type":"string"},{"name":"chain_id","type":"string"},{"name":"fee","type":"Fee"},{"name":"memo","type":"string"},{"name":"msgs","type":"Msg[]"},{"name":"sequence","type":"string"},{"name":"timeout_height","type":"string"}],"TypeAmount":[{"name":"denom","type":"string"},{"name":"amount","type":"string"}]},"primaryType":"Tx","domain":{"name":"Injective Web3","version":"1.0.0","chainId":"0x5","verifyingContract":"cosmos","salt":"0"},"message":{"account_number":"11","chain_id":"injective-777","fee":{"amount":[{"amount":"100000000000000","denom":"inj"}],"feePayer":"inj18j2myhaf2at75kwwaqxfstk4q28n4am45nlfg7","gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgSend","value":{"amount":[{"amount":"10000000000000000","denom":"inj"}],"from_address":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","to_address":"inj1995xnrrtnmtdgjmx0g937vf28dwefhkhy6gy5e"}}],"sequence":"85","timeout_height":"60821"}}',
	  sequence: 85,
	  signMode: 'SIGN_MODE_LEGACY_AMINO_JSON',
	  pubKeyType: '/injective.crypto.v1beta1.ethsecp256k1.PubKey',
	  feePayer: 'inj18j2myhaf2at75kwwaqxfstk4q28n4am45nlfg7',
	  feePayerSig: '0x4785c7554ca3a3310d854ddc64e7db8367def2e63834c889e0b560f6dbb4299f27005b5bf31aedead67704db8f885a8e8e01d2a5f88101c59a06b5b70fd8874400'
	}

fee payer sig: 0x4785c7554ca3a3310d854ddc64e7db8367def2e63834c889e0b560f6dbb4299f27005b5bf31aedead67704db8f885a8e8e01d2a5f88101c59a06b5b70fd8874400
signature: d3455fc7440a61f270d7bdc9a743d488c822db38d0f2c6d2722579eba25ade903d2e69bf71d2b9e8d3acccdbcdb7d38fb3bfcbd97d287dc7bf642b04546b111b1b

	tx: {
	  account_number: '11',
	  chain_id: 'injective-777',
	  fee: {
	    amount: [ [Object] ],
	    feePayer: 'inj18j2myhaf2at75kwwaqxfstk4q28n4am45nlfg7',
	    gas: '200000'
	  },
	  memo: '',
	  msgs: [ { type: 'cosmos-sdk/MsgSend', value: [Object] } ],
	  sequence: '85',
	  timeout_height: '60821'
	}

pub key: 0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296
*/
func TestItShouldPassVerifyEIP712_V1(t *testing.T) {
	encodingConfig := app.MakeEncodingConfig()
	ante.GlobalCdc = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)

	txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	account := "inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z"
	msgBankSend := &banktypes.MsgSend{
		FromAddress: account,
		ToAddress:   "inj1995xnrrtnmtdgjmx0g937vf28dwefhkhy6gy5e",
		Amount: []types.Coin{
			types.NewCoin("inj", types.NewInt(10000000000000000)),
		},
	}

	pubKeyHex := "0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	feePayerSigHex := "4785c7554ca3a3310d854ddc64e7db8367def2e63834c889e0b560f6dbb4299f27005b5bf31aedead67704db8f885a8e8e01d2a5f88101c59a06b5b70fd8874400"
	senderSigHex := "d3455fc7440a61f270d7bdc9a743d488c822db38d0f2c6d2722579eba25ade903d2e69bf71d2b9e8d3acccdbcdb7d38fb3bfcbd97d287dc7bf642b04546b111b1b"

	sig, err := hex.DecodeString(senderSigHex)
	require.NoError(t, err, "unexpected decode signature")
	pubKey := &ethsecp256k1.PubKey{
		Key: common.FromHex(pubKeyHex),
	}

	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 5,
	}
	extOpts.FeePayer = "inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku"
	extOpts.FeePayerSig, _ = hex.DecodeString(feePayerSigHex)

	option, err := codectypes.NewAnyWithValue(extOpts)
	require.NoError(t, err, "unexpected marshal extOpts to Any")

	txBuilder.SetExtensionOptions(option)
	txBuilder.SetGasLimit(200000)
	txBuilder.SetMsgs(msgBankSend)
	txBuilder.SetTimeoutHeight(60821)
	txBuilder.SetFeeAmount(types.NewCoins(types.NewCoin("inj", types.NewInt(100000000000000))))

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 11,
		Sequence:      85,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: sig,
	}, txBuilder.GetTx())
	require.Contains(t, err.Error(), "failed to verify delegated fee payer sig")
}

/*
prepareTx response: {
  data: '{"types":{"EIP712Domain":[{"name":"name","type":"string"},{"name":"version","type":"string"},{"name":"chainId","type":"uint256"},{"name":"verifyingContract","type":"string"},{"name":"salt","type":"string"}],"Tx":[{"name":"context","type":"string"},{"name":"msgs","type":"string"}]},"primaryType":"Tx","domain":{"name":"Injective Web3","version":"1.0.0","chainId":"0x5","verifyingContract":"cosmos","salt":"0"},"message":{"context":"{\\"account_number\\":2,\\"chain_id\\":\\"injective-777\\",\\"fee\\":{\\"amount\\":[{\\"denom\\":\\"inj\\",\\"amount\\":\\"2000000000000000000000000\\"}],\\"gas\\":200000},\\"memo\\":\\"\\",\\"sequence\\":0,\\"timeout_height\\":39953}","msgs":"[{\\"@type\\":\\"/injective.tokenfactory.v1beta1.MsgCreateDenom\\",\\"sender\\":\\"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z\\",\\"subdenom\\":\\"phuc\\",\\"name\\":\\"Phuc coin\\",\\"symbol\\":\\"PHUC\\"}]"}}',
  sequence: 0,
  signMode: 'SIGN_MODE_EIP712_V2',
  pubKeyType: '/injective.crypto.v1beta1.ethsecp256k1.PubKey'
}
fee payer sig: undefined
signature: 873b4c3e791f99c70a21a896ec144f3304fcfb0dc2388143c66d35b0ce7b1eca023ba2e82dc8dceac103defdf76d42f910bd23445ee3d9989d40bd863ace8a3f1b
tx: {
  context: '{"account_number":2,"chain_id":"injective-777","fee":{"amount":[{"denom":"inj","amount":"2000000000000000000000000"}],"gas":200000},"memo":"","sequence":0,"timeout_height":39953}',
  	        {"account_number":2,"chain_id":"injective-777","fee":{"amount":[{"denom":"inj","amount":"2000000000000000000000000"}],"gas":200000},"memo":"","sequence":0,"timeout_height":38935}
  msgs: '[{"@type":"/injective.tokenfactory.v1beta1.MsgCreateDenom","sender":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","subdenom":"phuc","name":"Phuc coin","symbol":"PHUC"}]'
         [{"@type":"/injective.tokenfactory.v1beta1.MsgCreateDenom","sender":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","subdenom":"phuc","name":"Phuc coin","symbol":"PHUC"}]
}
pub key: 0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296
*/

func TestItShouldPassVerifyEIP712V2_NoFeeDelegation(t *testing.T) {
	encodingConfig := app.MakeEncodingConfig()
	ante.GlobalCdc = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)
	txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)

	account := "inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z"
	msgCreateDenom := &tftypes.MsgCreateDenom{
		Sender:   account,
		Subdenom: "phuc",
		Name:     "Phuc coin",
		Symbol:   "PHUC",
	}

	pubKeyHex := "0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	senderSigHex := "873b4c3e791f99c70a21a896ec144f3304fcfb0dc2388143c66d35b0ce7b1eca023ba2e82dc8dceac103defdf76d42f910bd23445ee3d9989d40bd863ace8a3f1b"

	sig, err := hex.DecodeString(senderSigHex)
	require.NoError(t, err, "unexpected decode signature")
	pubKey := &ethsecp256k1.PubKey{
		Key: common.FromHex(pubKeyHex),
	}

	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 5,
	}
	option, err := codectypes.NewAnyWithValue(extOpts)
	require.NoError(t, err, "unexpected marshal extOpts to Any")
	txBuilder.SetExtensionOptions(option)

	txBuilder.SetGasLimit(200000)
	txBuilder.SetMsgs(msgCreateDenom)
	txBuilder.SetTimeoutHeight(39953)
	txBuilder.SetFeeAmount(types.NewCoins(types.NewCoin("inj", types.MustNewDecFromStr("2000000000000000000000000").RoundInt())))

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_EIP712_V2,
		Signature: sig,
	}, txBuilder.GetTx())
	require.NoError(t, err, "there must be no error when verifying EIP712 signature without feeDelegation")
}

/*
	prepareTx response: {
	  data: '{"types":{"EIP712Domain":[{"name":"name","type":"string"},{"name":"version","type":"string"},{"name":"chainId","type":"uint256"},{"name":"verifyingContract","type":"string"},{"name":"salt","type":"string"}],"Tx":[{"name":"context","type":"string"},{"name":"msgs","type":"string"}]},"primaryType":"Tx","domain":{"name":"Injective Web3","version":"1.0.0","chainId":"0x5","verifyingContract":"cosmos","salt":"0"},"message":{"context":"{\\"account_number\\":2,\\"chain_id\\":\\"injective-777\\",\\"fee\\":{\\"amount\\":[{\\"denom\\":\\"inj\\",\\"amount\\":\\"2000000000000000000000000\\"}],\\"gas\\":200000},\\"memo\\":\\"\\",\\"sequence\\":0,\\"timeout_height\\":167}","msgs":"[{\\"@type\\":\\"/cosmos.authz.v1beta1.MsgExec\\",\\"grantee\\":\\"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z\\",\\"msgs\\":[{\\"@type\\":\\"/injective.tokenfactory.v1beta1.MsgCreateDenom\\",\\"sender\\":\\"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku\\",\\"subdenom\\":\\"phuc\\",\\"name\\":\\"Phuc coin\\",\\"symbol\\":\\"PHUC\\"},{\\"@type\\":\\"/cosmos.bank.v1beta1.MsgSend\\",\\"from_address\\":\\"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku\\",\\"to_address\\":\\"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z\\",\\"amount\\":[{\\"denom\\":\\"inj\\",\\"amount\\":\\"10000000000000000\\"}]}]}]"}}',
	  sequence: 0,
	  signMode: 'SIGN_MODE_EIP712_V2',
	  pubKeyType: '/injective.crypto.v1beta1.ethsecp256k1.PubKey'
	}

fee payer sig: undefined
signature: 630e8fc44c03b8f3bef47c2f90f6ffefee8feb69e79a3db9fd6f954543a5a9b447feefccdf3a4802ef92fd79b9016e219197543a4528def8a06c21e4bebccaab1b

	tx: {
	  context: '{"account_number":2,"chain_id":"injective-777","fee":{"amount":[{"denom":"inj","amount":"2000000000000000000000000"}],"gas":200000},"memo":"","sequence":0,"timeout_height":167}',
	            {"account_number":2,"chain_id":"injective-777","fee":{"amount":[{"denom":"inj","amount":"2000000000000000000000000"}],"gas":200000},"memo":"","sequence":0,"timeout_height":167}
	  msgs: '[{"@type":"/cosmos.authz.v1beta1.MsgExec","grantee":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","msgs":[{"@type":"/injective.tokenfactory.v1beta1.MsgCreateDenom","sender":"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku","subdenom":"phuc","name":"Phuc coin","symbol":"PHUC"},{"@type":"/cosmos.bank.v1beta1.MsgSend","from_address":"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku","to_address":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","amount":[{"denom":"inj","amount":"10000000000000000"}]}]}]'
	         [{"@type":"/cosmos.authz.v1beta1.MsgExec","grantee":"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku","msgs":[{"@type":"/injective.tokenfactory.v1beta1.MsgCreateDenom","sender":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","subdenom":"phuc","name":"Phuc coin","symbol":"PHUC"},{"@type":"/cosmos.bank.v1beta1.MsgSend","from_address":"inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z","to_address":"inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku","amount":[{"denom":"inj","amount":"10000000000000000"}]}]}]
	}

pub key: 0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296
*/
func TestVerifyAuthzMessageEIP712_V2_ButFailedToVerifyByV1(t *testing.T) {
	encodingConfig := app.MakeEncodingConfig()
	ante.GlobalCdc = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)
	txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)

	// txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	granter := "inj14au322k9munkmx5wrchz9q30juf5wjgz2cfqku"
	grantee := "inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z"
	msgCreateDenom := &tftypes.MsgCreateDenom{
		Sender:   granter,
		Subdenom: "phuc",
		Name:     "Phuc coin",
		Symbol:   "PHUC",
	}

	msgSend := &banktypes.MsgSend{
		FromAddress: granter,
		ToAddress:   grantee,
		Amount: []types.Coin{
			types.NewCoin("inj", types.NewInt(10000000000000000)),
		},
	}

	msgCreateDenomAny, err := codectypes.NewAnyWithValue(msgCreateDenom)
	require.NoError(t, err, "unexpected error when wrap msgCreateDenom to any")

	msgSendAny, err := codectypes.NewAnyWithValue(msgSend)
	require.NoError(t, err, "unexpected error when wrap msgSend to any")

	msg := &authz.MsgExec{
		Grantee: grantee,
		Msgs:    []*codectypes.Any{msgCreateDenomAny, msgSendAny},
	}

	pubKeyHex := "0310c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	senderSigHex := "630e8fc44c03b8f3bef47c2f90f6ffefee8feb69e79a3db9fd6f954543a5a9b447feefccdf3a4802ef92fd79b9016e219197543a4528def8a06c21e4bebccaab1b"

	sig, err := hex.DecodeString(senderSigHex)
	require.NoError(t, err, "unexpected decode signature")
	pubKey := &ethsecp256k1.PubKey{
		Key: common.FromHex(pubKeyHex),
	}

	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 5,
	}
	option, err := codectypes.NewAnyWithValue(extOpts)
	require.NoError(t, err, "unexpected marshal extOpts to Any")
	txBuilder.SetExtensionOptions(option)

	txBuilder.SetGasLimit(200000)
	txBuilder.SetMsgs(msg)
	txBuilder.SetTimeoutHeight(167)
	txBuilder.SetFeeAmount(types.NewCoins(types.NewCoin("inj", types.MustNewDecFromStr("2000000000000000000000000").RoundInt())))

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       grantee,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_EIP712_V2,
		Signature: sig,
	}, txBuilder.GetTx())
	require.NoError(t, err, "there must be no error when verifying EIP712 signature without feeDelegation")

	// it cannot verify using current EIP712 due to the 2 types in []*Any{} are different
	// the error should be failed to pack and hash, not signature mismatch
	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(pubKey, authsigning.SignerData{
		Address:       grantee,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: sig,
	}, txBuilder.GetTx())
	require.Contains(t, err.Error(), "failed to pack and hash typedData primary type")
}

func TestItShouldFailedToVerifyDueToInvalidPublicKey(t *testing.T) {
	encodingConfig := app.MakeEncodingConfig()
	ante.GlobalCdc = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)
	txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)

	account := "inj17gkuet8f6pssxd8nycm3qr9d9y699rupv6397z"
	msgCreateDenom := &tftypes.MsgCreateDenom{
		Sender:   account,
		Subdenom: "phuc",
		Name:     "Phuc coin",
		Symbol:   "PHUC",
	}

	fakePubKeyHex := "0410c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	realPubKeyHex := "0410c141ef33adca78ca32bb062ee63edb758178311065f74f90562da0424f3296"
	realSenderSigHex := "873b4c3e791f99c70a21a896ec144f3304fcfb0dc2388143c66d35b0ce7b1eca023ba2e82dc8dceac103defdf76d42f910bd23445ee3d9989d40bd863ace8a3f1b"
	fakeSenderSigHex := "873b4c3e791f99c70a21a896ec144f3304fcfb0dc2388143c66d35b0ce7b1eca023ba2e82dc8dceac103defdf76d42f910bd23445ee3d9989d40bd863ace8a3f1b"

	realSig, err := hex.DecodeString(realSenderSigHex)
	require.NoError(t, err, "unexpected decode signature")
	fakeSig, err := hex.DecodeString(fakeSenderSigHex)
	require.NoError(t, err, "unexpected decode signature")

	fakePubKey := &ethsecp256k1.PubKey{
		Key: common.FromHex(fakePubKeyHex),
	}
	realPubkey := &ethsecp256k1.PubKey{
		Key: common.FromHex(realPubKeyHex),
	}

	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 5,
	}
	option, err := codectypes.NewAnyWithValue(extOpts)
	require.NoError(t, err, "unexpected marshal extOpts to Any")
	txBuilder.SetExtensionOptions(option)

	txBuilder.SetGasLimit(200000)
	txBuilder.SetMsgs(msgCreateDenom)
	txBuilder.SetTimeoutHeight(39953)
	txBuilder.SetFeeAmount(types.NewCoins(types.NewCoin("inj", types.MustNewDecFromStr("2000000000000000000000000").RoundInt())))

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(fakePubKey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_EIP712_V2,
		Signature: realSig,
	}, txBuilder.GetTx())
	require.Contains(t, err.Error(), "unable to verify signer signature of EIP712 typed data")

	_, err = ante.GenerateTypedDataAndVerifySignatureEIP712(realPubkey, authsigning.SignerData{
		Address:       account,
		ChainID:       "injective-777",
		AccountNumber: 2,
		Sequence:      0,
	}, &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_EIP712_V2,
		Signature: fakeSig,
	}, txBuilder.GetTx())
	require.Contains(t, err.Error(), "unable to verify signer signature of EIP712 typed data")
}
