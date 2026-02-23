package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxConcurrent = 5
	defaultPageSize      = 50
	maxRetries           = 3
)

// JiraClient handles HTTP communication with the Jira REST API.
type JiraClient struct {
	httpClient    *http.Client
	baseURL       string
	apiVersion    string
	authHeader    string
	maxConcurrent int
}

// NewJiraClient creates a new JiraClient with the given settings and secrets.
func NewJiraClient(settings JiraSettings, secrets map[string]string) *JiraClient {
	var authHeader string
	switch settings.AuthType {
	case "basic":
		creds := base64.StdEncoding.EncodeToString(
			[]byte(settings.Username + ":" + secrets["apiToken"]))
		authHeader = "Basic " + creds
	case "bearer":
		authHeader = "Bearer " + secrets["bearerToken"]
	}

	baseURL := strings.TrimRight(settings.URL, "/")

	return &JiraClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:       baseURL,
		apiVersion:    settings.APIVersion,
		authHeader:    authHeader,
		maxConcurrent: defaultMaxConcurrent,
	}
}

// apiPath returns the full API path for a given endpoint.
func (c *JiraClient) apiPath(endpoint string) string {
	return fmt.Sprintf("%s/rest/api/%s%s", c.baseURL, c.apiVersion, endpoint)
}

// doRequest performs an HTTP request with authentication, rate limit handling, and retries.
func (c *JiraClient) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	var lastErr error
	for attempt := range maxRetries {
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute request: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			waitDuration := retryAfterDuration(resp, attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitDuration):
				lastErr = fmt.Errorf("rate limited (429)")
				continue
			}
		}

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("jira API error %d: %s", resp.StatusCode, string(bodyBytes))
		}

		return resp, nil
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// retryAfterDuration computes the wait time from Retry-After header or exponential backoff.
func retryAfterDuration(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if seconds, err := strconv.Atoi(ra); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	// Exponential backoff with jitter
	base := math.Pow(2, float64(attempt)) * 1000 // milliseconds
	jitter := rand.Float64() * 500                // up to 500ms jitter
	return time.Duration(base+jitter) * time.Millisecond
}

// Get performs a GET request and returns the response body.
func (c *JiraClient) Get(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	u := c.apiPath(endpoint)
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	resp, err := c.doRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Post performs a POST request with a JSON body.
func (c *JiraClient) Post(ctx context.Context, endpoint string, body io.Reader) ([]byte, error) {
	u := c.apiPath(endpoint)
	resp, err := c.doRequest(ctx, http.MethodPost, u, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GetMyself calls /rest/api/{v}/myself for health checking.
func (c *JiraClient) GetMyself(ctx context.Context) (*JiraUser, error) {
	data, err := c.Get(ctx, "/myself", nil)
	if err != nil {
		return nil, err
	}
	var user JiraUser
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("parse myself response: %w", err)
	}
	return &user, nil
}

// SearchIssues performs a paginated JQL search, fetching all matching issues.
func (c *JiraClient) SearchIssues(ctx context.Context, jql string, fields []string, expand []string, maxResults int) ([]JiraIssue, error) {
	// First page to learn total
	firstPage, err := c.searchPage(ctx, jql, fields, expand, 0, defaultPageSize)
	if err != nil {
		return nil, err
	}

	total := firstPage.Total
	if maxResults > 0 && total > maxResults {
		total = maxResults
	}

	if total <= defaultPageSize {
		return firstPage.Issues[:min(len(firstPage.Issues), total)], nil
	}

	// Fetch remaining pages concurrently
	allIssues := make([]JiraIssue, 0, total)
	allIssues = append(allIssues, firstPage.Issues...)

	remaining := total - defaultPageSize
	var pages []int
	for startAt := defaultPageSize; remaining > 0; startAt += defaultPageSize {
		pages = append(pages, startAt)
		remaining -= defaultPageSize
	}

	type pageResult struct {
		startAt int
		issues  []JiraIssue
		err     error
	}

	results := make(chan pageResult, len(pages))
	sem := make(chan struct{}, c.maxConcurrent)

	var wg sync.WaitGroup
	for _, startAt := range pages {
		wg.Add(1)
		go func(sa int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := c.searchPage(ctx, jql, fields, expand, sa, defaultPageSize)
			if err != nil {
				results <- pageResult{startAt: sa, err: err}
				return
			}
			results <- pageResult{startAt: sa, issues: resp.Issues}
		}(startAt)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	pageMap := make(map[int][]JiraIssue)
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("fetch page at %d: %w", r.startAt, r.err)
		}
		pageMap[r.startAt] = r.issues
	}

	for _, startAt := range pages {
		allIssues = append(allIssues, pageMap[startAt]...)
	}

	if len(allIssues) > total {
		allIssues = allIssues[:total]
	}

	return allIssues, nil
}

// searchPage fetches a single page of search results.
func (c *JiraClient) searchPage(ctx context.Context, jql string, fields []string, expand []string, startAt, maxResults int) (*JiraSearchResponse, error) {
	params := url.Values{
		"jql":        {jql},
		"startAt":    {strconv.Itoa(startAt)},
		"maxResults": {strconv.Itoa(maxResults)},
	}
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}
	if len(expand) > 0 {
		params.Set("expand", strings.Join(expand, ","))
	}

	data, err := c.Get(ctx, "/search", params)
	if err != nil {
		return nil, err
	}

	var resp JiraSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}
	return &resp, nil
}

// GetIssueChangelog fetches the changelog for a single issue.
func (c *JiraClient) GetIssueChangelog(ctx context.Context, issueKey string) ([]JiraChangelogHistory, error) {
	endpoint := fmt.Sprintf("/issue/%s/changelog", issueKey)
	data, err := c.Get(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Values []JiraChangelogHistory `json:"values"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse changelog response: %w", err)
	}
	return resp.Values, nil
}

// GetIssueWorklogs fetches worklogs for a single issue.
func (c *JiraClient) GetIssueWorklogs(ctx context.Context, issueKey string) ([]JiraWorklog, error) {
	endpoint := fmt.Sprintf("/issue/%s/worklog", issueKey)
	data, err := c.Get(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp JiraWorklogResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse worklog response: %w", err)
	}
	return resp.Worklogs, nil
}
