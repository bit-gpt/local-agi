package actions

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/gmail"
	"github.com/sashabaranov/go-openai/jsonschema"
	gmailapi "google.golang.org/api/gmail/v1"
)

func NewGmailSendEmail(config map[string]string) *GmailSendEmailAction {
	return &GmailSendEmailAction{}
}

type GmailSendEmailAction struct{}

type Attachment struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

func (a *GmailSendEmailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
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

	gmailService, err := gmail.GetGmailClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Gmail client: %v", err)
	}

	message, err := createEmailMessageWithAttachments(result.To, result.Subject, result.Body, result.ContentType, result.CC, result.BCC, result.Attachments)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to create email message: %v", err)
	}

	sentMessage, err := gmailService.Users.Messages.Send("me", message).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to send email: %v", err)
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Email sent successfully. Message ID: %s", sentMessage.Id),
	}, nil
}

func (a *GmailSendEmailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "gmail-send-email",
		Description: "Send an email using Gmail with OAuth authentication.",
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

func (a *GmailSendEmailAction) Plannable() bool {
	return true
}

func createEmailMessageWithAttachments(to, subject, body, contentType, cc, bcc string, attachments []Attachment) (*gmailapi.Message, error) {
	var message strings.Builder
	var boundary string

	if len(attachments) > 0 {
		boundary = "----=_Part_" + generateBoundary()

		message.WriteString(fmt.Sprintf("To: %s\r\n", to))
		if cc != "" {
			message.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
		}
		if bcc != "" {
			message.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
		}
		message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
		message.WriteString("MIME-Version: 1.0\r\n")
		message.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
		message.WriteString("\r\n")

		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		if contentType == "html" {
			message.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		} else {
			message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		}
		message.WriteString("\r\n")
		message.WriteString(body)
		message.WriteString("\r\n")

		for _, attachment := range attachments {
			err := addAttachmentToMessage(&message, boundary, attachment)
			if err != nil {
				return nil, fmt.Errorf("failed to add attachment %s: %v", attachment.URL, err)
			}
		}

		message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		message.WriteString(fmt.Sprintf("To: %s\r\n", to))
		if cc != "" {
			message.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
		}
		if bcc != "" {
			message.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
		}
		message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

		if contentType == "html" {
			message.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		} else {
			message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		}
		message.WriteString("\r\n")
		message.WriteString(body)
	}

	raw := base64.URLEncoding.EncodeToString([]byte(message.String()))

	return &gmailapi.Message{
		Raw: raw,
	}, nil
}

func addAttachmentToMessage(message *strings.Builder, boundary string, attachment Attachment) error {
	fileData, fileName, contentType, err := downloadFromURL(attachment.URL)
	if err != nil {
		return fmt.Errorf("failed to download from URL: %v", err)
	}

	if attachment.FileName != "" {
		fileName = attachment.FileName
	}

	if attachment.ContentType != "" {
		contentType = attachment.ContentType
	} else if contentType == "" {
		contentType = "application/octet-stream"
	}

	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
	message.WriteString("Content-Transfer-Encoding: base64\r\n")
	message.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", fileName))
	message.WriteString("\r\n")

	encoded := base64.StdEncoding.EncodeToString(fileData)

	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		message.WriteString(encoded[i:end] + "\r\n")
	}

	return nil
}

func downloadFromURL(fileURL string) ([]byte, string, string, error) {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, "", "", fmt.Errorf("unsupported URL scheme: %s (only http and https are supported)", parsedURL.Scheme)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(fileURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("failed to download file, HTTP status: %d %s", resp.StatusCode, resp.Status)
	}

	const maxSize = 25 * 1024 * 1024
	if resp.ContentLength > maxSize {
		return nil, "", "", fmt.Errorf("file too large (%.2f MB), Gmail limit is 25MB",
			float64(resp.ContentLength)/(1024*1024))
	}

	// For small files (< 1MB), use memory for efficiency
	// For larger files, use temp directory for scalability
	if resp.ContentLength > 0 && resp.ContentLength < 1024*1024 {
		limitedReader := io.LimitReader(resp.Body, maxSize+1)
		fileData, err := io.ReadAll(limitedReader)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to read response body: %v", err)
		}

		if len(fileData) > maxSize {
			return nil, "", "", fmt.Errorf("file too large (%.2f MB), Gmail limit is 25MB",
				float64(len(fileData))/(1024*1024))
		}

		fileName := extractFilenameFromURL(parsedURL, resp)
		contentType := getContentType(resp, fileName)

		if err := validateContentType(contentType, fileName); err != nil {
			return nil, "", "", err
		}

		return fileData, fileName, contentType, nil
	}

	return downloadToTempFile(resp, parsedURL)
}

