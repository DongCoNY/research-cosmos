package keeper_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	v040auth "github.com/cosmos/cosmos-sdk/x/auth/migrations/v1"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/testpeggy"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"
	tmtypes "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

func TestPrefixRange(t *testing.T) {
	cases := map[string]struct {
		src      []byte
		expStart keeper.PrefixStart
		expEnd   keeper.PrefixEnd
		expPanic bool
	}{
		"normal":                 {src: []byte{1, 3, 4}, expStart: keeper.PrefixStart{1, 3, 4}, expEnd: keeper.PrefixEnd{1, 3, 5}},
		"normal short":           {src: []byte{79}, expStart: keeper.PrefixStart{79}, expEnd: keeper.PrefixEnd{80}},
		"empty case":             {src: []byte{}},
		"roll-over example 1":    {src: []byte{17, 28, 255}, expStart: keeper.PrefixStart{17, 28, 255}, expEnd: keeper.PrefixEnd{17, 29, 0}},
		"roll-over example 2":    {src: []byte{15, 42, 255, 255}, expStart: keeper.PrefixStart{15, 42, 255, 255}, expEnd: keeper.PrefixEnd{15, 43, 0, 0}},
		"pathological roll-over": {src: []byte{255, 255, 255, 255}, expStart: keeper.PrefixStart{255, 255, 255, 255}},
		"nil prohibited":         {expPanic: true},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			if tc.expPanic {
				require.Panics(t, func() {
					keeper.PrefixRange(tc.src)
				})
				return
			}
			start, end := keeper.PrefixRange(tc.src)
			assert.Equal(t, tc.expStart, start)
			assert.Equal(t, tc.expEnd, end)
		})
	}
}

func TestCurrentValsetNormalization(t *testing.T) {
	specs := map[string]struct {
		srcPowers []uint64
		expPowers []uint64
	}{
		"one": {
			srcPowers: []uint64{100},
			expPowers: []uint64{4294967295},
		},
		"two": {
			srcPowers: []uint64{100, 1},
			expPowers: []uint64{4252442866, 42524428},
		},
	}
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	for msg, spec := range specs {
		spec := spec
		t.Run(msg, func(t *testing.T) {
			operators := make([]testpeggy.MockStakingValidatorData, len(spec.srcPowers))
			for i, v := range spec.srcPowers {
				cAddr := bytes.Repeat([]byte{byte(i)}, v040auth.AddrLen)
				operators[i] = testpeggy.MockStakingValidatorData{
					// any unique addr
					Operator: cAddr,
					Power:    int64(v),
				}
				input.PeggyKeeper.SetEthAddressForValidator(ctx, cAddr, common.HexToAddress("0xf71402f886b45c134743F4c00750823Bbf5Fd045"))
			}
			input.PeggyKeeper.StakingKeeper = testpeggy.NewStakingKeeperWeightedMock(operators...)
			r := input.PeggyKeeper.GetCurrentValset(ctx)
			assert.Equal(t, spec.expPowers, types.BridgeValidators(r.Members).GetPowers())
		})
	}
}

func TestAttestationIterator(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	// add some attestations to the store

	att1 := &types.Attestation{
		Observed: true,
		Votes:    []string{},
	}
	dep1 := &types.MsgDepositClaim{
		EventNonce:     1,
		TokenContract:  testpeggy.TokenContractAddrs[0],
		Amount:         sdk.NewInt(100),
		EthereumSender: testpeggy.EthAddrs[0].String(),
		CosmosReceiver: testpeggy.AccAddrs[0].String(),
		Orchestrator:   testpeggy.AccAddrs[0].String(),
	}
	att2 := &types.Attestation{
		Observed: true,
		Votes:    []string{},
	}
	dep2 := &types.MsgDepositClaim{
		EventNonce:     2,
		TokenContract:  testpeggy.TokenContractAddrs[0],
		Amount:         sdk.NewInt(100),
		EthereumSender: testpeggy.EthAddrs[0].String(),
		CosmosReceiver: testpeggy.AccAddrs[0].String(),
		Orchestrator:   testpeggy.AccAddrs[0].String(),
	}
	input.PeggyKeeper.SetAttestation(ctx, dep1.EventNonce, dep1.ClaimHash(), att1)
	input.PeggyKeeper.SetAttestation(ctx, dep2.EventNonce, dep2.ClaimHash(), att2)

	attestations := []*types.Attestation{}
	input.PeggyKeeper.IterateAttestations(ctx, func(_ []byte, attestation *types.Attestation) (stop bool) {
		attestations = append(attestations, attestation)
		return false
	})

	require.Len(t, attestations, 2)
}

