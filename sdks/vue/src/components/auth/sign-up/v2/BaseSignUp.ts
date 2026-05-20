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

import {
  EmbeddedFlowComponentTypeV2 as EmbeddedFlowComponentType,
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowResponseType,
  EmbeddedFlowStatus,
  FlowMetadataResponse,
  withVendorCSSClassPrefix,
} from '@thunderid/browser';
import {
  type Component,
  type PropType,
  type Ref,
  type SetupContext,
  type VNode,
  defineComponent,
  h,
  ref,
  watch,
} from 'vue';
import useFlowMeta from '../../../../composables/useFlowMeta';
import useI18n from '../../../../composables/useI18n';
import {createVueLogger} from '../../../../utils/logger';
import {normalizeFlowResponse, extractErrorMessage} from '../../../../utils/v2/flowTransformer';
import getAuthComponentHeadings from '../../../../utils/v2/getAuthComponentHeadings';
import {handlePasskeyRegistration} from '../../../../utils/v2/passkey';
import Alert from '../../../primitives/Alert';
import Card from '../../../primitives/Card';
import Spinner from '../../../primitives/Spinner';
import Typography from '../../../primitives/Typography';
import {renderSignUpComponents} from '../../sign-in/AuthOptionFactory';

const logger: ReturnType<typeof createVueLogger> = createVueLogger('BaseSignUp');

/**
 * Passkey registration tracking state.
 */
interface PasskeyState {
  actionId: string | null;
  creationOptions: string | null;
  error: Error | null;
  flowId: string | null;
  isActive: boolean;
}

/**
 * Render props passed to the default scoped slot.
 */
export interface BaseSignUpRenderProps {
  components: any[];
  error?: Error | null;
  fieldErrors: Record<string, string>;
  handleInputChange: (name: string, value: string) => void;
  handleSubmit: (component: any, data?: Record<string, any>) => Promise<void>;
  isLoading: boolean;
  isValid: boolean;
  messages: {message: string; type: string}[];
  subtitle: string;
  title: string;
  touched: Record<string, boolean>;
  validateForm: () => {fieldErrors: Record<string, string>; isValid: boolean};
  values: Record<string, string>;
}

export interface BaseSignUpProps {
  afterSignUpUrl?: string;
  buttonClassName?: string;
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
  shouldRedirectAfterSignUp?: boolean;
  showLogo?: boolean;
  showSubtitle?: boolean;
  showTitle?: boolean;
  size?: 'small' | 'medium' | 'large';
  variant?: 'elevated' | 'outlined' | 'flat';
}

interface FieldDefinition {
  name: string;
  required: boolean;
  type: string;
}

const extractFormFields = (components: any[]): FieldDefinition[] => {
  const fields: FieldDefinition[] = [];
  const process = (comps: any[]): void => {
    comps.forEach((c: any) => {
      if (
        c.type === EmbeddedFlowComponentType.TextInput ||
        c.type === EmbeddedFlowComponentType.PasswordInput ||
        c.type === EmbeddedFlowComponentType.EmailInput ||
        c.type === EmbeddedFlowComponentType.Select
      ) {
        const fieldName: string = c.ref || c.id;
        fields.push({name: fieldName, required: c.required || false, type: c.type});
      }
      if (c.components && Array.isArray(c.components)) {
        process(c.components);
      }
    });
  };
  process(components);
  return fields;
};

/**
 * BaseSignUp — app-native sign-up presentation component.
 *
 * Manages the sign-up flow lifecycle including initialization, form state,
 * passkey registration, popup-based social OAuth, and renders the server-driven UI.
 */
