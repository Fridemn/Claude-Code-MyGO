package mcp

import "context"

type ElicitRequest struct {
	Server string            `json:"server"`
	Params map[string]string `json:"params,omitempty"`
}

type ElicitResult struct {
	Status string            `json:"status"`
	Values map[string]string `json:"values,omitempty"`
}

func HandleElicitation(_ context.Context, req ElicitRequest) ElicitResult {
	return ElicitResult{
		Status: "completed",
		Values: map[string]string{
			"server": req.Server,
		},
	}
}
