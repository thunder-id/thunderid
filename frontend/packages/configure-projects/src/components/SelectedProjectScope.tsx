// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation, QueryErrorNotice} from '@thunderid/components';
import {ProjectScopeProvider} from '@thunderid/contexts';
import {Button} from '@wso2/oxygen-ui';
import type {JSX, PropsWithChildren} from 'react';
import {useTranslation} from 'react-i18next';
import useProjectSelection from '../hooks/useProjectSelection';

/**
 * Scopes the pages beneath it to the selected project, so feature packages list and create their
 * resources in that project.
 *
 * The pages wait for the project list: until it loads, the selection is not known, and rendering
 * them unscoped would briefly list or create resources across every project. A cleared selection
 * is a deliberate choice to work across projects, so it does not wait. When the list cannot be
 * read, the header switcher is hidden, so the error offers that choice beside Refresh.
 */
export default function SelectedProjectScope({children}: PropsWithChildren): JSX.Element {
  const {t} = useTranslation();
  const {selectedProject, isCleared, isLoading, error, refetch, clearSelection} = useProjectSelection();

  if (!isCleared && isLoading) {
    return <PageLoadingAnimation />;
  }

  if (!isCleared && error) {
    return (
      <QueryErrorNotice
        error={error}
        t={t}
        variant="inline"
        onRetry={refetch}
        action={
          <Button variant="text" onClick={clearSelection}>
            {t('projects:switcher.clear', 'Clear project selection')}
          </Button>
        }
      />
    );
  }

  return <ProjectScopeProvider project={selectedProject}>{children}</ProjectScopeProvider>;
}
