/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useMemo} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link TemplateVariableDisplay} component.
 *
 * @public
 */
export interface TemplateVariableDisplayProps {
  /**
   * The text that may contain template literals like {{.VARIABLE_NAME}}
   */
  text: string;
  /**
   * Environment variables data as a string (e.g., "KEY=value\nKEY2=value2")
   */
  envData?: string | null;
  /**
   * Optional label to show before the value
   */
  label?: string;
}

/**
 * Parse environment variables string into a Map
 */
function parseEnvData(envData: string | null | undefined): Map<string, string> {
  const envMap = new Map<string, string>();

  if (!envData) {
    return envMap;
  }

  const lines = envData.split(/\r?\n|\r/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) {
      continue;
    }

    const equalIndex = trimmed.indexOf('=');
    if (equalIndex > 0) {
      const key = trimmed.slice(0, equalIndex).trim();
      const value = trimmed.slice(equalIndex + 1).trim();
      envMap.set(key, value);
    }
  }

  return envMap;
}

/**
 * Extract template variable name from patterns like {{.VARIABLE_NAME}}
 */
function extractVariableName(template: string): string | null {
  const match = /^\{\{\.([A-Z_][A-Z0-9_]*)\}\}$/.exec(template);
  return match ? match[1] : null;
}

/**
 * Component to display template literals with highlighted status and resolved values.
 * Template literals like {{.CONSOLE_CLIENT_ID}} are shown in green if the env file has a value,
 * red if not, with the actual value displayed as a label next to it.
 *
 * @public
 */
export default function TemplateVariableDisplay({
  text,
  envData = null,
  label = undefined,
}: TemplateVariableDisplayProps): JSX.Element {
  const {t} = useTranslation('importExport');
  const envMap = useMemo(() => parseEnvData(envData), [envData]);

  const content = useMemo((): ReactNode => {
    const trimmedText = text.trim();
    const varName = extractVariableName(trimmedText);

    // If it's a template variable
    if (varName) {
      const hasValue = envMap.has(varName);
      const value = envMap.get(varName);
      const hasActualValue = hasValue && value && value.trim() !== '';

      // Determine color: green if has value, yellow if variable exists but empty, red if missing
      let chipColor: 'success' | 'warning' | 'error' = 'success';
      let messageColor = 'text.secondary';
      let message = '';

      if (hasActualValue) {
        chipColor = 'success';
      } else if (hasValue) {
        // Variable exists but value is empty
        chipColor = 'warning';
        messageColor = 'warning.main';
        message = t('templateVariable.valueMissing');
      } else {
        // Variable doesn't exist
        chipColor = 'error';
        messageColor = 'error.main';
        message = t('templateVariable.valueMissing');
      }

      return (
        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
          <Chip
            label={trimmedText}
            size="small"
            color={chipColor}
            sx={{
              fontFamily: 'monospace',
              fontSize: '0.7rem',
              height: 20,
            }}
          />
          {hasActualValue ? (
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{
                fontFamily: 'monospace',
                px: 1,
                py: 0.25,
                bgcolor: 'action.hover',
                borderRadius: 0.5,
              }}
            >
              {value}
            </Typography>
          ) : (
            <Typography
              variant="caption"
              color={messageColor}
              sx={{
                fontStyle: 'italic',
              }}
            >
              {message}
            </Typography>
          )}
        </Stack>
      );
    }

    // Not a template variable, return as-is
    return (
      <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
        {text}
      </Typography>
    );
  }, [text, envMap, t]);

  if (!label) {
    return <Box>{content}</Box>;
  }

  return (
    <Stack direction="row" spacing={1} alignItems="center">
      <Typography variant="caption" color="text.secondary">
        {label}:
      </Typography>
      {content}
    </Stack>
  );
}
