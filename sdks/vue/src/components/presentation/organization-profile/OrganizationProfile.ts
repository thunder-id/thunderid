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
import {type Component, type PropType, type VNode, defineComponent, h} from 'vue';
import BaseOrganizationProfile from './BaseOrganizationProfile';
import useOrganization from '../../../composables/useOrganization';

/**
 * OrganizationProfile — styled organisation details component.
 *
 * Retrieves current organization from context and delegates to BaseOrganizationProfile.
 */
const OrganizationProfile: Component = defineComponent({
  name: 'OrganizationProfile',
  props: {
    className: {default: '', type: String},
    editable: {default: false, type: Boolean},
    onUpdate: {
      default: undefined,
      type: Function as PropType<(payload: Record<string, unknown>) => Promise<void>>,
    },
    title: {default: 'Organization Profile', type: String},
  },
  setup(
    props: {
      className: string;
      editable: boolean;
      onUpdate?: (payload: Record<string, unknown>) => Promise<void>;
      title: string;
    },
    {slots}: {slots: any},
  ): () => VNode | VNode[] | null {
    const {currentOrganization} = useOrganization();

    return (): VNode | VNode[] | null =>
      h(
        BaseOrganizationProfile,
        {
          class: withVendorCSSClassPrefix('organization-profile--styled'),
          className: props.className,
          editable: props.editable,
          onUpdate: props.onUpdate,
          organization: currentOrganization?.value ?? null,
          title: props.title,
        },
        slots,
      );
  },
});

export default OrganizationProfile;
