package actions

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/gmail"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailAddLabelToEmail(config map[string]string) *GmailAddLabelToEmailAction {
	return &GmailAddLabelToEmailAction{}
}

type GmailAddLabelToEmailAction struct{}

func (a *GmailAddLabelToEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		MessageID   string   `json:"message_id"`
		AddLabelIds []string `json:"add_label_ids"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.MessageID == "" {
		return types.ActionResult{}, fmt.Errorf("message_id is required")
	}

	if len(result.AddLabelIds) == 0 {
		return types.ActionResult{}, fmt.Errorf("add_label_ids is required and cannot be empty")
	}

	// Validate label IDs
	if err := validateLabelIds(result.AddLabelIds); err != nil {
		return types.ActionResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Verify labels exist before attempting to add them
	if err := verifyLabelsExist(ctx, gmailService, result.AddLabelIds); err != nil {
		return types.ActionResult{}, err
	}

	// Create the modify request
	modifyRequest := &gmailapi.ModifyMessageRequest{
		AddLabelIds: result.AddLabelIds,
	}

	// Apply the labels to the message
	modifiedMessage, err := gmailService.Users.Messages.Modify("me", result.MessageID, modifyRequest).Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, handleGmailError("failed to add labels to message", err)
	}

	if modifiedMessage == nil {
		return types.ActionResult{}, fmt.Errorf("message was not modified")
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Successfully added labels %s to message %s", strings.Join(result.AddLabelIds, ", "), result.MessageID),
	}, nil
}

func validateLabelIds(labelIds []string) error {
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

func isSystemLabel(labelId string) bool {
	systemLabels := map[string]bool{
		// Standard system labels
		"INBOX":     true,
		"SENT":      true,
		"DRAFT":     true,
		"SPAM":      true,
		"TRASH":     true,
		"UNREAD":    true,
		"STARRED":   true,
		"IMPORTANT": true,

		// Category labels
		"CATEGORY_PERSONAL":   true,
		"CATEGORY_SOCIAL":     true,
		"CATEGORY_PROMOTIONS": true,
		"CATEGORY_UPDATES":    true,
		"CATEGORY_FORUMS":     true,

		// Other system labels
		"CHAT":   true,
		"CHATS":  true,
		"ALL":    true,
		"OUTBOX": true,
	}

	return systemLabels[labelId]
}

func isValidCustomLabelId(labelId string) bool {
	// Custom labels should be alphanumeric with limited special characters
	// This is a basic validation - Gmail's actual rules might be more complex
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+$`, labelId)
	if err != nil {
		return false
	}
	return matched && len(labelId) > 0
}

func verifyLabelsExist(ctx context.Context, gmailService *gmailapi.Service, labelIds []string) error {
	// Get all labels for the user
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

	return nil
}

func handleGmailError(baseMessage string, err error) error {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "notFound") || strings.Contains(errStr, "not found"):
		return fmt.Errorf("%s: message or label not found", baseMessage)
	case strings.Contains(errStr, "permissionDenied") || strings.Contains(errStr, "permission denied"):
		return fmt.Errorf("%s: insufficient permissions", baseMessage)
	case strings.Contains(errStr, "quotaExceeded") || strings.Contains(errStr, "quota exceeded"):
		return fmt.Errorf("%s: Gmail API quota exceeded", baseMessage)
	case strings.Contains(errStr, "rateLimitExceeded") || strings.Contains(errStr, "rate limit"):
		return fmt.Errorf("%s: rate limit exceeded, please try again later", baseMessage)
	case strings.Contains(errStr, "badRequest") || strings.Contains(errStr, "bad request"):
		return fmt.Errorf("%s: invalid request parameters", baseMessage)
	default:
		return fmt.Errorf("%s: %v", baseMessage, err)
	}
}

func (a *GmailAddLabelToEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-add-label-to-email",
		Description: "Add one or more labels to an email message in Gmail. Labels help organize emails and can be used for filtering and searching. This action will verify that all specified labels exist before attempting to add them.",
		Properties: map[string]jsonschema.Definition{
			"message_id": {
				Type:        jsonschema.String,
				Description: "The ID of the email message to add labels to. This is the Gmail message ID, not the thread ID.",
			},
			"add_label_ids": {
				Type:        jsonschema.Array,
				Description: "Array of label IDs to add to the message. Can include system labels (INBOX, SENT, DRAFT, SPAM, TRASH, UNREAD, STARRED, IMPORTANT, CATEGORY_*) or custom label IDs. All labels must exist in the user's Gmail account.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
		},
		Required: []string{"message_id", "add_label_ids"},
	}
}

func (a *GmailAddLabelToEmailAction) Plannable() bool {
	return true
}

func GmailAddLabelToEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
