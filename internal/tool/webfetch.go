package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// WebFetchTool - Fetches content from URLs
// ============================================================================

// WebFetchTool fetches content from a URL and processes it
type WebFetchTool struct {
	client           *http.Client
	cache            *fetchCache
	domainCheckCache *domainCache
}

// WebFetchOutput represents the output of WebFetchTool
type WebFetchOutput struct {
	Bytes       int64  `json:"bytes"`
	Code        int    `json:"code"`
	CodeText    string `json:"codeText"`
	Result      string `json:"result"`
	DurationMs  int64  `json:"durationMs"`
	URL         string `json:"url"`
	ContentType string `json:"contentType,omitempty"`
}

// RedirectInfo contains information about a redirect to a different host
type RedirectInfo struct {
	Type        string `json:"type"`
	OriginalURL string `json:"originalUrl"`
	RedirectURL string `json:"redirectUrl"`
	StatusCode  int    `json:"statusCode"`
}

// WebFetchTool constants
const (
	maxURLLength         = 2000
	maxHTTPContentLength = 10 * 1024 * 1024 // 10MB
	maxMarkdownLength    = 100_000
	webFetchTimeoutMs    = 60_000
	maxRedirects         = 10
	cacheTTLMs           = 15 * 60 * 1000   // 15 minutes
	maxCacheSizeBytes    = 50 * 1024 * 1024 // 50MB
	domainCheckTTL       = 5 * 60 * 1000    // 5 minutes
)

// CreateWebFetchTool creates a new WebFetchTool instance
func CreateWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{
			Timeout: webFetchTimeoutMs * time.Millisecond,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// We handle redirects manually
				return http.ErrUseLastResponse
			},
		},
		cache:            createFetchCache(),
		domainCheckCache: createDomainCache(),
	}
}

func (WebFetchTool) Name() string { return "WebFetch" }
func (WebFetchTool) Description() string {
	return GetWebFetchDescription()
}
func (WebFetchTool) IsReadOnly(Input) bool { return true }

func (WebFetchTool) ParametersSchema() map[string]any {
	return SchemaObject(map[string]any{
		"url":    SchemaString("The URL to fetch content from"),
		"prompt": SchemaString("The prompt to run on the fetched content"),
	}, "url", "prompt")
}

