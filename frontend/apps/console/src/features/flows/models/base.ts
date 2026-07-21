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

import type {CSSProperties} from 'react';
import type {ElementTypes} from './elements';

/**
 * Base interface for a component or a primitive.
 */
export interface StrictBase<T = unknown> {
  /**
   * ID of the component or the primitive.
   */
  id: string;
  /**
   * Category of the component or the primitive.
   */
  category: string;
  /**
   * Type of the component or the primitive.
   */
  type: string;
  /**
   * Version of the component or the primitive.
   */
  version: string;
  /**
   * Is the component or the primitive deprecated.
   */
  deprecated: boolean;
  /**
   * Is the component or the primitive  deletable.
   */
  deletable: boolean;
  /**
   * Display properties of the component or the primitive.
   */
  display: BaseDisplay;
  /**
   * Configuration of the component or the primitive.
   */
  config: BaseConfig & T;
  /**
   * Base variant of the component or the primitive
   */
  variant?: unknown;
  /**
   * Variants of the component or the primitive.
   */
  variants?: Base<T>[];
  /**
   * Data added to the component by the flow builder.
   */
  data?: unknown;
}

/**
 * Interface representing a base component or a primitive.
 */
export interface Base<T = unknown> extends StrictBase<T> {
  /**
   * Type of the resource needed for visual editor operations.
   * @remarks This is a display only meta field and not being published to the backend.
   */
  resourceType: string;
}

export interface BaseDisplay {
  /**
   * Header text for the resource properties panel.
   * Falls back to type if not provided.
   */
  header?: string;
  /**
   * Fallback & i18n key value of the label.
   */
  label: string;
  /**
   * Image URL of the component or the primitive.
   */
  image: string;
  /**
   * Set for full-color brand logos (e.g. Google) that must not be inverted in
   * dark mode.
   */
  preserveImageColor?: boolean;
  /**
   * The default variant of the component or the primitive.
   */
  defaultVariant?: string;
  /**
   * Description of the component or the primitive.
   */
  description?: string;
  /**
   * Should the component be shown on the resource panel.
   */
  showOnResourcePanel: boolean;
  /**
   * Optional custom labels for an execution node's outcome handles (success / failure /
   * incomplete). When omitted, generic outcome labels are used.
   */
  outcomes?: {
    success?: string;
    failure?: string;
    incomplete?: string;
  };
}

/**
 * Interface representing an option for a field.
 */
export interface FieldOption {
  /**
   * The key of the field option.
   */
  key: string;

  /**
   * The value of the field option.
   */
  value: string;

  /**
   * The label of the field option.
   */
  label: string;
}

/**
 * Interface representing a strict field.
 */
export interface StrictField {
  /**
   * The name of the field.
   */
  name: string;
  /**
   * The type of the field.
   */
  type: typeof ElementTypes;
  /**
   * Options of the field.
   */
  options?: FieldOption[];
}

export type FieldKey = string;

export type FieldValue = unknown;

export type Field = StrictField & Record<FieldKey, FieldValue>;

export interface PasswordConfirmationProperties {
  /**
   * Whether password confirmation is required.
   */
  requireConfirmation?: boolean;
  /**
   * Hint text for the confirmation field.
   */
  confirmHint?: string;
  /**
   * Label for the confirmation field.
   */
  confirmLabel?: string;
  /**
   * Placeholder for the confirmation field.
   */
  confirmPlaceholder?: string;
}

export type Properties = (Field | CSSProperties) & PasswordConfirmationProperties;

export interface BaseConfig {
  /**
   * Field properties.
   */
  field: Field;
  /**
   * Styles of the component or the primitive.
   */
  styles: CSSProperties;
  /**
   * Identifier of the component or the primitive.
   */
  identifier?: string;
  /**
   * Label of the component or the primitive.
   */
  label?: string;
  /**
   * Placeholder of the component or the primitive.
   */
  placeholder?: string;
  /**
   * Hint of the component or the primitive.
   */
  hint?: string;
  /**
   * Is the component or the primitive required.
   */
  required?: boolean;
  /**
   * Should the password field require confirmation.
   */
  requireConfirmation?: boolean;
}
