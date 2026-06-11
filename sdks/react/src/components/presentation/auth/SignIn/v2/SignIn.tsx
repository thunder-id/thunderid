/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {
  ThunderIDRuntimeError,
  EmbeddedFlowComponentV2 as EmbeddedFlowComponent,
  EmbeddedFlowType,
  EmbeddedSignInFlowResponseV2,
  EmbeddedSignInFlowRequestV2,
  EmbeddedSignInFlowStatusV2,
  EmbeddedSignInFlowTypeV2,
  FlowMetadataResponse,
  Preferences,
  logger,
} from '@thunderid/browser';
import {FC, ReactElement, useState, useEffect, useRef, ReactNode} from 'react';
// eslint-disable-next-line import/no-named-as-default
import BaseSignIn, {BaseSignInProps} from './BaseSignIn';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';
import useTranslation from '../../../../../hooks/useTranslation';
import {useOAuthCallback} from '../../../../../hooks/v2/useOAuthCallback';
import {initiateOAuthRedirect} from '../../../../../utils/oauth';
import {extractErrorMessage, normalizeFlowResponse} from '../../../../../utils/v2/flowTransformer';
import {handlePasskeyAuthentication, handlePasskeyRegistration} from '../../../../../utils/v2/passkey';

/**
 * Render props function parameters
 */
export interface SignInRenderProps {
  /**
   * Additional data from the flow response containing contextual information
   * like consent prompt details and session timeouts.
   */
  additionalData?: Record<string, any>;

  /**
   * Current flow components
   */
  components: EmbeddedFlowComponent[];

  /**
   * Current error if any
   */
  error: Error | null;

  /**
   * Function to manually initialize the flow
   */
  initialize: () => Promise<void>;

  /**
   * Whether the flow has been initialized
   */
  isInitialized: boolean;

  /**
   * Loading state indicator
   */
  isLoading: boolean;

  /**
   * Flag indicating whether the flow step timeout has expired.
   * Consuming components can use this to disable submit buttons.
   */
  isTimeoutDisabled?: boolean;

  /**
   * Flow metadata returned by the platform (v2 only). `null` while loading or unavailable.
   */
  meta: FlowMetadataResponse | null;

  /**
   * Function to submit authentication data (primary)
   */
  onSubmit: (payload: EmbeddedSignInFlowRequestV2) => Promise<void>;
}

/**
 * Props for the SignIn component.
 * Matches the interface from the main SignIn component for consistency.
 */
export interface SignInProps {
  /**
   * Render props function for custom UI
   */
  children?: (props: SignInRenderProps) => ReactNode;

  /**
   * Custom CSS class name for the form container.
   */
  className?: string;

  /**
   * Callback function called when authentication fails.
   * @param error - The error that occurred during authentication.
   */
  onError?: (error: Error) => void;

  /**
   * Callback function called when authentication is successful.
   * @param authData - The authentication data returned upon successful completion.
   */
  onSuccess?: (authData: Record<string, any>) => void;

  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;

  /**
   * Size variant for the component.
   */
  size?: 'small' | 'medium' | 'large';

  /**
   * Theme variant for the component.
   */
  variant?: BaseSignInProps['variant'];
}

/**
 * State for tracking passkey registration
 */
interface PasskeyState {
  actionId: string | null;
  challenge: string | null;
  creationOptions: string | null;
  error: Error | null;
  executionId: string | null;
  isActive: boolean;
}

/**
 * A component-driven SignIn component that provides authentication flow with pre-built styling.
 * This component handles the flow API calls for authentication and delegates UI logic to BaseSignIn.
 * It automatically transforms simple input-based responses into component-driven UI format.
 *
 * @example
 * // Default UI
 * ```tsx
 * import { SignIn } from '@thunderid/react/component-driven';
 *
 * const App = () => {
 *   return (
 *     <SignIn
 *       onSuccess={(authData) => {
 *         console.log('Authentication successful:', authData);
 *       }}
 *       onError={(error) => {
 *         console.error('Authentication failed:', error);
 *       }}
 *       size="medium"
 *       variant="outlined"
 *     />
 *   );
 * };
 * ```
 *
 * @example
 * // Custom UI with render props
 * ```tsx
 * import { SignIn } from '@thunderid/react/component-driven';
 *
 * const App = () => {
 *   return (
 *     <SignIn
 *       onSuccess={(authData) => console.log('Success:', authData)}
 *       onError={(error) => console.error('Error:', error)}
 *     >
 *       {({signIn, isLoading, components, error, isInitialized}) => (
 *         <div className="custom-signin">
 *           <h1>Custom Sign In</h1>
 *           {!isInitialized ? (
 *             <p>Initializing...</p>
 *           ) : error ? (
 *             <div className="error">{error.message}</div>
 *           ) : (
 *             <form onSubmit={(e) => {
 *               e.preventDefault();
 *               signIn({inputs: {username: 'user', password: 'pass'}});
 *             }}>
 *               <button type="submit" disabled={isLoading}>
 *                 {isLoading ? 'Signing in...' : 'Sign In'}
 *               </button>
 *             </form>
 *           )}
 *         </div>
 *       )}
 *     </SignIn>
 *   );
 * };
 * ```
 */
