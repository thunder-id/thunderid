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
  type EmbeddedFlowExecuteRequestPayload,
  type EmbeddedFlowExecuteResponse,
  EmbeddedFlowResponseType,
  EmbeddedFlowType,
} from '@thunderid/browser';
import {BaseSignUp} from '@thunderid/vue';
import type {BaseSignUpRenderProps} from '@thunderid/vue';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import {navigateTo} from '#app';
import {useThunderID} from '#imports';

export type SignUpRenderProps = BaseSignUpRenderProps;

/**
 * Nuxt-specific SignUp container for the embedded registration flow.
 *
 * Mirrors the Vue SDK's `SignUp` container but replaces all `window.location`
 * redirects with Nuxt's `navigateTo` for SSR-safe navigation after a
 * successful sign-up.
 *
 * Uses `useThunderID()` from the Nuxt auto-import layer and delegates all UI
 * rendering to {@link BaseSignUp} from `@thunderid/vue`.
 *
 * Additionally, `window.location.href` for OAuth redirect URLs is replaced
 * with `navigateTo` so the redirect works in both SSR and CSR contexts.
 *
 * @example
 * ```vue
 * <ThunderIDSignUp @complete="onComplete" @error="onError" />
 * ```
 */
const SignUp: Component = defineComponent({
  name: 'SignUp',
  props: {
    afterSignUpUrl: {default: undefined, type: String},
    buttonClassName: {default: '', type: String},
    className: {default: '', type: String},
    errorClassName: {default: '', type: String},
    inputClassName: {default: '', type: String},
    messageClassName: {default: '', type: String},
    onComplete: {default: undefined, type: Function as PropType<(response: EmbeddedFlowExecuteResponse) => void>},
    onError: {default: undefined, type: Function as PropType<(error: Error) => void>},
    shouldRedirectAfterSignUp: {default: true, type: Boolean},
    showSubtitle: {default: true, type: Boolean},
    showTitle: {default: true, type: Boolean},
    size: {default: 'medium', type: String as PropType<'small' | 'medium' | 'large'>},
    variant: {default: 'outlined', type: String as PropType<'elevated' | 'outlined' | 'flat'>},
  },
  setup(props: any, {slots}: SetupContext): () => VNode | null {
    const {signUp, isInitialized, applicationId} = useThunderID();

    const handleInitialize = async (
      payload?: EmbeddedFlowExecuteRequestPayload,
    ): Promise<EmbeddedFlowExecuteResponse> => {
      // Guard URL parsing — `window` is only available on the client.
      let applicationIdFromUrl: string | null = null;
      if (import.meta.client) {
        const urlParams: URLSearchParams = new URL(window.location.href).searchParams;
        applicationIdFromUrl = urlParams.get('applicationId');
      }
      const effectiveApplicationId: string | undefined = applicationId || applicationIdFromUrl || undefined;

      // Use the EmbeddedFlowType enum (value: 'REGISTRATION') — passing the
      // string 'Registration' (the enum member *name*) is rejected by
      // ThunderID's flow API and yields an empty `data.components` response,
      // which makes BaseSignUp fall through to the "form is not available"
      // alert. Matches Next.js SignUp.tsx exactly.
      const initialPayload: any = payload || {
        flowType: EmbeddedFlowType.Registration,
        ...(effectiveApplicationId && {applicationId: effectiveApplicationId}),
      };

      return (await signUp(initialPayload)) as EmbeddedFlowExecuteResponse;
    };

    const handleOnSubmit = async (payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> =>
      (await signUp(payload)) as EmbeddedFlowExecuteResponse;

    const handleComplete = async (response: EmbeddedFlowExecuteResponse): Promise<void> => {
      props.onComplete?.(response);

      const oauthRedirectUrl: string | undefined = (response as any)?.redirectUrl;
      if (props.shouldRedirectAfterSignUp && oauthRedirectUrl) {
        // Use navigateTo instead of window.location.href — SSR-safe.
        await navigateTo(oauthRedirectUrl, {external: true});
        return;
      }

      if (
        props.shouldRedirectAfterSignUp &&
        response?.type !== EmbeddedFlowResponseType.Redirection &&
        props.afterSignUpUrl
      ) {
        await navigateTo(props.afterSignUpUrl, {external: true});
      }

      if (
        props.shouldRedirectAfterSignUp &&
        response?.type === EmbeddedFlowResponseType.Redirection &&
        response?.data?.redirectURL &&
        !response.data.redirectURL.includes('oauth') &&
        !response.data.redirectURL.includes('auth')
      ) {
        await navigateTo(response.data.redirectURL, {external: true});
      }
    };

    return (): VNode | null =>
      h(
        BaseSignUp,
        {
          afterSignUpUrl: props.afterSignUpUrl,
          buttonClassName: props.buttonClassName,
          className: props.className,
          errorClassName: props.errorClassName,
          inputClassName: props.inputClassName,
          isInitialized: isInitialized?.value ?? false,
          messageClassName: props.messageClassName,
          onComplete: handleComplete,
          onError: props.onError,
          onInitialize: handleInitialize,
          onSubmit: handleOnSubmit,
          showSubtitle: props.showSubtitle,
          showTitle: props.showTitle,
          size: props.size,
          variant: props.variant,
        },
        slots['default'] ? {default: (renderProps: any) => slots['default']!(renderProps)} : undefined,
      );
  },
});

export default SignUp;
