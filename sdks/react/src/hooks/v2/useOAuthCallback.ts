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

import {useEffect, useRef, type RefObject} from 'react';

export interface UseOAuthCallbackOptions {
  /**
   * Current executionId from component state
   */
  currentExecutionId: string | null;

  /**
   * SessionStorage key for executionId (defaults to 'thunderid_execution_id')
   */
  executionIdStorageKey?: string;

  /**
   * Whether the component is initialized and ready to process OAuth callback
   */
  isInitialized: boolean;

  /**
   * Whether a submission is currently in progress
   */
  isSubmitting?: boolean;

  /**
   * Callback when OAuth flow completes successfully
   */
  onComplete?: () => void;

  /**
   * Callback when OAuth flow encounters an error
   */
  onError?: (error: any) => void;

  /**
   * Function to handle flow response after submission
   */
  onFlowChange?: (response: any) => void;

  /**
   * Callback to set loading state at the start of OAuth processing
   */
  onProcessingStart?: () => void;

  /**
   * Function to submit OAuth code to the server
   */
  onSubmit: (payload: OAuthCallbackPayload) => Promise<any>;

  /**
   * Optional external ref to track processed state. If provided, the component
   * manages the ref (allowing resets on flow clear/retry). Otherwise hook manages internally.
   */
  processedRef?: RefObject<boolean>;

  /**
   * Additional handler for setting state (e.g., setExecutionId)
   */
  setExecutionId?: (executionId: string) => void;

  /**
   * Ref to mark that token validation was attempted (prevents duplicate validation)
   * Used in AcceptInvite to coordinate between OAuth callback and token validation
   */
  tokenValidationAttemptedRef?: RefObject<boolean>;
}

export interface OAuthCallbackPayload {
  executionId: string;
  inputs: {
    code: string;
    nonce?: string;
  };
}

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
 */
export function useOAuthCallback({
  currentExecutionId,
  executionIdStorageKey = 'thunderid_execution_id',
  isInitialized,
  isSubmitting = false,
  onComplete,
  onError,
  onFlowChange,
  onProcessingStart,
  onSubmit,
  processedRef,
  setExecutionId: setExecExecutionId,
  tokenValidationAttemptedRef,
}: UseOAuthCallbackOptions): void {
  const internalRef: any = useRef(false);
  const oauthCodeProcessedRef: any = processedRef ?? internalRef;

  useEffect(() => {
    if (!isInitialized || isSubmitting) {
      return;
    }

    const urlParams: URLSearchParams = new URLSearchParams(window.location.search);
    const code: string | null = urlParams.get('code');
    const nonce: string | null = urlParams.get('nonce');
    const state: string | null = urlParams.get('state');
    const executionIdFromUrl: string | null = urlParams.get('executionId');
    const error: string | null = urlParams.get('error');
    const errorDescription: string | null = urlParams.get('error_description');

    if (error) {
      oauthCodeProcessedRef.current = true;
      if (tokenValidationAttemptedRef) {
        tokenValidationAttemptedRef.current = true;
      }
      onError?.(new Error(errorDescription || error || 'OAuth authentication failed'));
      cleanupUrlParams();
      return;
    }

    if (!code || oauthCodeProcessedRef.current) {
      return;
    }

    if (tokenValidationAttemptedRef?.current) {
      return;
    }

    const storedExecutionId: string | null = sessionStorage.getItem(executionIdStorageKey);
    const executionIdToUse: string | null =
      currentExecutionId || storedExecutionId || executionIdFromUrl || state || null;

    if (!executionIdToUse) {
      oauthCodeProcessedRef.current = true;
      onError?.(new Error('Invalid flow. Missing executionId.'));
      cleanupUrlParams();
      return;
    }

    oauthCodeProcessedRef.current = true;

    if (tokenValidationAttemptedRef) {
      tokenValidationAttemptedRef.current = true;
    }

    onProcessingStart?.();

    if (!currentExecutionId && setExecExecutionId) {
      setExecExecutionId(executionIdToUse);
    }

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
  }, [
    isInitialized,
    currentExecutionId,
    isSubmitting,
    onSubmit,
    onComplete,
    onError,
    onFlowChange,
    setExecExecutionId,
    executionIdStorageKey,
  ]);
}
