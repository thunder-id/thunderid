// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import type {User} from '@thunderid/types';
import {Stack, TextField, InputAdornment, Tooltip, IconButton, FormControl, FormLabel} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';

interface QuickCopySectionProps {
  user: User;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
}

export default function QuickCopySection({user, copiedField, onCopyToClipboard}: QuickCopySectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('users:manageUser.sections.quickCopy.title', 'Quick copy')}
      description={t(
        'users:manageUser.sections.quickCopy.description',
        'Copy user identifiers for use in your application.',
      )}
    >
      <Stack spacing={3}>
        <FormControl fullWidth>
          <FormLabel htmlFor="user-id-input">{t('users:manageUser.sections.quickCopy.userId', 'User ID')}</FormLabel>
          <TextField
            fullWidth
            id="user-id-input"
            value={user.id}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <InputAdornment position="end">
                  <Tooltip
                    title={
                      copiedField === 'userId'
                        ? t('common:actions.copied', 'Copied')
                        : t('users:manageUser.sections.quickCopy.copyUserId', 'Copy user ID')
                    }
                  >
                    <IconButton
                      aria-label={
                        copiedField === 'userId'
                          ? t('common:actions.copied', 'Copied')
                          : t('users:manageUser.sections.quickCopy.copyUserId', 'Copy user ID')
                      }
                      onClick={() => {
                        onCopyToClipboard(user.id, 'userId').catch(() => null);
                      }}
                      edge="end"
                    >
                      {copiedField === 'userId' ? <Check size={16} /> : <Copy size={16} />}
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
