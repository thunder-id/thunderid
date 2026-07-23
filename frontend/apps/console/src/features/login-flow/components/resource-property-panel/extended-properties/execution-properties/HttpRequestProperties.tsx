/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {HTTP_METHODS} from './constants';
import KeyValueEditor from './KeyValueEditor';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function HttpRequestProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const headers = (properties.headers as Record<string, string>) || {};
  const headerEntries = Object.entries(headers);
  const responseMapping = (properties.responseMapping as Record<string, string>) || {};
  const responseMappingEntries = Object.entries(responseMapping);
  const errorHandling =
    (properties.errorHandling as {
      failOnError?: boolean;
      retryCount?: number;
      retryDelay?: number;
    }) || {};

  const handleStringPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, value, resource, true);
    },
    [resource, onChange],
  );

  const handleNumberPropertyChange = useCallback(
    (propertyName: string, value: string, min: number, max: number): void => {
      if (value === '') {
        onChange(`data.properties.${propertyName}`, min, resource);
        return;
      }

      const num = Number(value);
      if (Number.isNaN(num)) {
        return;
      }

      onChange(`data.properties.${propertyName}`, Math.min(max, Math.max(min, Math.floor(num))), resource);
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
        <TextField
          id="http-url"
          value={(properties.url as string) || ''}
          onChange={(e) => handleStringPropertyChange('url', e.target.value)}
          placeholder={t('flows:core.executions.httpRequest.url.placeholder')}
          fullWidth
          size="small"
        />
      </div>

      <div>
        <FormLabel htmlFor="http-method">{t('flows:core.executions.httpRequest.method.label')}</FormLabel>
        <Select
          id="http-method"
          value={(properties.method as string) || 'GET'}
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
        <TextField
          id="http-body"
          value={typeof properties.body === 'string' ? properties.body : JSON.stringify(properties.body ?? {}, null, 2)}
          onChange={(e) => {
            try {
              const parsed: unknown = JSON.parse(e.target.value);
              onChange('data.properties.body', parsed, resource);
            } catch {
              onChange('data.properties.body', e.target.value, resource);
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
        <TextField
          id="http-timeout"
          value={properties.timeout ?? 10}
          onChange={(e) => handleNumberPropertyChange('timeout', e.target.value, 1, 20)}
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
            <TextField
              id="retry-count"
              value={errorHandling.retryCount ?? 0}
              onChange={(e) => {
                const val = Math.min(5, Math.max(0, Math.floor(Number(e.target.value) || 0)));
                onChange('data.properties.errorHandling', {...errorHandling, retryCount: val}, resource, true);
              }}
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
            <TextField
              id="retry-delay"
              value={errorHandling.retryDelay ?? 0}
              onChange={(e) => {
                const val = Math.min(5000, Math.max(0, Math.floor(Number(e.target.value) || 0)));
                onChange('data.properties.errorHandling', {...errorHandling, retryDelay: val}, resource, true);
              }}
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
