package actions

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/mudler/LocalAGI/core/h402"
	"github.com/mudler/LocalAGI/core/state"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/sashabaranov/go-openai/jsonschema"
	"jaytaylor.com/html2text"
)

func NewBrowse(config map[string]string, pool *state.AgentPool) *BrowseAction {
	return &BrowseAction{
		pool: pool,
	}
}

func NewBrowseWithWallets(config map[string]string, wallets map[types.ServerWalletType]types.ServerWallet, payLimits map[string]float64, pool *state.AgentPool) *BrowseAction {
	return &BrowseAction{
		serverWallets: wallets,
		payLimits:     payLimits,
		pool:          pool,
	}
}

type BrowseAction struct {
	serverWallets map[types.ServerWalletType]types.ServerWallet
	payLimits     map[string]float64
	pool          *state.AgentPool
}

func (a *BrowseAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		URL string `json:"url"`
	}{}
	err := params.Unmarshal(&result)
	if err != nil {
		fmt.Printf("error: %v", err)
		return types.ActionResult{}, err
	}

	clientWrapper := h402.NewHTTPClientWrapper(h402.HTTPClientOptions{
		Timeout:       30 * time.Second,
		ForceHTTP1:    true,
		ServerWallets: a.serverWallets,
		PayLimits:     a.payLimits,
		AgentID:       sharedState.AgentID,
		Pool:          a.pool,
	})

	req, err := http.NewRequestWithContext(ctx, "GET", result.URL, nil)
	if err != nil {
		return types.ActionResult{}, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("ngrok-skip-browser-warning", "69420")

	respWithPaymentInfo, err := clientWrapper.DoWithPaymentInfo(req)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return types.ActionResult{}, err
	}

	if respWithPaymentInfo != nil && respWithPaymentInfo.PayLimitExceeded != nil {
		payLimitMessage := h402.FormatPayLimitErrorMessage(respWithPaymentInfo.PayLimitExceeded)
		fmt.Println("Pay limit exceeded:", payLimitMessage)
		return types.ActionResult{Result: payLimitMessage}, nil
	}

	if respWithPaymentInfo != nil && respWithPaymentInfo.InfoError != nil {
		infoMessage := fmt.Sprintf("%v", respWithPaymentInfo.InfoError)
		fmt.Println("Info error:", infoMessage)
		return types.ActionResult{Result: infoMessage}, nil
	}

	if respWithPaymentInfo == nil || respWithPaymentInfo.Response == nil {
		return types.ActionResult{}, fmt.Errorf("received nil response from HTTP client")
	}

	defer respWithPaymentInfo.Response.Body.Close()
	resp := respWithPaymentInfo.Response

	if resp.StatusCode >= 400 {
		return types.ActionResult{}, fmt.Errorf("website returned error %d: %s", resp.StatusCode, resp.Status)
	}

	pagebyte, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ActionResult{}, err
	}

	if len(pagebyte) < 100 {
		return types.ActionResult{}, fmt.Errorf("website returned insufficient content (likely blocked or error page)")
	}

	rendered, err := html2text.FromString(string(pagebyte), html2text.Options{
		PrettyTables: true,
	})

	if err != nil {
		return types.ActionResult{}, err
	}

	if len(rendered) < 50 {
		return types.ActionResult{}, fmt.Errorf("page content too short after conversion (likely JavaScript-only or blocked content)")
	}

	if len(rendered) > 32000 {
		rendered = rendered[:32000] + "\n\n[Content truncated to prevent overwhelming response...]"
	}

	resultMessage := fmt.Sprintf("Successfully browsed '%s':\n\n", result.URL)

	if respWithPaymentInfo != nil && respWithPaymentInfo.PaymentInfo != nil {
		var paymentMessage string
		if os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") == "true" {
			paymentMessage = h402.FormatPaymentMessage(respWithPaymentInfo.PaymentInfo)
		} else {
			paymentMessage = h402.FormatPaymentMessageWalletConnection(respWithPaymentInfo.PaymentInfo)
		}
		resultMessage += paymentMessage + "\n\n"
	}

	resultMessage += rendered

	return types.ActionResult{Result: resultMessage}, nil
}

func (a *BrowseAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "browse",
		Description: "Use this tool to visit an URL. It browse a website page and return the text content.",
		Properties: map[string]jsonschema.Definition{
			"url": {
				Type:        jsonschema.String,
				Description: "The website URL.",
			},
		},
		Required: []string{"url"},
	}
}

func (a *BrowseAction) Plannable() bool {
	return true
}
