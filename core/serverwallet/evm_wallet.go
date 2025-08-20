package serverwallet

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	coretypes "github.com/mudler/LocalAGI/core/types"
)

const erc20ABI = `[
	{
		"constant": true,
		"inputs": [{"name": "_owner", "type": "address"}],
		"name": "balanceOf",
		"outputs": [{"name": "balance", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{"name": "_to", "type": "address"},
			{"name": "_value", "type": "uint256"}
		],
		"name": "transfer",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"type": "function"
	}
]`

type EVMServerWallet struct {
	config     coretypes.ServerWalletConfig
	privateKey *ecdsa.PrivateKey
	address    string
	client     *ethclient.Client
	erc20ABI   abi.ABI
}

func NewEVMServerWallet(config coretypes.ServerWalletConfig) (*EVMServerWallet, error) {
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(config.PrivateKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	config.Address = address

	client, err := ethclient.Dial(GetDefaultChainConfig(coretypes.ServerWalletType(config.Type)).RPCUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create ethereum client: %v", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %v", err)
	}

	return &EVMServerWallet{
		config:     config,
		privateKey: privateKey,
		address:    address,
		client:     client,
		erc20ABI:   parsedABI,
	}, nil
}

func GenerateEVMServerWallet(walletType coretypes.ServerWalletType) (*EVMServerWallet, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	privateKeyHex := fmt.Sprintf("%x", crypto.FromECDSA(privateKey))

	config := coretypes.ServerWalletConfig{
		Type:       string(walletType),
		PrivateKey: privateKeyHex,
	}

	return NewEVMServerWallet(config)
}

func (w *EVMServerWallet) GetAddress() string {
	return w.address
}

func (w *EVMServerWallet) GetBalance(ctx context.Context) (*big.Int, error) {
	address := common.HexToAddress(w.address)
	balance, err := w.client.BalanceAt(ctx, address, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %v", err)
	}
	return balance, nil
}

func (w *EVMServerWallet) GetTokenBalance(ctx context.Context, tokenAddress string) (*big.Int, error) {
	if !common.IsHexAddress(tokenAddress) {
		return nil, fmt.Errorf("invalid token address: %s", tokenAddress)
	}

	tokenAddr := common.HexToAddress(tokenAddress)
	walletAddr := common.HexToAddress(w.address)

	data, err := w.erc20ABI.Pack("balanceOf", walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf call: %v", err)
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}

	result, err := w.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %v", err)
	}

	var balance *big.Int
	err = w.erc20ABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack balance: %v", err)
	}

	return balance, nil
}

func (w *EVMServerWallet) GetAllTokenBalances(ctx context.Context) (map[string]*big.Int, error) {
	chainConfig := coretypes.GetDefaultChainConfig(coretypes.ServerWalletType(w.config.Type))
	balances := make(map[string]*big.Int)

	for _, token := range chainConfig.SupportedTokens {
		balance, err := w.GetTokenBalance(ctx, token.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to get token balance: %v", err)
		}
		balances[token.Symbol] = balance
	}

	return balances, nil
}

func (w *EVMServerWallet) SendNative(ctx context.Context, to string, amount *big.Int) (string, error) {
	if !common.IsHexAddress(to) {
		return "", fmt.Errorf("invalid recipient address: %s", to)
	}

	fromAddress := common.HexToAddress(w.address)
	toAddress := common.HexToAddress(to)

	nonce, err := w.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	gasPrice, err := w.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %v", err)
	}

	chainConfig := coretypes.GetDefaultChainConfig(coretypes.ServerWalletType(w.config.Type))
	chainID := big.NewInt(chainConfig.ChainID)

	gasLimit, err := w.EstimateGas(ctx, to, amount, nil)
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %v", err)
	}

	tx := types.NewTransaction(nonce, toAddress, amount, gasLimit.Uint64(), gasPrice, nil)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), w.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction")
	}

	err = w.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

