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
import type {Organization} from '@thunderid/browser';
import {type Component, type PropType, type Ref, type VNode, defineComponent, h, ref} from 'vue';
import Card from '../../primitives/Card';
import {BuildingIcon, ChevronDownIcon} from '../../primitives/Icons';
import Spinner from '../../primitives/Spinner';
import Typography from '../../primitives/Typography';

const cls = (name: string): string => withVendorCSSClassPrefix(`organization-switcher${name}`);

/**
 * BaseOrganizationSwitcher — unstyled organisation dropdown switcher.
 *
 * Shows the current organization name and a dropdown list to switch.
 */
const BaseOrganizationSwitcher: Component = defineComponent({
  name: 'BaseOrganizationSwitcher',
  inheritAttrs: false,
  props: {
    className: {default: '', type: String},
    currentOrganization: {default: null, type: Object as PropType<Organization | null>},
    isLoading: {default: false, type: Boolean},
    onSwitch: {default: undefined, type: Function as PropType<(org: Organization) => void>},
    organizations: {default: () => [], type: Array as PropType<Organization[]>},
  },
  setup(
    props: {
      className: string;
      currentOrganization: Organization | null;
      isLoading: boolean;
      onSwitch?: (org: Organization) => void;
      organizations: Organization[];
    },
    {slots}: {slots: any},
  ): () => VNode | VNode[] | null {
    const isOpen: Ref<boolean> = ref(false);

    const toggle = (): void => {
      isOpen.value = !isOpen.value;
    };

    const handleSelect = (org: Organization): void => {
      isOpen.value = false;
      props.onSwitch?.(org);
    };

    return (): VNode | VNode[] | null => {
      if (slots.default) {
        return slots.default({
          currentOrganization: props.currentOrganization,
          handleSelect,
          isLoading: props.isLoading,
          isOpen: isOpen.value,
          organizations: props.organizations,
          toggle,
        });
      }

      const currentName: string = props.currentOrganization?.name ?? 'No Organization';

      const triggerButton: VNode = h(
        'button',
        {
          'aria-expanded': isOpen.value,
          'aria-haspopup': 'listbox',
          class: cls('__trigger'),
          onClick: toggle,
          type: 'button',
        },
        [
          h(BuildingIcon, {size: 16}),
          h(Typography, {class: cls('__trigger-label'), variant: 'body2'}, () => currentName),
          h(ChevronDownIcon, {size: 12}),
        ],
      );

      const dropdownChildren: VNode[] = [];

      if (props.isLoading) {
        dropdownChildren.push(h('div', {class: cls('__loading')}, [h(Spinner, {size: 'small'})]));
      } else if (props.organizations.length === 0) {
        dropdownChildren.push(
          h(Typography, {class: cls('__empty'), variant: 'body2'}, () => 'No organizations available'),
        );
      } else {
        props.organizations.forEach((org: Organization) => {
          const isActive: boolean = org.id === props.currentOrganization?.id;
          dropdownChildren.push(
            h(
              'button',
              {
                'aria-selected': isActive,
                class: [cls('__item'), isActive ? cls('__item--active') : ''],
                onClick: () => handleSelect(org),
                role: 'option',
                type: 'button',
              },
              [h(BuildingIcon, {size: 14}), h(Typography, {variant: 'body2'}, () => org.name)],
            ),
          );
        });
      }

      const dropdown: VNode | null = isOpen.value
        ? h('div', {class: cls('__dropdown'), role: 'listbox'}, dropdownChildren)
        : null;

      return h(Card, {class: [cls(''), props.className].filter(Boolean).join(' ')}, () => [triggerButton, dropdown]);
    };
  },
});

export default BaseOrganizationSwitcher;
