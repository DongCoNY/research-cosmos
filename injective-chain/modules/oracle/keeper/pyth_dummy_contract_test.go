package keeper_test

import (
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	eth "github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pyth state set by dummy contract", func() {
	var (
		player  te.TestPlayer
		priceID = eth.HexToHash("0xf9c0172ba10dfa4d19088d94f5bf61d3b54d5bd7483a322a982e1373ee8ea31b")
	)

	BeforeEach(func() {
		config := te.TestPlayerConfig{NumAccounts: 2, NumSpotMarkets: 1, InitContractRegistry: true}
		player = te.InitTest(config, nil)
	})

	var setupContract = func(price int64, updateParams bool) error {
		pythContr := te.MustNotErr(player.ReplayRegisterAndInitializeContract(&te.ActionRegisterAndInitializeContract{
			ActionStoreContractCode: te.ActionStoreContractCode{
				Path:       "../../exchange/wasm",
				Filename:   "dummy.wasm",
				ContractId: "dummy",
			},
			Message: make(map[string]any, 0),
		}))
		if updateParams {
			player.App.OracleKeeper.SetParams(player.Ctx, oracletypes.Params{PythContract: pythContr})
		}
		player.PerformEndBlockerAction(0, false)

		msg := make(map[string]map[string]any)
		msg["trigger_pyth_update"] = make(map[string]any)
		msg["trigger_pyth_update"]["price"] = price

		return player.ReplayExecuteContract(&te.ActionExecuteContract{
			ContractId:    "dummy",
			ExecutionType: "wasm",
			Message:       msg,
		})
	}

	var verifyPriceState = func(expPrice string) {
		state := player.App.OracleKeeper.GetPythPriceState(player.Ctx, priceID)
		Expect(state).To(Not(BeNil()))
		Expect(len(player.App.OracleKeeper.GetAllPythPriceStates(player.Ctx))).To(BeEquivalentTo(1))

		refPrice := player.App.OracleKeeper.GetPythPrice(player.Ctx, priceID.String(), "USD")
		Expect(refPrice).To(Not(BeNil()))
		Expect(*refPrice).To(BeEquivalentTo(sdk.MustNewDecFromStr(expPrice))) // we set exp -3

		refPrice = player.App.OracleKeeper.GetPythPrice(player.Ctx, priceID.String(), priceID.String())
		Expect(refPrice).To(Not(BeNil()))
		Expect(*refPrice).To(BeEquivalentTo(sdk.MustNewDecFromStr("1")))
	}

	Context("Dummy sets pyth prices", func() {

		BeforeEach(func() {
			te.OrFail(setupContract(123456, true))
		})

		It("price state and price are not nil", func() {
			state := player.App.OracleKeeper.GetPythPriceState(player.Ctx, priceID)
			Expect(state).To(Not(BeNil()))
			Expect(len(player.App.OracleKeeper.GetAllPythPriceStates(player.Ctx))).To(BeEquivalentTo(1))

			refPrice := player.App.OracleKeeper.GetPythPrice(player.Ctx, priceID.String(), "USD")
			Expect(refPrice).To(Not(BeNil()))
			Expect(*refPrice).To(BeEquivalentTo(sdk.MustNewDecFromStr("123.456"))) // we set exp -3

			refPrice = player.App.OracleKeeper.GetPythPrice(player.Ctx, priceID.String(), priceID.String())
			Expect(refPrice).To(Not(BeNil()))
			Expect(*refPrice).To(BeEquivalentTo(sdk.MustNewDecFromStr("1")))
		})
	})

	Context("Dummy sets pyth prices", func() {

		BeforeEach(func() {
			te.OrFail(setupContract(123456, true))
		})

		It("price state and price are not nil", func() {
			verifyPriceState("123.456")
		})

		It("Price can be read by CLI query", func() {
			feedStates := player.App.OracleKeeper.GetAllPriceFeedStates(player.Ctx)
			Expect(feedStates).To(Not(BeNil()))
		})
	})

	Context("Dummy sets max possible price (over 53 bits)", func() {

		BeforeEach(func() {
			te.OrFail(setupContract(9223372036854775807, true))
		})

		It("price state and price are not nil", func() {
			verifyPriceState("9223372036854775.807")
		})
	})

	Context("No pyth contract is authorised", func() {

		It("price state and price are not nil", func() {
			err := setupContract(123456, false)
			Expect(err).NotTo(BeNil())
			Expect(oracletypes.ErrPythContractNotFound.Is(err)).To(BeTrue())
			state := player.App.OracleKeeper.GetPythPriceState(player.Ctx, priceID)
			Expect(state).To(BeNil())
		})
	})

	Context("Another pyth contract is authorised", func() {

		It("price state and price are not nil", func() {
			player.App.OracleKeeper.SetParams(player.Ctx, oracletypes.Params{PythContract: te.SampleAccountAddrStr1})
			err := setupContract(123456, false)
			Expect(err).NotTo(BeNil())
			Expect(oracletypes.ErrUnauthorizedPythPriceRelay.Is(err)).To(BeTrue(), err.Error())
			state := player.App.OracleKeeper.GetPythPriceState(player.Ctx, priceID)
			Expect(state).To(BeNil())
		})
	})

})
