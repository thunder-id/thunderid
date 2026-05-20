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

import {ThunderIDRuntimeError, navigate} from '@thunderid/browser';
import {defineComponent, h, ref, type Component, type PropType, type Ref, type SetupContext, type VNode} from 'vue';
import BaseSignInButton from './BaseSignInButton';
import useThunderID from '../../composables/useThunderID';

/**
 * SignInButton — triggers `signIn()` from the ThunderID context.
 *
 * If a custom `signInUrl` is configured, navigates to it instead.
 * Falls back to i18n translation for the button text.
 */
const SignInButton: Component = defineComponent({
  name: 'SignInButton',
  props: {
    signInOptions: {default: undefined, type: Object as PropType<Record<string, any>>},
  },
  emits: ['click', 'error'],
  setup(props: {signInOptions?: Record<string, any>}, {slots, emit, attrs}: SetupContext): () => VNode {
    const {signIn, signInUrl, signInOptions: contextSignInOptions} = useThunderID();
    const isLoading: Ref<boolean> = ref(false);

    const handleSignIn = async (e?: MouseEvent): Promise<void> => {
      try {
        isLoading.value = true;
        if (signInUrl) {
          navigate(signInUrl);
        } else {
          await signIn(props.signInOptions ?? contextSignInOptions);
        }
        if (e) emit('click', e);
      } catch (error) {
        emit('error', error);
        throw new ThunderIDRuntimeError(
          `Sign in failed: ${error instanceof Error ? error.message : String(error)}`,
          'SignInButton-handleSignIn-RuntimeError-001',
          'vue',
          'Something went wrong while trying to sign in. Please try again later.',
        );
      } finally {
        isLoading.value = false;
      }
    };

    return (): VNode => {
      const slotContent: (() => VNode[]) | undefined = slots['default']
        ? (): VNode[] => slots['default']({isLoading: isLoading.value})
        : undefined;

      return h(
        BaseSignInButton,
        {
          class: attrs.class,
          isLoading: isLoading.value,
          onClick: handleSignIn,
          style: attrs.style,
        },
        slotContent,
      );
    };
  },
});

export default SignInButton;
