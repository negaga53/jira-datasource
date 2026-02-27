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
    const boardIdRaw = templateSrv.replace(String(query.boardId ?? ''), scopedVars);
    const sprintIdRaw = templateSrv.replace(String(query.sprintId ?? ''), scopedVars);
    return {
      ...defaultQuery,
      ...query,
      jql: templateSrv.replace(query.jql || '', scopedVars),
      boardId: boardIdRaw ? parseInt(boardIdRaw, 10) || undefined : undefined,
      sprintId: sprintIdRaw ? parseInt(sprintIdRaw, 10) || undefined : undefined,
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
        const boardId = getTemplateSrv().replace(query.boardId || '');
        path = `sprints?board=${encodeURIComponent(boardId)}`;
        break;
      case 'users':
        const projectKey = getTemplateSrv().replace(query.projectKey || '');
        path = `users?project=${encodeURIComponent(projectKey)}`;
        break;
      default:
        return [];
    }

    const options = await this.getResource<SelectOption[]>(path);
    return (options || []).map((opt) => ({ text: opt.label, value: opt.value }));
  }
}
