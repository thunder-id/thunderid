// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, TextField, InputAdornment, Tooltip, IconButton} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import type {ApiUserType} from '../../../types/user-types';

interface QuickCopySectionProps {
  userType: ApiUserType;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
}

export default function QuickCopySection({userType, copiedField, onCopyToClipboard}: QuickCopySectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('userTypes:edit.general.sections.quickCopy.title', 'Quick copy')}
      description={t(
        'userTypes:edit.general.sections.quickCopy.description',
        'Copy user type identifiers for use in your application.',
      )}
    >
      <Stack spacing={3}>
        <TextField
          labelPlacement="top"
          label={t('userTypes:edit.general.labels.userTypeHandle', 'User type handle')}
          fullWidth
          id="user-type-handle-input"
          value={userType.handle}
          InputProps={{
            readOnly: true,
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title={
                    copiedField === 'user_type_handle'
                      ? t('common:actions.copied', 'Copied')
                      : t('userTypes:edit.copyHandle', 'Copy user type handle')
                  }
                >
                  <IconButton
                    aria-label={
                      copiedField === 'user_type_handle'
                        ? t('common:actions.copied', 'Copied')
                        : t('userTypes:edit.copyHandle', 'Copy user type handle')
                    }
                    onClick={() => {
                      onCopyToClipboard(userType.handle, 'user_type_handle').catch(() => null);
                    }}
                    edge="end"
                  >
                    {copiedField === 'user_type_handle' ? <Check size={16} /> : <Copy size={16} />}
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
          label={t('userTypes:edit.general.labels.userTypeId', 'User type ID')}
          fullWidth
          id="user-type-id-input"
          value={userType.id}
          InputProps={{
            readOnly: true,
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip
                  title={
                    copiedField === 'user_type_id'
                      ? t('common:actions.copied', 'Copied')
                      : t('userTypes:edit.copyId', 'Copy user type ID')
                  }
                >
                  <IconButton
                    aria-label={
                      copiedField === 'user_type_id'
                        ? t('common:actions.copied', 'Copied')
                        : t('userTypes:edit.copyId', 'Copy user type ID')
                    }
                    onClick={() => {
                      onCopyToClipboard(userType.id, 'user_type_id').catch(() => null);
                    }}
                    edge="end"
                  >
                    {copiedField === 'user_type_id' ? <Check size={16} /> : <Copy size={16} />}
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
