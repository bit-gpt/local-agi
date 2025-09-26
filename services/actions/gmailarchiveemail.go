package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailArchiveEmail(config map[string]string) *GmailArchiveEmailAction {
	return &GmailArchiveEmailAction{}
}

type GmailArchiveEmailAction struct{}

func (a *GmailArchiveEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		MessageID string `json:"message_id"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.MessageID == "" {
		return types.ActionResult{}, fmt.Errorf("message_id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gmailService, err := oauth.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Archive the email by removing the INBOX label
	modifyRequest := &gmailapi.ModifyMessageRequest{
		RemoveLabelIds: []string{"INBOX"},
	}

	modifiedMessage, err := gmailService.Users.Messages.Modify("me", result.MessageID, modifyRequest).Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to archive message: %v", err)
	}

	if modifiedMessage == nil {
		return types.ActionResult{}, fmt.Errorf("message not found or could not be modified")
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Email with message ID %s has been successfully archived", result.MessageID),
	}, nil
}

func (a *GmailArchiveEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-archive-email",
		Description: "Archive an email message in Gmail by removing it from the inbox. The email will be moved to 'All Mail' and removed from the inbox view.",
		Properties: map[string]jsonschema.Definition{
			"message_id": {
				Type:        jsonschema.String,
				Description: "The ID of the email message to archive",
			},
		},
		Required: []string{"message_id"},
	}
}

func (a *GmailArchiveEmailAction) Plannable() bool {
	return true
}

func GmailArchiveEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
