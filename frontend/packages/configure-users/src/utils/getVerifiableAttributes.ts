// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {User} from '@thunderid/types';
import type {UserTypeDefinition} from '../models/users';

/**
 * Returns a user's contact attributes that can be verified, each mapped to when its stored value was verified
 * (undefined while unverified): top-level email and mobile attributes that are not credentials and hold a
 * stored value. A read-only user has none, and so does a user whose verification was not read.
 */
export default function getVerifiableAttributes(
  user: User,
  schema: UserTypeDefinition | undefined,
): Map<string, string | undefined> {
  const verifiable = new Map<string, string | undefined>();

  const {verifiedAttributes} = user;

  if (user.isReadOnly || !verifiedAttributes) {
    return verifiable;
  }
  Object.entries(schema ?? {}).forEach(([attribute, fieldDef]) => {
    const value = user.attributes?.[attribute];

    if (
      (fieldDef.type === 'email' || fieldDef.type === 'mobile') &&
      !fieldDef.credential &&
      typeof value === 'string' &&
      value.trim() !== ''
    ) {
      verifiable.set(attribute, verifiedAttributes[attribute]?.verifiedAt);
    }
  });

  return verifiable;
}
