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

type CardProps = Readonly<{
  variant: 'elevated' | 'outlined' | 'flat';
}>;

const Card: Component = defineComponent({
  name: 'Card',
  props: {
    variant: {
      default: 'elevated',
      type: String as PropType<'elevated' | 'outlined' | 'flat'>,
    },
  },
  setup(props: CardProps, {slots, attrs}: SetupContext): () => VNode {
    return (): VNode =>
      h(
        'div',
        {
          class: [
            withVendorCSSClassPrefix('card'),
            withVendorCSSClassPrefix(`card--${props.variant}`),
            (attrs.class as string) || '',
          ]
            .filter(Boolean)
            .join(' '),
          style: attrs.style,
        },
        slots['default']?.(),
      );
  },
});

export default Card;
