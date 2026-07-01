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
} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useCreateCredentialOffer from '../api/useCreateCredentialOffer';

export interface CredentialOfferDialogProps {
  open: boolean;
  handle: string | null;
  onClose: () => void;
}

/**
 * Generates an issuer-initiated credential offer for a credential configuration
 * and shows its openid-credential-offer:// deep link as a scannable QR + copyable link.
 */
export default function CredentialOfferDialog({open, handle, onClose}: CredentialOfferDialogProps): JSX.Element {
  const {t} = useTranslation('verifiable-credentials');
  const createOffer = useCreateCredentialOffer();
  const {mutate, reset, data, isPending, error} = createOffer;

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

  const deepLink = data?.credential_offer_uri ?? '';

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

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('offer.title')}</DialogTitle>
      <DialogContent>
        {isPending && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
            <CircularProgress size={32} />
          </Box>
        )}
        {error && (
          <Alert severity="warning" sx={{mb: 2}}>
            {t('offer.notConfigured')}
          </Alert>
        )}
        {!isPending && deepLink && (
          <Stack spacing={3} alignItems="center">
            <Box sx={{p: 2, bgcolor: 'common.white', borderRadius: 2}}>
              <QrCode value={deepLink} size={240} />
            </Box>
            <Button variant="contained" component="a" href={deepLink}>
              {t('offer.openInWallet')}
            </Button>
            <TextField
              fullWidth
              value={deepLink}
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <Tooltip title={copied ? t('common:actions.copied') : t('offer.copy')}>
                      <IconButton
                        aria-label={t('offer.copy')}
                        edge="end"
                        onClick={(): void => {
                          handleCopy(deepLink).catch(() => null);
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
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>{t('common:actions.close')}</Button>
      </DialogActions>
    </Dialog>
  );
}
