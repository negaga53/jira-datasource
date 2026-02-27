import React, { ChangeEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { InlineField, Input, Select, MultiSelect, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { getTemplateSrv } from '@grafana/runtime';
import { JiraDataSource } from '../datasource';
import { JiraDataSourceOptions, JiraQuery, QueryType, SelectOption, defaultQuery } from '../types';

type Props = QueryEditorProps<JiraDataSource, JiraQuery, JiraDataSourceOptions>;

const queryTypeOptions: Array<SelectableValue<QueryType>> = [
  { label: 'JQL Search', value: QueryType.JQL_SEARCH, description: 'Search issues via JQL, returns a table' },
  { label: 'Issue Count', value: QueryType.ISSUE_COUNT, description: 'Count issues over time (time series)' },
  { label: 'Cycle Time', value: QueryType.CYCLE_TIME, description: 'Compute cycle time between two statuses' },
  { label: 'Changelog', value: QueryType.CHANGELOG, description: 'Raw changelog entries for matching issues' },
  { label: 'Worklog', value: QueryType.WORKLOG, description: 'Worklogs for matching issues' },
  { label: 'Velocity / Throughput', value: QueryType.VELOCITY, description: 'Issues or story points resolved per interval' },
  { label: 'Flow Load (CFD)', value: QueryType.CFD, description: 'Cumulative flow diagram: issues per status over time' },
  { label: 'Sprint Burndown', value: QueryType.SPRINT_BURNDOWN, description: 'Sprint burndown chart with ideal line' },
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
  const [boardOptions, setBoardOptions] = useState<Array<SelectableValue<string>>>([]);
  const [sprintOptions, setSprintOptions] = useState<Array<SelectableValue<string>>>([]);

  // Load statuses and fields from backend
  useEffect(() => {
    datasource.getResource<SelectOption[]>('statuses').then((opts) => {
      setStatusOptions((opts || []).map((o) => ({ label: o.label, value: o.value })));
    }).catch(() => {});

    datasource.getResource<SelectOption[]>('fields').then((opts) => {
      setFieldOptions((opts || []).map((o) => ({ label: o.label, value: o.value })));
    }).catch(() => {});

    datasource.getResource<SelectOption[]>('boards').then((opts) => {
      setBoardOptions((opts || []).map((o) => ({ label: o.label, value: o.value })));
    }).catch(() => {});
  }, [datasource]);

  // Load sprints when board changes (resolves template variables)
  useEffect(() => {
    if (!q.boardId) {
      setSprintOptions([]);
      return;
    }

    const boardIdStr = String(q.boardId);
    const resolved = boardIdStr.includes('$') ? getTemplateSrv().replace(boardIdStr) : boardIdStr;
    const boardIdNum = parseInt(resolved, 10);

    if (!isNaN(boardIdNum)) {
      datasource
        .getResource<SelectOption[]>('sprints', { board: String(boardIdNum) })
        .then((opts) => {
          setSprintOptions((opts || []).map((o) => ({ label: o.label, value: o.value })));
        })
        .catch(() => setSprintOptions([]));
    } else {
      setSprintOptions([]);
    }
  }, [datasource, q.boardId]);

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

      {/* Velocity / Throughput fields */}
      {q.queryType === QueryType.VELOCITY && (
        <>
          <InlineField label="Interval" labelWidth={16} tooltip="Time interval for bucketing">
            <Select
              width={20}
              options={intervalOptions}
              value={intervalOptions.find((o) => o.value === (q.interval || '1w'))}
              onChange={(v) => onFieldChange('interval', v.value || '1w')}
            />
          </InlineField>
          <InlineField label="Story Point" labelWidth={16} tooltip="Custom field for story points (optional, leave empty for count only)">
            <Select
              width={40}
              options={fieldOptions}
              value={fieldOptions.find((o) => o.value === q.storyPointField)}
              onChange={(v) => onFieldChange('storyPointField', v.value || '')}
              isClearable
              placeholder="Select story point field..."
            />
          </InlineField>
        </>
      )}

      {/* CFD fields */}
      {q.queryType === QueryType.CFD && (
        <>
          <InlineField label="Interval" labelWidth={16} tooltip="Time interval for status snapshots">
            <Select
              width={20}
              options={intervalOptions}
              value={intervalOptions.find((o) => o.value === (q.interval || '1d'))}
              onChange={(v) => onFieldChange('interval', v.value || '1d')}
            />
          </InlineField>
          <InlineField label="Story Point" labelWidth={16} tooltip="Use story points instead of issue count (optional)">
            <Select
              width={40}
              options={fieldOptions}
              value={fieldOptions.find((o) => o.value === q.storyPointField)}
              onChange={(v) => onFieldChange('storyPointField', v.value || '')}
              isClearable
              placeholder="Count by issues (default)"
            />
          </InlineField>
        </>
      )}

      {/* Sprint Burndown fields */}
      {q.queryType === QueryType.SPRINT_BURNDOWN && (
        <>
          <InlineField label="Board" labelWidth={16} tooltip="Jira Agile board (supports variables like $board)">
            <Select
              width={40}
              options={boardOptions}
              value={
                boardOptions.find((o) => o.value === String(q.boardId || '')) ??
                (q.boardId != null ? { label: String(q.boardId), value: String(q.boardId) } : null)
              }
              onChange={(v) => onFieldChange('boardId', v?.value || undefined)}
              isClearable
              allowCustomValue
              placeholder="Select board or type $variable..."
            />
          </InlineField>
          <InlineField label="Sprint" labelWidth={16} tooltip="Sprint to chart (supports variables like $sprint)">
            <Select
              width={40}
              options={sprintOptions}
              value={
                sprintOptions.find((o) => o.value === String(q.sprintId || '')) ??
                (q.sprintId != null ? { label: String(q.sprintId), value: String(q.sprintId) } : null)
              }
              onChange={(v) => onFieldChange('sprintId', v?.value || undefined)}
              isClearable
              allowCustomValue
              placeholder="Select sprint or type $variable..."
            />
          </InlineField>
          <InlineField label="Story Point" labelWidth={16} tooltip="Use story points instead of issue count (optional)">
            <Select
              width={40}
              options={fieldOptions}
              value={fieldOptions.find((o) => o.value === q.storyPointField)}
              onChange={(v) => onFieldChange('storyPointField', v.value || '')}
              isClearable
              placeholder="Count by issues (default)"
            />
          </InlineField>
          <InlineField label="Done Statuses" labelWidth={16} tooltip="Statuses that count as done (auto-detected if empty)">
            <MultiSelect
              width={40}
              options={statusOptions}
              value={(q.doneStatuses || []).map((s) => {
                const opt = statusOptions.find((o) => o.value === s);
                return { label: opt?.label || s, value: s };
              })}
              onChange={(vals) => onFieldChange('doneStatuses', vals.map((v) => v.value || ''))}
              isClearable
              placeholder="Auto-detect from Jira"
            />
          </InlineField>
        </>
      )}
    </>
  );
}