func TestOrchestratorAddresses(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	ctx := input.Context
	k := input.PeggyKeeper

	var ethAddrs = []string{"0x8D1E749f2cC3b4d345Fda1beA633413505477bc1"}
	var valAddrs = []string{"injvaloper1p5w2pquxj0plswe4jwczxj86tzuupqpydg3n64"}
	var orchAddrs = []string{"inj1qwylyvtejkxxcagcx2scw206u050xzem3qu6w8"}

	for i := range ethAddrs {
		// set some addresses
		val, err1 := sdk.ValAddressFromBech32(valAddrs[i])
		orch, err2 := sdk.AccAddressFromBech32(orchAddrs[i])
		require.NoError(t, err1)
		require.NoError(t, err2)
		// set the orchestrator address
		k.SetOrchestratorValidator(ctx, val, orch)
		// set the ethereum address
		k.SetEthAddressForValidator(ctx, val, common.HexToAddress(ethAddrs[i]))
	}

	addresses := k.GetOrchestratorAddresses(ctx)
	for i := range addresses {
		res := addresses[i]
		validatorAddr, _ := sdk.ValAddressFromBech32(valAddrs[i])
		validatorAccountAddr := sdk.AccAddress(validatorAddr.Bytes()).String()
		assert.Equal(t, validatorAccountAddr, res.Sender)
		assert.Equal(t, orchAddrs[i], res.Orchestrator)
		assert.Equal(t, ethAddrs[i], res.EthAddress)
	}

}

func TestLastSlashedValsetNonce(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	k := input.PeggyKeeper
	ctx := input.Context

	vs := k.GetCurrentValset(ctx)

	i := 1
	for ; i < 10; i++ {
		vs.Height = uint64(i)
		vs.Nonce = uint64(i)
		k.StoreValsetUnsafe(ctx, vs)
	}

	latestValsetNonce := k.GetLatestValsetNonce(ctx)
	assert.Equal(t, latestValsetNonce, uint64(i-1))

	//  lastSlashedValsetNonce should be zero initially.
	lastSlashedValsetNonce := k.GetLastSlashedValsetNonce(ctx)
	assert.Equal(t, lastSlashedValsetNonce, uint64(0))
	unslashedValsets := k.GetUnslashedValsets(ctx, uint64(12))
	assert.Equal(t, len(unslashedValsets), 9)

	// check if last Slashed Valset nonce is set properly or not
	k.SetLastSlashedValsetNonce(ctx, uint64(3))
	lastSlashedValsetNonce = k.GetLastSlashedValsetNonce(ctx)
	assert.Equal(t, lastSlashedValsetNonce, uint64(3))

	// when maxHeight < lastSlashedValsetNonce, len(unslashedValsets) should be zero
	unslashedValsets = k.GetUnslashedValsets(ctx, uint64(2))
	assert.Equal(t, len(unslashedValsets), 0)

	// when maxHeight == lastSlashedValsetNonce, len(unslashedValsets) should be zero
	unslashedValsets = k.GetUnslashedValsets(ctx, uint64(3))
	assert.Equal(t, len(unslashedValsets), 0)

	// when maxHeight > lastSlashedValsetNonce && maxHeight <= latestValsetNonce
	unslashedValsets = k.GetUnslashedValsets(ctx, uint64(6))
	assert.Equal(t, len(unslashedValsets), 2)

	// when maxHeight > latestValsetNonce
	unslashedValsets = k.GetUnslashedValsets(ctx, uint64(15))
	assert.Equal(t, len(unslashedValsets), 6)
	fmt.Println("unslashedValsetsRange", unslashedValsets)
}

