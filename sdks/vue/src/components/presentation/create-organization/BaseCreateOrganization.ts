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
import Alert from '../../primitives/Alert';
import Button from '../../primitives/Button';
import Card from '../../primitives/Card';
import TextField from '../../primitives/TextField';
import Typography from '../../primitives/Typography';

const cls = (name: string): string => withVendorCSSClassPrefix(`create-organization${name}`);

interface BaseCreateOrganizationSetupProps {
  className: string;
  description: string;
  onCreate?: (name: string) => Promise<void> | void;
  title: string;
}

/**
 * BaseCreateOrganization — unstyled sub-organisation creation form.
 *
 * Provides a form with an org name input and create button.
 */
const BaseCreateOrganization: Component = defineComponent({
  name: 'BaseCreateOrganization',
  props: {
    className: {default: '', type: String},
    description: {default: 'Create a new sub-organization.', type: String},
    onCreate: {default: undefined, type: Function as PropType<(name: string) => Promise<void> | void>},
    title: {default: 'Create Organization', type: String},
  },
  setup(props: BaseCreateOrganizationSetupProps, {slots}: {slots: any}): () => VNode | VNode[] {
    const orgName: Ref<string> = ref('');
    const isSubmitting: Ref<boolean> = ref(false);
    const error: Ref<string | null> = ref<string | null>(null);

    const handleSubmit = async (): Promise<void> => {
      const name: string = orgName.value.trim();
      if (!name) {
        error.value = 'Organization name is required.';
        return;
      }

      error.value = null;
      isSubmitting.value = true;
      try {
        await props.onCreate?.(name);
        orgName.value = '';
      } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to create organization.';
      } finally {
        isSubmitting.value = false;
      }
    };

    return (): VNode | VNode[] | null => {
      if (slots.default) {
        return slots.default({
          error: error.value,
          handleSubmit,
          isSubmitting: isSubmitting.value,
          orgName: orgName.value,
          setOrgName: (v: string) => {
            orgName.value = v;
          },
        });
      }

      return h(Card, {class: [cls(''), props.className].filter(Boolean).join(' ')}, () => [
        h(Typography, {class: cls('__title'), variant: 'h6'}, () => props.title),
        props.description
          ? h(Typography, {class: cls('__description'), variant: 'body2'}, () => props.description)
          : null,
        error.value ? h(Alert, {class: cls('__error'), severity: 'error'}, () => error.value) : null,
        h(TextField, {
          class: cls('__input'),
          label: 'Organization Name',
          modelValue: orgName.value,
          'onUpdate:modelValue': (v: string) => {
            orgName.value = v;
          },
          placeholder: 'Enter organization name',
        }),
        h(
          Button,
          {
            class: cls('__submit'),
            color: 'primary',
            disabled: isSubmitting.value,
            loading: isSubmitting.value,
            onClick: handleSubmit,
            variant: 'solid',
          },
          () => 'Create',
        ),
      ]);
    };
  },
});

export default BaseCreateOrganization;
