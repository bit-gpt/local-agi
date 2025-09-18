package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mudler/LocalAGI/core/conversations"
	"github.com/mudler/LocalAGI/core/serverwallet"
	coreTypes "github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/db"
	models "github.com/mudler/LocalAGI/dbmodels"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/llm"
	"github.com/mudler/LocalAGI/pkg/utils"
	"github.com/mudler/LocalAGI/pkg/xlog"
	"github.com/mudler/LocalAGI/services"
	"github.com/mudler/LocalAGI/webui/types"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/mudler/LocalAGI/core/sse"
	"github.com/mudler/LocalAGI/core/state"

	"bytes"
	"io"

	"github.com/donseba/go-htmx"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"gorm.io/gorm"
)

// ValidationError represents a validation error with optional section switching info
type ValidationError struct {
	Message string `json:"message"`
	Section string `json:"section,omitempty"`
}

func (e ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) ValidationError {
	return ValidationError{Message: message}
}

func NewValidationErrorWithSection(message, section string) ValidationError {
	return ValidationError{Message: message, Section: section}
}

var (
	verificationKey string
	privyAppId      string
	privyApiKey     string
)

func init() {
	_ = godotenv.Load()
	var rawKey = os.Getenv("PRIVY_PUBLIC_KEY_PEM")
	verificationKey = strings.ReplaceAll(rawKey, `\n`, "\n")
	privyAppId = os.Getenv("PRIVY_APP_ID")
	privyApiKey = os.Getenv("PRIVY_APP_SECRET")
}

type (
	App struct {
		UserPools map[string]*state.AgentPool
		htmx      *htmx.HTMX
		config    *Config
		*fiber.App
		sharedState *coreTypes.AgentSharedState
	}
)

func NewApp(opts ...Option) *App {
	config := NewConfig(opts...)
	engine := html.NewFileSystem(http.FS(viewsfs), ".html")

	// Initialize a new Fiber app
	// Pass the engine to the Views
	webapp := fiber.New(fiber.Config{
		Views: engine,
	})

	a := &App{
		UserPools:   make(map[string]*state.AgentPool),
		htmx:        htmx.New(),
		config:      config,
		App:         webapp,
		sharedState: coreTypes.NewAgentSharedState(5 * time.Minute),
	}

	a.registerRoutes(webapp)

	return a
}

