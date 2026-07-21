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

import type {TrustedIssuer} from '../models/trusted-issuer';
import type {ConnectionResponse} from '@thunderid/configure-connections';

/**
 * Maps an OIDC connection API response to the narrower trusted-issuer shape this feature works
 * with. `idJagEnabled` defaults to `false` for the (unreachable in practice) case where it is
 * missing — callers are expected to have already filtered to entries where it is present.
 */
export default function mapConnectionToTrustedIssuer(connection: ConnectionResponse): TrustedIssuer {
  return {
    id: connection.id,
    name: connection.name,
    issuer: connection.issuer ?? '',
    jwksEndpoint: connection.jwksEndpoint ?? '',
    idJagEnabled: connection.idJagEnabled ?? false,
    tokenExchangeEnabled: connection.tokenExchangeEnabled ?? false,
    trustedTokenAudience: connection.trustedTokenAudience ?? undefined,
  };
}
