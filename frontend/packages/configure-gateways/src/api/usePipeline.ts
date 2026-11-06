// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import useProjectScope from '../contexts/ProjectScope';
import type {AutoApplySetting, Pipeline} from '../models/pipeline';
import type {Project} from '../models/project';

/** The environments a version is deployed and promoted through, in order: the project's, in one. */
export function usePipeline(): UseQueryResult<Pipeline> {
  const source = useDeploySource();
  const project = useProjectScope();
  return useQuery<Pipeline>({
    queryKey: [DeployQueryKeys.PIPELINE, project],
    queryFn: (): Promise<Pipeline> => source.pipeline(project),
    staleTime: 60_000,
  });
}

/** The projects deploys can be scoped to. */
export function useProjects(): UseQueryResult<Project[]> {
  const source = useDeploySource();
  return useQuery<Project[]>({
    queryKey: [DeployQueryKeys.PROJECTS],
    queryFn: (): Promise<Project[]> => source.projects(),
    staleTime: 60_000,
    retry: false,
  });
}

/** Whether a change saved here deploys to the first environment on its own. */
export function useAutoApply(): UseQueryResult<AutoApplySetting> {
  const source = useDeploySource();
  return useQuery<AutoApplySetting>({
    queryKey: [DeployQueryKeys.AUTO_APPLY],
    queryFn: (): Promise<AutoApplySetting> => source.autoApply(),
    retry: false,
  });
}

/**
 * Turns deploying a project's saved changes to its first environment on or off, or, with null, has it
 * follow the organization's default again.
 */
export function useSetProjectAutoSync(): UseMutationResult<void, Error, {project: string; enabled: boolean | null}> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<void, Error, {project: string; enabled: boolean | null}>({
    mutationFn: ({project, enabled}): Promise<void> => source.setProjectAutoSync(project, enabled),
    onSuccess: async (): Promise<void> => {
      await queryClient.invalidateQueries({queryKey: [DeployQueryKeys.PROJECT_STATUS]});
    },
  });
}

/** Turns deploying a saved change to the first environment on or off. */
export function useSetAutoApply(): UseMutationResult<void, Error, boolean> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<void, Error, boolean>({
    mutationFn: (enabled: boolean): Promise<void> => source.setAutoApply(enabled),
    onSuccess: async (): Promise<void> => {
      await queryClient.invalidateQueries({queryKey: [DeployQueryKeys.AUTO_APPLY]});
    },
  });
}
