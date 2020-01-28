package keeper

import (
	"fmt"

	wasmTypes "github.com/confio/go-cosmwasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/exported"
	"github.com/cosmos/cosmos-sdk/x/bank"
)

func (k Keeper) dispatchMessages(ctx sdk.Context, contract exported.Account, msgs []wasmTypes.CosmosMsg) sdk.Error {
	for _, msg := range msgs {
		if err := k.dispatchMessage(ctx, contract, msg); err != nil {
			return err
		}
	}
	return nil
}

func sendMsgIsEmpty(msg wasmTypes.SendMsg) bool {
	return msg.FromAddress == "" && msg.ToAddress == "" && len(msg.Amount) == 0
}

type msgWrapper interface{}

func isEmpty(msg msgWrapper) bool {
	switch m := msg.(type) {
	case wasmTypes.SendMsg:
		return len(m.FromAddress) == 0 && len(m.ToAddress) == 0 && len(m.Amount) == 0
	case wasmTypes.ContractMsg:
		return len(m.ContractAddr) == 0 && len(m.Msg) == 0 && len(m.Send) == 0
	}
	return true
}

func (k Keeper) dispatchMessage(ctx sdk.Context, contract exported.Account, msg wasmTypes.CosmosMsg) sdk.Error {
	if !isEmpty(msg.Send) && !isEmpty(msg.Contract) {
		return sdk.ErrInternal("single msg cannot contain multiple msgs")
	}

	// Handle MsgSend
	if !isEmpty(msg.Send) {
		sendMsg, err := parseToMsgSend(msg.Send)
		if err != nil {
			return err
		}

		return k.handleSdkMessage(ctx, contract, sendMsg)
	}

	// Handle MsgExecuteContract
	if !isEmpty(msg.Contract) {
		targetAddr, stderr := sdk.AccAddressFromBech32(msg.Contract.ContractAddr)
		if stderr != nil {
			return sdk.ErrInvalidAddress(msg.Contract.ContractAddr)
		}

		coins, err := parseToCoins(msg.Contract.Send)
		if err != nil {
			return err
		}

		err = k.ExecuteContract(ctx, targetAddr, contract.GetAddress(), coins, []byte(msg.Contract.Msg))
		if err != nil {
			return err
		}
	}

	if msg.Opaque.Data != "" {
		return sdk.ErrInternal("dispatch opaque message not yet implemented")
	}

	return sdk.ErrInternal(fmt.Sprintf("Unknown Msg: %#v", msg))
}

func parseToCoins(wasmCoins []wasmTypes.Coin) (coins sdk.Coins, err sdk.Error) {
	for _, coin := range wasmCoins {
		amount, ok := sdk.NewIntFromString(coin.Amount)
		if !ok {
			err = sdk.ErrInvalidCoins(fmt.Sprintf("Failed to parse %s", coin.Amount))
			return
		}
		c := sdk.Coin{
			Denom:  coin.Denom,
			Amount: amount,
		}
		coins = append(coins, c)
	}
	return
}

func parseToMsgSend(wasmMsg wasmTypes.SendMsg) (msgSend bank.MsgSend, err sdk.Error) {
	fromAddr, stderr := sdk.AccAddressFromBech32(wasmMsg.FromAddress)
	if stderr != nil {
		err = sdk.ErrInvalidAddress(wasmMsg.FromAddress)
		return
	}
	toAddr, stderr := sdk.AccAddressFromBech32(wasmMsg.ToAddress)
	if stderr != nil {
		err = sdk.ErrInvalidAddress(wasmMsg.ToAddress)
		return
	}

	coins, err := parseToCoins(wasmMsg.Amount)
	if err != nil {
		return
	}

	msgSend = bank.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      coins,
	}

	return
}

func (k Keeper) handleSdkMessage(ctx sdk.Context, contract exported.Account, msg sdk.Msg) sdk.Error {
	// make sure this account can send it
	contractAddr := contract.GetAddress()
	for _, acct := range msg.GetSigners() {
		if !acct.Equals(contractAddr) {
			return sdk.ErrUnauthorized("contract doesn't have permission")
		}
	}

	// find the handler and execute it
	h := k.router.Route(msg.Route())
	if h == nil {
		return sdk.ErrUnknownRequest(msg.Route())
	}
	res := h(ctx, msg)
	if !res.IsOK() {
		return sdk.NewError(res.Codespace, res.Code, res.Log)
	}
	return nil
}
