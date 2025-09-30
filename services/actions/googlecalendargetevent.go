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

// normalizeEventTime converts the event time to the specified timezone
func normalizeEventTime(dateTime, timeZone string) (string, error) {
	if dateTime == "" {
		return "", nil
	}

	// Parse the datetime (it comes with timezone offset from Google)
	t, err := time.Parse(time.RFC3339, dateTime)
	if err != nil {
		return "", fmt.Errorf("failed to parse datetime %s: %v", dateTime, err)
	}

	// If we have a specific timezone, convert to that timezone
	if timeZone != "" && timeZone != "UTC" {
		loc, err := time.LoadLocation(timeZone)
		if err != nil {
			return "", fmt.Errorf("invalid timezone %s: %v", timeZone, err)
		}
		// Convert to the target timezone
		t = t.In(loc)
	}

	return t.Format(time.RFC3339), nil
}

func NewGoogleCalendarGetEvent(config map[string]string) *GoogleCalendarGetEventAction {
	return &GoogleCalendarGetEventAction{}
}

type GoogleCalendarGetEventAction struct{}

type CalendarAttachment struct {
	FileId   string `json:"fileId,omitempty"`
	FileUrl  string `json:"fileUrl,omitempty"`
	Title    string `json:"title,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	IconLink string `json:"iconLink,omitempty"`
}

func (a *GoogleCalendarGetEventAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	calendarService, err := oauth.GetCalendarClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Calendar client: %v", err)
	}

	calendarID, ok := params["calendarId"].(string)
	if !ok || calendarID == "" {
		calendarID = "primary"
	}

	eventID, ok := params["eventId"].(string)
	if !ok || eventID == "" {
		return types.ActionResult{}, fmt.Errorf("eventId is required")
	}

	var fields []string
	if fieldsParam, ok := params["fields"].([]interface{}); ok {
		for _, field := range fieldsParam {
			if fieldStr, ok := field.(string); ok {
				fields = append(fields, fieldStr)
			}
		}
	}

	eventCall := calendarService.Events.Get(calendarID, eventID).Context(ctx)

	eventResponse, err := eventCall.Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get event %s from calendar %s: %v", eventID, calendarID, err)
	}

	if eventResponse == nil {
		return types.ActionResult{
			Result: fmt.Sprintf("Event '%s' not found in calendar '%s'", eventID, calendarID),
		}, nil
	}

	convertedEvent := Event{
		ID:                      eventResponse.Id,
		Summary:                 eventResponse.Summary,
		Description:             eventResponse.Description,
		Location:                eventResponse.Location,
		Status:                  eventResponse.Status,
		HtmlLink:                eventResponse.HtmlLink,
		Visibility:              eventResponse.Visibility,
		ColorId:                 eventResponse.ColorId,
		Transparency:            eventResponse.Transparency,
		Created:                 eventResponse.Created,
		Updated:                 eventResponse.Updated,
		RecurringEventId:        eventResponse.RecurringEventId,
		ICalUID:                 eventResponse.ICalUID,
		Sequence:                eventResponse.Sequence,
		HangoutLink:             eventResponse.HangoutLink,
		AnyoneCanAddSelf:        eventResponse.AnyoneCanAddSelf,
		GuestsCanInviteOthers:   eventResponse.GuestsCanInviteOthers != nil && *eventResponse.GuestsCanInviteOthers,
		GuestsCanModify:         eventResponse.GuestsCanModify,
		GuestsCanSeeOtherGuests: eventResponse.GuestsCanSeeOtherGuests != nil && *eventResponse.GuestsCanSeeOtherGuests,
		PrivateCopy:             eventResponse.PrivateCopy,
		Locked:                  eventResponse.Locked,
		EventType:               eventResponse.EventType,
	}

	// Normalize start time
	if eventResponse.Start != nil {
		normalizedStartTime := eventResponse.Start.DateTime
		if normalizedStartTime != "" && eventResponse.Start.TimeZone != "" {
			// Convert the datetime to match the specified timezone
			var err error
			normalizedStartTime, err = normalizeEventTime(eventResponse.Start.DateTime, eventResponse.Start.TimeZone)
			if err != nil {
				fmt.Printf("Warning: failed to normalize start time: %v\n", err)
				normalizedStartTime = eventResponse.Start.DateTime
			}
		}

		convertedEvent.Start = &EventTime{
			DateTime: normalizedStartTime,
			Date:     eventResponse.Start.Date,
			TimeZone: eventResponse.Start.TimeZone,
		}
	}

	// Normalize end time
	if eventResponse.End != nil {
		normalizedEndTime := eventResponse.End.DateTime
		if normalizedEndTime != "" && eventResponse.End.TimeZone != "" {
			// Convert the datetime to match the specified timezone
			var err error
			normalizedEndTime, err = normalizeEventTime(eventResponse.End.DateTime, eventResponse.End.TimeZone)
			if err != nil {
				fmt.Printf("Warning: failed to normalize end time: %v\n", err)
				normalizedEndTime = eventResponse.End.DateTime
			}
		}

		convertedEvent.End = &EventTime{
			DateTime: normalizedEndTime,
			Date:     eventResponse.End.Date,
			TimeZone: eventResponse.End.TimeZone,
		}
	}

	fmt.Println("Original Start DateTime:", eventResponse.Start.DateTime)
	fmt.Println("Original TimeZone:", eventResponse.Start.TimeZone)
	fmt.Println("Normalized Start DateTime:", convertedEvent.Start.DateTime)

	if len(eventResponse.Attendees) > 0 {
		for _, attendee := range eventResponse.Attendees {
			convertedEvent.Attendees = append(convertedEvent.Attendees, Attendee{
				Email:          attendee.Email,
				DisplayName:    attendee.DisplayName,
				ResponseStatus: attendee.ResponseStatus,
				Optional:       attendee.Optional,
			})
		}
	}

	if eventResponse.Creator != nil {
		convertedEvent.Creator = &Person{
			Email:       eventResponse.Creator.Email,
			DisplayName: eventResponse.Creator.DisplayName,
		}
	}

	if eventResponse.Organizer != nil {
		convertedEvent.Organizer = &Person{
			Email:       eventResponse.Organizer.Email,
			DisplayName: eventResponse.Organizer.DisplayName,
		}
	}

	if len(eventResponse.Recurrence) > 0 {
		convertedEvent.Recurrence = make([]string, len(eventResponse.Recurrence))
		copy(convertedEvent.Recurrence, eventResponse.Recurrence)
	}

	if eventResponse.OriginalStartTime != nil {
		normalizedOriginalTime := eventResponse.OriginalStartTime.DateTime
		if normalizedOriginalTime != "" && eventResponse.OriginalStartTime.TimeZone != "" {
			var err error
			normalizedOriginalTime, err = normalizeEventTime(eventResponse.OriginalStartTime.DateTime, eventResponse.OriginalStartTime.TimeZone)
			if err != nil {
				fmt.Printf("Warning: failed to normalize original start time: %v\n", err)
				normalizedOriginalTime = eventResponse.OriginalStartTime.DateTime
			}
		}

		convertedEvent.OriginalStartTime = &EventTime{
			DateTime: normalizedOriginalTime,
			Date:     eventResponse.OriginalStartTime.Date,
			TimeZone: eventResponse.OriginalStartTime.TimeZone,
		}
	}

	if eventResponse.ConferenceData != nil {
		convertedEvent.ConferenceData = &ConferenceData{
			ConferenceId: eventResponse.ConferenceData.ConferenceId,
			Signature:    eventResponse.ConferenceData.Signature,
			Notes:        eventResponse.ConferenceData.Notes,
		}

		if eventResponse.ConferenceData.CreateRequest != nil {
			convertedEvent.ConferenceData.CreateRequest = &CreateRequest{
				RequestId: eventResponse.ConferenceData.CreateRequest.RequestId,
			}
			if eventResponse.ConferenceData.CreateRequest.ConferenceSolutionKey != nil {
				convertedEvent.ConferenceData.CreateRequest.ConferenceSolutionKey = &ConferenceSolutionKey{
					Type: eventResponse.ConferenceData.CreateRequest.ConferenceSolutionKey.Type,
				}
			}
		}

		if len(eventResponse.ConferenceData.EntryPoints) > 0 {
			for _, ep := range eventResponse.ConferenceData.EntryPoints {
				convertedEvent.ConferenceData.EntryPoints = append(convertedEvent.ConferenceData.EntryPoints, EntryPoint{
					EntryPointType: ep.EntryPointType,
					Uri:            ep.Uri,
					Label:          ep.Label,
					Pin:            ep.Pin,
					AccessCode:     ep.AccessCode,
					MeetingCode:    ep.MeetingCode,
					Passcode:       ep.Passcode,
					Password:       ep.Password,
				})
			}
		}

		if eventResponse.ConferenceData.ConferenceSolution != nil {
			convertedEvent.ConferenceData.ConferenceSolution = &ConferenceSolution{
				Name:    eventResponse.ConferenceData.ConferenceSolution.Name,
				IconUri: eventResponse.ConferenceData.ConferenceSolution.IconUri,
			}
			if eventResponse.ConferenceData.ConferenceSolution.Key != nil {
				convertedEvent.ConferenceData.ConferenceSolution.Key = &ConferenceSolutionKey{
					Type: eventResponse.ConferenceData.ConferenceSolution.Key.Type,
				}
			}
		}
	}

	if len(eventResponse.Attachments) > 0 {
		for _, att := range eventResponse.Attachments {
			convertedEvent.Attachments = append(convertedEvent.Attachments, CalendarAttachment{
				FileId:   att.FileId,
				FileUrl:  att.FileUrl,
				Title:    att.Title,
				MimeType: att.MimeType,
				IconLink: att.IconLink,
			})
		}
	}

	if eventResponse.Source != nil {
		convertedEvent.Source = &EventSource{
			Title: eventResponse.Source.Title,
			Url:   eventResponse.Source.Url,
		}
	}

	formattedResponse := formatSingleEventResponse(convertedEvent, calendarID)

	return types.ActionResult{
		Result: formattedResponse,
	}, nil
}

func formatSingleEventResponse(event Event, calendarID string) string {
	var response strings.Builder

	// Header
	response.WriteString(fmt.Sprintf("Event from calendar '%s':\n\n", calendarID))
	response.WriteString(formatEventDetails(event, 0))

	return response.String()
}

func (a *GoogleCalendarGetEventAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-get-event",
		Description: "Retrieve a specific event from a Google Calendar by its event ID. Returns detailed information about the event including summary, time, location, attendees, and more.",
		Properties: map[string]jsonschema.Definition{
			"calendarId": {
				Type:        jsonschema.String,
				Description: "ID of the calendar (use 'primary' for the main calendar)",
			},
			"eventId": {
				Type:        jsonschema.String,
				Description: "ID of the event to retrieve",
			},
			"fields": {
				Type:        jsonschema.Array,
				Description: "Optional array of additional event fields to retrieve. Available fields are strictly validated. Default fields (id, summary, start, end, status, htmlLink, location, attendees) are always included.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
					Enum: []string{
						"id", "summary", "description", "start", "end", "location",
						"attendees", "colorId", "transparency", "extendedProperties",
						"reminders", "conferenceData", "attachments", "status",
						"htmlLink", "created", "updated", "creator", "organizer",
						"recurrence", "recurringEventId", "originalStartTime",
						"visibility", "iCalUID", "sequence", "hangoutLink",
						"anyoneCanAddSelf", "guestsCanInviteOthers", "guestsCanModify",
						"guestsCanSeeOtherGuests", "privateCopy", "locked", "source",
						"eventType",
					},
				},
			},
		},
		Required: []string{"calendarId", "eventId"},
	}
}

func (a *GoogleCalendarGetEventAction) Plannable() bool {
	return true
}

func GoogleCalendarGetEventConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
