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

export interface SchemaAttribute {
  caseExact: boolean;
  description?: string;
  displayName?: string;
  displayOrder?: string;
  multiValued: boolean;
  mutability: string;
  name: string;
  regEx?: string;
  required?: boolean;
  returned: string;
  sharedProfileValueResolvingMethod?: string;
  subAttributes?: SchemaAttribute[];
  supportedByDefault?: string;
  type: string;
  uniqueness: string;
}

/**
 * Represents a SCIM2 schema definition
 */
export interface Schema {
  /** Schema attributes */
  attributes: SchemaAttribute[];
  /** Schema description */
  description: string;
  /** Schema identifier */
  id: string;
  /** Schema name */
  name: string;
}

export interface FlattenedSchema extends Schema {
  schemaId: string;
}

/**
 * Well-known SCIM2 schema IDs
 */
export enum WellKnownSchemaIds {
  /** Core Schema */
  Core = 'urn:ietf:params:scim:schemas:core:2.0',
  /** Custom User Schema */
  CustomUser = 'urn:scim:schemas:extension:custom:User',
  /** Enterprise User Schema */
  EnterpriseUser = 'urn:ietf:params:scim:schemas:extension:enterprise:2.0:User',
  /** System User Schema */
  SystemUser = 'urn:scim:wso2:schema',
  /** User Schema */
  User = 'urn:ietf:params:scim:schemas:core:2.0:User',
}
