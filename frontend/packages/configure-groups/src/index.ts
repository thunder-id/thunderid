// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useAddGroupMembers} from './api/useAddGroupMembers';
export {default as useCreateGroup} from './api/useCreateGroup';
export {default as useDeleteGroup} from './api/useDeleteGroup';
export {default as useGetGroup} from './api/useGetGroup';
export {default as useGetGroupMembers} from './api/useGetGroupMembers';
export {default as useGetGroups} from './api/useGetGroups';
export {default as useRemoveGroupMember} from './api/useRemoveGroupMember';
export {default as useUpdateGroup} from './api/useUpdateGroup';

// Components
export {default as GroupDeleteDialog} from './components/GroupDeleteDialog';
export {default as GroupsList} from './components/GroupsList';

// Constants
export {default as GroupQueryKeys} from './constants/group-query-keys';

// Contexts
export {default as GroupCreateContext} from './contexts/GroupCreate/GroupCreateContext';
export {default as GroupCreateProvider} from './contexts/GroupCreate/GroupCreateProvider';
export {default as useGroupCreate} from './contexts/GroupCreate/useGroupCreate';

// Models
export * from './models/group';
export * from './models/group-create-flow';
export * from './models/requests';

// Pages
export {default as CreateGroupPage} from './pages/CreateGroupPage';
export {default as GroupEditPage} from './pages/GroupEditPage';
export {default as GroupsListPage} from './pages/GroupsListPage';

// Routes
export type {GroupRoutePaths} from './hooks/useGroupRoutes';
export {defaultGroupRoutePaths, default as useGroupRoutes} from './hooks/useGroupRoutes';
