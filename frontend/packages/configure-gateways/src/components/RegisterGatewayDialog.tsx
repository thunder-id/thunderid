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
import useRegisterGateway from '../api/useRegisterGateway';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {GatewayRegistration} from '../models/gateway';

export interface RegisterGatewayDialogProps {
  open: boolean;
  onClose: () => void;
  /** Called once the key has been seen, with the registered gateway. */
  onRegistered: (gateway: GatewayRegistration) => void;
}

interface FormValues {
  name: string;
  baseUrl: string;
  key: string;
  caCertificate: string;
}

const EMPTY_FORM: FormValues = {name: '', baseUrl: '', key: '', caCertificate: ''};

/**
 * Registers a gateway, then shows the key it holds. The key is returned only by the registration,
 * so it is shown here once, before the dialog hands over to the gateway's page.
 */
export default function RegisterGatewayDialog({open, onClose, onRegistered}: RegisterGatewayDialogProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('RegisterGatewayDialog');
  const registerGateway = useRegisterGateway();
  const [values, setValues] = useState<FormValues>(EMPTY_FORM);
  const [registration, setRegistration] = useState<GatewayRegistration | null>(null);

  const canSubmit = values.name.trim() !== '' && values.baseUrl.trim() !== '' && !registerGateway.isPending;

  const handleChange = (field: keyof FormValues, value: string): void => {
    if (registerGateway.isError) {
      registerGateway.reset();
    }
    setValues((prev) => ({...prev, [field]: value}));
  };

  const reset = (): void => {
    setValues(EMPTY_FORM);
    setRegistration(null);
    registerGateway.reset();
  };

  const handleClose = (): void => {
    if (registerGateway.isPending) return;
    // Closing after a registration still has to land the user on the new gateway.
    if (registration) {
      onRegistered(registration);
    } else {
      onClose();
    }
    reset();
  };

  // The key is shown only once, so while it is on screen only the explicit button closes the dialog.
  const handleDialogClose = (_event: object, reason: 'backdropClick' | 'escapeKeyDown'): void => {
    if (registration && (reason === 'backdropClick' || reason === 'escapeKeyDown')) return;
    handleClose();
  };

  const handleSubmit = (): void => {
    if (!canSubmit) return;

    registerGateway.mutate(
      {
        name: values.name.trim(),
        baseUrl: values.baseUrl.trim(),
        ...(values.key.trim() ? {key: values.key.trim()} : {}),
        ...(values.caCertificate.trim() ? {caCertificate: values.caCertificate.trim()} : {}),
      },
      {
        onSuccess: (result: GatewayRegistration) => {
          setRegistration(result);
        },
        onError: (err: Error) => {
          logger.error('Failed to register gateway', {error: err});
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleDialogClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {registration
          ? t('gateways:register.keyTitle', 'Save the gateway key')
          : t('gateways:register.title', 'Register a gateway')}
      </DialogTitle>
      <DialogContent>
        {registration ? (
          <Stack spacing={2}>
            <Typography variant="body2">
              {t(
                'gateways:register.keyDescription',
                'Configure {{name}} with this key so it accepts configuration from here.',
                {name: registration.name},
              )}
            </Typography>
            <CopyableField
              label={t('gateways:register.keyLabel', 'Gateway key')}
              value={registration.key}
              copyLabel={t('gateways:register.copyKey', 'Copy key')}
            />
            <Alert severity="warning">
              {t(
                'gateways:register.keyWarning',
                'This key will not be shown again. Copy it now. If it is lost, rotate the key from the gateway page.',
              )}
            </Alert>
          </Stack>
        ) : (
          <Stack spacing={3} sx={{pt: 1}}>
            <Typography variant="body2" color="text.secondary">
              {t(
                'gateways:register.description',
                'Record where a gateway answers. The gateway is not contacted until a configuration is applied to it.',
              )}
            </Typography>
            <FormControl fullWidth required>
              <FormLabel htmlFor="gateway-name-input">{t('gateways:form.name.label', 'Name')}</FormLabel>
              <TextField
                id="gateway-name-input"
                fullWidth
                value={values.name}
                onChange={(e) => handleChange('name', e.target.value)}
                placeholder={t('gateways:form.name.placeholder', 'e.g. production')}
              />
            </FormControl>
            <FormControl fullWidth required>
              <FormLabel htmlFor="gateway-base-url-input">{t('gateways:form.baseUrl.label', 'Base URL')}</FormLabel>
              <TextField
                id="gateway-base-url-input"
                fullWidth
                value={values.baseUrl}
                onChange={(e) => handleChange('baseUrl', e.target.value)}
                placeholder={t('gateways:form.baseUrl.placeholder', 'https://gateway.example.com:8090')}
                helperText={t('gateways:form.baseUrl.hint', 'Where the gateway answers. Each gateway registers once.')}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="gateway-key-input">{t('gateways:form.key.label', 'Key (optional)')}</FormLabel>
              <TextField
                id="gateway-key-input"
                fullWidth
                type="password"
                autoComplete="new-password"
                value={values.key}
                onChange={(e) => handleChange('key', e.target.value)}
                helperText={t(
                  'gateways:form.key.hint',
                  'Leave empty to generate one. Supply a key only when the gateway already holds one.',
                )}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="gateway-ca-certificate-input">
                {t('gateways:form.caCertificate.label', 'CA certificate (optional)')}
              </FormLabel>
              <TextField
                id="gateway-ca-certificate-input"
                fullWidth
                multiline
                minRows={4}
                value={values.caCertificate}
                onChange={(e) => handleChange('caCertificate', e.target.value)}
                helperText={t(
                  'gateways:form.caCertificate.hint',
                  'A PEM certificate to trust when calling this gateway, for one serving a certificate no public authority signed.',
                )}
                slotProps={{htmlInput: {sx: {fontFamily: 'monospace', fontSize: '0.8rem'}}}}
              />
            </FormControl>
            {registerGateway.error && (
              <Alert severity="error">
                {getErrorMessage(
                  registerGateway.error,
                  tForErrors,
                  'register.error',
                  'The gateway could not be registered. Please try again.',
                )}
              </Alert>
            )}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        {registration ? (
          <Button variant="contained" onClick={handleClose}>
            {t('gateways:register.done', 'I have saved the key')}
          </Button>
        ) : (
          <>
            <Button variant="outlined" onClick={handleClose} disabled={registerGateway.isPending}>
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button variant="contained" onClick={handleSubmit} disabled={!canSubmit}>
              {registerGateway.isPending
                ? t('gateways:register.submitting', 'Registering...')
                : t('gateways:register.submit', 'Register')}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
