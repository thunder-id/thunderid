/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {QrCode} from '@thunderid/design';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useInitiateVerification from '../api/useInitiateVerification';
import useVerificationStatus from '../api/useVerificationStatus';

export interface VerificationDialogProps {
  open: boolean;
  handle: string | null;
  onClose: () => void;
}

interface DecodedResult {
  claims: {name: string; value: string}[];
  keyBinding: boolean;
}

/** Decodes the verified_claims and holder-key-binding marker from a result token JWT. */
function decodeResult(token: string): DecodedResult | null {
  try {
    const part: string | undefined = token.split('.')[1];
    if (!part) {
      return null;
    }
    let b64: string = part.replace(/-/g, '+').replace(/_/g, '/');
    while (b64.length % 4 !== 0) {
      b64 += '=';
    }
    const payload = JSON.parse(atob(b64)) as {verified_claims?: Record<string, unknown>};
    const vc: Record<string, unknown> = payload.verified_claims ?? {};
    const claims: {name: string; value: string}[] = [];
    let keyBinding = false;
    Object.keys(vc).forEach((key: string): void => {
      if (key.startsWith('cnf.')) {
        keyBinding = true;
        return;
      }
      claims.push({name: key, value: String(vc[key])});
    });
    return {claims, keyBinding};
  } catch {
    return null;
  }
}

/**
 * Initiates an OpenID4VP verification transaction for a presentation definition,
 * renders its openid4vp:// request as a scannable QR, and polls for the live
 * result — showing the verified claims and holder key binding on completion.
 */
export default function VerificationDialog({open, handle, onClose}: VerificationDialogProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');
  const {mutate, reset, data, isPending, error} = useInitiateVerification();

  const txnId: string | null = data?.txn_id ?? null;
  const walletUrl: string = data?.wallet_url ?? '';
  const status = useVerificationStatus(open ? txnId : null);

  const [copied, setCopied] = useState<boolean>(false);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (open && handle) {
      mutate(handle);
    }
  }, [open, handle, mutate]);

  useEffect(
    () => () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    },
    [],
  );

  const handleCopy = useCallback(async (value: string): Promise<void> => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
  }, []);

  const handleClose = (): void => {
    reset();
    setCopied(false);
    onClose();
  };

  const statusValue: string | undefined = status.data?.status;
  const decoded: DecodedResult | null =
    statusValue === 'COMPLETED' && status.data?.result_token ? decodeResult(status.data.result_token) : null;

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('verify.title')}</DialogTitle>
      <DialogContent>
        {isPending && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
            <CircularProgress size={32} />
          </Box>
        )}
        {error && (
          <Alert severity="warning" sx={{mb: 2}}>
            {t('verify.notConfigured')}
          </Alert>
        )}
        {!isPending && walletUrl && (
          <Stack spacing={3} alignItems="center">
            <Typography variant="body2" color="text.secondary" textAlign="center">
              {t('verify.scanHint')}
            </Typography>
            <Box sx={{p: 2, bgcolor: 'common.white', borderRadius: 2}}>
              <QrCode value={walletUrl} size={240} />
            </Box>
            <Button variant="contained" component="a" href={walletUrl}>
              {t('verify.openInWallet')}
            </Button>
            <TextField
              fullWidth
              value={walletUrl}
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <Tooltip title={copied ? t('common:actions.copied') : t('verify.copy')}>
                      <IconButton
                        aria-label={t('verify.copy')}
                        edge="end"
                        onClick={(): void => {
                          handleCopy(walletUrl).catch(() => null);
                        }}
                      >
                        {copied ? <Check size={16} /> : <Copy size={16} />}
                      </IconButton>
                    </Tooltip>
                  </InputAdornment>
                ),
              }}
              sx={{'& input': {fontFamily: 'monospace', fontSize: '0.75rem'}}}
            />

            {(!statusValue || statusValue === 'PENDING') && (
              <Stack direction="row" spacing={1} alignItems="center">
                <CircularProgress size={16} />
                <Typography variant="body2" color="text.secondary">
                  {t('verify.waiting')}
                </Typography>
              </Stack>
            )}
            {statusValue === 'EXPIRED' && (
              <Alert severity="warning" sx={{width: '100%'}}>
                {t('verify.expired')}
              </Alert>
            )}
            {statusValue === 'FAILED' && (
              <Alert severity="error" sx={{width: '100%'}}>
                {status.data?.error ?? t('verify.failed')}
              </Alert>
            )}
            {statusValue === 'COMPLETED' && (
              <Stack spacing={2} sx={{width: '100%'}}>
                <Alert severity="success">{t('verify.completed')}</Alert>
                {decoded?.keyBinding && (
                  <Box>
                    <Chip color="success" size="small" label={t('verify.keyBindingVerified')} />
                  </Box>
                )}
                {decoded && decoded.claims.length > 0 && (
                  <Box>
                    <Typography variant="subtitle2" gutterBottom>
                      {t('verify.claimsTitle')}
                    </Typography>
                    <Stack spacing={1}>
                      {decoded.claims.map((claim) => (
                        <Stack key={claim.name} direction="row" spacing={2} justifyContent="space-between">
                          <Typography variant="body2" color="text.secondary">
                            {claim.name}
                          </Typography>
                          <Typography variant="body2" sx={{fontFamily: 'monospace'}}>
                            {claim.value}
                          </Typography>
                        </Stack>
                      ))}
                    </Stack>
                  </Box>
                )}
              </Stack>
            )}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>{t('common:actions.close')}</Button>
      </DialogActions>
    </Dialog>
  );
}
