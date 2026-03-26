package types

// GoalType represents the high-level category of user intent.
type GoalType string

const (
	// GoalFileManagement covers file operations (read, write, delete, move, copy).
	GoalFileManagement GoalType = "file_management"
	// GoalCodeExecution covers shell commands and script execution.
	GoalCodeExecution GoalType = "code_execution"
	// GoalCommunication covers sending messages and emails.
	GoalCommunication GoalType = "communication"
	// GoalInformationRetrieval covers search and lookup operations.
	GoalInformationRetrieval GoalType = "information_retrieval"
	// GoalScheduling covers cron and recurring task management.
	GoalScheduling GoalType = "scheduling"
	// GoalNoteTaking covers note, memory, and journal operations.
	GoalNoteTaking GoalType = "note_taking"
	// GoalWebBrowsing covers HTTP requests and browser automation.
	GoalWebBrowsing GoalType = "web_browsing"
	// GoalGitOperations covers git commands and repository management.
	GoalGitOperations GoalType = "git_operations"
	// GoalTextProcessing covers summarization, translation, and text transforms.
	GoalTextProcessing GoalType = "text_processing"
	// GoalSystemManagement covers system administration tasks.
	GoalSystemManagement GoalType = "system_management"
	// GoalCreative covers writing, composing, and generating content.
	GoalCreative GoalType = "creative"
	// GoalConversation covers casual chat with no tool use.
	GoalConversation GoalType = "conversation"
	// GoalCalendar covers calendar event management.
	GoalCalendar GoalType = "calendar"
)

// AllGoalTypes contains every defined goal type for enumeration and validation.
var AllGoalTypes = []GoalType{
	GoalFileManagement, GoalCodeExecution, GoalCommunication,
	GoalInformationRetrieval, GoalScheduling, GoalNoteTaking,
	GoalWebBrowsing, GoalGitOperations, GoalTextProcessing,
	GoalSystemManagement, GoalCreative, GoalConversation, GoalCalendar,
}

// StructuredIntent is the output of the intent parser.
type StructuredIntent struct {
	// Goal is the high-level category.
	Goal GoalType `json:"goal"`

	// PrimaryAction is the most likely action type.
	PrimaryAction ActionType `json:"primary_action"`

	// Parameters extracted from the user's message.
	Parameters map[string]string `json:"parameters"`

	// Confidence is the parser's confidence in this interpretation (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Destructive is true if the action could cause data loss.
	Destructive bool `json:"destructive"`

	// Sensitivity is the estimated data sensitivity level.
	Sensitivity SensitivityLevel `json:"sensitivity"`

	// RawInput is the original user message.
	RawInput string `json:"raw_input"`
}
