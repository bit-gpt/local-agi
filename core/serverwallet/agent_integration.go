package serverwallet

import (
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
)

func CreateServerWalletsFromConfig(serverWalletConfigs []types.ServerWalletConfig) (map[types.ServerWalletType]types.ServerWallet, error) {
	serverWallets := make(map[types.ServerWalletType]types.ServerWallet)

	for _, config := range serverWalletConfigs {
		walletType, err := ParseServerWalletType(config.Type)
		if err != nil {
			return nil, fmt.Errorf("invalid server wallet type in config: %v", err)
		}

		serverWalletConfig := types.ServerWalletConfig{
			Type:       string(walletType),
			Address:    config.Address,
			PrivateKey: config.PrivateKey,
		}

		serverWallet, err := NewServerWallet(serverWalletConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create server wallet for type %s: %v", walletType, err)
		}

		serverWallets[walletType] = serverWallet
	}

	return serverWallets, nil
}

func GenerateDefaultServerWalletsConfig() ([]types.ServerWalletConfig, error) {
	serverWalletConfigs := make([]types.ServerWalletConfig, 0)

	supportedTypes := GetSupportedServerWalletTypes()

	for _, walletType := range supportedTypes {
		serverWallet, err := GenerateNewServerWallet(walletType)
		if err != nil {
			return nil, fmt.Errorf("failed to generate server wallet for type %s: %v", walletType, err)
		}

		config := types.ServerWalletConfig{
			Type:       string(walletType),
			Address:    serverWallet.GetAddress(),
			PrivateKey: serverWallet.GetPrivateKey(),
		}

		serverWalletConfigs = append(serverWalletConfigs, config)
	}

	return serverWalletConfigs, nil
}

func GetServerWalletSummary(serverWallets map[types.ServerWalletType]types.ServerWallet) map[string]string {
	summary := make(map[string]string)

	for serverWalletType, serverWallet := range serverWallets {
		summary[string(serverWalletType)] = serverWallet.GetAddress()
	}

	return summary
}
