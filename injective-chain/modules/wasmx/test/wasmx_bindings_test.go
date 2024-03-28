package testwasmx

import (
	"encoding/json"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/wasmbinding"
	"github.com/InjectiveLabs/injective-core/injective-chain/wasmbinding/bindings"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockMessenger struct {
	shouldBeCalled bool
}

func (m MockMessenger) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg wasmvmtypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error) {
	if !m.shouldBeCalled {
		Expect(true).To(BeFalse(), "wrapped dispatch was called")
	}
	return []sdk.Event{}, [][]byte{[]byte("")}, nil
}

var _ = Describe("Wasmx ", func() {
	var (
		player      te.TestPlayer
		queryPlugin wasmbinding.QueryPlugin
		messenger   wasmkeeper.Messenger
		mock        MockMessenger
	)

	queryContractInfo := func(contractAddr string) (wasmxtypes.QueryContractRegistrationInfoResponse, error) {
		query := bindings.WasmxQuery{
			RegisteredContractInfo: &bindings.RegisteredContractInfo{ContractAddress: contractAddr},
		}

		bz, err := json.Marshal(query)
		te.OrFail(err)

		respBz, err := queryPlugin.HandleWasmxQuery(player.Ctx, bz)
		Expect(err).To(BeNil(), "query handling failed")

		var resp wasmxtypes.QueryContractRegistrationInfoResponse
		err = json.Unmarshal(respBz, &resp)

		return resp, err
	}

	BeforeEach(func() {
		config := te.TestPlayerConfig{NumAccounts: 2, NumSpotMarkets: 1, InitContractRegistry: true}
		player = te.InitTest(config, nil)
		app := player.App
		bk := app.BankKeeper.(bankkeeper.BaseKeeper)
		queryPlugin = *wasmbinding.NewQueryPlugin(&app.AuthzKeeper, &app.ExchangeKeeper, &app.OracleKeeper, &bk, &app.TokenFactoryKeeper, &app.WasmxKeeper, &app.FeeGrantKeeper)
		mock = MockMessenger{shouldBeCalled: true}

		decorator := wasmbinding.CustomMessageDecorator(
			app.MsgServiceRouter(), app.BankKeeper.(bankkeeper.BaseKeeper), &app.ExchangeKeeper, &app.TokenFactoryKeeper)

		messenger = decorator(mock)
	})

	Context("Wasm bindings", func() {
		var contrAddr sdk.AccAddress
		BeforeEach(func() {
			contrAddr = StoreAndRegisterInBBDummyContract(&player, 998, 100_000)
		})

		It("query works", func() {
			contr, err := queryContractInfo(contrAddr.String())
			Expect(err).To(BeNil(), "query returned incorrect response type")
			Expect(contr.Contract.GasPrice).To(Equal(uint64(998)))
		})

		It("Can update contract info", func() {
			updateMsg := wasmxtypes.MsgUpdateContract{
				Sender:          contrAddr.String(),
				ContractAddress: contrAddr.String(),
				GasLimit:        999999,
				GasPrice:        1200,
				AdminAddress:    te.SampleAccountAddrStr1,
			}
			injMsg := bindings.InjectiveMsg{
				WasmxMsg: bindings.WasmxMsg{
					UpdateContractMsg: &updateMsg,
				},
			}

			injMsgMarshalled := te.MustNotErr(json.Marshal(&injMsg))

			wrappedMsg := bindings.InjectiveMsgWrapper{
				Route:   wasmbinding.WasmxRoute,
				MsgData: injMsgMarshalled,
			}

			wrappedMsgMarshalled := te.MustNotErr(json.Marshal(&wrappedMsg))

			msg := wasmvmtypes.CosmosMsg{
				Custom: wrappedMsgMarshalled,
			}
			_, _, err := messenger.DispatchMsg(player.Ctx, contrAddr, "10", msg)
			Expect(err).To(BeNil(), "message handling failed")

			contr, err := queryContractInfo(contrAddr.String())
			Expect(err).To(BeNil(), "query returned incorrect response type")
			Expect(contr.Contract.GasPrice).To(Equal(uint64(1200)))
			Expect(contr.Contract.GasLimit).To(Equal(uint64(999999)))
		})

		It("Can deactivate and reactivate contract", func() {
			deactivateMsg := wasmxtypes.MsgDeactivateContract{
				Sender:          contrAddr.String(),
				ContractAddress: contrAddr.String(),
			}
			injMsg := bindings.InjectiveMsg{
				WasmxMsg: bindings.WasmxMsg{
					DeactivateContractMsg: &deactivateMsg,
				},
			}

			injMsgMarshalled := te.MustNotErr(json.Marshal(&injMsg))

			wrappedMsg := bindings.InjectiveMsgWrapper{
				Route:   wasmbinding.WasmxRoute,
				MsgData: injMsgMarshalled,
			}

			wrappedMsgMarshalled := te.MustNotErr(json.Marshal(&wrappedMsg))

			msg := wasmvmtypes.CosmosMsg{
				Custom: wrappedMsgMarshalled,
			}
			_, _, err := messenger.DispatchMsg(player.Ctx, contrAddr, "10", msg)
			Expect(err).To(BeNil(), "message handling failed")

			contr, err := queryContractInfo(contrAddr.String())
			Expect(err).To(BeNil(), "query returned incorrect response type")
			Expect(contr.Contract.IsExecutable).To(BeFalse())

			// reactivate
			activateMsg := wasmxtypes.MsgActivateContract{
				Sender:          contrAddr.String(),
				ContractAddress: contrAddr.String(),
			}
			injMsg = bindings.InjectiveMsg{
				WasmxMsg: bindings.WasmxMsg{
					ActivateContractMsg: &activateMsg,
				},
			}

			injMsgMarshalled = te.MustNotErr(json.Marshal(&injMsg))
			wrappedMsg = bindings.InjectiveMsgWrapper{
				Route:   wasmbinding.WasmxRoute,
				MsgData: injMsgMarshalled,
			}
			wrappedMsgMarshalled = te.MustNotErr(json.Marshal(&wrappedMsg))
			msg = wasmvmtypes.CosmosMsg{
				Custom: wrappedMsgMarshalled,
			}
			_, _, err = messenger.DispatchMsg(player.Ctx, contrAddr, "10", msg)
			Expect(err).To(BeNil(), "message handling failed")

			contr, err = queryContractInfo(contrAddr.String())
			Expect(err).To(BeNil(), "query returned incorrect response type")
			Expect(contr.Contract.IsExecutable).To(BeTrue())
		})
	})

})
