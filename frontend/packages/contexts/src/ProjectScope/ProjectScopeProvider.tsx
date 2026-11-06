// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo, type JSX, type PropsWithChildren} from 'react';
import ProjectScopeContext, {type ProjectScopeContextType} from './ProjectScopeContext';

/**
 * Props for the ProjectScopeProvider component.
 *
 * @public
 */
export interface ProjectScopeProviderProps extends PropsWithChildren {
  /**
   * The selected project. Leave it unset when no project is selected.
   */
  project?: {
    /** ID of the project. */
    id: string;
  };
}

/**
 * React context provider that tells feature packages which project the console is working in,
 * so they can scope their requests to it (for example `GET /applications?projectId=<id>`).
 * Selecting the project belongs to the host application: packages only read the answer through
 * `useProjectScope`.
 *
 * @example
 * ```tsx
 * <ProjectScopeProvider project={selectedProject}>
 *   <Outlet />
 * </ProjectScopeProvider>
 * ```
 *
 * @public
 */
export default function ProjectScopeProvider({project = undefined, children}: ProjectScopeProviderProps): JSX.Element {
  const projectId = project?.id;
  // An empty ID means no project, so requests are not scoped to a blank project.
  const value: ProjectScopeContextType = useMemo(() => (projectId ? {projectId} : {}), [projectId]);

  return <ProjectScopeContext.Provider value={value}>{children}</ProjectScopeContext.Provider>;
}