func TestPeggyBlacklist(t *testing.T) {
	input := testpeggy.CreateTestEnv(t)
	k := input.PeggyKeeper
	ctx := input.Context

	peggyBlacklistAddresses := []string{
		"0xaa05f7c7eb9af63d6cc03c36c4f4ef6c37431ee0",
		"0x7f367cc41522ce07553e823bf3be79a889debe1b",
		"0x1da5821544e25c636c1417ba96ade4cf6d2f9b5a",
		"0x7db418b5d567a4e0e8c59ad71be1fce48f3e6107",
		"0x72a5843cc08275c8171e582972aa4fda8c397b2a",
		"0x7f19720a857f834887fc9a7bc0a0fbe7fc7f8102",
		"0xd882cfc20f52f2599d84b8e8d58c7fb62cfe344b",
		"0x9f4cda013e354b8fc285bf4b9a60460cee7f7ea9",
		"0x308ed4b7b49797e1a98d3818bff6fe5385410370",
		"0xe7aa314c77f4233c18c6cc84384a9247c0cf367b",
		"0x19aa5fe80d33a56d56c78e82ea5e50e5d80b4dff",
		"0x2f389ce8bd8ff92de3402ffce4691d17fc4f6535",
		"0xc455f7fd3e0e12afd51fba5c106909934d8a0e4a",
		"0x48549a34ae37b12f6a30566245176994e17c6b4a",
		"0x5512d943ed1f7c8a43f3435c85f7ab68b30121b0",
		"0xa7e5d5a720f06526557c513402f2e6b5fa20b008",
		"0x3cbded43efdaf0fc77b9c55f6fc9988fcc9b757d",
		"0x67d40ee1a85bf4a4bb7ffae16de985e8427b6b45",
		"0x6f1ca141a28907f78ebaa64fb83a9088b02a8352",
		"0x6acdfba02d390b97ac2b2d42a63e85293bcc160e",
		"0x35663b9a8e4563eefdf852018548b4947b20fce6",
		"0xfae5a6d3bd9bd24a3ed2f2a8a6031c83976c19a2",
		"0x5eb95f30bd4409cfaadeba75cd8d9c2ce4ed992a",
		"0x029c2c986222dca39843bf420a28646c25d55b6d",
		"0x461270bd08dfa98edec980345fd56d578a2d8f49",
		"0xfec8a60023265364d066a1212fde3930f6ae8da7",
		"0x8576acc5c05d6ce88f4e49bf65bdf0c62f91353c",
		"0x901bb9583b24d97e995513c6778dc6888ab6870e",
		"0x7ff9cfad3877f21d41da833e2f775db0569ee3d9",
		"0x098b716b8aaf21512996dc57eb0615e2383e2f96",
		"0xa0e1c89ef1a489c9c7de96311ed5ce5d32c20e4b",
		"0x3cffd56b47b7b41c56258d9c7731abadc360e073",
		"0x53b6936513e738f44fb50d2b9476730c0ab3bfc1",
		"0xcce63fd31e9053c110c74cebc37c8e358a6aa5bd",
		"0x3e37627deaa754090fbfbb8bd226c1ce66d255e9",
		"0x35fb6f6db4fb05e6a4ce86f2c93691425626d4b1",
		"0xf7b31119c2682c88d88d455dbb9d5932c65cf1be",
		"0x08723392ed15743cc38513c4925f5e6be5c17243",
		"0x29875bd49350ac3f2ca5ceeb1c1701708c795ff3",
		"0x06caa9a5fd7e3dc3b3157973455cbe9b9c2b14d2",
		"0x2d66370666d7b9315e6e7fdb47f41ad722279833",
		"0x9ff43bd969e8dbc383d1aca50584c14266f3d876",
		"0xbfd88175e4ae6f7f2ee4b01bf96cf48d2bcb4196",
	}

	for _, blacklistAddress := range peggyBlacklistAddresses {
		k.SetEthereumBlacklistAddress(ctx, common.HexToAddress(blacklistAddress))
	}

	blacklistAddresses := k.GetAllEthereumBlacklistAddresses(ctx)
	assert.Equal(t, len(blacklistAddresses), 43)

	k.DeleteEthereumBlacklistAddress(ctx, common.HexToAddress("0xbfd88175e4ae6f7f2ee4b01bf96cf48d2bcb4196"))
	blacklistAddresses = k.GetAllEthereumBlacklistAddresses(ctx)

	assert.Equal(t, len(blacklistAddresses), 42)
}

