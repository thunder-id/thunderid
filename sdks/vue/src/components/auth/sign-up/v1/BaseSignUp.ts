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
  EmbeddedFlowComponent,
  EmbeddedFlowComponentType,
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowResponseType,
  EmbeddedFlowStatus,
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
import {renderSignUpComponents} from './options/SignUpOptionFactory';
import useFlow from '../../../../composables/useFlow';
import useI18n from '../../../../composables/useI18n';
import {createVueLogger} from '../../../../utils/logger';
import Alert from '../../../primitives/Alert';
import Card from '../../../primitives/Card';
import Logo from '../../../primitives/Logo';
import Spinner from '../../../primitives/Spinner';
import Typography from '../../../primitives/Typography';

const logger: ReturnType<typeof createVueLogger> = createVueLogger('BaseSignUpV1');

/**
 * Render-prop payload exposed via the default slot.
 */
export interface BaseSignUpRenderProps {
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
  values: Record<string, string>;
}

/**
 * V1 BaseSignUp — component-driven app-native sign-up for Vue.
 *
 * Mirrors `packages/react/.../SignUp/v1/BaseSignUp.tsx`. Reads the
 * `/api/server/v1/flow/execute` response shape (`TYPOGRAPHY`, `FORM`, `INPUT`,
 * `BUTTON`, `RICH_TEXT`, etc.) and renders it via the V1
 * `SignUpOptionFactory`. Tracks form state internally and submits steps via
 * the `onSubmit` prop until the flow completes.
 */
