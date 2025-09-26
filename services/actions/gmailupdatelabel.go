package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailUpdateLabel(config map[string]string) *GmailUpdateLabelAction {
	return &GmailUpdateLabelAction{}
}

type GmailUpdateLabelAction struct{}

func (a *GmailUpdateLabelAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		ID                    string `json:"id"`
		Name                  string `json:"name"`
		MessageListVisibility string `json:"messageListVisibility,omitempty"`
		LabelListVisibility   string `json:"labelListVisibility,omitempty"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.ID == "" {
		return types.ActionResult{}, fmt.Errorf("id is required")
	}

	if result.Name == "" {
		return types.ActionResult{}, fmt.Errorf("name is required")
	}

	// Validate visibility settings
	if err := validateVisibilitySettings(result.MessageListVisibility, result.LabelListVisibility); err != nil {
		return types.ActionResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gmailService, err := oauth.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Create the label update request
	label := &gmailapi.Label{
		Id:                    result.ID,
		Name:                  result.Name,
		MessageListVisibility: result.MessageListVisibility,
		LabelListVisibility:   result.LabelListVisibility,
	}

	updatedLabel, err := gmailService.Users.Labels.Update("me", result.ID, label).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "Label not found") {
			return types.ActionResult{}, fmt.Errorf("label with ID '%s' not found", result.ID)
		}
		if strings.Contains(err.Error(), "already exists") ||
			strings.Contains(err.Error(), "Label name exists") ||
			strings.Contains(err.Error(), "duplicate") {
			return types.ActionResult{}, fmt.Errorf("label name '%s' already exists", result.Name)
		}
		return types.ActionResult{}, fmt.Errorf("failed to update label: %v", err)
	}

	if updatedLabel == nil {
		return types.ActionResult{}, fmt.Errorf("label was not updated")
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Label '%s' updated successfully with ID: %s", updatedLabel.Name, updatedLabel.Id),
	}, nil
}

func (a *GmailUpdateLabelAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-update-label",
		Description: "Update an existing label in Gmail. You can modify the label name and visibility settings.",
		Properties: map[string]jsonschema.Definition{
			"id": {
				Type:        jsonschema.String,
				Description: "The ID of the label to update",
			},
			"name": {
				Type:        jsonschema.String,
				Description: "The new name for the label",
			},
			"messageListVisibility": {
				Type:        jsonschema.String,
				Description: "Controls whether messages with this label are shown in the message list. 'show' displays them normally, 'hide' hides them from the inbox view",
				Enum:        []string{"show", "hide"},
			},
			"labelListVisibility": {
				Type:        jsonschema.String,
				Description: "Controls when the label appears in Gmail's label list sidebar. 'labelShow' always shows the label, 'labelShowIfUnread' only shows it when there are unread messages, 'labelHide' hides it from the sidebar",
				Enum:        []string{"labelShow", "labelShowIfUnread", "labelHide"},
			},
		},
		Required: []string{"id", "name"},
	}
}

func (a *GmailUpdateLabelAction) Plannable() bool {
	return true
}

func GmailUpdateLabelConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
