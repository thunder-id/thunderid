// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useProjectScope as useSelectedProject} from '@thunderid/contexts';

/**
 * The id of the project deploys are scoped to, or undefined for the whole organization: the project the
 * console selected, through `ProjectScopeProvider` of `@thunderid/contexts`. Under it the pipeline is
 * the project's, a deploy check counts its changes, and a deploy carries its own resources with what
 * they need.
 */
export default function useProjectScope(): string | undefined {
  return useSelectedProject().projectId;
}
