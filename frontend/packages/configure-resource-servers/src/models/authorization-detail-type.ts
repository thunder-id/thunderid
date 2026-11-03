// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OutboundAuthentication} from '@thunderid/components';

/**
 * The endpoint of the resource owner's system that authorizes and enriches the details of a type. A
 * request fails when the endpoint cannot be reached.
 */
export interface ProcessorConfig {
  endpoint: string;
  /** How ThunderID authenticates to the processor. Stored credentials are returned masked. */
  authentication?: OutboundAuthentication;
}

/**
 * An RFC 9396 authorization details type.
 */
export interface AuthorizationDetailType {
  id: string;
  type: string;
  name: string;
  description?: string;
  version: number;
  schema: Record<string, unknown>;
  consentTemplate?: string;
  processor?: ProcessorConfig;
  reuse: boolean;
  /** Whether the type is defined in declarative configuration, so it cannot be edited or deleted. */
  isReadOnly?: boolean;
}

/**
 * The body that creates or updates an authorization details type.
 */
export interface AuthorizationDetailTypeRequest {
  type: string;
  name: string;
  description?: string;
  schema: Record<string, unknown>;
  consentTemplate?: string;
  processor?: ProcessorConfig;
  reuse: boolean;
}

/**
 * A page of authorization details types.
 */
export interface AuthorizationDetailTypeListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  types: AuthorizationDetailType[];
  links?: {rel: string; href: string}[];
}

/** Who supplies a field's value. */
type SuppliedBy = 'client' | 'processor';

/** How a field's value may change after the request. */
export type Modification = 'none' | 'decrease' | 'increase' | 'removeItems' | 'addItems';

/** A party that may change a field. */
export type ModifyingParty = 'processor' | 'user' | 'client';

/** How values are ordered when a field is decreased, increased, or limited; inferred from its type. */
export type ValueType = 'number' | 'date' | 'dateTime';

/**
 * The `policy` keyword of a schema property. The minimum and maximum of a number field are kept
 * here while editing but are written as the property's own `minimum` and `maximum` keywords.
 */
export interface FieldPolicy {
  suppliedBy?: SuppliedBy;
  modification?: Modification;
  modifiableBy?: ModifyingParty[];
  minimum?: number | string;
  maximum?: number | string;
}

/** The kinds of field the builder can describe. */
export type FieldKind = 'string' | 'number' | 'date' | 'dateTime' | 'boolean' | 'array' | 'object';

/** The kinds of item a list field can hold. */
export type ItemKind = 'string' | 'number';

/**
 * One property of a type's schema, as the field builder edits it.
 */
export interface SchemaField {
  /** A stable key for rendering; not part of the schema. */
  key: string;
  name: string;
  title: string;
  kind: FieldKind;
  required: boolean;
  /** The kind of each item of a list field. */
  itemKind: ItemKind;
  /** The values each item of a list field must be one of, as typed; empty when any value is allowed. */
  allowedValues?: string[];
  policy: FieldPolicy;
  /** The `maxLength` of a string field, as typed; empty or undefined when it has none. */
  maxLength?: string;
  /** The `minItems` of a list field, as typed; empty or undefined when it has none. */
  minItems?: string;
  /** The properties of an object field. */
  children: SchemaField[];
}
