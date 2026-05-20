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
  withVendorCSSClassPrefix,
  EmbeddedSignInFlowRequestV2 as EmbeddedSignInFlowRequest,
  EmbeddedFlowComponentV2 as EmbeddedFlowComponent,
  FlowMetadataResponse,
  Preferences,
} from '@thunderid/browser';
import {FC, useState, useCallback, useContext, ReactElement, ReactNode} from 'react';
import ComponentRendererContext, {
  ComponentRendererMap,
} from '../../../../../contexts/ComponentRenderer/ComponentRendererContext';
import FlowProvider from '../../../../../contexts/Flow/FlowProvider';
import useFlow from '../../../../../contexts/Flow/useFlow';
import ComponentPreferencesContext from '../../../../../contexts/I18n/ComponentPreferencesContext';
import useTheme from '../../../../../contexts/Theme/useTheme';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';
import {FormField, useForm} from '../../../../../hooks/useForm';
import useTranslation from '../../../../../hooks/useTranslation';
import {extractErrorMessage} from '../../../../../utils/v2/flowTransformer';
import AlertPrimitive from '../../../../primitives/Alert/Alert';
// eslint-disable-next-line import/no-named-as-default
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import {renderSignInComponents} from '../../AuthOptionFactory';
import useStyles from '../BaseSignIn.styles';

/**
 * Render props for custom UI rendering
 */
export interface BaseSignInRenderProps {
  /**
   * Flow components
   */
  components: EmbeddedFlowComponent[];

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
  handleSubmit: (component: EmbeddedFlowComponent, data?: Record<string, any>) => Promise<void>;

  /**
   * Loading state
   */
  isLoading: boolean;

  /**
   * Flag indicating if the step timer has reached zero
   */
  isTimeoutDisabled?: boolean;

  /**
   * Whether the form is valid
   */
  isValid: boolean;

  /**
   * Flow messages
   */
  messages: {message: string; type: string}[];

  /**
   * Flow metadata returned by the platform (v2 only). `null` while loading or unavailable.
   */
  meta: FlowMetadataResponse | null;

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
 * Props for the BaseSignIn component.
 */
export interface BaseSignInProps {
  /**
   * Additional data from the flow response.
   */
  additionalData?: Record<string, any>;

  /**
   * Custom CSS class name for the submit button.
   */
  buttonClassName?: string;

  /**
   * Render props function for custom UI
   */
  children?: (props: BaseSignInRenderProps) => ReactNode;

  /**
   * Custom CSS class name for the form container.
   */
  className?: string;

  /**
   * Array of flow components to render.
   */
  components?: EmbeddedFlowComponent[];

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

  /**
   * Flag to determine if the component is ready to be rendered.
   */
  isLoading?: boolean;

