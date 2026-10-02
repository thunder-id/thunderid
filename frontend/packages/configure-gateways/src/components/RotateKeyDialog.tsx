// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CopyableField} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useUpdateGateway from '../api/useUpdateGateway';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import generateGatewayKey from '../utils/generateGatewayKey';

export interface RotateKeyDialogProps {
  open: boolean;
  gatewayId: string;
  gatewayName: string;
  onClose: () => void;
}

/**
 * Replaces the key presented to a gateway. Nothing is generated on the server for an edit, so
 * the new value is typed or generated here, and the same value has to be given to the gateway.
 */
export default function RotateKeyDialog({open, gatewayId, gatewayName, onClose}: RotateKeyDialogProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('RotateKeyDialog');
  const updateGateway = useUpdateGateway();
  const [key, setKey] = useState('');

  const handleClose = (): void => {
    if (updateGateway.isPending) return;
    setKey('');
    updateGateway.reset();
    onClose();
  };

  const handleChange = (value: string): void => {
    if (updateGateway.isError) updateGateway.reset();
    setKey(value);
  };

  const handleRotate = (): void => {
    if (!key.trim()) return;

    updateGateway.mutate(
      {id: gatewayId, data: {key: key.trim()}},
      {
        onSuccess: () => {
          setKey('');
          updateGateway.reset();
          onClose();
        },
        onError: (err: Error) => {
          logger.error('Failed to rotate gateway key', {error: err});
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('gateways:rotateKey.title', 'Rotate key')}</DialogTitle>
      <DialogContent>
        <Stack spacing={3} sx={{pt: 1}}>
          <Typography variant="body2" color="text.secondary">
            {t(
              'gateways:rotateKey.description',
              'Replace the key presented to {{name}}. Give the gateway the same key, or it will refuse configuration from here.',
              {name: gatewayName},
            )}
          </Typography>
          <FormControl fullWidth required>
            <FormLabel htmlFor="gateway-new-key-input">{t('gateways:rotateKey.label', 'New key')}</FormLabel>
            <Stack direction="row" spacing={1} alignItems="flex-start">
              <TextField
                id="gateway-new-key-input"
                fullWidth
                type="password"
                autoComplete="new-password"
                value={key}
                onChange={(e) => handleChange(e.target.value)}
              />
              <Button variant="outlined" sx={{whiteSpace: 'nowrap'}} onClick={() => handleChange(generateGatewayKey())}>
                {t('gateways:rotateKey.generate', 'Generate')}
              </Button>
            </Stack>
          </FormControl>
          {key.trim() && (
            <>
              <CopyableField
                label={t('gateways:rotateKey.copyLabel', 'Key to give the gateway')}
                value={key.trim()}
                copyLabel={t('gateways:register.copyKey', 'Copy key')}
              />
              <Alert severity="warning">
                {t(
                  'gateways:rotateKey.warning',
                  'This key will not be shown again once the dialog is closed. Copy it now.',
                )}
              </Alert>
            </>
          )}
          {updateGateway.error && (
            <Alert severity="error">
              {getErrorMessage(
                updateGateway.error,
                tForErrors,
                'rotateKey.error',
                'The key could not be rotated. Please try again.',
              )}
            </Alert>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={updateGateway.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" onClick={handleRotate} disabled={!key.trim() || updateGateway.isPending}>
          {updateGateway.isPending
            ? t('gateways:rotateKey.submitting', 'Rotating...')
            : t('gateways:rotateKey.submit', 'Rotate key')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
