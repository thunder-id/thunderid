// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatus, AccessStatusReport} from '@thunderid/types';
import {Box, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import describeAccessStatus, {AccessStatuses} from './describeAccessStatus';
import type {LifecycleCategory} from './lifecycleAction';

export interface AccessStatusIndicatorProps {
  /**
   * Effective access status and what produced it, as returned by the server.
   */
  status?: AccessStatusReport;
  /**
   * Identity category, which decides the verb in the hover text. Defaults to `user`.
   */
  category?: LifecycleCategory;
  /**
   * Tab order of the indicator. A DataGrid cell passes its own, for the grid's roving focus. Defaults to `0`.
   */
  tabIndex?: number;
}

const DOT_COLOR: Record<AccessStatus, string> = {
  ACTIVE: 'success.main',
  LOCKED: 'warning.main',
  SUSPENDED: 'error.main',
};

/**
 * The effective access status as a coloured dot and a word, for listing rows. Active is shown quietly,
 * since an empty cell reads as a value that failed to load.
 */
export default function AccessStatusIndicator({
  status = undefined,
  category = 'user',
  tabIndex = 0,
}: AccessStatusIndicatorProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const description = describeAccessStatus(status, category, t);

  if (!description) {
    return null;
  }

  const hint = description.operatorNote ? `${description.hint} ${description.operatorNote}` : description.hint;

  return (
    // Focusable so the hint reaches keyboard users; the hint describes rather than names it.
    <Tooltip title={hint} describeChild>
      <Stack
        role="img"
        tabIndex={tabIndex}
        aria-label={description.label}
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{display: 'inline-flex'}}
      >
        <Box
          aria-hidden
          sx={{width: 8, height: 8, borderRadius: '50%', flex: 'none', bgcolor: DOT_COLOR[description.value]}}
        />
        <Typography
          variant="body2"
          color={description.value === AccessStatuses.ACTIVE ? 'text.secondary' : 'text.primary'}
        >
          {description.label}
        </Typography>
      </Stack>
    </Tooltip>
  );
}
