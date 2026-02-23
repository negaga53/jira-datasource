package plugin

import (
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

// registerRoutes sets up the resource handler routes.
func (d *Datasource) registerRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", d.handleProjects)
	mux.HandleFunc("/statuses", d.handleStatuses)
	mux.HandleFunc("/fields", d.handleFields)
	mux.HandleFunc("/issuetypes", d.handleIssueTypes)
	return mux
}

// newResourceHandler creates a CallResource handler from the HTTP mux.
func (d *Datasource) newResourceHandler() backend.CallResourceHandler {
	return httpadapter.New(d.registerRoutes())
}

func (d *Datasource) handleProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check cache
	if cached, ok := d.cache.Get("projects"); ok {
		writeJSON(w, cached)
		return
	}

	projects, err := d.jiraClient.GetProjects(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	options := make([]SelectOption, len(projects))
	for i, p := range projects {
		options[i] = SelectOption{Value: p.Key, Label: p.Name}
	}

	d.cache.Set("projects", options)
	writeJSON(w, options)
}

func (d *Datasource) handleStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if cached, ok := d.cache.Get("statuses"); ok {
		writeJSON(w, cached)
		return
	}

	statuses, err := d.jiraClient.GetStatuses(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Deduplicate statuses by name
	seen := make(map[string]bool)
	var options []SelectOption
	for _, s := range statuses {
		if !seen[s.Name] {
			seen[s.Name] = true
			options = append(options, SelectOption{Value: s.Name, Label: s.Name})
		}
	}

	d.cache.Set("statuses", options)
	writeJSON(w, options)
}

func (d *Datasource) handleFields(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if cached, ok := d.cache.Get("fields"); ok {
		writeJSON(w, cached)
		return
	}

	fields, err := d.jiraClient.GetFields(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	options := make([]SelectOption, len(fields))
	for i, f := range fields {
		options[i] = SelectOption{Value: f.ID, Label: f.Name}
	}

	d.cache.Set("fields", options)
	writeJSON(w, options)
}

func (d *Datasource) handleIssueTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	project := r.URL.Query().Get("project")
	if project == "" {
		http.Error(w, "project query parameter is required", http.StatusBadRequest)
		return
	}

	cacheKey := "issuetypes:" + project
	if cached, ok := d.cache.Get(cacheKey); ok {
		writeJSON(w, cached)
		return
	}

	issueTypes, err := d.jiraClient.GetIssueTypes(ctx, project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	options := make([]SelectOption, len(issueTypes))
	for i, t := range issueTypes {
		options[i] = SelectOption{Value: t.Name, Label: t.Name}
	}

	d.cache.Set(cacheKey, options)
	writeJSON(w, options)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
