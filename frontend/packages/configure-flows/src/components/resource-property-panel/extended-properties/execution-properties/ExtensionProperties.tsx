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
import {useMemo, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import useResourceFieldError from '../../../../hooks/useResourceFieldError';
import type {StepData} from '../../../../models/steps';

const toKeyList = (value: unknown): string[] =>
  Array.isArray(value) ? value.map((key) => String(key).trim()).filter((key) => key !== '') : [];

/**
 * `responseKeys` alone may also be stored as a comma-separated string; `inputKeys` and `runtimeDataKeys` are read only
 * as lists.
 */
const toResponseKeyList = (value: unknown): string[] => toKeyList(typeof value === 'string' ? value.split(',') : value);

const DEFAULT_MAX_ATTEMPTS = 5;

const RUNTIME_DATA_USER_ID = 'userID';
const RUNTIME_DATA_EXTENSION_PREFIX = 'extension.';

// Mirrors the server's allow list: only the user ID and data stored by an earlier extension may be sent.
const isSendableRuntimeDataKey = (key: string): boolean =>
  key === RUNTIME_DATA_USER_ID || key.startsWith(RUNTIME_DATA_EXTENSION_PREFIX);

interface KeyListFieldProps {
  id: string;
  label: string;
  placeholder: string;
  hint: string;
  error?: string;
  value: string[];
  onChange: (keys: string[]) => void;
}

function KeyListField({
  id,
  label,
  placeholder,
  hint,
  error = undefined,
  value,
  onChange,
}: KeyListFieldProps): ReactNode {
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
          <TextField
            {...params}
            size="small"
            placeholder={value.length === 0 ? placeholder : undefined}
            error={!!error}
          />
        )}
        fullWidth
        size="small"
      />
      <FormHelperText error={!!error}>{error ?? hint}</FormHelperText>
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
  const connectionUnlisted =
    !connectionsQuery.isLoading && connectionId !== '' && !connections.some((c) => c.id === connectionId);
  const connectionDangling = connectionsQuery.isSuccess && connectionUnlisted;
  const noConnections = connectionsQuery.isSuccess && connections.length === 0;
  const connectionFieldError: string = useResourceFieldError(resource?.id, 'data.properties.connectionId');
  const connectionError: string =
    connectionFieldError ||
    (connectionDangling
      ? t(
          'flows:core.executions.extension.connection.dangling',
          'This connection no longer exists. Select another connection.',
        )
      : '');
  const namespacePrefix = `extension.${resource?.id ?? t('flows:core.executions.extension.nodeIdPlaceholder', '<node ID>')}.`;
  const runtimeDataKeys = toKeyList(properties['runtimeDataKeys']);
  const unsendableRuntimeDataKeys = runtimeDataKeys.filter((key) => !isSendableRuntimeDataKey(key));

  // The server refuses an empty list at save, so a cleared list is removed from the properties rather than stored.
  const handleKeyListChange = (property: string, keys: string[]): void => {
    if (keys.length > 0) {
      onChange(`data.properties.${property}`, keys, resource);
      return;
    }
    onChange(
      'data.properties',
      Object.fromEntries(Object.entries(properties).filter(([key]) => key !== property)),
      resource,
    );
  };

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
          value={connectionsQuery.isLoading || connectionDangling ? '' : connectionId}
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
          {/* Keeps a stored id visible when the list failed to load, so the step does not look unset. */}
          {connectionUnlisted && !connectionDangling && <MenuItem value={connectionId}>{connectionId}</MenuItem>}
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
        onChange={(keys) => handleKeyListChange('inputKeys', keys)}
      />

      <KeyListField
        id="extension-runtime-data-keys"
        label={t('flows:core.executions.extension.runtimeDataKeys.label', 'Runtime data to send')}
        placeholder={t('flows:core.executions.extension.runtimeDataKeys.placeholder', 'extension.risk_check.riskScore')}
        hint={t(
          'flows:core.executions.extension.runtimeDataKeys.hint',
          'Runtime data keys to send: userID, or data an earlier extension stored, named by its full key such as extension.risk_check.riskScore. When empty, only the user ID is sent.',
        )}
        error={
          unsendableRuntimeDataKeys.length > 0
            ? t('flows:core.executions.extension.runtimeDataKeys.invalid', {
                defaultValue: 'Only userID and keys that start with "extension." can be sent. Remove {{keys}}.',
                keys: unsendableRuntimeDataKeys.join(', '),
              })
            : undefined
        }
        value={runtimeDataKeys}
        onChange={(keys) => handleKeyListChange('runtimeDataKeys', keys)}
      />

      <KeyListField
        id="extension-response-keys"
        label={t('flows:core.executions.extension.responseKeys.label', 'Stored response fields')}
        placeholder={t('flows:core.executions.extension.responseKeys.placeholder', 'riskScore')}
        hint={t('flows:core.executions.extension.responseKeys.hint', {
          defaultValue:
            'Top-level response fields to store for later steps, stored under {{prefix}}. Later steps refer to a field by its full key, such as {{prefix}}riskScore. When empty, every top-level field except actionStatus, failureReason, failureDescription, redirectUrl and null values is stored.',
          prefix: namespacePrefix,
        })}
        value={toResponseKeyList(properties['responseKeys'])}
        onChange={(keys) => handleKeyListChange('responseKeys', keys)}
      />

      <div>
        <FormLabel htmlFor="extension-max-attempts">
          {t('flows:core.executions.extension.maxAttempts.label', 'Max attempts')}
        </FormLabel>
        <DraftTextField
          id="extension-max-attempts"
          type="number"
          value={String((properties['maxAttempts'] as number | string | undefined) ?? DEFAULT_MAX_ATTEMPTS)}
          onCommit={(value) => onChange('data.properties.maxAttempts', Number(value), resource)}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          fullWidth
          size="small"
          inputProps={{min: 1, max: 20, step: 1}}
        />
        <FormHelperText>
          {t(
            'flows:core.executions.extension.maxAttempts.hint',
            'How many FAILED or INCOMPLETE answers this step accepts in one flow execution. After that, the step fails without calling the service, and the flow continues at onFailure, or ends if none is set.',
          )}
        </FormHelperText>
      </div>

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
