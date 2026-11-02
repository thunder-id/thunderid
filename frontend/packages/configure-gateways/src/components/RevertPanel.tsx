// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {Undo2} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangedValuesNotice from './ChangedValuesNotice';
import ChangeList from './ChangeList';
import {useRevert} from '../api/useGatewayVersions';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {ApplyResult, Change} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {errorCodeOf, isForbidden, REMOVALS_NEED_CONFIRMATION} from '../utils/apiError';

export interface RevertPanelProps {
  gateway: Gateway;
  /** The label of the version the gateway would go back to. */
  previousLabel?: string;
  onClose: () => void;
  onReverted: (gateway: Gateway, result: ApplyResult) => void;
}

/** Puts a gateway back on the version it ran before, after showing what that changes. */
export default function RevertPanel({
  gateway,
  previousLabel = undefined,
  onClose,
  onReverted,
}: RevertPanelProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const revert = useRevert();
  const [preview, setPreview] = useState<ApplyResult | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isWriting, setIsWriting] = useState(false);
  // Set once the control plane refused the rollback for removing more than its threshold allows.
  const [confirmRemovals, setConfirmRemovals] = useState(false);

  useEffect(() => {
    setPreview(null);
    setError(null);
    setConfirmRemovals(false);
    revert
      .mutateAsync({gatewayId: gateway.id, dryRun: true})
      .then(setPreview)
      .catch((caught: unknown) => setError(caught instanceof Error ? caught : new Error(String(caught))));
    // The preview is read once per gateway; the mutation object changes identity on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gateway.id]);

  const confirm = async (): Promise<void> => {
    setIsWriting(true);
    setError(null);
    try {
      onReverted(gateway, await revert.mutateAsync({gatewayId: gateway.id, confirmRemovals}));
    } catch (caught: unknown) {
      if (errorCodeOf(caught) === REMOVALS_NEED_CONFIRMATION) {
        setConfirmRemovals(true);
        return;
      }
      setError(caught instanceof Error ? caught : new Error(String(caught)));
    } finally {
      setIsWriting(false);
    }
  };

  const changes = (preview?.diff.changes ?? []).filter((change: Change) => change.change !== 'unchanged');
  // Counted as the control plane counts them for its threshold: a deletion the gateway holds back is
  // not sent, so it is not a removal.
  const removals = changes.filter(
    (change: Change) => change.change === 'deleted' && Boolean(change.id) && !change.excluded,
  ).length;
  const header = (
    <Stack direction="row" alignItems="center" spacing={1.5}>
      <Box sx={{display: 'flex', color: 'error.main'}}>
        <Undo2 size={18} />
      </Box>
      <Typography variant="subtitle1" fontWeight={600}>
        {previousLabel
          ? t('revertPanel.title', 'Roll back {{gateway}} to {{label}}', {gateway: gateway.name, label: previousLabel})
          : t('revertPanel.titleUnknown', 'Roll back {{gateway}}', {gateway: gateway.name})}
      </Typography>
    </Stack>
  );
  const footer = (
    <>
      <Button variant="text" color="inherit" onClick={onClose} disabled={isWriting}>
        {t('revertPanel.cancel', 'Cancel')}
      </Button>
      <Button
        variant="contained"
        color="error"
        startIcon={isWriting ? <CircularProgress size={14} color="inherit" /> : <Undo2 size={14} />}
        onClick={() => void confirm()}
        disabled={!preview || isWriting}
        sx={{borderRadius: 100}}
      >
        {confirmRemovals
          ? t('revertPanel.submitConfirm', 'Roll back and remove {{count}}', {count: removals})
          : t('revertPanel.submit', 'Roll back')}
      </Button>
    </>
  );
  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md">
      <Box data-testid="deploy-revert-panel">
        <DialogTitle>{header}</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={1.5}>
            <Typography variant="body2" data-testid="deploy-revert-what">
              {previousLabel
                ? t(
                    'revertPanel.what',
                    'A rollback goes back one step, to {{label}}, the version {{gateway}} ran before. To go further back, deploy an older version.',
                    {label: previousLabel, gateway: gateway.name},
                  )
                : t(
                    'revertPanel.whatUnknown',
                    'A rollback goes back one step, to the version {{gateway}} ran before. To go further back, deploy an older version.',
                    {gateway: gateway.name},
                  )}
            </Typography>
            {!preview && !error && (
              <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
                <CircularProgress size={18} />
                <Typography variant="body2" color="text.secondary">
                  {t('revertPanel.checking', 'Working out what rolling back would change...')}
                </Typography>
              </Box>
            )}
            {preview && (
              <Typography variant="body2" color="text.secondary">
                {changes.length === 0
                  ? t('revertPanel.noChanges', 'Rolling back changes nothing the gateway runs.')
                  : t(
                      'revertPanel.changes',
                      'Rolling back writes {{count}} resources as the earlier version has them.',
                      {
                        count: changes.length,
                      },
                    )}
              </Typography>
            )}
            {preview?.changed && <ChangedValuesNotice changed={preview.changed} />}
            {preview && changes.length > 0 && <ChangeList changes={changes} />}
            {confirmRemovals && (
              <Alert severity="warning" data-testid="deploy-confirm-removals">
                {t(
                  'revertPanel.confirmRemovals',
                  'Rolling back removes {{count}} resources from {{gateway}}. Roll back again to go ahead.',
                  {count: removals, gateway: gateway.name},
                )}
              </Alert>
            )}
            {error && (
              <Alert severity="error">
                {isForbidden(error)
                  ? t('revertPanel.forbidden', 'Your account may not roll this gateway back.')
                  : getErrorMessage(
                      error,
                      tForErrors,
                      'deploy:revertPanel.failed',
                      'The gateway could not be rolled back.',
                    )}
              </Alert>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>{footer}</DialogActions>
      </Box>
    </Dialog>
  );
}
