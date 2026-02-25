package plugin

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// handleCycleTime computes cycle time between two statuses from changelogs.
func (d *Datasource) handleCycleTime(ctx context.Context, query JiraQuery, _ backend.TimeRange) (data.Frames, error) {
	if query.StartStatus == "" || query.EndStatus == "" {
		return nil, fmt.Errorf("cycle time requires startStatus and endStatus")
	}

	// Search issues with changelog expansion
	issues, err := d.jiraClient.SearchIssues(ctx, query.JQL,
		[]string{"summary", "issuetype"}, []string{"changelog"}, query.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("cycle time search: %w", err)
	}

	// For issues without inline changelog, fetch separately
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

	if len(needChangelog) > 0 {
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
		if fetchErr != nil {
			return nil, fmt.Errorf("fetch changelogs: %w", fetchErr)
		}
	}

	// Compute cycle times
	var records []CycleTimeRecord
	for _, issue := range issues {
		if issue.Changelog == nil {
			continue
		}
		record := computeCycleTime(issue, query.StartStatus, query.EndStatus)
		if record != nil {
			records = append(records, *record)
		}
	}

	// Build frame
	n := len(records)
	keyField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	keyField.Name = "key"
	typeField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	typeField.Name = "issueType"
	startStatusField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	startStatusField.Name = "startStatus"
	endStatusField := data.NewFieldFromFieldType(data.FieldTypeString, n)
	endStatusField.Name = "endStatus"
	endDateField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, n)
	endDateField.Name = "endStatusDate"
	cycleField := data.NewFieldFromFieldType(data.FieldTypeFloat64, n)
	cycleField.Name = "cycleTimeDays"

	for i, r := range records {
		keyField.Set(i, r.Key)
		typeField.Set(i, r.IssueType)
		startStatusField.Set(i, r.StartStatus)
		endStatusField.Set(i, r.EndStatus)
		endDate := r.EndDate
		endDateField.Set(i, &endDate)
		cycleField.Set(i, r.CycleTimeDays)
	}

	frame := data.NewFrame("cycle_time",
		keyField, typeField, startStatusField, endStatusField, endDateField, cycleField,
	)

	// Add quantile value if requested
	if query.Quantile > 0 && len(records) > 0 {
		q := computeQuantile(records, query.Quantile)
		quantileField := data.NewFieldFromFieldType(data.FieldTypeFloat64, n)
		quantileField.Name = fmt.Sprintf("p%v", query.Quantile)
		for i := range records {
			quantileField.Set(i, q)
		}
		frame.Fields = append(frame.Fields, quantileField)
	}

	return data.Frames{frame}, nil
}

// computeCycleTime calculates the cycle time for a single issue.
func computeCycleTime(issue JiraIssue, startStatus, endStatus string) *CycleTimeRecord {
	if issue.Changelog == nil {
		return nil
	}

	record := &CycleTimeRecord{
		Key:         issue.Key,
		IssueType:   extractNestedName(issue.Fields, "issuetype"),
		StartStatus: startStatus,
		EndStatus:   endStatus,
	}

	foundStart := false
	foundEnd := false

	// Sort histories by created time
	histories := issue.Changelog.Histories
	sort.Slice(histories, func(i, j int) bool {
		return histories[i].Created < histories[j].Created
	})

	for _, history := range histories {
		t, err := parseJiraTime(history.Created)
		if err != nil {
			continue
		}
		for _, item := range history.Items {
			if item.Field != "status" {
				continue
			}
			// Match by status ID first, fall back to localized name for backward compatibility
			if !foundStart && (item.To == startStatus || item.ToString == startStatus) {
				record.StartDate = t
				foundStart = true
			}
			if foundStart && (item.To == endStatus || item.ToString == endStatus) {
				record.EndDate = t
				foundEnd = true
				break
			}
		}
		if foundEnd {
			break
		}
	}

	if !foundStart || !foundEnd {
		return nil
	}

	record.CycleTimeDays = record.EndDate.Sub(record.StartDate).Hours() / 24
	return record
}

// computeQuantile computes the given percentile of cycle times.
func computeQuantile(records []CycleTimeRecord, percentile float64) float64 {
	values := make([]float64, len(records))
	for i, r := range records {
		values[i] = r.CycleTimeDays
	}
	sort.Float64s(values)

	idx := (percentile / 100) * float64(len(values)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(values) {
		return values[lower]
	}
	frac := idx - float64(lower)
	return values[lower]*(1-frac) + values[upper]*frac
}
