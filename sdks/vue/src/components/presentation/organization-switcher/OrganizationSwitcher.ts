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
import {type Component, type VNode, defineComponent, h} from 'vue';
import BaseOrganizationSwitcher from './BaseOrganizationSwitcher';
import useOrganization from '../../../composables/useOrganization';

/**
 * OrganizationSwitcher — styled organisation switcher component.
 *
 * Retrieves organisations from context and delegates to BaseOrganizationSwitcher.
 */
const OrganizationSwitcher: Component = defineComponent({
  name: 'OrganizationSwitcher',
  props: {
    className: {default: '', type: String},
  },
  setup(props: {className: string}, {slots}: {slots: any}): () => VNode | VNode[] | null {
    const {currentOrganization, myOrganizations, isLoading, switchOrganization} = useOrganization();

    return (): VNode | VNode[] | null =>
      h(
        BaseOrganizationSwitcher,
        {
          class: withVendorCSSClassPrefix('organization-switcher--styled'),
          className: props.className,
          currentOrganization: currentOrganization?.value ?? null,
          isLoading: isLoading?.value ?? false,
          onSwitch: switchOrganization,
          organizations: myOrganizations?.value ?? [],
        },
        slots,
      );
  },
});

export default OrganizationSwitcher;
