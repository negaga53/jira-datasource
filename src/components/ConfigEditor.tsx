import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, Select } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';
import { JiraDataSourceOptions, JiraSecureJsonData } from '../types';

type Props = DataSourcePluginOptionsEditorProps<JiraDataSourceOptions, JiraSecureJsonData>;

const authTypeOptions: Array<SelectableValue<string>> = [
  { label: 'Basic Auth (Email + API Token)', value: 'basic' },
  { label: 'Bearer Token (PAT)', value: 'bearer' },
];

const apiVersionOptions: Array<SelectableValue<string>> = [
  { label: 'v2 (Server/DC & Cloud)', value: '2' },
  { label: 'v3 (Cloud only)', value: '3' },
];

export function ConfigEditor(props: Props) {
  const { options, onOptionsChange } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onJSONDataChange = <K extends keyof JiraDataSourceOptions>(key: K, value: JiraDataSourceOptions[K]) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, [key]: value },
    });
  };

  const onSecureChange = (key: keyof JiraSecureJsonData, value: string) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, [key]: value },
    });
  };

  const onResetSecret = (key: keyof JiraSecureJsonData) => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, [key]: false },
      secureJsonData: { ...secureJsonData, [key]: '' },
    });
  };

  return (
    <>
      <h3 className="page-heading">Jira Connection</h3>

      <InlineField label="Jira URL" labelWidth={20} tooltip="Base URL of your Jira instance (e.g. https://mycompany.atlassian.net)">
        <Input
          width={40}
          value={jsonData.url || ''}
          placeholder="https://mycompany.atlassian.net"
          onChange={(e: ChangeEvent<HTMLInputElement>) => onJSONDataChange('url', e.target.value)}
        />
      </InlineField>

      <InlineField label="API Version" labelWidth={20} tooltip="v2 works for both Cloud and Server. v3 is Cloud-only with extra features.">
        <Select
          width={40}
          options={apiVersionOptions}
          value={apiVersionOptions.find((o) => o.value === (jsonData.apiVersion || '2'))}
          onChange={(v) => onJSONDataChange('apiVersion', (v.value as '2' | '3') || '2')}
        />
      </InlineField>

      <h3 className="page-heading">Authentication</h3>

      <InlineField label="Auth Type" labelWidth={20}>
        <Select
          width={40}
          options={authTypeOptions}
          value={authTypeOptions.find((o) => o.value === (jsonData.authType || 'basic'))}
          onChange={(v) => onJSONDataChange('authType', (v.value as 'basic' | 'bearer') || 'basic')}
        />
      </InlineField>

      {(jsonData.authType || 'basic') === 'basic' && (
        <>
          <InlineField label="Username (Email)" labelWidth={20} tooltip="Email address used for Jira authentication">
            <Input
              width={40}
              value={jsonData.username || ''}
              placeholder="user@company.com"
              onChange={(e: ChangeEvent<HTMLInputElement>) => onJSONDataChange('username', e.target.value)}
            />
          </InlineField>
          <InlineField label="API Token" labelWidth={20} tooltip="Jira API token (generate at https://id.atlassian.com/manage-profile/security/api-tokens)">
            <SecretInput
              width={40}
              isConfigured={secureJsonFields?.apiToken || false}
              value={secureJsonData?.apiToken || ''}
              onReset={() => onResetSecret('apiToken')}
              onChange={(e: ChangeEvent<HTMLInputElement>) => onSecureChange('apiToken', e.target.value)}
            />
          </InlineField>
        </>
      )}

      {jsonData.authType === 'bearer' && (
        <InlineField label="Bearer Token" labelWidth={20} tooltip="Personal Access Token for Jira Server/DC">
          <SecretInput
            width={40}
            isConfigured={secureJsonFields?.bearerToken || false}
            value={secureJsonData?.bearerToken || ''}
            onReset={() => onResetSecret('bearerToken')}
            onChange={(e: ChangeEvent<HTMLInputElement>) => onSecureChange('bearerToken', e.target.value)}
          />
        </InlineField>
      )}

      <h3 className="page-heading">Advanced</h3>

      <InlineField label="Cache TTL (seconds)" labelWidth={20} tooltip="How long to cache Jira API responses on the server side">
        <Input
          width={40}
          type="number"
          value={jsonData.cacheTTLSeconds ?? 300}
          onChange={(e: ChangeEvent<HTMLInputElement>) =>
            onJSONDataChange('cacheTTLSeconds', parseInt(e.target.value, 10) || 300)
          }
        />
      </InlineField>
    </>
  );
}
