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

import {watch, type Ref} from 'vue';

export interface UseOAuthCallbackOptions {
  /** Current executionId from component state */
  currentExecutionId: Ref<string | null>;

  /** SessionStorage key for executionId (defaults to 'thunderid_execution_id') */
  executionIdStorageKey?: string;

  /** Whether the component is initialized and ready to process OAuth callback */
  isInitialized: Ref<boolean>;

  /** Whether a submission is currently in progress */
  isSubmitting?: Ref<boolean>;

  /** Callback when OAuth flow completes successfully */
  onComplete?: () => void;

  /** Callback when OAuth flow encounters an error */
  onError?: (error: any) => void;

  /** Callback to handle flow response after submission */
  onFlowChange?: (response: any) => void;

  /** Callback to set loading state at the start of OAuth processing */
  onProcessingStart?: () => void;

  /** Function to submit OAuth code to the server */
  onSubmit: (payload: OAuthCallbackPayload) => Promise<any>;

  /** Mutable flag to track whether OAuth has already been processed */
  processedFlag?: {value: boolean};

  /** Additional handler for setting state (e.g., setExecutionId) */
  setExecutionId?: (executionId: string) => void;

  /**
   * Mutable flag for token validation tracking.
   * Used in AcceptInvite to coordinate between OAuth callback and token validation.
   */
  tokenValidationAttemptedFlag?: {value: boolean};
}

export interface OAuthCallbackPayload {
  /** The execution ID of the active flow step */
  executionId: string;

  /** OAuth callback inputs extracted from the redirect URL */
  inputs: {
    /** The authorization code returned by the OAuth provider */
    code: string;

    /** Optional nonce for OIDC replay protection */
    nonce?: string;
  };
}

/**
 * Removes OAuth-related query parameters from the current URL without triggering a navigation.
 * This prevents re-processing the callback on subsequent renders or page interactions.
 */
function cleanupUrlParams(): void {
  if (typeof window === 'undefined') return;

  const url: URL = new URL(window.location.href);
  url.searchParams.delete('code');
  url.searchParams.delete('nonce');
  url.searchParams.delete('state');
  url.searchParams.delete('error');
  url.searchParams.delete('error_description');

  window.history.replaceState({}, '', url.toString());
}

/**
 * Processes OAuth callbacks by detecting auth code in URL, resolving executionId, and submitting to server.
 * Used by SignIn, SignUp, and AcceptInvite components.
 *
 * Vue composable equivalent of React's useOAuthCallback hook.
 */
export function useOAuthCallback({
  currentExecutionId,
  executionIdStorageKey = 'thunderid_execution_id',
  isInitialized,
  isSubmitting,
  onComplete,
  onError,
  onFlowChange,
  onProcessingStart,
  onSubmit,
  processedFlag,
  setExecutionId,
  tokenValidationAttemptedFlag,
}: UseOAuthCallbackOptions): void {
  /** Fallback mutable flag used when no external processedFlag is provided */
  const internalFlag: {value: boolean} = {value: false};

  /** Ensures OAuth code is submitted only once, even across reactive re-evaluations */
  const oauthCodeProcessedFlag: {value: boolean} = processedFlag ?? internalFlag;

  /** Tracks whether token validation has been attempted; used to coordinate with AcceptInvite */
  const tokenValidationFlag: {value: boolean} | undefined = tokenValidationAttemptedFlag;

  // Re-run whenever initialization state, executionId, or submission state changes.
  // `immediate: true` ensures the callback runs on mount to catch OAuth redirects on first load.
  watch(
    () => [isInitialized.value, currentExecutionId.value, isSubmitting?.value] as const,
    ([initialized, , submitting]: readonly [boolean, string | null, boolean | undefined]) => {
      // Wait until the component is ready and any in-flight submission has settled.
      if (!initialized || submitting) {
        return;
      }

      // Extract all OAuth-related parameters from the redirect URL.
      const urlParams: URLSearchParams = new URLSearchParams(window.location.search);
      const code: string | null = urlParams.get('code');
      const nonce: string | null = urlParams.get('nonce');
      const state: string | null = urlParams.get('state');
      const executionIdFromUrl: string | null = urlParams.get('executionId');
      const error: string | null = urlParams.get('error');
      const errorDescription: string | null = urlParams.get('error_description');

      // Handle OAuth provider errors (e.g., user denied consent) before processing the code.
      if (error) {
        oauthCodeProcessedFlag.value = true;
        if (tokenValidationFlag) {
          tokenValidationFlag.value = true;
        }
        onError?.(new Error(errorDescription || error || 'OAuth authentication failed'));
        cleanupUrlParams();
        return;
      }

      // Skip if there is no authorization code or if it has already been submitted.
      if (!code || oauthCodeProcessedFlag.value) {
        return;
      }

      // In AcceptInvite flows, token validation runs concurrently. If it has already
      // started, the OAuth callback should not interfere.
      if (tokenValidationFlag?.value) {
        return;
      }

      // Resolve executionId using the most specific available source:
      // component state > sessionStorage > URL param > OAuth state param.
      const storedExecutionId: string | null = sessionStorage.getItem(executionIdStorageKey);
      const executionIdToUse: string | null =
        currentExecutionId.value || storedExecutionId || executionIdFromUrl || state || null;

      // Cannot proceed without an executionId — the flow context is missing.
      if (!executionIdToUse) {
        oauthCodeProcessedFlag.value = true;
        onError?.(new Error('Invalid flow. Missing executionId.'));
        cleanupUrlParams();
        return;
      }

      // Mark as processed synchronously before the async submission to prevent
      // duplicate submissions if the watcher fires again during the await.
      oauthCodeProcessedFlag.value = true;

      if (tokenValidationFlag) {
        tokenValidationFlag.value = true;
      }

      // Signal the component to enter a loading state before the async work begins.
      onProcessingStart?.();

      // Sync the resolved executionId back into component state if it was sourced
      // from sessionStorage or the URL rather than reactive state.
      if (!currentExecutionId.value && setExecutionId) {
        setExecutionId(executionIdToUse);
      }

      // Submit the OAuth code in an IIFE to allow async/await inside a synchronous watcher callback.
      (async (): Promise<void> => {
        try {
          const payload: OAuthCallbackPayload = {
            executionId: executionIdToUse,
            inputs: {
              code,
              ...(nonce && {nonce}),
            },
          };

          const response: any = await onSubmit(payload);

          // Notify the component so it can update its flow state (e.g., move to the next step).
          onFlowChange?.(response);

          if (response?.flowStatus === 'COMPLETE' || response?.status === 'COMPLETE') {
            onComplete?.();
          }

          if (response?.flowStatus === 'ERROR' || response?.status === 'ERROR') {
            onError?.(response);
          }

          cleanupUrlParams();
        } catch (err) {
          onError?.(err);
          cleanupUrlParams();
        }
      })();
    },
    {immediate: true},
  );
}
