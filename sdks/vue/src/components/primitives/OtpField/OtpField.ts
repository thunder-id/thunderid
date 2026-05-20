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
import {type Component, type Ref, type SetupContext, type VNode, defineComponent, h, nextTick, ref} from 'vue';

type OtpFieldProps = Readonly<{
  disabled: boolean;
  error: string | undefined;
  label: string | undefined;
  length: number;
  modelValue: string;
  name: string | undefined;
  required: boolean;
}>;

const OtpField: Component = defineComponent({
  name: 'OtpField',
  props: {
    disabled: {default: false, type: Boolean},
    error: {default: undefined, type: String},
    label: {default: undefined, type: String},
    length: {default: 6, type: Number},
    modelValue: {default: '', type: String},
    name: {default: undefined, type: String},
    required: {default: false, type: Boolean},
  },
  emits: ['update:modelValue'],
  setup(props: OtpFieldProps, {emit, attrs}: SetupContext): () => VNode {
    const inputRefs: Ref<HTMLInputElement[]> = ref<HTMLInputElement[]>([]);

    const setRef = (el: unknown, index: number): void => {
      if (el) inputRefs.value[index] = el as HTMLInputElement;
    };

    const handleInput = (index: number, e: Event): void => {
      const target: HTMLInputElement = e.target as HTMLInputElement;
      const val: string = target.value.replace(/\D/g, '').slice(0, 1);
      target.value = val;

      const current: string[] = (props.modelValue || '').split('');
      while (current.length < props.length) current.push('');
      current[index] = val;
      emit('update:modelValue', current.join(''));

      if (val && index < props.length - 1) {
        nextTick(() => inputRefs.value[index + 1]?.focus());
      }
    };

    const handleKeydown = (index: number, e: KeyboardEvent): void => {
      if (e.key === 'Backspace' && !(e.target as HTMLInputElement).value && index > 0) {
        nextTick(() => inputRefs.value[index - 1]?.focus());
      }
    };

    return (): VNode => {
      const digits: string[] = (props.modelValue || '').split('');
      while (digits.length < props.length) digits.push('');

      return h(
        'div',
        {
          class: [withVendorCSSClassPrefix('otp-field'), (attrs.class as string) || ''].filter(Boolean).join(' '),
          style: attrs.style,
        },
        [
          props.label
            ? h('label', {class: withVendorCSSClassPrefix('otp-field__label')}, [
                props.label,
                props.required ? h('span', {class: withVendorCSSClassPrefix('otp-field__required')}, ' *') : null,
              ])
            : null,
          h(
            'div',
            {class: withVendorCSSClassPrefix('otp-field__inputs')},
            Array.from({length: props.length}, (_: unknown, i: number) =>
              h('input', {
                'aria-label': `Digit ${i + 1}`,
                class: withVendorCSSClassPrefix('otp-field__digit'),
                disabled: props.disabled,
                inputmode: 'numeric',
                key: i,
                maxlength: 1,
                onInput: (e: Event) => handleInput(i, e),
                onKeydown: (e: KeyboardEvent) => handleKeydown(i, e),
                ref: (el: unknown) => setRef(el, i),
                type: 'text',
                value: digits[i],
              }),
            ),
          ),
          props.error ? h('span', {class: withVendorCSSClassPrefix('otp-field__error')}, props.error) : null,
        ],
      );
    };
  },
});

export default OtpField;
