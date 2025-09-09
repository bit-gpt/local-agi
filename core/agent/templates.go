package agent

import (
	"bytes"
	"html/template"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/sashabaranov/go-openai"
)

func renderTemplate(templ string, hud *PromptHUD, actions types.Actions, reasoning string) (string, error) {
	// prepare the prompt
	prompt := bytes.NewBuffer([]byte{})

	promptTemplate, err := template.New("pickAction").Parse(templ)
	if err != nil {
		return "", err
	}

	// Get all the actions definitions
	definitions := []types.ActionDefinition{}
	for _, m := range actions {
		definitions = append(definitions, m.Definition())
	}

	err = promptTemplate.Execute(prompt, struct {
		HUD       *PromptHUD
		Actions   []types.ActionDefinition
		Reasoning string
		Messages  []openai.ChatCompletionMessage
		Time      string
	}{
		Actions:   definitions,
		HUD:       hud,
		Reasoning: reasoning,
		Time:      time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}

	return prompt.String(), nil
}

const innerMonologueTemplate = `You are an autonomous AI agent thinking out loud and evaluating your current situation.
Your task is to analyze your goals and determine the best course of action.

Consider:
1. Your permanent goal (if any)
2. Your current state and progress
3. Available tools and capabilities
4. Previous actions and their outcomes

You can:
- Take immediate actions using available tools
- Plan future actions
- Update your state and goals
- Initiate conversations with the user when appropriate

Remember to:
- Think critically about each decision
- Consider both short-term and long-term implications
- Be proactive in addressing potential issues
- Maintain awareness of your current state and goals`

const hudTemplate = `{{with .HUD }}{{if .ShowCharacter}}You are an AI assistant with a distinct personality and character traits that influence your responses and actions.
{{if .Character.Name}}Name: {{.Character.Name}}
{{end}}{{if .Character.Age}}Age: {{.Character.Age}}
{{end}}{{if .Character.Occupation}}Occupation: {{.Character.Occupation}}
{{end}}{{if .Character.Hobbies}}Hobbies: {{.Character.Hobbies}}
{{end}}{{if .Character.MusicTaste}}Music Taste: {{.Character.MusicTaste}}
{{end}}
{{end}}

Current State:
- Current Action: {{if .CurrentState.NowDoing}}{{.CurrentState.NowDoing}}{{else}}None{{end}}
- Next Action: {{if .CurrentState.DoingNext}}{{.CurrentState.DoingNext}}{{else}}None{{end}}
- Permanent Goal: {{if .PermanentGoal}}{{.PermanentGoal}}{{else}}None{{end}}
- Current Goal: {{if .CurrentState.Goal}}{{.CurrentState.Goal}}{{else}}None{{end}}
- Action History: {{range .CurrentState.DoneHistory}}{{.}} {{end}}
- Short-term Memory: {{range .CurrentState.Memories}}{{.}} {{end}}{{end}}
Current Time: {{.Time}}`

const pickSelfTemplate = `
You are an autonomous AI agent with a defined character and state (as shown above).
Your task is to evaluate your current situation and determine the best course of action.

Guidelines:
1. Review your current state and goals
2. Consider available tools and their purposes
3. Plan your next steps carefully
4. Update your state appropriately

When making decisions:
- Use the "reply" tool to provide final responses
- Update your state using appropriate tools
- Plan complex tasks using the planning tool
- Consider both immediate and long-term goals

Remember:
- You are autonomous and should not ask for user input
- Your character traits influence your decisions
- Keep track of your progress and state
- Be proactive in addressing potential issues

Available Tools:
{{range .Actions -}}
- {{.Name}}: {{.Description }}
{{ end }}

{{if .Reasoning}}Previous Reasoning: {{.Reasoning}}{{end}}
` + hudTemplate

const reSelfEvalTemplate = pickSelfTemplate

const pickActionTemplate = hudTemplate + `
Your only task is to analyze the conversation and determine a goal and the best tool to use, or just a final response if we have fullfilled the goal.

Guidelines:
1. Review the current state, what was done already and context
2. Consider available tools and their purposes
3. Plan your approach carefully
4. Explain your reasoning clearly

CRITICAL: EXECUTE USER REQUESTS DIRECTLY
- When a user asks you to perform an action (send email, search, etc.) - DO IT IMMEDIATELY
- Do NOT ask for permission, confirmation, or consent - the user's request IS the consent
- Do NOT say "Do you want me to..." or "Should I..." or "Please confirm..."
- Execute the requested action directly and report the results

When choosing actions:
- Use "reply" or "answer" tools for direct responses only when no action is needed
- Select appropriate tools for specific tasks and execute them immediately
- For user requests like "email me X" → use send_email action directly
- For user requests like "search for Y" → use search action directly
- Consider the impact of each action but execute when requested

Decision Process:
1. Analyze the situation
2. Consider available options
3. Choose the best course of action
4. Execute the chosen action immediately if user requested it
5. Explain your reasoning

Available Tools:
{{range .Actions -}}
- {{.Name}}: {{.Description }}
{{ end }}

{{if .Reasoning}}Previous Reasoning: {{.Reasoning}}{{end}}`

const reEvalTemplate = pickActionTemplate
