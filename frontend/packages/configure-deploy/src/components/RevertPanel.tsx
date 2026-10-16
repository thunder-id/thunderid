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
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {Undo2} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangeList from './ChangeList';
import {useRevert} from '../api/useGatewayVersions';
import type {ApplyResult, Change, Gateway} from '../models/deploy';

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
  const revert = useRevert();
  const [preview, setPreview] = useState<ApplyResult | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isWriting, setIsWriting] = useState(false);

  useEffect(() => {
    setPreview(null);
    setError(null);
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
      onReverted(gateway, await revert.mutateAsync({gatewayId: gateway.id}));
    } catch (caught: unknown) {
      setError(caught instanceof Error ? caught : new Error(String(caught)));
    } finally {
      setIsWriting(false);
    }
  };

  const changes = (preview?.diff.changes ?? []).filter((change: Change) => change.change !== 'unchanged');
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
        {t('revertPanel.submit', 'Roll back')}
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
            {preview && changes.length > 0 && <ChangeList changes={changes} />}
            {error && (
              <Alert severity="error">
                {error.message || t('revertPanel.failed', 'The gateway could not be rolled back.')}
              </Alert>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>{footer}</DialogActions>
      </Box>
    </Dialog>
  );
}
