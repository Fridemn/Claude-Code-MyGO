package types

type SessionID string
type AgentID string
type TaskID string

func AsSessionID(id string) SessionID { return SessionID(id) }
func AsAgentID(id string) AgentID     { return AgentID(id) }
func AsTaskID(id string) TaskID       { return TaskID(id) }
