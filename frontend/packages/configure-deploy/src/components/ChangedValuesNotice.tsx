// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {Info} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ChangedValues, ValueKind, ValueReference} from '../models/deploy';

export interface ChangedValuesNoticeProps {
  changed: ChangedValues;
}

const INFO = 'var(--oxygen-palette-info-mainChannel)';

/**
 * Names the values set on the gateway since the version it runs was deployed: they take effect with
 * this deploy though no version carries them, so they are shown before anything is written. Names
 * and what refers to them only, never a value.
 */
export default function ChangedValuesNotice({changed}: ChangedValuesNoticeProps): JSX.Element | null {
  const {t} = useTranslation('deploy');
  const references = changed.references ?? [];
  const entries: {kind: ValueKind; name: string}[] = [
    ...(changed.secrets ?? []).map((name: string) => ({kind: 'secret' as ValueKind, name})),
    ...(changed.variables ?? []).map((name: string) => ({kind: 'variable' as ValueKind, name})),
  ];
  if (entries.length === 0) return null;
  const usedBy = (kind: ValueKind, name: string): string =>
    references
      .filter((reference: ValueReference) => reference.kind === kind && reference.name === name)
      .map((reference: ValueReference) => reference.resourceName ?? reference.resourceId ?? reference.resourceType)
      .filter((value: string, index: number, all: string[]) => all.indexOf(value) === index)
      .join(', ');
  return (
    <Stack
      spacing={1}
      data-testid="changed-values"
      sx={{
        p: 2,
        borderRadius: 2,
        border: 1,
        borderColor: `rgba(${INFO} / 0.45)`,
        borderLeft: 4,
        borderLeftColor: 'info.main',
        bgcolor: `rgba(${INFO} / 0.06)`,
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{display: 'flex', color: 'info.main'}}>
          <Info size={18} />
        </Box>
        <Typography variant="subtitle2" sx={{flex: 1}}>
          {t('changed.title', 'Values set since the last deploy')}
        </Typography>
        <Chip size="small" variant="outlined" color="info" label={entries.length} />
      </Stack>
      <Typography variant="body2" color="text.secondary">
        {t('changed.hint', 'These values take effect with this deploy, though no version carries them.')}
      </Typography>
      <Stack spacing={0.5}>
        {entries.map((entry) => (
          <Stack key={`${entry.kind}:${entry.name}`} direction="row" alignItems="center" spacing={1}>
            <Chip
              size="small"
              variant="outlined"
              label={entry.kind === 'secret' ? t('changed.secret', 'Secret') : t('changed.variable', 'Variable')}
            />
            <Typography variant="body2" sx={{fontFamily: 'monospace'}}>
              {entry.name}
            </Typography>
            {usedBy(entry.kind, entry.name) && (
              <Typography variant="caption" color="text.secondary">
                {t('changed.usedBy', 'used by {{names}}', {names: usedBy(entry.kind, entry.name)})}
              </Typography>
            )}
          </Stack>
        ))}
      </Stack>
    </Stack>
  );
}
