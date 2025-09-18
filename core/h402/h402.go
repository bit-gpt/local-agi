package h402

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mudler/LocalAGI/core/sse"
	"github.com/mudler/LocalAGI/core/state"
	coretypes "github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/db"
	models "github.com/mudler/LocalAGI/dbmodels"
	"github.com/shopspring/decimal"
)

type H402PaymentRequirement struct {
	Namespace               string  `json:"namespace"`
	TokenAddress            string  `json:"tokenAddress"`
	AmountRequired          float64 `json:"amountRequired"`
	AmountRequiredFormat    string  `json:"amountRequiredFormat"`
	PayToAddress            string  `json:"payToAddress"`
	NetworkID               string  `json:"networkId"`
	Description             string  `json:"description"`
	Resource                string  `json:"resource"`
	Scheme                  string  `json:"scheme"`
	MimeType                string  `json:"mimeType"`
	EstimatedProcessingTime int     `json:"estimatedProcessingTime"`
	TokenDecimals           int     `json:"tokenDecimals"`
	TokenSymbol             string  `json:"tokenSymbol"`
}

type H402Response struct {
	H402Version int                      `json:"h402Version"`
	Error       string                   `json:"error"`
	Accepts     []H402PaymentRequirement `json:"accepts"`
}

type H402PaymentPayload struct {
	Type              string `json:"type"`
	Signature         string `json:"signature"`
	Transaction       string `json:"transaction,omitempty"`
	SignedTransaction string `json:"signedTransaction,omitempty"`
}

type H402Payment struct {
	H402Version int                `json:"h402Version"`
	Scheme      string             `json:"scheme"`
	Namespace   string             `json:"namespace"`
	NetworkID   string             `json:"networkId"`
	Resource    string             `json:"resource"`
	Payload     H402PaymentPayload `json:"payload"`
}

type H402PaymentResponseData struct {
	Success     bool   `json:"success"`
	Transaction string `json:"transaction"`
	Namespace   string `json:"namespace"`
}

type H402ResponseWithPaymentInfo struct {
	Response         *http.Response
	PaymentInfo      *PaymentInfo
	PayLimitExceeded *PayLimitError
	InfoError        error
}

type PayLimitError struct {
	Message         string
	TokenSymbol     string
	RequestedAmount float64
	LimitAmount     float64
}

type PaymentInfo struct {
	Amount       float64
	AmountFormat string
	WalletType   string
	Transaction  string
	Namespace    string
	TokenAddress string
}

type PaymentTransactionCreator interface {
	CreatePaymentTransaction(ctx context.Context, paymentReq H402PaymentRequirement) (string, string, error)
}

type H402Client struct {
	wallets   map[coretypes.ServerWalletType]coretypes.ServerWallet
	payLimits map[string]float64
	agentID   uuid.UUID
	pool      interface{}
}

func NewH402Client(wallet coretypes.ServerWallet, transactionCreator PaymentTransactionCreator) *H402Client {
	return &H402Client{
		wallets: map[coretypes.ServerWalletType]coretypes.ServerWallet{
			wallet.GetWalletType(): wallet,
		},
		payLimits: make(map[string]float64),
	}
}

func NewH402ClientWithWallets(wallets map[coretypes.ServerWalletType]coretypes.ServerWallet, payLimits map[string]float64, agentID uuid.UUID, pool interface{}) *H402Client {
	return &H402Client{
		wallets:   wallets,
		payLimits: payLimits,
		agentID:   agentID,
		pool:      pool,
	}
}

func NewH402ClientWithoutWallets(payLimits map[string]float64, agentID uuid.UUID, pool interface{}) *H402Client {
	return &H402Client{
		payLimits: payLimits,
		agentID:   agentID,
		pool:      pool,
	}
}

