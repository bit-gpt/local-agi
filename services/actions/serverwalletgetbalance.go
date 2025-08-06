package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mudler/LocalAGI/core/serverwallet"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GetServerWalletBalanceAction struct {
	serverWallets map[types.ServerWalletType]types.ServerWallet
}

func NewGetServerWalletBalanceAction(serverWallets map[types.ServerWalletType]types.ServerWallet) *GetServerWalletBalanceAction {
	return &GetServerWalletBalanceAction{
		serverWallets: serverWallets,
	}
}

func (a *GetServerWalletBalanceAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
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

	balance, err := serverWalletInstance.GetBalance(ctx)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get balance: %v", err)
	}

	chainConfig := serverwallet.GetDefaultChainConfig(serverWalletType)
	formattedBalance := serverwallet.FormatBalance(balance, chainConfig.Decimals)

	tokenBalances, err := serverWalletInstance.GetAllTokenBalances(ctx)
	if err != nil {
		tokenBalances = make(map[string]*big.Int)
	}

	formattedTokenBalances := make(map[string]string)
	for symbol, tokenBalance := range tokenBalances {
		var decimals int = 18
		for _, token := range chainConfig.SupportedTokens {
			if token.Symbol == symbol {
				decimals = token.Decimals
				break
			}
		}
		formattedTokenBalances[symbol] = serverwallet.FormatBalance(tokenBalance, decimals)
	}

	result := map[string]interface{}{
		"server_wallet_type": string(serverWalletType),
		"currency":           chainConfig.Symbol,
		"balance":            formattedBalance,
		"token_balances":     formattedTokenBalances,
	}

	resultJSON, _ := json.Marshal(result)
	return types.ActionResult{Result: string(resultJSON)}, nil
}

func (a *GetServerWalletBalanceAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "get_server_wallet_balance",
		Description: "Get the current native balance and all supported token balances of a server wallet",
		Properties: map[string]jsonschema.Definition{
			"server_wallet_type": {
				Type:        jsonschema.String,
				Description: "The type of server wallet (BNB, SOL, BASE)",
				Enum:        []string{"BNB", "SOL", "BASE"},
			},
		},
		Required: []string{"server_wallet_type"},
	}
}

func (a *GetServerWalletBalanceAction) Plannable() bool {
	return true
}

func GetServerWalletBalanceConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "enabled",
			Label:    "Enable Get Server Wallet Balance",
			Type:     config.FieldTypeCheckbox,
			Required: false,
			HelpText: "Enable the get server wallet balance action",
		},
	}
}
