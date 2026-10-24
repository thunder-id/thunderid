// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, TextField, InputAdornment, Tooltip, IconButton, FormControl, FormLabel} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import type {Role} from '../../../models/role';

interface QuickCopySectionProps {
  role: Role;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
}

export default function QuickCopySection({role, copiedField, onCopyToClipboard}: QuickCopySectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('roles:edit.general.sections.quickCopy.title', 'Quick copy')}
      description={t(
        'roles:edit.general.sections.quickCopy.description',
        'Copy role identifiers for use in your application.',
      )}
    >
      <Stack spacing={3}>
        <FormControl fullWidth>
          <FormLabel htmlFor="role-id-input">{t('roles:edit.general.labels.roleId', 'Role ID')}</FormLabel>
          <TextField
            fullWidth
            id="role-id-input"
            value={role.id}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <InputAdornment position="end">
                  <Tooltip
                    title={
                      copiedField === 'role_id'
                        ? t('common:actions.copied')
                        : t('roles:edit.general.sections.quickCopy.copyRoleId')
                    }
                  >
                    <IconButton
                      aria-label={
                        copiedField === 'role_id'
                          ? t('common:actions.copied')
                          : t('roles:edit.general.sections.quickCopy.copyRoleId')
                      }
                      onClick={() => {
                        onCopyToClipboard(role.id, 'role_id').catch(() => null);
                      }}
                      edge="end"
                    >
                      {copiedField === 'role_id' ? <Check size={16} /> : <Copy size={16} />}
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
        </FormControl>
      </Stack>
    </SettingsCard>
  );
}
