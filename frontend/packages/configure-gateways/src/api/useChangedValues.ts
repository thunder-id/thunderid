// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useApplyCheck} from './useGatewayVersions';
import type {ChangedValues} from '../models/deploy';

/** What the check of a target's changed values found, or that it could not find out. */
export interface ChangedValuesCheck {
  /** The values changed, undefined until the check answers and when it failed. */
  changed?: ChangedValues;
  /** The check failed, so whether any value changed is not known. */
  failed: boolean;
  /** Runs the check again. */
  retry: () => void;
}

/**
 * The values set on a target since the version it runs was deployed, such as a client secret
 * regenerated or a secret updated in its store. The target fills values in only when a version is
 * deployed, so these take effect once the version it runs is deployed again; the version itself has
 * not changed, so nothing else shows it is owed. Read from the same check `useHeldBack` reads, so it
 * costs no request of its own. A failed check is reported as one, not as no value changed.
 */
export default function useChangedValues(targetId: string, runningVersion: string | undefined): ChangedValuesCheck {
  const check = useApplyCheck(targetId, runningVersion);
  return {
    changed: check.data?.changed,
    failed: check.isError,
    retry: (): void => {
      check.refetch().catch(() => undefined);
    },
  };
}

/** How many values a target holds that were set since its version was deployed. */
export function changedCount(changed: ChangedValues | undefined): number {
  return (changed?.variables?.length ?? 0) + (changed?.secrets?.length ?? 0);
}
