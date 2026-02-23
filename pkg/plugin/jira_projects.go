package plugin

import (
	"context"
	"encoding/json"
	"net/url"
)

// GetProjects fetches all projects from Jira.
func (c *JiraClient) GetProjects(ctx context.Context) ([]JiraProject, error) {
	data, err := c.Get(ctx, "/project", nil)
	if err != nil {
		return nil, err
	}
	var projects []JiraProject
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// GetIssueTypes fetches issue types for a project.
func (c *JiraClient) GetIssueTypes(ctx context.Context, projectKey string) ([]JiraIssueType, error) {
	endpoint := "/project/" + url.PathEscape(projectKey)
	data, err := c.Get(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	var project struct {
		IssueTypes []JiraIssueType `json:"issueTypes"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, err
	}
	return project.IssueTypes, nil
}
