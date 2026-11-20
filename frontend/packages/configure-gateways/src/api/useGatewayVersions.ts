// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useQuery,
  useQueryClient,
  type InvalidateQueryFilters,
  type QueryClient,
  type UseMutationResult,
  type UseQueryOptions,
  type UseQueryResult,
} from '@tanstack/react-query';
import useDeploySource, {type DeploySource} from './source';
import DeployQueryKeys from '../constants/query-keys';
import useProjectScope from '../contexts/ProjectScope';
import type {AppliedConfiguration, AppliedVersion, ApplyRequest, ApplyResult, ValueReference} from '../models/deploy';

/** Which version a gateway runs. */
export function useAppliedVersion(gatewayId: string, enabled = true): UseQueryResult<AppliedVersion> {
  const source = useDeploySource();
  return useQuery<AppliedVersion>({
    queryKey: [DeployQueryKeys.APPLIED_VERSION, gatewayId],
    queryFn: (): Promise<AppliedVersion> => source.appliedVersion(gatewayId),
    enabled: enabled && Boolean(gatewayId),
  });
}

/** Everything a gateway runs, each resource as its own read returns it. */
export function useAppliedConfiguration(gatewayId: string, enabled = true): UseQueryResult<AppliedConfiguration> {
  const source = useDeploySource();
  return useQuery<AppliedConfiguration>({
    queryKey: [DeployQueryKeys.APPLIED_CONFIGURATION, gatewayId],
    queryFn: (): Promise<AppliedConfiguration> => source.appliedConfiguration(gatewayId),
    enabled: enabled && Boolean(gatewayId),
    staleTime: 30_000,
    retry: false,
  });
}

/** How long a preview, or a project's status, is taken as current before it is read again. */
export const PREVIEW_STALE_TIME = 30_000;

/**
 * The query that previews applying a version to a gateway, shared by every reader of the preview. The
 * control plane works it out from what it records of the gateway, without the gateway checking the
 * version, so reading it costs the gateway nothing but a look at its store.
 */
export function previewQuery(
  source: DeploySource,
  gatewayId: string,
  version: string | undefined,
  project: string | undefined,
  enabled = true,
): UseQueryOptions<ApplyResult> {
  return {
    queryKey: [DeployQueryKeys.PREVIEW, gatewayId, version, project],
    queryFn: (): Promise<ApplyResult> => source.preview(gatewayId, version ?? '', project),
    enabled: enabled && Boolean(gatewayId) && Boolean(version),
    staleTime: PREVIEW_STALE_TIME,
    // The preview is read when the deploy panel opens, and the user then works through it: refetching
    // when the window regains focus would move the list under them.
    refetchOnWindowFocus: false,
    retry: false,
  };
}

/**
 * What applying a version would do: the changes, with their keys for a selection, the values the
 * gateway lacks and the removals. Nothing is written, and the gateway is not asked to check the version.
 * Under a project, each change says where it stands with the project.
 */
export function usePreview(
  gatewayId: string,
  version: string | undefined,
  enabled = true,
): UseQueryResult<ApplyResult> {
  const source = useDeploySource();
  const project = useProjectScope();
  return useQuery<ApplyResult>(previewQuery(source, gatewayId, version, project, enabled));
}

