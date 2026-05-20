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
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowStatus,
  EmbeddedFlowResponseType,
  withVendorCSSClassPrefix,
  EmbeddedFlowComponentTypeV2 as EmbeddedFlowComponentType,
  createPackageComponentLogger,
  Preferences,
} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useCallback, useContext, useEffect, useRef, useState} from 'react';
import ComponentRendererContext, {
  ComponentRendererMap,
} from '../../../../../contexts/ComponentRenderer/ComponentRendererContext';
import FlowProvider from '../../../../../contexts/Flow/FlowProvider';
import useFlow from '../../../../../contexts/Flow/useFlow';
import ComponentPreferencesContext from '../../../../../contexts/I18n/ComponentPreferencesContext';
import useTheme from '../../../../../contexts/Theme/useTheme';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';
import {useForm, FormField} from '../../../../../hooks/useForm';
import useTranslation from '../../../../../hooks/useTranslation';
import {normalizeFlowResponse, extractErrorMessage} from '../../../../../utils/v2/flowTransformer';
import getAuthComponentHeadings from '../../../../../utils/v2/getAuthComponentHeadings';
import {handlePasskeyRegistration} from '../../../../../utils/v2/passkey';
import AlertPrimitive from '../../../../primitives/Alert/Alert';
// eslint-disable-next-line import/no-named-as-default
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Logo from '../../../../primitives/Logo/Logo';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import {renderSignUpComponents} from '../../AuthOptionFactory';
import useStyles from '../BaseSignUp.styles';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'BaseSignUp',
);

/**
 * State for tracking passkey registration
 */
interface PasskeyState {
  actionId: string | null;
  creationOptions: string | null;
  error: Error | null;
  executionId: string | null;
  isActive: boolean;
}

/**
 * Render props for custom UI rendering
 */
export interface BaseSignUpRenderProps {
  /**
   * Flow components
   */
  components: any[];

  /**
   * API error (if any)
   */
  error?: Error | null;

  /**
   * Field validation errors
   */
  fieldErrors: Record<string, string>;

  /**
   * Function to handle input changes
   */
  handleInputChange: (name: string, value: string) => void;

  /**
   * Function to handle form submission
   */
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;

  /**
   * Loading state
   */
  isLoading: boolean;

  /**
   * Whether the form is valid
   */
  isValid: boolean;

  /**
   * Flow messages
   */
  messages: {message: string; type: string}[];

  /**
   * Flow subtitle
   */
  subtitle: string;

  /**
   * Flow title
   */
  title: string;

  /**
   * Touched fields
   */
  touched: Record<string, boolean>;

  /**
   * Function to validate the form
   */
  validateForm: () => {fieldErrors: Record<string, string>; isValid: boolean};

  /**
   * Form values
   */
  values: Record<string, string>;
}

/**
 * Props for the BaseSignUp component.
 */
export interface BaseSignUpProps {
  /**
   * URL to redirect after successful sign-up.
   */
  afterSignUpUrl?: string;

  /**
   * Custom CSS class name for the submit button.
   */
  buttonClassName?: string;

  /**
   * Render props function for custom UI
   */
  children?: (props: BaseSignUpRenderProps) => ReactNode;

  /**
   * Custom CSS class name for the form container.
   */
  className?: string;

  /**
   * Error object to display
   */
  error?: Error | null;

  /**
   * Custom CSS class name for error messages.
   */
  errorClassName?: string;

  /**
   * Custom CSS class name for form inputs.
   */
  inputClassName?: string;

  isInitialized?: boolean;

  /**
   * Custom CSS class name for info messages.
   */
  messageClassName?: string;

  /**
   * Callback function called when the sign-up flow completes and requires redirection.
   * This allows platform-specific handling of redirects (e.g., Next.js router.push).
   * @param response - The response from the sign-up flow containing the redirect URL, etc.
   */
  onComplete?: (response: EmbeddedFlowExecuteResponse) => void;

