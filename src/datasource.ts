import {
  DataQueryRequest,
  DataQueryResponse,
  DataSourceApi,
  DataSourceInstanceSettings,
  MetricFindValue,
} from '@grafana/data';
import { getBackendSrv, getTemplateSrv } from '@grafana/runtime';

import { JiraQuery, JiraDataSourceOptions, SelectOption, VariableQuery, defaultQuery } from './types';

export class JiraDataSource extends DataSourceApi<JiraQuery, JiraDataSourceOptions> {
  url: string;

  constructor(instanceSettings: DataSourceInstanceSettings<JiraDataSourceOptions>) {
    super(instanceSettings);
    this.url = instanceSettings.url || '';
  }

  /** Execute queries via the backend plugin. */
  async query(request: DataQueryRequest<JiraQuery>): Promise<DataQueryResponse> {
    const templateSrv = getTemplateSrv();

    // Replace template variables in JQL
    const queries = request.targets.map((target) => ({
      ...defaultQuery,
      ...target,
      jql: templateSrv.replace(target.jql, request.scopedVars),
    }));

    return getBackendSrv().datasourceRequest({
      method: 'POST',
      url: `${this.url}/query`,
      data: { ...request, targets: queries },
    });
  }

  /** Health check — delegates to backend CheckHealth. */
  async testDatasource() {
    return getBackendSrv()
      .fetch({
        method: 'GET',
        url: `${this.url}/health`,
      })
      .toPromise()
      .then(() => ({
        status: 'success' as const,
        message: 'Data source is working',
      }))
      .catch((err: Error) => ({
        status: 'error' as const,
        message: err.message || 'Unknown error',
      }));
  }

  /** Template variable support via CallResource. */
  async metricFindQuery(query: VariableQuery): Promise<MetricFindValue[]> {
    if (!query || !query.queryType) {
      return [];
    }

    let path: string;
    switch (query.queryType) {
      case 'projects':
        path = '/projects';
        break;
      case 'statuses':
        path = '/statuses';
        break;
      case 'fields':
        path = '/fields';
        break;
      case 'issuetypes':
        path = `/issuetypes?project=${encodeURIComponent(query.projectKey || '')}`;
        break;
      case 'labels':
        path = '/fields';
        break;
      default:
        return [];
    }

    const options = await this.getResource<SelectOption[]>(path);
    return (options || []).map((opt) => ({ text: opt.label, value: opt.value }));
  }

  /** Fetch resource from the backend plugin. */
  async getResource<T>(path: string): Promise<T> {
    return getBackendSrv().get(`${this.url}/resources${path}`);
  }
}
