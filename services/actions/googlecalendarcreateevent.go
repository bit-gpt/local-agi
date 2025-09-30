package actions

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	gcutils "github.com/mudler/LocalAGI/pkg/utils/googlecalendar"
	"github.com/sashabaranov/go-openai/jsonschema"
	"google.golang.org/api/calendar/v3"
)

func NewGoogleCalendarCreateEvent(config map[string]string) *GoogleCalendarCreateEventAction {
	return &GoogleCalendarCreateEventAction{}
}

type GoogleCalendarCreateEventAction struct{}

func validateEventID(eventID string) error {
	if eventID == "" {
		return nil
	}

	if len(eventID) < 5 || len(eventID) > 1024 {
		return fmt.Errorf("eventId must be between 5 and 1024 characters")
	}

	// Must be base32hex encoding: lowercase letters a-v and digits 0-9
	validPattern := regexp.MustCompile(`^[a-v0-9]+$`)
	if !validPattern.MatchString(eventID) {
		return fmt.Errorf("eventId must only contain lowercase letters a-v and digits 0-9")
	}

	return nil
}

func (a *GoogleCalendarCreateEventAction) detectDuplicates(
	ctx context.Context,
	calendarService *calendar.Service,
	newEvent *calendar.Event,
	calendarsToCheck []string,
	threshold float64,
) ([]Event, float64, error) {
	if len(calendarsToCheck) == 0 {
		return nil, 0, nil
	}

	var duplicates []Event
	maxSimilarity := 0.0

	for _, calID := range calendarsToCheck {
		eventsCall := calendarService.Events.List(calID).Context(ctx)

		if newEvent.Start != nil && newEvent.Start.DateTime != "" {
			eventsCall = eventsCall.TimeMin(newEvent.Start.DateTime)
		}
		if newEvent.End != nil && newEvent.End.DateTime != "" {
			eventsCall = eventsCall.TimeMax(newEvent.End.DateTime)
		}

		eventsResponse, err := eventsCall.Do()
		if err != nil {
			continue
		}

		for _, existingEvent := range eventsResponse.Items {
			similarity := gcutils.CalculateEventSimilarity(newEvent, existingEvent)
			if similarity >= threshold {
				converted := Event{
					ID:       existingEvent.Id,
					Summary:  existingEvent.Summary,
					Location: existingEvent.Location,
					Status:   existingEvent.Status,
				}
				if existingEvent.Start != nil {
					converted.Start = &EventTime{
						DateTime: existingEvent.Start.DateTime,
						Date:     existingEvent.Start.Date,
					}
				}
				duplicates = append(duplicates, converted)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
		}
	}

	return duplicates, maxSimilarity, nil
}

func (a *GoogleCalendarCreateEventAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	calendarService, err := oauth.GetCalendarClient(sharedState.UserID)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get Calendar client: %v", err)
	}

	// required parameters
	calendarID, ok := params["calendarId"].(string)
	if !ok || calendarID == "" {
		calendarID = "primary"
	}

	summary, ok := params["summary"].(string)
	if !ok || summary == "" {
		return types.ActionResult{}, fmt.Errorf("summary is required")
	}

	start, ok := params["start"].(string)
	if !ok || start == "" {
		return types.ActionResult{}, fmt.Errorf("start time is required")
	}

	end, ok := params["end"].(string)
	if !ok || end == "" {
		return types.ActionResult{}, fmt.Errorf("end time is required")
	}

	// optional parameters
	eventID, _ := params["eventId"].(string)
	description, _ := params["description"].(string)
	location, _ := params["location"].(string)
	timeZone, _ := params["timeZone"].(string)
	colorId, _ := params["colorId"].(string)
	transparency, _ := params["transparency"].(string)
	visibility, _ := params["visibility"].(string)
	sendUpdates, _ := params["sendUpdates"].(string)

	if err := validateEventID(eventID); err != nil {
		return types.ActionResult{}, err
	}

	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Location:    location,
		ColorId:     colorId,
	}

	if eventID != "" {
		event.Id = eventID
	}

	isAllDay := !strings.Contains(start, "T")

	if isAllDay {
		event.Start = &calendar.EventDateTime{Date: start}
		event.End = &calendar.EventDateTime{Date: end}
	} else {
		if timeZone == "" {
			calendarTZ, err := getCalendarTimezone(ctx, calendarService, calendarID)
			if err == nil && calendarTZ != "" {
				timeZone = calendarTZ
			} else {
				timeZone = "UTC"
			}
		}

		formattedStart, err := formatTimeForAPI(start, timeZone)
		if err != nil {
			return types.ActionResult{}, fmt.Errorf("invalid start time: %v", err)
		}
		formattedEnd, err := formatTimeForAPI(end, timeZone)
		if err != nil {
			return types.ActionResult{}, fmt.Errorf("invalid end time: %v", err)
		}

		event.Start = &calendar.EventDateTime{
			DateTime: formattedStart,
			TimeZone: timeZone,
		}
		event.End = &calendar.EventDateTime{
			DateTime: formattedEnd,
			TimeZone: timeZone,
		}
	}

	if attendeesParam, ok := params["attendees"].([]interface{}); ok {
		for _, attendeeData := range attendeesParam {
			if attendeeMap, ok := attendeeData.(map[string]interface{}); ok {
				attendee := &calendar.EventAttendee{}

				if email, ok := attendeeMap["email"].(string); ok {
					attendee.Email = email
				}
				if displayName, ok := attendeeMap["displayName"].(string); ok {
					attendee.DisplayName = displayName
				}
				if optional, ok := attendeeMap["optional"].(bool); ok {
					attendee.Optional = optional
				}
				if responseStatus, ok := attendeeMap["responseStatus"].(string); ok {
					attendee.ResponseStatus = responseStatus
				}
				if comment, ok := attendeeMap["comment"].(string); ok {
					attendee.Comment = comment
				}
				if additionalGuests, ok := attendeeMap["additionalGuests"].(float64); ok {
					attendee.AdditionalGuests = int64(additionalGuests)
				}

				event.Attendees = append(event.Attendees, attendee)
			}
		}
	}

	if remindersParam, ok := params["reminders"].(map[string]interface{}); ok {
		event.Reminders = &calendar.EventReminders{}

		if useDefault, ok := remindersParam["useDefault"].(bool); ok {
			event.Reminders.UseDefault = useDefault
			// Force send UseDefault field when it's false to distinguish from not set
			if !useDefault {
				event.Reminders.ForceSendFields = []string{"UseDefault"}
			}
		}

		// Only process overrides if useDefault is explicitly false
		if !event.Reminders.UseDefault {
			if overrides, ok := remindersParam["overrides"].([]interface{}); ok {
				for _, override := range overrides {
					if overrideMap, ok := override.(map[string]interface{}); ok {
						reminder := &calendar.EventReminder{}

						if method, ok := overrideMap["method"].(string); ok {
							reminder.Method = method
						} else {
							reminder.Method = "popup"
						}

						if minutes, ok := overrideMap["minutes"].(float64); ok {
							reminder.Minutes = int64(minutes)
						}

						event.Reminders.Overrides = append(event.Reminders.Overrides, reminder)
					}
				}
			}
		}
	}

	if recurrenceParam, ok := params["recurrence"].([]interface{}); ok {
		for _, rule := range recurrenceParam {
			if ruleStr, ok := rule.(string); ok {
				event.Recurrence = append(event.Recurrence, ruleStr)
			}
		}
	}

	if transparency != "" {
		event.Transparency = transparency
	}

	if visibility != "" {
		event.Visibility = visibility
	}

	if guestsCanInviteOthers, ok := params["guestsCanInviteOthers"].(bool); ok {
		event.GuestsCanInviteOthers = &guestsCanInviteOthers
	}
	if guestsCanModify, ok := params["guestsCanModify"].(bool); ok {
		event.GuestsCanModify = guestsCanModify
	}
	if guestsCanSeeOtherGuests, ok := params["guestsCanSeeOtherGuests"].(bool); ok {
		event.GuestsCanSeeOtherGuests = &guestsCanSeeOtherGuests
	}
	if anyoneCanAddSelf, ok := params["anyoneCanAddSelf"].(bool); ok {
		event.AnyoneCanAddSelf = anyoneCanAddSelf
	}

	if conferenceDataParam, ok := params["conferenceData"].(map[string]interface{}); ok {
		if createRequest, ok := conferenceDataParam["createRequest"].(map[string]interface{}); ok {
			event.ConferenceData = &calendar.ConferenceData{
				CreateRequest: &calendar.CreateConferenceRequest{},
			}

			if requestId, ok := createRequest["requestId"].(string); ok {
				event.ConferenceData.CreateRequest.RequestId = requestId
			}

			if solutionKey, ok := createRequest["conferenceSolutionKey"].(map[string]interface{}); ok {
				if keyType, ok := solutionKey["type"].(string); ok {
					event.ConferenceData.CreateRequest.ConferenceSolutionKey = &calendar.ConferenceSolutionKey{
						Type: keyType,
					}
				}
			}
		}
	}

	if extendedPropsParam, ok := params["extendedProperties"].(map[string]interface{}); ok {
		event.ExtendedProperties = &calendar.EventExtendedProperties{}

		if private, ok := extendedPropsParam["private"].(map[string]interface{}); ok {
			event.ExtendedProperties.Private = make(map[string]string)
			for k, v := range private {
				if strVal, ok := v.(string); ok {
					event.ExtendedProperties.Private[k] = strVal
				}
			}
		}

		if shared, ok := extendedPropsParam["shared"].(map[string]interface{}); ok {
			event.ExtendedProperties.Shared = make(map[string]string)
			for k, v := range shared {
				if strVal, ok := v.(string); ok {
					event.ExtendedProperties.Shared[k] = strVal
				}
			}
		}
	}

	if attachmentsParam, ok := params["attachments"].([]interface{}); ok {
		for _, attachmentData := range attachmentsParam {
			if attachmentMap, ok := attachmentData.(map[string]interface{}); ok {
				attachment := &calendar.EventAttachment{}

				if fileUrl, ok := attachmentMap["fileUrl"].(string); ok {
					attachment.FileUrl = fileUrl
				}
				if title, ok := attachmentMap["title"].(string); ok {
					attachment.Title = title
				}
				if mimeType, ok := attachmentMap["mimeType"].(string); ok {
					attachment.MimeType = mimeType
				}
				if iconLink, ok := attachmentMap["iconLink"].(string); ok {
					attachment.IconLink = iconLink
				}
				if fileId, ok := attachmentMap["fileId"].(string); ok {
					attachment.FileId = fileId
				}

				event.Attachments = append(event.Attachments, attachment)
			}
		}
	}

	if sourceParam, ok := params["source"].(map[string]interface{}); ok {
		event.Source = &calendar.EventSource{}

		if url, ok := sourceParam["url"].(string); ok {
			event.Source.Url = url
		}
		if title, ok := sourceParam["title"].(string); ok {
			event.Source.Title = title
		}
	}

	var calendarsToCheck []string
	if calendarsParam, ok := params["calendarsToCheck"].([]interface{}); ok {
		for _, cal := range calendarsParam {
			if calStr, ok := cal.(string); ok {
				calendarsToCheck = append(calendarsToCheck, calStr)
			}
		}
	} else {
		calendarsToCheck = []string{calendarID}
	}

	threshold := 0.7
	if thresholdParam, ok := params["duplicateSimilarityThreshold"].(float64); ok {
		threshold = thresholdParam
	}

	allowDuplicates := false
	if allowParam, ok := params["allowDuplicates"].(bool); ok {
		allowDuplicates = allowParam
	}

	duplicates, maxSimilarity, err := a.detectDuplicates(ctx, calendarService, event, calendarsToCheck, threshold)
	if err == nil && len(duplicates) > 0 {
		if maxSimilarity >= 0.95 && !allowDuplicates {
			return types.ActionResult{}, fmt.Errorf("exact duplicate detected (similarity: %.2f). Similar events: %s. Use allowDuplicates=true to create anyway",
				maxSimilarity, formatDuplicatesList(duplicates))
		}

		if maxSimilarity >= threshold {
			// Warning about potential duplicates but continue
			fmt.Printf("Warning: potential duplicate events detected (similarity: %.2f): %s\n",
				maxSimilarity, formatDuplicatesList(duplicates))
		}
	}

	insertCall := calendarService.Events.Insert(calendarID, event).Context(ctx)

	if sendUpdates != "" {
		insertCall = insertCall.SendUpdates(sendUpdates)
	}

	if event.ConferenceData != nil {
		insertCall = insertCall.ConferenceDataVersion(1)
	}

	if len(event.Attachments) > 0 {
		insertCall = insertCall.SupportsAttachments(true)
	}

	createdEvent, err := insertCall.Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to create event: %v", err)
	}

	response := formatCreateEventResponse(createdEvent, calendarID)

	return types.ActionResult{
		Result: response,
	}, nil
}

