package plugin

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleVelocity computes throughput (issue count) and optionally story points
// resolved per time interval. It buckets by the issue's resolution date.
func (d *Datasource) handleVelocity(ctx context.Context, query JiraQuery, tr backend.TimeRange) (data.Frames, error) {
	// Fields to fetch from Jira
	searchFields := []string{"resolutiondate"}
	if query.StoryPointField != "" {
		searchFields = append(searchFields, query.StoryPointField)
	}

	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL, searchFields, nil, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("velocity search: %w", err)
	}

	interval := parseInterval(query.Interval)

	// Create time buckets
	countBuckets := make(map[time.Time]int64)
	pointBuckets := make(map[time.Time]float64)

	bucketStart := tr.From.Truncate(interval)
	for t := bucketStart; !t.After(tr.To); t = t.Add(interval) {
		countBuckets[t] = 0
		pointBuckets[t] = 0
	}

	for _, issue := range issues {
		dateStr := extractString(issue.Fields, "resolutiondate")
		t, err := parseJiraTime(dateStr)
		if err != nil {
			continue
		}
		bucket := t.Truncate(interval)
		if bucket.Before(tr.From) || bucket.After(tr.To) {
			continue
		}
		countBuckets[bucket]++

		if query.StoryPointField != "" {
			if points, ok := issue.Fields[query.StoryPointField].(float64); ok {
				pointBuckets[bucket] += points
			}
		}
	}

	// Sort buckets by time
	times := make([]time.Time, 0, len(countBuckets))
	for t := range countBuckets {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	n := len(times)
	timeField := data.NewFieldFromFieldType(data.FieldTypeTime, n)
	timeField.Name = "time"
	countField := data.NewFieldFromFieldType(data.FieldTypeInt64, n)
	countField.Name = "throughput"

	for i, t := range times {
		timeField.Set(i, t)
		countField.Set(i, countBuckets[t])
	}

	frame := data.NewFrame("velocity", timeField, countField)

	if query.StoryPointField != "" {
		pointField := data.NewFieldFromFieldType(data.FieldTypeFloat64, n)
		pointField.Name = "storyPoints"
		for i, t := range times {
			pointField.Set(i, pointBuckets[t])
		}
		frame.Fields = append(frame.Fields, pointField)
	}

	frame.Meta = &data.FrameMeta{
		PreferredVisualization: data.VisTypeGraph,
	}

	return data.Frames{frame}, nil
}