func MapNetworkIDToWalletType(networkId string) (coretypes.ServerWalletType, error) {
	switch networkId {
	case "56", "97":
		return coretypes.ServerWalletTypeBNB, nil
	case "8453", "84532":
		return coretypes.ServerWalletTypeBASE, nil
	case "mainnet", "devnet", "testnet":
		return coretypes.ServerWalletTypeSOL, nil
	default:
		if chainID, err := strconv.ParseInt(networkId, 10, 64); err == nil {
			switch chainID {
			case 56, 97:
				return coretypes.ServerWalletTypeBNB, nil
			case 8453, 84532:
				return coretypes.ServerWalletTypeBASE, nil
			}
		}
		return "", fmt.Errorf("unsupported network ID: %s", networkId)
	}
}

func (h *H402Client) makeInitialRequest(ctx context.Context, method, url string, requestBody []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if requestBody != nil {
		bodyReader = bytes.NewBuffer(requestBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial request: %v", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	fmt.Println("Sending H402 request to:", url)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make initial request: %v", err)
	}

	return resp, nil
}

func (h *H402Client) SendH402RequestWithPaymentInfo(ctx context.Context, method, url string, requestBody []byte) (*H402ResponseWithPaymentInfo, error) {
	resp, err := h.makeInitialRequest(ctx, method, url, requestBody)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusPaymentRequired {
		return &H402ResponseWithPaymentInfo{Response: resp, PaymentInfo: nil, PayLimitExceeded: nil, InfoError: nil}, nil
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Println("H402 response body:", string(body))

	var h402Response H402Response
	if err := json.Unmarshal(body, &h402Response); err != nil {
		return nil, fmt.Errorf("failed to parse H402 response: %v", err)
	}

	if len(h402Response.Accepts) == 0 {
		return nil, fmt.Errorf("no payment requirements found in H402 response")
	}

	paymentReq := h402Response.Accepts[0]

	if payLimitError := h.checkPayLimits(paymentReq); payLimitError != nil {
		if err := h.waitForPayment(ctx, payLimitError); err != nil {
			return &H402ResponseWithPaymentInfo{
				Response:         resp,
				PaymentInfo:      nil,
				PayLimitExceeded: payLimitError,
				InfoError:        nil,
			}, nil
		}
	}

	walletType, err := MapNetworkIDToWalletType(paymentReq.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to map network ID to wallet type: %v", err)
	}

	wallet, exists := h.wallets[walletType]
	if !exists {
		return nil, fmt.Errorf("no wallet available for network ID %s (wallet type %s)", paymentReq.NetworkID, walletType)
	}

	transactionCreator, ok := wallet.(PaymentTransactionCreator)
	if !ok {
		return nil, fmt.Errorf("wallet type %s does not implement PaymentTransactionCreator", walletType)
	}

	paymentTx, signature, err := transactionCreator.CreatePaymentTransaction(ctx, paymentReq)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "insufficient funds") ||
			strings.Contains(errStr, "insufficient balance") ||
			strings.Contains(errStr, "not enough") ||
			strings.Contains(errStr, "exceeds balance") {
			return &H402ResponseWithPaymentInfo{
				Response:         resp,
				PaymentInfo:      nil,
				PayLimitExceeded: nil,
				InfoError:        fmt.Errorf("please mention this at the start of your next response: Agent does't have enough funds to pay for this request"),
			}, nil
		}
		return nil, fmt.Errorf("failed to create payment transaction: %v", err)
	}

	fmt.Println("Payment transaction:", paymentTx)

	payload := H402PaymentPayload{
		Type:      GetPaymentType(walletType),
		Signature: signature,
	}

	if walletType == coretypes.ServerWalletTypeBNB || walletType == coretypes.ServerWalletTypeBASE {
		payload.SignedTransaction = paymentTx
	} else {
		payload.Transaction = paymentTx
	}

	payment := H402Payment{
		H402Version: 1,
		Scheme:      paymentReq.Scheme,
		Namespace:   paymentReq.Namespace,
		NetworkID:   paymentReq.NetworkID,
		Resource:    paymentReq.Resource,
		Payload:     payload,
	}

	paymentJSON, err := json.Marshal(payment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment: %v", err)
	}
	paymentBase64 := base64.StdEncoding.EncodeToString(paymentJSON)

	req2, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create payment request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-PAYMENT", paymentBase64)
	req2.Header.Set("Access-Control-Expose-Headers", "X-PAYMENT-RESPONSE")

	fmt.Println("Final Payment request:", req2)
	fmt.Println("Wallet type:", wallet.GetWalletType())

	client := &http.Client{}
	finalResp, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("failed to make payment request: %v", err)
	}

	fmt.Println("Final Payment response:", finalResp)

	paymentInfo := &PaymentInfo{
		Amount:       paymentReq.AmountRequired,
		AmountFormat: paymentReq.AmountRequiredFormat,
		WalletType:   string(walletType),
		Transaction:  paymentTx,
		Namespace:    paymentReq.Namespace,
		TokenAddress: paymentReq.TokenAddress,
	}

	if paymentResponseHeader := finalResp.Header.Get("X-Payment-Response"); paymentResponseHeader != "" {
		if paymentResponseData, err := base64.StdEncoding.DecodeString(paymentResponseHeader); err == nil {
			var responseData H402PaymentResponseData
			if err := json.Unmarshal(paymentResponseData, &responseData); err == nil {
				paymentInfo.Transaction = responseData.Transaction
				paymentInfo.Namespace = responseData.Namespace
			}
		}
	}

	return &H402ResponseWithPaymentInfo{Response: finalResp, PaymentInfo: paymentInfo, PayLimitExceeded: nil, InfoError: nil}, nil
}

