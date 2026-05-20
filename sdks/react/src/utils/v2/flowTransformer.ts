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

/**
 * @fileoverview Shared flow response transformer utilities for v2 embedded flows.
 *
 * This module provides reusable transformation functions for normalizing embedded flow
 * responses from ThunderID APIs. It handles both successful responses with component
 * extraction and error responses with proper error message extraction.
 *
 * Key features:
 * - Component extraction from flow meta structure
 * - Translation string resolution in components
 * - Error response detection and message extraction
 * - Configurable error handling (throw vs return errors)
 *
 * Usage:
 * ```typescript
 * import { normalizeFlowResponse } from '../../../utils/v2/flowTransformer';
 *
 * const { executionId, components } = normalizeFlowResponse(apiResponse, t, {
 *   defaultErrorKey: 'components.signIn.errors.generic'
 * });
 * ```
 *
 * This transformer is used by both SignIn and SignUp v2 components to ensure
 * consistent response handling across all embedded flows.
 */

import {EmbeddedFlowComponentV2 as EmbeddedFlowComponent, FlowMetadataResponse} from '@thunderid/browser';
import resolveTranslationsInArray from './resolveTranslationsInArray';
import {UseTranslation} from '../../hooks/useTranslation';

/**
 * Generic flow error response interface that covers common error structure
 */
export interface FlowErrorResponse {
  executionId: string;
  failureReason?: string;
  flowStatus: 'ERROR';
}

/**
 * Configuration options for flow transformation
 */
export interface FlowTransformOptions {
  /**
   * Default error message key for translation fallback
   * @default 'errors.flow.generic'
   */
  defaultErrorKey?: string;
  /**
   * Whether to resolve translation strings or keep them as i18n keys
   * @default true
   */
  resolveTranslations?: boolean;
  /**
   * Whether to throw errors or return them as normalized response
   * @default true
   */
  throwOnError?: boolean;
}

/**
 * Create a mapping from ref to identifier based on data.inputs array.
 * This handles cases where meta.components use 'ref' to reference inputs,
 * and data.inputs contain the actual 'identifier' field.
 *
 * @param response - The flow response object
 * @returns Map of ref to identifier
 */
const createInputRefMapping = (response: any): Map<string, string> => {
  const mapping: Map<string, string> = new Map<string, string>();

  if (response?.data?.inputs && Array.isArray(response.data.inputs)) {
    response.data.inputs.forEach((input: any) => {
      if (input.ref && input.identifier) {
        mapping.set(input.ref, input.identifier);
      }
    });
  }

  return mapping;
};

/**
 * Create a mapping from action ref to nextNode based on data.actions array.
 * This handles cases where meta.components reference actions by ref,
 * and data.actions contain the actual nextNode field for routing.
 *
 * @param response - The flow response object
 * @returns Map of action ref to nextNode
 */
const createActionRefMapping = (response: any): Map<string, string> => {
  const mapping: Map<string, string> = new Map<string, string>();

  if (response?.data?.actions && Array.isArray(response.data.actions)) {
    response.data.actions.forEach((action: any) => {
      if (action.ref && action.nextNode) {
        mapping.set(action.ref, action.nextNode);
      }
    });
  }

  return mapping;
};

/**
 * Apply input ref mapping to components recursively.
 * This ensures that component.ref values are mapped to the correct identifier
 * from data.inputs, enabling proper form submission.
 *
 * @param components - Array of components to transform
 * @param refMapping - Map of ref to identifier
 * @param actionMapping - Map of action ref to nextNode
 * @param inputsData - Array of input data for resolving SELECT options
 * @returns Transformed components with correct identifiers and action references
 */
const applyInputRefMapping = (
  components: EmbeddedFlowComponent[],
  refMapping: Map<string, string>,
  actionMapping: Map<string, string>,
  inputsData: any[] = [],
): EmbeddedFlowComponent[] =>
  components.map((component: EmbeddedFlowComponent) => {
    const transformedComponent: any = {...component} as EmbeddedFlowComponent & {
      actionRef?: string;
      options?: {label: string; value: string}[];
    };

    // If this component has a ref that maps to an identifier, update it
    if (transformedComponent.ref && refMapping.has(transformedComponent.ref)) {
      transformedComponent.ref = refMapping.get(transformedComponent.ref);
    }

    // For SELECT components, copy options from data.inputs
    // The component.id matches the input.ref in the data structure
    if (transformedComponent.type === 'SELECT' && component.id) {
      const inputData: any = inputsData.find((input: any) => input.ref === component.id);
      if (inputData?.options) {
        transformedComponent.options = inputData.options.map((opt: any) => {
          if (typeof opt === 'string') {
            return {label: opt, value: opt};
          }
          // Safely handle non-string values to prevent React key crashes
          const value: string = typeof opt.value === 'object' ? JSON.stringify(opt.value) : String(opt.value || '');
          const label: string = typeof opt.label === 'object' ? JSON.stringify(opt.label) : String(opt.label || value);

          return {label, value};
        });
      }
    }

    // If this is an action component, map its id to the nextNode
    // Store the nextNode reference as actionRef property for later use
    if (
      transformedComponent.type === 'ACTION' &&
      transformedComponent.id &&
      actionMapping.has(transformedComponent.id)
    ) {
      transformedComponent.actionRef = actionMapping.get(transformedComponent.id);
    }

    // Recursively apply to nested components
    if (transformedComponent.components && Array.isArray(transformedComponent.components)) {
      transformedComponent.components = applyInputRefMapping(
        transformedComponent.components,
        refMapping,
        actionMapping,
        inputsData,
      );
    }

    return transformedComponent;
  });

