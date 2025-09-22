package actions

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/gmail"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailSend(config map[string]string) *GmailSendAction {
	return &GmailSendAction{}
}

type GmailSendAction struct{}

func (a *GmailSendAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		To          string `json:"to"`
		Subject     string `json:"subject"`
		Body        string `json:"body"`
		ContentType string `json:"content_type,omitempty"`
		CC          string `json:"cc,omitempty"`
		BCC         string `json:"bcc,omitempty"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.To == "" {
		return types.ActionResult{}, fmt.Errorf("to address is required")
	}
	if result.Subject == "" {
		return types.ActionResult{}, fmt.Errorf("subject is required")
	}
	if result.Body == "" {
		return types.ActionResult{}, fmt.Errorf("body is required")
	}

	if !isValidEmail(result.To) {
		return types.ActionResult{}, fmt.Errorf("invalid 'to' email address format")
	}

	if err := validateEmailList(result.CC); err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid CC email: %v", err)
	}

	if err := validateEmailList(result.BCC); err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid BCC email: %v", err)
	}

	// Get Gmail client using the user's OAuth credentials
	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	message := createEmailMessage(result.To, result.Subject, result.Body, result.ContentType, result.CC, result.BCC)

	_, err = gmailService.Users.Messages.Send("me", message).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to send email: %v", err)
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Email sent successfully to %s", result.To),
	}, nil
}

func (a *GmailSendAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-send",
		Description: "Send an email using Gmail with OAuth authentication. Requires Gmail OAuth to be configured for the agent.",
		Properties: map[string]jsonschema.Definition{
			"to": {
				Type:        jsonschema.String,
				Description: "Recipient email address",
			},
			"subject": {
				Type:        jsonschema.String,
				Description: "Email subject line",
			},
			"body": {
				Type:        jsonschema.String,
				Description: "Email body content (plain text or HTML)",
			},
			"content_type": {
				Type:        jsonschema.String,
				Description: "Content type: 'html' for HTML emails, 'text' or empty for plain text (default: text)",
				Enum:        []string{"text", "html"},
			},
			"cc": {
				Type:        jsonschema.String,
				Description: "CC recipients (comma-separated email addresses, optional)",
			},
			"bcc": {
				Type:        jsonschema.String,
				Description: "BCC recipients (comma-separated email addresses, optional)",
			},
		},
		Required: []string{"to", "subject", "body"},
	}
}

func (a *GmailSendAction) Plannable() bool {
	return true
}

// createEmailMessage creates a Gmail message from the provided parameters
func createEmailMessage(to, subject, body, contentType, cc, bcc string) *gmailapi.Message {
	var message strings.Builder

	// Add headers
	message.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if cc != "" {
		message.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	if bcc != "" {
		message.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
	}
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	// Set content type based on parameter, default to plain text
	if contentType == "html" {
		message.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	} else {
		message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	}
	message.WriteString("\r\n")

	// Add body
	message.WriteString(body)

	// Encode message in base64url format
	raw := base64.URLEncoding.EncodeToString([]byte(message.String()))

	return &gmailapi.Message{
		Raw: raw,
	}
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func validateEmailList(emails string) error {
	if emails == "" {
		return nil
	}
	addresses := strings.Split(emails, ",")
	for _, addr := range addresses {
		if !isValidEmail(strings.TrimSpace(addr)) {
			return fmt.Errorf("invalid email address: %s", strings.TrimSpace(addr))
		}
	}
	return nil
}

func GmailSendConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
