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

type GetServerWalletAddressAction struct {
	serverWallets map[types.ServerWalletType]types.ServerWallet
}

func NewGetServerWalletAddressAction(serverWallets map[types.ServerWalletType]types.ServerWallet) *GetServerWalletAddressAction {
	return &GetServerWalletAddressAction{
		serverWallets: serverWallets,
	}
}

func (a *GetServerWalletAddressAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
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

	address := serverWalletInstance.GetAddress()
	chainConfig := types.GetDefaultChainConfig(serverWalletType)

	result := map[string]interface{}{
		"address":            address,
		"server_wallet_type": string(serverWalletType),
		"symbol":             chainConfig.Symbol,
		"explorer_url":       types.GetAddressExplorerURL(serverWalletType, address),
		// "qr_code_url":  fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s", address),
	}

	resultJSON, _ := json.Marshal(result)
	return types.ActionResult{Result: string(resultJSON)}, nil
}

func (a *GetServerWalletAddressAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "get_server_wallet_address",
		Description: "Get the server wallet address for receiving cryptocurrency",
		Properties: map[string]jsonschema.Definition{
			"server_wallet_type": {
				Type:        jsonschema.String,
				Description: "The type of serverwallet (BNB, SOL, BASE)",
				Enum:        []string{"BNB", "SOL", "BASE"},
			},
		},
		Required: []string{"server_wallet_type"},
	}
}

func (a *GetServerWalletAddressAction) Plannable() bool {
	return true
}

func GetServerWalletAddressConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "enabled",
			Label:    "Enable Get Server Wallet Address",
			Type:     config.FieldTypeCheckbox,
			Required: false,
			HelpText: "Enable the get server wallet address action",
		},
	}
}
