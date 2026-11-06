// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ComplexSelect, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ProjectQuickSelector from './ProjectQuickSelector';
import useProjectRoutes from '../hooks/useProjectRoutes';
import useProjectSelection from '../hooks/useProjectSelection';

/**
 * The header's project selector. Choosing another project keeps the current page; the clear button
 * in the selector's corner leaves the project and opens the projects page, after which a button
 * picks a project again. It is hidden while there are no projects to show, including when the list
 * cannot be read.
 *
 * The app places it in its own `Header.Switchers`: the header's context belongs to the app's copy of
 * Oxygen UI, which a package can resolve separately.
 */
export default function ProjectSwitcher(): ReactNode {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useProjectRoutes();
  const {projects, selectedProject, selectProject, clearSelection} = useProjectSelection();

  if (projects.length === 0) {
    return null;
  }

  const handleClear = (): void => {
    clearSelection();
    void navigate(routes.projects.list());
  };

  const handleQuickSelect = (id: string): void => {
    selectProject(id);
    void navigate(routes.applications.list());
  };

  const clearLabel = t('projects:switcher.clear', 'Clear project selection');

  return selectedProject ? (
    <Box sx={{position: 'relative'}}>
      <ComplexSelect
        value={selectedProject.id}
        size="small"
        sx={{minWidth: 180}}
        label={t('projects:switcher.label', 'Project')}
        onChange={(event) => selectProject(String(event.target.value))}
        renderValue={() => <ComplexSelect.MenuItem.Text primary={selectedProject.displayName} />}
      >
        {projects.map((project) => (
          <ComplexSelect.MenuItem key={project.id} value={project.id}>
            <ComplexSelect.MenuItem.Text primary={project.displayName} secondary={project.handle} />
          </ComplexSelect.MenuItem>
        ))}
      </ComplexSelect>
      {/* Sits over the select's top-right corner, beside its label, as a sibling so it is a
              control of its own rather than part of the select. */}
      <Tooltip title={clearLabel}>
        <IconButton
          size="small"
          aria-label={clearLabel}
          onClick={handleClear}
          sx={{
            position: 'absolute',
            top: (theme) => theme.spacing(0.25),
            right: (theme) => theme.spacing(0.5),
            p: 0.25,
            color: 'text.secondary',
          }}
        >
          <X size={14} />
        </IconButton>
      </Tooltip>
    </Box>
  ) : (
    <ProjectQuickSelector projects={projects} onSelect={handleQuickSelect} />
  );
}
