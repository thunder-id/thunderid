// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries} from '@tanstack/react-query';
import useDeploySource from './source';
import {previewQuery, usePreview} from './useGatewayVersions';
import useProjectScope from '../contexts/ProjectScope';
import type {ChangedValues} from '../models/deploy';

/** What the preview of a target's changed values found, or that it could not find out. */
export interface ChangedValuesCheck {
  /** The values changed, undefined until the preview answers and when it failed. */
  changed?: ChangedValues;
  /** The preview failed, so whether any value changed is not known. */
  failed: boolean;
  /** Reads the preview again. */
  retry: () => void;
}

/**
 * The values set on a target since the version it runs was deployed, such as a client secret
 * regenerated or a secret updated in its store. The target fills values in only when a version is
 * deployed, so these take effect once the version it runs is deployed again; the version itself has
 * not changed, so nothing else shows it is owed. They are read from the preview of deploying that version
 * again, which compares when the target's store says each value was set with when it was at the deploy.
 * A failed preview is reported as one, not as no value changed.
 */
export default function useChangedValues(targetId: string, runningVersion: string | undefined): ChangedValuesCheck {
  const preview = usePreview(targetId, runningVersion);
  return {
    changed: preview.data?.changed,
    failed: preview.isError,
    retry: (): void => {
      preview.refetch().catch(() => undefined);
    },
  };
}

/** How many values a target holds that were set since its version was deployed. */
export function changedCount(changed: ChangedValues | undefined): number {
  return (changed?.variables?.length ?? 0) + (changed?.secrets?.length ?? 0);
}

/**
 * The targets, of those given with the version each runs, that hold a value set since that version
 * was deployed, so the version takes a deploy again. Read from the same preview `useChangedValues`
 * reads, so it costs no request of its own. A target whose preview failed or has not answered is not
 * counted.
 */
export function useTargetsWithChangedValues(running: Record<string, string | undefined>): string[] {
  const source = useDeploySource();
  const project = useProjectScope();
  const targetIds = Object.keys(running);
  return useQueries({
    queries: targetIds.map((targetId: string) => previewQuery(source, targetId, running[targetId], project)),
    combine: (results) =>
      targetIds.filter((_targetId: string, index: number) => changedCount(results[index]?.data?.changed) > 0),
  });
}
