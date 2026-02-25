package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// agilePath returns the full Agile REST API URL for a given endpoint.
func (c *JiraClient) agilePath(endpoint string) string {
	return fmt.Sprintf("%s/rest/agile/1.0%s", c.baseURL, endpoint)
}

// GetAgile performs a GET request against the Agile REST API.
func (c *JiraClient) GetAgile(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	u := c.agilePath(endpoint)
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

// GetBoards fetches all Scrum/Kanban boards.
func (c *JiraClient) GetBoards(ctx context.Context) ([]JiraBoard, error) {
	var all []JiraBoard
	startAt := 0
	for {
		params := url.Values{
			"startAt":    {strconv.Itoa(startAt)},
			"maxResults": {"50"},
		}
		data, err := c.GetAgile(ctx, "/board", params)
		if err != nil {
			return nil, fmt.Errorf("get boards: %w", err)
		}
		var resp JiraBoardResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parse boards: %w", err)
		}
		all = append(all, resp.Values...)
		if resp.IsLast || len(resp.Values) == 0 {
			break
		}
		startAt += len(resp.Values)
	}
	return all, nil
}

// GetSprints fetches sprints for a given board.
func (c *JiraClient) GetSprints(ctx context.Context, boardID int) ([]JiraSprint, error) {
	endpoint := fmt.Sprintf("/board/%d/sprint", boardID)
	var all []JiraSprint
	startAt := 0
	for {
		params := url.Values{
			"startAt":    {strconv.Itoa(startAt)},
			"maxResults": {"50"},
		}
		data, err := c.GetAgile(ctx, endpoint, params)
		if err != nil {
			return nil, fmt.Errorf("get sprints: %w", err)
		}
		var resp JiraSprintResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parse sprints: %w", err)
		}
		all = append(all, resp.Values...)
		if resp.IsLast || len(resp.Values) == 0 {
			break
		}
		startAt += len(resp.Values)
	}
	return all, nil
}

// GetSprint fetches a single sprint by ID.
func (c *JiraClient) GetSprint(ctx context.Context, sprintID int) (*JiraSprint, error) {
	endpoint := fmt.Sprintf("/sprint/%d", sprintID)
	data, err := c.GetAgile(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get sprint: %w", err)
	}
	var sprint JiraSprint
	if err := json.Unmarshal(data, &sprint); err != nil {
		return nil, fmt.Errorf("parse sprint: %w", err)
	}
	return &sprint, nil
}
