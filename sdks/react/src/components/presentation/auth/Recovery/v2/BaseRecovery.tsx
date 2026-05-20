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
  EmbeddedFlowComponentTypeV2 as EmbeddedFlowComponentType,
  withVendorCSSClassPrefix,
  Preferences,
  FlowMetadataResponse,
} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useContext, useEffect, useState, useCallback, useRef} from 'react';
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
import AlertPrimitive from '../../../../primitives/Alert/Alert';
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Logo from '../../../../primitives/Logo/Logo';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import {renderRecoveryComponents} from '../../AuthOptionFactory';
import useStyles from '../../SignUp/BaseSignUp.styles';

/**
 * Render props for custom UI rendering.
 */
export interface BaseRecoveryRenderProps {
  components: any[];
  error?: Error | null;
  fieldErrors: Record<string, string>;
  handleInputChange: (name: string, value: string) => void;
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;
  isLoading: boolean;
  isValid: boolean;
  messages: {message: string; type: string}[];
  meta: FlowMetadataResponse | null;
  subtitle: string;
  title: string;
  touched: Record<string, boolean>;
  validateForm: () => {fieldErrors: Record<string, string>; isValid: boolean};
  values: Record<string, string>;
}

/**
 * Props for the BaseRecovery component.
 */
export interface BaseRecoveryProps {
  afterRecoveryUrl?: string;
  buttonClassName?: string;
  /**
   * Render props function for custom UI or static content
   */
  children?: ((props: BaseRecoveryRenderProps) => ReactNode) | ReactNode;
  className?: string;
  error?: Error | null;
  errorClassName?: string;
  inputClassName?: string;
  isInitialized?: boolean;
  messageClassName?: string;
  onComplete?: (response: EmbeddedFlowExecuteResponse) => void;
  onError?: (error: Error) => void;
  onFlowChange?: (response: EmbeddedFlowExecuteResponse) => void;
  onInitialize?: (payload?: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>;
  onSubmit?: (payload: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>;
  /**
   * Component-level preferences to override global i18n and theme settings.
   */
  preferences?: Preferences;
  showLogo?: boolean;
  showSubtitle?: boolean;
  showTitle?: boolean;
  size?: 'small' | 'medium' | 'large';
  variant?: CardProps['variant'];
}

/**
 * Internal component that renders the V2 recovery UI and manages flow state.
 *
 * @internal
 */
const BaseRecoveryContent: FC<BaseRecoveryProps> = ({
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
}: BaseRecoveryProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const customRenderers: ComponentRendererMap = useContext(ComponentRendererContext);
  const {t} = useTranslation();
  const {subtitle: flowSubtitle, title: flowTitle, messages: flowMessages, addMessage, clearMessages} = useFlow();
  const {meta} = useThunderID();
  const styles: any = useStyles(theme, colorScheme);

  const [isLoading, setIsLoading] = useState(false);
  const [isFlowInitialized, setIsFlowInitialized] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<EmbeddedFlowExecuteResponse | null>(null);
  const [apiError, setApiError] = useState<Error | null>(null);

  const initializationAttemptedRef: any = useRef(false);
  const challengeTokenRef: any = useRef<string | null>(null);

  const handleError: any = useCallback(
    (error: any) => {
      const errorMessage: string = error?.failureReason || extractErrorMessage(error, t);
      setApiError(error instanceof Error ? error : new Error(errorMessage));
      clearMessages();
      addMessage({message: errorMessage, type: 'error'});
    },
    [t, addMessage, clearMessages],
  );

  const normalizeFlowResponseLocal: any = useCallback(
    (response: EmbeddedFlowExecuteResponse): EmbeddedFlowExecuteResponse => {
      if (response?.data?.components && Array.isArray(response.data.components)) {
        return response;
      }

      if (response?.data) {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {defaultErrorKey: 'components.recovery.errors.generic', resolveTranslations: false},
          meta,
        );

        return {...response, data: {...response.data, components: components as any}};
      }

      return response;
    },
    [t, meta],
  );

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
            const fieldName: any = component.ref || component.id;
            fields.push({
              initialValue: '',
              name: fieldName,
              required: component.required || false,
              validator: (value: string) => {
                if (component.required && (!value || value.trim() === '')) {
                  return t('validations.required.field.error');
                }
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

  const handleInputChange = (name: string, value: string): void => {
    setFormValue(name, value);
  };

  const handleInputBlur = (name: string): void => {
    setFormTouched(name, true);
  };

  const handleSubmit = async (component: any, data?: Record<string, any>, skipValidation?: boolean): Promise<void> => {
    if (!currentFlow) return;

    if (!skipValidation) {
      touchAllFields();
      const validation: ReturnType<typeof validateForm> = validateForm();
      if (!validation.isValid) return;
    }

    setIsLoading(true);
    setApiError(null);
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
        ...((currentFlow as any).executionId && {executionId: (currentFlow as any).executionId}),
        flowType: (currentFlow as any).flowType || 'RECOVERY',
        ...(component.id && {action: component.id}),
        ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
        inputs: filteredInputs,
      } as any;

      const rawResponse: any = await onSubmit?.(payload);
      if (!rawResponse) return;
      const response: any = normalizeFlowResponseLocal(rawResponse);
      onFlowChange?.(response);

      if (response.challengeToken !== undefined) {
        challengeTokenRef.current = response.challengeToken ?? null;
      }

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
          _customRenderers: customRenderers,
          _theme: theme,
          buttonClassName: buttonClasses,
          inputClassName: inputClasses,
          meta,
          onInputBlur: handleInputBlur,
          onSubmit: handleSubmit,
          size,
          variant,
        },
      ),
    [
      customRenderers,
      buttonClasses,
      formErrors,
      formValues,
      handleInputBlur,
      handleSubmit,
      inputClasses,
      isFormValid,
      meta,
      isLoading,
      size,
      theme,
      touchedFields,
      variant,
    ],
  );

  useEffect(() => {
    if (isInitialized && !isFlowInitialized && !initializationAttemptedRef.current) {
      initializationAttemptedRef.current = true;

      (async (): Promise<void> => {
        setIsLoading(true);
        setApiError(null);
        clearMessages();

        try {
          const rawResponse: any = await onInitialize?.();
          if (!rawResponse) return;
          const response: any = normalizeFlowResponseLocal(rawResponse);

          if (response.challengeToken !== undefined) {
            challengeTokenRef.current = response.challengeToken ?? null;
          }

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
    isFlowInitialized,
    isInitialized,
    normalizeFlowResponseLocal,
    onComplete,
    onError,
    onFlowChange,
    onInitialize,
    setupFormFields,
    t,
  ]);

  if (children) {
    if (typeof children === 'function') {
      const renderProps: BaseRecoveryRenderProps = {
        components: currentFlow?.data?.components || [],
        error: apiError,
        fieldErrors: formErrors,
        handleInputChange,
        handleSubmit,
        isLoading,
        isValid: isFormValid,
        messages: flowMessages || [],
        meta,
        subtitle: flowSubtitle || t('recovery.subheading'),
        title: flowTitle || t('recovery.heading'),
        touched: touchedFields,
        validateForm: (): {fieldErrors: Record<string, string>; isValid: boolean} => {
          const result: ReturnType<typeof validateForm> = validateForm();
          return {fieldErrors: result.errors, isValid: result.isValid};
        },
        values: formValues,
      };

      return <div className={containerClasses}>{(children as any)(renderProps)}</div>;
    }

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

  const componentsToRender: any = currentFlow.data?.components || [];
  const {title, subtitle, componentsWithoutHeadings} = getAuthComponentHeadings(
    componentsToRender,
    flowTitle,
    flowSubtitle,
    t('recovery.heading'),
    t('recovery.subheading'),
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
              <Typography variant="body1">{t('errors.recovery.components.not.available')}</Typography>
            </AlertPrimitive>
          )}
        </div>
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

/**
 * BaseRecovery component for ThunderIDV2 that provides an embedded account/password recovery flow.
 * Accepts API functions as props to maintain framework independence.
 */
const BaseRecovery: FC<BaseRecoveryProps> = ({
  preferences,
  showLogo = true,
  ...rest
}: BaseRecoveryProps): ReactElement => {
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
        <BaseRecoveryContent showLogo={showLogo} {...rest} />
      </FlowProvider>
    </div>
  );

  if (!preferences) return content;

  return <ComponentPreferencesContext.Provider value={preferences}>{content}</ComponentPreferencesContext.Provider>;
};

export default BaseRecovery;
