package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestParseSettings(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(t *testing.T, s JiraSettings)
	}{
		{
			name: "valid basic auth settings",
			json: `{"url":"https://jira.example.com","authType":"basic","username":"user@example.com","apiVersion":"2","cacheTTLSeconds":600}`,
			check: func(t *testing.T, s JiraSettings) {
				if s.URL != "https://jira.example.com" {
					t.Errorf("URL = %q, want %q", s.URL, "https://jira.example.com")
				}
				if s.AuthType != "basic" {
					t.Errorf("AuthType = %q, want %q", s.AuthType, "basic")
				}
				if s.Username != "user@example.com" {
					t.Errorf("Username = %q, want %q", s.Username, "user@example.com")
				}
				if s.CacheTTLSeconds != 600 {
					t.Errorf("CacheTTLSeconds = %d, want 600", s.CacheTTLSeconds)
				}
			},
		},
		{
			name: "defaults applied",
			json: `{"url":"https://jira.example.com"}`,
			check: func(t *testing.T, s JiraSettings) {
				if s.APIVersion != "2" {
					t.Errorf("APIVersion = %q, want %q", s.APIVersion, "2")
				}
				if s.CacheTTLSeconds != 300 {
					t.Errorf("CacheTTLSeconds = %d, want 300", s.CacheTTLSeconds)
				}
				if s.AuthType != "basic" {
					t.Errorf("AuthType = %q, want %q", s.AuthType, "basic")
				}
			},
		},
		{
			name:    "missing URL",
			json:    `{"authType":"basic"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := backend.DataSourceInstanceSettings{
				JSONData: json.RawMessage(tt.json),
			}
			s, err := ParseSettings(settings)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(t *testing.T, q JiraQuery)
	}{
		{
			name: "valid jql search query",
			json: `{"queryType":"jql_search","jql":"project = TEST"}`,
			check: func(t *testing.T, q JiraQuery) {
				if q.QueryType != QueryTypeJQLSearch {
					t.Errorf("QueryType = %q, want %q", q.QueryType, QueryTypeJQLSearch)
				}
				if q.JQL != "project = TEST" {
					t.Errorf("JQL = %q, want %q", q.JQL, "project = TEST")
				}
				if q.MaxResults != 1000 {
					t.Errorf("MaxResults = %d, want 1000", q.MaxResults)
				}
			},
		},
		{
			name: "cycle time query",
			json: `{"queryType":"cycle_time","jql":"project = TEST","startStatus":"In Progress","endStatus":"Done","quantile":85}`,
			check: func(t *testing.T, q JiraQuery) {
				if q.QueryType != QueryTypeCycleTime {
					t.Errorf("QueryType = %q, want %q", q.QueryType, QueryTypeCycleTime)
				}
				if q.StartStatus != "In Progress" {
					t.Errorf("StartStatus = %q, want %q", q.StartStatus, "In Progress")
				}
				if q.EndStatus != "Done" {
					t.Errorf("EndStatus = %q, want %q", q.EndStatus, "Done")
				}
				if q.Quantile != 85 {
					t.Errorf("Quantile = %f, want 85", q.Quantile)
				}
			},
		},
		{
			name: "defaults applied for empty query type",
			json: `{"jql":"project = TEST"}`,
			check: func(t *testing.T, q JiraQuery) {
				if q.QueryType != QueryTypeJQLSearch {
					t.Errorf("QueryType = %q, want %q", q.QueryType, QueryTypeJQLSearch)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dq := backend.DataQuery{
				RefID: "A",
				JSON:  json.RawMessage(tt.json),
			}
			q, err := ParseQuery(dq)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, q)
			}
		})
	}
}

func TestParseJiraTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2024-01-15T10:30:00.000+0100", false},
		{"2024-01-15T10:30:00.000Z", false},
		{"2024-01-15T10:30:00Z", false},
		{"2024-01-15T10:30:00+01:00", false},
		{"", true},
		{"not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseJiraTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJiraTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCacheGetSet(t *testing.T) {
	c := NewCache(1 * time.Second)
	defer c.Close()

	// Test set and get
	c.Set("key1", "value1")
	val, ok := c.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Cache.Get(key1) = %v, %v; want value1, true", val, ok)
	}

	// Test miss
	_, ok = c.Get("nonexistent")
	if ok {
		t.Error("Cache.Get(nonexistent) should return false")
	}

	// Test expiration
	c2 := NewCache(1 * time.Millisecond)
	defer c2.Close()
	c2.Set("key2", "value2")
	time.Sleep(5 * time.Millisecond)
	_, ok = c2.Get("key2")
	if ok {
		t.Error("Cache.Get(key2) should have expired")
	}
}

func TestComputeCycleTime(t *testing.T) {
	issue := JiraIssue{
		Key:    "TEST-1",
		Fields: map[string]interface{}{"issuetype": map[string]interface{}{"name": "Story"}},
		Changelog: &JiraChangelog{
			Histories: []JiraChangelogHistory{
				{
					Created: "2024-01-10T10:00:00.000+0000",
					Items: []JiraChangelogItem{
						{Field: "status", To: "3", ToString: "In Progress"},
					},
				},
				{
					Created: "2024-01-13T10:00:00.000+0000",
					Items: []JiraChangelogItem{
						{Field: "status", To: "10001", ToString: "Done"},
					},
				},
			},
		},
	}

	// Match by status ID
	record := computeCycleTime(issue, "3", "10001")
	if record == nil {
		t.Fatal("computeCycleTime() with IDs returned nil")
	}
	if record.CycleTimeDays != 3.0 {
		t.Errorf("CycleTimeDays = %f, want 3.0", record.CycleTimeDays)
	}

	// Backward compatibility: match by localized name
	record2 := computeCycleTime(issue, "In Progress", "Done")
	if record2 == nil {
		t.Fatal("computeCycleTime() with names returned nil")
	}
	if record2.CycleTimeDays != 3.0 {
		t.Errorf("CycleTimeDays = %f, want 3.0", record2.CycleTimeDays)
	}
}

func TestComputeQuantile(t *testing.T) {
	records := []CycleTimeRecord{
		{CycleTimeDays: 1},
		{CycleTimeDays: 2},
		{CycleTimeDays: 3},
		{CycleTimeDays: 4},
		{CycleTimeDays: 5},
	}

	// Median (p50)
	q50 := computeQuantile(records, 50)
	if q50 != 3.0 {
		t.Errorf("p50 = %f, want 3.0", q50)
	}

	// p0
	q0 := computeQuantile(records, 0)
	if q0 != 1.0 {
		t.Errorf("p0 = %f, want 1.0", q0)
	}

	// p100
	q100 := computeQuantile(records, 100)
	if q100 != 5.0 {
		t.Errorf("p100 = %f, want 5.0", q100)
	}
}

func TestExtractString(t *testing.T) {
	fields := map[string]interface{}{
		"summary": "Test issue",
		"number":  42,
	}
	if got := extractString(fields, "summary"); got != "Test issue" {
		t.Errorf("extractString(summary) = %q, want %q", got, "Test issue")
	}
	if got := extractString(fields, "missing"); got != "" {
		t.Errorf("extractString(missing) = %q, want empty", got)
	}
}

func TestExtractNestedName(t *testing.T) {
	fields := map[string]interface{}{
		"status":   map[string]interface{}{"name": "Open"},
		"assignee": map[string]interface{}{"displayName": "John Doe"},
	}
	if got := extractNestedName(fields, "status"); got != "Open" {
		t.Errorf("extractNestedName(status) = %q, want %q", got, "Open")
	}
	if got := extractNestedName(fields, "assignee"); got != "John Doe" {
		t.Errorf("extractNestedName(assignee) = %q, want %q", got, "John Doe")
	}
	if got := extractNestedName(fields, "missing"); got != "" {
		t.Errorf("extractNestedName(missing) = %q, want empty", got)
	}
}

func TestParseInterval(t *testing.T) {
	if got := parseInterval("1h"); got != time.Hour {
		t.Errorf("parseInterval(1h) = %v", got)
	}
	if got := parseInterval("1d"); got != 24*time.Hour {
		t.Errorf("parseInterval(1d) = %v", got)
	}
	if got := parseInterval("1w"); got != 7*24*time.Hour {
		t.Errorf("parseInterval(1w) = %v", got)
	}
	if got := parseInterval(""); got != 24*time.Hour {
		t.Errorf("parseInterval('') = %v", got)
	}
}
