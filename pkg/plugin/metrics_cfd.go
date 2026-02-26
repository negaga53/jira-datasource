package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// statusEvent records a status transition at a point in time.
type statusEvent struct {
	time   time.Time
	status string
}

// issueTimeline tracks an issue's status over time.
type issueTimeline struct {
	created       time.Time
	initialStatus string
	events        []statusEvent
	storyPoints   float64
}

// handleCFD computes a Cumulative Flow Diagram: issue count (or story points)
// per status at each time interval.
func (d *Datasource) handleCFD(ctx context.Context, query JiraQuery, tr backend.TimeRange) (data.Frames, error) {
	searchFields := []string{"status", "created"}
	if query.StoryPointField != "" {
		searchFields = append(searchFields, query.StoryPointField)
	}

	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL,
		searchFields, []string{"changelog"}, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("cfd search: %w", err)
	}

	// Fetch full changelogs for issues with truncated data
	if err := d.fetchMissingChangelogs(ctx, issues); err != nil {
		return nil, fmt.Errorf("cfd fetch changelogs: %w", err)
	}

	interval := parseInterval(query.Interval)

	// Build time buckets
	var bucketTimes []time.Time
	for t := tr.From.Truncate(interval); !t.After(tr.To); t = t.Add(interval) {
		bucketTimes = append(bucketTimes, t)
	}
	if len(bucketTimes) == 0 {
		return data.Frames{data.NewFrame("cfd")}, nil
	}

	// Build timelines and collect all statuses
	statusSet := make(map[string]bool)
	var timelines []issueTimeline

	for _, issue := range issues {
		createdStr := extractString(issue.Fields, "created")
		created, err := parseJiraTime(createdStr)
		if err != nil {
			continue
		}

		// Determine initial status from first changelog entry or current status
		currentStatus := extractNestedName(issue.Fields, "status")
		initialStatus := currentStatus

		var events []statusEvent
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
					if !item.isStatusChange() {
						continue
					}
					statusSet[item.ToString] = true
					events = append(events, statusEvent{time: t, status: item.ToString})
				}
			}

			// The initial status is the FromString of the first status change
			if len(events) > 0 {
				for _, h := range histories {
					for _, item := range h.Items {
						if item.isStatusChange() {
							initialStatus = item.FromString
							goto foundInitial
						}
					}
				}
			}
		}
	foundInitial:
		statusSet[initialStatus] = true

		var sp float64
		if query.StoryPointField != "" {
			if v, ok := issue.Fields[query.StoryPointField].(float64); ok {
				sp = v
			}
		}

		timelines = append(timelines, issueTimeline{
			created:       created,
			initialStatus: initialStatus,
			events:        events,
			storyPoints:   sp,
		})
	}

	// Sort statuses for consistent ordering
	statuses := make([]string, 0, len(statusSet))
	for s := range statusSet {
		statuses = append(statuses, s)
	}
	sort.Strings(statuses)

	// For each time bucket, count issues per status
	type bucketCounts struct {
		counts map[string]float64
	}
	results := make([]bucketCounts, len(bucketTimes))
	for i := range results {
		results[i].counts = make(map[string]float64)
	}

	usePoints := query.StoryPointField != ""

	for _, tl := range timelines {
		for bi, bt := range bucketTimes {
			// Issue doesn't exist yet at this time
			if tl.created.After(bt) {
				continue
			}

			// Determine status at this time bucket
			status := tl.initialStatus
			for _, ev := range tl.events {
				if ev.time.After(bt) {
					break
				}
				status = ev.status
			}

			if usePoints {
				results[bi].counts[status] += tl.storyPoints
			} else {
				results[bi].counts[status]++
			}
		}
	}

	// Build frame
	n := len(bucketTimes)
	timeField := data.NewFieldFromFieldType(data.FieldTypeTime, n)
	timeField.Name = "time"
	for i, t := range bucketTimes {
		timeField.Set(i, t)
	}

	allFields := []*data.Field{timeField}
	for _, s := range statuses {
		f := data.NewFieldFromFieldType(data.FieldTypeFloat64, n)
		f.Name = s
		for i := range bucketTimes {
			f.Set(i, results[i].counts[s])
		}
		allFields = append(allFields, f)
	}

	frame := data.NewFrame("cfd", allFields...)
	frame.Meta = &data.FrameMeta{
		PreferredVisualization: data.VisTypeGraph,
	}

	return data.Frames{frame}, nil
}

// fetchMissingChangelogs fetches full changelogs for issues whose inline
// changelog was truncated.
func (d *Datasource) fetchMissingChangelogs(ctx context.Context, issues []JiraIssue) error {
	type indexed struct {
		idx   int
		issue JiraIssue
	}

	var needChangelog []indexed
	for i, issue := range issues {
		if issue.Changelog == nil || issue.Changelog.Total > issue.Changelog.MaxResults {
			needChangelog = append(needChangelog, indexed{idx: i, issue: issue})
		}
	}

	if len(needChangelog) == 0 {
		return nil
	}

	sem := make(chan struct{}, d.jiraClient.maxConcurrent)
	var mu sync.Mutex
	var fetchErr error

	var wg sync.WaitGroup
	for _, item := range needChangelog {
		wg.Add(1)
		go func(it indexed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			histories, err := d.jiraClient.GetIssueChangelog(ctx, it.issue.Key)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = err
				}
				return
			}
			if issues[it.idx].Changelog == nil {
				issues[it.idx].Changelog = &JiraChangelog{}
			}
			issues[it.idx].Changelog.Histories = histories
		}(item)
	}
	wg.Wait()
	return fetchErr
}
