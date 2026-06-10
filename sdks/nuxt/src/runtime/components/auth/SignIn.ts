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

import {navigateTo} from '#app';
import {EmbeddedFlowType} from '@thunderid/browser';
import {BaseSignIn} from '@thunderid/vue';
import {type Component, type PropType, type Ref, type SetupContext, type VNode, defineComponent, h, onMounted, ref} from 'vue';
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
 * Delegates all UI rendering to {@link BaseSignIn} from `@thunderid/vue`.
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
    const {signIn, afterSignInUrl, applicationId, scopes} = useThunderID();
    const components: Ref<any[]> = ref([]);
    const isFlowLoading: Ref<boolean> = ref(true);

    onMounted(async () => {
      try {
        const response: any = await signIn({
          flowType: EmbeddedFlowType.Authentication,
          ...(applicationId && {applicationId}),
          ...(scopes && {scopes}),
        });
        if (response?.meta?.components) {
          components.value = response.meta.components;
        } else if (response?.data?.meta?.components) {
          components.value = response.data.meta.components;
        }
      } catch (err) {
        emit('error', err instanceof Error ? err : new Error(String(err)));
      } finally {
        isFlowLoading.value = false;
      }
    });

    const handleOnSubmit = async (payload: any, request: any): Promise<any> =>
      (await signIn(payload, request)) as any;

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
        class: props.className,
        components: components.value,
        isLoading: isFlowLoading.value,
        onError: (err: Error) => emit('error', err),
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
