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

import {Platform} from '@thunderid/browser';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import SignInV1 from './v1/SignIn';
import SignInV2 from './v2/SignIn';
import useThunderID from '../../../composables/useThunderID';

export type {SignInRenderProps} from './v2/SignIn';

/**
 * SignIn — platform-aware sign-in component.
 *
 * Routes to the V1 (authenticator-based) flow by default or the V2
 * (component-driven) flow when `platform` is set to `Platform.ThunderID`.
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
  ): () => VNode {
    const {platform} = useThunderID();

    return (): VNode => {
      if (platform === Platform.ThunderID) {
        return h(
          SignInV2,
          {
            ...attrs,
            class: props.className,
            onError: (err: Error) => emit('error', err),
            onSuccess: (data: Record<string, any>) => emit('success', data),
            size: props.size,
            variant: props.variant,
          },
          slots,
        );
      }

      return h(
        SignInV1,
        {
          ...attrs,
          class: props.className,
          onError: (err: Error) => emit('error', err),
          onSuccess: (data: Record<string, any>) => emit('success', data),
          size: props.size,
          variant: props.variant,
        },
        slots,
      );
    };
  },
});

export default SignIn;
