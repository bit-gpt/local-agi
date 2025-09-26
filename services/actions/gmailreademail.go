package actions

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailReadEmail(config map[string]string) *GmailReadEmailAction {
	return &GmailReadEmailAction{}
}

type GmailReadEmailAction struct{}

const (
	MaxBodyLength       = 50000 // 50,000 characters (~50-100KB depending on character encoding)
	MaxSubjectLength    = 500   // Email subjects shouldn't be this long anyway
	MaxHeaderLength     = 1000  // For from/to/cc fields
	MaxAttachments      = 10    // Reasonable limit for attachment list
	MaxAttachmentName   = 255   // Standard filename limit
	TruncationIndicator = "... [CONTENT TRUNCATED DUE TO LENGTH]"
)

type EmailAttachment struct {
	FileName     string `json:"file_name"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	AttachmentID string `json:"attachment_id"`
}

type EmailContent struct {
	MessageID   string            `json:"message_id"`
	ThreadID    string            `json:"thread_id,omitempty"`
	Subject     string            `json:"subject"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	CC          string            `json:"cc,omitempty"`
	BCC         string            `json:"bcc,omitempty"`
	Date        string            `json:"date"`
	Body        string            `json:"body"`
	BodyHTML    string            `json:"body_html,omitempty"`
	Attachments []EmailAttachment `json:"attachments,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	IsRead      bool              `json:"is_read"`
}

func (a *GmailReadEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
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

	message, err := gmailService.Users.Messages.Get("me", result.MessageID).Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get message: %v", err)
	}

	if message == nil {
		return types.ActionResult{}, fmt.Errorf("message not found")
	}

	emailContent, err := parseGmailMessage(message, gmailService, ctx)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to parse message: %v", err)
	}

	validateAndTruncateContent(emailContent)

	formattedResponse := formatEmailAsText(emailContent)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func parseGmailMessage(message *gmailapi.Message, gmailService *gmailapi.Service, ctx context.Context) (*EmailContent, error) {
	content := &EmailContent{
		MessageID: message.Id,
		ThreadID:  message.ThreadId,
		Labels:    message.LabelIds,
		IsRead:    !contains(message.LabelIds, "UNREAD"),
	}

	headers := make(map[string]string)
	if message.Payload != nil {
		for _, header := range message.Payload.Headers {
			headers[strings.ToLower(header.Name)] = header.Value
		}
	}

	content.Subject = headers["subject"]
	content.From = headers["from"]
	content.To = headers["to"]
	content.CC = headers["cc"]
	content.BCC = headers["bcc"]
	content.Date = headers["date"]

	if message.Payload != nil {
		err := parseMessageParts(message.Payload, content, gmailService, message.Id, ctx)
		if err != nil {
			return nil, err
		}
	}

	if content.Body == "" && content.BodyHTML != "" {
		content.Body = htmlToPlainText(content.BodyHTML)
	}

	return content, nil
}

func parseMessageParts(part *gmailapi.MessagePart, content *EmailContent, gmailService *gmailapi.Service, messageID string, ctx context.Context) error {
	if strings.HasPrefix(part.MimeType, "multipart/") {
		for _, subPart := range part.Parts {
			err := parseMessageParts(subPart, content, gmailService, messageID, ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if strings.HasPrefix(part.MimeType, "text/") {
		body, err := getPartBody(part)
		if err != nil {
			return err
		}

		if body != "" {
			switch part.MimeType {
			case "text/html":
				if content.BodyHTML == "" {
					content.BodyHTML = body
				}
			case "text/plain":
				if content.Body == "" {
					content.Body = body
				}
			}
		}
		return nil
	}

	if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
		attachment, err := getAttachmentDetails(part, gmailService, messageID, ctx)
		if err != nil {
			fmt.Printf("Warning: failed to get attachment details: %v\n", err)
		} else {
			content.Attachments = append(content.Attachments, *attachment)
		}
	}

	for _, subPart := range part.Parts {
		err := parseMessageParts(subPart, content, gmailService, messageID, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func getPartBody(part *gmailapi.MessagePart) (string, error) {
	if part.Body == nil || part.Body.Data == "" {
		return "", nil
	}

	decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode part body: %v", err)
	}

	return string(decoded), nil
}

func getAttachmentDetails(part *gmailapi.MessagePart, gmailService *gmailapi.Service, messageID string, ctx context.Context) (*EmailAttachment, error) {
	attachmentInfo, err := gmailService.Users.Messages.Attachments.Get("me", messageID, part.Body.AttachmentId).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment info: %v", err)
	}

	attachment := &EmailAttachment{
		FileName:     part.Filename,
		ContentType:  part.MimeType,
		Size:         attachmentInfo.Size,
		AttachmentID: part.Body.AttachmentId,
	}

	return attachment, nil
}

func formatEmailAsText(content *EmailContent) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Subject: %s\n", content.Subject))
	result.WriteString(fmt.Sprintf("From: %s\n", content.From))
	result.WriteString(fmt.Sprintf("To: %s\n", content.To))

	if content.CC != "" {
		result.WriteString(fmt.Sprintf("CC: %s\n", content.CC))
	}
	if content.BCC != "" {
		result.WriteString(fmt.Sprintf("BCC: %s\n", content.BCC))
	}

	result.WriteString(fmt.Sprintf("Date: %s\n", content.Date))

	if content.ThreadID != "" {
		result.WriteString(fmt.Sprintf("Thread ID: %s\n", content.ThreadID))
	}

	result.WriteString(fmt.Sprintf("Status: %s\n", getReadStatus(content.IsRead)))

	if len(content.Labels) > 0 {
		result.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(content.Labels, ", ")))
	}

	result.WriteString("\n")

	if content.Body != "" {
		result.WriteString("Email body content:\n")
		result.WriteString(content.Body)
	} else if content.BodyHTML != "" {
		result.WriteString("Email body content (converted from HTML):\n")
		result.WriteString(htmlToPlainText(content.BodyHTML))
	} else {
		result.WriteString("Email body content:\n[No text content found]")
	}

	result.WriteString("\n\n")

	if len(content.Attachments) > 0 {
		result.WriteString(fmt.Sprintf("Attachments (%d", len(content.Attachments)))

		// Check if we had to truncate attachments during validation
		// This is a simple way - in a more sophisticated approach, you might track this in the EmailContent struct
		if len(content.Attachments) == MaxAttachments {
			result.WriteString(" - may be truncated")
		}
		result.WriteString("):\n")

		for i, attachment := range content.Attachments {
			sizeStr := formatFileSize(attachment.Size)
			result.WriteString(fmt.Sprintf("%d. %s\n", i+1, attachment.FileName))
			result.WriteString(fmt.Sprintf("   Type: %s\n", attachment.ContentType))
			result.WriteString(fmt.Sprintf("   Size: %s\n", sizeStr))
			result.WriteString(fmt.Sprintf("   ID: %s\n", attachment.AttachmentID))
			if i < len(content.Attachments)-1 {
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

func validateAndTruncateContent(content *EmailContent) {
	if len(content.Subject) > MaxSubjectLength {
		content.Subject = truncateString(content.Subject, MaxSubjectLength)
	}

	content.From = truncateString(content.From, MaxHeaderLength)
	content.To = truncateString(content.To, MaxHeaderLength)
	content.CC = truncateString(content.CC, MaxHeaderLength)
	content.BCC = truncateString(content.BCC, MaxHeaderLength)

	if len(content.Body) > MaxBodyLength {
		content.Body = truncateString(content.Body, MaxBodyLength)
	}
	if len(content.BodyHTML) > MaxBodyLength {
		content.BodyHTML = truncateString(content.BodyHTML, MaxBodyLength)
	}

	if len(content.Attachments) > MaxAttachments {
		content.Attachments = content.Attachments[:MaxAttachments]
	}

	for i := range content.Attachments {
		if len(content.Attachments[i].FileName) > MaxAttachmentName {
			content.Attachments[i].FileName = truncateString(content.Attachments[i].FileName, MaxAttachmentName)
		}
	}
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}

	// Ensure we don't cut in the middle of a UTF-8 character
	if maxLength <= len(TruncationIndicator) {
		return TruncationIndicator[:maxLength]
	}

	truncateAt := maxLength - len(TruncationIndicator)

	// Find a valid UTF-8 boundary
	for truncateAt > 0 && !utf8.ValidString(s[:truncateAt]) {
		truncateAt--
	}

	// Try to truncate at word boundary for better readability
	if truncateAt > 50 { // Only try word boundary for reasonably long content
		lastSpace := strings.LastIndex(s[:truncateAt], " ")
		if lastSpace > truncateAt-100 { // Don't go too far back
			truncateAt = lastSpace
		}
	}

	return s[:truncateAt] + TruncationIndicator
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getReadStatus(isRead bool) string {
	if isRead {
		return "Read"
	}
	return "Unread"
}

func htmlToPlainText(htmlContent string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(htmlContent, "")

	text = html.UnescapeString(text)

	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (a *GmailReadEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-read-email",
		Description: "Read an email message from Gmail using OAuth authentication. Returns full email content including subject, sender, recipients, body, and attachment details. Content is automatically truncated if it exceeds limits.",
		Properties: map[string]jsonschema.Definition{
			"message_id": {
				Type:        jsonschema.String,
				Description: "The ID of the email message to read",
			},
		},
		Required: []string{"message_id"},
	}
}

func (a *GmailReadEmailAction) Plannable() bool {
	return true
}

func GmailReadEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
