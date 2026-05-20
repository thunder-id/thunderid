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
import {type Component, type VNode, defineComponent, h} from 'vue';
import BaseOrganizationList from './BaseOrganizationList';
import useOrganization from '../../../composables/useOrganization';

/**
 * OrganizationList — styled organization list component.
 *
 * Retrieves organization list from context and delegates to BaseOrganizationList.
 */
const OrganizationList: Component = defineComponent({
  name: 'OrganizationList',
  props: {
    className: {
      default: '',
      type: String,
    },
  },
  emits: ['select'],
  setup(props: {className: string}, {slots, emit}: {emit: any; slots: any}): () => VNode | VNode[] | null {
    const {myOrganizations, isLoading, switchOrganization} = useOrganization();

    const handleSelect = async (org: IOrganization): Promise<void> => {
      emit('select', org);
      await switchOrganization(org);
    };

    return (): VNode | VNode[] | null =>
      h(
        BaseOrganizationList,
        {
          class: withVendorCSSClassPrefix('organization-list--styled'),
          className: props.className,
          isLoading: isLoading.value,
          onSelect: handleSelect,
          organizations: myOrganizations.value,
        },
        slots,
      );
  },
});

export default OrganizationList;
