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
import Card from '../../primitives/Card';
import {ChevronDownIcon, GlobeIcon} from '../../primitives/Icons';
import type {SelectOption} from '../../primitives/Select/Select';
import Typography from '../../primitives/Typography';

const cls = (name: string): string => withVendorCSSClassPrefix(`language-switcher${name}`);

interface BaseLanguageSwitcherSetupProps {
  className: string;
  currentLanguage: string;
  languages: SelectOption[];
  onLanguageChange?: (lang: string) => void;
}

/**
 * BaseLanguageSwitcher — unstyled language selection component.
 *
 * Shows the current language and a dropdown to select another.
 */
const BaseLanguageSwitcher: Component = defineComponent({
  name: 'BaseLanguageSwitcher',
  props: {
    className: {default: '', type: String},
    currentLanguage: {default: 'en', type: String},
    languages: {default: () => [{label: 'English', value: 'en'}], type: Array as PropType<SelectOption[]>},
    onLanguageChange: {default: undefined, type: Function as PropType<(lang: string) => void>},
  },
  setup(props: BaseLanguageSwitcherSetupProps, {slots}: {slots: any}): () => VNode | VNode[] {
    const isOpen: Ref<boolean> = ref(false);

    const toggle = (): void => {
      isOpen.value = !isOpen.value;
    };

    const handleSelect = (lang: string): void => {
      isOpen.value = false;
      props.onLanguageChange?.(lang);
    };

    return (): VNode | VNode[] => {
      if (slots.default) {
        return slots.default({
          currentLanguage: props.currentLanguage,
          handleSelect,
          isOpen: isOpen.value,
          languages: props.languages,
          toggle,
        });
      }

      const currentLabel: string =
        props.languages.find((l: SelectOption) => l.value === props.currentLanguage)?.label ?? props.currentLanguage;

      const triggerButton: VNode = h(
        'button',
        {
          'aria-expanded': isOpen.value,
          'aria-haspopup': 'listbox',
          class: cls('__trigger'),
          onClick: toggle,
          type: 'button',
        },
        [
          h(GlobeIcon, {size: 16}),
          h(Typography, {class: cls('__trigger-label'), variant: 'body2'}, () => currentLabel),
          h(ChevronDownIcon, {size: 12}),
        ],
      );

      const dropdownItems: VNode[] = props.languages.map((lang: SelectOption) => {
        const isActive: boolean = lang.value === props.currentLanguage;
        return h(
          'button',
          {
            'aria-selected': isActive,
            class: [cls('__item'), isActive ? cls('__item--active') : ''],
            onClick: () => handleSelect(lang.value),
            role: 'option',
            type: 'button',
          },
          [h(Typography, {variant: 'body2'}, () => lang.label)],
        );
      });

      const dropdown: VNode | null = isOpen.value
        ? h('div', {class: cls('__dropdown'), role: 'listbox'}, dropdownItems)
        : null;

      return h(Card, {class: [cls(''), props.className].filter(Boolean).join(' ')}, () => [triggerButton, dropdown]);
    };
  },
});

export default BaseLanguageSwitcher;
