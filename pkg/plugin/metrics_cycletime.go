package plugin

import (
	"context"
	"fmt"
	"math"
	"sort"

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
	if err := d.fetchMissingChangelogs(ctx, issues); err != nil {
		return nil, fmt.Errorf("fetch changelogs: %w", err)
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
		Key:       issue.Key,
		IssueType: extractNestedName(issue.Fields, "issuetype"),
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
				record.StartStatus = item.ToString // display the human-readable name
				foundStart = true
			}
			if foundStart && (item.To == endStatus || item.ToString == endStatus) {
				record.EndDate = t
				record.EndStatus = item.ToString // display the human-readable name
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
