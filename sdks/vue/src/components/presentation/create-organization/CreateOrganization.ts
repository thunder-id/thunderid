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
import BaseCreateOrganization from './BaseCreateOrganization';
import useOrganization from '../../../composables/useOrganization';

interface CreateOrganizationSetupProps {
  className: string;
  description: string;
  title: string;
}

/**
 * CreateOrganization — styled sub-organisation creation component.
 *
 * Retrieves createOrganization from context and delegates to BaseCreateOrganization.
 */
const CreateOrganization: Component = defineComponent({
  name: 'CreateOrganization',
  props: {
    className: {default: '', type: String},
    description: {default: 'Create a new sub-organization.', type: String},
    title: {default: 'Create Organization', type: String},
  },
  setup(props: CreateOrganizationSetupProps, {slots}: {slots: any}): () => VNode {
    const {createOrganization} = useOrganization();

    return (): VNode =>
      h(
        BaseCreateOrganization,
        {
          class: withVendorCSSClassPrefix('create-organization--styled'),
          className: props.className,
          description: props.description,
          onCreate: createOrganization
            ? async (name: string): Promise<void> => {
                await createOrganization({description: '', name, parentId: '', type: 'TENANT'}, '');
              }
            : undefined,
          title: props.title,
        },
        slots,
      );
  },
});

export default CreateOrganization;
