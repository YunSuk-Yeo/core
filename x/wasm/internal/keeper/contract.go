package keeper

import (
	"encoding/binary"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/terra-project/core/x/wasm/internal/types"
)

func (k Keeper) getContractDetails(ctx sdk.Context, contractAddress sdk.AccAddress) (codeInfo types.CodeInfo, contractStorePrefix prefix.Store, err sdk.Error) {
	store := ctx.KVStore(k.storeKey)

	bz := store.Get(types.GetContractInfoKey(contractAddress))
	if bz == nil {
		err = types.ErrNotFound("contract")
		return
	}

	var contractInfo types.ContractInfo
	k.cdc.MustUnmarshalBinaryBare(bz, &contractInfo)

	bz = store.Get(types.GetCodeInfoKey(contractInfo.CodeID))
	if bz == nil {
		err = types.ErrNotFound("contract info")
		return
	}

	k.cdc.MustUnmarshalBinaryBare(bz, &codeInfo)
	contractStoreKey := types.GetContractStoreKey(contractAddress)
	contractStorePrefix = prefix.NewStore(ctx.KVStore(k.storeKey), contractStoreKey)
	return
}

// StoreCode uploads and compiles a WASM contract bytecode, returning a short identifier for the stored code
func (k Keeper) StoreCode(ctx sdk.Context, creator sdk.AccAddress, wasmCode []byte) (codeID uint64, sdkErr sdk.Error) {
	wasmCode, err := k.uncompress(ctx, wasmCode)
	if err != nil {
		return 0, types.ErrCreateFailed(err)
	}

	codeHash, err := k.wasmer.Create(wasmCode)
	if err != nil {
		return 0, types.ErrCreateFailed(err)
	}

	codeID = k.increaseLastCodeID(ctx)
	contractInfo := types.NewCodeInfo(codeHash, creator)
	k.SetCodeInfo(ctx, codeID, contractInfo)

	return codeID, nil
}

// InstantiateContract creates an instance of a WASM contract
func (k Keeper) InstantiateContract(ctx sdk.Context, creator sdk.AccAddress, codeID uint64, initMsg []byte, deposit sdk.Coins) (contractAddress sdk.AccAddress, err sdk.Error) {
	// create contract address
	contractAddress = k.generateContractAddress(ctx, codeID)
	existingAccnt := k.accountKeeper.GetAccount(ctx, contractAddress)
	if existingAccnt != nil {
		err = types.ErrAccountExists(existingAccnt.GetAddress())
		return
	}

	// deposit initial contract funds
	err = k.bankKeeper.SendCoins(ctx, creator, contractAddress, deposit)
	if err != nil {
		return
	}

	contractAccount := k.accountKeeper.GetAccount(ctx, contractAddress)

	// get code info
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetCodeInfoKey(codeID))
	if bz == nil {
		err = types.ErrNotFound("contract")
		return
	}
	var codeInfo types.CodeInfo
	k.cdc.MustUnmarshalBinaryBare(bz, &codeInfo)

	// prepare params for contract instantiate call
	params := types.NewWasmAPIParams(ctx, creator, deposit, contractAccount)

	// create prefixed data store
	contractStoreKey := types.GetContractStoreKey(contractAddress)
	contractStore := prefix.NewStore(ctx.KVStore(k.storeKey), contractStoreKey)

	// instantiate wasm contract
	gas := k.gasForContract(ctx)
	res, err2 := k.wasmer.Instantiate(codeInfo.CodeHash, params, initMsg, contractStore, cosmwasmAPI, gas)
	if err2 != nil {
		err = types.ErrInstantiateFailed(err2)
		return
	}

	k.consumeGas(ctx, res.GasUsed)

	err = k.dispatchMessages(ctx, contractAccount, res.Messages)
	if err != nil {
		return
	}

	// persist contractInfo
	contractInfo := types.NewContractInfo(codeID, contractAddress, creator, initMsg)
	k.SetContractInfo(ctx, contractAddress, contractInfo)

	return contractAddress, nil
}

