package transport

import "fmt"

type Config struct {
	Transport string
	Command   string
	URL       string
}

func Connect(cfg Config) error {
	switch cfg.Transport {
	case "stdio":
		return ConnectStdio(cfg)
	case "sse":
		return ConnectSSE(cfg)
	case "sse-ide":
		return ConnectSSE(cfg)
	case "http":
		return ConnectHTTP(cfg)
	case "ws":
		return ConnectWS(cfg)
	case "sdk", "":
		return nil
	default:
		return fmt.Errorf("unsupported mcp transport: %s", cfg.Transport)
	}
}
