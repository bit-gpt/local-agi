package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	gcutils "github.com/mudler/LocalAGI/pkg/utils/googlecalendar"
	"github.com/sashabaranov/go-openai/jsonschema"
	"google.golang.org/api/calendar/v3"
)

func NewGoogleCalendarUpdateEvent(config map[string]string) *GoogleCalendarUpdateEventAction {
	return &GoogleCalendarUpdateEventAction{}
}

type GoogleCalendarUpdateEventAction struct{}

func (a *GoogleCalendarUpdateEventAction) checkConflicts(
	ctx context.Context,
	calendarService *calendar.Service,
	updatedEvent *calendar.Event,
	calendarsToCheck []string,
	excludeEventID string,
) ([]Event, error) {
	if len(calendarsToCheck) == 0 {
		return nil, nil
	}

	var conflicts []Event

	for _, calID := range calendarsToCheck {
		eventsCall := calendarService.Events.List(calID).Context(ctx)

		if updatedEvent.Start != nil && updatedEvent.Start.DateTime != "" {
			eventsCall = eventsCall.TimeMin(updatedEvent.Start.DateTime)
		}
		if updatedEvent.End != nil && updatedEvent.End.DateTime != "" {
			eventsCall = eventsCall.TimeMax(updatedEvent.End.DateTime)
		}

		eventsResponse, err := eventsCall.Do()
		if err != nil {
			continue
		}

		for _, existingEvent := range eventsResponse.Items {
			if existingEvent.Id == excludeEventID {
				continue
			}

			if gcutils.EventsOverlap(updatedEvent, existingEvent) {
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
				conflicts = append(conflicts, converted)
			}
		}
	}

	return conflicts, nil
}

func (a *GoogleCalendarUpdateEventAction) applyParamsToEvent(
	ctx context.Context,
	event *calendar.Event,
	params types.ActionParams,
	calendarService *calendar.Service,
	calendarID string,
) error {
	if summary, ok := params["summary"].(string); ok {
		event.Summary = summary
	}
	if description, ok := params["description"].(string); ok {
		event.Description = description
	}
	if location, ok := params["location"].(string); ok {
		event.Location = location
	}
	if colorId, ok := params["colorId"].(string); ok {
		event.ColorId = colorId
	}
	if transparency, ok := params["transparency"].(string); ok {
		event.Transparency = transparency
	}
	if visibility, ok := params["visibility"].(string); ok {
		event.Visibility = visibility
	}

	timeZone, _ := params["timeZone"].(string)
	if timeZone == "" {
		calendarTZ, err := getCalendarTimezone(ctx, calendarService, calendarID)
		if err == nil && calendarTZ != "" {
			timeZone = calendarTZ
		} else {
			timeZone = "UTC"
		}
	}

	if start, ok := params["start"].(string); ok {
		isAllDay := !strings.Contains(start, "T")
		if isAllDay {
			event.Start = &calendar.EventDateTime{Date: start}
		} else {
			fmt.Println("Start:", start)
			fmt.Println("Timezone:", timeZone)
			formattedStart, err := formatTimeForAPI(start, timeZone)
			if err != nil {
				return fmt.Errorf("invalid start time: %v", err)
			}
			event.Start = &calendar.EventDateTime{
				DateTime: formattedStart,
				TimeZone: timeZone,
			}
		}
	}

	if end, ok := params["end"].(string); ok {
		isAllDay := !strings.Contains(end, "T")
		if isAllDay {
			event.End = &calendar.EventDateTime{Date: end}
		} else {
			formattedEnd, err := formatTimeForAPI(end, timeZone)
			if err != nil {
				return fmt.Errorf("invalid end time: %v", err)
			}
			event.End = &calendar.EventDateTime{
				DateTime: formattedEnd,
				TimeZone: timeZone,
			}
		}
	}

	if attendeesParam, ok := params["attendees"].([]interface{}); ok {
		event.Attendees = nil
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
			if !useDefault {
				event.Reminders.ForceSendFields = []string{"UseDefault"}
			}
		}
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
		event.Recurrence = nil
		for _, rule := range recurrenceParam {
			if ruleStr, ok := rule.(string); ok {
				event.Recurrence = append(event.Recurrence, ruleStr)
			}
		}
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
		event.Attachments = nil
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

	return nil
}

func (a *GoogleCalendarUpdateEventAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
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

	existingEvent, err := calendarService.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to get existing event: %v", err)
	}

	modificationScope, _ := params["modificationScope"].(string)
	originalStartTime, _ := params["originalStartTime"].(string)
	futureStartDate, _ := params["futureStartDate"].(string)

	if modificationScope == "thisEventOnly" && originalStartTime == "" {
		// Default to the existing event's start time
		if existingEvent.Start != nil {
			if existingEvent.Start.DateTime != "" {
				originalStartTime = existingEvent.Start.DateTime
			} else if existingEvent.Start.Date != "" {
				originalStartTime = existingEvent.Start.Date
			}
		}
		// If still empty, return error
		if originalStartTime == "" {
			return types.ActionResult{}, fmt.Errorf("originalStartTime could not be determined from existing event")
		}
	}

	if modificationScope == "thisAndFollowing" && futureStartDate == "" {
		// Default to the existing event's start time
		if existingEvent.Start != nil {
			if existingEvent.Start.DateTime != "" {
				futureStartDate = existingEvent.Start.DateTime
			} else if existingEvent.Start.Date != "" {
				futureStartDate = existingEvent.Start.Date
			}
		}
		// If still empty, return error
		if futureStartDate == "" {
			return types.ActionResult{}, fmt.Errorf("futureStartDate could not be determined from existing event")
		}
	}

	err = a.applyParamsToEvent(ctx, existingEvent, params, calendarService, calendarID)
	if err != nil {
		return types.ActionResult{}, err
	}

	timeChanged := false
	if _, ok := params["start"].(string); ok {
		timeChanged = true
	}
	if _, ok := params["end"].(string); ok {
		timeChanged = true
	}

	checkConflicts := timeChanged
	if checkConflictsParam, ok := params["checkConflicts"].(bool); ok {
		checkConflicts = checkConflictsParam
	}

	if checkConflicts {
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

		conflicts, err := a.checkConflicts(ctx, calendarService, existingEvent, calendarsToCheck, eventID)
		if err == nil && len(conflicts) > 0 {
			fmt.Printf("Warning: potential conflicts detected: %s\n", formatDuplicatesList(conflicts))
		}
	}

	updateCall := calendarService.Events.Update(calendarID, eventID, existingEvent).Context(ctx)

	sendUpdates, _ := params["sendUpdates"].(string)
	if sendUpdates != "" {
		updateCall = updateCall.SendUpdates(sendUpdates)
	}

	if existingEvent.ConferenceData != nil && existingEvent.ConferenceData.CreateRequest != nil {
		updateCall = updateCall.ConferenceDataVersion(1)
	}

	if len(existingEvent.Attachments) > 0 {
		updateCall = updateCall.SupportsAttachments(true)
	}

	var updatedEvent *calendar.Event
	switch modificationScope {
	case "thisEventOnly":
		if originalStartTime != "" {
			timeZone, _ := params["timeZone"].(string)
			if timeZone == "" {
				calendarTZ, err := getCalendarTimezone(ctx, calendarService, calendarID)
				if err == nil && calendarTZ != "" {
					timeZone = calendarTZ
				} else {
					timeZone = "UTC"
				}
			}

			formattedOriginalStart, err := formatTimeForAPI(originalStartTime, timeZone)
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("invalid originalStartTime: %v", err)
			}

			instances, err := calendarService.Events.Instances(calendarID, eventID).Context(ctx).Do()
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("failed to get instances: %v", err)
			}

			originalTime, err := time.Parse(time.RFC3339, formattedOriginalStart)
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("failed to parse originalStartTime: %v", err)
			}

			var targetInstance *calendar.Event
			for _, instance := range instances.Items {
				if instance.Start != nil && instance.Start.DateTime != "" {
					instanceTime, err := time.Parse(time.RFC3339, instance.Start.DateTime)
					if err != nil {
						continue
					}
					if instanceTime.Equal(originalTime) {
						targetInstance = instance
						break
					}
				}
			}

			if targetInstance == nil {
				return types.ActionResult{}, fmt.Errorf("instance with originalStartTime not found")
			}

			err = a.applyParamsToEvent(ctx, targetInstance, params, calendarService, calendarID)
			if err != nil {
				return types.ActionResult{}, err
			}

			instanceUpdateCall := calendarService.Events.Update(calendarID, targetInstance.Id, targetInstance).Context(ctx)

			if sendUpdates != "" {
				instanceUpdateCall = instanceUpdateCall.SendUpdates(sendUpdates)
			}

			if targetInstance.ConferenceData != nil && targetInstance.ConferenceData.CreateRequest != nil {
				instanceUpdateCall = instanceUpdateCall.ConferenceDataVersion(1)
			}

			if len(targetInstance.Attachments) > 0 {
				instanceUpdateCall = instanceUpdateCall.SupportsAttachments(true)
			}

			updatedEvent, err = instanceUpdateCall.Do()
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("failed to update instance: %v", err)
			}
		}
	case "thisAndFollowing":
		if futureStartDate != "" {
			// Get timezone for parsing futureStartDate
			timeZone, _ := params["timeZone"].(string)
			if timeZone == "" {
				calendarTZ, err := getCalendarTimezone(ctx, calendarService, calendarID)
				if err == nil && calendarTZ != "" {
					timeZone = calendarTZ
				} else {
					timeZone = "UTC"
				}
			}

			formattedFutureStart, err := formatTimeForAPI(futureStartDate, timeZone)
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("invalid futureStartDate: %v", err)
			}

			futureTime, err := time.Parse(time.RFC3339, formattedFutureStart)
			if err != nil {
				return types.ActionResult{}, fmt.Errorf("invalid futureStartDate: %v", err)
			}
			if !futureTime.After(time.Now()) {
				return types.ActionResult{}, fmt.Errorf("futureStartDate must be in the future")
			}
		}
		updatedEvent, err = updateCall.Do()
		if err != nil {
			return types.ActionResult{}, fmt.Errorf("failed to update event: %v", err)
		}
	default:
		updatedEvent, err = updateCall.Do()
		if err != nil {
			return types.ActionResult{}, fmt.Errorf("failed to update event: %v", err)
		}
	}

	response := formatUpdateEventResponse(updatedEvent, calendarID)

	return types.ActionResult{
		Result: response,
	}, nil
}

