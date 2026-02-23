package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleJQLSearch executes a JQL search and returns a table frame.
func (d *Datasource) handleJQLSearch(ctx context.Context, query JiraQuery, _ backend.TimeRange) (data.Frames, error) {
	fields := query.Fields
	if len(fields) == 0 {
		fields = []string{"summary", "status", "assignee", "priority", "issuetype", "created", "updated"}
	}

	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL, fields, query.Expand, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("jql search: %w", err)
	}

	// Build frame columns
	keyField := data.NewFieldFromFieldType(data.FieldTypeString, len(issues))
	keyField.Name = "key"
	summaryField := data.NewFieldFromFieldType(data.FieldTypeString, len(issues))
	summaryField.Name = "summary"
	statusField := data.NewFieldFromFieldType(data.FieldTypeString, len(issues))
	statusField.Name = "status"
	assigneeField := data.NewFieldFromFieldType(data.FieldTypeString, len(issues))
	assigneeField.Name = "assignee"
	priorityField := data.NewFieldFromFieldType(data.FieldTypeString, len(issues))
	priorityField.Name = "priority"
	issueTypeField := data.NewFieldFromFieldType(data.FieldTypeString, len(issues))
	issueTypeField.Name = "issueType"
	createdField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, len(issues))
	createdField.Name = "created"
	updatedField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, len(issues))
	updatedField.Name = "updated"

	for i, issue := range issues {
		keyField.Set(i, issue.Key)
		summaryField.Set(i, extractString(issue.Fields, "summary"))
		statusField.Set(i, extractNestedName(issue.Fields, "status"))
		assigneeField.Set(i, extractNestedName(issue.Fields, "assignee"))
		priorityField.Set(i, extractNestedName(issue.Fields, "priority"))
		issueTypeField.Set(i, extractNestedName(issue.Fields, "issuetype"))

		if t, err := parseJiraTime(extractString(issue.Fields, "created")); err == nil {
			createdField.Set(i, &t)
		}
		if t, err := parseJiraTime(extractString(issue.Fields, "updated")); err == nil {
			updatedField.Set(i, &t)
		}
	}

	frame := data.NewFrame("issues",
		keyField, summaryField, statusField, assigneeField,
		priorityField, issueTypeField, createdField, updatedField,
	)

	return data.Frames{frame}, nil
}

// extractString extracts a string field from issue fields.
func extractString(fields map[string]interface{}, key string) string {
	if v, ok := fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractNestedName extracts .name from a nested object (e.g. status.name).
func extractNestedName(fields map[string]interface{}, key string) string {
	if v, ok := fields[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			if name, ok := m["name"].(string); ok {
				return name
			}
			if name, ok := m["displayName"].(string); ok {
				return name
			}
		}
	}
	return ""
}

// parseJiraTime parses a Jira datetime string.
func parseJiraTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	// Jira uses ISO 8601 format with timezone offset
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}
