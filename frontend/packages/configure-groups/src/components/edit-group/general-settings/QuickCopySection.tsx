// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, TextField, InputAdornment, Tooltip, IconButton, FormControl, FormLabel} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import type {Group} from '../../../models/group';

interface QuickCopySectionProps {
  group: Group;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
}

export default function QuickCopySection({group, copiedField, onCopyToClipboard}: QuickCopySectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('groups:edit.general.sections.quickCopy.title', 'Quick copy')}
      description={t(
        'groups:edit.general.sections.quickCopy.description',
        'Copy group identifiers for use in your application.',
      )}
    >
      <Stack spacing={3}>
        <FormControl fullWidth>
          <FormLabel htmlFor="group-id-input">
            {t('groups:edit.general.sections.quickCopy.groupId', 'Group ID')}
          </FormLabel>
          <TextField
            fullWidth
            id="group-id-input"
            value={group.id}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <InputAdornment position="end">
                  <Tooltip
                    title={
                      copiedField === 'group_id'
                        ? t('common:actions.copied')
                        : t('groups:edit.general.sections.quickCopy.copyGroupId')
                    }
                  >
                    <IconButton
                      aria-label={
                        copiedField === 'group_id'
                          ? t('common:actions.copied')
                          : t('groups:edit.general.sections.quickCopy.copyGroupId')
                      }
                      onClick={() => {
                        onCopyToClipboard(group.id, 'group_id').catch(() => null);
                      }}
                      edge="end"
                    >
                      {copiedField === 'group_id' ? <Check size={16} /> : <Copy size={16} />}
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