func (h *H402Client) SendH402RequestWithPaymentInfoAndWalletConnection(ctx context.Context, method, url string, requestBody []byte) (*H402ResponseWithPaymentInfo, error) {
	resp, err := h.makeInitialRequest(ctx, method, url, requestBody)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusPaymentRequired {
		return &H402ResponseWithPaymentInfo{Response: resp, PaymentInfo: nil, PayLimitExceeded: nil, InfoError: nil}, nil
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Println("H402 response body:", string(body))

	var h402Response H402Response
	if err := json.Unmarshal(body, &h402Response); err != nil {
		return nil, fmt.Errorf("failed to parse H402 response: %v", err)
	}

	if len(h402Response.Accepts) == 0 {
		return nil, fmt.Errorf("no payment requirements found in H402 response")
	}

	paymentRequests := make([]map[string]interface{}, len(h402Response.Accepts))
	for i, req := range h402Response.Accepts {
		// Get token decimals and symbol from chain config if not provided
		tokenDecimals := req.TokenDecimals
		tokenSymbol := req.TokenSymbol

		if tokenDecimals == 0 || tokenSymbol == "" {
			walletType, err := MapNetworkIDToWalletType(req.NetworkID)
			if err == nil {
				walletTypeStr := string(walletType)

				if tokenDecimals == 0 {
					tokenDecimals = getDecimalsForToken(req.TokenAddress, walletTypeStr)
				}
				if tokenSymbol == "" {
					tokenSymbol = detectTokenSymbolFromChainConfig(req.TokenAddress, walletTypeStr)
				}
			}
		}

		paymentRequests[i] = map[string]interface{}{
			"selectedRequestID":       uuid.New().String(),
			"namespace":               req.Namespace,
			"tokenAddress":            req.TokenAddress,
			"amountRequired":          req.AmountRequired,
			"amountRequiredFormat":    req.AmountRequiredFormat,
			"payToAddress":            req.PayToAddress,
			"networkId":               req.NetworkID,
			"description":             req.Description,
			"resource":                req.Resource,
			"scheme":                  req.Scheme,
			"mimeType":                req.MimeType,
			"estimatedProcessingTime": req.EstimatedProcessingTime,
			"tokenDecimals":           tokenDecimals,
			"tokenSymbol":             tokenSymbol,
		}
	}

	requestID := uuid.New()

	fmt.Println("User ID:", h.pool)

	userIDStr := h.pool.(*state.AgentPool).GetUserID()
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %v", err)
	}

	h402PendingRequest := models.H402PendingRequests{
		ID:        requestID,
		AgentID:   h.agentID,
		UserID:    userID,
		Status:    "Pending",
		CreatedAt: time.Now(),
	}

	if err := db.DB.Create(&h402PendingRequest).Error; err != nil {
		return nil, fmt.Errorf("failed to create h402 request: %v", err)
	}

	selectedRequestID, paymentHeader, err := h.waitForPaymentHeader(ctx, requestID, paymentRequests)

	if err != nil {
		return &H402ResponseWithPaymentInfo{
			Response:  resp,
			InfoError: err,
		}, nil
	}

	var paymentReq H402PaymentRequirement

	var found bool
	for _, req := range paymentRequests {
		if req["selectedRequestID"].(string) == selectedRequestID.String() {
			paymentReq = H402PaymentRequirement{
				Namespace:            req["namespace"].(string),
				TokenAddress:         req["tokenAddress"].(string),
				AmountRequired:       req["amountRequired"].(float64),
				AmountRequiredFormat: req["amountRequiredFormat"].(string),
				PayToAddress:         req["payToAddress"].(string),
				NetworkID:            req["networkId"].(string),
				Description:          req["description"].(string),
				Resource:             req["resource"].(string),
				Scheme:               req["scheme"].(string),
			}
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("payment request with ID %s not found", selectedRequestID.String())
	}

	walletType, err := MapNetworkIDToWalletType(paymentReq.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to map network ID to wallet type: %v", err)
	}

	req2, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create payment request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-PAYMENT", *paymentHeader)
	req2.Header.Set("Access-Control-Expose-Headers", "X-PAYMENT-RESPONSE")

	fmt.Println("Final Payment request:", req2)

	client := &http.Client{}
	finalResp, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("failed to make payment request: %v", err)
	}

	fmt.Println("Final Payment response:", finalResp)

	paymentInfo := &PaymentInfo{
		Amount:       paymentReq.AmountRequired,
		AmountFormat: paymentReq.AmountRequiredFormat,
		WalletType:   string(walletType),
		Transaction:  *paymentHeader,
		Namespace:    paymentReq.Namespace,
		TokenAddress: paymentReq.TokenAddress,
	}

	if paymentResponseHeader := finalResp.Header.Get("X-Payment-Response"); paymentResponseHeader != "" {
		if paymentResponseData, err := base64.StdEncoding.DecodeString(paymentResponseHeader); err == nil {
			var responseData H402PaymentResponseData
			if err := json.Unmarshal(paymentResponseData, &responseData); err == nil {
				paymentInfo.Transaction = responseData.Transaction
				paymentInfo.Namespace = responseData.Namespace
			}
		}
	}

	return &H402ResponseWithPaymentInfo{Response: finalResp, PaymentInfo: paymentInfo, PayLimitExceeded: nil, InfoError: nil}, nil
}

