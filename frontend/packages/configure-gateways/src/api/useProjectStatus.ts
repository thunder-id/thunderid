// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useMemo} from 'react';
import useDeploySource from './source';
import {useApplyCheck} from './useGatewayVersions';
import {usePipeline} from './usePipeline';
import {useVersions} from './useVersions';
import DeployQueryKeys from '../constants/query-keys';
import useProjectScope from '../contexts/ProjectScope';
import type {Change} from '../models/deploy';
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
 * yet, by resource type, as a deploy of the latest version would check them. Empty outside a project.
 */
export function useProjectChanges(): Record<string, number> {
  const project = useProjectScope();
  const pipeline = usePipeline();
  const versions = useVersions();
  const first = pipeline.data?.stages[0]?.gateway.id ?? '';
  const latest = versions.data?.[0]?.version;
  const check = useApplyCheck(first, latest, Boolean(project));
  return useMemo(() => {
    const counts: Record<string, number> = {};
    (check.data?.diff.changes ?? []).forEach((change: Change) => {
      if (change.section !== 'project' || change.change === 'unchanged') return;
      counts[change.resourceType] = (counts[change.resourceType] ?? 0) + 1;
    });
    return counts;
  }, [check.data]);
}
