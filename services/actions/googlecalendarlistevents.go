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
	ID                      string               `json:"id"`
	Summary                 string               `json:"summary"`
	Description             string               `json:"description,omitempty"`
	Location                string               `json:"location,omitempty"`
	Start                   *EventTime           `json:"start"`
	End                     *EventTime           `json:"end"`
	Status                  string               `json:"status"`
	HtmlLink                string               `json:"htmlLink"`
	Attendees               []Attendee           `json:"attendees,omitempty"`
	Creator                 *Person              `json:"creator,omitempty"`
	Organizer               *Person              `json:"organizer,omitempty"`
	Recurrence              []string             `json:"recurrence,omitempty"`
	Visibility              string               `json:"visibility,omitempty"`
	ColorId                 string               `json:"colorId,omitempty"`
	Transparency            string               `json:"transparency,omitempty"`
	Created                 string               `json:"created,omitempty"`
	Updated                 string               `json:"updated,omitempty"`
	RecurringEventId        string               `json:"recurringEventId,omitempty"`
	OriginalStartTime       *EventTime           `json:"originalStartTime,omitempty"`
	ICalUID                 string               `json:"iCalUID,omitempty"`
	Sequence                int64                `json:"sequence,omitempty"`
	HangoutLink             string               `json:"hangoutLink,omitempty"`
	AnyoneCanAddSelf        bool                 `json:"anyoneCanAddSelf,omitempty"`
	GuestsCanInviteOthers   bool                 `json:"guestsCanInviteOthers,omitempty"`
	GuestsCanModify         bool                 `json:"guestsCanModify,omitempty"`
	GuestsCanSeeOtherGuests bool                 `json:"guestsCanSeeOtherGuests,omitempty"`
	PrivateCopy             bool                 `json:"privateCopy,omitempty"`
	Locked                  bool                 `json:"locked,omitempty"`
	EventType               string               `json:"eventType,omitempty"`
	ConferenceData          *ConferenceData      `json:"conferenceData,omitempty"`
	Attachments             []CalendarAttachment `json:"attachments,omitempty"`
	Source                  *EventSource         `json:"source,omitempty"`
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

type ConferenceData struct {
	CreateRequest      *CreateRequest      `json:"createRequest,omitempty"`
	EntryPoints        []EntryPoint        `json:"entryPoints,omitempty"`
	ConferenceSolution *ConferenceSolution `json:"conferenceSolution,omitempty"`
	ConferenceId       string              `json:"conferenceId,omitempty"`
	Signature          string              `json:"signature,omitempty"`
	Notes              string              `json:"notes,omitempty"`
}

type CreateRequest struct {
	RequestId             string                 `json:"requestId,omitempty"`
	ConferenceSolutionKey *ConferenceSolutionKey `json:"conferenceSolutionKey,omitempty"`
}

type ConferenceSolutionKey struct {
	Type string `json:"type,omitempty"`
}

type EntryPoint struct {
	EntryPointType string `json:"entryPointType,omitempty"`
	Uri            string `json:"uri,omitempty"`
	Label          string `json:"label,omitempty"`
	Pin            string `json:"pin,omitempty"`
	AccessCode     string `json:"accessCode,omitempty"`
	MeetingCode    string `json:"meetingCode,omitempty"`
	Passcode       string `json:"passcode,omitempty"`
	Password       string `json:"password,omitempty"`
}

type ConferenceSolution struct {
	Key     *ConferenceSolutionKey `json:"key,omitempty"`
	Name    string                 `json:"name,omitempty"`
	IconUri string                 `json:"iconUri,omitempty"`
}

type EventSource struct {
	Title string `json:"title,omitempty"`
	Url   string `json:"url,omitempty"`
}

