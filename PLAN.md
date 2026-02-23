# Jira Datasource Plugin for Grafana — Implementation Plan

## 1. Project Overview

**Name:** `jira-datasource`
**Type:** Grafana Backend Datasource Plugin (Go backend + React/TypeScript frontend)
**Purpose:** Query Jira Cloud/Server via REST API and visualize data in Grafana dashboards — issue counts, cycle time, changelogs, sprint metrics, worklogs, and more.

### Reference Project

Based on analysis of [qvest-digital/tarent-jira-datasource](https://github.com/qvest-digital/tarent-jira-datasource), a frontend-only Grafana 9.x plugin limited to cycle time and changelog queries. Our implementation improves upon it with:

- **Go backend** (server-side processing, security, scalability, alerting support)
- **Latest Grafana SDK** (11.x+ via `@grafana/create-plugin`)
- **Multiple auth methods** (Basic Auth, Bearer Token, OAuth 2.0)
- **Rich query types** (JQL search, cycle time, sprint data, worklogs, issue counts, changelogs)
- **Template variable support** (`metricFindQuery`)
- **Server-side caching** with configurable TTL
- **Annotation support**
- **Alerting support**

---

## 2. Technology Stack

| Component | Technology | Version |
| --- | --- | --- |
| Scaffolding | `@grafana/create-plugin` | latest |
| Frontend | React + TypeScript | 18.x / 5.x |
| Grafana SDKs | `@grafana/data`, `@grafana/runtime`, `@grafana/ui` | 11.x+ |
| Backend | Go | 1.22+ |
| Backend SDK | `github.com/grafana/grafana-plugin-sdk-go` | latest |
| Build (Go) | Mage | latest |
| Build (Frontend) | Webpack (via create-plugin) | 5.x |
| Testing | Jest (frontend), Go testing (backend) | — |
| E2E Testing | Playwright (via create-plugin) | — |
| Docker | Docker Compose for local dev | — |
| Jira API | REST API v2 (Server) / v3 (Cloud) | — |

---

## 3. Project Structure

```
jira-datasource/
├── .config/                        # Grafana auto-generated config (webpack, jest, etc.)
├── .github/
│   └── workflows/
│       ├── ci.yml                  # Build, test, lint
│       └── release.yml             # Build, sign, release
├── docker-compose.yaml             # Local Grafana dev environment
├── Magefile.go                     # Go build targets
├── go.mod
├── go.sum
├── package.json
├── tsconfig.json
├── PLAN.md
├── README.md
├── LICENSE
├── pkg/
│   ├── main.go                     # Backend entry point
│   └── plugin/
│       ├── datasource.go           # Datasource struct, NewDatasource()
│       ├── query.go                # QueryData() implementation
│       ├── health.go               # CheckHealth() implementation
│       ├── resource.go             # CallResource() — for template variables, etc.
│       ├── jira_client.go          # HTTP client for Jira REST API
│       ├── jira_search.go          # JQL search with pagination
│       ├── jira_changelog.go       # Changelog fetching & processing
│       ├── jira_projects.go        # Project listing
│       ├── jira_fields.go          # Field metadata retrieval
│       ├── jira_worklogs.go        # Worklog retrieval
│       ├── metrics_cycletime.go    # Cycle time calculation
│       ├── metrics_issuecount.go   # Issue count over time
│       ├── cache.go                # Server-side in-memory cache with TTL
│       ├── models.go               # Shared types/structs
│       └── models_test.go          # Unit tests
├── src/
│   ├── module.ts                   # Plugin entry point
│   ├── datasource.ts               # Frontend DataSource class
│   ├── plugin.json                 # Plugin manifest
│   ├── types.ts                    # TypeScript interfaces
│   ├── components/
│   │   ├── ConfigEditor.tsx        # Datasource settings UI
│   │   ├── QueryEditor.tsx         # Query builder UI
│   │   └── VariableQueryEditor.tsx # Template variable editor
│   ├── img/
│   │   └── logo.svg
│   └── dashboards/
│       └── overview.json           # Bundled example dashboard
└── tests/
    └── integration/                # Playwright E2E tests
```

---

## 4. Authentication

### 4.1 Supported Methods

| Method | Jira Cloud | Jira Server/DC | How |
| --- | --- | --- | --- |
| **Basic Auth** (email + API token) | ✅ | ✅ | `Authorization: Basic base64(email:token)` |
| **Bearer Token** (PAT) | ❌ | ✅ | `Authorization: Bearer <token>` |
| **OAuth 2.0 (3LO)** | ✅ | ❌ | Future phase — authorization code grant |

### 4.2 Implementation

- **ConfigEditor** presents a dropdown for auth type, with conditional fields:
  - Basic Auth: URL, Username (email), API Token (secret)
  - Bearer Token: URL, Token (secret)
- Secrets stored via Grafana's `secureJsonData` — never exposed to the browser
- Go backend reads secrets from `DataSourceInstanceSettings.DecryptedSecureJSONData`
- The backend constructs the HTTP client in `NewDatasource()` with the appropriate `Authorization` header

### 4.3 Config Model

```typescript
// Frontend types
interface JiraDataSourceOptions extends DataSourceJsonData {
  url: string;              // Jira base URL
  authType: 'basic' | 'bearer';
  username?: string;        // For basic auth (email)
  apiVersion: 'v2' | 'v3'; // API version selection
  cacheTTLSeconds: number;  // Server-side cache TTL (default: 300)
}

interface JiraSecureJsonData {
  apiToken?: string;        // For basic auth
  bearerToken?: string;     // For bearer/PAT auth
}
```

```go
// Backend types
type JiraSettings struct {
    URL             string `json:"url"`
    AuthType        string `json:"authType"`
    Username        string `json:"username,omitempty"`
    APIVersion      string `json:"apiVersion"`
    CacheTTLSeconds int    `json:"cacheTTLSeconds"`
}
```

---

## 5. Jira REST API Integration

### 5.1 Endpoints Used

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/rest/api/{version}/search` | GET | Search issues via JQL with pagination |
| `/rest/api/{version}/search/jql` | GET | Enhanced JQL search (v3 Cloud) |
| `/rest/api/{version}/myself` | GET | Health check / connectivity test |
| `/rest/api/{version}/issue/{key}/changelog` | GET | Get changelog for a single issue |
| `/rest/api/{version}/changelog/bulkfetch` | POST | Bulk fetch changelogs (v3 Cloud) |
| `/rest/api/{version}/field` | GET | List available fields (for query editor dropdowns) |
| `/rest/api/{version}/project` | GET | List projects (for template variables) |
| `/rest/api/{version}/issue/{key}/worklog` | GET | Get worklogs for an issue |
| `/rest/api/{version}/status` | GET | List available statuses |

### 5.2 Pagination Strategy

Jira uses offset-based pagination with `startAt`, `maxResults`, and `total`:

```json
{ "startAt": 0, "maxResults": 50, "total": 200, "issues": [...] }
```

The Go backend will:
1. Fetch the first page to learn `total`
2. Fetch remaining pages **concurrently** (bounded goroutines, max 5 parallel)
3. Merge results and return a unified dataset

### 5.3 Rate Limiting

- Respect `Retry-After` and `X-RateLimit-*` headers
- Implement exponential backoff with jitter on 429 responses
- Configurable max concurrent requests (default: 5)

### 5.4 Authentication Header Construction (Go)

```go
func (c *JiraClient) authHeader() string {
    switch c.settings.AuthType {
    case "basic":
        creds := base64.StdEncoding.EncodeToString(
            []byte(c.settings.Username + ":" + c.secrets["apiToken"]))
        return "Basic " + creds
    case "bearer":
        return "Bearer " + c.secrets["bearerToken"]
    }
    return ""
}
```

---

## 6. Query Types & Data Frames

### 6.1 Query Model

```typescript
interface JiraQuery extends DataQuery {
  queryType: QueryType;
  jql: string;                   // JQL expression
  // Cycle time specific
  startStatus?: string;
  endStatus?: string;
  quantile?: number;             // 1-100
  // Issue count specific
  interval?: string;             // Time bucketing interval
  // Fields to include
  fields?: string[];
  // Expand options
  expand?: string[];             // changelog, renderedFields, etc.
}

enum QueryType {
  JQL_SEARCH = 'jql_search',
  ISSUE_COUNT = 'issue_count',
  CYCLE_TIME = 'cycle_time',
  CHANGELOG = 'changelog',
  WORKLOG = 'worklog',
}
```

### 6.2 Query Type Details

#### A. JQL Search (`jql_search`)
- **Input:** JQL string, optional fields list
- **Output:** Table frame — one row per issue, columns for key, summary, status, assignee, priority, created, updated, custom fields
- **Use case:** Issue lists, tables, stat panels

#### B. Issue Count (`issue_count`)
- **Input:** JQL string, time interval
- **Output:** Time series frame — timestamp + count
- **Use case:** Line charts showing issue creation/resolution over time
- **Implementation:** Backend buckets issues by `created`/`resolved` date into time intervals

#### C. Cycle Time (`cycle_time`)
- **Input:** JQL string, start status, end status, quantile percentage
- **Output:** Table frame — issue key, issue type, start→end dates, cycle time in days, quantile value
- **Use case:** Scatter plots, stat panels for agile metrics
- **Implementation:** Parse changelogs, find status transitions, calculate duration between start/end status changes

#### D. Changelog (`changelog`)
- **Input:** JQL string, optional field filter
- **Output:** Table frame — issue key, timestamp, field name, from value, to value, author
- **Use case:** Audit logs, status flow analysis
- **Implementation:** Expand changelogs in search or use bulk fetch, flatten into rows

#### E. Worklog (`worklog`)
- **Input:** JQL string, time range
- **Output:** Table/time series frame — issue key, author, time spent, date
- **Use case:** Time tracking dashboards
- **Implementation:** Fetch worklogs per issue, filter by time range

### 6.3 Data Frame Output Formats

| Query Type | Frame Type | Key Fields |
| --- | --- | --- |
| JQL Search | Table | key, summary, status, assignee, priority, created, updated |
| Issue Count | Time Series | time, count |
| Cycle Time | Table | key, issueType, startStatus, endStatus, endStatusDate, cycleTimeDays, quantile |
| Changelog | Table | key, issueType, created, field, fromValue, toValue, author |
| Worklog | Table | key, author, timeSpent, timeSpentSeconds, started, comment |

---

## 7. Frontend Components

### 7.1 ConfigEditor (`ConfigEditor.tsx`)

Fields:
- **Jira URL** — text input (required)
- **API Version** — select: `v2` (Server/DC) or `v3` (Cloud), default `v2`
- **Auth Type** — select: `Basic Auth` or `Bearer Token`
- **Username** — text input (shown for Basic Auth)
- **API Token / Bearer Token** — SecretInput (context-dependent label)
- **Cache TTL** — number input (seconds, default 300)

### 7.2 QueryEditor (`QueryEditor.tsx`)

Fields:
- **Query Type** — select dropdown (JQL Search, Issue Count, Cycle Time, Changelog, Worklog)
- **JQL** — textarea with syntax hints (always shown)
- Conditional fields per query type:
  - **Cycle Time:** Start Status (async select), End Status (async select), Quantile (number 1-100)
  - **Issue Count:** Interval selector (1h, 1d, 1w, 1M)
  - **Changelog:** Field filter (multi-select, loaded from `/rest/api/{v}/field`)
  - **JQL Search:** Field selector (multi-select)
  - **Worklog:** (just JQL + time range from Grafana)

### 7.3 VariableQueryEditor (`VariableQueryEditor.tsx`)

Supports template variable queries:
- `projects()` — list all project keys
- `statuses()` — list all statuses
- `fields()` — list all field names
- `issuetypes(projectKey)` — list issue types for a project
- `labels()` — list all labels via JQL

---

## 8. Backend Implementation

### 8.1 Entry Point (`pkg/main.go`)

```go
package main

import (
    "os"
    "github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
    "github.com/grafana/grafana-plugin-sdk-go/backend/log"
    "jira-datasource/pkg/plugin"
)

func main() {
    if err := datasource.Manage("jira-datasource", plugin.NewDatasource, datasource.ManageOpts{}); err != nil {
        log.DefaultLogger.Error(err.Error())
        os.Exit(1)
    }
}
```

### 8.2 Datasource Struct (`pkg/plugin/datasource.go`)

```go
type Datasource struct {
    jiraClient *JiraClient
    cache      *Cache
    settings   JiraSettings
}

func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
    // Parse jsonData → JiraSettings
    // Build HTTP client with auth headers from DecryptedSecureJSONData
    // Initialize cache with configured TTL
    // Return &Datasource{...}
}

