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

type DividerProps = Readonly<{
  orientation: 'horizontal' | 'vertical';
}>;

const Divider: Component = defineComponent({
  name: 'Divider',
  props: {
    orientation: {
      default: 'horizontal',
      type: String as PropType<'horizontal' | 'vertical'>,
    },
  },
  setup(props: DividerProps, {slots, attrs}: SetupContext): () => VNode {
    return (): VNode => {
      const hasContent = !!slots['default'];
      const cssClass: string = [
        withVendorCSSClassPrefix('divider'),
        withVendorCSSClassPrefix(`divider--${props.orientation}`),
        hasContent ? withVendorCSSClassPrefix('divider--with-content') : '',
        (attrs.class as string) || '',
      ]
        .filter(Boolean)
        .join(' ');

      if (hasContent) {
        return h('div', {class: cssClass, role: 'separator', style: attrs.style}, [
          h('span', {class: withVendorCSSClassPrefix('divider__line')}),
          h('span', {class: withVendorCSSClassPrefix('divider__content')}, slots['default']?.()),
          h('span', {class: withVendorCSSClassPrefix('divider__line')}),
        ]);
      }

      return h('hr', {class: cssClass, role: 'separator', style: attrs.style});
    };
  },
});

export default Divider;
