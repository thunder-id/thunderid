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
import {type Component, type PropType, type Ref, type VNode, defineComponent, h, ref} from 'vue';
import BaseUserDropdown, {type DropdownMenuItem} from './BaseUserDropdown';
import useThunderID from '../../../composables/useThunderID';
import UserProfileComponent from '../user-profile/UserProfile';

/**
 * UserDropdown — avatar button that opens a user identity menu.
 *
 * @example Default usage
 * ```vue
 * <UserDropdown />
 * ```
 *
 * @example With custom menu items and a separator
 * ```vue
 * <UserDropdown
 *   :menu-items="[
 *     { label: 'Settings', icon: h(SettingsIcon, { size: 15 }), onClick: goToSettings },
 *     { label: 'Help',     onClick: openHelp, separatorBefore: true },
 *   ]"
 * />
 * ```
 *
 * @example Small, left-aligned, no chevron
 * ```vue
 * <UserDropdown size="sm" menu-align="left" />
 * ```
 */
const UserDropdown: Component = defineComponent({
  name: 'UserDropdown',
  props: {
    /** Extra CSS class added to the root element. */
    className: {default: '', type: String},
    /**
     * How to align the dropdown panel relative to the trigger button.
     * - `'auto'` (default) — picks the side with more available viewport space at open time.
     * - `'left'` — panel left edge aligns with trigger left edge.
     * - `'right'` — panel right edge aligns with trigger right edge.
     */
    menuAlign: {
      default: 'auto',
      type: String as PropType<'auto' | 'left' | 'right'>,
    },
    /**
     * Extra items inserted between the Profile link and Sign Out.
     * Set `separatorBefore: true` on any item to add a divider line before it.
     * Set `danger: true` for destructive actions (red styling).
     * Pass an `icon` VNode to render an icon to the left of the label.
     */
    menuItems: {
      default: undefined,
      type: Array as PropType<DropdownMenuItem[]>,
    },
    /** Whether to show the animated down-chevron beside the avatar. Default `false`. */
    showChevron: {default: false, type: Boolean},
    /**
     * Overall density / avatar size of the component.
     * - `'sm'` — 28 px avatar, compact menu (180 px min-width).
     * - `'md'` (default) — 32 px avatar, standard menu (220 px min-width).
     * - `'lg'` — 38 px avatar, spacious menu (280 px min-width).
     */
    size: {
      default: 'md',
      type: String as PropType<'sm' | 'md' | 'lg'>,
    },
  },
  emits: ['profileClick'],
  setup(
    props: {
      className: string;
      menuAlign: 'auto' | 'left' | 'right';
      menuItems?: DropdownMenuItem[];
      showChevron: boolean;
      size: 'sm' | 'md' | 'lg';
    },
    {slots, emit}: {emit: any; slots: any},
  ): () => VNode | VNode[] | null {
    const {user, signOut} = useThunderID();
    const isProfileModalOpen: Ref<boolean> = ref(false);

    return (): VNode | VNode[] | null =>
      h(
        BaseUserDropdown,
        {
          class: withVendorCSSClassPrefix('user-dropdown--styled'),
          className: props.className,
          isProfileModalOpen: isProfileModalOpen.value,
          menuAlign: props.menuAlign,
          menuItems: props.menuItems,
          onProfileClick: (): void => {
            isProfileModalOpen.value = true;
            emit('profileClick');
          },
          onProfileModalClose: (): void => {
            isProfileModalOpen.value = false;
          },
          onSignOut: (): void => {
            signOut();
          },
          profileContent: isProfileModalOpen.value
            ? h(UserProfileComponent, {
                cardLayout: false,
                compact: true,
                editable: true,
              })
            : null,
          showChevron: props.showChevron,
          size: props.size,
          user: user.value,
        },
        slots,
      );
  },
});

export default UserDropdown;
