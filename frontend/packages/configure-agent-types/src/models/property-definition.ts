// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Schema property definitions for the agent type schema editor.
 */

interface BasePropertyDefinition {
  required?: boolean;
  unique?: boolean;
  displayName?: string;
}

/**
 * String property definition.
 */
export interface StringPropertyDefinition extends BasePropertyDefinition {
  type: 'string' | 'email' | 'mobile';
  credential?: boolean;
  enum?: string[];
  regex?: string;
}

/**
 * Number property definition.
 */
export interface NumberPropertyDefinition extends BasePropertyDefinition {
  type: 'number';
  credential?: boolean;
}

/**
 * Boolean property definition.
 */
export interface BooleanPropertyDefinition extends BasePropertyDefinition {
  type: 'boolean';
}

/**
 * Object property definition with nested properties.
 */
export interface ObjectPropertyDefinition extends BasePropertyDefinition {
  type: 'object';
  properties: Record<string, PropertyDefinition>;
}

/**
 * Array item definition (can be primitive or object).
 */
export type ArrayItemDefinition =
  | {
      type: 'string' | 'email' | 'mobile' | 'number' | 'boolean';
      enum?: string[];
    }
  | {
      type: 'object';
      properties: Record<string, PropertyDefinition>;
    };

/**
 * Array property definition.
 */
export interface ArrayPropertyDefinition extends BasePropertyDefinition {
  type: 'array';
  items: ArrayItemDefinition;
}

/**
 * Discriminated union of all property definition types.
 */
export type PropertyDefinition =
  | StringPropertyDefinition
  | NumberPropertyDefinition
  | BooleanPropertyDefinition
  | ObjectPropertyDefinition
  | ArrayPropertyDefinition;

/**
 * Agent type schema definition (key-value pairs of property definitions).
 */
export type AgentTypeDefinition = Record<string, PropertyDefinition>;

/**
 * Property type union for form inputs.
 */
export type PropertyType = 'email' | 'mobile' | 'string' | 'number' | 'boolean' | 'array' | 'object';

/**
 * UI property type including `enum` as a separate option (maps to string with enum values).
 */
export type UIPropertyType = PropertyType | 'enum';

/**
 * Schema property input type for create/edit forms.
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
