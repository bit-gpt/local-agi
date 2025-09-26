package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailCreateDraftEmail(config map[string]string) *GmailCreateDraftEmailAction {
	return &GmailCreateDraftEmailAction{}
}

type GmailCreateDraftEmailAction struct{}

func (a *GmailCreateDraftEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		To          string       `json:"to"`
		Subject     string       `json:"subject"`
		Body        string       `json:"body"`
		ContentType string       `json:"content_type,omitempty"`
		CC          string       `json:"cc,omitempty"`
		BCC         string       `json:"bcc,omitempty"`
		Attachments []Attachment `json:"attachments,omitempty"`
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

	if err := validateAttachments(result.Attachments); err != nil {
		return types.ActionResult{}, fmt.Errorf("attachment validation failed: %v", err)
	}

	gmailService, err := oauth.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	message, err := createEmailMessageWithAttachments(result.To, result.Subject, result.Body, result.ContentType, result.CC, result.BCC, result.Attachments)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to create email message: %v", err)
	}

	draft := &gmailapi.Draft{
		Message: message,
	}

	createdDraft, err := gmailService.Users.Drafts.Create("me", draft).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to create draft: %v", err)
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Draft ID is %s\n\n", createdDraft.Id),
	}, nil
}

func (a *GmailCreateDraftEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-create-draft-email",
		Description: "Create a draft email using Gmail with OAuth authentication. The draft will be saved but not sent.",
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
			"attachments": {
				Type:        jsonschema.Array,
				Description: "Array of file attachments (optional, max 10) - downloads files from URLs",
				Items: &jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"url": {
							Type:        jsonschema.String,
							Description: "URL to download and attach as a file",
						},
						"file_name": {
							Type:        jsonschema.String,
							Description: "Override filename for the attachment (optional, auto-detected for URLs)",
						},
						"content_type": {
							Type:        jsonschema.String,
							Description: "Override MIME content type (optional, auto-detected if not provided)",
						},
					},
					Required: []string{"url"},
				},
			},
		},
		Required: []string{"to", "subject", "body"},
	}
}

func (a *GmailCreateDraftEmailAction) Plannable() bool {
	return true
}

func GmailCreateDraftEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
