package keeper_test

import (
	"fmt"
	"sort"
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

	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

var _ = Describe("Binary Options MsgServer Tests", func() {
	var (
		testInput testexchange.TestInput
		app       *simapp.InjectiveApp
		ctx       sdk.Context

		market    *types.BinaryOptionsMarket
		msgServer types.MsgServer
		err       error
		admin     string
	)

	BeforeEach(func() { // market setup
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		testInput, ctx = testexchange.SetupTest(app, ctx, 0, 0, 3)
		msgServer = keeper.NewMsgServerImpl(app.ExchangeKeeper)

		oracleSymbol, oracleProvider := testInput.BinaryMarkets[0].OracleSymbol, testInput.BinaryMarkets[0].OracleProvider
		admin = testInput.BinaryMarkets[0].Admin

		startingPrice := sdk.NewDec(2000)

		app.OracleKeeper.SetProviderInfo(ctx, &oracletypes.ProviderInfo{
			Provider: oracleProvider,
			Relayers: []string{admin},
		})
		app.OracleKeeper.SetProviderPriceState(ctx, oracleProvider, oracletypes.NewProviderPriceState(oracleSymbol, startingPrice, ctx.BlockTime().Unix()))

		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))
		coin := sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, sdk.OneInt())
		app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
		app.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(coin))
		adminAccount, _ := sdk.AccAddressFromBech32(testInput.BinaryMarkets[0].Admin)
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, adminAccount, sdk.NewCoins(coin))
		app.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, sdk.NewCoins(coin))

		_, err = app.ExchangeKeeper.BinaryOptionsMarketLaunch(
			ctx,
			testInput.BinaryMarkets[0].Ticker,
			testInput.BinaryMarkets[0].OracleSymbol,
			testInput.BinaryMarkets[0].OracleProvider,
			oracletypes.OracleType_Provider,
			testInput.BinaryMarkets[0].OracleScaleFactor,
			testInput.BinaryMarkets[0].MakerFeeRate,
			testInput.BinaryMarkets[0].TakerFeeRate,
			testInput.BinaryMarkets[0].ExpirationTimestamp,
			testInput.BinaryMarkets[0].SettlementTimestamp,
			testInput.BinaryMarkets[0].Admin,
			testInput.BinaryMarkets[0].QuoteDenom,
			testInput.BinaryMarkets[0].MinPriceTickSize,
			testInput.BinaryMarkets[0].MinQuantityTickSize,
		)
		market = app.ExchangeKeeper.GetBinaryOptionsMarket(ctx, testInput.BinaryMarkets[0].MarketID, true)

		testexchange.OrFail(err)
	})

	Describe("InstantBinaryOptionsMarketLaunch", func() {
		var (
			err          error
			message      *types.MsgInstantBinaryOptionsMarketLaunch
			amountMinted *sdk.Coin
			amountNeeded math.Int
			marketID     common.Hash
		)
		sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))

		BeforeEach(func() {
			oracleSymbol, oracleProvider, admin := testInput.BinaryMarkets[1].OracleSymbol, testInput.BinaryMarkets[1].OracleProvider, testInput.BinaryMarkets[1].Admin
			startingPrice := sdk.NewDecWithPrec(5, 1)
			app.OracleKeeper.SetProviderPriceState(ctx, oracleProvider, oracletypes.NewProviderPriceState(oracleSymbol, startingPrice, ctx.BlockTime().Unix()))
			message = &types.MsgInstantBinaryOptionsMarketLaunch{
				Sender:              sender.String(),
				Ticker:              testInput.BinaryMarkets[1].Ticker,
				OracleSymbol:        testInput.BinaryMarkets[1].OracleSymbol,
				OracleProvider:      testInput.BinaryMarkets[1].OracleProvider,
				OracleType:          oracletypes.OracleType_Provider,
				OracleScaleFactor:   0,
				MakerFeeRate:        testInput.BinaryMarkets[1].MakerFeeRate,
				TakerFeeRate:        testInput.BinaryMarkets[1].TakerFeeRate,
				ExpirationTimestamp: testInput.BinaryMarkets[1].ExpirationTimestamp,
				SettlementTimestamp: testInput.BinaryMarkets[1].SettlementTimestamp,
				Admin:               admin,
				QuoteDenom:          testInput.BinaryMarkets[1].QuoteDenom,
				MinPriceTickSize:    testInput.BinaryMarkets[1].MinPriceTickSize,
				MinQuantityTickSize: testInput.BinaryMarkets[1].MinQuantityTickSize,
			}

			coin := sdk.NewCoin(testInput.BinaryMarkets[1].QuoteDenom, sdk.OneInt())
			app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
			err := app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, sdk.NewCoins(coin))
			testexchange.OrFail(err)

			amountNeeded = math.Int(sdk.NewDec(100))
			marketID = types.NewBinaryOptionsMarketID(testInput.BinaryMarkets[1].Ticker, testInput.BinaryMarkets[1].QuoteDenom, oracleSymbol, oracleProvider, oracletypes.OracleType_Provider)
		})

		JustBeforeEach(func() {
			_, err = msgServer.InstantBinaryOptionsMarketLaunch(sdk.WrapSDKContext(ctx), message)
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

			It("Should have correct market fields", func() {
				marketInfo := app.ExchangeKeeper.GetBinaryOptionsMarketByID(ctx, marketID)
				expectedRelayerFeeShareRate := sdk.NewDecWithPrec(40, 2)

				Expect(marketInfo.Ticker).To(Equal(testInput.BinaryMarkets[1].Ticker))
				Expect(marketInfo.QuoteDenom).To(Equal(testInput.BinaryMarkets[1].QuoteDenom))
				Expect(marketInfo.MakerFeeRate).To(Equal(testInput.BinaryMarkets[1].MakerFeeRate))
				Expect(marketInfo.TakerFeeRate).To(Equal(testInput.BinaryMarkets[1].TakerFeeRate))
				Expect(marketInfo.MinPriceTickSize).To(Equal(testInput.BinaryMarkets[1].MinPriceTickSize))
				Expect(marketInfo.MinQuantityTickSize).To(Equal(testInput.BinaryMarkets[1].MinQuantityTickSize))
				Expect(marketInfo.OracleSymbol).To(Equal(testInput.BinaryMarkets[1].OracleSymbol))
				Expect(marketInfo.OracleProvider).To(Equal(testInput.BinaryMarkets[1].OracleProvider))
				Expect(marketInfo.OracleType).To(Equal(oracletypes.OracleType_Provider))
				Expect(marketInfo.Admin).To(Equal(testInput.BinaryMarkets[1].Admin))
				Expect(marketInfo.Status).To(Equal(types.MarketStatus_Active))
				Expect(marketInfo.RelayerFeeShareRate).To(Equal(expectedRelayerFeeShareRate))
				Expect(marketInfo.ExpirationTimestamp).To(Equal(testInput.BinaryMarkets[1].ExpirationTimestamp))
				Expect(marketInfo.SettlementTimestamp).To(Equal(testInput.BinaryMarkets[1].SettlementTimestamp))
				Expect(marketInfo.SettlementPrice).To(Equal(testInput.BinaryMarkets[1].SettlementPrice))
			})
		})

		Context("With sender not having enough balance", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(50)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)
			})

			It("Should be invalid with insufficient funds error", func() {
				errorMessage := amountMinted.String() + " is smaller than " + amountNeeded.String() + "inj: "
				expectedError := "spendable balance " + errorMessage + sdkerrors.ErrInsufficientFunds.Error()
				Expect(err.Error()).To(Equal(expectedError))
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
		})

		Context("When binary options market already exists", func() {
			BeforeEach(func() {
				amountMinted = &sdk.Coin{
					Denom:  "inj",
					Amount: math.Int(sdk.NewDec(10000)),
				}

				amount := sdk.Coins{*amountMinted}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				msgServer.InstantBinaryOptionsMarketLaunch(sdk.WrapSDKContext(ctx), message)
			})

			It("Should be invalid with binary options market exists error", func() {
				errorMessagePart1 := "ticker " + message.Ticker + " quoteDenom " + message.QuoteDenom + ": "
				expectedError := errorMessagePart1 + types.ErrBinaryOptionsMarketExists.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("Prevent conditions", func() {
			It("Prevent perpetual market launch when an equivalent market launch proposal already exists", func() {
				// submit perpetual market launch proposal
				proposalContent := types.NewBinaryOptionsMarketLaunchProposal(
					"binary options market launch proposal",
					"binary options market launch proposal for specific market",
					message.Ticker,
					message.OracleSymbol,
					message.OracleProvider,
					message.OracleType,
					message.OracleScaleFactor,
					message.ExpirationTimestamp,
					message.SettlementTimestamp,
					message.Admin,
					message.QuoteDenom,
					message.MakerFeeRate,
					message.TakerFeeRate,
					message.MinPriceTickSize,
					message.MinQuantityTickSize,
				)
				proposalMsg, err := govtypesv1.NewLegacyContent(proposalContent, app.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String())
				Expect(err).To(BeNil())
				proposal, err := app.GovKeeper.SubmitProposal(ctx, []sdk.Msg{proposalMsg}, "", proposalContent.Title, proposalContent.Description, sender)
				Expect(err).To(BeNil())

				// proposal to active queue
				app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

				// prepare coins for instant market launch
				amount := sdk.Coins{sdk.NewCoin("inj", math.Int(sdk.NewDec(10000)))}
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amount)
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, amount)

				// try execution of instant perpetual market
				_, err = msgServer.InstantBinaryOptionsMarketLaunch(sdk.WrapSDKContext(ctx), message)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring(types.ErrMarketLaunchProposalAlreadyExists.Error()))
			})
		})
	})

	Describe("CreateBinaryOptionsLimitOrder for vanilla orders", func() {
		var (
			err          error
			subaccountID string
			message      *types.MsgCreateBinaryOptionsLimitOrder
			balanceUsed  sdk.Dec
			deposit      *types.Deposit
		)
		sender := testexchange.SampleAccountAddr1

		BeforeEach(func() {
			subaccountID = testexchange.SampleSubaccountAddr1.String()

			denom := testInput.BinaryMarkets[0].QuoteDenom
			mintAmount := types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor).TruncateInt()
			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(denom, mintAmount)))
			deposit = testexchange.GetBankAndDepositFunds(app, ctx, testexchange.SampleSubaccountAddr1, denom)

			price := types.GetScaledPrice(sdk.NewDecWithPrec(2, 1), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			message = types.NewMsgCreateBinaryOptionsLimitOrder(
				sender,
				market,
				subaccountID,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
			feePaid := market.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(message.Order.Margin)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceUsed).String()))
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

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceUsed).String()))
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
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with derivative market not found error", func() {
				errorMessage := "marketID " + message.Order.MarketId + ": "
				expectedError := errorMessage + types.ErrBinaryOptionsMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When price tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDecWithPrec(2001, 5)
			})

			It("Should be invalid with invalid price error", func() {
				errorMessage1 := "price " + message.Order.OrderInfo.Price.String()
				errorMessage2 := " must be a multiple of the minimum price tick size " + market.MinPriceTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When quantity tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(2, 5)
			})

			It("Should be invalid with invalid quantity error", func() {
				errorMessage1 := "quantity " + message.Order.OrderInfo.Quantity.String()
				errorMessage2 := " must be a multiple of the minimum quantity tick size " + market.MinQuantityTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidQuantity.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When deposits of subaccount are not existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.SubaccountId = testexchange.SampleSubaccountAddr3.String()
			})

			It("Should be invalid with insufficient depposit error", func() {
				Expect(types.ErrInsufficientDeposit.Is(err) || types.ErrInsufficientFunds.Is(err)).To(BeTrue(), "wrong error was thrown")
			})
		})

		Context("When already placed a market order", func() {
			BeforeEach(func() {
				subaccountIdSeller := testexchange.SampleSubaccountAddr2.String()
				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}
				testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(2, 1), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sender,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				binaryOptionsMarketBuyOrderMessage := types.NewMsgCreateBinaryOptionsMarketOrder(
					sender,
					market,
					subaccountID,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsMarketOrder(sdk.WrapSDKContext(ctx), binaryOptionsMarketBuyOrderMessage)
				testexchange.OrFail(err2)
			})

			It("Should be allowed", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When there are existing reduce-only orders", func() {
			BeforeEach(func() {
				subaccountIdBuyer := testexchange.SampleSubaccountAddr2.String()
				subaccountIdSeller := subaccountID

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(2, 1), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sender,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				price = types.GetScaledPrice(sdk.NewDecWithPrec(2, 1), market.OracleScaleFactor)
				quantity = sdk.NewDec(2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sender,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("which are better priced than vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {

					price := types.GetScaledPrice(sdk.NewDecWithPrec(201, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					existingReduceOnlyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sender,
						market,
						subaccountID,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_BUY,
						true,
					)

					// Create two reduce only orders.
					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountID),
						true,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountID),
						true,
					)

					Expect(metadataBefore.ReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(metadataBefore.AggregateReduceOnlyQuantity).To(Equal(metadataAfter.AggregateReduceOnlyQuantity))
				})
			})

			Context("which are equally priced to vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {

					price := types.GetScaledPrice(sdk.NewDecWithPrec(2, 1), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					existingReduceOnlyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sender,
						market,
						subaccountID,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_BUY,
						true,
					)

					// Create two reduce only orders.
					_, err1 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err1)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountID),
						true,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountID),
						true,
					)

					expectedReduceOnlyLimitOrderCount := metadataBefore.ReduceOnlyLimitOrderCount - 2
					expectedAggregateReduceOnlyQuantity := metadataBefore.AggregateReduceOnlyQuantity.Sub(sdk.NewDec(2))
					Expect(expectedReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(expectedAggregateReduceOnlyQuantity.String()).To(Equal(metadataAfter.AggregateReduceOnlyQuantity.String()))
				})
			})

			Context("which are worst priced than vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {

					price := types.GetScaledPrice(sdk.NewDecWithPrec(199, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					existingReduceOnlyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sender,
						market,
						subaccountID,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_BUY,
						true,
					)

					// Create two reduce only orders.
					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountID),
						true,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountID),
						true,
					)

					expectedReduceOnlyLimitOrderCount := metadataBefore.ReduceOnlyLimitOrderCount - 2
					expectedAggregateReduceOnlyQuantity := metadataBefore.AggregateReduceOnlyQuantity.Sub(sdk.NewDec(2))

					Expect(expectedReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(expectedAggregateReduceOnlyQuantity.String()).To(Equal(metadataAfter.AggregateReduceOnlyQuantity.String()))
				})
			})
		})

		Context("When there are already max allowed vanilla only orders", func() {
			BeforeEach(func() {
				for i := 0; i < 20; i++ {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sender,
						market,
						subaccountID,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_BUY,
						false,
					)

					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
					testexchange.OrFail(err2)
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				}
			})

			It("Should be invalid with max orders per side error", func() {
				Expect(err).Should(MatchError(types.ErrExceedsOrderSideCount))
			})
		})
	})

	Describe("CreateBinaryOptionsLimitOrder for reduce-only orders", func() {
		var (
			err                error
			message            *types.MsgCreateBinaryOptionsLimitOrder
			deposit            *types.Deposit
			buyerAddress       = testexchange.SampleAccountAddr1
			sellerAddress      = testexchange.SampleAccountAddr2
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {

			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(testexchange.NewDecFromFloat(0.1), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				buyerAddress,
				market,
				subaccountIdBuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				sellerAddress,
				market,
				subaccountIdSeller,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				false,
			)

			_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = types.NewMsgCreateBinaryOptionsLimitOrder(
				buyerAddress,
				market,
				subaccountIdBuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				true,
			)
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
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
		})

		Context("When quantity is greater than position quantity", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(21, 1)
			})

			It("Should be resized", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When direction of the order is same with direction of existing position", func() {
			BeforeEach(func() {
				message.Order.OrderType = types.OrderType_BUY
			})

			It("Should be invalid with invalid reduce only position direction error", func() {
				expectedError := types.ErrInvalidReduceOnlyPositionDirection.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When position does not exist", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.SubaccountId = types.MustSdkAddressWithNonceToSubaccountID(buyerAddress, 2).String()
			})

			It("Should be invalid with position not found error", func() {
				errorMessage1 := "Position for marketID " + common.HexToHash(message.Order.MarketId).Hex()
				errorMessage2 := " subaccountID " + common.HexToHash(message.Order.OrderInfo.SubaccountId).Hex() + " not found: "
				expectedError := errorMessage1 + errorMessage2 + types.ErrPositionNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When reduce-only order's quantity is greater than position's quantity", func() {
			Context("while there is no existing reduce-only order", func() {
				BeforeEach(func() {
					message.Order.OrderInfo.Quantity = sdk.NewDec(3)
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("while there is existing reduce-only order", func() {
				BeforeEach(func() {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					existingReduceOnlyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						message.Order.OrderInfo.SubaccountId,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						true,
					)

					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		Context("When there is an existing vanilla order", func() {
			Context("which is better priced than reduce-only order", func() {
				BeforeEach(func() {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(190, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(2)

					existingVanillaOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						message.Order.OrderInfo.SubaccountId,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						false,
					)

					msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be resized", func() {
					Expect(err).Should(BeNil())
				})
			})

			Context("which is equally priced with reduce-only order", func() {
				BeforeEach(func() {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(2)

					existingVanillaOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						message.Order.OrderInfo.SubaccountId,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						false,
					)

					msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be resided", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("which is worst priced than reduce-only order", func() {
				BeforeEach(func() {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(2001, 4), market.OracleScaleFactor)
					quantity := sdk.NewDec(2)

					existingVanillaOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						message.Order.OrderInfo.SubaccountId,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						false,
					)

					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("CreateBinaryOptionsMarketOrder for vanilla orders", func() {
		var (
			err                error
			message            *types.MsgCreateBinaryOptionsMarketOrder
			balanceUsed        sdk.Dec
			deposit            *types.Deposit
			buyerAddress       = testexchange.SampleAccountAddr1
			sellerAddress      = testexchange.SampleAccountAddr2
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			price := types.GetScaledPrice(sdk.NewDecWithPrec(210, 3), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				buyerAddress,
				market,
				subaccountIdBuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			price = types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			message = types.NewMsgCreateBinaryOptionsMarketOrder(
				sellerAddress,
				market,
				subaccountIdSeller,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				false,
			)
		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateBinaryOptionsMarketOrder(sdk.WrapSDKContext(ctx), message)

			feePaid := market.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(message.Order.Margin)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
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

			It("Should have updated balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
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
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
				Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})

		Context("When market does not exist", func() {
			BeforeEach(func() {
				message.Order.MarketId = "0x9"
			})

			It("Should be invalid with binary options market not found error", func() {
				errorMessage := "marketID " + message.Order.MarketId + ": "
				expectedError := errorMessage + types.ErrBinaryOptionsMarketNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When price tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = sdk.NewDecWithPrec(2001, 5)
			})

			It("Should be invalid with invalid price error", func() {
				errorMessage1 := "price " + message.Order.OrderInfo.Price.String()
				errorMessage2 := " must be a multiple of the minimum price tick size " + market.MinPriceTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When quantity tick size is wrong", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(2, 5)
			})

			It("Should be invalid with invalid quantity error", func() {
				errorMessage1 := "quantity " + message.Order.OrderInfo.Quantity.String()
				errorMessage2 := " must be a multiple of the minimum quantity tick size " + market.MinQuantityTickSize.String() + ": "
				expectedError := errorMessage1 + errorMessage2 + types.ErrInvalidQuantity.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When deposits of subaccount are insufficient", func() {
			BeforeEach(func() {
				amount := app.ExchangeKeeper.GetSpendableFunds(ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom)
				_, err2 := app.ExchangeKeeper.DecrementDepositOrChargeFromBank(ctx, common.HexToHash(subaccountIdSeller), market.QuoteDenom, amount)
				testexchange.OrFail(err2)
			})

			It("Should be invalid with insufficient deposit error", func() {
				Expect(testexchange.IsExpectedInsufficientFundsErrorType(common.HexToHash(message.Order.OrderInfo.SubaccountId), err)).To(BeTrue())
			})
		})

		Context("When deposits of subaccount are not existing", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.SubaccountId = testexchange.SampleSubaccountAddr3.String()
			})

			It("Should be invalid with insufficient depposit error", func() {
				Expect(testexchange.IsExpectedInsufficientFundsErrorType(common.HexToHash(message.Order.OrderInfo.SubaccountId), err)).To(BeTrue())
			})
		})

		Context("When there is no liquidity in the orderbook", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When worst price is not valid", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Price = types.GetScaledPrice(sdk.NewDecWithPrec(2200, 4), market.OracleScaleFactor)
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When already placed another market order", func() {
			BeforeEach(func() {
				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				price = types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity = sdk.NewDec(2)

				binaryOptionsMarketSellOrderMessage := types.NewMsgCreateBinaryOptionsMarketOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsMarketOrder(sdk.WrapSDKContext(ctx), binaryOptionsMarketSellOrderMessage)
				testexchange.OrFail(err2)
			})

			It("Should be invalid with derivative market order already exists error", func() {
				expectedError := types.ErrMarketOrderAlreadyExists.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When already placed a limit order", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

			})

			It("Should be invalid with derivative limit order already exists error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When there are existing reduce-only orders", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(4)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				price = types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity = sdk.NewDec(2)

				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				price = types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity = sdk.NewDec(10)

				binaryOptionsLimitBuyOrderMessage1 := types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage1)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("which are worst priced than vanilla order", func() {
				var metadataBefore *types.SubaccountOrderbookMetadata
				var metadataAfter *types.SubaccountOrderbookMetadata
				BeforeEach(func() {

					price := types.GetScaledPrice(sdk.NewDecWithPrec(201, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					existingReduceOnlyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sellerAddress,
						market,
						subaccountIdSeller,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						true,
					)

					// Create two reduce only orders.
					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)
					_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

					metadataBefore = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountIdSeller),
						false,
					)
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Has correct metadata", func() {
					metadataAfter = app.ExchangeKeeper.GetSubaccountOrderbookMetadata(
						ctx,
						common.HexToHash(market.MarketId),
						common.HexToHash(subaccountIdSeller),
						false,
					)

					expectedReduceOnlyLimitOrderCount := metadataBefore.ReduceOnlyLimitOrderCount - 2
					expectedAggregateReduceOnlyQuantity := metadataBefore.AggregateReduceOnlyQuantity.Sub(sdk.NewDec(2))

					Expect(expectedReduceOnlyLimitOrderCount).To(Equal(metadataAfter.ReduceOnlyLimitOrderCount))
					Expect(expectedAggregateReduceOnlyQuantity.String()).To(Equal(metadataAfter.AggregateReduceOnlyQuantity.String()))
				})
			})
		})
	})

	Describe("CreateBinaryOptionsMarketOrder for reduce-only orders", func() {
		var (
			err                error
			message            *types.MsgCreateBinaryOptionsMarketOrder
			deposit            *types.Deposit
			buyerAddress       = testexchange.SampleAccountAddr1
			sellerAddress      = testexchange.SampleAccountAddr2
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				buyerAddress,
				market,
				subaccountIdBuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			price = types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				sellerAddress,
				market,
				subaccountIdSeller,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				false,
			)

			_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			binaryOptionsLimitBuyOrderMessage2 := types.NewMsgCreateBinaryOptionsLimitOrder(
				sellerAddress,
				market,
				subaccountIdSeller,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage2)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			message = types.NewMsgCreateBinaryOptionsMarketOrder(
				buyerAddress,
				market,
				subaccountIdBuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				true,
			)

		})

		JustBeforeEach(func() {
			_, err = msgServer.CreateBinaryOptionsMarketOrder(sdk.WrapSDKContext(ctx), message)
		})

		Context("With all valid fields", func() {
			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When quantity is greater than position quantity", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.Quantity = sdk.NewDecWithPrec(21, 1)
			})

			It("Should be resized", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("When direction of the order is same with direction of existing position", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(4)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				message.Order.OrderType = types.OrderType_BUY
			})

			It("Should be invalid with invalid reduce only position direction error", func() {
				expectedError := types.ErrInvalidReduceOnlyPositionDirection.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When there is no liquidity in the orderbook", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			It("Should be invalid with slippage exceeds worst price error", func() {
				expectedError := types.ErrSlippageExceedsWorstPrice.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		//Context("When worst price is not valid", func() {
		//	BeforeEach(func() {
		//		message.Order.OrderInfo.Price = types.GetScaledPrice(sdk.NewDecWithPrec(2200, 4), market.OracleScaleFactor*2)
		//	})
		//
		//	It("Should be invalid with invalid price error", func() {
		//		errorMessage1 := "price must be less than " + types.GetScaledPrice(sdk.OneDec(), market.GetOracleScaleFactor()).String() + ": "
		//		expectedError := errorMessage1 + types.ErrInvalidPrice.Error()
		//		Expect(err.Error()).To(Equal(expectedError))
		//	})
		//})

		Context("When position does not exist", func() {
			BeforeEach(func() {
				message.Order.OrderInfo.SubaccountId = types.MustSdkAddressWithNonceToSubaccountID(sellerAddress, 2).String()
			})

			It("Should be invalid with position not found error", func() {
				errorMessage1 := "Position for marketID " + common.HexToHash(message.Order.MarketId).Hex()
				errorMessage2 := " subaccountID " + common.HexToHash(message.Order.OrderInfo.SubaccountId).Hex() + " not found: "
				expectedError := errorMessage1 + errorMessage2 + types.ErrPositionNotFound.Error()
				Expect(err.Error()).To(Equal(expectedError))
			})
		})

		Context("When reduce-only order's quantity is greater than position's quantity", func() {
			Context("while there is no existing reduce-only order", func() {
				BeforeEach(func() {
					message.Order.OrderInfo.Quantity = sdk.NewDec(3)
				})

				It("Should be resized", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("while there is existing reduce-only order", func() {
				BeforeEach(func() {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(210, 3), market.OracleScaleFactor)
					quantity := sdk.NewDec(1)

					existingReduceOnlyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sellerAddress,
						market,
						message.Order.OrderInfo.SubaccountId,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						true,
					)

					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingReduceOnlyOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be invalid with insufficient position quantity error", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		Context("When there is an existing vanilla order", func() {
			// TODO - Check if can reproduce with better priced than reduce-only
			Context("which is worst priced than reduce-only order", func() {
				BeforeEach(func() {
					price := types.GetScaledPrice(sdk.NewDecWithPrec(2001, 4), market.OracleScaleFactor)
					quantity := sdk.NewDec(2)

					existingVanillaOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
						sellerAddress,
						market,
						message.Order.OrderInfo.SubaccountId,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						price,
						quantity,
						types.OrderType_SELL,
						false,
					)

					_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), existingVanillaOrderMessage)
					testexchange.OrFail(err2)

					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
					Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				})

				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("CreateBinaryOptionsMarketOrder for vanilla orders special cases", func() {
		var (
			err                               error
			message                           *types.MsgCreateBinaryOptionsMarketOrder
			balanceUsed                       sdk.Dec
			deposit                           *types.Deposit
			balanceBefore                     *types.Deposit
			balanceAfterCreation              *types.Deposit
			balanceAfterEndBlocker            *types.Deposit
			binaryOptionsLimitBuyOrderMessage *types.MsgCreateBinaryOptionsLimitOrder
			buyerAddress                      = testexchange.SampleAccountAddr1
			sellerAddress                     = testexchange.SampleAccountAddr2
			subaccountIdBuyer                 = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller                = testexchange.SampleSubaccountAddr2.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			binaryOptionsLimitBuyOrderMessage = types.NewMsgCreateBinaryOptionsLimitOrder(
				buyerAddress,
				market,
				subaccountIdBuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			price = types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			message = types.NewMsgCreateBinaryOptionsMarketOrder(
				sellerAddress,
				market,
				subaccountIdSeller,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				false,
			)
		})

		Context("When the order to be executed against market order is getting canceled", func() {
			BeforeEach(func() {
				balanceBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.BinaryMarkets[0].QuoteDenom)
				_, err = msgServer.CreateBinaryOptionsMarketOrder(sdk.WrapSDKContext(ctx), message)

				feePaid := market.TakerFeeRate.Mul(message.Order.OrderInfo.Price).Mul(message.Order.OrderInfo.Quantity)
				balanceUsed = feePaid.Add(message.Order.Margin)

				balanceAfterCreation = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.BinaryMarkets[0].QuoteDenom)

				derivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					common.HexToHash(market.MarketId),
					true,
				)

				restingOrder := derivativeOrders[0]

				cancelMessage := &types.MsgCancelBinaryOptionsOrder{
					Sender:       buyerAddress.String(),
					MarketId:     market.MarketId,
					SubaccountId: subaccountIdBuyer,
					OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
				}

				_, err2 := msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), cancelMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				balanceAfterEndBlocker = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIdSeller), testInput.BinaryMarkets[0].QuoteDenom)
			})

			It("Should be valid", func() {
				Expect(err).To(BeNil())
			})

			It("Should not be executed and have correct balances", func() {
				availableBalanceAfterCreation := deposit.AvailableBalance.Sub(balanceUsed)

				Expect(balanceBefore.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(balanceBefore.TotalBalance).To(Equal(deposit.TotalBalance))

				Expect(balanceAfterCreation.AvailableBalance).To(Equal(availableBalanceAfterCreation))
				Expect(balanceAfterCreation.TotalBalance).To(Equal(deposit.TotalBalance))

				Expect(balanceAfterEndBlocker.AvailableBalance).To(Equal(deposit.AvailableBalance))
				Expect(balanceAfterEndBlocker.TotalBalance).To(Equal(deposit.TotalBalance))
			})
		})
	})

	Describe("CancelBinaryOptionsOrder for vanilla orders", func() {
		var (
			err                            error
			message                        *types.MsgCancelBinaryOptionsOrder
			balanceUsed                    sdk.Dec
			deposit                        *types.Deposit
			binaryOptionsLimitOrderMessage *types.MsgCreateBinaryOptionsLimitOrder
			sender                         = testexchange.SampleAccountAddr1
			subaccountID                   = testexchange.SampleSubaccountAddr1.String()
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			binaryOptionsLimitOrderMessage = types.NewMsgCreateBinaryOptionsLimitOrder(
				sender,
				market,
				subaccountID,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
			testexchange.OrFail(err2)
			feePaid := market.MakerFeeRate.Mul(binaryOptionsLimitOrderMessage.Order.OrderInfo.Price).Mul(binaryOptionsLimitOrderMessage.Order.OrderInfo.Quantity)
			balanceUsed = feePaid.Add(binaryOptionsLimitOrderMessage.Order.Margin)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			derivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
				ctx,
				common.HexToHash(market.MarketId),
				true,
			)

			Expect(len(derivativeOrders)).To((Equal(1)))
			restingOrder := derivativeOrders[0]

			message = &types.MsgCancelBinaryOptionsOrder{
				Sender:       sender.String(),
				MarketId:     market.MarketId,
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), message)
		})

		Describe("When they are not filled", func() {
			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
				})

				It("Should have correct event", func() {
					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.BinaryMarkets[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Margin).To(Equal(binaryOptionsLimitOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
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
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
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
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
				})
			})

			Context("When market does not exist", func() {
				BeforeEach(func() {
					message.MarketId = common.HexToHash("0x9").Hex()
				})

				It("Should be invalid with derivative market not found error", func() {
					errorMessage := "active derivative market doesn't exist " + message.MarketId + ": "
					expectedError := errorMessage + types.ErrDerivativeMarketNotFound.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have not updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceUsed).String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
				})
			})

			Context("When order does not exist", func() {
				BeforeEach(func() {
					message.SubaccountId = types.MustSdkAddressWithNonceToSubaccountID(sender, 2).String()
				})

				It("Should be invalid with order does not exist error", func() {
					expectedError := "Derivative Limit Order doesn't exist: " + types.ErrOrderDoesntExist.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have not updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceUsed).String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
				})
			})
		})

		Describe("When they are partially filled", func() {
			BeforeEach(func() {
				partialSender := testexchange.SampleAccountAddr3
				partialSubaccountID := testexchange.SampleSubaccountAddr3.String()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, partialSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(1)

				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					partialSender,
					market,
					partialSubaccountID,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have updated balances", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.Sub(balanceUsed.Quo(sdk.NewDec(2))).String()))
					Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.Sub(balanceUsed.Quo(sdk.NewDec(2))).String()))
				})

				It("Should have correct event", func() {
					expectedFillable := binaryOptionsLimitOrderMessage.Order.OrderInfo.Quantity.Sub(sdk.OneDec())

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.BinaryMarkets[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(expectedFillable))
							Expect(event.LimitOrder.Margin).To(Equal(binaryOptionsLimitOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(binaryOptionsLimitOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})
		})

		Describe("When they are fully filled", func() {
			BeforeEach(func() {
				fullSender := testexchange.SampleAccountAddr3
				fullSubaccountID := testexchange.SampleSubaccountAddr3.String()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, fullSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					fullSender,
					market,
					fullSubaccountID,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be invalid with order does not exist error", func() {
					expectedError := "Derivative Limit Order doesn't exist: " + types.ErrOrderDoesntExist.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})

				It("Should have updated balances for all filled quantity", func() {
					depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

					Expect(depositAfter.AvailableBalance).To(Equal(deposit.AvailableBalance.Sub(balanceUsed)))
					Expect(depositAfter.TotalBalance).To(Equal(deposit.TotalBalance.Sub(balanceUsed)))
				})
			})
		})
	})

	Describe("CancelBinaryOptionsOrder for transient buy orders", func() {
		var (
			err          error
			sender       = testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			message      *types.MsgCancelBinaryOptionsOrder
			deposit      *types.Deposit
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(1121), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(1121), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(sdk.NewDecWithPrec(3, 4), market.OracleScaleFactor)
			quantity := sdk.NewDec(40)

			binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				sender,
				market,
				subaccountID,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.BinaryMarkets[0].MarketID,
				true,
			)

			Expect(len(transientLimitOrders)).To(Equal(1))
			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelBinaryOptionsOrder{
				Sender:       sender.String(),
				MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					true,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					true,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelBinaryOptionsOrder for transient sell orders", func() {
		var (
			err          error
			sender       = testexchange.SampleAccountAddr1
			subaccountID = testexchange.SampleSubaccountAddr1.String()
			message      *types.MsgCancelBinaryOptionsOrder
			deposit      *types.Deposit
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(5121), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(5121), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(sdk.NewDecWithPrec(1000, 4), market.OracleScaleFactor)
			quantity := sdk.NewDec(3)

			binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				sender,
				market,
				subaccountID,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				false,
			)

			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.BinaryMarkets[0].MarketID,
				false,
			)

			Expect(len(transientLimitOrders)).To(Equal(1))
			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelBinaryOptionsOrder{
				Sender:       sender.String(),
				MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
				SubaccountId: subaccountID,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountID), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(deposit.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(deposit.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					false,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					false,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelBinaryOptionsOrder for transient reduce-only buy orders", func() {
		var (
			err                error
			buyer              = testexchange.SampleAccountAddr1
			subaccountIDbuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIDseller = testexchange.SampleSubaccountAddr2.String()
			message            *types.MsgCancelBinaryOptionsOrder
			deposit            *types.Deposit
			depositBefore      *types.Deposit
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(7121), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(7121), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDbuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIDseller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			var margin sdk.Dec
			price := sdk.NewDecWithPrec(2000, 4)
			quantity := sdk.NewDec(2)
			matchBuyerAndSeller(
				testInput,
				app,
				ctx,
				msgServer,
				types.MarketType_BinaryOption,
				margin,
				quantity,
				price,
				false,
				common.HexToHash(subaccountIDbuyer),
				common.HexToHash(subaccountIDseller),
			)

			depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIDbuyer), testInput.BinaryMarkets[0].QuoteDenom)

			price = types.GetScaledPrice(sdk.NewDecWithPrec(1800, 4), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				buyer,
				market,
				subaccountIDbuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				true,
			)

			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.BinaryMarkets[0].MarketID,
				true,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelBinaryOptionsOrder{
				Sender:       buyer.String(),
				MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
				SubaccountId: subaccountIDbuyer,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIDbuyer), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(depositBefore.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(depositBefore.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					true,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					true,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelBinaryOptionsOrder for transient reduce-only sell orders", func() {
		var (
			err                error
			buyer              = testexchange.SampleAccountAddr1
			subaccountIDbuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIDseller = testexchange.SampleSubaccountAddr2.String()
			message            *types.MsgCancelBinaryOptionsOrder
			deposit            *types.Deposit
			depositBefore      *types.Deposit
		)
		// sender := sdk.AccAddress(common.FromHex("90f8bf6a479f320ead074411a4b0e7944ea8c9c1"))

		BeforeEach(func() {
			// subaccountID = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
			// seller := "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000002"

			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(7121), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(7121), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDbuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIDseller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			var margin sdk.Dec
			price := sdk.NewDecWithPrec(2000, 4)
			quantity := sdk.NewDec(2)
			matchBuyerAndSeller(
				testInput,
				app,
				ctx,
				msgServer,
				types.MarketType_BinaryOption,
				margin,
				quantity,
				price,
				true,
				common.HexToHash(subaccountIDbuyer),
				common.HexToHash(subaccountIDseller),
			)

			depositBefore = testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIDbuyer), testInput.BinaryMarkets[0].QuoteDenom)

			price = types.GetScaledPrice(sdk.NewDecWithPrec(1800, 4), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				buyer,
				market,
				subaccountIDbuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				true,
			)

			_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
			testexchange.OrFail(err)

			transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
				ctx,
				testInput.BinaryMarkets[0].MarketID,
				false,
			)

			transientOrder := transientLimitOrders[0]

			message = &types.MsgCancelBinaryOptionsOrder{
				Sender:       buyer.String(),
				MarketId:     testInput.BinaryMarkets[0].MarketID.Hex(),
				SubaccountId: subaccountIDbuyer,
				OrderHash:    "0x" + common.Bytes2Hex(transientOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), message)
			testexchange.OrFail(err)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
		})

		Context("With all valid fields", func() {
			It("Should have correct balances", func() {
				depositAfter := testexchange.GetBankAndDepositFunds(app, ctx, common.HexToHash(subaccountIDbuyer), testInput.BinaryMarkets[0].QuoteDenom)

				Expect(depositAfter.AvailableBalance.String()).To(Equal(depositBefore.AvailableBalance.String()))
				Expect(depositAfter.TotalBalance.String()).To(Equal(depositBefore.TotalBalance.String()))
			})

			It("Should be deleted from transient store", func() {
				transientLimitOrders := app.ExchangeKeeper.GetAllTransientDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					false,
				)

				Expect(len(transientLimitOrders)).To(Equal(0))
			})

			It("Should not exist as resting order", func() {
				restingLimitOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
					ctx,
					testInput.BinaryMarkets[0].MarketID,
					false,
				)

				Expect(len(restingLimitOrders)).To(Equal(0))
			})
		})
	})

	Describe("CancelBinaryOptionsOrder for reduce-only orders", func() {
		var (
			err                                      error
			buyer                                    = testexchange.SampleAccountAddr1
			seller                                   = testexchange.SampleAccountAddr2
			subaccountIDbuyer                        = testexchange.SampleSubaccountAddr1.String()
			subaccountIDseller                       = testexchange.SampleSubaccountAddr2.String()
			message                                  *types.MsgCancelBinaryOptionsOrder
			deposit                                  *types.Deposit
			binaryOptionsLimitReduceOnlyOrderMessage *types.MsgCreateBinaryOptionsLimitOrder
		)

		BeforeEach(func() {
			deposit = &types.Deposit{
				AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
			}

			testexchange.MintAndDeposit(app, ctx, subaccountIDbuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
			testexchange.MintAndDeposit(app, ctx, subaccountIDseller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

			price := types.GetScaledPrice(sdk.NewDecWithPrec(2000, 4), market.OracleScaleFactor)
			quantity := sdk.NewDec(2)

			binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				buyer,
				market,
				subaccountIDbuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_BUY,
				false,
			)

			_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
			testexchange.OrFail(err2)

			price = types.GetScaledPrice(sdk.NewDecWithPrec(2000, 4), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
				seller,
				market,
				subaccountIDseller,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				false,
			)

			_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			price = types.GetScaledPrice(sdk.NewDecWithPrec(2000, 4), market.OracleScaleFactor)
			quantity = sdk.NewDec(2)

			binaryOptionsLimitReduceOnlyOrderMessage = types.NewMsgCreateBinaryOptionsLimitOrder(
				buyer,
				market,
				subaccountIDbuyer,
				"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
				price,
				quantity,
				types.OrderType_SELL,
				true,
			)

			_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitReduceOnlyOrderMessage)
			testexchange.OrFail(err2)

			ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
			Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			derivativeOrders := app.ExchangeKeeper.GetAllDerivativeLimitOrdersByMarketDirection(
				ctx,
				common.HexToHash(market.MarketId),
				false,
			)

			restingOrder := derivativeOrders[0]

			message = &types.MsgCancelBinaryOptionsOrder{
				Sender:       buyer.String(),
				MarketId:     market.MarketId,
				SubaccountId: subaccountIDbuyer,
				OrderHash:    "0x" + common.Bytes2Hex(restingOrder.OrderHash),
			}
		})

		JustBeforeEach(func() {
			_, err = msgServer.CancelBinaryOptionsOrder(sdk.WrapSDKContext(ctx), message)
		})

		Describe("When they are not filled", func() {
			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have correct event", func() {
					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.BinaryMarkets[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Margin).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})
		})

		Describe("When they are partially filled", func() {
			BeforeEach(func() {
				partialFiller := testexchange.SampleAccountAddr3
				partialFillerSubaccountID := testexchange.SampleSubaccountAddr3.String()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, partialFillerSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(2000, 4), market.OracleScaleFactor)
				quantity := sdk.NewDec(1)

				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					partialFiller,
					market,
					partialFillerSubaccountID,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)
				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be valid", func() {
					Expect(err).To(BeNil())
				})

				It("Should have correct event", func() {
					expectedFillable := binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity.Sub(sdk.OneDec())

					emittedABCIEvents := ctx.EventManager().ABCIEvents()
					for _, emittedABCIEvent := range emittedABCIEvents {
						if testexchange.ShouldIgnoreEvent(emittedABCIEvent) {
							continue
						}

						parsedEvent, err1 := sdk.ParseTypedEvent(emittedABCIEvent)
						testexchange.OrFail(err1)

						switch event := parsedEvent.(type) {
						case *types.EventCancelDerivativeOrder:
							Expect(common.HexToHash(event.MarketId)).To(Equal(testInput.BinaryMarkets[0].MarketID))
							Expect(event.IsLimitCancel).To(Equal(true))
							Expect(event.LimitOrder.OrderInfo.SubaccountId).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.SubaccountId))
							Expect(event.LimitOrder.OrderInfo.Price).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.Price))
							Expect(event.LimitOrder.OrderInfo.Quantity).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.Quantity))
							Expect(event.LimitOrder.Fillable).To(Equal(expectedFillable))
							Expect(event.LimitOrder.Margin).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.Margin))
							Expect(event.LimitOrder.OrderType).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderType))
							Expect(event.LimitOrder.OrderInfo.FeeRecipient).To(Equal(binaryOptionsLimitReduceOnlyOrderMessage.Order.OrderInfo.FeeRecipient))
							Expect("0x" + common.Bytes2Hex(event.LimitOrder.OrderHash)).To(Equal(message.OrderHash))
						}
					}
				})
			})
		})

		Describe("When they are fully filled", func() {
			BeforeEach(func() {
				partialFiller := testexchange.SampleAccountAddr3
				partialFillerSubaccountID := testexchange.SampleSubaccountAddr3.String()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, partialFillerSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(2000, 4), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					partialFiller,
					market,
					partialFillerSubaccountID,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be invalid with order does not exist error", func() {
					expectedError := "Derivative Limit Order doesn't exist: " + types.ErrOrderDoesntExist.Error()
					Expect(err.Error()).To(Equal(expectedError))
				})
			})
		})

		Describe("When position has been netted out", func() {
			BeforeEach(func() {
				price := types.GetScaledPrice(sdk.NewDecWithPrec(1900, 4), market.OracleScaleFactor)
				quantity := sdk.NewDec(2)

				binaryOptionsLimitNettingOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					buyer,
					market,
					subaccountIDbuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitNettingOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

				filler := testexchange.SampleAccountAddr3
				fillerSubaccountID := testexchange.SampleSubaccountAddr3.String()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, fillerSubaccountID, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price = types.GetScaledPrice(sdk.NewDecWithPrec(2000, 4), market.OracleScaleFactor)
				quantity = sdk.NewDec(2)

				binaryOptionsLimitOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					filler,
					market,
					fillerSubaccountID,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("With all valid fields", func() {
				It("Should be invalid with derivative position does not exist error", func() {
					Expect(err).NotTo(BeNil())
				})
			})
		})
	})

	Describe("Admin update messages", func() {
		var (
			msg  *types.MsgAdminUpdateBinaryOptionsMarket
			resp *types.MsgAdminUpdateBinaryOptionsMarketResponse
		)
		BeforeEach(func() {
			msg = &types.MsgAdminUpdateBinaryOptionsMarket{
				Sender:              admin,
				MarketId:            market.MarketId,
				SettlementPrice:     nil,
				ExpirationTimestamp: market.ExpirationTimestamp,
				SettlementTimestamp: market.SettlementTimestamp,
				Status:              types.MarketStatus_Unspecified,
			}
		})
		JustBeforeEach(func() {
			Expect(msg.ValidateBasic()).To(BeNil())
			resp, err = msgServer.AdminUpdateBinaryOptionsMarket(sdk.WrapSDKContext(ctx), msg)
		})
		It("should return without errors", func() {
			Expect(msg.ValidateBasic()).To(BeNil())
			Expect(resp).To(Not(BeNil()))
			Expect(err).To(BeNil())
		})

		When("market does not exists", func() {
			BeforeEach(func() {
				msg.MarketId = types.NewBinaryOptionsMarketID(market.Ticker, market.QuoteDenom, market.OracleSymbol+"awd", market.OracleProvider, market.OracleType).String()
			})
			It("should return error", func() {
				Expect(resp).To(BeNil())
				expError := sdkerrors.Wrapf(types.ErrBinaryOptionsMarketNotFound, "marketID %s", msg.MarketId).Error()
				Expect(err.Error()).To(Equal(expError))
			})
		})

		When("sender is not an admin", func() {
			BeforeEach(func() {
				msg.Sender = testexchange.FeeRecipient
			})
			It("should return error", func() {
				Expect(resp).To(BeNil())
				expError := sdkerrors.Wrapf(types.ErrSenderIsNotAnAdmin, "sender %s, admin %s", msg.Sender, market.Admin).Error()
				Expect(err.Error()).To(Equal(expError))
			})
		})

		When("market was demolished already", func() {
			BeforeEach(func() {
				settlementTime := time.Unix(market.SettlementTimestamp+1, 0)
				ctx = ctx.WithBlockTime(settlementTime)
				exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)
			})
			It("should return error", func() {
				Expect(resp).To(BeNil())
				expError := sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "can't update market that was demolished already").Error()
				Expect(err.Error()).To(Equal(expError))
			})
		})

		When("ExpirationTimestamp is wrong", func() {
			It("Expiration timestamp should be > 0", func() {
				msg.ExpirationTimestamp = -1
				Expect(msg.ValidateBasic()).To(Equal(types.ErrInvalidExpiry))
			})
			It("Expiration timestamp should be < SettlementTimestamp", func() {
				msg.ExpirationTimestamp = msg.SettlementTimestamp + 1
				Expect(msg.ValidateBasic()).To(Equal(types.ErrInvalidExpiry))
			})
			When("Expiration timestamp < market.SettlementTimestamp", func() {
				BeforeEach(func() {
					msg.ExpirationTimestamp = msg.SettlementTimestamp + 1
					msg.SettlementTimestamp = 0
				})
				It("should return error", func() {
					Expect(resp).To(BeNil())
					expError := sdkerrors.Wrap(types.ErrInvalidExpiry, "expiration timestamp should be prior to settlement timestamp").Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
			When("Expiration timestamp is in the past", func() {
				BeforeEach(func() {
					msg.ExpirationTimestamp = ctx.BlockTime().Unix() - 1
				})
				It("should return error", func() {
					Expect(resp).To(BeNil())
					expError := sdkerrors.Wrapf(types.ErrInvalidExpiry, "expiration timestamp %d is in the past", msg.ExpirationTimestamp).Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
			When("Market is expired already", func() {
				BeforeEach(func() {
					expiryTime := time.Unix(market.ExpirationTimestamp+1, 0)
					ctx = ctx.WithBlockTime(expiryTime)
					exchange.NewBlockHandler(app.ExchangeKeeper).BeginBlocker(ctx)

					msg.ExpirationTimestamp += 2
				})
				It("should return error", func() {
					Expect(resp).To(BeNil())
					expError := sdkerrors.Wrap(types.ErrInvalidExpiry, "cannot change expiration time of an expired market").Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
		})

		When("SettlementTimestamp is wrong", func() {
			BeforeEach(func() {
				msg.ExpirationTimestamp = 0
			})
			It("Settlement timestamp should be > 0", func() {
				msg.SettlementTimestamp = -1
				Expect(msg.ValidateBasic()).To(Equal(types.ErrInvalidSettlement))
			})
			When("Settlement timestamp is in the past", func() {
				BeforeEach(func() {
					msg.SettlementTimestamp = ctx.BlockTime().Unix() - 1
				})
				It("should return error", func() {
					Expect(resp).To(BeNil())
					expError := sdkerrors.Wrapf(types.ErrInvalidSettlement, "SettlementTimestamp %d should be in future", msg.SettlementTimestamp).Error()
					Expect(err.Error()).To(Equal(expError))
				})
			})
		})

		When("SettlementPrice is set", func() {
			It("Settlement price should not be negative (except -1)", func() {
				sp := sdk.NewDec(-100)
				msg.SettlementPrice = &sp
				expError := sdkerrors.Wrap(types.ErrInvalidPrice, msg.SettlementPrice.String()).Error()
				Expect(msg.ValidateBasic().Error()).To(Equal(expError))

				*msg.SettlementPrice = sdk.NewDecWithPrec(-2, 1)
				expError = sdkerrors.Wrap(types.ErrInvalidPrice, msg.SettlementPrice.String()).Error()
				Expect(msg.ValidateBasic().Error()).To(Equal(expError))
			})
			It("Market Status should be set to Demolished when settlement price is set", func() {
				sp := sdk.NewDec(-1)
				msg.SettlementPrice = &sp
				expError := sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", msg.Status.String()).Error()
				Expect(msg.ValidateBasic().Error()).To(Equal(expError))

				*msg.SettlementPrice = sdk.ZeroDec()
				expError = sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", msg.Status.String()).Error()
				Expect(msg.ValidateBasic().Error()).To(Equal(expError))

				*msg.SettlementPrice = sdk.NewDecWithPrec(2, 1)
				expError = sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", msg.Status.String()).Error()
				Expect(msg.ValidateBasic().Error()).To(Equal(expError))

				*msg.SettlementPrice = sdk.OneDec()
				expError = sdkerrors.Wrapf(types.ErrInvalidMarketStatus, "status should be set to demolished when the settlement price is set, status: %s", msg.Status.String()).Error()
				Expect(msg.ValidateBasic().Error()).To(Equal(expError))

				msg.Status = types.MarketStatus_Demolished
				Expect(msg.ValidateBasic()).To(BeNil())
			})
		})

		It("Market status should be either Unspecified or Demolished", func() {
			msg.Status = types.MarketStatus_Demolished
			Expect(msg.ValidateBasic()).To(BeNil())

			msg.Status = types.MarketStatus_Paused
			expError := sdkerrors.Wrap(types.ErrInvalidMarketStatus, msg.Status.String()).Error()
			Expect(msg.ValidateBasic().Error()).To(Equal(expError))

			msg.Status = types.MarketStatus_Active
			expError = sdkerrors.Wrap(types.ErrInvalidMarketStatus, msg.Status.String()).Error()
			Expect(msg.ValidateBasic().Error()).To(Equal(expError))

			msg.Status = types.MarketStatus_Expired
			expError = sdkerrors.Wrap(types.ErrInvalidMarketStatus, msg.Status.String()).Error()
			Expect(msg.ValidateBasic().Error()).To(Equal(expError))
		})
	})

	Describe("Cancel RO orders when better priced order pushes it beyond position size", func() {
		var (
			err                error
			message            *types.MsgCreateBinaryOptionsLimitOrder
			deposit            *types.Deposit
			marketId           common.Hash
			buyerAddress       = testexchange.SampleAccountAddr1
			sellerAddress      = testexchange.SampleAccountAddr2
			subaccountIdBuyer  = testexchange.SampleSubaccountAddr1.String()
			subaccountIdSeller = testexchange.SampleSubaccountAddr2.String()
		)
		getROCount := func(orders []*types.TrimmedDerivativeLimitOrder) int {
			return keeper.Count(orders, func(ord *types.TrimmedDerivativeLimitOrder) bool { return ord.Margin.IsZero() })
		}
		//getVanillaCount := func(orders []*types.TrimmedDerivativeLimitOrder) int {
		//	return keeper.Count(orders, func(ord *types.TrimmedDerivativeLimitOrder) bool { return ord.Margin.IsPositive() })
		//}
		getFirstRO := func(orders []*types.TrimmedDerivativeLimitOrder) *types.TrimmedDerivativeLimitOrder {
			first := keeper.FindFirst(orders, func(ord *types.TrimmedDerivativeLimitOrder) bool { return ord.Margin.IsZero() })
			return first
		}
		printOrders := func(orders []*types.TrimmedDerivativeLimitOrder) {
			sort.SliceStable(orders, func(i, j int) bool {
				return orders[i].Price.LT(orders[j].Price)
			})
			fmt.Println(keeper.GetReadableSlice(orders, "-", func(ord *types.TrimmedDerivativeLimitOrder) string {
				ro := ""
				if ord.Margin.IsZero() {
					ro = " ro"
				}
				return fmt.Sprintf("%v(%v%v)", ord.Price.TruncateInt(), ord.Quantity.TruncateInt(), ro)
			}))
		}
		_ = printOrders

		Context("Position is Short", func() {
			BeforeEach(func() {
				marketId = market.MarketID()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
				testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(6)

				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
				// position is created
			})

			JustBeforeEach(func() {
				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(190, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(180, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(170, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					true,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)
				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(160, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(150, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					true,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(140, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_BUY,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("adding order with best price", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(210, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Last RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(len(subaccountOrders)).To(Equal(7))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(getFirstRO(subaccountOrders).Price.String()).To(Equal(types.GetScaledPrice(sdk.NewDecWithPrec(170, 3), market.OracleScaleFactor).String()))
				})
			})

			Context("Add behind first RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(165, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Last RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(getFirstRO(subaccountOrders).Price.String()).To(Equal(types.GetScaledPrice(sdk.NewDecWithPrec(170, 3), market.OracleScaleFactor).String()))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("Add big position behind first RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(165, 3), market.OracleScaleFactor),
						sdk.NewDec(5),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("First RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(getFirstRO(subaccountOrders).Price.String()).To(Equal(types.GetScaledPrice(sdk.NewDecWithPrec(170, 3), market.OracleScaleFactor).String()))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("Add big position before first RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(185, 3), market.OracleScaleFactor),
						sdk.NewDec(5),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Both RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(len(subaccountOrders)).To(Equal(6))
					Expect(getROCount(subaccountOrders)).To(Equal(0))
				})
			})

			Context("Add behind second RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(145, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("No RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(len(subaccountOrders)).To(Equal(8))
					Expect(getROCount(subaccountOrders)).To(Equal(2))
				})
			})

			Context("Add when there's vanilla same priced as RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(170, 3), market.OracleScaleFactor),
						sdk.NewDec(2),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))

					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(175, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Both RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(0))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("New vanilla order same price as worst RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(150, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_BUY,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Last RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})
		})

		Context("Position is Long", func() {
			BeforeEach(func() {
				marketId = market.MarketID()

				deposit = &types.Deposit{
					AvailableBalance: types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
					TotalBalance:     types.GetScaledPrice(sdk.NewDec(100000), market.OracleScaleFactor),
				}

				testexchange.MintAndDeposit(app, ctx, subaccountIdBuyer, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))
				testexchange.MintAndDeposit(app, ctx, subaccountIdSeller, sdk.NewCoins(sdk.NewCoin(testInput.BinaryMarkets[0].QuoteDenom, deposit.AvailableBalance.TruncateInt())))

				price := types.GetScaledPrice(sdk.NewDecWithPrec(200, 3), market.OracleScaleFactor)
				quantity := sdk.NewDec(6)

				binaryOptionsLimitBuyOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_BUY,
					false,
				)

				_, err2 := msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitBuyOrderMessage)
				testexchange.OrFail(err2)

				binaryOptionsLimitSellOrderMessage := types.NewMsgCreateBinaryOptionsLimitOrder(
					sellerAddress,
					market,
					subaccountIdSeller,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					price,
					quantity,
					types.OrderType_SELL,
					false,
				)

				_, err2 = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), binaryOptionsLimitSellOrderMessage)
				testexchange.OrFail(err2)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())

			})

			JustBeforeEach(func() {
				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(100, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(110, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(120, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(130, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					true,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(140, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(150, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					true,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				message = types.NewMsgCreateBinaryOptionsLimitOrder(
					buyerAddress,
					market,
					subaccountIdBuyer,
					"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
					types.GetScaledPrice(sdk.NewDecWithPrec(160, 3), market.OracleScaleFactor),
					sdk.NewDec(1),
					types.OrderType_SELL,
					false,
				)
				_, err = msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message)
				testexchange.OrFail(err)

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				Expect(app.ExchangeKeeper.IsMetadataInvariantValid(ctx)).To(BeTrue())
			})

			Context("adding order with best price", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(90, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Last RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(getFirstRO(subaccountOrders).Price.String()).To(Equal(types.GetScaledPrice(sdk.NewDecWithPrec(130, 3), market.OracleScaleFactor).String()))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("Add behind first RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(135, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Last RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(getFirstRO(subaccountOrders).Price.String()).To(Equal(types.GetScaledPrice(sdk.NewDecWithPrec(130, 3), market.OracleScaleFactor).String()))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("Add big position behind first RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(135, 3), market.OracleScaleFactor),
						sdk.NewDec(5),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("First RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(getFirstRO(subaccountOrders).Price.String()).To(Equal(types.GetScaledPrice(sdk.NewDecWithPrec(130, 3), market.OracleScaleFactor).String()))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("Add big position before first RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(125, 3), market.OracleScaleFactor),
						sdk.NewDec(5),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Both RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(len(subaccountOrders)).To(Equal(6))
					Expect(getROCount(subaccountOrders)).To(Equal(0))
				})
			})

			Context("Add behind second RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(155, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("No RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(len(subaccountOrders)).To(Equal(8))
					Expect(getROCount(subaccountOrders)).To(Equal(2))
				})
			})

			Context("Add when there's vanilla same priced as RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(130, 3), market.OracleScaleFactor),
						sdk.NewDec(2),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))

					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(125, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Both RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(0))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})

			Context("New vanilla order same price as worst RO order", func() {
				JustBeforeEach(func() {
					message = types.NewMsgCreateBinaryOptionsLimitOrder(
						buyerAddress,
						market,
						subaccountIdBuyer,
						"inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
						types.GetScaledPrice(sdk.NewDecWithPrec(150, 3), market.OracleScaleFactor),
						sdk.NewDec(1),
						types.OrderType_SELL,
						false,
					)
					testexchange.ReturnOrFail(msgServer.CreateBinaryOptionsLimitOrder(sdk.WrapSDKContext(ctx), message))
					ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)
				})

				It("Last RO should be cancelled", func() {
					var subaccountOrders = app.ExchangeKeeper.GetAllTraderDerivativeLimitOrders(ctx, marketId, common.HexToHash(subaccountIdBuyer))
					Expect(getROCount(subaccountOrders)).To(Equal(1))
					Expect(len(subaccountOrders)).To(Equal(7))
				})
			})
		})
	})
})
