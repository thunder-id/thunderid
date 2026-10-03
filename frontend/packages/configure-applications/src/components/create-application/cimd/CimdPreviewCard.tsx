// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Card, CardContent, Chip, Divider, Stack, Typography} from '@wso2/oxygen-ui';
import {FileBadge, KeyRound, Laptop, UserRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CimdPreview} from '../../../models/cimd';
import isLoopbackRedirectUri from '../../../utils/isLoopbackRedirectUri';

/**
 * Props for the {@link CimdPreviewCard} component.
 *
 * @public
 */
export interface CimdPreviewCardProps {
  /** The validated document, mapped to ThunderID's OAuth client values. */
  preview: CimdPreview;
  /** Optional marker per redirect URI, used by the re-fetch dialog to show additions and removals. */
  redirectUriMarker?: (uri: string) => ReactNode;
  /** Redirect URIs to list in addition to the preview's own, for example ones a re-fetch would remove. */
  extraRedirectUris?: string[];
}

function Row({label, children}: {label: string; children: ReactNode}): JSX.Element {
  return (
    <Stack direction={{xs: 'column', sm: 'row'}} spacing={{xs: 0.5, sm: 2}} alignItems="flex-start">
      <Typography variant="body2" color="text.secondary" sx={{minWidth: 140}}>
        {label}
      </Typography>
      <Box sx={{flex: 1, minWidth: 0}}>{children}</Box>
    </Stack>
  );
}

/**
 * What the administrator approves when registering a client from its Client ID Metadata Document:
 * who publishes it, how it authenticates, and where sign-in may return to.
 *
 * @public
 */
export default function CimdPreviewCard({
  preview,
  redirectUriMarker = undefined,
  extraRedirectUris = [],
}: CimdPreviewCardProps): JSX.Element {
  const {t} = useTranslation();
  const isConfidential = preview.tokenEndpointAuthMethod === 'private_key_jwt';

  return (
    <Card variant="outlined" data-testid="cimd-preview-card">
      <CardContent>
        <Stack spacing={2}>
          <Stack direction="row" spacing={1.5} alignItems="center" justifyContent="space-between">
            <Stack direction="row" spacing={1.5} alignItems="center">
              <FileBadge size={22} />
              <Typography variant="h5">{preview.clientName}</Typography>
            </Stack>
            <Chip
              size="small"
              variant="outlined"
              label={t('applications:cimd.preview.publishedBy', 'Published by {{host}}', {host: preview.clientIdHost})}
            />
          </Stack>

          <Divider />

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

          {preview.jwksUri && (
            <Row label={t('applications:cimd.preview.keys', 'Keys')}>
              <Typography variant="body2" sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}>
                {preview.jwksUri}
              </Typography>
            </Row>
          )}

          {preview.hasInlineJwks && (
            <Row label={t('applications:cimd.preview.keys', 'Keys')}>
              <Typography variant="body2">
                {t('applications:cimd.preview.inlineKeys', 'Included in the document')}
              </Typography>
            </Row>
          )}

          <Row label={t('applications:cimd.preview.redirectUris', 'Redirect URIs')}>
            <Stack spacing={0.5}>
              {[...preview.redirectUris, ...extraRedirectUris].map((uri) => (
                <Stack key={uri} direction="row" spacing={1} alignItems="center">
                  {redirectUriMarker?.(uri)}
                  <Typography variant="body2" sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}>
                    {uri}
                  </Typography>
                  {isLoopbackRedirectUri(uri) && <Laptop size={14} aria-label="loopback" />}
                </Stack>
              ))}
            </Stack>
          </Row>

          <Row label={t('applications:cimd.preview.grants', 'Grants')}>
            <Typography variant="body2">{preview.grantTypes.join(', ')}</Typography>
          </Row>

          {preview.loopbackOnly && (
            <Alert severity="warning" icon={<Laptop size={18} />}>
              {t(
                'applications:cimd.preview.loopbackWarning',
                "This client only returns to the user's device. Any program on the device can present this identity.",
              )}
            </Alert>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}
