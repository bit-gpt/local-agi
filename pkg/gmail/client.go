package gmail

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/mudler/LocalAGI/db"
	models "github.com/mudler/LocalAGI/dbmodels"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

// GetGmailClient returns an authenticated Gmail client for a user
func GetGmailClient(userID uuid.UUID) (*gmail.Service, error) {
	// 1. Get active Gmail OAuth for agent
	var gmailOAuth models.OAuth
	err := db.DB.Where("UserID = ? AND Platform = ? AND IsActive = ?", userID, models.PlatformGmail, true).First(&gmailOAuth).Error
	if err != nil {
		return nil, fmt.Errorf("no active Gmail OAuth found for user: %v", err)
	}

	// 2. Check if token needs refresh
	if gmailOAuth.IsTokenExpired() || gmailOAuth.NeedsRefresh() {
		if err := refreshGmailToken(&gmailOAuth); err != nil {
			return nil, fmt.Errorf("failed to refresh Gmail token: %v", err)
		}
		// Reload the updated token
		if err := db.DB.Where("UserID = ? AND Platform = ? AND IsActive = ?", userID, models.PlatformGmail, true).First(&gmailOAuth).Error; err != nil {
			return nil, fmt.Errorf("failed to reload refreshed token: %v", err)
		}
	}

	// 3. Create OAuth client
	config := getGmailOAuthConfig()
	token := &oauth2.Token{
		AccessToken:  gmailOAuth.AccessToken,
		RefreshToken: gmailOAuth.RefreshToken,
		Expiry:       gmailOAuth.TokenExpiry,
	}

	client := config.Client(context.Background(), token)

	// 4. Create Gmail service
	gmailService, err := gmail.New(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %v", err)
	}

	return gmailService, nil
}

func getGmailOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GMAIL_CLIENT_ID"),
		ClientSecret: os.Getenv("GMAIL_CLIENT_SECRET"),
		Scopes: []string{
			gmail.GmailSendScope,     // Send emails
			gmail.GmailComposeScope,  // Create and send drafts
			gmail.GmailModifyScope,   // Read, modify, create, and delete emails and labels
			gmail.GmailReadonlyScope, // Read emails and settings
			gmail.GmailLabelsScope,   // Manage labels
		},
		Endpoint: google.Endpoint,
	}
}

func refreshGmailToken(gmailOAuth *models.OAuth) error {
	config := getGmailOAuthConfig()

	token := &oauth2.Token{
		AccessToken:  gmailOAuth.AccessToken,
		RefreshToken: gmailOAuth.RefreshToken,
		Expiry:       gmailOAuth.TokenExpiry,
	}

	tokenSource := config.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}

	updates := map[string]interface{}{
		"AccessToken": newToken.AccessToken,
		"TokenExpiry": newToken.Expiry,
	}

	if newToken.RefreshToken != "" {
		updates["RefreshToken"] = newToken.RefreshToken
	}

	return db.DB.Model(gmailOAuth).Updates(updates).Error
}
