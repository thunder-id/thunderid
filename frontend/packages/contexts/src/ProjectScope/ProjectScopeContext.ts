// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';

/**
 * Project scope context interface that carries the project the console is working in.
 *
 * @public
 */
export interface ProjectScopeContextType {
  /**
   * ID of the selected project, or undefined when the console is not working in a project.
   */
  projectId?: string;
}

/**
 * React context holding the selected project for the component tree beneath it.
 *
 * Defaults to an empty object so that feature packages rendered without a `ProjectScopeProvider`
 * (the main console, unit tests) work across every project. Consume via `useProjectScope`.
 *
 * @public
 */
const ProjectScopeContext: Context<ProjectScopeContextType> = createContext<ProjectScopeContextType>({});

export default ProjectScopeContext;
