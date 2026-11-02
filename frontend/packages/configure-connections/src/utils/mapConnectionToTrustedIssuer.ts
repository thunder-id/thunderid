// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ConnectionResponse} from '../models/connection';
import type {TrustedIssuer} from '../models/trusted-issuer';

/**
 * Maps an OIDC connection API response to the narrower trusted-issuer shape this feature works
 * with. `idJagEnabled` defaults to `false` for the (unreachable in practice) case where it is
 * missing — callers are expected to have already filtered to entries where it is present.
 */
export default function mapConnectionToTrustedIssuer(connection: ConnectionResponse): TrustedIssuer {
  return {
    id: connection.id,
    name: connection.name,
    ouId: connection.ouId,
    issuer: connection.issuer ?? '',
    jwksEndpoint: connection.jwksEndpoint ?? '',
    idJagEnabled: connection.idJagEnabled ?? false,
    tokenExchangeEnabled: connection.tokenExchangeEnabled ?? false,
    trustedTokenAudience: connection.trustedTokenAudience ?? undefined,
  };
}
