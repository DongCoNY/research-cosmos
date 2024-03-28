package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ = Describe("Derivative MsgServer Tests", func() {
	var (
		testInput        testexchange.TestInput
		app              *simapp.InjectiveApp
		msgServer        types.MsgServer
		ctx              sdk.Context
		quoteDenom       string
		derivativeMarket *types.DerivativeMarket
	)

	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 1, 0)

		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
		startingPrice := sdk.NewDec(2000)

		app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

		sender := sdk.AccAddress(common.FromHex("727aee334987c52fa7b567b2662bdbb68614e48c"))
		coin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
		testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, sender, coin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

		var err error
		derivativeMarket, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
			ctx,
			testInput.Perps[0].Ticker,
			testInput.Perps[0].QuoteDenom,
			oracleBase,
			oracleQuote,
			0,
			oracleType,
			testInput.Perps[0].InitialMarginRatio,
			testInput.Perps[0].MaintenanceMarginRatio,
			testInput.Perps[0].MakerFeeRate,
			testInput.Perps[0].TakerFeeRate,
			testInput.Perps[0].MinPriceTickSize,
			testInput.Perps[0].MinQuantityTickSize,
		)
		testexchange.OrFail(err)

		derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, testInput.Perps[0].MarketID, true)
		if derivativeMarket == nil {
			testexchange.OrFail(errors.New("GetDerivativeMarket returned nil"))
		}

		quoteDenom = testInput.Perps[0].QuoteDenom
	})

	Describe("CreateDerivativeLimitOrder buy PO orders", func() {
		var (
			err           error
			message       *types.MsgCreateDerivativeLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
			senderAcc     = testexchange.SampleAccountAddrStr1
			subaccountID  = testexchange.SampleSubaccountAddr1
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(4012),
				TotalBalance:     sdk.NewDec(4012),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID.String(), sdk.NewCoins(sdk.NewCoin(quoteDenom, sdk.NewInt(4012))))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: senderAcc,
				Order: types.DerivativeOrder{
					Margin:   sdk.NewDec(4000),
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID.String(),
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = message.Order.Margin.Add(derivativeMarket.MakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity))
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID, quoteDenom)

				Expect(depositAfter.String()).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).String()))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(5000)
			})

			It("Should be invalid with insufficient funds error", func() {
				Expect(testexchange.IsExpectedInsufficientFundsErrorType(subaccountID, err)).To(BeTrue())
			})

			It("Should have not updated balances", func() {
				depositAfter := app.ExchangeKeeper.GetSpendableFunds(ctx, subaccountID, quoteDenom)
				Expect(depositAfter).To(Equal(deposit.AvailableBalance))
			})
		})
	})

	Describe("CreateDerivativeLimitOrder buy PO orders in negative maker fee market", func() {
		var (
			err           error
			subaccountID  = testexchange.SampleSubaccountAddr1.String()
			sender        = testexchange.SampleAccountAddr1
			message       *types.MsgCreateDerivativeLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)
		negativeMakerFee := sdk.NewDecWithPrec(-1, 3)

		BeforeEach(func() {
			err = app.ExchangeKeeper.ScheduleDerivativeMarketParamUpdate(ctx, &types.DerivativeMarketParamUpdateProposal{
				Title:                  "Update Derivative market param",
				Description:            "Update Derivative market description",
				MarketId:               derivativeMarket.MarketId,
				MakerFeeRate:           &negativeMakerFee,
				TakerFeeRate:           &derivativeMarket.TakerFeeRate,
				RelayerFeeShareRate:    &derivativeMarket.RelayerFeeShareRate,
				MinPriceTickSize:       &derivativeMarket.MinPriceTickSize,
				MinQuantityTickSize:    &derivativeMarket.MinQuantityTickSize,
				InitialMarginRatio:     &derivativeMarket.InitialMarginRatio,
				MaintenanceMarginRatio: &derivativeMarket.MaintenanceMarginRatio,
				Status:                 types.MarketStatus_Active,
			})
			testexchange.OrFail(err)

			proposals := make([]types.DerivativeMarketParamUpdateProposal, 0)
			app.ExchangeKeeper.IterateDerivativeMarketParamUpdates(ctx, func(p *types.DerivativeMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})

			err := app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &proposals[0])
			testexchange.OrFail(err)

			// need to update market instance or some methods will use old maker fee
			derivativeMarket = app.ExchangeKeeper.GetDerivativeMarket(ctx, derivativeMarket.MarketID(), true)

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(4012),
				TotalBalance:     sdk.NewDec(4012),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					Margin:   sdk.NewDec(4000),
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = message.Order.Margin
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := app.ExchangeKeeper.GetSpendableFunds(ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.String()).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).String()))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(5000)
			})

			It("Should be invalid with insufficient funds error", func() {
				Expect(testexchange.IsExpectedInsufficientFundsErrorType(common.HexToHash(subaccountID), err)).To(BeTrue())
			})

			It("Should have not updated balances", func() {
				depositAfter := app.ExchangeKeeper.GetSpendableFunds(ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter).To(Equal(deposit.AvailableBalance))
			})
		})
	})

	Describe("CreateDerivativeLimitOrder buy PO orders in negative maker fee market", func() {
		var (
			err           error
			subaccountID  = testexchange.SampleSubaccountAddr1.String()
			sender        = testexchange.SampleAccountAddr1
			message       *types.MsgCreateDerivativeLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)
		negativeMakerFee := sdk.NewDecWithPrec(-1, 3)

		BeforeEach(func() {
			err = app.ExchangeKeeper.ScheduleDerivativeMarketParamUpdate(ctx, &types.DerivativeMarketParamUpdateProposal{
				Title:                  "Update Derivative market param",
				Description:            "Update Derivative market description",
				MarketId:               derivativeMarket.MarketId,
				MakerFeeRate:           &negativeMakerFee,
				TakerFeeRate:           &derivativeMarket.TakerFeeRate,
				RelayerFeeShareRate:    &derivativeMarket.RelayerFeeShareRate,
				MinPriceTickSize:       &derivativeMarket.MinPriceTickSize,
				MinQuantityTickSize:    &derivativeMarket.MinQuantityTickSize,
				InitialMarginRatio:     &derivativeMarket.InitialMarginRatio,
				MaintenanceMarginRatio: &derivativeMarket.MaintenanceMarginRatio,
				Status:                 types.MarketStatus_Active,
			})
			testexchange.OrFail(err)

			proposals := make([]types.DerivativeMarketParamUpdateProposal, 0)
			app.ExchangeKeeper.IterateDerivativeMarketParamUpdates(ctx, func(p *types.DerivativeMarketParamUpdateProposal) (stop bool) {
				proposals = append(proposals, *p)
				return false
			})

			err := app.ExchangeKeeper.ExecuteDerivativeMarketParamUpdateProposal(ctx, &proposals[0])
			testexchange.OrFail(err)

			derivativeMarket = app.ExchangeKeeper.GetDerivativeMarketByID(ctx, derivativeMarket.MarketID())

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(4012),
				TotalBalance:     sdk.NewDec(4012),
			}
			// app.ExchangeKeeper.SetDeposit(ctx, common.HexToHash(subaccountID), quoteDenom, deposit)
			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					Margin:   sdk.NewDec(4000),
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = message.Order.Margin
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)
				// depositAfter := app.ExchangeKeeper.GetDeposit(ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(5000)
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(message.Order.SubaccountID(), quoteDenom, deposit.AvailableBalance, balanceNeeded)
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

			It("Should be invalid with Derivative market not found error", func() {
				errorMessage := "active derivative market for marketID " + message.Order.MarketId + " not found: "
				expectedError := errorMessage + types.ErrDerivativeMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When order crosses order book", func() {
			BeforeEach(func() {
				oppositeSideOrderMsg := &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						Margin:   sdk.NewDec(4000),
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), oppositeSideOrderMsg)
				testexchange.OrFail(err)
				testexchange.EndBlockerAndCommit(app, ctx)

				testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))
			})

			It("Should throw order crosses book error", func() {
				Expect(err.Error()).To(Equal(types.ErrExceedsTopOfBookPrice.Error()))
			})
		})

		Context("When cancelled order", func() {

			JustBeforeEach(func() {
				testexchange.EndBlockerAndCommit(app, ctx)
				err = app.ExchangeKeeper.CancelAllRestingDerivativeLimitOrdersForSubaccount(ctx, derivativeMarket, common.HexToHash(subaccountID), false, true)
				Expect(err).To(BeNil())
			})

			It("Deposit should not change", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})
		})

		Context("When PO order is matched with negative maker fees", func() {
			seller := testexchange.SampleAccountAddr2
			sellerSubaccountId := testexchange.SampleSubaccountAddr2.String()

			JustBeforeEach(func() {
				testexchange.EndBlockerAndCommit(app, ctx)

				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(4012),
					TotalBalance:     sdk.NewDec(4012),
				}
				testexchange.MintAndDeposit(app, ctx, sellerSubaccountId, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

				oppositeSideOrderMsg := &types.MsgCreateDerivativeLimitOrder{
					Sender: seller.String(),
					Order: types.DerivativeOrder{
						Margin:   sdk.NewDec(4000),
						MarketId: derivativeMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: sellerSubaccountId,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_SELL,
						TriggerPrice: nil,
					},
				}

				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), oppositeSideOrderMsg)
				testexchange.OrFail(err)

				testexchange.EndBlockerAndCommit(app, ctx)
			})

			It("Deposits should be updated", func() {
				sellerAvailableDeposit := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(sellerSubaccountId), quoteDenom)
				Expect(sellerAvailableDeposit.AvailableBalance.String()).To(Equal(sdk.ZeroDec().String()))

				expectedDeposit := sdk.NewDec(12).Sub(sdk.NewDec(4000).Mul(negativeMakerFee).Mul(sdk.NewDec(6).Quo(sdk.NewDec(10)))).String()
				depositBuyerAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositBuyerAfter.AvailableBalance.String()).To(Equal(expectedDeposit))
				Expect(depositBuyerAfter.TotalBalance.String()).To(Equal(expectedDeposit))
			})
		})
	})

	Describe("CreateDerivativeLimitOrder sell orders", func() {
		var (
			err           error
			subaccountID  = testexchange.SampleSubaccountAddr1.String()
			sender        = testexchange.SampleAccountAddr1
			message       *types.MsgCreateDerivativeLimitOrder
			balanceNeeded sdk.Dec
			deposit       *types.Deposit
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(4004),
				TotalBalance:     sdk.NewDec(4004),
			}
			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateDerivativeLimitOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					Margin:   sdk.NewDec(4000),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL_PO,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = message.Order.Margin.Add(derivativeMarket.MakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity))
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)
				Expect(depositAfter.AvailableBalance.String()).To(Equal(sdk.NewDec(0).String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.Margin = sdk.NewDec(5000)
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(message.Order.SubaccountID(), quoteDenom, deposit.AvailableBalance, balanceNeeded)
				Expect(err.Error()).To(Equal(expectedError))
			})

			It("Should have not updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When order crosses order book", func() {
			BeforeEach(func() {
				deposit = &types.Deposit{
					AvailableBalance: sdk.NewDec(4012),
					TotalBalance:     sdk.NewDec(4012),
				}
				testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

				oppositeSideOrderMsg := &types.MsgCreateDerivativeLimitOrder{
					Sender: sender.String(),
					Order: types.DerivativeOrder{
						MarketId: derivativeMarket.MarketId,
						Margin:   sdk.NewDec(4000),
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2000),
							Quantity:     sdk.NewDec(2),
						},
						OrderType:    types.OrderType_BUY,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), oppositeSideOrderMsg)
				testexchange.OrFail(err)
				testexchange.EndBlockerAndCommit(app, ctx)

				testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))
			})

			It("Should throw order crosses book error", func() {
				Expect(err.Error()).To(Equal(types.ErrExceedsTopOfBookPrice.Error()))
			})
		})
	})

	Describe("CreateDerivativeMarketOrder buy orders", func() {
		var (
			err          error
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			sender       = testexchange.SampleAccountAddr1
			message      *types.MsgCreateDerivativeMarketOrder
			deposit      *types.Deposit
		)

		BeforeEach(func() {
			senderLimit := testexchange.SampleAccountAddr2
			subaccountIDLimit := testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(6000),
				TotalBalance:     sdk.NewDec(6000),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDLimit, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			derivativeLimitOrderMessage := &types.MsgCreateDerivativeLimitOrder{
				Sender: senderLimit.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					Margin:   sdk.NewDec(5000),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIDLimit,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(50),
					},
					OrderType:    types.OrderType_SELL,
					TriggerPrice: nil,
				},
			}

			_, err2 := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), derivativeLimitOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(51),
				TotalBalance:     sdk.NewDec(51),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))
		})

		It("Should be reverted for buys", func() {
			message = &types.MsgCreateDerivativeMarketOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					Margin:   sdk.NewDec(4000),
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}
			err = message.ValidateBasic()

			errorMessage := "Derivative market order can't be a post only order: "
			expectedError := errorMessage + types.ErrInvalidOrderTypeForMessage.Error()
			Expect(err.Error()).To(Equal(expectedError))
		})

		It("Should be reverted for sells", func() {
			message = &types.MsgCreateDerivativeMarketOrder{
				Sender: sender.String(),
				Order: types.DerivativeOrder{
					MarketId: derivativeMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2000),
						Quantity:     sdk.NewDec(2),
					},
					OrderType:    types.OrderType_SELL_PO,
					TriggerPrice: nil,
				},
			}
			err = message.ValidateBasic()

			errorMessage := "Derivative market order can't be a post only order: "
			expectedError := errorMessage + types.ErrInvalidOrderTypeForMessage.Error()
			Expect(err.Error()).To(Equal(expectedError))
		})
	})
})
