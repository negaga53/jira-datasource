package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"golang.org/x/text/unicode/norm"
)

// QueryType enumerates the supported query types.
type QueryType string

const (
	QueryTypeJQLSearch      QueryType = "jql_search"
	QueryTypeIssueCount     QueryType = "issue_count"
	QueryTypeCycleTime      QueryType = "cycle_time"
	QueryTypeChangelog      QueryType = "changelog"
	QueryTypeWorklog        QueryType = "worklog"
	QueryTypeVelocity       QueryType = "velocity"
	QueryTypeCFD            QueryType = "cfd"
	QueryTypeSprintBurndown QueryType = "sprint_burndown"
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
	RefID           string    `json:"refId"`
	QueryType       QueryType `json:"queryType"`
	JQL             string    `json:"jql"`
	StartStatus     string    `json:"startStatus,omitempty"`
	EndStatus       string    `json:"endStatus,omitempty"`
	Quantile        float64   `json:"quantile,omitempty"`
	Interval        string    `json:"interval,omitempty"`
	Fields          []string  `json:"fields,omitempty"`
	Expand          []string  `json:"expand,omitempty"`
	MaxResults      int       `json:"maxResults,omitempty"`
	StoryPointField string    `json:"storyPointField,omitempty"`
	BoardID         int       `json:"boardId,omitempty"`
	SprintID        int       `json:"sprintId,omitempty"`
	DoneStatuses    []string  `json:"doneStatuses,omitempty"`
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
	FieldID    string `json:"fieldId"`
	FieldType  string `json:"fieldtype"`
	From       string `json:"from"`
	FromString string `json:"fromString"`
	To         string `json:"to"`
	ToString   string `json:"toString"`
}

// isStatusChange checks whether a changelog item is a status change,
// handling both the internal field ID and potentially localized field names.
func (item JiraChangelogItem) isStatusChange() bool {
	return strings.EqualFold(item.Field, "status") || item.FieldID == "status"
}

// normalizeString applies Unicode NFC normalization to ensure
// accented characters (e.g. é) match regardless of encoding form.
func normalizeString(s string) string {
	return norm.NFC.String(s)
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
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	UntranslatedName string             `json:"untranslatedName,omitempty"`
	StatusCategory   JiraStatusCategory `json:"statusCategory,omitempty"`
}

// EnglishName returns the untranslated (English) status name, falling back to
// the localized Name when UntranslatedName is not available.
func (s JiraStatus) EnglishName() string {
	if s.UntranslatedName != "" {
		return s.UntranslatedName
	}
	return s.Name
}

// statusMatcher builds a set of all known representations for a list of status
// names (English, localized, or IDs) so that changelog item.To and
// item.ToString can be matched reliably regardless of locale.
type statusMatcher struct {
	matchSet map[string]bool
}

// newStatusMatcher creates a matcher. For each name in queryStatuses it adds
// the English name, the localized name, and the status ID to the match set.
func newStatusMatcher(queryStatuses []string, allStatuses []JiraStatus) *statusMatcher {
	// Index statuses by every known representation.
	type statusInfo struct {
		id, name, english string
	}
	byKey := make(map[string][]statusInfo)
	for _, s := range allStatuses {
		info := statusInfo{id: s.ID, name: normalizeString(s.Name), english: normalizeString(s.EnglishName())}
		byKey[s.ID] = append(byKey[s.ID], info)
		byKey[normalizeString(s.Name)] = append(byKey[normalizeString(s.Name)], info)
		byKey[normalizeString(s.EnglishName())] = append(byKey[normalizeString(s.EnglishName())], info)
	}

	matchSet := make(map[string]bool)
	for _, qs := range queryStatuses {
		key := normalizeString(qs)
		matchSet[qs] = true
		matchSet[key] = true
		for _, info := range byKey[qs] {
			matchSet[info.id] = true
			matchSet[info.name] = true
			matchSet[info.english] = true
		}
		for _, info := range byKey[key] {
			matchSet[info.id] = true
			matchSet[info.name] = true
			matchSet[info.english] = true
		}
	}
	return &statusMatcher{matchSet: matchSet}
}

// Matches returns true if the changelog item's To (ID) or ToString (name)
// corresponds to one of the statuses tracked by this matcher.
func (m *statusMatcher) Matches(item JiraChangelogItem) bool {
	return m.matchSet[item.To] || m.matchSet[normalizeString(item.ToString)]
}

// MatchesStatus returns true if the given status string (ID or name) is in the
// match set.
func (m *statusMatcher) MatchesStatus(s string) bool {
	return m.matchSet[s] || m.matchSet[normalizeString(s)]
}

// JiraStatusCategory represents a Jira status category.
type JiraStatusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
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

// --- Jira Agile API types ---

// JiraBoard represents a Jira Agile board.
type JiraBoard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// JiraBoardResponse is the response from the boards endpoint.
type JiraBoardResponse struct {
	MaxResults int         `json:"maxResults"`
	StartAt    int         `json:"startAt"`
	Total      int         `json:"total"`
	IsLast     bool        `json:"isLast"`
	Values     []JiraBoard `json:"values"`
}

// JiraSprint represents a Jira sprint.
type JiraSprint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"`
	StartDate    string `json:"startDate,omitempty"`
	EndDate      string `json:"endDate,omitempty"`
	CompleteDate string `json:"completeDate,omitempty"`
}

// JiraSprintResponse is the response from the sprints endpoint.
type JiraSprintResponse struct {
	MaxResults int          `json:"maxResults"`
	StartAt    int          `json:"startAt"`
	Total      int          `json:"total"`
	IsLast     bool         `json:"isLast"`
	Values     []JiraSprint `json:"values"`
}
