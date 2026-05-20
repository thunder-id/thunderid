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

type TranslationFn = (key: string, params?: Record<string, string | number>) => string;

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

    if (transformedComponent.ref && refMapping.has(transformedComponent.ref)) {
      transformedComponent.ref = refMapping.get(transformedComponent.ref);
    }

    if (transformedComponent.type === 'SELECT' && component.id) {
      const inputData: any = inputsData.find((input: any) => input.ref === component.id);
      if (inputData?.options) {
        transformedComponent.options = inputData.options.map((opt: any) => {
          if (typeof opt === 'string') {
            return {label: opt, value: opt};
          }
          const value: string = typeof opt.value === 'object' ? JSON.stringify(opt.value) : String(opt.value || '');
          const label: string = typeof opt.label === 'object' ? JSON.stringify(opt.label) : String(opt.label || value);

          return {label, value};
        });
      }
    }

    if (
      transformedComponent.type === 'ACTION' &&
      transformedComponent.id &&
      actionMapping.has(transformedComponent.id)
    ) {
      transformedComponent.actionRef = actionMapping.get(transformedComponent.id);
    }

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
 */
export const transformComponents = (
  response: any,
  t: TranslationFn,
  resolveTranslations = true,
  meta?: FlowMetadataResponse | null,
): EmbeddedFlowComponent[] => {
  if (!response?.data?.meta?.components) {
    return [];
  }

  let {components} = response.data.meta;

  const refMapping: Map<string, string> = createInputRefMapping(response);
  const actionMapping: Map<string, string> = createActionRefMapping(response);
  const inputsData: any[] = response?.data?.inputs || [];

  if (refMapping.size > 0 || actionMapping.size > 0 || inputsData.length > 0) {
    components = applyInputRefMapping(components, refMapping, actionMapping, inputsData);
  }

  return resolveTranslations ? resolveTranslationsInArray(components, t, undefined, meta) : components;
};

/**
 * Extract error message from flow error response.
 * Supports any flow error response that follows the standard structure.
 * Prioritizes failureReason if present, otherwise falls back to translated generic message.
 */
export const extractErrorMessage = (
  error: FlowErrorResponse | any,
  t: TranslationFn,
  defaultErrorKey = 'errors.flow.generic',
): string => {
  if (error && typeof error === 'object' && error.failureReason) {
    return error.failureReason;
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return t(defaultErrorKey);
};

/**
 * Check if a response is an error response and extract the error message.
 */
export const checkForErrorResponse = (
  response: any,
  t: TranslationFn,
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
 */
export const normalizeFlowResponse = (
  response: any,
  t: TranslationFn,
  options: FlowTransformOptions = {},
  meta?: FlowMetadataResponse | null,
): {
  additionalData: Record<string, any>;
  components: EmbeddedFlowComponent[];
  executionId: string;
} => {
  const {throwOnError = true, defaultErrorKey = 'errors.flow.generic', resolveTranslations = true} = options;

  const errorMessage: string | null = checkForErrorResponse(response, t, defaultErrorKey);

  if (errorMessage && throwOnError) {
    throw response;
  }

  const additionalData: Record<string, any> = (response?.data?.additionalData as Record<string, any>) ?? {};

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
