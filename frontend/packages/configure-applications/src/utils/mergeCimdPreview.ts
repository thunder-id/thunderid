// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isCimdGrantAllowed} from './oauth2Rules';
import {OAuth2GrantTypes} from '../models/oauth';
import type {OAuth2Config} from '../models/oauth';

/**
 * Applies a re-fetched metadata document to a CIMD client's OAuth configuration. The document owns the
 * redirect URIs, the authentication method and its keys, PKCE, and the grants it declares. The other
 * grants the application already holds are kept while the document rules still permit them for the
 * document's authentication method; when token exchange is dropped that way, identity assertions are
 * turned off. Every other setting stays as configured.
 */
export default function mergeCimdPreview(current: OAuth2Config, preview: OAuth2Config): OAuth2Config {
  const previewGrants = preview.grantTypes ?? [];
  const keptGrants = (current.grantTypes ?? []).filter(
    (grant) =>
      grant !== OAuth2GrantTypes.AUTHORIZATION_CODE &&
      !previewGrants.includes(grant) &&
      isCimdGrantAllowed(grant, preview.tokenEndpointAuthMethod),
  );
  const grantTypes = [...previewGrants, ...keptGrants];
  const {token} = current;
  const turnsIdJagOff = token?.idJag?.enabled === true && !grantTypes.includes(OAuth2GrantTypes.TOKEN_EXCHANGE);

  return {
    ...current,
    redirectUris: preview.redirectUris,
    tokenEndpointAuthMethod: preview.tokenEndpointAuthMethod,
    publicClient: preview.publicClient,
    pkceRequired: preview.pkceRequired,
    certificate: preview.certificate ?? null,
    grantTypes,
    ...(token && turnsIdJagOff && {token: {...token, idJag: {...token.idJag, enabled: false}}}),
  };
}
