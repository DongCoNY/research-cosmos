package wasmbinding_test

import (
	"encoding/json"
	"fmt"
	"time"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	tfkeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/keeper"
	tftypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/wasmbinding"
	"github.com/InjectiveLabs/injective-core/injective-chain/wasmbinding/bindings"
)

type MockMessanger struct {
	shouldBeCalled bool
}

func (m MockMessanger) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg wasmvmtypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error) {
	if !m.shouldBeCalled {
		Expect(true).To(BeFalse(), "wrapped dispatch was called")
	}
	return []sdk.Event{}, [][]byte{[]byte("")}, nil
}

var _ = Describe("Message plugin tests", func() {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	var (
		app              *simapp.InjectiveApp
		ctx              sdk.Context
		mock             MockMessanger
		messanger        wasmkeeper.Messenger
		subaccountID     string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
		contractAddress  sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID))
		subaccountID_2   string         = "727aee334987c52fa7b567b2662bdbb68614e48c000000000000000000000001"
		someOtherAddress sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID_2))
	)

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
	})

	Context("not a custom message", func() {
		JustBeforeEach(func() {
			mock = MockMessanger{shouldBeCalled: true}
			decorator := wasmbinding.CustomMessageDecorator(
				app.MsgServiceRouter(), app.BankKeeper.(bankkeeper.BaseKeeper), &app.ExchangeKeeper, &app.TokenFactoryKeeper)

			messanger = decorator(mock)
		})

		It("should be dispatched using wrapped messanger", func() {
			msg := wasmvmtypes.CosmosMsg{
				Custom: nil,
			}
			_, _, err := messanger.DispatchMsg(ctx, contractAddress, "10", msg)
			Expect(err).To(BeNil(), "message handling failed")
		})
	})

	Context("custom message", func() {
		JustBeforeEach(func() {
			mock = MockMessanger{shouldBeCalled: false}
			decorator := wasmbinding.CustomMessageDecorator(
				app.MsgServiceRouter(), app.BankKeeper.(bankkeeper.BaseKeeper), &app.ExchangeKeeper, &app.TokenFactoryKeeper)

			messanger = decorator(mock)
		})

		Context("msg is not a json", func() {
			It("should fail", func() {
				msg := wasmvmtypes.CosmosMsg{
					Custom: []byte("ha"),
				}
				_, _, err := messanger.DispatchMsg(ctx, contractAddress, "10", msg)
				Expect(err).To(Not(BeNil()), "message handling succeeded")
				Expect(err.Error()).To(ContainSubstring("Error parsing msg data"), "wrong error returned")
			})
		})

		Context("msg is not a InjectiveMsg", func() {
			It("should fail", func() {
				msg := wasmvmtypes.CosmosMsg{
					Custom: []byte("{\"ha\": 2}"),
				}
				_, _, err := messanger.DispatchMsg(ctx, contractAddress, "10", msg)
				Expect(err).To(Not(BeNil()), "message handling succeeded")
				Expect(err.Error()).To(ContainSubstring("injective msg"), "wrong error returned")
			})
		})

		Context("msg is InjectiveMsg", func() {

			Context("sdk message is nil", func() {

				It("should fail", func() {
					injMsg := bindings.InjectiveMsg{
						ExchangeMsg: bindings.ExchangeMsg{
							CreateSpotLimitOrder: nil,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "exchange",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(Not(BeNil()), "message handling succeeded")
					Expect(err.Error()).To(ContainSubstring("Unknown Injective Wasm Message"), "wrong error returned")
				})
			})

			Context("message fails validate basic", func() {

				var testInput testexchange.TestInput
				JustBeforeEach(func() {
					testInput, ctx = testexchange.SetupTest(app, ctx, 1, 1, 0)
				})
				It("should fail", func() {
					exchangeMsg := testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(15)), sdk.NewDec(int64(100)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID))
					exchangeMsg.Sender = someOtherAddress.String()
					injMsg := bindings.InjectiveMsg{
						ExchangeMsg: bindings.ExchangeMsg{
							CreateSpotLimitOrder: exchangeMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "exchange",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(Not(BeNil()), "message handling succeeded")
					Expect(err.Error()).To(ContainSubstring("subaccount id is not valid"), "wrong error returned")
				})
			})

			Context("message signed by address that is not the contracts", func() {

				var testInput testexchange.TestInput
				JustBeforeEach(func() {
					testInput, ctx = testexchange.SetupTest(app, ctx, 1, 1, 0)
				})
				It("should fail", func() {
					exchangeMsg := testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(15)), sdk.NewDec(int64(100)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID))
					exchangeMsg.Sender = someOtherAddress.String()
					exchangeMsg.Order.OrderInfo.SubaccountId = subaccountID_2
					injMsg := bindings.InjectiveMsg{
						ExchangeMsg: bindings.ExchangeMsg{
							CreateSpotLimitOrder: exchangeMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "exchange",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(Not(BeNil()), "message handling succeeded")
					Expect(err.Error()).To(ContainSubstring("contract doesn't have permission: unauthorized"), "wrong error returned")
				})
			})

			Context("valid exchange message", func() {
				var (
					testInput testexchange.TestInput
					marketID  common.Hash
				)

				JustBeforeEach(func() {
					testInput, ctx = testexchange.SetupTest(app, ctx, 1, 0, 0)

					var err error
					spotMarket, err := app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
					testexchange.OrFail(err)

					marketID = spotMarket.MarketID()

					coinsToCreate := []sdk.Coin{sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.NewInt(10000000)), sdk.NewCoin(testInput.Spots[0].BaseDenom, sdk.NewInt(10000000))}

					for _, coin := range coinsToCreate {
						app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
						app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, contractAddress, sdk.NewCoins(coin))
						app.ExchangeKeeper.IncrementDepositWithCoinOrSendToBank(ctx, common.HexToHash(subaccountID), coin)
						err = app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, sdk.NewCoins(coin))
						testexchange.OrFail(err)
					}
				})
				It("should succeed", func() {
					price := 15
					quantity := 100

					exchangeMsg := testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(price)), sdk.NewDec(int64(quantity)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID))
					injMsg := bindings.InjectiveMsg{
						ExchangeMsg: bindings.ExchangeMsg{
							CreateSpotLimitOrder: exchangeMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "exchange",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(BeNil(), "message handling failed")

					testexchange.EndBlockerAndCommit(app, ctx)
					orders := testexchange.GetAllSpotOrdersSorted(app, ctx, common.HexToHash(subaccountID), marketID)
					testexchange.VerifySpotOrder(orders, 0, float64(quantity), float64(quantity), float64(price), true)
				})
			})

			Context("invalid denom mint message", func() {
				var subdenom string = "bitcoin"
				var fullDenom = fmt.Sprintf("factory/%s/someOther", contractAddress.String())

				JustBeforeEach(func() {
					var err error

					app.TokenFactoryKeeper.SetParams(ctx, tftypes.Params{
						DenomCreationFee: nil,
					})

					msgServer := tfkeeper.NewMsgServerImpl(app.TokenFactoryKeeper)
					msg := tftypes.MsgCreateDenom{
						Sender:   contractAddress.String(),
						Subdenom: subdenom,
					}
					_, err = msgServer.CreateDenom(sdk.WrapSDKContext(ctx), &msg)
					testexchange.OrFail(err)
				})

				It("should fail", func() {
					mintMsg := bindings.MintTokens{
						MintTo: someOtherAddress.String(),
						Amount: sdk.NewCoin(fullDenom, sdk.NewInt(1000)),
					}

					injMsg := bindings.InjectiveMsg{
						TokenFactoryMsg: bindings.TokenFactoryMsg{
							MintTokens: &mintMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "tokenFactory",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(Not(BeNil()), "message handling succeeded")

					gomega.Expect(app.BankKeeper.GetBalance(ctx, someOtherAddress, fullDenom).Amount.String()).To(Equal("0"), "tf denom was created and balance could be fetched")
				})
			})

			Context("empty mint to address in message", func() {
				It("should fail", func() {
					mintMsg := bindings.MintTokens{
						MintTo: "",
						Amount: sdk.NewCoin("someDenom", sdk.NewInt(1000)),
					}

					injMsg := bindings.InjectiveMsg{
						TokenFactoryMsg: bindings.TokenFactoryMsg{
							MintTokens: &mintMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "tokenFactory",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(Not(BeNil()), "message handling succeeded")
				})
			})

			Context("wrong mint to address in message", func() {
				var subdenom string = "bitcoin"
				var fullDenom = fmt.Sprintf("factory/%s/%s", contractAddress.String(), subdenom)

				JustBeforeEach(func() {
					var err error

					app.TokenFactoryKeeper.SetParams(ctx, tftypes.Params{
						DenomCreationFee: nil,
					})

					msgServer := tfkeeper.NewMsgServerImpl(app.TokenFactoryKeeper)
					msg := tftypes.MsgCreateDenom{
						Sender:   contractAddress.String(),
						Subdenom: subdenom,
					}
					_, err = msgServer.CreateDenom(sdk.WrapSDKContext(ctx), &msg)
					testexchange.OrFail(err)
				})
				It("should fail", func() {
					mintMsg := bindings.MintTokens{
						MintTo: "ha ha ha",
						Amount: sdk.NewCoin(fullDenom, sdk.NewInt(1000)),
					}

					injMsg := bindings.InjectiveMsg{
						TokenFactoryMsg: bindings.TokenFactoryMsg{
							MintTokens: &mintMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "tokenFactory",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(Not(BeNil()), "message handling succeeded")
				})
			})

			Context("valid denom mint message", func() {
				var subdenom string = "bitcoin"
				var fullDenom = fmt.Sprintf("factory/%s/%s", contractAddress.String(), subdenom)

				JustBeforeEach(func() {
					var err error

					app.TokenFactoryKeeper.SetParams(ctx, tftypes.Params{
						DenomCreationFee: nil,
					})

					msgServer := tfkeeper.NewMsgServerImpl(app.TokenFactoryKeeper)
					msg := tftypes.MsgCreateDenom{
						Sender:   contractAddress.String(),
						Subdenom: subdenom,
					}
					_, err = msgServer.CreateDenom(sdk.WrapSDKContext(ctx), &msg)
					testexchange.OrFail(err)
				})

				It("should succeed", func() {
					toMint := sdk.NewInt(1000)
					mintMsg := bindings.MintTokens{
						MintTo: someOtherAddress.String(),
						Amount: sdk.NewCoin(fullDenom, toMint),
					}

					injMsg := bindings.InjectiveMsg{
						TokenFactoryMsg: bindings.TokenFactoryMsg{
							MintTokens: &mintMsg,
						},
					}

					injMsgMarshalled, err := json.Marshal(&injMsg)
					testexchange.OrFail(err)

					wrappedMsg := bindings.InjectiveMsgWrapper{
						Route:   "tokenFactory",
						MsgData: injMsgMarshalled,
					}

					wrappedMsgMarshalled, err := json.Marshal(&wrappedMsg)
					testexchange.OrFail(err)

					msg := wasmvmtypes.CosmosMsg{
						Custom: wrappedMsgMarshalled,
					}
					_, _, err = messanger.DispatchMsg(ctx, contractAddress, "10", msg)
					Expect(err).To(BeNil(), "message handling failed")

					balance := app.BankKeeper.GetBalance(ctx, someOtherAddress, fullDenom)
					Expect(balance.Amount.String()).To(Equal(toMint.String()), "tf denom balance was incorrect")
				})
			})
		})
	})
})
