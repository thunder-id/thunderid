// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {HTTP_METHODS} from './constants';
import DraftTextField from './DraftTextField';
import KeyValueEditor from './KeyValueEditor';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import VariableBinding from './VariableBinding';
import {NUMBER_VARIABLES, TEXT_VARIABLES} from './variableBindingTypes';
import type {StepData} from '../../../../models/steps';

function HttpRequestProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const headers = (properties['headers'] as Record<string, string>) || {};
  const headerEntries = Object.entries(headers);
  const responseMapping = (properties['responseMapping'] as Record<string, string>) || {};
  const responseMappingEntries = Object.entries(responseMapping);
  const errorHandling =
    (properties['errorHandling'] as {
      failOnError?: boolean;
      retryCount?: number;
      retryDelay?: number;
    }) || {};

  const entriesToRecord = (entries: [string, string][]): Record<string, string> => Object.fromEntries(entries);

  const updateHeaderEntries = (updater: (prev: [string, string][]) => [string, string][]): void => {
    onChange('data.properties.headers', entriesToRecord(updater(headerEntries)), resource);
  };

  const updateResponseMappingEntries = (updater: (prev: [string, string][]) => [string, string][]): void => {
    onChange('data.properties.responseMapping', entriesToRecord(updater(responseMappingEntries)), resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.httpRequest.description')}
      </Typography>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="url"
        accepts={TEXT_VARIABLES}
        label={t('flows:core.executions.httpRequest.url.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            id="http-url"
            value={(stored as string) || ''}
            onCommit={commit}
            placeholder={t('flows:core.executions.httpRequest.url.placeholder')}
            fullWidth
            size="small"
          />
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="method"
        accepts={TEXT_VARIABLES}
        showLabel
        labelId="http-method-label"
        htmlFor="http-method"
        label={t('flows:core.executions.httpRequest.method.label')}
      >
        {({commit, value: stored}) => (
          <Select
            labelId="http-method-label"
            id="http-method"
            value={(stored as string) || 'GET'}
            onChange={(e) => commit(e.target.value)}
            fullWidth
          >
            {HTTP_METHODS.map((method) => (
              <MenuItem key={method} value={method}>
                {method}
              </MenuItem>
            ))}
          </Select>
        )}
      </VariableBinding>

      <div>
        <FormLabel>{t('flows:core.executions.httpRequest.headers.label')}</FormLabel>
        <KeyValueEditor
          entries={headerEntries}
          onAdd={() => updateHeaderEntries((prev) => [...prev, ['', '']])}
          onRemove={(index) => updateHeaderEntries((prev) => prev.filter((_, i) => i !== index))}
          onKeyChange={(index, newKey) =>
            updateHeaderEntries((prev) => prev.map((entry, i) => (i === index ? [newKey, entry[1]] : entry)))
          }
          onValueChange={(index, newValue) =>
            updateHeaderEntries((prev) => prev.map((entry, i) => (i === index ? [entry[0], newValue] : entry)))
          }
          keyLabel={t('flows:core.executions.httpRequest.headers.keyLabel')}
          valueLabel={t('flows:core.executions.httpRequest.headers.valueLabel')}
          keyPlaceholder={t('flows:core.executions.httpRequest.headers.keyPlaceholder')}
          valuePlaceholder={t('flows:core.executions.httpRequest.headers.valuePlaceholder')}
          addLabel={t('flows:core.executions.httpRequest.headers.addLabel')}
          removeLabel={t('common:actions.delete', 'Delete')}
        />
      </div>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="body"
        accepts={TEXT_VARIABLES}
        label={t('flows:core.executions.httpRequest.body.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            id="http-body"
            value={typeof stored === 'string' ? stored : JSON.stringify(stored ?? {}, null, 2)}
            // Committing on blur also means a half-typed body is not repeatedly parsed and
            // stored as a raw string on the way to becoming valid JSON.
            onCommit={(value) => {
              try {
                const parsed: unknown = JSON.parse(value);
                commit(parsed);
              } catch {
                commit(value);
              }
            }}
            placeholder={t('flows:core.executions.httpRequest.body.placeholder')}
            fullWidth
            size="small"
            multiline
            minRows={3}
          />
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="timeout"
        accepts={NUMBER_VARIABLES}
        label={t('flows:core.executions.httpRequest.timeout.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            helperText={t('flows:core.executions.httpRequest.timeout.hint')}
            id="http-timeout"
            value={String((stored as number | string | undefined) ?? 10)}
            onCommit={(value) => commit(Number(value))}
            normalize={(raw) => clampToInteger(raw, 1, 20)}
            placeholder={t('flows:core.executions.httpRequest.timeout.placeholder')}
            fullWidth
            size="small"
            type="number"
            inputProps={{min: 1, max: 20}}
          />
        )}
      </VariableBinding>

      <div>
        <FormLabel>{t('flows:core.executions.httpRequest.responseMapping.label')}</FormLabel>
        <KeyValueEditor
          entries={responseMappingEntries}
          onAdd={() => updateResponseMappingEntries((prev) => [...prev, ['', '']])}
          onRemove={(index) => updateResponseMappingEntries((prev) => prev.filter((_, i) => i !== index))}
          onKeyChange={(index, newKey) =>
            updateResponseMappingEntries((prev) => prev.map((entry, i) => (i === index ? [newKey, entry[1]] : entry)))
          }
          onValueChange={(index, newValue) =>
            updateResponseMappingEntries((prev) => prev.map((entry, i) => (i === index ? [entry[0], newValue] : entry)))
          }
          keyLabel={t('flows:core.executions.httpRequest.responseMapping.keyLabel')}
          valueLabel={t('flows:core.executions.httpRequest.responseMapping.valueLabel')}
          keyPlaceholder={t('flows:core.executions.httpRequest.responseMapping.keyPlaceholder')}
          valuePlaceholder={t('flows:core.executions.httpRequest.responseMapping.valuePlaceholder')}
          addLabel={t('flows:core.executions.httpRequest.responseMapping.addLabel')}
          removeLabel={t('common:actions.delete', 'Delete')}
        />
      </div>

      <div>
        <FormLabel>{t('flows:core.executions.httpRequest.errorHandling.label')}</FormLabel>
        <Stack gap={1} sx={{pl: 1}}>
          <FormControlLabel
            control={
              <Checkbox
                checked={!!errorHandling.failOnError}
                onChange={(e) => {
                  onChange(
                    'data.properties.errorHandling',
                    {...errorHandling, failOnError: e.target.checked},
                    resource,
                  );
                }}
                size="small"
              />
            }
            label={t('flows:core.executions.httpRequest.errorHandling.failOnError.label')}
          />

          <DraftTextField
            labelPlacement="top"
            label={t('flows:core.executions.httpRequest.errorHandling.retryCount.label')}
            helperText={t('flows:core.executions.httpRequest.errorHandling.retryCount.hint')}
            id="retry-count"
            value={String(errorHandling.retryCount ?? 0)}
            onCommit={(value) =>
              onChange('data.properties.errorHandling', {...errorHandling, retryCount: Number(value)}, resource)
            }
            normalize={(raw) => clampToInteger(raw, 0, 5)}
            placeholder={t('flows:core.executions.httpRequest.errorHandling.retryCount.placeholder')}
            fullWidth
            size="small"
            type="number"
            inputProps={{min: 0, max: 5}}
          />

          <DraftTextField
            labelPlacement="top"
            label={t('flows:core.executions.httpRequest.errorHandling.retryDelay.label')}
            helperText={t('flows:core.executions.httpRequest.errorHandling.retryDelay.hint')}
            id="retry-delay"
            value={String(errorHandling.retryDelay ?? 0)}
            onCommit={(value) =>
              onChange('data.properties.errorHandling', {...errorHandling, retryDelay: Number(value)}, resource)
            }
            normalize={(raw) => clampToInteger(raw, 0, 5000)}
            placeholder={t('flows:core.executions.httpRequest.errorHandling.retryDelay.placeholder')}
            fullWidth
            size="small"
            type="number"
            inputProps={{min: 0, max: 5000}}
          />
        </Stack>
      </div>
    </Stack>
  );
}

export default HttpRequestProperties;
