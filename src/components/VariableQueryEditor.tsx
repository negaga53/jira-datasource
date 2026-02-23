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
    </>
  );
}
