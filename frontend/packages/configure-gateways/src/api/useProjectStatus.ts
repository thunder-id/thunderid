// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useMemo} from 'react';
import useDeploySource from './source';
import {PREVIEW_STALE_TIME} from './useGatewayVersions';
import {usePipeline} from './usePipeline';
import DeployQueryKeys from '../constants/query-keys';
import useProjectScope from '../contexts/ProjectScope';
import type {PipelineStage} from '../models/pipeline';
import type {ProjectStatus} from '../models/projectStatus';

/** A project's status as the control plane answers it: when and by whom, and the rollback, sit in `project`. */
interface ProjectStatusAnswer extends Omit<ProjectStatus, 'autoSync'> {
  project: ProjectStatus['project'] & Pick<ProjectStatus, 'deployedAt' | 'deployedBy' | 'rollback' | 'lastAction'>;
  autoSync?: {
    enabled: boolean;
    inherited: boolean;
    lastRun?: {at: string; version?: {hash: string; number?: number} | string; error?: {message?: string} | string};
  };
}

/** Reads a project's status from the control plane's answer into the shape the console shows. */
export function readProjectStatus(answer: ProjectStatusAnswer): ProjectStatus {
  const {deployedAt, deployedBy, rollback, lastAction, ...project} = answer.project;
  const lastRun = answer.autoSync?.lastRun;
  return {
    ...answer,
    project,
    deployedAt: answer.deployedAt ?? deployedAt,
    deployedBy: answer.deployedBy ?? deployedBy,
    rollback: answer.rollback ?? rollback,
    lastAction: answer.lastAction ?? lastAction,
    autoSync: answer.autoSync && {
      ...answer.autoSync,
      lastRun: lastRun && {
        at: lastRun.at,
        version: typeof lastRun.version === 'object' ? lastRun.version.hash : lastRun.version,
        error: typeof lastRun.error === 'object' ? lastRun.error.message : lastRun.error,
      },
    },
  };
}

/** How the selected project stands on one target, against `from`, the target before it. */
export default function useProjectStatus(targetId: string, from?: string): UseQueryResult<ProjectStatus> {
  const source = useDeploySource();
  const project = useProjectScope();
  return useQuery<ProjectStatus>({
    queryKey: [DeployQueryKeys.PROJECT_STATUS, targetId, project, from],
    queryFn: async (): Promise<ProjectStatus> =>
      readProjectStatus((await source.projectStatus(targetId, project ?? '', from)) as ProjectStatusAnswer),
    enabled: Boolean(project) && Boolean(targetId),
    retry: false,
    staleTime: PREVIEW_STALE_TIME,
    refetchOnWindowFocus: false,
  });
}

/**
 * How the selected project stands on each gateway of its pipeline, each against the one before it,
 * keyed by target id.
 */
export function useProjectStatuses(stages: PipelineStage[]): {
  byGateway: Record<string, ProjectStatus | undefined>;
  /** The targets whose status could not be read. */
  failed: Record<string, boolean>;
  /** Reads a target's status again. */
  retry: (targetId: string) => void;
  isLoading: boolean;
} {
  const source = useDeploySource();
  const project = useProjectScope();
  const results = useQueries({
    queries: stages.map((stage: PipelineStage, index: number) => {
      const from = index > 0 ? stages[index - 1].gateway.id : undefined;
      return {
        queryKey: [DeployQueryKeys.PROJECT_STATUS, stage.gateway.id, project, from],
        queryFn: async (): Promise<ProjectStatus> =>
          readProjectStatus((await source.projectStatus(stage.gateway.id, project ?? '', from)) as ProjectStatusAnswer),
        enabled: Boolean(project),
        retry: false,
        staleTime: PREVIEW_STALE_TIME,
        refetchOnWindowFocus: false,
      };
    }),
  });
  // The results array is new on every render; when each was last read, or failed, is what changes.
  const readAt = results.map((result) => `${result.dataUpdatedAt}:${result.errorUpdatedAt}:${result.isError}`).join();
  const loading = results.some((result) => result.isLoading);
  return useMemo(
    () => ({
      byGateway: Object.fromEntries(
        stages.map((stage: PipelineStage, index: number) => [stage.gateway.id, results[index]?.data]),
      ),
      failed: Object.fromEntries(
        stages.map((stage: PipelineStage, index: number) => [stage.gateway.id, results[index]?.isError === true]),
      ),
      retry: (targetId: string) => {
        const at = stages.findIndex((stage: PipelineStage) => stage.gateway.id === targetId);
        results[at]?.refetch().catch(() => undefined);
      },
      isLoading: loading,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [stages, readAt, loading],
  );
}

/**
 * How many of the selected project's own changes the first gateway of its pipeline does not run
 * yet, by resource type, against the latest version, as the project's status there counts them. Empty
 * outside a project.
 */
export function useProjectChanges(): Record<string, number> {
  const pipeline = usePipeline();
  const status = useProjectStatus(pipeline.data?.stages[0]?.gateway.id ?? '');
  return useMemo(
    () =>
      Object.fromEntries(
        (status.data?.project.byType ?? []).map(({resourceType, count}: {resourceType: string; count: number}) => [
          resourceType,
          count,
        ]),
      ),
    [status.data],
  );
}

/**
 * How many changes each of the projects has that a gateway does not run yet, and how many of the
 * resources no project owns, each project read against the latest version. A project's count is its own
 * changes and what they need that the gateway lacks, as its gateway card counts it, since a deploy of the
 * project sends both.
 */
export function useProjectsPending(
  targetId: string,
  projects: string[],
): {byProject: Record<string, number>; shared: number; isLoading: boolean; isError: boolean} {
  const source = useDeploySource();
  const results = useQueries({
    queries: projects.map((project: string) => ({
      queryKey: [DeployQueryKeys.PROJECT_STATUS, targetId, project, undefined],
      queryFn: async (): Promise<ProjectStatus> =>
        readProjectStatus((await source.projectStatus(targetId, project)) as ProjectStatusAnswer),
      enabled: Boolean(targetId),
      retry: false,
      staleTime: PREVIEW_STALE_TIME,
      refetchOnWindowFocus: false,
    })),
  });
  // The results array is new on every render; when each was last read, or failed, is what changes.
  const readAt = results.map((result) => `${result.dataUpdatedAt}:${result.errorUpdatedAt}:${result.isError}`).join();
  const loading = results.some((result) => result.isLoading);
  return useMemo(
    () => ({
      byProject: Object.fromEntries(
        projects.map((project: string, index: number) => {
          const state = results[index]?.data?.project;
          return [project, (state?.pending ?? 0) + (state?.required ?? 0)];
        }),
      ),
      shared: Math.max(0, ...results.map((result) => result.data?.shared.pending ?? 0)),
      // While any project is still being read, or once one could not be, the counts above fall short.
      isLoading: loading,
      isError: results.some((result) => result.isError),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [projects, readAt, loading],
  );
}
