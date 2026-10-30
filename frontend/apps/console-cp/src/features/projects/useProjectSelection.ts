// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useSyncExternalStore} from 'react';
import useGetProjects, {type Project} from './useGetProjects';

/** Where the selection is remembered, so it survives a reload and every layout mount. */
export const PROJECT_SELECTION_STORAGE_KEY = 'thunderid.console-cp.selectedProject';

/**
 * The stored selection: a project ID, `''` once the user has cleared the selection, or `null`
 * while nothing has been chosen yet (the first project is then shown as selected).
 */
type StoredSelection = string | null;

let fallback: StoredSelection = null;
const listeners = new Set<() => void>();

function readSelection(): StoredSelection {
  try {
    return localStorage.getItem(PROJECT_SELECTION_STORAGE_KEY);
  } catch {
    return fallback;
  }
}

function writeSelection(value: string): void {
  fallback = value;
  try {
    localStorage.setItem(PROJECT_SELECTION_STORAGE_KEY, value);
  } catch {
    // Storage can be unavailable (a private window, blocked site data); the in-memory value still applies.
  }
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export interface ProjectSelection {
  /** The deployment's projects, oldest first. */
  projects: Project[];
  /** The selected project, or undefined when the selection is cleared or there are no projects. */
  selectedProject: Project | undefined;
  /** True once the user has cleared the selection to work across all projects. */
  isCleared: boolean;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
  selectProject: (id: string) => void;
  clearSelection: () => void;
}

/**
 * The project the console is working in. The header switcher, the sidebar and the projects page
 * all read it, and the selection is shared between them without a provider, since the dashboard
 * layout mounts once per route group.
 */
export default function useProjectSelection(): ProjectSelection {
  const {data, isLoading, error, refetch} = useGetProjects();
  const stored = useSyncExternalStore(subscribe, readSelection, () => null);

  const projects = data?.projects ?? [];
  const isCleared = stored === '';
  const selectedProject = isCleared ? undefined : (projects.find((project) => project.id === stored) ?? projects[0]);

  const selectProject = useCallback((id: string) => writeSelection(id), []);
  const clearSelection = useCallback(() => writeSelection(''), []);

  return {
    projects,
    selectedProject,
    isCleared,
    isLoading,
    error,
    refetch: () => void refetch(),
    selectProject,
    clearSelection,
  };
}
