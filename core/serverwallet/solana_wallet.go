package serverwallet

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mudler/LocalAGI/core/h402"
	coretypes "github.com/mudler/LocalAGI/core/types"
)

type SolanaServerWallet struct {
	config     coretypes.ServerWalletConfig
	privateKey solana.PrivateKey
	publicKey  solana.PublicKey
	address    string
	client     *rpc.Client
}

func NewSolanaServerWallet(config coretypes.ServerWalletConfig) (*SolanaServerWallet, error) {
	privateKeyBytes, err := hex.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %v", err)
	}

	privateKey := solana.PrivateKey(privateKeyBytes)
	publicKey := privateKey.PublicKey()
	address := publicKey.String()
	config.Address = address

	clientInstance := rpc.New(GetDefaultChainConfig(coretypes.ServerWalletType(config.Type)).RPCUrl)

	return &SolanaServerWallet{
		config:     config,
		privateKey: privateKey,
		publicKey:  publicKey,
		address:    address,
		client:     clientInstance,
	}, nil
}

func GenerateSolanaServerWallet() (*SolanaServerWallet, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	privateKeyHex := hex.EncodeToString(privateKey)

	config := coretypes.ServerWalletConfig{
		Type:       string(coretypes.ServerWalletTypeSOL),
		PrivateKey: privateKeyHex,
	}

	wallet, err := NewSolanaServerWallet(config)
	if err != nil {
		return nil, err
	}

	expectedPublicKey := solana.PublicKeyFromBytes(publicKey)
	if !wallet.publicKey.Equals(expectedPublicKey) {
		return nil, fmt.Errorf("key generation verification failed")
	}

	return wallet, nil
}

func (w *SolanaServerWallet) GetAddress() string {
	return w.address
}

func (w *SolanaServerWallet) GetBalance(ctx context.Context) (*big.Int, error) {
	balance, err := w.client.GetBalance(ctx, w.publicKey, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %v", err)
	}

	// Balance is in lamports (1 SOL = 1,000,000,000 lamports)
	return big.NewInt(int64(balance.Value)), nil
}

type TokenAccountData struct {
	Mint            solana.PublicKey
	Owner           solana.PublicKey
	Amount          uint64
	DelegateOption  uint32
	Delegate        solana.PublicKey
	State           uint8
	IsNativeOption  uint32
	IsNative        uint64
	DelegatedAmount uint64
	CloseAuthority  solana.PublicKey
}

func parseTokenAccountData(data []byte) (*TokenAccountData, error) {
	if len(data) < 165 {
		return nil, fmt.Errorf("invalid token account data length: %d", len(data))
	}

	account := &TokenAccountData{}

	copy(account.Mint[:], data[0:32])

	copy(account.Owner[:], data[32:64])

	account.Amount = binary.LittleEndian.Uint64(data[64:72])

	account.DelegateOption = binary.LittleEndian.Uint32(data[72:76])

	copy(account.Delegate[:], data[76:108])

	account.State = data[108]

	account.IsNativeOption = binary.LittleEndian.Uint32(data[109:113])

	account.IsNative = binary.LittleEndian.Uint64(data[113:121])

	account.DelegatedAmount = binary.LittleEndian.Uint64(data[121:129])

	copy(account.CloseAuthority[:], data[129:161])

	return account, nil
}

func (w *SolanaServerWallet) GetTokenBalance(ctx context.Context, tokenMintAddress string) (*big.Int, error) {
	mintPubkey, err := solana.PublicKeyFromBase58(tokenMintAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid token mint address: %v", err)
	}

	associatedTokenAccount, _, err := solana.FindAssociatedTokenAddress(w.publicKey, mintPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to find associated token account: %v", err)
	}

	accountInfo, err := w.client.GetAccountInfo(ctx, associatedTokenAccount)
	if err != nil {
		return big.NewInt(0), nil
	}

	if accountInfo.Value == nil || len(accountInfo.Value.Data.GetBinary()) == 0 {
		return big.NewInt(0), nil
	}

	tokenData, err := parseTokenAccountData(accountInfo.Value.Data.GetBinary())
	if err != nil {
		return nil, fmt.Errorf("failed to parse token account data: %v", err)
	}

	return big.NewInt(int64(tokenData.Amount)), nil
}

func (w *SolanaServerWallet) GetAllTokenBalances(ctx context.Context) (map[string]*big.Int, error) {
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

func (w *SolanaServerWallet) SendNative(ctx context.Context, to string, amount *big.Int) (string, error) {
	toPubkey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return "", fmt.Errorf("invalid recipient address: %v", err)
	}

	recent, err := w.client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get recent blockhash: %v", err)
	}

	instruction := system.NewTransferInstruction(
		amount.Uint64(),
		w.publicKey,
		toPubkey,
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recent.Value.Blockhash,
		solana.TransactionPayer(w.publicKey),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %v", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(w.publicKey) {
			return &w.privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction")
	}

	signature, err := w.client.SendTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signature.String(), nil
}

