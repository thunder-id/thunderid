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
  EmbeddedFlowComponentType,
  EmbeddedFlowResponseType,
  withVendorCSSClassPrefix,
  createPackageComponentLogger,
} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useEffect, useState, useCallback, useRef} from 'react';
import {renderSignUpComponents} from './SignUpOptionFactory';
import FlowProvider from '../../../../../contexts/Flow/FlowProvider';
import useFlow from '../../../../../contexts/Flow/useFlow';
import useTheme from '../../../../../contexts/Theme/useTheme';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';
import {useForm, FormField} from '../../../../../hooks/useForm';
import useTranslation from '../../../../../hooks/useTranslation';
import AlertPrimitive from '../../../../primitives/Alert/Alert';
// eslint-disable-next-line import/no-named-as-default
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Logo from '../../../../primitives/Logo/Logo';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import useStyles from '../BaseSignUp.styles';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'BaseSignUp',
);

/**
 * Render props for custom UI rendering
 */
export interface BaseSignUpRenderProps {
  /**
   * Flow components
   */
  components: any[];

  /**
   * Form errors
   */
  errors: Record<string, string>;

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
  validateForm: () => {errors: Record<string, string>; isValid: boolean};

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
 * Component that consumes FlowContext and renders the sign-up UI.
 *
 * @internal
 */
const BaseSignUpContent: FC<BaseSignUpProps> = ({
  afterSignUpUrl,
  onInitialize,
  onSubmit,
  onError,
  onFlowChange,
  onComplete,
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
  const {t} = useTranslation();
  const {subtitle: flowSubtitle, title: flowTitle, messages: flowMessages, addMessage, clearMessages} = useFlow();
  useThunderID();
  const styles: any = useStyles(theme, colorScheme);

  const handleError: any = useCallback(
    (error: any) => {
      let errorMessage: string = t('errors.signup.flow.failure');

      if (error && typeof error === 'object') {
        // Handle ThunderID error format with code and description/message
        if (error.code && (error.message || error.description)) {
          errorMessage = error.description || error.message;
        } else if (error instanceof Error && error.name === 'ThunderIDAPIError') {
          try {
            const errorResponse: any = JSON.parse(error.message);
            if (errorResponse.description) {
              errorMessage = errorResponse.description;
            } else if (errorResponse.message) {
              errorMessage = errorResponse.message;
            } else {
              errorMessage = error.message;
            }
          } catch {
            errorMessage = error.message;
          }
        } else if (error.message) {
          errorMessage = error.message;
        }
      } else if (typeof error === 'string') {
        errorMessage = error;
      }

      // Clear existing messages and add the error message
      clearMessages();
      addMessage({
        message: errorMessage,
        type: 'error',
      });
    },
    [t, addMessage, clearMessages],
  );

  const [isLoading, setIsLoading] = useState(false);
  const [isFlowInitialized, setIsFlowInitialized] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<EmbeddedFlowExecuteResponse | null>(null);

  const initializationAttemptedRef: any = useRef(false);

  /**
   * Extract form fields from flow components
   */
  const extractFormFields: any = useCallback(
    (components: any[]): FormField[] => {
      const fields: FormField[] = [];

      const processComponents = (comps: any[]): any => {
        comps.forEach((component: any) => {
          if (component.type === EmbeddedFlowComponentType.Input) {
            const config: any = component.config || {};
            fields.push({
              initialValue: config.defaultValue || '',
              name: config.name || component.id,
              required: config.required || false,
              validator: (value: string) => {
                if (config.required && (!value || value.trim() === '')) {
                  return t('validations.required.field.error');
                }
                // Add email validation if it's an email field
                if (config.type === 'email' && value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
                  return t('field.email.invalid');
                }
                // Add password strength validation if it's a password field
                if (config.type === 'password' && value && value.length < 8) {
                  return t('field.password.weak');
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
    validateOnChange: true,
  });

  const {
    values: formValues,
    touched: touchedFields,
    errors: formErrors,
    isValid: isFormValid,
    setValue: setFormValue,
    setTouched: setFormTouched,
    validateForm,
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
   */
  const handleInputChange = (name: string, value: string): void => {
    setFormValue(name, value);
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

      /**
       * Use `let` for messageHandler and popupMonitor to resolve circular references:
       * messageHandler <-> cleanup <-> popupMonitor.
       * All are assigned before any of them can be invoked at runtime.
       */
      let hasProcessedCallback: any = false; // Prevent multiple processing
      let popupMonitor: any;
      let messageHandler: (event: MessageEvent) => Promise<void>;

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
          const payload: EmbeddedFlowExecuteRequestPayload = {
            ...(currentFlow.flowId && {flowId: currentFlow.flowId}),
            actionId: '',
            flowType: (currentFlow as any).flowType || 'REGISTRATION',
            inputs: {
              code,
              state,
            },
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
                  ...(currentFlow.flowId && {flowId: currentFlow.flowId}),
                  actionId: '',
                  flowType: (currentFlow as any).flowType || 'REGISTRATION',
                  inputs: {
                    code,
                    state,
                  },
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
  const handleSubmit = async (component: any, data?: Record<string, any>): Promise<void> => {
    if (!currentFlow) {
      return;
    }

    setIsLoading(true);
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

      const actionId: string = component.id;

      const payload: EmbeddedFlowExecuteRequestPayload = {
        ...(currentFlow.flowId && {flowId: currentFlow.flowId}),
        flowType: (currentFlow as any).flowType || 'REGISTRATION',
        inputs: filteredInputs,
        ...(actionId && {actionId: actionId}),
      } as any;

      const response: any = await onSubmit(payload);
      onFlowChange?.(response);

      if (response.flowStatus === EmbeddedFlowStatus.Complete) {
        onComplete?.(response);
        return;
      }

      if (response.flowStatus === EmbeddedFlowStatus.Incomplete) {
        if (handleRedirectionIfNeeded(response)) {
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
          buttonClassName: buttonClasses,
          inputClassName: inputClasses,
          onSubmit: handleSubmit,
          size,
          variant,
        },
      ),
    [
      formValues,
      touchedFields,
      formErrors,
      isFormValid,
      isLoading,
      size,
      variant,
      inputClasses,
      buttonClasses,
      handleSubmit,
    ],
  );

  // Initialize the flow on component mount
  useEffect(() => {
    if (isInitialized && !isFlowInitialized && !initializationAttemptedRef.current) {
      initializationAttemptedRef.current = true;

      (async (): Promise<void> => {
        setIsLoading(true);
        clearMessages();

        try {
          const response: any = await onInitialize();

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
    afterSignUpUrl,
    t,
  ]);

  // If render props are provided, use them
  if (children) {
    const renderProps: BaseSignUpRenderProps = {
      components: currentFlow?.data?.components || [],
      errors: formErrors,
      handleInputChange,
      handleSubmit,
      isLoading,
      isValid: isFormValid,
      messages: flowMessages || [],
      subtitle: flowSubtitle || t('signup.subheading'),
      title: flowTitle || t('signup.heading'),
      touched: touchedFields,
      validateForm,
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

  return (
    <CardPrimitive className={cx(containerClasses, styles.card)} variant={variant}>
      {(showTitle || showSubtitle) && (
        <CardPrimitive.Header className={styles.header}>
          {showTitle && (
            <CardPrimitive.Title level={2} className={styles.title}>
              {flowTitle || t('signup.heading')}
            </CardPrimitive.Title>
          )}
          {showSubtitle && (
            <Typography variant="body1" className={styles.subtitle}>
              {flowSubtitle || t('signup.subheading')}
            </Typography>
          )}
        </CardPrimitive.Header>
      )}
      <CardPrimitive.Content>
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
          {currentFlow.data?.components && currentFlow.data.components.length > 0 ? (
            renderComponents(currentFlow.data.components)
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
 * BaseSignUp component that provides embedded sign-up flow for ThunderID.
 * This component handles both the presentation layer and sign-up flow logic.
 * It accepts API functions as props to maintain framework independence.
 *
 * @internal
 */
const BaseSignUp: FC<BaseSignUpProps> = ({showLogo = true, ...rest}: BaseSignUpProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: any = useStyles(theme, colorScheme);

  return (
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
};

export default BaseSignUp;
