package keeper_test

import (
	"context"
	"encoding/json"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var _ = Describe("MsgServer Tests", func() {
	var (
		app            *simapp.InjectiveApp
		msgServer      types.MsgServer
		ctx            sdk.Context
		goCtx          context.Context
		priceFeedPairs []types.PriceFeedInfo

		// OracleAccAddrs holds the sdk.AccAddresses
		acc1, _        = sdk.AccAddressFromBech32("inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff")
		acc2, _        = sdk.AccAddressFromBech32("inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43")
		acc3, _        = sdk.AccAddressFromBech32("inj13tqdeq5hv9hjr9sz58a42xkww05q6pwf5reey9")
		OracleAccAddrs = []sdk.AccAddress{acc1, acc2, acc3}
	)

	var _ = BeforeSuite(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(1618997040, 0)})
		msgServer = keeper.NewMsgServerImpl(app.OracleKeeper)
		goCtx = sdk.WrapSDKContext(ctx)
		ctx = EndBlockerAndCommit(ctx, 1)

		// init priceFeedPairs
		priceFeedPairs = []types.PriceFeedInfo{
			{Base: "BTC", Quote: "USDT"},
			{Base: "ETH", Quote: "USDT"},
			{Base: "INJ", Quote: "USDT"},
		}

		// set authorized oracles for PriceFeed
		app.OracleKeeper.SetPriceFeedRelayer(ctx, priceFeedPairs[0].Base, priceFeedPairs[0].Quote, OracleAccAddrs[0])
		app.OracleKeeper.SetPriceFeedRelayer(ctx, priceFeedPairs[0].Base, priceFeedPairs[0].Quote, OracleAccAddrs[1])
		app.OracleKeeper.SetPriceFeedRelayer(ctx, priceFeedPairs[1].Base, priceFeedPairs[1].Quote, OracleAccAddrs[0])
		app.OracleKeeper.SetPriceFeedRelayer(ctx, priceFeedPairs[1].Base, priceFeedPairs[1].Quote, OracleAccAddrs[1])
		app.OracleKeeper.SetPriceFeedRelayer(ctx, priceFeedPairs[2].Base, priceFeedPairs[2].Quote, OracleAccAddrs[2])

		ctx = EndBlockerAndCommit(ctx, 5)
		goCtx = sdk.WrapSDKContext(ctx)
	})

	Describe("Cumulative price tests", func() {
		Context("PriceFeed cumulative price", func() {
			msg := &types.MsgRelayPriceFeedPrice{
				Sender: OracleAccAddrs[0].String(),
				Base:   []string{"BTC"},
				Quote:  []string{"USDT"},
				Price:  nil,
			}

			It("Initial price", func() {
				msg.Price = []sdk.Dec{sdk.NewDec(58000)}
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).To(BeNil())

				// assert price
				data := app.OracleKeeper.GetPriceFeedPriceState(ctx, msg.Base[0], msg.Quote[0])
				Expect(data.CumulativePrice).To(BeEquivalentTo(sdk.NewDec(0)))

				// fast forward 5 blocks (5000ms)
				ctx = EndBlockerAndCommit(ctx, 5)
				goCtx = sdk.WrapSDKContext(ctx)
			})

			It("Increasing price after 5 blocks", func() {
				// update price
				msg.Price = []sdk.Dec{sdk.NewDec(59120)}
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).To(BeNil())

				// assert price
				data := app.OracleKeeper.GetPriceFeedPriceState(ctx, msg.Base[0], msg.Quote[0])
				Expect(data.CumulativePrice).To(BeEquivalentTo(sdk.NewDec(290000)))

				// fast forward 8 blocks (8000ms)
				ctx = EndBlockerAndCommit(ctx, 8)
				goCtx = sdk.WrapSDKContext(ctx)
			})

			It("Decreasing price after 8 blocks", func() {
				// update price
				msg.Price = []sdk.Dec{sdk.NewDec(56000)}
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).To(BeNil())

				// assert price
				data := app.OracleKeeper.GetPriceFeedPriceState(ctx, msg.Base[0], msg.Quote[0])
				Expect(data.CumulativePrice).To(BeEquivalentTo(sdk.NewDec(762960)))

				// fast forward 4 blocks (4000ms)
				ctx = EndBlockerAndCommit(ctx, 4)
				goCtx = sdk.WrapSDKContext(ctx)
			})

			It("Same price after 4 blocks", func() {
				// update price
				msg.Price = []sdk.Dec{sdk.NewDec(56000)}
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).To(BeNil())

				// assert price
				data := app.OracleKeeper.GetPriceFeedPriceState(ctx, msg.Base[0], msg.Quote[0])
				Expect(data.CumulativePrice).To(BeEquivalentTo(sdk.NewDec(986960)))
			})
		})
	})

	Describe("PriceFeed msgServer tests", func() {
		Context("Push price with unauthorized oracle", func() {
			It("Should fail", func() {
				msg := &types.MsgRelayPriceFeedPrice{
					Sender: OracleAccAddrs[2].String(),
					Base:   []string{"BTC"},
					Quote:  []string{"USDT"},
					Price:  []sdk.Dec{sdk.NewDec(58000)},
				}
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).ToNot(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
		})

		Context("Push price with authorized oracle", func() {
			var msg1 = &types.MsgRelayPriceFeedPrice{
				Sender: OracleAccAddrs[0].String(),
				Base:   []string{"BTC"},
				Quote:  []string{"USDT"},
				Price:  []sdk.Dec{sdk.NewDec(58000)},
			}
			var msg2 = &types.MsgRelayPriceFeedPrice{
				Sender: OracleAccAddrs[2].String(),
				Base:   []string{"INJ"},
				Quote:  []string{"USDT"},
				Price:  []sdk.Dec{sdk.NewDec(24)},
			}

			It("Should pass", func() {
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg1)
				Expect(err).To(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
			It("Should pass", func() {
				msg1.Sender = OracleAccAddrs[1].String()
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg1)
				Expect(err).To(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
			It("Should pass", func() {
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg2)
				Expect(err).To(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
		})

		Context("Push multiple prices with partially authorized oracle", func() {
			It("Should fail", func() {
				msg := &types.MsgRelayPriceFeedPrice{
					Sender: OracleAccAddrs[2].String(),
					Base:   []string{"ETH", "INJ"},
					Quote:  []string{"USDT", "USDT"},
					Price:  []sdk.Dec{sdk.NewDec(3400), sdk.NewDec(24)},
				}
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).NotTo(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
		})

		Context("Push multiple prices with authorized oracle", func() {
			msg := &types.MsgRelayPriceFeedPrice{
				Sender: OracleAccAddrs[0].String(),
				Base:   []string{"BTC", "ETH"},
				Quote:  []string{"USDT", "USDT"},
				Price:  []sdk.Dec{sdk.NewDec(58000), sdk.NewDec(3400)},
			}

			It("Should pass", func() {
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).To(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})

			It("Should pass", func() {
				msg.Sender = OracleAccAddrs[1].String()
				_, err := msgServer.RelayPriceFeedPrice(goCtx, msg)
				Expect(err).To(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
		})

		Context("Assert PriceFeed state", func() {
			var exportedStateJSON = []byte(`[{"base":"INJ","quote":"USDT","price_state":{"price":"24.000000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064},"relayers":["inj13tqdeq5hv9hjr9sz58a42xkww05q6pwf5reey9"]},{"base":"ETH","quote":"USDT","price_state":{"price":"3400.000000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064},"relayers":["inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff","inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"]},{"base":"BTC","quote":"USDT","price_state":{"price":"58000.000000000000000000","cumulative_price":"986960.000000000000000000","timestamp":1618997064},"relayers":["inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff","inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"]}]`)

			It("Should pass", func() {
				// get current state
				state := app.OracleKeeper.GetAllPriceFeedStates(ctx)
				Expect(state).NotTo(BeNil())

				// parse exported state
				var exportedState []types.PriceFeedState
				err := json.Unmarshal(exportedStateJSON, &exportedState)
				Expect(err).To(BeNil())
				for i, pair := range state {
					exportedState[i].PriceState.Timestamp = pair.PriceState.Timestamp
				}

				// assert
				stateBytes, _ := json.Marshal(state)
				exportedStateBytes, _ := json.Marshal(exportedState)
				Expect(stateBytes).To(BeEquivalentTo(exportedStateBytes))
			})
		})
	})

	Describe("Coinbase msgServer tests", func() {
		Context("Push prices with valid signatures", func() {
			It("Should pass", func() {
				msg := &types.MsgRelayCoinbaseMessages{
					Sender: OracleAccAddrs[0].String(),
					Messages: [][]byte{
						common.FromHex("0x000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000607fe06c00000000000000000000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000000000000cdd578cf00000000000000000000000000000000000000000000000000000000000000006707269636573000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034254430000000000000000000000000000000000000000000000000000000000"),
						common.FromHex("0x000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000607fee4000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000891e9d880000000000000000000000000000000000000000000000000000000000000006707269636573000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034554480000000000000000000000000000000000000000000000000000000000"),
						common.FromHex("0x000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000607fef3000000000000000000000000000000000000000000000000000000000000000c0000000000000000000000000000000000000000000000000000000000056facc00000000000000000000000000000000000000000000000000000000000000067072696365730000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000358545a0000000000000000000000000000000000000000000000000000000000"),
					},
					Signatures: [][]byte{
						common.FromHex("0x755d64ab12b52711b6ed6cea26b4005fe44884546bc6fbcb0ca31fd369e90a6f856cd792fb473603af598cb9946d3a5ceb627b26074b0294dcefd8d0d8f171d9000000000000000000000000000000000000000000000000000000000000001c"),
						common.FromHex("0x18a821b64b1a100cc1ff68c5b2ba2fa40de6f7abeb49981366b359af9d9f131e0db75d82358cf4e5850c38bff62d626034464740ba5e222c3aeeb05ea51c59f3000000000000000000000000000000000000000000000000000000000000001b"),
						common.FromHex("0x946c8037ce20231cdde2bb30cea45f4a2f60916d4e3a28d6e9ee82ff6a83d6fcb44073ed9561bb8b0f54e6256234e50770eded2042582c81a99e78581873759a000000000000000000000000000000000000000000000000000000000000001c"),
					},
				}

				_, err := msgServer.RelayCoinbaseMessages(goCtx, msg)

				Expect(err).To(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
		})

		Context("Push prices with partially invalid signatures", func() {
			It("Should fail", func() {
				msg := &types.MsgRelayCoinbaseMessages{
					Sender: OracleAccAddrs[0].String(),
					Messages: [][]byte{
						common.FromHex("0x0a0000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000607fee4000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000891e9d880000000000000000000000000000000000000000000000000000000000000006707269636573000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034554480000000000000000000000000000000000000000000000000000000000"),
					},
					Signatures: [][]byte{
						common.FromHex("0x18a821b64b1a100cc1ff68c5b2ba2fa40de6f7abeb49981366b359af9d9f131e0db75d82358cf4e5850c38bff62d626034464740ba5e222c3aeeb05ea51c59f3000000000000000000000000000000000000000000000000000000000000001b"),
					},
				}

				_, err := msgServer.RelayCoinbaseMessages(goCtx, msg)

				Expect(err).NotTo(BeNil())
				ctx = EndBlockerAndCommit(ctx, 1)
			})
		})

		Context("Assert Coinbase state", func() {
			var exportedStateJSON = []byte(`[{"kind":"prices","timestamp":1618993260,"key":"BTC","value":55253110000,"price_state":{"price":"55253.110000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997040}},{"kind":"prices","timestamp":1618996800,"key":"ETH","value":2300485000,"price_state":{"price":"2300.485000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997040}},{"kind":"prices","timestamp":1618997040,"key":"XTZ","value":5700300,"price_state":{"price":"5.700300000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997040}}]`)

			It("Should pass", func() {
				// get current state
				state := app.OracleKeeper.GetAllCoinbasePriceStates(ctx)
				Expect(state).NotTo(BeNil())

				// parse exported state
				var exportedState []types.CoinbasePriceState
				err := json.Unmarshal(exportedStateJSON, &exportedState)
				Expect(err).To(BeNil())
				for i, pair := range state {
					exportedState[i].PriceState.Timestamp = pair.PriceState.Timestamp
				}

				// assert
				stateBytes, _ := json.Marshal(state)
				exportedStateBytes, _ := json.Marshal(exportedState)
				Expect(stateBytes).To(BeEquivalentTo(exportedStateBytes))
			})
		})
	})

	Describe("Module genesis tests", func() {
		var exportedStateJSON = []byte(`{"params":{},"price_feed_price_states":[{"base":"INJ","quote":"USDT","price_state":{"price":"24.000000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064},"relayers":["inj13tqdeq5hv9hjr9sz58a42xkww05q6pwf5reey9"]},{"base":"ETH","quote":"USDT","price_state":{"price":"3400.000000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064},"relayers":["inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff","inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"]},{"base":"BTC","quote":"USDT","price_state":{"price":"58000.000000000000000000","cumulative_price":"986960.000000000000000000","timestamp":1618997064},"relayers":["inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff","inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"]}],"coinbase_price_states":[{"kind":"prices","timestamp":1618993260,"key":"BTC","value":55253110000,"price_state":{"price":"55253.110000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064}},{"kind":"prices","timestamp":1618996800,"key":"ETH","value":2300485000,"price_state":{"price":"2300.485000000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064}},{"kind":"prices","timestamp":1618997040,"key":"XTZ","value":5700300,"price_state":{"price":"5.700300000000000000","cumulative_price":"0.000000000000000000","timestamp":1618997064}}],"band_ibc_params":{"ibc_request_interval":7,"ibc_version":"bandchain-1","ibc_port_id":"oracle"},"historical_price_records":[{"oracle":3,"symbol_id":"BTC","latest_price_records":[{"timestamp":1618997064,"price":"55253.110000000000000000"}]},{"oracle":3,"symbol_id":"ETH","latest_price_records":[{"timestamp":1618997064,"price":"2300.485000000000000000"}]},{"oracle":3,"symbol_id":"XTZ","latest_price_records":[{"timestamp":1618997064,"price":"5.700300000000000000"}]},{"oracle":2,"symbol_id":"BTC/USDT","latest_price_records":[{"timestamp":1618997047,"price":"58000.000000000000000000"},{"timestamp":1618997052,"price":"59120.000000000000000000"},{"timestamp":1618997060,"price":"56000.000000000000000000"},{"timestamp":1618997064,"price":"58000.000000000000000000"}]},{"oracle":2,"symbol_id":"ETH/USDT","latest_price_records":[{"timestamp":1618997064,"price":"3400.000000000000000000"}]},{"oracle":2,"symbol_id":"INJ/USDT","latest_price_records":[{"timestamp":1618997064,"price":"24.000000000000000000"}]}]}`)

		Context("Assert module state", func() {
			It("Should pass", func() {
				// get current state
				state := app.OracleKeeper.ExportGenesis(ctx)
				Expect(state).NotTo(BeNil())

				// parse exported state
				var exportedState types.GenesisState
				err := json.Unmarshal(exportedStateJSON, &exportedState)
				Expect(err).To(BeNil())
				for i, pair := range state.PriceFeedPriceStates {
					exportedState.PriceFeedPriceStates[i].PriceState.Timestamp = pair.PriceState.Timestamp
				}
				for i, pair := range state.CoinbasePriceStates {
					exportedState.CoinbasePriceStates[i].PriceState.Timestamp = pair.PriceState.Timestamp
				}

				// assert
				stateBytes, _ := json.Marshal(state)
				exportedStateBytes, _ := json.Marshal(exportedState)
				Expect(stateBytes).To(BeEquivalentTo(exportedStateBytes))
			})
		})

		Context("Re-import module state", func() {
			It("Should pass", func() {
				// export
				exportedState := app.OracleKeeper.ExportGenesis(ctx)

				// re-import
				app.OracleKeeper.InitGenesis(ctx, *exportedState)

				// export again and assert
				state := app.OracleKeeper.ExportGenesis(ctx)
				stateBytes, _ := json.Marshal(state)
				exportedStateBytes, _ := json.Marshal(exportedState)
				Expect(stateBytes).To(BeEquivalentTo(string(exportedStateBytes)))
			})
		})
	})
})
