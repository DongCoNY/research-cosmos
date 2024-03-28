package types_test

import (
	sdksecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/InjectiveLabs/injective-core/injective-chain/crypto/ethsecp256k1"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Reclamation", func() {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	Describe("Validate Reclamation", func() {

		// example public key
		pubKeyBz := common.Hex2Bytes("035ddc4d5642b9383e2f087b2ee88b7207f6286ebc9f310e9df1406eccc2c31813")

		lockedPubKey := sdksecp256k1.PubKey{
			Key: pubKeyBz,
		}

		correctPubKey := ethsecp256k1.PubKey{
			Key: pubKeyBz,
		}

		lockedAddress := sdk.AccAddress(lockedPubKey.Address())
		correctAddress := sdk.AccAddress(correctPubKey.Address())
		//balancesToUnlock := sdk.NewCoins(sdk.NewCoin("inj", sdk.NewInt(69)))

		It("derived addresses should be valid", func() {

			expectedLockedAddress := "inj1yd5dkwkzthdwvnd67c5j8qc38t5v7k208szaq2"
			expectedCorrectAddress := "inj1hkhdaj2a2clmq5jq6mspsggqs32vynpk228q3r"

			Expect(lockedAddress.String()).To(Equal(expectedLockedAddress))
			Expect(correctAddress.String()).To(Equal(expectedCorrectAddress))
		})

		It("should be valid with correct signature for a new private key + signature", func() {
			privKey, err := ethsecp256k1.GenerateKey()
			Expect(err).To(BeNil())

			pubKeyBz := privKey.PubKey().Bytes()

			lockedPubKey := sdksecp256k1.PubKey{
				Key: pubKeyBz,
			}

			correctPubKey := ethsecp256k1.PubKey{
				Key: pubKeyBz,
			}

			lockedAddress := sdk.AccAddress(lockedPubKey.Address())
			correctAddress := sdk.AccAddress(correctPubKey.Address())

			data := types.ConstructFundsReclaimMessage(
				correctAddress,
				lockedAddress,
			)

			msgSignData := types.MsgSignData{
				Signer: lockedAddress.Bytes(),
				Data:   []byte(data),
			}

			stdTx := legacytx.NewStdTx(
				[]sdk.Msg{&types.MsgSignDoc{
					SignType: "sign/MsgSignData",
					Value:    msgSignData,
				}},
				legacytx.StdFee{
					Amount: sdk.Coins{},
					Gas:    0,
				},
				[]legacytx.StdSignature{
					{
						PubKey:    &lockedPubKey,
						Signature: nil,
					},
				},
				"",
			)

			aminoJSONHandler := legacytx.NewStdTxSignModeHandler()

			signingData := signing.SignerData{
				ChainID:       "",
				AccountNumber: 0,
				Sequence:      0,
			}

			signBz, err := aminoJSONHandler.GetSignBytes(
				signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
				signingData,
				stdTx,
			)

			Expect(err).To(BeNil())

			sdkPrivKey := sdksecp256k1.PrivKey{
				Key: privKey.Bytes(),
			}

			signature, err := sdkPrivKey.Sign(signBz)
			Expect(err).To(BeNil())

			ok := lockedPubKey.VerifySignature(signBz, signature)
			Expect(ok).To(BeTrue())

			msgReclaim := types.MsgReclaimLockedFunds{
				Sender:              correctAddress.String(),
				LockedAccountPubKey: pubKeyBz,
				Signature:           signature,
			}

			err = msgReclaim.ValidateBasic()
			Expect(err).To(BeNil())
		})
	})
})