func (h *H402Client) SendH402Request(ctx context.Context, method, url string, requestBody []byte) (*http.Response, error) {
	result, err := h.SendH402RequestWithPaymentInfo(ctx, method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func detectTokenSymbolFromChainConfig(tokenAddress, walletType string) string {
	if tokenAddress == "11111111111111111111111111111111" {
		return "SOL"
	} else if tokenAddress == "0x0000000000000000000000000000000000000000" {
		switch walletType {
		case "BNB":
			return "BNB"
		case "BASE":
			return "ETH"
		}
	}

	var serverWalletType coretypes.ServerWalletType
	switch walletType {
	case "SOL":
		serverWalletType = coretypes.ServerWalletTypeSOL
	case "BNB":
		serverWalletType = coretypes.ServerWalletTypeBNB
	case "BASE":
		serverWalletType = coretypes.ServerWalletTypeBASE
	default:
		return ""
	}

	supportedTokens := coretypes.GetSupportedTokens(serverWalletType)

	for _, token := range supportedTokens {
		if strings.EqualFold(token.Address, tokenAddress) {
			return token.Symbol
		}
	}

	return ""
}

func convertSmallestUnitToHumanReadable(amount float64, tokenAddress, walletType string) float64 {
	decimals := getDecimalsForToken(tokenAddress, walletType)

	divisor := 1.0
	for i := 0; i < decimals; i++ {
		divisor *= 10.0
	}

	return amount / divisor
}

func formatAmountWithPrecision(amount float64, decimals int) string {
	dec := decimal.NewFromFloat(amount)

	rounded := dec.Round(int32(decimals))

	formatted := rounded.StringFixed(int32(decimals))
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")

	return formatted
}

func getDecimalsForToken(tokenAddress, walletType string) int {
	var serverWalletType coretypes.ServerWalletType
	switch walletType {
	case "SOL":
		serverWalletType = coretypes.ServerWalletTypeSOL
	case "BNB":
		serverWalletType = coretypes.ServerWalletTypeBNB
	case "BASE":
		serverWalletType = coretypes.ServerWalletTypeBASE
	default:
		return 18
	}

	if tokenAddress == "11111111111111111111111111111111" ||
		tokenAddress == "0x0000000000000000000000000000000000000000" {
		chainConfig := coretypes.GetDefaultChainConfig(serverWalletType)
		return chainConfig.Decimals
	} else {
		supportedTokens := coretypes.GetSupportedTokens(serverWalletType)
		for _, token := range supportedTokens {
			if strings.EqualFold(token.Address, tokenAddress) {
				return token.Decimals
			}
		}
		return 18
	}
}

func FormatPaymentMessage(paymentInfo *PaymentInfo) string {
	if paymentInfo == nil {
		return ""
	}

	walletName := strings.ToLower(paymentInfo.WalletType)

	currency := detectTokenSymbolFromChainConfig(paymentInfo.TokenAddress, paymentInfo.WalletType)

	switch paymentInfo.WalletType {
	case "SOL":
		walletName = "Solana"
	case "BNB":
		walletName = "BNB Smart Chain"
	case "BASE":
		walletName = "Base"
	}

	var displayAmount float64
	if paymentInfo.AmountFormat == "smallestUnit" {
		displayAmount = convertSmallestUnitToHumanReadable(paymentInfo.Amount, paymentInfo.TokenAddress, paymentInfo.WalletType)
	} else {
		displayAmount = paymentInfo.Amount
	}

	decimals := getDecimalsForToken(paymentInfo.TokenAddress, paymentInfo.WalletType)

	amountStr := formatAmountWithPrecision(displayAmount, decimals)

	return fmt.Sprintf("Please mention this at the start of your next response: **You have paid %s %s from the %s server wallet to access this page.**",
		amountStr, currency, walletName)
}

func FormatPaymentMessageWalletConnection(paymentInfo *PaymentInfo) string {
	if paymentInfo == nil {
		return ""
	}

	currency := detectTokenSymbolFromChainConfig(paymentInfo.TokenAddress, paymentInfo.WalletType)

	var displayAmount float64
	if paymentInfo.AmountFormat == "smallestUnit" {
		displayAmount = convertSmallestUnitToHumanReadable(paymentInfo.Amount, paymentInfo.TokenAddress, paymentInfo.WalletType)
	} else {
		displayAmount = paymentInfo.Amount
	}

	decimals := getDecimalsForToken(paymentInfo.TokenAddress, paymentInfo.WalletType)

	amountStr := formatAmountWithPrecision(displayAmount, decimals)

	return fmt.Sprintf("Please mention this at the start of your next response: **You have paid %s %s to access this page.**",
		amountStr, currency)
}

func FormatPayLimitMessage(payLimitError *PayLimitError) string {
	if payLimitError == nil {
		return ""
	}

	return fmt.Sprintf("%s You can either approve or cancel this payment.", payLimitError.Message)
}

func FormatPayLimitErrorMessage(payLimitError *PayLimitError) string {
	if payLimitError == nil {
		return ""
	}

	return fmt.Sprintf("Please mention this at the start of your next response: **%s You can update the pay limit in the agent server wallet settings.**", payLimitError.Message)
}

func GetPaymentType(walletType coretypes.ServerWalletType) string {
	switch walletType {
	case "SOL":
		return "signTransaction"
	case "BNB":
		return "signedTransaction"
	case "BASE":
		return "signedTransaction"
	default:
		return "signTransaction"
	}
}

func (h *H402Client) checkPayLimits(paymentReq H402PaymentRequirement) *PayLimitError {
	if len(h.payLimits) == 0 {
		return nil
	}

	tokenSymbol := h.getTokenSymbolFromPaymentReq(paymentReq)
	if tokenSymbol == "" {
		fmt.Printf("Warning: unable to determine token symbol for payment validation\n")
		return nil
	}

	limit, exists := h.payLimits[tokenSymbol]
	if !exists {
		fmt.Printf("No pay limit configured for token %s, allowing payment\n", tokenSymbol)
		return nil
	}

	paymentAmount := paymentReq.AmountRequired
	if paymentReq.AmountRequiredFormat == "smallestUnit" {
		walletType := h.getWalletTypeFromPaymentReq(paymentReq)
		paymentAmount = convertSmallestUnitToHumanReadable(paymentAmount, paymentReq.TokenAddress, string(walletType))
	}

	if paymentAmount > limit {
		walletType := h.getWalletTypeFromPaymentReq(paymentReq)
		decimals := getDecimalsForToken(paymentReq.TokenAddress, string(walletType))

		requestedAmountStr := formatAmountWithPrecision(paymentAmount, decimals)
		limitAmountStr := formatAmountWithPrecision(limit, decimals)

		return &PayLimitError{
			Message:         fmt.Sprintf("Payment blocked: The page is requesting %s %s, which exceeds the configured limit of %s %s.", requestedAmountStr, tokenSymbol, limitAmountStr, tokenSymbol),
			TokenSymbol:     tokenSymbol,
			RequestedAmount: paymentAmount,
			LimitAmount:     limit,
		}
	}

	fmt.Printf("Pay limit check passed: %.6f %s <= %.6f %s\n",
		paymentAmount, tokenSymbol, limit, tokenSymbol)
	return nil
}

func (h *H402Client) getTokenSymbolFromPaymentReq(paymentReq H402PaymentRequirement) string {
	walletType := h.getWalletTypeFromPaymentReq(paymentReq)
	return detectTokenSymbolFromChainConfig(paymentReq.TokenAddress, string(walletType))
}

func (h *H402Client) getWalletTypeFromPaymentReq(paymentReq H402PaymentRequirement) coretypes.ServerWalletType {
	walletType, err := MapNetworkIDToWalletType(paymentReq.NetworkID)
	if err != nil {
		return ""
	}
	return walletType
}

func (h *H402Client) waitForPayment(ctx context.Context, payLimitError *PayLimitError) error {
	if h.agentID == uuid.Nil {
		return fmt.Errorf("invalid agent ID: cannot wait for payment")
	}

	if err := h.setAgentPayLimitStatus("WAITING"); err != nil {
		return fmt.Errorf("failed to set agent pay limit status: %v", err)
	}

	if h.pool != nil {
		if agentPool, ok := h.pool.(*state.AgentPool); ok {
			manager := agentPool.GetManager(h.agentID.String())
			if manager != nil {
				manager.Send(
					sse.NewMessage(FormatPayLimitMessage(payLimitError)).WithEvent("request_payment_approval"))
			}
		}
	}

	fmt.Printf("Pay limit exceeded, set agent %s to payLimitStatus=WAITING, waiting for payment...\n", h.agentID)

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			if err := h.setAgentPayLimitStatus("CANCELLED"); err != nil {
				fmt.Printf("Failed to reset agent pay limit status after timeout: %v\n", err)
			}
			fmt.Printf("Payment timeout for agent %s, setting payLimitStatus=CANCELLED\n", h.agentID)
			return fmt.Errorf("payment timeout: %s", payLimitError.Message)
		case <-ticker.C:
			payLimitStatus, err := h.getAgentPayLimitStatus()
			if err != nil {
				fmt.Printf("Error checking agent pay limit status: %v\n", err)
				continue
			}
			if payLimitStatus == "APPROVED" {
				fmt.Printf("Payment approved %s \n", h.agentID)
				return nil
			}
			if payLimitStatus == "CANCELLED" {
				fmt.Printf("Payment cancelled %s \n", h.agentID)
				return fmt.Errorf("payment cancelled: %s", payLimitError.Message)
			}
		}
	}
}

