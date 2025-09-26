package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mudler/LocalAGI/db"
	models "github.com/mudler/LocalAGI/dbmodels"
	"github.com/mudler/LocalAGI/pkg/oauth"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// Platform-specific OAuth configurations
func getPlatformOAuthConfig(platform string) (*oauth2.Config, error) {
	return oauth.GetOAuthConfig(platform)
}

// Platform-specific profile fetching
func getPlatformProfile(platform string, config *oauth2.Config, token *oauth2.Token) (email string, extraData map[string]interface{}, error error) {
	ctx := context.Background()
	client := config.Client(ctx, token)

	switch platform {
	case models.PlatformGmail:
		gmailService, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return "", nil, fmt.Errorf("failed to create Gmail service: %v", err)
		}

		profile, err := gmailService.Users.GetProfile("me").Do()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get Gmail profile: %v", err)
		}

		return profile.EmailAddress, nil, nil

	case models.PlatformGoogleCalendar:
		oauth2Service, err := googleoauth2.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return "", nil, fmt.Errorf("failed to create OAuth2 service: %v", err)
		}

		userInfo, err := oauth2Service.Userinfo.Get().Do()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get user info: %v", err)
		}

		return userInfo.Email, nil, nil

	case models.PlatformGithub:
		// TODO: Implement GitHub profile fetching
		return "", nil, fmt.Errorf("GitHub OAuth not yet implemented")
	default:
		return "", nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// Generic OAuth handlers
func (a *App) InitiateOAuth() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		platform := c.Params("platform")

		// 1. Get user ID from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		// 2. Get platform-specific OAuth config
		config, err := getPlatformOAuthConfig(platform)
		if err != nil {
			return errorJSONMessage(c, err.Error())
		}

		if config.ClientID == "" || config.ClientSecret == "" {
			return errorJSONMessage(c, fmt.Sprintf("%s OAuth not configured. Please set environment variables", platform))
		}

		// 3. Check if user already has OAuth configured for this platform
		var existingOAuth models.OAuth
		err = db.DB.Where("UserID = ? AND Platform = ? AND IsActive = ?", userIDStr, platform, true).First(&existingOAuth).Error
		if err == nil {
			return c.JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("%s OAuth already configured for this user", platform),
				"email":   existingOAuth.Email,
			})
		}

		// 4. Create redirect URL with user ID and platform
		redirectURL := strings.Replace(config.RedirectURL, "{id}", userIDStr, 1)
		redirectURL = strings.Replace(redirectURL, "{platform}", platform, 1)
		config.RedirectURL = redirectURL

		// 5. Generate state parameter for security
		state := fmt.Sprintf("%s:%s:%s", userIDStr, platform, uuid.New().String())

		// 6. Generate authorization URL
		authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

		return c.JSON(fiber.Map{
			"success":  true,
			"auth_url": authURL,
			"state":    state,
		})
	}
}

