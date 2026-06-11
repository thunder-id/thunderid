/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// API hooks
export {default as useGetResourceServers} from './api/useGetResourceServers';
export {default as useGetResourceServer} from './api/useGetResourceServer';
export {default as useCreateResourceServer} from './api/useCreateResourceServer';
export {default as useUpdateResourceServer} from './api/useUpdateResourceServer';
export {default as useDeleteResourceServer} from './api/useDeleteResourceServer';
export {default as useGetResources} from './api/useGetResources';
export {default as useCreateResource} from './api/useCreateResource';
export {default as useUpdateResource} from './api/useUpdateResource';
export {default as useDeleteResource} from './api/useDeleteResource';
export {default as useGetServerActions} from './api/useGetServerActions';
export {default as useGetResourceActions} from './api/useGetResourceActions';
export {default as useCreateAction} from './api/useCreateAction';
export {default as useUpdateAction} from './api/useUpdateAction';
export {default as useDeleteAction} from './api/useDeleteAction';

// Components
export {default as ResourceServersList} from './components/ResourceServersList';
export {default as ResourceServerDeleteDialog} from './components/ResourceServerDeleteDialog';
export type {ResourceServerDeleteDialogProps} from './components/ResourceServerDeleteDialog';

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
} from './models/resource-server';

// Pages
export {default as ResourceServersListPage} from './pages/ResourceServersListPage';
export {default as ResourceServerEditPage} from './pages/ResourceServerEditPage';
export {default as CreateResourceServerPage} from './pages/CreateResourceServerPage';
