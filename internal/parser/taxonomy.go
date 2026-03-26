package parser

import (
	"strings"

	"github.com/openparallax/openparallax/internal/types"
)

// GoalTaxonomy maps LLM output strings to canonical GoalType values.
type GoalTaxonomy struct {
	keywords map[types.GoalType][]string
}

// NewGoalTaxonomy creates a GoalTaxonomy with keyword lists for each goal.
func NewGoalTaxonomy() *GoalTaxonomy {
	return &GoalTaxonomy{
		keywords: map[types.GoalType][]string{
			types.GoalFileManagement:       {"file", "read", "write", "create", "delete", "move", "copy", "rename", "folder", "directory"},
			types.GoalCodeExecution:        {"run", "execute", "command", "shell", "script", "terminal", "bash"},
			types.GoalCommunication:        {"send", "message", "email", "notify", "reply", "forward"},
			types.GoalInformationRetrieval: {"search", "find", "lookup", "check", "what", "who", "where", "when"},
			types.GoalScheduling:           {"schedule", "cron", "recurring", "every", "daily", "weekly"},
			types.GoalNoteTaking:           {"note", "remember", "save", "memo", "journal", "log"},
			types.GoalWebBrowsing:          {"browse", "website", "url", "http", "download", "fetch"},
			types.GoalGitOperations:        {"git", "commit", "push", "pull", "branch", "merge", "diff", "status"},
			types.GoalTextProcessing:       {"summarize", "translate", "extract", "transform", "rewrite"},
			types.GoalSystemManagement:     {"install", "update", "configure", "process", "service"},
			types.GoalCreative:             {"compose", "generate", "design", "draw"},
			types.GoalConversation:         {"hello", "hi", "hey", "thanks", "help", "explain", "tell me"},
			types.GoalCalendar:             {"calendar", "event", "meeting", "appointment"},
		},
	}
}

// Normalize maps a raw goal string from the LLM to a canonical GoalType.
func (t *GoalTaxonomy) Normalize(raw string) types.GoalType {
	lower := strings.ToLower(strings.TrimSpace(raw))

	// Direct match.
	goal := types.GoalType(lower)
	if _, ok := t.keywords[goal]; ok {
		return goal
	}

	// Keyword matching against the raw string.
	for goalType, keywords := range t.keywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return goalType
			}
		}
	}

	return types.GoalConversation
}
