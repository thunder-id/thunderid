// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {KeyRound, Laptop, Minus, Plus, RefreshCw, UserRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import usePreviewCimdDocument from '../../../api/usePreviewCimdDocument';
import CertificateTypes from '../../../constants/certificate-types';
import type {Application} from '../../../models/application';
import type {CimdPreview} from '../../../models/cimd';
import {InboundAuthTypes} from '../../../models/inbound-auth';
import type {OAuth2Config} from '../../../models/oauth';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import getCimdErrorMessage from '../../../utils/getCimdErrorMessage';
import isLoopbackRedirectUri from '../../../utils/isLoopbackRedirectUri';
import CimdPreviewCard from '../../create-application/cimd/CimdPreviewCard';

/**
 * Props for the {@link CimdDocumentSection} component.
 *
 * @public
 */
export interface CimdDocumentSectionProps {
  application: Application;
  oauth2Config: OAuth2Config;
  onFieldChange: (field: keyof Application, value: unknown) => void;
  isReadOnly: boolean;
}

function Row({label, children}: {label: string; children: ReactNode}): JSX.Element {
  return (
    <Stack direction={{xs: 'column', sm: 'row'}} spacing={{xs: 0.5, sm: 2}} alignItems="flex-start">
      <Typography variant="body2" color="text.secondary" sx={{minWidth: 160}}>
        {label}
      </Typography>
      <Box sx={{flex: 1, minWidth: 0}}>{children}</Box>
    </Stack>
  );
}

/**
 * The values a metadata-document client takes from its document, read-only, with a Re-fetch action
 * that previews what changed and applies it on confirmation. Applied values join the page's pending
 * edits and are saved with the page's Save action.
 *
 * @public
 */
