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

import {cx} from '@emotion/css';
import {
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowStatus,
  EmbeddedFlowComponentType,
  withVendorCSSClassPrefix,
} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, useEffect, useState, useCallback, useRef} from 'react';
import {renderRecoveryComponents} from './RecoveryOptionFactory';
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
import useStyles from '../../SignUp/BaseSignUp.styles';

/**
 * Render props for custom UI rendering.
 */
export interface BaseRecoveryRenderProps {
  components: any[];
  errors: Record<string, string>;
  handleInputChange: (name: string, value: string) => void;
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;
  isLoading: boolean;
  isValid: boolean;
  messages: {message: string; type: string}[];
  subtitle: string;
  title: string;
  touched: Record<string, boolean>;
  validateForm: () => {errors: Record<string, string>; isValid: boolean};
  values: Record<string, string>;
}

/**
 * Props for the BaseRecovery component.
 */
export interface BaseRecoveryProps {
  afterRecoveryUrl?: string;
  buttonClassName?: string;
  className?: string;
  errorClassName?: string;
  inputClassName?: string;
  isInitialized?: boolean;
  messageClassName?: string;
  onComplete?: (response: EmbeddedFlowExecuteResponse) => void;
  onError?: (error: Error) => void;
  onFlowChange?: (response: EmbeddedFlowExecuteResponse) => void;
  onInitialize?: (payload?: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>;
  onSubmit?: (payload: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>;
  showLogo?: boolean;
  showSubtitle?: boolean;
  showTitle?: boolean;
  size?: 'small' | 'medium' | 'large';
  variant?: CardProps['variant'];
}

/**
 * Internal component that renders the recovery UI and manages flow state.
 *
 * @internal
 */
const BaseRecoveryContent: FC<PropsWithChildren<BaseRecoveryProps>> = ({
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
}: PropsWithChildren<BaseRecoveryProps>): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const {t} = useTranslation();
  const {subtitle: flowSubtitle, title: flowTitle, messages: flowMessages, addMessage, clearMessages} = useFlow();
  useThunderID();
  const styles: any = useStyles(theme, colorScheme);

  const handleError: any = useCallback(
    (error: any) => {
      let errorMessage: string = t('errors.recovery.flow.failure');

      if (error && typeof error === 'object') {
        if (error.code && (error.message || error.description)) {
          errorMessage = error.description || error.message;
        } else if (error instanceof Error && error.name === 'ThunderIDAPIError') {
          try {
            const errorResponse: any = JSON.parse(error.message);
            errorMessage = errorResponse.description || errorResponse.message || error.message;
          } catch {
            errorMessage = error.message;
          }
        } else if (error.message) {
          errorMessage = error.message;
        }
      } else if (typeof error === 'string') {
        errorMessage = error;
      }

      clearMessages();
      addMessage({message: errorMessage, type: 'error'});
    },
    [t, addMessage, clearMessages],
  );

  const [isLoading, setIsLoading] = useState(false);
  const [isFlowInitialized, setIsFlowInitialized] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<EmbeddedFlowExecuteResponse | null>(null);

  const initializationAttemptedRef: any = useRef(false);

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
                if (config.type === 'email' && value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
                  return t('field.email.invalid');
                }
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
    touchAllFields,
    reset: resetForm,
  } = form;

  const setupFormFields: any = useCallback(
    (flowResponse: EmbeddedFlowExecuteResponse) => {
      const fields: any = extractFormFields(flowResponse.data?.components || []);
      const initialValues: Record<string, string> = {};
      fields.forEach((field: any) => {
        initialValues[field.name] = field.initialValue || '';
      });
      resetForm();
      Object.keys(initialValues).forEach((key: any) => setFormValue(key, initialValues[key]));
    },
    [extractFormFields, resetForm, setFormValue],
  );

  const handleInputChange: (name: string, value: string) => void = useCallback(
    (name: string, value: string): void => {
      setFormValue(name, value);
      setFormTouched(name, true);
    },
    [setFormValue, setFormTouched],
  );

