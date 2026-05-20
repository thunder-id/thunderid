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
  EmbeddedSignInFlowHandleRequestPayload,
  EmbeddedSignInFlowHandleResponse,
  EmbeddedSignInFlowInitiateResponse,
} from '@thunderid/browser';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import BaseSignIn from './BaseSignIn';
import useThunderID from '../../../../composables/useThunderID';

/**
 * V1 SignIn — app-native sign-in component using the authenticator-based flow.
 *
 * Initialises the flow with `signIn({ response_mode: 'direct' })` and delegates
 * all UI rendering to `BaseSignInV1`.
 */
const SignIn: Component = defineComponent({
  name: 'SignInV1',
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
    {emit, attrs}: SetupContext,
  ): () => VNode {
    const {signIn, afterSignInUrl, isInitialized, isLoading} = useThunderID();

    const handleInitialize = async (): Promise<EmbeddedSignInFlowInitiateResponse> =>
      (await signIn({response_mode: 'direct'})) as EmbeddedSignInFlowInitiateResponse;

    const handleOnSubmit = async (
      payload: EmbeddedSignInFlowHandleRequestPayload,
      request: any,
    ): Promise<EmbeddedSignInFlowHandleResponse> =>
      (await signIn(payload, request)) as EmbeddedSignInFlowHandleResponse;

    const handleSuccess = (authData: Record<string, any>): void => {
      emit('success', authData);

      if (authData && afterSignInUrl) {
        const url: URL = new URL(afterSignInUrl, window.location.origin);
        Object.entries(authData).forEach(([key, value]: [string, any]) => {
          if (value !== undefined && value !== null) {
            url.searchParams.append(key, String(value));
          }
        });
        window.location.href = url.toString();
      }
    };

    return (): VNode =>
      h(BaseSignIn, {
        ...attrs,
        afterSignInUrl,
        class: props.className,
        isLoading: isLoading.value || !isInitialized.value,
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