func (t *WebFetchTool) Call(ctx context.Context, in Input, _ Runtime) (Result, error) {
	start := time.Now()

	// Extract inputs
	webURL := GetString(in, "url")
	prompt := GetString(in, "prompt")

	if strings.TrimSpace(webURL) == "" {
		return Result{}, fmt.Errorf("url is required")
	}

	// Validate URL
	if err := validateWebURL(webURL); err != nil {
		return Result{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Check cache
	if cached, ok := t.cache.Get(webURL); ok {
		return Result{
			Content: WebFetchOutput{
				Bytes:       cached.Bytes,
				Code:        cached.Code,
				CodeText:    cached.CodeText,
				Result:      cached.Content,
				DurationMs:  time.Since(start).Milliseconds(),
				URL:         webURL,
				ContentType: cached.ContentType,
			},
		}, nil
	}

	// Parse URL and upgrade to HTTPS if needed
	parsedURL, err := url.Parse(webURL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "http" {
		parsedURL.Scheme = "https"
		webURL = parsedURL.String()
	}

	// Fetch with redirect handling
	response, err := t.fetchWithPermittedRedirects(ctx, webURL)
	if err != nil {
		return Result{}, err
	}

	// Handle redirect to different host
	if redirect, ok := response.(*RedirectInfo); ok {
		statusText := webStatusTextForCode(redirect.StatusCode)
		message := fmt.Sprintf(`REDIRECT DETECTED: The URL redirects to a different host.

Original URL: %s
Redirect URL: %s
Status: %d %s

To complete your request, I need to fetch content from the redirected URL. Please use WebFetch again with these parameters:
- url: "%s"
- prompt: "%s"`, redirect.OriginalURL, redirect.RedirectURL, redirect.StatusCode, statusText, redirect.RedirectURL, prompt)

		return Result{
			Content: WebFetchOutput{
				Bytes:       int64(len(message)),
				Code:        redirect.StatusCode,
				CodeText:    statusText,
				Result:      message,
				DurationMs:  time.Since(start).Milliseconds(),
				URL:         webURL,
			},
		}, nil
	}

	// Process the fetched content
	fetched := response.(*webFetchedContent)

	// Convert HTML to markdown-like text
	var result string
	if strings.Contains(fetched.ContentType, "text/html") {
		result = htmlToMarkdown(fetched.Content)
	} else {
		result = fetched.Content
	}

	// Truncate if too long
	if len(result) > maxMarkdownLength {
		result = result[:maxMarkdownLength] + "\n\n[Content truncated due to length...]"
	}

	// Apply prompt to content
	if prompt != "" {
		result = webApplyPromptToContent(prompt, result, isWebPreapprovedHost(parsedURL.Hostname(), parsedURL.Path))
	}

	// Cache the result
	t.cache.Set(webURL, &webCacheEntry{
		Content:     result,
		Bytes:       fetched.Bytes,
		Code:        fetched.Code,
		CodeText:    fetched.CodeText,
		ContentType: fetched.ContentType,
	})

	output := WebFetchOutput{
		Bytes:       fetched.Bytes,
		Code:        fetched.Code,
		CodeText:    fetched.CodeText,
		Result:      result,
		DurationMs:  time.Since(start).Milliseconds(),
		URL:         webURL,
		ContentType: fetched.ContentType,
	}

	return Result{Content: output}, nil
}

// validateWebURL validates a URL string
func validateWebURL(rawURL string) error {
	if len(rawURL) > maxURLLength {
		return fmt.Errorf("URL exceeds maximum length of %d characters", maxURLLength)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("could not parse URL: %w", err)
	}

	// Check for username/password in URL (security concern)
	if parsed.User != nil {
		return fmt.Errorf("URLs with username/password are not allowed")
	}

	// Check hostname has at least 2 parts (not internal/privileged)
	hostname := parsed.Hostname()
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return fmt.Errorf("hostname must be a valid public domain")
	}

	return nil
}

// isPermittedWebRedirect checks if a redirect is safe to follow
func isPermittedWebRedirect(originalURL, redirectURL string) bool {
	parsedOriginal, err1 := url.Parse(originalURL)
	parsedRedirect, err2 := url.Parse(redirectURL)

	if err1 != nil || err2 != nil {
		return false
	}

	// Protocol must match
	if parsedRedirect.Scheme != parsedOriginal.Scheme {
		return false
	}

	// Port must match
	if parsedRedirect.Port() != parsedOriginal.Port() {
		return false
	}

	// No username/password in redirect
	if parsedRedirect.User != nil {
		return false
	}

	// Hostname comparison (allow adding/removing www.)
	stripWww := func(hostname string) string {
		return strings.TrimPrefix(hostname, "www.")
	}

	return stripWww(parsedOriginal.Hostname()) == stripWww(parsedRedirect.Hostname())
}

// webFetchedContent represents successfully fetched content
type webFetchedContent struct {
	Content     string
	Bytes       int64
	Code        int
	CodeText    string
	ContentType string
}

func (t *WebFetchTool) fetchWithPermittedRedirects(ctx context.Context, webURL string) (any, error) {
	visited := make(map[string]int)
	currentURL := webURL

	for i := 0; i < maxRedirects; i++ {
		// Check for redirect loops
		if count := visited[currentURL]; count > 0 {
			return nil, fmt.Errorf("redirect loop detected")
		}
		visited[currentURL]++

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "Claude-Code-Go/1.0")
		req.Header.Set("Accept", "text/markdown, text/html, */*")

		resp, err := t.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch URL: %w", err)
		}

		// Handle redirects
		if resp.StatusCode == 301 || resp.StatusCode == 302 ||
			resp.StatusCode == 307 || resp.StatusCode == 308 {
			location := resp.Header.Get("Location")
			resp.Body.Close()

			if location == "" {
				return nil, fmt.Errorf("redirect missing Location header")
			}

			// Resolve relative URLs
			redirectURL, err := url.Parse(location)
			if err != nil {
				return nil, fmt.Errorf("invalid redirect URL: %w", err)
			}
			if !redirectURL.IsAbs() {
				base, _ := url.Parse(currentURL)
				redirectURL = base.ResolveReference(redirectURL)
			}

			redirectURLStr := redirectURL.String()

			// Check if redirect is permitted
			if !isPermittedWebRedirect(currentURL, redirectURLStr) {
				return &RedirectInfo{
					Type:        "redirect",
					OriginalURL: currentURL,
					RedirectURL: redirectURLStr,
					StatusCode:  resp.StatusCode,
				}, nil
			}

			currentURL = redirectURLStr
			continue
		}

		defer resp.Body.Close()

		// Check content length
		if resp.ContentLength > maxHTTPContentLength {
			return nil, fmt.Errorf("content too large (%d bytes)", resp.ContentLength)
		}

		// Read body with limit
		limitedReader := io.LimitReader(resp.Body, maxHTTPContentLength)
		body, err := io.ReadAll(limitedReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		contentType := resp.Header.Get("Content-Type")

		return &webFetchedContent{
			Content:     string(body),
			Bytes:       int64(len(body)),
			Code:        resp.StatusCode,
			CodeText:    resp.Status,
			ContentType: contentType,
		}, nil
	}

	return nil, fmt.Errorf("too many redirects (exceeded %d)", maxRedirects)
}

func webStatusTextForCode(code int) string {
	switch code {
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 307:
		return "Temporary Redirect"
	case 308:
		return "Permanent Redirect"
	default:
		return http.StatusText(code)
	}
}

// ============================================================================
// Preapproved Hosts for WebFetch
// ============================================================================

// webPreapprovedHosts contains hosts that are automatically allowed for WebFetch
var webPreapprovedHosts = map[string]bool{
	// Anthropic
	"platform.claude.com":       true,
	"code.claude.com":           true,
	"modelcontextprotocol.io":   true,
	"agentskills.io":            true,

	// Top Programming Languages
	"docs.python.org":           true,
	"en.cppreference.com":       true,
	"docs.oracle.com":           true,
	"learn.microsoft.com":       true,
	"developer.mozilla.org":     true,
	"go.dev":                    true,
	"pkg.go.dev":                true,
	"www.php.net":               true,
	"docs.swift.org":            true,
	"kotlinlang.org":            true,
	"ruby-doc.org":              true,
	"doc.rust-lang.org":         true,
	"www.typescriptlang.org":    true,

	// Web & JavaScript Frameworks/Libraries
	"react.dev":                 true,
	"angular.io":                true,
	"vuejs.org":                 true,
	"nextjs.org":                true,
	"expressjs.com":             true,
	"nodejs.org":                true,
	"bun.sh":                    true,
	"jquery.com":                true,
	"getbootstrap.com":          true,
	"tailwindcss.com":           true,
	"d3js.org":                  true,
	"threejs.org":               true,
	"redux.js.org":              true,
	"webpack.js.org":            true,
	"jestjs.io":                 true,
	"reactrouter.com":           true,

	// Python Frameworks & Libraries
	"docs.djangoproject.com":    true,
	"flask.palletsprojects.com": true,
	"fastapi.tiangolo.com":      true,
	"pandas.pydata.org":         true,
	"numpy.org":                 true,
	"www.tensorflow.org":        true,
	"pytorch.org":               true,
	"scikit-learn.org":          true,
	"matplotlib.org":            true,
	"requests.readthedocs.io":   true,
	"jupyter.org":               true,

	// PHP Frameworks
	"laravel.com":               true,
	"symfony.com":               true,
	"wordpress.org":             true,

	// Java Frameworks & Libraries
	"docs.spring.io":            true,
	"hibernate.org":             true,
	"tomcat.apache.org":         true,
	"gradle.org":                true,
	"maven.apache.org":          true,

	// .NET & C# Frameworks
	"asp.net":                   true,
	"dotnet.microsoft.com":      true,
	"nuget.org":                 true,
	"blazor.net":                true,

	// Mobile Development
	"reactnative.dev":           true,
	"docs.flutter.dev":          true,
	"developer.apple.com":       true,
	"developer.android.com":     true,

	// Data Science & Machine Learning
	"keras.io":                  true,
	"spark.apache.org":          true,
	"huggingface.co":            true,
	"www.kaggle.com":            true,

	// Databases
	"www.mongodb.com":           true,
	"redis.io":                  true,
	"www.postgresql.org":        true,
	"dev.mysql.com":             true,
	"www.sqlite.org":            true,
	"graphql.org":               true,
	"prisma.io":                 true,

	// Cloud & DevOps
	"docs.aws.amazon.com":       true,
	"cloud.google.com":          true,
	"kubernetes.io":             true,
	"www.docker.com":            true,
	"www.terraform.io":          true,
	"www.ansible.com":           true,
	"vercel.com":                true,
	"docs.netlify.com":          true,
	"devcenter.heroku.com":      true,

	// Testing & Monitoring
	"cypress.io":                true,
	"selenium.dev":              true,

	// Game Development
	"docs.unity.com":            true,
	"docs.unrealengine.com":     true,

	// Other Essential Tools
	"git-scm.com":               true,
	"nginx.org":                 true,
	"httpd.apache.org":          true,

	// Common code hosts
	"github.com":                true,
	"raw.githubusercontent.com": true,
	"gist.github.com":           true,
	"npmjs.com":                 true,
	"pypi.org":                  true,
	"crates.io":                 true,
	"godoc.org":                 true,
	"readthedocs.io":            true,
	"stackoverflow.com":         true,
	"stackexchange.com":         true,
	"wikipedia.org":             true,
	"wikimedia.org":             true,
}

// webPathScopedHosts contains hosts that are preapproved for specific paths only
var webPathScopedHosts = map[string][]string{
	"github.com": {"/anthropics"},
}

// isWebPreapprovedHost checks if a hostname is in the preapproved list
func isWebPreapprovedHost(hostname, pathname string) bool {
	// Check full hostname first
	if webPreapprovedHosts[hostname] {
		return true
	}

	// Check path-scoped hosts
	if prefixes, ok := webPathScopedHosts[hostname]; ok {
		for _, prefix := range prefixes {
			if pathname == prefix || strings.HasPrefix(pathname, prefix+"/") {
				return true
			}
		}
	}

	return false
}

// ============================================================================
// HTML to Markdown Conversion
// ============================================================================

// htmlToMarkdown converts HTML content to a simplified markdown-like format
func htmlToMarkdown(html string) string {
	// Remove scripts, styles, and non-content elements
	html = webRemoveTagContent(html, "script")
	html = webRemoveTagContent(html, "style")
	html = webRemoveTagContent(html, "nav")
	html = webRemoveTagContent(html, "footer")
	html = webRemoveTagContent(html, "aside")
	html = webRemoveTagContent(html, "head")

	// Convert common HTML elements to markdown
	replacements := []struct {
		from string
		to   string
	}{
		{"</p>", "\n\n"},
		{"<br>", "\n"},
		{"<br/>", "\n"},
		{"<br />", "\n"},
		{"</h1>", "\n\n"},
		{"</h2>", "\n\n"},
		{"</h3>", "\n\n"},
		{"</h4>", "\n\n"},
		{"</h5>", "\n\n"},
		{"</h6>", "\n\n"},
		{"</li>", "\n"},
		{"</tr>", "\n"},
		{"</td>", " "},
		{"</th>", " "},
		{"<li>", "- "},
		{"</a>", ""},
		{"</div>", "\n"},
		{"</span>", ""},
		{"</article>", "\n\n"},
		{"</section>", "\n\n"},
	}

	for _, r := range replacements {
		html = strings.ReplaceAll(html, r.from, r.to)
	}

	// Handle headers with markdown syntax
	headerPatterns := []struct {
		tag      string
		markdown string
	}{
		{"<h1>", "# "},
		{"<h2>", "## "},
		{"<h3>", "### "},
		{"<h4>", "#### "},
		{"<h5>", "##### "},
		{"<h6>", "###### "},
	}

	for _, h := range headerPatterns {
		html = strings.ReplaceAll(html, h.tag, h.markdown)
		html = strings.ReplaceAll(html, strings.ToUpper(h.tag), h.markdown)
	}

	// Remove remaining HTML tags
	var result strings.Builder
	inTag := false
	inEntity := false
	for _, ch := range html {
		switch {
		case ch == '<':
			inTag = true
		case ch == '>':
			inTag = false
		case ch == '&':
			inEntity = true
			result.WriteRune(ch)
		case ch == ';':
			if inEntity {
				inEntity = false
			}
			result.WriteRune(ch)
		case !inTag:
			result.WriteRune(ch)
		}
	}

	text := result.String()

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Clean up whitespace
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "\r\n", "\n")

	// Collapse multiple spaces
	spacePattern := regexp.MustCompile(`[ \t]+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// Collapse multiple blank lines
	blankLinePattern := regexp.MustCompile(`\n{3,}`)
	text = blankLinePattern.ReplaceAllString(text, "\n\n")

	// Trim lines
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	return strings.TrimSpace(text)
}

// webRemoveTagContent removes content between opening and closing tags
func webRemoveTagContent(html, tag string) string {
	lowerHTML := strings.ToLower(html)
	lowerTag := strings.ToLower(tag)

	openTag := "<" + lowerTag
	closeTag := "</" + lowerTag + ">"

	for {
		start := strings.Index(lowerHTML, openTag)
		if start == -1 {
			break
		}
		end := strings.Index(lowerHTML[start:], closeTag)
		if end == -1 {
			break
		}
		end += start + len(closeTag)
		html = html[:start] + html[end:]
		lowerHTML = lowerHTML[:start] + lowerHTML[end:]
	}

	return html
}

// ============================================================================
// Prompt Processing
// ============================================================================

func webApplyPromptToContent(prompt, content string, isPreapprovedDomain bool) string {
	var guidelines string
	if isPreapprovedDomain {
		guidelines = `Provide a concise response based on the content above. Include relevant details, code examples, and documentation excerpts as needed.`
	} else {
		guidelines = `Provide a concise response based only on the content above. In your response:
- Enforce a strict 125-character maximum for quotes from any source document. Open Source Software is ok as long as we respect the license.
- Use quotation marks for exact language from articles; any language outside of the quotation should never be word-for-word the same.
- You are not a lawyer and never comment on the legality of your own prompts and responses.
- Never produce or reproduce exact song lyrics.`
	}

	return fmt.Sprintf(`
Web page content:
---
%s
---

%s

%s
`, content, prompt, guidelines)
}

// ============================================================================
// Cache Implementation
// ============================================================================

type webCacheEntry struct {
	Content     string
	Bytes       int64
	Code        int
	CodeText    string
	ContentType string
	expiresAt   time.Time
	size        int
}

type fetchCache struct {
	mu        sync.RWMutex
	entries   map[string]*webCacheEntry
	totalSize int64
}

func createFetchCache() *fetchCache {
	return &fetchCache{
		entries: make(map[string]*webCacheEntry),
	}
}

func (c *fetchCache) Get(url string) (*webCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[url]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry, true
}

func (c *fetchCache) Set(url string, entry *webCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove old entry if exists
	if old, ok := c.entries[url]; ok {
		c.totalSize -= int64(old.size)
		delete(c.entries, url)
	}

	// Evict entries if over size limit
	entry.size = len(entry.Content)
	if entry.size < 1 {
		entry.size = 1
	}
	entry.expiresAt = time.Now().Add(cacheTTLMs * time.Millisecond)

	for c.totalSize+int64(entry.size) > maxCacheSizeBytes && len(c.entries) > 0 {
		// Remove oldest entry (simple eviction strategy)
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.expiresAt
			}
		}
		if oldestKey != "" {
			c.totalSize -= int64(c.entries[oldestKey].size)
			delete(c.entries, oldestKey)
		}
	}

	c.entries[url] = entry
	c.totalSize += int64(entry.size)
}

type domainCache struct {
	mu      sync.RWMutex
	allowed map[string]time.Time
}

func createDomainCache() *domainCache {
	return &domainCache{
		allowed: make(map[string]time.Time),
	}
}

func (c *domainCache) IsAllowed(domain string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if t, ok := c.allowed[domain]; ok {
		return time.Now().Before(t)
	}
	return false
}

func (c *domainCache) SetAllowed(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.allowed[domain] = time.Now().Add(domainCheckTTL * time.Millisecond)
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