package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mudler/LocalAGI/core/serverwallet"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type WaitForTransactionConfirmationAction struct {
	serverWallets map[types.ServerWalletType]types.ServerWallet
}

func NewWaitForTransactionConfirmationAction(serverWallets map[types.ServerWalletType]types.ServerWallet) *WaitForTransactionConfirmationAction {
	return &WaitForTransactionConfirmationAction{
		serverWallets: serverWallets,
	}
}

func (a *WaitForTransactionConfirmationAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	serverWalletTypeStr, ok := params["server_wallet_type"].(string)
	if !ok {
		return types.ActionResult{}, fmt.Errorf("server_wallet_type parameter is required")
	}

	serverWalletType, err := serverwallet.ParseServerWalletType(serverWalletTypeStr)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid server wallet type: %v", err)
	}

	serverWalletInstance, exists := a.serverWallets[serverWalletType]
	if !exists {
		return types.ActionResult{}, fmt.Errorf("server wallet not configured for type: %s", serverWalletType)
	}

	transactionHash, ok := params["transaction_hash"].(string)
	if !ok {
		return types.ActionResult{}, fmt.Errorf("transaction_hash parameter is required")
	}

	var result map[string]interface{}

	switch serverWalletType {
	case types.ServerWalletTypeBNB, types.ServerWalletTypeBASE:
		if evmWallet, ok := serverWalletInstance.(*serverwallet.EVMServerWallet); ok {
			receipt, err := evmWallet.WaitForTransaction(ctx, transactionHash)
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("failed to wait for transaction confirmation: %v", err)
			}

			result = map[string]interface{}{
				"server_wallet_type": string(serverWalletType),
				"status":             "confirmed",
				"success":            receipt.Status == 1,
				"explorer_url":       types.GetTransactionExplorerURL(serverWalletType, transactionHash),
			}

			if receipt.Status != 1 {
				result["status"] = "failed"
				result["success"] = false
			}
		} else {
			return types.ActionResult{}, fmt.Errorf("invalid wallet type for EVM operations")
		}

	case types.ServerWalletTypeSOL:
		if solanaWallet, ok := serverWalletInstance.(*serverwallet.SolanaServerWallet); ok {
			err := solanaWallet.WaitForTransaction(ctx, transactionHash)
			if err != nil {
				result = map[string]interface{}{
					"server_wallet_type": string(serverWalletType),
					"status":             "failed",
					"success":            false,
					"error":              err.Error(),
					"explorer_url":       types.GetTransactionExplorerURL(serverWalletType, transactionHash),
				}
			} else {
				result = map[string]interface{}{
					"server_wallet_type": string(serverWalletType),
					"status":             "confirmed",
					"success":            true,
					"explorer_url":       types.GetTransactionExplorerURL(serverWalletType, transactionHash),
				}
			}
		} else {
			return types.ActionResult{}, fmt.Errorf("invalid server wallet type for Solana operations")
		}

	default:
		return types.ActionResult{}, fmt.Errorf("unsupported server wallet type: %s", serverWalletType)
	}

	resultJSON, _ := json.Marshal(result)
	return types.ActionResult{Result: string(resultJSON)}, nil
}

func (a *WaitForTransactionConfirmationAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "wait_for_transaction_confirmation",
		Description: "Wait for a transaction to be confirmed on the blockchain and return its status",
		Properties: map[string]jsonschema.Definition{
			"server_wallet_type": {
				Type:        jsonschema.String,
				Description: "The type of server wallet (BNB, SOL, BASE)",
				Enum:        []string{"BNB", "SOL", "BASE"},
			},
			"transaction_hash": {
				Type:        jsonschema.String,
				Description: "The transaction hash/signature to wait for confirmation",
			},
		},
		Required: []string{"server_wallet_type", "transaction_hash"},
	}
}

func (a *WaitForTransactionConfirmationAction) Plannable() bool {
	return true
}

func WaitForTransactionConfirmationConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "enabled",
			Label:    "Enable Wait For Transaction Confirmation",
			Type:     config.FieldTypeCheckbox,
			Required: false,
			HelpText: "Enable the wait for transaction confirmation action",
		},
	}
}
