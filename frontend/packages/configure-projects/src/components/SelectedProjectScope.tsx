// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation, QueryErrorNotice} from '@thunderid/components';
import {ProjectScopeProvider} from '@thunderid/contexts';
import {Button} from '@wso2/oxygen-ui';
import type {JSX, PropsWithChildren} from 'react';
import {useTranslation} from 'react-i18next';
import {Navigate, useNavigate} from 'react-router';
import useProjectRoutes from '../hooks/useProjectRoutes';
import useProjectSelection from '../hooks/useProjectSelection';

/**
 * Scopes the pages beneath it to the project the route names, so feature packages list and create
 * their resources in that project. A page the route puts in no project works across every project.
 *
 * The pages wait for the project list: until it loads, the project behind the handle is not known,
 * and rendering them unscoped would briefly list or create resources across every project. A handle
 * no project has leads to the projects page.
 */
export default function SelectedProjectScope({children}: PropsWithChildren): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useProjectRoutes();
  const {selectedHandle, selectedProject, isLoading, error, refetch} = useProjectSelection();

  if (!selectedHandle) {
    return <ProjectScopeProvider project={undefined}>{children}</ProjectScopeProvider>;
  }

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={t}
        variant="inline"
        onRetry={refetch}
        action={
          <Button variant="text" onClick={() => void navigate(routes.projects.list())}>
            {t('projects:switcher.clear', 'Clear project selection')}
          </Button>
        }
      />
    );
  }

  if (!selectedProject) {
    return <Navigate to={routes.projects.list()} replace />;
  }

  return <ProjectScopeProvider project={selectedProject}>{children}</ProjectScopeProvider>;
}
