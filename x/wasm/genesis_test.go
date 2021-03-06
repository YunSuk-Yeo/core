package wasm

import (
	"encoding/json"
	"testing"

	wasmTypes "github.com/confio/go-cosmwasm/types"
	"github.com/stretchr/testify/require"
	"github.com/terra-project/core/x/wasm/internal/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestInitGenesis(t *testing.T) {
	data, cleanup := setupTest(t)
	defer cleanup()

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator := createFakeFundedAccount(data.ctx, data.acctKeeper, deposit.Add(deposit))
	fred := createFakeFundedAccount(data.ctx, data.acctKeeper, topUp)

	h := data.module.NewHandler()

	msg := MsgStoreCode{
		Sender:       creator,
		WASMBytecode: testContract,
	}
	res := h(data.ctx, msg)
	require.True(t, res.IsOK())

	bytecode, sdkErr := data.keeper.GetBytecode(data.ctx, 1)
	require.NoError(t, sdkErr)
	require.Equal(t, testContract, bytecode)

	_, _, bob := keyPubAddr()
	initMsg := initMsg{
		Verifier:    fred.String(),
		Beneficiary: bob.String(),
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	initCmd := MsgInstantiateContract{
		Sender:    creator,
		CodeID:    1,
		InitMsg:   initMsgBz,
		InitCoins: deposit,
	}
	res = h(data.ctx, initCmd)
	require.True(t, res.IsOK())

	// Check contract address
	var contractAddr sdk.AccAddress
	for _, event := range res.Events {
		if event.Type == types.EventTypeInstantiateContract {
			for _, attr := range event.Attributes {
				if string(attr.GetKey()) == types.AttributeKeyContractAddress {
					contractAddr, err = sdk.AccAddressFromBech32(string(attr.GetValue()))
					require.NoError(t, err)
					break
				}
			}
		}
	}

	require.False(t, contractAddr.Empty())
	_, sdkErr = data.keeper.GetContractInfo(data.ctx, contractAddr)
	require.NoError(t, sdkErr)

	execCmd := MsgExecuteContract{
		Sender:   fred,
		Contract: contractAddr,
		Msg:      []byte("{}"),
		Coins:    topUp,
	}
	res = h(data.ctx, execCmd)
	require.True(t, res.IsOK())

	// ensure all contract state is as after init
	bytecode, sdkErr = data.keeper.GetBytecode(data.ctx, 1)
	require.NoError(t, sdkErr)
	require.Equal(t, testContract, bytecode)

	expectedContractInfo := NewContractInfo(1, contractAddr, creator, initMsgBz)
	contractInfo, sdkErr := data.keeper.GetContractInfo(data.ctx, contractAddr)
	require.NoError(t, sdkErr)
	require.Equal(t, expectedContractInfo, contractInfo)

	iter := data.keeper.GetContractStoreIterator(data.ctx, contractAddr)
	var models []Model
	for ; iter.Valid(); iter.Next() {
		models = append(models, Model{Key: string(iter.Key()), Value: string(iter.Value())})
	}

	expectedConfigState := state{
		Verifier:    wasmTypes.CanonicalAddress(fred),
		Beneficiary: wasmTypes.CanonicalAddress(bob),
		Funder:      wasmTypes.CanonicalAddress(creator),
	}

	assertContractStore(t, models, expectedConfigState)

	// export into genstate
	genState := ExportGenesis(data.ctx, data.keeper)

	// create new app to import genstate into
	newData, newCleanup := setupTest(t)
	defer newCleanup()

	// initialize new app with genstate
	InitGenesis(newData.ctx, newData.keeper, genState)

	// run same checks again on newdata, to make sure it was reinitialized correctly
	bytecode, err = data.keeper.GetBytecode(data.ctx, 1)
	require.NoError(t, err)
	require.Equal(t, testContract, bytecode)

	contractInfo, err = data.keeper.GetContractInfo(data.ctx, contractAddr)
	require.NoError(t, err)
	require.Equal(t, expectedContractInfo, contractInfo)

	iter = data.keeper.GetContractStoreIterator(data.ctx, contractAddr)
	models = []Model{}
	for ; iter.Valid(); iter.Next() {
		models = append(models, Model{Key: string(iter.Key()), Value: string(iter.Value())})
	}

	assertContractStore(t, models, expectedConfigState)
}
