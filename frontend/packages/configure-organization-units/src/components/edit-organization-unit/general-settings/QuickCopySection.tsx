// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, TextField, InputAdornment, Tooltip, IconButton} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import type {OrganizationUnit} from '../../../models/organization-unit';

/**
 * Props for the {@link QuickCopySection} component.
 */
interface QuickCopySectionProps {
  /**
   * The organization unit being displayed
   */
  organizationUnit: OrganizationUnit;
  /**
   * The name of the field that was recently copied to clipboard
   */
  copiedField: string | null;
  /**
   * Callback function to copy text to clipboard
   * @param text - The text to copy
   * @param fieldName - The name of the field being copied
   */
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
}

/**
 * Section component for quickly copying organization unit identifiers.
 *
 * Displays read-only text fields with copy buttons for:
 * - Handle (unique identifier)
 * - Organization Unit ID
 *
 * Provides visual feedback when values are copied.
 *
 * @param props - Component props
 * @returns Quick copy UI within a SettingsCard
 */
export default function QuickCopySection({organizationUnit, copiedField, onCopyToClipboard}: QuickCopySectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('organizationUnits:edit.general.sections.quickCopy.title')}
      description={t('organizationUnits:edit.general.sections.quickCopy.description')}
    >
      <Stack spacing={3}>
        <TextField
          labelPlacement="top"
          label={t('organizationUnits:edit.general.handle.label')}
          fullWidth
          id="handle-input"
          value={organizationUnit.handle}
          InputProps={{
            readOnly: true,
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip title={copiedField === 'handle' ? t('common:actions.copied') : t('common:actions.copy')}>
                  <IconButton
                    aria-label={copiedField === 'handle' ? t('common:actions.copied') : t('common:actions.copy')}
                    onClick={() => {
                      onCopyToClipboard(organizationUnit.handle, 'handle').catch(() => null);
                    }}
                    edge="end"
                  >
                    {copiedField === 'handle' ? <Check size={16} /> : <Copy size={16} />}
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            ),
          }}
          sx={{
            '& input': {
              fontFamily: 'monospace',
              fontSize: '0.875rem',
            },
          }}
        />

        <TextField
          labelPlacement="top"
          label={t('organizationUnits:edit.general.ou.id.label')}
          fullWidth
          id="ou-id-input"
          value={organizationUnit.id}
          InputProps={{
            readOnly: true,
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip title={copiedField === 'ou_id' ? t('common:actions.copied') : t('common:actions.copy')}>
                  <IconButton
                    aria-label={copiedField === 'ou_id' ? t('common:actions.copied') : t('common:actions.copy')}
                    onClick={() => {
                      onCopyToClipboard(organizationUnit.id, 'ou_id').catch(() => null);
                    }}
                    edge="end"
                  >
                    {copiedField === 'ou_id' ? <Check size={16} /> : <Copy size={16} />}
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            ),
          }}
          sx={{
            '& input': {
              fontFamily: 'monospace',
              fontSize: '0.875rem',
            },
          }}
        />
      </Stack>
    </SettingsCard>
  );
}
