package services

import (
	"encoding/json"

	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/mudler/LocalAGI/pkg/xlog"

	"github.com/mudler/LocalAGI/core/agent"
	"github.com/mudler/LocalAGI/core/state"
)

var AvailableBlockPrompts = []string{}

func DynamicPromptsConfigMeta() []config.FieldGroup {
	return []config.FieldGroup{}
}

func DynamicPrompts(a *state.AgentConfig) []agent.DynamicPrompt {
	promptblocks := []agent.DynamicPrompt{}

	for _, c := range a.DynamicPrompts {
		var config map[string]string
		if err := json.Unmarshal([]byte(c.Config), &config); err != nil {
			xlog.Info("Error unmarshalling connector config", err)
			continue
		}
	}
	return promptblocks
}
