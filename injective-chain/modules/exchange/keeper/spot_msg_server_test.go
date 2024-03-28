package keeper_test

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Spot MsgServer Tests", func() {
	var (
		testInput  testexchange.TestInput
		app        *simapp.InjectiveApp
		msgServer  types.MsgServer
		ctx        sdk.Context
		spotMarket *types.SpotMarket
		baseDenom  string
		quoteDenom string
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 3, 0, 0)

		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)
		app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
		spotMarket = app.ExchangeKeeper.GetSpotMarket(ctx, testInput.Spots[0].MarketID, true)
		if spotMarket == nil {
			testexchange.OrFail(errors.New("GetSpotMarket returned nil"))
		}

		baseDenom = testInput.Spots[0].BaseDenom
		quoteDenom = testInput.Spots[0].QuoteDenom
	})

	Describe("InstantSpotMarketLaunch", func() {
		var (
			err          error
			message      *types.MsgInstantSpotMarketLaunch
			amountMinted *sdk.Coin
			amountNeeded math.Int
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			message = &types.MsgInstantSpotMarketLaunch{
				Sender:              sender.String(),
				Ticker:              testInput.Spots[1].Ticker,
				BaseDenom:           testInput.Spots[1].BaseDenom,
				QuoteDenom:          testInput.Spots[1].QuoteDenom,
				MinPriceTickSize:    testInput.Spots[1].MinPriceTickSize,
				MinQuantityTickSize: testInput.Spots[1].MinQuantityTickSize,
			}

			amountNeeded = math.Int(sdk.NewDec(1000))
		})

		JustBeforeEach(func() {
			_, err = msgServer.InstantSpotMarketLaunch(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount.Sub(amountNeeded)))
			})

			It("Should have funded community pool", func() {
				poolBalanceAfter := app.DistrKeeper.GetFeePool(ctx)

				Expect(len(poolBalanceAfter.CommunityPool)).NotTo(BeZero())

				for _, coin := range poolBalanceAfter.CommunityPool {
					Expect(coin.Denom).To(Equal("inj"))
					Expect(coin.Amount.String()).To(Equal(amountNeeded.ToDec().String()))
				}
			})
		})

		Context("With sender not having enough balance", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(500)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with insufficient funds error", func() {
				errorMessage := "spendable balance " + amountMinted.String() + " is smaller than " + amountNeeded.String() + "inj: "
				expectedError := errorMessage + sdkerrors.ErrInsufficientFunds.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount))
			})
		})

		Context("When baseDenom is invalid", func() {
			BeforeEach(func() {
				message.BaseDenom = "SMTH"

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with invalid base denom error", func() {
				errorMessage := "denom " + message.BaseDenom + " does not exist in supply: "
				expectedError := errorMessage + types.ErrInvalidBaseDenom.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount))
			})
		})

		Context("When quoteDenom is invalid", func() {
			BeforeEach(func() {
				message.QuoteDenom = "SMTH"

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with invalid quote denom error", func() {
				errorMessage := "denom " + message.QuoteDenom + " does not exist in supply: "
				expectedError := errorMessage + types.ErrInvalidQuoteDenom.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should not have updated balances", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount))
			})
		})

		Context("When spot market already exists", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				msgServer.InstantSpotMarketLaunch(sdk.WrapSDKContext(ctx), message)
			})

			It("Should be invalid with spot market exists error", func() {
				errorMessagePart1 := "ticker " + message.Ticker + " baseDenom " + message.BaseDenom
				errorMessagePart2 := " quoteDenom " + message.QuoteDenom + ": "
				expectedError := errorMessagePart1 + errorMessagePart2 + types.ErrSpotMarketExists.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have updated balances for one launch", func() {
				balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

				Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount.Sub(amountNeeded)))
			})

			Context("With different ticker", func() {
				BeforeEach(func() {
					message.Ticker = "SMTH/SMTH"
				})

				It("Should be invalid with spot market exists error", func() {
					errorMessagePart1 := "ticker " + message.Ticker + " baseDenom " + message.BaseDenom
					errorMessagePart2 := " quoteDenom " + message.QuoteDenom + ": "
					expectedError := errorMessagePart1 + errorMessagePart2 + types.ErrSpotMarketExists.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have updated balances for one launch", func() {
					balanceAfter := app.BankKeeper.GetBalance(ctx, sender, "inj")

					Expect(balanceAfter.Amount).To(Equal(amountMinted.Amount.Sub(amountNeeded)))
				})
			})
		})

		Context("Prevent conditions", func() {
			It("Prevent launch when an equivalent market launch proposal already exists", func() {
				message.Ticker = testInput.Spots[2].Ticker
				message.BaseDenom = testInput.Spots[2].BaseDenom
				message.QuoteDenom = testInput.Spots[2].QuoteDenom

				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
				content := types.NewSpotMarketLaunchProposal(
					"spot market launch proposal",
					"spot market launch proposal for specific market",
					message.Ticker,
					message.BaseDenom,
					message.QuoteDenom,
					message.MinPriceTickSize,
					message.MinQuantityTickSize,
					nil,
					nil,
				)
				proposalMsg, err := govtypesv1.NewLegacyContent(content, app.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String())
				Expect(err).To(BeNil())
				proposal, err := app.GovKeeper.SubmitProposal(ctx, []sdk.Msg{proposalMsg}, "", content.Title, content.Description, sender)
				Expect(err).To(BeNil())

				app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

				_, err = msgServer.InstantSpotMarketLaunch(sdk.WrapSDKContext(ctx), message)
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("CreateSpotLimitOrder buy orders", func() {
		var (
			err           error
			subaccountID  string
			message       *types.MsgCreateSpotLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(51),
				TotalBalance:     sdk.NewDec(51),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.Order.OrderInfo.SubaccountId = ""
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.Order.OrderInfo.SubaccountId = simpleSubaccountId
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.FeeRecipient = ""
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have deposited relayer fee share back to sender", func() {
				counterPartyAddress := testexchange.SampleAccountAddr2
				counterPartySubaccountId := testexchange.SampleDefaultSubaccountAddr2.String()
				testexchange.MintAndDeposit(app, ctx, counterPartySubaccountId, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))

				sellOrder := &types.MsgCreateSpotLimitOrder{
					Sender: counterPartyAddress.String(),
					Order: types.SpotOrder{
						MarketId: spotMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: counterPartySubaccountId,
							FeeRecipient: testexchange.SampleAccountAddr3.String(),
							Price:        sdk.NewDec(2),
							Quantity:     sdk.NewDec(25),
						},
						OrderType:    types.OrderType_SELL,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), sellOrder)
				testexchange.OrFail(err)
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)
				if !testexchange.IsUsingDefaultSubaccount() {
					defaultSubaccount := types.MustSdkAddressWithNonceToSubaccountID(types.SubaccountIDToSdkAddress(common.HexToHash(subaccountID)), 0)
					depositDefaultAfter := testexchange.GetBankAndDepositFunds(app, ctx, defaultSubaccount, quoteDenom)
					depositAfter.AvailableBalance = depositAfter.AvailableBalance.Add(depositDefaultAfter.AvailableBalance)
					depositAfter.TotalBalance = depositAfter.TotalBalance.Add(depositDefaultAfter.TotalBalance)
				}

				expectedFeeReturn := spotMarket.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity).Mul(spotMarket.RelayerFeeShareRate)
				expectedChange := balanceNeeded.Sub(expectedFeeReturn)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(expectedChange).String()), "incorrect available balance")
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.Sub(expectedChange).String()), "incorrect total balance")
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(3)
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), quoteDenom, deposit.AvailableBalance, balanceNeeded)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with spot market not found error", func() {
				errorMessage := "active spot market doesn't exist " + message.Order.MarketId + ": "
				expectedError := errorMessage + types.ErrSpotMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		// TODO - Somehow add more decimals that allowed
	})

	Describe("CreateSpotLimitOrder buy orders with PostOnly mode", func() {
		var (
			err                       error
			subaccountID              string
			counterparty_subaccountID string
			message                   *types.MsgCreateSpotLimitOrder
			counterpartyOrders        []*types.MsgCreateSpotLimitOrder
			balanceNeeded             sdk.Dec
			deposit                   *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1
		counterparty := testexchange.SampleAccountAddr2

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			counterparty_subaccountID = testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(200),
				TotalBalance:     sdk.NewDec(200),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, counterparty_subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))

			exchangeParams := app.ExchangeKeeper.GetParams(ctx)
			exchangeParams.PostOnlyModeHeightThreshold = ctx.BlockHeight() + 2000
			app.ExchangeKeeper.SetParams(ctx, exchangeParams)

			message = &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}
			counterpartyOrders = createCounterpartyOrders(
				counterparty.String(),
				counterparty_subaccountID,
				spotMarket.MarketId)
		})

		sendOrders := func() {
			for _, counterpartyOrder := range counterpartyOrders {
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), counterpartyOrder)
			}
			//ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			//Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
		}

		Context("With all valid fields", func() {
			BeforeEach(sendOrders)
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("Crossing Top of the Book", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(5)
				sendOrders()
			})
			It("Should fail with post-only error", func() {
				expectedError := "Post-only order exceeds top of book price"
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("PO order with all valid fields", func() {
			BeforeEach(func() {
				message.Order.OrderType = types.OrderType_BUY_PO
				sendOrders()
				balanceNeeded = (sdk.NewDec(1).Add(spotMarket.MakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("PO order crossing Top of the Book", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(5)
				message.Order.OrderType = types.OrderType_BUY_PO
				sendOrders()
			})
			It("Should fail with post-only error", func() {
				expectedError := "Post-only order exceeds top of book price"
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateSpotLimitOrder sell orders", func() {
		var (
			err           error
			subaccountID  string
			message       *types.MsgCreateSpotLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(50),
				TotalBalance:     sdk.NewDec(50),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(50),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = message.Order.OrderInfo.Quantity
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDec(51)
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), baseDenom, deposit.AvailableBalance, balanceNeeded)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateSpotLimitOrder sell orders with PostOnly mode", func() {
		var (
			err                       error
			subaccountID              string
			counterparty_subaccountID string
			message                   *types.MsgCreateSpotLimitOrder
			counterpartyOrders        []*types.MsgCreateSpotLimitOrder
			balanceNeeded             sdk.Dec
			deposit                   *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1
		counterparty := testexchange.SampleAccountAddr2

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			counterparty_subaccountID = testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(200),
				TotalBalance:     sdk.NewDec(200),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, counterparty_subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(testInput.Spots[0].BaseDenom, deposit.AvailableBalance.TruncateInt())))

			exchangeParams := app.ExchangeKeeper.GetParams(ctx)
			exchangeParams.PostOnlyModeHeightThreshold = ctx.BlockHeight() + 2000
			app.ExchangeKeeper.SetParams(ctx, exchangeParams)

			message = &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}
			counterpartyOrders = createCounterpartyOrders(
				counterparty.String(),
				counterparty_subaccountID,
				spotMarket.MarketId)
		})

		sendOrders := func() {
			for _, counterpartyOrder := range counterpartyOrders {
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), counterpartyOrder)
			}
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = message.Order.OrderInfo.Quantity
		}

		Context("With all valid fields", func() {
			BeforeEach(sendOrders)
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("Crossing Top of the Book", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(2)
				sendOrders()
			})
			It("Should fail with post-only error", func() {
				expectedError := "Post-only order exceeds top of book price"
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("PO order with all valid fields", func() {
			BeforeEach(func() {
				message.Order.OrderType = types.OrderType_SELL_PO
				sendOrders()
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("PO order crossing Top of the Book", func() {
			BeforeEach(func() {
				message.Order.OrderType = types.OrderType_SELL_PO
				message.Order.OrderInfo.Price = sdk.NewDec(2)
				sendOrders()
			})
			It("Should fail with post-only error", func() {
				expectedError := "Post-only order exceeds top of book price"
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateSpotMarketOrder buy orders", func() {
		var (
			err                   error
			subaccountID          string
			message               *types.MsgCreateSpotMarketOrder
			spotLimitOrderMessage *types.MsgCreateSpotLimitOrder
			balanceNeeded         sdk.Dec
			deposit               *types.Deposit
		)
		BeforeEach(func() {
			seller := testexchange.SampleAccountAddr2
			subaccountIDLimit := testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(50),
				TotalBalance:     sdk.NewDec(50),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDLimit, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt())))

			spotLimitOrderMessage = &types.MsgCreateSpotLimitOrder{
				Sender: seller.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIDLimit,
						FeeRecipient: testexchange.SampleAccountAddrStr3,
						Price:        sdk.NewDec(1),
						Quantity:     sdk.NewDec(50),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			buyer := testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(51),
				TotalBalance:     sdk.NewDec(51),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotMarketOrder{
				Sender: buyer.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.Order.OrderInfo.SubaccountId = ""
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.Order.OrderInfo.SubaccountId = simpleSubaccountId
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty fee recipient", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.FeeRecipient = ""
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have deposited relayer fee share back to sender", func() {
				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				if !testexchange.IsUsingDefaultSubaccount() {
					defaultSubaccount := types.MustSdkAddressWithNonceToSubaccountID(types.SubaccountIDToSdkAddress(common.HexToHash(subaccountID)), 0)
					depositDefaultAfter := testexchange.GetBankAndDepositFunds(app, ctx, defaultSubaccount, quoteDenom)
					depositAfter.AvailableBalance = depositAfter.AvailableBalance.Add(depositDefaultAfter.AvailableBalance)
					depositAfter.TotalBalance = depositAfter.TotalBalance.Add(depositDefaultAfter.TotalBalance)
				}

				margin := spotLimitOrderMessage.Order.OrderInfo.Price.Mul(message.Order.OrderInfo.Quantity)
				fee := margin.Mul(spotMarket.TakerFeeRate).Mul(sdk.NewDec(1).Sub(spotMarket.RelayerFeeShareRate))
				balanceNeeded = margin.Add(fee)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).String()), "incorrect available balance")
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.Sub(balanceNeeded).String()), "incorrect total balance")
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(3)
			})

			It("Should be invalid with insufficient deposit error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), quoteDenom, deposit.AvailableBalance, balanceNeeded)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with spot market not found error", func() {
				errorMessage := "active spot market doesn't exist " + message.Order.MarketId + ": "
				expectedError := errorMessage + types.ErrSpotMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateSpotMarketOrder buy orders with PostOnly mode", func() {
		var (
			err                   error
			subaccountID          string
			message               *types.MsgCreateSpotMarketOrder
			spotLimitOrderMessage *types.MsgCreateSpotLimitOrder
			deposit_limit         *types.Deposit
			deposit_market        *types.Deposit
		)
		BeforeEach(func() {
			seller := testexchange.SampleAccountAddr2
			subaccountIDLimit := testexchange.SampleSubaccountAddr2.String()

			deposit_limit = &types.Deposit{
				AvailableBalance: sdk.NewDec(50),
				TotalBalance:     sdk.NewDec(50),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDLimit, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit_limit.AvailableBalance.TruncateInt())))

			exchangeParams := app.ExchangeKeeper.GetParams(ctx)
			exchangeParams.PostOnlyModeHeightThreshold = ctx.BlockHeight() + 2000
			app.ExchangeKeeper.SetParams(ctx, exchangeParams)

			spotLimitOrderMessage = &types.MsgCreateSpotLimitOrder{
				Sender: seller.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIDLimit,
						FeeRecipient: testexchange.SampleAccountAddrStr3,
						Price:        sdk.NewDec(1),
						Quantity:     sdk.NewDec(50),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			buyer := testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit_market = &types.Deposit{
				AvailableBalance: sdk.NewDec(51),
				TotalBalance:     sdk.NewDec(51),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit_market.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotMarketOrder{
				Sender: buyer.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should fail with post-only error", func() {
				expectedError := fmt.Sprintf("cannot create market orders in post only mode until height %d: exchange is in post-only mode", ctx.BlockHeight()+2000-1)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit_market.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit_market.TotalBalance))
			})
		})
	})

	Describe("CreateSpotMarketOrder sell orders", func() {
		var (
			err           error
			subaccountID  string
			message       *types.MsgCreateSpotMarketOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)

		BeforeEach(func() {
			seller := testexchange.SampleAccountAddr2
			subaccountIDLimit := testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(121),
				TotalBalance:     sdk.NewDec(121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDLimit, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			spotLimitOrderMessage := &types.MsgCreateSpotLimitOrder{
				Sender: seller.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIDLimit,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(40),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			buyer := testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(30),
				TotalBalance:     sdk.NewDec(30),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotMarketOrder{
				Sender: buyer.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(1),
						Quantity:     sdk.NewDec(30),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)
				_ = balanceNeeded

				Expect(depositAfter.AvailableBalance).To(Equal(sdk.NewDec(0)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDec(31)
			})

			It("Should be invalid with insufficient deposit error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), baseDenom, deposit.AvailableBalance, message.Order.OrderInfo.Quantity)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with spot market not found error", func() {
				errorMessage := "active spot market doesn't exist " + message.Order.MarketId + ": "
				expectedError := errorMessage + types.ErrSpotMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CreateSpotMarketOrder sell orders with PostOnly mode", func() {
		var (
			err            error
			subaccountID   string
			message        *types.MsgCreateSpotMarketOrder
			deposit_limit  *types.Deposit
			deposit_market *types.Deposit
		)

		BeforeEach(func() {
			seller := testexchange.SampleAccountAddr2
			subaccountIDLimit := testexchange.SampleSubaccountAddr1.String()

			deposit_limit = &types.Deposit{
				AvailableBalance: sdk.NewDec(121),
				TotalBalance:     sdk.NewDec(121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDLimit, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit_limit.AvailableBalance.TruncateInt())))

			exchangeParams := app.ExchangeKeeper.GetParams(ctx)
			exchangeParams.PostOnlyModeHeightThreshold = ctx.BlockHeight() + 2000
			app.ExchangeKeeper.SetParams(ctx, exchangeParams)

			spotLimitOrderMessage := &types.MsgCreateSpotLimitOrder{
				Sender: seller.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIDLimit,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(40),
					},
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)

			buyer := testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit_market = &types.Deposit{
				AvailableBalance: sdk.NewDec(30),
				TotalBalance:     sdk.NewDec(30),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit_market.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotMarketOrder{
				Sender: buyer.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(1),
						Quantity:     sdk.NewDec(30),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotMarketOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should fail with post-only error", func() {
				expectedError := fmt.Sprintf("cannot create market orders in post only mode until height %d: exchange is in post-only mode", ctx.BlockHeight()+2000)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit_market.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit_market.TotalBalance))
			})
		})
	})

	Describe("CancelSpotOrder", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCancelSpotOrder
			balanceUsed  sdk.Dec
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(121),
				TotalBalance:     sdk.NewDec(121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			spotLimitOrderMessage := &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(40),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)
			balanceUsed = (sdk.NewDec(1).Add(spotMarket.MakerFeeRate)).Mul(spotLimitOrderMessage.Order.OrderInfo.Price).Mul(spotLimitOrderMessage.Order.OrderInfo.Quantity)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			limitOrders, _, _ := app.ExchangeKeeper.GetFillableSpotLimitOrdersByMarketDirection(
				ctx,
				testInput.Spots[0].MarketID,
				true,
				sdk.NewDec(40),
			)

			restingOrder := limitOrders[0]

			message = &types.MsgCancelSpotOrder{
				Sender:       sender.String(),
				MarketId:     spotMarket.MarketId,
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With empty subaccount id", func() {
			BeforeEach(func() {
				if !testexchange.IsUsingDefaultSubaccount() {
					Skip("only makes sense with default subaccount")
				}
				message.SubaccountId = ""
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("With simplified subaccount id", func() {
			BeforeEach(func() {
				simpleSubaccountId := "1"
				if testexchange.IsUsingDefaultSubaccount() {
					simpleSubaccountId = "0"
				}
				message.SubaccountId = simpleSubaccountId
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.MarketId = common.HexToHash("0x9").Hex()
			})

			It("Should be invalid with spot market not found error", func() {
				errorMessage := "active spot market doesn't exist " + message.MarketId + ": "
				expectedError := errorMessage + types.ErrSpotMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When order does not exist", func() {
			BeforeEach(func() {
				anotherSubaccount, _ := types.SdkAddressWithNonceToSubaccountID(sender, 2)
				message.SubaccountId = anotherSubaccount.String()
			})

			It("Should be invalid with order does not exist error", func() {
				expectedError := "Spot Limit Order is nil: " + types.ErrOrderDoesntExist.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CancelSpotOrder for transient buy orders", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCancelSpotOrder
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(121),
				TotalBalance:     sdk.NewDec(121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			spotLimitOrderMessage := &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(40),
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersByMarketDirection(
				ctx,
				testInput.Spots[0].MarketID,
				true,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelSpotOrder{
				Sender:       sender.String(),
				MarketId:     spotMarket.MarketId,
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), message)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersByMarketDirection(
					ctx,
					testInput.Spots[0].MarketID,
					true,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(
					ctx,
					testInput.Spots[0].MarketID,
					true,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelSpotOrder for transient sell orders", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCancelSpotOrder
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(121),
				TotalBalance:     sdk.NewDec(121),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt())))

			spotLimitOrderMessage := &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(3),
						Quantity:     sdk.NewDec(40),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}

			msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), spotLimitOrderMessage)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersByMarketDirection(
				ctx,
				testInput.Spots[0].MarketID,
				false,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelSpotOrder{
				Sender:       sender.String(),
				MarketId:     spotMarket.MarketId,
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), message)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), baseDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientSpotLimitOrdersByMarketDirection(
					ctx,
					testInput.Spots[0].MarketID,
					false,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllSpotLimitOrdersByMarketDirection(
					ctx,
					testInput.Spots[0].MarketID,
					false,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("Create orders with client order id", func() {
		var (
			err           error
			subaccountID  string
			message       *types.MsgCreateSpotLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(100),
				TotalBalance:     sdk.NewDec(100),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.Spots[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
						Cid:          "my_great_order_1",
					},
					OrderType:    types.OrderType_BUY,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("Using cid to cancel", func() {
			It("Should be cancellable", func() {
				message := &types.MsgCancelSpotOrder{
					Sender:       sender.String(),
					MarketId:     spotMarket.MarketId,
					SubaccountId: subaccountID,
					OrderHash:    "",
					Cid:          "my_great_order_1",
				}
				_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), message)
				Expect(err).To(BeNil())

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

		})

		Context("Creating another order with the same cid", func() {
			It("Should fail", func() {
				message = &types.MsgCreateSpotLimitOrder{
					Sender: sender.String(),
					Order: types.SpotOrder{
						MarketId: spotMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2),
							Quantity:     sdk.NewDec(5),
							Cid:          "my_great_order_1",
						},
						OrderType:    types.OrderType_BUY,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
				Expect(err.Error()).To(Equal(types.ErrClientOrderIdAlreadyExists.Error()))

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)
				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

		})

		Context("Creating another order with different cid", func() {
			It("Should be valid", func() {
				message = &types.MsgCreateSpotLimitOrder{
					Sender: sender.String(),
					Order: types.SpotOrder{
						MarketId: spotMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2),
							Quantity:     sdk.NewDec(1),
							Cid:          "my_great_order_2",
						},
						OrderType:    types.OrderType_BUY,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
				var balanceNeededForSecondOrder = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)

				Expect(err).To(BeNil())

				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)
				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).Sub(balanceNeededForSecondOrder)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

		})

		Context("If cancelled, cid should be reusable", func() {
			JustBeforeEach(func() {
				var cancelMessage = &types.MsgCancelSpotOrder{
					Sender:       sender.String(),
					MarketId:     spotMarket.MarketId,
					SubaccountId: subaccountID,
					OrderHash:    "",
					Cid:          "my_great_order_1",
				}
				_, err = msgServer.CancelSpotOrder(sdk.WrapSDKContext(ctx), cancelMessage)

				var createMessage = &types.MsgCreateSpotLimitOrder{
					Sender: sender.String(),
					Order: types.SpotOrder{
						MarketId: spotMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2),
							Quantity:     sdk.NewDec(5),
							Cid:          "my_great_order_1",
						},
						OrderType:    types.OrderType_BUY,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), createMessage)
				balanceNeeded = (sdk.NewDec(1).Add(spotMarket.TakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(createMessage.Order.OrderInfo.Quantity)
			})
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances based only on last order", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})

		})

	})
})

func createCounterpartyOrders(
	counterparty_address string,
	counterparty_subaccountID string,
	marketId string,
) (counterpartyOrders []*types.MsgCreateSpotLimitOrder) {
	counterpartyOrders = append(counterpartyOrders,
		&types.MsgCreateSpotLimitOrder{
			Sender: counterparty_address,
			Order: types.SpotOrder{
				MarketId: marketId,
				OrderInfo: types.OrderInfo{
					SubaccountId: counterparty_subaccountID,
					FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					Price:        sdk.NewDec(2),
					Quantity:     sdk.NewDec(3),
				},
				OrderType:    types.OrderType_BUY_PO,
				TriggerPrice: nil,
			},
		})
	counterpartyOrders = append(counterpartyOrders,
		&types.MsgCreateSpotLimitOrder{
			Sender: counterparty_address,
			Order: types.SpotOrder{
				MarketId: marketId,
				OrderInfo: types.OrderInfo{
					SubaccountId: counterparty_subaccountID,
					FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					Price:        sdk.NewDec(4),
					Quantity:     sdk.NewDec(3),
				},
				OrderType:    types.OrderType_SELL_PO,
				TriggerPrice: nil,
			},
		})
	return counterpartyOrders
}
