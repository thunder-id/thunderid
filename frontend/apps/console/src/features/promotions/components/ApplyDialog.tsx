// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DiffSummaryChips from './DiffSummaryChips';
import MissingVariablesNotice from './MissingVariablesNotice';
import ResourceDiffList from './ResourceDiffList';
import useApplyVersion from '../api/useApplyVersion';
import useCheckVariables from '../api/useCheckVariables';
import useGetEnvironmentDiff from '../api/useGetEnvironmentDiff';

export interface ApplyDialogProps {
  open: boolean;
  envId: string;
  envName: string;
  /** The version to apply, as a sequence number. */
  version: string;
  /**
   * Called with the job id when the data plane has not taken the configuration yet, so the caller
   * can follow it. Not called when the apply completed in the same request.
   */
  onQueued?: (jobId: string) => void;
  onClose: () => void;
}

/**
 * Confirms an apply by showing what it would change on the data plane first.
 *
 * An apply rewrites a running deployment, so it is worth seeing beforehand rather than reading the
 * result afterwards. The comparison is against what is currently applied there, not against the
 * previous version in the list, because those differ whenever versions were skipped.
 */
export default function ApplyDialog({
  open,
  envId,
  envName,
  version,
  onQueued = undefined,
  onClose,
}: ApplyDialogProps): JSX.Element {
  const {t} = useTranslation();
  const {data: diff, isLoading, error} = useGetEnvironmentDiff(open ? envId : '', 'applied', version);
  const {data: variableStatus} = useCheckVariables(open ? envId : '', version);
  const applyVersion = useApplyVersion();

  const handleApply = (): void => {
    applyVersion.mutate(
      {envId, version},
      {
        onSuccess: (result): void => {
          // Work the data plane has not taken yet is handed back to the page, which follows it.
          if (result.status !== 'done') {
            onQueued?.(result.jobId);
          }
          onClose();
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>{t('promotions:apply.title', 'Apply configuration')}</DialogTitle>
      <DialogContent dividers>
        <Typography variant="body2" sx={{mb: 2}}>
          {t('promotions:apply.body', 'Version {{version}} will be applied to {{env}}.', {env: envName, version})}
        </Typography>

        <MissingVariablesNotice
          missing={variableStatus?.missing ?? []}
          missingSecrets={variableStatus?.missingSecrets ?? []}
        />

        {isLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
            <CircularProgress size={24} />
          </Box>
        )}
        {error && (
          <Alert severity="error">
            {t('promotions:apply.diffError', 'The changes could not be loaded, so this apply cannot be reviewed.')}
          </Alert>
        )}
        {diff && (
          <>
            <DiffSummaryChips summary={diff.summary} />
            {(diff.summary.added ?? 0) + (diff.summary.updated ?? 0) + (diff.summary.deleted ?? 0) === 0 && (
              <Alert severity="info" sx={{mt: 2}}>
                {t('promotions:apply.noChanges', 'This version matches what is already applied. Nothing would change.')}
              </Alert>
            )}
            <Box sx={{mt: 2}}>
              <ResourceDiffList diff={diff} />
            </Box>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('common:actions.cancel', 'Cancel')}</Button>
        <Button variant="contained" disabled={applyVersion.isPending} onClick={handleApply}>
          {t('promotions:detail.apply', 'Apply')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
