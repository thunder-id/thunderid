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

import {type Organization as IOrganization, withVendorCSSClassPrefix} from '@thunderid/browser';
import {type Component, type PropType, type VNode, defineComponent, h} from 'vue';
import {BuildingIcon} from '../../primitives/Icons';
import Spinner from '../../primitives/Spinner';
import Typography from '../../primitives/Typography';

/**
 * BaseOrganizationList — unstyled list of organizations.
 */
const BaseOrganizationList: Component = defineComponent({
  name: 'BaseOrganizationList',
  inheritAttrs: false,
  props: {
    className: {default: '', type: String},
    isLoading: {default: false, type: Boolean},
    onSelect: {default: undefined, type: Function as PropType<(org: IOrganization) => void>},
    organizations: {default: () => [], type: Array as PropType<IOrganization[]>},
  },
  setup(
    props: {
      className: string;
      isLoading: boolean;
      onSelect?: (org: IOrganization) => void;
      organizations: IOrganization[];
    },
    {slots}: {slots: any},
  ): () => VNode | VNode[] | null {
    return (): VNode | VNode[] | null => {
      if (slots.default) {
        return slots.default({isLoading: props.isLoading, organizations: props.organizations});
      }

      const prefix: typeof withVendorCSSClassPrefix = withVendorCSSClassPrefix;
      const children: VNode[] = [];

      if (props.isLoading) {
        children.push(h('div', {class: prefix('organization-list__loading')}, [h(Spinner)]));
      } else if (props.organizations.length === 0) {
        children.push(
          h(Typography, {class: prefix('organization-list__empty'), variant: 'body2'}, () => 'No organizations found'),
        );
      } else {
        props.organizations.forEach((org: IOrganization) => {
          children.push(
            h(
              'button',
              {
                class: prefix('organization-list__item'),
                key: org.id,
                onClick: () => props.onSelect?.(org),
                type: 'button',
              },
              [h(BuildingIcon, {size: 16}), h(Typography, {variant: 'body1'}, () => org.name || org.id)],
            ),
          );
        });
      }

      return h('div', {class: [prefix('organization-list'), props.className].filter(Boolean).join(' ')}, children);
    };
  },
});

export default BaseOrganizationList;
