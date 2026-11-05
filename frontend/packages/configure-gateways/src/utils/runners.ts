// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppliedVersion} from '../models/deploy';
import type {Gateway} from '../models/gateway';

export interface Runners {
  /** The names of the targets that run each version, by its hash. */
  running: Record<string, string[]>;
  /** The names of the targets that hold the version they run only in part, by its hash. */
  partly: Record<string, string[]>;
}

/**
 * Where each version is deployed, so a list of versions can name the targets that run it, and keep one
 * a target holds only in part deployable to it. A target is named by `nameOf`, its own name otherwise.
 */
export default function runnersOf(
  gateways: Gateway[],
  applied: Record<string, AppliedVersion | undefined>,
  nameOf: (gateway: Gateway) => string = (gateway: Gateway) => gateway.name,
): Runners {
  const running: Record<string, string[]> = {};
  const partly: Record<string, string[]> = {};
  for (const gateway of gateways) {
    const entry = applied[gateway.id];
    const hash = entry?.appliedVersion;
    if (!hash) continue;
    running[hash] = [...(running[hash] ?? []), nameOf(gateway)];
    if (entry.complete === false) partly[hash] = [...(partly[hash] ?? []), nameOf(gateway)];
  }
  return {running, partly};
}