func formatUpdateEventResponse(event *calendar.Event, calendarID string) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("✓ Event updated successfully in calendar '%s'\n\n", calendarID))
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
		response.WriteString(fmt.Sprintf("Attendees: %d\n", len(event.Attendees)))
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

func (a *GoogleCalendarUpdateEventAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-update-event",
		Description: "Update an existing event in Google Calendar. All fields are optional except calendarId and eventId.",
		Properties: map[string]jsonschema.Definition{
			"calendarId": {
				Type:        jsonschema.String,
				Description: "ID of the calendar (use 'primary' for the main calendar)",
			},
			"eventId": {
				Type:        jsonschema.String,
				Description: "ID of the event to update",
			},
			"summary": {
				Type:        jsonschema.String,
				Description: "Updated title of the event",
			},
			"description": {
				Type:        jsonschema.String,
				Description: "Updated description/notes",
			},
			"start": {
				Type:        jsonschema.String,
				Description: "Updated start time: '2024-01-01T10:00:00' for timed events or '2024-01-01' for all-day events",
			},
			"end": {
				Type:        jsonschema.String,
				Description: "Updated end time: '2024-01-01T11:00:00' for timed events or '2024-01-02' for all-day events (exclusive)",
			},
			"timeZone": {
				Type:        jsonschema.String,
				Description: "Updated timezone as IANA Time Zone Database name. If not provided, uses the calendar's default timezone.",
			},
			"location": {
				Type:        jsonschema.String,
				Description: "Updated location",
			},
			"attendees": {
				Type:        jsonschema.Array,
				Description: "Updated attendee list",
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
							Description: "Number of additional guests",
						},
					},
					Required: []string{"email"},
				},
			},
			"colorId": {
				Type:        jsonschema.String,
				Description: "Updated color ID",
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
									Description: "Minutes before the event",
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
				Description: "Updated recurrence rules in RFC5545 format",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"sendUpdates": {
				Type:        jsonschema.String,
				Enum:        []string{"all", "externalOnly", "none"},
				Description: "Whether to send update notifications",
			},
			"modificationScope": {
				Type:        jsonschema.String,
				Enum:        []string{"thisAndFollowing", "all", "thisEventOnly"},
				Description: "Scope for recurring event modifications",
			},
			"originalStartTime": {
				Type:        jsonschema.String,
				Description: "Original start time in ISO 8601 format (required when modificationScope is 'thisEventOnly')",
			},
			"futureStartDate": {
				Type:        jsonschema.String,
				Description: "Start date for future instances in ISO 8601 format (required when modificationScope is 'thisAndFollowing')",
			},
			"checkConflicts": {
				Type:        jsonschema.Boolean,
				Description: "Whether to check for conflicts when updating (default: true when changing time)",
			},
			"calendarsToCheck": {
				Type:        jsonschema.Array,
				Description: "List of calendar IDs to check for conflicts",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"conferenceData": {
				Type:        jsonschema.Object,
				Description: "Conference properties for the event",
				Properties: map[string]jsonschema.Definition{
					"createRequest": {
						Type:        jsonschema.Object,
						Description: "Request to generate a new conference",
						Properties: map[string]jsonschema.Definition{
							"requestId": {
								Type:        jsonschema.String,
								Description: "Client-generated unique ID",
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
			"transparency": {
				Type:        jsonschema.String,
				Enum:        []string{"opaque", "transparent"},
				Description: "Whether the event blocks time. 'opaque' means busy, 'transparent' means available",
			},
			"visibility": {
				Type:        jsonschema.String,
				Enum:        []string{"default", "public", "private", "confidential"},
				Description: "Visibility of the event",
			},
			"guestsCanInviteOthers": {
				Type:        jsonschema.Boolean,
				Description: "Whether attendees can invite others",
			},
			"guestsCanModify": {
				Type:        jsonschema.Boolean,
				Description: "Whether attendees can modify the event",
			},
			"guestsCanSeeOtherGuests": {
				Type:        jsonschema.Boolean,
				Description: "Whether attendees can see other attendees",
			},
			"anyoneCanAddSelf": {
				Type:        jsonschema.Boolean,
				Description: "Whether anyone can add themselves",
			},
			"extendedProperties": {
				Type:        jsonschema.Object,
				Description: "Extended properties for the event",
				Properties: map[string]jsonschema.Definition{
					"private": {
						Type:        jsonschema.Object,
						Description: "Properties private to the app",
					},
					"shared": {
						Type:        jsonschema.Object,
						Description: "Properties visible to all attendees",
					},
				},
			},
			"attachments": {
				Type:        jsonschema.Array,
				Description: "File attachments for the event",
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
							Description: "MIME type",
						},
						"iconLink": {
							Type:        jsonschema.String,
							Description: "URL of the icon",
						},
						"fileId": {
							Type:        jsonschema.String,
							Description: "Google Drive file ID",
						},
					},
					Required: []string{"fileUrl"},
				},
			},
		},
		Required: []string{"calendarId", "eventId"},
	}
}

func (a *GoogleCalendarUpdateEventAction) Plannable() bool {
	return true
}

func GoogleCalendarUpdateEventConfigMeta() []config.Field {
	return []config.Field{}
}
