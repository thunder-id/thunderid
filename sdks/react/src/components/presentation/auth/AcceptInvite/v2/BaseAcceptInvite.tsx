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
import {FlowMetadataResponse, Preferences} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useCallback, useContext, useEffect, useRef, useState} from 'react';
import useStyles from './BaseAcceptInvite.styles';
import ComponentRendererContext, {
  ComponentRendererMap,
} from '../../../../../contexts/ComponentRenderer/ComponentRendererContext';
import useTheme from '../../../../../contexts/Theme/useTheme';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';
import useTranslation from '../../../../../hooks/useTranslation';
import {useOAuthCallback} from '../../../../../hooks/v2/useOAuthCallback';
import {initiateOAuthRedirect} from '../../../../../utils/oauth';
import {normalizeFlowResponse, extractErrorMessage} from '../../../../../utils/v2/flowTransformer';
import AlertPrimitive from '../../../../primitives/Alert/Alert';
import Button from '../../../../primitives/Button/Button';
// eslint-disable-next-line import/no-named-as-default
import CardPrimitive, {CardProps} from '../../../../primitives/Card/Card';
import Spinner from '../../../../primitives/Spinner/Spinner';
import Typography from '../../../../primitives/Typography/Typography';
import {renderInviteUserComponents} from '../../AuthOptionFactory';

/**
 * Flow response structure from the backend.
 */
export interface AcceptInviteFlowResponse {
  challengeToken?: string;
  data?: {
    additionalData?: Record<string, string>;
    components?: any[];
    meta?: {
      components?: any[];
    };
    redirectURL?: string;
  };
  executionId: string;
  failureReason?: string;
  flowStatus: 'INCOMPLETE' | 'COMPLETE' | 'ERROR';
  type?: 'VIEW' | 'REDIRECTION';
}

/**
 * Render props for custom UI rendering of AcceptInvite.
 */
export interface BaseAcceptInviteRenderProps {
  /**
   * Flow components from the current step.
   */
  components: any[];

  /**
   * API error (if any).
   */
  error?: Error | null;

  /**
   * Current flow ID from URL.
   */
  executionId?: string;

  /**
   * Field validation errors.
   */
  fieldErrors: Record<string, string>;

  /**
   * Navigate to sign in page.
   */
  goToSignIn?: () => void;

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
   * Invite token from URL.
   */
  inviteToken?: string;

  /**
   * Whether the flow has completed successfully.
   */
  isComplete: boolean;

  /**
   * Loading state.
   */
  isLoading: boolean;

  /**
   * Whether the token validation failed.
   */
  isTokenInvalid: boolean;

  /**
   * Whether the form is valid.
   */
  isValid: boolean;

  /**
   * Whether the invite token is being validated.
   */
  isValidatingToken: boolean;

  /**
   * Flow metadata returned by the platform (v2 only). `null` while loading or unavailable.
   */
  meta: FlowMetadataResponse | null;

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
 * Props for the BaseAcceptInvite component.
 */
export interface BaseAcceptInviteProps {
  /**
   * Render props function for custom UI.
   * If not provided, default UI will be rendered.
   */
  children?: (props: BaseAcceptInviteRenderProps) => ReactNode;

  /**
   * Custom CSS class name.
   */
  className?: string;

  /**
   * Flow ID from the invite link URL.
   */
  executionId?: string;

  /**
   * Invite token from the invite link URL.
   */
  inviteToken?: string;

  /**
   * Callback when the flow completes successfully.
   */
  onComplete?: () => void;

  /**
   * Callback when an error occurs.
   */
  onError?: (error: Error) => void;

  /**
   * Callback when the flow state changes.
   */
  onFlowChange?: (response: AcceptInviteFlowResponse) => void;

  /**
   * Callback to navigate to sign in page.
   */
  onGoToSignIn?: () => void;

