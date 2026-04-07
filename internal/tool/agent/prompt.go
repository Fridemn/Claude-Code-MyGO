package agent

import (
	"strings"
)

const AgentToolDescription = "Launch a new agent to handle complex, multi-step tasks autonomously"

// GetPrompt returns the tool prompt for the Agent tool
func GetPrompt(availableAgents []AgentDefinition, isCoordinator bool, allowedAgentTypes []string) string {
	var effectiveAgents []AgentDefinition
	if allowedAgentTypes != nil {
		for _, a := range availableAgents {
			for _, t := range allowedAgentTypes {
				if a.GetAgentType() == t {
					effectiveAgents = append(effectiveAgents, a)
					break
				}
			}
		}
	} else {
		effectiveAgents = availableAgents
	}

	agentListSection := buildAgentListSection(effectiveAgents)

	// Coordinator mode gets a slim prompt
	if isCoordinator {
		return strings.TrimSpace(agentToolPromptCore + "\n\n" + agentListSection)
	}

	// Full prompt for non-coordinator mode
	return strings.TrimSpace(agentToolPromptCore + "\n\n" + agentListSection + "\n\n" +
		whenNotToUseSection + "\n\n" + usageNotesSection + "\n\n" + writingThePromptSection + "\n\n" + examplesSection)
}

const agentToolPromptCore = `Launch a new agent to handle complex, multi-step tasks autonomously.

The Agent tool launches specialized agents (subprocesses) that autonomously handle complex tasks. Each agent type has specific capabilities and tools available to it.

When using the Agent tool, specify a subagent_type parameter to select which agent type to use. If omitted, the general-purpose agent is used.`

const whenNotToUseSection = `When NOT to use the Agent tool:
- If you want to read a specific file path, use the Read tool or Glob tool instead of the Agent tool, to find the match more quickly
- If you are searching for a specific class definition like "class Foo", use the Glob tool instead, to find the match more quickly
- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead of the Agent tool, to find the match more quickly
- Other tasks that are not related to the agent descriptions above`

const usageNotesSection = `Usage notes:
- Always include a short description (3-5 words) summarizing what the agent will do
- Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses
- When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.
- You can optionally run agents in the background using the run_in_background parameter. When an agent runs in the background, you will be automatically notified when it completes — do NOT sleep, poll, or proactively check on its progress. Continue with other work or respond to the user instead.
- **Foreground vs background**: Use foreground (default) when you need the agent's results before you can proceed — e.g., research agents whose findings inform your next steps. Use background when you have genuinely independent work to do in parallel.
- To continue a previously spawned agent, use SendMessage with the agent's ID or name as the to field. The agent resumes with its full context preserved. Each Agent invocation starts fresh — provide a complete task description.
- The agent's outputs should generally be trusted
- Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.), since it is not aware of the user's intent
- If the agent description mentions that it should be used proactively, then you should try your best to use it without the user having to ask you for it first. Use your judgement.
- If the user specifies that they want you to run agents "in parallel", you MUST send a single message with multiple Agent tool use content blocks. For example, if you need to launch both a build-validator agent and a test-runner agent in parallel, send a single message with both tool calls.`

const writingThePromptSection = `## Writing the prompt

Brief the agent like a smart colleague who just walked into the room — it hasn't seen this conversation, doesn't know what you've tried, doesn't understand why this task matters.
- Explain what you're trying to accomplish and why.
- Describe what you've already learned or ruled out.
- Give enough context about the surrounding problem that the agent can make judgment calls rather than just following a narrow instruction.
- If you need a short response, say so ("report in under 200 words").
- Lookups: hand over the exact command. Investigations: hand over the question — prescribed steps become dead weight when the premise is wrong.

Terse command-style prompts produce shallow, generic work.

**Never delegate understanding.** Don't write "based on your findings, fix the bug" or "based on the research, implement it." Those phrases push synthesis onto the agent instead of doing it yourself. Write prompts that prove you understood: include file paths, line numbers, what specifically to change.`

const examplesSection = `Example usage:

<example_agent_descriptions>
"test-runner": use this agent after you are done writing code to run tests
"greeting-responder": use this agent to respond to user greetings with a friendly joke
</example_agent_descriptions>

<example>
user: "Please write a function that checks if a number is prime"
assistant: I'm going to use the Write tool to write the following code:
<code>
function isPrime(n) {
  if (n <= 1) return false
  for (let i = 2; i * i <= n; i++) {
    if (n % i === 0) return false
  }
  return true
}
</code>
<commentary>
Since a significant piece of code was written and the task was completed, now use the test-runner agent to run the tests
</commentary>
assistant: Uses the Agent tool to launch the test-runner agent
</example>

<example>
user: "Hello"
<commentary>
Since the user is greeting, use the greeting-responder agent to respond with a friendly joke
</commentary>
assistant: "I'm going to use the Agent tool to launch the greeting-responder agent"
</example>
`

func buildAgentListSection(agents []AgentDefinition) string {
	if len(agents) == 0 {
		return "Available agent types: none configured"
	}

	lines := []string{"Available agent types and the tools they have access to:"}
	for _, agent := range agents {
		line := formatAgentLine(agent)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func formatAgentLine(agent AgentDefinition) string {
	toolsDesc := getToolsDescription(agent)
	return "- " + agent.GetAgentType() + ": " + agent.GetWhenToUse() + " (Tools: " + toolsDesc + ")"
}

func getToolsDescription(agent AgentDefinition) string {
	tools := agent.GetTools()
	disallowed := agent.GetDisallowedTools()

	hasAllowlist := tools != nil && len(tools) > 0
	hasDenylist := disallowed != nil && len(disallowed) > 0

	if hasAllowlist && hasDenylist {
		// Both defined: filter allowlist by denylist
		denySet := make(map[string]bool)
		for _, t := range disallowed {
			denySet[t] = true
		}
		var effective []string
		for _, t := range tools {
			if !denySet[t] {
				effective = append(effective, t)
			}
		}
		if len(effective) == 0 {
			return "None"
		}
		return strings.Join(effective, ", ")
	} else if hasAllowlist {
		return strings.Join(tools, ", ")
	} else if hasDenylist {
		return "All tools except " + strings.Join(disallowed, ", ")
	}
	return "All tools"
}
