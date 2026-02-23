package plugin

import (
	"context"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Ensure Datasource implements required interfaces.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is the main plugin struct.
type Datasource struct {
	jiraClient      *JiraClient
	cache           *Cache
	settings        JiraSettings
	resourceHandler backend.CallResourceHandler
}

// NewDatasource creates a new datasource instance from Grafana settings.
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	jiraSettings, err := ParseSettings(settings)
	if err != nil {
		return nil, err
	}

	secrets := settings.DecryptedSecureJSONData

	client := NewJiraClient(jiraSettings, secrets)
	cache := NewCache(time.Duration(jiraSettings.CacheTTLSeconds) * time.Second)

	ds := &Datasource{
		jiraClient: client,
		cache:      cache,
		settings:   jiraSettings,
	}
	ds.resourceHandler = ds.newResourceHandler()

	log.DefaultLogger.Info("Jira datasource initialized", "url", jiraSettings.URL, "authType", jiraSettings.AuthType)

	return ds, nil
}

// CallResource handles resource requests (for template variables, async selects).
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

// Dispose cleans up resources when the datasource is removed or settings change.
func (d *Datasource) Dispose() {
	d.cache.Close()
	log.DefaultLogger.Info("Jira datasource disposed")
}
