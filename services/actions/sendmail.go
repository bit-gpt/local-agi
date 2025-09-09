package actions

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func NewSendMail(config map[string]string) *SendMailAction {
	return &SendMailAction{
		username: config["username"],
		password: config["password"],
		email:    config["email"],
		smtpHost: config["smtpHost"],
		smtpPort: config["smtpPort"],
	}
}

type SendMailAction struct {
	username string
	password string
	email    string
	smtpHost string
	smtpPort string
}

func (a *SendMailAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		Message string `json:"message"`
		To      string `json:"to"`
		Subject string `json:"subject"`
	}{}
	err := params.Unmarshal(&result)
	if err != nil {
		fmt.Printf("error: %v", err)

		return types.ActionResult{}, err
	}

	// Create the email message with proper headers
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", a.email, result.To, result.Subject, result.Message)

	// Send email with proper SSL/TLS handling
	err = a.sendEmailWithTLS(result.To, []byte(message))
	if err != nil {
		return types.ActionResult{}, err
	}
	return types.ActionResult{Result: fmt.Sprintf("Email sent to %s", result.To)}, nil
}

// sendEmailWithTLS handles both SSL (port 465) and STARTTLS (port 587) connections
func (a *SendMailAction) sendEmailWithTLS(to string, message []byte) error {
	serverAddr := fmt.Sprintf("%s:%s", a.smtpHost, a.smtpPort)

	// Check if we're using SSL port (465) or STARTTLS port (587)
	if a.smtpPort == "465" {
		// SSL connection for port 465
		return a.sendWithSSL(serverAddr, to, message)
	} else {
		// STARTTLS connection for port 587 and others
		return a.sendWithSTARTTLS(serverAddr, to, message)
	}
}

// sendWithSSL handles SSL connections (port 465)
func (a *SendMailAction) sendWithSSL(serverAddr, to string, message []byte) error {
	// Create TLS connection
	tlsConfig := &tls.Config{
		ServerName: a.smtpHost,
	}

	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect via SSL: %v", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, a.smtpHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Quit()

	// Authenticate - use username (API key) instead of email for Mailjet
	auth := smtp.PlainAuth("", a.username, a.password, a.smtpHost)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %v", err)
	}

	// Send email
	if err = client.Mail(a.email); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %v", err)
	}

	_, err = w.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	return w.Close()
}

// sendWithSTARTTLS handles STARTTLS connections (port 587 and others)
func (a *SendMailAction) sendWithSTARTTLS(serverAddr, to string, message []byte) error {
	// Connect to server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, a.smtpHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Quit()

	// Start TLS if supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: a.smtpHost,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %v", err)
		}
	}

	// Authenticate - use username (API key) instead of email for Mailjet
	auth := smtp.PlainAuth("", a.username, a.password, a.smtpHost)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %v", err)
	}

	// Send email
	if err = client.Mail(a.email); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %v", err)
	}

	_, err = w.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	return w.Close()
}

func (a *SendMailAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "send_email",
		Description: "Send an email.",
		Properties: map[string]jsonschema.Definition{
			"to": {
				Type:        jsonschema.String,
				Description: "The email address to send the email to.",
			},
			"subject": {
				Type:        jsonschema.String,
				Description: "The subject of the email.",
			},
			"message": {
				Type:        jsonschema.String,
				Description: "The message to send.",
			},
		},
		Required: []string{"to", "subject", "message"},
	}
}

func (a *SendMailAction) Plannable() bool {
	return true
}

// SendMailConfigMeta returns the metadata for SendMail action configuration fields
func SendMailConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "smtpHost",
			Label:    "SMTP Host",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "SMTP server host (e.g., smtp.gmail.com)",
		},
		{
			Name:         "smtpPort",
			Label:        "SMTP Port",
			Type:         config.FieldTypeText,
			Required:     true,
			DefaultValue: "587",
			HelpText:     "SMTP server port (e.g., 587)",
		},
		{
			Name:     "username",
			Label:    "SMTP Username",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "SMTP username/email address",
		},
		{
			Name:     "password",
			Label:    "SMTP Password",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "SMTP password or app password",
		},
		{
			Name:     "email",
			Label:    "From Email",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "Sender email address",
		},
	}
}
