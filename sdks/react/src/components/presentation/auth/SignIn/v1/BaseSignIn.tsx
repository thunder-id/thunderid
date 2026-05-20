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

import {cx} from '@emotion/css';
import {
  EmbeddedSignInFlowAuthenticator,
  EmbeddedSignInFlowInitiateResponse,
  EmbeddedSignInFlowHandleResponse,
  EmbeddedSignInFlowStepType,
  EmbeddedSignInFlowStatus,
  EmbeddedSignInFlowAuthenticatorPromptType,
  ApplicationNativeAuthenticationConstants,
  ThunderIDAPIError,
  withVendorCSSClassPrefix,
  EmbeddedSignInFlowHandleRequestPayload,
  EmbeddedFlowExecuteRequestConfig,
  handleWebAuthnAuthentication,
  createPackageComponentLogger,
} from '@thunderid/browser';
import {FC, FormEvent, RefObject, useEffect, useState, useCallback, useRef, ReactElement} from 'react';
import {createSignInOptionFromAuthenticator} from './options/SignInOptionFactory';
import FlowProvider from '../../../../../contexts/Flow/FlowProvider';
import useFlow from '../../../../../contexts/Flow/useFlow';
import useTheme from '../../../../../contexts/Theme/useTheme';
import {useForm, FormField} from '../../../../../hooks/useForm';
import useTranslation from '../../../../../hooks/useTranslation';
import AlertPrimitive, {AlertVariant} from '../../../../primitives/Alert/Alert';
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Divider from '../../../../primitives/Divider/Divider';
import Logo from '../../../../primitives/Logo/Logo';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import useStyles from '../BaseSignIn.styles';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'BaseSignIn',
);

/**
 * Check if the authenticator is a passkey/FIDO authenticator
 */
const isPasskeyAuthenticator = (authenticator: EmbeddedSignInFlowAuthenticator): boolean =>
  authenticator.authenticatorId === ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Passkey &&
  authenticator.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.InternalPrompt &&
  (authenticator.metadata as any)?.additionalData?.challengeData;

/**
 * Props for the BaseSignIn component.
 */
export interface BaseSignInProps {
  afterSignInUrl?: string;

  /**
   * Custom CSS class name for the submit button.
   */
  buttonClassName?: string;

  /**
   * Custom CSS class name for the form container.
   */
  className?: string;

  /**
   * Custom CSS class name for error messages.
   */
  errorClassName?: string;

  /**
   * Custom CSS class name for form inputs.
   */
  inputClassName?: string;

  /**
   * Flag to determine the component is ready to be rendered.
   */
  isLoading?: boolean;

  /**
   * Custom CSS class name for info messages.
   */
  messageClassName?: string;

  /**
   * Callback function called when authentication fails.
   * @param error - The error that occurred during authentication.
   */
  onError?: (error: Error) => void;

  /**
   * Callback function called when authentication flow status changes.
   * @param response - The current authentication response.
   */
  onFlowChange?: (response: EmbeddedSignInFlowInitiateResponse | EmbeddedSignInFlowHandleResponse) => void;

  /**
   * Function to initialize authentication flow.
   * @returns Promise resolving to the initial authentication response.
   */
  onInitialize?: () => Promise<EmbeddedSignInFlowInitiateResponse>;

  /**
   * Function to handle authentication steps.
   * @param payload - The authentication payload.
   * @returns Promise resolving to the authentication response.
   */
  onSubmit?: (
    payload: EmbeddedSignInFlowHandleRequestPayload,
    request: EmbeddedFlowExecuteRequestConfig,
  ) => Promise<EmbeddedSignInFlowHandleResponse>;

  /**
   * Callback function called when authentication is successful.
   * @param authData - The authentication data returned upon successful completion.
   */
  onSuccess?: (authData: Record<string, any>) => void;

  /**
   * Whether to show the logo.
   */
  showLogo?: boolean;
  /**
   * Whether to show the subtitle.
   */
  showSubtitle?: boolean;

  /**
   * Whether to show the title.
   */
  showTitle?: boolean;

  /**
   * Size variant for the component.
   */
  size?: 'small' | 'medium' | 'large';

  /**
   * Theme variant for the component.
   */
  variant?: CardProps['variant'];
}

/**
 * `T3JnYW5pemF0aW9uQXV0aGVudGljYXRvcjpTU08` - OrganizationSSO
 *    Currently, `App-Native Authentication` doesn't support organization SSO.
 *    Tracker: TODO: Create `product-is` issue for this.
 */
const HIDDEN_AUTHENTICATORS: string[] = ['T3JnYW5pemF0aW9uQXV0aGVudGljYXRvcjpTU08'];

/**
 * Internal component that consumes FlowContext and renders the sign-in UI.
 */
