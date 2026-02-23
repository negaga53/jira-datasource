package plugin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// QueryType enumerates the supported query types.
type QueryType string

const (
	QueryTypeJQLSearch  QueryType = "jql_search"
	QueryTypeIssueCount QueryType = "issue_count"
	QueryTypeCycleTime  QueryType = "cycle_time"
	QueryTypeChangelog  QueryType = "changelog"
	QueryTypeWorklog    QueryType = "worklog"
)

// JiraSettings holds the parsed datasource configuration.
type JiraSettings struct {
	URL             string `json:"url"`
	AuthType        string `json:"authType"`
	Username        string `json:"username,omitempty"`
	APIVersion      string `json:"apiVersion"`
	CacheTTLSeconds int    `json:"cacheTTLSeconds"`
}

// ParseSettings extracts JiraSettings from Grafana datasource instance settings.
func ParseSettings(s backend.DataSourceInstanceSettings) (JiraSettings, error) {
	var settings JiraSettings
	if err := json.Unmarshal(s.JSONData, &settings); err != nil {
		return settings, fmt.Errorf("failed to parse settings: %w", err)
	}
	if settings.URL == "" {
		return settings, fmt.Errorf("jira URL is required")
	}
	if settings.APIVersion == "" {
		settings.APIVersion = "2"
	}
	if settings.CacheTTLSeconds <= 0 {
		settings.CacheTTLSeconds = 300
	}
	if settings.AuthType == "" {
		settings.AuthType = "basic"
	}
	return settings, nil
}

// JiraQuery represents a single query from the frontend.
type JiraQuery struct {
	RefID       string    `json:"refId"`
	QueryType   QueryType `json:"queryType"`
	JQL         string    `json:"jql"`
	StartStatus string    `json:"startStatus,omitempty"`
	EndStatus   string    `json:"endStatus,omitempty"`
	Quantile    float64   `json:"quantile,omitempty"`
	Interval    string    `json:"interval,omitempty"`
	Fields      []string  `json:"fields,omitempty"`
	Expand      []string  `json:"expand,omitempty"`
	MaxResults  int       `json:"maxResults,omitempty"`
}

// ParseQuery deserializes a backend.DataQuery into a JiraQuery.
func ParseQuery(q backend.DataQuery) (JiraQuery, error) {
	var jq JiraQuery
	if err := json.Unmarshal(q.JSON, &jq); err != nil {
		return jq, fmt.Errorf("failed to parse query: %w", err)
	}
	jq.RefID = q.RefID
	if jq.QueryType == "" {
		jq.QueryType = QueryTypeJQLSearch
	}
	if jq.MaxResults <= 0 {
		jq.MaxResults = 1000
	}
	return jq, nil
}

// --- Jira API response types ---

// JiraSearchResponse is the response from /rest/api/{v}/search/jql.
type JiraSearchResponse struct {
	StartAt        int         `json:"startAt"`
	MaxResults     int         `json:"maxResults"`
	Total          int         `json:"total"`
	Issues         []JiraIssue `json:"issues"`
	IsLast         bool        `json:"isLast"`
	NextPageToken  string      `json:"nextPageToken,omitempty"`
}

// JiraIssue represents a single Jira issue.
type JiraIssue struct {
	ID        string                 `json:"id"`
	Key       string                 `json:"key"`
	Self      string                 `json:"self"`
	Fields    map[string]interface{} `json:"fields"`
	Changelog *JiraChangelog         `json:"changelog,omitempty"`
}

// JiraChangelog holds the changelog for an issue (when expanded).
type JiraChangelog struct {
	StartAt    int                    `json:"startAt"`
	MaxResults int                    `json:"maxResults"`
	Total      int                    `json:"total"`
	Histories  []JiraChangelogHistory `json:"histories"`
}

// JiraChangelogHistory represents a single changelog entry.
type JiraChangelogHistory struct {
	ID      string                  `json:"id"`
	Author  JiraUser                `json:"author"`
	Created string                  `json:"created"`
	Items   []JiraChangelogItem     `json:"items"`
}

// JiraChangelogItem represents a single field change in a changelog entry.
type JiraChangelogItem struct {
	Field      string `json:"field"`
	FieldType  string `json:"fieldtype"`
	FromString string `json:"fromString"`
	ToString   string `json:"toString"`
}

// JiraUser represents a Jira user.
type JiraUser struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	AccountID    string `json:"accountId,omitempty"`
	Name         string `json:"name,omitempty"`
}

// JiraProject represents a Jira project.
type JiraProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// JiraField represents a Jira field.
type JiraField struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
}

// JiraStatus represents a Jira status.
type JiraStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// JiraIssueType represents a Jira issue type.
type JiraIssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// JiraWorklog represents a single worklog entry.
type JiraWorklog struct {
	ID               string   `json:"id"`
	Author           JiraUser `json:"author"`
	TimeSpent        string   `json:"timeSpent"`
	TimeSpentSeconds int64    `json:"timeSpentSeconds"`
	Started          string   `json:"started"`
	Comment          string   `json:"comment,omitempty"`
}

// JiraWorklogResponse is the response for worklog endpoints.
type JiraWorklogResponse struct {
	StartAt    int           `json:"startAt"`
	MaxResults int           `json:"maxResults"`
	Total      int           `json:"total"`
	Worklogs   []JiraWorklog `json:"worklogs"`
}

// CycleTimeRecord holds the computed cycle time for a single issue.
type CycleTimeRecord struct {
	Key           string
	IssueType     string
	StartStatus   string
	EndStatus     string
	StartDate     time.Time
	EndDate       time.Time
	CycleTimeDays float64
}

// SelectOption is used for resource handler responses (dropdowns).
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
