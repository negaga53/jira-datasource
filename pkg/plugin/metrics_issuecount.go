package plugin

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleIssueCount buckets issues by creation date and returns a time series.
func (d *Datasource) handleIssueCount(ctx context.Context, query JiraQuery, tr backend.TimeRange) (data.Frames, error) {
	fields := []string{"created"}
	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL, fields, nil, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("issue count search: %w", err)
	}

	interval := parseInterval(query.Interval)

	// Bucket issues into time intervals
	buckets := make(map[time.Time]int64)

	// Create all empty buckets in the time range
	bucketStart := tr.From.Truncate(interval)
	for t := bucketStart; t.Before(tr.To); t = t.Add(interval) {
		buckets[t] = 0
	}

	for _, issue := range issues {
		created := extractString(issue.Fields, "created")
		t, err := parseJiraTime(created)
		if err != nil {
			continue
		}
		bucket := t.Truncate(interval)
		if bucket.Before(tr.From) || bucket.After(tr.To) {
			continue
		}
		buckets[bucket]++
	}

	// Sort buckets by time
	times := make([]time.Time, 0, len(buckets))
	for t := range buckets {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	timeField := data.NewFieldFromFieldType(data.FieldTypeTime, len(times))
	timeField.Name = "time"
	countField := data.NewFieldFromFieldType(data.FieldTypeInt64, len(times))
	countField.Name = "count"

	for i, t := range times {
		timeField.Set(i, t)
		countField.Set(i, buckets[t])
	}

	frame := data.NewFrame("issue_count", timeField, countField)
	frame.Meta = &data.FrameMeta{
		PreferredVisualization: data.VisTypeGraph,
	}

	return data.Frames{frame}, nil
}

// parseInterval converts an interval string into a time.Duration.
func parseInterval(interval string) time.Duration {
	switch interval {
	case "1h":
		return time.Hour
	case "1d", "":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1M":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}