func TestPeggyBankEvents(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 16, 12, 0, 0, 0, time.UTC),
	})

	input, _ := testpeggy.SetupFiveValChain(t)
	app.StakingKeeper = &input.StakingKeeper
	// msg server
	dep1 := &types.MsgDepositClaim{
		EventNonce:     1,
		TokenContract:  testpeggy.TokenContractAddrs[0],
		Amount:         sdk.NewInt(100),
		EthereumSender: testpeggy.EthAddrs[0].String(),
		CosmosReceiver: testpeggy.AccAddrs[0].String(),
		Orchestrator:   testpeggy.AccAddrs[0].String(),
	}
	anyClaim, _ := codectypes.NewAnyWithValue(dep1)
	att1 := &types.Attestation{
		Observed: false,
		Votes: []string{
			testpeggy.ValAddrs[0].String(),
			testpeggy.ValAddrs[1].String(),
			testpeggy.ValAddrs[2].String(),
			testpeggy.ValAddrs[3].String(),
			testpeggy.ValAddrs[4].String(),
		},
		Claim: anyClaim,
	}

	// simulate attestation
	app.PeggyKeeper.SetAttestation(ctx, dep1.EventNonce, dep1.ClaimHash(), att1)

	// run all endblock events to check all attestations
	endBlockResult := app.EndBlocker(ctx, tmtypes.RequestEndBlock{
		Height: ctx.BlockHeight(),
	})

	expectedBalanceUpdateJson := fmt.Sprintf(
		`[{"addr":"L4oMAfZovKc+xRwNjad9QZWCNHA=","denom":"cGVnZ3kweDZCMTc1NDc0RTg5MDk0QzQ0RGE5OGI5NTRFZWRlQUM0OTUyNzFkMEY=","amt":"0"},{"addr":"%s","denom":"cGVnZ3kweDZCMTc1NDc0RTg5MDk0QzQ0RGE5OGI5NTRFZWRlQUM0OTUyNzFkMEY=","amt":"100"}]`,
		base64.StdEncoding.EncodeToString(testpeggy.AccAddrs[0].Bytes()),
	)

	var (
		actualBankUpdate      []banktypes.BalanceUpdate
		expectedBalanceUpdate []banktypes.BalanceUpdate
	)

	update := endBlockResult.Events[len(endBlockResult.Events)-1].Attributes[0].GetValue()
	err := json.Unmarshal([]byte(update), &actualBankUpdate)
	assert.NoError(t, err, "unmarshal actual balanceUpdate should have no errors")
	sort.Slice(actualBankUpdate, func(i, j int) bool {
		return string(actualBankUpdate[i].Addr) < string(actualBankUpdate[j].Addr)
	})

	err = json.Unmarshal([]byte(expectedBalanceUpdateJson), &expectedBalanceUpdate)
	assert.NoError(t, err, "unmarshal expected balanceUpdate have no errors")
	sort.Slice(expectedBalanceUpdate, func(i, j int) bool {
		return string(expectedBalanceUpdate[i].Addr) < string(expectedBalanceUpdate[j].Addr)
	})

	assert.GreaterOrEqual(t, len(endBlockResult.Events), 1, "end block result len should greater or equal 1")
	assert.Equal(t, "cosmos.bank.v1beta1.EventSetBalances", endBlockResult.Events[len(endBlockResult.Events)-1].Type, "last event is not cosmos.bank.v1beta1.EventSetBalances")
	assert.Equal(t, expectedBalanceUpdate, actualBankUpdate)

	assert.Equal(t,
		"balance_updates",
		string(endBlockResult.Events[len(endBlockResult.Events)-1].Attributes[0].GetKey()),
		"attribute[0] key is not expected",
	)
}