  const handleSubmit = async (component: any, data?: Record<string, any>, skipValidation?: boolean): Promise<void> => {
    if (!currentFlow) return;

    if (!skipValidation) {
      touchAllFields();
      const validation: ReturnType<typeof validateForm> = validateForm();
      if (!validation.isValid) return;
    }

    setIsLoading(true);
    clearMessages();

    try {
      const filteredInputs: Record<string, any> = {};
      if (data) {
        Object.entries(data).forEach(([key, value]: [string, any]) => {
          if (value !== null && value !== undefined && value !== '') {
            filteredInputs[key] = value;
          }
        });
      }

      const payload: EmbeddedFlowExecuteRequestPayload = {
        ...(currentFlow.flowId && {flowId: currentFlow.flowId}),
        flowType: (currentFlow as any).flowType || 'RECOVERY',
        inputs: filteredInputs,
        ...(component.id && {actionId: component.id as string}),
      } as any;

      const response: any = await onSubmit?.(payload);
      if (!response) return;
      onFlowChange?.(response);

      if (response.flowStatus === EmbeddedFlowStatus.Complete) {
        onComplete?.(response);
        return;
      }

      if (response.flowStatus === EmbeddedFlowStatus.Incomplete) {
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
      withVendorCSSClassPrefix('recovery'),
      withVendorCSSClassPrefix(`recovery--${size}`),
      withVendorCSSClassPrefix(`recovery--${variant}`),
    ],
    className,
  );

  const inputClasses: any = cx(
    [
      withVendorCSSClassPrefix('recovery__input'),
      size === 'small' && withVendorCSSClassPrefix('recovery__input--small'),
      size === 'large' && withVendorCSSClassPrefix('recovery__input--large'),
    ],
    inputClassName,
  );

  const buttonClasses: any = cx(
    [
      withVendorCSSClassPrefix('recovery__button'),
      size === 'small' && withVendorCSSClassPrefix('recovery__button--small'),
      size === 'large' && withVendorCSSClassPrefix('recovery__button--large'),
    ],
    buttonClassName,
  );

  const errorClasses: any = cx([withVendorCSSClassPrefix('recovery__error')], errorClassName);
  const messageClasses: any = cx([withVendorCSSClassPrefix('recovery__messages')], messageClassName);

  const renderComponents: any = useCallback(
    (components: any[]): ReactElement[] =>
      renderRecoveryComponents(
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
      buttonClasses,
      formErrors,
      formValues,
      handleInputChange,
      handleSubmit,
      inputClasses,
      isFormValid,
      isLoading,
      size,
      touchedFields,
      variant,
    ],
  );

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
    clearMessages,
    handleError,
    isFlowInitialized,
    isInitialized,
    onComplete,
    onError,
    onFlowChange,
    onInitialize,
    setupFormFields,
  ]);

  if (children) {
    return <div className={containerClasses}>{children}</div>;
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
            <AlertPrimitive.Description>{t('errors.recovery.flow.initialization.failure')}</AlertPrimitive.Description>
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
              {flowTitle || t('recovery.heading')}
            </CardPrimitive.Title>
          )}
          {showSubtitle && (
            <Typography variant="body1" className={styles.subtitle}>
              {flowSubtitle || t('recovery.subheading')}
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
              <Typography variant="body1">{t('errors.recovery.components.not.available')}</Typography>
            </AlertPrimitive>
          )}
        </div>
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

/**
 * BaseRecovery component for ThunderID V1 that provides an embedded account/password recovery flow.
 * Accepts API functions as props to maintain framework independence.
 *
 * @internal
 */
const BaseRecovery: FC<PropsWithChildren<BaseRecoveryProps>> = ({
  showLogo = true,
  ...rest
}: PropsWithChildren<BaseRecoveryProps>): ReactElement => {
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
        <BaseRecoveryContent showLogo={showLogo} {...rest} />
      </FlowProvider>
    </div>
  );
};

export default BaseRecovery;