func (a *App) Notify(pool *state.AgentPool) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		payload := struct {
			Message string `form:"message"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			return err
		}

		query := payload.Message
		if query == "" {
			_, _ = c.Write([]byte("Please enter a message."))
			return nil
		}

		a := pool.GetAgent(c.Params("name"))
		a.Ask(
			coreTypes.WithText(query),
		)
		_, _ = c.Write([]byte("Message sent"))

		return nil
	}
}

func (a *App) Delete() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID and agent from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		agentId := agent.ID.String()

		// 2. Archive in DB (soft delete)
		if err := db.DB.
			Model(&models.Agent{}).
			Where("ID = ?", agent.ID).
			Update("archive", true).Error; err != nil {
			return errorJSONMessage(c, "Failed to archive agent in DB: "+err.Error())
		}

		// 3. Remove from in-memory pool if exists
		if pool, ok := a.UserPools[userIDStr]; ok {
			if err := pool.Remove(agentId); err != nil {
				xlog.Warn("Agent archived in DB but failed to remove from memory", "error", err)
			}
		}

		xlog.Info("Agent archived", "user", userIDStr, "agent", agentId)
		return statusJSONMessage(c, "ok")
	}
}

func errorJSONMessage(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusInternalServerError).JSON(struct {
		Error string `json:"error"`
	}{Error: message})
}

func errorJSONMessageWithValidation(c *fiber.Ctx, err error) error {
	if validationErr, ok := err.(ValidationError); ok {
		response := struct {
			Error   string `json:"error"`
			Section string `json:"section,omitempty"`
		}{
			Error:   validationErr.Message,
			Section: validationErr.Section,
		}
		return c.Status(http.StatusInternalServerError).JSON(response)
	}
	return errorJSONMessage(c, err.Error())
}

func statusJSONMessage(c *fiber.Ctx, message string) error {
	return c.JSON(struct {
		Status string `json:"status"`
	}{Status: message})
}

func (a *App) Pause() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID and agent from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		agentId := agent.ID.String()

		// 2. Get or init pool
		pool, ok := a.UserPools[userIDStr]
		if !ok {
			// Rehydrate pool from DB (no file fallback)
			newPool, err := state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to load agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = newPool
			pool = newPool
		}

		// 3. Pause agent if exists in memory
		if agentInstance := pool.GetAgent(agentId); agentInstance != nil {
			xlog.Info("Pausing agent", "Id", agentId)
			agentInstance.Pause()
		} else {
			return errorJSONMessage(c, "Agent is not active in memory")
		}

		return statusJSONMessage(c, "ok")
	}
}

func (a *App) Start() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID and agent from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		agentId := agent.ID.String()

		// 2. Load or create in-memory pool
		pool, ok := a.UserPools[userIDStr]
		if !ok {
			newPool, err := state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to load agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = newPool
			pool = newPool
		}

		// 3. Try to get the agent from memory
		agentInstance := pool.GetAgent(agentId)

		// 4. If agent not in memory, load from context agent config
		if agentInstance == nil {
			// Parse agent config
			var config state.AgentConfig
			if err := json.Unmarshal(agent.Config, &config); err != nil {
				return errorJSONMessage(c, "Failed to parse agent config: "+err.Error())
			}

			if config.ServerWallets == nil && os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") == "true" {
				walletsConfig, err := serverwallet.GenerateDefaultServerWalletsConfig()
				if err != nil {
					return errorJSONMessage(c, "Failed to create wallets: "+err.Error())
				}
				config.ServerWallets = walletsConfig

				// Update the agent config in the database with the new wallets
				configJSON, err := json.Marshal(config)
				if err != nil {
					return errorJSONMessage(c, "Failed to serialize config with wallets: "+err.Error())
				}
				agent.Config = configJSON
				if err := db.DB.Save(&agent).Error; err != nil {
					return errorJSONMessage(c, "Failed to update agent config in DB: "+err.Error())
				}
			}

			// Create agent in memory
			if err := pool.CreateAgent(agentId, &config); err != nil {
				return errorJSONMessage(c, "Failed to create agent in memory: "+err.Error())
			}

			// Get the newly created agent
			agentInstance = pool.GetAgent(agentId)
			if agentInstance == nil {
				return errorJSONMessage(c, "Failed to get newly created agent")
			}
		}

		// 5. Resume agent
		xlog.Info("Starting agent", "id", agentId)
		agentInstance.Resume()

		return statusJSONMessage(c, "ok")
	}
}

func (a *App) Create() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return errorJSONMessage(c, "Invalid user ID")
		}

		var config state.AgentConfig
		if err := c.BodyParser(&config); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		if err := validateAgentConfig(&config); err != nil {
			return errorJSONMessageWithValidation(c, err)
		}

		if err := validateModel(config.Model); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		if config.MultimodalModel == "" {
			config.MultimodalModel = os.Getenv("LOCALAGI_MULTIMODAL_MODEL")
		}
		if config.LocalRAGURL == "" {
			config.LocalRAGURL = os.Getenv("LOCALAGI_LOCALRAG_URL")
		}
		if config.LocalRAGAPIKey == "" {
			config.LocalRAGAPIKey = os.Getenv("LOCALAGI_LOCALRAG_API_KEY")
		}

		if os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") == "true" {
			config.PayLimits = coreTypes.GetDefaultPayLimits()

			walletsConfig, err := serverwallet.GenerateDefaultServerWalletsConfig()
			if err != nil {
				return errorJSONMessage(c, "Failed to create wallets: "+err.Error())
			}
			config.ServerWallets = walletsConfig
		}

		configJSON, err := json.Marshal(config)
		if err != nil {
			return errorJSONMessage(c, "Failed to serialize config")
		}

		id := uuid.New()
		agent := models.Agent{
			ID:     id,
			UserID: userID,
			Name:   config.Name,
			Config: configJSON,
		}

		if err := db.DB.Create(&agent).Error; err != nil {
			return errorJSONMessage(c, "Failed to store agent: "+err.Error())
		}

		var pool *state.AgentPool
		if p, ok := a.UserPools[userIDStr]; ok {
			pool = p
		} else {
			pool, err = state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to create agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = pool
		}

		if err := pool.CreateAgent(id.String(), &config); err != nil {
			return errorJSONMessage(c, "Failed to initialize agent: "+err.Error())
		}

		return statusJSONMessage(c, "ok")
	}
}

func (a *App) GetTemplates() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		var allTemplates = services.GetAllTemplates()

		var templates []fiber.Map
		for _, template := range allTemplates {
			templates = append(templates, fiber.Map{
				"id":          template.ID,
				"name":        template.Name,
				"description": template.Description,
				"category":    template.Category,
				"icon":        template.Icon,
			})
		}

		return c.JSON(fiber.Map{
			"success":   true,
			"templates": templates,
		})
	}
}

func (a *App) GetTemplateConfig() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		templateId := c.Params("templateId")

		template, err := services.GetTemplate(templateId)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{
				"success": false,
				"error":   "Template not found: " + err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"config":  template.Config,
		})
	}
}

func (a *App) GetAgentConfig() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		var config state.AgentConfig
		if err := json.Unmarshal(agent.Config, &config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to parse agent config",
				"success": false,
			})
		}

		// Remove wallets from agent config response entirely
		configBytes, err := json.Marshal(config)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to serialize config",
				"success": false,
			})
		}

		var configMap map[string]interface{}
		if err := json.Unmarshal(configBytes, &configMap); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to parse config map",
				"success": false,
			})
		}

		// Remove wallets entirely from the config response
		delete(configMap, "server_wallets")

		if os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") == "true" {
			configMap["server_wallets_enabled"] = true
		} else {
			configMap["server_wallets_enabled"] = false
		}

		return c.JSON(configMap)
	}
}

func (a *App) GetAgentServerWallets() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		if os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") != "true" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Server wallets are not enabled",
				"success": false,
			})
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		var config state.AgentConfig
		if err := json.Unmarshal(agent.Config, &config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to parse agent config",
				"success": false,
			})
		}

		ctx := context.Background()
		serverWalletResults := make([]map[string]interface{}, 0)

		for _, serverWalletConfig := range config.ServerWallets {
			serverWalletInfo := map[string]interface{}{
				"type":        serverWalletConfig.Type,
				"address":     serverWalletConfig.Address,
				"private_key": serverWalletConfig.PrivateKey,
			}

			serverWalletInstance, err := serverwallet.NewServerWallet(serverWalletConfig)
			if err != nil {
				serverWalletInfo["error"] = fmt.Sprintf("Failed to create server wallet: %v", err)
				serverWalletResults = append(serverWalletResults, serverWalletInfo)
				continue
			}

			nativeBalance, err := serverWalletInstance.GetBalance(ctx)
			if err != nil {
				serverWalletInfo["balance_error"] = fmt.Sprintf("Failed to get native balance: %v", err)
			} else {
				chainConfig := coreTypes.GetDefaultChainConfig(serverWalletInstance.GetWalletType())
				nativeBalanceFormatted := serverwallet.FormatBalance(nativeBalance, chainConfig.Decimals)
				serverWalletInfo["balance"] = nativeBalanceFormatted
				serverWalletInfo["currency"] = chainConfig.Symbol
			}

			tokenBalances, err := serverWalletInstance.GetAllTokenBalances(ctx)
			if err != nil {
				serverWalletInfo["token_balances_error"] = fmt.Sprintf("Failed to get token balances: %v", err)
			} else {
				tokenBalanceArray := make([]map[string]interface{}, 0)
				chainConfig := coreTypes.GetDefaultChainConfig(serverWalletInstance.GetWalletType())

				for tokenSymbol, balance := range tokenBalances {
					var tokenDecimals int = 18
					for _, token := range chainConfig.SupportedTokens {
						if token.Symbol == tokenSymbol {
							tokenDecimals = token.Decimals
							break
						}
					}

					tokenBalanceFormatted := serverwallet.FormatBalance(balance, tokenDecimals)
					tokenBalanceArray = append(tokenBalanceArray, map[string]interface{}{
						"symbol":   tokenSymbol,
						"currency": tokenSymbol,
						"balance":  tokenBalanceFormatted,
					})
				}
				serverWalletInfo["token_balances"] = tokenBalanceArray
			}

			serverWalletResults = append(serverWalletResults, serverWalletInfo)
		}

		return c.JSON(fiber.Map{
			"server_wallets": serverWalletResults,
			"success":        true,
		})
	}
}

// UpdateAgentConfig handles updating an agent's configuration
func (a *App) UpdateAgentConfig() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		agentId := agent.ID.String()

		var newConfig state.AgentConfig
		if err := c.BodyParser(&newConfig); err != nil {
			xlog.Error("Error parsing agent config", "error", err)
			return errorJSONMessage(c, "Invalid agent config: "+err.Error())
		}

		if err := validateAgentConfig(&newConfig); err != nil {
			return errorJSONMessageWithValidation(c, err)
		}

		if err := validateModel(newConfig.Model); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		var currentConfig state.AgentConfig
		if err := json.Unmarshal(agent.Config, &currentConfig); err != nil {
			return errorJSONMessage(c, "Failed to parse current agent config")
		}
		newConfig.ServerWallets = currentConfig.ServerWallets

		newConfigJSON, err := json.Marshal(newConfig)
		if err != nil {
			return errorJSONMessage(c, "Failed to serialize config")
		}
		agent.Config = newConfigJSON

		if agent.Name != newConfig.Name {
			agent.Name = newConfig.Name
		}

		if err := db.DB.Save(&agent).Error; err != nil {
			return errorJSONMessage(c, "Failed to update config in DB: "+err.Error())
		}

		pool, ok := a.UserPools[userIDStr]
		if ok {
			wasRunning := false
			if existingAgent := pool.GetAgent(agentId); existingAgent != nil {
				wasRunning = !existingAgent.Paused()
			}

			if existingAgent := pool.GetAgent(agentId); existingAgent != nil {
				existingAgent.Stop()

				pool.RemoveAgentOnly(agentId)

				if err := pool.CreateAgentWithExistingManager(agentId, &newConfig, !wasRunning); err != nil {
					xlog.Error("Failed to recreate agent in memory", "error", err)
					return errorJSONMessage(c, "Agent config updated in DB but failed to reload in memory")
				}

			}
		}

		xlog.Info("Updated agent", "id", agentId)
		return statusJSONMessage(c, "ok")
	}
}

func (a *App) UpdateAgentPayLimits() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		agentId := agent.ID.String()

		var payLimitsRequest struct {
			PayLimits map[string]float64 `json:"pay_limits"`
		}

		if err := c.BodyParser(&payLimitsRequest); err != nil {
			xlog.Error("Error parsing pay limits", "error", err)
			return errorJSONMessage(c, "Invalid pay limits format: "+err.Error())
		}

		if err := validatePayLimits(payLimitsRequest.PayLimits); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		var currentConfig state.AgentConfig
		if err := json.Unmarshal(agent.Config, &currentConfig); err != nil {
			return errorJSONMessage(c, "Failed to parse current agent config")
		}

		currentConfig.PayLimits = payLimitsRequest.PayLimits

		newConfigJSON, err := json.Marshal(currentConfig)
		if err != nil {
			return errorJSONMessage(c, "Failed to serialize config")
		}
		agent.Config = newConfigJSON

		if err := db.DB.Save(&agent).Error; err != nil {
			return errorJSONMessage(c, "Failed to update pay limits in DB: "+err.Error())
		}

		pool, ok := a.UserPools[userIDStr]
		if ok {
			wasRunning := false
			if existingAgent := pool.GetAgent(agentId); existingAgent != nil {
				wasRunning = !existingAgent.Paused()
				existingAgent.Stop()
				pool.RemoveAgentOnly(agentId)

				if err := pool.CreateAgentWithExistingManager(agentId, &currentConfig, !wasRunning); err != nil {
					xlog.Error("Failed to recreate agent in memory with updated pay limits", "error", err)
					return errorJSONMessage(c, "Pay limits updated in DB but failed to reload agent in memory")
				}
			}
		}

		xlog.Info("Updated agent pay limits", "id", agentId, "pay_limits", payLimitsRequest.PayLimits)
		return statusJSONMessage(c, "ok")
	}
}

func (a *App) UpdateAgentPayLimitStatus() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		var requestBody struct {
			PayLimitStatus string `json:"pay_limit_status"`
		}

		if err := c.BodyParser(&requestBody); err != nil {
			return errorJSONMessage(c, "Invalid request body: "+err.Error())
		}

		if requestBody.PayLimitStatus != "APPROVED" && requestBody.PayLimitStatus != "CANCELLED" {
			return errorJSONMessage(c, "Invalid pay_limit_status. Must be either 'APPROVED' or 'CANCELLED'")
		}

		agentId := agent.ID.String()

		if err := db.DB.Model(&models.Agent{}).Where("ID = ?", agent.ID).Update("payLimitStatus", requestBody.PayLimitStatus).Error; err != nil {
			return errorJSONMessage(c, "Failed to update pay limit status in DB: "+err.Error())
		}

		xlog.Info("Updated agent pay limit status", "id", agentId, "status", requestBody.PayLimitStatus)
		return c.JSON(fiber.Map{
			"success":        true,
			"message":        "Pay limit status updated successfully",
			"payLimitStatus": requestBody.PayLimitStatus,
		})
	}
}

func (a *App) ExportAgent() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get agent from context (set by RequireActiveAgent middleware)
		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		// 2. Parse the agent config from database
		var config state.AgentConfig
		if err := json.Unmarshal(agent.Config, &config); err != nil {
			return errorJSONMessage(c, "Failed to parse agent config: "+err.Error())
		}

		// 3. Set the filename for download
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", config.Name))

		// 4. Return the config as JSON
		return c.JSON(config)
	}
}

func (a *App) ImportAgent() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return errorJSONMessage(c, "Invalid user ID")
		}

		// 2. Get uploaded file
		file, err := c.FormFile("file")
		if err != nil {
			return errorJSONMessage(c, "Failed to get uploaded file: "+err.Error())
		}

		// 3. Read file content directly from memory (no filesystem needed)
		src, err := file.Open()
		if err != nil {
			return errorJSONMessage(c, "Failed to open uploaded file: "+err.Error())
		}
		defer src.Close()

		data, err := io.ReadAll(src)
		if err != nil {
			return errorJSONMessage(c, "Failed to read file content: "+err.Error())
		}

		// 4. Parse JSON config
		var config state.AgentConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return errorJSONMessage(c, "Invalid JSON format: "+err.Error())
		}

		xlog.Info("Importing agent", "name", config.Name)

		// 5. Validate config fields
		if err := validateAgentConfig(&config); err != nil {
			return errorJSONMessageWithValidation(c, err)
		}

		// 6. Validate and set model
		if err := validateModel(config.Model); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		// 7. Apply fallback values from env if fields are empty
		if config.MultimodalModel == "" {
			config.MultimodalModel = os.Getenv("LOCALAGI_MULTIMODAL_MODEL")
		}
		if config.LocalRAGURL == "" {
			config.LocalRAGURL = os.Getenv("LOCALAGI_LOCALRAG_URL")
		}
		if config.LocalRAGAPIKey == "" {
			config.LocalRAGAPIKey = os.Getenv("LOCALAGI_LOCALRAG_API_KEY")
		}

		// 8. Serialize the enriched config to JSON
		configJSON, err := json.Marshal(config)
		if err != nil {
			return errorJSONMessage(c, "Failed to serialize config")
		}

		// 9. Store config in DB
		id := uuid.New()
		agent := models.Agent{
			ID:     id,
			UserID: userID,
			Name:   config.Name,
			Config: configJSON,
		}

		if err := db.DB.Create(&agent).Error; err != nil {
			return errorJSONMessage(c, "Failed to store agent: "+err.Error())
		}

		// 10. Ensure agent pool is initialized
		var pool *state.AgentPool
		if p, ok := a.UserPools[userIDStr]; ok {
			pool = p
		} else {
			pool, err = state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to create agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = pool
		}

		// 11. Register agent in the in-memory pool
		if err := pool.CreateAgent(id.String(), &config); err != nil {
			return errorJSONMessage(c, "Failed to initialize agent: "+err.Error())
		}

		return statusJSONMessage(c, "ok")
	}
}

func (a *App) OldChat(pool *state.AgentPool) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		payload := struct {
			Message string `json:"message"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			return err
		}
		agentName := c.Params("name")
		manager := pool.GetManager(agentName)

		query := strings.Clone(payload.Message)
		if query == "" {
			_, _ = c.Write([]byte("Please enter a message."))
			return nil
		}
		manager.Send(
			sse.NewMessage(
				chatDiv(query, "gray"),
			).WithEvent("messages"))

		go func() {
			a := pool.GetAgent(agentName)
			if a == nil {
				xlog.Info("Agent not found in pool", c.Params("name"))
				return
			}
			res := a.Ask(
				coreTypes.WithText(query),
			)
			if res.Error != nil {
				xlog.Error("Error asking agent", "agent", agentName, "error", res.Error)
			} else {
				xlog.Info("we got a response from the agent", "agent", agentName, "response", res.Response)
			}
			manager.Send(
				sse.NewMessage(
					chatDiv(res.Response, "blue"),
				).WithEvent("messages"))
			manager.Send(
				sse.NewMessage(
					disabledElement("inputMessage", false), // show again the input
				).WithEvent("message_status"))

			//result := `<i>done</i>`
			//	_, _ = w.Write([]byte(result))
		}()

		manager.Send(
			sse.NewMessage(
				loader() + disabledElement("inputMessage", true),
			).WithEvent("message_status"))

		return nil
	}
}

