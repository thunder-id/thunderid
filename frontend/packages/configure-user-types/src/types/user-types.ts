// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ApiPaginationLink} from '@thunderid/types';

/**
 * TypeScript types and interfaces for User Types feature
 * Based on the OpenAPI specification for UserType endpoints
 */

/**
 * Base property definition types for user type
 */
interface BasePropertyDefinition {
  required?: boolean;
  unique?: boolean;
  displayName?: string;
}

/**
 * String property definition
 */
export interface StringPropertyDefinition extends BasePropertyDefinition {
  type: 'string';
  credential?: boolean;
  enum?: string[];
  regex?: string;
}

/**
 * Number property definition
 */
export interface NumberPropertyDefinition extends BasePropertyDefinition {
  type: 'number';
  credential?: boolean;
}

/**
 * Boolean property definition
 */
export interface BooleanPropertyDefinition extends BasePropertyDefinition {
  type: 'boolean';
}

/**
 * Object property definition with nested properties
 */
export interface ObjectPropertyDefinition extends BasePropertyDefinition {
  type: 'object';
  properties: Record<string, PropertyDefinition>;
}

/**
 * Array item definition (can be primitive or object)
 */
export type ArrayItemDefinition =
  | {
      type: 'string' | 'number' | 'boolean';
      enum?: string[];
    }
  | {
      type: 'object';
      properties: Record<string, PropertyDefinition>;
    };

/**
 * Array property definition
 */
export interface ArrayPropertyDefinition extends BasePropertyDefinition {
  type: 'array';
  items: ArrayItemDefinition;
}

/**
 * Discriminated union of all property definition types
 */
export type PropertyDefinition =
  | StringPropertyDefinition
  | NumberPropertyDefinition
  | BooleanPropertyDefinition
  | ObjectPropertyDefinition
  | ArrayPropertyDefinition;

/**
 * User type schema definition (key-value pairs of property definitions)
 */
export type UserTypeDefinition = Record<string, PropertyDefinition>;

/**
 * SCIM type/primary metadata for a property mapped to a multi-valued SCIM target.
 */
export interface ScimMultiValuedMeta {
  type: string;
  primary: boolean;
}

/**
 * SCIM attribute mapping of a user type (schema property name to SCIM target).
 */
export interface ScimMapping {
  attributeMap?: Record<string, string>;
  multiValuedMeta?: Record<string, ScimMultiValuedMeta>;
}

/**
 * System-level metadata for a user type.
 */
export interface SystemAttributes {
  display?: string;
  isScimCoreType?: boolean;
  scimMapping?: ScimMapping;
}

/**
 * Complete User Type object as returned by API
 */
export interface ApiUserType {
  id: string;
  handle: string;
  displayName: string;
  ouId: string;
  ouHandle?: string;
  allowSelfRegistration: boolean;
  systemAttributes?: SystemAttributes;
  schema: UserTypeDefinition;
  isReadOnly?: boolean;
}

/**
 * User Type list item (minimal representation)
 */
export interface UserTypeListItem {
  id: string;
  handle: string;
  displayName: string;
  ouId: string;
  ouHandle?: string;
  allowSelfRegistration: boolean;
  systemAttributes?: SystemAttributes;
  isReadOnly?: boolean;
}

/**
 * Response for GET /user-types (list with pagination)
 */
export interface UserTypeListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  types: UserTypeListItem[];
  links?: ApiPaginationLink[];
}

/**
 * Request body for POST /user-types (create)
 */
export interface CreateUserTypeRequest {
  handle: string;
  displayName: string;
  ouId: string;
  allowSelfRegistration?: boolean;
  systemAttributes?: SystemAttributes;
  schema: UserTypeDefinition;
}

/**
 * Request body for PUT /user-types/{id} (update)
 */
export interface UpdateUserTypeRequest {
  displayName: string;
  ouId: string;
  allowSelfRegistration?: boolean;
  systemAttributes?: SystemAttributes;
  schema: UserTypeDefinition;
}

/**
 * Query parameters for listing user types
 */
export interface UserTypeListParams {
  limit?: number;
  offset?: number;
}

/**
 * API Error structure
 */
export interface ApiError {
  code: string;
  message: string;
  description: string;
}

/**
 * Property type union for form inputs
 */
export type PropertyType = 'string' | 'number' | 'boolean' | 'array' | 'object';

/**
 * UI property type including 'enum' as a separate option (maps to string with enum values)
 */
export type UIPropertyType = PropertyType | 'enum';

/**
 * Schema property input type for create/edit forms
 */
export interface SchemaPropertyInput {
  id: string;
  name: string;
  displayName: string;
  type: UIPropertyType;
  required: boolean;
  unique: boolean;
  credential: boolean;
  enum: string[];
  regex: string;
  /** Preserved array item definition for round-trip fidelity. */
  items?: ArrayItemDefinition;
  /** Preserved nested object properties for round-trip fidelity. */
  properties?: Record<string, PropertyDefinition>;
}

/**
 * A predefined attribute from the front-end attribute library. It is a schema
 * property definition without the transient row `id`, which is assigned when
 * the attribute is added to a schema.
 */
export type LibraryAttribute = Omit<SchemaPropertyInput, 'id'>;
