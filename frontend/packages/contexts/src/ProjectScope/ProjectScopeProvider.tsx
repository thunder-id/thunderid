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
   * ID of the selected project. Leave it unset when no project is selected.
   */
  projectId?: string;
  /**
   * ID of the selected project's root organization unit.
   */
  rootOuId?: string;
}

/**
 * React context provider that tells feature packages which project the console is working in,
 * so they can scope their requests to it (for example `GET /applications?projectId=<id>`).
 * Selecting the project belongs to the host application: packages only read the answer through
 * `useProjectScope`.
 *
 * @example
 * ```tsx
 * <ProjectScopeProvider projectId={selectedProject?.id} rootOuId={selectedProject?.rootOuId}>
 *   <Outlet />
 * </ProjectScopeProvider>
 * ```
 *
 * @public
 */
export default function ProjectScopeProvider({
  projectId = undefined,
  rootOuId = undefined,
  children,
}: ProjectScopeProviderProps): JSX.Element {
  // An empty ID means no project, so requests are not scoped to a blank project.
  const value: ProjectScopeContextType = useMemo(
    () => (projectId ? {projectId, ...(rootOuId ? {rootOuId} : {})} : {}),
    [projectId, rootOuId],
  );

  return <ProjectScopeContext.Provider value={value}>{children}</ProjectScopeContext.Provider>;
}