func formatDuplicatesList(duplicates []Event) string {
	var list []string
	for _, dup := range duplicates {
		list = append(list, fmt.Sprintf("%s (ID: %s)", dup.Summary, dup.ID))
	}
	return strings.Join(list, ", ")
}

func formatCreateEventResponse(event *calendar.Event, calendarID string) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("✓ Event created successfully in calendar '%s'\n\n", calendarID))
	response.WriteString(fmt.Sprintf("Title: %s\n", event.Summary))
	response.WriteString(fmt.Sprintf("Event ID: %s\n", event.Id))
	response.WriteString(fmt.Sprintf("Status: %s\n", event.Status))

	if event.Start != nil {
		if event.Start.DateTime != "" {
			response.WriteString(fmt.Sprintf("Start: %s", event.Start.DateTime))
			if event.Start.TimeZone != "" {
				response.WriteString(fmt.Sprintf(" (%s)", event.Start.TimeZone))
			}
			response.WriteString("\n")
		} else if event.Start.Date != "" {
			response.WriteString(fmt.Sprintf("Start Date: %s (All-day)\n", event.Start.Date))
		}
	}

	if event.End != nil {
		if event.End.DateTime != "" {
			response.WriteString(fmt.Sprintf("End: %s", event.End.DateTime))
			if event.End.TimeZone != "" {
				response.WriteString(fmt.Sprintf(" (%s)", event.End.TimeZone))
			}
			response.WriteString("\n")
		} else if event.End.Date != "" {
			response.WriteString(fmt.Sprintf("End Date: %s (All-day)\n", event.End.Date))
		}
	}

	if event.Location != "" {
		response.WriteString(fmt.Sprintf("Location: %s\n", event.Location))
	}

	if event.Description != "" {
		description := event.Description
		if len(description) > 200 {
			description = description[:200] + "..."
		}
		response.WriteString(fmt.Sprintf("Description: %s\n", description))
	}

	if len(event.Attendees) > 0 {
		response.WriteString(fmt.Sprintf("Attendees: %d invited\n", len(event.Attendees)))
	}

	if event.ConferenceData != nil && len(event.ConferenceData.EntryPoints) > 0 {
		for _, entryPoint := range event.ConferenceData.EntryPoints {
			if entryPoint.Uri != "" {
				response.WriteString(fmt.Sprintf("Conference Link: %s\n", entryPoint.Uri))
				break
			}
		}
	}

	if len(event.Recurrence) > 0 {
		response.WriteString(fmt.Sprintf("Recurrence: %s\n", strings.Join(event.Recurrence, "; ")))
	}

	if event.HtmlLink != "" {
		response.WriteString(fmt.Sprintf("\nView event: %s\n", event.HtmlLink))
	}

	return response.String()
}

