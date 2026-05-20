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
  type EmbeddedSignInFlowHandleRequestPayload,
  type EmbeddedSignInFlowHandleResponse,
  type EmbeddedSignInFlowInitiateResponse,
} from '@thunderid/browser';
import {BaseSignIn} from '@thunderid/vue';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import {navigateTo} from '#app';
import {useThunderID} from '#imports';

/**
 * Nuxt-specific SignIn container for the embedded (app-native) sign-in flow.
 *
 * Mirrors the Vue SDK's `SignIn` container but replaces all `window.location`
 * navigation with Nuxt's `navigateTo` so redirects after a successful embedded
 * sign-in are SSR-safe.
 *
 * Uses `useThunderID()` from the Nuxt auto-import layer — the Nuxt-specific
 * wrapper that provides Nitro-route-aware `signIn`, `signOut`, `signUp`.
 *
 * Delegates all UI rendering to {@link BaseSignIn} from `@thunderid/vue`, which
 * itself is platform-aware (routes to V1 authenticator or V2 component flow).
 *
 * @example
 * ```vue
 * <ThunderIDSignIn @success="onSignIn" @error="onError" />
 * ```
 */
const SignIn: Component = defineComponent({
  emits: ['error', 'success'],
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
  setup(
    props: Readonly<{className: string; size: 'small' | 'medium' | 'large'; variant: 'elevated' | 'outlined' | 'flat'}>,
    {emit, attrs}: SetupContext,
  ): () => VNode {
    const {signIn, afterSignInUrl, isInitialized, isLoading} = useThunderID();

    const handleInitialize = async (): Promise<EmbeddedSignInFlowInitiateResponse> =>
      // Pass flowId='' to trigger the embedded-flow initiation path in useThunderID.
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (await signIn({flowId: ''} as any, {} as any)) as EmbeddedSignInFlowInitiateResponse;

    const handleOnSubmit = async (
      payload: EmbeddedSignInFlowHandleRequestPayload,
      request: any,
    ): Promise<EmbeddedSignInFlowHandleResponse> =>
      (await signIn(payload, request)) as EmbeddedSignInFlowHandleResponse;

    const handleSuccess = async (authData: Record<string, any>): Promise<void> => {
      emit('success', authData);

      if (authData && afterSignInUrl) {
        if (import.meta.client) {
          // Build the full URL with auth data params (client-only: needs window.location.origin).
          const url: URL = new URL(afterSignInUrl as string, window.location.origin);
          Object.entries(authData).forEach(([key, value]: [string, any]) => {
            if (value !== undefined && value !== null) {
              url.searchParams.append(key, String(value));
            }
          });
          await navigateTo(url.pathname + url.search + url.hash);
        } else {
          // On SSR, just navigate to the base afterSignInUrl (no auth data params).
          await navigateTo(afterSignInUrl as string);
        }
      }
    };

    return (): VNode =>
      h(BaseSignIn, {
        ...attrs,
        afterSignInUrl,
        class: props.className,
        isLoading: (isLoading?.value ?? false) || !(isInitialized?.value ?? true),
        onError: (err: Error) => emit('error', err),
        onInitialize: handleInitialize,
        onSubmit: handleOnSubmit,
        onSuccess: handleSuccess,
        showLogo: true,
        showSubtitle: true,
        showTitle: true,
        size: props.size,
        variant: props.variant,
      });
  },
});

export default SignIn;
