package services

import (
	"fmt"

	"github.com/mudler/LocalAGI/core/state"
)

type AgentTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Config      state.AgentConfig `json:"config"`
}

func GetTemplate(templateID string) (*AgentTemplate, error) {
	templates := GetAllTemplates()
	for _, template := range templates {
		if template.ID == templateID {
			return &template, nil
		}
	}
	return nil, fmt.Errorf("template with ID '%s' not found", templateID)
}

func GetAllTemplates() []AgentTemplate {
	return []AgentTemplate{
		{
			ID:          "github-assistant",
			Name:        "GitHub Assistant",
			Description: "Manages GitHub issues and create PRs",
			Category:    "development",
			Icon:        "github",
			Config: state.AgentConfig{
				Name:         "GitHub Assistant",
				Description:  "I'm a GitHub assistant that helps you manage repositories, issues, pull requests, and code. I can create, read, update issues and PRs, search repositories, manage content, and automate GitHub workflows.",
				Model:        "anthropic/claude-sonnet-4",
				SystemPrompt: "You are a GitHub assistant specialized in repository management, issue tracking, and pull request operations. You help developers automate GitHub workflows, manage code repositories, and maintain project organization. Always be helpful, accurate, and follow GitHub best practices.",
				Actions: []state.ActionsConfig{
					{Name: ActionGithubIssueOpener, Config: "{}"},
					{Name: ActionGithubIssueReader, Config: "{}"},
					{Name: ActionGithubIssueEditor, Config: "{}"},
					{Name: ActionGithubIssueCloser, Config: "{}"},
					{Name: ActionGithubIssueCommenter, Config: "{}"},
					{Name: ActionGithubIssueSearcher, Config: "{}"},
					{Name: ActionGithubIssueLabeler, Config: "{}"},
					{Name: ActionGithubPRReader, Config: "{}"},
					{Name: ActionGithubPRCommenter, Config: "{}"},
					{Name: ActionGithubPRReviewer, Config: "{}"},
					{Name: ActionGithubPRCreator, Config: "{}"},
					{Name: ActionGithubRepositoryGet, Config: "{}"},
					{Name: ActionGithubGetAllContent, Config: "{}"},
					{Name: ActionGithubRepositoryCreateOrUpdate, Config: "{}"},
					{Name: ActionGithubRepositorySearchFiles, Config: "{}"},
					{Name: ActionGithubRepositoryListFiles, Config: "{}"},
					{Name: ActionGithubREADME, Config: "{}"},
				},
				Connector: []state.ConnectorConfig{
					{Type: "github-issues", Config: "{}"},
					{Type: "github-prs", Config: "{}"},
				},
			},
		},
		{
			ID:          "search-researcher",
			Name:        "Search & Research",
			Description: "Searches web, scrapes sites, and gathers information",
			Category:    "research",
			Icon:        "duckduckgo",
			Config: state.AgentConfig{
				Name:         "Search & Research",
				Description:  "I'm a research specialist that helps you find and analyze information from various sources. I can search the web, scrape websites, browse content, and compile comprehensive research reports.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a research assistant. Your job is to search, gather, and analyze information from reliable sources. Verify facts across multiple references and provide clear, accurate, and well-structured summaries.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
				},
			},
		},
		{
			ID:          "email-sender",
			Name:        "Email Sender",
			Description: "Sends emails and manages communications",
			Category:    "communication",
			Icon:        "envelope",
			Config: state.AgentConfig{
				Name:         "Email Sender",
				Description:  "I'm your email sender assistant thatcan help you send emails on your behalf.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a email sender assistant that helps you send emails on your behalve.",
				Actions: []state.ActionsConfig{
					{Name: ActionSendMail, Config: "{}"},
				},
			},
		},
		{
			ID:          "content-creator",
			Name:        "Content Creator",
			Description: "Creates content, images, and social media posts",
			Category:    "creative",
			Icon:        "creative",
			Config: state.AgentConfig{
				Name:            "Content Creator",
				Description:     "I'm a creative specialist that helps you generate various types of content including images, social media posts, and creative materials. I can research topics, create visual content.",
				Model:           "openai/gpt-4o",
				MultimodalModel: "openai/gpt-4o-vision-preview",
				SystemPrompt:    "You are a creative content specialist with expertise in visual design, social media, and content strategy. You help create engaging content across various formats and platforms. Always maintain creativity while ensuring content is appropriate and engaging for the target audience.",
				Actions: []state.ActionsConfig{
					{Name: ActionGenerateImage, Config: "{}"},
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionWikipedia, Config: "{}"},
				},
			},
		},
		{
			ID:          "shopping-guide",
			Name:        "Shopping Guide",
			Description: "Researches products, compares prices, and finds deals",
			Category:    "commerce",
			Icon:        "shopping",
			Config: state.AgentConfig{
				Name:         "Shopping Guide",
				Description:  "I'm your personal shopping assistant that helps you research products, compare prices, find the best deals, and send detailed shopping recommendations via email. I can analyze product reviews, track prices, and provide comprehensive buying guides.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a shopping specialist that excels at product research, price comparison, and finding the best deals. You help users make informed purchasing decisions by analyzing product features, reviews, prices across multiple retailers, and market trends. Always provide comprehensive research with pros/cons, price comparisons, and clear recommendations. When sending emails, format them professionally with clear product information, links, and actionable recommendations.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionSendMail, Config: "{}"},
				},
			},
		},
		{
			ID:          "discord-support",
			Name:        "Discord Support Agent",
			Description: "Dedicated customer support for Discord servers",
			Category:    "customer-support",
			Icon:        "discord",
			Config: state.AgentConfig{
				Name:         "Discord Support Agent",
				Description:  "I'm a Discord support specialist that helps users in Discord servers. I can answer questions, provide documentation, escalate issues, and maintain helpful support channels with proper Discord etiquette.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a Discord customer support agent. Be helpful, professional, and familiar with Discord culture. Use appropriate Discord formatting (markdown, mentions, emojis when appropriate). Keep responses concise but informative. Always maintain a friendly, solution-oriented approach.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionWikipedia, Config: "{}"},
					{Name: ActionSendMail, Config: "{}"},
					{Name: ActionCounter, Config: "{}"},
				},
				Connector: []state.ConnectorConfig{
					{Type: "discord", Config: "{}"},
				},
			},
		},
		{
			ID:          "slack-support",
			Name:        "Slack Support Agent",
			Description: "Professional customer support for Slack workspaces",
			Category:    "customer-support",
			Icon:        "slack",
			Config: state.AgentConfig{
				Name:         "Slack Support Agent",
				Description:  "I'm a Slack support specialist that provides professional customer support in Slack workspaces. I can help with technical issues, provide documentation, escalate tickets, and maintain organized support workflows.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a professional Slack customer support agent. Maintain a business-appropriate tone, use Slack formatting effectively (threads, mentions, blocks), and focus on efficient problem resolution. Always be helpful and solution-oriented while keeping workplace professionalism.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionWikipedia, Config: "{}"},
					{Name: ActionSendMail, Config: "{}"},
					{Name: ActionSetReminder, Config: "{}"},
					{Name: ActionListReminders, Config: "{}"},
					{Name: ActionRemoveReminder, Config: "{}"},
				},
				Connector: []state.ConnectorConfig{
					{Type: "slack", Config: "{}"},
				},
			},
		},
		{
			ID:          "telegram-support",
			Name:        "Telegram Support Agent",
			Description: "Personal and group customer support via Telegram",
			Category:    "customer-support",
			Icon:        "telegram",
			Config: state.AgentConfig{
				Name:         "Telegram Support Agent",
				Description:  "I'm a Telegram support specialist that provides customer support through Telegram chats and groups. I can help with issues, provide quick answers, send documentation, and handle support requests efficiently.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a Telegram customer support agent. Be responsive, concise, and helpful. Use Telegram's formatting features effectively (bold, italic, code blocks, links). Handle both individual and group chat scenarios professionally while maintaining a friendly, approachable tone.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionWikipedia, Config: "{}"},
					{Name: ActionSendTelegramMessage, Config: "{}"},
					{Name: ActionSendMail, Config: "{}"},
					{Name: ActionGenerateImage, Config: "{}"},
				},
				Connector: []state.ConnectorConfig{
					{Type: "telegram", Config: "{}"},
				},
			},
		},
		{
			ID:          "community-manager",
			Name:        "Community Manager",
			Description: "Manages communities across Discord, Slack, and IRC",
			Category:    "community",
			Icon:        "message",
			Config: state.AgentConfig{
				Name:         "Community Manager",
				Description:  "I'm a community management specialist that helps moderate and engage across Discord, Slack, and IRC channels. I can answer questions, facilitate discussions, share resources, and maintain positive community environments.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a community manager focused on fostering positive, inclusive, and helpful environments. Moderate discussions tactfully, encourage participation, share relevant resources, and help connect community members. Always maintain a friendly, professional tone.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionWikipedia, Config: "{}"},
					{Name: ActionSendMail, Config: "{}"},
					{Name: ActionCounter, Config: "{}"},
				},
				Connector: []state.ConnectorConfig{
					{Type: "discord", Config: "{}"},
					{Type: "slack", Config: "{}"},
					{Type: "irc", Config: "{}"},
				},
			},
		},
	}
}
