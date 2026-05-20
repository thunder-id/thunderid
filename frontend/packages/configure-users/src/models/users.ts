/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import type {User, ApiPaginationLink} from '@thunderid/types';

/**
 * Error response structure
 */
export interface ApiError {
  code: string;
  message: string;
  description: string;
}

/**
 * User object with additional details for display purposes
 * Currently an alias for User, can be extended in the future with computed/display-specific fields
 */
export type UserWithDetails = User;

/**
 * User list response with pagination
 */
export interface UserListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  users: User[];
  links?: ApiPaginationLink[];
}

/**
 * Create user request payload
 */
export interface CreateUserRequest {
  ouId: string;
  type: string;
  groups?: string[];
  attributes?: Record<string, unknown>;
}

/**
 * Update user request payload
 */
export interface UpdateUserRequest {
  ouId?: string;
  type?: string;
  groups?: string[];
  attributes?: Record<string, unknown>;
}

/**
 * Create user by path request payload
 */
export interface CreateUserByPathRequest {
  ouId?: string; // Optional - can be inferred from path
  type: string;
  groups?: string[];
  attributes?: Record<string, unknown>;
}

/**
 * Authentication response
 */
export interface AuthenticateUserResponse {
  id: string;
  type: string;
  ouId: string;
}

/**
 * User group object
 */
export interface UserGroup {
  id: string;
  name: string;
}

/**
 * User group list response with pagination
 */
export interface UserGroupListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  groups: UserGroup[];
  links?: ApiPaginationLink[];
}

/**
 * Base property definition
 */
export interface BasePropertyDefinition {
  type: string;
  required?: boolean;
  displayName?: string;
}

/**
 * String property definition
 */
export interface StringPropertyDefinition extends BasePropertyDefinition {
  type: 'string';
  credential?: boolean;
  unique?: boolean;
  enum?: string[];
  regex?: string;
}

/**
 * Number property definition
 */
export interface NumberPropertyDefinition extends BasePropertyDefinition {
  type: 'number';
  credential?: boolean;
  unique?: boolean;
}

/**
 * Boolean property definition
 */
export interface BooleanPropertyDefinition extends BasePropertyDefinition {
  type: 'boolean';
}

/**
 * Object property definition
 */
export interface ObjectPropertyDefinition extends BasePropertyDefinition {
  type: 'object';
  properties: Record<string, PropertyDefinition>;
}

/**
 * Array property definition
 */
export interface ArrayPropertyDefinition extends BasePropertyDefinition {
  type: 'array';
  items: StringPropertyDefinition | NumberPropertyDefinition | BooleanPropertyDefinition | ObjectPropertyDefinition;
}

/**
 * Union type for all property definitions
 */
export type PropertyDefinition =
  | StringPropertyDefinition
  | NumberPropertyDefinition
  | BooleanPropertyDefinition
  | ObjectPropertyDefinition
  | ArrayPropertyDefinition;

/**
 * User type definition
 */
export type UserTypeDefinition = Record<string, PropertyDefinition>;

/**
 * User type object
 */
export interface ApiUserType {
  id: string;
  name: string;
  schema: UserTypeDefinition;
}

/**
 * User type list query parameters
 */
export interface SchemaListParams {
  limit?: number;
  offset?: number;
}

/**
 * User type list response
 */
export interface UserTypeListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  types: SchemaInterface[];
}

export interface SchemaInterface {
  id: string;
  name: string;
  ouId: string;
}
