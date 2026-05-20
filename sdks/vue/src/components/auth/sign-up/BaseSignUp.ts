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
import BaseSignUpV1 from './v1/BaseSignUp';
import BaseSignUpV2 from './v2/BaseSignUp';
import useThunderID from '../../../composables/useThunderID';

export type {BaseSignUpRenderProps, BaseSignUpProps} from './v2/BaseSignUp';

/**
 * BaseSignUp — platform-aware base sign-up component.
 *
 * Routes to V1 (component-driven, V1 flow API: `TYPOGRAPHY` / `INPUT` /
 * `BUTTON` / `FORM` shapes) or V2 (`ThunderIDV2` platform with `BLOCK` / `STACK`
 * / `TEXT_INPUT` shapes) based on the `platform` value resolved by
 * {@link useThunderID}.
 *
 * Mirrors the React `BaseSignUp` dispatcher and matches the existing pattern
 * already used by `BaseSignIn` in this package.
 */
const BaseSignUp: Component = defineComponent({
  name: 'BaseSignUp',
  inheritAttrs: false,
  setup(_props: Record<string, unknown>, {attrs, slots}: SetupContext): () => VNode {
    const {platform} = useThunderID();

    return (): VNode => {
      if (platform === Platform.ThunderID) {
        return h(BaseSignUpV2, {...attrs}, slots);
      }
      return h(BaseSignUpV1, {...attrs}, slots);
    };
  },
});

export default BaseSignUp;
