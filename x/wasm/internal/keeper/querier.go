package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/terra-project/core/x/wasm/internal/types"
)

// NewQuerier creates a new querier
func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, sdk.Error) {
		switch path[0] {
		case types.QueryGetBytecode:
			return queryBytecode(ctx, req, keeper)
		case types.QueryGetCodeInfo:
			return queryCodeInfo(ctx, req, keeper)
		case types.QueryGetContractInfo:
			return queryContractInfo(ctx, req, keeper)
		case types.QueryGetStore:
			return queryStore(ctx, req, keeper)
		case types.QueryGetMsg:
			return queryMsg(ctx, req, keeper)
		default:
			return nil, sdk.ErrUnknownRequest("unknown data query endpoint")
		}
	}
}

func queryBytecode(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	var params types.QueryCodeIDParams
	err := types.ModuleCdc.UnmarshalJSON(req.Data, &params)

	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}

	byteCode, err := keeper.GetBytecode(ctx, params.CodeID)
	if err != nil {
		return nil, sdk.ErrUnknownRequest("loading wasm code: " + err.Error())
	}

	bz, err := codec.MarshalJSONIndent(keeper.cdc, byteCode)
	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}
	return bz, nil
}

func queryCodeInfo(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	var params types.QueryCodeIDParams
	err := types.ModuleCdc.UnmarshalJSON(req.Data, &params)

	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}

	codeInfo, err := keeper.GetCodeInfo(ctx, params.CodeID)
	if err != nil {
		return nil, sdk.ErrUnknownRequest("loading wasm code: " + err.Error())
	}

	bz, err := codec.MarshalJSONIndent(keeper.cdc, codeInfo)
	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}
	return bz, nil
}

func queryContractInfo(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	var params types.QueryContractAddressParams
	err := types.ModuleCdc.UnmarshalJSON(req.Data, &params)

	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}

	contractInfo, sdkErr := keeper.GetContractInfo(ctx, params.ContractAddress)
	if sdkErr != nil {
		return nil, sdkErr
	}

	bz, err := codec.MarshalJSONIndent(keeper.cdc, contractInfo)
	if err != nil {
		return nil, sdk.ErrInvalidAddress(err.Error())
	}
	return bz, nil
}

func queryStore(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	var params types.QueryStoreParams
	err := types.ModuleCdc.UnmarshalJSON(req.Data, &params)
	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}

	models := keeper.queryToStore(ctx, params.ContractAddress, params.Key)
	bz, err := codec.MarshalJSONIndent(keeper.cdc, models)
	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}
	return bz, nil
}

func queryMsg(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	var params types.QueryMsgParams
	err := types.ModuleCdc.UnmarshalJSON(req.Data, &params)
	if err != nil {
		return nil, sdk.ErrUnknownRequest(err.Error())
	}

	return keeper.queryToContract(ctx, params.ContractAddress, params.Msg)
}