func (d *Datasource) Dispose() {
    // Cleanup: stop cache eviction timers, close HTTP client
}
```

### 8.3 Health Check (`pkg/plugin/health.go`)

```go
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
    // GET /rest/api/{version}/myself
    // Return HealthStatusOk + user display name on success
    // Return HealthStatusError + error message on failure
}
```

### 8.4 Query Data (`pkg/plugin/query.go`)

```go
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
    response := backend.NewQueryDataResponse()
    for _, q := range req.Queries {
        // Unmarshal query JSON → JiraQuery
        // Route to handler by queryType:
        //   jql_search    → handleJQLSearch()
        //   issue_count   → handleIssueCount()
        //   cycle_time    → handleCycleTime()
        //   changelog     → handleChangelog()
        //   worklog       → handleWorklog()
        // Set response per refID
    }
    return response, nil
}
```

### 8.5 Resource Handler (`pkg/plugin/resource.go`)

Exposes REST endpoints for the frontend (template variables, async selects):

| Path | Response |
| --- | --- |
| `GET /projects` | `[{ "value": "PROJ", "label": "Project Name" }]` |
| `GET /statuses` | `[{ "value": "In Progress", "label": "In Progress" }]` |
| `GET /fields` | `[{ "value": "status", "label": "Status" }]` |
| `GET /issuetypes?project=KEY` | `[{ "value": "Bug", "label": "Bug" }]` |

### 8.6 Caching (`pkg/plugin/cache.go`)

- In-memory `sync.Map` with TTL-based expiration
- Cache key: hash of (endpoint + query parameters)
- Default TTL: 300s (configurable via datasource settings)
- Cache invalidation on datasource settings change (via `Dispose()` + `NewDatasource()`)

---

## 9. Plugin Manifest (`src/plugin.json`)

```json
{
  "type": "datasource",
  "name": "Jira",
  "id": "jira-datasource",
  "backend": true,
  "executable": "gpx_jira-datasource",
  "alerting": true,
  "metrics": true,
  "logs": true,
  "info": {
    "description": "Jira datasource for Grafana — query issues, changelogs, cycle time, and more via JQL",
    "author": { "name": "Your Name" },
    "keywords": ["jira", "atlassian", "datasource", "jql"],
    "logos": {
      "small": "img/logo.svg",
      "large": "img/logo.svg"
    },
    "version": "1.0.0"
  },
  "dependencies": {
    "grafanaDependency": ">=10.0.0",
    "plugins": []
  },
  "includes": [
    {
      "type": "dashboard",
      "name": "Jira Overview",
      "path": "dashboards/overview.json"
    }
  ]
}
```

---

## 10. Development & Build

### 10.1 Scaffolding

```bash
npx @grafana/create-plugin@latest
# Select: datasource, with backend
# Plugin ID: jira-datasource
```

Then overlay our custom code onto the generated scaffold.

### 10.2 Development Workflow

```bash
# Frontend — watch mode
npm install
npm run dev

