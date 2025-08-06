package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/mudler/LocalAGI/core/serverwallet"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type EstimateTransactionFeeAction struct {
	serverWallets map[types.ServerWalletType]types.ServerWallet
}

func NewEstimateTransactionFeeAction(serverWallets map[types.ServerWalletType]types.ServerWallet) *EstimateTransactionFeeAction {
	return &EstimateTransactionFeeAction{
		serverWallets: serverWallets,
	}
}

func (a *EstimateTransactionFeeAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ServerWalletTypeStr, ok := params["server_wallet_type"].(string)
	if !ok {
		return types.ActionResult{}, fmt.Errorf("server_wallet_type parameter is required")
	}

	serverWalletType, err := serverwallet.ParseServerWalletType(ServerWalletTypeStr)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid server wallet type: %v", err)
	}

	serverWalletInstance, exists := a.serverWallets[serverWalletType]
	if !exists {
		return types.ActionResult{}, fmt.Errorf("server wallet not configured for type: %s", serverWalletType)
	}

	recipient, ok := params["recipient"].(string)
	if !ok {
		return types.ActionResult{}, fmt.Errorf("recipient parameter is required")
	}

	amountStr, ok := params["amount"].(string)
	if !ok {
		return types.ActionResult{}, fmt.Errorf("amount parameter is required")
	}

	var fee *big.Int
	var tokenAddress string
	var amount *big.Int

	if tokenSymbol, ok := params["token_symbol"].(string); ok && tokenSymbol != "" {
		supportedTokens := types.GetSupportedTokens(serverWalletType)
		var foundToken *types.SupportedToken
		for _, token := range supportedTokens {
			if strings.EqualFold(token.Symbol, tokenSymbol) {
				foundToken = &token
				break
			}
		}

		if foundToken == nil {
			var supportedSymbols []string
			for _, token := range supportedTokens {
				supportedSymbols = append(supportedSymbols, token.Symbol)
			}
			return types.ActionResult{}, fmt.Errorf("token symbol %s is not supported. Supported tokens: %v", tokenSymbol, supportedSymbols)
		}

		amount, err = serverwallet.ParseAmount(amountStr, foundToken.Decimals)
		if err != nil {
			return types.ActionResult{}, fmt.Errorf("invalid amount: %v", err)
		}

		tokenAddress = foundToken.Address

		fee, err = serverWalletInstance.EstimateGasForTokenTransfer(ctx, tokenAddress, recipient, amount)

	} else {
		chainConfig := serverwallet.GetDefaultChainConfig(serverWalletType)
		amount, err = serverwallet.ParseAmount(amountStr, chainConfig.Decimals)
		if err != nil {
			return types.ActionResult{}, fmt.Errorf("invalid amount: %v", err)
		}
		fee, err = serverWalletInstance.EstimateGas(ctx, recipient, amount, nil)
	}

	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to estimate fee: %v", err)
	}

	chainConfig := serverwallet.GetDefaultChainConfig(serverWalletType)
	formattedFee := serverwallet.FormatBalance(fee, chainConfig.Decimals)

	result := map[string]interface{}{
		"server_wallet_type": string(serverWalletType),
		"estimated_fee":      formattedFee,
		"fee_wei":            fee.String(),
		"symbol":             chainConfig.Symbol,
		"recipient":          recipient,
		"amount":             amountStr,
	}

	if tokenSymbol, ok := params["token_symbol"].(string); ok && tokenSymbol != "" {
		result["token_symbol"] = tokenSymbol
		result["token_address"] = tokenAddress
	}

	resultJSON, _ := json.Marshal(result)
	return types.ActionResult{Result: string(resultJSON)}, nil
}

func (a *EstimateTransactionFeeAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "estimate_transaction_fee",
		Description: "Estimate the fee for a cryptocurrency or token transaction",
		Properties: map[string]jsonschema.Definition{
			"server_wallet_type": {
				Type:        jsonschema.String,
				Description: "The type of server wallet (BNB, SOL, BASE)",
				Enum:        []string{"BNB", "SOL", "BASE"},
			},
			"recipient": {
				Type:        jsonschema.String,
				Description: "The recipient's wallet address",
			},
			"amount": {
				Type:        jsonschema.String,
				Description: "The amount to send (e.g., '1.5' for 1.5 tokens)",
			},
			"token_symbol": {
				Type:        jsonschema.String,
				Description: "Optional token symbol (USDC or USDT). Leave empty for native cryptocurrency.",
			},
		},
		Required: []string{"server_wallet_type", "recipient", "amount"},
	}
}

func (a *EstimateTransactionFeeAction) Plannable() bool {
	return true
}

func EstimateTransactionFeeConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "enabled",
			Label:    "Enable Estimate Transaction Fee",
			Type:     config.FieldTypeCheckbox,
			Required: false,
			HelpText: "Enable the estimate transaction fee action",
		},
	}
}
