# Jira Datasource Plugin for Grafana

A Grafana backend datasource plugin that connects to Jira Cloud/Server via REST API. Query issues, track cycle time, visualize changelogs, worklogs, and issue counts — all from your Grafana dashboards.

## Features

- **JQL Search** — Query issues with full JQL support, displayed as tables
- **Issue Count** — Time series showing issue creation over time
- **Cycle Time** — Measure duration between status transitions (agile metrics)
- **Changelog** — Audit trail of field changes across issues
- **Worklog** — Time tracking data per issue and author
- **Velocity / Throughput** — Resolved issues or story points per time interval
- **Flow Load (CFD)** — Cumulative Flow Diagram showing issue count or story points per status over time
- **Sprint Burndown** — Remaining work vs ideal burndown line for a sprint
- **Template Variables** — Dynamic dashboards with project, status, field, board, and sprint selectors
- **Server-side Caching** — Configurable TTL to reduce API calls
- **Alerting Support** — Backend queries compatible with Grafana alerting

## Authentication

| Method | Jira Cloud | Jira Server/DC |
| --- | --- | --- |
| Basic Auth (email + API token) | ✅ | ✅ |
| Bearer Token (PAT) | ❌ | ✅ |

## Getting Started

### Prerequisites

- Grafana 10.0+
- Go 1.22+ (for backend)
- Node.js 20+ (for frontend)

### Development

```bash
# Install frontend dependencies
npm install

# Start frontend in watch mode
npm run dev

# Build backend
CGO_ENABLED=0 go build -o dist/gpx_jira-datasource_linux_amd64 ./pkg

# Start Grafana with the plugin
docker compose up -d
```

Open Grafana at http://localhost:3000 and add the Jira datasource.

### Configuration

1. Go to **Connections → Data sources → Add data source**
2. Search for **Jira**
3. Configure:
   - **Jira URL**: Your Jira instance URL (e.g., `https://mycompany.atlassian.net`)
   - **Auth Type**: Basic Auth or Bearer Token
   - **Credentials**: Email + API Token, or Personal Access Token
   - **API Version**: v2 (Server/DC + Cloud) or v3 (Cloud only)

### Query Types

| Type | Output | Use Case |
| --- | --- | --- |
| JQL Search | Table | Issue lists, stat panels |
| Issue Count | Time Series | Trend charts |
| Cycle Time | Table | Agile metrics, scatter plots |
| Changelog | Table | Audit logs, flow analysis |
| Worklog | Table | Time tracking |
| Velocity / Throughput | Time Series | Resolved issues or story points per interval |
| Flow Load (CFD) | Time Series (stacked) | Cumulative flow diagram by status |
| Sprint Burndown | Time Series | Remaining vs ideal work in a sprint |

### Template Variables

Use the variable query editor with these functions:
- `projects()` — All project keys
- `statuses()` — All available statuses
- `fields()` — All field names
- `issuetypes(PROJECT_KEY)` — Issue types for a project
- `boards()` — All Jira Agile boards
- `sprints(BOARD_ID)` — Sprints for a board

## Building

```bash
# Frontend
npm run build

# Backend (all platforms)
# Requires mage: go install github.com/magefile/mage@latest
mage buildAll
```

## License

Apache 2.0
