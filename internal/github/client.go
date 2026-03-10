package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// GitHubClient defines all GitHub operations required by Loom.
// It extends the base Client interface (Ping) with issue, PR,
// review, tag, and release operations.
type GitHubClient interface {
	Client
	CreateIssue(ctx context.Context, title, body string, labels []string) (*Issue, error)
	AddComment(ctx context.Context, issueNumber int, body string) error
	CloseIssue(ctx context.Context, issueNumber int) error
	ListPRs(ctx context.Context, branch string) ([]*PR, error)
	GetPR(ctx context.Context, prNumber int) (*PR, error)
	GetCheckRuns(ctx context.Context, sha string) ([]*CheckRun, error)
	MergePR(ctx context.Context, prNumber int, commitMessage string) error
	RequestReview(ctx context.Context, prNumber int, reviewer string) error
	GetReviewStatus(ctx context.Context, prNumber int) (string, error)
	CreateTag(ctx context.Context, tagName, sha string) (*Tag, error)
	CreateRelease(ctx context.Context, tagName, name, body string) (*Release, error)
}

// HTTPClient is the concrete net/http-based implementation of GitHubClient.
type HTTPClient struct {
	baseURL    string
	token      string
	owner      string
	repo       string
	httpClient *http.Client
	// retryBase is the base duration for exponential back-off on HTTP 429
	// responses. Defaults to 100 ms. Override via WithRetryBase option.
	retryBase time.Duration
}

// Option configures an HTTPClient.
type Option func(*HTTPClient)

// WithRetryBase sets the base duration for exponential back-off on HTTP 429
// responses. Useful in tests to set a very short duration (e.g. 1 ms).
func WithRetryBase(d time.Duration) Option {
	return func(c *HTTPClient) { c.retryBase = d }
}

// NewHTTPClient constructs an HTTPClient pointed at baseURL
// (e.g. "https://api.github.com").
func NewHTTPClient(baseURL, token, owner, repo string, opts ...Option) *HTTPClient {
	c := &HTTPClient{
		baseURL:    baseURL,
		token:      token,
		owner:      owner,
		repo:       repo,
		httpClient: &http.Client{},
		retryBase:  100 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *HTTPClient) repoBase() string {
	return fmt.Sprintf("%s/repos/%s/%s", c.baseURL, c.owner, c.repo)
}

// ── internal API response shapes ──────────────────────────────────────────

type prAPIResponse struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Draft  bool   `json:"draft"`
	State  string `json:"state"`
	Head   struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func (p *prAPIResponse) toPR() *PR {
	return &PR{
		Number:  p.Number,
		Title:   p.Title,
		Draft:   p.Draft,
		HeadSHA: p.Head.SHA,
		State:   p.State,
	}
}

type checkRunsEnvelope struct {
	CheckRuns []*CheckRun `json:"check_runs"`
}

type tagAPIResponse struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

// ── rate-limit helpers ────────────────────────────────────────────────────

type rateLimitInfo struct {
	limit     int
	remaining int
	reset     time.Time
}

func parseRateLimitHeaders(resp *http.Response) rateLimitInfo {
	limitStr := resp.Header.Get("X-RateLimit-Limit")
	remainingStr := resp.Header.Get("X-RateLimit-Remaining")
	resetStr := resp.Header.Get("X-RateLimit-Reset")

	if limitStr == "" || remainingStr == "" {
		return rateLimitInfo{}
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit == 0 {
		return rateLimitInfo{}
	}

	remaining, err := strconv.Atoi(remainingStr)
	if err != nil {
		return rateLimitInfo{}
	}

	var reset time.Time
	if resetStr != "" {
		unix, parseErr := strconv.ParseInt(resetStr, 10, 64)
		if parseErr == nil {
			reset = time.Unix(unix, 0)
		}
	}

	return rateLimitInfo{limit: limit, remaining: remaining, reset: reset}
}

// ── doRequest ─────────────────────────────────────────────────────────────

const maxRetries = 3

// doRequest executes method against rawURL, JSON-encoding reqBody when
// non-nil. It decodes a successful response into v (when non-nil), handles
// X-RateLimit-* headers, and retries on HTTP 429 with exponential back-off
// up to maxRetries times. Rate-limit warnings are logged for successful
// responses, but retries are never issued for a successful response to avoid
// duplicating state-mutating operations (CreateIssue, MergePR, etc.).
func (c *HTTPClient) doRequest(ctx context.Context, method, rawURL string, reqBody, v interface{}) error {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("execute request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response body: %w", readErr)
		}

		// HTTP 429: exponential back-off and retry.
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < maxRetries {
				backoff := time.Duration(1<<uint(attempt)) * c.retryBase
				slog.Warn("HTTP 429 received, backing off",
					"backoff", backoff, "attempt", attempt)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return fmt.Errorf("context done during 429 back-off: %w", ctx.Err())
				}
				continue
			}
			return fmt.Errorf("HTTP 429: rate limited after %d retries: %s",
				maxRetries, string(respBody))
		}

		// Other HTTP errors.
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		// Successful response: inspect rate-limit headers for warnings only.
		// We never retry a successful response even when remaining == 0, since
		// retrying a state-mutating request (CreateIssue, MergePR, …) could
		// create duplicate resources.
		rl := parseRateLimitHeaders(resp)
		if rl.limit > 0 {
			if rl.remaining == 0 {
				slog.Warn("GitHub rate limit exhausted after successful request",
					"reset", rl.reset)
			} else if rl.remaining*10 < rl.limit {
				slog.Warn("GitHub rate limit low",
					"remaining", rl.remaining, "limit", rl.limit)
			}
		}

		// Decode and return.
		if v != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, v); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
		return nil
	}

	return fmt.Errorf("max retries exceeded")
}

