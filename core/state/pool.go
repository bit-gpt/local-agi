package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mudler/LocalAgent/core/agent"
	. "github.com/mudler/LocalAgent/core/agent"
	"github.com/mudler/LocalAgent/core/sse"
	"github.com/mudler/LocalAgent/pkg/localrag"
	"github.com/mudler/LocalAgent/pkg/utils"

	"github.com/mudler/LocalAgent/pkg/xlog"
)

type AgentPool struct {
	sync.Mutex
	file                               string
	pooldir                            string
	pool                               AgentPoolData
	agents                             map[string]*Agent
	managers                           map[string]sse.Manager
	agentStatus                        map[string]*Status
	apiURL, model, localRAGAPI, apiKey string
	availableActions                   func(*AgentConfig) func(ctx context.Context) []Action
	connectors                         func(*AgentConfig) []Connector
	timeout                            string
}

type Status struct {
	ActionResults []ActionState
}

func (s *Status) addResult(result ActionState) {
	// If we have more than 10 results, remove the oldest one
	if len(s.ActionResults) > 10 {
		s.ActionResults = s.ActionResults[1:]
	}

	s.ActionResults = append(s.ActionResults, result)
}

func (s *Status) Results() []ActionState {
	return s.ActionResults
}

type AgentPoolData map[string]AgentConfig

func loadPoolFromFile(path string) (*AgentPoolData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	poolData := &AgentPoolData{}
	err = json.Unmarshal(data, poolData)
	return poolData, err
}

func NewAgentPool(
	model, apiURL, apiKey, directory string,
	LocalRAGAPI string,
	availableActions func(*AgentConfig) func(ctx context.Context) []agent.Action,
	connectors func(*AgentConfig) []Connector,
	timeout string,
) (*AgentPool, error) {
	// if file exists, try to load an existing pool.
	// if file does not exist, create a new pool.

	poolfile := filepath.Join(directory, "pool.json")

	if _, err := os.Stat(poolfile); err != nil {
		// file does not exist, create a new pool
		return &AgentPool{
			file:             poolfile,
			pooldir:          directory,
			apiURL:           apiURL,
			model:            model,
			localRAGAPI:      LocalRAGAPI,
			apiKey:           apiKey,
			agents:           make(map[string]*Agent),
			pool:             make(map[string]AgentConfig),
			agentStatus:      make(map[string]*Status),
			managers:         make(map[string]sse.Manager),
			connectors:       connectors,
			availableActions: availableActions,
			timeout:          timeout,
		}, nil
	}

	poolData, err := loadPoolFromFile(poolfile)
	if err != nil {
		return nil, err
	}
	return &AgentPool{
		file:             poolfile,
		apiURL:           apiURL,
		pooldir:          directory,
		model:            model,
		apiKey:           apiKey,
		agents:           make(map[string]*Agent),
		managers:         make(map[string]sse.Manager),
		agentStatus:      map[string]*Status{},
		pool:             *poolData,
		connectors:       connectors,
		localRAGAPI:      LocalRAGAPI,
		availableActions: availableActions,
		timeout:          timeout,
	}, nil
}

// CreateAgent adds a new agent to the pool
// and starts it.
// It also saves the state to the file.
func (a *AgentPool) CreateAgent(name string, agentConfig *AgentConfig) error {
	if _, ok := a.pool[name]; ok {
		return fmt.Errorf("agent %s already exists", name)
	}
	a.pool[name] = *agentConfig
	if err := a.Save(); err != nil {
		return err
	}

	return a.startAgentWithConfig(name, agentConfig)
}

func (a *AgentPool) List() []string {
	var agents []string
	for agent := range a.pool {
		agents = append(agents, agent)
	}
	// return a sorted list
	sort.SliceStable(agents, func(i, j int) bool {
		return agents[i] < agents[j]
	})
	return agents
}

func (a *AgentPool) GetStatusHistory(name string) *Status {
	a.Lock()
	defer a.Unlock()
	return a.agentStatus[name]
}

