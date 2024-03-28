package keeper_test

import (
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/ocr/types"
)

var _ = Describe("OCR report functions test", func() {
	var (
		app *simapp.InjectiveApp
		ctx sdk.Context

		params     types.Params
		testParams = types.Params{
			LinkDenom:           "link",
			PayoutBlockInterval: 1000,
			ModuleAdmin:         sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String(),
		}
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		ctx = app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ChainID: testChainID, Time: time.Unix(1618997040, 0)})
	})

	Describe("params test", func() {
		Context("get params before initialization", func() {
			JustBeforeEach(func() {
				params = app.OcrKeeper.GetParams(ctx)
			})
			It("should be default params", func() {
				Expect(params).To(BeEquivalentTo(types.DefaultParams()))
			})
		})
		Context("params after set", func() {
			JustBeforeEach(func() {
				app.OcrKeeper.SetParams(ctx, testParams)
				params = app.OcrKeeper.GetParams(ctx)
			})
			It("should be new params", func() {
				Expect(params).To(BeEquivalentTo(testParams))
				Expect(app.OcrKeeper.LinkDenom(ctx)).To(BeEquivalentTo(testParams.LinkDenom))
			})
		})
	})
})