/** What putting a gateway back on what it ran before would do, previewed as `usePreview` previews a deploy. */
export function useRevertPreview(gatewayId: string): UseQueryResult<ApplyResult> {
  const source = useDeploySource();
  const project = useProjectScope();
  return useQuery<ApplyResult>({
    queryKey: [DeployQueryKeys.REVERT_PREVIEW, gatewayId, project],
    queryFn: (): Promise<ApplyResult> => source.revertPreview(gatewayId, project),
    enabled: Boolean(gatewayId),
    staleTime: PREVIEW_STALE_TIME,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/**
 * Asks the gateway itself what applying a version would do: it checks the version without writing it
 * and says what it would refuse. Run only when asked, since the gateway does the work.
 */
export function useApplyCheck(): UseMutationResult<ApplyResult, Error, ApplyCheckInput> {
  const source = useDeploySource();
  return useMutation<ApplyResult, Error, ApplyCheckInput>({
    mutationFn: ({gatewayId, data}: ApplyCheckInput): Promise<ApplyResult> => source.applyCheck(gatewayId, data),
  });
}

export interface ApplyCheckInput {
  gatewayId: string;
  /** The deploy to check, as it would be sent. */
  data: ApplyRequest;
}

export interface ApplyInput {
  gatewayId: string;
  data: ApplyRequest;
}

/**
 * Reads again what a write to a gateway changed: what it runs and holds, its previews, the references of
 * what it runs, and the status of each project there and on the gateway after it, which is read against it.
 */
function invalidateWrittenGateway(queryClient: QueryClient, gatewayId: string): void {
  [
    DeployQueryKeys.APPLIED_VERSION,
    DeployQueryKeys.APPLIED_CONFIGURATION,
    DeployQueryKeys.MASTER,
    DeployQueryKeys.MASTER_HISTORY,
    DeployQueryKeys.RESOURCE_HISTORY,
    DeployQueryKeys.PREVIEW,
    DeployQueryKeys.REVERT_PREVIEW,
    DeployQueryKeys.RUNNING_REFERENCES,
  ].forEach((key: string) => replaceReads(queryClient, {queryKey: [key, gatewayId]}));
  // A project's status is keyed by its gateway, the project and the gateway it is read against.
  replaceReads(queryClient, {
    queryKey: [DeployQueryKeys.PROJECT_STATUS],
    predicate: ({queryKey}) => queryKey[1] === gatewayId || queryKey[3] === gatewayId,
  });
}

/**
 * Reads the queries matching the filters again. A read still running started before the write and may
 * answer with what the gateway held then; a first read would be joined rather than restarted, so each is
 * cancelled first.
 */
function replaceReads(queryClient: QueryClient, filters: InvalidateQueryFilters): void {
  void queryClient
    .cancelQueries(filters)
    .then(() => queryClient.invalidateQueries(filters))
    .catch(() => undefined);
}

/** Writes a version to a gateway, so the gateway runs it. */
export function useApplyVersion(): UseMutationResult<ApplyResult, Error, ApplyInput> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<ApplyResult, Error, ApplyInput>({
    mutationFn: ({gatewayId, data}: ApplyInput): Promise<ApplyResult> => source.apply(gatewayId, data),
    onSuccess: (result: ApplyResult, {gatewayId}: ApplyInput) => {
      if (result.dryRun) return;
      invalidateWrittenGateway(queryClient, gatewayId);
    },
  });
}

export interface RevertInput {
  gatewayId: string;
  dryRun?: boolean;
  confirmRemovals?: boolean;
  /** Rolls back only this project's own resources. */
  project?: string;
  /** The plan a preview of the rollback reported, which the rollback must still follow. */
  plan?: string;
}

/** Puts a gateway back on the version it ran before the one it runs now. */
export function useRevert(): UseMutationResult<ApplyResult, Error, RevertInput> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<ApplyResult, Error, RevertInput>({
    mutationFn: ({
      gatewayId,
      dryRun = false,
      confirmRemovals = false,
      project,
      plan,
    }: RevertInput): Promise<ApplyResult> => source.revert(gatewayId, dryRun, confirmRemovals, project, plan),
    onSuccess: (result: ApplyResult, {gatewayId}: RevertInput) => {
      if (result.dryRun) return;
      invalidateWrittenGateway(queryClient, gatewayId);
    },
  });
}

/** Where the version a target runs refers to values. */
export function useRunningReferences(gatewayId: string, enabled = true): UseQueryResult<ValueReference[]> {
  const source = useDeploySource();
  return useQuery<ValueReference[]>({
    queryKey: [DeployQueryKeys.RUNNING_REFERENCES, gatewayId],
    queryFn: (): Promise<ValueReference[]> => source.runningReferences(gatewayId),
    enabled: enabled && Boolean(gatewayId),
    retry: false,
  });
}