func (a *AgentPool) startAgentWithConfig(name string, config *AgentConfig) error {
	manager := sse.NewManager(5)
	ctx := context.Background()
	model := a.model
	if config.Model != "" {
		model = config.Model
	}
	if config.PeriodicRuns == "" {
		config.PeriodicRuns = "10m"
	}

	connectors := a.connectors(config)

	actions := a.availableActions(config)(ctx)

	stateFile, characterFile := a.stateFiles(name)

	actionsLog := []string{}
	for _, action := range actions {
		actionsLog = append(actionsLog, action.Definition().Name.String())
	}
	connectorLog := []string{}
	for _, connector := range connectors {
		connectorLog = append(connectorLog, fmt.Sprintf("%+v", connector))
	}

	xlog.Info(
		"Creating agent",
		"name", name,
		"model", model,
		"api_url", a.apiURL,
		"actions", actionsLog,
		"connectors", connectorLog,
	)

	opts := []Option{
		WithModel(model),
		WithLLMAPIURL(a.apiURL),
		WithContext(ctx),
		WithPeriodicRuns(config.PeriodicRuns),
		WithPermanentGoal(config.PermanentGoal),
		WithCharacter(Character{
			Name: name,
		}),
		WithActions(
			actions...,
		),
		WithStateFile(stateFile),
		WithCharacterFile(characterFile),
		WithLLMAPIKey(a.apiKey),
		WithTimeout(a.timeout),
		WithRAGDB(localrag.NewWrappedClient(a.localRAGAPI, name)),
		WithAgentReasoningCallback(func(state ActionCurrentState) bool {
			xlog.Info(
				"Agent is thinking",
				"agent", name,
				"reasoning", state.Reasoning,
				"action", state.Action.Definition().Name,
				"params", state.Params,
			)

			manager.Send(
				sse.NewMessage(
					fmt.Sprintf(`Thinking: %s`, utils.HTMLify(state.Reasoning)),
				).WithEvent("status"),
			)

			for _, c := range connectors {
				if !c.AgentReasoningCallback()(state) {
					return false
				}
			}
			return true
		}),
		WithSystemPrompt(config.SystemPrompt),
		WithAgentResultCallback(func(state ActionState) {
			a.Lock()
			if _, ok := a.agentStatus[name]; !ok {
				a.agentStatus[name] = &Status{}
			}

			a.agentStatus[name].addResult(state)
			a.Unlock()
			xlog.Debug(
				"Calling agent result callback",
			)

			text := fmt.Sprintf(`Reasoning: %s
			Action taken: %+v
			Parameters: %+v
			Result: %s`,
				state.Reasoning,
				state.ActionCurrentState.Action.Definition().Name,
				state.ActionCurrentState.Params,
				state.Result)
			manager.Send(
				sse.NewMessage(
					utils.HTMLify(
						text,
					),
				).WithEvent("status"),
			)

			for _, c := range connectors {
				c.AgentResultCallback()(state)
			}
		}),
	}
	if config.HUD {
		opts = append(opts, EnableHUD)
	}

	if config.StandaloneJob {
		opts = append(opts, EnableStandaloneJob)
	}

	if config.LongTermMemory {
		opts = append(opts, EnableLongTermMemory)
	}

	if config.SummaryLongTermMemory {
		opts = append(opts, EnableSummaryMemory)
	}

	if config.CanStopItself {
		opts = append(opts, CanStopItself)
	}
	if config.InitiateConversations {
		opts = append(opts, EnableInitiateConversations)
	}
	if config.RandomIdentity {
		if config.IdentityGuidance != "" {
			opts = append(opts, WithRandomIdentity(config.IdentityGuidance))
		} else {
			opts = append(opts, WithRandomIdentity())
		}
	}

	if config.EnableKnowledgeBase {
		opts = append(opts, EnableKnowledgeBase)
	}
	if config.EnableReasoning {
		opts = append(opts, EnableForceReasoning)
	}

	if config.KnowledgeBaseResults > 0 {
		opts = append(opts, EnableKnowledgeBaseWithResults(config.KnowledgeBaseResults))
	}

	xlog.Info("Starting agent", "name", name, "config", config)
	agent, err := New(opts...)
	if err != nil {
		return err
	}

	a.agents[name] = agent
	a.managers[name] = manager

	go func() {
		if err := agent.Run(); err != nil {
			xlog.Error("Agent stopped", "error", err.Error())
			panic(err)
		}
	}()

	for _, c := range connectors {
		go c.Start(agent)
	}

	go func() {
		for {
			time.Sleep(1 * time.Second) // Send a message every seconds
			manager.Send(sse.NewMessage(
				utils.HTMLify(agent.State().String()),
			).WithEvent("hud"))
		}
	}()

	return nil
}

// Starts all the agents in the pool
func (a *AgentPool) StartAll() error {
	for name, config := range a.pool {
		if a.agents[name] != nil { // Agent already started
			continue
		}
		if err := a.startAgentWithConfig(name, &config); err != nil {
			return err
		}
	}
	return nil
}

func (a *AgentPool) StopAll() {
	for _, agent := range a.agents {
		agent.Stop()
	}
}

func (a *AgentPool) Stop(name string) {
	if agent, ok := a.agents[name]; ok {
		agent.Stop()
	}
}

func (a *AgentPool) Start(name string) error {
	if agent, ok := a.agents[name]; ok {
		err := agent.Run()
		if err != nil {
			return fmt.Errorf("agent %s failed to start: %w", name, err)
		}
		xlog.Info("Agent started", "name", name)
		return nil
	}
	if config, ok := a.pool[name]; ok {
		return a.startAgentWithConfig(name, &config)
	}

	return fmt.Errorf("agent %s not found", name)
}

func (a *AgentPool) stateFiles(name string) (string, string) {
	stateFile := filepath.Join(a.pooldir, fmt.Sprintf("%s.state.json", name))
	characterFile := filepath.Join(a.pooldir, fmt.Sprintf("%s.character.json", name))

	return stateFile, characterFile
}

func (a *AgentPool) Remove(name string) error {

	// Cleanup character and state
	stateFile, characterFile := a.stateFiles(name)

	os.Remove(stateFile)
	os.Remove(characterFile)

	a.Stop(name)
	delete(a.agents, name)
	delete(a.pool, name)
	if err := a.Save(); err != nil {
		return err
	}
	return nil
}

func (a *AgentPool) Save() error {
	data, err := json.MarshalIndent(a.pool, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.file, data, 0644)
}

func (a *AgentPool) GetAgent(name string) *Agent {
	return a.agents[name]
}

func (a *AgentPool) GetConfig(name string) *AgentConfig {
	agent, exists := a.pool[name]
	if !exists {
		return nil
	}
	return &agent
}

func (a *AgentPool) GetManager(name string) sse.Manager {
	return a.managers[name]
}