const BaseSignUp: Component = defineComponent({
  name: 'BaseSignUpV1',
  props: {
    afterSignUpUrl: {default: undefined, type: String},
    buttonClassName: {default: '', type: String},
    className: {default: '', type: String},
    errorClassName: {default: '', type: String},
    inputClassName: {default: '', type: String},
    isInitialized: {default: true, type: Boolean},
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
    shouldRedirectAfterSignUp: {default: true, type: Boolean},
    showLogo: {default: true, type: Boolean},
    showSubtitle: {default: true, type: Boolean},
    showTitle: {default: true, type: Boolean},
    size: {default: 'medium', type: String as PropType<'small' | 'medium' | 'large'>},
    variant: {default: 'outlined', type: String as PropType<'elevated' | 'outlined' | 'flat'>},
  },
  emits: ['error', 'flowChange', 'complete'],
  setup(props: any, {slots, emit}: SetupContext): () => VNode | null {
    const {t} = useI18n();
    const {title: flowTitle, subtitle: flowSubtitle, messages: flowMessages, addMessage, clearMessages} = useFlow();

    // ── Reactive state ──
    const isLoading: Ref<boolean> = ref(false);
    const isFlowInitialized: Ref<boolean> = ref(false);
    const currentFlow: Ref<EmbeddedFlowExecuteResponse | null> = ref(null);
    const formValues: Ref<Record<string, string>> = ref({});
    const touchedFields: Ref<Record<string, boolean>> = ref({});
    const formErrors: Ref<Record<string, string>> = ref({});

    let initializationAttempted = false;

    // ── Error handling ──

    const handleError = (err: any): void => {
      let errorMessage: string = t('errors.signup.flow.failure') || 'Sign-up failed';

      if (err && typeof err === 'object') {
        if (err.code && (err.message || err.description)) {
          errorMessage = err.description || err.message;
        } else if (err.message) {
          errorMessage = err.message;
        }
      } else if (typeof err === 'string') {
        errorMessage = err;
      }

      clearMessages();
      addMessage({message: errorMessage, type: 'error'});
    };

    // ── Form helpers ──

    /**
     * Walk the V1 component tree and collect every INPUT's bound parameter
     * name. The parameter name comes from `config.identifier` (a SCIM claim
     * URI) or `config.name`, falling back to the component id.
     */
    const collectInputNames = (components: EmbeddedFlowComponent[]): string[] => {
      const names: string[] = [];
      const walk = (comps: EmbeddedFlowComponent[]): void => {
        comps.forEach((component: EmbeddedFlowComponent) => {
          const cfg: any = (component as any).config || {};
          if (component.type === EmbeddedFlowComponentType.Input) {
            const name: string = (cfg.name as string) || (cfg.identifier as string) || component.id;
            if (name) names.push(name);
          }
          const children: EmbeddedFlowComponent[] = (component as any).components || [];
          if (children.length > 0) walk(children);
        });
      };
      walk(components);
      return names;
    };

    const setupFormFields = (response: EmbeddedFlowExecuteResponse): void => {
      const componentTree: EmbeddedFlowComponent[] = response.data?.components || [];
      const names: string[] = collectInputNames(componentTree);
      const initial: Record<string, string> = {};
      names.forEach((name: string) => {
        initial[name] = '';
      });
      formValues.value = initial;
      touchedFields.value = {};
      formErrors.value = {};
    };

    const handleInputChange = (name: string, value: string): void => {
      formValues.value = {...formValues.value, [name]: value};
      touchedFields.value = {...touchedFields.value, [name]: true};
      // Clear any prior error on input
      if (formErrors.value[name]) {
        const next: Record<string, string> = {...formErrors.value};
        delete next[name];
        formErrors.value = next;
      }
    };

    const isFormValid = (): boolean => Object.keys(formErrors.value).length === 0;

    /**
     * Mirror the React V1 popup-based redirection handler for social/IdP
     * registration steps. Opens a popup, waits for the OAuth code, and submits
     * `{code, state}` as the next flow step.
     *
     * Returns `true` if redirection was handled (caller should not fall
     * through), `false` otherwise.
     */
    const handleRedirectionIfNeeded = (response: EmbeddedFlowExecuteResponse): boolean => {
      if (response?.type !== EmbeddedFlowResponseType.Redirection || !(response as any)?.data?.redirectURL) {
        return false;
      }
      if (typeof window === 'undefined') return false;

      const redirectUrl: string = (response as any).data.redirectURL;
      const popup: Window | null = window.open(
        redirectUrl,
        'oauth_popup',
        'width=500,height=600,scrollbars=yes,resizable=yes',
      );

      if (!popup) {
        logger.error('Failed to open popup window for social sign-up redirect');
        return false;
      }

      let processed = false;
      let popupMonitor: ReturnType<typeof setInterval> | undefined;
      let messageHandler: (event: MessageEvent) => Promise<void>;

      const cleanup = (): void => {
        window.removeEventListener('message', messageHandler);
        if (popupMonitor) clearInterval(popupMonitor);
      };

      const continueWithCode = async (code: string, state: string): Promise<void> => {
        const payload: any = {
          ...(currentFlow.value?.flowId && {flowId: currentFlow.value.flowId}),
          actionId: '',
          flowType: ((currentFlow.value as any)?.flowType as string) || 'REGISTRATION',
          inputs: {code, state},
        };
        try {
          const next: any = await props.onSubmit(payload);
          props.onFlowChange?.(next);
          emit('flowChange', next);
          if (next.flowStatus === EmbeddedFlowStatus.Complete) {
            props.onComplete?.(next);
            emit('complete', next);
          } else if (next.flowStatus === EmbeddedFlowStatus.Incomplete) {
            currentFlow.value = next;
            setupFormFields(next);
          }
        } catch (err) {
          handleError(err);
          props.onError?.(err as Error);
          emit('error', err);
        } finally {
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
        const {code, state} = (event.data || {}) as {code?: string; state?: string};
        if (code && state && !processed) {
          processed = true;
          await continueWithCode(code, state);
        }
      };
      window.addEventListener('message', messageHandler);

      popupMonitor = setInterval(async () => {
        try {
          if (popup.closed) {
            cleanup();
            return;
          }
          if (processed) return;

          // Same-origin URL inspection. Throws on cross-origin (expected while
          // the popup is on the IdP's domain) — swallow and try next tick.
          let popupUrl: string | undefined;
          try {
            popupUrl = popup.location.href;
          } catch {
            return;
          }
          if (!popupUrl) return;

          if (popupUrl.includes('code=') || popupUrl.includes('error=')) {
            const url: URL = new URL(popupUrl);
            const code: string | null = url.searchParams.get('code');
            const state: string | null = url.searchParams.get('state');
            const error: string | null = url.searchParams.get('error');
            if (error) {
              processed = true;
              logger.error(`OAuth error during social sign-up: ${error}`);
              popup.close();
              cleanup();
              return;
            }
            if (code && state) {
              processed = true;
              await continueWithCode(code, state);
            }
          }
        } catch (err) {
          logger.error('Error monitoring sign-up popup');
        }
      }, 1000);

      return true;
    };

    // ── Step submission ──

    const handleSubmit = async (component: any, data?: Record<string, any>): Promise<void> => {
      if (!currentFlow.value) return;

      isLoading.value = true;
      clearMessages();

      try {
        const filteredInputs: Record<string, any> = {};
        // Prefer explicit `data` if the component handler passes it; otherwise
        // submit the entire form snapshot. Empty strings are stripped to match
        // the React V1 behaviour.
        const sourceInputs: Record<string, any> = data ?? formValues.value;
        Object.entries(sourceInputs).forEach(([key, value]: [string, any]) => {
          if (value !== null && value !== undefined && value !== '') {
            filteredInputs[key] = value;
          }
        });

        const actionId: string | undefined = component?.actionId || component?.id;

        const payload: any = {
          ...(currentFlow.value.flowId && {flowId: currentFlow.value.flowId}),
          flowType: ((currentFlow.value as any).flowType as string) || 'REGISTRATION',
          inputs: filteredInputs,
          ...(actionId && {actionId}),
        };

        const response: any = await props.onSubmit(payload);
        props.onFlowChange?.(response);
        emit('flowChange', response);

        if (response?.flowStatus === EmbeddedFlowStatus.Complete) {
          props.onComplete?.(response);
          emit('complete', response);
          return;
        }

        if (response?.flowStatus === EmbeddedFlowStatus.Incomplete) {
          if (handleRedirectionIfNeeded(response)) return;
          currentFlow.value = response;
          setupFormFields(response);
        }
      } catch (err) {
        handleError(err);
        props.onError?.(err as Error);
        emit('error', err);
      } finally {
        isLoading.value = false;
      }
    };

    // ── Flow initialization ──

    watch(
      () => [props.isInitialized, isFlowInitialized.value] as [boolean, boolean],
      ([initialized, flowInit]: [boolean, boolean]) => {
        if (!initialized || flowInit || initializationAttempted) return;
        if (!props.onInitialize) return;
        initializationAttempted = true;

        (async (): Promise<void> => {
          isLoading.value = true;
          clearMessages();
          try {
            const response: EmbeddedFlowExecuteResponse = await props.onInitialize();
            currentFlow.value = response;
            isFlowInitialized.value = true;
            props.onFlowChange?.(response);
            emit('flowChange', response);

            if (response?.flowStatus === EmbeddedFlowStatus.Complete) {
              props.onComplete?.(response);
              emit('complete', response);
              return;
            }
            if (response?.flowStatus === EmbeddedFlowStatus.Incomplete) {
              setupFormFields(response);
            }
          } catch (err) {
            handleError(err);
            props.onError?.(err as Error);
            emit('error', err);
          } finally {
            isLoading.value = false;
          }
        })();
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

      // Render-props (scoped slot) escape hatch
      if (slots['default']) {
        const renderProps: BaseSignUpRenderProps = {
          components: (currentFlow.value?.data?.components as any[]) || [],
          errors: formErrors.value,
          handleInputChange,
          handleSubmit,
          isLoading: isLoading.value,
          isValid: isFormValid(),
          messages: (flowMessages.value as {message: string; type: string}[]) || [],
          subtitle: flowSubtitle.value || t('signup.subheading') || '',
          title: flowTitle.value || t('signup.heading') || '',
          touched: touchedFields.value,
          values: formValues.value,
        };
        return h('div', {class: containerClass}, slots['default'](renderProps));
      }

      // Loading state (initial flow fetch)
      if (!isFlowInitialized.value && isLoading.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h('div', {style: 'display:flex;justify-content:center;padding:2rem'}, h(Spinner)),
        );
      }

      // Failed to obtain a flow at all
      if (!currentFlow.value) {
        return h(Card, {class: containerClass, variant: props.variant}, () =>
          h(
            Alert,
            {variant: 'error'},
            () => t('errors.signup.flow.initialization.failure') || 'Failed to initialize sign-up flow',
          ),
        );
      }

      const components: EmbeddedFlowComponent[] = currentFlow.value.data?.components || [];

      const rendered: VNode[] = renderSignUpComponents(
        components,
        formValues.value,
        touchedFields.value,
        formErrors.value,
        isLoading.value,
        isFormValid(),
        handleInputChange,
        handleSubmit,
        {
          buttonClassName: props.buttonClassName,
          inputClassName: props.inputClassName,
          size: props.size,
        },
      );

      const cardChildren: VNode[] = [];

      if (props.showLogo) {
        cardChildren.push(h('div', {style: 'display:flex;justify-content:center;margin-bottom:1rem'}, [h(Logo)]));
      }

      if (props.showTitle || props.showSubtitle) {
        const headerChildren: VNode[] = [];
        if (props.showTitle) {
          headerChildren.push(
            h(Typography, {variant: 'h2'}, {default: () => flowTitle.value || t('signup.heading') || 'Sign Up'}),
          );
        }
        if (props.showSubtitle) {
          headerChildren.push(
            h(
              Typography,
              {variant: 'body1'},
              {default: () => flowSubtitle.value || t('signup.subheading') || 'Create your account'},
            ),
          );
        }
        cardChildren.push(h('div', {style: 'padding: 0 1rem 1rem'}, headerChildren));
      }

      // Flow-level messages (errors, info)
      if (flowMessages.value && flowMessages.value.length > 0) {
        cardChildren.push(
          h(
            'div',
            {style: 'padding: 0 1rem'},
            flowMessages.value.map((msg: any, i: number) =>
              h(
                Alert,
                {
                  class: props.messageClassName,
                  key: msg.id || i,
                  variant: msg.type?.toLowerCase() === 'error' ? 'error' : 'info',
                },
                () => msg.message,
              ),
            ),
          ),
        );
      }

      cardChildren.push(
        h(
          'form',
          {
            class: withVendorCSSClassPrefix('signup__form'),
            onSubmit: (e: Event): void => {
              e.preventDefault();
              // Submit-type buttons in the V1 flow are handled inline by the
              // `onSubmit` handler attached to the BUTTON component; the
              // native form submit is a fallback (e.g. enter-key in a field).
              handleSubmit({config: {type: 'submit'}, type: 'BUTTON'});
            },
            style: 'padding: 1rem;display:flex;flex-direction:column;gap:0.75rem',
          },
          rendered.length > 0
            ? rendered
            : [
                h(
                  Alert,
                  {variant: 'warning'},
                  () => t('errors.signup.components.not.available') || 'No components available',
                ),
              ],
        ),
      );

      return h(Card, {class: containerClass, variant: props.variant}, () => cardChildren);
    };
  },
});

export default BaseSignUp;
