// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import type {JSX, PropsWithChildren} from 'react';
import useProjectSelection from './useProjectSelection';

/**
 * Scopes the pages beneath it to the selected project, so feature packages list and create their
 * resources in that project.
 */
export default function SelectedProjectScope({children}: PropsWithChildren): JSX.Element {
  const {selectedProject} = useProjectSelection();

  return (
    <ProjectScopeProvider projectId={selectedProject?.id} rootOuId={selectedProject?.rootOuId}>
      {children}
    </ProjectScopeProvider>
  );
}
