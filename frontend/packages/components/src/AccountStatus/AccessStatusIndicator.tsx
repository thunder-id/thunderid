// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatus, AccessStatusReport, LifecycleCategory} from '@thunderid/types';
import {Box, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import describeAccessStatus from './describeAccessStatus';

export interface AccessStatusIndicatorProps {
  /**
   * Effective access status and what produced it, as returned by the server.
   */
  status?: AccessStatusReport;
  /**
   * Identity category, which decides the verb in the hover text. Defaults to `user`.
   */
  category?: LifecycleCategory;
}

const DOT_COLOR: Record<AccessStatus, string> = {
  ACTIVE: 'success.main',
  LOCKED: 'warning.main',
  SUSPENDED: 'error.main',
};

/**
 * The effective access status as a coloured dot and a word, for listing rows.
 *
 * Active is shown, but quietly: an empty status cell reads as a value that failed to load. The holds
 * keep the primary text colour so they stand out in a column of mostly active rows.
 */
export default function AccessStatusIndicator({
  status = undefined,
  category = 'user',
}: AccessStatusIndicatorProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const description = describeAccessStatus(status, category, t);

  if (!description) {
    return null;
  }

  const hint = description.operatorNote ? `${description.hint} ${description.operatorNote}` : description.hint;

  return (
    <Tooltip title={hint}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{display: 'inline-flex'}}>
        <Box
          aria-hidden
          sx={{width: 8, height: 8, borderRadius: '50%', flex: 'none', bgcolor: DOT_COLOR[description.value]}}
        />
        <Typography variant="body2" color={description.value === 'ACTIVE' ? 'text.secondary' : 'text.primary'}>
          {description.label}
        </Typography>
      </Stack>
    </Tooltip>
  );
}