const BaseSignInContent: FC<BaseSignInProps> = ({
  afterSignInUrl,
  onInitialize,
  isLoading: externalIsLoading,
  onSubmit,
  onSuccess,
  onError,
  onFlowChange,
  className = '',
  inputClassName = '',
  buttonClassName = '',
  errorClassName = '',
  messageClassName = '',
  size = 'medium',
  variant = 'outlined',
  showTitle = true,
  showSubtitle = true,
}: BaseSignInProps): ReactElement => {
  const {theme} = useTheme();
  const {t} = useTranslation();
  const {subtitle: flowSubtitle, title: flowTitle, messages: flowMessages} = useFlow();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, theme.vars.colors.text.primary);

  const [isSignInInitializationRequestLoading, setIsSignInInitializationRequestLoading] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<EmbeddedSignInFlowInitiateResponse | null>(null);
  const [currentAuthenticator, setCurrentAuthenticator] = useState<EmbeddedSignInFlowAuthenticator | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<{message: string; type: string}[]>([]);

  const isLoading: boolean = externalIsLoading || isSignInInitializationRequestLoading;

  const reRenderCheckRef: RefObject<boolean> = useRef<boolean>(false);

  const formFields: FormField[] =
    currentAuthenticator?.metadata?.params?.map((param: any) => ({
      initialValue: '',
      name: param.param,
      required: currentAuthenticator.requiredParams.includes(param.param),
      validator: (value: string): string | null => {
        if (currentAuthenticator.requiredParams.includes(param.param) && (!value || value.trim() === '')) {
          return t('validations.required.field.error');
        }
        return null;
      },
    })) || [];

  const form: any = useForm<Record<string, string>>({
    fields: formFields,
    initialValues: {},
    requiredMessage: t('validations.required.field.error'),
    validateOnBlur: true,
    validateOnChange: false,
  });

  const {
    values: formValues,
    touched: touchedFields,
    setValue: setFormValue,
    setTouched: setFormTouched,
    validateForm,
    touchAllFields,
    reset: resetForm,
  } = form;

  /**
   * Setup form fields based on the current authenticator.
   */
  const setupFormFields: any = useCallback(
    (authenticator: EmbeddedSignInFlowAuthenticator): void => {
      const initialValues: Record<string, string> = {};
      authenticator.metadata?.params?.forEach((param: any) => {
        initialValues[param.param] = '';
      });

      // Reset form with new values
      resetForm();

      // Set initial values for all fields
      Object.keys(initialValues).forEach((key: string) => {
        setFormValue(key, initialValues[key]);
      });
    },
    [resetForm, setFormValue],
  );

  /**
   * Check if the response contains a redirection URL and perform the redirect if necessary.
   * @param response - The authentication response
   * @returns true if a redirect was performed, false otherwise
   */
  const handleRedirectionIfNeeded = (response: EmbeddedSignInFlowHandleResponse): boolean => {
    if (
      response &&
      'nextStep' in response &&
      response.nextStep &&
      (response.nextStep as any).stepType === EmbeddedSignInFlowStepType.AuthenticatorPrompt &&
      (response.nextStep as any).authenticators?.length === 1
    ) {
      const responseAuthenticator: any = (response.nextStep as any).authenticators[0];
      if (
        responseAuthenticator.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.RedirectionPrompt &&
        responseAuthenticator.metadata?.additionalData?.redirectUrl
      ) {
        /**
         * Open a popup window to handle redirection prompts
         */
        const redirectUrl: string = responseAuthenticator.metadata?.additionalData?.redirectUrl;
        const popup: Window | null = window.open(
          redirectUrl,
          'oauth_popup',
          'width=500,height=600,scrollbars=yes,resizable=yes',
        );

        if (!popup) {
          logger.error('Failed to open popup window');
          return false;
        }

        /**
         * Forward declarations for mutually referencing variables.
         * `messageHandler`, `cleanup`, and `popupMonitor` reference each other,
         * so they are declared with `let` first and assigned below.
         */
        let messageHandler: any;
        let popupMonitor: any;

        const cleanup = (): void => {
          window.removeEventListener('message', messageHandler);
          if (popupMonitor) {
            clearInterval(popupMonitor);
          }
        };

        /**
         * Add an event listener to the window to capture the message from the popup
         */
        messageHandler = async function messageEventHandler(event: MessageEvent): Promise<void> {
          /**
           * Check if the message is from our popup window
           */
          if (event.source !== popup) {
            // Don't log every message rejection to reduce noise
            if (event.source !== window && event.source !== window.parent) {
              // TODO: Add logs
            }
            return;
          }

          /**
           * Check the origin of the message to ensure it's from a trusted source
           */
          const expectedOrigin: string = afterSignInUrl ? new URL(afterSignInUrl).origin : window.location.origin;
          if (event.origin !== expectedOrigin && event.origin !== window.location.origin) {
            return;
          }

          const {code, state} = event.data;

          if (code && state) {
            const payload: EmbeddedSignInFlowHandleRequestPayload = {
              flowId: currentFlow.flowId,
              selectedAuthenticator: {
                authenticatorId: responseAuthenticator.authenticatorId,
                params: {
                  code,
                  state,
                },
              },
            };

            await onSubmit(payload, {
              method: currentFlow?.links[0].method,
              url: currentFlow?.links[0].href,
            });

            popup.close();
            cleanup();
          } else {
            // TODO: Add logs
          }
        };

        window.addEventListener('message', messageHandler);

        /**
         * Monitor popup for closure and URL changes
         */
        let hasProcessedCallback = false; // Prevent multiple processing
        popupMonitor = setInterval(async (): Promise<void> => {
          try {
            if (popup.closed) {
              cleanup();

              return;
            }

            // Skip if we've already processed a callback
            if (hasProcessedCallback) {
              return;
            }

            // Try to access popup URL to check for callback
            try {
              const popupUrl: string = popup.location.href;

              // Check if we've been redirected to the callback URL
              if (popupUrl && (popupUrl.includes('code=') || popupUrl.includes('error='))) {
                hasProcessedCallback = true; // Set flag to prevent multiple processing

                // Parse the URL for OAuth parameters
                const url: URL = new URL(popupUrl);
                const code: string | null = url.searchParams.get('code');
                const state: string | null = url.searchParams.get('state');
                const oauthError: string | null = url.searchParams.get('error');

                if (oauthError) {
                  logger.error('OAuth error:');
                  popup.close();
                  cleanup();
                  return;
                }

                if (code && state) {
                  const payload: EmbeddedSignInFlowHandleRequestPayload = {
                    flowId: currentFlow.flowId,
                    selectedAuthenticator: {
                      authenticatorId: responseAuthenticator.authenticatorId,
                      params: {
                        code,
                        state,
                      },
                    },
                  };

                  const submitResponse: any = await onSubmit(payload, {
                    method: currentFlow?.links[0].method,
                    url: currentFlow?.links[0].href,
                  });

                  popup.close();

                  onFlowChange?.(submitResponse);

                  if (submitResponse?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
                    onSuccess?.(submitResponse.authData);
                  }
                }
              }
            } catch (e) {
              // Cross-origin error is expected when popup navigates to OAuth provider
              // This is normal and we can ignore it
            }
          } catch (e) {
            logger.error('Error monitoring popup:');
          }
        }, 1000);

        return true;
      }
    }
    return false;
  };

  /**
   * Handle form submission.
   */
  const handleSubmit = async (submittedValues: Record<string, string>): Promise<void> => {
    if (!currentFlow || !currentAuthenticator) {
      return;
    }

    // Mark all fields as touched before validation
    touchAllFields();

    const validation: any = validateForm();
    if (!validation.isValid) {
      return;
    }

    setIsSignInInitializationRequestLoading(true);
    setError(null);
    setMessages([]);

    try {
      const payload: EmbeddedSignInFlowHandleRequestPayload = {
        flowId: currentFlow.flowId,
        selectedAuthenticator: {
          authenticatorId: currentAuthenticator.authenticatorId,
          params: submittedValues,
        },
      };

      const response: any = await onSubmit(payload, {
        method: currentFlow?.links[0].method,
        url: currentFlow?.links[0].href,
      });
      onFlowChange?.(response);

      if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
        onSuccess?.(response.authData);
        return;
      }

      if (
        response?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
        response?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
      ) {
        setError(t('errors.signin.flow.completion.failure'));
        return;
      }

      // Check if the response contains a redirection URL and redirect if needed
      if (handleRedirectionIfNeeded(response)) {
        return;
      }

      if (response && 'flowId' in response && 'nextStep' in response) {
        const nextStepResponse: any = response;
        setCurrentFlow(nextStepResponse);

        if (nextStepResponse.nextStep?.authenticators?.length > 0) {
          if (
            nextStepResponse.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
            nextStepResponse.nextStep.authenticators.length > 1
          ) {
            setCurrentAuthenticator(null);
          } else {
            const nextAuthenticator: any = nextStepResponse.nextStep.authenticators[0];
            setCurrentAuthenticator(nextAuthenticator);
            setupFormFields(nextAuthenticator);
          }
        }

        if (nextStepResponse.nextStep?.messages) {
          setMessages(
            nextStepResponse.nextStep.messages.map((msg: any) => ({
              message: msg.message || '',
              type: msg.type || 'INFO',
            })),
          );
        }
      }
    } catch (err) {
      const errorMessage: string = err instanceof ThunderIDAPIError ? err.message : t('errors.signin.flow.failure');
      setError(errorMessage);
      onError?.(err as Error);
    } finally {
      setIsSignInInitializationRequestLoading(false);
    }
  };

  /**
   * Handle authenticator selection for multi-option prompts.
   */
  const handleAuthenticatorSelection = async (
    authenticator: EmbeddedSignInFlowAuthenticator,
    formData?: Record<string, string>,
  ): Promise<void> => {
    if (!currentFlow) {
      return;
    }

    // Mark all fields as touched if we have form data (i.e., this is a submission)
    if (formData) {
      touchAllFields();
    }

    setIsSignInInitializationRequestLoading(true);
    setError(null);
    setMessages([]);

    try {
      // Handle passkey/FIDO authentication
      if (isPasskeyAuthenticator(authenticator)) {
        try {
          const challengeData: any = (authenticator.metadata as any)?.additionalData?.challengeData;
          if (!challengeData) {
            throw new Error('Missing challenge data for passkey authentication');
          }

          const tokenResponse: any = await handleWebAuthnAuthentication(challengeData);

          const payload: EmbeddedSignInFlowHandleRequestPayload = {
            flowId: currentFlow.flowId,
            selectedAuthenticator: {
              authenticatorId: authenticator.authenticatorId,
              params: {
                tokenResponse,
              },
            },
          };

          const response: any = await onSubmit(payload, {
            method: currentFlow?.links[0].method,
            url: currentFlow?.links[0].href,
          });
          onFlowChange?.(response);

          if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
            onSuccess?.(response.authData);
            return;
          }

          if (
            response?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
            response?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
          ) {
            setError(t('errors.signin.flow.passkeys.completion.failure'));
            return;
          }

          // Handle next step if authentication is not complete
          if (response && 'flowId' in response && 'nextStep' in response) {
            const nextStepResponse: any = response;
            setCurrentFlow(nextStepResponse);

            if (nextStepResponse.nextStep?.authenticators?.length > 0) {
              if (
                nextStepResponse.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
                nextStepResponse.nextStep.authenticators.length > 1
              ) {
                setCurrentAuthenticator(null);
              } else {
                const nextAuthenticator: any = nextStepResponse.nextStep.authenticators[0];

                // Check if the next authenticator is also a passkey - if so, auto-trigger it
                if (isPasskeyAuthenticator(nextAuthenticator)) {
                  // Recursively handle the passkey authenticator without showing UI
                  handleAuthenticatorSelection(nextAuthenticator);
                  return;
                }
                setCurrentAuthenticator(nextAuthenticator);
                setupFormFields(nextAuthenticator);
              }
            }

            if (nextStepResponse.nextStep?.messages) {
              setMessages(
                nextStepResponse.nextStep.messages.map((msg: any) => ({
                  message: msg.message || '',
                  type: msg.type || 'INFO',
                })),
              );
            }
          }
        } catch (passkeyError) {
          logger.error('Passkey authentication error:');

          // Provide more context for common errors
          let errorMessage: string =
            passkeyError instanceof Error ? passkeyError.message : t('errors.signin.flow.passkeys.failure');

          // Add additional context for security errors
          if (passkeyError instanceof Error && passkeyError.message.includes('security')) {
            errorMessage +=
              ' This may be due to browser security settings, an insecure connection, or device restrictions.';
          }

          setError(errorMessage);
        }
      } else if (authenticator.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.RedirectionPrompt) {
        const payload: EmbeddedSignInFlowHandleRequestPayload = {
          flowId: currentFlow.flowId,
          selectedAuthenticator: {
            authenticatorId: authenticator.authenticatorId,
            params: {},
          },
        };

        const response: any = await onSubmit(payload, {
          method: currentFlow?.links[0].method,
          url: currentFlow?.links[0].href,
        });
        onFlowChange?.(response);

        if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
          onSuccess?.(response.authData);
          return;
        }

        // Check if the response contains a redirection URL and redirect if needed
        if (handleRedirectionIfNeeded(response)) {
          /* empty - redirect handled */
        }
      } else if (formData) {
        const validation: any = validateForm();
        if (!validation.isValid) {
          return;
        }

        const formPayload: EmbeddedSignInFlowHandleRequestPayload = {
          flowId: currentFlow.flowId,
          selectedAuthenticator: {
            authenticatorId: authenticator.authenticatorId,
            params: formData,
          },
        };

        const formResponse: any = await onSubmit(formPayload, {
          method: currentFlow?.links[0].method,
          url: currentFlow?.links[0].href,
        });
        onFlowChange?.(formResponse);

        if (formResponse?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
          onSuccess?.(formResponse.authData);
          return;
        }

        if (
          formResponse?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
          formResponse?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
        ) {
          setError('Authentication failed. Please check your credentials and try again.');
          return;
        }

        // Check if the response contains a redirection URL and redirect if needed
        if (handleRedirectionIfNeeded(formResponse)) {
          return;
        }

        if (formResponse && 'flowId' in formResponse && 'nextStep' in formResponse) {
          const nextStepResponse: any = formResponse;
          setCurrentFlow(nextStepResponse);

          if (nextStepResponse.nextStep?.authenticators?.length > 0) {
            if (
              nextStepResponse.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
              nextStepResponse.nextStep.authenticators.length > 1
            ) {
              setCurrentAuthenticator(null);
            } else {
              const nextAuthenticator: any = nextStepResponse.nextStep.authenticators[0];

              // Check if the next authenticator is a passkey - if so, auto-trigger it
              if (isPasskeyAuthenticator(nextAuthenticator)) {
                // Recursively handle the passkey authenticator without showing UI
                handleAuthenticatorSelection(nextAuthenticator);
                return;
              }
              setCurrentAuthenticator(nextAuthenticator);
              setupFormFields(nextAuthenticator);
            }
          }

          if (nextStepResponse.nextStep?.messages) {
            setMessages(
              nextStepResponse.nextStep.messages.map((msg: any) => ({
                message: msg.message || '',
                type: msg.type || 'INFO',
              })),
            );
          }
        }
      } else {
        // Check if the authenticator requires user input
        const hasParams: boolean = authenticator.metadata?.params && authenticator.metadata.params.length > 0;

        if (!hasParams) {
          // If no parameters are required, directly authenticate
          const payload: EmbeddedSignInFlowHandleRequestPayload = {
            flowId: currentFlow.flowId,
            selectedAuthenticator: {
              authenticatorId: authenticator.authenticatorId,
              params: {},
            },
          };

          const response: any = await onSubmit(payload, {
            method: currentFlow?.links[0].method,
            url: currentFlow?.links[0].href,
          });
          onFlowChange?.(response);

          if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
            onSuccess?.(response.authData);
            return;
          }

          if (
            response?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
            response?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
          ) {
            setError('Authentication failed. Please try again.');
            return;
          }

          // Check if the response contains a redirection URL and redirect if needed
          if (handleRedirectionIfNeeded(response)) {
            return;
          }

          if (response && 'flowId' in response && 'nextStep' in response) {
            const nextStepResponse: any = response;
            setCurrentFlow(nextStepResponse);

            if (nextStepResponse.nextStep?.authenticators?.length > 0) {
              if (
                nextStepResponse.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
                nextStepResponse.nextStep.authenticators.length > 1
              ) {
                setCurrentAuthenticator(null);
              } else {
                const nextAuthenticator: any = nextStepResponse.nextStep.authenticators[0];

                // Check if the next authenticator is a passkey - if so, auto-trigger it
                if (isPasskeyAuthenticator(nextAuthenticator)) {
                  // Recursively handle the passkey authenticator without showing UI
                  handleAuthenticatorSelection(nextAuthenticator);
                  return;
                }
                setCurrentAuthenticator(nextAuthenticator);
                setupFormFields(nextAuthenticator);
              }
            }

            if (nextStepResponse.nextStep?.messages) {
              setMessages(
                nextStepResponse.nextStep.messages.map((msg: any) => ({
                  message: msg.message || '',
                  type: msg.type || 'INFO',
                })),
              );
            }
          }
        } else {
          // If parameters are required, show the form
          setCurrentAuthenticator(authenticator);
          setupFormFields(authenticator);
        }
      }
    } catch (err) {
      const errorMessage: string = err instanceof ThunderIDAPIError ? err?.message : 'Authenticator selection failed';
      setError(errorMessage);
      onError?.(err as Error);
    } finally {
      setIsSignInInitializationRequestLoading(false);
    }
  };

  /**
   * Handle input value changes.
   */
  const handleInputChange = (param: string, value: string): void => {
    setFormValue(param, value);
    setFormTouched(param, true);
  };

  /**
   * Check if current flow has multiple authenticator options.
   */
  const hasMultipleOptions: any = useCallback(
    (): boolean =>
      currentFlow &&
      'nextStep' in currentFlow &&
      currentFlow.nextStep?.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
      currentFlow.nextStep?.authenticators &&
      currentFlow.nextStep.authenticators.length > 1,
    [currentFlow],
  );

  /**
   * Get available authenticators for selection.
   */
  const getAvailableAuthenticators: any = useCallback((): EmbeddedSignInFlowAuthenticator[] => {
    if (!currentFlow || !('nextStep' in currentFlow) || !currentFlow.nextStep?.authenticators) {
      return [];
    }
    return currentFlow.nextStep.authenticators;
  }, [currentFlow]);

  // Generate CSS classes
  const containerClasses: string = cx(
    [
      withVendorCSSClassPrefix('signin'),
      withVendorCSSClassPrefix(`signin--${size}`),
      withVendorCSSClassPrefix(`signin--${variant}`),
    ],
    className,
  );

  const inputClasses: string = cx(
    [
      withVendorCSSClassPrefix('signin__input'),
      size === 'small' && withVendorCSSClassPrefix('signin__input--small'),
      size === 'large' && withVendorCSSClassPrefix('signin__input--large'),
    ],
    inputClassName,
  );

  const buttonClasses: string = cx(
    [
      withVendorCSSClassPrefix('signin__button'),
      size === 'small' && withVendorCSSClassPrefix('signin__button--small'),
      size === 'large' && withVendorCSSClassPrefix('signin__button--large'),
    ],
    buttonClassName,
  );

  const errorClasses: string = cx([withVendorCSSClassPrefix('signin__error')], errorClassName);

  const messageClasses: string = cx([withVendorCSSClassPrefix('signin__messages')], messageClassName); // Initialize the flow on component mount

  useEffect(() => {
    if (isLoading) {
      return;
    }

    // React 18.x Strict.Mode has a new check for `Ensuring reusable state` to facilitate an upcoming react feature.
    // https://reactjs.org/docs/strict-mode.html#ensuring-reusable-state
    // This will remount all the useEffects to ensure that there are no unexpected side effects.
    // When react remounts the SignIn, it will send two authorize requests.
    // https://github.com/reactwg/react-18/discussions/18#discussioncomment-795623
    if (reRenderCheckRef.current) {
      return;
    }

    reRenderCheckRef.current = true;

    (async (): Promise<void> => {
      setIsSignInInitializationRequestLoading(true);
      setError(null);

      try {
        const response: any = await onInitialize();

        setCurrentFlow(response);
        setIsInitialized(true);
        onFlowChange?.(response);

        if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
          onSuccess?.(response.authData || {});
          return;
        }

        if (response?.nextStep?.authenticators?.length > 0) {
          if (
            response.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
            response.nextStep.authenticators.length > 1
          ) {
            setCurrentAuthenticator(null);
          } else {
            const authenticator: any = response.nextStep.authenticators[0];
            setCurrentAuthenticator(authenticator);
            setupFormFields(authenticator);
          }
        }

        if (response && 'nextStep' in response && response.nextStep && 'messages' in response.nextStep) {
          const stepMessages: any[] = response.nextStep.messages || [];
          setMessages(
            stepMessages.map((msg: any) => ({
              message: msg.message || '',
              type: msg.type || 'INFO',
            })),
          );
        }
      } catch (err) {
        const errorMessage: string = err instanceof ThunderIDAPIError ? err.message : t('errors.signin.initialization');
        setError(errorMessage);
        onError?.(err as Error);
      } finally {
        setIsSignInInitializationRequestLoading(false);
      }
    })();
  }, [isLoading]);

  if (!isInitialized && isLoading) {
    return (
      <CardPrimitive className={cx(containerClasses, styles['card'])} data-testid="thunderid-signin" variant={variant}>
        <CardPrimitive.Content>
          <div className={styles['loadingContainer']}>
            <Spinner size="medium" />
            <Typography variant="body1" className={styles['loadingText']}>
              {t('messages.loading.placeholder')}
            </Typography>
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  if (hasMultipleOptions() && !currentAuthenticator) {
    const availableAuthenticators: EmbeddedSignInFlowAuthenticator[] = getAvailableAuthenticators();

    const userPromptAuthenticators: any[] = availableAuthenticators.filter(
      (auth: any) =>
        auth.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.UserPrompt ||
        // Fallback: LOCAL authenticators with params are typically user prompts
        (auth.idp === 'LOCAL' && auth.metadata?.params && auth.metadata.params.length > 0),
    );

    const optionAuthenticators: any[] = availableAuthenticators
      .filter((auth: any) => !userPromptAuthenticators.includes(auth))
      .filter((authenticator: any) => !HIDDEN_AUTHENTICATORS.includes(authenticator.authenticatorId));

    return (
      <CardPrimitive className={cx(containerClasses, styles['card'])} data-testid="thunderid-signin" variant={variant}>
        {(showTitle || showSubtitle) && (
          <CardPrimitive.Header className={styles['header']}>
            {showTitle && (
              <CardPrimitive.Title level={2} className={styles['title']}>
                {flowTitle || t('signin.heading')}
              </CardPrimitive.Title>
            )}
            {showSubtitle && (
              <Typography variant="body1" className={styles['subtitle']}>
                {flowSubtitle || t('signin.subheading')}
              </Typography>
            )}
          </CardPrimitive.Header>
        )}
        <CardPrimitive.Content>
          {flowMessages && flowMessages.length > 0 && (
            <div className={styles['flowMessagesContainer']}>
              {flowMessages.map((flowMessage: any, index: number) => (
                <AlertPrimitive
                  key={flowMessage.id || index}
                  variant={flowMessage.type}
                  className={cx(styles['flowMessageItem'], messageClasses)}
                >
                  <AlertPrimitive.Description>{flowMessage.message}</AlertPrimitive.Description>
                </AlertPrimitive>
              ))}
            </div>
          )}
          {messages.length > 0 && (
            <div className={styles['messagesContainer']}>
              {messages.map((message: any, index: number) => {
                let messageVariant: AlertVariant;
                const lowerType: string = message.type.toLowerCase();
                if (lowerType === 'error') {
                  messageVariant = 'error';
                } else if (lowerType === 'warning') {
                  messageVariant = 'warning';
                } else if (lowerType === 'success') {
                  messageVariant = 'success';
                } else {
                  messageVariant = 'info';
                }

                return (
                  <AlertPrimitive
                    key={index}
                    variant={messageVariant}
                    className={cx(styles['messageItem'], messageClasses)}
                  >
                    <AlertPrimitive.Description>{message.message}</AlertPrimitive.Description>
                  </AlertPrimitive>
                );
              })}
            </div>
          )}
          {error && (
            <AlertPrimitive variant="error" className={cx(styles['errorContainer'], errorClasses)}>
              <AlertPrimitive.Title>Error</AlertPrimitive.Title>
              <AlertPrimitive.Description>{error}</AlertPrimitive.Description>
            </AlertPrimitive>
          )}

          <div className={styles['contentContainer']}>
            {/* Render USER_PROMPT authenticators as form fields */}
            {userPromptAuthenticators.map((authenticator: any, index: number) => (
              <div key={authenticator.authenticatorId} className={styles['authenticatorItem']}>
                {index > 0 && <Divider className={styles['divider']}>OR</Divider>}
                <form
                  className={styles['form']}
                  onSubmit={(e: FormEvent): void => {
                    e.preventDefault();
                    const formData: Record<string, string> = {};
                    authenticator.metadata?.params?.forEach((param: any) => {
                      formData[param.param] = formValues[param.param] || '';
                    });
                    handleAuthenticatorSelection(authenticator, formData);
                  }}
                >
                  {createSignInOptionFromAuthenticator(
                    authenticator,
                    formValues,
                    touchedFields,
                    isLoading,
                    handleInputChange,
                    (auth: any, formData: any) => handleAuthenticatorSelection(auth, formData),
                    {
                      buttonClassName: buttonClasses,
                      error,
                      inputClassName: inputClasses,
                    },
                  )}
                </form>
              </div>
            ))}

            {/* Add divider between user prompts and option authenticators if both exist */}
            {userPromptAuthenticators.length > 0 && optionAuthenticators.length > 0 && (
              <Divider className={styles['divider']}>OR</Divider>
            )}

            {/* Render all other authenticators (REDIRECTION_PROMPT, multi-option buttons, etc.) */}
            {optionAuthenticators.map((authenticator: any) => (
              <div key={authenticator.authenticatorId} className={styles['authenticatorItem']}>
                {createSignInOptionFromAuthenticator(
                  authenticator,
                  formValues,
                  touchedFields,
                  isLoading,
                  handleInputChange,
                  (auth: any, formData: any) => handleAuthenticatorSelection(auth, formData),
                  {
                    buttonClassName: buttonClasses,
                    error,
                    inputClassName: inputClasses,
                  },
                )}
              </div>
            ))}
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  if (!currentAuthenticator) {
    return (
      <CardPrimitive
        className={cx(containerClasses, styles['noAuthenticatorCard'])}
        data-testid="thunderid-signin"
        variant={variant}
      >
        <CardPrimitive.Content>
          {error && (
            <AlertPrimitive variant="error" className={styles['errorAlert']}>
              <AlertPrimitive.Title>{t('errors.heading') || 'Error'}</AlertPrimitive.Title>
              <AlertPrimitive.Description>{error}</AlertPrimitive.Description>
            </AlertPrimitive>
          )}
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // If the current authenticator is a passkey, auto-trigger it instead of showing a form
  if (isPasskeyAuthenticator(currentAuthenticator) && !isLoading) {
    // Auto-trigger passkey authentication
    useEffect(() => {
      handleAuthenticatorSelection(currentAuthenticator);
    }, [currentAuthenticator]);

    // Show loading state while passkey authentication is in progress
    return (
      <CardPrimitive className={cx(containerClasses, styles['card'])} data-testid="thunderid-signin" variant={variant}>
        <CardPrimitive.Content>
          <div className={styles['centeredContainer']}>
            <div className={styles['passkeyContainer']}>
              <Spinner size="large" />
            </div>
            <Typography variant="body1">{t('passkey.authenticating') || 'Authenticating with passkey...'}</Typography>
            <Typography variant="body2" className={styles['passkeyText']}>
              {t('passkey.instruction') || 'Please use your fingerprint, face, or security key to authenticate.'}
            </Typography>
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  return (
    <CardPrimitive className={cx(containerClasses, styles['card'])} data-testid="thunderid-signin" variant={variant}>
      <CardPrimitive.Header className={styles['header']}>
        <CardPrimitive.Title level={2} className={styles['title']}>
          {flowTitle || t('signin.heading')}
        </CardPrimitive.Title>
        <Typography variant="body1" className={styles['subtitle']}>
          {flowSubtitle || t('signin.subheading')}
        </Typography>
        {flowMessages && flowMessages.length > 0 && (
          <div className={styles['flowMessagesContainer']}>
            {flowMessages.map((flowMessage: any, index: number) => (
              <AlertPrimitive
                key={flowMessage.id || index}
                variant={flowMessage.type}
                className={cx(styles['flowMessageItem'], messageClasses)}
              >
                <AlertPrimitive.Description>{flowMessage.message}</AlertPrimitive.Description>
              </AlertPrimitive>
            ))}
          </div>
        )}
        {messages.length > 0 && (
          <div className={styles['messagesContainer']}>
            {messages.map((message: any, index: number) => {
              const messageTypeToVariant: Record<string, AlertVariant> = {
                error: 'error',
                success: 'success',
                warning: 'warning',
              };
              const alertVariant: AlertVariant = messageTypeToVariant[message.type.toLowerCase()] || 'info';

              return (
                <AlertPrimitive
                  key={index}
                  variant={alertVariant}
                  className={cx(styles['messageItem'], messageClasses)}
                >
                  <AlertPrimitive.Description>{message.message}</AlertPrimitive.Description>
                </AlertPrimitive>
              );
            })}
          </div>
        )}
      </CardPrimitive.Header>

      <CardPrimitive.Content>
        {error && (
          <AlertPrimitive variant="error" className={cx(styles['errorContainer'], errorClasses)}>
            <AlertPrimitive.Title>{t('errors.heading')}</AlertPrimitive.Title>
            <AlertPrimitive.Description>{error}</AlertPrimitive.Description>
          </AlertPrimitive>
        )}

        <form
          className={styles['form']}
          onSubmit={(e: FormEvent<HTMLFormElement>): void => {
            e.preventDefault();
            const formData: Record<string, string> = {};
            currentAuthenticator.metadata?.params?.forEach((param: any) => {
              formData[param.param] = formValues[param.param] || '';
            });
            handleSubmit(formData);
          }}
        >
          {createSignInOptionFromAuthenticator(
            currentAuthenticator,
            formValues,
            touchedFields,
            isLoading,
            handleInputChange,
            (authenticator: any, formData: any) => handleSubmit(formData || formValues),
            {
              buttonClassName: buttonClasses,
              error,
              inputClassName: inputClasses,
            },
          )}
        </form>
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

/**
 * Base SignIn component that provides native authentication flow.
 * This component handles both the presentation layer and authentication flow logic.
 * It accepts API functions as props to maintain framework independence.
 *
 * @example
 * ```tsx
 * import { BaseSignIn } from '@thunderid/react';
 *
 * const MySignIn = () => {
 *   return (
 *     <BaseSignIn
 *       onInitialize={async () => {
 *         // Your API call to initialize authentication
 *         return await initializeAuth();
 *       }}
 *       onSubmit={async (payload) => {
 *         // Your API call to handle authentication
 *         return await handleAuth(payload);
 *       }}
 *       onSuccess={(authData) => {
 *         console.log('Success:', authData);
 *       }}
 *       onError={(error) => {
 *         console.error('Error:', error);
 *       }}
 *       className="max-w-md mx-auto"
 *     />
 *   );
 * };
 * ```
 */
const BaseSignIn: FC<BaseSignInProps> = ({showLogo = true, ...rest}: BaseSignInProps): ReactElement => {
  const {theme} = useTheme();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, theme.vars.colors.text.primary);

  return (
    <div>
      {showLogo && (
        <div className={styles['logoContainer']}>
          <Logo size="large" />
        </div>
      )}
      <FlowProvider>
        <BaseSignInContent showLogo={showLogo} {...rest} />
      </FlowProvider>
    </div>
  );
};

export default BaseSignIn;