// Chat provides a JSON-based API for chat functionality
// This is designed to work better with the React UI
func (a *App) Chat() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID and agent from context
		userID, ok := c.Locals("id").(string)
		if !ok || userID == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		agentId := agent.ID.String()

		// 2. Parse body
		var payload struct {
			Message string `json:"message"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return errorJSONMessage(c, "Invalid request")
		}

		message := strings.TrimSpace(payload.Message)
		if message == "" {
			return errorJSONMessage(c, "Message cannot be empty")
		}

		// 3. Parse agent config
		var config state.AgentConfig
		if err := json.Unmarshal(agent.Config, &config); err != nil {
			return errorJSONMessage(c, "Invalid agent config")
		}

		// 4. Ensure in-memory pool exists
		var pool *state.AgentPool
		if p, ok := a.UserPools[userID]; ok {
			pool = p
		} else {
			pool := state.NewEmptyAgentPool(
				userID,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			a.UserPools[userID] = pool
		}

		// 5. Start agent in memory if not running
		if pool.GetAgent(agentId) == nil {
			if err := pool.CreateAgent(agentId, &config); err != nil {
				return errorJSONMessage(c, "Failed to start agent: "+err.Error())
			}
		}

		// 6. Emit user message via SSE
		manager := pool.GetManager(agentId)
		messageID := fmt.Sprintf("%d", time.Now().UnixNano())

		send := func(event string, data map[string]interface{}) {
			manager.Send(
				sse.NewMessage(mustStringify(data)).WithEvent(event),
			)
		}

		// 7. Save user message to DB
		_ = db.DB.Create(&models.AgentMessage{
			ID:        uuid.New(),
			AgentID:   agent.ID,
			Sender:    "user",
			Content:   message,
			Type:      "message",
			CreatedAt: time.Now(),
		})

		// Send processing status
		statusData, err := json.Marshal(map[string]interface{}{
			"status":    "processing",
			"timestamp": time.Now().Format(time.RFC3339),
		})

		if err != nil {
			xlog.Error("Error marshaling status message", "error", err)
		} else {
			manager.Send(
				sse.NewMessage(string(statusData)).WithEvent("json_message_status"))
		}

		// 8. Ask agent asynchronously with streaming support
		go func() {
			var fullContent strings.Builder
			agentMessageID := messageID + "-agent"
			// Stream callback to send partial responses
			streamCallback := func(chunk string) {
				fullContent.WriteString(chunk)

				// Send streaming chunk via SSE
				send("json_message_chunk", map[string]interface{}{
					"id":        agentMessageID,
					"sender":    "agent",
					"content":   fullContent.String(),
					"createdAt": time.Now().Format(time.RFC3339),
				})
			}

			response := pool.GetAgent(agentId).Ask(
				coreTypes.WithText(message),
				coreTypes.WithStreamCallback(streamCallback),
			)

			if response.Error != nil {
				send("json_error", map[string]interface{}{
					"error":     response.Error.Error(),
					"createdAt": time.Now().Format(time.RFC3339),
				})
				return
			}

			// Send final complete message for both streamed and non-streamed responses
			send("json_message", map[string]interface{}{
				"id":        agentMessageID,
				"sender":    "agent",
				"content":   response.Response,
				"type":      "message",
				"createdAt": time.Now().Format(time.RFC3339),
				"final":     true, // Mark as final message to replace streaming content
			})

			// Save agent reply to DB
			_ = db.DB.Create(&models.AgentMessage{
				ID:        uuid.New(),
				AgentID:   agent.ID,
				Sender:    "agent",
				Content:   response.Response,
				Type:      "message",
				CreatedAt: time.Now(),
			})

			// Send completed status
			completedData, err := json.Marshal(map[string]interface{}{
				"status":    "completed",
				"timestamp": time.Now().Format(time.RFC3339),
			})
			if err != nil {
				xlog.Error("Error marshaling completed status", "error", err)
			} else {
				manager.Send(
					sse.NewMessage(string(completedData)).WithEvent("json_message_status"))
			}

		}()

		// 9. Immediate 202 response
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"status":     "message_received",
			"message_id": messageID,
		})
	}
}

func (a *App) GetActionDefinition() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		// 2. Get or create user pool
		var pool *state.AgentPool
		if p, ok := a.UserPools[userIDStr]; ok {
			pool = p
		} else {
			var err error
			pool, err = state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to create agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = pool
		}

		payload := struct {
			Config map[string]string `json:"config"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			xlog.Error("Error parsing action payload", "error", err)
			return errorJSONMessage(c, err.Error())
		}

		actionName := c.Params("name")

		xlog.Debug("Executing action", "action", actionName, "config", payload.Config)
		action, err := services.Action(actionName, "", payload.Config, pool, map[string]string{}, nil, nil)
		if err != nil {
			xlog.Error("Error creating action", "error", err)
			return errorJSONMessage(c, err.Error())
		}

		return c.JSON(action.Definition())
	}
}