// Modified HandleOAuthCallback that just sends postMessage and closes
func (a *App) HandleOAuthCallback() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		platform := c.Params("platform")

		// 2. Get authorization code and state from query parameters
		authCode := c.Query("code")
		state := c.Query("state")
		errorParam := c.Query("error")

		if errorParam != "" {
			return a.renderCloseWindow(c, platform, false, fmt.Sprintf("OAuth error: %s", errorParam), "")
		}

		if authCode == "" {
			return a.renderCloseWindow(c, platform, false, "Authorization code missing", "")
		}

		if state == "" {
			return a.renderCloseWindow(c, platform, false, "State parameter missing", "")
		}

		// 3. Validate state parameter
		stateParts := strings.Split(state, ":")
		userIDStr := stateParts[0]

		fmt.Println("stateParts", stateParts)

		if len(stateParts) != 3 || stateParts[1] != platform {
			return a.renderCloseWindow(c, platform, false, "Invalid state parameter", "")
		}

		// 4. Get OAuth config and exchange code for token
		config, err := getPlatformOAuthConfig(platform)
		if err != nil {
			return a.renderCloseWindow(c, platform, false, err.Error(), "")
		}

		redirectURL := strings.Replace(config.RedirectURL, "{id}", userIDStr, 1)
		redirectURL = strings.Replace(redirectURL, "{platform}", platform, 1)
		config.RedirectURL = redirectURL

		ctx := context.Background()
		token, err := config.Exchange(ctx, authCode)
		if err != nil {
			return a.renderCloseWindow(c, platform, false, fmt.Sprintf("Failed to exchange authorization code: %v", err), "")
		}

		// 5. Get platform-specific profile information
		email, extraData, err := getPlatformProfile(platform, config, token)
		if err != nil {
			return a.renderCloseWindow(c, platform, false, err.Error(), "")
		}

		// 6. Deactivate any existing OAuth for this user and platform
		if err := db.DB.Model(&models.OAuth{}).
			Where("UserID = ? AND Platform = ?", userIDStr, platform).
			Update("IsActive", false).Error; err != nil {
			return a.renderCloseWindow(c, platform, false, "Failed to deactivate existing OAuth", "")
		}

		// 7. Prepare extra data JSON
		extraDataJSON, err := json.Marshal(extraData)
		if err != nil {
			return a.renderCloseWindow(c, platform, false, fmt.Sprintf("Failed to marshal extra data: %v", err), "")
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return a.renderCloseWindow(c, platform, false, "Invalid user ID", "")
		}

		// 8. Store OAuth credentials in database
		oauth := models.OAuth{
			UserID:       userID,
			Platform:     platform,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			TokenExpiry:  token.Expiry,
			Email:        email,
			IsActive:     true,
			Scopes:       strings.Join(config.Scopes, ","),
			ExtraData:    string(extraDataJSON),
		}

		if err := db.DB.Create(&oauth).Error; err != nil {
			return a.renderCloseWindow(c, platform, false, fmt.Sprintf("Failed to store OAuth credentials: %v", err), "")
		}

		// 9. Success - send postMessage and close
		return a.renderCloseWindow(c, platform, true, "OAuth configured successfully", email)
	}
}

// Minimal page that sends postMessage and closes
func (a *App) renderCloseWindow(c *fiber.Ctx, platform string, success bool, message string, email string) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
	<html>
	<head><title>OAuth Complete</title></head>
	<body>
	<script>
	if (window.opener) {
		window.opener.postMessage({
			type: 'OAUTH_COMPLETE',
			platform: '%s',
			success: %t,
			message: '%s',
			status: {
				connected: %t,
				email: '%s'
			}
		}, window.location.origin);
	}
	window.close();
	</script>
	</body>
	</html>`, platform, success, message, success, email)

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (a *App) GetOAuthStatus() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		platform := c.Params("platform")

		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		// 2. Check for active OAuth for this platform
		var oauth models.OAuth
		err := db.DB.Where("UserID = ? AND Platform = ? AND IsActive = ?", userIDStr, platform, true).First(&oauth).Error

		if err != nil {
			return c.JSON(fiber.Map{
				"success":     true,
				"connected":   false,
				"email":       nil,
				"token_valid": false,
			})
		}

		// 3. Check if token needs refresh
		needsRefresh := oauth.NeedsRefresh()
		isExpired := oauth.IsTokenExpired()

		return c.JSON(fiber.Map{
			"success":       true,
			"connected":     true,
			"email":         oauth.Email,
			"token_valid":   !isExpired,
			"needs_refresh": needsRefresh,
			"expires_at":    oauth.TokenExpiry,
		})
	}
}

func (a *App) DisconnectOAuth() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		platform := c.Params("platform")

		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		// 2. Find and deactivate OAuth for this platform
		result := db.DB.Model(&models.OAuth{}).
			Where("UserID = ? AND Platform = ? AND IsActive = ?", userIDStr, platform, true).
			Update("IsActive", false)

		if result.Error != nil {
			return errorJSONMessage(c, fmt.Sprintf("Failed to disconnect %s OAuth: %v", platform, result.Error))
		}

		if result.RowsAffected == 0 {
			return errorJSONMessage(c, fmt.Sprintf("No active %s OAuth connection found for this user", platform))
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("%s OAuth disconnected successfully", platform),
		})
	}
}
