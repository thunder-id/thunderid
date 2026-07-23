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

import type {EmbeddedFlowComponent} from '@thunderid/react';

/**
 * Represents a normalized embedded flow component with all optional properties
 * that may appear in embedded flow components (fields, display elements, buttons, layout containers).
 *
 * @remarks
 * This type extends {@link EmbeddedFlowComponent} and adds optional fields for
 * display, input, layout, and action components used in flow UIs.
 */
export type FlowComponent = EmbeddedFlowComponent & {
  /** Placeholder text for input fields */
  placeholder?: string;
  /** Whether the field is required */
  required?: boolean;
  /** Options for select fields */
  options?: unknown[];
  /** Hint or helper text */
  hint?: string;
  /** Visual variant (e.g., button style, text style) */
  variant?: string;
  /** Event type for actions (e.g., submit, trigger) */
  eventType?: string;
  /** Image source URL */
  src?: string;
  /** Image alt text */
  alt?: string;
  /** Image width (px) */
  width?: string;
  /** Image height (px) */
  height?: string;
  /** Icon name */
  name?: string;
  /** Icon size (px) */
  size?: number;
  /** Icon color */
  color?: string;
  /** Stack layout direction */
  direction?: 'row' | 'column';
  /** Stack gap (spacing) */
  gap?: number;
  /** Stack alignment */
  align?: string;
  /** Stack justification */
  justify?: string;
  /** Start icon for trigger buttons */
  startIcon?: string;
  /** Image for trigger buttons */
  image?: string;
  /** Data source key for dynamic components (e.g., COPYABLE_TEXT). References a key in additionalData. */
  source?: string;
};

/**
 * Props for field adapters rendered inside form blocks.
 *
 * @remarks
 * Used by input adapters (text, password, select, etc.) to receive form state and handlers.
 */
export interface FlowFieldProps {
  /** The flow component definition */
  component: FlowComponent;
  /** Current form values */
  values: Record<string, string>;
  /** Touched fields */
  touched?: Record<string, boolean>;
  /** Field error messages */
  fieldErrors?: Record<string, string>;
  /** Whether the form is loading/submitting */
  isLoading: boolean;
  /** Template resolver for dynamic text */
  resolve: (template: string | undefined) => string | undefined;
  /** Input change handler */
  onInputChange: (field: string, value: string) => void;
}

/**
 * Props for the top-level FlowComponentRenderer factory.
 *
 * @remarks
 * Consumers wrap their submit / trigger handlers into the normalized `onSubmit`
 * callback so each adapter doesn't need to know the underlying API shape.
 */
export interface FlowComponentRendererProps {
  /** The flow component definition */
  component: EmbeddedFlowComponent;
  /** Index of the component in the list */
  index: number;
  /** Current form values */
  values: Record<string, string>;
  /** Touched fields */
  touched?: Record<string, boolean>;
  /** Field error messages */
  fieldErrors?: Record<string, string>;
  /** Whether the form is loading/submitting */
  isLoading: boolean;
  /** Template resolver for dynamic text */
  resolve: (template: string | undefined) => string | undefined;
  /** Input change handler */
  onInputChange: (field: string, value: string) => void;
  /**
   * Called whenever an ACTION fires (submit or trigger).
   * @param action - The action component that fired.
   * @param inputs - Current form values at the time of submission.
   */
  onSubmit: (action: EmbeddedFlowComponent, inputs: Record<string, string>) => void;
  /**
   * Optional client-side validation hook called before form submission.
   * Return `false` to prevent the submit from proceeding.
   */
  onValidate?: (components: EmbeddedFlowComponent[]) => boolean;
  /** Optional max size (px) to cap image dimensions, e.g. when rendered inside a Stack */
  maxImageSize?: number;
  /** Additional step data from the flow response */
  additionalData?: Record<string, unknown>;
}
