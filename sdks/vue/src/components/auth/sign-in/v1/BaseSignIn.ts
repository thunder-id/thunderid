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
import {createSignInOptionFromAuthenticator} from './options/SignInOptionFactory';
import useFlow from '../../../../composables/useFlow';
import useI18n from '../../../../composables/useI18n';
import Alert from '../../../primitives/Alert';
import Card from '../../../primitives/Card';
import Divider from '../../../primitives/Divider';
import Logo from '../../../primitives/Logo';
import Spinner from '../../../primitives/Spinner';
import Typography from '../../../primitives/Typography';

/**
 * Authenticators that are currently hidden from the UI.
 * OrganizationSSO is not yet supported in app-native authentication.
 */
const HIDDEN_AUTHENTICATORS: string[] = ['T3JnYW5pemF0aW9uQXV0aGVudGljYXRvcjpTU08'];

const isPasskeyAuthenticator = (authenticator: EmbeddedSignInFlowAuthenticator): boolean =>
  authenticator.authenticatorId === ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Passkey &&
  authenticator.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.InternalPrompt &&
  !!(authenticator.metadata as any)?.additionalData?.challengeData;

/**
 * V1 BaseSignIn component — authenticator-based app-native sign-in for Vue.
 *
 * Handles multi-step authentication flows, form rendering per-authenticator,
 * redirect popups for OAuth, and passkey/FIDO WebAuthn.
 */
