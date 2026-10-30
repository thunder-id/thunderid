// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import ProjectScopeContext, {type ProjectScopeContextType} from './ProjectScopeContext';

/**
 * React hook for reading the project the console is working in.
 *
 * Returns the value from the nearest `ProjectScopeProvider`, or an empty object when there is
 * none, in which case `projectId` is undefined and requests should not be scoped to a project.
 *
 * @public
 */
export default function useProjectScope(): ProjectScopeContextType {
  return useContext(ProjectScopeContext);
}
