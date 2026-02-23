package plugin

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleChangelog fetches and flattens changelogs for issues matching the JQL.
func (d *Datasource) handleChangelog(ctx context.Context, query JiraQuery, _ backend.TimeRange) (data.Frames, error) {
	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL,
		[]string{"summary", "issuetype", "created"}, []string{"changelog"}, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("changelog search: %w", err)
	}

	type row struct {
		key       string
		issueType string
		created   string
		field     string
		from      string
		to        string
		author    string
		timestamp string
	}

	var rows []row
	for _, issue := range issues {
		if issue.Changelog == nil {
			continue
		}
		issueType := extractNestedName(issue.Fields, "issuetype")
		created := extractString(issue.Fields, "created")
		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				// Apply field filter if specified
				if len(query.Fields) > 0 {
					match := false
					for _, f := range query.Fields {
						if f == item.Field {
							match = true
							break
						}
					}
					if !match {
						continue
					}
				}
				rows = append(rows, row{
					key:       issue.Key,
					issueType: issueType,
					created:   created,
					field:     item.Field,
					from:      item.FromString,
					to:        item.ToString,
					author:    history.Author.DisplayName,
					timestamp: history.Created,
				})
			}
		}
	}

	n := len(rows)
	keyField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	keyField.Name = "key"
	typeField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	typeField.Name = "issueType"
	createdField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, n)
	createdField.Name = "created"
	tsField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, n)
	tsField.Name = "changeTimestamp"
	fieldNameField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	fieldNameField.Name = "field"
	fromField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	fromField.Name = "fromValue"
	toField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	toField.Name = "toValue"
	authorField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	authorField.Name = "author"

	for i, r := range rows {
		keyField.Set(i, r.key)
		typeField.Set(i, r.issueType)
		if t, err := parseJiraTime(r.created); err == nil {
			createdField.Set(i, &t)
		}
		if t, err := parseJiraTime(r.timestamp); err == nil {
			tsField.Set(i, &t)
		}
		fieldNameField.Set(i, r.field)
		fromField.Set(i, r.from)
		toField.Set(i, r.to)
		authorField.Set(i, r.author)
	}

	frame := data.NewFrame("changelog",
		keyField, typeField, createdField, tsField,
		fieldNameField, fromField, toField, authorField,
	)

	return data.Frames{frame}, nil
}