// ExecuteContract executes the contract instance
func (k Keeper) ExecuteContract(ctx sdk.Context, contractAddress sdk.AccAddress, caller sdk.AccAddress, coins sdk.Coins, msg []byte) sdk.Error {
	codeInfo, storePrefix, sdkerr := k.getContractDetails(ctx, contractAddress)
	if sdkerr != nil {
		return sdkerr
	}

	// add more funds
	sdkerr = k.bankKeeper.SendCoins(ctx, caller, contractAddress, coins)
	if sdkerr != nil {
		return sdkerr
	}
	contractAccount := k.accountKeeper.GetAccount(ctx, contractAddress)
	params := types.NewWasmAPIParams(ctx, caller, coins, contractAccount)

	gas := k.gasForContract(ctx)
	res, err := k.wasmer.Execute(codeInfo.CodeHash, params, msg, storePrefix, cosmwasmAPI, gas)
	if err != nil {
		return types.ErrExecuteFailed(err)
	}

	k.consumeGas(ctx, res.GasUsed)

	sdkerr = k.dispatchMessages(ctx, contractAccount, res.Messages)
	if sdkerr != nil {
		return sdkerr
	}

	return nil
}

func (k Keeper) gasForContract(ctx sdk.Context) uint64 {
	meter := ctx.GasMeter()
	remaining := (meter.Limit() - meter.GasConsumed()) * k.GasMultiplier(ctx)
	if remaining > k.MaxContractGas(ctx) {
		return k.MaxContractGas(ctx)
	}
	return remaining
}

// converts contract gas usage to sdk gas and consumes it
func (k Keeper) consumeGas(ctx sdk.Context, gas uint64) {
	consumed := gas / k.GasMultiplier(ctx)
	ctx.GasMeter().ConsumeGas(consumed, "wasm contract")
}

// generates a contract address from codeID + instanceID
// and increases last instanceID
func (k Keeper) generateContractAddress(ctx sdk.Context, codeID uint64) sdk.AccAddress {
	instanceID := k.increaseLastInstanceID(ctx)
	// NOTE: It is possible to get a duplicate address if either codeID or instanceID
	// overflow 32 bits. This is highly improbable, but something that could be refactored.
	contractID := codeID<<32 + instanceID
	return addrFromUint64(contractID)
}

// GetNextCodeID returns next code ID which is sequentially increasing
func (k Keeper) GetNextCodeID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.LastCodeIDKey)
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	return id
}

func (k Keeper) increaseLastCodeID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.LastCodeIDKey)
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	bz = sdk.Uint64ToBigEndian(id + 1)
	store.Set(types.LastCodeIDKey, bz)
	return id
}

func (k Keeper) increaseLastInstanceID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.LastInstanceIDKey)
	id := uint64(1)
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	bz = sdk.Uint64ToBigEndian(id + 1)
	store.Set(types.LastInstanceIDKey, bz)
	return id
}

func addrFromUint64(id uint64) sdk.AccAddress {
	addr := make([]byte, 20)
	addr[0] = 'C'
	binary.PutUvarint(addr[1:], id)
	return sdk.AccAddress(crypto.AddressHash(addr))
}

func (k Keeper) queryToStore(ctx sdk.Context, contractAddress sdk.AccAddress, key []byte) (result []byte) {
	if key == nil {
		return result
	}

	prefixStoreKey := types.GetContractStoreKey(contractAddress)
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), prefixStoreKey)

	result = prefixStore.Get(key)

	return
}

func (k Keeper) queryToContract(ctx sdk.Context, contractAddr sdk.AccAddress, key []byte) ([]byte, sdk.Error) {
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(k.queryGasLimit))

	codeInfo, contractStorePrefix, err := k.getContractDetails(ctx, contractAddr)
	if err != nil {
		return nil, err
	}
	queryResult, gasUsed, qErr := k.wasmer.Query(codeInfo.CodeHash, key, contractStorePrefix, cosmwasmAPI, k.gasForContract(ctx))
	if qErr != nil {

		return nil, sdk.ErrInternal(qErr.Error())
	}

	k.consumeGas(ctx, gasUsed)
	return queryResult, nil
}
