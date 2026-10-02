// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  AlertTitle,
  Box,
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
import useCaptureConfigurationVersion from '../api/useCaptureConfigurationVersion';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {ConfigurationVersion} from '../models/gateway';

export interface CaptureVersionDialogProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Captures the current configuration as a new numbered version, with an optional note.
 */
export default function CaptureVersionDialog({open, onClose}: CaptureVersionDialogProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('CaptureVersionDialog');
  const capture = useCaptureConfigurationVersion();
  const [note, setNote] = useState('');
  // A capture that left resources out stays open on what it left out, so it is seen before the version
  // is applied anywhere.
  const [captured, setCaptured] = useState<ConfigurationVersion | null>(null);

  const handleClose = (): void => {
    if (capture.isPending) return;
    setNote('');
    setCaptured(null);
    capture.reset();
    onClose();
  };

  const handleCapture = (): void => {
    capture.mutate(note.trim() ? {note: note.trim()} : {}, {
      onSuccess: (version: ConfigurationVersion) => {
        if (version.skipped?.length) {
          setCaptured(version);
          return;
        }
        setNote('');
        capture.reset();
        onClose();
      },
      onError: (err: Error) => {
        logger.error('Failed to capture configuration version', {error: err});
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('gateways:capture.title', 'Capture current configuration')}</DialogTitle>
      <DialogContent>
        {captured?.skipped?.length ? (
          <Alert severity="warning" sx={{mt: 1}}>
            <AlertTitle>
              {t('gateways:capture.skipped.title', 'Version {{version}} was captured without {{count}} resource', {
                version: captured.version,
                count: captured.skipped.length,
              })}
            </AlertTitle>
            <Typography variant="body2" sx={{mb: 1}}>
              {t(
                'gateways:capture.skipped.body',
                'These could not be exported, so the version does not include them. Fix them and capture again to include them.',
              )}
            </Typography>
            <Box component="ul" sx={{m: 0, pl: 2}}>
              {captured.skipped.map((resource) => (
                <li key={`${resource.resourceType}/${resource.resourceId ?? resource.error}`}>
                  <Typography variant="body2">
                    {t('gateways:capture.skipped.item', '{{type}} {{id}}: {{error}}', {
                      type: resource.resourceType,
                      id: resource.resourceId ?? '',
                      error: resource.error,
                    })}
                  </Typography>
                </li>
              ))}
            </Box>
          </Alert>
        ) : (
          <Stack spacing={3} sx={{pt: 1}}>
            <Typography variant="body2" color="text.secondary">
              {t(
                'gateways:capture.description',
                'Records the configuration as it is now as a new version, which can then be applied to a gateway. Users and groups are included.',
              )}
            </Typography>
            <FormControl fullWidth>
              <FormLabel htmlFor="capture-note-input">{t('gateways:capture.note.label', 'Note (optional)')}</FormLabel>
              <TextField
                id="capture-note-input"
                fullWidth
                value={note}
                onChange={(e) => {
                  if (capture.isError) capture.reset();
                  setNote(e.target.value);
                }}
                placeholder={t('gateways:capture.note.placeholder', 'e.g. Added the payments application')}
              />
            </FormControl>
            {capture.error && (
              <Alert severity="error">
                {getErrorMessage(
                  capture.error,
                  tForErrors,
                  'capture.error',
                  'The configuration could not be captured. Please try again.',
                )}
              </Alert>
            )}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        {captured ? (
          <Button variant="contained" onClick={handleClose}>
            {t('gateways:capture.done', 'Done')}
          </Button>
        ) : (
          <>
            <Button variant="outlined" onClick={handleClose} disabled={capture.isPending}>
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button variant="contained" onClick={handleCapture} disabled={capture.isPending}>
              {capture.isPending
                ? t('gateways:capture.submitting', 'Capturing...')
                : t('gateways:capture.submit', 'Capture')}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
