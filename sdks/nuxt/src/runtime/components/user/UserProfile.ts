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

import {withVendorCSSClassPrefix} from '@thunderid/browser';
import {BaseUserProfile} from '@thunderid/vue';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import {useUser} from '#imports';

/**
 * Nuxt-specific UserProfile container.
 *
 * Reads user profile data from `useUser()` (Nuxt auto-import, re-exported
 * from `@thunderid/vue`) and delegates rendering to {@link BaseUserProfile}
 * from `@thunderid/vue`.
 *
 * Preserves the same prop/slot API as the Vue SDK's `UserProfile` component
 * so consumers don't need to change their templates.
 *
 * @example
 * ```vue
 * <ThunderIDUserProfile :editable="true" title="My Profile" />
 * ```
 */
const UserProfile: Component = defineComponent({
  name: 'UserProfile',
  props: {
    cardLayout: {default: true, type: Boolean},
    className: {default: '', type: String},
    editable: {default: true, type: Boolean},
    hideFields: {default: () => [], type: Array as PropType<string[]>},
    showFields: {default: () => [], type: Array as PropType<string[]>},
    title: {default: 'Profile', type: String},
  },
  setup(
    props: Readonly<{
      cardLayout: boolean;
      className: string;
      editable: boolean;
      hideFields: string[];
      showFields: string[];
      title: string;
    }>,
    {slots}: SetupContext,
  ): () => VNode {
    const {flattenedProfile, schemas, updateProfile} = useUser();

    return (): VNode =>
      h(
        BaseUserProfile,
        {
          cardLayout: props.cardLayout,
          class: withVendorCSSClassPrefix('user-profile--styled'),
          className: props.className,
          editable: props.editable,
          flattenedProfile: flattenedProfile?.value,
          hideFields: props.hideFields,
          onUpdate: updateProfile,
          schemas: schemas?.value,
          showFields: props.showFields,
          title: props.title,
        },
        slots,
      );
  },
});

export default UserProfile;
