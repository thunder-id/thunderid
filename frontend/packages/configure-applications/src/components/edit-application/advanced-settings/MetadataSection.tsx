// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import type {Application} from '@thunderid/configure-applications';

/**
 * Props for the {@link MetadataSection} component.
 */
interface MetadataSectionProps {
  /**
   * The application to display metadata for
   */
  application: Application;
}

/**
 * Section component for displaying application metadata (read-only).
 *
 * Shows:
 * - Created at timestamp (formatted as locale string)
 * - Updated at timestamp (formatted as locale string)
 *
 * Returns null if no metadata timestamps are available.
 *
 * @param props - Component props
 * @returns Metadata display UI within a SettingsCard, or null
 */
export default function MetadataSection({application}: MetadataSectionProps) {
  const {t} = useTranslation();

  if (!application.createdAt && !application.updatedAt) {
    return null;
  }

  return (
    <SettingsCard title={t('applications:edit.advanced.labels.metadata')}>
      <Stack spacing={2}>
        {application.createdAt && (
          <Box>
            <Typography variant="subtitle2" color="text.secondary">
              {t('applications:edit.advanced.labels.createdAt')}
            </Typography>
            <Typography variant="body1">{new Date(application.createdAt).toLocaleString()}</Typography>
          </Box>
        )}
        {application.updatedAt && (
          <Box>
            <Typography variant="subtitle2" color="text.secondary">
              {t('applications:edit.advanced.labels.updatedAt')}
            </Typography>
            <Typography variant="body1">{new Date(application.updatedAt).toLocaleString()}</Typography>
          </Box>
        )}
      </Stack>
    </SettingsCard>
  );
}
