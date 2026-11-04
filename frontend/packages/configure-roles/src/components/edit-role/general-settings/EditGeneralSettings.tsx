// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, TextField, IconButton, InputAdornment, Tooltip} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useRef, useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import QuickCopySection from './QuickCopySection';
import type {Role} from '../../../models/role';

interface EditGeneralSettingsProps {
  role: Role;
}

/**
 * General settings tab content for the Role edit page.
 * Displays Organization Unit info.
 */
export default function EditGeneralSettings({role}: EditGeneralSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    },
    [],
  );

  const handleCopyToClipboard = useCallback(async (text: string, fieldName: string): Promise<void> => {
    await navigator.clipboard.writeText(text);
    setCopiedField(fieldName);
    if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    copyTimeoutRef.current = setTimeout(() => setCopiedField(null), 2000);
  }, []);

  return (
    <Stack spacing={3}>
      <QuickCopySection role={role} copiedField={copiedField} onCopyToClipboard={handleCopyToClipboard} />

      {/* Organization Unit */}
      <SettingsCard
        title={t('roles:edit.general.sections.organizationUnit.title')}
        description={t('roles:edit.general.sections.organizationUnit.description')}
      >
        <Stack spacing={2}>
          {role.ouHandle && (
            <TextField
              labelPlacement="top"
              label={t('roles:edit.general.sections.organizationUnit.handleLabel', 'Handle')}
              id="ou-handle-input"
              value={role.ouHandle}
              fullWidth
              size="small"
              slotProps={{
                input: {
                  readOnly: true,
                  endAdornment: (
                    <InputAdornment position="end">
                      <Tooltip
                        title={
                          copiedField === 'ouHandle'
                            ? t('common:actions.copied')
                            : t('roles:edit.general.sections.organizationUnit.copyHandle', 'Copy handle')
                        }
                      >
                        <IconButton
                          aria-label={t('roles:edit.general.sections.organizationUnit.copyHandle', 'Copy handle')}
                          onClick={() => {
                            handleCopyToClipboard(role.ouHandle!, 'ouHandle').catch(() => {
                              /* noop */
                            });
                          }}
                          edge="end"
                        >
                          {copiedField === 'ouHandle' ? <Check size={16} /> : <Copy size={16} />}
                        </IconButton>
                      </Tooltip>
                    </InputAdornment>
                  ),
                },
              }}
              sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
            />
          )}
          <TextField
            labelPlacement="top"
            label={t('roles:edit.general.sections.organizationUnit.idLabel', 'ID')}
            id="ou-id-input"
            value={role.ouId}
            fullWidth
            size="small"
            slotProps={{
              input: {
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <Tooltip
                      title={
                        copiedField === 'ouId'
                          ? t('common:actions.copied')
                          : t('roles:edit.general.sections.organizationUnit.copyId')
                      }
                    >
                      <IconButton
                        aria-label={t('roles:edit.general.sections.organizationUnit.copyId')}
                        onClick={() => {
                          handleCopyToClipboard(role.ouId, 'ouId').catch(() => {
                            /* noop */
                          });
                        }}
                        edge="end"
                      >
                        {copiedField === 'ouId' ? <Check size={16} /> : <Copy size={16} />}
                      </IconButton>
                    </Tooltip>
                  </InputAdornment>
                ),
              },
            }}
            sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
          />
        </Stack>
      </SettingsCard>
    </Stack>
  );
}
