// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatusReport} from '@thunderid/types';
import {Alert, AlertTitle, Box, Typography, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import describeAccessStatus, {AccessStatuses} from './describeAccessStatus';
import type {LifecycleCategory} from './lifecycleAction';

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
 * A suspension or an ended lifespan is a warning: it lasts until an administrator acts. A lock is information: it
 * either lifts on its own or is the expected landing after a release. Each live lock is listed with
 * when it ends.
 */
export default function AccessStatusBanner({
  status = undefined,
  category = 'user',
  sx = undefined,
}: AccessStatusBannerProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const description = describeAccessStatus(status, category, t);

  if (!description || description.value === AccessStatuses.ACTIVE) {
    return null;
  }

  return (
    <Alert
      role="status"
      severity={
        description.value === AccessStatuses.SUSPENDED || description.value === AccessStatuses.EXPIRED
          ? 'warning'
          : 'info'
      }
      data-testid="access-status-banner"
      sx={sx}
    >
      <AlertTitle>{description.title}</AlertTitle>
      {description.hint}
      {description.locks && description.locks.length > 0 && (
        <Box component="ul" sx={{m: 0, mt: 0.5, pl: 2.5}}>
          {description.locks.map((lock) => (
            <Typography key={lock} component="li" variant="body2">
              {lock}
            </Typography>
          ))}
        </Box>
      )}
      {description.operatorNote && (
        <Typography variant="body2" sx={{mt: 0.5, whiteSpace: 'pre-wrap'}}>
          {description.operatorNote}
        </Typography>
      )}
    </Alert>
  );
}
