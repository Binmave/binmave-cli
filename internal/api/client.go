package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Binmave/binmave-cli/internal/auth"
	"github.com/Binmave/binmave-cli/internal/config"
)

// Client is the API client for the Binmave backend
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      *auth.TokenInfo
}

// NewClient creates a new API client
func NewClient() (*Client, error) {
	token, err := auth.GetValidToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	if token == nil {
		return nil, fmt.Errorf("not logged in. Run 'binmave login' first")
	}

	return &Client{
		baseURL: config.GetServer(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}, nil
}

// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token.AccessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// Try to refresh token
		newToken, err := auth.RefreshAccessToken(c.token.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("unauthorized and token refresh failed: %w", err)
		}
		c.token = newToken

		// Retry the request
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token.AccessToken))
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
	}

	return resp, nil
}

// decodeResponse decodes a JSON response
func decodeResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// ListAgents returns a list of agents
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	// Use the filterable endpoint with no filters to get all agents
	params := url.Values{
		"take": {"1000"}, // Get up to 1000 agents
	}

	resp, err := c.doRequest(ctx, "GET", "/api/agents/filterable?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result LoadResult
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	// Convert the data to []Agent
	data, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}

	var agents []Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, err
	}

	return agents, nil
}

// GetAgentStats returns agent statistics
func (c *Client) GetAgentStats(ctx context.Context) (*AgentStats, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/agents/stats", nil)
	if err != nil {
		return nil, err
	}

	var stats AgentStats
	if err := decodeResponse(resp, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// ListScripts returns a list of scripts
func (c *Client) ListScripts(ctx context.Context) ([]Script, error) {
	params := url.Values{
		"take": {"1000"}, // Get up to 1000 scripts
	}

	resp, err := c.doRequest(ctx, "GET", "/api/scripts/filterable?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result LoadResult
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	// Convert the data to []Script
	data, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}

	var scripts []Script
	if err := json.Unmarshal(data, &scripts); err != nil {
		return nil, err
	}

	return scripts, nil
}

// GetScript returns a single script by ID
func (c *Client) GetScript(ctx context.Context, id int) (*Script, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/scripts/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var script Script
	if err := decodeResponse(resp, &script); err != nil {
		return nil, err
	}

	return &script, nil
}

// ExecuteScript executes a script on the specified agents
func (c *Client) ExecuteScript(ctx context.Context, scriptID int, req ExecuteRequest) (*ExecuteResponse, error) {
	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/scripts/%d/execute", scriptID), req)
	if err != nil {
		return nil, err
	}

	var result ExecuteResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetExecution returns details of an execution
func (c *Client) GetExecution(ctx context.Context, id string) (*Execution, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/scripts/executions/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var execution Execution
	if err := decodeResponse(resp, &execution); err != nil {
		return nil, err
	}

	return &execution, nil
}

// GetExecutionStatus returns the current status of an execution
func (c *Client) GetExecutionStatus(ctx context.Context, id string) (*ExecutionStatus, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/scripts/executions/%s/status", id), nil)
	if err != nil {
		return nil, err
	}

	var status ExecutionStatus
	if err := decodeResponse(resp, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetExecutionResults returns the results of an execution
func (c *Client) GetExecutionResults(ctx context.Context, id string, page, pageSize int) (*ExecutionResultsPage, error) {
	params := url.Values{
		"page":     {fmt.Sprintf("%d", page)},
		"pageSize": {fmt.Sprintf("%d", pageSize)},
	}

	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/scripts/executions/%s/results?%s", id, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	var results ExecutionResultsPage
	if err := decodeResponse(resp, &results); err != nil {
		return nil, err
	}

	return &results, nil
}

// ListRecentExecutions returns recent executions
func (c *Client) ListRecentExecutions(ctx context.Context, limit int) ([]ExecutionListItem, error) {
	params := url.Values{
		"limit": {fmt.Sprintf("%d", limit)},
	}

	resp, err := c.doRequest(ctx, "GET", "/api/scripts/executions/recent?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var executions []ExecutionListItem
	if err := decodeResponse(resp, &executions); err != nil {
		return nil, err
	}

	return executions, nil
}

// GetAllExecutionResults returns all results for an execution (fetches all pages)
func (c *Client) GetAllExecutionResults(ctx context.Context, id string) ([]ExecutionResult, error) {
	var allResults []ExecutionResult
	page := 1
	pageSize := 100

	for {
		results, err := c.GetExecutionResults(ctx, id, page, pageSize)
		if err != nil {
			return nil, err
		}

		allResults = append(allResults, results.Results...)

		// Check if we have all results
		if len(allResults) >= results.TotalCount || len(results.Results) < pageSize {
			break
		}

		page++
	}

	return allResults, nil
}

// GetExecutionErrors returns all error results for an execution
func (c *Client) GetExecutionErrors(ctx context.Context, id string) ([]ExecutionResult, error) {
	allResults, err := c.GetAllExecutionResults(ctx, id)
	if err != nil {
		return nil, err
	}

	var errors []ExecutionResult
	for _, r := range allResults {
		if r.HasError {
			errors = append(errors, r)
		}
	}

	return errors, nil
}
