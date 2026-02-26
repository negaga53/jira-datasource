package plugin

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleSprintBurndown computes a sprint burndown chart showing remaining work
// (issue count or story points) over the sprint duration.
func (d *Datasource) handleSprintBurndown(ctx context.Context, query JiraQuery, _ backend.TimeRange) (data.Frames, error) {
	if query.SprintID <= 0 {
		return nil, fmt.Errorf("sprint burndown requires sprintId")
	}

	// Fetch sprint details
	sprint, err := d.jiraClient.GetSprint(ctx, query.SprintID)
	if err != nil {
		return nil, fmt.Errorf("get sprint: %w", err)
	}

	sprintStart, err := parseJiraTime(sprint.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parse sprint start date: %w", err)
	}

	// Use EndDate or CompleteDate
	sprintEndStr := sprint.EndDate
	if sprint.CompleteDate != "" {
		sprintEndStr = sprint.CompleteDate
	}
	sprintEnd, err := parseJiraTime(sprintEndStr)
	if err != nil {
		return nil, fmt.Errorf("parse sprint end date: %w", err)
	}

	// Search for issues in this sprint
	jql := fmt.Sprintf("sprint = %d", query.SprintID)
	if query.JQL != "" {
		jql = fmt.Sprintf("sprint = %d AND (%s)", query.SprintID, query.JQL)
	}

	searchFields := []string{"status", "created"}
	if query.StoryPointField != "" {
		searchFields = append(searchFields, query.StoryPointField)
	}

	issues, err := d.jiraClient.SearchIssues(ctx, jql, searchFields, []string{"changelog"}, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("burndown search: %w", err)
	}

	// Fetch full changelogs
	if err := d.fetchMissingChangelogs(ctx, issues); err != nil {
		return nil, fmt.Errorf("burndown fetch changelogs: %w", err)
	}

	// Determine which statuses are "done"
	doneSet := make(map[string]bool)
	for _, s := range query.DoneStatuses {
		doneSet[s] = true
	}
	// If no done statuses specified, fetch from Jira and use "done" category
	if len(doneSet) == 0 {
		statuses, err := d.jiraClient.GetStatuses(ctx)
		if err == nil {
			for _, s := range statuses {
				if s.StatusCategory.Key == "done" {
					doneSet[s.Name] = true
					doneSet[s.ID] = true
				}
			}
		}
	}

	usePoints := query.StoryPointField != ""

	// Compute total scope
	var totalScope float64
	type burndownEvent struct {
		time     time.Time
		statusID string
		status   string
	}
	type issueInfo struct {
		storyPoints float64
		events      []burndownEvent
		created     time.Time
	}
	var issueInfos []issueInfo

	for _, issue := range issues {
		var sp float64
		if usePoints {
			if v, ok := issue.Fields[query.StoryPointField].(float64); ok {
				sp = v
			}
		} else {
			sp = 1 // count mode
		}
		totalScope += sp

		createdStr := extractString(issue.Fields, "created")
		created, _ := parseJiraTime(createdStr)

		var events []burndownEvent
		if issue.Changelog != nil {
			histories := issue.Changelog.Histories
			sort.Slice(histories, func(i, j int) bool {
				return histories[i].Created < histories[j].Created
			})
			for _, h := range histories {
				t, err := parseJiraTime(h.Created)
				if err != nil {
					continue
				}
				for _, item := range h.Items {
					if item.Field == "status" {
						events = append(events, burndownEvent{time: t, statusID: item.To, status: item.ToString})
					}
				}
			}
		}

		issueInfos = append(issueInfos, issueInfo{
			storyPoints: sp,
			events:      events,
			created:     created,
		})
	}

	// Build daily buckets over sprint duration
	interval := 24 * time.Hour
	var bucketTimes []time.Time
	for t := sprintStart.Truncate(interval); !t.After(sprintEnd); t = t.Add(interval) {
		bucketTimes = append(bucketTimes, t)
	}
	if len(bucketTimes) == 0 {
		return data.Frames{data.NewFrame("burndown")}, nil
	}

	n := len(bucketTimes)
	timeField := data.NewFieldFromFieldType(data.FieldTypeTime, n)
	timeField.Name = "time"
	remainingField := data.NewFieldFromFieldType(data.FieldTypeFloat64, n)
	remainingField.Name = "remaining"
	idealField := data.NewFieldFromFieldType(data.FieldTypeFloat64, n)
	idealField.Name = "ideal"

	sprintDays := float64(n - 1)
	if sprintDays < 1 {
		sprintDays = 1
	}
	dailyBurn := totalScope / sprintDays

	for bi, bt := range bucketTimes {
		timeField.Set(bi, bt)

		// Ideal burndown line
		idealRemaining := totalScope - dailyBurn*float64(bi)
		if idealRemaining < 0 {
			idealRemaining = 0
		}
		idealField.Set(bi, idealRemaining)

		// Actual remaining: total scope minus completed work
		var completed float64
		for _, info := range issueInfos {
			// Find the issue's status at this time
			isDone := false
			for _, ev := range info.events {
				if ev.time.After(bt.Add(interval)) {
					break
				}
				isDone = doneSet[ev.statusID] || doneSet[ev.status]
			}
			if isDone {
				completed += info.storyPoints
			}
		}
		remainingField.Set(bi, totalScope-completed)
	}

	frame := data.NewFrame("burndown", timeField, remainingField, idealField)
	frame.Meta = &data.FrameMeta{
		PreferredVisualization: data.VisTypeGraph,
	}

	return data.Frames{frame}, nil
}
