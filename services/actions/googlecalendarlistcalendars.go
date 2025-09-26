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

func NewGoogleCalendarListCalendars(config map[string]string) *GoogleCalendarListCalendarsAction {
	return &GoogleCalendarListCalendarsAction{}
}

type GoogleCalendarListCalendarsAction struct{}

type Calendar struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	TimeZone    string `json:"timeZone,omitempty"`
	AccessRole  string `json:"accessRole,omitempty"`
	Selected    bool   `json:"selected,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

func (a *GoogleCalendarListCalendarsAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	calendarService, err := oauth.GetCalendarClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Calendar client: %v", err)
	}

	// Get all calendars
	calendarsResponse, err := calendarService.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to list calendars: %v", err)
	}

	if calendarsResponse == nil || len(calendarsResponse.Items) == 0 {
		return types.ActionResult{
			Result: "No calendars found in Google Calendar account",
		}, nil
	}

	// Convert to our struct format
	var calendars []Calendar
	for _, cal := range calendarsResponse.Items {
		calendar := Calendar{
			ID:          cal.Id,
			Summary:     cal.Summary,
			Description: cal.Description,
			TimeZone:    cal.TimeZone,
			AccessRole:  cal.AccessRole,
			Selected:    cal.Selected,
			Primary:     cal.Primary,
		}
		calendars = append(calendars, calendar)
	}

	formattedResponse := formatCalendarsResponse(calendars)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatCalendarsResponse(calendars []Calendar) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("Found %d calendars in Google Calendar account:\n\n", len(calendars)))

	// Separate primary calendar from others
	var primaryCalendars []Calendar
	var otherCalendars []Calendar

	for _, calendar := range calendars {
		if calendar.Primary {
			primaryCalendars = append(primaryCalendars, calendar)
		} else {
			otherCalendars = append(otherCalendars, calendar)
		}
	}

	// Display primary calendar first
	if len(primaryCalendars) > 0 {
		response.WriteString("Primary Calendar:\n")
		response.WriteString(strings.Repeat("-", 50) + "\n")
		for i, calendar := range primaryCalendars {
			response.WriteString(formatCalendarDetails(calendar, i+1))
			if i < len(primaryCalendars)-1 {
				response.WriteString("\n")
			}
		}
		response.WriteString("\n\n")
	}

	// Display other calendars
	if len(otherCalendars) > 0 {
		response.WriteString("Other Calendars:\n")
		response.WriteString(strings.Repeat("-", 50) + "\n")
		for i, calendar := range otherCalendars {
			response.WriteString(formatCalendarDetails(calendar, i+1))
			if i < len(otherCalendars)-1 {
				response.WriteString("\n")
			}
		}
	}

	return response.String()
}

func formatCalendarDetails(calendar Calendar, index int) string {
	var details strings.Builder

	details.WriteString(fmt.Sprintf("%d. %s\n", index, calendar.Summary))
	details.WriteString(fmt.Sprintf("   ID: %s\n", calendar.ID))
	details.WriteString(fmt.Sprintf("   Access Role: %s\n", calendar.AccessRole))

	if calendar.Description != "" {
		details.WriteString(fmt.Sprintf("   Description: %s\n", calendar.Description))
	}
	if calendar.TimeZone != "" {
		details.WriteString(fmt.Sprintf("   Time Zone: %s\n", calendar.TimeZone))
	}

	if calendar.Selected {
		details.WriteString("   Status: Selected\n")
	}
	if calendar.Primary {
		details.WriteString("   Status: Primary\n")
	}

	return details.String()
}

func (a *GoogleCalendarListCalendarsAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-list-calendars",
		Description: "List all calendars in Google Calendar account. Returns both primary and secondary calendars with their details like ID, name, description, timezone, and access role.",
		Properties:  map[string]jsonschema.Definition{},
		Required:    []string{},
	}
}

func (a *GoogleCalendarListCalendarsAction) Plannable() bool {
	return true
}

func GoogleCalendarListCalendarsConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