func (w *SolanaServerWallet) SendToken(ctx context.Context, tokenMintAddress, to string, amount *big.Int) (string, error) {
	// Validate that the token is supported
	if !coretypes.IsTokenSupported(coretypes.ServerWalletType(w.config.Type), tokenMintAddress) {
		supportedTokens := coretypes.GetSupportedTokens(coretypes.ServerWalletType(w.config.Type))
		var supportedSymbols []string
		for _, token := range supportedTokens {
			supportedSymbols = append(supportedSymbols, token.Symbol)
		}
		return "", fmt.Errorf("token address %s is not supported. Only %v tokens are allowed",
			tokenMintAddress, supportedSymbols)
	}

	mintPubkey, err := solana.PublicKeyFromBase58(tokenMintAddress)
	if err != nil {
		return "", fmt.Errorf("invalid token mint address: %v", err)
	}

	toPubkey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return "", fmt.Errorf("invalid recipient address: %v", err)
	}

	sourceTokenAccount, _, err := solana.FindAssociatedTokenAddress(w.publicKey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("failed to find source token account: %v", err)
	}

	destinationTokenAccount, _, err := solana.FindAssociatedTokenAddress(toPubkey, mintPubkey)
	if err != nil {
		return "", fmt.Errorf("failed to find destination token account: %v", err)
	}

	_, err = w.client.GetAccountInfo(ctx, destinationTokenAccount)
	accountExists := err == nil

	recent, err := w.client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get recent blockhash: %v", err)
	}

	var instructions []solana.Instruction

	if !accountExists {
		createATAInstruction := associatedtokenaccount.NewCreateInstruction(
			w.publicKey,
			toPubkey,
			mintPubkey,
		).Build()
		instructions = append(instructions, createATAInstruction)
	}

	transferInstruction := token.NewTransferInstruction(
		amount.Uint64(),
		sourceTokenAccount,
		destinationTokenAccount,
		w.publicKey,
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, transferInstruction)

	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(w.publicKey),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %v", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(w.publicKey) {
			return &w.privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction")
	}

	signature, err := w.client.SendTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signature.String(), nil
}

func (w *SolanaServerWallet) EstimateGasForTokenTransfer(ctx context.Context, tokenMintAddress, to string, amount *big.Int) (*big.Int, error) {
	mintPubkey, err := solana.PublicKeyFromBase58(tokenMintAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid token mint address: %v", err)
	}

	toPubkey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient address: %v", err)
	}

	destinationTokenAccount, _, err := solana.FindAssociatedTokenAddress(toPubkey, mintPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to find destination token account: %v", err)
	}

	_, err = w.client.GetAccountInfo(ctx, destinationTokenAccount)
	accountExists := err == nil

	baseFee := int64(5000)

	if !accountExists {
		// Creating ATA costs approximately 2,039,280 lamports (rent) + 5000 lamports (transaction fee)
		baseFee += 2044280
	}

	return big.NewInt(baseFee), nil
}

func (w *SolanaServerWallet) EstimateGas(ctx context.Context, to string, amount *big.Int, data []byte) (*big.Int, error) {
	return big.NewInt(5000), nil
}

func (w *SolanaServerWallet) GetWalletType() coretypes.ServerWalletType {
	return coretypes.ServerWalletType(w.config.Type)
}

func (w *SolanaServerWallet) GetPrivateKey() string {
	return w.config.PrivateKey
}

func (w *SolanaServerWallet) GetRecentBlockhash(ctx context.Context) (string, error) {
	recent, err := w.client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get recent blockhash: %v", err)
	}
	return recent.Value.Blockhash.String(), nil
}

func (w *SolanaServerWallet) GetTransactionStatus(ctx context.Context, signature string) (*rpc.GetTransactionResult, error) {
	sig := solana.MustSignatureFromBase58(signature)
	return w.client.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding: solana.EncodingBase64,
	})
}

func (w *SolanaServerWallet) WaitForTransaction(ctx context.Context, signature string) error {
	return w.WaitForTransactionWithTimeout(ctx, signature, 60*time.Second)
}

func (w *SolanaServerWallet) WaitForTransactionWithTimeout(ctx context.Context, signature string, timeout time.Duration) error {
	sig := solana.MustSignatureFromBase58(signature)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("transaction confirmation timeout: %v", timeoutCtx.Err())
		case <-ticker.C:
			status, err := w.client.GetSignatureStatuses(timeoutCtx, true, sig)
			if err != nil {
				continue
			}

			if len(status.Value) > 0 && status.Value[0] != nil {
				txStatus := status.Value[0]

				if txStatus.Err != nil {
					return fmt.Errorf("transaction failed: %v", txStatus.Err)
				}

				switch txStatus.ConfirmationStatus {
				case rpc.ConfirmationStatusProcessed, rpc.ConfirmationStatusConfirmed, rpc.ConfirmationStatusFinalized:
					return nil
				}
			}
		}
	}
}

