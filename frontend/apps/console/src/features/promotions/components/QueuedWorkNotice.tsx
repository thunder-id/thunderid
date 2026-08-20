// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueryClient} from '@tanstack/react-query';
import {Alert, Box, CircularProgress, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {RotateCcw} from '@wso2/oxygen-ui-icons-react';
import {useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDataPlaneJob from '../api/useDataPlaneJob';
import PromotionQueryKeys from '../constants/promotion-query-keys';

/**
 * Props for {@link QueuedWorkNotice}.
 */
export interface QueuedWorkNoticeProps {
  /** The queued work to follow. Nothing is shown when absent. */
  jobId?: string;
  /** Called once the work has finished, successfully or not. */
  onSettled?: () => void;
}

/**
 * Reports work that is queued for a data plane rather than already carried out.
 *
 * An apply is delivered by the Control Plane pod holding that data plane's connection, which is not
 * always the pod that accepted the request. When it is not, the request returns immediately with
 * nothing applied yet, and this follows it to completion so the page does not quietly show stale
 * state. It refreshes itself, and offers a manual refresh for anyone unwilling to wait.
 */
export default function QueuedWorkNotice({
  jobId = undefined,
  onSettled = undefined,
}: QueuedWorkNoticeProps): JSX.Element | null {
  const {t} = useTranslation('promotions');
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {data: job, isFetching, refetch} = useDataPlaneJob(jobId);

  const status: string | undefined = job?.status;
  const settled: boolean = status === 'done' || status === 'failed';

  useEffect((): void => {
    if (!settled) {
      return;
    }
    // The environment now holds a different version, so anything showing it has to be re-read.
    queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.ENVIRONMENTS]}).catch(() => {
      // Ignore invalidation errors.
    });
    onSettled?.();
  }, [settled, queryClient, onSettled]);

  if (!jobId || status === 'done') {
    return null;
  }

  if (status === 'failed') {
    return (
      <Alert severity="error" sx={{mb: 2}}>
        {t('apply.jobFailed', 'The data plane rejected this configuration.')} {job?.error}
      </Alert>
    );
  }

  return (
    <Alert
      severity="info"
      // A spinner rather than the severity icon: the work is in progress, and a static icon next to
      // "queued" reads as something waiting on the reader rather than on the system.
      icon={<CircularProgress size={20} />}
      sx={{mb: 2}}
      action={
        <Tooltip title={t('apply.jobRefresh', 'Check again')}>
          <Box component="span">
            <IconButton
              size="small"
              aria-label={t('apply.jobRefresh', 'Check again')}
              disabled={isFetching}
              onClick={(): void => {
                refetch().catch(() => {
                  // Ignore refetch errors; the poll will try again.
                });
              }}
            >
              <RotateCcw size={16} />
            </IconButton>
          </Box>
        </Tooltip>
      }
    >
      {t('apply.jobPending', 'Configuration is queued and will be applied as soon as the data plane is reached.')}
    </Alert>
  );
}
