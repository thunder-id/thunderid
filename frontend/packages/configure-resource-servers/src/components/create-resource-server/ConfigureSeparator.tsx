// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Box,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {DELIMITER_OPTIONS} from '../../config/resource-server-delimiters';
import {DEFAULT_PERMISSION_DELIMITER} from '../../constants/permission-constants';
import type {PermissionDelimiter} from '../../models/permissions';
import {isValidPermissionDelimiter} from '../../utils/isValidPermissionDelimiter';

interface ConfigureSeparatorProps {
  delimiter: PermissionDelimiter;
  hierarchicalPermissions: boolean;
  onHierarchicalPermissionsChange: (enabled: boolean) => void;
  onDelimiterChange: (delimiter: PermissionDelimiter) => void;
  onReadyChange?: (isReady: boolean) => void;
}

export default function ConfigureSeparator({
  delimiter,
  hierarchicalPermissions,
  onHierarchicalPermissionsChange,
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
          {t('resourceServers:create.separator.title', 'Configure permission settings')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t(
            'resourceServers:create.separator.subtitle',
            'Choose how permissions are matched and separated. Both settings are fixed after creation.',
          )}
        </Typography>
      </Stack>

      <FormControl fullWidth>
        <FormControlLabel
          control={
            <Checkbox
              id="resource-server-hierarchy-input"
              checked={hierarchicalPermissions}
              onChange={(event) => onHierarchicalPermissionsChange(event.target.checked)}
              inputProps={{'aria-describedby': 'resource-server-hierarchy-hint'}}
            />
          }
          label={t('resourceServers:create.separator.hierarchyLabel', 'Use hierarchy-aware matching')}
        />
        <Typography id="resource-server-hierarchy-hint" variant="body2" color="text.secondary">
          {t(
            'resourceServers:create.separator.hierarchyHint',
            'When enabled, granting a permission also grants its descendants, including permissions added later.',
          )}
        </Typography>
      </FormControl>

      <FormControl fullWidth required error={!isDelimiterValid}>
        <FormLabel id="resource-server-separator-label" htmlFor="resource-server-separator-select">
          {t('resourceServers:create.separator.label', 'Permission delimiter')}
        </FormLabel>
        <Select
          id="resource-server-separator-select"
          labelId="resource-server-separator-label"
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
