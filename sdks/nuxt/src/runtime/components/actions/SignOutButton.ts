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
import {BaseSignOutButton} from '@thunderid/vue';
import {type Component, type Ref, type SetupContext, type VNode, defineComponent, h, ref} from 'vue';
import {useThunderID} from '#imports';

/**
 * Nuxt-specific SignOutButton container.
 *
 * Imports {@link BaseSignOutButton} from `@thunderid/vue` and wires the
 * `signOut` action through the Nuxt-specific {@link useThunderID} composable
 * (auto-import layer), which uses Nuxt's `navigateTo` instead of
 * `window.location`.
 *
 * @example
 * ```vue
 * <ThunderIDSignOutButton />
 * <ThunderIDSignOutButton class="btn-secondary">Sign out</ThunderIDSignOutButton>
 * ```
 */
const SignOutButton: Component = defineComponent({
  emits: ['click', 'error'],
  name: 'SignOutButton',
  setup(_: {}, {slots, emit, attrs}: SetupContext): () => VNode {
    const {signOut} = useThunderID();
    const isLoading: Ref<boolean> = ref(false);

    const handleSignOut = async (e?: MouseEvent): Promise<void> => {
      try {
        isLoading.value = true;
        // signOut comes from the Nuxt plugin's THUNDERID_KEY which uses navigateTo.
        await signOut();
        if (e) emit('click', e);
      } catch (error) {
        emit('error', error);
        throw new ThunderIDRuntimeError(
          `Sign out failed: ${error instanceof Error ? error.message : String(error)}`,
          'SignOutButton-handleSignOut-RuntimeError-001',
          'nuxt',
          'Something went wrong while trying to sign out. Please try again later.',
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
        BaseSignOutButton,
        {
          class: attrs['class'],
          isLoading: isLoading.value,
          onClick: handleSignOut,
          style: attrs['style'],
        },
        slotContent,
      );
    };
  },
});

export default SignOutButton;
