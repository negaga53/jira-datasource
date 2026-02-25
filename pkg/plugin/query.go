package plugin

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// QueryData handles incoming queries routed by queryType.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()

	for _, q := range req.Queries {
		query, err := ParseQuery(q)
		if err != nil {
			response.Responses[q.RefID] = backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
			continue
		}

		var frames data.Frames
		switch query.QueryType {
		case QueryTypeJQLSearch:
			frames, err = d.handleJQLSearch(ctx, query, q.TimeRange)
		case QueryTypeIssueCount:
			frames, err = d.handleIssueCount(ctx, query, q.TimeRange)
		case QueryTypeCycleTime:
			frames, err = d.handleCycleTime(ctx, query, q.TimeRange)
		case QueryTypeChangelog:
			frames, err = d.handleChangelog(ctx, query, q.TimeRange)
		case QueryTypeWorklog:
			frames, err = d.handleWorklog(ctx, query, q.TimeRange)
		case QueryTypeVelocity:
			frames, err = d.handleVelocity(ctx, query, q.TimeRange)
		case QueryTypeCFD:
			frames, err = d.handleCFD(ctx, query, q.TimeRange)
		case QueryTypeSprintBurndown:
			frames, err = d.handleSprintBurndown(ctx, query, q.TimeRange)
		default:
			err = fmt.Errorf("unsupported query type: %s", query.QueryType)
		}

		if err != nil {
			response.Responses[q.RefID] = backend.ErrDataResponse(backend.StatusInternal, err.Error())
			continue
		}

		response.Responses[q.RefID] = backend.DataResponse{Frames: frames}
	}

	return response, nil
}