const BaseSignIn: Component = defineComponent({
  name: 'BaseSignInV1',
  props: {
    afterSignInUrl: {default: undefined, type: String},
    buttonClassName: {default: '', type: String},
    className: {default: '', type: String},
    errorClassName: {default: '', type: String},
    inputClassName: {default: '', type: String},
    isLoading: {default: false, type: Boolean},
    messageClassName: {default: '', type: String},
    onInitialize: {default: undefined, type: Function as PropType<() => Promise<EmbeddedSignInFlowInitiateResponse>>},
    onSubmit: {
      default: undefined,
      type: Function as PropType<
        (
          payload: EmbeddedSignInFlowHandleRequestPayload,
          request: EmbeddedFlowExecuteRequestConfig,
        ) => Promise<EmbeddedSignInFlowHandleResponse>
      >,
    },
    showLogo: {default: true, type: Boolean},
    showSubtitle: {default: true, type: Boolean},
    showTitle: {default: true, type: Boolean},
    size: {default: 'medium', type: String as PropType<'small' | 'medium' | 'large'>},
    variant: {default: 'outlined', type: String as PropType<'elevated' | 'outlined' | 'flat'>},
  },
  emits: ['error', 'flowChange', 'success'],
  setup(props: any, {emit}: SetupContext): () => VNode {
    const {t} = useI18n();
    const {title: flowTitle, subtitle: flowSubtitle, messages: flowMessages} = useFlow();

    // ── Reactive state ──
    const isInitRequestLoading: Ref<boolean> = ref(false);
    const isInitialized: Ref<boolean> = ref(false);
    const currentFlow: Ref<EmbeddedSignInFlowInitiateResponse | null> = ref(null);
    const currentAuthenticator: Ref<EmbeddedSignInFlowAuthenticator | null> = ref(null);
    const error: Ref<string | null> = ref(null);
    const messages: Ref<{message: string; type: string}[]> = ref([]);
    const formValues: Ref<Record<string, string>> = ref({});
    const touchedFields: Ref<Record<string, boolean>> = ref({});

    const isLoading = (): boolean => props.isLoading || isInitRequestLoading.value;

    // ── Form helpers ──

    const setupFormFields = (authenticator: EmbeddedSignInFlowAuthenticator): void => {
      const vals: Record<string, string> = {};
      authenticator.metadata?.params?.forEach((param: any) => {
        vals[param.param] = '';
      });
      formValues.value = vals;
      touchedFields.value = {};
    };

    const handleInputChange = (param: string, value: string): void => {
      formValues.value = {...formValues.value, [param]: value};
      touchedFields.value = {...touchedFields.value, [param]: true};
    };

    const touchAllFields = (): void => {
      const touched: Record<string, boolean> = {};
      Object.keys(formValues.value).forEach((key: string) => {
        touched[key] = true;
      });
      touchedFields.value = touched;
    };

    const validateForm = (): boolean => {
      if (!currentAuthenticator.value) return true;
      const required: string[] = currentAuthenticator.value.requiredParams || [];

      return required.every((key: string) => {
        const val: string = formValues.value[key] || '';

        return !!val && val.trim() !== '';
      });
    };

    // ── Next step processing ──

    let handleAuthenticatorSelection: (
      authenticator: EmbeddedSignInFlowAuthenticator,
      formData?: Record<string, string>,
    ) => Promise<void>;

    const processNextStep = (response: any): void => {
      if (response && 'flowId' in response && 'nextStep' in response) {
        currentFlow.value = response;

        if (response.nextStep?.authenticators?.length > 0) {
          if (
            response.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
            response.nextStep.authenticators.length > 1
          ) {
            currentAuthenticator.value = null;
          } else {
            const nextAuth: EmbeddedSignInFlowAuthenticator = response.nextStep.authenticators[0];
            if (isPasskeyAuthenticator(nextAuth)) {
              handleAuthenticatorSelection(nextAuth).catch((err: unknown) => {
                emit('error', err);
              });
              return;
            }
            currentAuthenticator.value = nextAuth;
            setupFormFields(nextAuth);
          }
        }

        if (response.nextStep?.messages) {
          messages.value = response.nextStep.messages.map((msg: any) => ({
            message: msg.message || '',
            type: msg.type || 'INFO',
          }));
        }
      }
    };

    // ── Redirect popup (OAuth flows) ──

    const handleRedirectionIfNeeded = (response: EmbeddedSignInFlowHandleResponse): boolean => {
      if (
        response &&
        'nextStep' in response &&
        response.nextStep &&
        (response.nextStep as any).stepType === EmbeddedSignInFlowStepType.AuthenticatorPrompt &&
        (response.nextStep as any).authenticators?.length === 1
      ) {
        const responseAuth: any = (response.nextStep as any).authenticators[0];
        if (
          responseAuth.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.RedirectionPrompt &&
          responseAuth.metadata?.additionalData?.redirectUrl
        ) {
          const {redirectUrl} = responseAuth.metadata.additionalData;
          const popup: Window | null = window.open(
            redirectUrl,
            'oauth_popup',
            'width=500,height=600,scrollbars=yes,resizable=yes',
          );

          if (!popup) return false;

          let messageHandler: any;
          let popupMonitor: any;
          let hasProcessedCallback = false;

          const cleanup = (): void => {
            window.removeEventListener('message', messageHandler);
            if (popupMonitor) clearInterval(popupMonitor);
          };

          messageHandler = async (event: MessageEvent): Promise<void> => {
            if (event.source !== popup) return;
            const expectedOrigin: string = props.afterSignInUrl
              ? new URL(props.afterSignInUrl).origin
              : window.location.origin;
            if (event.origin !== expectedOrigin && event.origin !== window.location.origin) return;

            const {code, state} = event.data;
            if (code && state) {
              const payload: EmbeddedSignInFlowHandleRequestPayload = {
                flowId: currentFlow.value.flowId,
                selectedAuthenticator: {
                  authenticatorId: responseAuth.authenticatorId,
                  params: {code, state},
                },
              };
              await props.onSubmit(payload, {
                method: currentFlow.value?.links[0].method,
                url: currentFlow.value?.links[0].href,
              });
              popup.close();
              cleanup();
            }
          };

          window.addEventListener('message', messageHandler);

          popupMonitor = setInterval(async (): Promise<void> => {
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
                  const oauthError: string | null = url.searchParams.get('error');

                  if (oauthError) {
                    popup.close();
                    cleanup();
                    return;
                  }
                  if (code && state) {
                    const payload: EmbeddedSignInFlowHandleRequestPayload = {
                      flowId: currentFlow.value.flowId,
                      selectedAuthenticator: {
                        authenticatorId: responseAuth.authenticatorId,
                        params: {code, state},
                      },
                    };
                    const submitResponse: any = await props.onSubmit(payload, {
                      method: currentFlow.value?.links[0].method,
                      url: currentFlow.value?.links[0].href,
                    });
                    popup.close();
                    emit('flowChange', submitResponse);
                    if (submitResponse?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
                      emit('success', submitResponse.authData);
                    }
                  }
                }
              } catch {
                // Cross-origin error expected during OAuth redirect
              }
            } catch {
              // Ignore popup monitoring errors
            }
          }, 1000);

          return true;
        }
      }
      return false;
    };

    // ── Form submission ──

    const handleSubmit = async (submittedValues: Record<string, string>): Promise<void> => {
      if (!currentFlow.value || !currentAuthenticator.value) return;

      touchAllFields();
      if (!validateForm()) return;

      isInitRequestLoading.value = true;
      error.value = null;
      messages.value = [];

      try {
        const payload: EmbeddedSignInFlowHandleRequestPayload = {
          flowId: currentFlow.value.flowId,
          selectedAuthenticator: {
            authenticatorId: currentAuthenticator.value.authenticatorId,
            params: submittedValues,
          },
        };

        const response: any = await props.onSubmit(payload, {
          method: currentFlow.value.links[0].method,
          url: currentFlow.value.links[0].href,
        });
        emit('flowChange', response);

        if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
          emit('success', response.authData);
          return;
        }
        if (
          response?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
          response?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
        ) {
          error.value = t('errors.signin.flow.completion.failure');
          return;
        }
        if (handleRedirectionIfNeeded(response)) return;

        processNextStep(response);
      } catch (err: any) {
        error.value = err instanceof ThunderIDAPIError ? err.message : t('errors.signin.flow.failure');
        emit('error', err);
      } finally {
        isInitRequestLoading.value = false;
      }
    };

    // ── Authenticator selection (multi-option, passkey, redirect, form) ──

    handleAuthenticatorSelection = async (
      authenticator: EmbeddedSignInFlowAuthenticator,
      formData?: Record<string, string>,
    ): Promise<void> => {
      if (!currentFlow.value) return;

      if (formData) touchAllFields();

      isInitRequestLoading.value = true;
      error.value = null;
      messages.value = [];

      try {
        // Passkey / FIDO
        if (isPasskeyAuthenticator(authenticator)) {
          const challengeData: any = (authenticator.metadata as any)?.additionalData?.challengeData;
          if (!challengeData) throw new Error('Missing challenge data for passkey authentication');

          const tokenResponse: any = await handleWebAuthnAuthentication(challengeData);
          const payload: EmbeddedSignInFlowHandleRequestPayload = {
            flowId: currentFlow.value.flowId,
            selectedAuthenticator: {
              authenticatorId: authenticator.authenticatorId,
              params: {tokenResponse},
            },
          };
          const response: any = await props.onSubmit(payload, {
            method: currentFlow.value.links[0].method,
            url: currentFlow.value.links[0].href,
          });
          emit('flowChange', response);

          if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
            emit('success', response.authData);
            return;
          }
          if (
            response?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
            response?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
          ) {
            error.value = t('errors.signin.flow.passkeys.completion.failure');
            return;
          }
          processNextStep(response);
          return;
        }

        // Redirection prompt (social login first-touch)
        if (authenticator.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.RedirectionPrompt) {
          const payload: EmbeddedSignInFlowHandleRequestPayload = {
            flowId: currentFlow.value.flowId,
            selectedAuthenticator: {
              authenticatorId: authenticator.authenticatorId,
              params: {},
            },
          };
          const response: any = await props.onSubmit(payload, {
            method: currentFlow.value.links[0].method,
            url: currentFlow.value.links[0].href,
          });
          emit('flowChange', response);

          if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
            emit('success', response.authData);
            return;
          }
          handleRedirectionIfNeeded(response);
          return;
        }

        // Form data submission
        if (formData) {
          if (!validateForm()) return;

          const formPayload: EmbeddedSignInFlowHandleRequestPayload = {
            flowId: currentFlow.value.flowId,
            selectedAuthenticator: {
              authenticatorId: authenticator.authenticatorId,
              params: formData,
            },
          };
          const formResponse: any = await props.onSubmit(formPayload, {
            method: currentFlow.value.links[0].method,
            url: currentFlow.value.links[0].href,
          });
          emit('flowChange', formResponse);

          if (formResponse?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
            emit('success', formResponse.authData);
            return;
          }
          if (
            formResponse?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
            formResponse?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
          ) {
            error.value = t('errors.signin.flow.completion.failure');
            return;
          }
          if (handleRedirectionIfNeeded(formResponse)) return;
          processNextStep(formResponse);
          return;
        }

        // No form data — direct selection or show form
        const hasParams = !!(authenticator.metadata?.params && authenticator.metadata.params.length > 0);
        if (!hasParams) {
          const payload: EmbeddedSignInFlowHandleRequestPayload = {
            flowId: currentFlow.value.flowId,
            selectedAuthenticator: {
              authenticatorId: authenticator.authenticatorId,
              params: {},
            },
          };
          const response: any = await props.onSubmit(payload, {
            method: currentFlow.value.links[0].method,
            url: currentFlow.value.links[0].href,
          });
          emit('flowChange', response);

          if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
            emit('success', response.authData);
            return;
          }
          if (
            response?.flowStatus === EmbeddedSignInFlowStatus.FailCompleted ||
            response?.flowStatus === EmbeddedSignInFlowStatus.FailIncomplete
          ) {
            error.value = t('errors.signin.flow.completion.failure');
            return;
          }
          if (handleRedirectionIfNeeded(response)) return;
          processNextStep(response);
        } else {
          currentAuthenticator.value = authenticator;
          setupFormFields(authenticator);
        }
      } catch (err: any) {
        const errorMessage: string = err instanceof ThunderIDAPIError ? err.message : t('errors.signin.flow.failure');
        error.value = errorMessage;
        emit('error', err);
      } finally {
        isInitRequestLoading.value = false;
      }
    };

    // ── Multi-option checks ──

    const hasMultipleOptions = (): boolean =>
      !!(
        currentFlow.value &&
        'nextStep' in currentFlow.value &&
        currentFlow.value.nextStep?.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
        currentFlow.value.nextStep?.authenticators &&
        currentFlow.value.nextStep.authenticators.length > 1
      );

    const getAvailableAuthenticators = (): EmbeddedSignInFlowAuthenticator[] => {
      if (!currentFlow.value || !('nextStep' in currentFlow.value) || !currentFlow.value.nextStep?.authenticators) {
        return [];
      }
      return currentFlow.value.nextStep.authenticators;
    };

    // ── Initialize flow on mount ──

    let initAttempted = false;

    watch(
      () => props.isLoading,
      (loading: boolean) => {
        if (!loading && !initAttempted && props.onInitialize) {
          initAttempted = true;
          (async (): Promise<void> => {
            isInitRequestLoading.value = true;
            error.value = null;

            try {
              const response: any = await props.onInitialize();
              currentFlow.value = response;
              isInitialized.value = true;
              emit('flowChange', response);

              if (response?.flowStatus === EmbeddedSignInFlowStatus.SuccessCompleted) {
                emit('success', response.authData || {});
                return;
              }

              if (response?.nextStep?.authenticators?.length > 0) {
                if (
                  response.nextStep.stepType === EmbeddedSignInFlowStepType.MultiOptionsPrompt &&
                  response.nextStep.authenticators.length > 1
                ) {
                  currentAuthenticator.value = null;
                } else {
                  const authenticator: EmbeddedSignInFlowAuthenticator = response.nextStep.authenticators[0];
                  currentAuthenticator.value = authenticator;
                  setupFormFields(authenticator);
                }
              }

              if (response?.nextStep?.messages) {
                messages.value = response.nextStep.messages.map((msg: any) => ({
                  message: msg.message || '',
                  type: msg.type || 'INFO',
                }));
              }
            } catch (err: any) {
              error.value = err instanceof ThunderIDAPIError ? err.message : t('errors.signin.initialization');
              emit('error', err);
            } finally {
              isInitRequestLoading.value = false;
            }
          })();
        }
      },
      {immediate: true},
    );

    // ── Render helpers ──

    const renderAlertVariant = (type: string): string => {
      const lower: string = type.toLowerCase();
      if (lower === 'error') return 'error';
      if (lower === 'warning') return 'warning';
      if (lower === 'success') return 'success';
      return 'info';
    };

    const renderMessages = (): VNode[] =>
      messages.value.map((msg: any, i: number) =>
        h(Alert, {key: i, severity: renderAlertVariant(msg.type)}, {default: () => msg.message}),
      );

    const renderError = (): VNode | null =>
      error.value ? h(Alert, {severity: 'error'}, {default: () => error.value}) : null;

    // ── Main render function ──

    return (): VNode => {
      const cardClass: string = [
        withVendorCSSClassPrefix('signin'),
        withVendorCSSClassPrefix(`signin--${props.size}`),
        withVendorCSSClassPrefix(`signin--${props.variant}`),
        props.className,
      ]
        .filter(Boolean)
        .join(' ');

      // Loading state
      if (!isInitialized.value && isLoading()) {
        return h('div', {}, [
          props.showLogo ? h('div', {class: withVendorCSSClassPrefix('signin__logo')}, [h(Logo)]) : null,
          h(
            Card,
            {class: cardClass, variant: props.variant},
            {
              default: () => [
                h('div', {class: withVendorCSSClassPrefix('signin__loading')}, [
                  h(Spinner, {size: 'medium'}),
                  h(Typography, {variant: 'body1'}, {default: () => t('messages.loading.placeholder')}),
                ]),
              ],
            },
          ),
        ]);
      }

      // Multi-option prompt (no single authenticator selected)
      if (hasMultipleOptions() && !currentAuthenticator.value) {
        const available: EmbeddedSignInFlowAuthenticator[] = getAvailableAuthenticators();

        const userPromptAuths: EmbeddedSignInFlowAuthenticator[] = available.filter(
          (auth: any) =>
            auth.metadata?.promptType === EmbeddedSignInFlowAuthenticatorPromptType.UserPrompt ||
            (auth.idp === 'LOCAL' && auth.metadata?.params && auth.metadata.params.length > 0),
        );

        const optionAuths: EmbeddedSignInFlowAuthenticator[] = available
          .filter((auth: any) => !userPromptAuths.includes(auth))
          .filter((auth: any) => !HIDDEN_AUTHENTICATORS.includes(auth.authenticatorId));

        return h('div', {}, [
          props.showLogo ? h('div', {class: withVendorCSSClassPrefix('signin__logo')}, [h(Logo)]) : null,
          h(
            Card,
            {class: cardClass, variant: props.variant},
            {
              default: () => {
                const children: VNode[] = [];

                // Header
                if (props.showTitle || props.showSubtitle) {
                  children.push(
                    h('div', {class: withVendorCSSClassPrefix('signin__header')}, [
                      props.showTitle
                        ? h(Typography, {variant: 'h2'}, {default: () => flowTitle.value || t('signin.heading')})
                        : null,
                      props.showSubtitle
                        ? h(
                            Typography,
                            {variant: 'body1'},
                            {default: () => flowSubtitle.value || t('signin.subheading')},
                          )
                        : null,
                    ]),
                  );
                }

                // Flow messages
                if (flowMessages.value?.length > 0) {
                  children.push(
                    h(
                      'div',
                      {class: withVendorCSSClassPrefix('signin__flow-messages')},
                      flowMessages.value.map((fm: any, i: number) =>
                        h(Alert, {key: fm.id || i, severity: fm.type}, {default: () => fm.message}),
                      ),
                    ),
                  );
                }

                // Local messages & error
                if (messages.value.length > 0) children.push(h('div', {}, renderMessages()));
                const errNode: VNode | null = renderError();
                if (errNode) children.push(errNode);

                // User prompt authenticators (forms)
                userPromptAuths.forEach((auth: EmbeddedSignInFlowAuthenticator, index: number) => {
                  if (index > 0) children.push(h(Divider, {}, {default: () => 'OR'}));
                  children.push(
                    h(
                      'form',
                      {
                        onSubmit: (e: Event) => {
                          e.preventDefault();
                          const fd: Record<string, string> = {};
                          auth.metadata?.params?.forEach((p: any) => {
                            fd[p.param] = formValues.value[p.param] || '';
                          });
                          handleAuthenticatorSelection(auth, fd);
                        },
                      },
                      [
                        createSignInOptionFromAuthenticator(
                          auth,
                          formValues.value,
                          touchedFields.value,
                          isLoading(),
                          handleInputChange,
                          (a: any, fd: any) => handleAuthenticatorSelection(a, fd),
                          t,
                          {
                            buttonClassName: props.buttonClassName,
                            error: error.value,
                            inputClassName: props.inputClassName,
                          },
                        ),
                      ],
                    ),
                  );
                });

                // Divider between user prompts and options
                if (userPromptAuths.length > 0 && optionAuths.length > 0) {
                  children.push(h(Divider, {}, {default: () => 'OR'}));
                }

                // Option authenticators (social, multi-option buttons)
                optionAuths.forEach((auth: EmbeddedSignInFlowAuthenticator) => {
                  children.push(
                    h('div', {key: auth.authenticatorId}, [
                      createSignInOptionFromAuthenticator(
                        auth,
                        formValues.value,
                        touchedFields.value,
                        isLoading(),
                        handleInputChange,
                        (a: any, fd: any) => handleAuthenticatorSelection(a, fd),
                        t,
                        {
                          buttonClassName: props.buttonClassName,
                          error: error.value,
                          inputClassName: props.inputClassName,
                        },
                      ),
                    ]),
                  );
                });

                return children;
              },
            },
          ),
        ]);
      }

      // No authenticator available (error state)
      if (!currentAuthenticator.value) {
        return h('div', {}, [
          props.showLogo ? h('div', {class: withVendorCSSClassPrefix('signin__logo')}, [h(Logo)]) : null,
          h(
            Card,
            {class: cardClass, variant: props.variant},
            {
              default: () => {
                const errNode: VNode | null = renderError();
                return errNode
                  ? [errNode]
                  : [h(Typography, {variant: 'body1'}, {default: () => t('messages.loading.placeholder')})];
              },
            },
          ),
        ]);
      }

      // Passkey auto-trigger
      if (isPasskeyAuthenticator(currentAuthenticator.value) && !isLoading()) {
        handleAuthenticatorSelection(currentAuthenticator.value);
        return h('div', {}, [
          props.showLogo ? h('div', {class: withVendorCSSClassPrefix('signin__logo')}, [h(Logo)]) : null,
          h(
            Card,
            {class: cardClass, variant: props.variant},
            {
              default: () => [
                h('div', {style: 'text-align:center'}, [
                  h(Spinner, {size: 'large'}),
                  h(
                    Typography,
                    {variant: 'body1'},
                    {default: () => t('passkey.authenticating') || 'Authenticating with passkey...'},
                  ),
                ]),
              ],
            },
          ),
        ]);
      }

      // Single authenticator form
      return h('div', {}, [
        props.showLogo ? h('div', {class: withVendorCSSClassPrefix('signin__logo')}, [h(Logo)]) : null,
        h(
          Card,
          {class: cardClass, variant: props.variant},
          {
            default: () => {
              const children: VNode[] = [];

              // Header
              children.push(
                h('div', {class: withVendorCSSClassPrefix('signin__header')}, [
                  h(Typography, {variant: 'h2'}, {default: () => flowTitle.value || t('signin.heading')}),
                  h(Typography, {variant: 'body1'}, {default: () => flowSubtitle.value || t('signin.subheading')}),
                ]),
              );

              // Flow messages
              if (flowMessages.value?.length > 0) {
                children.push(
                  h(
                    'div',
                    {class: withVendorCSSClassPrefix('signin__flow-messages')},
                    flowMessages.value.map((fm: any, i: number) =>
                      h(Alert, {key: fm.id || i, severity: fm.type}, {default: () => fm.message}),
                    ),
                  ),
                );
              }

              // Local messages & error
              if (messages.value.length > 0) children.push(h('div', {}, renderMessages()));
              const errNode: VNode | null = renderError();
              if (errNode) children.push(errNode);

              // Form
              children.push(
                h(
                  'form',
                  {
                    class: withVendorCSSClassPrefix('signin__form'),
                    onSubmit: (e: Event) => {
                      e.preventDefault();
                      const fd: Record<string, string> = {};
                      currentAuthenticator.value.metadata?.params?.forEach((p: any) => {
                        fd[p.param] = formValues.value[p.param] || '';
                      });
                      handleSubmit(fd);
                    },
                  },
                  [
                    createSignInOptionFromAuthenticator(
                      currentAuthenticator.value,
                      formValues.value,
                      touchedFields.value,
                      isLoading(),
                      handleInputChange,
                      (_: any, fd: any) => handleSubmit(fd || formValues.value),
                      t,
                      {
                        buttonClassName: props.buttonClassName,
                        error: error.value,
                        inputClassName: props.inputClassName,
                      },
                    ),
                  ],
                ),
              );

              return children;
            },
          },
        ),
      ]);
    };
  },
});

export default BaseSignIn;
