package actions

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/mudler/LocalAGI/core/serverwallet"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GetAllServerWalletBalancesAction struct {
	serverWallets map[types.ServerWalletType]types.ServerWallet
}

func NewGetAllServerWalletBalancesAction(serverWallets map[types.ServerWalletType]types.ServerWallet) *GetAllServerWalletBalancesAction {
	return &GetAllServerWalletBalancesAction{
		serverWallets: serverWallets,
	}
}

func (a *GetAllServerWalletBalancesAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	results := make(map[string]interface{})

	for serverWalletType, serverWalletInstance := range a.serverWallets {
		balance, err := serverWalletInstance.GetBalance(ctx)
		if err != nil {
			results[string(serverWalletType)] = map[string]interface{}{
				"error": err.Error(),
			}
			continue
		}

		chainConfig := types.GetDefaultChainConfig(serverWalletType)
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

		results[string(serverWalletType)] = map[string]interface{}{
			"server_wallet_type": string(serverWalletType),
			"currency":           chainConfig.Symbol,
			"balance":            formattedBalance,
			"token_balances":     formattedTokenBalances,
		}
	}

	resultJSON, _ := json.Marshal(results)
	return types.ActionResult{Result: string(resultJSON)}, nil
}

func (a *GetAllServerWalletBalancesAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "get_all_server_wallet_balances",
		Description: "Get balances for all configured server wallets",
		Properties:  map[string]jsonschema.Definition{},
		Required:    []string{},
	}
}

func (a *GetAllServerWalletBalancesAction) Plannable() bool {
	return true
}

func GetAllServerWalletBalancesConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "enabled",
			Label:    "Enable Get All Server Wallet Balances",
			Type:     config.FieldTypeCheckbox,
			Required: false,
			HelpText: "Enable the get all server wallet balances action",
		},
	}
}
