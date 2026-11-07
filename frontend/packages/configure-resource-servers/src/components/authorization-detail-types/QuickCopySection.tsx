// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useCopyToClipboard} from '@thunderid/hooks';
import {IconButton, InputAdornment, Stack, TextField, Tooltip} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

type CopiedField = 'id' | 'type';

export interface QuickCopySectionProps {
  id: string;
  type: string;
  /** The error of the last save when it is about the type identifier, shown under it. */
  typeError?: string | null;
}

/** The identifiers of a saved authorization details type, each with a button that copies it. */
export default function QuickCopySection({id, type, typeError = null}: QuickCopySectionProps): JSX.Element {
  const {t} = useTranslation();
  const {copied, copy} = useCopyToClipboard();
  const [lastField, setLastField] = useState<CopiedField | null>(null);
  const copiedField = copied ? lastField : null;

  const copyAdornment = (text: string, field: CopiedField, label: string): JSX.Element => (
    <InputAdornment position="end">
      <Tooltip title={copiedField === field ? t('common:actions.copied', 'Copied!') : label}>
        <IconButton
          aria-label={label}
          onClick={() => {
            setLastField(field);
            copy(text).catch(() => null);
          }}
          edge="end"
        >
          {copiedField === field ? <Check size={16} /> : <Copy size={16} />}
        </IconButton>
      </Tooltip>
    </InputAdornment>
  );

  return (
    <SettingsCard
      title={t('authorizationDetailTypes:quickCopy.title', 'Quick copy')}
      description={t(
        'authorizationDetailTypes:quickCopy.description',
        'Copy the identifiers clients and integrations use for this intent.',
      )}
    >
      <Stack spacing={3}>
        <TextField
          labelPlacement="top"
          label={t('authorizationDetailTypes:quickCopy.id', 'Intent ID')}
          fullWidth
          id="authz-type-id"
          size="small"
          value={id}
          slotProps={{
            input: {
              readOnly: true,
              endAdornment: copyAdornment(id, 'id', t('authorizationDetailTypes:quickCopy.copyId', 'Copy intent ID')),
            },
          }}
          sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
        />
        <TextField
          labelPlacement="top"
          label={t('authorizationDetailTypes:form.type', 'Type identifier')}
          fullWidth
          data-save-error-target="type"
          id="authz-type-value"
          size="small"
          value={type}
          error={Boolean(typeError)}
          helperText={typeError ?? undefined}
          slotProps={{
            input: {
              readOnly: true,
              endAdornment: copyAdornment(
                type,
                'type',
                t('authorizationDetailTypes:quickCopy.copyType', 'Copy type identifier'),
              ),
            },
          }}
          sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
        />
      </Stack>
    </SettingsCard>
  );
}
