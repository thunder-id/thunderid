// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useAddRoleAssignments} from './api/useAddRoleAssignments';
export {default as useCreateRole} from './api/useCreateRole';
export {default as useDeleteRole} from './api/useDeleteRole';
export {default as useGetRole} from './api/useGetRole';
export {default as useGetRoleAssignments} from './api/useGetRoleAssignments';
export {default as useGetRoles} from './api/useGetRoles';
export {default as useRemoveRoleAssignment} from './api/useRemoveRoleAssignment';
export {default as useUpdateRole} from './api/useUpdateRole';

// Components
export {default as RoleDeleteDialog} from './components/RoleDeleteDialog';
export {default as RolesList} from './components/RolesList';

// Constants
export {default as RoleQueryKeys} from './constants/role-query-keys';

// Contexts
export {default as RoleCreateContext} from './contexts/RoleCreate/RoleCreateContext';
export {default as RoleCreateProvider} from './contexts/RoleCreate/RoleCreateProvider';
export {default as useRoleCreate} from './contexts/RoleCreate/useRoleCreate';

// Models
export * from './models/requests';
export * from './models/role';
export * from './models/role-create-flow';

// Pages
export {default as CreateRolePage} from './pages/CreateRolePage';
export {default as RoleEditPage} from './pages/RoleEditPage';
export {default as RolesListPage} from './pages/RolesListPage';

// Routes
export type {RoleRoutePaths} from './hooks/useRoleRoutes';
export {defaultRoleRoutePaths, default as useRoleRoutes} from './hooks/useRoleRoutes';