/**
 * Transform and resolve translations in components from flow response.
 * This function extracts components from the response meta structure and optionally resolves
 * any translation strings within them. It also handles mapping of input refs to identifiers
 * and action refs to nextNode values.
 *
 * @param response - The flow response object containing components in meta structure
 * @param t - Translation function from useTranslation hook
 * @param resolveTranslations - Whether to resolve translation strings or keep them as i18n keys (default: true)
 * @returns Array of flow components with resolved or unresolved translations
 */
export const transformComponents = (
  response: any,
  t: UseTranslation['t'],
  resolveTranslations = true,
  meta?: FlowMetadataResponse | null,
): EmbeddedFlowComponent[] => {
  if (!response?.data?.meta?.components) {
    return [];
  }

  let {components} = response.data.meta;

  // Create mapping from ref to identifier based on data.inputs
  const refMapping: Map<string, string> = createInputRefMapping(response);

  // Create mapping from action ref to nextNode based on data.actions
  const actionMapping: Map<string, string> = createActionRefMapping(response);

  // Get inputs data for SELECT option resolution
  const inputsData: any[] = response?.data?.inputs || [];

  // Apply ref and action mapping if there are any mappings
  if (refMapping.size > 0 || actionMapping.size > 0 || inputsData.length > 0) {
    components = applyInputRefMapping(components, refMapping, actionMapping, inputsData);
  }

  return resolveTranslations ? resolveTranslationsInArray(components, t, undefined, meta) : components;
};

/**
 * Extract error message from flow error response.
 * Supports any flow error response that follows the standard structure.
 * Prioritizes failureReason if present, otherwise falls back to translated generic message.
 *
 * @param error - The error response object
 * @param t - Translation function for fallback messages
 * @param defaultErrorKey - Default translation key for generic errors
 * @returns Extracted error message or fallback
 */
export const extractErrorMessage = (
  error: FlowErrorResponse | any,
  t: UseTranslation['t'],
  defaultErrorKey = 'errors.flow.generic',
): string => {
  // Check for failureReason in the error object
  if (error && typeof error === 'object' && error.failureReason) {
    return error.failureReason;
  }

  // Check if error is a standard Error object with a message
  if (error instanceof Error && error.message) {
    return error.message;
  }

  // Fallback to a generic error message
  return t(defaultErrorKey);
};

/**
 * Check if a response is an error response and extract the error message.
 * This function identifies error responses by checking for ERROR status and failure reasons.
 *
 * @param response - The flow response to check
 * @param t - Translation function for error messages
 * @param defaultErrorKey - Default translation key for generic errors
 * @returns Error message string if response is an error, null otherwise
 */
export const checkForErrorResponse = (
  response: any,
  t: UseTranslation['t'],
  defaultErrorKey = 'errors.flow.generic',
): string | null => {
  if (response?.flowStatus === 'ERROR') {
    return extractErrorMessage(response, t, defaultErrorKey);
  }

  return null;
};

/**
 * Generic flow response normalizer that handles both success and error responses.
 * This is the main transformer function that should be used by all flow components.
 *
 * @param response - The raw flow response from the API
 * @param t - Translation function from useTranslation hook
 * @param options - Configuration options for transformation behavior
 * @returns Normalized flow response with executionId and transformed components
 * @throws {any} The original response if it's an error and throwOnError is true
 */
export const normalizeFlowResponse = (
  response: any,
  t: UseTranslation['t'],
  options: FlowTransformOptions = {},
  meta?: FlowMetadataResponse | null,
): {
  additionalData: Record<string, any>;
  components: EmbeddedFlowComponent[];
  executionId: string;
} => {
  const {throwOnError = true, defaultErrorKey = 'errors.flow.generic', resolveTranslations = true} = options;

  // Check if this is an error response
  const errorMessage: string | null = checkForErrorResponse(response, t, defaultErrorKey);

  if (errorMessage && throwOnError) {
    // Throw the original response so it can be caught by error handling
    throw response;
  }

  const additionalData: Record<string, any> = (response?.data?.additionalData as Record<string, any>) ?? {};

  // The consent prompt is serialized as a JSON string (array) by the backend.
  // Parse it and wrap in the expected {purposes: [...]} structure.
  if (typeof additionalData['consentPrompt'] === 'string') {
    try {
      const parsed: any = JSON.parse(additionalData['consentPrompt']);
      additionalData['consentPrompt'] = {purposes: Array.isArray(parsed) ? parsed : []};
    } catch {
      // Leave unparseable value as-is
    }
  }

  return {
    additionalData,
    components: transformComponents(response, t, resolveTranslations, meta),
    executionId: response.executionId,
  };
};
