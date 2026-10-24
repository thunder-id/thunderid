// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormLabel, IconButton, InputAdornment, Stack, TextField, Tooltip} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useConnectionOU from '../api/useConnectionOU';

interface OrganizationUnitSectionProps {
  ouId: string;
}

interface CopyFieldProps {
  id: string;
  label: string;
  value: string;
  copyLabel: string;
  copied: boolean;
  onCopy: () => void;
}

function CopyField({id, label, value, copyLabel, copied, onCopy}: CopyFieldProps): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <FormControl fullWidth>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <TextField
        id={id}
        value={value}
        fullWidth
        size="small"
        slotProps={{
          input: {
            readOnly: true,
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip title={copied ? t('form.copied') : copyLabel}>
                  <IconButton aria-label={copyLabel} onClick={onCopy} edge="end" data-testid={`${id}-copy`}>
                    {copied ? <Check size={16} /> : <Copy size={16} />}
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            ),
          },
        }}
        sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
      />
    </FormControl>
  );
}

/**
 * The organization unit a connection belongs to, with its handle and id each copyable. The handle is left out until it
 * is known.
 */
export default function OrganizationUnitSection({ouId}: OrganizationUnitSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const ouQuery = useConnectionOU(ouId);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    },
    [],
  );

  const copy = useCallback((text: string, field: string): void => {
    if (!navigator.clipboard?.writeText) {
      return;
    }
    navigator.clipboard
      .writeText(text)
      .then(() => {
        setCopiedField(field);
        if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
        copyTimeoutRef.current = setTimeout(() => setCopiedField(null), 2000);
      })
      .catch(() => {
        // Clipboard write can fail silently (e.g. permissions); no user-facing error needed.
      });
  }, []);

  const handle: string | undefined = ouQuery.data?.handle;

  return (
    <SettingsCard
      title={t('detail.organizationUnit.title', 'Organization unit')}
      description={t('detail.organizationUnit.description', 'The organization unit this connection belongs to.')}
    >
      <Stack spacing={2}>
        {handle && (
          <CopyField
            id="connection-ou-handle"
            label={t('detail.organizationUnit.handleLabel', 'Handle')}
            value={handle}
            copyLabel={t('detail.organizationUnit.copyHandle', 'Copy handle')}
            copied={copiedField === 'ouHandle'}
            onCopy={() => copy(handle, 'ouHandle')}
          />
        )}
        <CopyField
          id="connection-ou-id"
          label={t('detail.organizationUnit.idLabel', 'ID')}
          value={ouId}
          copyLabel={t('detail.organizationUnit.copyId', 'Copy organization unit ID')}
          copied={copiedField === 'ouId'}
          onCopy={() => copy(ouId, 'ouId')}
        />
      </Stack>
    </SettingsCard>
  );
}
