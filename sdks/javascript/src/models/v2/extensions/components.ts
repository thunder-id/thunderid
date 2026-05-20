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

import type {EmbeddedFlowComponent as EmbeddedFlowComponentV2} from '../embedded-flow-v2';
import type {FlowMetadataResponse} from '../flow-meta-v2';

/**
 * Framework-agnostic context passed to every custom component renderer.
 * Contains form state and callbacks needed to render and submit flow components.
 */
export interface ComponentRenderContext {
  /**
   * Extra payload propagated by the flow engine for component rendering.
   */
  additionalData?: Record<string, any>;
  /**
   * Authentication flow type currently being rendered.
   */
  authType: 'signin' | 'signup';
  /**
   * Validation messages keyed by field name.
   */
  formErrors: Record<string, string>;
  /**
   * Current form values keyed by field name.
   */
  formValues: Record<string, string>;
  /**
   * Whether the current form state passes validation.
   */
  isFormValid: boolean;
  /**
   * Indicates whether a submit action is currently in progress.
   */
  isLoading: boolean;
  /**
   * Optional flow metadata associated with the current step.
   */
  meta?: FlowMetadataResponse | null;
  /**
   * Optional callback fired when an input loses focus.
   */
  onInputBlur?: (name: string) => void;
  /**
   * Callback to update the value of a named input field.
   */
  onInputChange: (name: string, value: string) => void;
  /**
   * Optional submit handler for progressing the flow.
   */
  onSubmit?: (component: EmbeddedFlowComponentV2, data?: Record<string, any>, skipValidation?: boolean) => void;
  /**
   * Tracks whether each field has been interacted with.
   */
  touchedFields: Record<string, boolean>;
}

/**
 * A function that renders a flow component of a given type.
 * `TElement` is `unknown` at the JS SDK level; each framework narrows it
 * (React: `ReactElement`, Vue: `VNode`, etc.).
 *
 * Returning `null` hides the component. If no renderer is registered for a
 * component type, the SDK falls back to its built-in rendering.
 */
export type ComponentRenderer<TElement = unknown> = (
  component: EmbeddedFlowComponentV2,
  context: ComponentRenderContext,
) => TElement | null;

/**
 * Extension configuration for flow component rendering.
 * Keyed by component type string (e.g. `"PASSWORD_INPUT"`, `"ACTION"`).
 */
export interface ComponentsExtensions {
  /**
   * Custom renderers keyed by flow component type.
   */
  renderers?: Record<string, ComponentRenderer<unknown>>;
}
