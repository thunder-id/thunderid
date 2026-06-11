/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
  ThunderIDRuntimeError,
  type ConsentPurposeDataV2 as ConsentPurposeData,
  EmbeddedFlowComponentV2 as EmbeddedFlowComponent,
  EmbeddedFlowType,
  EmbeddedSignInFlowRequestV2,
  EmbeddedSignInFlowResponseV2,
  EmbeddedSignInFlowStatusV2,
  EmbeddedSignInFlowTypeV2,
  FlowMetadataResponse,
} from '@thunderid/browser';
import {
  type Component,
  type PropType,
  type Ref,
  type SetupContext,
  type VNode,
  defineComponent,
  h,
  onMounted,
  onUnmounted,
  ref,
  watch,
} from 'vue';
import BaseSignIn from './BaseSignIn';
import useFlowMeta from '../../../../composables/useFlowMeta';
import useI18n from '../../../../composables/useI18n';
import useThunderID from '../../../../composables/useThunderID';
import {useOAuthCallback} from '../../../../composables/v2/useOAuthCallback';
import {initiateOAuthRedirect} from '../../../../utils/oauth';
import {extractErrorMessage, normalizeFlowResponse} from '../../../../utils/v2/flowTransformer';
import {handlePasskeyAuthentication, handlePasskeyRegistration} from '../../../../utils/v2/passkey';

const EXECUTION_ID_STORAGE_KEY = 'thunderid_execution_id';

interface PasskeyState {
  actionId: string | null;
  challenge: string | null;
  creationOptions: string | null;
  error: Error | null;
  executionId: string | null;
  isActive: boolean;
}

/**
 * Render props passed to the default scoped slot for custom UI rendering.
 */
export interface SignInRenderProps {
  additionalData?: Record<string, any>;
  components: EmbeddedFlowComponent[];
  error: Error | null;
  initialize: () => Promise<void>;
  isInitialized: boolean;
  isLoading: boolean;
  isTimeoutDisabled?: boolean;
  meta: FlowMetadataResponse | null;
  onSubmit: (payload: EmbeddedSignInFlowRequestV2) => Promise<void>;
}

/**
 * SignIn — app-native sign-in component with full flow lifecycle management.
 *
 * Initializes the authentication flow, handles passkey authentication/registration,
 * OAuth redirect flows, and renders the UI via `BaseSignIn` or a scoped slot.
 *
 * @example
 * ```vue
 * <!-- Default UI -->
 * <SignIn
 *   @success="(data) => console.log('Authenticated:', data)"
 *   @error="(err) => console.error('Auth failed:', err)"
 * />
 *
 * <!-- Custom UI via scoped slot -->
 * <SignIn v-slot="{ components, onSubmit, isLoading, error }">
 *   <!-- your custom sign-in UI here -->
 * </SignIn>
 * ```
 */
