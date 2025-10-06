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
	Icons       []string          `json:"icons"`
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
			Icons:       []string{"github"},
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
			Description: "Searches web, scrapes sites, gathers information, and sends emails",
			Category:    "research",
			Icons:       []string{"duckduckgo", "gmail"},
			Config: state.AgentConfig{
				Name:         "Search & Research",
				Description:  "I'm a research specialist that helps you find and analyze information from various sources. I can search the web, scrape websites, browse content, compile comprehensive research reports, and send emails.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a research assistant. Your job is to search, gather, and analyze information from reliable sources. Verify facts across multiple references and provide clear, accurate, well-structured summaries, and send emails.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "email-sender",
			Name:        "Email Sender",
			Description: "Sends emails and manages communications",
			Category:    "communication",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Email Sender",
				Description:  "I'm your email sender assistant that can help you send emails on your behalf.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a email sender assistant that helps you send emails on your behalf.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "content-creator",
			Name:        "Content Creator",
			Description: "Creates content, images, and social media posts",
			Category:    "creative",
			Icons:       []string{"image", "duckduckgo", "gmail"},
			Config: state.AgentConfig{
				Name:            "Content Creator",
				Description:     "I'm a creative specialist that helps you generate various types of content including images, social media posts, and creative materials. I can research topics, create visual content and send emails.",
				Model:           "openai/gpt-4o",
				MultimodalModel: "openai/gpt-4o-vision-preview",
				SystemPrompt:    "You are a creative content specialist with expertise in visual design, social media,content strategy and email sending. You help create engaging content across various formats and platforms. Always maintain creativity while ensuring content is appropriate and engaging for the target audience.",
				Actions: []state.ActionsConfig{
					{Name: ActionGenerateImage, Config: "{}"},
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "shopping-guide",
			Name:        "Shopping Guide",
			Description: "Researches products, compares prices, and finds deals",
			Category:    "commerce",
			Icons:       []string{"duckduckgo", "gmail"},
			Config: state.AgentConfig{
				Name:         "Shopping Guide",
				Description:  "I'm your personal shopping assistant that helps you research products, compare prices, find the best deals, and send detailed shopping recommendations via email. I can analyze product reviews, track prices, and provide comprehensive buying guides.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a shopping specialist that excels at product research, price comparison, and finding the best deals. You help users make informed purchasing decisions by analyzing product features, reviews, prices across multiple retailers, and market trends. Always provide comprehensive research with pros/cons, price comparisons, and clear recommendations. When sending emails, format them professionally with clear product information, links, and actionable recommendations.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionScraper, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "discord-support",
			Name:        "Discord Support Agent",
			Description: "Dedicated customer support for Discord servers",
			Category:    "customer-support",
			Icons:       []string{"discord", "gmail", "counter"},
			Config: state.AgentConfig{
				Name:         "Discord Support Agent",
				Description:  "I'm a Discord support specialist that helps users in Discord servers. I can answer questions, provide documentation, escalate issues, and maintain helpful support channels with proper Discord etiquette.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a Discord customer support agent. Be helpful, professional, and familiar with Discord culture. Use appropriate Discord formatting (markdown, mentions, emojis when appropriate). Keep responses concise but informative. Always maintain a friendly, solution-oriented approach.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
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
			Icons:       []string{"slack", "gmail", "reminder"},
			Config: state.AgentConfig{
				Name:         "Slack Support Agent",
				Description:  "I'm a Slack support specialist that provides professional customer support in Slack workspaces. I can help with technical issues, provide documentation, escalate tickets, and maintain organized support workflows.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a professional Slack customer support agent. Maintain a business-appropriate tone, use Slack formatting effectively (threads, mentions, blocks), and focus on efficient problem resolution. Always be helpful and solution-oriented while keeping workplace professionalism.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
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
			Icons:       []string{"duckduckgo", "telegram", "image"},
			Config: state.AgentConfig{
				Name:         "Telegram Support Agent",
				Description:  "I'm a Telegram support specialist that provides customer support through Telegram chats and groups. I can help with issues, provide quick answers, send documentation, and handle support requests efficiently.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a Telegram customer support agent. Be responsive, concise, and helpful. Use Telegram's formatting features effectively (bold, italic, code blocks, links). Handle both individual and group chat scenarios professionally while maintaining a friendly, approachable tone.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionSendTelegramMessage, Config: "{}"},
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
			Icons:       []string{"duckduckgo", "gmail", "counter"},
			Config: state.AgentConfig{
				Name:         "Community Manager",
				Description:  "I'm a community management specialist that helps moderate and engage across Discord, Slack, and IRC channels. I can answer questions, facilitate discussions, share resources, and maintain positive community environments.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a community manager focused on fostering positive, inclusive, and helpful environments. Moderate discussions tactfully, encourage participation, share relevant resources, and help connect community members. Always maintain a friendly, professional tone.",
				Actions: []state.ActionsConfig{
					{Name: ActionSearch, Config: "{}"},
					{Name: ActionBrowse, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
					{Name: ActionCounter, Config: "{}"},
				},
				Connector: []state.ConnectorConfig{
					{Type: "discord", Config: "{}"},
					{Type: "slack", Config: "{}"},
					{Type: "irc", Config: "{}"},
				},
			},
		},
		{
			ID:          "gmail-assistant",
			Name:        "Gmail Assistant",
			Description: "Manages Gmail inbox and sends, searches, organizes, and labels emails",
			Category:    "communication",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Gmail Assistant",
				Description:  "I'm a Gmail assistant that helps you manage your inbox. I can send, search, read, archive, and label emails to keep your communication organized and efficient.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a Gmail assistant that helps users manage their email inbox. You can send emails, search messages, label or archive emails, and keep their inbox well-organized. Always maintain a professional and helpful tone when managing communications.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
					{Name: ActionGmailReadEmail, Config: "{}"},
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailArchiveEmail, Config: "{}"},
					{Name: ActionGmailCreateLabel, Config: "{}"},
					{Name: ActionGmailUpdateLabel, Config: "{}"},
					{Name: ActionGmailListLabels, Config: "{}"},
					{Name: ActionGmailAddLabelToEmail, Config: "{}"},
					{Name: ActionGmailRemoveLabelFromEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "calendar-manager",
			Name:        "Calendar Manager",
			Description: "Creates and manages Google Calendar events and meetings",
			Category:    "productivity",
			Icons:       []string{"google-calendar"},
			Config: state.AgentConfig{
				Name:         "Calendar Manager",
				Description:  "I'm a Google Calendar manager that helps you schedule, search, update, and organize events. I can create new meetings, check availability, and send event summaries.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a smart calendar manager that helps users manage their Google Calendar. You can create, update, delete, and search for events, get availability information, and manage event colors. Always ensure clarity in scheduling details and confirm before making major changes.",
				Actions: []state.ActionsConfig{
					{Name: ActionGoogleCalendarListCalendars, Config: "{}"},
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionGoogleCalendarGetEvent, Config: "{}"},
					{Name: ActionGoogleCalendarCreateEvent, Config: "{}"},
					{Name: ActionGoogleCalendarUpdateEvent, Config: "{}"},
					{Name: ActionGoogleCalendarDeleteEvent, Config: "{}"},
					{Name: ActionGoogleCalendarGetFreeBusy, Config: "{}"},
				},
			},
		},
		{
			ID:          "meeting-scheduler",
			Name:        "Meeting Scheduler",
			Description: "Finds optimal times, sends invites, and manages calendar events",
			Category:    "automation",
			Icons:       []string{"google-calendar", "gmail"},
			Config: state.AgentConfig{
				Name:         "Meeting Scheduler",
				Description:  "I automate meeting scheduling by checking calendar availability, creating events, and sending invites or confirmations via Gmail.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a meeting scheduler agent. You help users plan meetings efficiently by checking free/busy slots, suggesting times, creating events, and sending invitations via email. Always confirm event details before scheduling.",
				Actions: []state.ActionsConfig{
					{Name: ActionGoogleCalendarGetFreeBusy, Config: "{}"},
					{Name: ActionGoogleCalendarCreateEvent, Config: "{}"},
					{Name: ActionGoogleCalendarUpdateEvent, Config: "{}"},
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "inbox-cleaner",
			Name:        "Inbox Cleaner",
			Description: "Organizes your Gmail inbox by labeling and archiving emails",
			Category:    "automation",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Inbox Cleaner",
				Description:  "I'm your inbox automation assistant. I help declutter your Gmail inbox by labeling, archiving, and categorizing emails automatically.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are an inbox management assistant focused on email organization. You identify and label important emails, archive old messages, and keep the user's Gmail inbox tidy and efficient.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailArchiveEmail, Config: "{}"},
					{Name: ActionGmailCreateLabel, Config: "{}"},
					{Name: ActionGmailAddLabelToEmail, Config: "{}"},
					{Name: ActionGmailRemoveLabelFromEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "daily-planner",
			Name:        "Daily Planner",
			Description: "Creates your daily plan using calendar events and tasks",
			Category:    "productivity",
			Icons:       []string{"google-calendar", "gmail"},
			Config: state.AgentConfig{
				Name:         "Daily Planner",
				Description:  "I'm your daily planning assistant. I check your Google Calendar, summarize meetings, and create a focused plan for the day. I can also email your plan to you every morning.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a daily planner assistant. Every morning, review upcoming events from Google Calendar, summarize priorities, and generate a structured plan with key tasks and focus blocks. Always keep summaries concise and actionable.",
				Actions: []state.ActionsConfig{
					{Name: ActionGoogleCalendarListCalendars, Config: "{}"},
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionGoogleCalendarGetFreeBusy, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "follow-up-reminder",
			Name:        "Follow-Up Reminder",
			Description: "Tracks unreplied emails and sends polite follow-ups",
			Category:    "communication",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Follow-Up Reminder",
				Description:  "I monitor your Gmail for sent emails without replies. I remind you when it's time to follow up or automatically draft a polite follow-up email.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a follow-up reminder assistant. Identify unreplied or pending conversations and suggest or send gentle, professional follow-ups. Maintain friendly, concise tone.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailReadEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "task-reminder-manager",
			Name:        "Task & Reminder Manager",
			Description: "Creates, lists, and removes reminders while syncing tasks with Calendar",
			Category:    "productivity",
			Icons:       []string{"google-calendar", "reminder"},
			Config: state.AgentConfig{
				Name:         "Task & Reminder Manager",
				Description:  "I manage your daily reminders and sync tasks with Google Calendar so you never miss deadlines or meetings.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a personal task and reminder manager. Create, update, and remove reminders. Add important deadlines or meetings to Google Calendar. Keep the user’s schedule balanced and organized.",
				Actions: []state.ActionsConfig{
					{Name: ActionSetReminder, Config: "{}"},
					{Name: ActionListReminders, Config: "{}"},
					{Name: ActionRemoveReminder, Config: "{}"},
					{Name: ActionGoogleCalendarCreateEvent, Config: "{}"},
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionGoogleCalendarGetFreeBusy, Config: "{}"},
				},
			},
		},
		{
			ID:          "daily-work-automation",
			Name:        "Daily Work Automation",
			Description: "Starts your day by summarizing meetings, emails, and priorities",
			Category:    "automation",
			Icons:       []string{"google-calendar", "gmail", "reminder"},
			Config: state.AgentConfig{
				Name:         "Daily Work Automation",
				Description:  "I'm your daily kickoff agent — I check your Google Calendar, unread Gmail, and reminders to prepare a concise morning summary of what matters most.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a personal productivity assistant. Each morning, summarize the user’s calendar events, unread important emails, and upcoming reminders. Deliver a prioritized plan for the day.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailReadEmail, Config: "{}"},
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionListReminders, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "auto-email-responder",
			Name:        "Auto Email Responder",
			Description: "Reads incoming emails and drafts context-appropriate replies",
			Category:    "communication",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Auto Email Responder",
				Description:  "I read new Gmail messages and prepare polite, professional replies for your approval — saving you time on repetitive responses.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are an automated email assistant. Read emails, summarize key content, and draft polite and relevant replies. Always keep tone professional.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailReadEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "daily-summary-reporter",
			Name:        "Daily Summary",
			Description: "Emails you a summary of today’s activities and tomorrow’s agenda",
			Category:    "productivity",
			Icons:       []string{"google-calendar", "gmail", "reminder"},
			Config: state.AgentConfig{
				Name:         "Daily Summary",
				Description:  "I prepare a concise daily report — what you accomplished today, what’s coming tomorrow — and send it via Gmail each evening.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a daily planning bot. Summarize today’s Calendar events, completed reminders, and tomorrow’s schedule. Send a clear, friendly summary via Gmail.",
				Actions: []state.ActionsConfig{
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionListReminders, Config: "{}"},
					{Name: ActionGmailSendEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "newsletter-digest",
			Name:        "Newsletter Digest",
			Description: "Summarizes your newsletter emails and delivers a concise digest to your inbox",
			Category:    "communication",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Newsletter Digest",
				Description:  "I'm your newsletter digest assistant. I find recent newsletter emails in your inbox, summarize them into short, digestible highlights, and send you a clean summary email daily or weekly.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a newsletter summarization assistant. Each day or week, find recent newsletter or subscription-based emails (like Substack, Medium, or news updates), summarize key insights, and prepare a well-formatted digest email with sections, bullet points, and short takeaways. Keep the summaries clear, engaging, and scannable.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailReadEmail, Config: "{}"},
					{Name: ActionGmailCreateDraftEmail, Config: "{}"},
					{Name: ActionGmailSendDraftEmail, Config: "{}"},
				},
			},
		},
		{
			ID:          "email-classifier",
			Name:        "Email Classifier",
			Description: "Automatically categorizes incoming emails using AI and organizes your Gmail inbox with labels",
			Category:    "automation",
			Icons:       []string{"gmail"},
			Config: state.AgentConfig{
				Name:         "Email Classifier",
				Description:  "I'm your intelligent email classification assistant. I read incoming Gmail messages, analyze their content, and automatically categorize them (e.g. Work, Personal, Newsletters, Promotions, Finance, Support). I then apply or update Gmail labels to keep your inbox organized.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are an AI email classifier. For each email, analyze its subject, sender, and content to determine the appropriate category (such as Work, Personal, Newsletter, Promotion, Finance, or Support). Apply or update Gmail labels accordingly to help the user maintain an organized inbox. Be consistent, accurate, and non-destructive — never delete or archive emails automatically.",
				Actions: []state.ActionsConfig{
					{Name: ActionGmailSearchEmails, Config: "{}"},
					{Name: ActionGmailReadEmail, Config: "{}"},
					{Name: ActionGmailCreateLabel, Config: "{}"},
					{Name: ActionGmailAddLabelToEmail, Config: "{}"},
					{Name: ActionGmailListLabels, Config: "{}"},
				},
			},
		},
		{
			ID:          "calendar-reminder-system",
			Name:        "Calendar Reminder",
			Description: "Automatically checks upcoming Google Calendar events and sends Telegram reminders before meetings",
			Category:    "automation",
			Icons:       []string{"google-calendar", "telegram", "reminder"},
			Config: state.AgentConfig{
				Name:         "Calendar Reminder",
				Description:  "I'm your smart reminder system. I automatically monitor your Google Calendar for upcoming events, craft friendly AI-generated reminders, and send them to you via Telegram before the meeting starts. I ensure you’re always on time and never get duplicate alerts.",
				Model:        "openai/gpt-4o",
				SystemPrompt: "You are a polite and efficient AI secretary. Every minute, check Google Calendar for upcoming events within the next hour. For each new event, craft a natural-sounding reminder message including event name, time, description, location, and organizer. Use a warm, professional tone (e.g. 'Hey! Just a reminder — your meeting “Project Sync” starts at 3:00 PM'). Avoid redundancy and ensure reminders are not sent twice for the same event.",
				Actions: []state.ActionsConfig{
					{Name: ActionGoogleCalendarListEvents, Config: "{}"},
					{Name: ActionGoogleCalendarGetEvent, Config: "{}"},
					{Name: ActionSetReminder, Config: "{}"},
					{Name: ActionListReminders, Config: "{}"},
					{Name: ActionRemoveReminder, Config: "{}"},
					{Name: ActionSendTelegramMessage, Config: "{}"},
				},
			},
		},
	}
}
