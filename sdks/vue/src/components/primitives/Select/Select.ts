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
import {type Component, type SetupContext, type VNode, defineComponent, h, type PropType} from 'vue';

export interface SelectOption {
  disabled?: boolean;
  label: string;
  value: string;
}

type SelectProps = Readonly<{
  disabled: boolean;
  error: string | undefined;
  helperText: string | undefined;
  label: string | undefined;
  modelValue: string;
  name: string | undefined;
  options: SelectOption[];
  placeholder: string | undefined;
  required: boolean;
}>;

const Select: Component = defineComponent({
  name: 'ThunderIDSelect',
  props: {
    disabled: {default: false, type: Boolean},
    error: {default: undefined, type: String},
    helperText: {default: undefined, type: String},
    label: {default: undefined, type: String},
    modelValue: {default: '', type: String},
    name: {default: undefined, type: String},
    options: {default: () => [], type: Array as PropType<SelectOption[]>},
    placeholder: {default: undefined, type: String},
    required: {default: false, type: Boolean},
  },
  emits: ['update:modelValue'],
  setup(props: SelectProps, {emit, attrs}: SetupContext): () => VNode {
    return (): VNode => {
      const hasError = !!props.error;
      const wrapperClass: string = [
        withVendorCSSClassPrefix('select'),
        hasError ? withVendorCSSClassPrefix('select--error') : '',
        (attrs.class as string) || '',
      ]
        .filter(Boolean)
        .join(' ');

      let helperContent: VNode | null;
      if (hasError) {
        helperContent = h('span', {class: withVendorCSSClassPrefix('select__error')}, props.error);
      } else if (props.helperText) {
        helperContent = h('span', {class: withVendorCSSClassPrefix('select__helper')}, props.helperText);
      } else {
        helperContent = null;
      }

      return h('div', {class: wrapperClass, style: attrs.style}, [
        props.label
          ? h('label', {class: withVendorCSSClassPrefix('select__label'), for: props.name}, [
              props.label,
              props.required ? h('span', {class: withVendorCSSClassPrefix('select__required')}, ' *') : null,
            ])
          : null,
        h(
          'select',
          {
            class: withVendorCSSClassPrefix('select__input'),
            'data-testid': attrs['data-testid'],
            disabled: props.disabled,
            id: props.name,
            name: props.name,
            onChange: (e: Event) => emit('update:modelValue', (e.target as HTMLSelectElement).value),
            required: props.required,
            value: props.modelValue,
          },
          [
            props.placeholder ? h('option', {disabled: true, value: ''}, props.placeholder) : null,
            ...props.options.map((opt: SelectOption) => h('option', {key: opt.value, value: opt.value}, opt.label)),
          ],
        ),
        helperContent,
      ]);
    };
  },
});

export default Select;
