import React, { ChangeEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { InlineField, Input, Select, MultiSelect, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { JiraDataSource } from '../datasource';
import { JiraDataSourceOptions, JiraQuery, QueryType, SelectOption, defaultQuery } from '../types';

type Props = QueryEditorProps<JiraDataSource, JiraQuery, JiraDataSourceOptions>;

const queryTypeOptions: Array<SelectableValue<QueryType>> = [
  { label: 'JQL Search', value: QueryType.JQL_SEARCH, description: 'Search issues via JQL, returns a table' },
  { label: 'Issue Count', value: QueryType.ISSUE_COUNT, description: 'Count issues over time (time series)' },
  { label: 'Cycle Time', value: QueryType.CYCLE_TIME, description: 'Compute cycle time between two statuses' },
  { label: 'Changelog', value: QueryType.CHANGELOG, description: 'Raw changelog entries for matching issues' },
  { label: 'Worklog', value: QueryType.WORKLOG, description: 'Worklogs for matching issues' },
];

const intervalOptions: Array<SelectableValue<string>> = [
  { label: '1 hour', value: '1h' },
  { label: '1 day', value: '1d' },
  { label: '1 week', value: '1w' },
  { label: '1 month', value: '1M' },
];

export function QueryEditor(props: Props) {
  const { query, onChange, onRunQuery, datasource } = props;
  const q = useMemo(() => ({ ...defaultQuery, ...query } as JiraQuery), [query]);

  const [statusOptions, setStatusOptions] = useState<Array<SelectableValue<string>>>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);

  // Load statuses and fields from backend
  useEffect(() => {
    datasource.getResource<SelectOption[]>('statuses').then((opts) => {
      setStatusOptions((opts || []).map((o) => ({ label: o.label, value: o.value })));
    }).catch(() => {});

    datasource.getResource<SelectOption[]>('fields').then((opts) => {
      setFieldOptions((opts || []).map((o) => ({ label: o.label, value: o.value })));
    }).catch(() => {});
  }, [datasource]);

  const onFieldChange = useCallback(
    <K extends keyof JiraQuery>(key: K, value: JiraQuery[K]) => {
      onChange({ ...q, [key]: value });
      onRunQuery();
    },
    [q, onChange, onRunQuery]
  );

  return (
    <>
      <InlineField label="Query Type" labelWidth={16}>
        <Select
          width={30}
          options={queryTypeOptions}
          value={queryTypeOptions.find((o) => o.value === q.queryType)}
          onChange={(v) => onFieldChange('queryType', v.value || QueryType.JQL_SEARCH)}
        />
      </InlineField>

      <InlineField label="JQL" labelWidth={16} grow tooltip="Jira Query Language expression. Supports Grafana variables like $project">
        <TextArea
          value={q.jql || ''}
          placeholder='project = "MYPROJ" AND status = "Open"'
          rows={3}
          onChange={(e: ChangeEvent<HTMLTextAreaElement>) => onFieldChange('jql', e.target.value)}
        />
      </InlineField>

      {/* Cycle Time fields */}
      {q.queryType === QueryType.CYCLE_TIME && (
        <>
          <InlineField label="Start Status" labelWidth={16} tooltip="Status that starts the cycle">
            <Select
              width={30}
              options={statusOptions}
              value={statusOptions.find((o) => o.value === q.startStatus)}
              onChange={(v) => onFieldChange('startStatus', v.value || '')}
              isClearable
            />
          </InlineField>
          <InlineField label="End Status" labelWidth={16} tooltip="Status that ends the cycle">
            <Select
              width={30}
              options={statusOptions}
              value={statusOptions.find((o) => o.value === q.endStatus)}
              onChange={(v) => onFieldChange('endStatus', v.value || '')}
              isClearable
            />
          </InlineField>
          <InlineField label="Quantile (%)" labelWidth={16} tooltip="Percentile to compute (e.g. 85 for p85)">
            <Input
              width={15}
              type="number"
              min={1}
              max={100}
              value={q.quantile ?? ''}
              placeholder="85"
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                onFieldChange('quantile', parseInt(e.target.value, 10) || undefined)
              }
            />
          </InlineField>
        </>
      )}

      {/* Issue Count fields */}
      {q.queryType === QueryType.ISSUE_COUNT && (
        <InlineField label="Interval" labelWidth={16} tooltip="Time interval for bucketing">
          <Select
            width={20}
            options={intervalOptions}
            value={intervalOptions.find((o) => o.value === (q.interval || '1d'))}
            onChange={(v) => onFieldChange('interval', v.value || '1d')}
          />
        </InlineField>
      )}

      {/* Changelog field filter */}
      {q.queryType === QueryType.CHANGELOG && (
        <InlineField label="Field Filter" labelWidth={16} tooltip="Only show changes for these fields (leave empty for all)">
          <MultiSelect
            width={40}
            options={fieldOptions}
            value={(q.fields || []).map((f) => {
              const opt = fieldOptions.find((o) => o.value === f);
              return { label: opt?.label || f, value: f };
            })}
            onChange={(vals) => onFieldChange('fields', vals.map((v) => v.value || ''))}
            isClearable
          />
        </InlineField>
      )}

      {/* JQL Search field selector */}
      {q.queryType === QueryType.JQL_SEARCH && (
        <InlineField label="Fields" labelWidth={16} tooltip="Fields to include in results (leave empty for defaults)">
          <MultiSelect
            width={40}
            options={fieldOptions}
            value={(q.fields || []).map((f) => {
              const opt = fieldOptions.find((o) => o.value === f);
              return { label: opt?.label || f, value: f };
            })}
            onChange={(vals) => onFieldChange('fields', vals.map((v) => v.value || ''))}
            isClearable
          />
        </InlineField>
      )}
    </>
  );
}
