// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useCreateProject} from './api/useCreateProject';
export {default as useDeleteProject} from './api/useDeleteProject';
export {default as useGetProjects} from './api/useGetProjects';

// Components
export {default as ProjectCard} from './components/ProjectCard';
export type {ProjectCardProps} from './components/ProjectCard';
export {default as ProjectDeleteDialog} from './components/ProjectDeleteDialog';
export type {ProjectDeleteDialogProps} from './components/ProjectDeleteDialog';
export {default as ProjectSwitcher} from './components/ProjectSwitcher';
export {default as SelectedProjectScope} from './components/SelectedProjectScope';

// Constants
export {default as ProjectQueryKeys} from './constants/project-query-keys';

// Hooks
export type {ProjectSelection} from './hooks/useProjectSelection';
export {PROJECT_HANDLE_PARAM, default as useProjectSelection} from './hooks/useProjectSelection';

// Models
export * from './models/project';
export * from './models/requests';

// Pages
export {default as ProjectCreatePage} from './pages/ProjectCreatePage';
export {default as ProjectsListPage} from './pages/ProjectsListPage';

// Routes
export type {ProjectRoutePaths} from './hooks/useProjectRoutes';
export {defaultProjectRoutePaths, default as useProjectRoutes} from './hooks/useProjectRoutes';
