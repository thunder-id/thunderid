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
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowResponseType,
  EmbeddedFlowType,
} from '@thunderid/browser';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import BaseSignUp from './BaseSignUp';
import type {BaseSignUpRenderProps} from './BaseSignUp';
import useThunderID from '../../../../composables/useThunderID';

export type SignUpRenderProps = BaseSignUpRenderProps;

/**
 * SignUp — embedded sign-up component that handles API calls and delegates UI to BaseSignUp.
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
      const urlParams: URLSearchParams = new URL(window.location.href).searchParams;
      const applicationIdFromUrl: string | null = urlParams.get('applicationId');
      const effectiveApplicationId: string | undefined = applicationId || applicationIdFromUrl || undefined;

      const initialPayload: any = payload || {
        flowType: EmbeddedFlowType.Registration,
        ...(effectiveApplicationId && {applicationId: effectiveApplicationId}),
      };

      return (await signUp(initialPayload)) as EmbeddedFlowExecuteResponse;
    };

    const handleOnSubmit = async (payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> =>
      (await signUp(payload)) as EmbeddedFlowExecuteResponse;

    const handleComplete = (response: EmbeddedFlowExecuteResponse): void => {
      props.onComplete?.(response);

      const oauthRedirectUrl: string | undefined = (response as any)?.redirectUrl;
      if (props.shouldRedirectAfterSignUp && oauthRedirectUrl) {
        window.location.href = oauthRedirectUrl;
        return;
      }

      if (
        props.shouldRedirectAfterSignUp &&
        response?.type !== EmbeddedFlowResponseType.Redirection &&
        props.afterSignUpUrl
      ) {
        window.location.href = props.afterSignUpUrl;
      }

      if (
        props.shouldRedirectAfterSignUp &&
        response?.type === EmbeddedFlowResponseType.Redirection &&
        response?.data?.redirectURL &&
        !response.data.redirectURL.includes('oauth') &&
        !response.data.redirectURL.includes('auth')
      ) {
        window.location.href = response.data.redirectURL;
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
        slots['default'] ? {default: (renderProps: any) => slots['default'](renderProps)} : undefined,
      );
  },
});

export default SignUp;