const BaseSignUp: Component = defineComponent({
  name: 'BaseSignUp',
  props: {
    afterSignUpUrl: {default: undefined, type: String},
    buttonClassName: {default: '', type: String},
    className: {default: '', type: String},
    error: {default: null, type: Object as PropType<Error | null>},
    errorClassName: {default: '', type: String},
    inputClassName: {default: '', type: String},
    isInitialized: {default: false, type: Boolean},
    messageClassName: {default: '', type: String},
    onComplete: {default: undefined, type: Function as PropType<(response: EmbeddedFlowExecuteResponse) => void>},
    onError: {default: undefined, type: Function as PropType<(error: Error) => void>},
    onFlowChange: {
      default: undefined,
      type: Function as PropType<(response: EmbeddedFlowExecuteResponse) => void>,
    },
    onInitialize: {
      default: undefined,
      type: Function as PropType<(payload?: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>>,
    },
    onSubmit: {
      default: undefined,
      type: Function as PropType<(payload: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse>>,
    },
    showSubtitle: {default: true, type: Boolean},
    showTitle: {default: true, type: Boolean},
    size: {
      default: 'medium',
      type: String as PropType<'small' | 'medium' | 'large'>,
    },
    variant: {
      default: 'outlined',
      type: String as PropType<'elevated' | 'outlined' | 'flat'>,
    },
  },
  emits: ['error', 'complete', 'flowChange'],
  setup(props: any, {slots}: SetupContext): () => VNode | null {
    const {meta: flowMetaRef} = useFlowMeta();
    const {t} = useI18n();

    // ── State ──
    const isLoading: Ref<boolean> = ref(false);
    const isFlowInitialized: Ref<boolean> = ref(false);
    const currentFlow: Ref<EmbeddedFlowExecuteResponse | null> = ref(null);
    const apiError: Ref<Error | null> = ref(null);
    const flowMessages: Ref<{message: string; type: string}[]> = ref([]);
    const passkeyState: Ref<PasskeyState> = ref({
      actionId: null,
      creationOptions: null,
      error: null,
      flowId: null,
      isActive: false,
    });

    // Form state
    const formValues: Ref<Record<string, string>> = ref({});
    const touchedFields: Ref<Record<string, boolean>> = ref({});
    const formErrors: Ref<Record<string, string>> = ref({});
    const isFormValid: Ref<boolean> = ref(true);

    // One-time flags (plain mutable, not reactive)
    let initializationAttempted = false;
    let passkeyProcessed = false;

    // ── Helpers ──

    const handleError = (error: any): void => {
      const errorMessage: string = error?.failureReason || extractErrorMessage(error, t);
      apiError.value = error instanceof Error ? error : new Error(errorMessage);
      flowMessages.value = [{message: errorMessage, type: 'error'}];
    };

    const normalizeFlowResponseLocal = (response: EmbeddedFlowExecuteResponse): EmbeddedFlowExecuteResponse => {
      if (response?.data?.components && Array.isArray(response.data.components)) {
        return response;
      }
      if (response?.data) {
        const {components} = normalizeFlowResponse(
          response,
          t,
          {defaultErrorKey: 'components.signUp.errors.generic', resolveTranslations: false},
          (flowMetaRef as Ref<FlowMetadataResponse | null>).value,
        );
        return {...response, data: {...response.data, components: components as any}};
      }
      return response;
    };

    const setupFormFields = (flowResponse: EmbeddedFlowExecuteResponse): void => {
      const fields: FieldDefinition[] = extractFormFields(flowResponse.data?.components || []);
      const initialValues: Record<string, string> = {};
      fields.forEach((f: FieldDefinition) => {
        initialValues[f.name] = '';
      });
      formValues.value = initialValues;
      touchedFields.value = {};
      formErrors.value = {};
      isFormValid.value = true;
    };

    const computeFormErrors = (): Record<string, string> => {
      const components: any[] = currentFlow.value?.data?.components || [];
      const fields: FieldDefinition[] = extractFormFields(components);
      const errors: Record<string, string> = {};
      fields.forEach((field: FieldDefinition) => {
        const value: string = formValues.value[field.name] || '';
        if (field.required && (!value || value.trim() === '')) {
          errors[field.name] = t('validations.required.field.error') || 'This field is required';
        }
        if (
          (field.type === EmbeddedFlowComponentType.EmailInput || field.type === 'EMAIL') &&
          value &&
          !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
        ) {
          errors[field.name] = t('field.email.invalid') || 'Invalid email address';
        }
      });
      return errors;
    };

    const touchAllFields = (): void => {
      const fields: FieldDefinition[] = extractFormFields(currentFlow.value?.data?.components || []);
      const newTouched: Record<string, boolean> = {};
      fields.forEach((f: FieldDefinition) => {
        newTouched[f.name] = true;
      });
      touchedFields.value = newTouched;
    };

    const validateForm = (): {fieldErrors: Record<string, string>; isValid: boolean} => {
      touchAllFields();
      const errors: Record<string, string> = computeFormErrors();
      formErrors.value = errors;
      const valid: boolean = Object.keys(errors).length === 0;
      isFormValid.value = valid;
      return {fieldErrors: errors, isValid: valid};
    };

    // ── Input handlers ──

    const handleInputChange = (name: string, value: string): void => {
      formValues.value = {...formValues.value, [name]: value};
    };

    const handleInputBlur = (name: string): void => {
      touchedFields.value = {...touchedFields.value, [name]: true};
    };

    // ── Popup OAuth for social sign-up ──

    const handleRedirectionIfNeeded = (response: EmbeddedFlowExecuteResponse): boolean => {
      if (response?.type !== EmbeddedFlowResponseType.Redirection || !response?.data?.redirectURL) {
        return false;
      }

      const redirectUrl: string = response.data.redirectURL;
      const popup: Window | null = window.open(
        redirectUrl,
        'oauth_popup',
        'width=500,height=600,scrollbars=yes,resizable=yes',
      );

      if (!popup) {
        logger.error('Failed to open popup window');
        return false;
      }

      let hasProcessedCallback = false;
      let popupMonitor: ReturnType<typeof setInterval> | null = null;
      let messageHandler: ((event: MessageEvent) => Promise<void>) | null = null;

      const cleanup = (): void => {
        if (messageHandler) window.removeEventListener('message', messageHandler);
        if (popupMonitor) clearInterval(popupMonitor);
      };

      const processOAuthCode = async (code: string, state: string): Promise<void> => {
        const payload: EmbeddedFlowExecuteRequestPayload = {
          ...(currentFlow.value?.flowId && {flowId: currentFlow.value.flowId}),
          action: '',
          flowType: (currentFlow.value as any)?.flowType || 'REGISTRATION',
          inputs: {code, state},
        } as any;

        try {
          const continueResponse: EmbeddedFlowExecuteResponse = await props.onSubmit(payload);
          props.onFlowChange?.(continueResponse);

          if (continueResponse.flowStatus === EmbeddedFlowStatus.Complete) {
            props.onComplete?.(continueResponse);
          } else if (continueResponse.flowStatus === EmbeddedFlowStatus.Incomplete) {
            currentFlow.value = continueResponse;
            setupFormFields(continueResponse);
          }
          popup.close();
          cleanup();
        } catch (err) {
          handleError(err);
          props.onError?.(err as Error);
          popup.close();
          cleanup();
        }
      };

      messageHandler = async (event: MessageEvent): Promise<void> => {
        if (event.source !== popup) return;
        const expectedOrigin: string = props.afterSignUpUrl
          ? new URL(props.afterSignUpUrl).origin
          : window.location.origin;
        if (event.origin !== expectedOrigin && event.origin !== window.location.origin) return;
        const {code, state} = event.data;
        if (code && state) {
          await processOAuthCode(code, state);
        }
      };

      window.addEventListener('message', messageHandler);

      popupMonitor = setInterval(async () => {
        try {
          if (popup.closed) {
            cleanup();
            return;
          }
          if (hasProcessedCallback) return;
          try {
            const popupUrl: string = popup.location.href;
            if (popupUrl && (popupUrl.includes('code=') || popupUrl.includes('error='))) {
              hasProcessedCallback = true;
              const url: URL = new URL(popupUrl);
              const code: string | null = url.searchParams.get('code');
              const state: string | null = url.searchParams.get('state');
              const error: string | null = url.searchParams.get('error');

              if (error) {
                logger.error('OAuth error');
                popup.close();
                cleanup();
                return;
              }
              if (code && state) {
                await processOAuthCode(code, state);
              }
            }
          } catch {
            // Cross-origin error expected during OAuth redirect
          }
        } catch {
          logger.error('Error monitoring popup');
        }
      }, 1000);

      return true;
    };

    // ── Submit handler ──

    const handleSubmit = async (
      component: any,
      data?: Record<string, any>,
      skipValidation?: boolean,
    ): Promise<void> => {
      if (!currentFlow.value) return;

      if (!skipValidation) {
        const validation: {fieldErrors: Record<string, string>; isValid: boolean} = validateForm();
        if (!validation.isValid) return;
      }

      isLoading.value = true;
      apiError.value = null;
      flowMessages.value = [];

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
          ...(currentFlow.value.flowId && {flowId: currentFlow.value.flowId}),
          flowType: (currentFlow.value as any).flowType || 'REGISTRATION',
          ...(component.id && {action: component.id}),
          inputs: filteredInputs,
        } as any;

        const rawResponse: EmbeddedFlowExecuteResponse = await props.onSubmit(payload);
        const response: EmbeddedFlowExecuteResponse = normalizeFlowResponseLocal(rawResponse);
        props.onFlowChange?.(response);

        if (response.flowStatus === EmbeddedFlowStatus.Complete) {
          props.onComplete?.(response);
          return;
        }

        if (response.flowStatus === EmbeddedFlowStatus.Incomplete) {
          if (handleRedirectionIfNeeded(response)) return;

          if (response.data?.additionalData?.['passkeyCreationOptions']) {
            const {passkeyCreationOptions} = response.data.additionalData as any;
            const effectiveFlowId: string | undefined = response.flowId || currentFlow.value?.flowId;
            passkeyProcessed = false;
            passkeyState.value = {
              actionId: component.id || 'submit',
              creationOptions: passkeyCreationOptions,
              error: null,
              flowId: effectiveFlowId || null,
              isActive: true,
            };
            isLoading.value = false;
            return;
          }

          currentFlow.value = response;
          setupFormFields(response);
        }
      } catch (err) {
        handleError(err);
        props.onError?.(err as Error);
      } finally {
        isLoading.value = false;
      }
    };

    // ── Passkey registration watch ──

    watch(
      () => passkeyState.value,
      async (state: PasskeyState) => {
        if (!state.isActive || !state.creationOptions || !state.flowId) return;
        if (passkeyProcessed) return;
        passkeyProcessed = true;

        try {
          const passkeyResponse: string = await handlePasskeyRegistration(state.creationOptions);
          const passkeyObj: any = JSON.parse(passkeyResponse);
          const inputs: Record<string, string> = {
            attestationObject: passkeyObj.response.attestationObject,
            clientDataJSON: passkeyObj.response.clientDataJSON,
            credentialId: passkeyObj.id,
          };

          const payload: EmbeddedFlowExecuteRequestPayload = {
            actionId: state.actionId || 'submit',
            flowId: state.flowId,
            flowType: (currentFlow.value as any)?.flowType || 'REGISTRATION',
            inputs,
          } as any;

          const nextResponse: EmbeddedFlowExecuteResponse = await props.onSubmit(payload);
          const processed: EmbeddedFlowExecuteResponse = normalizeFlowResponseLocal(nextResponse);
          props.onFlowChange?.(processed);

          if (processed.flowStatus === EmbeddedFlowStatus.Complete) {
            props.onComplete?.(processed);
          } else {
            currentFlow.value = processed;
            setupFormFields(processed);
          }

          passkeyState.value = {actionId: null, creationOptions: null, error: null, flowId: null, isActive: false};
        } catch (error: unknown) {
          passkeyState.value = {...passkeyState.value, error: error as Error, isActive: false};
          handleError(error);
          props.onError?.(error as Error);
        }
      },
      {deep: true},
    );

    // ── Flow initialization ──

    watch(
      () => [props.isInitialized, isFlowInitialized.value] as [boolean, boolean],
      ([initialized, flowInit]: [boolean, boolean]) => {
        // Skip if URL has OAuth code params
        const urlParams: URLSearchParams = new URL(window.location.href).searchParams;
        if (urlParams.get('code') || urlParams.get('state')) return;

        if (initialized && !flowInit && !initializationAttempted) {
          initializationAttempted = true;

          (async (): Promise<void> => {
            isLoading.value = true;
            apiError.value = null;
            flowMessages.value = [];

            try {
              const rawResponse: EmbeddedFlowExecuteResponse = await props.onInitialize();
              const response: EmbeddedFlowExecuteResponse = normalizeFlowResponseLocal(rawResponse);
              currentFlow.value = response;
              isFlowInitialized.value = true;
              props.onFlowChange?.(response);

              if (response.flowStatus === EmbeddedFlowStatus.Complete) {
                props.onComplete?.(response);
                return;
              }
              if (response.flowStatus === EmbeddedFlowStatus.Incomplete) {
                setupFormFields(response);
              }
            } catch (err) {
              handleError(err);
              props.onError?.(err as Error);
            } finally {
              isLoading.value = false;
            }
          })();
        }
      },
      {immediate: true},
    );

    // ── Render ──

    return (): VNode | null => {
      const containerClass: string = [
        withVendorCSSClassPrefix('signup'),
        withVendorCSSClassPrefix(`signup--${props.size}`),
        withVendorCSSClassPrefix(`signup--${props.variant}`),
        props.className,
      ]
        .filter(Boolean)
        .join(' ');

      // Scoped slot / render props
      if (slots['default']) {
        const renderProps: BaseSignUpRenderProps = {
          components: currentFlow.value?.data?.components || [],
          error: apiError.value,
          fieldErrors: formErrors.value,
          handleInputChange,
          handleSubmit,
          isLoading: isLoading.value,
          isValid: isFormValid.value,
          messages: flowMessages.value,
          subtitle: t('signup.subheading') || 'Create your account',
          title: t('signup.heading') || 'Sign Up',
          touched: touchedFields.value,
          validateForm: (): {fieldErrors: Record<string, string>; isValid: boolean} => {
            const result: {fieldErrors: Record<string, string>; isValid: boolean} = validateForm();
            return {fieldErrors: result.fieldErrors, isValid: result.isValid};
          },
          values: formValues.value,
        };
        return h('div', {class: containerClass}, slots['default'](renderProps));
      }

      // Loading state
      if (!isFlowInitialized.value && isLoading.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h('div', {style: 'display:flex;justify-content:center;padding:2rem'}, h(Spinner)),
        );
      }

      // No flow available
      if (!currentFlow.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h(
            Alert,
            {variant: 'error'},
            () => t('errors.signup.flow.initialization.failure') || 'Failed to initialize sign-up flow',
          ),
        );
      }

      // Extract headings
      const componentsToRender: any[] = currentFlow.value.data?.components || [];
      const {title, subtitle, componentsWithoutHeadings} = getAuthComponentHeadings(
        componentsToRender,
        undefined,
        undefined,
        t('signup.heading') || 'Sign Up',
        t('signup.subheading') || 'Create your account',
      );

      const meta: FlowMetadataResponse | null = (flowMetaRef as Ref<FlowMetadataResponse | null>).value;

      const renderedComponents: VNode[] =
        componentsWithoutHeadings.length > 0
          ? renderSignUpComponents(
              componentsWithoutHeadings,
              formValues.value,
              touchedFields.value,
              formErrors.value,
              isLoading.value,
              isFormValid.value,
              handleInputChange,
              {
                buttonClassName: props.buttonClassName,
                inputClassName: props.inputClassName,
                meta,
                onInputBlur: handleInputBlur,
                onSubmit: handleSubmit,
                size: props.size,
                t,
                variant: props.variant,
              },
            )
          : [];

      return h(Card, {class: containerClass, variant: props.variant}, () => [
        // Header with title/subtitle
        props.showTitle || props.showSubtitle
          ? h('div', {style: 'padding: 1rem 1rem 0'}, [
              props.showTitle ? h(Typography, {variant: 'h5'}, () => title) : null,
              props.showSubtitle
                ? h(Typography, {style: 'margin-top: 0.25rem', variant: 'body1'}, () => subtitle)
                : null,
            ])
          : null,
        // External error
        props.error
          ? h(
              'div',
              {style: 'padding: 0 1rem'},
              h(Alert, {variant: 'error'}, () => props.error.message),
            )
          : null,
        // Flow messages
        flowMessages.value.length > 0
          ? h(
              'div',
              {style: 'padding: 0 1rem'},
              flowMessages.value.map((msg: {message: string; type: string}, i: number) =>
                h(Alert, {key: i, variant: msg.type === 'error' ? 'error' : 'info'}, () => msg.message),
              ),
            )
          : null,
        // Components
        h(
          'div',
          {style: 'padding: 1rem'},
          renderedComponents.length > 0
            ? renderedComponents
            : [
                h(
                  Alert,
                  {variant: 'warning'},
                  () => t('errors.signup.components.not.available') || 'No components available',
                ),
              ],
        ),
      ]);
    };
  },
});

export default BaseSignUp;
