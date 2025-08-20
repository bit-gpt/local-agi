package types

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
)

type ServerWalletType string

const (
	ServerWalletTypeBNB  ServerWalletType = "BNB"
	ServerWalletTypeSOL  ServerWalletType = "SOL"
	ServerWalletTypeBASE ServerWalletType = "BASE"
)

type ServerWallet interface {
	GetAddress() string

	GetBalance(ctx context.Context) (*big.Int, error)

	GetTokenBalance(ctx context.Context, tokenAddress string) (*big.Int, error)

	GetAllTokenBalances(ctx context.Context) (map[string]*big.Int, error)

	SendNative(ctx context.Context, to string, amount *big.Int) (string, error)

	SendToken(ctx context.Context, tokenAddress, to string, amount *big.Int) (string, error)

	EstimateGas(ctx context.Context, to string, amount *big.Int, data []byte) (*big.Int, error)

	EstimateGasForTokenTransfer(ctx context.Context, tokenAddress, recipient string, amount *big.Int) (*big.Int, error)

	GetWalletType() ServerWalletType

	GetPrivateKey() string
}

type Transaction struct {
	Hash      string   `json:"hash"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Amount    *big.Int `json:"amount"`
	Token     string   `json:"token,omitempty"`
	Timestamp int64    `json:"timestamp"`
	Status    string   `json:"status"`
	Fee       *big.Int `json:"fee"`
	Type      string   `json:"type"` // "native" or "token"
}

type SupportedToken struct {
	Symbol   string `json:"symbol"`
	Address  string `json:"address"`
	Decimals int    `json:"decimals"`
}

type ChainConfig struct {
	ChainID         int64            `json:"chain_id"`
	RPCUrl          string           `json:"rpc_url"`
	ExplorerURL     string           `json:"explorer_url"`
	Symbol          string           `json:"symbol"`
	Decimals        int              `json:"decimals"`
	SupportedTokens []SupportedToken `json:"supported_tokens"`
}

func GetDefaultChainConfig(serverWalletType ServerWalletType) ChainConfig {
	switch serverWalletType {
	case ServerWalletTypeBNB:
		if os.Getenv("ENV") == "DEV" {
			return ChainConfig{
				ChainID:     97,
				RPCUrl:      "https://bsc-testnet-dataseed.bnbchain.org",
				ExplorerURL: "https://testnet.bscscan.com/",
				Symbol:      "BNB",
				Decimals:    18,
				SupportedTokens: []SupportedToken{
					{Symbol: "USDC", Address: "0x64544969ed7EBf5f083679233325356EbE738930", Decimals: 18},
					{Symbol: "USDT", Address: "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd", Decimals: 18},
				},
			}
		}
		return ChainConfig{
			ChainID:     56,
			RPCUrl:      "https://bsc-dataseed1.binance.org/",
			ExplorerURL: "https://bscscan.com",
			Symbol:      "BNB",
			Decimals:    18,
			SupportedTokens: []SupportedToken{
				{Symbol: "USDC", Address: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", Decimals: 18},
				{Symbol: "USDT", Address: "0x55d398326f99059fF775485246999027B3197955", Decimals: 18},
			},
		}
	case ServerWalletTypeBASE:
		if os.Getenv("ENV") == "DEV" {
			return ChainConfig{
				ChainID:     84532,
				RPCUrl:      "https://sepolia.base.org",
				ExplorerURL: "https://sepolia.basescan.org",
				Symbol:      "ETH",
				Decimals:    18,
				SupportedTokens: []SupportedToken{
					{Symbol: "USDC", Address: "0x036CbD53842c5426634e7929541eC2318f3dCF7e", Decimals: 6},
					{Symbol: "USDT", Address: "0x22c0db4cc9b339e34956a5699e5e95dc0e00c800", Decimals: 6},
				},
			}
		}
		return ChainConfig{
			ChainID:     8453,
			RPCUrl:      "https://mainnet.base.org",
			ExplorerURL: "https://basescan.org",
			Symbol:      "ETH",
			Decimals:    18,
			SupportedTokens: []SupportedToken{
				{Symbol: "USDC", Address: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", Decimals: 6},
				{Symbol: "USDT", Address: "0xfde4C96c8593536E31F229EA8f37b2ADa2699bb2", Decimals: 6},
			},
		}
	case ServerWalletTypeSOL:
		if os.Getenv("ENV") == "DEV" {
			return ChainConfig{
				ChainID:     0,
				RPCUrl:      "https://api.devnet.solana.com",
				ExplorerURL: "https://explorer.solana.com",
				Symbol:      "SOL",
				Decimals:    9,
				SupportedTokens: []SupportedToken{
					{Symbol: "USDC", Address: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", Decimals: 6},
					{Symbol: "USDT", Address: "EJwZgeZrdC8TXTQbQBoL6bfuAnFUUy1PVCMB4DYPzVaS", Decimals: 6},
				},
			}
		}
		return ChainConfig{
			ChainID:     0,
			RPCUrl:      "https://api.mainnet-beta.solana.com",
			ExplorerURL: "https://explorer.solana.com",
			Symbol:      "SOL",
			Decimals:    9,
			SupportedTokens: []SupportedToken{
				{Symbol: "USDC", Address: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", Decimals: 6},
				{Symbol: "USDT", Address: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", Decimals: 6},
			},
		}
	default:
		return ChainConfig{}
	}
}

func ValidateServerWalletType(serverWalletType ServerWalletType) bool {
	switch serverWalletType {
	case ServerWalletTypeBNB, ServerWalletTypeSOL, ServerWalletTypeBASE:
		return true
	default:
		return false
	}
}

func IsTokenSupported(serverWalletType ServerWalletType, tokenAddress string) bool {
	config := GetDefaultChainConfig(serverWalletType)
	for _, token := range config.SupportedTokens {
		if strings.EqualFold(token.Address, tokenAddress) {
			return true
		}
	}
	return false
}

func GetSupportedTokens(serverWalletType ServerWalletType) []SupportedToken {
	config := GetDefaultChainConfig(serverWalletType)
	return config.SupportedTokens
}

func GetAddressExplorerURL(serverWalletType ServerWalletType, address string) string {
	config := GetDefaultChainConfig(serverWalletType)

	switch serverWalletType {
	case ServerWalletTypeBNB, ServerWalletTypeBASE:
		return fmt.Sprintf("%s/address/%s", config.ExplorerURL, address)
	case ServerWalletTypeSOL:
		url := fmt.Sprintf("%s/address/%s", config.ExplorerURL, address)
		if os.Getenv("ENV") == "DEV" {
			url += "?cluster=devnet"
		}
		return url
	default:
		return fmt.Sprintf("%s/address/%s", config.ExplorerURL, address)
	}
}

func GetTransactionExplorerURL(serverWalletType ServerWalletType, txHash string) string {
	config := GetDefaultChainConfig(serverWalletType)

	switch serverWalletType {
	case ServerWalletTypeBNB, ServerWalletTypeBASE:
		return fmt.Sprintf("%s/tx/%s", config.ExplorerURL, txHash)
	case ServerWalletTypeSOL:
		url := fmt.Sprintf("%s/tx/%s", config.ExplorerURL, txHash)
		if os.Getenv("ENV") == "DEV" {
			url += "?cluster=devnet"
		}
		return url
	default:
		return fmt.Sprintf("%s/tx/%s", config.ExplorerURL, txHash)
	}
}

func GetDefaultPayLimits() map[string]float64 {
	return map[string]float64{
		"ETH":  0.01,
		"BNB":  0.1,
		"SOL":  0.5,
		"USDC": 50,
		"USDT": 50,
	}
}

type ServerWalletConfig struct {
	Type       string `json:"type"`
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
}
