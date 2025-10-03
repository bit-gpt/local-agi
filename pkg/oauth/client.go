package oauth

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/mudler/LocalAGI/db"
	models "github.com/mudler/LocalAGI/dbmodels"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type ServiceCreator[T any] func(*http.Client) (T, error)

type PlatformConfig struct {
	ClientIDEnv     string
	ClientSecretEnv string
	RedirectURLEnv  string
	Scopes          []string
	Endpoint        oauth2.Endpoint
}

var platformConfigs = map[string]PlatformConfig{
	models.PlatformGmail: {
		ClientIDEnv:     "GMAIL_CLIENT_ID",
		ClientSecretEnv: "GMAIL_CLIENT_SECRET",
		RedirectURLEnv:  "GMAIL_REDIRECT_URL",
		Scopes: []string{
			gmail.GmailSendScope,
			gmail.GmailComposeScope,
			gmail.GmailModifyScope,
			gmail.GmailReadonlyScope,
			gmail.GmailLabelsScope,
		},
		Endpoint: google.Endpoint,
	},
	models.PlatformGoogleCalendar: {
		ClientIDEnv:     "GMAIL_CLIENT_ID",
		ClientSecretEnv: "GMAIL_CLIENT_SECRET",
		RedirectURLEnv:  "GOOGLE_CALENDAR_REDIRECT_URL",
		Scopes: []string{
			calendar.CalendarScope,
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	},
}

func GetOAuthConfig(platform string) (*oauth2.Config, error) {
	config, exists := platformConfigs[platform]
	if !exists {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	clientID := os.Getenv(config.ClientIDEnv)
	clientSecret := os.Getenv(config.ClientSecretEnv)
	redirectURL := os.Getenv(config.RedirectURLEnv)

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("%s OAuth not configured. Please set environment variables", platform)
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       config.Scopes,
		Endpoint:     config.Endpoint,
	}, nil
}

func GetAuthenticatedClient(userID uuid.UUID, platform string) (*http.Client, error) {
	var oauthRecord models.OAuth
	err := db.DB.Where("UserID = ? AND Platform = ? AND IsActive = ?", userID, platform, true).First(&oauthRecord).Error
	if err != nil {
		return nil, fmt.Errorf("no active %s OAuth found for user: %v", platform, err)
	}

	if oauthRecord.IsTokenExpired() || oauthRecord.NeedsRefresh() {
		if err := refreshToken(&oauthRecord, platform); err != nil {
			return nil, fmt.Errorf("failed to refresh %s token: %v", platform, err)
		}
		if err := db.DB.Where("UserID = ? AND Platform = ? AND IsActive = ?", userID, platform, true).First(&oauthRecord).Error; err != nil {
			return nil, fmt.Errorf("failed to reload refreshed token: %v", err)
		}
	}

	config, err := GetOAuthConfig(platform)
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{
		AccessToken:  oauthRecord.AccessToken,
		RefreshToken: oauthRecord.RefreshToken,
		Expiry:       oauthRecord.TokenExpiry,
	}

	return config.Client(context.Background(), token), nil
}

func GetService[T any](userID uuid.UUID, platform string, serviceCreator ServiceCreator[T]) (T, error) {
	var zero T

	client, err := GetAuthenticatedClient(userID, platform)
	if err != nil {
		return zero, err
	}

	service, err := serviceCreator(client)
	if err != nil {
		return zero, fmt.Errorf("failed to create %s service: %v", platform, err)
	}

	return service, nil
}

func refreshToken(oauthRecord *models.OAuth, platform string) error {
	config, err := GetOAuthConfig(platform)
	if err != nil {
		return err
	}

	token := &oauth2.Token{
		AccessToken:  oauthRecord.AccessToken,
		RefreshToken: oauthRecord.RefreshToken,
		Expiry:       oauthRecord.TokenExpiry,
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

	return db.DB.Model(oauthRecord).Updates(updates).Error
}

func CreateGmailService(client *http.Client) (*gmail.Service, error) {
	return gmail.NewService(context.Background(), option.WithHTTPClient(client))
}

func CreateCalendarService(client *http.Client) (*calendar.Service, error) {
	return calendar.NewService(context.Background(), option.WithHTTPClient(client))
}

func GetGmailClient(userID uuid.UUID) (*gmail.Service, error) {
	return GetService(userID, models.PlatformGmail, CreateGmailService)
}

func GetCalendarClient(userID uuid.UUID) (*calendar.Service, error) {
	return GetService(userID, models.PlatformGoogleCalendar, CreateCalendarService)
}
