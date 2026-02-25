package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleJQLSearch executes a JQL search and returns a table frame.
func (d *Datasource) handleJQLSearch(ctx context.Context, query JiraQuery, _ backend.TimeRange) (data.Frames, error) {
	fields := query.Fields
	useDefaults := len(fields) == 0
	if useDefaults {
		fields = []string{"summary", "status", "assignee", "priority", "issuetype", "created", "updated"}
	}

	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL, fields, query.Expand, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("jql search: %w", err)
	}

	n := len(issues)

	// Always include the issue key
	keyField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	keyField.Name = "key"
	for i, issue := range issues {
		keyField.Set(i, issue.Key)
	}

	if useDefaults {
		// Default fields with typed columns (preserves original behavior)
		return d.buildDefaultSearchFrame(keyField, issues, n)
	}

	// Dynamic field handling for user-selected fields
	fieldColumns := make([]*data.Field, len(fields))
	for i, f := range fields {
		fieldColumns[i] = data.NewFieldFromFieldType(data.FieldTypeString, n)
		fieldColumns[i].Name = f
	}

	for i, issue := range issues {
		for j, f := range fields {
			fieldColumns[j].Set(i, extractFieldValue(issue.Fields, f))
		}
	}

	allFields := make([]*data.Field, 0, 1+len(fieldColumns))
	allFields = append(allFields, keyField)
	allFields = append(allFields, fieldColumns...)
	frame := data.NewFrame("issues", allFields...)

	return data.Frames{frame}, nil
}

// buildDefaultSearchFrame builds the response frame with typed columns for the default field set.
func (d *Datasource) buildDefaultSearchFrame(keyField *data.Field, issues []JiraIssue, n int) (data.Frames, error) {
	summaryField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	summaryField.Name = "summary"
	statusField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	statusField.Name = "status"
	assigneeField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	assigneeField.Name = "assignee"
	priorityField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	priorityField.Name = "priority"
	issueTypeField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	issueTypeField.Name = "issueType"
	createdField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, n)
	createdField.Name = "created"
	updatedField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, n)
	updatedField.Name = "updated"

	for i, issue := range issues {
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

// extractFieldValue extracts a display value from an issue field, handling
// strings, nested objects (with name/displayName), arrays, and numbers.
func extractFieldValue(fields map[string]interface{}, key string) string {
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case map[string]interface{}:
		if name, ok := val["name"].(string); ok {
			return name
		}
		if name, ok := val["displayName"].(string); ok {
			return name
		}
		if value, ok := val["value"].(string); ok {
			return value
		}
		return ""
	case []interface{}:
		var parts []string
		for _, item := range val {
			if m, ok := item.(map[string]interface{}); ok {
				if name, ok := m["name"].(string); ok {
					parts = append(parts, name)
				} else if name, ok := m["displayName"].(string); ok {
					parts = append(parts, name)
				}
			} else if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}
