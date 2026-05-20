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
import {type Component, type SetupContext, type VNode, defineComponent, h} from 'vue';

type CheckboxProps = Readonly<{
  disabled: boolean;
  error: string | undefined;
  label: string | undefined;
  modelValue: boolean;
  name: string | undefined;
  required: boolean;
}>;

const Checkbox: Component = defineComponent({
  name: 'ThunderIDCheckbox',
  props: {
    disabled: {default: false, type: Boolean},
    error: {default: undefined, type: String},
    label: {default: undefined, type: String},
    modelValue: {default: false, type: Boolean},
    name: {default: undefined, type: String},
    required: {default: false, type: Boolean},
  },
  emits: ['update:modelValue'],
  setup(props: CheckboxProps, {emit, attrs}: SetupContext): () => VNode {
    return (): VNode => {
      const wrapperClass: string = [
        withVendorCSSClassPrefix('checkbox'),
        props.error ? withVendorCSSClassPrefix('checkbox--error') : '',
        (attrs.class as string) || '',
      ]
        .filter(Boolean)
        .join(' ');

      return h('div', {class: wrapperClass, style: attrs.style}, [
        h('label', {class: withVendorCSSClassPrefix('checkbox__wrapper')}, [
          h('input', {
            checked: props.modelValue,
            class: withVendorCSSClassPrefix('checkbox__input'),
            'data-testid': attrs['data-testid'],
            disabled: props.disabled,
            id: props.name,
            name: props.name,
            onChange: (e: Event) => emit('update:modelValue', (e.target as HTMLInputElement).checked),
            required: props.required,
            type: 'checkbox',
          }),
          props.label ? h('span', {class: withVendorCSSClassPrefix('checkbox__label')}, props.label) : null,
        ]),
        props.error ? h('span', {class: withVendorCSSClassPrefix('checkbox__error')}, props.error) : null,
      ]);
    };
  },
});

export default Checkbox;
