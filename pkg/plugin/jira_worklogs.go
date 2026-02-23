package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleWorklog fetches worklogs for issues matching the JQL.
func (d *Datasource) handleWorklog(ctx context.Context, query JiraQuery, tr backend.TimeRange) (data.Frames, error) {
	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL,
		[]string{"summary"}, nil, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("worklog search: %w", err)
	}

	type worklogRow struct {
		key              string
		author           string
		timeSpent        string
		timeSpentSeconds int64
		started          string
		comment          string
	}

	var (
		mu       sync.Mutex
		rows     []worklogRow
		fetchErr error
	)

	sem := make(chan struct{}, d.jiraClient.maxConcurrent)
	var wg sync.WaitGroup

	for _, issue := range issues {
		wg.Add(1)
		go func(issueKey string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			worklogs, err := d.jiraClient.GetIssueWorklogs(ctx, issueKey)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = err
				}
				return
			}

			for _, wl := range worklogs {
				// Filter by time range
				started, parseErr := parseJiraTime(wl.Started)
				if parseErr != nil {
					continue
				}
				if started.Before(tr.From) || started.After(tr.To) {
					continue
				}
				rows = append(rows, worklogRow{
					key:              issueKey,
					author:           wl.Author.DisplayName,
					timeSpent:        wl.TimeSpent,
					timeSpentSeconds: wl.TimeSpentSeconds,
					started:          wl.Started,
					comment:          wl.Comment,
				})
			}
		}(issue.Key)
	}

	wg.Wait()
	if fetchErr != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", fetchErr)
	}

	n := len(rows)
	keyField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	keyField.Name = "key"
	authorField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	authorField.Name = "author"
	timeSpentField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	timeSpentField.Name = "timeSpent"
	secondsField := data.NewFieldFromFieldType(data.FieldTypeInt64, n)
	secondsField.Name = "timeSpentSeconds"
	startedField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, n)
	startedField.Name = "started"
	commentField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	commentField.Name = "comment"

	for i, r := range rows {
		keyField.Set(i, r.key)
		authorField.Set(i, r.author)
		timeSpentField.Set(i, r.timeSpent)
		secondsField.Set(i, r.timeSpentSeconds)
		if t, err := parseJiraTime(r.started); err == nil {
			startedField.Set(i, &t)
		}
		commentField.Set(i, r.comment)
	}

	frame := data.NewFrame("worklogs",
		keyField, authorField, timeSpentField, secondsField, startedField, commentField,
	)

	return data.Frames{frame}, nil
}
