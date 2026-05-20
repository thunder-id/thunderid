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
  EmbeddedFlowType,
  FlowMetadataResponse,
  logger,
  OrganizationUnitListResponse,
  Preferences,
} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useCallback, useContext, useEffect, useRef, useState} from 'react';
import useStyles from './BaseInviteUser.styles';
import ComponentRendererContext, {
  ComponentRendererMap,
} from '../../../../../contexts/ComponentRenderer/ComponentRendererContext';
import useTheme from '../../../../../contexts/Theme/useTheme';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';
import useTranslation from '../../../../../hooks/useTranslation';
import {normalizeFlowResponse, extractErrorMessage} from '../../../../../utils/v2/flowTransformer';
import AlertPrimitive from '../../../../primitives/Alert/Alert';
// eslint-disable-next-line import/no-named-as-default
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import {renderInviteUserComponents} from '../../AuthOptionFactory';

/**
 * Flow response structure from the backend.
 */
export interface InviteUserFlowResponse {
  challengeToken?: string;
  data?: {
    additionalData?: Record<string, any>;
    components?: any[];
    meta?: {
      components?: any[];
    };
  };
  executionId: string;
  failureReason?: string;
  flowStatus: 'INCOMPLETE' | 'COMPLETE' | 'ERROR';
  type?: 'VIEW' | 'REDIRECTION';
}

/**
 * Render props for custom UI rendering of InviteUser.
 */
export interface BaseInviteUserRenderProps {
  /**
   * Additional data from the current flow response (e.g. rootOuId, inviteLink).
   */
  additionalData?: Record<string, any>;

  /**
   * Flow components from the current step.
   */
  components: any[];

  /**
   * API error (if any).
   */
  error?: Error | null;

  /**
   * Current flow execution ID.
   */
  executionId?: string;

  /**
   * Field validation errors.
   */
  fieldErrors: Record<string, string>;

  /**
   * Current flow execution ID.
   */
  flowExecId?: string;

  /**
   * Function to handle input blur.
   */
  handleInputBlur: (name: string) => void;

  /**
   * Function to handle input changes.
   */
  handleInputChange: (name: string, value: string) => void;

  /**
   * Function to handle form submission.
   */
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;

  /**
   * Loading state.
   */
  isLoading: boolean;

  /**
   * Whether the form is valid.
   */
  isValid: boolean;

  /**
   * Flow metadata returned by the platform (v2 only). `null` while loading or unavailable.
   */
  meta: FlowMetadataResponse | null;

  /**
   * Reset the flow to invite another user.
   */
  resetFlow: () => void;

  /**
   * Subtitle for the current step.
   */
  subtitle?: string;

  /**
   * Title for the current step.
   */
  title?: string;

  /**
   * Touched fields.
   */
  touched: Record<string, boolean>;

  /**
   * Form values for the current step.
   */
  values: Record<string, string>;
}

/**
 * Props for the BaseInviteUser component.
 */
export interface BaseInviteUserProps {
  /**
   * Render props function for custom UI.
   * If not provided, default UI will be rendered.
   */
  children?: (props: BaseInviteUserRenderProps) => ReactNode;

  /**
   * Custom CSS class name.
   */
  className?: string;

  /**
   * Function to fetch child organization units.
   * When provided, enables the OU tree picker for OU_SELECT components.
   */
  fetchOrganizationUnitChildren?: (
    parentId: string,
    limit: number,
    offset: number,
  ) => Promise<OrganizationUnitListResponse>;

  /**
   * Whether the SDK is initialized.
   */
  isInitialized?: boolean;

  /**
   * Callback when an error occurs.
   */
  onError?: (error: Error) => void;

  /**
   * Callback when the flow state changes.
   */
  onFlowChange?: (response: InviteUserFlowResponse) => void;

  /**
   * Function to initialize the invite user flow.
   * This should make an authenticated request to the flow/execute endpoint.
   */
  onInitialize: (payload: Record<string, any>) => Promise<InviteUserFlowResponse>;

