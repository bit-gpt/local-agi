package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/gmail"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailCreateLabel(config map[string]string) *GmailCreateLabelAction {
	return &GmailCreateLabelAction{}
}

type GmailCreateLabelAction struct{}

func (a *GmailCreateLabelAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		Name                  string `json:"name"`
		MessageListVisibility string `json:"messageListVisibility,omitempty"`
		LabelListVisibility   string `json:"labelListVisibility,omitempty"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
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

	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Create the label
	label := &gmailapi.Label{
		Name:                  result.Name,
		MessageListVisibility: result.MessageListVisibility,
		LabelListVisibility:   result.LabelListVisibility,
	}

	createdLabel, err := gmailService.Users.Labels.Create("me", label).Context(ctx).Do()
	if err != nil {
		if strings.Contains(err.Error(), "already exists") ||
			strings.Contains(err.Error(), "Label name exists") ||
			strings.Contains(err.Error(), "duplicate") {
			return types.ActionResult{}, fmt.Errorf("label '%s' already exists", result.Name)
		}
		return types.ActionResult{}, fmt.Errorf("failed to create label: %v", err)
	}

	if createdLabel == nil {
		return types.ActionResult{}, fmt.Errorf("label was not created")
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Label '%s' created successfully with ID: %s", createdLabel.Name, createdLabel.Id),
	}, nil
}

func validateVisibilitySettings(messageListVisibility, labelListVisibility string) error {
	validMessageListVisibility := map[string]bool{
		"show": true,
		"hide": true,
	}

	validLabelListVisibility := map[string]bool{
		"labelShow":         true,
		"labelShowIfUnread": true,
		"labelHide":         true,
	}

	if !validMessageListVisibility[messageListVisibility] {
		return fmt.Errorf("invalid messageListVisibility: %s. Valid values are: show, hide", messageListVisibility)
	}

	if !validLabelListVisibility[labelListVisibility] {
		return fmt.Errorf("invalid labelListVisibility: %s. Valid values are: labelShow, labelShowIfUnread, labelHide", labelListVisibility)
	}

	return nil
}

func (a *GmailCreateLabelAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-create-label",
		Description: "Create a new label in Gmail. Labels help organize emails and can be used for filtering and searching.",
		Properties: map[string]jsonschema.Definition{
			"name": {
				Type:        jsonschema.String,
				Description: "The name of the label to create",
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
		Required: []string{"name"},
	}
}

func (a *GmailCreateLabelAction) Plannable() bool {
	return true
}

func GmailCreateLabelConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
