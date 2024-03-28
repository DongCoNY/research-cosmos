package keeper_test

import (
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdksecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/crypto/ethsecp256k1"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Account MsgServer Tests", func() { // TODO - Test events
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		msgServer types.MsgServer
		ctx       sdk.Context
		baseDenom string
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)
	sender := sdk.AccAddress(common.HexToAddress("0x90f8bf6a479f320ead074411a4b0e7944ea8c9c1").Bytes())

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
		baseDenom = testInput.Spots[0].BaseDenom

	})

	Describe("Deposit", func() {
		var (
			err                        error
			subaccountID               common.Hash
			amountToDeposit            sdk.Coin
			shouldInitializeSubaccount bool
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
		})

		JustBeforeEach(func() {
			if shouldInitializeSubaccount {
				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				app.ExchangeKeeper.SetDeposit(ctx, subaccountID, baseDenom, zeroDeposit)
			}

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountID.Hex(),
				Amount:       amountToDeposit,
			}

			err = deposit.ValidateBasic()
			if err == nil {
				_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
			}
		})

		Context("With all valid fields to an existing subaccount", func() {
			BeforeEach(func() {
				shouldInitializeSubaccount = true
				subaccountID = exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
				amountToDeposit = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				deposit := testexchange.GetBankAndDepositFunds(app, ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
				Expect(deposit.TotalBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
			})
		})

		Context("With all valid fields to a non-existing subaccount", func() {
			BeforeEach(func() {
				shouldInitializeSubaccount = false
				subaccountID = common.HexToHash("0x2968698C6b9Ed6D44b667a0b1F312a3b5D94Ded7000000000000000000000001")
				amountToDeposit = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				deposit := testexchange.GetBankAndDepositFunds(app, ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
				Expect(deposit.TotalBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
			})
		})

		Context("With empty subaccountID", func() {
			BeforeEach(func() {
				shouldInitializeSubaccount = true
				subaccountID = common.Hash{}
				amountToDeposit = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be invalid", func() {
				Expect(errors.Is(err, exchangetypes.ErrBadSubaccountID)).To(BeTrue())
			})

			It("Should NOT have updated balances", func() {
				defaultSubaccountID := exchangetypes.SdkAddressToSubaccountID(sender)
				deposit := app.ExchangeKeeper.GetDeposit(ctx, defaultSubaccountID, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(sdk.ZeroDec()))
				Expect(deposit.TotalBalance).To(Equal(sdk.ZeroDec()))
			})
		})

		Context("With more funds than sender owns", func() {
			BeforeEach(func() {
				shouldInitializeSubaccount = true
				subaccountID = exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
				amountToDeposit = sdk.NewCoin(baseDenom, sdk.NewInt(51))
			})

			It("Should be invalid with insufficient funds error", func() {
				errorMessage := "spendable balance " + sdk.NewInt(50).String() + baseDenom + " is smaller than " + amountToDeposit.Amount.String() + baseDenom + ": "
				expectedError := "deposit failed: " + errorMessage + sdkerrors.ErrInsufficientFunds.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(deposit.TotalBalance).To(Equal(sdk.NewDec(0)))
			})
		})

		Context("With not existing baseDenom", func() {
			BeforeEach(func() {
				shouldInitializeSubaccount = true
				fakeBaseDenom := "SMTH"
				subaccountID = exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
				amountToDeposit = sdk.NewCoin(fakeBaseDenom, sdk.NewInt(50))
			})

			It("Should be invalid with invalid coins error", func() {
				expectedError := sdkerrors.ErrInvalidCoins.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(deposit.TotalBalance).To(Equal(sdk.NewDec(0)))
			})
		})
	})

	Describe("Deposit with simplified subaccount id", func() {
		var (
			err             error
			subaccountID    string
			amountToDeposit sdk.Coin
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
		})

		JustBeforeEach(func() {
			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountID,
				Amount:       amountToDeposit,
			}

			err = deposit.ValidateBasic()
			if err == nil {
				_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
			}
		})

		Context("With non-default simplified subaccount id", func() {
			BeforeEach(func() {
				subaccountID = "1"
				amountToDeposit = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				subaccountIdHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountID)
				deposit := testexchange.GetBankAndDepositFunds(app, ctx, subaccountIdHash, baseDenom)

				Expect(deposit.AvailableBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
				Expect(deposit.TotalBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
			})
		})
	})

	Describe("Deposit to external subaccount id", func() {
		var (
			err             error
			subaccountID    common.Hash
			amountToDeposit sdk.Coin
			receiver        = sdk.AccAddress(common.HexToAddress("inj1k76gksekskykj5q5dv4tj5squf8fw307nkl99n").Bytes())
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
		})

		JustBeforeEach(func() {
			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountID.Hex(),
				Amount:       amountToDeposit,
			}

			err = deposit.ValidateBasic()
			if err == nil {
				_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
			}
		})

		Context("With non-default simplified subaccount id", func() {
			BeforeEach(func() {
				subaccountID = types.MustSdkAddressWithNonceToSubaccountID(receiver, 1)
				amountToDeposit = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				deposit := testexchange.GetBankAndDepositFunds(app, ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
				Expect(deposit.TotalBalance.String()).To(Equal(amountToDeposit.Amount.ToDec().String()))
			})
		})
	})

	Describe("Withdrawal", func() {
		var (
			err              error
			subaccountID     common.Hash
			amountToWithdraw sdk.Coin
			startingDeposit  *types.Deposit
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountID := exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountID.Hex(),
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)

			startingDeposit = app.ExchangeKeeper.GetDeposit(ctx, subaccountID, baseDenom)
		})

		JustBeforeEach(func() {
			withdrawal := &types.MsgWithdraw{
				Sender:       sender.String(),
				SubaccountId: subaccountID.Hex(),
				Amount:       amountToWithdraw,
			}

			_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), withdrawal)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				subaccountID = exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
				amountToWithdraw = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(deposit.TotalBalance).To(Equal(sdk.NewDec(0)))
			})
		})

		Context("With more funds than sender owns", func() {
			BeforeEach(func() {
				subaccountID = exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
				amountToWithdraw = sdk.NewCoin(baseDenom, sdk.NewInt(51))
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := "withdrawal failed: " + types.ErrInsufficientDeposit.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(startingDeposit.AvailableBalance))
				Expect(deposit.TotalBalance).To(Equal(startingDeposit.TotalBalance))
			})
		})

		Context("With not existing baseDenom", func() {
			BeforeEach(func() {
				fakeBaseDenom := "SMTH"
				subaccountID = exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
				amountToWithdraw = sdk.NewCoin(fakeBaseDenom, sdk.NewInt(50))
			})

			It("Should be invalid with invalid coins error", func() {
				expectedError := sdkerrors.ErrInvalidCoins.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subaccountID, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(startingDeposit.AvailableBalance))
				Expect(deposit.TotalBalance).To(Equal(startingDeposit.TotalBalance))
			})
		})
	})

	Describe("Withdrawal with simplified subaccount id", func() {
		var (
			err              error
			subaccountID     string
			amountToWithdraw sdk.Coin
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountID := exchangetypes.MustSdkAddressWithNonceToSubaccountID(sender, 1)
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountID.Hex(),
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
		})

		JustBeforeEach(func() {
			withdrawal := &types.MsgWithdraw{
				Sender:       sender.String(),
				SubaccountId: subaccountID,
				Amount:       amountToWithdraw,
			}

			_, err = msgServer.Withdraw(sdk.WrapSDKContext(ctx), withdrawal)
		})

		Context("With non-default simplified subaccount id", func() {
			BeforeEach(func() {
				subaccountID = "1"
				amountToWithdraw = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				subaccountIdHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountID)
				deposit := app.ExchangeKeeper.GetDeposit(ctx, subaccountIdHash, baseDenom)

				Expect(deposit.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(deposit.TotalBalance).To(Equal(sdk.NewDec(0)))
			})
		})
	})

	Describe("SubaccountTransfer", func() {
		var (
			err              error
			subaccountIDFrom string
			subaccountIDTo   string
			amountToTransfer sdk.Coin
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountIDFrom := "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountIDFrom,
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
		})

		JustBeforeEach(func() {
			subaccountTransfer := &types.MsgSubaccountTransfer{
				Sender:                  sender.String(),
				SourceSubaccountId:      subaccountIDFrom,
				DestinationSubaccountId: subaccountIDTo,
				Amount:                  amountToTransfer,
			}

			_, err = msgServer.SubaccountTransfer(sdk.WrapSDKContext(ctx), subaccountTransfer)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				subaccountIDFrom = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
				subaccountIDTo = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				app.ExchangeKeeper.SetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDFrom), baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(0)))

				depositTo := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(50)))
			})
		})

		Context("With more funds than sender owns", func() {
			BeforeEach(func() {
				subaccountIDFrom = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
				subaccountIDTo = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				app.ExchangeKeeper.SetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(51))
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := types.ErrInsufficientDeposit.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDFrom), baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(50)))

				depositTo := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(0)))
			})
		})
	})

	Describe("SubaccountTransfer with simplified ids", func() {
		var (
			err              error
			subaccountIDFrom string
			subaccountIDTo   string
			amountToTransfer sdk.Coin
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountIDFrom := "1"
			subaccountIDFromHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDFrom)
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountIDFromHash.Hex(),
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
		})

		JustBeforeEach(func() {
			subaccountTransfer := &types.MsgSubaccountTransfer{
				Sender:                  sender.String(),
				SourceSubaccountId:      subaccountIDFrom,
				DestinationSubaccountId: subaccountIDTo,
				Amount:                  amountToTransfer,
			}

			_, err = msgServer.SubaccountTransfer(sdk.WrapSDKContext(ctx), subaccountTransfer)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				subaccountIDFrom = "1"
				subaccountIDTo = "2"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				subaccountIDToHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDTo)
				app.ExchangeKeeper.SetDeposit(ctx, subaccountIDToHash, baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				subaccountIDFromHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDFrom)
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, subaccountIDFromHash, baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(0)))

				subaccountIDToHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDTo)
				depositTo := app.ExchangeKeeper.GetDeposit(ctx, subaccountIDToHash, baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(50)))
			})
		})
	})

	Describe("SubaccountTransfer", func() {
		var (
			err              error
			subaccountIDFrom string
			subaccountIDTo   string
			amountToTransfer sdk.Coin
		)

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountIDFrom := "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountIDFrom,
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
		})

		JustBeforeEach(func() {
			subaccountTransfer := &types.MsgSubaccountTransfer{
				Sender:                  sender.String(),
				SourceSubaccountId:      subaccountIDFrom,
				DestinationSubaccountId: subaccountIDTo,
				Amount:                  amountToTransfer,
			}

			_, err = msgServer.SubaccountTransfer(sdk.WrapSDKContext(ctx), subaccountTransfer)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				subaccountIDFrom = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
				subaccountIDTo = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				app.ExchangeKeeper.SetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDFrom), baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(0)))

				depositTo := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(50)))
			})
		})
	})

	Describe("ExternalTransfer", func() {
		var (
			err              error
			subaccountIDFrom string
			subaccountIDTo   string
			amountToTransfer sdk.Coin
		)
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountIDFrom := "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: subaccountIDFrom,
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
		})

		JustBeforeEach(func() {
			externalTransfer := &types.MsgExternalTransfer{
				Sender:                  sender.String(),
				SourceSubaccountId:      subaccountIDFrom,
				DestinationSubaccountId: subaccountIDTo,
				Amount:                  amountToTransfer,
			}

			_, err = msgServer.ExternalTransfer(sdk.WrapSDKContext(ctx), externalTransfer)
		})

		Context("With all valid fields to existing subaccount", func() {
			BeforeEach(func() {
				subaccountIDFrom = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
				subaccountIDTo = "0x90f8bf6a479f320ead074411a4b1e7944ea8c9c1000000000000000000000002"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				app.ExchangeKeeper.SetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDFrom), baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(0)))

				depositTo := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(50)))
			})
		})

		Context("With more funds than sender owns", func() {
			BeforeEach(func() {
				subaccountIDFrom = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
				subaccountIDTo = "0x90f8bf6a479f320ead074411a4b1e7944ea8c9c1000000000000000000000002"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				app.ExchangeKeeper.SetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(51))
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := types.ErrInsufficientDeposit.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDFrom), baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(50)))

				depositTo := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(0)))
			})
		})

		Context("With all valid fields to a non-existent subaccount", func() {
			BeforeEach(func() {
				subaccountIDFrom = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
				subaccountIDTo = "2968698C6b9Ed6D44b667a0b1F312a3b5D94Ded7000000000000000000000001"
				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDFrom), baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(0)))

				depositTo := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountIDTo), baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(50)))
			})
		})
	})

	Describe("ExternalTransfer simplified subaccount id", func() {
		var (
			err              error
			subaccountIDFrom string
			subaccountIDTo   string
			amountToTransfer sdk.Coin
		)
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))

		BeforeEach(func() {
			amountToMint := &sdk.Coin{
				Denom:  baseDenom,
				Amount: sdk.NewInt(50),
			}

			amount := sdk.Coins{*amountToMint}
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
			app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

			subaccountIDFrom := "1"
			amountToDeposit := sdk.NewCoin(baseDenom, sdk.NewInt(50))

			deposit := &types.MsgDeposit{
				Sender:       sender.String(),
				SubaccountId: types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDFrom).Hex(),
				Amount:       amountToDeposit,
			}

			_, err = msgServer.Deposit(sdk.WrapSDKContext(ctx), deposit)
		})

		JustBeforeEach(func() {
			externalTransfer := &types.MsgExternalTransfer{
				Sender:                  sender.String(),
				SourceSubaccountId:      subaccountIDFrom,
				DestinationSubaccountId: subaccountIDTo,
				Amount:                  amountToTransfer,
			}

			err = externalTransfer.ValidateBasic()
			Expect(err).To(BeNil())

			_, err = msgServer.ExternalTransfer(sdk.WrapSDKContext(ctx), externalTransfer)
		})

		Context("With all valid fields to external subaccount", func() {
			BeforeEach(func() {
				subaccountIDFrom = "1"
				subaccountIDTo = "0x2968698C6b9Ed6D44b667a0b1F312a3b5D94Ded7000000000000000000000001"

				zeroDeposit := &types.Deposit{
					AvailableBalance: sdk.NewDec(0),
					TotalBalance:     sdk.NewDec(0),
				}
				subaccountIDToHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDTo)
				app.ExchangeKeeper.SetDeposit(ctx, subaccountIDToHash, baseDenom, zeroDeposit)

				amountToTransfer = sdk.NewCoin(baseDenom, sdk.NewInt(50))
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				subaccountIDFromHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDFrom)
				depositFrom := app.ExchangeKeeper.GetDeposit(ctx, subaccountIDFromHash, baseDenom)

				Expect(depositFrom.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositFrom.TotalBalance).To(Equal(sdk.NewDec(0)))

				subaccountIDToHash := types.MustGetSubaccountIDOrDeriveFromNonce(sender, subaccountIDTo)
				depositTo := app.ExchangeKeeper.GetDeposit(ctx, subaccountIDToHash, baseDenom)

				Expect(depositTo.AvailableBalance).To(Equal(sdk.NewDec(50)))
				Expect(depositTo.TotalBalance).To(Equal(sdk.NewDec(50)))
			})
		})
	})

	Describe("RewardsOptOut", func() {
		var (
			err    error
			sender = sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		)

		BeforeEach(func() {
			registerMsg := &types.MsgRewardsOptOut{
				Sender: sender.String(),
			}
			_, err = msgServer.RewardsOptOut(sdk.WrapSDKContext(ctx), registerMsg)
		})

		It("Should be valid", func() {
			Expect(err).To(BeNil())
		})

		It("Should set store", func() {
			isRegisteredAsDMM := app.ExchangeKeeper.GetIsOptedOutOfRewards(ctx, sender)
			Expect(isRegisteredAsDMM).To(BeTrue())
		})

		Context("When the account is already opted out", func() {
			BeforeEach(func() {
				registerMsg := &types.MsgRewardsOptOut{
					Sender: sender.String(),
				}
				_, err = msgServer.RewardsOptOut(sdk.WrapSDKContext(ctx), registerMsg)
			})

			It("Revert the message", func() {
				Expect(err.Error()).To(Equal(exchangetypes.ErrAlreadyOptedOutOfRewards.Error()))
			})
		})
	})

	Describe("ReclaimLockedFunds", func() {
		var (
			coinsToUnlock sdk.Coins

			lockedPrivKey, _ = ethsecp256k1.GenerateKey()
			sdkLockedPrivKey = sdksecp256k1.PrivKey{
				Key: lockedPrivKey.Bytes(),
			}
			lockedPubKeyBz = lockedPrivKey.PubKey().Bytes()
			lockedPubKey   = sdksecp256k1.PubKey{
				Key: lockedPubKeyBz,
			}
			correctPubKey = ethsecp256k1.PubKey{
				Key: lockedPubKeyBz,
			}
			lockedSender     = sdk.AccAddress(lockedPubKey.Address())
			correctRecipient = sdk.AccAddress(correctPubKey.Address())

			otherLockedPrivKey, _ = ethsecp256k1.GenerateKey()
			otherLockedPubKeyBz   = otherLockedPrivKey.PubKey().Bytes()

			otherSenderLockedPubKey = sdksecp256k1.PubKey{
				Key: otherLockedPubKeyBz,
			}

			otherSenderSdkprivKey = sdksecp256k1.PrivKey{
				Key: otherLockedPrivKey.Bytes(),
			}

			// otherSenderLockedAddress = sdk.AccAddress(otherSenderLockedPubKey.Address())

			generateReclamationSignature = func(recipient, signer sdk.AccAddress, sdkPubKey sdksecp256k1.PubKey, sdkPrivKey sdksecp256k1.PrivKey) (signatureBytes []byte, err error) {
				data := types.ConstructFundsReclaimMessage(recipient, signer)

				msgSignData := types.MsgSignData{
					Signer: signer.Bytes(),
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
							PubKey:    &sdkPubKey,
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

				signature, err := sdkPrivKey.Sign(signBz)

				return signature, err
			}
		)

		Context("All valid, account doesn't exist", func() {
			BeforeEach(func() {
				coin := &sdk.Coin{
					Denom:  baseDenom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
			})

			It("Recieved funds in bank", func() {
				signature, err := generateReclamationSignature(correctRecipient, lockedSender, lockedPubKey, sdkLockedPrivKey)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(BeNil())

				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, baseDenom)
				Expect(bankBalance.Amount.String()).To(Equal(coinsToUnlock[0].Amount.String()), "incorrect amount deposited to bank")
			})
		})

		Context("All valid, account exists but made no transfer", func() {
			BeforeEach(func() {
				coin := &sdk.Coin{
					Denom:  baseDenom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
				app.AccountKeeper.NewAccountWithAddress(ctx, correctRecipient)
			})

			It("Recieved funds in bank", func() {
				signature, err := generateReclamationSignature(correctRecipient, lockedSender, lockedPubKey, sdkLockedPrivKey)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(BeNil())

				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, baseDenom)
				Expect(bankBalance.Amount.String()).To(Equal(coinsToUnlock[0].Amount.String()), "incorrect amount deposited to bank")
			})
		})

		Context("Invalid as account made a transfer", func() {
			BeforeEach(func() {
				coin := &sdk.Coin{
					Denom:  baseDenom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
				account := app.AccountKeeper.NewAccountWithAddress(ctx, lockedSender)
				account.SetSequence(1)
				app.AccountKeeper.SetAccount(ctx, account)
			})

			It("Doesn't send funds to bank", func() {
				signature, err := generateReclamationSignature(correctRecipient, lockedSender, lockedPubKey, sdkLockedPrivKey)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(Not(BeNil()))
				Expect(err).To(Equal(types.ErrInvalidAddress), "invalid error returned")

				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, baseDenom)
				Expect(bankBalance.Amount.String()).To(Equal(sdk.NewInt(0).String()), "amount deposited to bank")
			})
		})

		Context("Invalid only with peggy coins", func() {
			var (
				peggy1denom = "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7"
				peggy2denom = "peggy0xdAC17F958D2ee523a2206206994597C13D831ec8"
			)
			BeforeEach(func() {
				coin1 := &sdk.Coin{
					Denom:  peggy1denom,
					Amount: sdk.NewInt(50),
				}
				coin2 := &sdk.Coin{
					Denom:  peggy2denom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin1, *coin2}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
				app.AccountKeeper.NewAccountWithAddress(ctx, correctRecipient)
			})

			JustBeforeEach(func() {
				signature, err := generateReclamationSignature(correctRecipient, lockedSender, lockedPubKey, sdkLockedPrivKey)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(Not(BeNil()))
				Expect(err).To(Equal(types.ErrNoFundsToUnlock), "invalid error returned")
			})

			It("Doesn't send funds to bank", func() {
				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, peggy1denom)
				Expect(bankBalance.Amount.String()).To(Equal(sdk.NewInt(0).String()), "peggy1denom amount deposited to bank")

				bankBalance = app.BankKeeper.GetBalance(ctx, correctRecipient, peggy2denom)
				Expect(bankBalance.Amount.String()).To(Equal(sdk.NewInt(0).String()), "peggy2denom amount deposited to bank")
			})
		})

		Context("All valid with mixed coins", func() {
			var (
				peggy2denom = "peggy0xdAC17F958D2ee523a2206206994597C13D831ec8"
				coin1       *sdk.Coin
			)
			BeforeEach(func() {
				coin1 = &sdk.Coin{
					Denom:  baseDenom,
					Amount: sdk.NewInt(50),
				}

				coin2 := &sdk.Coin{
					Denom:  peggy2denom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin1, *coin2}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
				app.AccountKeeper.NewAccountWithAddress(ctx, correctRecipient)
			})

			It("Received valid funds in bank", func() {
				signature, err := generateReclamationSignature(correctRecipient, lockedSender, lockedPubKey, sdkLockedPrivKey)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(BeNil())

				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, baseDenom)
				Expect(bankBalance.Amount.String()).To(Equal(coinsToUnlock[0].Amount.String()), "incorrect amount deposited to bank")

				bankBalance = app.BankKeeper.GetBalance(ctx, correctRecipient, peggy2denom)
				Expect(bankBalance.Amount.String()).To(Equal(sdk.NewInt(0).String()), "peggy2denom amount deposited to bank")
			})
		})

		Context("Signed with a different key", func() {
			BeforeEach(func() {
				coin := &sdk.Coin{
					Denom:  baseDenom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
				app.AccountKeeper.NewAccountWithAddress(ctx, lockedSender)
			})

			It("Doesn't send funds to bank", func() {
				signature, err := generateReclamationSignature(correctRecipient, lockedSender, otherSenderLockedPubKey, otherSenderSdkprivKey)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring("signature verification failed with signature"), "invalid error returned")

				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, baseDenom)
				Expect(bankBalance.Amount.String()).To(Equal(sdk.NewInt(0).String()), "something was deposited to bank")
			})
		})

		Context("Signed different signature message key", func() {
			BeforeEach(func() {
				coin := &sdk.Coin{
					Denom:  baseDenom,
					Amount: sdk.NewInt(50),
				}

				coinsToUnlock = sdk.Coins{*coin}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coinsToUnlock)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, lockedSender, coinsToUnlock)
				app.AccountKeeper.NewAccountWithAddress(ctx, lockedSender)
			})

			It("Doesn't send funds to bank", func() {
				data := types.ConstructFundsReclaimMessage(correctRecipient, lockedSender)

				msgSignData := types.MsgSignData{
					Signer: lockedSender.Bytes(),
					Data:   []byte(data),
				}

				stdTx := legacytx.NewStdTx(
					[]sdk.Msg{&types.MsgSignDoc{
						SignType: "bla bla bla",
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

				signature, err := sdkLockedPrivKey.Sign(signBz)
				Expect(err).To(BeNil())

				msg := types.MsgReclaimLockedFunds{
					Sender:              lockedSender.String(),
					LockedAccountPubKey: lockedPubKeyBz,
					Signature:           signature,
				}

				_, err = msgServer.ReclaimLockedFunds(sdk.WrapSDKContext(ctx), &msg)
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring("signature verification failed with signature"), "invalid error returned")

				bankBalance := app.BankKeeper.GetBalance(ctx, correctRecipient, baseDenom)
				Expect(bankBalance.Amount.String()).To(Equal(sdk.NewInt(0).String()), "something was deposited to bank")
			})
		})

	})
})
