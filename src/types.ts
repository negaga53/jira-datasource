import { DataQuery, DataSourceJsonData } from '@grafana/data';

export enum QueryType {
  JQL_SEARCH = 'jql_search',
  ISSUE_COUNT = 'issue_count',
  CYCLE_TIME = 'cycle_time',
  CHANGELOG = 'changelog',
  WORKLOG = 'worklog',
}

export interface JiraQuery extends DataQuery {
  queryType: QueryType;
  jql: string;
  startStatus?: string;
  endStatus?: string;
  quantile?: number;
  interval?: string;
  fields?: string[];
  expand?: string[];
  maxResults?: number;
}

export const defaultQuery: Partial<JiraQuery> = {
  queryType: QueryType.JQL_SEARCH,
  jql: '',
  maxResults: 1000,
};

export interface JiraDataSourceOptions extends DataSourceJsonData {
  url: string;
  authType: 'basic' | 'bearer';
  username?: string;
  apiVersion: '2' | '3';
  cacheTTLSeconds: number;
}

export interface JiraSecureJsonData {
  apiToken?: string;
  bearerToken?: string;
}

export interface SelectOption {
  value: string;
  label: string;
}

export interface VariableQuery {
  queryType: 'projects' | 'statuses' | 'fields' | 'issuetypes' | 'labels';
  projectKey?: string;
}
