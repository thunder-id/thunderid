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

import {ThunderIDRuntimeError} from '@thunderid/browser';
import {BaseSignInButton} from '@thunderid/vue';
import {type Component, type PropType, type Ref, type SetupContext, type VNode, defineComponent, h, ref} from 'vue';
import {navigateTo} from '#app';
import {useThunderID} from '#imports';

/**
 * Nuxt-specific SignInButton container.
 *
 * Mirrors the Next.js SDK's SignInButton: imports {@link BaseSignInButton} from
 * `@thunderid/vue` and wires navigation through Nuxt's `navigateTo` so
 * server-side redirects work correctly (no `window.location` access).
 *
 * The `signIn` action and `signInUrl` come from the Nuxt-specific
 * {@link useThunderID} composable which is provided via the Nuxt auto-import
 * layer — not directly from `@thunderid/vue`.
 *
 * @example
 * ```vue
 * <ThunderIDSignInButton />
 * <ThunderIDSignInButton class="btn-primary">Log in</ThunderIDSignInButton>
 * ```
 */
const SignInButton: Component = defineComponent({
  emits: ['click', 'error'],
  name: 'SignInButton',
  props: {
    signInOptions: {default: undefined, type: Object as PropType<Record<string, any>>},
  },
  setup(props: {signInOptions?: Record<string, any>}, {slots, emit, attrs}: SetupContext): () => VNode {
    const {signIn, signInUrl, signInOptions: contextSignInOptions} = useThunderID();
    const isLoading: Ref<boolean> = ref(false);

    const handleSignIn = async (e?: MouseEvent): Promise<void> => {
      try {
        isLoading.value = true;

        if (signInUrl) {
          // Use Nuxt's navigateTo — SSR-safe, works on both server and client.
          await navigateTo(signInUrl, {external: true});
        } else {
          await signIn(props.signInOptions ?? contextSignInOptions);
        }

        if (e) emit('click', e);
      } catch (error) {
        emit('error', error);
        throw new ThunderIDRuntimeError(
          `Sign in failed: ${error instanceof Error ? error.message : String(error)}`,
          'SignInButton-handleSignIn-RuntimeError-001',
          'nuxt',
          'Something went wrong while trying to sign in. Please try again later.',
        );
      } finally {
        isLoading.value = false;
      }
    };

    return (): VNode => {
      const slotContent: (() => VNode[]) | undefined = slots['default']
        ? (): VNode[] => slots['default']!({isLoading: isLoading.value})
        : undefined;

      return h(
        BaseSignInButton,
        {
          class: attrs['class'],
          isLoading: isLoading.value,
          onClick: handleSignIn,
          style: attrs['style'],
        },
        slotContent,
      );
    };
  },
});

export default SignInButton;
