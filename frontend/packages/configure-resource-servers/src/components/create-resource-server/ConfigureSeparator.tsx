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

import {Box, FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {DELIMITER_OPTIONS} from '../../config/resource-server-delimiters';
import {DEFAULT_PERMISSION_DELIMITER} from '../../constants/permission-constants';
import type {PermissionDelimiter} from '../../models/permissions';
import {isValidPermissionDelimiter} from '../../utils/isValidPermissionDelimiter';

interface ConfigureSeparatorProps {
  delimiter: PermissionDelimiter;
  onDelimiterChange: (delimiter: PermissionDelimiter) => void;
  onReadyChange?: (isReady: boolean) => void;
}

export default function ConfigureSeparator({
  delimiter,
  onDelimiterChange,
  onReadyChange = undefined,
}: ConfigureSeparatorProps): JSX.Element {
  const {t} = useTranslation();

  const isDelimiterValid = isValidPermissionDelimiter(delimiter);

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(isDelimiterValid);
    }
  }, [isDelimiterValid, onReadyChange]);

  const permissionPreview = `<resource>${delimiter}<action>`;

  return (
    <Stack direction="column" spacing={4}>
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1" gutterBottom>
          {t('resourceServers:create.separator.title', 'Choose your permission delimiter')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t(
            'resourceServers:create.separator.subtitle',
            'The delimiter character joins parts of a permission string. This cannot be changed after creation.',
          )}
        </Typography>
      </Stack>

      <FormControl fullWidth required error={!isDelimiterValid}>
        <FormLabel htmlFor="resource-server-separator-select">
          {t('resourceServers:create.separator.label', 'Permission Delimiter')}
        </FormLabel>
        <Select
          id="resource-server-separator-select"
          value={isValidPermissionDelimiter(delimiter) ? delimiter : DEFAULT_PERMISSION_DELIMITER}
          onChange={(e) => {
            const val = e.target.value;
            if (isValidPermissionDelimiter(val)) {
              onDelimiterChange(val);
            }
          }}
        >
          {DELIMITER_OPTIONS.map((opt) => (
            <MenuItem key={opt.value} value={opt.value}>
              {t(opt.labelKey, opt.labelFallback)}
            </MenuItem>
          ))}
        </Select>
        {!isDelimiterValid ? (
          <FormHelperText>
            {t('resourceServers:create.separator.invalid', 'Select a valid delimiter: . _ : - /')}
          </FormHelperText>
        ) : (
          <FormHelperText>
            {t(
              'resourceServers:create.separator.hint',
              'Choose the character that separates parts of a permission string.',
            )}
          </FormHelperText>
        )}
      </FormControl>

      <Box
        sx={{
          p: 1.5,
          bgcolor: 'action.hover',
          borderRadius: 1,
          border: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Typography variant="caption" color="text.secondary" display="block" sx={{mb: 0.5}}>
          {t('resourceServers:create.separator.previewLabel', 'Example permission')}
        </Typography>
        <Typography variant="body2" sx={{fontFamily: 'monospace'}}>
          {permissionPreview}
        </Typography>
      </Box>
    </Stack>
  );
}
