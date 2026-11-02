// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useThunderID} from '@thunderid/react';
import type {Actor} from '../models/deploy';

/** The fields of the signed-in user's profile that name them, in order of preference. */
const NAME_FIELDS = ['username', 'userName', 'preferred_username', 'name', 'email'];

/**
 * How a user is named: the name recorded with them, or, when none was and they are the one signed in,
 * the name they signed in with, and their id otherwise.
 */
export default function useActorName(actor: Actor): string {
  const {user} = useThunderID() as {user?: Record<string, unknown> | null};
  if (actor.name) return actor.name;
  if (user && (user['sub'] === actor.id || user['id'] === actor.id)) {
    const field = NAME_FIELDS.find((key: string) => typeof user[key] === 'string' && user[key] !== '');
    if (field) return String(user[field]);
  }
  return actor.id;
}
