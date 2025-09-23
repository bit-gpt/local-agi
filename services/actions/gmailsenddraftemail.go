package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/gmail"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailSendDraftEmail(config map[string]string) *GmailSendDraftEmailAction {
	return &GmailSendDraftEmailAction{}
}

type GmailSendDraftEmailAction struct{}

func (a *GmailSendDraftEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		DraftID string `json:"draft_id"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.DraftID == "" {
		return types.ActionResult{}, fmt.Errorf("draft_id is required")
	}

	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	_, err = gmailService.Users.Drafts.Get("me", result.DraftID).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get draft: %v", err)
	}

	sentMessage, err := gmailService.Users.Drafts.Send("me", &gmailapi.Draft{
		Id: result.DraftID,
	}).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to send draft: %v", err)
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Draft sent successfully. Message ID: %s", sentMessage.Id),
	}, nil
}

func (a *GmailSendDraftEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-send-draft-email",
		Description: "Send a draft email using Gmail with OAuth authentication. The draft will be sent and removed from drafts.",
		Properties: map[string]jsonschema.Definition{
			"draft_id": {
				Type:        jsonschema.String,
				Description: "The ID of the draft to send",
			},
		},
		Required: []string{"draft_id"},
	}
}

func (a *GmailSendDraftEmailAction) Plannable() bool {
	return true
}

func GmailSendDraftEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
