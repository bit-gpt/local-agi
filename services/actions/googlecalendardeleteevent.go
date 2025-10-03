package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/oauth"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func NewGoogleCalendarDeleteEvent(config map[string]string) *GoogleCalendarDeleteEventAction {
	return &GoogleCalendarDeleteEventAction{}
}

type GoogleCalendarDeleteEventAction struct{}

func (a *GoogleCalendarDeleteEventAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
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

	sendUpdates, ok := params["sendUpdates"].(string)
	if !ok || sendUpdates == "" {
		sendUpdates = "all"
	}

	// Validate sendUpdates value
	validSendUpdates := map[string]bool{
		"all":          true,
		"externalOnly": true,
		"none":         true,
	}
	if !validSendUpdates[sendUpdates] {
		return types.ActionResult{}, fmt.Errorf("invalid sendUpdates value: %s (must be 'all', 'externalOnly', or 'none')", sendUpdates)
	}

	deleteCall := calendarService.Events.Delete(calendarID, eventID).Context(ctx)

	// Set the sendUpdates parameter
	deleteCall = deleteCall.SendUpdates(sendUpdates)

	err = deleteCall.Do()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to delete event %s from calendar %s: %v", eventID, calendarID, err)
	}

	var notificationMsg string
	switch sendUpdates {
	case "all":
		notificationMsg = "Cancellation notifications sent to all attendees."
	case "externalOnly":
		notificationMsg = "Cancellation notifications sent to external attendees only."
	case "none":
		notificationMsg = "No cancellation notifications sent."
	}

	result := fmt.Sprintf("Successfully deleted event '%s' from calendar '%s'.\n%s",
		eventID, calendarID, notificationMsg)

	return types.ActionResult{
		Result: result,
	}, nil
}

func (a *GoogleCalendarDeleteEventAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "google-calendar-delete-event",
		Description: "Delete a specific event from a Google Calendar by its event ID. Optionally sends cancellation notifications to attendees.",
		Properties: map[string]jsonschema.Definition{
			"calendarId": {
				Type:        jsonschema.String,
				Description: "ID of the calendar (use 'primary' for the main calendar)",
			},
			"eventId": {
				Type:        jsonschema.String,
				Description: "ID of the event to delete",
			},
			"sendUpdates": {
				Type:        jsonschema.String,
				Description: "Whether to send cancellation notifications. Options: 'all' (send to all attendees), 'externalOnly' (send only to external attendees), 'none' (don't send notifications). Default: 'all'",
				Enum:        []string{"all", "externalOnly", "none"},
			},
		},
		Required: []string{"calendarId", "eventId"},
	}
}

func (a *GoogleCalendarDeleteEventAction) Plannable() bool {
	return true
}

func GoogleCalendarDeleteEventConfigMeta() []config.Field {
	return []config.Field{
		// No configuration needed - uses OAuth credentials from agent
	}
}