// ── Ping ──────────────────────────────────────────────────────────────────

// Ping verifies connectivity to the GitHub API.
func (c *HTTPClient) Ping(ctx context.Context) error {
	if err := c.doRequest(ctx, http.MethodGet, c.repoBase(), nil, nil); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// ── Issue operations ──────────────────────────────────────────────────────

// CreateIssue creates a new GitHub issue and returns the created resource.
func (c *HTTPClient) CreateIssue(ctx context.Context, title, body string, labels []string) (*Issue, error) {
	reqBody := struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels,omitempty"`
	}{Title: title, Body: body, Labels: labels}

	var issue Issue
	if err := c.doRequest(ctx, http.MethodPost,
		c.repoBase()+"/issues", reqBody, &issue); err != nil {
		return nil, fmt.Errorf("creating issue: %w", err)
	}
	return &issue, nil
}

// AddComment posts a comment on an issue or pull request.
func (c *HTTPClient) AddComment(ctx context.Context, issueNumber int, body string) error {
	reqBody := struct {
		Body string `json:"body"`
	}{Body: body}

	endpoint := fmt.Sprintf("%s/issues/%d/comments", c.repoBase(), issueNumber)
	if err := c.doRequest(ctx, http.MethodPost, endpoint, reqBody, nil); err != nil {
		return fmt.Errorf("adding comment: %w", err)
	}
	return nil
}

// CloseIssue closes an issue by patching its state to "closed".
func (c *HTTPClient) CloseIssue(ctx context.Context, issueNumber int) error {
	reqBody := struct {
		State string `json:"state"`
	}{State: "closed"}

	endpoint := fmt.Sprintf("%s/issues/%d", c.repoBase(), issueNumber)
	if err := c.doRequest(ctx, http.MethodPatch, endpoint, reqBody, nil); err != nil {
		return fmt.Errorf("closing issue: %w", err)
	}
	return nil
}

// ── PR operations ─────────────────────────────────────────────────────────

// ListPRs returns open pull requests filtered by head branch.
func (c *HTTPClient) ListPRs(ctx context.Context, branch string) ([]*PR, error) {
	params := url.Values{}
	params.Set("head", branch)
	params.Set("state", "open")
	pullsURL := fmt.Sprintf("%s/pulls?%s", c.repoBase(), params.Encode())

	var raw []prAPIResponse
	if err := c.doRequest(ctx, http.MethodGet, pullsURL, nil, &raw); err != nil {
		return nil, fmt.Errorf("listing PRs: %w", err)
	}

	prs := make([]*PR, len(raw))
	for i := range raw {
		prs[i] = raw[i].toPR()
	}
	return prs, nil
}

