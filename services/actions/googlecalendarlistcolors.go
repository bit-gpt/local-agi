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

func NewGoogleCalendarListColors(config map[string]string) *GoogleCalendarListColorsAction {
	return &GoogleCalendarListColorsAction{}
}

type GoogleCalendarListColorsAction struct{}

type ColorDefinition struct {
	Background string `json:"background"`
	Foreground string `json:"foreground"`
}

type ColorsResponse struct {
	EventColors    map[string]ColorDefinition `json:"event,omitempty"`
	CalendarColors map[string]ColorDefinition `json:"calendar,omitempty"`
	Updated        string                     `json:"updated,omitempty"`
}

func (a *GoogleCalendarListColorsAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	calendarService, err := oauth.GetCalendarClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Calendar client: %v", err)
	}

	colorsCall := calendarService.Colors.Get().Context(ctx)

	colorsResponse, err := colorsCall.Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to retrieve calendar colors: %v", err)
	}

	if colorsResponse == nil {
		return types.ActionResult{
			Result: "No color information available",
		}, nil
	}

	// Convert the response to our format
	colors := ColorsResponse{
		Updated: colorsResponse.Updated,
	}

	// Convert event colors
	if len(colorsResponse.Event) > 0 {
		colors.EventColors = make(map[string]ColorDefinition)

		count := 0
		for id, color := range colorsResponse.Event {
			if count >= 50 {
				break
			}
			colors.EventColors[id] = ColorDefinition{
				Background: color.Background,
				Foreground: color.Foreground,
			}
			count++
		}
	}

	// Convert calendar colors
	if len(colorsResponse.Calendar) > 0 {
		colors.CalendarColors = make(map[string]ColorDefinition)

		count := 0
		for id, color := range colorsResponse.Calendar {
			if count >= 50 {
				break
			}
			colors.CalendarColors[id] = ColorDefinition{
				Background: color.Background,
				Foreground: color.Foreground,
			}
			count++
		}
	}

	formattedResponse := formatColorsResponse(colors)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatColorsResponse(colors ColorsResponse) string {
	var response strings.Builder

	response.WriteString("Google Calendar Color Palette\n")
	response.WriteString("=============================\n\n")

	if colors.Updated != "" {
		response.WriteString(fmt.Sprintf("Last Updated: %s\n\n", colors.Updated))
	}

	// Format event colors
	if len(colors.EventColors) > 0 {
		response.WriteString("Event Colors:\n")
		response.WriteString("-------------\n")
		for id, color := range colors.EventColors {
			response.WriteString(fmt.Sprintf("Color ID %s:\n", id))
			response.WriteString(fmt.Sprintf("  Background: %s\n", color.Background))
			response.WriteString(fmt.Sprintf("  Foreground: %s\n", color.Foreground))
			response.WriteString("\n")
		}
	}

	// Format calendar colors
	if len(colors.CalendarColors) > 0 {
		response.WriteString("Calendar Colors:\n")
		response.WriteString("----------------\n")
		for id, color := range colors.CalendarColors {
			response.WriteString(fmt.Sprintf("Color ID %s:\n", id))
			response.WriteString(fmt.Sprintf("  Background: %s\n", color.Background))
			response.WriteString(fmt.Sprintf("  Foreground: %s\n", color.Foreground))
			response.WriteString("\n")
		}
	}

	if len(colors.EventColors) == 0 && len(colors.CalendarColors) == 0 {
		response.WriteString("No color information available.\n")
	}

	response.WriteString("\nNote: Use these color IDs when creating or updating events/calendars.\n")

	return response.String()
}

func (a *GoogleCalendarListColorsAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-list-colors",
		Description: "Retrieve the color palette available for Google Calendar events and calendars. Returns color IDs with their corresponding background and foreground hex color codes. Use these color IDs when creating or updating events to apply specific colors.",
		Properties:  map[string]jsonschema.Definition{},
		Required:    []string{},
	}
}

func (a *GoogleCalendarListColorsAction) Plannable() bool {
	return true
}

func GoogleCalendarListColorsConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
