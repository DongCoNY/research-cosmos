package keeper_test

import (
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

		_ = baseDenom
	})

	Describe("CreateSpotLimitOrder buy PO orders", func() {
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

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

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
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), message)
			balanceNeeded = (sdk.NewDec(1).Add(spotMarket.MakerFeeRate)).Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), quoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceNeeded).String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})
		})

		Context("With more deposits needed than existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDec(3)
			})

			It("Should be invalid with insufficient funds error", func() {
				expectedError := testexchange.GetInsufficientFundsErrorMessage(common.HexToHash(subaccountID), quoteDenom, deposit.AvailableBalance, balanceNeeded)
				Expect(err.Error()).Should(Equal(expectedError))
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

		Context("When order crosses order book", func() {
			BeforeEach(func() {
				oppositeSideOrderMsg := &types.MsgCreateSpotLimitOrder{
					Sender: sender.String(),
					Order: types.SpotOrder{
						MarketId: spotMarket.MarketId,
						OrderInfo: types.OrderInfo{
							SubaccountId: subaccountID,
							FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
							Price:        sdk.NewDec(2),
							Quantity:     sdk.NewDec(25),
						},
						OrderType:    types.OrderType_SELL,
						TriggerPrice: nil,
					},
				}
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), oppositeSideOrderMsg)
				testexchange.OrFail(err)
				testexchange.EndBlockerAndCommit(app, ctx)
			})

			It("Should throw order crosses book error", func() {
				Expect(err.Error()).To(Equal(types.ErrExceedsTopOfBookPrice.Error()))
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
				AvailableBalance: sdk.NewDec(150),
				TotalBalance:     sdk.NewDec(150),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			message = &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(150),
					},
					OrderType:    types.OrderType_SELL_PO,
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
				message.Order.OrderInfo.Quantity = sdk.NewDec(151)
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

		Context("When order crosses order book", func() {
			BeforeEach(func() {
				oppositeSideOrderMsg := &types.MsgCreateSpotLimitOrder{
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
				_, err = msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), oppositeSideOrderMsg)
				testexchange.OrFail(err)
				testexchange.EndBlockerAndCommit(app, ctx)
			})

			It("Should throw order crosses book error", func() {
				Expect(err.Error()).To(Equal(types.ErrExceedsTopOfBookPrice.Error()))
			})
		})
	})

	Describe("CreateSpotMarketOrder buy orders", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCreateSpotMarketOrder
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountIDLimit := testexchange.SampleSubaccountAddr2.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(50),
				TotalBalance:     sdk.NewDec(50),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDLimit, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))

			spotLimitOrderMessage := &types.MsgCreateSpotLimitOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountIDLimit,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
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

			subaccountID = testexchange.SampleSubaccountAddr1.String()

			deposit = &types.Deposit{
				AvailableBalance: sdk.NewDec(51),
				TotalBalance:     sdk.NewDec(51),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(baseDenom, deposit.AvailableBalance.TruncateInt()), sdk.NewCoin(quoteDenom, deposit.AvailableBalance.TruncateInt())))
		})

		It("Should be reverted for buys", func() {
			message = &types.MsgCreateSpotMarketOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_BUY_PO,
					TriggerPrice: nil,
				},
			}
			err = message.ValidateBasic()

			errorMessage := "Spot market order can't be a post only order: "
			expectedError := errorMessage + types.ErrInvalidOrderTypeForMessage.Error()
			Expect(err.Error()).To(Equal(expectedError))
		})

		It("Should be reverted for sells", func() {
			message = &types.MsgCreateSpotMarketOrder{
				Sender: sender.String(),
				Order: types.SpotOrder{
					MarketId: spotMarket.MarketId,
					OrderInfo: types.OrderInfo{
						SubaccountId: subaccountID,
						FeeRecipient: "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						Price:        sdk.NewDec(2),
						Quantity:     sdk.NewDec(25),
					},
					OrderType:    types.OrderType_SELL_PO,
					TriggerPrice: nil,
				},
			}
			err = message.ValidateBasic()

			errorMessage := "Spot market order can't be a post only order: "
			expectedError := errorMessage + types.ErrInvalidOrderTypeForMessage.Error()
			Expect(err.Error()).To(Equal(expectedError))
		})
	})
})