func getCalendarTimezone(ctx context.Context, calendarService *calendar.Service, calendarID string) (string, error) {
	cal, err := calendarService.Calendars.Get(calendarID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to get calendar: %v", err)
	}
	return cal.TimeZone, nil
}

func (a *GoogleCalendarCreateEventAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-create-event",
		Description: "Create a new event in Google Calendar with full control over all event properties including attendees, reminders, recurrence, and conference settings.",
		Properties: map[string]jsonschema.Definition{
			"calendarId": {
				Type:        jsonschema.String,
				Description: "ID of the calendar (use 'primary' for the main calendar)",
			},
			"eventId": {
				Type:        jsonschema.String,
				Description: "Optional custom event ID (5-1024 characters, base32hex encoding: lowercase letters a-v and digits 0-9 only). If not provided, Google Calendar will generate one.",
			},
			"summary": {
				Type:        jsonschema.String,
				Description: "Title of the event",
			},
			"description": {
				Type:        jsonschema.String,
				Description: "Description/notes for the event",
			},
			"start": {
				Type:        jsonschema.String,
				Description: "Event start time: '2025-01-01T10:00:00' for timed events or '2025-01-01' for all-day events",
			},
			"end": {
				Type:        jsonschema.String,
				Description: "Event end time: '2025-01-01T11:00:00' for timed events or '2025-01-02' for all-day events (exclusive)",
			},
			"timeZone": {
				Type:        jsonschema.String,
				Description: "Timezone as IANA Time Zone Database name (e.g., America/Los_Angeles). Takes priority over calendar's default timezone. Only used for timezone-naive datetime strings.",
			},
			"location": {
				Type:        jsonschema.String,
				Description: "Location of the event",
			},
			"attendees": {
				Type:        jsonschema.Array,
				Description: "List of event attendees with their details",
				Items: &jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"email": {
							Type:        jsonschema.String,
							Description: "Email address of the attendee",
						},
						"displayName": {
							Type:        jsonschema.String,
							Description: "Display name of the attendee",
						},
						"optional": {
							Type:        jsonschema.Boolean,
							Description: "Whether this is an optional attendee",
						},
						"responseStatus": {
							Type:        jsonschema.String,
							Enum:        []string{"needsAction", "declined", "tentative", "accepted"},
							Description: "Attendee's response status",
						},
						"comment": {
							Type:        jsonschema.String,
							Description: "Attendee's response comment",
						},
						"additionalGuests": {
							Type:        jsonschema.Integer,
							Description: "Number of additional guests the attendee is bringing",
						},
					},
					Required: []string{"email"},
				},
			},
			"colorId": {
				Type:        jsonschema.String,
				Description: "Color ID for the event (use list-colors to see available IDs)",
			},
			"reminders": {
				Type:        jsonschema.Object,
				Description: "Reminder settings for the event",
				Properties: map[string]jsonschema.Definition{
					"useDefault": {
						Type:        jsonschema.Boolean,
						Description: "Whether to use the default reminders",
					},
					"overrides": {
						Type:        jsonschema.Array,
						Description: "Custom reminders",
						Items: &jsonschema.Definition{
							Type: jsonschema.Object,
							Properties: map[string]jsonschema.Definition{
								"method": {
									Type:        jsonschema.String,
									Enum:        []string{"email", "popup"},
									Description: "Reminder method",
								},
								"minutes": {
									Type:        jsonschema.Integer,
									Description: "Minutes before the event to trigger the reminder",
								},
							},
							Required: []string{"minutes"},
						},
					},
				},
				Required: []string{"useDefault"},
			},
			"recurrence": {
				Type:        jsonschema.Array,
				Description: "Recurrence rules in RFC5545 format (e.g., [\"RRULE:FREQ=WEEKLY;COUNT=5\"])",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"transparency": {
				Type:        jsonschema.String,
				Enum:        []string{"opaque", "transparent"},
				Description: "Whether the event blocks time on the calendar. 'opaque' means busy, 'transparent' means free.",
			},
			"visibility": {
				Type:        jsonschema.String,
				Enum:        []string{"default", "public", "private", "confidential"},
				Description: "Visibility of the event. Use 'public' for public events, 'private' for private events visible to attendees.",
			},
			"guestsCanInviteOthers": {
				Type:        jsonschema.Boolean,
				Description: "Whether attendees can invite others to the event. Default is true.",
			},
			"guestsCanModify": {
				Type:        jsonschema.Boolean,
				Description: "Whether attendees can modify the event. Default is false.",
			},
			"guestsCanSeeOtherGuests": {
				Type:        jsonschema.Boolean,
				Description: "Whether attendees can see the list of other attendees. Default is true.",
			},
			"anyoneCanAddSelf": {
				Type:        jsonschema.Boolean,
				Description: "Whether anyone can add themselves to the event. Default is false.",
			},
			"sendUpdates": {
				Type:        jsonschema.String,
				Enum:        []string{"all", "externalOnly", "none"},
				Description: "Whether to send notifications about the event creation. 'all' sends to all guests, 'externalOnly' to non-Google Calendar users only, 'none' sends no notifications.",
			},
			"conferenceData": {
				Type:        jsonschema.Object,
				Description: "Conference properties for the event. Use createRequest to add a new conference.",
				Properties: map[string]jsonschema.Definition{
					"createRequest": {
						Type:        jsonschema.Object,
						Description: "Request to generate a new conference",
						Properties: map[string]jsonschema.Definition{
							"requestId": {
								Type:        jsonschema.String,
								Description: "Client-generated unique ID for this request to ensure idempotency",
							},
							"conferenceSolutionKey": {
								Type:        jsonschema.Object,
								Description: "Conference solution to create",
								Properties: map[string]jsonschema.Definition{
									"type": {
										Type:        jsonschema.String,
										Enum:        []string{"hangoutsMeet", "eventHangout", "eventNamedHangout", "addOn"},
										Description: "Conference solution type",
									},
								},
								Required: []string{"type"},
							},
						},
						Required: []string{"requestId", "conferenceSolutionKey"},
					},
				},
			},
			"extendedProperties": {
				Type:        jsonschema.Object,
				Description: "Extended properties for storing application-specific data. Max 300 properties totaling 32KB.",
				Properties: map[string]jsonschema.Definition{
					"private": {
						Type:        jsonschema.Object,
						Description: "Properties private to the application. Keys can have max 44 chars, values max 1024 chars.",
					},
					"shared": {
						Type:        jsonschema.Object,
						Description: "Properties visible to all attendees. Keys can have max 44 chars, values max 1024 chars.",
					},
				},
			},
			"attachments": {
				Type:        jsonschema.Array,
				Description: "File attachments for the event. Requires calendar to support attachments.",
				Items: &jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"fileUrl": {
							Type:        jsonschema.String,
							Description: "URL of the attached file",
						},
						"title": {
							Type:        jsonschema.String,
							Description: "Title of the attachment",
						},
						"mimeType": {
							Type:        jsonschema.String,
							Description: "MIME type of the attachment",
						},
						"iconLink": {
							Type:        jsonschema.String,
							Description: "URL of the icon for the attachment",
						},
						"fileId": {
							Type:        jsonschema.String,
							Description: "ID of the attached file in Google Drive",
						},
					},
					Required: []string{"fileUrl"},
				},
			},
			"source": {
				Type:        jsonschema.Object,
				Description: "Source of the event, such as a web page or email message.",
				Properties: map[string]jsonschema.Definition{
					"url": {
						Type:        jsonschema.String,
						Description: "URL of the source",
					},
					"title": {
						Type:        jsonschema.String,
						Description: "Title of the source",
					},
				},
				Required: []string{"url", "title"},
			},
			"calendarsToCheck": {
				Type:        jsonschema.Array,
				Description: "List of calendar IDs to check for conflicts (defaults to just the target calendar)",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"duplicateSimilarityThreshold": {
				Type:        jsonschema.Number,
				Description: "Threshold for duplicate detection (0-1, default: 0.7). Events with similarity above this are flagged as potential duplicates",
			},
			"allowDuplicates": {
				Type:        jsonschema.Boolean,
				Description: "If true, allows creation even when exact duplicates are detected (similarity >= 0.95). Default is false which blocks duplicate creation",
			},
		},
		Required: []string{"calendarId", "summary", "start", "end"},
	}
}

func (a *GoogleCalendarCreateEventAction) Plannable() bool {
	return true
}

func GoogleCalendarCreateEventConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
