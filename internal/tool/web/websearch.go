package web


import (
	"claude-go/internal/tool"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// WebSearchTool - Searches the web for information
// ============================================================================

// WebSearchTool searches the web for information
type WebSearchTool struct {
	client *http.Client
}

// WebSearchResult represents a single search result
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// WebSearchOutput represents the output of WebSearchTool
type WebSearchOutput struct {
	Query           string            `json:"query"`
	Results         []WebSearchResult `json:"results"`
	DurationSeconds float64           `json:"durationSeconds"`
}

// WebSearchTool creates a new WebSearchTool instance
func CreateWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (WebSearchTool) Name() string          { return "WebSearch" }
func (WebSearchTool) Description() string   { return "search the web for current information" }
func (WebSearchTool) IsReadOnly(tool.Input) bool { return true }

func (WebSearchTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "The search query to use",
			"minLength":   2,
		},
		"allowed_domains": map[string]any{
			"type":        "array",
			"description": "Only include search results from these domains",
			"items": map[string]any{
				"type": "string",
			},
		},
		"blocked_domains": map[string]any{
			"type":        "array",
			"description": "Never include search results from these domains",
			"items": map[string]any{
				"type": "string",
			},
		},
	}, "query")
}

func (t *WebSearchTool) Call(ctx context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	start := time.Now()

	query := tool.GetString(in, "query")
	allowedDomains := tool.GetStringSlice(in, "allowed_domains")
	blockedDomains := tool.GetStringSlice(in, "blocked_domains")

	// Validate input
	if strings.TrimSpace(query) == "" {
		return tool.Result{}, fmt.Errorf("query is required")
	}

	if len(query) < 2 {
		return tool.Result{}, fmt.Errorf("query must be at least 2 characters")
	}

	// Cannot specify both allowed and blocked domains
	if len(allowedDomains) > 0 && len(blockedDomains) > 0 {
		return tool.Result{}, fmt.Errorf("cannot specify both allowed_domains and blocked_domains in the same request")
	}

	// Perform search
	results, err := t.performSearch(ctx, query, allowedDomains, blockedDomains)
	if err != nil {
		return tool.Result{}, fmt.Errorf("search failed: %w", err)
	}

	duration := time.Since(start).Seconds()

	// Build formatted output
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Web search results for query: \"%s\"\n\n", query))

	if len(results) == 0 {
		content.WriteString("No results found.\n")
	} else {
		for i, result := range results {
			content.WriteString(fmt.Sprintf("## tool.Result %d\n", i+1))
			content.WriteString(fmt.Sprintf("**Title:** %s\n", result.Title))
			content.WriteString(fmt.Sprintf("**URL:** [%s](%s)\n", result.URL, result.URL))
			if result.Snippet != "" {
				content.WriteString(fmt.Sprintf("**Snippet:** %s\n", result.Snippet))
			}
			content.WriteString("\n")
		}

		// Add sources section
		content.WriteString("Sources:\n")
		for _, result := range results {
			content.WriteString(fmt.Sprintf("- [%s](%s)\n", result.Title, result.URL))
		}
	}

	content.WriteString("\nREMINDER: You MUST include the sources above in your response using markdown hyperlinks.")

	output := WebSearchOutput{
		Query:           query,
		Results:         results,
		DurationSeconds: duration,
	}

	return tool.Result{
		Content: content.String(),
		Meta: map[string]any{
			"query":           output.Query,
			"results":         output.Results,
			"durationSeconds": output.DurationSeconds,
		},
	}, nil
}

// performSearch performs the web search using DuckDuckGo
func (t *WebSearchTool) performSearch(ctx context.Context, query string, allowedDomains, blockedDomains []string) ([]WebSearchResult, error) {
	// Build search URL with domain filters
	var searchQuery string
	if len(allowedDomains) > 0 {
		// Add site: filters for allowed domains
		siteFilters := make([]string, len(allowedDomains))
		for i, domain := range allowedDomains {
			siteFilters[i] = "site:" + domain
		}
		searchQuery = query + " " + strings.Join(siteFilters, " OR ")
	} else if len(blockedDomains) > 0 {
		// Add -site: filters for blocked domains
		for _, domain := range blockedDomains {
			searchQuery += " -site:" + domain
		}
		searchQuery = query + searchQuery
	} else {
		searchQuery = query
	}

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(searchQuery))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Claude-Go/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	results := parseDuckDuckGoHTML(string(body))

	// Apply domain filtering
	results = filterByDomains(results, allowedDomains, blockedDomains)

	// Limit results to 10
	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}

