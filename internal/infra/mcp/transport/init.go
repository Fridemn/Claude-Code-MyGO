package transport

import mcpsdk "github.com/mark3labs/mcp-go/mcp"

// initRequest creates a standard MCP initialization request
func initRequest() mcpsdk.InitializeRequest {
	return mcpsdk.InitializeRequest{
		Params: mcpsdk.InitializeParams{
			ProtocolVersion: mcpsdk.LATEST_PROTOCOL_VERSION,
			Capabilities: mcpsdk.ClientCapabilities{
				Roots: &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{},
			},
			ClientInfo: mcpsdk.Implementation{
				Name:    "claude-go",
				Version: "1.0.0",
			},
		},
	}
}