func (w *EVMServerWallet) SendToken(ctx context.Context, tokenAddress, to string, amount *big.Int) (string, error) {
	if !common.IsHexAddress(tokenAddress) {
		return "", fmt.Errorf("invalid token address: %s", tokenAddress)
	}
	if !common.IsHexAddress(to) {
		return "", fmt.Errorf("invalid recipient address: %s", to)
	}

	// Validate that the token is supported
	if !coretypes.IsTokenSupported(coretypes.ServerWalletType(w.config.Type), tokenAddress) {
		supportedTokens := coretypes.GetSupportedTokens(coretypes.ServerWalletType(w.config.Type))
		var supportedSymbols []string
		for _, token := range supportedTokens {
			supportedSymbols = append(supportedSymbols, token.Symbol)
		}
		return "", fmt.Errorf("token address %s is not supported. Only %v tokens are allowed",
			tokenAddress, supportedSymbols)
	}

	fromAddress := common.HexToAddress(w.address)
	tokenAddr := common.HexToAddress(tokenAddress)
	toAddress := common.HexToAddress(to)

	data, err := w.erc20ABI.Pack("transfer", toAddress, amount)
	if err != nil {
		return "", fmt.Errorf("failed to pack transfer call: %v", err)
	}

	nonce, err := w.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	gasPrice, err := w.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %v", err)
	}

	gasLimit, err := w.EstimateGas(ctx, tokenAddress, big.NewInt(0), data)
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %v", err)
	}

	chainConfig := coretypes.GetDefaultChainConfig(coretypes.ServerWalletType(w.config.Type))
	chainID := big.NewInt(chainConfig.ChainID)

	tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit.Uint64(), gasPrice, data)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), w.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction")
	}

	err = w.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

func (w *EVMServerWallet) EstimateGas(ctx context.Context, to string, amount *big.Int, data []byte) (*big.Int, error) {
	if !common.IsHexAddress(to) {
		return nil, fmt.Errorf("invalid recipient address: %s", to)
	}

	fromAddress := common.HexToAddress(w.address)
	toAddress := common.HexToAddress(to)

	msg := ethereum.CallMsg{
		From:  fromAddress,
		To:    &toAddress,
		Value: amount,
		Data:  data,
	}

	gasLimit, err := w.client.EstimateGas(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate gas: %v", err)
	}

	return big.NewInt(int64(gasLimit)), nil
}

func (w *EVMServerWallet) GetWalletType() coretypes.ServerWalletType {
	return coretypes.ServerWalletType(w.config.Type)
}

func (w *EVMServerWallet) GetPrivateKey() string {
	return w.config.PrivateKey
}

func (w *EVMServerWallet) GetGasPrice(ctx context.Context) (*big.Int, error) {
	return w.client.SuggestGasPrice(ctx)
}

func (w *EVMServerWallet) GetNonce(ctx context.Context) (uint64, error) {
	address := common.HexToAddress(w.address)
	return w.client.PendingNonceAt(ctx, address)
}

func (w *EVMServerWallet) GetTransactionReceipt(ctx context.Context, txHash string) (*types.Receipt, error) {
	hash := common.HexToHash(txHash)
	return w.client.TransactionReceipt(ctx, hash)
}

func (w *EVMServerWallet) EstimateGasForTokenTransfer(ctx context.Context, tokenAddress, recipient string, amount *big.Int) (*big.Int, error) {
	if !common.IsHexAddress(tokenAddress) {
		return nil, fmt.Errorf("invalid token address: %s", tokenAddress)
	}
	if !common.IsHexAddress(recipient) {
		return nil, fmt.Errorf("invalid recipient address: %s", recipient)
	}

	toAddress := common.HexToAddress(recipient)
	data, err := w.erc20ABI.Pack("transfer", toAddress, amount)

	if err != nil {
		return nil, fmt.Errorf("failed to pack transfer call: %v", err)
	}

	return w.EstimateGas(ctx, tokenAddress, big.NewInt(0), data)
}

func (w *EVMServerWallet) WaitForTransaction(ctx context.Context, txHash string) (*types.Receipt, error) {
	return w.WaitForTransactionWithTimeout(ctx, txHash, 60*time.Second)
}

func (w *EVMServerWallet) WaitForTransactionWithTimeout(ctx context.Context, txHash string, timeout time.Duration) (*types.Receipt, error) {
	hash := common.HexToHash(txHash)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("transaction confirmation timeout: %v", timeoutCtx.Err())
		case <-ticker.C:
			receipt, err := w.client.TransactionReceipt(timeoutCtx, hash)
			if err == nil {
				return receipt, nil
			}

			if strings.Contains(err.Error(), "not found") {
				continue
			}

			return nil, err
		}
	}
}

