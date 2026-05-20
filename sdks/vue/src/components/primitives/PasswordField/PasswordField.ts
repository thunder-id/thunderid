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
import {type Component, type Ref, type SetupContext, type VNode, defineComponent, h, ref} from 'vue';
import {EyeIcon, EyeOffIcon} from '../Icons';

type PasswordFieldProps = Readonly<{
  disabled: boolean;
  error: string | undefined;
  label: string | undefined;
  modelValue: string;
  name: string | undefined;
  placeholder: string | undefined;
  required: boolean;
}>;

const PasswordField: Component = defineComponent({
  name: 'PasswordField',
  props: {
    disabled: {default: false, type: Boolean},
    error: {default: undefined, type: String},
    label: {default: undefined, type: String},
    modelValue: {default: '', type: String},
    name: {default: undefined, type: String},
    placeholder: {default: undefined, type: String},
    required: {default: false, type: Boolean},
  },
  emits: ['update:modelValue', 'blur'],
  setup(props: PasswordFieldProps, {emit, attrs}: SetupContext): () => VNode {
    const visible: Ref<boolean> = ref(false);

    return (): VNode => {
      const hasError = !!props.error;
      const wrapperClass: string = [
        withVendorCSSClassPrefix('password-field'),
        hasError ? withVendorCSSClassPrefix('password-field--error') : '',
        (attrs.class as string) || '',
      ]
        .filter(Boolean)
        .join(' ');

      return h('div', {class: wrapperClass, style: attrs.style}, [
        props.label
          ? h(
              'label',
              {
                class: withVendorCSSClassPrefix('password-field__label'),
                for: props.name,
              },
              [
                props.label,
                props.required ? h('span', {class: withVendorCSSClassPrefix('password-field__required')}, ' *') : null,
              ],
            )
          : null,
        h('div', {class: withVendorCSSClassPrefix('password-field__wrapper')}, [
          h('input', {
            class: withVendorCSSClassPrefix('password-field__input'),
            'data-testid': attrs['data-testid'],
            disabled: props.disabled,
            id: props.name,
            name: props.name,
            onBlur: () => emit('blur'),
            onInput: (e: Event) => emit('update:modelValue', (e.target as HTMLInputElement).value),
            placeholder: props.placeholder,
            required: props.required,
            type: visible.value ? 'text' : 'password',
            value: props.modelValue,
          }),
          h(
            'button',
            {
              'aria-label': visible.value ? 'Hide password' : 'Show password',
              class: withVendorCSSClassPrefix('password-field__toggle'),
              onClick: () => {
                visible.value = !visible.value;
              },
              tabindex: -1,
              type: 'button',
            },
            visible.value ? EyeOffIcon() : EyeIcon(),
          ),
        ]),
        hasError ? h('span', {class: withVendorCSSClassPrefix('password-field__error')}, props.error) : null,
      ]);
    };
  },
});

export default PasswordField;
