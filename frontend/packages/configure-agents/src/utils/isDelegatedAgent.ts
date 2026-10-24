// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DELEGATION_GRANTS} from '../constants/delegationGrants';
import type {AgentInboundAuthConfig} from '../models/agent';

/**
 * Whether the grant types include one that lets an agent act on behalf of a signed-in user:
 * authorization code, CIBA or token exchange. Delegated mode is on exactly then.
 */
export function hasDelegationGrant(grantTypes?: string[]): boolean {
  return (grantTypes ?? []).some((grant) => DELEGATION_GRANTS.includes(grant));
}

/**
 * Whether an agent acts on behalf of a signed-in user. The server does not store the flag, so an
 * update has to send it for the agent to be held to the delegated grant rules.
 */
export default function isDelegatedAgent(inboundAuthConfig?: AgentInboundAuthConfig[]): boolean {
  return hasDelegationGrant(inboundAuthConfig?.find((auth) => auth.type === 'oauth2')?.config?.grantTypes);
}
