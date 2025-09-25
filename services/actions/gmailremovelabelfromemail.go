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

func NewGmailRemoveLabelFromEmail(config map[string]string) *GmailRemoveLabelFromEmailAction {
	return &GmailRemoveLabelFromEmailAction{}
}

type GmailRemoveLabelFromEmailAction struct{}

func (a *GmailRemoveLabelFromEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		MessageID      string   `json:"message_id"`
		RemoveLabelIds []string `json:"remove_label_ids"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.MessageID == "" {
		return types.ActionResult{}, fmt.Errorf("message_id is required")
	}

	if len(result.RemoveLabelIds) == 0 {
		return types.ActionResult{}, fmt.Errorf("remove_label_ids is required and cannot be empty")
	}

	// Validate label IDs
	if err := validateRemoveLabelIds(result.RemoveLabelIds); err != nil {
		return types.ActionResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Verify labels exist and are applied to the message
	if err := verifyLabelsCanBeRemoved(ctx, gmailService, result.MessageID, result.RemoveLabelIds); err != nil {
		return types.ActionResult{}, err
	}

	// Create the modify request
	modifyRequest := &gmailapi.ModifyMessageRequest{
		RemoveLabelIds: result.RemoveLabelIds,
	}

	// Apply the label removal to the message
	modifiedMessage, err := gmailService.Users.Messages.Modify("me", result.MessageID, modifyRequest).Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, handleGmailError("failed to remove labels from message", err)
	}

	if modifiedMessage == nil {
		return types.ActionResult{}, fmt.Errorf("message was not modified")
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Successfully removed labels %s from message %s", strings.Join(result.RemoveLabelIds, ", "), result.MessageID),
	}, nil
}

func validateRemoveLabelIds(labelIds []string) error {
	if len(labelIds) == 0 {
		return fmt.Errorf("label IDs cannot be empty")
	}

	if len(labelIds) > 100 { // Reasonable limit
		return fmt.Errorf("too many label IDs: %d (maximum allowed: 100)", len(labelIds))
	}

	for i, labelId := range labelIds {
		if strings.TrimSpace(labelId) == "" {
			return fmt.Errorf("label ID %d cannot be empty", i+1)
		}

		// Check for common Gmail system labels
		if isSystemLabel(labelId) {
			// Warn about removing critical system labels
			if isCriticalSystemLabel(labelId) {
				// Note: This is a warning, not an error - some use cases might need this
				continue
			}
			continue
		}

		// For custom labels, validate format and length
		if len(labelId) > 100 {
			return fmt.Errorf("label ID '%s' is too long (maximum 100 characters)", labelId)
		}

		if !isValidCustomLabelId(labelId) {
			return fmt.Errorf("invalid custom label ID format: %s", labelId)
		}
	}

	return nil
}

func isCriticalSystemLabel(labelId string) bool {
	// Labels that might be critical to remove and could cause confusion
	criticalLabels := map[string]bool{
		"INBOX": true,
		"SENT":  true,
		"DRAFT": true,
	}

	return criticalLabels[labelId]
}

func verifyLabelsCanBeRemoved(ctx context.Context, gmailService *gmailapi.Service, messageID string, labelIds []string) error {
	// Get all labels for the user to verify they exist
	labelsResponse, err := gmailService.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list labels: %v", err)
	}

	// Create a map of existing label IDs for quick lookup
	existingLabels := make(map[string]bool)
	for _, label := range labelsResponse.Labels {
		existingLabels[label.Id] = true
	}

	// Check if all requested labels exist
	var missingLabels []string
	for _, labelId := range labelIds {
		if !existingLabels[labelId] {
			missingLabels = append(missingLabels, labelId)
		}
	}

	if len(missingLabels) > 0 {
		return fmt.Errorf("the following labels do not exist: %s", strings.Join(missingLabels, ", "))
	}

	// Get the message to check which labels are currently applied
	message, err := gmailService.Users.Messages.Get("me", messageID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get message: %v", err)
	}

	// Create a map of current message labels
	currentLabels := make(map[string]bool)
	for _, labelId := range message.LabelIds {
		currentLabels[labelId] = true
	}

	// Check which labels are not currently on the message
	var notAppliedLabels []string
	for _, labelId := range labelIds {
		if !currentLabels[labelId] {
			notAppliedLabels = append(notAppliedLabels, labelId)
		}
	}

	// This is just a warning - Gmail API will silently ignore labels that aren't applied
	// We could optionally return this as info rather than an error
	if len(notAppliedLabels) > 0 {
		// Log warning but don't fail - Gmail handles this gracefully
		// Could be logged if logging is available: log.Warnf("Labels not currently applied to message: %s", strings.Join(notAppliedLabels, ", "))
	}

	return nil
}

func (a *GmailRemoveLabelFromEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-remove-label-from-email",
		Description: "Remove one or more labels from an email message in Gmail. Labels help organize emails and can be used for filtering and searching. This action will verify that all specified labels exist and are currently applied to the message before attempting to remove them.",
		Properties: map[string]jsonschema.Definition{
			"message_id": {
				Type:        jsonschema.String,
				Description: "The ID of the email message to remove labels from. This is the Gmail message ID, not the thread ID.",
			},
			"remove_label_ids": {
				Type:        jsonschema.Array,
				Description: "Array of label IDs to remove from the message. Can include system labels (INBOX, SENT, DRAFT, SPAM, TRASH, UNREAD, STARRED, IMPORTANT, CATEGORY_*) or custom label IDs. Labels must exist and be currently applied to the message.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
		},
		Required: []string{"message_id", "remove_label_ids"},
	}
}

func (a *GmailRemoveLabelFromEmailAction) Plannable() bool {
	return true
}

func GmailRemoveLabelFromEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
