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
import {type Component, type SetupContext, type VNode, defineComponent, h} from 'vue';
import SignUpV1 from './v1/SignUp';
import SignUpV2 from './v2/SignUp';
import useThunderID from '../../../composables/useThunderID';

export type {SignUpRenderProps} from './v2/SignUp';

/**
 * SignUp — platform-aware sign-up container.
 *
 * Routes to V1 (default, component-driven V1 flow API) or V2 (`ThunderIDV2`
 * platform) based on the `platform` value from {@link useThunderID}. Mirrors
 * the existing `SignIn` dispatcher pattern in this package.
 */
const SignUp: Component = defineComponent({
  name: 'SignUp',
  inheritAttrs: false,
  setup(_props: Record<string, unknown>, {attrs, slots}: SetupContext): () => VNode {
    const {platform} = useThunderID();

    return (): VNode => {
      if (platform === Platform.ThunderID) {
        return h(SignUpV2, {...attrs}, slots);
      }
      return h(SignUpV1, {...attrs}, slots);
    };
  },
});

export default SignUp;
