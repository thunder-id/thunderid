// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Change} from '../models/deploy';

/** A change is sent when it writes its resource: a removal or an unchanged resource needs nothing sent. */
const sends = (change: Change): boolean => change.change === 'added' || change.change === 'updated';

/**
 * What a selection brings along: each resource a selected change needs that the target lacks or holds
 * differently, through what those need in turn, as the control plane widens a selection deployed with
 * dependencies. Returns the keys brought along, each with the selected key that first needed it.
 */
export function withNeeded(selected: ReadonlySet<string>, changes: Change[]): Map<string, string> {
  const byKey = new Map(changes.map((change: Change): [string, Change] => [change.key, change]));
  const required = new Map<string, string>();
  const chosen = new Set(selected);
  const queue = [...selected];
  while (queue.length > 0) {
    const key = queue.shift()!;
    const from = byKey.get(key);
    // A removal needs nothing sent, whatever its document named.
    if (!from || !sends(from)) continue;
    for (const needed of from.needs ?? []) {
      const change = byKey.get(needed);
      if (!change || chosen.has(needed) || !sends(change)) continue;
      chosen.add(needed);
      required.set(needed, required.get(key) ?? key);
      queue.push(needed);
    }
  }
  return required;
}
