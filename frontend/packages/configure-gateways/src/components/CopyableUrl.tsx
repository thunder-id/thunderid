// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/** A URL to read and copy, in a quiet unbordered strip so it does not read as a field to type in. */
export default function CopyableUrl({url}: {url: string}): JSX.Element {
  const {t} = useTranslation('deploy');
  const [copied, setCopied] = useState(false);
  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  };
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 0.5,
        pl: 1.25,
        pr: 0.5,
        py: 0.25,
        borderRadius: 1,
        bgcolor: 'action.hover',
      }}
    >
      <Typography variant="caption" noWrap sx={{flex: 1, minWidth: 0, fontFamily: 'monospace'}} title={url}>
        {url}
      </Typography>
      <Tooltip title={copied ? t('url.copied', 'Copied') : t('url.copy', 'Copy the URL')}>
        <IconButton size="small" aria-label={t('url.copy', 'Copy the URL')} onClick={() => void copy()}>
          {copied ? <Check size={14} /> : <Copy size={14} />}
        </IconButton>
      </Tooltip>
    </Box>
  );
}
