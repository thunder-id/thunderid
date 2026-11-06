// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useProjectSelection} from '@thunderid/configure-projects';
import {ProjectScopeProvider} from '@thunderid/contexts';
import {useEffect, type JSX, type ReactNode} from 'react';
import {useParams} from 'react-router';

/**
 * Scopes a project's page to the project its route names, and selects that project, so a link to the
 * page opens the console in the project.
 */
export default function RouteProjectScope({children}: {children: ReactNode}): JSX.Element {
  const {projectId = ''} = useParams<{projectId: string}>();
  const {selectProject} = useProjectSelection();
  useEffect(() => {
    if (projectId) selectProject(projectId);
  }, [projectId, selectProject]);
  return (
    <ProjectScopeProvider key={projectId} project={{id: projectId}}>
      {children}
    </ProjectScopeProvider>
  );
}
