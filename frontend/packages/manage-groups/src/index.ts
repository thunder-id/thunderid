// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Hooks
export {
  default as useManageGroupRoutes,
  defaultManageGroupRoutePaths,
  type ManageGroupRoutePaths,
} from './hooks/useManageGroupRoutes';

// Pages
export {default as GroupsListPage, type GroupsListPageProps} from './pages/GroupsListPage';
export {default as GroupCreateDialog} from './pages/GroupCreateDialog';
export {default as GroupDetailsPage, type GroupDetailsPageProps} from './pages/GroupDetailsPage';
