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

func NewGoogleCalendarListEvents(config map[string]string) *GoogleCalendarListEventsAction {
	return &GoogleCalendarListEventsAction{}
}

type GoogleCalendarListEventsAction struct{}

type Event struct {
	ID          string     `json:"id"`
	Summary     string     `json:"summary"`
	Description string     `json:"description,omitempty"`
	Location    string     `json:"location,omitempty"`
	Start       *EventTime `json:"start"`
	End         *EventTime `json:"end"`
	Status      string     `json:"status"`
	HtmlLink    string     `json:"htmlLink"`
	Attendees   []Attendee `json:"attendees,omitempty"`
	Creator     *Person    `json:"creator,omitempty"`
	Organizer   *Person    `json:"organizer,omitempty"`
	Recurrence  []string   `json:"recurrence,omitempty"`
	Visibility  string     `json:"visibility,omitempty"`
}

type EventTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type Attendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
	Optional       bool   `json:"optional,omitempty"`
}

type Person struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
}

// formatTimeForAPI converts time strings to RFC3339 format required by Google Calendar API
func formatTimeForAPI(timeStr, timeZone string) (string, error) {
	if timeStr == "" {
		return "", nil
	}

	// If already in RFC3339 format (has timezone), return as-is
	if strings.Contains(timeStr, "Z") || strings.Contains(timeStr, "+") || strings.Contains(timeStr, "-") && len(timeStr) > 19 {
		return timeStr, nil
	}

	// Parse the time string
	var t time.Time
	var err error

	// Try parsing common formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err = time.Parse(format, timeStr)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("unable to parse time '%s': %v", timeStr, err)
	}

	// If we have a timezone, apply it
	if timeZone != "" {
		loc, err := time.LoadLocation(timeZone)
		if err != nil {
			return "", fmt.Errorf("invalid timezone '%s': %v", timeZone, err)
		}
		// Interpret the parsed time in the specified timezone
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
	} else {
		// Default to UTC if no timezone specified
		t = t.UTC()
	}

	return t.Format(time.RFC3339), nil
}

func (a *GoogleCalendarListEventsAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	calendarService, err := oauth.GetCalendarClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Calendar client: %v", err)
	}

	// Extract parameters
	calendarID, ok := params["calendarId"].(string)
	if !ok || calendarID == "" {
		calendarID = "primary"
	}

	query, _ := params["query"].(string)
	timeMin, _ := params["timeMin"].(string)
	timeMax, _ := params["timeMax"].(string)
	timeZone, _ := params["timeZone"].(string)

	// Format time parameters for API
	formattedTimeMin, err := formatTimeForAPI(timeMin, timeZone)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid timeMin parameter: %v", err)
	}

	formattedTimeMax, err := formatTimeForAPI(timeMax, timeZone)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("invalid timeMax parameter: %v", err)
	}

	// Handle fields parameter
	var fields []string
	if fieldsParam, ok := params["fields"].([]interface{}); ok {
		for _, field := range fieldsParam {
			if fieldStr, ok := field.(string); ok {
				fields = append(fields, fieldStr)
			}
		}
	}

	// Handle extended properties
	var privateExtendedProps []string
	if propsParam, ok := params["privateExtendedProperty"].([]interface{}); ok {
		for _, prop := range propsParam {
			if propStr, ok := prop.(string); ok {
				privateExtendedProps = append(privateExtendedProps, propStr)
			}
		}
	}

	var sharedExtendedProps []string
	if propsParam, ok := params["sharedExtendedProperty"].([]interface{}); ok {
		for _, prop := range propsParam {
			if propStr, ok := prop.(string); ok {
				sharedExtendedProps = append(sharedExtendedProps, propStr)
			}
		}
	}

	// Build the events list call
	eventsCall := calendarService.Events.List(calendarID).Context(ctx)

	if query != "" {
		eventsCall = eventsCall.Q(query)
	}
	if formattedTimeMin != "" {
		eventsCall = eventsCall.TimeMin(formattedTimeMin)
	}
	if formattedTimeMax != "" {
		eventsCall = eventsCall.TimeMax(formattedTimeMax)
	}
	if timeZone != "" {
		eventsCall = eventsCall.TimeZone(timeZone)
	}

	// Add extended property filters
	for _, prop := range privateExtendedProps {
		eventsCall = eventsCall.PrivateExtendedProperty(prop)
	}
	for _, prop := range sharedExtendedProps {
		eventsCall = eventsCall.SharedExtendedProperty(prop)
	}

	// Execute the call
	eventsResponse, err := eventsCall.Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to list events for calendar %s: %v. Parameters: timeMin='%s' (formatted: '%s'), timeMax='%s' (formatted: '%s'), timeZone='%s', query='%s'",
			calendarID, err, timeMin, formattedTimeMin, timeMax, formattedTimeMax, timeZone, query)
	}

	if eventsResponse == nil || len(eventsResponse.Items) == 0 {
		return types.ActionResult{
			Result: fmt.Sprintf("No events found in calendar '%s' with the specified criteria", calendarID),
		}, nil
	}

	// Convert to our struct format
	var events []Event
	for _, event := range eventsResponse.Items {
		convertedEvent := Event{
			ID:          event.Id,
			Summary:     event.Summary,
			Description: event.Description,
			Location:    event.Location,
			Status:      event.Status,
			HtmlLink:    event.HtmlLink,
			Visibility:  event.Visibility,
		}

		// Convert start time
		if event.Start != nil {
			convertedEvent.Start = &EventTime{
				DateTime: event.Start.DateTime,
				Date:     event.Start.Date,
				TimeZone: event.Start.TimeZone,
			}
		}

		// Convert end time
		if event.End != nil {
			convertedEvent.End = &EventTime{
				DateTime: event.End.DateTime,
				Date:     event.End.Date,
				TimeZone: event.End.TimeZone,
			}
		}

		// Convert attendees
		if len(event.Attendees) > 0 {
			for _, attendee := range event.Attendees {
				convertedEvent.Attendees = append(convertedEvent.Attendees, Attendee{
					Email:          attendee.Email,
					DisplayName:    attendee.DisplayName,
					ResponseStatus: attendee.ResponseStatus,
					Optional:       attendee.Optional,
				})
			}
		}

		// Convert creator
		if event.Creator != nil {
			convertedEvent.Creator = &Person{
				Email:       event.Creator.Email,
				DisplayName: event.Creator.DisplayName,
			}
		}

		// Convert organizer
		if event.Organizer != nil {
			convertedEvent.Organizer = &Person{
				Email:       event.Organizer.Email,
				DisplayName: event.Organizer.DisplayName,
			}
		}

		// Copy recurrence rules
		if len(event.Recurrence) > 0 {
			convertedEvent.Recurrence = make([]string, len(event.Recurrence))
			copy(convertedEvent.Recurrence, event.Recurrence)
		}

		events = append(events, convertedEvent)
	}

	formattedResponse := formatEventsResponse(events, calendarID, query, timeMin, timeMax)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatEventsResponse(events []Event, calendarID, query, timeMin, timeMax string) string {
	var response strings.Builder

	// Header with search criteria
	response.WriteString(fmt.Sprintf("Found %d events", len(events)))
	if calendarID != "" {
		response.WriteString(fmt.Sprintf(" in calendar '%s'", calendarID))
	}
	if query != "" {
		response.WriteString(fmt.Sprintf(" matching query '%s'", query))
	}
	if timeMin != "" || timeMax != "" {
		response.WriteString(" within time range")
		if timeMin != "" {
			response.WriteString(fmt.Sprintf(" from %s", timeMin))
		}
		if timeMax != "" {
			response.WriteString(fmt.Sprintf(" to %s", timeMax))
		}
	}
	response.WriteString(":\n\n")

	// List events
	for i, event := range events {
		response.WriteString(formatEventDetails(event, i+1))
		if i < len(events)-1 {
			response.WriteString("\n" + strings.Repeat("-", 60) + "\n\n")
		}
	}

	return response.String()
}

