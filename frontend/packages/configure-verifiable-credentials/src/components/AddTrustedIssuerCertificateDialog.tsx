// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormHelperText,
  FormLabel,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import {Upload} from '@wso2/oxygen-ui-icons-react';
import {useState, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/** A single PEM block: the BEGIN/END CERTIFICATE lines around base64 content. */
const PEM_CERTIFICATE_PATTERN = /^-----BEGIN CERTIFICATE-----\s+[A-Za-z0-9+/=\s]+-----END CERTIFICATE-----$/;

export interface AddTrustedIssuerCertificateDialogProps {
  open: boolean;
  /** Names already registered on the definition; a new name must not repeat one of these. */
  existingNames: string[];
  onClose: () => void;
  onAdd: (entry: {name: string; certificate: string}) => void;
}

interface FieldErrors {
  name?: string;
  certificate?: string;
}

/**
 * Dialog to add a trusted issuer certificate to a presentation definition. The certificate is pasted
 * as PEM or read from an uploaded file. Only the PEM shape is checked here; the server validates the
 * certificate itself when the definition is saved.
 */
export default function AddTrustedIssuerCertificateDialog({
  open,
  existingNames,
  onClose,
  onAdd,
}: AddTrustedIssuerCertificateDialogProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');

  const [name, setName] = useState<string>('');
  const [certificate, setCertificate] = useState<string>('');
  const [errors, setErrors] = useState<FieldErrors>({});

  const resetAndClose = (): void => {
    setName('');
    setCertificate('');
    setErrors({});
    onClose();
  };

  const validate = (): FieldErrors => {
    const result: FieldErrors = {};
    const trimmedName = name.trim();
    const trimmedCertificate = certificate.trim();

    if (trimmedName === '') {
      result.name = t('form.issuerTrust.addDialog.errors.nameRequired', 'Enter a name for the certificate.');
    } else if (existingNames.includes(trimmedName)) {
      result.name = t(
        'form.issuerTrust.addDialog.errors.nameDuplicate',
        'A certificate with this name already exists in this presentation definition.',
      );
    }

    if (trimmedCertificate === '') {
      result.certificate = t('form.issuerTrust.addDialog.errors.certificateRequired', 'Paste or upload a certificate.');
    } else if (!PEM_CERTIFICATE_PATTERN.test(trimmedCertificate)) {
      result.certificate = t(
        'form.issuerTrust.addDialog.errors.certificateInvalid',
        'Enter a single PEM-encoded X.509 certificate, including its BEGIN CERTIFICATE and END CERTIFICATE lines.',
      );
    }

    return result;
  };

  const handleAdd = (): void => {
    const result = validate();
    if (result.name ?? result.certificate) {
      setErrors(result);
      return;
    }
    onAdd({name: name.trim(), certificate: certificate.trim()});
    resetAndClose();
  };

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>): void => {
    const input = e.target;
    const file = input.files?.[0];
    // Clear the selection so picking the same file again still fires a change event.
    input.value = '';
    if (!file) {
      return;
    }
    file
      .text()
      .then((content: string): void => {
        setCertificate(content);
        setErrors((prev: FieldErrors) => ({...prev, certificate: undefined}));
      })
      .catch((): void => {
        setErrors((prev: FieldErrors) => ({
          ...prev,
          certificate: t(
            'form.issuerTrust.addDialog.errors.fileRead',
            'The file could not be read. Try again or paste the certificate instead.',
          ),
        }));
      });
  };

  return (
    <Dialog open={open} onClose={resetAndClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('form.issuerTrust.addDialog.title', 'Add trusted issuer certificate')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 3}}>
          {t(
            'form.issuerTrust.addDialog.description',
            'Paste a PEM-encoded X.509 certificate or upload a certificate file.',
          )}
        </DialogContentText>
        <Stack spacing={3}>
          <FormControl fullWidth required>
            <FormLabel htmlFor="vp-trusted-certificate-name">
              {t('form.issuerTrust.addDialog.name.label', 'Name')}
            </FormLabel>
            <TextField
              fullWidth
              id="vp-trusted-certificate-name"
              value={name}
              placeholder={t('form.issuerTrust.addDialog.name.placeholder', 'Example issuer CA')}
              error={errors.name !== undefined}
              helperText={
                errors.name ??
                t(
                  'form.issuerTrust.addDialog.name.hint',
                  'A label that identifies this certificate within the presentation definition.',
                )
              }
              onChange={(e: ChangeEvent<HTMLInputElement>): void => {
                setName(e.target.value);
                setErrors((prev: FieldErrors) => ({...prev, name: undefined}));
              }}
            />
          </FormControl>
          <Stack spacing={1}>
            <FormControl fullWidth required>
              <FormLabel htmlFor="vp-trusted-certificate-pem">
                {t('form.issuerTrust.addDialog.certificate.label', 'Certificate')}
              </FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={6}
                maxRows={12}
                id="vp-trusted-certificate-pem"
                value={certificate}
                placeholder={t(
                  'form.issuerTrust.addDialog.certificate.placeholder',
                  'Paste the PEM-encoded certificate here',
                )}
                error={errors.certificate !== undefined}
                helperText={
                  errors.certificate ??
                  t('form.issuerTrust.addDialog.certificate.hint', 'A single PEM-encoded X.509 certificate.')
                }
                onChange={(e: ChangeEvent<HTMLInputElement>): void => {
                  setCertificate(e.target.value);
                  setErrors((prev: FieldErrors) => ({...prev, certificate: undefined}));
                }}
                sx={{'& textarea': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
              />
            </FormControl>
            <Stack direction="row" spacing={2} alignItems="center">
              <Button component="label" variant="outlined" size="small" startIcon={<Upload size={16} />}>
                {t('form.issuerTrust.addDialog.upload', 'Upload file')}
                <input
                  hidden
                  type="file"
                  accept=".pem,.crt,.cer"
                  data-testid="vp-trusted-certificate-file"
                  onChange={handleFileChange}
                />
              </Button>
              <FormHelperText sx={{m: 0}}>
                {t('form.issuerTrust.addDialog.uploadHint', 'Accepts .pem, .crt, and .cer files in PEM format.')}
              </FormHelperText>
            </Stack>
          </Stack>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={resetAndClose}>{t('common:actions.cancel')}</Button>
        <Button variant="contained" onClick={handleAdd}>
          {t('common:actions.add')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
