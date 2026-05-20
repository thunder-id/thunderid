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
import {defineComponent, h, ref, type Component, type Ref, type SetupContext, type VNode} from 'vue';
import BaseSignUpButton from './BaseSignUpButton';
import useThunderID from '../../composables/useThunderID';

/**
 * SignUpButton — triggers `signUp()` from the ThunderID context.
 *
 * If a custom `signUpUrl` is configured, navigates to it instead.
 * Falls back to i18n translation for the button text.
 */
const SignUpButton: Component = defineComponent({
  name: 'SignUpButton',
  emits: ['click', 'error'],
  setup(_: {}, {slots, emit, attrs}: SetupContext): () => VNode {
    const {signUp, signUpUrl} = useThunderID();
    const isLoading: Ref<boolean> = ref(false);

    const handleSignUp = async (e?: MouseEvent): Promise<void> => {
      try {
        isLoading.value = true;
        if (signUpUrl) {
          navigate(signUpUrl);
        } else {
          await signUp();
        }
        if (e) emit('click', e);
      } catch (error) {
        emit('error', error);
        throw new ThunderIDRuntimeError(
          `Sign up failed: ${error instanceof Error ? error.message : String(error)}`,
          'SignUpButton-handleSignUp-RuntimeError-001',
          'vue',
          'Something went wrong while trying to sign up. Please try again later.',
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
        BaseSignUpButton,
        {
          class: attrs.class,
          isLoading: isLoading.value,
          onClick: handleSignUp,
          style: attrs.style,
        },
        slotContent,
      );
    };
  },
});

export default SignUpButton;