func (w *EVMServerWallet) CreatePaymentTransaction(ctx context.Context, paymentReq H402PaymentRequirement) (string, string, error) {
	if !common.IsHexAddress(paymentReq.PayToAddress) {
		return "", "", fmt.Errorf("invalid recipient address: %s", paymentReq.PayToAddress)
	}
	toAddress := common.HexToAddress(paymentReq.PayToAddress)
	fromAddress := common.HexToAddress(w.address)

	var amount *big.Int
	if paymentReq.TokenAddress == "0x0000000000000000000000000000000000000000" {
		// Native ETH/BNB payment
		if paymentReq.AmountRequiredFormat == "smallestUnit" {
			// Amount is already in wei
			amountFloat := new(big.Float).SetFloat64(paymentReq.AmountRequired)
			amount, _ = amountFloat.Int(nil)
		} else {
			// Amount is in human-readable ETH/BNB, convert to wei
			amountFloat := new(big.Float).SetFloat64(paymentReq.AmountRequired)
			weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
			amountInWei := new(big.Float).Mul(amountFloat, weiPerEth)
			amount, _ = amountInWei.Int(nil)
		}
	} else {
		// Token payment - get token decimals from chain config
		chainConfig := coretypes.GetDefaultChainConfig(coretypes.ServerWalletType(w.config.Type))
		decimals := 18 // Default to 18 decimals
		for _, token := range chainConfig.SupportedTokens {
			if strings.EqualFold(token.Address, paymentReq.TokenAddress) {
				decimals = token.Decimals
				break
			}
		}

		if paymentReq.AmountRequiredFormat == "smallestUnit" {
			// Amount is already in smallest token units
			amountFloat := new(big.Float).SetFloat64(paymentReq.AmountRequired)
			amount, _ = amountFloat.Int(nil)
		} else {
			// Amount is in human-readable token units, convert to smallest units
			amountFloat := new(big.Float).SetFloat64(paymentReq.AmountRequired)
			multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
			amountInTokens := new(big.Float).Mul(amountFloat, multiplier)
			amount, _ = amountInTokens.Int(nil)
		}
	}

	nonce, err := w.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", "", fmt.Errorf("failed to get nonce: %v", err)
	}

	gasPrice, err := w.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get gas price: %v", err)
	}

	chainConfig := coretypes.GetDefaultChainConfig(coretypes.ServerWalletType(w.config.Type))
	chainID := big.NewInt(chainConfig.ChainID)

	var tx *types.Transaction

	if paymentReq.TokenAddress == "0x0000000000000000000000000000000000000000" {
		gasLimit, err := w.EstimateGas(ctx, paymentReq.PayToAddress, amount, nil)
		if err != nil {
			return "", "", fmt.Errorf("failed to estimate gas: %v", err)
		}
		tx = types.NewTransaction(nonce, toAddress, amount, gasLimit.Uint64(), gasPrice, nil)
	} else {
		if !common.IsHexAddress(paymentReq.TokenAddress) {
			return "", "", fmt.Errorf("invalid token address: %s", paymentReq.TokenAddress)
		}
		tokenAddr := common.HexToAddress(paymentReq.TokenAddress)

		data, err := w.erc20ABI.Pack("transfer", toAddress, amount)
		if err != nil {
			return "", "", fmt.Errorf("failed to pack transfer call: %v", err)
		}

		gasLimit, err := w.EstimateGas(ctx, paymentReq.TokenAddress, big.NewInt(0), data)
		if err != nil {
			return "", "", fmt.Errorf("failed to estimate gas: %v", err)
		}

		tx = types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit.Uint64(), gasPrice, data)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), w.privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	txBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal transaction: %v", err)
	}
	txHex := "0x" + hex.EncodeToString(txBytes)

	signatureHex := signedTx.Hash().Hex()

	return txHex, signatureHex, nil
}

func (w *EVMServerWallet) GetH402Client() *H402Client {
	return NewH402Client(w, w)
}

func (w *EVMServerWallet) SendH402Request(ctx context.Context, method, url string, requestBody []byte) (*http.Response, error) {
	client := w.GetH402Client()
	return client.SendH402Request(ctx, method, url, requestBody)
}
