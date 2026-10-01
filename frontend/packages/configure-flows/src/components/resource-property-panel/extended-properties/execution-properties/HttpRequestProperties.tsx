// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {HTTP_METHODS} from './constants';
import DraftTextField from './DraftTextField';
import KeyValueEditor from './KeyValueEditor';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
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

  const handleStringPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  const handleNumberPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, Number(value), resource);
    },
    [resource, onChange],
  );

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

      <div>
        <FormLabel htmlFor="http-url">{t('flows:core.executions.httpRequest.url.label')}</FormLabel>
        <DraftTextField
          id="http-url"
          value={(properties['url'] as string) || ''}
          onCommit={(value) => handleStringPropertyChange('url', value)}
          placeholder={t('flows:core.executions.httpRequest.url.placeholder')}
          fullWidth
          size="small"
        />
      </div>

      <div>
        <FormLabel htmlFor="http-method">{t('flows:core.executions.httpRequest.method.label')}</FormLabel>
        <Select
          id="http-method"
          value={(properties['method'] as string) || 'GET'}
          onChange={(e) => onChange('data.properties.method', e.target.value, resource)}
          fullWidth
        >
          {HTTP_METHODS.map((method) => (
            <MenuItem key={method} value={method}>
              {method}
            </MenuItem>
          ))}
        </Select>
      </div>

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
          keyPlaceholder={t('flows:core.executions.httpRequest.headers.keyPlaceholder')}
          valuePlaceholder={t('flows:core.executions.httpRequest.headers.valuePlaceholder')}
        />
      </div>

      <div>
        <FormLabel htmlFor="http-body">{t('flows:core.executions.httpRequest.body.label')}</FormLabel>
        <DraftTextField
          id="http-body"
          value={
            typeof properties['body'] === 'string'
              ? properties['body']
              : JSON.stringify(properties['body'] ?? {}, null, 2)
          }
          // Committing on blur also means a half-typed body is not repeatedly parsed and
          // stored as a raw string on the way to becoming valid JSON.
          onCommit={(value) => {
            try {
              const parsed: unknown = JSON.parse(value);
              onChange('data.properties.body', parsed, resource);
            } catch {
              onChange('data.properties.body', value, resource);
            }
          }}
          placeholder={t('flows:core.executions.httpRequest.body.placeholder')}
          fullWidth
          size="small"
          multiline
          minRows={3}
        />
      </div>

      <div>
        <FormLabel htmlFor="http-timeout">{t('flows:core.executions.httpRequest.timeout.label')}</FormLabel>
        <DraftTextField
          id="http-timeout"
          value={String((properties['timeout'] as number | string | undefined) ?? 10)}
          onCommit={(value) => handleNumberPropertyChange('timeout', value)}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          placeholder={t('flows:core.executions.httpRequest.timeout.placeholder')}
          fullWidth
          size="small"
          type="number"
          inputProps={{min: 1, max: 20}}
        />
        <FormHelperText>{t('flows:core.executions.httpRequest.timeout.hint')}</FormHelperText>
      </div>

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
          keyPlaceholder={t('flows:core.executions.httpRequest.responseMapping.keyPlaceholder')}
          valuePlaceholder={t('flows:core.executions.httpRequest.responseMapping.valuePlaceholder')}
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

          <div>
            <FormLabel htmlFor="retry-count">
              {t('flows:core.executions.httpRequest.errorHandling.retryCount.label')}
            </FormLabel>
            <DraftTextField
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
            <FormHelperText>{t('flows:core.executions.httpRequest.errorHandling.retryCount.hint')}</FormHelperText>
          </div>

          <div>
            <FormLabel htmlFor="retry-delay">
              {t('flows:core.executions.httpRequest.errorHandling.retryDelay.label')}
            </FormLabel>
            <DraftTextField
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
            <FormHelperText>{t('flows:core.executions.httpRequest.errorHandling.retryDelay.hint')}</FormHelperText>
          </div>
        </Stack>
      </div>
    </Stack>
  );
}

export default HttpRequestProperties;
