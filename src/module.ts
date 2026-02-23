import { DataSourcePlugin } from '@grafana/data';
import { JiraDataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { VariableQueryEditor } from './components/VariableQueryEditor';
import { JiraQuery, JiraDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<JiraDataSource, JiraQuery, JiraDataSourceOptions>(JiraDataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor)
  .setVariableQueryEditor(VariableQueryEditor);
