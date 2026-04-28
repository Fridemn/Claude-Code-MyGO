package repl

import (
	"encoding/json"
	"regexp"
	"strings"

	"claude-go/internal/tool"
)

var primitiveCallPattern = regexp.MustCompile(`(?is)\b(Read|Write|Edit|Glob|Grep|Bash|NotebookEdit|Agent)\s*\(`)
var jsObjectKeyPattern = regexp.MustCompile(`([{\s,])([A-Za-z_][A-Za-z0-9_]*)\s*:`)

type PrimitiveCall struct {
	Name  string
	Input tool.Input
}

// ExtractFirstPrimitiveCall tries to infer the first primitive tool call from a
// REPL script and returns its tool name and input payload.
func ExtractFirstPrimitiveCall(script string) (string, tool.Input, bool) {
	calls, ok := ExtractPrimitiveCalls(script)
	if !ok || len(calls) == 0 {
		return "", nil, false
	}
	return calls[0].Name, calls[0].Input, true
}

// ExtractPrimitiveCalls parses supported REPL script shapes into an execution plan.
// It intentionally avoids heuristic extraction for complex JS control flow:
// - JSON plans can contain multiple steps.
// - Function-style scripts must contain exactly one primitive call.
func ExtractPrimitiveCalls(script string) ([]PrimitiveCall, bool) {
	script = strings.TrimSpace(script)
	if script == "" {
		return nil, false
	}

	if calls, ok := extractPrimitiveCallsFromJSON(script); ok && len(calls) > 0 {
		return calls, true
	}
	call, ok := extractSinglePrimitiveFromFunctionCall(script)
	if !ok {
		return nil, false
	}
	return []PrimitiveCall{call}, true
}

func extractPrimitiveCallsFromJSON(script string) ([]PrimitiveCall, bool) {
	var payload any
	if err := json.Unmarshal([]byte(script), &payload); err != nil {
		return nil, false
	}
	calls := extractPrimitiveCallsFromAny(payload)
	if len(calls) == 0 {
		return nil, false
	}
	return calls, true
}

func extractPrimitiveCallsFromAny(payload any) []PrimitiveCall {
	collected := make([]PrimitiveCall, 0)
	switch typed := payload.(type) {
	case []any:
		for _, item := range typed {
			collected = append(collected, extractPrimitiveCallsFromAny(item)...)
		}
	case map[string]any:
		if name, input, ok := readPrimitiveCallShape(typed); ok {
			collected = append(collected, PrimitiveCall{Name: name, Input: input})
		}
		for _, key := range []string{"calls", "tools", "steps", "operations"} {
			if nested, ok := typed[key]; ok {
				collected = append(collected, extractPrimitiveCallsFromAny(nested)...)
			}
		}
	}
	return collected
}

func readPrimitiveCallShape(obj map[string]any) (string, tool.Input, bool) {
	var rawName string
	for _, key := range []string{"tool", "name", "toolName"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			rawName = value
			break
		}
	}
	if rawName == "" {
		return "", nil, false
	}

	name := normalizePrimitiveToolName(rawName)
	if !IsREPLOnlyTool(name) {
		return "", nil, false
	}

	input := tool.Input{}
	switch rawInput := obj["input"].(type) {
	case map[string]any:
		for k, v := range rawInput {
			input[k] = v
		}
	case nil:
	default:
		input["value"] = rawInput
	}
	if len(input) == 0 {
		for _, key := range []string{"args", "arguments", "params"} {
			if nested, ok := obj[key].(map[string]any); ok {
				for k, v := range nested {
					input[k] = v
				}
				break
			}
		}
	}
	if len(input) == 0 {
		// Support flat JSON call shapes:
		// {"tool":"Read","file_path":"..."}.
		for k, v := range obj {
			switch k {
			case "tool", "name", "toolName", "input", "args", "arguments", "params", "calls", "tools", "steps", "operations":
				continue
			default:
				input[k] = v
			}
		}
	}

	return name, input, true
}

