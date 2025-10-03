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
	calendar "google.golang.org/api/calendar/v3"
)

func NewGoogleCalendarGetFreeBusy(config map[string]string) *GoogleCalendarGetFreeBusyAction {
	return &GoogleCalendarGetFreeBusyAction{}
}

type GoogleCalendarGetFreeBusyAction struct{}

func (a *GoogleCalendarGetFreeBusyAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	calendarService, err := oauth.GetCalendarClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Calendar client: %v", err)
	}

	// Parse calendar ID
	calendarID, ok := params["calendarId"].(string)
	if !ok || calendarID == "" {
		calendarID = "primary"
	}

	var calendarItems []*calendar.FreeBusyRequestItem
	calendarItems = append(calendarItems, &calendar.FreeBusyRequestItem{
		Id: calendarID,
	})

	// Parse time boundaries
	timeMin, ok := params["timeMin"].(string)
	if !ok || timeMin == "" {
		return types.ActionResult{}, fmt.Errorf("timeMin is required")
	}

	timeMax, ok := params["timeMax"].(string)
	if !ok || timeMax == "" {
		return types.ActionResult{}, fmt.Errorf("timeMax is required")
	}

	// Optional: timeZone (get it first as we need it for time formatting)
	// If not provided, try to get the calendar's timezone
	timeZone := ""
	if tz, ok := params["timeZone"].(string); ok && tz != "" {
		timeZone = tz
	} else {
		calendarTZ, err := getCalendarTimezone(ctx, calendarService, calendarID)
		if err == nil && calendarTZ != "" {
			timeZone = calendarTZ
		} else {
			timeZone = "UTC"
		}
	}

	// Validate and format time boundaries using existing utility
	timeMinFormatted, err := formatTimeForAPI(timeMin, timeZone)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid timeMin format: %v", err)
	}

	timeMaxFormatted, err := formatTimeForAPI(timeMax, timeZone)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid timeMax format: %v", err)
	}

	// Build FreeBusy request
	freeBusyRequest := &calendar.FreeBusyRequest{
		TimeMin: timeMinFormatted,
		TimeMax: timeMaxFormatted,
		Items:   calendarItems,
	}

	// Set timeZone if provided
	if timeZone != "" {
		freeBusyRequest.TimeZone = timeZone
	}

	// Execute FreeBusy query
	freeBusyResponse, err := calendarService.Freebusy.Query(freeBusyRequest).Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to query free/busy information: %v", err)
	}

	// Format the response
	formattedResponse := formatFreeBusyResponse(freeBusyResponse, timeMin, timeMax)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatFreeBusyResponse(response *calendar.FreeBusyResponse, timeMin, timeMax string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Free/Busy Query Results\n"))
	result.WriteString(fmt.Sprintf("Time Range: %s to %s\n\n", timeMin, timeMax))

	if len(response.Calendars) == 0 {
		result.WriteString("No calendar information returned.\n")
		return result.String()
	}

	for calID, calData := range response.Calendars {
		result.WriteString(fmt.Sprintf("Calendar: %s\n", calID))

		if len(calData.Errors) > 0 {
			result.WriteString("  Errors:\n")
			for _, e := range calData.Errors {
				result.WriteString(fmt.Sprintf("    - %s: %s\n", e.Domain, e.Reason))
			}
		}

		if len(calData.Busy) == 0 {
			result.WriteString("  Status: Free (no busy periods)\n")
		} else {
			result.WriteString(fmt.Sprintf("  Busy Periods (%d):\n", len(calData.Busy)))
			for i, period := range calData.Busy {
				startTime, _ := time.Parse(time.RFC3339, period.Start)
				endTime, _ := time.Parse(time.RFC3339, period.End)

				result.WriteString(fmt.Sprintf("    %d. Start: %s\n", i+1, formatTime(startTime)))
				result.WriteString(fmt.Sprintf("       End:   %s\n", formatTime(endTime)))
				result.WriteString(fmt.Sprintf("       Duration: %s\n", formatDuration(endTime.Sub(startTime))))
			}
		}

		result.WriteString("\n")
	}

	return result.String()
}

func formatTime(t time.Time) string {
	return t.Format("Mon Jan 02, 2006 at 3:04 PM MST")
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (a *GoogleCalendarGetFreeBusyAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-get-freebusy",
		Description: "Query free/busy information for a Google Calendar within a specified time range. Returns periods when the calendar is busy with events.",
		Properties: map[string]jsonschema.Definition{
			"calendarId": {
				Type:        jsonschema.String,
				Description: "ID of the calendar to query (use 'primary' for the main calendar)",
			},
			"timeMin": {
				Type:        jsonschema.String,
				Description: "Start time boundary in ISO 8601 format. Preferred: '2024-01-01T00:00:00' (uses timeZone parameter or calendar timezone). Also accepts: '2024-01-01T00:00:00Z' or '2024-01-01T00:00:00-08:00'.",
			},
			"timeMax": {
				Type:        jsonschema.String,
				Description: "End time boundary in ISO 8601 format. Preferred: '2024-01-01T23:59:59' (uses timeZone parameter or calendar timezone). Also accepts: '2024-01-01T23:59:59Z' or '2024-01-01T23:59:59-08:00'.",
			},
			"timeZone": {
				Type:        jsonschema.String,
				Description: "Timezone for the query (e.g., 'America/New_York', 'Europe/London'). If not specified, uses the calendar's default timezone.",
			},
		},
		Required: []string{"calendarId", "timeMin", "timeMax"},
	}
}

func (a *GoogleCalendarGetFreeBusyAction) Plannable() bool {
	return true
}

func GoogleCalendarGetFreeBusyConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