func formatEventDetails(event Event, index int) string {
	var details strings.Builder

	details.WriteString(fmt.Sprintf("%d. %s\n", index, event.Summary))
	details.WriteString(fmt.Sprintf("   ID: %s\n", event.ID))
	details.WriteString(fmt.Sprintf("   Status: %s\n", event.Status))

	// Format start and end times
	if event.Start != nil {
		if event.Start.DateTime != "" {
			details.WriteString(fmt.Sprintf("   Start: %s", event.Start.DateTime))
			if event.Start.TimeZone != "" {
				details.WriteString(fmt.Sprintf(" (%s)", event.Start.TimeZone))
			}
			details.WriteString("\n")
		} else if event.Start.Date != "" {
			details.WriteString(fmt.Sprintf("   Start Date: %s\n", event.Start.Date))
		}
	}

	if event.End != nil {
		if event.End.DateTime != "" {
			details.WriteString(fmt.Sprintf("   End: %s", event.End.DateTime))
			if event.End.TimeZone != "" {
				details.WriteString(fmt.Sprintf(" (%s)", event.End.TimeZone))
			}
			details.WriteString("\n")
		} else if event.End.Date != "" {
			details.WriteString(fmt.Sprintf("   End Date: %s\n", event.End.Date))
		}
	}

	if event.Location != "" {
		details.WriteString(fmt.Sprintf("   Location: %s\n", event.Location))
	}

	if event.Description != "" {
		// Truncate long descriptions
		description := event.Description
		if len(description) > 200 {
			description = description[:200] + "..."
		}
		details.WriteString(fmt.Sprintf("   Description: %s\n", description))
	}

	if len(event.Attendees) > 0 {
		details.WriteString("   Attendees:\n")
		for _, attendee := range event.Attendees {
			status := ""
			if attendee.ResponseStatus != "" {
				status = fmt.Sprintf(" (%s)", attendee.ResponseStatus)
			}
			optional := ""
			if attendee.Optional {
				optional = " [Optional]"
			}
			name := attendee.Email
			if attendee.DisplayName != "" {
				name = attendee.DisplayName
			}
			details.WriteString(fmt.Sprintf("     - %s%s%s\n", name, status, optional))
		}
	}

	if event.Creator != nil && event.Creator.Email != "" {
		creator := event.Creator.Email
		if event.Creator.DisplayName != "" {
			creator = event.Creator.DisplayName
		}
		details.WriteString(fmt.Sprintf("   Creator: %s\n", creator))
	}

	if event.Organizer != nil && event.Organizer.Email != "" {
		organizer := event.Organizer.Email
		if event.Organizer.DisplayName != "" {
			organizer = event.Organizer.DisplayName
		}
		details.WriteString(fmt.Sprintf("   Organizer: %s\n", organizer))
	}

	if len(event.Recurrence) > 0 {
		details.WriteString(fmt.Sprintf("   Recurrence: %s\n", strings.Join(event.Recurrence, "; ")))
	}

	if event.Visibility != "" {
		details.WriteString(fmt.Sprintf("   Visibility: %s\n", event.Visibility))
	}

	if event.HtmlLink != "" {
		details.WriteString(fmt.Sprintf("   Link: %s\n", event.HtmlLink))
	}

	return details.String()
}

func (a *GoogleCalendarListEventsAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-list-events",
		Description: "List events from a Google Calendar with various search and filtering options. Returns events with details like summary, time, location, attendees, and more.",
		Properties: map[string]jsonschema.Definition{
			"calendarId": {
				Type:        jsonschema.String,
				Description: "ID of the calendar (use 'primary' for the main calendar)",
			},
			"query": {
				Type:        jsonschema.String,
				Description: "Free text search query (searches summary, description, location, attendees, etc.)",
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
				Description: "Timezone as IANA Time Zone Database name (e.g., America/Los_Angeles). Takes priority over calendar's default timezone. Only used for timezone-naive datetime strings.",
			},
			"fields": {
				Type:        jsonschema.Array,
				Description: "Optional array of additional event fields to retrieve. Default fields (id, summary, start, end, status, htmlLink, location, attendees) are always included.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
					Enum: []string{
						"creator", "organizer", "recurrence", "visibility",
						"description", "transparency", "sequence", "reminders",
						"extendedProperties", "hangoutLink", "conferenceData",
					},
				},
			},
			"privateExtendedProperty": {
				Type:        jsonschema.Array,
				Description: "Filter by private extended properties (key=value format). Matches events that have all specified properties.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"sharedExtendedProperty": {
				Type:        jsonschema.Array,
				Description: "Filter by shared extended properties (key=value format). Matches events that have all specified properties.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
		},
		Required: []string{},
	}
}

func (a *GoogleCalendarListEventsAction) Plannable() bool {
	return true
}

func GoogleCalendarListEventsConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