func downloadToTempFile(resp *http.Response, parsedURL *url.URL) ([]byte, string, string, error) {
	tempFile, err := os.CreateTemp("", "gmail-attachment-*")
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	const maxSize = 25 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize+1)

	written, err := io.Copy(tempFile, limitedReader)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to download file: %v", err)
	}

	if written > maxSize {
		return nil, "", "", fmt.Errorf("file too large (%.2f MB), Gmail limit is 25MB",
			float64(written)/(1024*1024))
	}

	tempFile.Seek(0, 0)
	fileData, err := io.ReadAll(tempFile)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read temp file: %v", err)
	}

	fileName := extractFilenameFromURL(parsedURL, resp)
	contentType := getContentType(resp, fileName)

	return fileData, fileName, contentType, nil
}

func getContentType(resp *http.Response, fileName string) string {
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(fileName))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	return contentType
}

func validateContentType(contentType, fileName string) error {
	// Get file extension for additional validation
	ext := strings.ToLower(filepath.Ext(fileName))

	allowedContentTypes := map[string]bool{
		// Documents
		"application/pdf":    true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true, // .docx
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true, // .xlsx
		"application/vnd.ms-powerpoint":                                             true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true, // .pptx
		"text/plain": true,
		"text/csv":   true,
		"text/html":  true,
		"text/rtf":   true,

		// Images
		"image/jpeg":    true,
		"image/png":     true,
		"image/gif":     true,
		"image/webp":    true,
		"image/svg+xml": true,

		// Data formats
		"application/json": true,
		"application/xml":  true,
		"text/xml":         true,
	}

	if allowedContentTypes[contentType] {
		return nil
	}

	allowedExtensions := map[string]bool{
		// Documents
		".pdf": true,
		".doc": true, ".docx": true,
		".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true,
		".txt": true, ".csv": true, ".html": true, ".rtf": true,

		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true,

		// Data formats
		".json": true, ".xml": true,
	}

	if allowedExtensions[ext] {
		return nil
	}

	return fmt.Errorf("unsupported file type: %s (extension: %s). Only common document, image, and data formats are allowed", contentType, ext)
}

func extractFilenameFromURL(parsedURL *url.URL, resp *http.Response) string {
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		if strings.Contains(contentDisposition, "filename=") {
			parts := strings.Split(contentDisposition, "filename=")
			if len(parts) > 1 {
				filename := strings.Trim(parts[1], "\"")
				if filename != "" {
					return filename
				}
			}
		}
	}

	path := parsedURL.Path
	if path == "" || path == "/" {
		return "attachment"
	}

	filename := filepath.Base(path)
	if filename == "" || filename == "." {
		return "attachment"
	}

	return filename
}

func validateAttachments(attachments []Attachment) error {
	const maxAttachments = 10 // Limit to 10 attachments per email

	if len(attachments) > maxAttachments {
		return fmt.Errorf("too many attachments: %d (maximum allowed: %d)", len(attachments), maxAttachments)
	}

	for i, attachment := range attachments {
		if attachment.URL == "" {
			return fmt.Errorf("attachment %d: url is required", i+1)
		}

		if err := validateURL(attachment.URL); err != nil {
			return fmt.Errorf("attachment %d: %v", i+1, err)
		}
	}
	return nil
}

func validateURL(fileURL string) error {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http and https are supported)", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	return nil
}

func generateBoundary() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func validateEmailList(emails string) error {
	if emails == "" {
		return nil
	}
	addresses := strings.Split(emails, ",")
	for _, addr := range addresses {
		if !isValidEmail(strings.TrimSpace(addr)) {
			return fmt.Errorf("invalid email address: %s", strings.TrimSpace(addr))
		}
	}
	return nil
}

func GmailSendEmailConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
