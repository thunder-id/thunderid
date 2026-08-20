// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useRegenerateEnvironmentSecret from '../api/useRegenerateEnvironmentSecret';
import type {SecretEntry} from '../models/promotion';

interface GenerateMissingSecretsDialogProps {
  open: boolean;
  envId: string;
  /** Every credential the Data Plane does not hold, of both kinds. */
  missing: SecretEntry[];
  onClose: () => void;
}

/** One generated credential, kept only until the dialog closes. */
interface Issued {
  name: string;
  value: string;
  error?: string;
}

/**
 * Fills every missing credential that can be generated, in one pass.
 *
 * Setting these one at a time is the common case after a first capture, when nothing has been captured
 * yet and every application, agent and user needs one. The values are shown together afterwards because
 * the Data Plane keeps only hashes of them.
 */
export default function GenerateMissingSecretsDialog({
  open,
  envId,
  missing,
  onClose,
}: GenerateMissingSecretsDialogProps): JSX.Element {
  const {t} = useTranslation();
  const regenerate = useRegenerateEnvironmentSecret();
  const [issued, setIssued] = useState<Issued[]>([]);
  const [running, setRunning] = useState<boolean>(false);
  const [copied, setCopied] = useState<boolean>(false);

  // Reset as the dialog opens, during render rather than in an effect: an effect would render the
  // stale contents once before clearing them.
  const [wasOpen, setWasOpen] = useState<boolean>(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setIssued([]);
      setRunning(false);
      setCopied(false);
    }
  }

  const generatable: SecretEntry[] = missing.filter((secret: SecretEntry) => secret.kind === 'hash');
  const manual: SecretEntry[] = missing.filter((secret: SecretEntry) => secret.kind !== 'hash');

  const handleGenerate = async (): Promise<void> => {
    setRunning(true);
    const results: Issued[] = [];
    // Sequential rather than concurrent: each write goes to the same Data Plane, and a partial failure
    // is easier to read when the order matches the list.
    for (const secret of generatable) {
      try {
        const result = await regenerate.mutateAsync({envId, name: secret.name});
        results.push({name: secret.name, value: result.value});
      } catch (error) {
        results.push({name: secret.name, value: '', error: (error as Error).message});
      }
    }
    setIssued(results);
    setRunning(false);
  };

  const handleCopyAll = (): void => {
    const body: string = issued
      .filter((entry: Issued) => !entry.error)
      .map((entry: Issued) => `${entry.name}=${entry.value}`)
      .join('\n');
    navigator.clipboard.writeText(body).then(
      () => {
        setCopied(true);
      },
      () => {
        // Clipboard access can be refused; the values stay on screen to copy by hand.
      },
    );
  };

  const failed: number = issued.filter((entry: Issued) => entry.error).length;

  return (
    <Dialog open={open} onClose={running ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {issued.length > 0
          ? t('promotions:secrets.generatedTitle', 'Secrets issued')
          : t('promotions:secrets.generateAllTitle', 'Generate missing secrets')}
      </DialogTitle>
      <DialogContent>
        {issued.length === 0 && (
          <>
            <DialogContentText>
              {t(
                'promotions:secrets.generateAllMessage',
                'A new value is issued for each of these and stored on the Data Plane. You will be shown the values once.',
              )}
            </DialogContentText>
            <Box sx={{mt: 2}}>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                {generatable.map((secret: SecretEntry) => (
                  <Chip key={secret.name} size="small" label={secret.name} />
                ))}
              </Stack>
            </Box>
            {manual.length > 0 && (
              <Alert severity="info" sx={{mt: 2}}>
                {t(
                  'promotions:secrets.generateAllManual',
                  '{{count}} credential is issued by an external service and has to be set by hand.',
                  {count: manual.length},
                )}
              </Alert>
            )}
          </>
        )}

        {issued.length > 0 && (
          <>
            <Alert severity={failed > 0 ? 'warning' : 'success'}>
              {failed > 0
                ? t('promotions:secrets.generatedPartial', '{{failed}} of {{total}} could not be stored', {
                    failed,
                    total: issued.length,
                  })
                : t('promotions:secrets.generatedAll', '{{count}} secret stored on the Data Plane', {
                    count: issued.length,
                  })}
            </Alert>
            <Alert severity="warning" sx={{mt: 2}}>
              {t(
                'promotions:secrets.generatedNotice',
                'These values are shown only now. Copy them before closing: the Data Plane keeps only hashes.',
              )}
            </Alert>
            <Box sx={{mt: 2, p: 2, bgcolor: 'action.hover', borderRadius: 1}}>
              {issued.map((entry: Issued) => (
                <Typography
                  key={entry.name}
                  variant="body2"
                  color={entry.error ? 'error' : undefined}
                  sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}
                >
                  {entry.error ? `${entry.name}: ${entry.error}` : `${entry.name}=${entry.value}`}
                </Typography>
              ))}
            </Box>
          </>
        )}
      </DialogContent>
      <DialogActions>
        {issued.length > 0 ? (
          <>
            <Button onClick={handleCopyAll}>
              {copied ? t('promotions:secrets.copied', 'Copied') : t('promotions:secrets.copyAll', 'Copy all')}
            </Button>
            <Button variant="contained" onClick={onClose}>
              {t('promotions:secrets.done', 'Done')}
            </Button>
          </>
        ) : (
          <>
            <Button onClick={onClose} disabled={running}>
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button
              variant="contained"
              disabled={running || generatable.length === 0}
              onClick={() => {
                void handleGenerate();
              }}
            >
              {running
                ? t('promotions:secrets.generating', 'Generating...')
                : t('promotions:secrets.generateCount', 'Generate {{count}}', {count: generatable.length})}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