  /**
   * Function to submit flow step data.
   * This should make an authenticated request to the flow/execute endpoint.
   */
  onSubmit: (payload: Record<string, any>) => Promise<InviteUserFlowResponse>;

  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;

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
 * Base component for invite user flow.
 * Handles the flow logic for creating a user and generating an invite link.
 *
 * When no children are provided, renders a default UI with:
 * - Loading spinner during initialization
 * - Error alerts for failures
 * - Flow components (user type selection, user details form)
 * - Invite link display with copy functionality
 *
 * Flow steps handled:
 * 1. User type selection (if multiple types available)
 * 2. User details input (username, email)
 * 3. Invite link generation
 */
const BaseInviteUser: FC<BaseInviteUserProps> = ({
  onInitialize,
  onSubmit,
  onError,
  onFlowChange,
  className = '',
  children,
  fetchOrganizationUnitChildren,
  isInitialized = true,
  preferences,
  size = 'medium',
  variant = 'outlined',
  showTitle = true,
  showSubtitle = true,
}: BaseInviteUserProps): ReactElement => {
  const {meta, isInitialized: isSdkInitialized, getStorageManager} = useThunderID();
  const {t} = useTranslation(preferences?.i18n);
  const {theme} = useTheme();
  const customRenderers: ComponentRendererMap = useContext(ComponentRendererContext);
  const styles: any = useStyles(theme, theme.vars.colors.text.primary);
  const [isLoading, setIsLoading] = useState(false);
  const [isFlowInitialized, setIsFlowInitialized] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<InviteUserFlowResponse | null>(null);
  const [apiError, setApiError] = useState<Error | null>(null);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [touchedFields, setTouchedFields] = useState<Record<string, boolean>>({});
  const [isFormValid, setIsFormValid] = useState(true);
  const challengeTokenRef: any = useRef<string | null>(null);

  const initializationAttemptedRef: any = useRef(false);

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
   * Handle error responses and extract meaningful error messages.
   * Uses the transformer's extractErrorMessage function for consistency.
   */
  const handleError: any = useCallback(
    (error: any) => {
      // Extract error message from response failureReason or use extractErrorMessage
      const errorMessage: string =
        error?.failureReason || extractErrorMessage(error, t, 'components.inviteUser.errors.generic');

      // Set the API error state
      setApiError(error instanceof Error ? error : new Error(errorMessage));

      // Call the onError callback if provided
      onError?.(error instanceof Error ? error : new Error(errorMessage));
    },
    [t, onError],
  );

  /**
   * Normalize flow response to ensure component-driven format.
   * Transforms data.meta.components to data.components.
   */
  const normalizeFlowResponseLocal: any = useCallback(
    (response: InviteUserFlowResponse): InviteUserFlowResponse => {
      if (!response?.data?.meta?.components) {
        return response;
      }

      try {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {
            defaultErrorKey: 'components.inviteUser.errors.generic',
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
      } catch {
        // If transformer throws (e.g., error response), return as-is
        return response;
      }
    },
    [t, children],
  );

  /**
   * Handle input value changes.
   */
  const handleInputChange: any = useCallback((name: string, value: string) => {
    setFormValues((prev: any) => ({...prev, [name]: value}));
    // Clear error when user starts typing
    setFormErrors((prev: any) => {
      const newErrors: any = {...prev};
      delete newErrors[name];
      return newErrors;
    });
  }, []);

  /**
   * Handle input blur.
   */
  const handleInputBlur: any = useCallback((name: string) => {
    setTouchedFields((prev: any) => ({...prev, [name]: true}));
  }, []);

  /**
   * Validate required fields based on components.
   */
  const validateForm: any = useCallback(
    (components: any[]): {errors: Record<string, string>; isValid: boolean} => {
      const errors: Record<string, string> = {};

      const validateComponents = (comps: any[]): any => {
        comps.forEach((comp: any) => {
          if (
            (comp.type === 'TEXT_INPUT' ||
              comp.type === 'EMAIL_INPUT' ||
              comp.type === 'SELECT' ||
              comp.type === 'PHONE_INPUT' ||
              comp.type === 'OTP_INPUT') &&
            comp.required &&
            comp.ref
          ) {
            const value: any = formValues[comp.ref];
            if (!value || value.trim() === '') {
              errors[comp.ref] = `${comp.label || comp.ref} is required`;
            }
            // Email validation
            if (comp.type === 'EMAIL_INPUT' && value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
              errors[comp.ref] = 'Please enter a valid email address';
            }
          }
          if (comp.components && Array.isArray(comp.components)) {
            validateComponents(comp.components);
          }
        });
      };

      validateComponents(components);

      return {errors, isValid: Object.keys(errors).length === 0};
    },
    [formValues],
  );

  /**
   * Handle form submission.
   */
  const handleSubmit: any = useCallback(
    async (component: any, data?: Record<string, any>) => {
      if (!currentFlow) {
        return;
      }

      // Validate form before submission
      const components: any = currentFlow.data?.components || [];
      const validation: any = validateForm(components);

      if (!validation.isValid) {
        setFormErrors(validation.errors);
        setIsFormValid(false);
        // Mark all fields as touched
        const touched: Record<string, boolean> = {};
        Object.keys(validation.errors).forEach((key: any) => {
          touched[key] = true;
        });
        setTouchedFields((prev: any) => ({...prev, ...touched}));
        return;
      }

      setIsLoading(true);
      setApiError(null);
      setIsFormValid(true);

      try {
        // Build payload with form values
        const inputs: any = data || formValues;

        const payload: Record<string, any> = {
          executionId: currentFlow.executionId,
          inputs,
          verbose: true,
          ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
        };

        // Add action ID if component has one
        if (component?.id) {
          payload['action'] = component.id;
        }

        const rawResponse: any = await onSubmit(payload);
        const response: any = normalizeFlowResponseLocal(rawResponse);
        onFlowChange?.(response);

        await setChallengeToken(response.challengeToken ?? null);

        // Check for error status
        if (response.flowStatus === 'ERROR') {
          handleError(response);
          return;
        }

        // Update current flow and reset form for next step
        setCurrentFlow(response);
        setFormValues({});
        setFormErrors({});
        setTouchedFields({});
      } catch (err) {
        handleError(err);
      } finally {
        setIsLoading(false);
      }
    },
    [currentFlow, formValues, validateForm, onSubmit, onFlowChange, handleError, normalizeFlowResponseLocal],
  );

  /**
   * Reset the flow to invite another user.
   */
  const resetFlow: any = useCallback(() => {
    setIsFlowInitialized(false);
    setCurrentFlow(null);
    setApiError(null);
    setFormValues({});
    setFormErrors({});
    setTouchedFields({});
    initializationAttemptedRef.current = false;
  }, []);

  /**
   * Initialize the flow on component mount.
   */
  useEffect(() => {
    if (isInitialized && !isFlowInitialized && !initializationAttemptedRef.current) {
      initializationAttemptedRef.current = true;

      (async (): Promise<void> => {
        setIsLoading(true);
        setApiError(null);

        try {
          const payload: any = {
            flowType: EmbeddedFlowType.UserOnboarding,
            verbose: true,
          };

          const rawResponse: any = await onInitialize(payload);
          const response: any = normalizeFlowResponseLocal(rawResponse);
          await setChallengeToken(response.challengeToken ?? null);
          setCurrentFlow(response);
          setIsFlowInitialized(true);
          onFlowChange?.(response);

          // Check for immediate error
          if (response.flowStatus === 'ERROR') {
            handleError(response);
          }
        } catch (err) {
          handleError(err);
        } finally {
          setIsLoading(false);
        }
      })();
    }
  }, [isInitialized, isFlowInitialized, onInitialize, onFlowChange, handleError, normalizeFlowResponseLocal]);

  /**
   * Recalculate form validity whenever form values or components change.
   * This ensures the submit button is enabled/disabled correctly as the user types.
   */
  useEffect(() => {
    if (currentFlow && isFlowInitialized) {
      const components: any = currentFlow.data?.components || [];
      if (components.length > 0) {
        const validation: any = validateForm(components);
        setIsFormValid(validation.isValid);
      }
    }
  }, [formValues, currentFlow, isFlowInitialized, validateForm]);

  /**
   * Extract title and subtitle from components.
   */
  const extractHeadings: any = useCallback((components: any[]): {subtitle?: string; title?: string} => {
    let title: string | undefined;
    let subtitle: string | undefined;

    components.forEach((comp: any) => {
      if (comp.type === 'TEXT') {
        if (comp.variant === 'HEADING_1' && !title) {
          title = comp.label;
        } else if ((comp.variant === 'HEADING_2' || comp.variant === 'SUBTITLE_1') && !subtitle) {
          subtitle = comp.label;
        }
      }
    });

    return {subtitle, title};
  }, []);

  /**
   * Filter out heading components for default rendering.
   */
  const filterHeadings: any = useCallback(
    (components: any[]): any[] =>
      components.filter(
        (comp: any) => !(comp.type === 'TEXT' && (comp.variant === 'HEADING_1' || comp.variant === 'HEADING_2')),
      ),
    [],
  );

  /**
   * Render form components using the factory.
   */
  const renderComponents: any = useCallback(
    (components: any[]): ReactElement[] =>
      renderInviteUserComponents(
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
          additionalData: currentFlow?.data?.additionalData,
          fetchOrganizationUnitChildren,
          onInputBlur: handleInputBlur,
          onSubmit: handleSubmit,
          size,
          variant,
        },
      ),
    [
      customRenderers,
      currentFlow?.data?.additionalData,
      fetchOrganizationUnitChildren,
      formValues,
      touchedFields,
      formErrors,
      isLoading,
      isFormValid,
      handleInputChange,
      handleInputBlur,
      handleSubmit,
      size,
      theme,
      variant,
    ],
  );

  // Get components from normalized response, with fallback to meta.components
  const components: any = currentFlow?.data?.components || currentFlow?.data?.meta?.components || [];
  const {title, subtitle} = extractHeadings(components);
  const componentsWithoutHeadings: any = filterHeadings(components);

  // Render props
  const renderProps: BaseInviteUserRenderProps = {
    additionalData: currentFlow?.data?.additionalData,
    components,
    error: apiError,
    executionId: currentFlow?.executionId,
    fieldErrors: formErrors,
    handleInputBlur,
    handleInputChange,
    handleSubmit,
    isLoading,
    isValid: isFormValid,
    meta,
    resetFlow,
    subtitle,
    title,
    touched: touchedFields,
    values: formValues,
  };

  // If children render prop is provided, use it for custom UI
  if (children) {
    return <div className={className}>{children(renderProps)}</div>;
  }

  // Default rendering

  // Waiting for SDK initialization
  if (!isInitialized) {
    return (
      <CardPrimitive className={cx(className, styles.card)} variant={variant}>
        <CardPrimitive.Content>
          <div style={{display: 'flex', justifyContent: 'center', padding: '2rem'}}>
            <Spinner size="medium" />
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // Loading state during initialization
  if (!isFlowInitialized && isLoading) {
    return (
      <CardPrimitive className={cx(className, styles.card)} variant={variant}>
        <CardPrimitive.Content>
          <div style={{display: 'flex', justifyContent: 'center', padding: '2rem'}}>
            <Spinner size="medium" />
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // Error state during initialization
  if (!currentFlow && apiError) {
    return (
      <CardPrimitive className={cx(className, styles.card)} variant={variant}>
        <CardPrimitive.Content>
          <AlertPrimitive variant="error">
            <AlertPrimitive.Title>Error</AlertPrimitive.Title>
            <AlertPrimitive.Description>{apiError.message}</AlertPrimitive.Description>
          </AlertPrimitive>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // Flow components
  return (
    <CardPrimitive className={cx(className, styles.card)} variant={variant}>
      {(showTitle || showSubtitle) && (title || subtitle) && (
        <CardPrimitive.Header className={styles.header}>
          {showTitle && title && (
            <CardPrimitive.Title level={2} className={styles.title}>
              {title}
            </CardPrimitive.Title>
          )}
          {showSubtitle && subtitle && (
            <Typography variant="body1" className={styles.subtitle}>
              {subtitle}
            </Typography>
          )}
        </CardPrimitive.Header>
      )}
      <CardPrimitive.Content>
        {apiError && (
          <div style={{marginBottom: '1rem'}}>
            <AlertPrimitive variant="error">
              <AlertPrimitive.Description>{apiError.message}</AlertPrimitive.Description>
            </AlertPrimitive>
          </div>
        )}
        <div>
          {componentsWithoutHeadings && componentsWithoutHeadings.length > 0
            ? renderComponents(componentsWithoutHeadings)
            : !isLoading && (
                <AlertPrimitive variant="warning">
                  <Typography variant="body1">No form components available</Typography>
                </AlertPrimitive>
              )}
          {isLoading && (
            <div style={{display: 'flex', justifyContent: 'center', padding: '1rem'}}>
              <Spinner size="small" />
            </div>
          )}
        </div>
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

export default BaseInviteUser;
