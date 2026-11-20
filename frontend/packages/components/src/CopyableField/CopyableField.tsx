// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Box, Stack, Typography, Tooltip, IconButton} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useRef, useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface CopyableFieldProps {
  /**
   * Small label rendered above the value (e.g. "Application ID", "Token endpoint").
   */
  label: string;
  /**
   * The value displayed in monospace and copied to the clipboard.
   */
  value: string;
  /**
   * Tooltip text shown before copying. Falls back to a generic "Copy" label.
   */
  copyLabel?: string;
}

/**
 * A labeled, bordered row displaying a monospace identifier or URL with a click-to-copy button.
 * Used for read-only identifiers and endpoints in resource overview panels (e.g. application IDs,
 * OIDC endpoints).
 */
export default function CopyableField({label, value, copyLabel = undefined}: CopyableFieldProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('CopyableField');
  const [copied, setCopied] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    },
    [],
  );

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
    timeoutRef.current = setTimeout(() => setCopied(false), 2000);
  }, [value]);

  const handleClick = () => {
    handleCopy().catch((error: unknown) => {
      logger.error('Failed to copy to clipboard', error instanceof Error ? error : {error});
    });
  };

  return (
    <Box sx={{mb: 1.5, '&:last-child': {mb: 0}}}>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
        {label}
      </Typography>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '7px',
          px: 1.25,
          py: 1,
          bgcolor: 'action.hover',
        }}
      >
        <Typography
          variant="caption"
          title={value}
          // A value longer than the field scrolls sideways, with the mouse or the arrow keys, so all of
          // it can be read in place.
          role="group"
          tabIndex={0}
          aria-label={label}
          sx={{
            flex: 1,
            fontFamily: 'monospace',
            fontSize: '0.75rem',
            overflowX: 'auto',
            scrollbarWidth: 'none',
            whiteSpace: 'nowrap',
            minWidth: 0,
          }}
        >
          {value}
        </Typography>
        <Tooltip title={copied ? t('common:actions.copied') : (copyLabel ?? t('common:actions.copy'))}>
          <IconButton size="small" onClick={handleClick} aria-label={copyLabel ?? t('common:actions.copy')}>
            {copied ? <Check size={13} /> : <Copy size={13} />}
          </IconButton>
        </Tooltip>
      </Stack>
    </Box>
  );
}
