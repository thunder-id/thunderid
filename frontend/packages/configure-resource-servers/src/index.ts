// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API hooks
export {default as useGetResourceServers} from './api/useGetResourceServers';
export {default as useGetResourceServer} from './api/useGetResourceServer';
export {default as useCreateResourceServer} from './api/useCreateResourceServer';
export {default as useUpdateResourceServer} from './api/useUpdateResourceServer';
export {default as useDeleteResourceServer} from './api/useDeleteResourceServer';
export {default as useGetDefaultResourceServer} from './api/useGetDefaultResourceServer';
export {default as useSetDefaultResourceServer} from './api/useSetDefaultResourceServer';
export {default as useGetResources} from './api/useGetResources';
export {default as useCreateResource} from './api/useCreateResource';
export {default as useUpdateResource} from './api/useUpdateResource';
export {default as useDeleteResource} from './api/useDeleteResource';
export {default as useGetServerActions} from './api/useGetServerActions';
export {default as useGetResourceActions} from './api/useGetResourceActions';
export {default as useCreateAction} from './api/useCreateAction';
export {default as useUpdateAction} from './api/useUpdateAction';
export {default as useDeleteAction} from './api/useDeleteAction';
export {default as useSubtreePermissions} from './api/useSubtreePermissions';

// Components
export {default as PermissionCatalog} from './components/permission-catalog/PermissionCatalog';
export type {PermissionCatalogProps} from './components/permission-catalog/PermissionCatalog';
export {default as SelectedScopesField} from './components/permission-catalog/SelectedScopesField';
export type {SelectedScopesFieldProps} from './components/permission-catalog/SelectedScopesField';
export {default as ResourceServersList} from './components/ResourceServersList';
export {default as ResourceServerDeleteDialog} from './components/ResourceServerDeleteDialog';
export type {ResourceServerDeleteDialogProps} from './components/ResourceServerDeleteDialog';
export {default as SetDefaultResourceServerDialog} from './components/SetDefaultResourceServerDialog';
export type {SetDefaultResourceServerDialogProps} from './components/SetDefaultResourceServerDialog';

// Constants
export {default as ResourceServerQueryKeys} from './constants/resource-server-query-keys';

// Models
export type {
  ResourceServer,
  ResourceServerListResponse,
  Resource,
  ResourceListResponse,
  Action,
  ActionListResponse,
  CreateResourceServerRequest,
  UpdateResourceServerRequest,
  CreateResourceRequest,
  UpdateResourceRequest,
  CreateActionRequest,
  UpdateActionRequest,
  ResourcePermissions,
  DefaultResourceServerValue,
  ServerConfigLayers,
  DefaultResourceServerConfigResponse,
} from './models/resource-server';

// Utils
export {
  isPermissionSelected,
  togglePermission,
  mergePermissions,
  removePermissions,
  getSubtreeSelectionState,
  arePermissionsEqual,
} from './utils/permissionSelection';
export type {SelectionState} from './utils/permissionSelection';

// Pages
export {default as ResourceServersListPage} from './pages/ResourceServersListPage';
export {default as ResourceServerEditPage} from './pages/ResourceServerEditPage';
export {default as CreateResourceServerPage} from './pages/CreateResourceServerPage';

// Routes
export type {ResourceServerRoutePaths} from './hooks/useResourceServerRoutes';
export {defaultResourceServerRoutePaths, default as useResourceServerRoutes} from './hooks/useResourceServerRoutes';