// filterByDomains applies domain filtering to search results
func filterByDomains(results []WebSearchResult, allowedDomains, blockedDomains []string) []WebSearchResult {
	if len(allowedDomains) == 0 && len(blockedDomains) == 0 {
		return results
	}

	filtered := make([]WebSearchResult, 0, len(results))
	for _, result := range results {
		domain := extractDomain(result.URL)

		// Check blocked domains first
		if len(blockedDomains) > 0 {
			blocked := false
			for _, b := range blockedDomains {
				if domain == b || strings.HasSuffix(domain, "."+b) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
		}

		// Check allowed domains
		if len(allowedDomains) > 0 {
			allowed := false
			for _, a := range allowedDomains {
				if domain == a || strings.HasSuffix(domain, "."+a) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		filtered = append(filtered, result)
	}

	return filtered
}

func extractDomain(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// parseDuckDuckGoHTML parses DuckDuckGo HTML search results
func parseDuckDuckGoHTML(html string) []WebSearchResult {
	var results []WebSearchResult

	// Find all result links using regex
	resultPattern := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)

	matches := resultPattern.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		resultURL := match[1]
		title := strings.TrimSpace(match[2])

		// DuckDuckGo uses redirect URLs - extract actual URL
		if strings.Contains(resultURL, "uddg=") {
			parsed, err := url.Parse(resultURL)
			if err == nil {
				if uddg := parsed.Query().Get("uddg"); uddg != "" {
					resultURL = uddg
				}
			}
		}

		if resultURL == "" || title == "" {
			continue
		}

		results = append(results, WebSearchResult{
			Title: title,
			URL:   resultURL,
		})
	}

	// Extract snippets separately (if available)
	snippetPattern := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>([^<]*)</a>`)
	snippetMatches := snippetPattern.FindAllStringSubmatch(html, -1)
	for i, match := range snippetMatches {
		if len(match) < 2 {
			continue
		}
		if i < len(results) {
			results[i].Snippet = strings.TrimSpace(match[1])
		}
	}

	return results
}

// ============================================================================
// Tool Registration
// ============================================================================

// RegisterWebTools registers web-related tools to the given registry
func RegisterWebTools(r *tool.Registry) {
	r.Register(CreateWebSearchTool())
}

// ============================================================================
// Tool Description Prompts
// ============================================================================

// GetWebSearchDescription returns the tool description for WebSearch
func GetWebSearchDescription(currentMonthYear string) string {
	return fmt.Sprintf(`
- Allows Claude to search the web and use the results to inform responses
- Provides up-to-date information for current events and recent data
- Returns search result information formatted as search result blocks, including links as markdown hyperlinks
- Use this tool for accessing information beyond Claude's knowledge cutoff
- Searches are performed automatically within a single API call

CRITICAL REQUIREMENT - You MUST follow this:
  - After answering the user's question, you MUST include a "Sources:" section at the end of your response
  - In the Sources section, list all relevant URLs from the search results as markdown hyperlinks: [Title](URL)
  - This is MANDATORY - never skip including sources in your response
  - Example format:

    [Your answer here]

    Sources:
    - [Source Title 1](https://example.com/1)
    - [Source Title 2](https://example.com/2)

Usage notes:
  - Domain filtering is supported to include or block specific websites
  - Web search is only available in the US

IMPORTANT - Use the correct year in search queries:
  - The current month is %s. You MUST use this year when searching for recent information, documentation, or current events.
  - Example: If the user asks for "latest React docs", search for "React documentation" with the current year, NOT last year
`, currentMonthYear)
}

// GetWebFetchDescription returns the tool description for WebFetch
func GetWebFetchDescription() string {
	return `
- Fetches content from a specified URL and processes it using an AI model
- Takes a URL and a prompt as input
- Fetches the URL content, converts HTML to markdown
- Processes the content with the prompt using a small, fast model
- Returns the model's response about the content
- Use this tool when you need to retrieve and analyze web content

Usage notes:
  - IMPORTANT: If an MCP-provided web fetch tool is available, prefer using that tool instead of this one, as it may have fewer restrictions.
  - The URL must be a fully-formed valid URL
  - HTTP URLs will be automatically upgraded to HTTPS
  - The prompt should describe what information you want to extract from the page
  - This tool is read-only and does not modify any files
  - Results may be summarized if the content is very large
  - Includes a self-cleaning 15-minute cache for faster responses when repeatedly accessing the same URL
  - When a URL redirects to a different host, the tool will inform you and provide the redirect URL in a special format. You should then make a new WebFetch request with the redirect URL to fetch the content.
  - For GitHub URLs, prefer using the gh CLI via Bash instead (e.g., gh pr view, gh issue view, gh api).
`
}
