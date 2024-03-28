package keeper_test

import (
	"time"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
)

var _ = Describe("Oracle Provider Test", func() {
	// state
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context
	)

	// test fixtures
	var (
		timeZero = int64(1600000000)

		provider1Name = "provider1Name" // has 3 relayers
		provider2Name = "provider2Name" // has 1 relayer
		provider3Name = "provider3Name" // this one has no relayers

		rel11Str             = "inj1rgmw7dlgwqpwwf3j8zy4qvg9zkvtgeuy568fff"
		rel11, _             = sdk.AccAddressFromBech32(rel11Str)
		rel12Str             = "inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"
		rel12, _             = sdk.AccAddressFromBech32(rel12Str)
		rel13Str             = "inj13tqdeq5hv9hjr9sz58a42xkww05q6pwf5reey9"
		rel13, _             = sdk.AccAddressFromBech32(rel13Str)
		provider1Relayers    = []sdk.AccAddress{rel11, rel12, rel13}
		provider1RelayersStr = []string{rel11Str, rel12Str, rel13Str}

		rel21Str             = "inj13tqdeq5hv9hjr9sz58a42xkww05q6pwf5reey8"
		rel21, _             = sdk.AccAddressFromBech32(rel21Str)
		provider2RelayersStr = []string{rel21Str}

		unusedRelayerStr = "inj1dqryh824u0w7p6ajk2gsr29tgj6d0nkfwsgs46"

		symbol1      = "symbol1"
		symbol1Price = sdk.NewDec(1)

		symbol2      = "symbol2"
		symbol2Price = sdk.NewDec(0)
	)

	var _ = BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: "3", Time: time.Unix(timeZero, 0)})

		provider1 := types.ProviderInfo{Provider: provider1Name, Relayers: provider1RelayersStr}
		provider2 := types.ProviderInfo{Provider: provider2Name, Relayers: provider2RelayersStr}
		provider3 := types.ProviderInfo{Provider: provider3Name, Relayers: nil}

		_ = app.OracleKeeper.SetProviderInfo(ctx, &provider1)
		_ = app.OracleKeeper.SetProviderInfo(ctx, &provider2)
		_ = app.OracleKeeper.SetProviderInfo(ctx, &provider3)

		price1 := types.ProviderPriceState{Symbol: symbol1, State: &types.PriceState{Price: symbol1Price, Timestamp: 1000}}
		price2 := types.ProviderPriceState{Symbol: symbol2, State: &types.PriceState{Price: symbol2Price, Timestamp: 1200}}

		app.OracleKeeper.SetProviderPriceState(ctx, provider1Name, &price1)
		app.OracleKeeper.SetProviderPriceState(ctx, provider1Name, &price2)
	})

	Describe("Relayers and providers", func() {

		When("Provider doesn't exists", func() {

			It("Check all operations", func() {
				const missingProvider = "xxx"
				result := app.OracleKeeper.IsProviderRelayer(ctx, missingProvider, rel11)
				Expect(result).Should(BeFalse())

				result2 := app.OracleKeeper.GetProviderRelayers(ctx, missingProvider)
				Expect(result2).Should(BeNil())

				result3 := app.OracleKeeper.DeleteProviderRelayers(ctx, missingProvider, []string{rel11.String()})
				Expect(result3).Should(Not(BeNil())) // must throw error

				result4 := app.OracleKeeper.GetProviderInfo(ctx, missingProvider)
				Expect(result4).Should(BeNil())

				result6 := app.OracleKeeper.GetProviderPriceState(ctx, missingProvider, "aaa")
				Expect(result6).Should(BeNil())

				result7 := app.OracleKeeper.GetProviderPriceStates(ctx, missingProvider)
				Expect(result7).Should(BeEmpty())
			})
		})

		When("Performing basic relayer operations", func() {

			It("Check that is relayer of a given provider ", func() {
				result := app.OracleKeeper.IsProviderRelayer(ctx, provider1Name, rel11)
				Expect(result).Should(BeTrue())
			})

			It("Check that is not a relayer of a given provider ", func() {
				result := app.OracleKeeper.IsProviderRelayer(ctx, provider1Name, rel21)
				Expect(result).Should(BeFalse())
			})

			It("Save provider info", func() {
				providerInfo := types.ProviderInfo{
					Provider: "NewProvider",
					Relayers: []string{unusedRelayerStr},
				}
				result := app.OracleKeeper.SetProviderInfo(ctx, &providerInfo)
				Expect(result).Should(BeNil())
			})

			It("Get Provider's info", func() {
				result := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				Expect(result.Provider).Should(Equal(provider1Name))
				Expect(len(result.Relayers)).Should(Equal(3))
				Expect(result.Relayers).Should(Equal(provider1RelayersStr))
			})

			It("Get Provider's relayers", func() {
				result := app.OracleKeeper.GetProviderRelayers(ctx, provider1Name)
				Expect(len(result)).Should(Equal(3))
				Expect(result).Should(Equal(provider1Relayers))
			})

			It("Get all providers info", func() {
				result := app.OracleKeeper.GetAllProviderInfos(ctx)
				Expect(len(result)).Should(Equal(3))
				Expect(result[0].Provider).Should(Equal(provider1Name))
				Expect(result[1].Provider).Should(Equal(provider2Name))
				Expect(result[2].Provider).Should(Equal(provider3Name))
			})

			It("Add a new relayer to a provider", func() {
				providerInfo := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				relayers := providerInfo.Relayers
				relayers = append(relayers, unusedRelayerStr)
				providerInfo.Relayers = relayers
				err := app.OracleKeeper.SetProviderInfo(ctx, providerInfo)
				Expect(err).Should(BeNil())

				// check it's saved correctly
				result := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				Expect(len(result.Relayers)).Should(Equal(4))
				Expect(result.Relayers[3]).Should(Equal(unusedRelayerStr))
			})

			It("Delete a single relayer from a provider", func() {
				err := app.OracleKeeper.DeleteProviderRelayers(ctx, provider1Name, []string{rel11Str})
				Expect(err).Should(BeNil())

				// check it's saved correctly
				result := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				Expect(len(result.Relayers)).Should(Equal(2))
				Expect(result.Relayers).Should(ContainElement(rel12Str))
				Expect(result.Relayers).Should(ContainElement(rel13Str))
			})

			It("Delete two relayers from a provider", func() {
				err := app.OracleKeeper.DeleteProviderRelayers(ctx, provider1Name, []string{rel11Str, rel13Str})
				Expect(err).Should(BeNil())

				// check it's saved correctly
				result := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				Expect(len(result.Relayers)).Should(Equal(1))
				Expect(result.Relayers[0]).Should(Equal(rel12Str))
			})

			It("Delete all relayers from a provider", func() {
				err := app.OracleKeeper.DeleteProviderRelayers(ctx, provider1Name, []string{rel11Str, rel12Str, rel13Str})
				Expect(err).Should(BeNil())

				// check it's saved correctly
				result := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				Expect(len(result.Relayers)).Should(Equal(0))
			})

		}) // When("Performing basic relayer operations")

		When("Performing relayer operations - edge cases", func() {

			It("Overwrite provider info", func() {
				providerInfo := types.ProviderInfo{
					Provider: provider1Name,
					Relayers: []string{unusedRelayerStr},
				}
				err := app.OracleKeeper.SetProviderInfo(ctx, &providerInfo)
				Expect(err).Should(BeNil())

				result := app.OracleKeeper.GetProviderInfo(ctx, provider1Name)
				Expect(len(result.Relayers)).Should(Equal(1))
				Expect(result.Relayers[0]).Should(Equal(unusedRelayerStr))
			})

			It("Save existing relayer to another provider", func() {
				providerInfo := types.ProviderInfo{
					Provider: provider3Name,
					Relayers: []string{rel11Str},
				}
				err := app.OracleKeeper.SetProviderInfo(ctx, &providerInfo)
				Expect(err).Should(Not(BeNil()))
			})

		})

		When("Getting and setting price info from provider", func() {

			It("Reading specific symbol price", func() {
				price1 := app.OracleKeeper.GetProviderPrice(ctx, provider1Name, symbol1)
				Expect(*price1).Should(Equal(sdk.NewDec(1)))

				price2 := app.OracleKeeper.GetProviderPrice(ctx, provider1Name, symbol2)
				Expect(*price2).Should(Equal(sdk.NewDec(0)))
			})

			It("Reading provider price state", func() {
				price1 := app.OracleKeeper.GetProviderPriceState(ctx, provider1Name, symbol1)
				Expect(price1.State.Price).Should(Equal(sdk.NewDec(1)))

				price2 := app.OracleKeeper.GetProviderPriceState(ctx, provider1Name, symbol2)
				Expect(price2.State.Price).Should(Equal(sdk.NewDec(0)))
			})

			It("Reading provider price states", func() {
				result := app.OracleKeeper.GetProviderPriceStates(ctx, provider1Name)
				Expect(len(result)).Should(Equal(2))
				Expect(result[0].State.Price).Should(Equal(sdk.NewDec(1)))
				Expect(result[1].State.Price).Should(Equal(sdk.NewDec(0)))
			})

			It("Reading all providers states", func() {
				result := app.OracleKeeper.GetAllProviderStates(ctx)
				Expect(len(result)).Should(Equal(3))
				provider1State := *result[0]

				Expect(provider1State.ProviderInfo.Provider).Should(Equal(provider1Name))
				Expect(len(provider1State.ProviderInfo.Relayers)).Should(Equal(3))

				Expect(len(provider1State.ProviderPriceStates)).Should(Equal(2))
				Expect(provider1State.ProviderPriceStates[1].State.Price).Should(Equal(sdk.NewDec(0)))

				grpcResponse, err := app.OracleKeeper.OracleProviderPrices(sdk.WrapSDKContext(ctx), &types.QueryOracleProviderPricesRequest{})
				Expect(err).Should(BeNil())
				Expect(len(grpcResponse.GetProviderState())).Should(Equal(3))
			})

			It("Writing price state", func() {
				newSymbol1Price := sdk.NewDec(0)
				newPrice1State := types.ProviderPriceState{Symbol: symbol1, State: &types.PriceState{Price: newSymbol1Price, Timestamp: 1600}}
				app.OracleKeeper.SetProviderPriceState(ctx, provider1Name, &newPrice1State)

				updatedPrice := app.OracleKeeper.GetProviderPrice(ctx, provider1Name, symbol1)
				Expect(*updatedPrice).Should(Equal(sdk.NewDec(0)))
			})

			It("Writing price state for other provider", func() {
				newSymbol1Price := sdk.NewDec(0)
				newPrice1State := types.ProviderPriceState{Symbol: symbol1, State: &types.PriceState{Price: newSymbol1Price, Timestamp: 1600}}
				app.OracleKeeper.SetProviderPriceState(ctx, provider2Name, &newPrice1State)

				updatedPrice := app.OracleKeeper.GetProviderPrice(ctx, provider2Name, symbol1)
				Expect(*updatedPrice).Should(Equal(sdk.NewDec(0)))

				notChangedPrice := app.OracleKeeper.GetProviderPrice(ctx, provider1Name, symbol1)
				Expect(*notChangedPrice).Should(Equal(sdk.NewDec(1)))
			})
		})
	})

})
