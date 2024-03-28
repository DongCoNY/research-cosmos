package keeper_test

import (
	"encoding/json"
	"fmt"
	te "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	tokenfactorytypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("CW20 Adapter", func() {
	var (
		simulationError error
		hooks           map[string]te.TestPlayerHook
		tp              te.TestPlayer
	)
	BeforeEach(func() {
		hooks = make(map[string]te.TestPlayerHook)
	})

	var runTest = func(file string, shouldNotFail bool) {
		filepath := fmt.Sprintf("%v/%v.json", "./scenarios/cw20_adapter", file)
		tp = te.LoadReplayableTest(filepath)
		simulationError = tp.ReplayTest(te.DefaultFlags, &hooks)
		if shouldNotFail {
			Expect(simulationError).To(BeNil())
		}
	}

	var queryRegisteredContracts = func(address sdk.AccAddress) (*[]string, error) {
		type QueryRegisteredContracts struct{}

		type QueryData struct {
			Data QueryRegisteredContracts `json:"registered_contracts"`
		}
		queryData := QueryData{
			Data: QueryRegisteredContracts{},
		}
		queryDataBz, err := json.Marshal(queryData)
		if err != nil {
			return nil, err
		}
		bz, err := tp.App.WasmKeeper.QuerySmart(tp.Ctx, address, queryDataBz)
		if err != nil {
			return nil, err
		}
		var result []string
		if err := json.Unmarshal(bz, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}

	Context("performing basic operations", func() {
		When("cw20 token is preregistered", func() {
			denom := ""
			BeforeEach(func() {
				hooks["setup"] = func(params *te.TestPlayerHookParams) {
					tp.App.TokenFactoryKeeper.SetParams(tp.Ctx, tokenfactorytypes.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("inj", te.MustOk(sdk.NewIntFromString("10"))))})
				}
				hooks["after_send_cw20"] = func(params *te.TestPlayerHookParams) {
					denom = fmt.Sprintf("factory/%v/%v", tp.ContractsById["cw20adapter"], tp.ContractsById["cw20b"])
					tfBalance := tp.GetBankBalance(1, denom).MustFloat64()
					Expect(tfBalance).To(Equal(100.0))

					cw20Balance := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[1].AccAddress.String())).Int64()
					Expect(cw20Balance).To(Equal(int64(900)))
				}
				hooks["after_redeem"] = func(params *te.TestPlayerHookParams) {
					denom = fmt.Sprintf("factory/%v/%v", tp.ContractsById["cw20adapter"], tp.ContractsById["cw20b"])
					tfBalance := tp.GetBankBalance(1, denom).MustFloat64()
					Expect(tfBalance).To(Equal(25.0))

					cw20Balance := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[1].AccAddress.String())).Int64()
					Expect(cw20Balance).To(Equal(int64(975)))
				}
			})
			It("token factory tokens are minted", func() {
				runTest("cw20adapter_01_preregistered", true)
				contracts := *te.MustNotErr(queryRegisteredContracts(te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20adapter"]))))
				Expect(len(contracts)).To(Equal(1))
			})
		})

		When("cw20 token is lazily registered", func() {
			denom := ""
			BeforeEach(func() {
				hooks["setup"] = func(params *te.TestPlayerHookParams) {
					tp.App.TokenFactoryKeeper.SetParams(tp.Ctx, tokenfactorytypes.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("inj", te.MustOk(sdk.NewIntFromString("10"))))})
				}
				hooks["after_send_cw20"] = func(params *te.TestPlayerHookParams) {
					denom = fmt.Sprintf("factory/%v/%v", tp.ContractsById["cw20adapter"], tp.ContractsById["cw20b"])
					tfBalance := tp.GetBankBalance(1, denom).MustFloat64()
					Expect(tfBalance).To(Equal(100.0))

					cw20Balance := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[1].AccAddress.String())).Int64()
					Expect(cw20Balance).To(Equal(int64(900)))
				}
				hooks["after_redeem"] = func(params *te.TestPlayerHookParams) {
					denom = fmt.Sprintf("factory/%v/%v", tp.ContractsById["cw20adapter"], tp.ContractsById["cw20b"])
					tfBalance := tp.GetBankBalance(1, denom).MustFloat64()
					Expect(tfBalance).To(Equal(25.0))

					cw20Balance := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[1].AccAddress.String())).Int64()
					Expect(cw20Balance).To(Equal(int64(975)))
				}
			})
			It("token factory tokens are minted and redeemed", func() {
				runTest("cw20adapter_02_lazy", true)
				contracts := *te.MustNotErr(queryRegisteredContracts(te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20adapter"]))))
				Expect(len(contracts)).To(Equal(1))
			})
		})

		When("cw20 token is redeemed by send to other contract", func() {
			It("cw20 tokens are moved to that other contract", func() {
				runTest("cw20adapter_ok_lazy_redeem_send", true)
				contracts := *te.MustNotErr(queryRegisteredContracts(te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20adapter"]))))
				Expect(len(contracts)).To(Equal(1))

				contracts2 := *te.MustNotErr(queryRegisteredContracts(te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20adapter2"]))))
				Expect(len(contracts2)).To(Equal(1))

				cw20Balance := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[1].AccAddress.String())).Int64()
				Expect(cw20Balance).To(Equal(int64(900)))

				cw20BalanceAd1 := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[4].AccAddress.String())).Int64()
				Expect(cw20BalanceAd1).To(Equal(int64(25)))

				cw20BalanceAd2 := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[5].AccAddress.String())).Int64()
				Expect(cw20BalanceAd2).To(Equal(int64(75)))
			})
		})

		When("contract metadata is updated", func() {
			It("it's updated", func() {
				runTest("cw20adapter_ok_metadata", true)
				denom := fmt.Sprintf("factory/%v/%v", tp.ContractsById["cw20adapter"], tp.ContractsById["cw20b"])
				metadata := te.MustOk(tp.App.BankKeeper.GetDenomMetaData(tp.Ctx, denom))
				Expect(metadata.Name).To(Equal("Solana"))
				Expect(metadata.Symbol).To(Equal("SOL"))
				Expect(len(metadata.DenomUnits)).To(Equal(2))
				Expect(int(metadata.DenomUnits[1].Exponent)).To(Equal(6))
			})
		})
	})

	Context("performing failed operations", func() {
		When("no funds to create new denom", func() {
			BeforeEach(func() {
				hooks["setup"] = func(params *te.TestPlayerHookParams) {
					tp.App.TokenFactoryKeeper.SetParams(tp.Ctx, tokenfactorytypes.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("inj", te.MustOk(sdk.NewIntFromString("10"))))})
				}
				runTest("cw20adapter_03_lazy_no_funds", false)
			})
			It("cw20 funds should be reverted", func() {
				denom := fmt.Sprintf("factory/%v/%v", tp.ContractsById["cw20adapter"], tp.ContractsById["cw20b"])
				tfBalance := tp.GetBankBalance(1, denom).MustFloat64()
				Expect(tfBalance).To(Equal(0.0))

				cw20Balance := te.MustNotErr(tp.App.ExchangeKeeper.QueryTokenBalance(tp.Ctx, te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20b"])), (*tp.Accounts)[1].AccAddress.String())).Int64()
				Expect(cw20Balance).To(Equal(int64(1000)))
			})
		})

		When("trying to register non-cw20 token", func() {
			BeforeEach(func() {
				hooks["setup"] = func(params *te.TestPlayerHookParams) {
					tp.App.TokenFactoryKeeper.SetParams(tp.Ctx, tokenfactorytypes.Params{DenomCreationFee: sdk.NewCoins(sdk.NewCoin("inj", te.MustOk(sdk.NewIntFromString("10"))))})
				}
				hooks["register"] = func(params *te.TestPlayerHookParams) {
					Expect(params.Error).ToNot(BeNil())
					Expect(strings.Contains(params.Error.Error(), "Address is not cw-20 contract")).To(BeTrue())
				}
			})
			It("address is not registered", func() {
				runTest("cw20adapter_04_non_cw20_register", false)
				contracts := *te.MustNotErr(queryRegisteredContracts(te.MustNotErr(sdk.AccAddressFromBech32(tp.ContractsById["cw20adapter"]))))
				Expect(len(contracts)).To(Equal(0))
			})
		})
	})
})
