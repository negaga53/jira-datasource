package plugin

import (
	"context"
	"encoding/json"
)

// GetFields fetches all available fields from Jira.
func (c *JiraClient) GetFields(ctx context.Context) ([]JiraField, error) {
	data, err := c.Get(ctx, "/field", nil)
	if err != nil {
		return nil, err
	}
	var fields []JiraField
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// GetStatuses fetches all available statuses from Jira.
func (c *JiraClient) GetStatuses(ctx context.Context) ([]JiraStatus, error) {
	data, err := c.Get(ctx, "/status", nil)
	if err != nil {
		return nil, err
	}
	var statuses []JiraStatus
	if err := json.Unmarshal(data, &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}
