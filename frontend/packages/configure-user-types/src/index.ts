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

// APIs
export {default as useCreateUserType} from './api/useCreateUserType';
export {default as useDeleteUserType} from './api/useDeleteUserType';
export {default as useGetUserType} from './api/useGetUserType';
export {default as useGetUserTypes} from './api/useGetUserTypes';
export {default as useUpdateUserType} from './api/useUpdateUserType';
export type {UpdateUserTypeVariables} from './api/useUpdateUserType';

// Components
export {default as UserTypesList} from './components/UserTypesList';
export {default as ConfigureGeneral} from './components/create-user-type/ConfigureGeneral';
export type {ConfigureGeneralProps} from './components/create-user-type/ConfigureGeneral';
export {default as ConfigureName} from './components/create-user-type/ConfigureName';
export type {ConfigureNameProps} from './components/create-user-type/ConfigureName';
export {default as ConfigureProperties} from './components/create-user-type/ConfigureProperties';
export type {ConfigurePropertiesProps} from './components/create-user-type/ConfigureProperties';
export {default as UserTypeDeleteDialog} from './components/edit-user-type/UserTypeDeleteDialog';
export type {UserTypeDeleteDialogProps} from './components/edit-user-type/UserTypeDeleteDialog';
export {default as EditGeneralSettings} from './components/edit-user-type/general-settings/EditGeneralSettings';
export type {EditGeneralSettingsProps} from './components/edit-user-type/general-settings/EditGeneralSettings';
export {default as QuickCopySection} from './components/edit-user-type/general-settings/QuickCopySection';
export {default as EditSchemaSettings} from './components/edit-user-type/schema-settings/EditSchemaSettings';
export type {EditSchemaSettingsProps} from './components/edit-user-type/schema-settings/EditSchemaSettings';
export {default as AttributeLibraryPanel} from './components/shared/AttributeLibraryPanel';
export type {AttributeLibraryPanelProps} from './components/shared/AttributeLibraryPanel';
export {default as SchemaPropertyEditor} from './components/shared/SchemaPropertyEditor';
export type {SchemaPropertyEditorProps} from './components/shared/SchemaPropertyEditor';

// Constants
export {default as UserTypeQueryKeys} from './constants/userTypeQueryKeys';

// Contexts
export {default as UserTypeCreateContext} from './contexts/UserTypeCreate/UserTypeCreateContext';
export type {UserTypeCreateContextType} from './contexts/UserTypeCreate/UserTypeCreateContext';
export {default as UserTypeCreateProvider} from './contexts/UserTypeCreate/UserTypeCreateProvider';
export {default as useUserTypeCreate} from './contexts/UserTypeCreate/useUserTypeCreate';

// Models
export {UserTypeCreateFlowStep} from './models/user-type-create-flow';
export type {
  ApiError,
  ApiUserType,
  ArrayItemDefinition,
  ArrayPropertyDefinition,
  BooleanPropertyDefinition,
  CreateUserTypeRequest,
  LibraryAttribute,
  NumberPropertyDefinition,
  ObjectPropertyDefinition,
  PropertyDefinition,
  PropertyType,
  SchemaPropertyInput,
  StringPropertyDefinition,
  SystemAttributes,
  UIPropertyType,
  UpdateUserTypeRequest,
  UserTypeDefinition,
  UserTypeListItem,
  UserTypeListParams,
  UserTypeListResponse,
} from './types/user-types';

// Pages
export {default as CreateUserTypePage} from './pages/CreateUserTypePage';
export {default as UserTypesListPage} from './pages/UserTypesListPage';
export {default as ViewUserTypePage} from './pages/ViewUserTypePage';
