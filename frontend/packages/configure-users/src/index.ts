// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useCreateUser} from './api/useCreateUser';
export {default as useDeleteUser} from './api/useDeleteUser';
export {default as useGetUser} from './api/useGetUser';
export {default as useGetUserUsages} from './api/useGetUserUsages';
export {default as useGetUsers} from './api/useGetUsers';
export {default as useGetUserType} from './api/useGetUserType';
export {default as useGetUserTypes} from './api/useGetUserTypes';
export {default as useUpdateUser} from './api/useUpdateUser';
export * from './api/useUpdateUser';

// Components
export {default as ArrayFieldInput} from './components/ArrayFieldInput';
export * from './components/ArrayFieldInput';
export {default as CredentialFieldInput} from './components/CredentialFieldInput';
export * from './components/CredentialFieldInput';
export {default as UserDeleteDialog} from './components/UserDeleteDialog';
export * from './components/UserDeleteDialog';
export {default as UsersList} from './components/UsersList';
export {default as QuickCopySection} from './components/edit-user/QuickCopySection';

// Constants
export {default as UserConstants} from './constants/user-constants';
export {default as UserQueryKeys} from './constants/user-query-keys';

// Models
export * from './models/users';

// Pages
// UserAddPage picks by plane: a form where no flow can run, and the flow-driven journey where one
// can.
export {default as UserAddPage} from './pages/UserAddRoute';
export {default as UserAddFlowPage} from './pages/UserAddPage';
export {default as UserAddFormPage} from './pages/UserAddFormPage';
export {default as UserCreatePage} from './pages/UserCreatePage';
export {default as UserEditPage} from './pages/UserEditPage';
export {default as UsersListPage} from './pages/UsersListPage';

// Routes
export type {UserRoutePaths} from './hooks/useUserRoutes';
export {default as useFlowTextResolver} from './hooks/useFlowTextResolver';
export {defaultUserRoutePaths, default as useUserRoutes} from './hooks/useUserRoutes';

// Utils
export {default as renderSchemaField} from './utils/renderSchemaField';
export * from './utils/dropNonConformingAttributes';
export {default as getUserErrorMessage} from './utils/getUserErrorMessage';

// The flow-backed deletion, exported so a console that runs flows can install it as its
// administration action. A console that does not install it deletes through the users API.
export {default as deleteUserViaFlow} from './utils/deleteUserViaFlow';
