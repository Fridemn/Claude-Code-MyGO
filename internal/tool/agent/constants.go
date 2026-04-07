package agent

// Agent tool constants
const (
	// Tool names
	AgentToolName        = "Agent"
	LegacyAgentToolName  = "Task"

	// Verification agent type
	VerificationAgentType = "verification"

	// Built-in agent types that run once and return a report
	// These skip the agentId/SendMessage/usage trailer to save tokens
)

// OneShotBuiltinAgentTypes contains agent types that run once and return a report
var OneShotBuiltinAgentTypes = map[string]bool{
	"Explore": true,
	"Plan":    true,
}