# Backend — build
mage -v build:linux

# Start Grafana locally
docker compose up -d

# Run tests
npm test              # Jest
go test ./pkg/...     # Go tests
npm run e2e           # Playwright
```

### 10.3 Docker Compose (local dev)

```yaml
services:
  grafana:
    image: grafana/grafana-enterprise:latest
    ports: ["3000:3000"]
    volumes:
      - ./dist:/var/lib/grafana/plugins/jira-datasource
      - ./provisioning:/etc/grafana/provisioning
    environment:
      GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS: jira-datasource
      GF_LOG_LEVEL: debug
```

---

## 11. Implementation Phases

### Phase 1 — Foundation (MVP)
1. Scaffold project with `@grafana/create-plugin`
2. Implement Go backend skeleton (`NewDatasource`, `CheckHealth`, `QueryData`, `CallResource`)
3. Implement Jira HTTP client with Basic Auth + Bearer Token
4. Implement JQL Search query type (table output)
5. Implement ConfigEditor (URL, auth type, credentials)
6. Implement QueryEditor (query type selector, JQL input)
7. Health check via `/rest/api/{v}/myself`
8. Docker Compose dev environment
9. Basic unit tests (Go + Jest)

### Phase 2 — Core Query Types
1. Implement Issue Count query type (time series)
2. Implement Cycle Time query type (with changelog parsing)
3. Implement Changelog query type (raw changelog table)
4. Implement server-side caching
5. Pagination with concurrent fetching
6. Rate limit handling (429 retry with backoff)
7. Extend QueryEditor with conditional fields per query type

### Phase 3 — Advanced Features
1. Implement Worklog query type
2. Implement template variable support (`CallResource` + `VariableQueryEditor`)
3. Annotation support (mark events on graphs from Jira)
4. Bundled example dashboard (`overview.json`)
5. API version toggle (v2/v3) with Cloud-specific endpoints (bulk changelog)
6. E2E tests with Playwright
7. CI/CD pipeline (GitHub Actions)

### Phase 4 — Polish & Release
1. Plugin signing
2. README with screenshots and usage docs
3. Logo / branding
4. Grafana plugin validator pass
5. Publish to Grafana plugin catalog (optional)

---

## 12. Key Design Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Backend vs Frontend-only | **Backend (Go)** | Server-side processing, secure credential handling, alerting support, no browser limitations |
| Jira API version | **v2 default, v3 optional** | v2 is universal (Cloud + Server); v3 is Cloud-only with extra features |
| Caching | **Server-side in-memory** | Faster than client-side, shared across users, no IndexedDB issues |
| Pagination | **Concurrent bounded fetch** | Performance with safety — max 5 parallel requests |
| Auth storage | **Grafana secureJsonData** | Industry standard — encrypted at rest, never sent to browser |
| Query editor UX | **Query type selector + conditional fields** | Clean UI, only shows relevant options per query type |
| Template variables | **CallResource + dedicated editor** | Standard Grafana pattern for backend datasources |

---

## 13. Risks & Mitigations

| Risk | Mitigation |
| --- | --- |
| Jira API rate limits | Exponential backoff, caching, configurable concurrency |
| Large result sets (10k+ issues) | Pagination with limits, configurable max results |
| Jira Server vs Cloud API differences | API version toggle, adapter pattern in Go client |
| Grafana SDK breaking changes | Pin SDK versions, follow migration guides |
| Plugin signing for distribution | Use `@grafana/sign-plugin`, or run unsigned in dev |
