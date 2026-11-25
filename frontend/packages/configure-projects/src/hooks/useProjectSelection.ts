// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback} from 'react';
import {useNavigate, useParams} from 'react-router';
import useProjectRoutes from './useProjectRoutes';
import useGetProjects from '../api/useGetProjects';
import type {Project} from '../models/project';

/** The route parameter that names the project a page is in, by its handle. */
export const PROJECT_HANDLE_PARAM = 'projectHandle';

export interface ProjectSelection {
  /** The deployment's projects, oldest first. */
  projects: Project[];
  /** The handle the current route names, or undefined when the page is not in a project. */
  selectedHandle: string | undefined;
  /** The project the current route names, or undefined when it names none or no such project exists. */
  selectedProject: Project | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
  /** Opens a project: its home, or the given path within it. */
  selectProject: (handle: string, path?: string) => void;
}

/**
 * The project the console is working in, as the route names it. A page in a project is under the
 * project's path, so the project survives a reload and a shared link opens the console in it.
 */
export default function useProjectSelection(): ProjectSelection {
  const {data, isLoading, error, refetch} = useGetProjects();
  const navigate = useNavigate();
  const routes = useProjectRoutes();
  const selectedHandle = useParams()[PROJECT_HANDLE_PARAM];

  const projects = data?.projects ?? [];
  const selectedProject = selectedHandle ? projects.find((project) => project.handle === selectedHandle) : undefined;

  const selectProject = useCallback(
    (handle: string, path?: string) => void navigate(routes.projects.home(handle, path)),
    [navigate, routes],
  );

  return {
    projects,
    selectedHandle,
    selectedProject,
    isLoading,
    error,
    refetch: () => void refetch(),
    selectProject,
  };
}
