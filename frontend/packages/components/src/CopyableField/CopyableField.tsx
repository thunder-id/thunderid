// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCopyToClipboard} from '@thunderid/hooks';
import {FormControl, FormLabel, IconButton, InputAdornment, TextField, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useId, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface CopyableFieldProps {
  /**
   * The `id` used to associate the label with the input. Generated when omitted.
   */
  id?: string;
  /**
   * Label rendered above the value (e.g. "Application ID", "Token endpoint").
   */
  label: string;
  /**
   * The value displayed in monospace and copied to the clipboard.
   */
  value: string;
  /**
   * Tooltip and aria-label for the copy button. Falls back to a generic "Copy" label.
   */
  copyLabel?: string;
  /**
   * Optional helper text shown below the label.
   */
  hint?: string;
}

/**
 * A read-only monospace field with a copy-to-clipboard button inside the input. The single
 * console-wide treatment for copyable identifiers, secrets, and endpoints. Confirms a copy by
 * switching the icon to a check and the tooltip to "Copied!".
 */
export default function CopyableField({
  id = undefined,
  label,
  value,
  copyLabel = undefined,
  hint = undefined,
}: CopyableFieldProps): JSX.Element {
  const {t} = useTranslation();
  const generatedId = useId();
  const fieldId = id ?? generatedId;
  const {copied, copy} = useCopyToClipboard();
  const buttonLabel = copied ? t('common:actions.copied') : (copyLabel ?? t('common:actions.copy'));

  return (
    <FormControl fullWidth>
      <FormLabel htmlFor={fieldId}>{label}</FormLabel>
      {hint && (
        <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
          {hint}
        </Typography>
      )}
      <TextField
        fullWidth
        id={fieldId}
        value={value}
        slotProps={{
          input: {
            readOnly: true,
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip title={buttonLabel}>
                  <IconButton
                    aria-label={buttonLabel}
                    edge="end"
                    size="small"
                    onClick={() => {
                      copy(value).catch(() => null);
                    }}
                  >
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