func extractSinglePrimitiveFromFunctionCall(script string) (PrimitiveCall, bool) {
	locs := primitiveCallPattern.FindAllStringSubmatchIndex(script, -1)
	if len(locs) != 1 {
		// Avoid pretending we understand multi-step / branchy JS scripts.
		return PrimitiveCall{}, false
	}
	loc := locs[0]

	rawName := script[loc[2]:loc[3]]
	name := normalizePrimitiveToolName(rawName)
	if !IsREPLOnlyTool(name) {
		return PrimitiveCall{}, false
	}

	openParen := strings.Index(script[loc[0]:loc[1]], "(")
	if openParen < 0 {
		return PrimitiveCall{Name: name, Input: tool.Input{}}, true
	}
	start := loc[0] + openParen + 1
	arg, ok := sliceBalancedArgs(script, start)
	if !ok {
		return PrimitiveCall{Name: name, Input: tool.Input{}}, true
	}

	parsed := parseLooseObject(arg)
	if raw, ok := parsed["raw"].(string); ok && len(parsed) == 1 {
		parsed = normalizePrimitiveShorthandInput(name, raw)
	}
	return PrimitiveCall{Name: name, Input: parsed}, true
}

func sliceBalancedArgs(script string, start int) (string, bool) {
	depth := 1
	inString := false
	stringQuote := byte(0)
	escaped := false

	for i := start; i < len(script); i++ {
		ch := script[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == stringQuote {
				inString = false
				stringQuote = 0
			}
			continue
		}

		if ch == '"' || ch == '\'' || ch == '`' {
			inString = true
			stringQuote = ch
			continue
		}

		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return strings.TrimSpace(script[start:i]), true
			}
		}
	}
	return "", false
}

func parseLooseObject(raw string) tool.Input {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tool.Input{}
	}

	tryCandidates := []string{
		raw,
		strings.ReplaceAll(raw, `'`, `"`),
	}
	for _, candidate := range tryCandidates {
		if parsed, ok := parseJSONObject(candidate); ok {
			return parsed
		}
	}

	quoted := jsObjectKeyPattern.ReplaceAllString(tryCandidates[len(tryCandidates)-1], `${1}"${2}":`)
	if parsed, ok := parseJSONObject(quoted); ok {
		return parsed
	}
	return tool.Input{"raw": raw}
}

func parseJSONObject(candidate string) (tool.Input, bool) {
	candidate = strings.TrimSpace(candidate)
	if !(strings.HasPrefix(candidate, "{") && strings.HasSuffix(candidate, "}")) {
		return nil, false
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(candidate), &obj); err != nil {
		return nil, false
	}

	input := tool.Input{}
	for k, v := range obj {
		input[k] = v
	}
	return input, true
}

func normalizePrimitiveToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	for _, candidate := range REPLOnlyTools {
		if strings.EqualFold(candidate, trimmed) {
			return candidate
		}
	}
	return trimmed
}

func normalizePrimitiveShorthandInput(toolName, raw string) tool.Input {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tool.Input{}
	}

	if value, ok := parseQuotedLiteral(raw); ok {
		switch normalizePrimitiveToolName(toolName) {
		case "Read", "Write", "Edit", "NotebookEdit":
			return tool.Input{"file_path": value}
		case "Glob", "Grep":
			return tool.Input{"pattern": value}
		case "Bash":
			return tool.Input{"command": value}
		case "Agent":
			return tool.Input{"prompt": value}
		default:
			return tool.Input{"value": value}
		}
	}
	return tool.Input{"raw": raw}
}

func parseQuotedLiteral(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	quote := raw[0]
	if (quote != '"' && quote != '\'' && quote != '`') || raw[len(raw)-1] != quote {
		return "", false
	}
	inner := raw[1 : len(raw)-1]
	switch quote {
	case '"':
		var decoded string
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return "", false
		}
		return decoded, true
	case '\'':
		return strings.ReplaceAll(inner, `\'`, `'`), true
	case '`':
		return inner, true
	default:
		return "", false
	}
}
