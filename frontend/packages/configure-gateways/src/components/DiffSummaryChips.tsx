// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {DiffSummary} from '../models/gateway';

export interface DiffSummaryChipsProps {
  summary: DiffSummary;
}

/**
 * Compact counts of what a diff contains.
 */
export default function DiffSummaryChips({summary}: DiffSummaryChipsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
      <Chip
        size="small"
        color="success"
        variant="outlined"
        label={t('gateways:diff.addedCount', '{{count}} added', {count: summary.added})}
      />
      <Chip
        size="small"
        color="warning"
        variant="outlined"
        label={t('gateways:diff.updatedCount', '{{count}} updated', {count: summary.updated})}
      />
      <Chip
        size="small"
        color="error"
        variant="outlined"
        label={t('gateways:diff.deletedCount', '{{count}} deleted', {count: summary.deleted})}
      />
      <Chip
        size="small"
        variant="outlined"
        label={t('gateways:diff.unchangedCount', '{{count}} unchanged', {count: summary.unchanged})}
      />
    </Stack>
  );
}
