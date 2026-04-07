# Checkpoint 7: Services - API & Auth

## Scope
Implement API client and authentication services.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/services/api/claude.ts` | Claude API client |
| `src/services/api/client.ts` | API client config |
| `src/services/api/errors.ts` | API errors |
| `src/services/api/withRetry.ts` | Retry logic |
| `src/services/oauth/*.ts` | OAuth services |
| `src/services/analytics/*.ts` | Analytics |
| `src/services/policyLimits/*.ts` | Policy limits |

### Target Structure
```
internal/infra/services/
├── api/
│   ├── client.go         # API client
│   ├── messages.go       # Message streaming
│   ├── errors.go         # Error types
│   └── retry.go          # Retry logic
├── auth/
│   ├── oauth.go          # OAuth flow
│   ├── tokens.go         # Token management
│   └── refresh.go        # Token refresh
├── analytics/
│   ├── events.go         # Event logging
│   └── growthbook.go     # Feature flags
└── policy/
    └── limits.go         # Policy limits
```

## Implementation Details

### 7.1 API Client
```go
// api/client.go
type Client struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func NewClient(opts ClientOptions) *Client
func (c *Client) StreamMessages(ctx context.Context, params MessageParams) (<-chan MessageEvent, error)
```

### 7.2 Message Streaming
```go
// api/messages.go
func StreamMessages(ctx context.Context, client *Client, params BetaMessageStreamParams) (<-chan SDKMessage, error)
func AccumulateUsage(total *Usage, delta Usage)
func UpdateUsage(usage *Usage, delta BetaMessageDeltaUsage)
```

### 7.3 OAuth
```go
// auth/oauth.go
func StartOAuthFlow(provider string) (*OAuthResult, error)
func RefreshToken(provider string) (string, error)
func RevokeToken(provider string) error
```

### 7.4 Analytics
```go
// analytics/events.go
func LogEvent(name string, properties map[string]any)
func SetUserProperty(key string, value any)

// analytics/growthbook.go
func GetFeatureValue[T any](key string, defaultValue T) T
func IsFeatureEnabled(key string) bool
```

## Parity Checklist
- [ ] API client
- [ ] Message streaming
- [ ] Error handling
- [ ] Retry logic
- [ ] OAuth flow
- [ ] Token management
- [ ] Analytics
- [ ] Feature flags

## Next Checkpoint
- [Checkpoint 8: Services - MCP](./checkpoint-8.md)