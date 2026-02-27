import React, { ChangeEvent } from 'react';
import { InlineField, Input, Select } from '@grafana/ui';
import { SelectableValue } from '@grafana/data';
import { VariableQuery } from '../types';

interface Props {
  query: VariableQuery;
  onChange: (query: VariableQuery) => void;
}

const variableQueryTypeOptions: Array<SelectableValue<VariableQuery['queryType']>> = [
  { label: 'Projects', value: 'projects' },
  { label: 'Statuses', value: 'statuses' },
  { label: 'Fields', value: 'fields' },
  { label: 'Issue Types', value: 'issuetypes' },
  { label: 'Labels', value: 'labels' },
  { label: 'Boards', value: 'boards' },
  { label: 'Sprints', value: 'sprints' },
  { label: 'Users', value: 'users' },
];

export function VariableQueryEditor({ query, onChange }: Props) {
  return (
    <>
      <InlineField label="Query Type" labelWidth={16}>
        <Select
          width={30}
          options={variableQueryTypeOptions}
          value={variableQueryTypeOptions.find((o) => o.value === query.queryType)}
          onChange={(v) => onChange({ ...query, queryType: v.value || 'projects' })}
        />
      </InlineField>

      {query.queryType === 'issuetypes' && (
        <InlineField label="Project Key" labelWidth={16} tooltip="Project to fetch issue types for">
          <Input
            width={20}
            value={query.projectKey || ''}
            placeholder="PROJ"
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              onChange({ ...query, projectKey: e.target.value })
            }
          />
        </InlineField>
      )}

      {query.queryType === 'sprints' && (
        <InlineField label="Board ID" labelWidth={16} tooltip="Board ID to fetch sprints for (supports variables like $board)">
          <Input
            width={20}
            value={query.boardId || ''}
            placeholder="$board or 42"
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              onChange({ ...query, boardId: e.target.value })
            }
          />
        </InlineField>
      )}
    </>
  );
}
