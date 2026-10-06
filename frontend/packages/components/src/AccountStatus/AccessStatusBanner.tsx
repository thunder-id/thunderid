// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatusReport, LifecycleCategory} from '@thunderid/types';
import {Alert, AlertTitle, Typography, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import describeAccessStatus from './describeAccessStatus';

export interface AccessStatusBannerProps {
  /**
   * Effective access status and what produced it, as returned by the server.
   */
  status?: AccessStatusReport;
  /**
   * Identity category, which decides the verb in the message. Defaults to `user`.
   */
  category?: LifecycleCategory;
  /** Spacing from the host layout. */
  sx?: SxProps<Theme>;
}

/**
 * The hold on a record, as a banner on its page. Renders nothing for an active record.
 *
 * A suspension is a warning: it lasts until an administrator releases it. A lock is information: it
 * either lifts on its own or is the expected landing after a release.
 */
export default function AccessStatusBanner({
  status = undefined,
  category = 'user',
  sx = undefined,
}: AccessStatusBannerProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const description = describeAccessStatus(status, category, t);

  if (!description || description.value === 'ACTIVE') {
    return null;
  }

  return (
    <Alert role="status" severity={description.value === 'SUSPENDED' ? 'warning' : 'info'} sx={sx}>
      <AlertTitle>{description.label}</AlertTitle>
      {description.hint}
      {description.operatorNote && (
        <Typography variant="body2" sx={{mt: 0.5, whiteSpace: 'pre-wrap'}}>
          {description.operatorNote}
        </Typography>
      )}
    </Alert>
  );
}