// formatTimeForAPI converts time strings to RFC3339 format required by Google Calendar API
func formatTimeForAPI(timeStr, timeZone string) (string, error) {
	if timeStr == "" {
		return "", nil
	}

	// If already in RFC3339 format (has timezone), return as-is
	if strings.Contains(timeStr, "Z") || strings.Contains(timeStr, "+") ||
		(strings.Contains(timeStr, "-") && len(timeStr) > 19) {
		return timeStr, nil
	}

	// Determine location
	var loc *time.Location
	var err error
	if timeZone != "" {
		loc, err = time.LoadLocation(timeZone)
		if err != nil {
			return "", fmt.Errorf("invalid timezone '%s': %v", timeZone, err)
		}
	} else {
		loc = time.UTC
	}

	// Try parsing common formats in the specified location
	var t time.Time
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err = time.ParseInLocation(format, timeStr, loc)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("unable to parse time '%s': %v", timeStr, err)
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

	if timeZone == "" {
		calendarTZ, err := getCalendarTimezone(ctx, calendarService, calendarID)
		if err == nil && calendarTZ != "" {
			timeZone = calendarTZ
		} else {
			timeZone = "UTC"
		}
	}

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

	// Limit to first 20 events if more than 20 are found
	eventsToProcess := eventsResponse.Items
	if len(eventsResponse.Items) > 20 {
		eventsToProcess = eventsResponse.Items[:20]
	}

	// Convert to our struct format
	var events []Event
	for _, event := range eventsToProcess {
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
			normalizedStartTime := event.Start.DateTime
			if normalizedStartTime != "" && event.Start.TimeZone != "" {
				var err error
				normalizedStartTime, err = normalizeEventTime(event.Start.DateTime, event.Start.TimeZone)
				if err != nil {
					fmt.Printf("Warning: failed to normalize start time for event %s: %v\n", event.Id, err)
					normalizedStartTime = event.Start.DateTime
				}
			}

			convertedEvent.Start = &EventTime{
				DateTime: normalizedStartTime,
				Date:     event.Start.Date,
				TimeZone: event.Start.TimeZone,
			}
		}

		// Convert end time
		if event.End != nil {
			normalizedEndTime := event.End.DateTime
			if normalizedEndTime != "" && event.End.TimeZone != "" {
				var err error
				normalizedEndTime, err = normalizeEventTime(event.End.DateTime, event.End.TimeZone)
				if err != nil {
					fmt.Printf("Warning: failed to normalize end time for event %s: %v\n", event.Id, err)
					normalizedEndTime = event.End.DateTime
				}
			}

			convertedEvent.End = &EventTime{
				DateTime: normalizedEndTime,
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

	// Additional optional fields
	if event.ColorId != "" {
		details.WriteString(fmt.Sprintf("   Color ID: %s\n", event.ColorId))
	}

	if event.Transparency != "" {
		details.WriteString(fmt.Sprintf("   Transparency: %s\n", event.Transparency))
	}

	if event.Created != "" {
		details.WriteString(fmt.Sprintf("   Created: %s\n", event.Created))
	}

	if event.Updated != "" {
		details.WriteString(fmt.Sprintf("   Updated: %s\n", event.Updated))
	}

	if event.RecurringEventId != "" {
		details.WriteString(fmt.Sprintf("   Recurring Event ID: %s\n", event.RecurringEventId))
	}

	if event.OriginalStartTime != nil {
		if event.OriginalStartTime.DateTime != "" {
			details.WriteString(fmt.Sprintf("   Original Start: %s", event.OriginalStartTime.DateTime))
			if event.OriginalStartTime.TimeZone != "" {
				details.WriteString(fmt.Sprintf(" (%s)", event.OriginalStartTime.TimeZone))
			}
			details.WriteString("\n")
		} else if event.OriginalStartTime.Date != "" {
			details.WriteString(fmt.Sprintf("   Original Start Date: %s\n", event.OriginalStartTime.Date))
		}
	}

	if event.ICalUID != "" {
		details.WriteString(fmt.Sprintf("   iCal UID: %s\n", event.ICalUID))
	}

	if event.Sequence != 0 {
		details.WriteString(fmt.Sprintf("   Sequence: %d\n", event.Sequence))
	}

	if event.HangoutLink != "" {
		details.WriteString(fmt.Sprintf("   Hangout Link: %s\n", event.HangoutLink))
	}

	if event.EventType != "" {
		details.WriteString(fmt.Sprintf("   Event Type: %s\n", event.EventType))
	}

	// Guest permissions
	guestPermissions := []string{}
	if event.AnyoneCanAddSelf {
		guestPermissions = append(guestPermissions, "Anyone can add self")
	}
	if event.GuestsCanInviteOthers {
		guestPermissions = append(guestPermissions, "Guests can invite others")
	}
	if event.GuestsCanModify {
		guestPermissions = append(guestPermissions, "Guests can modify")
	}
	if event.GuestsCanSeeOtherGuests {
		guestPermissions = append(guestPermissions, "Guests can see other guests")
	}
	if event.PrivateCopy {
		guestPermissions = append(guestPermissions, "Private copy")
	}
	if event.Locked {
		guestPermissions = append(guestPermissions, "Locked")
	}

	if len(guestPermissions) > 0 {
		details.WriteString(fmt.Sprintf("   Guest Permissions: %s\n", strings.Join(guestPermissions, ", ")))
	}

	if event.ConferenceData != nil {
		details.WriteString(fmt.Sprintf("   Conference Data: %s\n", event.ConferenceData.ConferenceId))
		details.WriteString(fmt.Sprintf("     - %s\n", event.ConferenceData.Signature))
		details.WriteString(fmt.Sprintf("     - %s\n", event.ConferenceData.Notes))
		if event.ConferenceData.CreateRequest != nil {
			details.WriteString(fmt.Sprintf("     - Create Request: %s\n", event.ConferenceData.CreateRequest.RequestId))
		}
		if event.ConferenceData.ConferenceSolution != nil {
			details.WriteString(fmt.Sprintf("     - Conference Solution: %s\n", event.ConferenceData.ConferenceSolution.Name))
		}
	}

	if len(event.Attachments) > 0 {
		details.WriteString("   Attachments:\n")
		for _, attachment := range event.Attachments {
			details.WriteString(fmt.Sprintf("     - %s\n", attachment.FileId))
			details.WriteString(fmt.Sprintf("     - %s\n", attachment.FileUrl))
			details.WriteString(fmt.Sprintf("     - %s\n", attachment.Title))
			details.WriteString(fmt.Sprintf("     - %s\n", attachment.MimeType))
			details.WriteString(fmt.Sprintf("     - %s\n", attachment.IconLink))
		}
	}

	if event.Source != nil {
		details.WriteString(fmt.Sprintf("   Source: %s\n", event.Source.Title))
		details.WriteString(fmt.Sprintf("     - %s\n", event.Source.Url))
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