  /**
   * Timer flag disabling actions
   */
  isTimeoutDisabled?: boolean;

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
   * Function to handle form submission.
   * @param payload - The form data to submit.
   * @param component - The component that triggered the submission.
   */
  onSubmit?: (payload: EmbeddedSignInFlowRequest, component: EmbeddedFlowComponent) => Promise<void>;

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
  variant?: CardProps['variant'];
}

/**
 * Internal component that consumes FlowContext and renders the sign-in UI.
 */
const BaseSignInContent: FC<BaseSignInProps> = ({
  components = [],
  onSubmit,
  onError,
  error: externalError,
  className = '',
  inputClassName = '',
  buttonClassName = '',
  messageClassName = '',
  size = 'medium',
  variant = 'outlined',
  isLoading: externalIsLoading,
  children,
  additionalData = {},
  isTimeoutDisabled = false,
}: BaseSignInProps): ReactElement => {
  const {meta} = useThunderID();
  const {theme} = useTheme();
  const customRenderers: ComponentRendererMap = useContext(ComponentRendererContext);
  const {t} = useTranslation();
  const {subtitle: flowSubtitle, title: flowTitle, messages: flowMessages, addMessage, clearMessages} = useFlow();
  const styles: any = useStyles(theme, theme.vars.colors.text.primary);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [apiError, setApiError] = useState<Error | null>(null);

  const isLoading: boolean = externalIsLoading || isSubmitting;

  /**
   * Handle error responses and extract meaningful error messages
   * Uses the transformer's extractErrorMessage function for consistency
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
   * Extract form fields from flow components
   */
  const extractFormFields: (components: EmbeddedFlowComponent[]) => FormField[] = useCallback(
    (flowComponents: EmbeddedFlowComponent[]): FormField[] => {
      const fields: FormField[] = [];

      const processComponents = (comps: EmbeddedFlowComponent[]): any => {
        comps.forEach((component: any) => {
          if (
            component.type === 'TEXT_INPUT' ||
            component.type === 'PASSWORD_INPUT' ||
            component.type === 'EMAIL_INPUT' ||
            component.type === 'PHONE_INPUT' ||
            component.type === 'OTP_INPUT'
          ) {
            const identifier: string = component.ref;
            fields.push({
              initialValue: '',
              name: identifier,
              required: component.required || false,
              validator: (value: string) => {
                if (component.required && (!value || value.trim() === '')) {
                  return t('validations.required.field.error');
                }
                // Add email validation if it's an email field
                if (
                  (component.type === 'EMAIL_INPUT' || component.variant === 'EMAIL') &&
                  value &&
                  !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
                ) {
                  return t('field.email.invalid');
                }

                return null;
              },
            });
          }
          if (component.components) {
            processComponents(component.components);
          }
        });
      };

      processComponents(flowComponents);
      return fields;
    },
    [t],
  );

  const formFields: FormField[] = components ? extractFormFields(components) : [];

  const form: ReturnType<typeof useForm> = useForm<Record<string, string>>({
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
  } = form;

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
   * Handle component submission (for buttons and actions).
   */
  const handleSubmit = async (
    component: EmbeddedFlowComponent,
    data?: Record<string, any>,
    skipValidation?: boolean,
  ): Promise<void> => {
    // Only validate for form submit actions, skip for social/trigger actions
    if (!skipValidation) {
      // Mark all fields as touched before validation
      touchAllFields();

      const validation: ReturnType<typeof validateForm> = validateForm();

      if (!validation.isValid) {
        return;
      }
    }

    setIsSubmitting(true);
    setApiError(null);
    clearMessages();

    try {
      // Filter out empty or undefined input values
      const filteredInputs: Record<string, any> = {};
      if (data) {
        Object.keys(data).forEach((key: any) => {
          if (data[key] !== undefined && data[key] !== null && data[key] !== '') {
            filteredInputs[key] = data[key];
          }
        });
      }

      let payload: EmbeddedSignInFlowRequest = {};

      // For V2, we always send inputs and action
      payload = {
        ...payload,
        ...(component.id && {action: component.id}),
        inputs: filteredInputs,
      };

      await onSubmit?.(payload, component);
    } catch (err) {
      handleError(err);
      onError?.(err as Error);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Generate CSS classes
  const containerClasses: any = cx(
    [
      withVendorCSSClassPrefix('signin'),
      withVendorCSSClassPrefix(`signin--${size}`),
      withVendorCSSClassPrefix(`signin--${variant}`),
    ],
    className,
  );

  const inputClasses: any = cx(
    [
      withVendorCSSClassPrefix('signin__input'),
      size === 'small' && withVendorCSSClassPrefix('signin__input--small'),
      size === 'large' && withVendorCSSClassPrefix('signin__input--large'),
    ],
    inputClassName,
  );

  const buttonClasses: any = cx(
    [
      withVendorCSSClassPrefix('signin__button'),
      size === 'small' && withVendorCSSClassPrefix('signin__button--small'),
      size === 'large' && withVendorCSSClassPrefix('signin__button--large'),
    ],
    buttonClassName,
  );

  const messageClasses: any = cx([withVendorCSSClassPrefix('signin__messages')], messageClassName);

  /**
   * Render components based on flow data using the factory
   */
  const renderComponents: any = useCallback(
    (flowComponents: EmbeddedFlowComponent[]): ReactElement[] =>
      renderSignInComponents(
        flowComponents,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        handleInputChange,
        {
          _customRenderers: customRenderers,
          _theme: theme,
          additionalData,
          buttonClassName: buttonClasses,
          inputClassName: inputClasses,
          isTimeoutDisabled,
          meta,
          onInputBlur: handleInputBlur,
          onSubmit: handleSubmit,
          size,
          t,
          variant,
        },
      ),
    [
      additionalData,
      customRenderers,
      formValues,
      touchedFields,
      formErrors,
      isFormValid,
      meta,
      t,
      theme,
      isLoading,
      size,
      variant,
      inputClasses,
      buttonClasses,
      handleInputBlur,
      handleSubmit,
      isTimeoutDisabled,
    ],
  );

  // If render props are provided, use them
  if (children) {
    const renderProps: BaseSignInRenderProps = {
      components,
      error: apiError,
      fieldErrors: formErrors,
      handleInputChange,
      handleSubmit,
      isLoading,
      isTimeoutDisabled,
      isValid: isFormValid,
      messages: flowMessages || [],
      meta,
      subtitle: flowSubtitle,
      title: flowTitle || t('signin.heading'),
      touched: touchedFields,
      validateForm: () => {
        const result: any = validateForm();
        return {fieldErrors: result.errors, isValid: result.isValid};
      },
      values: formValues,
    };

    return (
      <div className={containerClasses} data-testid="thunderid-signin">
        {children(renderProps)}
      </div>
    );
  }

  // Default UI rendering
  if (isLoading) {
    return (
      <CardPrimitive className={cx(containerClasses, styles.card)} data-testid="thunderid-signin" variant={variant}>
        <CardPrimitive.Content>
          <div style={{display: 'flex', justifyContent: 'center', padding: '2rem'}}>
            <Spinner />
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  if (!components || components.length === 0) {
    return (
      <CardPrimitive className={cx(containerClasses, styles.card)} data-testid="thunderid-signin" variant={variant}>
        <CardPrimitive.Content>
          <AlertPrimitive variant="warning">
            <Typography variant="body1">{t('errors.signin.components.not.available')}</Typography>
          </AlertPrimitive>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  return (
    <CardPrimitive className={cx(containerClasses, styles.card)} data-testid="thunderid-signin" variant={variant}>
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
            {flowMessages.map((message: any, index: any) => (
              <AlertPrimitive
                key={index}
                variant={message.type === 'error' ? 'error' : 'info'}
                className={cx(styles.flowMessageItem, messageClasses)}
              >
                <AlertPrimitive.Description>{message.message}</AlertPrimitive.Description>
              </AlertPrimitive>
            ))}
          </div>
        )}
        <div className={styles.contentContainer}>{renderComponents(components)}</div>
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

/**
 * Base SignIn component that provides generic authentication flow.
 * This component handles component-driven UI rendering and can transform input
 * structure to component-driven format automatically.
 *
 * @example
 * // Default UI
 * ```tsx
 * import { BaseSignIn } from '@thunderid/react';
 *
 * const MySignIn = () => {
 *   return (
 *     <BaseSignIn
 *       components={components}
 *       onSubmit={async (payload) => {
 *         return await handleAuth(payload);
 *       }}
 *       onSuccess={(authData) => {
 *         console.log('Success:', authData);
 *       }}
 *       className="max-w-md mx-auto"
 *     />
 *   );
 * };
 * ```
 *
 * @example
 * // Custom UI with render props
 * ```tsx
 * <BaseSignIn components={components} onSubmit={handleSubmit}>
 *   {({values, errors, handleInputChange, handleSubmit, isLoading, components}) => (
 *     <div className="custom-form">
 *       <input
 *         name="username"
 *         value={values.username || ''}
 *         onChange={(e) => handleInputChange('username', e.target.value)}
 *       />
 *       {errors.username && <span>{errors.username}</span>}
 *       <button
 *         onClick={() => handleSubmit(components[0], values)}
 *         disabled={isLoading}
 *       >
 *         Sign In
 *       </button>
 *     </div>
 *   )}
 * </BaseSignIn>
 * ```
 */
const BaseSignIn: FC<BaseSignInProps> = ({preferences, ...rest}: BaseSignInProps): ReactElement => {
  const content: ReactElement = (
    <FlowProvider>
      <BaseSignInContent {...rest} />
    </FlowProvider>
  );

  if (!preferences) return content;

  return <ComponentPreferencesContext.Provider value={preferences}>{content}</ComponentPreferencesContext.Provider>;
};

export default BaseSignIn;
