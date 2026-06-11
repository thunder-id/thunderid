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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations
 * under the License.
 */

import {EmbeddedSignInFlowStatusV2, EmbeddedSignInFlowTypeV2, navigate as browserNavigate} from '@thunderid/browser';
import {FC, useEffect, useRef} from 'react';
import useThunderID from '../../../contexts/ThunderID/useThunderID';

export interface TokenCallbackProps {
  /**
   * Callback function called when an error occurs
   */
  onError?: (error: Error) => void;

  /**
   * Function to navigate to a different path.
   * If not provided, falls back to the browser navigate utility (SPA navigation via History API for same-origin paths).
   * Provide this prop to enable framework-specific navigation (e.g., from React Router).
   */
  onNavigate?: (path: string) => void;

  /**
   * Callback function called when authentication is successful
   */
  onSuccess?: (authData: Record<string, any>) => void;

  /**
   * Custom path for the sign-in page. Defaults to '/signin'
   */
  signInPath?: string;

  /**
   * Custom path for the sign-up page. Defaults to '/signup'
   */
  signUpPath?: string;
}

export const TokenCallback: FC<TokenCallbackProps> = ({
  onNavigate,
  onError,
  onSuccess,
  signInPath = '/signin',
  signUpPath = '/signup',
}: TokenCallbackProps) => {
  const processingRef: any = useRef(false);
  const {isInitialized, isLoading, signIn, signUp, getStorageManager} = useThunderID();

  const navigate = (path: string): void => {
    if (onNavigate) {
      onNavigate(path);
    } else {
      browserNavigate(path);
    }
  };

  const clearTokenFromUrl = (): void => {
    if (!window?.location?.href) {
      return;
    }

    const url: URL = new URL(window.location.href);
    url.searchParams.delete('token');
    url.searchParams.delete('type');
    window.history.replaceState({}, '', url.toString());
  };

  const initiateOAuthRedirect = (redirectURL: string, isRegistrationFlow?: boolean): void => {
    const redirectUrlObj: URL = new URL(redirectURL);
    const state: string = redirectUrlObj.searchParams.get('state') || crypto.randomUUID();

    sessionStorage.setItem(
      `thunderid_oauth_${state}`,
      JSON.stringify({
        path: isRegistrationFlow ? signUpPath : signInPath,
        timestamp: Date.now(),
      }),
    );

    browserNavigate(redirectUrlObj.toString());
  };

  const buildSignInPath = (
    executionId?: string | null,
    applicationId?: string | null,
    isRegistrationFlow?: boolean,
  ): string => {
    const params: URLSearchParams = new URLSearchParams();
    if (executionId) {
      params.set('executionId', executionId);
    }
    if (applicationId) {
      params.set('applicationId', applicationId);
    }

    const basePath = isRegistrationFlow ? signUpPath : signInPath;
    return params.toString() ? `${basePath}?${params.toString()}` : basePath;
  };

  const redirectWithError = (error: Error, isRegistrationFlow?: boolean): void => {
    sessionStorage.removeItem('thunderid_execution_id');

    onError?.(error);

    const params: URLSearchParams = new URLSearchParams();
    params.set('error', 'token_verification_failed');
    params.set('error_description', error.message);
    const basePath = isRegistrationFlow ? signUpPath : signInPath;
    navigate(`${basePath}?${params.toString()}`);
  };

  useEffect(() => {
    if (!isInitialized || isLoading) {
      return;
    }

    const processTokenCallback = async (): Promise<void> => {
      if (processingRef.current) {
        return;
      }
      processingRef.current = true;

      // Read URL params before clearing them so they're accessible in both try and catch.
      const searchParams: URLSearchParams = new URLSearchParams(window.location.search);
      const executionId: string | null = searchParams.get('id') || searchParams.get('executionId');
      const token: string | null = searchParams.get('token');
      const applicationId: string | null = searchParams.get('applicationId');
      const isRegistrationFlow: boolean = searchParams.get('type') === 'REGISTRATION';

      clearTokenFromUrl();

      try {
        const storageManager: any = await getStorageManager();

        if (!executionId || !token) {
          const error: Error = new Error('Missing executionId or token in callback URL');
          redirectWithError(error, isRegistrationFlow);
          return;
        }

        let response: any;

        if (isRegistrationFlow) {
          response = await signUp({executionId, inputs: {token}});
        } else {
          response = await signIn({executionId, inputs: {token}});
        }

        if (response.type === EmbeddedSignInFlowTypeV2.Redirection) {
          const redirectURL: string | undefined = (response.data as any)?.redirectURL || (response as any)?.redirectURL;
          const nextExecutionId: string = response.executionId || executionId;
          sessionStorage.setItem('thunderid_execution_id', nextExecutionId);

          if (redirectURL) {
            initiateOAuthRedirect(redirectURL, isRegistrationFlow);
            return;
          }
        }

        if (response.flowStatus === EmbeddedSignInFlowStatusV2.Complete) {
          const redirectUrl: string | undefined = (response as any)?.redirectUrl || (response as any)?.redirect_uri;

          sessionStorage.removeItem('thunderid_execution_id');
          await storageManager.removeHybridDataParameter('authId');

          onSuccess?.({
            redirectUrl,
            ...((response.data as Record<string, any>) || {}),
          });

          if (redirectUrl) {
            window.location.href = redirectUrl;
            return;
          }

          navigate(isRegistrationFlow ? signUpPath : signInPath);
          return;
        }

        if (response.flowStatus === EmbeddedSignInFlowStatusV2.Error) {
          const failureReason: string | undefined = (response as any)?.failureReason;
          const error: Error = new Error(failureReason || 'Token validation failed. Please try again.');
          await storageManager.removeHybridDataParameter('authId');
          redirectWithError(error, isRegistrationFlow);
          return;
        }

        const nextExecutionId: string = response.executionId || executionId;
        sessionStorage.setItem('thunderid_execution_id', nextExecutionId);

        if (response.challengeToken) {
          await storageManager.setTemporaryDataParameter('challengeToken', response.challengeToken);
        }

        navigate(buildSignInPath(nextExecutionId, applicationId, isRegistrationFlow));
      } catch (err) {
        const error: Error = err instanceof Error ? err : new Error('Token callback processing failed');
        // eslint-disable-next-line no-console
        console.error('Token callback error:', err);
        const storageManager: any = await getStorageManager();
        if (storageManager) {
          await storageManager.removeHybridDataParameter('authId');
        }
        redirectWithError(error, isRegistrationFlow);
      }
    };

    processTokenCallback();
  }, [
    getStorageManager,
    isInitialized,
    isLoading,
    onError,
    onNavigate,
    onSuccess,
    signIn,
    signUp,
    signInPath,
    signUpPath,
  ]);

  return null;
};

export default TokenCallback;
