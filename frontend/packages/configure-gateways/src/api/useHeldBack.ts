// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import {useApplyCheck} from './useGatewayVersions';
import type {Change} from '../models/deploy';

/**
 * What a target holds back of the version it runs: the changes a deploy left out, which the target
 * goes on leaving alone until a later deploy selects them again. It is read as what applying the
 * running version again would leave alone, so nothing is written.
 */
export default function useHeldBack(targetId: string, runningVersion: string | undefined): Change[] {
  const check = useApplyCheck(targetId, runningVersion);
  return useMemo(
    () => (check.data?.diff.changes ?? []).filter((change: Change) => change.excluded && change.change !== 'unchanged'),
    [check.data],
  );
}
