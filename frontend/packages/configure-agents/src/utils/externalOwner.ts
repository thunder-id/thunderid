// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AgentOwner, AgentOwnerIdType} from '../models/agent';

/** The longest owner ID the server stores, in characters. */
const MAX_OWNER_ID_LENGTH = 256;

/** An E.164 number, as the server checks it. */
const MOBILE_PATTERN = /^\+[1-9][0-9]{6,14}$/;

/** A bare address. The server parses it strictly, so this only catches what is plainly not one. */
const EMAIL_PATTERN = /^[^\s@<>(),;:"]+@[^\s@<>(),;:"]+$/;

/**
 * Why an external owner's ID is not accepted, or undefined when it is. The server makes the same
 * checks and rejects a failing ID with AGT-1049, so this only saves a round trip.
 */
export function getExternalOwnerIdError(
  id: string,
  idType: AgentOwnerIdType,
): 'empty' | 'tooLong' | 'format' | undefined {
  const trimmed = id.trim();
  if (!trimmed) return 'empty';
  if ([...trimmed].length > MAX_OWNER_ID_LENGTH) return 'tooLong';
  if (idType === 'email' && !EMAIL_PATTERN.test(trimmed)) return 'format';
  if (idType === 'mobile' && !MOBILE_PATTERN.test(trimmed)) return 'format';
  return undefined;
}

/** Whether an owner is external and its ID would be refused. A local or absent owner never is. */
export function isInvalidExternalOwner(owner?: AgentOwner): boolean {
  return owner?.isLocal === false && getExternalOwnerIdError(owner.id, owner.idType ?? 'uuid') !== undefined;
}