  /**
   * Callback function called when sign-up fails.
   * @param error - The error that occurred during sign-up.
   */
  onError?: (error: Error) => void;

  /**
   * Callback function called when sign-up flow status changes.
   * @param response - The current sign-up response.
   */
  onFlowChange?: (response: EmbeddedFlowExecuteResponse) => void;

  /**
   * Function to initialize sign-up flow.
   * @returns Promise resolving to the initial sign-up response.
   */
  onInitialize?: (payload?: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>;

  /**
   * Function to handle sign-up steps.
   * @param payload - The sign-up payload.
   * @returns Promise resolving to the sign-up response.
   */
  onSubmit?: (payload: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>;
  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;

  /**
   *  Whether to redirect after sign-up.
   */
  shouldRedirectAfterSignUp?: boolean;

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
 * Internal component that consumes FlowContext and renders the sign-up UI.
 */
const BaseSignUpContent: FC<BaseSignUpProps> = ({
  afterSignUpUrl,
  onInitialize,
  onSubmit,
  onError,
  onFlowChange,
  onComplete,
  error: externalError,
  className = '',
  inputClassName = '',
  buttonClassName = '',
  errorClassName = '',
  messageClassName = '',
  size = 'medium',
  variant = 'outlined',
  isInitialized,
  children,
  showTitle = true,
  showSubtitle = true,
}: BaseSignUpProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const customRenderers: ComponentRendererMap = useContext(ComponentRendererContext);
  const {t} = useTranslation();
  const {subtitle: flowSubtitle, title: flowTitle, messages: flowMessages, addMessage, clearMessages} = useFlow();
  const {meta, isInitialized: isSdkInitialized, getStorageManager} = useThunderID();
  const styles: any = useStyles(theme, colorScheme);

  const [isLoading, setIsLoading] = useState(false);
  const [isFlowInitialized, setIsFlowInitialized] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<EmbeddedFlowExecuteResponse | null>(null);
  const [apiError, setApiError] = useState<Error | null>(null);
  const [passkeyState, setPasskeyState] = useState<PasskeyState>({
    actionId: null,
    creationOptions: null,
    error: null,
    executionId: null,
    isActive: false,
  });
  const challengeTokenRef: any = useRef<string | null>(null);

  const initializationAttemptedRef: any = useRef(false);
  const passkeyProcessedRef: any = useRef(false);

  /**
   * Restore any challenge token persisted before an OAuth redirect.
   */
  useEffect(() => {
    if (!isSdkInitialized) return;

    (async (): Promise<void> => {
      try {
        const storageManager: any = await getStorageManager();
        const tempData: any = await storageManager?.getTemporaryData();
        if (tempData?.challengeToken) {
          challengeTokenRef.current = tempData.challengeToken as string;
        }
      } catch {
        // StorageManager unavailable — continue without persisted token
      }
    })();
  }, [isSdkInitialized]);

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
   * Handle error responses and extract meaningful error messages
   * Uses the transformer's extractErrorMessage function.
   */
  const handleError: any = useCallback(
    (error: any) => {
      // Extract error message from response failureReason or use extractErrorMessage
      const errorMessage: string = error?.failureReason || extractErrorMessage(error, t);

      // Set the API error state
      setApiError(error instanceof Error ? error : new Error(errorMessage));

      // Clear existing messages and add the error message
      clearMessages();
      addMessage({
        message: errorMessage,
        type: 'error',
      });
    },
    [t, addMessage, clearMessages],
  );

  /**
   * Normalize flow response to ensure component-driven format
   * Uses normalizeFlowResponse for modern API format responses
   */
  const normalizeFlowResponseLocal: any = useCallback(
    (response: EmbeddedFlowExecuteResponse): EmbeddedFlowExecuteResponse => {
      // If response already has components, return as-is
      if (response?.data?.components && Array.isArray(response.data.components)) {
        return response;
      }

      // Use the transformer to handle meta.components structure
      if (response?.data) {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {
            defaultErrorKey: 'components.signUp.errors.generic',
            resolveTranslations: false,
          },
          meta,
        );

        return {
          ...response,
          data: {
            ...response.data,
            components: components as any,
          },
        };
      }

      // Return as-is if no transformation needed
      return response;
    },
    [t, children],
  );

  /**
   * Extract form fields from flow components
   */
  const extractFormFields: any = useCallback(
    (components: any[]): FormField[] => {
      const fields: FormField[] = [];

      const processComponents = (comps: any[]): any => {
        comps.forEach((component: any) => {
          if (
            component.type === EmbeddedFlowComponentType.TextInput ||
            component.type === EmbeddedFlowComponentType.PasswordInput ||
            component.type === EmbeddedFlowComponentType.EmailInput ||
            component.type === EmbeddedFlowComponentType.Select
          ) {
            // Use component.ref (mapped identifier) as the field name instead of component.id
            // This ensures form field names match what the input components use
            const fieldName: any = component.ref || component.id;

            fields.push({
              initialValue: '',
              name: fieldName,
              required: component.required || false,
              validator: (value: string) => {
                if (component.required && (!value || value.trim() === '')) {
                  return t('validations.required.field.error');
                }
                // Add email validation if it's an email field
                if (
                  (component.type === EmbeddedFlowComponentType.EmailInput || component.variant === 'EMAIL') &&
                  value &&
                  !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
                ) {
                  return t('field.email.invalid');
                }

                return null;
              },
            });
          }

          if (component.components && Array.isArray(component.components)) {
            processComponents(component.components);
          }
        });
      };

      processComponents(components);
      return fields;
    },
    [t],
  );

  const formFields: any = currentFlow?.data?.components ? extractFormFields(currentFlow.data.components) : [];

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
    errors: formErrors,
    isValid: isFormValid,
    setValue: setFormValue,
    setTouched: setFormTouched,
    validateForm,
    touchAllFields,
    reset: resetForm,
  } = form;

  /**
   * Setup form fields based on the current flow.
   */
  const setupFormFields: any = useCallback(
    (flowResponse: EmbeddedFlowExecuteResponse) => {
      const fields: any = extractFormFields(flowResponse.data?.components || []);
      const initialValues: Record<string, string> = {};

      fields.forEach((field: any) => {
        initialValues[field.name] = field.initialValue || '';
      });

      resetForm();

      Object.keys(initialValues).forEach((key: any) => {
        setFormValue(key, initialValues[key]);
      });
    },
    [extractFormFields, resetForm, setFormValue],
  );

  /**
   * Handle input value changes.
   * Only updates the value without marking as touched.
   * Touched state is set on blur to avoid premature validation.
   */
  const handleInputChange = (name: string, value: string): void => {
    setFormValue(name, value);
  };

  /**
   * Handle input blur event.
   * Marks the field as touched, which triggers validation.
   */
  const handleInputBlur = (name: string): void => {
    setFormTouched(name, true);
  };

  /**
   * Check if the response contains a redirection URL and perform the redirect if necessary.
   * @param response - The sign-up response
   * @returns true if a redirect was performed, false otherwise
   */
  const handleRedirectionIfNeeded = (response: EmbeddedFlowExecuteResponse): boolean => {
    if (response?.type === EmbeddedFlowResponseType.Redirection && response?.data?.redirectURL) {
      /**
       * Open a popup window to handle redirection prompts for social sign-up
       */
      const redirectUrl: any = response.data.redirectURL;
      const popup: any = window.open(redirectUrl, 'oauth_popup', 'width=500,height=600,scrollbars=yes,resizable=yes');

      if (!popup) {
        logger.error('Failed to open popup window');
        return false;
      }

      let hasProcessedCallback: any = false; // Prevent multiple processing
      let popupMonitor: ReturnType<typeof setInterval> | null = null;
      let messageHandler: ((event: MessageEvent) => Promise<void>) | null = null;

      /**
       * Clean up event listener and popup monitor
       */
      const cleanup = (): void => {
        if (messageHandler) {
          window.removeEventListener('message', messageHandler);
        }
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
          return;
        }

        /**
         * Check the origin of the message to ensure it's from a trusted source
         */
        const expectedOrigin: any = afterSignUpUrl ? new URL(afterSignUpUrl).origin : window.location.origin;
        if (event.origin !== expectedOrigin && event.origin !== window.location.origin) {
          return;
        }

        const {code, state} = event.data;

        if (code && state) {
          hasProcessedCallback = true;

          const payload: EmbeddedFlowExecuteRequestPayload = {
            ...((currentFlow as any).executionId && {executionId: (currentFlow as any).executionId}),
            action: '',
            flowType: (currentFlow as any).flowType || 'REGISTRATION',
            inputs: {
              code,
              state,
            },
            ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
          } as any;

          try {
            const continueResponse: any = await onSubmit(payload);
            onFlowChange?.(continueResponse);

            if (continueResponse.flowStatus === EmbeddedFlowStatus.Complete) {
              onComplete?.(continueResponse);
            } else if (continueResponse.flowStatus === EmbeddedFlowStatus.Incomplete) {
              setCurrentFlow(continueResponse);
              setupFormFields(continueResponse);
            }

            popup.close();
            cleanup();
          } catch (err) {
            handleError(err);
            onError?.(err as Error);
            popup.close();
            cleanup();
          }
        }
      };

      window.addEventListener('message', messageHandler);

      /**
       * Monitor popup for closure and URL changes
       */
      popupMonitor = setInterval(async () => {
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
            const popupUrl: any = popup.location.href;

            // Check if we've been redirected to the callback URL
            if (popupUrl && (popupUrl.includes('code=') || popupUrl.includes('error='))) {
              hasProcessedCallback = true; // Set flag to prevent multiple processing

              // Parse the URL for OAuth parameters
              const url: any = new URL(popupUrl);
              const code: any = url.searchParams.get('code');
              const state: any = url.searchParams.get('state');
              const error: any = url.searchParams.get('error');

              if (error) {
                logger.error('OAuth error:');
                popup.close();
                cleanup();
                return;
              }

              if (code && state) {
                const payload: EmbeddedFlowExecuteRequestPayload = {
                  ...((currentFlow as any).executionId && {executionId: (currentFlow as any).executionId}),
                  action: '',
                  flowType: (currentFlow as any).flowType || 'REGISTRATION',
                  inputs: {
                    code,
                    state,
                  },
                  ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
                } as any;

                try {
                  const continueResponse: any = await onSubmit(payload);
                  onFlowChange?.(continueResponse);

                  if (continueResponse.flowStatus === EmbeddedFlowStatus.Complete) {
                    onComplete?.(continueResponse);
                  } else if (continueResponse.flowStatus === EmbeddedFlowStatus.Incomplete) {
                    setCurrentFlow(continueResponse);
                    setupFormFields(continueResponse);
                  }

                  popup.close();
                } catch (err) {
                  handleError(err);
                  onError?.(err as Error);
                  popup.close();
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

    return false;
  };

  /**
   * Handle component submission (for buttons outside forms).
   */
  const handleSubmit = async (component: any, data?: Record<string, any>, skipValidation?: boolean): Promise<void> => {
    if (!currentFlow) {
      return;
    }

    // Only validate for form submit actions, skip for social/trigger actions
    if (!skipValidation) {
      // Mark all fields as touched before validation
      touchAllFields();

      const validation: ReturnType<typeof validateForm> = validateForm();

      if (!validation.isValid) {
        return;
      }
    }

    setIsLoading(true);
    setApiError(null);
    clearMessages();

    try {
      // Filter out empty or undefined input values
      const filteredInputs: Record<string, any> = {};
      if (data) {
        Object.entries(data).forEach(([key, value]: [string, any]) => {
          if (value !== null && value !== undefined && value !== '') {
            filteredInputs[key] = value;
          }
        });
      }

      const payload: EmbeddedFlowExecuteRequestPayload = {
        ...((currentFlow as any).executionId && {executionId: (currentFlow as any).executionId}),
        flowType: (currentFlow as any).flowType || 'REGISTRATION',
        ...(component.id && {action: component.id}),
        ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
        inputs: filteredInputs,
      } as any;

      const rawResponse: any = await onSubmit(payload);
      const response: any = normalizeFlowResponseLocal(rawResponse);
      onFlowChange?.(response);

      await setChallengeToken(response.challengeToken ?? null);

      if (response.flowStatus === EmbeddedFlowStatus.Complete) {
        onComplete?.(response);
        return;
      }

      if (response.flowStatus === EmbeddedFlowStatus.Incomplete) {
        if (handleRedirectionIfNeeded(response)) {
          return;
        }

        if (response.data?.additionalData?.passkeyCreationOptions) {
          const {passkeyCreationOptions}: any = response.data.additionalData;
          const effectiveExecutionIdForPasskey: any = response.executionId || (currentFlow as any)?.executionId;

          // Reset passkey processed ref to allow processing
          passkeyProcessedRef.current = false;

          // Set passkey state to trigger the passkey
          setPasskeyState({
            actionId: component.id || 'submit',
            creationOptions: passkeyCreationOptions,
            error: null,
            executionId: effectiveExecutionIdForPasskey,
            isActive: true,
          });
          setIsLoading(false);
          return;
        }
        setCurrentFlow(response);
        setupFormFields(response);
      }
    } catch (err) {
      handleError(err);
      onError?.(err as Error);
    } finally {
      setIsLoading(false);
    }
  };

  /**
   * Handle passkey registration when passkey state becomes active.
   * This effect auto-triggers the browser passkey popup and submits the result.
   */
  useEffect(() => {
    if (!passkeyState.isActive || !passkeyState.creationOptions || !passkeyState.executionId) {
      return;
    }

    // Prevent re-processing
    if (passkeyProcessedRef.current) {
      return;
    }
    passkeyProcessedRef.current = true;

    const performPasskeyRegistration = async (): Promise<void> => {
      const passkeyResponse: any = await handlePasskeyRegistration(passkeyState.creationOptions);
      const passkeyResponseObj: any = JSON.parse(passkeyResponse);

      const inputs: any = {
        attestationObject: passkeyResponseObj.response.attestationObject,
        clientDataJSON: passkeyResponseObj.response.clientDataJSON,
        credentialId: passkeyResponseObj.id,
      };

      // After successful registration, submit the result to the server
      const payload: EmbeddedFlowExecuteRequestPayload = {
        actionId: passkeyState.actionId || 'submit',
        executionId: passkeyState.executionId,
        flowType: (currentFlow as any)?.flowType || 'REGISTRATION',
        inputs,
        ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
      } as any;

      const nextResponse: any = await onSubmit(payload);
      const processedResponse: any = normalizeFlowResponseLocal(nextResponse);
      onFlowChange?.(processedResponse);

      if (processedResponse.flowStatus === EmbeddedFlowStatus.Complete) {
        onComplete?.(processedResponse);
      } else {
        setCurrentFlow(processedResponse);
        setupFormFields(processedResponse);
      }
    };

    performPasskeyRegistration()
      .then(() => {
        setPasskeyState({actionId: null, creationOptions: null, error: null, executionId: null, isActive: false});
      })
      .catch((error: any) => {
        setPasskeyState((prev: any) => ({...prev, error: error as Error, isActive: false}));
        handleError(error);
        onError?.(error as Error);
      });
  }, [passkeyState.isActive, passkeyState.creationOptions, passkeyState.executionId]);

  const containerClasses: any = cx(
    [
      withVendorCSSClassPrefix('signup'),
      withVendorCSSClassPrefix(`signup--${size}`),
      withVendorCSSClassPrefix(`signup--${variant}`),
    ],
    className,
  );

  const inputClasses: any = cx(
    [
      withVendorCSSClassPrefix('signup__input'),
      size === 'small' && withVendorCSSClassPrefix('signup__input--small'),
      size === 'large' && withVendorCSSClassPrefix('signup__input--large'),
    ],
    inputClassName,
  );

  const buttonClasses: any = cx(
    [
      withVendorCSSClassPrefix('signup__button'),
      size === 'small' && withVendorCSSClassPrefix('signup__button--small'),
      size === 'large' && withVendorCSSClassPrefix('signup__button--large'),
    ],
    buttonClassName,
  );

  const errorClasses: any = cx([withVendorCSSClassPrefix('signup__error')], errorClassName);

  const messageClasses: any = cx([withVendorCSSClassPrefix('signup__messages')], messageClassName);

  /**
   * Render form components based on flow data using the factory
   */
  const renderComponents: any = useCallback(
    (components: any[]): ReactElement[] =>
      renderSignUpComponents(
        components,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        handleInputChange,
        {
          _customRenderers: customRenderers,
          _theme: theme,
          buttonClassName: buttonClasses,
          inputClassName: inputClasses,
          onInputBlur: handleInputBlur,
          onSubmit: handleSubmit,
          size,
          variant,
        },
      ),
    [
      customRenderers,
      formValues,
      touchedFields,
      formErrors,
      isFormValid,
      isLoading,
      size,
      theme,
      variant,
      inputClasses,
      buttonClasses,
      handleSubmit,
      handleInputBlur,
    ],
  );

  /**
   * Parse URL parameters to check for OAuth redirect state.
   */
  const getUrlParams = (): any => {
    const urlParams: any = new URL(window?.location?.href ?? '').searchParams;
    return {
      code: urlParams.get('code'),
      error: urlParams.get('error'),
      state: urlParams.get('state'),
    };
  };

  // Initialize the flow on component mount
  useEffect(() => {
    // Skip initialization if we're in an OAuth redirect state.
    const urlParams: any = getUrlParams();
    if (urlParams.code || urlParams.state) {
      return;
    }

    if (isInitialized && !isFlowInitialized && !initializationAttemptedRef.current) {
      initializationAttemptedRef.current = true;

      (async (): Promise<void> => {
        setIsLoading(true);
        setApiError(null);
        clearMessages();

        try {
          const rawResponse: any = await onInitialize();
          const response: any = normalizeFlowResponseLocal(rawResponse);

          await setChallengeToken(response.challengeToken ?? null);
          setCurrentFlow(response);
          setIsFlowInitialized(true);
          onFlowChange?.(response);

          if (response.flowStatus === EmbeddedFlowStatus.Complete) {
            onComplete?.(response);
            return;
          }

          if (response.flowStatus === EmbeddedFlowStatus.Incomplete) {
            setupFormFields(response);
          }
        } catch (err) {
          handleError(err);
          onError?.(err as Error);
        } finally {
          setIsLoading(false);
        }
      })();
    }
  }, [
    isInitialized,
    isFlowInitialized,
    onInitialize,
    onComplete,
    onError,
    onFlowChange,
    setupFormFields,
    normalizeFlowResponseLocal,
    afterSignUpUrl,
    t,
  ]);

  // If render props are provided, use them
  if (children) {
    const renderProps: BaseSignUpRenderProps = {
      components: currentFlow?.data?.components || [],
      error: apiError,
      fieldErrors: formErrors,
      handleInputChange,
      handleSubmit,
      isLoading,
      isValid: isFormValid,
      messages: flowMessages || [],
      subtitle: flowSubtitle || t('signup.subheading'),
      title: flowTitle || t('signup.heading'),
      touched: touchedFields,
      validateForm: () => {
        const result: any = validateForm();
        return {fieldErrors: result.errors, isValid: result.isValid};
      },
      values: formValues,
    };

    return <div className={containerClasses}>{children(renderProps)}</div>;
  }

  if (!isFlowInitialized && isLoading) {
    return (
      <CardPrimitive className={cx(containerClasses, styles.card)} variant={variant}>
        <CardPrimitive.Content>
          <div className={styles.loadingContainer}>
            <Spinner size="medium" />
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  if (!currentFlow) {
    return (
      <CardPrimitive className={cx(containerClasses, styles.card)} variant={variant}>
        <CardPrimitive.Content>
          <AlertPrimitive variant="error" className={errorClasses}>
            <AlertPrimitive.Title>{t('errors.heading')}</AlertPrimitive.Title>
            <AlertPrimitive.Description>{t('errors.signup.flow.initialization.failure')}</AlertPrimitive.Description>
          </AlertPrimitive>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // Extract heading and subheading components and filter them from the main components
  const componentsToRender: any = currentFlow.data?.components || [];
  const {title, subtitle, componentsWithoutHeadings} = getAuthComponentHeadings(
    componentsToRender,
    flowTitle,
    flowSubtitle,
    t('signup.heading'),
    t('signup.subheading'),
  );

  return (
    <CardPrimitive className={cx(containerClasses, styles.card)} variant={variant}>
      {(showTitle || showSubtitle) && (
        <CardPrimitive.Header className={styles.header}>
          {showTitle && (
            <CardPrimitive.Title level={2} className={styles.title}>
              {title}
            </CardPrimitive.Title>
          )}
          {showSubtitle && (
            <Typography variant="body1" className={styles.subtitle}>
              {subtitle}
            </Typography>
          )}
        </CardPrimitive.Header>
      )}
      <CardPrimitive.Content>
        {externalError && (
          <div className={styles.flowMessagesContainer}>
            <AlertPrimitive variant="error" className={cx(styles.flowMessageItem, messageClasses)}>
              <AlertPrimitive.Description>{externalError.message}</AlertPrimitive.Description>
            </AlertPrimitive>
          </div>
        )}
        {flowMessages && flowMessages.length > 0 && (
          <div className={styles.flowMessagesContainer}>
            {flowMessages.map((message: any, index: number) => (
              <AlertPrimitive
                key={message.id || index}
                variant={message.type?.toLowerCase() === 'error' ? 'error' : 'info'}
                className={cx(styles.flowMessageItem, messageClasses)}
              >
                <AlertPrimitive.Description>{message.message}</AlertPrimitive.Description>
              </AlertPrimitive>
            ))}
          </div>
        )}
        <div className={styles.contentContainer}>
          {componentsWithoutHeadings && componentsWithoutHeadings.length > 0 ? (
            renderComponents(componentsWithoutHeadings)
          ) : (
            <AlertPrimitive variant="warning">
              <Typography variant="body1">{t('errors.signup.components.not.available')}</Typography>
            </AlertPrimitive>
          )}
        </div>
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

/**
 * BaseSignUp component that provides embedded sign-up flow for ThunderIDV2.
 * This component handles both the presentation layer and sign-up flow logic.
 * It accepts API functions as props to maintain framework independence.
 */
const BaseSignUp: FC<BaseSignUpProps> = ({preferences, showLogo = true, ...rest}: BaseSignUpProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: any = useStyles(theme, colorScheme);

  const content: ReactElement = (
    <div>
      {showLogo && (
        <div className={styles.logoContainer}>
          <Logo size="large" />
        </div>
      )}
      <FlowProvider>
        <BaseSignUpContent showLogo={showLogo} {...rest} />
      </FlowProvider>
    </div>
  );

  if (!preferences) return content;

  return <ComponentPreferencesContext.Provider value={preferences}>{content}</ComponentPreferencesContext.Provider>;
};

export default BaseSignUp;