// GetPR fetches a single pull request by number.
func (c *HTTPClient) GetPR(ctx context.Context, prNumber int) (*PR, error) {
	endpoint := fmt.Sprintf("%s/pulls/%d", c.repoBase(), prNumber)

	var raw prAPIResponse
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &raw); err != nil {
		return nil, fmt.Errorf("getting PR: %w", err)
	}
	return raw.toPR(), nil
}

// GetCheckRuns returns all check runs for a given commit SHA.
func (c *HTTPClient) GetCheckRuns(ctx context.Context, sha string) ([]*CheckRun, error) {
	endpoint := fmt.Sprintf("%s/commits/%s/check-runs", c.repoBase(), sha)

	var envelope checkRunsEnvelope
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &envelope); err != nil {
		return nil, fmt.Errorf("getting check runs: %w", err)
	}
	return envelope.CheckRuns, nil
}

// MergePR merges a pull request using the provided commit message.
func (c *HTTPClient) MergePR(ctx context.Context, prNumber int, commitMessage string) error {
	reqBody := struct {
		CommitMessage string `json:"commit_message"`
	}{CommitMessage: commitMessage}

	endpoint := fmt.Sprintf("%s/pulls/%d/merge", c.repoBase(), prNumber)
	if err := c.doRequest(ctx, http.MethodPut, endpoint, reqBody, nil); err != nil {
		return fmt.Errorf("merging PR: %w", err)
	}
	return nil
}

// ── Review operations ─────────────────────────────────────────────────────

// RequestReview requests a review from reviewer on the given PR.
func (c *HTTPClient) RequestReview(ctx context.Context, prNumber int, reviewer string) error {
	reqBody := struct {
		Reviewers []string `json:"reviewers"`
	}{Reviewers: []string{reviewer}}

	endpoint := fmt.Sprintf("%s/pulls/%d/requested_reviewers", c.repoBase(), prNumber)
	if err := c.doRequest(ctx, http.MethodPost, endpoint, reqBody, nil); err != nil {
		return fmt.Errorf("requesting review: %w", err)
	}
	return nil
}

// GetReviewStatus returns the aggregated review state for a PR.
// CHANGES_REQUESTED beats APPROVED beats PENDING.
func (c *HTTPClient) GetReviewStatus(ctx context.Context, prNumber int) (string, error) {
	endpoint := fmt.Sprintf("%s/pulls/%d/reviews", c.repoBase(), prNumber)

	var reviews []Review
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &reviews); err != nil {
		return "", fmt.Errorf("getting review status: %w", err)
	}

	status := "PENDING"
	for _, r := range reviews {
		switch r.State {
		case "CHANGES_REQUESTED":
			return "CHANGES_REQUESTED", nil
		case "APPROVED":
			status = "APPROVED"
		}
	}
	return status, nil
}

// ── Tag / Release operations ──────────────────────────────────────────────

// CreateTag creates a lightweight Git tag ref pointing at sha.
func (c *HTTPClient) CreateTag(ctx context.Context, tagName, sha string) (*Tag, error) {
	reqBody := struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	}{
		Ref: "refs/tags/" + tagName,
		SHA: sha,
	}

	var raw tagAPIResponse
	if err := c.doRequest(ctx, http.MethodPost,
		c.repoBase()+"/git/refs", reqBody, &raw); err != nil {
		return nil, fmt.Errorf("creating tag: %w", err)
	}
	return &Tag{Ref: raw.Ref, SHA: raw.Object.SHA}, nil
}

// CreateRelease creates a GitHub release associated with tagName.
func (c *HTTPClient) CreateRelease(ctx context.Context, tagName, name, body string) (*Release, error) {
	reqBody := struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
	}{TagName: tagName, Name: name, Body: body}

	var release Release
	if err := c.doRequest(ctx, http.MethodPost,
		c.repoBase()+"/releases", reqBody, &release); err != nil {
		return nil, fmt.Errorf("creating release: %w", err)
	}
	return &release, nil
}
