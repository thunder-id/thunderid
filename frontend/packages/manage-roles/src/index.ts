// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Hooks
export {
  default as useManageRoleRoutes,
  defaultManageRoleRoutePaths,
  type ManageRoleRoutePaths,
} from './hooks/useManageRoleRoutes';

// Pages
export {default as RolesListPage, type RolesListPageProps} from './pages/RolesListPage';
export {default as RoleCreateDialog} from './pages/RoleCreateDialog';
export {default as RoleDetailsPage, type RoleDetailsPageProps} from './pages/RoleDetailsPage';