func (w *SolanaServerWallet) CreatePaymentTransaction(ctx context.Context, paymentReq h402.H402PaymentRequirement) (string, string, error) {
	toPubkey, err := solana.PublicKeyFromBase58(paymentReq.PayToAddress)
	if err != nil {
		return "", "", fmt.Errorf("invalid recipient address: %v", err)
	}

	var amountLamports uint64
	if paymentReq.TokenAddress == "11111111111111111111111111111111" {
		// Native SOL payment
		if paymentReq.AmountRequiredFormat == "smallestUnit" {
			// Amount is already in lamports
			amountLamports = uint64(paymentReq.AmountRequired)
		} else {
			// Amount is in human-readable SOL, convert to lamports using precise arithmetic
			amountFloat := new(big.Float).SetFloat64(paymentReq.AmountRequired)
			lamportsPerSol := new(big.Float).SetInt(big.NewInt(1e9))
			amountInLamports := new(big.Float).Mul(amountFloat, lamportsPerSol)
			amountInt, _ := amountInLamports.Int(nil)
			amountLamports = amountInt.Uint64()
		}
	} else {
		// Token payment - get token decimals from chain config
		chainConfig := coretypes.GetDefaultChainConfig(coretypes.ServerWalletTypeSOL)
		decimals := 6 // Default to 6 decimals (common for USDC/USDT)
		for _, token := range chainConfig.SupportedTokens {
			if strings.EqualFold(token.Address, paymentReq.TokenAddress) {
				decimals = token.Decimals
				break
			}
		}

		if paymentReq.AmountRequiredFormat == "smallestUnit" {
			// Amount is already in smallest token units
			amountLamports = uint64(paymentReq.AmountRequired)
		} else {
			// Amount is in human-readable token units, convert to smallest units using precise arithmetic
			amountFloat := new(big.Float).SetFloat64(paymentReq.AmountRequired)
			multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
			amountInTokens := new(big.Float).Mul(amountFloat, multiplier)
			amountInt, _ := amountInTokens.Int(nil)
			amountLamports = amountInt.Uint64()
		}
	}

	recent, err := w.client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", "", fmt.Errorf("failed to get recent blockhash: %v", err)
	}

	var instructions []solana.Instruction

	if paymentReq.TokenAddress == "11111111111111111111111111111111" {
		instruction := system.NewTransferInstruction(
			amountLamports,
			w.publicKey,
			toPubkey,
		).Build()
		instructions = append(instructions, instruction)
	} else {
		mintPubkey, err := solana.PublicKeyFromBase58(paymentReq.TokenAddress)
		if err != nil {
			return "", "", fmt.Errorf("invalid token mint address: %v", err)
		}

		sourceTokenAccount, _, err := solana.FindAssociatedTokenAddress(w.publicKey, mintPubkey)
		if err != nil {
			return "", "", fmt.Errorf("failed to find source token account: %v", err)
		}

		destinationTokenAccount, _, err := solana.FindAssociatedTokenAddress(toPubkey, mintPubkey)
		if err != nil {
			return "", "", fmt.Errorf("failed to find destination token account: %v", err)
		}

		_, err = w.client.GetAccountInfo(ctx, destinationTokenAccount)
		accountExists := err == nil

		if !accountExists {
			createATAInstruction := associatedtokenaccount.NewCreateInstruction(
				w.publicKey,
				toPubkey,
				mintPubkey,
			).Build()
			instructions = append(instructions, createATAInstruction)
		}

		transferInstruction := token.NewTransferInstruction(
			amountLamports,
			sourceTokenAccount,
			destinationTokenAccount,
			w.publicKey,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, transferInstruction)
	}

	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(w.publicKey),
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create transaction: %v", err)
	}

	signatures, err := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(w.publicKey) {
			return &w.privateKey
		}
		return nil
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal transaction: %v", err)
	}
	txBase64 := base64.StdEncoding.EncodeToString(txBytes)

	if len(signatures) == 0 {
		return "", "", fmt.Errorf("no signatures found")
	}
	signatureStr := signatures[0].String()

	return txBase64, signatureStr, nil
}

func (w *SolanaServerWallet) GetH402Client() *h402.H402Client {
	return h402.NewH402Client(w, w)
}

func (w *SolanaServerWallet) SendH402Request(ctx context.Context, method, url string, requestBody []byte) (*http.Response, error) {
	client := w.GetH402Client()
	return client.SendH402Request(ctx, method, url, requestBody)
}
