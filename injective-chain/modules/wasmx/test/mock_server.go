package testwasmx

import (
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type WasmViewKeeperMock struct {
	getContractHistoryHandler     func(ctx sdk.Context, contractAddr sdk.AccAddress) []wasmdtypes.ContractCodeHistoryEntry
	querySmartHandler             func(ctx sdk.Context, contractAddr sdk.AccAddress, req []byte) ([]byte, error)
	queryRawHandler               func(ctx sdk.Context, contractAddress sdk.AccAddress, key []byte) []byte
	HasContractInfoHandler        func(ctx sdk.Context, contractAddress sdk.AccAddress) bool
	GetContractInfoHandler        func(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo
	iterateContractInfoHandler    func(ctx sdk.Context, cb func(sdk.AccAddress, wasmdtypes.ContractInfo) bool)
	iterateContractsByCodeHandler func(ctx sdk.Context, codeID uint64, cb func(address sdk.AccAddress) bool)
	iterateContractStateHandler   func(ctx sdk.Context, contractAddress sdk.AccAddress, cb func(key, value []byte) bool)
	getCodeInfoHandler            func(ctx sdk.Context, codeID uint64) *wasmdtypes.CodeInfo
	iterateCodeInfosHandler       func(ctx sdk.Context, cb func(uint64, wasmdtypes.CodeInfo) bool)
	getByteCodeHandler            func(ctx sdk.Context, codeID uint64) ([]byte, error)
	isPinnedCodeHandler           func(ctx sdk.Context, codeID uint64) bool
}

func (mock WasmViewKeeperMock) IterateContractsByCreator(ctx sdk.Context, creator sdk.AccAddress, cb func(address sdk.AccAddress) bool) {
	panic("Not needed")
}

func (mock WasmViewKeeperMock) GetParams(ctx sdk.Context) wasmdtypes.Params {
	panic("Not needed")
}

func (mock WasmViewKeeperMock) GetContractHistory(ctx sdk.Context, contractAddr sdk.AccAddress) []wasmdtypes.ContractCodeHistoryEntry {
	return mock.getContractHistoryHandler(ctx, contractAddr)
}

func (mock WasmViewKeeperMock) QuerySmart(ctx sdk.Context, contractAddr sdk.AccAddress, req []byte) ([]byte, error) {
	return mock.querySmartHandler(ctx, contractAddr, req)
}

func (mock WasmViewKeeperMock) QueryRaw(ctx sdk.Context, contractAddress sdk.AccAddress, key []byte) []byte {
	return mock.queryRawHandler(ctx, contractAddress, key)
}

func (mock WasmViewKeeperMock) HasContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress) bool {
	return mock.HasContractInfoHandler(ctx, contractAddress)
}

func (mock WasmViewKeeperMock) GetContractInfo(ctx sdk.Context, contractAddress sdk.AccAddress) *wasmdtypes.ContractInfo {
	return mock.GetContractInfoHandler(ctx, contractAddress)
}

func (mock WasmViewKeeperMock) IterateContractInfo(ctx sdk.Context, cb func(sdk.AccAddress, wasmdtypes.ContractInfo) bool) {
	mock.iterateContractInfoHandler(ctx, cb)
}

func (mock WasmViewKeeperMock) IterateContractsByCode(ctx sdk.Context, codeID uint64, cb func(address sdk.AccAddress) bool) {
	mock.iterateContractsByCodeHandler(ctx, codeID, cb)
}

func (mock WasmViewKeeperMock) IterateContractState(ctx sdk.Context, contractAddress sdk.AccAddress, cb func(key, value []byte) bool) {
	mock.iterateContractStateHandler(ctx, contractAddress, cb)
}

func (mock WasmViewKeeperMock) GetCodeInfo(ctx sdk.Context, codeID uint64) *wasmdtypes.CodeInfo {
	return mock.getCodeInfoHandler(ctx, codeID)
}

func (mock WasmViewKeeperMock) IterateCodeInfos(ctx sdk.Context, cb func(uint64, wasmdtypes.CodeInfo) bool) {
	mock.iterateCodeInfosHandler(ctx, cb)
}

func (mock WasmViewKeeperMock) GetByteCode(ctx sdk.Context, codeID uint64) ([]byte, error) {
	return mock.getByteCodeHandler(ctx, codeID)
}

func (mock WasmViewKeeperMock) IsPinnedCode(ctx sdk.Context, codeID uint64) bool {
	return mock.isPinnedCodeHandler(ctx, codeID)
}

type WasmOpsKeeperMock struct {
	CreateHandler      *func(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte, instantiateAccess *wasmdtypes.AccessConfig) (codeID uint64, checksum []byte, err error)
	instantiateHandler *func(ctx sdk.Context, codeID uint64, creator, admin sdk.AccAddress, initMsg []byte, label string, deposit sdk.Coins) (sdk.AccAddress, []byte, error)
	executeHanlder     *func(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, msg []byte, coins sdk.Coins) ([]byte, error)
	migrateHanlder     *func(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, newCodeID uint64, msg []byte) ([]byte, error)
	SudoHandler        *func(ctx sdk.Context, contractAddress sdk.AccAddress, msg []byte) ([]byte, error)
	PinCodeHandler     *func(ctx sdk.Context, codeID uint64) error
	UnpinCodeHandler   *func(ctx sdk.Context, codeID uint64) error
}

func (mock WasmOpsKeeperMock) Instantiate2(ctx sdk.Context, codeID uint64, creator, admin sdk.AccAddress, initMsg []byte, label string, deposit sdk.Coins, salt []byte, fixMsg bool) (sdk.AccAddress, []byte, error) {
	panic("Not used")
}

func (mock WasmOpsKeeperMock) SetAccessConfig(ctx sdk.Context, codeID uint64, caller sdk.AccAddress, newConfig wasmdtypes.AccessConfig) error {
	panic("Not used")
}

func (mock WasmOpsKeeperMock) Create(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte, instantiateAccess *wasmdtypes.AccessConfig) (codeID uint64, checksum []byte, err error) {
	if mock.CreateHandler != nil {
		return (*mock.CreateHandler)(ctx, creator, wasmCode, instantiateAccess)
	}
	return 0, nil, nil
}

func (mock WasmOpsKeeperMock) Instantiate(ctx sdk.Context, codeID uint64, creator, admin sdk.AccAddress, initMsg []byte, label string, deposit sdk.Coins) (sdk.AccAddress, []byte, error) {
	if mock.instantiateHandler != nil {
		return (*mock.instantiateHandler)(ctx, codeID, creator, admin, initMsg, label, deposit)
	}
	return []byte(""), []byte(""), nil
}

func (mock WasmOpsKeeperMock) Execute(ctx sdk.Context, contractAddress, caller sdk.AccAddress, msg []byte, coins sdk.Coins) ([]byte, error) {
	if mock.executeHanlder != nil {
		return (*mock.executeHanlder)(ctx, contractAddress, caller, msg, coins)
	}
	return []byte(""), nil
}

func (mock WasmOpsKeeperMock) Migrate(ctx sdk.Context, contractAddress, caller sdk.AccAddress, newCodeID uint64, msg []byte) ([]byte, error) {
	if mock.migrateHanlder != nil {
		return (*mock.migrateHanlder)(ctx, contractAddress, caller, newCodeID, msg)
	}
	return []byte(""), nil
}

func (mock WasmOpsKeeperMock) Sudo(ctx sdk.Context, contractAddress sdk.AccAddress, msg []byte) ([]byte, error) {
	if mock.SudoHandler != nil {
		return (*mock.SudoHandler)(ctx, contractAddress, msg)
	}
	return []byte(""), nil
}

func (mock WasmOpsKeeperMock) UpdateContractAdmin(ctx sdk.Context, contractAddress, caller, newAdmin sdk.AccAddress) error {
	return nil
}

func (mock WasmOpsKeeperMock) ClearContractAdmin(ctx sdk.Context, contractAddress, caller sdk.AccAddress) error {
	return nil
}

func (mock WasmOpsKeeperMock) PinCode(ctx sdk.Context, codeID uint64) error {
	if mock.PinCodeHandler != nil {
		return (*mock.PinCodeHandler)(ctx, codeID)
	}
	return nil
}

func (mock WasmOpsKeeperMock) UnpinCode(ctx sdk.Context, codeID uint64) error {
	if mock.UnpinCodeHandler != nil {
		return (*mock.UnpinCodeHandler)(ctx, codeID)
	}
	return nil
}

func (mock WasmOpsKeeperMock) SetContractInfoExtension(ctx sdk.Context, contract sdk.AccAddress, extra wasmdtypes.ContractInfoExtension) error {
	return nil
}
