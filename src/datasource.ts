import {
  DataQueryRequest,
  DataQueryResponse,
  DataSourceInstanceSettings,
  MetricFindValue,
} from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { JiraQuery, JiraDataSourceOptions, SelectOption, VariableQuery, defaultQuery } from './types';

export class JiraDataSource extends DataSourceWithBackend<JiraQuery, JiraDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<JiraDataSourceOptions>) {
    super(instanceSettings);
  }

  /** Apply template variable substitution to JQL before sending to backend. */
  applyTemplateVariables(query: JiraQuery, scopedVars: Record<string, { text: string; value: string }>): JiraQuery {
    const templateSrv = getTemplateSrv();
    return {
      ...defaultQuery,
      ...query,
      jql: templateSrv.replace(query.jql || '', scopedVars),
    };
  }

  /** Template variable support via CallResource. */
  async metricFindQuery(query: VariableQuery): Promise<MetricFindValue[]> {
    if (!query || !query.queryType) {
      return [];
    }

    let path: string;
    switch (query.queryType) {
      case 'projects':
        path = 'projects';
        break;
      case 'statuses':
        path = 'statuses';
        break;
      case 'fields':
        path = 'fields';
        break;
      case 'issuetypes':
        path = `issuetypes?project=${encodeURIComponent(query.projectKey || '')}`;
        break;
      case 'labels':
        path = 'fields';
        break;
      case 'boards':
        path = 'boards';
        break;
      case 'sprints':
        path = `sprints?board=${encodeURIComponent(query.boardId || '')}`;
        break;
      default:
        return [];
    }

    const options = await this.getResource<SelectOption[]>(path);
    return (options || []).map((opt) => ({ text: opt.label, value: opt.value }));
  }
}
