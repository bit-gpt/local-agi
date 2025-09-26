package actions

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/gmail"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func NewGmailSearchEmails(config map[string]string) *GmailSearchEmailsAction {
	return &GmailSearchEmailsAction{}
}

type GmailSearchEmailsAction struct{}

type EmailSearchResult struct {
	MessageID string `json:"message_id"`
	Subject   string `json:"subject"`
	From      string `json:"from"`
	Date      string `json:"date"`
	IsRead    bool   `json:"is_read"`
}

func (a *GmailSearchEmailsAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		Query      string `json:"query"`
		MaxResults int    `json:"maxResults"`
	}{}

	err := params.Unmarshal(&result)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("error parsing parameters: %v", err)
	}

	if result.Query == "" {
		return types.ActionResult{}, fmt.Errorf("query is required")
	}

	if result.MaxResults <= 0 {
		result.MaxResults = 10 // Default to 10 results
	}

	if result.MaxResults > 50 {
		result.MaxResults = 50 // Gmail API limit
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	// Search for messages
	searchRequest := gmailService.Users.Messages.List("me").
		Q(result.Query).
		MaxResults(int64(result.MaxResults))

	searchResponse, err := searchRequest.Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to search messages: %v", err)
	}

	if searchResponse == nil || len(searchResponse.Messages) == 0 {
		return types.ActionResult{
			Result: fmt.Sprintf("No emails found matching the search criteria: \"%s\"", result.Query),
		}, nil
	}

	// Get basic message details for each result
	var searchResults []EmailSearchResult
	var errors []string

	for _, message := range searchResponse.Messages {
		msg, err := gmailService.Users.Messages.Get("me", message.Id).
			Format("metadata").
			MetadataHeaders("Subject", "From", "Date").
			Context(ctx).Do()
		if err != nil {
			errorMsg := fmt.Sprintf("failed to get message %s: %v", message.Id, err)
			errors = append(errors, errorMsg)
			log.Printf("Warning: %s", errorMsg)
			continue
		}

		emailResult := EmailSearchResult{
			MessageID: msg.Id,
			IsRead:    !contains(msg.LabelIds, "UNREAD"),
		}

		// Extract headers
		if msg.Payload != nil {
			for _, header := range msg.Payload.Headers {
				switch strings.ToLower(header.Name) {
				case "subject":
					emailResult.Subject = header.Value
				case "from":
					emailResult.From = header.Value
				case "date":
					emailResult.Date = header.Value
				}
			}
		}

		searchResults = append(searchResults, emailResult)
	}

	// If we couldn't retrieve any messages due to errors
	if len(searchResults) == 0 && len(errors) > 0 {
		return types.ActionResult{}, fmt.Errorf("failed to retrieve any message details: %s", strings.Join(errors, "; "))
	}

	formattedResponse := formatSearchResults(searchResults, result.Query, len(errors))

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatSearchResults(results []EmailSearchResult, query string, errorCount int) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("Found %d emails matching query: \"%s\"", len(results), query))

	if errorCount > 0 {
		response.WriteString(fmt.Sprintf(" (%d emails could not be retrieved)", errorCount))
	}
	response.WriteString("\n\n")

	for i, result := range results {
		readStatus := "Unread"
		if result.IsRead {
			readStatus = "Read"
		}

		response.WriteString(fmt.Sprintf("%d. Subject: %s\n", i+1, result.Subject))
		response.WriteString(fmt.Sprintf("   From: %s\n", result.From))
		response.WriteString(fmt.Sprintf("   Date: %s\n", result.Date))
		response.WriteString(fmt.Sprintf("   Message ID: %s\n", result.MessageID))
		response.WriteString(fmt.Sprintf("   Status: %s\n", readStatus))
		if i < len(results)-1 {
			response.WriteString("\n")
		}
	}

	return response.String()
}

func (a *GmailSearchEmailsAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-search-emails",
		Description: "Search for emails in Gmail using Gmail search syntax. Supports queries like 'from:sender@example.com', 'subject:meeting', 'has:attachment', 'after:2024/01/01', 'is:unread', etc. Returns email subjects with message IDs.",
		Properties: map[string]jsonschema.Definition{
			"query": {
				Type:        jsonschema.String,
				Description: "Gmail search query. Examples: 'from:sender@example.com', 'subject:meeting notes', 'has:attachment', 'after:2024/01/01', 'is:unread', 'label:work'",
			},
			"maxResults": {
				Type:        jsonschema.Number,
				Description: "Maximum number of results to return (default: 10, max: 100)",
			},
		},
		Required: []string{"query"},
	}
}

func (a *GmailSearchEmailsAction) Plannable() bool {
	return true
}

func GmailSearchEmailsConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