func (a *App) ExecuteAction() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		// Parse user ID to UUID
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return errorJSONMessage(c, "Invalid user ID")
		}

		// 2. Get or create user pool
		var pool *state.AgentPool
		if p, ok := a.UserPools[userIDStr]; ok {
			pool = p
		} else {
			var err error
			pool, err = state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to create agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = pool
		}

		payload := struct {
			Config map[string]string      `json:"config"`
			Params coreTypes.ActionParams `json:"params"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			xlog.Error("Error parsing action payload", "error", err)
			return errorJSONMessage(c, err.Error())
		}

		actionName := c.Params("name")

		// 3. Check if both config and params are empty
		if len(payload.Config) == 0 && len(payload.Params) == 0 {
			return errorJSONMessage(c, "Action execution requires both config or params to be provided")
		}

		// 4. Validate action config
		if payload.Config != nil {
			// Convert map[string]string to map[string]interface{} for validation
			configForValidation := make(map[string]interface{})
			for k, v := range payload.Config {
				configForValidation[k] = v
			}

			if err := validateActionFields(actionName, configForValidation, 1); err != nil {
				xlog.Error("Action config validation failed", "action", actionName, "error", err)
				return errorJSONMessage(c, err.Error())
			}
		}

		// 5. Validate action params against schema
		if payload.Params != nil {
			action, err := services.Action(actionName, "", payload.Config, pool, map[string]string{}, nil, nil)
			if err != nil {
				xlog.Error("Error creating action for validation", "error", err)
				return errorJSONMessage(c, "Failed to create action for validation: "+err.Error())
			}

			definition := action.Definition()

			// Check required fields
			for _, required := range definition.Required {
				if _, exists := payload.Params[required]; !exists {
					return errorJSONMessage(c, fmt.Sprintf("Required parameter '%s' is missing", required))
				}
			}

			// Validate parameter types and values
			for paramName, paramValue := range payload.Params {
				if prop, exists := definition.Properties[paramName]; exists {
					if err := validateParamValue(paramName, paramValue, prop); err != nil {
						return errorJSONMessage(c, err.Error())
					}
				}
			}
		}

		// 6. Create action execution record
		executionID := uuid.New()
		actionExecution := models.ActionExecution{
			ID:         executionID,
			UserID:     userID,
			ActionName: actionName,
			Status:     "running",
			CreatedAt:  time.Now(),
		}

		if err := db.DB.Create(&actionExecution).Error; err != nil {
			xlog.Error("Failed to create action execution record", "error", err)
			// Continue with execution even if DB logging fails
		}

		xlog.Debug("Executing action", "action", actionName, "config", payload.Config, "params", payload.Params)
		action, err := services.Action(actionName, "", payload.Config, pool, map[string]string{}, nil, nil)

		if err != nil {
			// Update status to error
			_ = db.DB.Model(&actionExecution).Updates(map[string]interface{}{
				"Status":    "error",
				"UpdatedAt": time.Now(),
			})
			xlog.Error("Error creating action", "error", err)
			return errorJSONMessage(c, err.Error())
		}

		ctx, cancel := context.WithTimeout(c.Context(), 200*time.Second)
		defer cancel()

		res, err := action.Run(ctx, a.sharedState, payload.Params)
		if err != nil {
			// Update status to error
			_ = db.DB.Model(&actionExecution).Updates(map[string]interface{}{
				"Status":    "error",
				"UpdatedAt": time.Now(),
			})
			xlog.Error("Error running action", "error", err)
			return errorJSONMessage(c, err.Error())
		}

		// 8. Update status to success
		_ = db.DB.Model(&actionExecution).Updates(map[string]interface{}{
			"Status":    "success",
			"UpdatedAt": time.Now(),
		})

		xlog.Info("Action executed successfully", "action", actionName, "executionId", executionID, "result", res)
		return c.JSON(res)
	}
}

func (a *App) ListActions() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		return c.JSON(services.AvailableActions)
	}
}

// createToolCallResponse generates a proper tool call response for user-defined actions
func (a *App) createToolCallResponse(id, agentName string, actionState coreTypes.ActionState, conv []openai.ChatCompletionMessage) types.ResponseBody {
	// Create tool call ID
	toolCallID := fmt.Sprintf("call_%d", time.Now().UnixNano())

	// Get function name and arguments
	functionName := actionState.Action.Definition().Name.String()
	argumentsJSON, err := json.Marshal(actionState.Params)
	if err != nil {
		xlog.Error("Error marshaling action params for tool call", "error", err)
		// Fallback to empty arguments
		argumentsJSON = []byte("{}")
	}

	// Create message object with reasoning
	messageObj := types.ResponseMessage{
		Type:   "message",
		ID:     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Status: "completed",
		Role:   "assistant",
		Content: []types.MessageContentItem{
			{
				Type: "output_text",
				Text: actionState.Reasoning,
			},
		},
	}

	// Create function tool call object
	functionToolCall := types.FunctionToolCall{
		Arguments: string(argumentsJSON),
		CallID:    toolCallID,
		Name:      functionName,
		Type:      "function_call",
		ID:        fmt.Sprintf("tool_%d", time.Now().UnixNano()),
		Status:    "completed",
	}

	// Create response with both message and tool call in output array
	return types.ResponseBody{
		ID:        id,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Status:    "completed",
		Model:     agentName,
		Output: []interface{}{
			messageObj,
			functionToolCall,
		},
		Usage: types.UsageInfo{
			InputTokens:  0, // TODO: calculate actual usage
			OutputTokens: 0,
			TotalTokens:  0,
		},
	}
}

func (a *App) Responses(pool *state.AgentPool, tracker *conversations.ConversationTracker[string]) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		var request types.RequestBody
		if err := c.BodyParser(&request); err != nil {
			return err
		}

		request.SetInputByType()

		var previousResponseID string
		conv := []openai.ChatCompletionMessage{}
		if request.PreviousResponseID != nil {
			previousResponseID = *request.PreviousResponseID
			conv = tracker.GetConversation(previousResponseID)
		}

		agentName := request.Model
		messages := append(conv, request.ToChatCompletionMessages()...)

		agent := pool.GetAgent(agentName)
		if agent == nil {
			xlog.Info("Agent not found in pool", c.Params("name"))
			return c.Status(http.StatusInternalServerError).JSON(types.ResponseBody{Error: "Agent not found"})
		}

		// Prepare job options
		jobOptions := []coreTypes.JobOption{
			coreTypes.WithConversationHistory(messages),
		}

		// Add tools if present in the request
		if len(request.Tools) > 0 {
			builtinTools, userTools := types.SeparateTools(request.Tools)
			if len(builtinTools) > 0 {
				jobOptions = append(jobOptions, coreTypes.WithBuiltinTools(builtinTools))
				xlog.Debug("Adding builtin tools to job", "count", len(builtinTools), "agent", agentName)
			}
			if len(userTools) > 0 {
				jobOptions = append(jobOptions, coreTypes.WithUserTools(userTools))
				xlog.Debug("Adding user tools to job", "count", len(userTools), "agent", agentName)
			}
		}

		var choice types.ToolChoice
		if err := json.Unmarshal(request.ToolChoice, &choice); err == nil {
			if choice.Type == "function" {
				jobOptions = append(jobOptions, coreTypes.WithToolChoice(choice.Name))
			}
		}

		res := agent.Ask(jobOptions...)
		if res.Error != nil {
			xlog.Error("Error asking agent", "agent", agentName, "error", res.Error)

			return c.Status(http.StatusInternalServerError).JSON(types.ResponseBody{Error: res.Error.Error()})
		} else {
			xlog.Info("we got a response from the agent", "agent", agentName, "response", res.Response)
		}

		id := uuid.New().String()

		// Check if this is a user-defined tool call
		if res.Response == "" && len(res.State) > 0 {
			// Get the last action from state
			lastAction := res.State[len(res.State)-1]
			if coreTypes.IsActionUserDefined(lastAction.Action) {
				xlog.Debug("Detected user-defined action, creating tool call response", "action", lastAction.Action.Definition().Name)

				// Generate tool call response
				response := a.createToolCallResponse(id, agentName, lastAction, conv)
				tracker.SetConversation(id, conv) // Save conversation without adding assistant message
				return c.JSON(response)
			}
		}

		// Regular text response
		conv = append(conv, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: res.Response,
		})

		tracker.SetConversation(id, conv)

		response := types.ResponseBody{
			ID:        id,
			Object:    "response",
			CreatedAt: time.Now().Unix(),
			Status:    "completed",
			Model:     agentName,
			Output: []interface{}{
				types.ResponseMessage{
					Type:   "message",
					ID:     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
					Status: "completed",
					Role:   "assistant",
					Content: []types.MessageContentItem{
						{
							Type: "output_text",
							Text: res.Response,
						},
					},
				},
			},
		}

		return c.JSON(response)
	}
}

type AgentRole struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
}

func (a *App) GenerateGroupProfiles() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		var request struct {
			Descript string `json:"description"`
		}

		if err := c.BodyParser(&request); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		var results struct {
			Agents []AgentRole `json:"agents"`
		}

		// Get user ID from context (may be empty if no auth middleware)
		userIDStr, _ := c.Locals("id").(string)
		userID := uuid.Nil
		if userIDStr != "" {
			userID, _ = uuid.Parse(userIDStr)
		}

		xlog.Debug("Generating group", "description", request.Descript)
		client := llm.NewClient(os.Getenv("LOCALAGI_LLM_API_KEY"), os.Getenv("LOCALAGI_LLM_API_URL"), "10m")
		err := llm.GenerateTypedJSONWithGuidance(c.Context(), client, request.Descript, os.Getenv("LOCALAGI_MODEL"), userID, uuid.Nil, jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"agents": {
					Type: jsonschema.Array,
					Items: &jsonschema.Definition{
						Type:     jsonschema.Object,
						Required: []string{"name", "description", "system_prompt"},
						Properties: map[string]jsonschema.Definition{
							"name": {
								Type:        jsonschema.String,
								Description: "The name of the agent",
							},
							"description": {
								Type:        jsonschema.String,
								Description: "The description of the agent",
							},
							"system_prompt": {
								Type:        jsonschema.String,
								Description: "The system prompt for the agent",
							},
						},
					},
				},
			},
		}, &results)
		if err != nil {
			xlog.Error("Error generating group profiles", "error", err)
			return errorJSONMessage(c, err.Error())
		}

		return c.JSON(results.Agents)
	}
}

func (a *App) CreateGroup() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return errorJSONMessage(c, "Invalid user ID")
		}

		var config struct {
			Agents      []AgentRole       `json:"agents"`
			AgentConfig state.AgentConfig `json:"agent_config"`
		}
		if err := c.BodyParser(&config); err != nil {
			return errorJSONMessage(c, err.Error())
		}

		// 2. Get or create user pool
		pool, ok := a.UserPools[userIDStr]
		if !ok {
			var err error
			pool, err = state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return errorJSONMessage(c, "Failed to create agent pool: "+err.Error())
			}
			a.UserPools[userIDStr] = pool
		}

		agentConfig := &config.AgentConfig
		for _, agent := range config.Agents {
			xlog.Info("Creating agent", "name", agent.Name, "description", agent.Description)
			agentConfig.Name = agent.Name
			agentConfig.Description = agent.Description
			agentConfig.SystemPrompt = agent.SystemPrompt

			// 3. Validate config fields
			if err := validateAgentConfig(agentConfig); err != nil {
				return errorJSONMessageWithValidation(c, err)
			}

			// 4. Validate model
			if err := validateModel(agentConfig.Model); err != nil {
				return errorJSONMessage(c, fmt.Sprintf("Agent '%s': %s", agent.Name, err.Error()))
			}

			// 4. Apply fallback values from env if fields are empty
			if agentConfig.MultimodalModel == "" {
				agentConfig.MultimodalModel = os.Getenv("LOCALAGI_MULTIMODAL_MODEL")
			}
			if agentConfig.LocalRAGURL == "" {
				agentConfig.LocalRAGURL = os.Getenv("LOCALAGI_LOCALRAG_URL")
			}
			if agentConfig.LocalRAGAPIKey == "" {
				agentConfig.LocalRAGAPIKey = os.Getenv("LOCALAGI_LOCALRAG_API_KEY")
			}

			if os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") == "true" {
				walletsConfig, err := serverwallet.GenerateDefaultServerWalletsConfig()
				if err != nil {
					return errorJSONMessage(c, "Failed to create wallets: "+err.Error())
				}
				agentConfig.ServerWallets = walletsConfig
			}

			// 5. Serialize the enriched config to JSON
			configJSON, err := json.Marshal(agentConfig)
			if err != nil {
				return errorJSONMessage(c, "Failed to serialize config")
			}

			// 6. Store config in DB
			id := uuid.New()
			agentModel := models.Agent{
				ID:     id,
				UserID: userID,
				Name:   agentConfig.Name,
				Config: configJSON,
			}

			if err := db.DB.Create(&agentModel).Error; err != nil {
				return errorJSONMessage(c, "Failed to store agent: "+err.Error())
			}

			// 7. Create agent in memory pool using the DB ID
			if err := pool.CreateAgent(id.String(), agentConfig); err != nil {
				return errorJSONMessage(c, err.Error())
			}
		}

		return statusJSONMessage(c, "ok")
	}
}

// ListModels endpoint returns available models (local + filtered OpenRouter)
func (a *App) ListModels(pool *state.AgentPool) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		allModels := getAvailableModels()
		return c.JSON(fiber.Map{"models": allModels})
	}
}

// GetAgentConfigMeta returns the metadata for agent configuration fields, including available models
func (a *App) GetAgentConfigMeta() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID
		userID, ok := c.Locals("id").(string)
		if !ok || userID == "" {
			return errorJSONMessage(c, "User ID missing from session")
		}

		// 2. Prepare config metadata for form rendering
		configMeta := state.NewAgentConfigMeta(
			services.ActionsConfigMeta(),
			services.ConnectorsConfigMeta(),
			services.DynamicPromptsConfigMeta(),
			services.FiltersConfigMeta(),
		)

		// 3. Add available models (could be filtered per-user later)
		models := getAvailableModels()
		for i, field := range configMeta.Fields {
			if field.Name == "model" {
				options := []config.FieldOption{}
				for _, m := range models {
					label := m["name"].(string)
					if src, ok := m["source"].(string); ok {
						label = "[" + src + "] " + label
					}
					options = append(options, config.FieldOption{
						Value: m["id"].(string),
						Label: label,
					})
				}
				configMeta.Fields[i].Options = options
			}
		}

		// 4. (Optional) Add user-specific flags/limits here if needed
		_ = userID // Placeholder if needed later

		return c.JSON(configMeta)
	}
}

// getLocalModels returns the local model(s) configured via environment variables
func getLocalModels() []map[string]interface{} {
	modelName := os.Getenv("MODEL_NAME")
	// Remove dependency on LOCALAGI_MODEL since we always use agent config
	if modelName == "" {
		return nil
	}
	return []map[string]interface{}{
		{"id": "local/" + modelName, "name": modelName, "description": "Local model: " + modelName},
	}
}

// getOpenRouterModels fetches and filters OpenRouter models for latest OpenAI, Anthropic, and Alibaba
func getOpenRouterModels() []map[string]interface{} {
	openrouterApiKey := os.Getenv("LOCALAGI_LLM_API_KEY")
	if openrouterApiKey == "" {
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+openrouterApiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) // Ensure full body is read
		resp.Body.Close()
	}()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}
	// Only allow latest OpenAI, Anthropic, and Alibaba models
	allowed := map[string]bool{
		"openai/gpt-4o":                        true,
		"openai/gpt-4-turbo":                   true,
		"openai/gpt-4.1":                       true,
		"openai/o4-mini":                       true,
		"openai/o4-mini-high":                  true,
		"openai/o3":                            true,
		"anthropic/claude-3.5-sonnet":          true,
		"anthropic/claude-3.7-sonnet":          true,
		"qwen/qwen-2.5-7b-instruct":            true,
		"qwen/qwq-32b":                         true,
		"qwen/qwen-2.5-72b-instruct":           true,
		"google/gemini-2.5-pro-exp-03-25:free": true,
		"deepseek/deepseek-chat-v3-0324:free":  true,
		"qwen/qwq-32b:free":                    true,
	}
	models := []map[string]interface{}{}
	for _, m := range result.Data {
		id, _ := m["id"].(string)
		if allowed[id] {
			m["source"] = "openrouter"
			m["id"] = id // Prefix to avoid collision
			models = append(models, m)
		}
	}
	return models
}

// getAvailableModels returns both local and filtered OpenRouter models
// func getAvailableModels() []map[string]interface{} {
// 	// localModels := getLocalModels()
// 	openrouterModels := getOpenRouterModels()
// 	return openrouterModels
// }

func getAvailableModels() []map[string]interface{} {
	openrouterModels := getOpenRouterModels()

	// Prioritize gpt-4o as the first option
	var gpt4oModel map[string]interface{}
	var otherModels []map[string]interface{}

	for _, model := range openrouterModels {
		if model["id"].(string) == "openai/gpt-4o" {
			gpt4oModel = model
		} else {
			otherModels = append(otherModels, model)
		}
	}

	// Return with gpt-4o first if it exists
	var reorderedModels []map[string]interface{}
	if gpt4oModel != nil {
		reorderedModels = append(reorderedModels, gpt4oModel)
	}
	reorderedModels = append(reorderedModels, otherModels...)

	return reorderedModels
}

// validateModel checks if the provided model is valid and available
func validateModel(model string) error {
	if model == "" {
		return fmt.Errorf("model is required")
	}

	availableModels := getAvailableModels()
	for _, availableModel := range availableModels {
		if availableModel["id"].(string) == model {
			return nil
		}
	}

	return fmt.Errorf("model '%s' is not available. Please choose from available models", model)
}

func validatePayLimits(payLimits map[string]float64) error {
	if payLimits == nil {
		return nil
	}

	type tokenLimit struct {
		min float64
		max float64
	}

	tokenLimits := map[string]tokenLimit{
		"ETH":  {min: 0.0001, max: 0.02},
		"BNB":  {min: 0.0001, max: 0.2},
		"SOL":  {min: 0.0001, max: 1},
		"USDC": {min: 0.1, max: 100},
		"USDT": {min: 0.1, max: 100},
	}

	for token, limit := range payLimits {
		tokenLimitInfo, supported := tokenLimits[token]
		if !supported {
			return fmt.Errorf("unsupported token '%s'. Supported tokens are: ETH, BNB, SOL, USDC, USDT", token)
		}

		if limit < 0 {
			return fmt.Errorf("pay limit for token '%s' cannot be negative", token)
		}

		if limit > 0 && limit < tokenLimitInfo.min {
			return fmt.Errorf("pay limit for token '%s' is too low (minimum %.3f)", token, tokenLimitInfo.min)
		}

		if limit > tokenLimitInfo.max {
			return fmt.Errorf("pay limit for token '%s' is too high (maximum %.2f)", token, tokenLimitInfo.max)
		}
	}

	return nil
}

// validateAgentConfig validates all agent configuration fields
func validateAgentConfig(config *state.AgentConfig) error {
	// Name validation
	if config.Name == "" {
		return NewValidationErrorWithSection("name is required", "basic-section")
	}
	if len(config.Name) > 50 {
		return NewValidationErrorWithSection("name must be 50 characters or less", "basic-section")
	}

	// Description validation
	if len(config.Description) > 500 {
		return NewValidationErrorWithSection("description must be 500 characters or less", "basic-section")
	}

	// System prompt validation
	if len(config.SystemPrompt) > 10000 {
		return NewValidationErrorWithSection("system prompt must be 10,000 characters or less", "prompts-section")
	}

	// Identity guidance validation
	if len(config.IdentityGuidance) > 1000 {
		return NewValidationErrorWithSection("identity guidance must be 1,000 characters or less", "prompts-section")
	}

	// Permanent goal validation
	if len(config.PermanentGoal) > 2000 {
		return NewValidationErrorWithSection("permanent goal must be 2,000 characters or less", "prompts-section")
	}

	// Periodic runs validation (if provided, should be a valid duration)
	if config.PeriodicRuns != "" {
		if _, err := time.ParseDuration(config.PeriodicRuns); err != nil {
			return NewValidationErrorWithSection(fmt.Sprintf("periodic runs must be a valid duration (e.g., '10m', '1h'): %v", err), "advanced-section")
		}
	}

	// Validate numeric fields have reasonable values
	if config.KnowledgeBaseResults < 0 {
		return NewValidationErrorWithSection("knowledge base results must be non-negative", "memory-section")
	}
	if config.KnowledgeBaseResults > 50 {
		return NewValidationErrorWithSection("knowledge base results must be 50 or less", "memory-section")
	}

	if config.LoopDetectionSteps < 0 {
		return NewValidationErrorWithSection("loop detection steps must be non-negative", "advanced-section")
	}
	if config.LoopDetectionSteps > 10 {
		return NewValidationErrorWithSection("loop detection steps must be 10 or less", "advanced-section")
	}

	if config.LocalRAGURL != "" && !isValidURL(config.LocalRAGURL) {
		return NewValidationErrorWithSection("local RAG URL is not a valid URL format", "memory-section")
	}

	// Connector validations
	if len(config.Connector) > 20 {
		return NewValidationErrorWithSection("maximum 20 connectors allowed", "connectors-section")
	}

	// Get connector field groups for validation
	connectorGroups := services.ConnectorsConfigMeta()
	validConnectorTypes := make(map[string]bool)
	for _, group := range connectorGroups {
		validConnectorTypes[group.Name] = true
	}

	for i, connector := range config.Connector {
		if connector.Type == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("connector %d: type is required", i+1), "connectors-section")
		}
		if len(connector.Type) > 50 {
			return NewValidationErrorWithSection(fmt.Sprintf("connector %d: type must be 50 characters or less", i+1), "connectors-section")
		}

		// Validate connector type exists
		if !validConnectorTypes[connector.Type] {
			return NewValidationErrorWithSection(fmt.Sprintf("connector %d: invalid connector type '%s'", i+1, connector.Type), "connectors-section")
		}

		// Parse and validate connector config JSON
		if connector.Config != "" {
			var connectorConfig map[string]interface{}
			if err := json.Unmarshal([]byte(connector.Config), &connectorConfig); err != nil {
				return NewValidationErrorWithSection(fmt.Sprintf("connector %d: invalid JSON in config: %v", i+1, err), "connectors-section")
			}

			// Validate required fields and values based on connector type
			if err := validateConnectorFields(connector.Type, connectorConfig, i+1); err != nil {
				if validationErr, ok := err.(ValidationError); ok {
					return validationErr
				}
				return NewValidationErrorWithSection(err.Error(), "connectors-section")
			}

			// Basic validations for all fields
			for fieldName, fieldValue := range connectorConfig {
				if fieldValue == nil {
					continue
				}

				fieldValueStr := fmt.Sprintf("%v", fieldValue)

				// Validate field length
				if len(fieldValueStr) > 1000 {
					return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): field '%s' must be 1000 characters or less", i+1, connector.Type, fieldName), "connectors-section")
				}

				// Special validation for token fields
				if strings.Contains(fieldName, "token") || strings.Contains(fieldName, "Token") {
					if fieldValueStr != "" {
						if len(fieldValueStr) < 10 {
							return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): %s must be at least 10 characters", i+1, connector.Type, fieldName), "connectors-section")
						}
						if len(fieldValueStr) > 500 {
							return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): %s must be 500 characters or less", i+1, connector.Type, fieldName), "connectors-section")
						}
					}
				}

				// Validate duration fields
				if (strings.Contains(fieldName, "Duration") || strings.Contains(fieldName, "Interval")) && fieldValueStr != "" {
					if _, err := time.ParseDuration(fieldValueStr); err != nil {
						return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): field '%s' must be a valid duration (e.g., '5m', '1h'): %v", i+1, connector.Type, fieldName, err), "connectors-section")
					}
				}

				// Validate port fields
				if fieldName == "port" && fieldValueStr != "" {
					if port, err := strconv.Atoi(fieldValueStr); err != nil || port < 1 || port > 65535 {
						return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): port must be a valid port number (1-65535)", i+1, connector.Type), "connectors-section")
					}
				}

				// Validate boolean-like fields
				if strings.Contains(fieldName, "always") || strings.Contains(fieldName, "Always") ||
					strings.Contains(fieldName, "Reply") || strings.Contains(fieldName, "Limit") {
					if fieldValueStr != "" && fieldValueStr != "true" && fieldValueStr != "false" &&
						fieldValueStr != "1" && fieldValueStr != "0" {
						// Allow these to be strings or booleans, just warn if not recognizable
					}
				}
			}
		}
	}

	// Actions validations
	if len(config.Actions) > 50 {
		return NewValidationErrorWithSection("maximum 50 actions allowed", "actions-section")
	}

	// Get action field groups for validation
	actionGroups := services.ActionsConfigMeta()
	validActionTypes := make(map[string]bool)
	for _, group := range actionGroups {
		validActionTypes[group.Name] = true
	}

	for i, action := range config.Actions {
		if action.Name == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d: type is required", i+1), "actions-section")
		}
		if len(action.Name) > 100 {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d: type must be 100 characters or less", i+1), "actions-section")
		}

		// Validate action type exists
		if !validActionTypes[action.Name] {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d: invalid action type '%s'", i+1, action.Name), "actions-section")
		}

		// Parse and validate action config JSON
		if action.Config != "" {
			var actionConfig map[string]interface{}
			if err := json.Unmarshal([]byte(action.Config), &actionConfig); err != nil {
				return NewValidationErrorWithSection(fmt.Sprintf("action %d: invalid JSON in config: %v", i+1, err), "actions-section")
			}

			// Validate required fields and values based on action type
			if err := validateActionFields(action.Name, actionConfig, i+1); err != nil {
				if validationErr, ok := err.(ValidationError); ok {
					return validationErr
				}
				return NewValidationErrorWithSection(err.Error(), "actions-section")
			}

			// Basic validations for all fields
			for fieldName, fieldValue := range actionConfig {
				if fieldValue == nil {
					continue
				}

				fieldValueStr := fmt.Sprintf("%v", fieldValue)

				// Validate field length
				if len(fieldValueStr) > 1000 {
					return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): field '%s' must be 1000 characters or less", i+1, action.Name, fieldName), "actions-section")
				}

				// Special validation for token/key fields
				if strings.Contains(fieldName, "token") || strings.Contains(fieldName, "Token") ||
					strings.Contains(fieldName, "key") || strings.Contains(fieldName, "Key") {
					if fieldValueStr != "" {
						if len(fieldValueStr) < 10 {
							return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): %s must be at least 10 characters", i+1, action.Name, fieldName), "actions-section")
						}
						if len(fieldValueStr) > 2000 {
							return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): %s must be 2,000 characters or less", i+1, action.Name, fieldName), "actions-section")
						}
					}
				}

				// Validate URL fields
				if (strings.Contains(fieldName, "URL") || strings.Contains(fieldName, "Url") ||
					strings.Contains(fieldName, "Host") || strings.Contains(fieldName, "host")) && fieldValueStr != "" {
					if !isValidURL(fieldValueStr) && !isValidHost(fieldValueStr) {
						return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): field '%s' must be a valid URL or host", i+1, action.Name, fieldName), "actions-section")
					}
				}

				// Validate email fields
				if (strings.Contains(fieldName, "email") || strings.Contains(fieldName, "Email")) && fieldValueStr != "" {
					if !isValidEmail(fieldValueStr) {
						return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): field '%s' must be a valid email address", i+1, action.Name, fieldName), "actions-section")
					}
				}

				// Validate port fields
				if (strings.Contains(fieldName, "port") || strings.Contains(fieldName, "Port")) && fieldValueStr != "" {
					if port, err := strconv.Atoi(fieldValueStr); err != nil || port < 1 || port > 65535 {
						return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): %s must be a valid port number (1-65535)", i+1, action.Name, fieldName), "actions-section")
					}
				}
			}
		}
	}

	// Dynamic prompts validations
	if len(config.DynamicPrompts) > 20 {
		return NewValidationErrorWithSection("maximum 20 dynamic prompts allowed", "prompts-section")
	}
	for i, prompt := range config.DynamicPrompts {
		if prompt.Type == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("dynamic prompt %d: type is required", i+1), "prompts-section")
		}
		if len(prompt.Type) > 50 {
			return NewValidationErrorWithSection(fmt.Sprintf("dynamic prompt %d: type must be 50 characters or less", i+1), "prompts-section")
		}
	}

	// MCP servers validations
	if len(config.MCPServers) > 10 {
		return NewValidationErrorWithSection("maximum 10 MCP servers allowed", "mcp-section")
	}
	for i, server := range config.MCPServers {
		if server.URL == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("MCP server %d: URL is required", i+1), "mcp-section")
		}
		if !isValidURL(server.URL) {
			return NewValidationErrorWithSection(fmt.Sprintf("MCP server %d: URL is not a valid URL format", i+1), "mcp-section")
		}
		if !isValidMCPServerURL(server.URL) {
			return NewValidationErrorWithSection(fmt.Sprintf("MCP server %d: URL must be from allowed domains (server.smithery.ai or glama.ai/mcp/instances)", i+1), "mcp-section")
		}
		if len(server.URL) > 500 {
			return NewValidationErrorWithSection(fmt.Sprintf("MCP server %d: URL must be 500 characters or less", i+1), "mcp-section")
		}
	}

	return nil
}

// validateConnectorFields validates specific fields based on connector type
func validateConnectorFields(connectorType string, config map[string]interface{}, connectorIndex int) error {
	switch connectorType {
	case "discord":
		return validateDiscordFields(config, connectorIndex)
	case "slack":
		return validateSlackFields(config, connectorIndex)
	case "telegram":
		return validateTelegramFields(config, connectorIndex)
	case "github-issues", "github-prs":
		return validateGitHubFields(config, connectorIndex, connectorType)
	case "irc":
		return validateIRCFields(config, connectorIndex)
	case "twitter":
		return validateTwitterFields(config, connectorIndex)
	}
	return nil
}

// validateDiscordFields validates Discord-specific required fields
func validateDiscordFields(config map[string]interface{}, index int) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (discord): token is required", index), "connectors-section")
	}
	return nil
}

// validateSlackFields validates Slack-specific required fields
func validateSlackFields(config map[string]interface{}, index int) error {
	if appToken, ok := config["appToken"]; !ok || appToken == nil || fmt.Sprintf("%v", appToken) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (slack): appToken is required", index), "connectors-section")
	}
	if botToken, ok := config["botToken"]; !ok || botToken == nil || fmt.Sprintf("%v", botToken) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (slack): botToken is required", index), "connectors-section")
	}
	return nil
}

// validateTelegramFields validates Telegram-specific required fields
func validateTelegramFields(config map[string]interface{}, index int) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (telegram): token is required", index), "connectors-section")
	}
	return nil
}

// validateGitHubFields validates GitHub-specific required fields
func validateGitHubFields(config map[string]interface{}, index int, connectorType string) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): token is required", index, connectorType), "connectors-section")
	}
	if repository, ok := config["repository"]; !ok || repository == nil || fmt.Sprintf("%v", repository) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): repository is required", index, connectorType), "connectors-section")
	}
	if owner, ok := config["owner"]; !ok || owner == nil || fmt.Sprintf("%v", owner) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (%s): owner is required", index, connectorType), "connectors-section")
	}
	return nil
}

// validateIRCFields validates IRC-specific required fields
func validateIRCFields(config map[string]interface{}, index int) error {
	requiredFields := []string{"server", "port", "nickname", "channel"}
	for _, field := range requiredFields {
		if value, ok := config[field]; !ok || value == nil || fmt.Sprintf("%v", value) == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("connector %d (irc): %s is required", index, field), "connectors-section")
		}
	}
	return nil
}

// validateTwitterFields validates Twitter-specific required fields
func validateTwitterFields(config map[string]interface{}, index int) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (twitter): token is required", index), "connectors-section")
	}
	if botUsername, ok := config["botUsername"]; !ok || botUsername == nil || fmt.Sprintf("%v", botUsername) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("connector %d (twitter): botUsername is required", index), "connectors-section")
	}
	return nil
}

// isValidURL performs basic URL validation
func isValidURL(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

// isValidHost performs basic host validation (for SMTP hosts, etc.)
func isValidHost(str string) bool {
	// Allow hostname or hostname:port format
	return len(str) > 0 && !strings.Contains(str, "/") && !strings.HasPrefix(str, ".")
}

// isValidEmail performs basic email validation
func isValidEmail(str string) bool {
	return strings.Contains(str, "@") && strings.Contains(str, ".") && len(strings.TrimSpace(str)) > 5
}

// isValidMCPServerURL validates that the MCP server URL is from allowed domains
func isValidMCPServerURL(str string) bool {
	// Parse the URL to extract the hostname
	if !strings.HasPrefix(str, "https://") {
		return false
	}

	// Remove https:// prefix and find the first slash to get the hostname
	urlWithoutScheme := str[8:] // Remove "https://"
	slashIndex := strings.Index(urlWithoutScheme, "/")
	if slashIndex == -1 {
		return false // No path found
	}

	hostname := urlWithoutScheme[:slashIndex]
	path := urlWithoutScheme[slashIndex:]

	// Check for allowed domains with specific path requirements
	switch hostname {
	case "server.smithery.ai":
		return true // Any path allowed for smithery
	case "glama.ai":
		return strings.HasPrefix(path, "/mcp/instances/") // Must be MCP instances path
	default:
		return false
	}
}

// validateActionFields validates specific fields based on action type
func validateActionFields(actionType string, config map[string]interface{}, actionIndex int) error {
	switch actionType {
	case "search":
		return validateSearchFields(config, actionIndex)
	case "generate_image":
		return validateGenerateImageFields(config, actionIndex)
	case "github-issue-labeler", "github-issue-opener", "github-issue-closer",
		"github-issue-commenter", "github-issue-reader", "github-issue-searcher":
		return validateBasicGitHubFields(config, actionIndex, actionType)
	case "github-repository-get-content", "github-get-all-repository-content":
		return validateBasicGitHubFields(config, actionIndex, actionType)
	case "github-repository-create-or-update-content":
		return validateGitHubCreateUpdateFields(config, actionIndex, actionType)
	case "github-readme":
		return validateGitHubReadmeFields(config, actionIndex)
	case "github-pr-reader", "github-pr-commenter", "github-pr-reviewer", "github-pr-creator":
		return validateGitHubPRFields(config, actionIndex, actionType)
	case "twitter-post":
		return validateTwitterActionFields(config, actionIndex)
	case "send-mail":
		return validateSendMailFields(config, actionIndex)
	case "shell-command":
		return validateShellCommandFields(config, actionIndex)
	// Actions with no required fields: scraper, wikipedia, browse, counter, call_agents
	case "scraper", "wikipedia", "browse", "counter", "call_agents":
		return nil
	}
	return nil
}

// validateSearchFields validates search action fields
func validateSearchFields(config map[string]interface{}, index int) error {
	if results, ok := config["results"]; ok && results != nil {
		resultsStr := fmt.Sprintf("%v", results)
		if resultsInt, err := strconv.Atoi(resultsStr); err != nil || resultsInt < 1 || resultsInt > 100 {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d (search): results must be a number between 1 and 100", index), "actions-section")
		}
	}
	return nil
}

// validateGenerateImageFields validates generate image action fields
func validateGenerateImageFields(config map[string]interface{}, index int) error {
	requiredFields := []string{"apiKey", "apiURL", "model"}
	for _, field := range requiredFields {
		if value, ok := config[field]; !ok || value == nil || fmt.Sprintf("%v", value) == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d (generate_image): %s is required", index, field), "actions-section")
		}
	}
	return nil
}

// validateBasicGitHubFields validates basic GitHub action fields (token, repository, owner)
func validateBasicGitHubFields(config map[string]interface{}, index int, actionType string) error {
	requiredFields := []string{"token", "repository", "owner"}
	for _, field := range requiredFields {
		fmt.Println("ABBBBBBBBB", config[field])
		if value, ok := config[field]; !ok || value == nil || fmt.Sprintf("%v", value) == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): %s is required", index, actionType, field), "actions-section")
		}
	}
	return nil
}

// validateGitHubCreateUpdateFields validates GitHub repository create/update action fields
func validateGitHubCreateUpdateFields(config map[string]interface{}, index int, actionType string) error {
	// First validate the basic required fields
	if err := validateBasicGitHubFields(config, index, actionType); err != nil {
		return err
	}

	// Validate commitMail if provided (should be valid email format)
	if commitMail, ok := config["commitMail"]; ok && commitMail != nil {
		commitMailStr := fmt.Sprintf("%v", commitMail)
		if commitMailStr != "" && !isValidEmail(commitMailStr) {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): commitMail must be a valid email address", index, actionType), "actions-section")
		}
	}

	// Validate commitAuthor if provided (reasonable length)
	if commitAuthor, ok := config["commitAuthor"]; ok && commitAuthor != nil {
		commitAuthorStr := fmt.Sprintf("%v", commitAuthor)
		if len(commitAuthorStr) > 100 {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): commitAuthor must be 100 characters or less", index, actionType), "actions-section")
		}
	}

	return nil
}

// validateGitHubReadmeFields validates GitHub README action fields
func validateGitHubReadmeFields(config map[string]interface{}, index int) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("action %d (github-readme): token is required", index), "actions-section")
	}
	return nil
}

// validateGitHubPRFields validates GitHub PR action fields
func validateGitHubPRFields(config map[string]interface{}, index int, actionType string) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("action %d (%s): token is required", index, actionType), "actions-section")
	}
	// repository and owner are not required for some PR actions
	return nil
}

// validateTwitterActionFields validates Twitter action fields
func validateTwitterActionFields(config map[string]interface{}, index int) error {
	if token, ok := config["token"]; !ok || token == nil || fmt.Sprintf("%v", token) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("action %d (twitter-post): token is required", index), "actions-section")
	}
	return nil
}

// validateSendMailFields validates send mail action fields
func validateSendMailFields(config map[string]interface{}, index int) error {
	requiredFields := []string{"smtpHost", "smtpPort", "username", "password", "email"}
	for _, field := range requiredFields {
		if value, ok := config[field]; !ok || value == nil || fmt.Sprintf("%v", value) == "" {
			return NewValidationErrorWithSection(fmt.Sprintf("action %d (send-mail): %s is required", index, field), "actions-section")
		}
	}
	return nil
}

// validateShellCommandFields validates shell command action fields
func validateShellCommandFields(config map[string]interface{}, index int) error {
	if privateKey, ok := config["privateKey"]; !ok || privateKey == nil || fmt.Sprintf("%v", privateKey) == "" {
		return NewValidationErrorWithSection(fmt.Sprintf("action %d (shell-command): privateKey is required", index), "actions-section")
	}
	return nil
}

// validateParamValue validates a parameter value against its schema definition
func validateParamValue(paramName string, paramValue interface{}, schema jsonschema.Definition) error {
	if paramValue == nil {
		return nil // Allow nil values for optional parameters
	}

	switch schema.Type {
	case jsonschema.String:
		if _, ok := paramValue.(string); !ok {
			return fmt.Errorf("parameter '%s' must be a string", paramName)
		}
		// Check enum values if specified
		if len(schema.Enum) > 0 {
			valueStr := paramValue.(string)
			valid := false
			for _, enum := range schema.Enum {
				if enum == valueStr {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("parameter '%s' must be one of: %v", paramName, schema.Enum)
			}
		}
	case jsonschema.Number, jsonschema.Integer:
		switch paramValue.(type) {
		case int, int32, int64, float32, float64:
			// Valid numeric types
		default:
			return fmt.Errorf("parameter '%s' must be a number", paramName)
		}
	case jsonschema.Boolean:
		if _, ok := paramValue.(bool); !ok {
			return fmt.Errorf("parameter '%s' must be a boolean", paramName)
		}
	case jsonschema.Array:
		if _, ok := paramValue.([]interface{}); !ok {
			return fmt.Errorf("parameter '%s' must be an array", paramName)
		}
		// Could add more detailed array validation here if needed
	case jsonschema.Object:
		if _, ok := paramValue.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter '%s' must be an object", paramName)
		}
		// Could add more detailed object validation here if needed
	}
	return nil
}

// Proxy OpenRouter chat/completion endpoint
func (a *App) ProxyOpenRouterChat() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "OpenRouter API key not configured"})
		}

		userID, ok := c.Locals("id").(string)
		if !ok || userID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "User ID missing"})
		}

		agentId := c.Params("id")
		if agentId == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Agent ID is required"})
		}

		// Capture original request body
		body := c.Body()

		// Extract user message content
		userContent, err := extractUserContent(body)
		if err == nil && userContent != "" {
			_ = db.DB.Create(&models.AgentMessage{
				ID:        uuid.New(),
				AgentID:   uuid.MustParse(agentId),
				Sender:    "user",
				Content:   userContent,
				Type:      "message",
				CreatedAt: time.Now(),
			})
		}

		// Forward the request to OpenRouter
		openrouterURL := "https://openrouter.ai/api/v1/chat/completions"
		if c.Query("type") == "completion" {
			openrouterURL = "https://openrouter.ai/api/v1/completions"
		}

		req, err := http.NewRequest("POST", openrouterURL, bytes.NewReader(body))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer func() {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read response body"})
		}

		// Extract assistant message content
		agentContent, err := extractAgentContent(respBody)
		if err == nil && agentContent != "" {
			_ = db.DB.Create(&models.AgentMessage{
				ID:        uuid.New(),
				AgentID:   uuid.MustParse(agentId),
				Sender:    "agent",
				Content:   agentContent,
				Type:      "message",
				CreatedAt: time.Now(),
			})
		}

		c.Status(resp.StatusCode)
		c.Set("Content-Type", resp.Header.Get("Content-Type"))
		c.Send(respBody)
		return nil
	}
}

func extractUserContent(body []byte) (string, error) {
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	for _, msg := range payload.Messages {
		if msg.Role == "user" {
			return msg.Content, nil
		}
	}
	return "", errors.New("no user message found")
}

func extractAgentContent(body []byte) (string, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	if len(payload.Choices) > 0 {
		return payload.Choices[0].Message.Content, nil
	}

	return "", errors.New("no agent response found")
}

// PrivyClaims holds the JWT fields
type PrivyClaims struct {
	AppId      string `json:"aud,omitempty"`
	Expiration uint64 `json:"exp,omitempty"`
	Issuer     string `json:"iss,omitempty"`
	UserId     string `json:"sub,omitempty"`
}

func (c *PrivyClaims) Valid() error {
	if c.AppId != privyAppId {
		return errors.New("aud claim must match your Privy App ID")
	}
	if c.Issuer != "privy.io" {
		return errors.New("iss claim must be 'privy.io'")
	}
	if c.Expiration < uint64(time.Now().Unix()) {
		return errors.New("token is expired")
	}
	return nil
}

func (a *App) RequireUser() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get token from cookies
		tokenStr := c.Cookies("privy-token")
		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Missing Privy token",
			})
		}

		// 2. Parse public key
		pubKey, err := jwt.ParseECPublicKeyFromPEM([]byte(verificationKey))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Could not parse public key",
			})
		}

		// 3. Parse JWT
		claims := &PrivyClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodES256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return pubKey, nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid or expired token",
			})
		}

		// 4. Validate claims
		if err := claims.Valid(); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "JWT validation failed: " + err.Error(),
			})
		}

		// 5. Find or create user
		var user models.User
		result := db.DB.First(&user, "privyID = ?", claims.UserId)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Fetch user info from Privy
				privyUser, err := utils.GetPrivyUserByDID(claims.UserId, privyAppId, privyApiKey)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"error":   "Failed to fetch user from Privy",
					})
				}

				user = models.User{
					Email:   privyUser.GetEmail(),
					PrivyID: claims.UserId,
				}

				if err := db.DB.Create(&user).Error; err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"error":   "Failed to create user",
					})
				}

				c.Locals("email", user.Email)
			} else {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   "DB error while fetching user",
				})
			}
		}

		// 6. Set user context
		c.Locals("id", user.ID.String())
		c.Locals("privyId", user.PrivyID)

		return c.Next()
	}
}

func (a *App) RequireActiveAgent() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID from context (must be called after RequireUser)
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "User ID missing from context",
			})
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid user ID",
			})
		}

		// 2. Get agent ID from route parameter
		agentId := c.Params("id")
		if agentId == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Agent ID is required",
			})
		}

		// 3. Check if agent exists and is not archived
		var agent models.Agent
		if err := db.DB.
			Where("ID = ? AND UserID = ? AND archive = false", agentId, userID).
			First(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"success": false,
					"error":   "Agent not found or has been archived",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to query agent",
			})
		}

		// 4. Set agent context for potential use in handlers
		c.Locals("agent", &agent)

		return c.Next()
	}
}

func (a *App) RequireActiveStatusAgent() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID from context (must be called after RequireUser)
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "User ID missing from context",
			})
		}

		// 2. Get agent from context (must be called after RequireActiveAgent)
		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Agent not found in context",
			})
		}

		agentId := agent.ID.String()

		// 3. Check if user has an agent pool in memory
		pool, ok := a.UserPools[userIDStr]
		if !ok {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"success": false,
				"error":   "Agent is paused. Please resume the agent first",
			})
		}

		// 4. Check if agent is actually running in memory and active
		agentInstance := pool.GetAgent(agentId)
		if agentInstance == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"success": false,
				"error":   "Agent is paused. Please resume the agent first",
			})
		}

		// 5. Check if agent is paused
		if agentInstance.Paused() {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"success": false,
				"error":   "Agent is paused. Please resume the agent first",
			})
		}

		// Agent is active and running, continue to handler
		return c.Next()
	}
}

func (a *App) RequireServerWalletsEnabled() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		if os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") != "true" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Server wallets are not enabled",
			})
		}
		return c.Next()
	}
}

func (a *App) GetAgentDetails() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get user ID and agent from context
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User ID missing",
			})
		}

		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Agent not found in context",
			})
		}

		agentId := agent.ID.String()

		// 2. Load or create pool in memory
		var pool *state.AgentPool
		var exists bool
		if pool, exists = a.UserPools[userIDStr]; !exists {
			var err error
			pool, err = state.NewAgentPool(
				userIDStr,
				"", // Always use model from agent config
				os.Getenv("LOCALAGI_MULTIMODAL_MODEL"),
				os.Getenv("LOCALAGI_IMAGE_MODEL"),
				os.Getenv("LOCALAGI_LOCALRAG_URL"),
				services.Actions(map[string]string{
					services.ActionConfigSSHBoxURL: os.Getenv("LOCALAGI_SSHBOX_URL"),
				}),
				services.Connectors,
				services.DynamicPrompts,
				services.Filters,
				os.Getenv("LOCALAGI_TIMEOUT"),
				os.Getenv("LOCALAGI_ENABLE_CONVERSATIONS_LOGGING") == "true",
			)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to load agent pool",
				})
			}
			a.UserPools[userIDStr] = pool
		}

		// 3. Just check if agent is running in memory, don't create it
		active := false
		if instance := pool.GetAgent(agentId); instance != nil {
			active = !instance.Paused()
		}

		// 5. Return status
		return c.JSON(fiber.Map{
			"active": active,
		})
	}
}

func mustStringify(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (a *App) GetChatHistory() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get agent from context
		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return errorJSONMessage(c, "Agent not found in context")
		}

		// 2. Fetch messages (you can sort DESC + limit for recent)
		var messages []models.AgentMessage
		if err := db.DB.
			Where("AgentID = ?", agent.ID).
			Order("createdAt ASC").
			Find(&messages).Error; err != nil {
			return errorJSONMessage(c, "Failed to fetch messages: "+err.Error())
		}

		// // 3. Format for frontend
		// formatted := make([]fiber.Map, 0, len(messages))
		// for _, msg := range messages {
		// 	formatted = append(formatted, fiber.Map{
		// 		"sender":    msg.Sender,
		// 		"content":   msg.Content,
		// 		"createdAt": msg.CreatedAt,
		// 	})
		// }

		return c.JSON(fiber.Map{
			"messages": messages,
		})
	}
}

func (a *App) ClearChat() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// 1. Get agent from context
		agent, ok := c.Locals("agent").(*models.Agent)
		if !ok || agent == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Agent not found in context"})
		}

		// 2. Delete agent messages from DB
		if err := db.DB.
			Where("AgentID = ?", agent.ID).
			Delete(&models.AgentMessage{}).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to clear chat"})
		}

		// 3. Optionally: clear in-memory HUD or status if needed

		return c.JSON(fiber.Map{"success": true, "message": "Chat cleared"})
	}
}

// GetUsage returns the LLM usage records for the authenticated user
func (a *App) GetUsage() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("id").(string)
		if !ok || userID == "" {
			return errorJSONMessage(c, "User ID missing")
		}
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return errorJSONMessage(c, "Invalid user ID")
		}
		var usages []models.LLMUsage
		if err := db.DB.Where("UserID = ?", userUUID).Find(&usages).Error; err != nil {
			return errorJSONMessage(c, "Failed to fetch usage data: "+err.Error())
		}
		return c.JSON(usages)
	}
}

func (a *App) SubmitPaymentHeader() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("id").(string)
		if !ok || userIDStr == "" {
			return errorJSONMessage(c, "User ID missing")
		}

		requestIDStr := c.Params("requestId")
		if requestIDStr == "" {
			return errorJSONMessage(c, "Request ID missing")
		}

		requestID, err := uuid.Parse(requestIDStr)
		if err != nil {
			return errorJSONMessage(c, "Invalid request ID format")
		}

		var payload struct {
			SelectedRequestID string `json:"selectedRequestID"`
			PaymentHeader     string `json:"paymentHeader"`
			Status            string `json:"status"`
		}

		if err := c.BodyParser(&payload); err != nil {
			return errorJSONMessage(c, "Invalid request body: "+err.Error())
		}

		var request models.H402PendingRequests
		if err := db.DB.Where("ID = ? AND UserID = ?", requestID, userIDStr).
			First(&request).Error; err != nil {
			return errorJSONMessage(c, "Request not found or access denied")
		}

		var responseMessage string
		var responseStatus string

		if payload.Status == "CANCELLED" {
			if err := db.DB.Model(&request).Updates(map[string]interface{}{
				"Status": "CANCELLED",
			}).Error; err != nil {
				return errorJSONMessage(c, "Failed to update request: "+err.Error())
			}
			responseMessage = "Signed transaction submitted for cancelled request"
			responseStatus = "CANCELLED"
		} else {

			if payload.PaymentHeader == "" {
				return errorJSONMessage(c, "Signed transaction is required")
			}

			if payload.SelectedRequestID == "" {
				return errorJSONMessage(c, "Selected request ID is required")
			}

			selectedRequestID, err := uuid.Parse(payload.SelectedRequestID)

			if err != nil {
				return errorJSONMessage(c, "Invalid selected request ID format")
			}

			if err := db.DB.Model(&request).Updates(map[string]interface{}{
				"PaymentHeader":     payload.PaymentHeader,
				"SelectedRequestID": selectedRequestID,
				"Status":            "APPROVED",
			}).Error; err != nil {
				return errorJSONMessage(c, "Failed to update request: "+err.Error())
			}
			responseMessage = "Signed transaction submitted successfully"
			responseStatus = "APPROVED"
		}

		return c.JSON(fiber.Map{
			"status":  "success",
			"message": responseMessage,
			"data": fiber.Map{
				"requestId":         requestID.String(),
				"selectedRequestID": payload.SelectedRequestID,
				"paymentHeader":     payload.PaymentHeader,
				"status":            responseStatus,
			},
		})
	}
}
