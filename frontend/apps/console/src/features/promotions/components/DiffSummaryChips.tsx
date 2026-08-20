// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, Stack} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {DiffSummary} from '../models/promotion';

/**
 * Compact counts of what a diff contains.
 */
export default function DiffSummaryChips({summary}: {summary: DiffSummary}): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack direction="row" spacing={1}>
      <Chip
        size="small"
        color="success"
        label={t('promotions:diff.addedCount', '{{count}} added', {count: summary.added})}
      />
      <Chip
        size="small"
        color="warning"
        label={t('promotions:diff.updatedCount', '{{count}} updated', {count: summary.updated})}
      />
      <Chip
        size="small"
        color="error"
        label={t('promotions:diff.deletedCount', '{{count}} deleted', {count: summary.deleted})}
      />
    </Stack>
  );
}
