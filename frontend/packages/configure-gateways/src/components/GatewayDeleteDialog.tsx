// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteGateway from '../api/useDeleteGateway';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {Gateway} from '../models/gateway';

export interface GatewayDeleteDialogProps {
  open: boolean;
  gateway: Gateway | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function GatewayDeleteDialog({
  open,
  gateway,
  onClose,
  onSuccess,
}: GatewayDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('GatewayDeleteDialog');
  const deleteGateway = useDeleteGateway();

  const handleClose = (): void => {
    if (deleteGateway.isPending) return;
    deleteGateway.reset();
    onClose();
  };

  const handleDelete = (): void => {
    if (!gateway) return;

    deleteGateway.mutate(gateway.id, {
      onSuccess: () => {
        deleteGateway.reset();
        onSuccess();
      },
      onError: (err: Error) => {
        logger.error('Failed to remove gateway', {error: err});
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t('gateways:delete.title', 'Remove gateway')}</DialogTitle>
      <DialogContent>
        <Typography variant="body2">
          {t('gateways:delete.confirm', 'Remove the registration of')} <strong>{gateway?.name}</strong>?
        </Typography>
        <Alert severity="info" sx={{mt: 2}}>
          {t(
            'gateways:delete.disclaimer',
            'The gateway itself is not changed. It keeps running with the configuration it already holds.',
          )}
        </Alert>
        {deleteGateway.error && (
          <Alert severity="error" sx={{mt: 2}}>
            {getErrorMessage(
              deleteGateway.error,
              tForErrors,
              'delete.error',
              'The gateway could not be removed. Please try again.',
            )}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={deleteGateway.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" color="error" onClick={handleDelete} disabled={deleteGateway.isPending}>
          {deleteGateway.isPending
            ? t('gateways:delete.submitting', 'Removing...')
            : t('gateways:delete.submit', 'Remove')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