export default function CimdDocumentSection({
  application,
  oauth2Config,
  onFieldChange,
  isReadOnly,
}: CimdDocumentSectionProps): JSX.Element {
  const {t} = useTranslation();
  const previewDocument = usePreviewCimdDocument();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [updateDetails, setUpdateDetails] = useState(false);

  const currentRedirectUris = oauth2Config.redirectUris ?? [];
  const currentJwksUri =
    oauth2Config.certificate?.type === CertificateTypes.JWKS_URI ? oauth2Config.certificate.value : undefined;
  const isConfidential = oauth2Config.tokenEndpointAuthMethod === TokenEndpointAuthMethods.PRIVATE_KEY_JWT;

  const openRefetch = (): void => {
    setUpdateDetails(false);
    setDialogOpen(true);
    previewDocument.mutate(oauth2Config.clientId ?? '');
  };

  const preview: CimdPreview | undefined = previewDocument.data;
  const removedUris = preview ? currentRedirectUris.filter((uri) => !preview.redirectUris.includes(uri)) : [];
  const addedUris = preview ? preview.redirectUris.filter((uri) => !currentRedirectUris.includes(uri)) : [];
  const authChanged = preview ? preview.tokenEndpointAuthMethod !== oauth2Config.tokenEndpointAuthMethod : false;
  const keysChanged = preview
    ? JSON.stringify(preview.oauth2Config.certificate ?? null) !== JSON.stringify(oauth2Config.certificate ?? null)
    : false;
  const hasChanges = removedUris.length > 0 || addedUris.length > 0 || authChanged || keysChanged;

  const applyDocument = (): void => {
    if (!preview) return;
    // The previewed values replace the document's fields as returned; everything else stays as configured.
    const updatedConfig: OAuth2Config = {
      ...oauth2Config,
      ...preview.oauth2Config,
      certificate: preview.oauth2Config.certificate ?? null,
    };
    onFieldChange(
      'inboundAuthConfig',
      application.inboundAuthConfig?.map((config) =>
        config.type === InboundAuthTypes.OAUTH2 ? {...config, config: updatedConfig} : config,
      ),
    );
    if (updateDetails) {
      onFieldChange('name', preview.clientName);
      if (preview.clientUri) onFieldChange('url', preview.clientUri);
      if (preview.tosUri) onFieldChange('tosUri', preview.tosUri);
      if (preview.policyUri) onFieldChange('policyUri', preview.policyUri);
      if (preview.contacts.length > 0) onFieldChange('contacts', preview.contacts);
    }
    setDialogOpen(false);
  };

  const marker = (uri: string): ReactNode => {
    if (addedUris.includes(uri)) return <Plus size={14} color="green" aria-label="added" />;
    if (removedUris.includes(uri)) return <Minus size={14} color="red" aria-label="removed" />;
    return <Box sx={{width: 14}} />;
  };

  return (
    <>
      <SettingsCard
        title={t('applications:cimd.edit.title', 'CIMD')}
        description={t(
          'applications:cimd.edit.description',
          "These values come from the client's metadata document. Re-fetch it to apply the vendor's changes.",
        )}
        headerAction={
          <Button
            variant="outlined"
            size="small"
            startIcon={<RefreshCw size={14} />}
            onClick={openRefetch}
            disabled={isReadOnly}
            data-testid="cimd-refetch-button"
          >
            {t('applications:cimd.edit.refetch', 'Re-fetch document')}
          </Button>
        }
      >
        <Stack spacing={2}>
          <Row label={t('applications:cimd.edit.documentUrl', 'Document URL')}>
            <Typography variant="body2" sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}>
              {oauth2Config.clientId}
            </Typography>
          </Row>
          <Row label={t('applications:cimd.preview.signInType', 'Sign-in type')}>
            <Stack direction="row" spacing={1} alignItems="center">
              {isConfidential ? <KeyRound size={16} /> : <UserRound size={16} />}
              <Typography variant="body2">
                {isConfidential
                  ? t('applications:cimd.preview.confidential', 'Private key JWT, signed by the vendor')
                  : t('applications:cimd.preview.public', 'Public client, no secret')}
              </Typography>
            </Stack>
          </Row>
          {currentJwksUri && (
            <Row label={t('applications:cimd.preview.keys', 'Keys')}>
              <Typography variant="body2" sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}>
                {currentJwksUri}
              </Typography>
            </Row>
          )}
          <Row label={t('applications:cimd.preview.redirectUris', 'Redirect URIs')}>
            <Stack spacing={0.5}>
              {currentRedirectUris.map((uri) => (
                <Stack key={uri} direction="row" spacing={1} alignItems="center">
                  <Typography variant="body2" sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}>
                    {uri}
                  </Typography>
                  {isLoopbackRedirectUri(uri) && (
                    <Chip
                      size="small"
                      icon={<Laptop size={12} />}
                      label={t('applications:cimd.edit.device', 'Device')}
                    />
                  )}
                </Stack>
              ))}
            </Stack>
          </Row>
        </Stack>
      </SettingsCard>

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{t('applications:cimd.refetch.title', 'Re-fetch metadata document')}</DialogTitle>
        <DialogContent>
          {previewDocument.isPending && (
            <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
              <CircularProgress />
            </Box>
          )}
          {previewDocument.isError && (
            <Alert severity="error">
              {getCimdErrorMessage(previewDocument.error, t)}{' '}
              {t('applications:cimd.refetch.unchanged', 'The application is unchanged.')}
            </Alert>
          )}
          {preview && (
            <Stack spacing={2}>
              <Alert severity={hasChanges ? 'info' : 'success'}>
                {hasChanges
                  ? t(
                      'applications:cimd.refetch.changed',
                      'The document has changed. Review the differences before applying them.',
                    )
                  : t('applications:cimd.refetch.noChanges', 'The document matches what this application uses.')}
              </Alert>
              {authChanged && (
                <Alert severity="warning">
                  {t('applications:cimd.refetch.authChanged', 'The sign-in type changes from {{from}} to {{to}}.', {
                    from: oauth2Config.tokenEndpointAuthMethod,
                    to: preview.tokenEndpointAuthMethod,
                  })}
                </Alert>
              )}
              <CimdPreviewCard preview={preview} redirectUriMarker={marker} extraRedirectUris={removedUris} />
              <FormControlLabel
                control={<Checkbox checked={updateDetails} onChange={(_e, checked) => setUpdateDetails(checked)} />}
                label={t('applications:cimd.refetch.updateDetails', 'Also update the name and links from the document')}
              />
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>{t('common:actions.cancel', 'Cancel')}</Button>
          <Button
            variant="contained"
            onClick={applyDocument}
            disabled={!preview || (!hasChanges && !updateDetails)}
            data-testid="cimd-apply-button"
          >
            {t('applications:cimd.refetch.apply', 'Apply')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
