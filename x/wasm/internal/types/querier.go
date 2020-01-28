package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// query endpoints supported by the wasm Querier
const (
	QueryGetBytecode     = "bytecode"
	QueryGetCodeInfo     = "codeInfo"
	QueryGetContractInfo = "contractInfo"
	QueryGetStore        = "store"
	QueryGetMsg          = "msg"
)

// QueryCodeIDParams defines the params for the following queries:
// - 'custom/wasm/codeInfo
// - 'custom/wasm/bytecode
type QueryCodeIDParams struct {
	CodeID uint64
}

// NewQueryCodeIDParams returns QueryCodeIDParams instance
func NewQueryCodeIDParams(codeID uint64) QueryCodeIDParams {
	return QueryCodeIDParams{codeID}
}

// QueryContractAddressParams defines the params for the following queries:
// - 'custom/wasm/contractInfo
type QueryContractAddressParams struct {
	ContractAddress sdk.AccAddress
}

// NewQueryContractAddressParams returns QueryContractAddressParams instance
func NewQueryContractAddressParams(contractAddress sdk.AccAddress) QueryContractAddressParams {
	return QueryContractAddressParams{contractAddress}
}

// QueryStoreParams defines the params for the following queries:
// - 'custom/wasm/store'
type QueryStoreParams struct {
	ContractAddress sdk.AccAddress
	Key             []byte
}

// NewQueryStoreParams returns QueryStoreParams instance
func NewQueryStoreParams(contractAddress sdk.AccAddress, key []byte) QueryStoreParams {
	return QueryStoreParams{contractAddress, key}
}

// QueryMsgParams defines the params for the following queries:
// - 'custom/wasm/msg'
type QueryMsgParams struct {
	ContractAddress sdk.AccAddress
	Msg             []byte
}

// NewQueryMsgParams returns QueryMsgParams instance
func NewQueryMsgParams(contractAddress sdk.AccAddress, msg []byte) QueryMsgParams {
	return QueryMsgParams{contractAddress, msg}
}