const SignIn: FC<SignInProps> = ({
  className,
  preferences,
  size = 'medium',
  onSuccess,
  onError,
  variant,
  children,
}: SignInProps): ReactElement => {
  const {applicationId, afterSignInUrl, signIn, isInitialized, isLoading, meta, getStorageManager, scopes} =
    useThunderID();
  const {t} = useTranslation(preferences?.i18n);

  // State management for the flow
  const [components, setComponents] = useState<EmbeddedFlowComponent[]>([]);
  const [additionalData, setAdditionalData] = useState<Record<string, any>>({});
  const [currentExecutionId, setCurrentExecutionId] = useState<string | null>(null);
  const challengeTokenRef: any = useRef<string | null>(null);
  const [isStorageReady, setIsStorageReady] = useState(false);
  const [isFlowInitialized, setIsFlowInitialized] = useState(false);
  const [flowError, setFlowError] = useState<Error | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isTimeoutDisabled, setIsTimeoutDisabled] = useState<boolean>(false);
  const [passkeyState, setPasskeyState] = useState<PasskeyState>({
    actionId: null,
    challenge: null,
    creationOptions: null,
    error: null,
    executionId: null,
    isActive: false,
  });
  const initializationAttemptedRef: any = useRef(false);
  const oauthCodeProcessedRef: any = useRef(false);
  const passkeyProcessedRef: any = useRef(false);
  /**
   * Sets executionId between sessionStorage and state.
   * This ensures both are always in sync.
   */
  const setExecutionId = (executionId: string | null): void => {
    setCurrentExecutionId(executionId);
    if (executionId) {
      sessionStorage.setItem('thunderid_execution_id', executionId);
    } else {
      sessionStorage.removeItem('thunderid_execution_id');
    }
  };

  /**
   * Restore any challenge token persisted before an OAuth redirect.
   * Waits for SDK initialization before reading from storage.
   */
  useEffect(() => {
    if (!isInitialized) return;

    (async (): Promise<void> => {
      try {
        const storageManager: any = await getStorageManager();
        const tempData: any = await storageManager?.getTemporaryData();
        if (tempData?.challengeToken) {
          challengeTokenRef.current = tempData.challengeToken as string;
        }
      } finally {
        setIsStorageReady(true);
      }
    })();
  }, [isInitialized]);

  /**
   * Updates challengeTokenRef immediately (stale-closure safe) and persists via
   * the provider's StorageManager so the token survives OAuth redirects.
   */
  const setChallengeToken = async (challengeToken: string | null): Promise<void> => {
    challengeTokenRef.current = challengeToken;
    try {
      const storageManager: any = await getStorageManager();
      if (storageManager) {
        if (challengeToken) {
          await storageManager.setTemporaryDataParameter('challengeToken', challengeToken);
        } else {
          await storageManager.removeTemporaryDataParameter('challengeToken');
        }
      }
    } catch {
      logger.warn('Failed to persist challenge token in storage.');
    }
  };

  /**
   * Clear all flow-related storage and state.
   */
  const clearFlowState = async (): Promise<void> => {
    setExecutionId(null);
    await setChallengeToken(null);
    setIsFlowInitialized(false);
    try {
      const storageManager: any = await getStorageManager();
      await storageManager?.removeHybridDataParameter?.('authId');
    } catch {
      logger.warn('Failed to clear authId from hybrid storage.');
    }
    setIsTimeoutDisabled(false);
    // Reset refs to allow new flows to start properly
    oauthCodeProcessedRef.current = false;
  };

  /**
   * Parse URL parameters used in flows.
   */
  const getUrlParams = (): any => {
    const urlParams: any = new URL(window?.location?.href ?? '').searchParams;

    return {
      applicationId: urlParams.get('applicationId'),
      authId: urlParams.get('authId'),
      code: urlParams.get('code'),
      error: urlParams.get('error'),
      errorDescription: urlParams.get('error_description'),
      executionId: urlParams.get('executionId'),
      nonce: urlParams.get('nonce'),
      state: urlParams.get('state'),
    };
  };

  /**
   * Handle authId from URL and store it in sessionStorage.
   */
  const handleAuthId = async (authId: string | null): Promise<void> => {
    if (authId) {
      try {
        const storageManager: any = await getStorageManager();
        await storageManager?.setHybridDataParameter?.('authId', authId);
      } catch {
        logger.warn('Failed to store authId in hybrid storage.');
      }
    }
  };

  /**
   * Clean up OAuth-related URL parameters from the browser URL.
   */
  const cleanupOAuthUrlParams = (includeNonce = false): void => {
    if (!window?.location?.href) return;
    const url: any = new URL(window.location.href);
    url.searchParams.delete('error');
    url.searchParams.delete('error_description');
    url.searchParams.delete('code');
    url.searchParams.delete('state');
    if (includeNonce) {
      url.searchParams.delete('nonce');
    }
    window?.history?.replaceState({}, '', url.toString());
  };

  /**
   * Clean up flow-related URL parameters (executionId, authId) from the browser URL.
   * Used after executionId is set in state to prevent using invalidated executionId from URL.
   */
  const cleanupFlowUrlParams = (): void => {
    if (!window?.location?.href) return;
    const url: any = new URL(window.location.href);
    url.searchParams.delete('executionId');
    url.searchParams.delete('authId');
    url.searchParams.delete('applicationId');
    window?.history?.replaceState({}, '', url.toString());
  };

  /**
   * Set error state and call onError callback.
   * Ensures isFlowInitialized is true so errors can be displayed in the UI.
   */
  const setError = (error: Error): void => {
    setFlowError(error);
    setIsFlowInitialized(true);
    onError?.(error);
  };

  /**
   * Handle OAuth error from URL parameters.
   * Clears flow state, creates error, and cleans up URL.
   */
  const handleOAuthError = (error: string, errorDescription: string | null): void => {
    clearFlowState();
    const errorMessage: any = errorDescription || `OAuth error: ${error}`;
    const err: any = new ThunderIDRuntimeError(errorMessage, 'SIGN_IN_ERROR', 'react');
    setError(err);
    cleanupOAuthUrlParams(true);
  };

  /**
   * Handle REDIRECTION response by storing flow state and redirecting to OAuth provider.
   */
  const handleRedirection = async (response: EmbeddedSignInFlowResponseV2): Promise<boolean> => {
    if (response.type === EmbeddedSignInFlowTypeV2.Redirection) {
      const redirectURL: any = (response.data as any)?.redirectURL || (response as any)?.redirectURL;

      if (redirectURL && window?.location) {
        if (response.executionId) {
          setExecutionId(response.executionId);
        }
        await setChallengeToken(response.challengeToken ?? null);

        const urlParams: any = getUrlParams();
        handleAuthId(urlParams.authId);

        initiateOAuthRedirect(redirectURL);
        return true;
      }
    }
    return false;
  };

  /**
   * Initialize the authentication flow.
   * Priority: executionId > applicationId (from context) > applicationId (from URL)
   */
  const initializeFlow = async (): Promise<void> => {
    const urlParams: any = getUrlParams();

    // Reset OAuth code processed ref when starting a new flow
    oauthCodeProcessedRef.current = false;

    handleAuthId(urlParams.authId);

    const effectiveApplicationId: any = applicationId || urlParams.applicationId;

    if (!urlParams.executionId && !effectiveApplicationId) {
      const error: any = new ThunderIDRuntimeError(
        'Either executionId or applicationId is required for authentication',
        'SIGN_IN_ERROR',
        'react',
      );
      setError(error);
      throw error;
    }

    try {
      setFlowError(null);

      let response: EmbeddedSignInFlowResponseV2;

      if (urlParams.executionId) {
        response = (await signIn({
          executionId: urlParams.executionId,
          ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
        })) as EmbeddedSignInFlowResponseV2;
      } else {
        response = (await signIn({
          applicationId: effectiveApplicationId,
          flowType: EmbeddedFlowType.Authentication,
          ...(scopes && {scopes}),
        })) as EmbeddedSignInFlowResponseV2;
      }

      if (await handleRedirection(response)) {
        return;
      }

      const {
        executionId: normalizedExecutionId,
        components: normalizedComponents,
        additionalData: normalizedAdditionalData,
      } = normalizeFlowResponse(
        response,
        t,
        {
          resolveTranslations: false,
        },
        meta,
      );

      await setChallengeToken(response.challengeToken ?? null);

      if (normalizedExecutionId && normalizedComponents) {
        setExecutionId(normalizedExecutionId);
        setComponents(normalizedComponents);
        setAdditionalData(normalizedAdditionalData ?? {});
        setIsFlowInitialized(true);
        setIsTimeoutDisabled(false);
        // Clean up executionId from URL after setting it in state
        cleanupFlowUrlParams();
      }
    } catch (error) {
      const err: any = error;
      await clearFlowState();

      setError(err instanceof ThunderIDRuntimeError ? err : new Error(extractErrorMessage(err, t)));
      initializationAttemptedRef.current = false;
    }
  };

  /**
   * Initialize the flow and handle cleanup of stale flow state.
   */
  useEffect(() => {
    const urlParams: any = getUrlParams();

    // Check for OAuth error in URL
    if (urlParams.error) {
      handleOAuthError(urlParams.error, urlParams.errorDescription);
      return;
    }

    handleAuthId(urlParams.authId);

    // Skip OAuth code processing - let the dedicated OAuth useEffect handle it
    // No action needed here as the dedicated useEffect will handle it
  }, []);

  useEffect(() => {
    // Only initialize if we're not processing an OAuth callback or submission
    const currentUrlParams: any = getUrlParams();
    if (
      isInitialized &&
      !isLoading &&
      isStorageReady &&
      !isFlowInitialized &&
      !initializationAttemptedRef.current &&
      !currentExecutionId &&
      !currentUrlParams.code &&
      !currentUrlParams.state &&
      !isSubmitting &&
      !oauthCodeProcessedRef.current
    ) {
      initializationAttemptedRef.current = true;
      initializeFlow();
    }
  }, [isInitialized, isLoading, isStorageReady, isFlowInitialized, currentExecutionId]);

  /**
   * Handle step timeout if configured in additionalData.
   */
  useEffect(() => {
    const timeoutMs: number = Number(additionalData?.['stepTimeout']) || 0;
    if (timeoutMs <= 0 || !isFlowInitialized) {
      setIsTimeoutDisabled(false);
      return undefined;
    }

    const remaining: number = Math.max(0, Math.floor((timeoutMs - Date.now()) / 1000));

    const handleTimeout = (): void => {
      const errorMessage: string = t('errors.signin.timeout') || 'Time allowed to complete the step has expired.';
      setError(new Error(errorMessage));
      setIsTimeoutDisabled(true);
    };

    if (remaining <= 0) {
      handleTimeout();
      return undefined;
    }

    const timerId: any = setTimeout(() => {
      handleTimeout();
    }, remaining * 1000);

    return () => clearTimeout(timerId);
  }, [additionalData?.['stepTimeout'], isFlowInitialized, t]);

  /**
   * Handle form submission from BaseSignIn or render props.
   */
  const handleSubmit = async (payload: EmbeddedSignInFlowRequestV2): Promise<void> => {
    // Use executionId from payload if available, otherwise fall back to currentExecutionId
    const effectiveExecutionId: any = payload.executionId || currentExecutionId;

    if (!effectiveExecutionId) {
      throw new Error('No active flow ID');
    }

    const processedInputs: Record<string, any> = {...payload.inputs};

    // Auto-compile consent decisions if we are currently on a consent prompt step
    if (additionalData?.['consentPrompt']) {
      try {
        const consentPromptRawData: any = additionalData['consentPrompt'];
        const purposes: any[] =
          typeof consentPromptRawData === 'string'
            ? JSON.parse(consentPromptRawData)
            : consentPromptRawData.purposes || consentPromptRawData;

        // Find the action component to determine if it was a deny action
        let isDeny = false;
        if (payload.action) {
          // Flatten components to find the action
          const findAction = (comps: any[]): any => {
            if (!comps || comps.length === 0) return null;

            const found: any = comps.find((c: any) => c.id === payload.action);
            if (found) return found;

            return comps.reduce((acc: any, c: any) => {
              if (acc) return acc;
              if (c.components) return findAction(c.components);
              return null;
            }, null);
          };

          const submitAction: any = findAction(components);

          if (submitAction && submitAction.variant?.toLowerCase() !== 'primary') {
            isDeny = true;
          }
        }

        const decisions: any = {
          purposes: purposes.map((p: any) => ({
            approved: !isDeny,
            elements: [
              ...(p.essential || []).map((e: any) => ({
                approved: !isDeny,
                name: e.name,
              })),
              ...(p.optional || []).map((e: any) => {
                const key = `__consent_opt__${p.purposeId}__${e.name}`;
                return {
                  approved: isDeny ? false : processedInputs[key] !== 'false',
                  name: e.name,
                };
              }),
            ],
            purposeName: p.purposeName,
          })),
        };
        processedInputs['consent_decisions'] = JSON.stringify(decisions);

        // Cleanup temporary consent tracking fields from inputs
        Object.keys(processedInputs).forEach((key: string) => {
          if (key.startsWith('__consent_opt__')) {
            delete processedInputs[key];
          }
        });
      } catch (e) {
        // Failed to construct consent_decisions payload automatically
      }
    }

    try {
      setIsSubmitting(true);
      setFlowError(null);

      const response: EmbeddedSignInFlowResponseV2 = (await signIn({
        executionId: effectiveExecutionId,
        ...payload,
        inputs: processedInputs,
        ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
      })) as EmbeddedSignInFlowResponseV2;

      if (await handleRedirection(response)) {
        return;
      }
      if (
        response.data?.additionalData?.['passkeyChallenge'] ||
        response.data?.additionalData?.['passkeyCreationOptions']
      ) {
        const {passkeyChallenge, passkeyCreationOptions}: any = response.data.additionalData;
        const effectiveExecutionIdForPasskey: any = response.executionId || effectiveExecutionId;

        // Reset passkey processed ref to allow processing
        passkeyProcessedRef.current = false;

        await setChallengeToken(response.challengeToken ?? null);

        // Set passkey state to trigger the passkey
        setPasskeyState({
          actionId: 'submit',
          challenge: passkeyChallenge,
          creationOptions: passkeyCreationOptions,
          error: null,
          executionId: effectiveExecutionIdForPasskey,
          isActive: true,
        });
        setIsSubmitting(false);

        return;
      }

      const {
        executionId: normalizedExecutionId,
        components: normalizedComponents,
        additionalData: normalizedAdditionalData,
      } = normalizeFlowResponse(
        response,
        t,
        {
          resolveTranslations: false,
        },
        meta,
      );

      // Handle Error flow status - flow has failed and is invalidated
      if (response.flowStatus === EmbeddedSignInFlowStatusV2.Error) {
        await clearFlowState();
        const err: any = new Error(extractErrorMessage(response, t));
        setError(err);
        cleanupFlowUrlParams();
        // Throw the error so it's caught by the catch block and propagated to BaseSignIn
        throw err;
      }

      if (response.flowStatus === EmbeddedSignInFlowStatusV2.Complete) {
        // Get redirectUrl from response (from /oauth2/auth/callback) or fall back to afterSignInUrl
        const redirectUrl: any = (response as any)?.redirectUrl || (response as any)?.redirect_uri;
        const finalRedirectUrl: any = redirectUrl || afterSignInUrl;

        // Clear submitting state before redirect
        setIsSubmitting(false);

        // Clear all OAuth-related storage on successful completion
        setExecutionId(null);
        await setChallengeToken(null);
        setIsFlowInitialized(false);
        sessionStorage.removeItem('thunderid_execution_id');
        try {
          const storageManager: any = await getStorageManager();
          await storageManager?.removeHybridDataParameter?.('authId');
        } catch {
          logger.warn('Failed to clear authId from hybrid storage after completion.');
        }

        // Clean up OAuth URL params before redirect
        cleanupOAuthUrlParams(true);

        if (onSuccess) {
          onSuccess({
            redirectUrl: finalRedirectUrl,
            ...(response.data || {}),
          });
        }

        if (finalRedirectUrl && window?.location) {
          window.location.href = finalRedirectUrl;
        }

        return;
      }

      // Always update challenge token on any INCOMPLETE response — token rotates every step.
      await setChallengeToken(response.challengeToken ?? null);

      // Update executionId if response contains a new one
      if (normalizedExecutionId && normalizedComponents) {
        setExecutionId(normalizedExecutionId);
        setComponents(normalizedComponents);
        setAdditionalData(normalizedAdditionalData ?? {});
        setIsTimeoutDisabled(false);
        // Ensure flow is marked as initialized when we have components
        setIsFlowInitialized(true);
        // Clean up executionId from URL after setting it in state
        cleanupFlowUrlParams();

        // Display error from INCOMPLETE response
        if ((response as any)?.error) {
          setFlowError(new Error(extractErrorMessage(response, t)));
        }
      }
    } catch (error) {
      const err: any = error;
      await clearFlowState();

      setError(err instanceof ThunderIDRuntimeError ? err : new Error(extractErrorMessage(err, t)));
      return;
    } finally {
      setIsSubmitting(false);
    }
  };

  /**
   * Handle authentication errors.
   */
  const handleError = (error: Error): void => {
    setError(error);
  };

  useOAuthCallback({
    currentExecutionId,
    isInitialized: isInitialized && !isLoading && isStorageReady,
    isSubmitting,
    onError: (err: any) => {
      clearFlowState();
      setError(err instanceof Error ? err : new Error(String(err)));
    },
    onSubmit: async (payload: any) => handleSubmit({executionId: payload.executionId, inputs: payload.inputs}),
    processedRef: oauthCodeProcessedRef,
    setExecutionId,
  });

  /**
   * Handle passkey authentication/registration when passkey state becomes active.
   * This effect auto-triggers the browser passkey popup and submits the result.
   */
  useEffect(() => {
    if (
      !passkeyState.isActive ||
      (!passkeyState.challenge && !passkeyState.creationOptions) ||
      !passkeyState.executionId
    ) {
      return;
    }

    // Prevent re-processing
    if (passkeyProcessedRef.current) {
      return;
    }
    passkeyProcessedRef.current = true;

    const performPasskeyProcess = async (): Promise<void> => {
      let inputs: Record<string, string>;

      if (passkeyState.challenge) {
        const passkeyResponse: any = await handlePasskeyAuthentication(passkeyState.challenge);
        const passkeyResponseObj: any = JSON.parse(passkeyResponse);

        inputs = {
          authenticatorData: passkeyResponseObj.response.authenticatorData,
          clientDataJSON: passkeyResponseObj.response.clientDataJSON,
          credentialId: passkeyResponseObj.id,
          signature: passkeyResponseObj.response.signature,
          userHandle: passkeyResponseObj.response.userHandle,
        };
      } else if (passkeyState.creationOptions) {
        const passkeyResponse: any = await handlePasskeyRegistration(passkeyState.creationOptions);
        const passkeyResponseObj: any = JSON.parse(passkeyResponse);

        inputs = {
          attestationObject: passkeyResponseObj.response.attestationObject,
          clientDataJSON: passkeyResponseObj.response.clientDataJSON,
          credentialId: passkeyResponseObj.id,
        };
      } else {
        throw new Error('No passkey challenge or creation options available');
      }

      await handleSubmit({
        executionId: passkeyState.executionId ?? undefined,
        inputs,
      });
    };

    performPasskeyProcess()
      .then(() => {
        setPasskeyState({
          actionId: null,
          challenge: null,
          creationOptions: null,
          error: null,
          executionId: null,
          isActive: false,
        });
      })
      .catch((error: any) => {
        setPasskeyState((prev: any) => ({...prev, error: error as Error, isActive: false}));
        setFlowError(error as Error);
        onError?.(error as Error);
      });
  }, [passkeyState.isActive, passkeyState.challenge, passkeyState.creationOptions, passkeyState.executionId]);

  if (children) {
    const renderProps: SignInRenderProps = {
      additionalData,
      components,
      error: flowError,
      initialize: initializeFlow,
      isInitialized: isFlowInitialized,
      isLoading: isLoading || isSubmitting || !isInitialized,
      isTimeoutDisabled,
      meta,
      onSubmit: handleSubmit,
    };

    return <>{children(renderProps)}</>;
  }
  // Otherwise, render the default BaseSignIn component
  return (
    <BaseSignIn
      additionalData={additionalData}
      components={components}
      isLoading={isLoading || !isInitialized || !isFlowInitialized}
      isTimeoutDisabled={isTimeoutDisabled}
      onSubmit={handleSubmit}
      onError={handleError}
      error={flowError}
      className={className}
      size={size}
      variant={variant}
      preferences={preferences}
    />
  );
};

export default SignIn;
