// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {ConnectionTypes, useConnectionInstances} from '@thunderid/configure-connections';
import {
  Alert,
  Autocomplete,
  Chip,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useMemo, useState, type KeyboardEvent, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import useResourceFieldError from '../../../../hooks/useResourceFieldError';
import type {StepData} from '../../../../models/steps';

const toKeyList = (value: unknown): string[] =>
  Array.isArray(value) ? value.map((key) => String(key).trim()).filter((key) => key !== '') : [];

/**
 * `responseKeys` alone may also be stored as a comma-separated string; `inputKeys` and `runtimeDataKeys` are read only
 * as lists.
 */
const toResponseKeyList = (value: unknown): string[] => toKeyList(typeof value === 'string' ? value.split(',') : value);

// The server refuses an empty list at save, so a cleared list is removed rather than stored.
const toStoredKeyList = (keys: string[]): string[] | undefined => (keys.length > 0 ? keys : undefined);

const MIN_ATTEMPTS = 1;
const MAX_ATTEMPTS = 20;
const DEFAULT_MAX_ATTEMPTS = 5;

const parseMaxAttempts = (raw: string): number | null => {
  const parsed = raw.trim() === '' ? NaN : Number(raw);
  return Number.isInteger(parsed) && parsed >= MIN_ATTEMPTS && parsed <= MAX_ATTEMPTS ? parsed : null;
};

interface MaxAttemptsFieldProps {
  value: number;
  onCommit: (value: number) => void;
}

/**
 * Keeps the draft locally and commits on blur or Enter, so an out-of-range draft is shown with its message instead of
 * being stored.
 */
function MaxAttemptsField({value, onCommit}: MaxAttemptsFieldProps): ReactNode {
  const {t} = useTranslation();
  const [draft, setDraft] = useState<string>(String(value));
  const [syncedValue, setSyncedValue] = useState<number>(value);

  if (value !== syncedValue) {
    setSyncedValue(value);
    setDraft(String(value));
  }

  const parsed = parseMaxAttempts(draft);

  const commit = (): void => {
    if (parsed !== null && parsed !== value) {
      onCommit(parsed);
    }
  };

  return (
    <div>
      <FormLabel htmlFor="extension-max-attempts">
        {t('flows:core.executions.extension.maxAttempts.label', 'Max attempts')}
      </FormLabel>
      <TextField
        id="extension-max-attempts"
        type="number"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e: KeyboardEvent<HTMLDivElement>) => {
          if (e.key === 'Enter') {
            commit();
          }
        }}
        error={parsed === null}
        fullWidth
        size="small"
        inputProps={{min: MIN_ATTEMPTS, max: MAX_ATTEMPTS, step: 1}}
      />
      <FormHelperText error={parsed === null}>
        {parsed === null
          ? t('flows:core.executions.extension.maxAttempts.invalid', 'Enter a whole number from 1 to 20.')
          : t(
              'flows:core.executions.extension.maxAttempts.hint',
              'How many rejected answers this step accepts in one sign-in before it stops the flow.',
            )}
      </FormHelperText>
    </div>
  );
}

interface KeyListFieldProps {
  id: string;
  label: string;
  placeholder: string;
  hint: string;
  value: string[];
  onChange: (keys: string[]) => void;
}

function KeyListField({id, label, placeholder, hint, value, onChange}: KeyListFieldProps): ReactNode {
  return (
    <div>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <Autocomplete
        id={id}
        multiple
        freeSolo
        autoSelect
        clearOnBlur
        options={[] as string[]}
        value={value}
        onChange={(_event: SyntheticEvent, newValue: string[]) =>
          onChange([...new Set(newValue.map((key) => key.trim()).filter((key) => key !== ''))])
        }
        renderTags={(tags: string[], getTagProps) =>
          tags.map((tag: string, index: number) => {
            const {key, ...tagProps} = getTagProps({index});
            return <Chip key={key} size="small" label={tag} {...tagProps} />;
          })
        }
        renderInput={(params) => (
          <TextField {...params} size="small" placeholder={value.length === 0 ? placeholder : undefined} />
        )}
        fullWidth
        size="small"
      />
      <FormHelperText>{hint}</FormHelperText>
    </div>
  );
}

function ExtensionProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const connectionsQuery = useConnectionInstances(ConnectionTypes.HTTP);
  const connections = connectionsQuery.data ?? [];

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const connectionId = (properties['connectionId'] as string | undefined) ?? '';
  // Keeps a stored id visible when its connection was deleted or the list failed to load, so the step does not look unset.
  const connectionUnlisted =
    !connectionsQuery.isLoading && connectionId !== '' && !connections.some((c) => c.id === connectionId);
  const noConnections = connectionsQuery.isSuccess && connections.length === 0;
  const connectionError: string = useResourceFieldError(resource?.id, 'data.properties.connectionId');
  const namespacePrefix = `extension.${resource?.id ?? t('flows:core.executions.extension.nodeIdPlaceholder', '<node ID>')}.`;
  const storedMaxAttempts = properties['maxAttempts'];
  const maxAttempts = typeof storedMaxAttempts === 'number' ? storedMaxAttempts : DEFAULT_MAX_ATTEMPTS;

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.extension.description',
          'Calls your service through a service endpoint connection and routes the flow on its answer.',
        )}
      </Typography>

      <div>
        <FormLabel id="extension-connection-label" htmlFor="extension-connection" required>
          {t('flows:core.executions.extension.connection.label', 'Connection')}
        </FormLabel>
        <Select
          id="extension-connection"
          labelId="extension-connection-label"
          value={connectionsQuery.isLoading ? '' : connectionId}
          onChange={(e) => onChange('data.properties.connectionId', e.target.value, resource)}
          displayEmpty
          fullWidth
          size="small"
          disabled={connectionsQuery.isLoading || noConnections}
          error={!!connectionError}
        >
          <MenuItem value="" disabled>
            {connectionsQuery.isLoading
              ? t('common:status.loading', 'Loading...')
              : t('flows:core.executions.extension.connection.placeholder', 'Select a service endpoint connection')}
          </MenuItem>
          {connections.map((connection) => (
            <MenuItem key={connection.id} value={connection.id}>
              {connection.name}
            </MenuItem>
          ))}
          {connectionUnlisted && <MenuItem value={connectionId}>{connectionId}</MenuItem>}
        </Select>
        <FormHelperText error={!!connectionError}>
          {connectionError ||
            t(
              'flows:core.executions.extension.connection.hint',
              'The URL, headers, timeout and authentication come from the connection.',
            )}
        </FormHelperText>
      </div>

      {connectionsQuery.error && (
        <QueryErrorNotice
          error={connectionsQuery.error}
          t={t}
          variant="inline"
          fallbackKey="flows:core.executions.extension.connection.loadError"
          fallbackDefaultValue="Failed to load service endpoint connections."
          onRetry={() => void connectionsQuery.refetch()}
        />
      )}

      {noConnections && (
        <Alert severity="warning">
          {t(
            'flows:core.executions.extension.connection.noConnections',
            'No service endpoint connections exist. Create one under Connections, then select it here.',
          )}
        </Alert>
      )}

      <KeyListField
        id="extension-input-keys"
        label={t('flows:core.executions.extension.inputKeys.label', 'Inputs to send')}
        placeholder={t('flows:core.executions.extension.inputKeys.placeholder', 'username')}
        hint={t(
          'flows:core.executions.extension.inputKeys.hint',
          'User inputs to send to the service. None are sent by default. Add a password or code only when the service needs it.',
        )}
        value={toKeyList(properties['inputKeys'])}
        onChange={(keys) => onChange('data.properties.inputKeys', toStoredKeyList(keys), resource)}
      />

      <KeyListField
        id="extension-runtime-data-keys"
        label={t('flows:core.executions.extension.runtimeDataKeys.label', 'Runtime data to send')}
        placeholder={t('flows:core.executions.extension.runtimeDataKeys.placeholder', 'extension.risk_check.riskScore')}
        hint={t(
          'flows:core.executions.extension.runtimeDataKeys.hint',
          'Runtime data keys to send. Name data an earlier extension stored by its full key, such as extension.risk_check.riskScore. When empty, only the application ID and user ID are sent. Secret flow state is never sent.',
        )}
        value={toKeyList(properties['runtimeDataKeys'])}
        onChange={(keys) => onChange('data.properties.runtimeDataKeys', toStoredKeyList(keys), resource)}
      />

      <KeyListField
        id="extension-response-keys"
        label={t('flows:core.executions.extension.responseKeys.label', 'Stored response fields')}
        placeholder={t('flows:core.executions.extension.responseKeys.placeholder', 'riskScore')}
        hint={t('flows:core.executions.extension.responseKeys.hint', {
          defaultValue:
            'Top-level response fields to store for later steps, stored under {{prefix}}. Later steps refer to a field by its full key, such as {{prefix}}riskScore. When empty, every top-level field is stored.',
          prefix: namespacePrefix,
        })}
        value={toResponseKeyList(properties['responseKeys'])}
        onChange={(keys) => onChange('data.properties.responseKeys', toStoredKeyList(keys), resource)}
      />

      <MaxAttemptsField
        key={resource?.id}
        value={maxAttempts}
        onCommit={(value) => onChange('data.properties.maxAttempts', value, resource)}
      />

      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.extension.contract.hint',
          'The service must answer with actionStatus SUCCESS, FAILED, INCOMPLETE or ERROR.',
        )}
      </Typography>
    </Stack>
  );
}

export default ExtensionProperties;
