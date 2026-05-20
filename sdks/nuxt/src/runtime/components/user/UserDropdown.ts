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
import {BaseUserDropdown, UserProfile as UserProfileComponent} from '@thunderid/vue';
import {type Component, type Ref, type VNode, defineComponent, h, ref} from 'vue';
import {useThunderID, useUser} from '#imports';

/**
 * Nuxt-specific UserDropdown container.
 *
 * Reads `user` and `signOut` from `useThunderID()` (Nuxt auto-import) and
 * profile data from `useUser()`, then delegates rendering to
 * {@link BaseUserDropdown} from `@thunderid/vue`.
 *
 * The `signOut` action comes from the Nuxt plugin's THUNDERID_KEY so it uses
 * `navigateTo` for the redirect instead of `window.location`.
 *
 * The embedded profile modal renders the Nuxt-specific `UserProfile` so that
 * profile update handlers are also wired through the Nuxt auto-import layer.
 *
 * @example
 * ```vue
 * <ThunderIDUserDropdown />
 * ```
 */
const UserDropdown: Component = defineComponent({
  emits: ['profileClick'],
  name: 'UserDropdown',
  props: {
    className: {default: '', type: String},
  },
  setup(props: {className: string}, {slots, emit}: {emit: any; slots: any}): () => VNode | VNode[] | null {
    const {user, signOut} = useThunderID();
    useUser();
    const isProfileModalOpen: Ref<boolean> = ref(false);

    return (): VNode | VNode[] | null =>
      h(
        BaseUserDropdown,
        {
          class: withVendorCSSClassPrefix('user-dropdown--styled'),
          className: props.className,
          isProfileModalOpen: isProfileModalOpen.value,
          onProfileClick: (): void => {
            isProfileModalOpen.value = true;
            emit('profileClick');
          },
          onProfileModalClose: (): void => {
            isProfileModalOpen.value = false;
          },
          onSignOut: (): void => {
            // signOut from the Nuxt plugin uses navigateTo — SSR-safe.
            signOut();
          },
          // Inline profile content avoids creating a circular dependency on the
          // Nuxt UserProfile container; UserProfileComponent from @thunderid/vue
          // reads its data from the OrganizationProvider / UserProvider context
          // wired up by ThunderIDRoot, so it works identically.
          profileContent: isProfileModalOpen.value
            ? h(UserProfileComponent, {
                cardLayout: false,
                editable: true,
              })
            : null,
          user: user.value,
        },
        slots,
      );
  },
});

export default UserDropdown;
