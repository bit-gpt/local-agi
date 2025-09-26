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
)

func NewGmailListLabels(config map[string]string) *GmailListLabelsAction {
	return &GmailListLabelsAction{}
}

type GmailListLabelsAction struct{}

type Label struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	MessageListVisibility string `json:"messageListVisibility,omitempty"`
	LabelListVisibility   string `json:"labelListVisibility,omitempty"`
	Type                  string `json:"type,omitempty"`
	MessagesTotal         int64  `json:"messagesTotal,omitempty"`
	MessagesUnread        int64  `json:"messagesUnread,omitempty"`
	ThreadsTotal          int64  `json:"threadsTotal,omitempty"`
	ThreadsUnread         int64  `json:"threadsUnread,omitempty"`
}

func (a *GmailListLabelsAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gmailService, err := oauth.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Get all labels
	labelsResponse, err := gmailService.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to list labels: %v", err)
	}

	if labelsResponse == nil || len(labelsResponse.Labels) == 0 {
		return types.ActionResult{
			Result: "No labels found in Gmail account",
		}, nil
	}

	// Convert to our struct format
	var labels []Label
	for _, gmailLabel := range labelsResponse.Labels {
		label := Label{
			ID:                    gmailLabel.Id,
			Name:                  gmailLabel.Name,
			MessageListVisibility: gmailLabel.MessageListVisibility,
			LabelListVisibility:   gmailLabel.LabelListVisibility,
			Type:                  gmailLabel.Type,
			MessagesTotal:         gmailLabel.MessagesTotal,
			MessagesUnread:        gmailLabel.MessagesUnread,
			ThreadsTotal:          gmailLabel.ThreadsTotal,
			ThreadsUnread:         gmailLabel.ThreadsUnread,
		}
		labels = append(labels, label)
	}

	formattedResponse := formatLabelsResponse(labels)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatLabelsResponse(labels []Label) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("Found %d labels in Gmail account:\n\n", len(labels)))

	// Separate system labels from user labels
	var systemLabels []Label
	var userLabels []Label

	for _, label := range labels {
		if label.Type == "system" {
			systemLabels = append(systemLabels, label)
		} else {
			userLabels = append(userLabels, label)
		}
	}

	// Display system labels first
	if len(systemLabels) > 0 {
		response.WriteString("System Labels:\n")
		response.WriteString(strings.Repeat("-", 50) + "\n")
		for i, label := range systemLabels {
			response.WriteString(formatLabelDetails(label, i+1))
			if i < len(systemLabels)-1 {
				response.WriteString("\n")
			}
		}
		response.WriteString("\n\n")
	}

	// Display user labels
	if len(userLabels) > 0 {
		response.WriteString("User Labels:\n")
		response.WriteString(strings.Repeat("-", 50) + "\n")
		for i, label := range userLabels {
			response.WriteString(formatLabelDetails(label, i+1))
			if i < len(userLabels)-1 {
				response.WriteString("\n")
			}
		}
	}

	return response.String()
}

func formatLabelDetails(label Label, index int) string {
	var details strings.Builder

	details.WriteString(fmt.Sprintf("%d. %s\n", index, label.Name))
	details.WriteString(fmt.Sprintf("   ID: %s\n", label.ID))
	details.WriteString(fmt.Sprintf("   Type: %s\n", label.Type))

	if label.MessageListVisibility != "" {
		details.WriteString(fmt.Sprintf("   Message List Visibility: %s\n", label.MessageListVisibility))
	}
	if label.LabelListVisibility != "" {
		details.WriteString(fmt.Sprintf("   Label List Visibility: %s\n", label.LabelListVisibility))
	}

	// Only show counts for labels that have them
	if label.MessagesTotal > 0 || label.MessagesUnread > 0 {
		details.WriteString(fmt.Sprintf("   Messages: %d total, %d unread\n", label.MessagesTotal, label.MessagesUnread))
	}
	if label.ThreadsTotal > 0 || label.ThreadsUnread > 0 {
		details.WriteString(fmt.Sprintf("   Threads: %d total, %d unread\n", label.ThreadsTotal, label.ThreadsUnread))
	}

	return details.String()
}

func (a *GmailListLabelsAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-list-labels",
		Description: "List all labels in Gmail account. Returns both system labels (like INBOX, SENT, DRAFT) and user-created labels with their properties and message counts.",
		Properties:  map[string]jsonschema.Definition{},
		Required:    []string{},
	}
}

func (a *GmailListLabelsAction) Plannable() bool {
	return true
}

func GmailListLabelsConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