  /**
   * Function to submit flow step data.
   * This makes a request to the flow/execute endpoint.
   */
  onSubmit: (payload: Record<string, any>) => Promise<AcceptInviteFlowResponse>;

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
 * Base component for accept invite flow (end-user).
 * Handles the flow logic for validating an invite token and setting a password.
 *
 * When no children are provided, renders a default UI with:
 * - Loading spinner during token validation
 * - Error alerts for invalid/expired tokens
 * - Password form with validation
 * - Success state with sign-in redirect
 *
 * Flow steps handled:
 * 1. Validate invite token (automatic on mount)
 * 2. Password input
 * 3. Flow completion
 */
const BaseAcceptInvite: FC<BaseAcceptInviteProps> = ({
  executionId,
  inviteToken,
  onSubmit,
  onComplete,
  onError,
  onFlowChange,
  onGoToSignIn,
  className = '',
  children,
  preferences,
  size = 'medium',
  variant = 'outlined',
  showTitle = true,
  showSubtitle = true,
}: BaseAcceptInviteProps): ReactElement => {
  const {meta, isInitialized, getStorageManager} = useThunderID();
  const {t} = useTranslation(preferences?.i18n);
  const {theme} = useTheme();
  const customRenderers: ComponentRendererMap = useContext(ComponentRendererContext);
  const styles: any = useStyles(theme, theme.vars.colors.text.primary);
  const [isLoading, setIsLoading] = useState(false);
  const [isValidatingToken, setIsValidatingToken] = useState(true);
  const [isTokenInvalid, setIsTokenInvalid] = useState(false);
  const [isComplete, setIsComplete] = useState(false);
  const [currentFlow, setCurrentFlow] = useState<AcceptInviteFlowResponse | null>(null);
  const [apiError, setApiError] = useState<Error | null>(null);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [touchedFields, setTouchedFields] = useState<Record<string, boolean>>({});
  const [isFormValid, setIsFormValid] = useState(true);
  const [isStorageReady, setIsStorageReady] = useState(false);
  const challengeTokenRef: any = useRef<string | null>(null);

  const tokenValidationAttemptedRef: any = useRef(false);

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
    const storageManager: any = await getStorageManager();
    if (storageManager) {
      if (challengeToken) {
        await storageManager.setTemporaryDataParameter('challengeToken', challengeToken);
      } else {
        await storageManager.removeTemporaryDataParameter('challengeToken');
      }
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
        error?.failureReason || extractErrorMessage(error, t, 'components.acceptInvite.errors.generic');

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
    (response: AcceptInviteFlowResponse): AcceptInviteFlowResponse => {
      if (!response?.data?.meta?.components) {
        return response;
      }

      try {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {
            defaultErrorKey: 'components.acceptInvite.errors.generic',
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
   * Handle OAuth callback when returning from OAuth provider.
   * This hook processes the authorization code and continues the flow.
   */
  useOAuthCallback({
    currentExecutionId: executionId ?? null,
    isInitialized: isStorageReady,
    onComplete: () => {
      setIsValidatingToken(false);
      onComplete?.();
    },
    onError: (error: any) => {
      if (!error?.flowStatus) {
        setIsTokenInvalid(true);
      }
      setIsValidatingToken(false);
      handleError(error);
    },
    onFlowChange: (response: any) => {
      onFlowChange?.(response);
      if (response.flowStatus === 'COMPLETE') {
        setIsComplete(true);
        const completionComponents: any[] = response.data?.components || response.data?.meta?.components || [];
        if (completionComponents.length > 0) {
          setCurrentFlow(response);
        }
      } else {
        // Initialize currentFlow for next steps if not complete
        setCurrentFlow(response);
        setFormValues({});
        setFormErrors({});
        setTouchedFields({});
      }
    },
    onProcessingStart: () => {
      setIsValidatingToken(true);
    },
    onSubmit: async (payload: any) => {
      const rawResponse: any = await onSubmit({
        ...payload,
        ...(challengeTokenRef.current ? {challengeToken: challengeTokenRef.current} : {}),
      });
      const response: any = normalizeFlowResponseLocal(rawResponse);
      await setChallengeToken(response.challengeToken ?? null);
      return response;
    },
    tokenValidationAttemptedRef,
  });

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
    setIsFormValid(true);
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
            (comp.type === 'PASSWORD_INPUT' ||
              comp.type === 'TEXT_INPUT' ||
              comp.type === 'EMAIL_INPUT' ||
              comp.type === 'PHONE_INPUT' ||
              comp.type === 'OTP_INPUT') &&
            comp.required &&
            comp.ref
          ) {
            const value: any = formValues[comp.ref];
            if (!value || value.trim() === '') {
              errors[comp.ref] = t('validations.required.field.error');
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
    [formValues, t],
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
        setIsFormValid(false);
        setFormErrors(validation.errors);
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

        // Handle OAuth redirect response
        if (response.type === 'REDIRECTION') {
          const redirectURL: any = response.data?.redirectURL || response?.redirectURL;
          if (redirectURL && typeof window !== 'undefined') {
            // Initiate OAuth redirect with secure state management
            initiateOAuthRedirect(redirectURL);
            return;
          }
        }

        // Check for completion
        if (response.flowStatus === 'COMPLETE') {
          setIsComplete(true);
          const completionComponents: any[] = response.data?.components || response.data?.meta?.components || [];
          if (completionComponents.length > 0) {
            setCurrentFlow(response);
          } else {
            onComplete?.();
          }
          return;
        }

        // Check for error status
        if (response.flowStatus === 'ERROR') {
          handleError(response);
          return;
        }

        // Update current flow and reset form state for next step, preserving input values
        setCurrentFlow(response);
        setFormErrors({});
        setTouchedFields({});
      } catch (err) {
        handleError(err);
      } finally {
        setIsLoading(false);
      }
    },
    [
      currentFlow,
      formValues,
      validateForm,
      onSubmit,
      onFlowChange,
      onComplete,
      handleError,
      normalizeFlowResponseLocal,
    ],
  );

  /**
   * Validate invite token on component mount.
   */
  useEffect(() => {
    // Skip validation if already validated
    if (tokenValidationAttemptedRef.current) {
      return;
    }

    // If an OAuth code is present, this is a return from an OAuth redirect.
    // useOAuthCallback will handle the code — skip the initial invite validation.
    const urlParams: URLSearchParams = new URLSearchParams(window.location.search);
    if (urlParams.get('code')) {
      return;
    }

    // Validate required params for initial invite link
    if (!executionId || !inviteToken) {
      setIsValidatingToken(false);
      setIsTokenInvalid(true);
      handleError(new Error('Invalid invite link. Missing executionId or inviteToken.'));
      return;
    }

    tokenValidationAttemptedRef.current = true;

    (async (): Promise<void> => {
      setIsValidatingToken(true);
      setApiError(null);

      try {
        // Store executionId in sessionStorage for OAuth callback
        if (executionId) {
          sessionStorage.setItem('thunderid_execution_id', executionId);
        }

        // Send the invite token to validate and continue the flow
        const payload: any = {
          executionId,
          inputs: {
            inviteToken,
          },
          verbose: true,
        };

        const rawResponse: any = await onSubmit(payload);
        const response: any = normalizeFlowResponseLocal(rawResponse);
        onFlowChange?.(response);

        await setChallengeToken(response.challengeToken ?? null);

        // Check for error (invalid token)
        if (response.flowStatus === 'ERROR') {
          setIsTokenInvalid(true);
          handleError(response);
          return;
        }

        // Token is valid, show the password form
        setCurrentFlow(response);
      } catch (err) {
        setIsTokenInvalid(true);
        handleError(err);
      } finally {
        setIsValidatingToken(false);
      }
    })();
  }, [executionId, inviteToken, onSubmit, onFlowChange, handleError, normalizeFlowResponseLocal]);

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

  // Get components from normalized response, with fallback to meta.components
  const components: any = currentFlow?.data?.components || currentFlow?.data?.meta?.components || [];
  const {title, subtitle} = extractHeadings(components);
  const componentsWithoutHeadings: any = filterHeadings(components);

  // Render props
  const renderProps: BaseAcceptInviteRenderProps = {
    components,
    error: apiError,
    executionId,
    fieldErrors: formErrors,
    goToSignIn: onGoToSignIn,
    handleInputBlur,
    handleInputChange,
    handleSubmit,
    inviteToken,
    isComplete,
    isLoading,
    isTokenInvalid,
    isValid: isFormValid,
    isValidatingToken,
    meta,
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

  // Loading state during token validation
  if (isValidatingToken) {
    return (
      <CardPrimitive className={cx(className, styles.card)} variant={variant}>
        <CardPrimitive.Content>
          <div style={{alignItems: 'center', display: 'flex', flexDirection: 'column', gap: '1rem', padding: '2rem'}}>
            <Spinner size="medium" />
            <Typography variant="body1">Validating your invite link...</Typography>
          </div>
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // Invalid token state
  if (isTokenInvalid) {
    return (
      <CardPrimitive className={cx(className, styles.card)} variant={variant}>
        <CardPrimitive.Header className={styles.header}>
          <CardPrimitive.Title level={2} className={styles.title}>
            Invalid Invite Link
          </CardPrimitive.Title>
        </CardPrimitive.Header>
        <CardPrimitive.Content>
          <AlertPrimitive variant="error">
            <AlertPrimitive.Title>Unable to verify invite</AlertPrimitive.Title>
            <AlertPrimitive.Description>
              {apiError?.message ||
                'This invite link is invalid or has expired. Please contact your administrator for a new invite.'}
            </AlertPrimitive.Description>
          </AlertPrimitive>
          {onGoToSignIn && (
            <div style={{display: 'flex', justifyContent: 'center', marginTop: '1.5rem'}}>
              <Button variant="outline" onClick={onGoToSignIn}>
                Go to Sign In
              </Button>
            </div>
          )}
        </CardPrimitive.Content>
      </CardPrimitive>
    );
  }

  // Flow components (password form)
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
            ? renderInviteUserComponents(
                componentsWithoutHeadings,
                formValues,
                touchedFields,
                formErrors,
                isLoading,
                isFormValid,
                handleInputChange,
                {
                  _customRenderers: customRenderers,
                  _theme: theme,
                  onInputBlur: handleInputBlur,
                  onSubmit: handleSubmit,
                  size,
                  variant,
                },
              )
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
        {onGoToSignIn && (
          <div style={{marginTop: '1.5rem', textAlign: 'center'}}>
            <Typography variant="body2">
              Already have an account?{' '}
              <Button variant="text" onClick={onGoToSignIn} style={{minWidth: 'auto', padding: 0}}>
                Sign In
              </Button>
            </Typography>
          </div>
        )}
      </CardPrimitive.Content>
    </CardPrimitive>
  );
};

export default BaseAcceptInvite;
