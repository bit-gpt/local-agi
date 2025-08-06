package serverwallet

import (
	"fmt"
	"math/big"
	"strings"

	coretypes "github.com/mudler/LocalAGI/core/types"
)

func NewServerWallet(config coretypes.ServerWalletConfig) (coretypes.ServerWallet, error) {
	if !coretypes.ValidateServerWalletType(coretypes.ServerWalletType(config.Type)) {
		return nil, fmt.Errorf("unsupported wallet type: %s", config.Type)
	}

	if config.PrivateKey == "" {
		return nil, fmt.Errorf("private key is required")
	}

	switch config.Type {
	case string(coretypes.ServerWalletTypeBNB), string(coretypes.ServerWalletTypeBASE):
		return NewEVMServerWallet(config)
	case string(coretypes.ServerWalletTypeSOL):
		return NewSolanaServerWallet(config)
	default:
		return nil, fmt.Errorf("unsupported wallet type: %s", config.Type)
	}
}

func GenerateNewServerWallet(walletType coretypes.ServerWalletType) (coretypes.ServerWallet, error) {
	if !coretypes.ValidateServerWalletType(walletType) {
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}

	switch walletType {
	case coretypes.ServerWalletTypeBNB, coretypes.ServerWalletTypeBASE:
		return GenerateEVMServerWallet(walletType)
	case coretypes.ServerWalletTypeSOL:
		return GenerateSolanaServerWallet()
	default:
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}
}

func ParseServerWalletType(s string) (coretypes.ServerWalletType, error) {
	serverWalletType := coretypes.ServerWalletType(strings.ToUpper(s))
	if !coretypes.ValidateServerWalletType(serverWalletType) {
		return "", fmt.Errorf("unsupported wallet type: %s", s)
	}
	return serverWalletType, nil
}

func GetSupportedServerWalletTypes() []coretypes.ServerWalletType {
	return []coretypes.ServerWalletType{
		coretypes.ServerWalletTypeBNB,
		coretypes.ServerWalletTypeSOL,
		coretypes.ServerWalletTypeBASE,
	}
}

func FormatBalance(balance *big.Int, decimals int) string {
	if balance == nil {
		return "0"
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	quotient := new(big.Int).Div(balance, divisor)
	remainder := new(big.Int).Mod(balance, divisor)

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return quotient.String()
	}

	remainderStr := remainder.String()
	for len(remainderStr) < decimals {
		remainderStr = "0" + remainderStr
	}

	remainderStr = strings.TrimRight(remainderStr, "0")
	if remainderStr == "" {
		return quotient.String()
	}

	return fmt.Sprintf("%s.%s", quotient.String(), remainderStr)
}

func GetDefaultChainConfig(serverWalletType coretypes.ServerWalletType) coretypes.ChainConfig {
	return coretypes.GetDefaultChainConfig(serverWalletType)
}