const SignIn: Component = defineComponent({
  name: 'SignIn',
  props: {
    className: {default: '', type: String},
    size: {
      default: 'medium',
      type: String as PropType<'small' | 'medium' | 'large'>,
    },
    variant: {
      default: 'outlined',
      type: String as PropType<'elevated' | 'outlined' | 'flat'>,
    },
  },
  emits: ['error', 'success'],
  setup(
    props: Readonly<{className: string; size: 'small' | 'medium' | 'large'; variant: 'elevated' | 'outlined' | 'flat'}>,
    {slots, emit, attrs}: SetupContext,
  ): () => VNode | null {
    const {
      applicationId,
      afterSignInUrl,
      signIn,
      isInitialized,
      isLoading: sdkLoading,
      scopes,
      getStorageManager,
    } = useThunderID();
    const {meta: flowMeta} = useFlowMeta();
    const {t} = useI18n();

    // Flow state
    const components: Ref<EmbeddedFlowComponent[]> = ref([]);
    const additionalData: Ref<Record<string, any>> = ref({});
    const currentExecutionId: Ref<string | null> = ref(null);
    const isFlowInitialized: Ref<boolean> = ref(false);
    const flowError: Ref<Error | null> = ref(null);
    const isSubmitting: Ref<boolean> = ref(false);
    const isTimeoutDisabled: Ref<boolean> = ref(false);
    const passkeyState: Ref<PasskeyState> = ref({
      actionId: null,
      challenge: null,
      creationOptions: null,
      error: null,
      executionId: null,
      isActive: false,
    });

    // Track one-time initialization and OAuth processing
    let initializationAttempted = false;
    const oauthCodeProcessedFlag: {value: boolean} = {value: false};
    let passkeyProcessed = false;

    // ── Helpers ──────────────────────────────────────────────────────────

    const persistExecutionId = (executionId: string | null): void => {
      currentExecutionId.value = executionId;
      if (executionId) {
        sessionStorage.setItem(EXECUTION_ID_STORAGE_KEY, executionId);
      } else {
        sessionStorage.removeItem(EXECUTION_ID_STORAGE_KEY);
      }
    };

    const clearFlowState = async (): Promise<void> => {
      persistExecutionId(null);
      isFlowInitialized.value = false;
      const sm = getStorageManager();
      if (sm) {
        await sm.removeHybridDataParameter('authId');
      }
      isTimeoutDisabled.value = false;
      oauthCodeProcessedFlag.value = false;
    };

    interface UrlParams {
      applicationId: string | null;
      authId: string | null;
      code: string | null;
      error: string | null;
      errorDescription: string | null;
      executionId: string | null;
      nonce: string | null;
      state: string | null;
    }

    const getUrlParams = (): UrlParams => {
      const params: URLSearchParams = new URLSearchParams(window?.location?.search ?? '');
      return {
        applicationId: params.get('applicationId'),
        authId: params.get('authId'),
        code: params.get('code'),
        error: params.get('error'),
        errorDescription: params.get('error_description'),
        executionId: params.get('executionId'),
        nonce: params.get('nonce'),
        state: params.get('state'),
      };
    };

    const cleanupOAuthUrlParams = (): void => {
      if (!window?.location?.href) return;
      const url: URL = new URL(window.location.href);
      ['error', 'error_description', 'code', 'state', 'nonce'].forEach((p: string) => url.searchParams.delete(p));
      window.history.replaceState({}, '', url.toString());
    };

    const cleanupFlowUrlParams = (): void => {
      if (!window?.location?.href) return;
      const url: URL = new URL(window.location.href);
      ['executionId', 'authId', 'applicationId'].forEach((p: string) => url.searchParams.delete(p));
      window.history.replaceState({}, '', url.toString());
    };

    const setError = (error: Error): void => {
      flowError.value = error;
      isFlowInitialized.value = true;
      emit('error', error);
    };

    // ── Flow initialization ───────────────────────────────────────────────

    const initializeFlow = async (): Promise<void> => {
      const urlParams: UrlParams = getUrlParams();

      oauthCodeProcessedFlag.value = false;

      if (urlParams.authId) {
        const sm = getStorageManager();
        if (sm) {
          await sm.setHybridDataParameter('authId', urlParams.authId);
        }
      }

      const effectiveApplicationId: string | null | undefined = applicationId || urlParams.applicationId;

      if (!urlParams.executionId && !effectiveApplicationId) {
        const err: ThunderIDRuntimeError = new ThunderIDRuntimeError(
          'Either executionId or applicationId is required for authentication',
          'SIGN_IN_ERROR',
          'vue',
        );
        setError(err);
        throw err;
      }

      try {
        flowError.value = null;

        let response: EmbeddedSignInFlowResponseV2;

        if (urlParams.executionId) {
          response = (await signIn({executionId: urlParams.executionId})) as EmbeddedSignInFlowResponseV2;
        } else {
          response = (await signIn({
            applicationId: effectiveApplicationId,
            flowType: EmbeddedFlowType.Authentication,
            ...(scopes && {scopes}),
          })) as EmbeddedSignInFlowResponseV2;
        }

        // Handle OAuth redirect types
        if (response.type === EmbeddedSignInFlowTypeV2.Redirection) {
          const redirectURL: string | undefined = (response.data as any)?.redirectURL || (response as any)?.redirectURL;
          if (redirectURL && window?.location) {
            if (response.executionId) persistExecutionId(response.executionId);
            if (urlParams.authId) {
              const sm = getStorageManager();
              if (sm) {
                await sm.setHybridDataParameter('authId', urlParams.authId);
              }
            }
            initiateOAuthRedirect(redirectURL);
            return;
          }
        }

        const {
          executionId: normalizedExecutionId,
          components: normalizedComponents,
          additionalData: normalizedAdditionalData,
        } = normalizeFlowResponse(response, t, {resolveTranslations: false}, flowMeta.value);

        if (normalizedExecutionId && normalizedComponents) {
          persistExecutionId(normalizedExecutionId);
          components.value = normalizedComponents;
          additionalData.value = normalizedAdditionalData ?? {};
          isFlowInitialized.value = true;
          isTimeoutDisabled.value = false;
          cleanupFlowUrlParams();
        }
      } catch (error: unknown) {
        const err: any = error as any;
        clearFlowState();
        setError(new Error(extractErrorMessage(err, t)));
        initializationAttempted = false;
      }
    };

    // ── Submit handler ────────────────────────────────────────────────────

    const handleSubmit = async (payload: EmbeddedSignInFlowRequestV2): Promise<void> => {
      const effectiveExecutionId: string | null = payload.executionId || currentExecutionId.value;

      if (!effectiveExecutionId) {
        throw new Error('No active flow ID');
      }

      const processedInputs: Record<string, any> = {...payload.inputs};

      // Auto-compile consent decisions if on a consent prompt step
      if (additionalData.value?.['consentPrompt']) {
        try {
          const consentRaw: any = additionalData.value['consentPrompt'];
          const purposes: ConsentPurposeData[] =
            typeof consentRaw === 'string' ? JSON.parse(consentRaw) : consentRaw.purposes || consentRaw;

          let isDeny = false;
          if (payload.action) {
            const findAction = (comps: any[]): any => {
              if (!comps?.length) return null;
              const found: any = comps.find((c: any) => c.id === payload.action);
              if (found) return found;
              return comps.reduce((acc: any, c: any) => acc || (c.components ? findAction(c.components) : null), null);
            };
            const submitAction: any = findAction(components.value as any[]);
            if (submitAction && submitAction.variant?.toLowerCase() !== 'primary') {
              isDeny = true;
            }
          }

          const decisions: Record<string, unknown> = {
            purposes: purposes.map((p) => ({
              approved: !isDeny,
              elements: [
                ...(p.essential ?? []).map((e) => ({approved: !isDeny, name: e.name})),
                ...(p.optional ?? []).map((e) => {
                  const key = `__consent_opt__${p.purposeId}__${e.name}`;
                  return {approved: isDeny ? false : processedInputs[key] !== 'false', name: e.name};
                }),
              ],
              purposeName: p.purposeName,
            })),
          };
          processedInputs['consent_decisions'] = JSON.stringify(decisions);

          Object.keys(processedInputs).forEach((key: string) => {
            if (key.startsWith('__consent_opt__')) delete processedInputs[key];
          });
        } catch {
          // Ignore consent construction failures
        }
      }

      try {
        isSubmitting.value = true;
        flowError.value = null;

        const response: EmbeddedSignInFlowResponseV2 = (await signIn({
          executionId: effectiveExecutionId,
          ...payload,
          inputs: processedInputs,
        })) as EmbeddedSignInFlowResponseV2;

        // Handle OAuth redirect
        if (response.type === EmbeddedSignInFlowTypeV2.Redirection) {
          const redirectURL: string | undefined = (response.data as any)?.redirectURL || (response as any)?.redirectURL;
          if (redirectURL && window?.location) {
            if (response.executionId) persistExecutionId(response.executionId);
            const urlParams: UrlParams = getUrlParams();
            if (urlParams.authId) {
              const sm = getStorageManager();
              if (sm) {
                await sm.setHybridDataParameter('authId', urlParams.authId);
              }
            }
            initiateOAuthRedirect(redirectURL);
            return;
          }
        }

        // Handle passkey challenge in response
        if (
          response.data?.additionalData?.['passkeyChallenge'] ||
          response.data?.additionalData?.['passkeyCreationOptions']
        ) {
          const {passkeyChallenge, passkeyCreationOptions} = response.data.additionalData as any;
          passkeyProcessed = false;
          passkeyState.value = {
            actionId: 'submit',
            challenge: passkeyChallenge || null,
            creationOptions: passkeyCreationOptions || null,
            error: null,
            executionId: response.executionId || effectiveExecutionId,
            isActive: true,
          };
          isSubmitting.value = false;
          return;
        }

        const {
          executionId: normalizedExecutionId,
          components: normalizedComponents,
          additionalData: normalizedAdditionalData,
        } = normalizeFlowResponse(response, t, {resolveTranslations: false}, flowMeta.value);

        // Handle error flow status
        if (response.flowStatus === EmbeddedSignInFlowStatusV2.Error) {
          clearFlowState();
          const err: Error = new Error(extractErrorMessage(response, t));
          setError(err);
          cleanupFlowUrlParams();
          throw err;
        }

        // Handle flow completion
        if (response.flowStatus === EmbeddedSignInFlowStatusV2.Complete) {
          const redirectUrl: string | undefined = (response as any)?.redirectUrl || (response as any)?.redirect_uri;
          const finalRedirectUrl: string | undefined = redirectUrl || afterSignInUrl;

          isSubmitting.value = false;
          persistExecutionId(null);
          isFlowInitialized.value = false;
          const sm = getStorageManager();
          if (sm) {
            await sm.removeHybridDataParameter('authId');
          }
          cleanupOAuthUrlParams();

          emit('success', {
            redirectUrl: finalRedirectUrl,
            ...(response.data || {}),
          });

          if (finalRedirectUrl && window?.location) {
            window.location.href = finalRedirectUrl;
          }
          return;
        }

        // Update flow state for next step
        if (normalizedExecutionId && normalizedComponents) {
          persistExecutionId(normalizedExecutionId);
          components.value = normalizedComponents;
          additionalData.value = normalizedAdditionalData ?? {};
          isTimeoutDisabled.value = false;
          isFlowInitialized.value = true;
          cleanupFlowUrlParams();

          if ((response as any)?.error) {
            flowError.value = new Error(extractErrorMessage(response, t));
          }
        }
      } catch (error: unknown) {
        const err: any = error as any;
        if (err instanceof Error && flowError.value === err) {
          // Already set; re-throw
          throw err;
        }
        clearFlowState();
        setError(new Error(extractErrorMessage(err, t)));
      } finally {
        isSubmitting.value = false;
      }
    };

    // ── Step timeout ──────────────────────────────────────────────────────

    let timeoutHandle: ReturnType<typeof setTimeout> | null = null;

    const scheduleTimeout = (timeoutMs: number): void => {
      if (timeoutHandle) clearTimeout(timeoutHandle);
      if (timeoutMs <= 0 || !isFlowInitialized.value) {
        isTimeoutDisabled.value = false;
        return;
      }
      const remaining: number = Math.max(0, Math.floor((timeoutMs - Date.now()) / 1000));
      if (remaining <= 0) {
        isTimeoutDisabled.value = true;
        setError(new Error(t('errors.signin.timeout') || 'Time allowed to complete the step has expired.'));
        return;
      }
      timeoutHandle = setTimeout(() => {
        isTimeoutDisabled.value = true;
        setError(new Error(t('errors.signin.timeout') || 'Time allowed to complete the step has expired.'));
      }, remaining * 1000);
    };

    watch(
      () => [additionalData.value?.['stepTimeout'], isFlowInitialized.value] as [number | undefined, boolean],
      ([timeoutMs]: [number | undefined, boolean]) => {
        scheduleTimeout(Number(timeoutMs) || 0);
      },
    );

    onUnmounted(() => {
      if (timeoutHandle) clearTimeout(timeoutHandle);
    });

    // ── Passkey processing ────────────────────────────────────────────────

    watch(
      () => passkeyState.value,
      async (state: PasskeyState) => {
        if (!state.isActive || (!state.challenge && !state.creationOptions) || !state.executionId) return;
        if (passkeyProcessed) return;
        passkeyProcessed = true;

        try {
          let inputs: Record<string, string>;

          if (state.challenge) {
            const passkeyResponse: string = await handlePasskeyAuthentication(state.challenge);
            const obj: any = JSON.parse(passkeyResponse);
            inputs = {
              authenticatorData: obj.response.authenticatorData,
              clientDataJSON: obj.response.clientDataJSON,
              credentialId: obj.id,
              signature: obj.response.signature,
              userHandle: obj.response.userHandle,
            };
          } else if (state.creationOptions) {
            const passkeyResponse: string = await handlePasskeyRegistration(state.creationOptions);
            const obj: any = JSON.parse(passkeyResponse);
            inputs = {
              attestationObject: obj.response.attestationObject,
              clientDataJSON: obj.response.clientDataJSON,
              credentialId: obj.id,
            };
          } else {
            throw new Error('No passkey challenge or creation options available');
          }

          await handleSubmit({executionId: state.executionId, inputs});

          passkeyState.value = {
            actionId: null,
            challenge: null,
            creationOptions: null,
            error: null,
            executionId: null,
            isActive: false,
          };
        } catch (error: unknown) {
          const err: Error = error as Error;
          passkeyState.value = {...passkeyState.value, error: err, isActive: false};
          flowError.value = err;
          emit('error', err);
        }
      },
      {deep: true},
    );

    // ── OAuth callback (via composable) ─────────────────────────────────

    useOAuthCallback({
      currentExecutionId,
      executionIdStorageKey: EXECUTION_ID_STORAGE_KEY,
      isInitialized,
      isSubmitting,
      onError: (err: any) => {
        // Guard against double-processing when handleSubmit already set the error
        if (!flowError.value) {
          clearFlowState();
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      },
      onSubmit: (payload: EmbeddedSignInFlowRequestV2) =>
        handleSubmit({executionId: payload.executionId, inputs: payload.inputs}),
      processedFlag: oauthCodeProcessedFlag,
      setExecutionId: persistExecutionId,
    });

    // ── Lifecycle ─────────────────────────────────────────────────────────

    onMounted(async () => {
      const urlParams: UrlParams = getUrlParams();

      if (urlParams.authId) {
        const sm = getStorageManager();
        if (sm) {
          await sm.setHybridDataParameter('authId', urlParams.authId);
        }
      }
    });

    // Initialize flow when SDK is ready (OAuth callback is handled by useOAuthCallback)
    watch(
      () =>
        [
          isInitialized.value,
          sdkLoading.value,
          isFlowInitialized.value,
          currentExecutionId.value,
          isSubmitting.value,
        ] as [boolean, boolean, boolean, string | null, boolean],
      ([initialized, loading, flowInit, executionId, submitting]: [
        boolean,
        boolean,
        boolean,
        string | null,
        boolean,
      ]) => {
        const urlParams: UrlParams = getUrlParams();
        const hasOAuthCode = !!urlParams.code;
        const hasOAuthState = !!urlParams.state;

        // Initialize flow when SDK is ready and no flow is active
        if (
          initialized &&
          !loading &&
          !flowInit &&
          !initializationAttempted &&
          !executionId &&
          !hasOAuthCode &&
          !hasOAuthState &&
          !submitting &&
          !oauthCodeProcessedFlag.value
        ) {
          initializationAttempted = true;
          initializeFlow();
        }
      },
    );

    // ── Render ────────────────────────────────────────────────────────────

    return (): VNode | null => {
      const combinedIsLoading: boolean = sdkLoading.value || isSubmitting.value || !isInitialized.value;

      // Scoped slot / render props pattern
      if (slots['default']) {
        const renderProps: SignInRenderProps = {
          additionalData: additionalData.value,
          components: components.value,
          error: flowError.value,
          initialize: initializeFlow,
          isInitialized: isFlowInitialized.value,
          isLoading: combinedIsLoading,
          isTimeoutDisabled: isTimeoutDisabled.value,
          meta: flowMeta.value,
          onSubmit: handleSubmit,
        };
        return h('div', {}, slots['default'](renderProps));
      }

      // Default BaseSignIn rendering
      return h(BaseSignIn, {
        ...attrs,
        additionalData: additionalData.value,
        class: props.className,
        components: components.value,
        error: flowError.value,
        isLoading: combinedIsLoading || !isFlowInitialized.value,
        isTimeoutDisabled: isTimeoutDisabled.value,
        onError: (err: Error) => emit('error', err),
        onSubmit: handleSubmit,
        size: props.size,
        variant: props.variant,
      });
    };
  },
});

export default SignIn;