func (h *H402Client) setAgentPayLimitStatus(payLimitStatus string) error {
	result := db.DB.Model(&models.Agent{}).Where("ID = ?", h.agentID).Update("payLimitStatus", payLimitStatus)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found with ID: %s", h.agentID)
	}
	return nil
}

func (h *H402Client) getAgentPayLimitStatus() (string, error) {
	var payLimitStatus string
	if err := db.DB.Model(&models.Agent{}).Where("ID = ?", h.agentID).Pluck("payLimitStatus", &payLimitStatus).Error; err != nil {
		return "", err
	}
	return payLimitStatus, nil
}

func (h *H402Client) waitForPaymentHeader(ctx context.Context, requestID uuid.UUID, paymentRequests []map[string]interface{}) (uuid.UUID, *string, error) {

	if h.pool != nil {
		if agentPool, ok := h.pool.(*state.AgentPool); ok {
			manager := agentPool.GetManager(h.agentID.String())
			if manager != nil {
				requestData := struct {
					RequestID       string                   `json:"requestId"`
					PaymentRequests []map[string]interface{} `json:"paymentRequests"`
				}{
					RequestID:       requestID.String(),
					PaymentRequests: paymentRequests,
				}

				paymentRequestsData, err := json.Marshal(requestData)
				if err != nil {
					return uuid.Nil, nil, fmt.Errorf("failed to marshal payment request: %v", err)
				}

				manager.Send(
					sse.NewMessage(string(paymentRequestsData)).WithEvent("request_payment_header"))
			}
		}
	}

	fmt.Printf("Waiting for payment header for request %s (agent %s)...\n", requestID, h.agentID)

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			if err := h.setH402RequestStatus(requestID, "CANCELLED"); err != nil {
				fmt.Printf("Failed to set request status to cancelled after timeout: %v\n", err)
			}
			fmt.Printf("Payment header timeout for request %s, setting status=Cancelled\n", requestID)
			return uuid.Nil, nil, fmt.Errorf("payment header timeout")
		case <-ticker.C:
			selectedRequestID, paymentHeader, status, err := h.getH402RequestPaymentHeader(requestID)
			if err != nil {
				fmt.Printf("Error checking payment header status: %v\n", err)
				continue
			}
			if status == "APPROVED" && selectedRequestID != uuid.Nil && paymentHeader != nil {
				fmt.Printf("Payment header approved for request %s\n", requestID)
				return selectedRequestID, paymentHeader, nil
			}
			if status == "CANCELLED" {
				fmt.Printf("Payment header cancelled for request %s\n", requestID)
				return uuid.Nil, nil, fmt.Errorf("payment header cancelled")
			}
		}
	}
}

func (h *H402Client) setH402RequestStatus(requestID uuid.UUID, status string) error {
	result := db.DB.Model(&models.H402PendingRequests{}).Where("ID = ?", requestID).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no request found with ID %s", requestID)
	}
	return nil
}

func (h *H402Client) getH402RequestPaymentHeader(requestID uuid.UUID) (uuid.UUID, *string, string, error) {
	var request models.H402PendingRequests
	if err := db.DB.Where("ID = ?", requestID).First(&request).Error; err != nil {
		return uuid.Nil, nil, "", err
	}
	if request.SelectedRequestID == nil {
		return uuid.Nil, nil, request.Status, nil
	}
	return *request.SelectedRequestID, request.PaymentHeader, request.Status, nil
}
