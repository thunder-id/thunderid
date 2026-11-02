// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import isLoopbackRedirectUri from './isLoopbackRedirectUri';
import CertificateTypes from '../constants/certificate-types';
import type {CimdPreview, CimdPreviewResponse} from '../models/cimd';
import type {OAuth2Config} from '../models/oauth';

/**
 * Maps a `POST /cimd/preview` response to what the review shows, keeping the returned OAuth
 * configuration unchanged so that create stores exactly what was approved.
 */
export default function toCimdPreview(response: CimdPreviewResponse): CimdPreview {
  const oauth2Config: OAuth2Config = response.inboundAuthConfig[0]?.config ?? {};
  const clientId = oauth2Config.clientId ?? '';
  const redirectUris = oauth2Config.redirectUris ?? [];
  const certificate = oauth2Config.certificate;

  return {
    clientId,
    // The server returns only valid Client Identifier URLs.
    clientIdHost: new URL(clientId).host,
    clientName: response.name,
    clientUri: response.url,
    tosUri: response.tosUri,
    policyUri: response.policyUri,
    contacts: response.contacts ?? [],
    tokenEndpointAuthMethod: oauth2Config.tokenEndpointAuthMethod ?? '',
    jwksUri: certificate?.type === CertificateTypes.JWKS_URI ? certificate.value : undefined,
    hasInlineJwks: certificate?.type === CertificateTypes.JWKS,
    redirectUris,
    grantTypes: oauth2Config.grantTypes ?? [],
    loopbackOnly: redirectUris.length > 0 && redirectUris.every(isLoopbackRedirectUri),
    oauth2Config,
  };
}
